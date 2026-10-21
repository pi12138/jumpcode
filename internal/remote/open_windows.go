//go:build windows

package remote

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func openURI(uri string) error {
	if _, err := windows.UTF16PtrFromString(uri); err != nil {
		return fmt.Errorf("VS Code URI 无效: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取 jc 程序路径失败: %w", err)
	}
	cmd := exec.Command(exe, "--open-uri-child", uri)
	// 子进程没有控制台；默认的标准输入输出连接到空设备。
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() > 1 {
			return fmt.Errorf("无法通过 vscode:// 打开 VS Code，请确认已安装桌面版并注册协议: %w", syscall.Errno(exit.ExitCode()))
		}
		return fmt.Errorf("无法通过 vscode:// 打开 VS Code，请确认已安装桌面版并注册协议: %w", err)
	}
	return nil
}

func HandleOpenChild(args []string) {
	if len(args) == 0 || args[0] != "--open-uri-child" {
		return
	}
	if len(args) != 2 || !strings.HasPrefix(args[1], "vscode://") {
		os.Exit(1)
	}
	if err := shellExecuteURI(args[1]); err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && errno != 0 {
			os.Exit(int(errno))
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func shellExecuteURI(uri string) error {
	target, err := windows.UTF16PtrFromString(uri)
	if err != nil {
		return err
	}
	// 使用 Windows 已注册的 vscode:// 协议，不经过 cmd.exe 或 code.cmd。
	return windows.ShellExecute(0, nil, target, nil, nil, 1)
}
