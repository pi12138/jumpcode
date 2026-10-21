//go:build windows

package server

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"jumpcode/internal/i18n"
)

func StartBackground(dir string, specs []string) error {
	addresses, err := resolveListenAddresses(specs)
	if err != nil {
		return err
	}
	if state, err := readDaemonState(dir); err == nil {
		if daemonRequest(state, "GET") == nil {
			return i18n.Errorf("后台服务已在 %s 运行", "Background server is already running at %s", formatListenAddresses(stateListenAddresses(state)))
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
	args := []string{"--lang", i18n.Language, "server"}
	for _, address := range addresses {
		args = append(args, "--listen", address)
	}
	args = append(args, "--daemon-child")
	cmd := exec.Command(exe, args...)
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
		if err == nil && state.PID == cmd.Process.Pid && slices.Equal(stateListenAddresses(state), addresses) && daemonRequest(state, "GET") == nil {
			fmt.Printf(i18n.T("后台服务已启动：PID %d，%s\n日志：%s\n", "Background server started: PID %d, %s\nLog: %s\n"), state.PID, formatListenAddresses(addresses), logPath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	cmd.Process.Kill()
	return i18n.Errorf("后台服务未能在 10 秒内启动，请查看日志：%s", "Background server did not start within 10 seconds; check log: %s", logPath)
}
