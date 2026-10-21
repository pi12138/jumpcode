package cmd

import (
	"slices"
	"testing"
)

func TestOpenTargetClassification(t *testing.T) {
	for _, input := range []string{".", `..\project`, `.\project`, "../project", "./project", `C:\Users\me\project`, `c:/Users/me/project`, `C:project`, `\\server\share\project`, "project"} {
		if isRemoteInput(input) {
			t.Errorf("本地目录被误判为远程目标: %q", input)
		}
	}
	for _, input := range []string{"devvm:/home/me/project", "me@host:~/project", "devvm:relative"} {
		if !isRemoteInput(input) {
			t.Errorf("远程目标被误判为本地目录: %q", input)
		}
	}
}

func TestShortCommands(t *testing.T) {
	root := newRootCommand()
	for _, tc := range []struct {
		args []string
		name string
	}{
		{[]string{"pair", "create"}, "create"},
		{[]string{"p", "list"}, "list"},
		{[]string{"p", "r"}, "revoke"},
		{[]string{"pair", "join"}, "join"},
		{[]string{"pair", "status"}, "status"},
		{[]string{"c"}, "client"},
		{[]string{"s", "st"}, "status"},
		{[]string{"s", "startup", "install"}, "install"},
	} {
		cmd, remaining, err := root.Find(tc.args)
		if err != nil || cmd.Name() != tc.name || len(remaining) != 0 {
			t.Errorf("别名 %v: cmd=%v err=%v", tc.args, cmd, err)
		}
	}
}

func TestPairCommandsOnlyAtTopLevel(t *testing.T) {
	root := newRootCommand()
	for _, tc := range []struct {
		args   []string
		parent string
	}{
		{[]string{"server", "pair"}, "server"},
		{[]string{"s", "p"}, "server"},
		{[]string{"server", "revoke"}, "server"},
		{[]string{"s", "r"}, "server"},
		{[]string{"client", "pair"}, "client"},
		{[]string{"c", "p"}, "client"},
	} {
		cmd, remaining, err := root.Find(tc.args)
		if err != nil || cmd.Name() != tc.parent || !slices.Equal(remaining, tc.args[1:]) {
			t.Errorf("配对命令不应注册在 %s 下: args=%v cmd=%v remaining=%v err=%v", tc.parent, tc.args, cmd, remaining, err)
		}
	}
}
