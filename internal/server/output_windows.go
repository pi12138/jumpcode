//go:build windows

package server

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

// decodeWindowsOutput 将原生命令的代码页输出转成 Go 使用的 UTF-8。
func decodeWindowsOutput(output []byte, codePage uint32) (string, error) {
	if utf8.Valid(output) {
		return string(output), nil
	}
	count, err := windows.MultiByteToWideChar(codePage, 0, &output[0], int32(len(output)), nil, 0)
	if err != nil {
		return "", fmt.Errorf("代码页 %d: %w", codePage, err)
	}
	units := make([]uint16, count)
	if _, err := windows.MultiByteToWideChar(codePage, 0, &output[0], int32(len(output)), &units[0], count); err != nil {
		return "", fmt.Errorf("代码页 %d: %w", codePage, err)
	}
	return windows.UTF16ToString(units), nil
}
