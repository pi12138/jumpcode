package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/protocol"
)

type testNetwork struct {
	mu        sync.Mutex
	addresses []string
	err       error
	calls     int
}

func (n *testNetwork) resolve([]string) ([]string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
	return slices.Clone(n.addresses), n.err
}

func (n *testNetwork) set(addresses []string, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.addresses, n.err = slices.Clone(addresses), err
}

func (n *testNetwork) callCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls
}

type networkTestLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *networkTestLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *networkTestLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type autoTestServer struct {
	dir     string
	server  *http.Server
	client  *http.Client
	network *testNetwork
	changes chan struct{}
	log     *networkTestLog
	done    chan struct{}
	err     error
}

func startAutoTestServer(t *testing.T, initial []string, specs ...string) *autoTestServer {
	t.Helper()
	dir := t.TempDir()
	certFile, keyFile, fingerprint, err := config.EnsureCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	client, err := protocol.NewHTTPClient(fingerprint, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	h := &autoTestServer{
		dir: dir, client: client, network: &testNetwork{addresses: slices.Clone(initial)},
		changes: make(chan struct{}, 32), log: &networkTestLog{}, done: make(chan struct{}),
		server: &http.Server{
			Handler:   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
			TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}},
		},
	}
	if len(specs) == 0 {
		specs = []string{"auto"}
	}
	m := autoListenerManager{
		server: h.server, specs: specs, changes: h.changes,
		resolve: h.network.resolve, logger: log.New(h.log, "", 0),
		debounce: 20 * time.Millisecond, retry: 40 * time.Millisecond,
	}
	go func() {
		h.err = m.run(taskStatePath(dir))
		close(h.done)
	}()
	t.Cleanup(func() {
		client.CloseIdleConnections()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		h.server.Shutdown(ctx)
		select {
		case <-h.done:
			if h.err != nil {
				t.Errorf("自动监听异常退出: %v\n%s", h.err, h.log.String())
			}
		case <-ctx.Done():
			h.server.Close()
			t.Error("自动监听未能停止")
		}
	})
	return h
}

func freeTestAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	return address
}

func (h *autoTestServer) waitAddresses(t *testing.T, want ...string) daemonState {
	t.Helper()
	want = slices.Clone(want)
	slices.Sort(want)
	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		state, err := readControlState(taskStatePath(h.dir))
		if err == nil && slices.Equal(stateListenAddresses(state), want) {
			return state
		}
		select {
		case <-h.done:
			t.Fatalf("服务提前退出: %v\n%s", h.err, h.log.String())
		case <-deadline:
			t.Fatalf("等待监听 %v 超时，当前状态 %+v: %v\n%s", want, state, err, h.log.String())
		case <-tick.C:
		}
	}
}

func (h *autoTestServer) checkHTTPS(t *testing.T, address string) {
	t.Helper()
	response, err := h.client.Get("https://" + address)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("监听 %s 返回 %d", address, response.StatusCode)
	}
}

