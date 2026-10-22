//go:build windows

package server

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsTaskScheduler struct {
	service *taskDispatch
	folder  *taskDispatch
	sid     string
}

func connectTaskScheduler() (taskScheduler, error) {
	runtime.LockOSThread()
	// S_FALSE 表示该线程已经初始化，仍需配对调用 CoUninitialize。
	err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
	if err != nil && err != syscall.Errno(1) {
		runtime.UnlockOSThread()
		return nil, err
	}
	s := &windowsTaskScheduler{}
	ok := false
	defer func() {
		if !ok {
			s.Close()
		}
	}()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	s.sid = user.User.Sid.String()
	clsid, _ := windows.GUIDFromString("{0F87369F-A4E5-4CFC-BD3E-73E6154572DD}")
	iid, _ := windows.GUIDFromString("{00020400-0000-0000-C000-000000000046}")
	hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&s.service)))
	if err := comResult(hr, "CoCreateInstance(Schedule.Service)"); err != nil {
		return nil, err
	}
	v, err := s.service.call("Connect", 1, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	v.clear()
	s.folder, err = s.service.object("GetFolder", `\`)
	if err != nil {
		return nil, err
	}
	ok = true
	return s, nil
}

func (s *windowsTaskScheduler) Name() string { return "Jumpcode Server " + s.sid }

func (s *windowsTaskScheduler) Close() {
	s.folder.release()
	s.service.release()
	windows.CoUninitialize()
	runtime.UnlockOSThread()
}

func missingTask(err error) error {
	var native *taskCOMError
	if errors.As(err, &native) && native.Code == 0x80070002 {
		return os.ErrNotExist
	}
	return err
}

func (s *windowsTaskScheduler) Query() (scheduledTask, error) {
	task, err := s.folder.object("GetTask", s.Name())
	if err != nil {
		return scheduledTask{}, missingTask(err)
	}
	defer task.release()
	state, err := task.number("State")
	if err != nil {
		return scheduledTask{}, err
	}
	result, err := task.number("LastTaskResult")
	return scheduledTask{State: int(state), LastResult: result}, err
}

func (s *windowsTaskScheduler) Install(executable, workingDir, language string, specs []string) error {
	return s.register(taskDefinitionXML(s.sid, executable, workingDir, language, specs), 2) // TASK_CREATE，不覆盖已有任务。
}

func (s *windowsTaskScheduler) register(definition string, flags int) error {
	v, err := s.folder.call("RegisterTask", 1, s.Name(), definition, flags, s.sid, nil, 3, nil) // TASK_LOGON_INTERACTIVE_TOKEN
	if err == nil {
		v.clear()
	}
	return err
}

func (s *windowsTaskScheduler) Start() error {
	task, err := s.folder.object("GetTask", s.Name())
	if err != nil {
		return missingTask(err)
	}
	defer task.release()
	v, err := task.call("Run", 1, nil)
	if err == nil {
		v.clear()
	}
	return err
}

func (s *windowsTaskScheduler) Delete() error {
	v, err := s.folder.call("DeleteTask", 1, s.Name(), 0)
	if err == nil {
		v.clear()
	}
	return missingTask(err)
}
