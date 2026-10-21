package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"jumpcode/internal/config"
)

func TestRequestTimeoutRejectsNonPositiveValues(t *testing.T) {
	for _, command := range []string{"client", "join"} {
		if command == "join" && runtime.GOOS != "linux" {
			continue
		}
		for _, timeout := range []string{"0", "-1s"} {
			t.Run(command+"/"+timeout, func(t *testing.T) {
				dir := pairCommandConfigDir(t)
				args := []string{"client", "."}
				if command == "join" {
					args = []string{"pair", "join", "--server", "192.0.2.1:59684", "--fingerprint", strings.Repeat("a", 64)}
				}
				output, err := executePairCommand(t, append(args, "--timeout", timeout)...)
				if err == nil || !strings.Contains(err.Error(), "请求超时时间必须大于 0") || strings.Contains(output, "配对密钥:") {
					t.Fatalf("应在读取配置或密钥前拒绝无效超时: output=%q err=%v", output, err)
				}
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatalf("无效超时不应创建配置目录: %v", err)
				}
			})
		}
	}
}

func TestRequestTimeoutFlagsCancelSlowServer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("client 和 pair join 在 Linux 客户端运行")
	}
	for _, command := range []string{"client", "join"} {
		t.Run(command, func(t *testing.T) {
			dir := pairCommandConfigDir(t)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-time.After(2 * time.Second):
					if r.URL.Path == "/v1/ping" {
						w.WriteHeader(http.StatusNoContent)
					} else {
						w.WriteHeader(http.StatusAccepted)
					}
				}
			}))
			defer server.Close()
			sum := sha256.Sum256(server.Certificate().Raw)
			fingerprint := hex.EncodeToString(sum[:])
			address := strings.TrimPrefix(server.URL, "https://")
			if err := config.WriteJSON(filepath.Join(dir, "client.json"), config.Client{
				Server: address, Fingerprint: fingerprint, Token: "existing-token",
			}); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(dir, "client.json"))
			if err != nil {
				t.Fatal(err)
			}
			root := newRootCommand()
			root.SetIn(strings.NewReader("new-pairing-token\n"))
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)
			args := []string{"c", t.TempDir(), "--timeout", "300ms"}
			if command == "join" {
				args = []string{"p", "join", "--server", address, "--fingerprint", fingerprint, "--timeout", "300ms"}
			}
			root.SetArgs(args)
			err = root.Execute()
			var timeoutError net.Error
			if !errors.As(err, &timeoutError) || !timeoutError.Timeout() {
				t.Fatalf("自定义超时应中断等待服务端响应: output=%q err=%v", output.String(), err)
			}
			after, err := os.ReadFile(filepath.Join(dir, "client.json"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("请求超时不应修改已有配对: err=%v", err)
			}
		})
	}
}
