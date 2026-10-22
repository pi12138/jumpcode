//go:build windows

package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// 只验证定义，不注册或启动任务，避免在开发机和 CI 留下持久任务。
func TestNativeTaskSchedulerValidatesDefinition(t *testing.T) {
	wantSize := uintptr(16)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		wantSize = 24
	}
	if unsafe.Sizeof(taskVariant{}) != wantSize {
		t.Fatalf("VARIANT 布局不匹配: %d", unsafe.Sizeof(taskVariant{}))
	}
	scheduler, err := connectTaskScheduler()
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	s := scheduler.(*windowsTaskScheduler)
	definition := taskDefinitionXML(s.sid, `C:\用户 & 工具\jc.exe`, `C:\用户 & 工具`, "zh-CN", []string{"auto"})
	if err := s.register(definition, 1); err != nil { // TASK_VALIDATE_ONLY
		t.Fatalf("Windows 应接受 Unicode XML 和直接执行 jc server 的任务定义: %v", err)
	}
	if err := s.register("<invalid>", 1); err == nil {
		t.Fatal("必须保留原生调度器的 XML 错误")
	}
	if _, err := s.Query(); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("查询任务必须能识别未安装或返回实际调度器状态: %v", err)
	}
}

func TestRunTaskUsesCurrentProcessAndSeparateState(t *testing.T) {
	helper := startTaskProcess(t, "service")
	dir := helper.dir
	manual := filepath.Join(dir, "daemon.json")
	probe := helper.probe(t)
	if !probe.HadConsole || !probe.Detached || probe.PID != helper.cmd.Process.Pid {
		t.Fatalf("任务进程必须从新建的控制台脱离并保留 PID: %+v", probe)
	}
	var state daemonState
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		state, err = readControlState(taskStatePath(dir))
		if err == nil && daemonRequest(state, http.MethodGet) == nil {
			ready = true
			break
		}
		select {
		case <-helper.done:
			t.Fatalf("服务在就绪前退出: %v", helper.err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("任务模式服务未就绪")
	}
	defer daemonRequest(state, http.MethodPost)
	if state.PID != helper.cmd.Process.Pid || state.Address != helper.address {
		t.Fatalf("任务必须在当前 PID 运行: %+v", state)
	}
	if err := daemonRequest(state, http.MethodPost); err != nil {
		t.Fatal(err)
	}
	select {
	case <-helper.done:
		if helper.err != nil {
			t.Fatal(helper.err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("任务模式服务未停止")
	}
	if _, err := os.Stat(taskStatePath(dir)); !os.IsNotExist(err) {
		t.Fatalf("正常退出应清理任务状态: %v", err)
	}
	if data, err := os.ReadFile(manual); err != nil || string(data) != "manual-state" {
		t.Fatal("任务模式不能更改手动后台状态")
	}
	data, err := os.ReadFile(filepath.Join(dir, "server.log"))
	if err != nil || !strings.Contains(string(data), "Windows 计划任务") || !strings.Contains(string(data), "正常停止") || !strings.Contains(string(data), "已脱离控制台") {
		t.Fatalf("任务运行日志缺失: %s, %v", data, err)
	}
	for _, marker := range []string{"task-stdout-test", "task-stderr-test", "task-native-stderr-test", "task-default-log-test"} {
		if !strings.Contains(string(data), marker) {
			t.Fatalf("脱离控制台后日志缺失 %s: %s", marker, data)
		}
	}
}
