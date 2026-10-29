//go:build windows

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"jumpcode/internal/config"

	"golang.org/x/sys/windows"
)

type taskConsoleProbe struct {
	PID        int
	HadConsole bool
	Detached   bool
	Error      string
}

type taskTestProcess struct {
	cmd     *exec.Cmd
	dir     string
	address string
	done    chan struct{}
	err     error
}

// GC 回归测试替换标准流前，保留测试进程最初的标准流，避免干扰测试设施。
var taskTestOriginalStandardFiles [3]*os.File

func startTaskProcess(t *testing.T, mode string) *taskTestProcess {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	dir := t.TempDir()
	if err := os.WriteFile(statePath(dir), []byte("manual-state"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTaskConsoleProcessHelper$")
	cmd.Env = append(os.Environ(), "JC_TEST_TASK_PROCESS="+mode, "JC_TEST_TASK_DIR="+dir, "JC_TEST_TASK_ADDRESS="+address, "GOTRACEBACK=single")
	// 只在隔离的测试子进程中创建、释放控制台，保护 go test 和 CI 的终端。
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	helper := &taskTestProcess{cmd: cmd, dir: dir, address: address, done: make(chan struct{})}
	go func() {
		helper.err = cmd.Wait()
		close(helper.done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-helper.done:
		case <-time.After(3 * time.Second):
			t.Error("任务测试进程未退出")
		}
	})
	return helper
}

func (p *taskTestProcess) probe(t *testing.T) taskConsoleProbe {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(p.dir, "console-probe.json"))
		if err == nil {
			var probe taskConsoleProbe
			if err := json.Unmarshal(data, &probe); err != nil {
				t.Fatal(err)
			}
			if probe.Error != "" {
				t.Fatal(probe.Error)
			}
			return probe
		}
		select {
		case <-p.done:
			// 子进程可能在本轮读取后才完成检查并触发预期的 panic。
			if _, err := os.Stat(filepath.Join(p.dir, "console-probe.json")); err == nil {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(p.dir, "server.log"))
			t.Fatalf("任务进程在控制台检查前退出: %v, 日志: %s", p.err, data)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("等待任务控制台检查超时")
	return taskConsoleProbe{}
}

// 此入口仅由上面的测试进程使用，不会触碰运行 go test 的父进程控制台。
func TestTaskConsoleProcessHelper(t *testing.T) {
	mode := os.Getenv("JC_TEST_TASK_PROCESS")
	if mode == "" {
		return
	}
	dir := os.Getenv("JC_TEST_TASK_DIR")
	var retiredHandles []windows.Handle
	if mode == "gc" {
		// 只在隔离子进程中控制 GC，使句柄复用发生在 finalizer 执行之前。
		debug.SetGCPercent(-1)
		taskTestOriginalStandardFiles = [3]*os.File{os.Stdin, os.Stdout, os.Stderr}
		streams := make([]*os.File, 3)
		for i := range streams {
			file, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			streams[i] = file
			retiredHandles = append(retiredHandles, windows.Handle(file.Fd()))
		}
		os.Stdin, os.Stdout, os.Stderr = streams[0], streams[1], streams[2]
	}
	processList := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	var process [1]uint32
	count, _, _ := processList.Call(uintptr(unsafe.Pointer(&process[0])), 1)
	probe := taskConsoleProbe{PID: os.Getpid(), HadConsole: count > 0}
	go func() {
		for {
			state, err := readControlState(taskStatePath(dir))
			if err == nil && state.PID == os.Getpid() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		count, _, _ := processList.Call(uintptr(unsafe.Pointer(&process[0])), 1)
		probe.Detached = count == 0
		if mode == "gc" {
			if err := checkTaskRetiredHandlesAfterGC(retiredHandles); err != nil {
				probe.Error = err.Error()
			}
		}
		if _, err := fmt.Fprintln(os.Stdout, "task-stdout-test"); err != nil {
			probe.Error = err.Error()
		}
		if _, err := fmt.Fprintln(os.Stderr, "task-stderr-test"); err != nil {
			probe.Error = err.Error()
		}
		log.Print("task-default-log-test")
		handle, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
		if err == nil {
			var written uint32
			err = windows.WriteFile(handle, []byte("task-native-stderr-test\n"), &written, nil)
		}
		if err != nil {
			probe.Error = err.Error()
		}
		if err := config.WriteJSON(filepath.Join(dir, "console-probe.json"), probe); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if mode == "panic" {
			panic("jc-task-runtime-panic-test")
		}
	}()
	if err := RunTask(dir, []string{os.Getenv("JC_TEST_TASK_ADDRESS")}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

// 用空设备模拟旧标准流，让原生 CloseHandle 模拟 FreeConsole 释放句柄。
// 再让事件对象复用这些编号，验证 GC 不会通过旧 os.File 误关新对象。
func checkTaskRetiredHandlesAfterGC(retired []windows.Handle) error {
	for _, handle := range retired {
		if err := windows.CloseHandle(handle); err != nil {
			return fmt.Errorf("释放旧标准流句柄失败: %w", err)
		}
	}
	var events []windows.Handle
	defer func() {
		for _, event := range events {
			windows.CloseHandle(event)
		}
	}()
	reused := make(map[windows.Handle]bool)
	for range 4096 {
		event, err := windows.CreateEvent(nil, 1, 0, nil)
		if err != nil {
			return fmt.Errorf("创建句柄复用测试事件失败: %w", err)
		}
		events = append(events, event)
		for _, handle := range retired {
			if event == handle {
				reused[event] = true
			}
		}
		if len(reused) == len(retired) {
			break
		}
	}
	if len(reused) != len(retired) {
		return fmt.Errorf("未能复用全部旧标准流句柄: %d/%d", len(reused), len(retired))
	}
	for range 10 {
		runtime.GC()
		time.Sleep(10 * time.Millisecond) // finalizer 异步执行，留出运行机会。
	}
	for event := range reused {
		result, err := windows.WaitForSingleObject(event, 0)
		if err != nil || result != uint32(windows.WAIT_TIMEOUT) {
			return fmt.Errorf("GC 后复用的句柄 %d 已失效: result=%d, err=%v", event, result, err)
		}
	}
	return nil
}

func TestTaskConsoleReleasedHandlesSurviveGC(t *testing.T) {
	helper := startTaskProcess(t, "gc")
	probe := helper.probe(t)
	if !probe.HadConsole || !probe.Detached {
		t.Fatalf("测试进程应先创建再脱离控制台: %+v", probe)
	}
	state, err := readControlState(taskStatePath(helper.dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := daemonRequest(state, http.MethodGet); err != nil {
		t.Fatalf("强制 GC 后服务必须仍可响应: %v", err)
	}
	if err := daemonRequest(state, http.MethodPost); err != nil {
		t.Fatal(err)
	}
	select {
	case <-helper.done:
		if helper.err != nil {
			t.Fatalf("GC 后服务必须正常退出: %v", helper.err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("GC 后任务进程未停止")
	}
}

func TestTaskRuntimePanicIsLoggedAfterConsoleDetach(t *testing.T) {
	helper := startTaskProcess(t, "panic")
	probe := helper.probe(t)
	if !probe.HadConsole || !probe.Detached {
		t.Fatalf("测试进程应先创建再脱离控制台: %+v", probe)
	}
	select {
	case <-helper.done:
		if helper.err == nil {
			t.Fatal("panic 必须返回非零退出码")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("panic 后任务进程未退出")
	}
	data, err := os.ReadFile(filepath.Join(helper.dir, "server.log"))
	if err != nil || !strings.Contains(string(data), "panic: jc-task-runtime-panic-test") || !strings.Contains(string(data), "goroutine") {
		t.Fatalf("脱离控制台后必须保留 panic 和调用栈: %s, %v", data, err)
	}
}
