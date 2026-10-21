package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
)

func TestStartInstalledTaskRequiresInstalledTask(t *testing.T) {
	dir := t.TempDir()
	queryErr := errors.New("任务不存在")
	var calls [][]string
	err := startInstalledTask(dir, "Jumpcode Test", func(args ...string) error {
		calls = append(calls, args)
		return queryErr
	}, time.Second)
	if !errors.Is(err, queryErr) || !strings.Contains(err.Error(), "jc s startup install") {
		t.Fatalf("未安装时应提示安装并保留查询错误: %v", err)
	}
	if len(calls) != 1 || !slices.Equal(calls[0], []string{"/query", "/tn", "Jumpcode Test"}) {
		t.Fatalf("查询失败后不应运行任务: %v", calls)
	}
	if _, err := os.Stat(statePath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("未安装时不应创建后台状态: %v", err)
	}
}

func TestStartInstalledTaskHandlesRunningServer(t *testing.T) {
	for _, mode := range []string{"task", "daemon", ""} {
		t.Run("mode="+mode, func(t *testing.T) {
			dir := t.TempDir()
			control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer test-secret" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer control.Close()
			state := daemonState{PID: 42, Address: "192.168.56.1:59684", Control: strings.TrimPrefix(control.URL, "http://"), Token: "test-secret", Mode: mode}
			if err := config.WriteJSON(statePath(dir), state); err != nil {
				t.Fatal(err)
			}
			var calls [][]string
			err := startInstalledTask(dir, "Jumpcode Test", func(args ...string) error {
				calls = append(calls, args)
				return nil
			}, time.Second)
			if mode == "task" && err != nil {
				t.Fatalf("重复启动已运行的登录任务应成功: %v", err)
			}
			if mode != "task" && (err == nil || !strings.Contains(err.Error(), "jc s stop")) {
				t.Fatalf("普通后台服务应提示先停止: %v", err)
			}
			if len(calls) != 1 || calls[0][0] != "/query" {
				t.Fatalf("服务已运行时不应再次触发任务: %v", calls)
			}
			saved, err := readDaemonState(dir)
			if err != nil || saved.Token != state.Token || saved.Mode != mode {
				t.Fatalf("不应覆盖已有后台状态: state=%+v err=%v", saved, err)
			}
		})
	}
}

func TestStartInstalledTaskWaitsForReadiness(t *testing.T) {
	dir := t.TempDir()
	probes := make(chan struct{}, 10)
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		probes <- struct{}{}
		if len(probes) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer control.Close()
	state := daemonState{PID: 43, Address: "192.168.56.1:59700", Addresses: []string{"192.168.56.1:59700", "192.168.1.10:59700"}, Control: strings.TrimPrefix(control.URL, "http://"), Token: "test-secret", Mode: "task"}
	var calls [][]string
	err := startInstalledTask(dir, "Jumpcode Test", func(args ...string) error {
		calls = append(calls, args)
		if args[0] == "/run" {
			return config.WriteJSON(statePath(dir), state)
		}
		return nil
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !slices.Equal(calls[1], []string{"/run", "/tn", "Jumpcode Test"}) {
		t.Fatalf("应运行已有任务，不重新传入监听参数: %v", calls)
	}
	if len(probes) < 2 {
		t.Fatal("不能在服务尚未就绪时报告启动成功")
	}
	data, err := os.ReadFile(filepath.Join(dir, "server.log"))
	if err != nil || !strings.Contains(string(data), "已确认登录启动服务就绪") || strings.Contains(string(data), state.Token) {
		t.Fatalf("应记录启动和就绪日志且不泄漏凭据: log=%q err=%v", data, err)
	}
	saved, err := readDaemonState(dir)
	if err != nil || !slices.Equal(saved.Addresses, state.Addresses) {
		t.Fatalf("应保留任务实际使用的多 IP 配置: state=%+v err=%v", saved, err)
	}
}

func TestStartInstalledTaskReportsFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		runErr error
		mode   string
		stall  bool
	}{
		{name: "调度器拒绝启动", runErr: errors.New("任务已禁用")},
		{name: "已接受但没有服务就绪"},
		{name: "普通后台进程不能冒充任务服务", mode: "daemon"},
		{name: "控制接口无响应也应受启动超时限制", mode: "task", stall: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.stall {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer control.Close()
			started := time.Now()
			err := startInstalledTask(dir, "Jumpcode Test", func(args ...string) error {
				if args[0] == "/run" {
					if tc.mode != "" {
						return config.WriteJSON(statePath(dir), daemonState{PID: 44, Address: "192.168.56.1:59684", Control: strings.TrimPrefix(control.URL, "http://"), Token: "test-secret", Mode: tc.mode})
					}
					return tc.runErr
				}
				return nil
			}, 50*time.Millisecond)
			if err == nil {
				t.Fatal("调度器拒绝启动或服务未就绪时不能返回成功")
			}
			if tc.stall && time.Since(started) >= time.Second {
				t.Fatalf("控制请求不应超出启动等待期限: %s", time.Since(started))
			}
			if tc.runErr != nil && !errors.Is(err, tc.runErr) {
				t.Fatalf("应保留调度器错误: %v", err)
			}
			if tc.runErr == nil && !strings.Contains(err.Error(), filepath.Join(dir, "server.log")) {
				t.Fatalf("等待超时应提示查看日志: %v", err)
			}
			data, logErr := os.ReadFile(filepath.Join(dir, "server.log"))
			if logErr != nil || !strings.Contains(string(data), "失败") || strings.Contains(string(data), "已确认登录启动服务就绪") {
				t.Fatalf("失败应有日志且不能记录就绪: log=%q err=%v", data, logErr)
			}
		})
	}
}

func TestStartInstalledTaskPreservesInvalidState(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(statePath(dir), []byte("invalid JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls int
	err := startInstalledTask(dir, "Jumpcode Test", func(...string) error {
		calls++
		return nil
	}, time.Second)
	if err == nil || calls != 1 {
		t.Fatalf("状态损坏时应保留错误，不触发新的服务: calls=%d err=%v", calls, err)
	}
}

func TestStartInstalledTaskEnglishErrors(t *testing.T) {
	previousLanguage := i18n.Language
	i18n.Language = "en-US"
	t.Cleanup(func() { i18n.Language = previousLanguage })
	err := startInstalledTask(t.TempDir(), "Jumpcode Test", func(...string) error { return os.ErrNotExist }, time.Second)
	if err == nil || !strings.Contains(err.Error(), "Sign-in task is not installed") {
		t.Fatalf("英文环境应返回英文提示: %v", err)
	}
}
