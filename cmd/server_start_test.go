package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestServerStartRejectsOverrides(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"s", "start", "extra"}, "extra"},
		{[]string{"s", "start", "--listen", "auto"}, "unknown flag"},
		{[]string{"s", "start", "-d"}, "unknown shorthand flag"},
	} {
		root := newRootCommand()
		root.SetArgs(tc.args)
		root.SetOut(new(bytes.Buffer))
		root.SetErr(new(bytes.Buffer))
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("start 必须在执行前拒绝额外参数: args=%v err=%v", tc.args, err)
		}
	}
}

func TestServerStartRequiresWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 可运行服务端")
	}
	root := newRootCommand()
	root.SetArgs([]string{"s", "start"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "Windows") {
		t.Fatalf("非 Windows 应返回明确的平台限制: %v", err)
	}
}
