//go:build windows

package server

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32             = windows.NewLazySystemDLL("ole32.dll")
	oleaut32          = windows.NewLazySystemDLL("oleaut32.dll")
	coCreateInstance  = ole32.NewProc("CoCreateInstance")
	sysAllocStringLen = oleaut32.NewProc("SysAllocStringLen")
	sysFreeString     = oleaut32.NewProc("SysFreeString")
	sysStringLen      = oleaut32.NewProc("SysStringLen")
	variantClear      = oleaut32.NewProc("VariantClear")
)

// VARIANT 的联合体包含两个指针；Windows 64 位下总长为 24 字节，32 位下为 16 字节。
type taskVariant struct {
	Kind     uint16
	Reserved [3]uint16
	Value    uint64
	Extra    [2*unsafe.Sizeof(uintptr(0)) - 8]byte
}

func (v *taskVariant) clear() {
	variantClear.Call(uintptr(unsafe.Pointer(v)))
}

func (v *taskVariant) takeDispatch() (*taskDispatch, error) {
	if v.Kind != 9 || v.Value == 0 { // VT_DISPATCH
		return nil, fmt.Errorf("Task Scheduler: expected IDispatch, VARIANT type %d", v.Kind)
	}
	object := *(**taskDispatch)(unsafe.Pointer(&v.Value))
	v.Kind, v.Value = 0, 0 // 转移接口引用的所有权，避免 VariantClear 提前 Release。
	return object, nil
}

type taskDispatch struct{ vtable *[7]uintptr }

func (d *taskDispatch) release() {
	if d != nil {
		syscall.SyscallN(d.vtable[2], uintptr(unsafe.Pointer(d)))
	}
}

type taskCOMError struct {
	Code    uint32
	Message string
}

func (e *taskCOMError) Error() string {
	return fmt.Sprintf("Task Scheduler: HRESULT 0x%08X: %s", e.Code, e.Message)
}

func comResult(hr uintptr, operation string) error {
	if int32(hr) < 0 {
		return &taskCOMError{Code: uint32(hr), Message: operation}
	}
	return nil
}

func bstrText(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	n, _, _ := sysStringLen.Call(uintptr(unsafe.Pointer(ptr)))
	return windows.UTF16ToString(unsafe.Slice(ptr, int(n)))
}

// call 只封装计划任务实际使用的方法和只读属性，不引入通用 COM 依赖。
func (d *taskDispatch) call(name string, flags uint16, args ...any) (result taskVariant, err error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return result, err
	}
	var iid windows.GUID
	var id int32
	hr, _, _ := syscall.SyscallN(d.vtable[5], uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&namePtr)), 1, 0, uintptr(unsafe.Pointer(&id)))
	if err := comResult(hr, name); err != nil {
		return result, err
	}
	variants := make([]taskVariant, len(args))
	defer func() {
		for i := range variants {
			variants[i].clear()
		}
	}()
	for i, arg := range args {
		v := &variants[len(args)-1-i] // IDispatch 的参数顺序与调用顺序相反。
		switch value := arg.(type) {
		case nil:
		case string:
			chars, err := windows.UTF16FromString(value)
			if err != nil {
				return result, err
			}
			ptr, _, _ := sysAllocStringLen.Call(uintptr(unsafe.Pointer(&chars[0])), uintptr(len(chars)-1))
			runtime.KeepAlive(chars)
			if ptr == 0 {
				return result, fmt.Errorf("Task Scheduler: SysAllocStringLen failed")
			}
			v.Kind, v.Value = 8, uint64(ptr) // VT_BSTR
		case int:
			v.Kind, v.Value = 3, uint64(uint32(value)) // VT_I4
		default:
			return result, fmt.Errorf("Task Scheduler: unsupported argument %T", arg)
		}
	}
	params := struct {
		Args       *taskVariant
		Named      *int32
		Count      uint32
		NamedCount uint32
	}{Count: uint32(len(variants))}
	if len(variants) > 0 {
		params.Args = &variants[0]
	}
	var exception struct {
		Code        uint16
		Reserved    uint16
		Source      *uint16
		Description *uint16
		HelpFile    *uint16
		HelpContext uint32
		ReservedPtr uintptr
		Deferred    uintptr
		Status      int32
	}
	var argErr uint32
	hr, _, _ = syscall.SyscallN(d.vtable[6], uintptr(unsafe.Pointer(d)), uintptr(id), uintptr(unsafe.Pointer(&iid)), 0, uintptr(flags), uintptr(unsafe.Pointer(&params)), uintptr(unsafe.Pointer(&result)), uintptr(unsafe.Pointer(&exception)), uintptr(unsafe.Pointer(&argErr)))
	runtime.KeepAlive(variants)
	runtime.KeepAlive(d)
	if uint32(hr) == 0x80020009 && exception.Deferred != 0 { // DISP_E_EXCEPTION
		syscall.SyscallN(exception.Deferred, uintptr(unsafe.Pointer(&exception)))
	}
	description := bstrText(exception.Description)
	for _, ptr := range []*uint16{exception.Source, exception.Description, exception.HelpFile} {
		if ptr != nil {
			sysFreeString.Call(uintptr(unsafe.Pointer(ptr)))
		}
	}
	if err := comResult(hr, name); err != nil {
		result.clear()
		if exception.Status < 0 {
			hr = uintptr(uint32(exception.Status))
		}
		return taskVariant{}, &taskCOMError{Code: uint32(hr), Message: name + ": " + description}
	}
	return result, nil
}

func (d *taskDispatch) object(name string, args ...any) (*taskDispatch, error) {
	v, err := d.call(name, 1, args...)
	if err != nil {
		return nil, err
	}
	defer v.clear()
	return v.takeDispatch()
}

func (d *taskDispatch) number(name string) (uint32, error) {
	v, err := d.call(name, 2)
	if err != nil {
		return 0, err
	}
	defer v.clear()
	if v.Kind != 3 { // VT_I4
		return 0, fmt.Errorf("Task Scheduler: %s returned VARIANT type %d", name, v.Kind)
	}
	return uint32(v.Value), nil
}
