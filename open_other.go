//go:build !windows

package main

import "errors"

func openURI(string) error {
	return errors.New("首版仅支持在 Windows 桌面端打开 VS Code")
}
