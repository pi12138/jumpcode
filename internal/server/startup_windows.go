//go:build windows

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"

	"golang.org/x/sys/windows"
)

func startupTaskIdentity() (string, string, error) {
	current, err := user.Current()
	if err != nil {
		return "", "", fmt.Errorf("获取当前 Windows 用户失败: %w", err)
	}
	if current.Uid == "" {
		return "", "", errors.New("当前 Windows 用户没有 SID")
	}
	return "Jumpcode Server " + current.Uid, current.Uid, nil
}

func runSchtasks(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "schtasks.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("任务计划程序超时: %w", ctx.Err())
		}
		codePage, _ := windows.GetConsoleOutputCP()
		if codePage == 0 {
			codePage = 1 // CP_OEMCP：无控制台时使用系统 OEM 代码页。
		}
		message, decodeErr := decodeWindowsOutput(output, codePage)
		if decodeErr != nil {
			return fmt.Errorf("任务计划程序失败: %w: 输出解码失败: %v; %q", err, decodeErr, output)
		}
		return fmt.Errorf("任务计划程序失败: %w: %s", err, strings.TrimSpace(message))
	}
	return nil
}

func InstallStartup(specs []string) error {
	specs, err := normalizeListenSpecs(specs)
	if err != nil {
		return err
	}
	name, sid, err := startupTaskIdentity()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取 jc 程序路径失败: %w", err)
	}
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(powershell); err != nil {
		return fmt.Errorf("找不到 Windows PowerShell: %w", err)
	}
	definition, err := startupTaskXML(sid, exe, powershell, specs, i18n.Language)
	if err != nil {
		return err
	}
	dir, err := config.ServerDir()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".jc-task-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(definition); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// 不使用 /f，避免覆盖同名的现有任务。
	if err := runSchtasks("/create", "/tn", name, "/xml", f.Name()); err != nil {
		return err
	}
	fmt.Printf(i18n.T("已安装登录启动任务 %q，监听配置: %s\n下次登录时自动启动；日志: %s\n", "Installed sign-in task %q, listen settings: %s\nIt starts at the next sign-in; log: %s\n"), name, strings.Join(specs, ", "), filepath.Join(dir, "server.log"))
	return nil
}

func StartupStatus() error {
	name, _, err := startupTaskIdentity()
	if err != nil {
		return err
	}
	if err := runSchtasks("/query", "/tn", name); err != nil {
		return i18n.Errorf("登录启动任务未安装或无法查询: %w", "Sign-in task is not installed or cannot be queried: %w", err)
	}
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if err == nil && state.Mode == "task" && daemonRequest(state, "GET") == nil {
		fmt.Printf(i18n.T("登录启动任务已安装；服务运行中：PID %d，%s\n", "Sign-in task installed; server running: PID %d, %s\n"), state.PID, formatListenAddresses(stateListenAddresses(state)))
	} else {
		fmt.Println(i18n.T("登录启动任务已安装；当前没有由任务启动的服务", "Sign-in task installed; no task-managed server is running"))
	}
	fmt.Printf(i18n.T("日志：%s\n", "Log: %s\n"), filepath.Join(dir, "server.log"))
	return nil
}

func UninstallStartup() error {
	name, _, err := startupTaskIdentity()
	if err != nil {
		return err
	}
	if err := runSchtasks("/delete", "/tn", name, "/f"); err != nil {
		return err
	}
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if err == nil && state.Mode == "task" && daemonRequest(state, "GET") == nil {
		if err := Stop(dir); err != nil {
			return i18n.Errorf("已卸载登录启动任务，但停止服务失败: %w", "Sign-in task removed, but stopping the server failed: %w", err)
		}
	}
	fmt.Printf(i18n.T("已卸载登录启动任务 %q\n", "Removed sign-in task %q\n"), name)
	return nil
}

func RunAtLogin(dir string, specs []string) error {
	return runLoggedServer(dir, specs, "task")
}
