//go:build windows

package i18n

import (
	"syscall"
)

func systemLanguage() string {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	langID, _, _ := proc.Call()
	if langID&0x3ff == 0x04 {
		return "zh-CN"
	}
	return "en-US"
}
