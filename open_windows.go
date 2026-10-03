//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func openURI(uri string) error {
	target, err := windows.UTF16PtrFromString(uri)
	if err != nil {
		return fmt.Errorf("VS Code URI 无效: %w", err)
	}
	// 使用 Windows 已注册的 vscode:// 协议，不经过 cmd.exe 或 code.cmd。
	if err := windows.ShellExecute(0, nil, target, nil, nil, 1); err != nil {
		return fmt.Errorf("无法通过 vscode:// 打开 VS Code，请确认已安装桌面版并注册协议: %w", err)
	}
	return nil
}
