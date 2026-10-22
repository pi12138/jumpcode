//go:build windows

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
