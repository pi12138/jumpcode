//go:build !windows

package server

import (
	"errors"

	"jumpcode/internal/i18n"
)

func StartBackground(_ string, _ []string) error {
	return errors.New(i18n.T("后台服务仅支持 Windows", "Background server supports Windows only"))
}
