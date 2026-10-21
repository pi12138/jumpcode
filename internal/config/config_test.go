package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClientConfigIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 使用 ACL 管理访问权限，不使用 Unix 文件权限位")
	}
	dir := t.TempDir()
	if err := WriteJSON(filepath.Join(dir, "client.json"), Client{Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "client.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("客户端配置权限过宽: %v", info.Mode().Perm())
	}
}
