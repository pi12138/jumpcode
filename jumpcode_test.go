package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestRemoteTargetParsing(t *testing.T) {
	target, err := parseRemoteTarget("me@192.168.56.10:~/项目 with spaces")
	if err != nil {
		t.Fatal(err)
	}
	if target.Host != "me@192.168.56.10" || target.Path != "~/项目 with spaces" {
		t.Fatalf("解析结果错误: %#v", target)
	}
	for _, value := range []string{"-oProxyCommand=evil:/tmp/a", "devvm:relative", "devvm:/tmp/a\n"} {
		if _, err := parseRemoteTarget(value); err == nil {
			t.Errorf("应拒绝非法目标: %q", value)
		}
	}
}

func TestOpenTargetClassification(t *testing.T) {
	for _, input := range []string{".", `..\project`, `.\project`, "../project", "./project", `C:\Users\me\project`, `c:/Users/me/project`, `C:project`, `\\server\share\project`, "project"} {
		if isRemoteInput(input) {
			t.Errorf("本地目录被误判为远程目标: %q", input)
		}
	}
	for _, input := range []string{"devvm:/home/me/project", "me@host:~/project", "devvm:relative"} {
		if !isRemoteInput(input) {
			t.Errorf("远程目标被误判为本地目录: %q", input)
		}
	}
}

func TestShortCommands(t *testing.T) {
	root := newRootCommand()
	for _, tc := range []struct {
		args []string
		name string
	}{
		{[]string{"c", "p"}, "pair"},
		{[]string{"s", "p"}, "pair"},
		{[]string{"s", "r"}, "revoke"},
		{[]string{"s", "st"}, "status"},
		{[]string{"s", "startup", "install"}, "install"},
	} {
		cmd, _, err := root.Find(tc.args)
		if err != nil || cmd.Name() != tc.name {
			t.Errorf("别名 %v: cmd=%v err=%v", tc.args, cmd, err)
		}
	}
}

func TestStartupTaskDefinition(t *testing.T) {
	data, err := startupTaskXML("S-1-5-21-123", `C:\Users\O'Brien\jc.exe`, `C:\Windows\powershell.exe`, "192.168.56.1:59684", "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	var task startupTaskDefinition
	if err := xml.Unmarshal(data, &task); err != nil {
		t.Fatal(err)
	}
	if task.XMLName.Space != "http://schemas.microsoft.com/windows/2004/02/mit/task" {
		t.Fatalf("任务 XML 命名空间错误: %s", task.XMLName.Space)
	}
	if task.Triggers.LogonTrigger.UserID != "S-1-5-21-123" || task.Principals.Principal.UserID != "S-1-5-21-123" || task.Principals.Principal.LogonType != "InteractiveToken" {
		t.Fatalf("任务未绑定当前用户的交互式登录: %+v", task)
	}
	if task.Settings.ExecutionTimeLimit != "PT0S" || task.Settings.RestartOnFailure.Count != 3 || task.Settings.MultipleInstancesPolicy != "IgnoreNew" {
		t.Fatalf("任务运行策略不正确: %+v", task.Settings)
	}
	if task.Actions.Context != "Author" {
		t.Fatalf("任务动作未绑定用户主体: %+v", task.Actions)
	}
	const prefix = "-NoLogo -NoProfile -NonInteractive -WindowStyle Hidden -EncodedCommand "
	if !strings.HasPrefix(task.Actions.Exec.Arguments, prefix) {
		t.Fatalf("任务未隐藏控制台: %s", task.Actions.Exec.Arguments)
	}
	encoded := strings.TrimPrefix(task.Actions.Exec.Arguments, prefix)
	bytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes)%2 != 0 {
		t.Fatal("PowerShell 命令不是 UTF-16LE")
	}
	units := make([]uint16, len(bytes)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(bytes[i*2:])
	}
	script := string(utf16.Decode(units))
	if !strings.Contains(script, `& 'C:\Users\O''Brien\jc.exe' --lang 'zh-CN' server --listen '192.168.56.1:59684' --task-run`) || !strings.HasSuffix(script, "; exit $LASTEXITCODE") {
		t.Fatalf("任务命令转义或退出码传递错误: %s", script)
	}
}

