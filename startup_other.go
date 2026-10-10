//go:build !windows

package main

import "errors"

func installStartupTask(string) error {
	return errors.New(tr("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}

func startupTaskStatus() error {
	return errors.New(tr("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}

func uninstallStartupTask() error {
	return errors.New(tr("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}

func runStartupServer(_, _ string) error {
	return errors.New(tr("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}
