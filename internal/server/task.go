package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
)

type scheduledTask struct {
	State      int
	LastResult uint32
}

func (s scheduledTask) active() bool { return s.State == 2 || s.State == 4 }

// 所有进程的创建都交给计划任务；停止通过服务自己的控制端点完成。
type taskScheduler interface {
	Name() string
	Query() (scheduledTask, error)
	Install(executable, workingDir, language string, specs []string) error
	Start() error
	Delete() error
	Close()
}

func taskStatePath(dir string) string { return filepath.Join(dir, "task.json") }

// RunTask 在计划任务启动的当前进程中直接运行服务。
func RunTask(dir string, specs []string) error {
	return withServerLog(dir, specs, func(logger *log.Logger) error {
		logger.Print("运行模式：Windows 计划任务")
		output, ok := logger.Writer().(*os.File)
		if !ok {
			return errors.New(i18n.T("计划任务日志未连接到文件", "Task log is not connected to a file"))
		}
		if err := detachTaskConsole(output); err != nil {
			return err
		}
		logger.Print("已脱离控制台，标准输出和错误输出已连接到服务日志")
		if state, err := readControlState(taskStatePath(dir)); err == nil && daemonRequest(state, http.MethodGet) == nil {
			return errors.New(i18n.T("计划任务服务已在运行", "Task server is already running"))
		}
		return runServerWithLog(dir, specs, taskStatePath(dir), logger)
	})
}

// ManageTask 为 jc task 的五个命令提供统一的日志与平台检查。
func ManageTask(dir, action string, specs []string) error {
	if runtime.GOOS != "windows" {
		return errors.New(i18n.T("jc task 任务管理仅支持 Windows", "jc task management supports Windows only"))
	}
	return withTaskLog(dir, action, func(logger *log.Logger) error {
		scheduler, err := connectTaskScheduler()
		if err != nil {
			return err
		}
		defer scheduler.Close()
		manager := taskManager{dir: dir, scheduler: scheduler, timeout: 10 * time.Second}
		if err := manager.execute(action, specs); err != nil {
			return err
		}
		logger.Printf("计划任务 %s 操作完成：%s", scheduler.Name(), action)
		return nil
	})
}

func withTaskLog(dir, action string, run func(*log.Logger) error) error {
	if err := config.EnsureDir(dir); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return i18n.Errorf("打开服务日志失败: %w", "Failed to open server log: %w", err)
	}
	defer f.Close()
	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("计划任务管理请求：%s，PID: %d", action, os.Getpid())
	if err := run(logger); err != nil {
		logger.Printf("计划任务 %s 失败：%v", action, err)
		return err
	}
	return nil
}

type taskManager struct {
	dir       string
	scheduler taskScheduler
	timeout   time.Duration
}

