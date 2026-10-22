package server

import (
	"context"
	"encoding/xml"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"jumpcode/internal/config"
)

type fakeTaskScheduler struct {
	mu        sync.Mutex
	installed bool
	state     int
	queryErr  error
	starts    int
	deletes   int
	specs     []string
	onStart   func() error
}

func (*fakeTaskScheduler) Name() string { return "Jumpcode Server test-user" }
func (*fakeTaskScheduler) Close()       {}
func (s *fakeTaskScheduler) Query() (scheduledTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queryErr != nil {
		return scheduledTask{}, s.queryErr
	}
	if !s.installed {
		return scheduledTask{}, os.ErrNotExist
	}
	return scheduledTask{State: s.state}, nil
}
func (s *fakeTaskScheduler) Install(_, _, _ string, specs []string) error {
	s.installed, s.state, s.specs = true, 3, slices.Clone(specs)
	return nil
}
func (s *fakeTaskScheduler) Start() error {
	s.starts++
	s.setState(4)
	if s.onStart != nil {
		return s.onStart()
	}
	return nil
}
func (s *fakeTaskScheduler) Delete() error {
	s.deletes++
	s.installed = false
	return nil
}
func (s *fakeTaskScheduler) setState(state int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func TestTaskInstallPreservesAutoUntilRun(t *testing.T) {
	for _, specs := range [][]string{nil, {"auto:59700", "127.0.0.1:59701"}} {
		s := &fakeTaskScheduler{}
		m := taskManager{scheduler: s}
		if err := m.execute("install", specs); err != nil {
			t.Fatal(err)
		}
		want := specs
		if len(want) == 0 {
			want = []string{"auto"}
		}
		if !slices.Equal(s.specs, want) || s.starts != 0 {
			t.Fatalf("安装只应保存配置，不启动或枚举网卡: %+v", s)
		}
		if err := m.execute("install", []string{"auto"}); err == nil {
			t.Fatal("禁止覆盖已有任务")
		}
	}
	s := &fakeTaskScheduler{}
	if err := (taskManager{scheduler: s}).execute("install", []string{"0.0.0.0:1234"}); err == nil || s.installed {
		t.Fatal("安装前必须验证监听参数")
	}
}

func writeTestTaskControl(t *testing.T, dir string, s *fakeTaskScheduler) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer task-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		if r.Method == http.MethodPost {
			os.Remove(taskStatePath(dir))
			s.setState(3)
		}
	}))
	t.Cleanup(server.Close)
	if err := config.WriteJSON(taskStatePath(dir), daemonState{PID: 123, Address: "127.0.0.1:59684", Control: strings.TrimPrefix(server.URL, "http://"), Token: "task-secret"}); err != nil {
		t.Fatal(err)
	}
	return server
}

