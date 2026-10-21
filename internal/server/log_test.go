package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"jumpcode/internal/pairing"
	"jumpcode/internal/remote"
)

func TestBackgroundLogsLifecycleAndPreservesErrors(t *testing.T) {
	for _, mode := range []string{"daemon", "task"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if err := withServerLog(dir, []string{"auto"}, mode, func(*log.Logger) error { return nil }); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("模拟监听失败")
			if err := withServerLog(dir, []string{"auto"}, mode, func(*log.Logger) error { return failure }); !errors.Is(err, failure) {
				t.Fatalf("服务错误应返回调用方: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "server.log"))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			if strings.Count(content, "服务启动") != 2 || strings.Count(content, "服务已正常停止") != 1 || strings.Count(content, "服务退出") != 1 || !strings.Contains(content, failure.Error()) || !strings.Contains(content, "版本:") {
				t.Fatalf("追加日志未完整记录启动、正常停止和失败: %s", content)
			}
			for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
				if !regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} `).MatchString(line) || !strings.Contains(line, "模式: "+mode) {
					t.Fatalf("生命周期日志缺少时间或模式: %s", line)
				}
			}
		})
	}
}

func TestBackgroundRequestLogsHideCredentialsAndPaths(t *testing.T) {
	dir := t.TempDir()
	_, token, err := pairing.Create(dir, "devvm", "fixed-host")
	if err != nil {
		t.Fatal(err)
	}
	const project = "/home/me/私有项目 name"
	err = withServerLog(dir, []string{"auto"}, "daemon", func(logger *log.Logger) error {
		calls := 0
		opener := logOpenRequests(logger, func(target remote.Target) error {
			calls++
			if calls == 2 {
				return fmt.Errorf("模拟协议失败: %s %s %s", target.Path, url.PathEscape(target.Path), (&url.URL{Path: target.Path}).EscapedPath())
			}
			return nil
		})
		handler := newServerHandler(dir, opener)
		for _, status := range []int{http.StatusAccepted, http.StatusBadGateway} {
			req := httptest.NewRequest(http.MethodPost, "/v1/open", strings.NewReader(fmt.Sprintf(`{"path":%q}`, project)))
			req.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != status {
				t.Fatalf("打开请求返回码错误: %d, %s", response.Code, response.Body.String())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, secret := range []string{token, project, url.PathEscape(project), (&url.URL{Path: project}).EscapedPath()} {
		if strings.Contains(content, secret) {
			t.Fatalf("日志泄漏凭据或项目路径: %s", content)
		}
	}
	for _, event := range []string{"已接受打开请求", "打开请求失败", "fixed-host", "模拟协议失败"} {
		if !strings.Contains(content, event) {
			t.Fatalf("缺少请求事件 %q: %s", event, content)
		}
	}
}