func TestDaemonStateRejectsNonLocalControl(t *testing.T) {
	dir := t.TempDir()
	if err := writeJSON(statePath(dir), daemonState{PID: 42, Address: "192.168.1.1:1234", Control: "example.com:1234", Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := readDaemonState(dir); err == nil {
		t.Fatal("后台控制地址必须限定在 127.0.0.1")
	}
}

func TestDaemonControlRequiresTokenAndStopsServer(t *testing.T) {
	dir := t.TempDir()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mainServer := &http.Server{}
	done := make(chan error, 1)
	go func() { done <- mainServer.Serve(listener) }()
	cleanup, err := serveDaemonControl(dir, listener.Addr().String(), mainServer)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	state, err := readDaemonState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := daemonRequest(state, http.MethodGet); err != nil {
		t.Fatalf("正确令牌应能查询状态: %v", err)
	}
	state.Token = "wrong"
	if err := daemonRequest(state, http.MethodPost); err == nil {
		t.Fatal("错误令牌不能停止服务")
	}
	state, _ = readDaemonState(dir)
	if err := daemonRequest(state, http.MethodPost); err != nil {
		t.Fatalf("正确令牌应能停止服务: %v", err)
	}
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			t.Fatalf("停止后返回异常: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("停止请求未关闭主服务")
	}
}

func TestVSCodeURLs(t *testing.T) {
	remote, err := url.Parse(remoteVSCodeURL("devvm", "/home/me/项目 #1"))
	if err != nil {
		t.Fatal(err)
	}
	if remote.Scheme != "vscode" || remote.Host != "vscode-remote" || remote.Path != "/ssh-remote+devvm/home/me/项目 #1/" {
		t.Fatalf("远程 URI 错误: %s", remote)
	}
	if !strings.Contains(remote.String(), "%23") {
		t.Fatalf("特殊字符应编码: %s", remote)
	}
	local, err := url.Parse(localVSCodeURL("C:/Users/me/My Project"))
	if err != nil {
		t.Fatal(err)
	}
	if local.Host != "file" || local.Path != "/C:/Users/me/My Project/" {
		t.Fatalf("本地 URI 错误: %s", local)
	}
	if got := localVSCodeURL("C:/"); got != "vscode://file/C:/" {
		t.Fatalf("磁盘根目录 URI 错误: %s", got)
	}
}

func TestPairedClientCanOnlyOpenBoundHost(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()
	certPath, keyPath, fingerprint, err := ensureCertificate(serverDir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := pairClient(serverDir, "vm1", "fixed-host")
	if err != nil {
		t.Fatal(err)
	}
	var opened []remoteTarget
	server := httptest.NewUnstartedServer(newServerHandler(serverDir, func(target remoteTarget) error {
		opened = append(opened, target)
		return nil
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "https://")

	if err := pairWithServer(clientDir, address, fingerprint, token); err != nil {
		t.Fatalf("正确凭据应能配对: %v", err)
	}
	workspace := t.TempDir()
	if err := openViaClient(clientDir, workspace); err != nil {
		t.Fatalf("正确凭据应能打开目录: %v", err)
	}
	if len(opened) != 1 || opened[0].Host != "fixed-host" || opened[0].Path != workspace {
		t.Fatalf("请求必须绑定固定主机和目录: %#v", opened)
	}

	cfg, err := readClientConfig(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	client, err := newPinnedHTTPClient(cfg.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	badRequest, _ := json.Marshal(map[string]string{"path": "/tmp/another", "host": "attacker"})
	resp, err := authorizedRequest(client, cfg, http.MethodPost, "/v1/open", bytes.NewReader(badRequest))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || len(opened) != 1 {
		t.Fatalf("服务端应拒绝自选主机: status=%d opened=%#v", resp.StatusCode, opened)
	}

	if err := revokeClient(serverDir, "vm1"); err != nil {
		t.Fatal(err)
	}
	if err := openViaClient(clientDir, workspace); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("撤销后必须拒绝请求，得到: %v", err)
	}
}

func TestWrongCertificateFingerprintFails(t *testing.T) {
	serverDir := t.TempDir()
	certPath, keyPath, _, err := ensureCertificate(serverDir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(newServerHandler(serverDir, func(remoteTarget) error { return nil }))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "https://")
	_, token, err := pairClient(serverDir, "vm1", "fixed-host")
	if err != nil {
		t.Fatal(err)
	}
	if err := pairWithServer(t.TempDir(), address, strings.Repeat("0", 64), token); err == nil || !strings.Contains(err.Error(), "指纹不匹配") {
		t.Fatalf("错误证书指纹必须拒绝，得到: %v", err)
	}
}

func TestClientConfigIsPrivate(t *testing.T) {
	dir := t.TempDir()
	if err := writeJSON(filepath.Join(dir, "client.json"), clientConfig{Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "client.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("客户端配置权限过宽: %v", info.Mode().Perm())
	}
}
