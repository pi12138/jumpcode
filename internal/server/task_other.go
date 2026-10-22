//go:build !windows

package server

import "os"

func connectTaskScheduler() (taskScheduler, error) {
	return nil, CheckPlatform()
}

func detachTaskConsole(_ *os.File) error {
	return CheckPlatform()
}
