//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func startDaemon(dir, address string) error {
	if err := validateListenAddress(address); err != nil {
		return err
	}
	if state, err := readDaemonState(dir); err == nil {
		if daemonRequest(state, "GET") == nil {
			return errorf("后台服务已在 https://%s 运行", "Background server is already running at https://%s", state.Address)
		}
	}
	if err := os.Remove(statePath(dir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(dir, "server.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(exe, "--lang", language, "server", "--listen", address, "--daemon-child")
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Release()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		state, err := readDaemonState(dir)
		if err == nil && state.PID == cmd.Process.Pid && state.Address == address && daemonRequest(state, "GET") == nil {
			fmt.Printf(tr("后台服务已启动：PID %d，https://%s\n日志：%s\n", "Background server started: PID %d, https://%s\nLog: %s\n"), state.PID, address, logPath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	cmd.Process.Kill()
	return errorf("后台服务未能在 10 秒内启动，请查看日志：%s", "Background server did not start within 10 seconds; check log: %s", logPath)
}
