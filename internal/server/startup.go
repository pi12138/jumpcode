package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
)

// startInstalledTask 只运行已安装的任务，监听配置由任务定义提供。
func startInstalledTask(dir, name string, runTask func(...string) error, timeout time.Duration) error {
	if err := runTask("/query", "/tn", name); err != nil {
		return i18n.Errorf("登录启动任务未安装或无法查询，请先执行 jc s startup install 或检查任务权限: %w", "Sign-in task is not installed or cannot be queried; run jc s startup install or check task permissions: %w", err)
	}
	state, err := readDaemonState(dir)
	if err == nil && daemonRequest(state, http.MethodGet) == nil {
		if state.Mode != "task" {
			return errors.New(i18n.T("已有普通后台服务运行，请先执行 jc s stop，再执行 jc s start", "A standalone background server is running; run jc s stop before jc s start"))
		}
		fmt.Printf(i18n.T("登录启动服务已在运行：PID %d，%s\n", "Sign-in server is already running: PID %d, %s\n"), state.PID, formatListenAddresses(stateListenAddresses(state)))
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return i18n.Errorf("读取后台状态失败: %w", "Failed to read background state: %w", err)
	}
	if err := config.EnsureDir(dir); err != nil {
		return err
	}
	logPath := filepath.Join(dir, "server.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return i18n.Errorf("打开服务日志失败: %w", "Failed to open server log: %w", err)
	}
	defer f.Close()
	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("请求立即启动登录启动任务 %q", name)
	if err := runTask("/run", "/tn", name); err != nil {
		logger.Printf("登录启动任务启动请求失败: %v", err)
		return i18n.Errorf("启动登录启动任务失败: %w", "Failed to start sign-in task: %w", err)
	}

	// 调度器接受 /run 不代表服务已经可用，必须确认控制接口就绪。
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := readDaemonState(dir)
		if err == nil && state.Mode == "task" && daemonRequestContext(ctx, state, http.MethodGet) == nil {
			logger.Printf("已确认登录启动服务就绪，PID: %d", state.PID)
			fmt.Printf(i18n.T("登录启动服务已启动：PID %d，%s\n日志：%s\n", "Sign-in server started: PID %d, %s\nLog: %s\n"), state.PID, formatListenAddresses(stateListenAddresses(state)), logPath)
			return nil
		}
		select {
		case <-ctx.Done():
			err := i18n.Errorf("登录启动任务已触发，但服务未在 %s 内就绪，请查看日志及任务状态：%s", "Sign-in task was triggered, but the server was not ready within %s; check the log and task status: %s", timeout, logPath)
			logger.Printf("等待登录启动服务失败: %v", err)
			return err
		case <-ticker.C:
		}
	}
}
