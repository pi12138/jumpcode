//go:build !windows

package remote

import (
	"errors"
)

func HandleOpenChild(_ []string) {}

func openURI(string) error {
	return errors.New("首版仅支持在 Windows 桌面端打开 VS Code")
}
