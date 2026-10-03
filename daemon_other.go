//go:build !windows

package main

import "errors"

func startDaemon(_, _ string) error {
	return errors.New(tr("后台服务仅支持 Windows", "Background server supports Windows only"))
}
