package remote

import (
	"net/url"
	"strings"
	"testing"
)

func TestRemoteTargetParsing(t *testing.T) {
	target, err := ParseTarget("me@192.168.56.10:~/项目 with spaces")
	if err != nil {
		t.Fatal(err)
	}
	if target.Host != "me@192.168.56.10" || target.Path != "~/项目 with spaces" {
		t.Fatalf("解析结果错误: %#v", target)
	}
	for _, value := range []string{"-oProxyCommand=evil:/tmp/a", "devvm:relative", "devvm:/tmp/a\n"} {
		if _, err := ParseTarget(value); err == nil {
			t.Errorf("应拒绝非法目标: %q", value)
		}
	}
}

func TestVSCodeURLs(t *testing.T) {
	remote, err := url.Parse(remoteVSCodeURL("devvm", "/home/me/项目 #1"))
	if err != nil {
		t.Fatal(err)
	}
	if remote.Scheme != "vscode" || remote.Host != "vscode-remote" || remote.Path != "/ssh-remote+devvm/home/me/项目 #1/" {
		t.Fatalf("远程 URI 错误: %s", remote)
	}
	if !strings.Contains(remote.String(), "%23") {
		t.Fatalf("特殊字符应编码: %s", remote)
	}
	local, err := url.Parse(localVSCodeURL("C:/Users/me/My Project"))
	if err != nil {
		t.Fatal(err)
	}
	if local.Host != "file" || local.Path != "/C:/Users/me/My Project/" {
		t.Fatalf("本地 URI 错误: %s", local)
	}
	if got := localVSCodeURL("C:/"); got != "vscode://file/C:/" {
		t.Fatalf("磁盘根目录 URI 错误: %s", got)
	}
}
