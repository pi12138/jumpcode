//go:build windows

package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestStartupTaskAcceptedByWindowsScheduler(t *testing.T) {
	_, sid, err := startupTaskIdentity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	definition, err := startupTaskXML(sid, filepath.Join(dir, "中文 O'Brien", "jc.exe"), powershell, []string{"auto"}, "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "中文任务.xml")
	if err := os.WriteFile(path, definition, 0600); err != nil {
		t.Fatal(err)
	}
	// TASK_VALIDATE_ONLY (1) 使用真实调度器校验 XML，不创建任务或运行程序。
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$xml = [System.IO.File]::ReadAllText(%s)
$service = New-Object -ComObject 'Schedule.Service'
$service.Connect()
$folder = $service.GetFolder('\')
$null = $folder.RegisterTask('Jumpcode XML validation', $xml, 1, $null, $null, 3, $null)
`, psLiteral(path))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPowerShell(script))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		codePage, _ := windows.GetConsoleOutputCP()
		if codePage == 0 {
			codePage = 1
		}
		message, decodeErr := decodeWindowsOutput(output, codePage)
		t.Fatalf("Windows 任务计划程序拒绝生成的 XML: err=%v context=%v output=%q decode=%v", err, ctx.Err(), message, decodeErr)
	}
}
