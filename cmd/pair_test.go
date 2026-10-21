package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"jumpcode/internal/config"
)

func TestPairManagementOutput(t *testing.T) {
	serverDir := t.TempDir()
	const tokenHash = "secret-token-hash"
	if err := config.WriteServer(serverDir, config.Server{Clients: []config.PairedClient{
		{Name: "devvm", SSHHost: "me@192.168.56.10", TokenHash: tokenHash},
	}}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := listPairs(&output, serverDir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "devvm") || !strings.Contains(output.String(), "me@192.168.56.10") || strings.Contains(output.String(), tokenHash) {
		t.Fatalf("配对列表内容错误或泄漏哈希: %q", output.String())
	}
	output.Reset()
	if err := listPairs(&output, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "暂无配对") && !strings.Contains(output.String(), "No pairings") {
		t.Fatalf("空配对列表输出错误: %q", output.String())
	}

	clientDir := t.TempDir()
	const token = "secret-pairing-token"
	if err := config.WriteJSON(filepath.Join(clientDir, "client.json"), config.Client{
		Server: "192.168.56.1:59684", Fingerprint: strings.Repeat("a", 64), Token: token,
	}); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := pairStatus(&output, clientDir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "192.168.56.1:59684") || !strings.Contains(output.String(), strings.Repeat("a", 64)) || strings.Contains(output.String(), token) {
		t.Fatalf("配对状态内容错误或泄漏密钥: %q", output.String())
	}
}
