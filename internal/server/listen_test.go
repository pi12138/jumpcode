package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"slices"
	"testing"
	"time"

	"jumpcode/internal/config"
	"jumpcode/internal/protocol"
)

func TestListenSpecs(t *testing.T) {
	for _, tc := range []struct {
		input []string
		want  []string
	}{
		{nil, []string{"auto"}},
		{[]string{"auto", "192.168.56.1:59684"}, []string{"auto", "192.168.56.1:59684"}},
		{[]string{"auto:59700"}, []string{"auto:59700"}},
	} {
		got, err := normalizeListenSpecs(tc.input)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("监听参数 %v: got=%v err=%v", tc.input, got, err)
		}
	}
	for _, input := range [][]string{{"auto:"}, {"auto:0"}, {"auto:65536"}, {"0.0.0.0:59684"}, {"192.168.1.1:0"}} {
		if _, err := normalizeListenSpecs(input); err == nil {
			t.Errorf("应拒绝监听参数 %v", input)
		}
	}
	got, err := resolveListenAddresses([]string{"192.168.56.1:59684", "192.168.56.1:59684", "10.0.0.1:59684"})
	if err != nil || !slices.Equal(got, []string{"192.168.56.1:59684", "10.0.0.1:59684"}) {
		t.Fatalf("多地址去重错误: got=%v err=%v", got, err)
	}
}

func TestAutoIPv4Candidates(t *testing.T) {
	interfaces := []net.Interface{
		{Index: 1, Name: "down", Flags: 0},
		{Index: 2, Name: "loopback", Flags: net.FlagUp | net.FlagLoopback},
		{Index: 3, Name: "wifi", Flags: net.FlagUp},
		{Index: 4, Name: "vm", Flags: net.FlagUp},
	}
	addresses := map[int][]net.Addr{
		3: {&net.IPNet{IP: net.ParseIP("192.168.1.10")}, &net.IPNet{IP: net.ParseIP("169.254.1.2")}, &net.IPNet{IP: net.ParseIP("2001:db8::1")}},
		4: {&net.IPNet{IP: net.ParseIP("10.0.0.2")}, &net.IPNet{IP: net.ParseIP("192.168.1.10")}, &net.IPNet{IP: net.ParseIP("8.8.8.8")}},
	}
	got, err := autoIPv4Candidates(interfaces, func(iface net.Interface) ([]net.Addr, error) { return addresses[iface.Index], nil })
	if err != nil || !slices.Equal(got, []string{"10.0.0.2", "192.168.1.10"}) {
		t.Fatalf("自动候选地址错误: got=%v err=%v", got, err)
	}
}

func TestMultipleHTTPSListeners(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, fingerprint, err := config.EnsureCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	listeners, err := bindListenAddresses([]string{"127.0.0.1:0", "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, listener := range listeners {
			listener.Close()
		}
	}()
	server := &http.Server{
		Handler:   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13},
	}
	done := make(chan error, 1)
	go func() { done <- serveHTTPSOnListeners(server, listeners) }()
	client, err := protocol.NewHTTPClient(fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	for _, listener := range listeners {
		resp, err := client.Get("https://" + listener.Addr().String())
		if err != nil {
			t.Fatalf("监听地址 %s 无法响应: %v", listener.Addr(), err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("监听地址 %s 返回 %d", listener.Addr(), resp.StatusCode)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("多个监听端口未在停止服务后释放")
	}
}
