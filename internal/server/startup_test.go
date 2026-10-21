package server

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestStartupTaskDefinition(t *testing.T) {
	data, err := startupTaskXML("S-1-5-21-123", `C:\Users\测试🐱 O'Brien\工具 & jc.exe`, `C:\系统工具🐱\powershell.exe`, []string{"192.168.56.1:59684", "auto:59685"}, "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0xff, 0xfe}) || len(data)%2 != 0 {
		t.Fatal("任务 XML 文件必须是带 BOM 的 UTF-16LE")
	}
	fileUnits := make([]uint16, (len(data)-2)/2)
	for i := range fileUnits {
		fileUnits[i] = binary.LittleEndian.Uint16(data[2+i*2:])
	}
	text := string(utf16.Decode(fileUnits))
	const header = "<?xml version=\"1.0\" encoding=\"UTF-16\"?>\r\n"
	if !strings.HasPrefix(text, header) {
		t.Fatalf("任务 XML 声明与文件编码不一致: %q", text)
	}
	var task startupTaskDefinition
	// 文件已解码为 UTF-8，去掉原始编码声明后验证 XML 内容。
	if err := xml.Unmarshal([]byte(strings.TrimPrefix(text, header)), &task); err != nil {
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
	if task.Actions.Exec.Command != `C:\系统工具🐱\powershell.exe` {
		t.Fatalf("UTF-16 XML 中的 Unicode 路径损坏: %q", task.Actions.Exec.Command)
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
	if !strings.Contains(script, `& 'C:\Users\测试🐱 O''Brien\工具 & jc.exe' --lang 'zh-CN' server --listen '192.168.56.1:59684' --listen 'auto:59685' --task-run`) || !strings.HasSuffix(script, "; exit $LASTEXITCODE") {
		t.Fatalf("任务命令转义或退出码传递错误: %s", script)
	}
}
