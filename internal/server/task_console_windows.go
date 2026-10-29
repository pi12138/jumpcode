//go:build windows

package server

import (
	"log"
	"os"
	"syscall"

	"jumpcode/internal/i18n"

	"golang.org/x/sys/windows"
)

var taskFreeConsole = windows.NewLazySystemDLL("kernel32.dll").NewProc("FreeConsole")

// FreeConsole 可能直接关闭控制台句柄，但旧 os.File 的 finalizer 仍会再次关闭
// 同一编号。Windows 复用该编号后，GC 就可能误关日志、socket 或 runtime IOCP。
// 必须保留这些 Go 对象直到进程退出，不能在脱离后对它们调用 Close。
var taskDetachedStandardFiles []*os.File

// detachTaskConsole 只用于计划任务进程。FreeConsole 不改变 PID；脱离后关闭
// 原控制台不会终止服务。普通命令与前台 server 不调用此函数。
func detachTaskConsole(logFile *os.File) error {
	// 单独持有标准流的文件句柄，服务返回后 Cobra 和 Go runtime 仍能记录错误。
	output, err := os.OpenFile(logFile.Name(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return i18n.Errorf("打开计划任务输出日志失败: %w", "Failed to open task output log: %w", err)
	}
	input, err := os.Open(os.DevNull)
	if err != nil {
		output.Close()
		return i18n.Errorf("打开计划任务空输入失败: %w", "Failed to open task null input: %w", err)
	}
	ready := false
	defer func() {
		if !ready {
			input.Close()
			output.Close()
		}
	}()

	// 不通过隐藏窗口来模拟后台运行；同时兼容经典控制台与 Windows Terminal。
	// FreeConsole 在没有关联控制台时也成功，启动瞬间的闪窗仍取决于 Windows。
	taskDetachedStandardFiles = append(taskDetachedStandardFiles, os.Stdin, os.Stdout, os.Stderr)
	result, _, callErr := taskFreeConsole.Call()
	if result == 0 {
		if callErr == syscall.Errno(0) {
			callErr = windows.ERROR_GEN_FAILURE
		}
		return i18n.Errorf("脱离计划任务控制台失败: %w", "Failed to detach the task console: %w", callErr)
	}
	for _, stream := range []struct {
		id   uint32
		file *os.File
	}{
		{windows.STD_INPUT_HANDLE, input},
		{windows.STD_OUTPUT_HANDLE, output},
		{windows.STD_ERROR_HANDLE, output},
	} {
		if err := windows.SetStdHandle(stream.id, windows.Handle(stream.file.Fd())); err != nil {
			return i18n.Errorf("设置计划任务标准流 %d 失败: %w", "Failed to set task standard stream %d: %w", stream.id, err)
		}
	}
	os.Stdin, os.Stdout, os.Stderr = input, output, output
	log.SetOutput(output)
	ready = true
	return nil
}
