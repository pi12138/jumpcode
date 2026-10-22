package cmd

import (
	"bytes"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestTaskPublicCommands(t *testing.T) {
	cmd := newTaskCommand()
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
		if sub.Name() != "install" && sub.Flags().Lookup("listen") != nil {
			t.Fatalf("%s 不应允许覆盖已安装的监听配置", sub.Name())
		}
	}
	if !slices.Equal(names, []string{"install", "start", "status", "stop", "uninstall"}) {
		t.Fatalf("任务命令仅应公开五个子命令: %v", names)
	}
	for _, name := range []string{"daemon-child", "task-run"} {
		if flag := newServerCommand().Flags().Lookup(name); flag == nil || !flag.Hidden {
			t.Fatalf("内部参数必须隐藏: %s", name)
		}
	}
}

func TestServerRejectsMixedRunModes(t *testing.T) {
	for _, flags := range [][]string{{"-d", "--task-run"}, {"--daemon-child", "--task-run"}, {"-d", "--daemon-child"}} {
		cmd := newRootCommand()
		cmd.SetArgs(append([]string{"server"}, flags...))
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "参数不能组合") && !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("运行模式应互斥: %v, %v", flags, err)
		}
	}
}

func TestTaskCommandsRejectNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("非 Windows 平台检查")
	}
	for _, action := range []string{"install", "uninstall", "start", "stop", "status"} {
		cmd := newRootCommand()
		cmd.SetArgs([]string{"task", action})
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "Windows") {
			t.Fatalf("%s 应说明仅支持 Windows: %v", action, err)
		}
	}
}
