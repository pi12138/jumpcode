package server

import (
	"net"
	"net/http"
	"slices"
	"testing"
	"time"

	"jumpcode/internal/config"
)

func TestDaemonStateRejectsNonLocalControl(t *testing.T) {
	dir := t.TempDir()
	if err := config.WriteJSON(statePath(dir), daemonState{PID: 42, Address: "192.168.1.1:1234", Control: "example.com:1234", Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := readDaemonState(dir); err == nil {
		t.Fatal("后台控制地址必须限定在 127.0.0.1")
	}
}

func TestDaemonControlRequiresTokenAndStopsServer(t *testing.T) {
	dir := t.TempDir()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mainServer := &http.Server{}
	done := make(chan error, 1)
	go func() { done <- mainServer.Serve(listener) }()
	cleanup, err := serveDaemonControl(dir, []string{listener.Addr().String()}, mainServer)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	state, err := readDaemonState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stateListenAddresses(state), []string{listener.Addr().String()}) {
		t.Fatalf("后台状态未记录监听地址: %v", stateListenAddresses(state))
	}
	if err := daemonRequest(state, http.MethodGet); err != nil {
		t.Fatalf("正确令牌应能查询状态: %v", err)
	}
	state.Token = "wrong"
	if err := daemonRequest(state, http.MethodPost); err == nil {
		t.Fatal("错误令牌不能停止服务")
	}
	state, _ = readDaemonState(dir)
	if err := daemonRequest(state, http.MethodPost); err != nil {
		t.Fatalf("正确令牌应能停止服务: %v", err)
	}
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			t.Fatalf("停止后返回异常: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("停止请求未关闭主服务")
	}
}