func (m taskManager) execute(action string, specs []string) error {
	if action == "install" {
		return m.install(specs)
	}
	if action == "status" {
		return m.status()
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	switch action {
	case "start":
		return m.start(ctx)
	case "stop":
		return m.stop(ctx)
	case "uninstall":
		if err := m.stop(ctx); err != nil {
			return err
		}
		if err := m.scheduler.Delete(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fmt.Println(i18n.T("计划任务已卸载，凭据和日志已保留", "Task uninstalled; credentials and logs retained"))
		return nil
	default:
		return i18n.Errorf("未知任务操作 %q", "Unknown task action %q", action)
	}
}

func (m taskManager) install(specs []string) error {
	specs, err := normalizeListenSpecs(specs)
	if err != nil {
		return err
	}
	if _, err := m.scheduler.Query(); err == nil {
		return errors.New(i18n.T("计划任务已存在，请先卸载再安装以更新配置", "Task already exists; uninstall it before installing a new configuration"))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	if err := m.scheduler.Install(executable, filepath.Dir(executable), i18n.Language, specs); err != nil {
		return err
	}
	fmt.Printf(i18n.T("计划任务已安装：%s\n下次登录时自动启动；立即启动请执行 jc task start\n", "Task installed: %s\nStarts at next sign-in; run jc task start to start now\n"), m.scheduler.Name())
	return nil
}

func (m taskManager) start(ctx context.Context) error {
	task, err := m.scheduler.Query()
	if errors.Is(err, os.ErrNotExist) {
		return errors.New(i18n.T("计划任务未安装，请先执行 jc task install", "Task is not installed; run jc task install first"))
	}
	if err != nil {
		return err
	}
	if !task.active() {
		if err := m.scheduler.Start(); err != nil {
			return err
		}
	}
	for {
		task, err := m.scheduler.Query()
		if err != nil {
			return err
		}
		state, checkErr := readControlState(taskStatePath(m.dir))
		if checkErr == nil {
			checkErr = controlRequest(ctx, state, http.MethodGet)
		}
		if checkErr == nil && task.State == 4 && len(stateListenAddresses(state)) > 0 {
			fmt.Printf(i18n.T("计划任务服务已就绪：PID %d，%s\n日志：%s\n", "Task server ready: PID %d, %s\nLog: %s\n"), state.PID, formatListenAddresses(stateListenAddresses(state)), filepath.Join(m.dir, "server.log"))
			return nil
		}
		if checkErr == nil && len(stateListenAddresses(state)) == 0 {
			checkErr = errors.New(formatListenAddresses(nil))
		}
		if err := taskPoll(ctx); err != nil {
			return i18n.Errorf("计划任务已请求启动，但服务未在等待时间内就绪（可能仍在启动或重试）；调度器状态 %d，上次结果 0x%08X，控制接口检查: %v；日志：%s: %w", "Task start requested, but the server was not ready in time (it may still be starting or retrying); scheduler state %d, last result 0x%08X, control check: %v; log: %s: %w", task.State, task.LastResult, checkErr, filepath.Join(m.dir, "server.log"), err)
		}
	}
}

func (m taskManager) stop(ctx context.Context) error {
	stopSent := false
	for {
		task, queryErr := m.scheduler.Query()
		if queryErr != nil && !errors.Is(queryErr, os.ErrNotExist) {
			return queryErr
		}
		state, stateErr := readControlState(taskStatePath(m.dir))
		if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
			return stateErr
		}
		alive := stateErr == nil && controlRequest(ctx, state, http.MethodGet) == nil
		if !alive && (errors.Is(queryErr, os.ErrNotExist) || !task.active()) {
			fmt.Println(i18n.T("计划任务服务已停止", "Task server stopped"))
			return nil
		}
		if alive && !stopSent {
			if err := controlRequest(ctx, state, http.MethodPost); err != nil {
				return i18n.Errorf("停止计划任务服务失败: %w", "Failed to stop task server: %w", err)
			}
			stopSent = true
		}
		if err := taskPoll(ctx); err != nil {
			return i18n.Errorf("计划任务服务未在等待时间内停止，请查看日志：%s: %w", "Task server did not stop in time; check log: %s: %w", filepath.Join(m.dir, "server.log"), err)
		}
	}
}

func taskPoll(ctx context.Context) error {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (m taskManager) status() error {
	fmt.Printf(i18n.T("任务：%s\n日志：%s\n", "Task: %s\nLog: %s\n"), m.scheduler.Name(), filepath.Join(m.dir, "server.log"))
	task, err := m.scheduler.Query()
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println(i18n.T("计划任务未安装", "Task is not installed"))
	} else if err != nil {
		return err
	} else {
		names := []string{i18n.T("未知", "Unknown"), i18n.T("已禁用", "Disabled"), i18n.T("排队中", "Queued"), i18n.T("就绪（未运行）", "Ready (stopped)"), i18n.T("运行中", "Running")}
		state := names[0]
		if task.State >= 0 && task.State < len(names) {
			state = names[task.State]
		}
		fmt.Printf(i18n.T("调度器状态：%s\n上次结果：0x%08X\n", "Scheduler state: %s\nLast result: 0x%08X\n"), state, task.LastResult)
	}
	state, stateErr := readControlState(taskStatePath(m.dir))
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		return stateErr
	}
	if stateErr == nil && daemonRequest(state, http.MethodGet) == nil {
		if len(stateListenAddresses(state)) == 0 {
			fmt.Printf(i18n.T("服务运行中但尚未就绪：PID %d，%s\n", "Server running but not ready: PID %d, %s\n"), state.PID, formatListenAddresses(nil))
		} else {
			fmt.Printf(i18n.T("服务已就绪：PID %d，%s\n", "Server ready: PID %d, %s\n"), state.PID, formatListenAddresses(stateListenAddresses(state)))
		}
	} else {
		fmt.Println(i18n.T("服务未就绪（未运行或正在启动）", "Server is not ready (stopped or starting)"))
	}
	return nil
}
