package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
)

func pairCommandConfigDir(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("APPDATA", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func executePairCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	language := i18n.Language
	i18n.Language = "zh-CN"
	t.Cleanup(func() { i18n.Language = language })
	root := newRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	err := root.Execute()
	return output.String(), err
}

func TestPairListCommand(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("配对命令仅支持 Linux 和 Windows")
	}
	for _, paired := range []bool{false, true} {
		name := "未配对"
		if paired {
			name = "已配对"
		}
		t.Run(name, func(t *testing.T) {
			dir := pairCommandConfigDir(t)
			const token = "secret-pairing-token"
			const tokenHash = "secret-token-hash"
			if paired {
				if err := config.WriteServer(dir, config.Server{Clients: []config.PairedClient{
					{Name: "devvm", SSHHost: "me@192.168.56.10", TokenHash: tokenHash},
				}}); err != nil {
					t.Fatal(err)
				}
				if err := config.WriteJSON(filepath.Join(dir, "client.json"), config.Client{
					Server: "192.168.56.1:59684", Fingerprint: strings.Repeat("a", 64), Token: token,
				}); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range []string{"pair", "p"} {
				output, err := executePairCommand(t, command, "list")
				if err != nil {
					t.Fatalf("%s list 执行失败: %v", command, err)
				}
				if strings.Contains(output, token) || strings.Contains(output, tokenHash) {
					t.Fatalf("列表泄漏凭据: %q", output)
				}
				if !paired {
					if !strings.Contains(output, "暂无配对凭据") {
						t.Fatalf("未配对时输出错误: %q", output)
					}
					if _, err := os.Stat(dir); !os.IsNotExist(err) {
						t.Fatalf("查看列表不应创建配置目录: %v", err)
					}
				} else if runtime.GOOS == "linux" {
					if !strings.Contains(output, "192.168.56.1:59684") || !strings.Contains(output, strings.Repeat("a", 64)) || strings.Contains(output, "devvm") {
						t.Fatalf("Linux 应列出客户端保存的配对: %q", output)
					}
				} else if !strings.Contains(output, "devvm") || !strings.Contains(output, "me@192.168.56.10") || strings.Contains(output, "192.168.56.1:59684") {
					t.Fatalf("Windows 应列出服务端签发的配对: %q", output)
				}
			}
		})
	}
}

func TestPairListReportsInvalidConfig(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("配对命令仅支持 Linux 和 Windows")
	}
	dir := pairCommandConfigDir(t)
	if err := config.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	name := "server.json"
	if runtime.GOOS == "linux" {
		name = "client.json"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := executePairCommand(t, "pair", "list")
	if err == nil || !strings.Contains(err.Error(), "解析") || strings.Contains(output, "暂无配对凭据") {
		t.Fatalf("配置损坏应明确报错: output=%q err=%v", output, err)
	}
}

func TestPairCommandPlatformErrors(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("此测试覆盖 Linux 和 Windows 端的误用")
	}
	dir := pairCommandConfigDir(t)
	cases := [][]string{
		{"create", "--name", "devvm", "--ssh-host", "devvm"},
		{"revoke", "--name", "devvm"},
	}
	want := "Windows 服务端"
	if runtime.GOOS == "windows" {
		cases = [][]string{
			{"join", "--server", "192.168.56.1:59684", "--fingerprint", strings.Repeat("a", 64)},
			{"status"},
		}
		want = "Linux 客户端"
	}
	for _, args := range cases {
		t.Run(args[0], func(t *testing.T) {
			_, err := executePairCommand(t, append([]string{"pair"}, args...)...)
			if err == nil || !strings.Contains(err.Error(), "jc pair "+args[0]) || !strings.Contains(err.Error(), want) {
				t.Fatalf("平台错误应包含实际配对命令和执行位置: %v", err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("不支持的平台不应创建配置目录: %v", err)
			}
		})
	}
}

func TestPairStatusCommandReadsLocalConfig(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pair status 在 Linux 客户端运行")
	}
	dir := pairCommandConfigDir(t)
	output, err := executePairCommand(t, "pair", "status")
	if err != nil || !strings.Contains(output, "暂无配对凭据") {
		t.Fatalf("未配对时状态输出错误: output=%q err=%v", output, err)
	}
	// 不可用的地址也应能离线查看，且不能将不完整配置误报为未配对。
	if err := config.WriteJSON(filepath.Join(dir, "client.json"), config.Client{
		Server: "192.0.2.1:59684", Fingerprint: strings.Repeat("a", 64), Token: "secret-pairing-token",
	}); err != nil {
		t.Fatal(err)
	}
	output, err = executePairCommand(t, "pair", "status")
	if err != nil || !strings.Contains(output, "192.0.2.1:59684") || strings.Contains(output, "secret-pairing-token") {
		t.Fatalf("应离线显示本地配对且隐藏密钥: output=%q err=%v", output, err)
	}
	if err := config.WriteJSON(filepath.Join(dir, "client.json"), config.Client{}); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"list", "status"} {
		output, err = executePairCommand(t, "pair", command)
		if err == nil || !strings.Contains(err.Error(), "配置不完整") || strings.Contains(output, "暂无配对凭据") {
			t.Fatalf("配置不完整应明确报错: output=%q err=%v", output, err)
		}
	}
}

func TestPairManagementOutput(t *testing.T) {
	serverDir := t.TempDir()
	const tokenHash = "secret-token-hash"
	if err := config.WriteServer(serverDir, config.Server{Clients: []config.PairedClient{
		{Name: "devvm", SSHHost: "me@192.168.56.10", TokenHash: tokenHash},
	}}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := listPairs(&output, serverDir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "devvm") || !strings.Contains(output.String(), "me@192.168.56.10") || strings.Contains(output.String(), tokenHash) {
		t.Fatalf("配对列表内容错误或泄漏哈希: %q", output.String())
	}
	output.Reset()
	if err := listPairs(&output, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "暂无配对") && !strings.Contains(output.String(), "No pairings") {
		t.Fatalf("空配对列表输出错误: %q", output.String())
	}

	clientDir := t.TempDir()
	const token = "secret-pairing-token"
	if err := config.WriteJSON(filepath.Join(clientDir, "client.json"), config.Client{
		Server: "192.168.56.1:59684", Fingerprint: strings.Repeat("a", 64), Token: token,
	}); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := pairStatus(&output, clientDir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "192.168.56.1:59684") || !strings.Contains(output.String(), strings.Repeat("a", 64)) || strings.Contains(output.String(), token) {
		t.Fatalf("配对状态内容错误或泄漏密钥: %q", output.String())
	}
}
