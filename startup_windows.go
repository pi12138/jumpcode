//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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
		return fmt.Errorf("任务计划程序失败: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func installStartupTask(address string) error {
	if err := validateListenAddress(address); err != nil {
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
	definition, err := startupTaskXML(sid, exe, powershell, address, language)
	if err != nil {
		return err
	}
	dir, err := serverConfigDir()
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
	fmt.Printf(tr("已安装登录启动任务 %q，监听地址: %s\n下次登录时自动启动；日志: %s\n", "Installed sign-in task %q, listen address: %s\nIt starts at the next sign-in; log: %s\n"), name, address, filepath.Join(dir, "server.log"))
	return nil
}

func startupTaskStatus() error {
	name, _, err := startupTaskIdentity()
	if err != nil {
		return err
	}
	if err := runSchtasks("/query", "/tn", name); err != nil {
		return errorf("登录启动任务未安装或无法查询: %w", "Sign-in task is not installed or cannot be queried: %w", err)
	}
	dir, err := configDir()
	if err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if err == nil && state.Mode == "task" && daemonRequest(state, "GET") == nil {
		fmt.Printf(tr("登录启动任务已安装；服务运行中：PID %d，https://%s\n", "Sign-in task installed; server running: PID %d, https://%s\n"), state.PID, state.Address)
	} else {
		fmt.Println(tr("登录启动任务已安装；当前没有由任务启动的服务", "Sign-in task installed; no task-managed server is running"))
	}
	fmt.Printf(tr("日志：%s\n", "Log: %s\n"), filepath.Join(dir, "server.log"))
	return nil
}

func uninstallStartupTask() error {
	name, _, err := startupTaskIdentity()
	if err != nil {
		return err
	}
	if err := runSchtasks("/delete", "/tn", name, "/f"); err != nil {
		return err
	}
	dir, err := configDir()
	if err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if err == nil && state.Mode == "task" && daemonRequest(state, "GET") == nil {
		if err := stopDaemon(dir); err != nil {
			return errorf("已卸载登录启动任务，但停止服务失败: %w", "Sign-in task removed, but stopping the server failed: %w", err)
		}
	}
	fmt.Printf(tr("已卸载登录启动任务 %q\n", "Removed sign-in task %q\n"), name)
	return nil
}

func runStartupServer(dir, address string) error {
	if err := ensureConfigDir(dir); err != nil {
		return err
	}
	logPath := filepath.Join(dir, "server.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("打开服务日志失败: %w", err)
	}
	defer f.Close()
	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("登录启动服务，监听地址: %s，PID: %d", address, os.Getpid())
	err = runServerModeWithLog(dir, address, true, logger)
	if err != nil {
		logger.Printf("服务退出，错误: %v", err)
		return err
	}
	logger.Print("服务已正常停止")
	return nil
}
