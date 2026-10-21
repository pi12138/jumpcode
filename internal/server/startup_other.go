//go:build !windows

package server

import (
	"errors"

	"jumpcode/internal/i18n"
)

func Start(string) error {
	return CheckPlatform()
}

func InstallStartup([]string) error {
	return errors.New(i18n.T("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}

func StartupStatus() error {
	return errors.New(i18n.T("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}

func UninstallStartup() error {
	return errors.New(i18n.T("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}

func RunAtLogin(_ string, _ []string) error {
	return errors.New(i18n.T("登录启动任务仅支持 Windows", "Sign-in tasks are supported on Windows only"))
}
