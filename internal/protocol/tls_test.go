package protocol

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTLSHandshakeTimeoutBeforeRequestDeadline(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// 接受 TCP 连接并读取客户端握手，但不返回任何 TLS 数据。
		io.Copy(io.Discard, conn)
	}()
	client, err := NewHTTPClient(strings.Repeat("a", 64), DefaultRequestTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+listener.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	var timeoutError net.Error
	if !errors.As(err, &timeoutError) || !timeoutError.Timeout() || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TLS 握手应在总请求截止前超时: %v", err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("TLS 超时后未关闭连接")
	}
}