func TestAutoListenersFollowEventsAndSurviveNetworkLoss(t *testing.T) {
	a, b, c := freeTestAddress(t), freeTestAddress(t), freeTestAddress(t)
	h := startAutoTestServer(t, []string{a, b})
	initial := h.waitAddresses(t, a, b)
	h.checkHTTPS(t, b)
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp4", a, h.client.Transport.(*http.Transport).TLSClientConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	reader := bufio.NewReader(connection)
	request := func() {
		t.Helper()
		connection.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := fmt.Fprintf(connection, "GET / HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
			t.Fatal(err)
		}
		response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
		if err != nil {
			t.Fatalf("未变化地址的既有连接应继续可用: %v", err)
		}
		response.Body.Close()
	}
	request()
	h.network.set([]string{a, c}, nil)
	for range 20 {
		h.changes <- struct{}{}
	}
	current := h.waitAddresses(t, a, c)
	request()
	h.checkHTTPS(t, c)
	if current.PID != initial.PID || current.Control != initial.Control || current.Token != initial.Token {
		t.Fatal("网络变化不应更换进程或控制端点")
	}
	if conn, err := net.DialTimeout("tcp4", b, 100*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("已消失的地址不能继续接受新连接")
	}
	connection.Close()
	// 全部地址消失时保留控制端点，收到后续事件后恢复服务。
	h.network.set(nil, nil)
	h.changes <- struct{}{}
	offline := h.waitAddresses(t)
	if offline.Address != "" || offline.Addresses == nil || daemonRequest(offline, http.MethodGet) != nil {
		t.Fatal("无监听时应明确记录空地址列表，并保留控制能力")
	}
	h.network.set([]string{c}, nil)
	h.changes <- struct{}{}
	h.waitAddresses(t, c)
	h.checkHTTPS(t, c)
	// 空监听状态也必须能正常停止，不能等待下一次网络通知。
	h.network.set(nil, nil)
	h.changes <- struct{}{}
	offline = h.waitAddresses(t)
	if err := daemonRequest(offline, http.MethodPost); err != nil {
		t.Fatal(err)
	}
	select {
	case <-h.done:
	case <-time.After(3 * time.Second):
		t.Fatal("断网期间无法正常停止")
	}
	if _, err := os.Stat(taskStatePath(h.dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("停止后必须清理运行状态")
	}
	for _, text := range []string{"已新增监听", "已移除监听", "监听地址已更新", formatListenAddresses(nil)} {
		if !strings.Contains(h.log.String(), text) {
			t.Fatalf("缺少网络变化日志 %q", text)
		}
	}
}

func TestAutoListenersRetryFailuresWithoutNewEvents(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	a, b := freeTestAddress(t), occupied.Addr().String()
	h := startAutoTestServer(t, []string{a, b})
	h.waitAddresses(t, a)
	h.checkHTTPS(t, a)
	// 同一绑定错误重复重试时只记录一次，恢复后无需另一个网络事件。
	time.Sleep(120 * time.Millisecond)
	occupied.Close()
	h.waitAddresses(t, a, b)
	h.checkHTTPS(t, b)
	if count := strings.Count(h.log.String(), i18n.T("监听 "+b+" 失败", "Failed to listen on "+b)); count != 1 {
		t.Fatalf("重复绑定失败日志应合并，得到 %d 条\n%s", count, h.log.String())
	}
	h.network.set(nil, errors.New("模拟网卡枚举失败"))
	h.changes <- struct{}{}
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(h.log.String(), "模拟网卡枚举失败") {
		if time.Now().After(deadline) {
			t.Fatal("未记录网卡枚举失败")
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.waitAddresses(t, a, b)
	h.checkHTTPS(t, a)
	h.network.set([]string{a}, nil)
	h.waitAddresses(t, a)
	// 健康且没有网络事件时，不按重试周期持续枚举网卡。
	calls := h.network.callCount()
	time.Sleep(120 * time.Millisecond)
	if h.network.callCount() != calls {
		t.Fatal("健康状态不应进行周期轮询")
	}
}

func TestAutoListenersKeepExplicitAddressWhenCandidatesDisappear(t *testing.T) {
	fixed, automatic := freeTestAddress(t), freeTestAddress(t)
	h := startAutoTestServer(t, []string{fixed, automatic}, "auto:59700", fixed)
	h.waitAddresses(t, fixed, automatic)
	h.network.set([]string{fixed}, nil)
	h.changes <- struct{}{}
	h.waitAddresses(t, fixed)
	h.checkHTTPS(t, fixed)
}

func TestAutoListenersRejectUnavailableExplicitAddress(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	m := autoListenerManager{
		server: &http.Server{}, specs: []string{"auto", occupied.Addr().String()},
		logger: log.New(&networkTestLog{}, "", 0),
	}
	if err := m.run(""); err == nil {
		t.Fatal("明确指定的地址绑定失败时，应拒绝启动")
	}
}

func TestAutoListenersStartOfflineAndRequireListenerForReadiness(t *testing.T) {
	h := startAutoTestServer(t, nil)
	h.waitAddresses(t)
	scheduler := &fakeTaskScheduler{installed: true, state: 4}
	m := taskManager{dir: h.dir, scheduler: scheduler, timeout: 30 * time.Millisecond}
	if err := m.execute("start", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("只有控制端点不能报告服务已就绪: %v", err)
	}
	address := freeTestAddress(t)
	h.network.set([]string{address}, nil)
	h.changes <- struct{}{}
	h.waitAddresses(t, address)
	if err := m.execute("start", nil); err != nil {
		t.Fatal(err)
	}
	if scheduler.starts != 0 {
		t.Fatal("等待网络时不能重复创建任务进程")
	}
	h.checkHTTPS(t, address)
}
