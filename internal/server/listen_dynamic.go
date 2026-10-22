package server

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"slices"
	"sort"
	"time"

	"jumpcode/internal/i18n"
)

type listenerExit struct {
	address  string
	listener net.Listener
	err      error
}

type autoListenerManager struct {
	server   *http.Server
	specs    []string
	changes  <-chan struct{}
	logger   *log.Logger
	resolve  func([]string) ([]string, error)
	debounce time.Duration
	retry    time.Duration
}

// run 串行处理通知、监听退出和重试，保留未变化的监听及已有连接。
func (m autoListenerManager) run(controlFile string) (runErr error) {
	if m.resolve == nil {
		m.resolve = resolveListenAddresses
	}
	if m.debounce == 0 {
		m.debounce = 500 * time.Millisecond
	}
	if m.retry == 0 {
		m.retry = 3 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.server.RegisterOnShutdown(cancel)
	active := make(map[string]net.Listener)
	exits := make(chan listenerExit)
	defer func() {
		cancel()
		for _, listener := range active {
			listener.Close()
		}
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := m.server.Shutdown(shutdownCtx); err != nil {
			m.server.Close()
			runErr = errors.Join(runErr, err)
		}
	}()
	start := func(address string, listener net.Listener) {
		active[address] = listener
		go func() {
			err := m.server.Serve(tls.NewListener(listener, m.server.TLSConfig))
			select {
			case exits <- listenerExit{address, listener, err}:
			case <-ctx.Done():
			}
		}()
	}
	// 混合配置中的明确地址仍要求启动时全部绑定成功。
	var fixed []string
	for _, spec := range m.specs {
		if _, auto, _ := autoListenPort(spec); !auto && !slices.Contains(fixed, spec) {
			fixed = append(fixed, spec)
		}
	}
	bound, err := bindListenAddresses(fixed)
	if err != nil {
		return err
	}
	for i, listener := range bound {
		start(fixed[i], listener)
	}
	addresses := func() []string {
		result := make([]string, 0, len(active))
		for address := range active {
			result = append(result, address)
		}
		sort.Strings(result)
		return result
	}
	failures := make(map[string]string)
	recordFailure := func(key string, err error) {
		if message := err.Error(); failures[key] != message {
			m.logger.Printf("%s，稍后重试", message)
			failures[key] = message
		}
	}
	reconcile := func() bool {
		desired, err := m.resolve(m.specs)
		if err != nil {
			recordFailure("enumerate", err)
			return true
		}
		delete(failures, "enumerate")
		for address, listener := range active {
			if !slices.Contains(desired, address) {
				delete(active, address)
				if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
					m.logger.Printf("关闭监听 %s 失败: %v", address, err)
				}
				m.logger.Printf("已移除监听：%s", address)
			}
		}
		for address := range failures {
			if !slices.Contains(desired, address) {
				delete(failures, address)
			}
		}
		for _, address := range desired {
			if active[address] != nil {
				continue
			}
			listener, err := net.Listen("tcp4", address)
			if err != nil {
				recordFailure(address, i18n.Errorf("监听 %s 失败: %w", "Failed to listen on %s: %w", address, err))
				continue
			}
			delete(failures, address)
			start(address, listener)
			m.logger.Printf("已新增监听：%s", address)
		}
		return len(failures) > 0
	}
	pendingRetry := reconcile()
	lastAddresses := addresses()
	update := func([]string) error { return nil }
	if controlFile != "" {
		var cleanup func()
		update, cleanup, err = serveControl(controlFile, lastAddresses, m.server)
		if err != nil {
			return err
		}
		defer cleanup()
	}
	m.logger.Printf("当前监听：%s", formatListenAddresses(lastAddresses))
	publish := func() error {
		current := addresses()
		if slices.Equal(current, lastAddresses) {
			return nil
		}
		if err := update(current); err != nil {
			return i18n.Errorf("更新监听状态失败: %w", "Failed to update listener state: %w", err)
		}
		lastAddresses = current
		m.logger.Printf("监听地址已更新：%s", formatListenAddresses(current))
		return nil
	}
	var changeTimer, retryTimer *time.Timer
	var changeC, retryC <-chan time.Time
	stopTimers := func() {
		if changeTimer != nil {
			changeTimer.Stop()
		}
		if retryTimer != nil {
			retryTimer.Stop()
		}
		changeTimer, retryTimer, changeC, retryC = nil, nil, nil, nil
	}
	defer stopTimers()
	scheduleRetry := func(pending bool) {
		if retryTimer != nil {
			retryTimer.Stop()
		}
		retryTimer, retryC = nil, nil
		if pending {
			retryTimer = time.NewTimer(m.retry)
			retryC = retryTimer.C
		}
	}
	scheduleRetry(pendingRetry)
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-m.changes:
			if !ok {
				return errors.New(i18n.T("网络变化通知意外关闭", "Network change notifications closed unexpectedly"))
			}
			if changeTimer == nil {
				changeTimer = time.NewTimer(m.debounce)
				changeC = changeTimer.C
			}
			continue
		case result := <-exits:
			if active[result.address] != result.listener {
				continue // 已移除或已替换的监听正常退出，不影响其他监听。
			}
			if errors.Is(result.err, http.ErrServerClosed) {
				return nil
			}
			delete(active, result.address)
			result.listener.Close()
			recordFailure(result.address, i18n.Errorf("监听 %s 已退出: %v", "Listener %s exited: %v", result.address, result.err))
			if err := publish(); err != nil {
				return err
			}
			scheduleRetry(true)
			continue
		case <-changeC:
		case <-retryC:
		}
		stopTimers()
		pendingRetry = reconcile()
		if err := publish(); err != nil {
			return err
		}
		scheduleRetry(pendingRetry)
	}
}