func TestTaskLifecycleAndStateIsolation(t *testing.T) {
	dir := t.TempDir()
	s := &fakeTaskScheduler{installed: true, state: 3}
	s.onStart = func() error {
		writeTestTaskControl(t, dir, s)
		return nil
	}
	manualState := []byte("manual daemon state must remain unchanged")
	if err := os.WriteFile(statePath(dir), manualState, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.json"), []byte("credentials"), 0600); err != nil {
		t.Fatal(err)
	}
	m := taskManager{dir: dir, scheduler: s, timeout: time.Second}
	for _, action := range []string{"start", "start", "status", "stop", "stop", "uninstall", "uninstall"} {
		if err := m.execute(action, nil); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	if s.starts != 1 || s.installed {
		t.Fatalf("重复启动不应创建新实例，卸载应删除任务: %+v", s)
	}
	data, err := os.ReadFile(statePath(dir))
	if err != nil || string(data) != string(manualState) {
		t.Fatalf("任务操作不能控制手动后台服务: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "server.json")); err != nil || string(data) != "credentials" {
		t.Fatal("卸载不应删除凭据")
	}
}

func TestTaskStartRequiresRegistrationAndReadiness(t *testing.T) {
	s := &fakeTaskScheduler{}
	m := taskManager{dir: t.TempDir(), scheduler: s, timeout: 30 * time.Millisecond}
	if err := m.execute("start", nil); err == nil || s.starts != 0 {
		t.Fatal("未注册不能启动")
	}
	s.installed, s.state = true, 3
	if err := m.execute("start", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("调度器接受启动但服务未就绪应超时: %v", err)
	}
	if s.starts != 1 {
		t.Fatal("应仅请求一次启动")
	}
}

func TestTaskUninstallKeepsUnresponsiveTask(t *testing.T) {
	s := &fakeTaskScheduler{installed: true, state: 4}
	m := taskManager{dir: t.TempDir(), scheduler: s, timeout: 30 * time.Millisecond}
	if err := m.execute("uninstall", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("服务未停止应返回超时: %v", err)
	}
	if !s.installed || s.deletes != 0 {
		t.Fatal("停止失败不能删除任务或强行杀死进程")
	}
}

func TestTaskQueryErrorsAreNotAbsence(t *testing.T) {
	want := errors.New("access denied")
	s := &fakeTaskScheduler{queryErr: want}
	m := taskManager{dir: t.TempDir(), scheduler: s, timeout: time.Second}
	for _, action := range []string{"install", "start", "stop", "status", "uninstall"} {
		if err := m.execute(action, nil); !errors.Is(err, want) {
			t.Fatalf("%s 应保留调度器错误: %v", action, err)
		}
	}
}

func TestTaskDefinitionUnicodeAndPolicy(t *testing.T) {
	definition := taskDefinitionXML("S-1-test", `C:\用户 & 工具\jc.exe`, `C:\用户 & 工具`, "zh-CN", []string{"auto", "127.0.0.1:59685"})
	var task struct {
		Triggers struct {
			UserID string `xml:"LogonTrigger>UserId"`
		}
		Principals struct {
			LogonType string `xml:"Principal>LogonType"`
			RunLevel  string `xml:"Principal>RunLevel"`
		}
		Settings struct {
			Instances     string `xml:"MultipleInstancesPolicy"`
			Limit         string `xml:"ExecutionTimeLimit"`
			OnBattery     bool   `xml:"DisallowStartIfOnBatteries"`
			StopOnBattery bool   `xml:"StopIfGoingOnBatteries"`
		}
		Actions struct {
			Command    string `xml:"Exec>Command"`
			Arguments  string `xml:"Exec>Arguments"`
			WorkingDir string `xml:"Exec>WorkingDirectory"`
		}
	}
	if err := xml.Unmarshal([]byte(definition), &task); err != nil {
		t.Fatal(err)
	}
	if task.Actions.Command != `C:\用户 & 工具\jc.exe` || task.Actions.WorkingDir != `C:\用户 & 工具` || task.Actions.Arguments != "--lang zh-CN server --listen auto --listen 127.0.0.1:59685 --task-run" {
		t.Fatalf("应直接运行同一个 jc server，保留 Unicode 路径: %+v", task.Actions)
	}
	if task.Triggers.UserID != "S-1-test" || task.Principals.LogonType != "InteractiveToken" || task.Principals.RunLevel != "LeastPrivilege" || task.Settings.Instances != "IgnoreNew" || task.Settings.Limit != "PT0S" || task.Settings.OnBattery || task.Settings.StopOnBattery {
		t.Fatalf("任务账户、权限或持续运行配置有误: %+v", task)
	}
}

func TestTaskManagementFailuresAreLogged(t *testing.T) {
	dir := t.TempDir()
	want := errors.New("scheduler unavailable")
	err := withTaskLog(dir, "start", func(_ *log.Logger) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("日志包装不能吞掉错误: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "start") || !strings.Contains(string(data), want.Error()) {
		t.Fatalf("管理操作失败应有日志: %s", data)
	}
}
