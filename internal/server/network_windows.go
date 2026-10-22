//go:build windows

package server

import (
	"errors"
	"sync"

	"jumpcode/internal/i18n"

	"golang.org/x/sys/windows"
)

var networkChangeSubscribers sync.Map

// NewCallback 分配的回调不能释放，因此所有订阅共用同一个回调入口。
var networkChangeCallback = windows.NewCallback(func(_, _, _ uintptr) uintptr {
	networkChangeSubscribers.Range(func(key, _ any) bool {
		select {
		case key.(chan struct{}) <- struct{}{}:
		default:
		}
		return true
	})
	return 0
})

func watchNetworkChanges() (<-chan struct{}, func() error, error) {
	changes := make(chan struct{}, 1)
	networkChangeSubscribers.Store(changes, struct{}{})
	var addressHandle, interfaceHandle windows.Handle
	if err := windows.NotifyUnicastIpAddressChange(windows.AF_INET, networkChangeCallback, nil, false, &addressHandle); err != nil {
		networkChangeSubscribers.Delete(changes)
		return nil, nil, i18n.Errorf("订阅 IPv4 地址变化失败: %w", "Failed to subscribe to IPv4 address changes: %w", err)
	}
	if err := windows.NotifyIpInterfaceChange(windows.AF_INET, networkChangeCallback, nil, false, &interfaceHandle); err != nil {
		networkChangeSubscribers.Delete(changes)
		return nil, nil, errors.Join(i18n.Errorf("订阅 IPv4 接口变化失败: %w", "Failed to subscribe to IPv4 interface changes: %w", err), cancelNetworkChanges(addressHandle))
	}
	var once sync.Once
	var closeErr error
	return changes, func() error {
		once.Do(func() {
			networkChangeSubscribers.Delete(changes)
			// 在管理线程取消，不能在 Windows 通知回调内调用取消接口。
			closeErr = errors.Join(cancelNetworkChanges(interfaceHandle), cancelNetworkChanges(addressHandle))
		})
		// 通知通道不关闭，避免正在返回的回调向已关闭通道发送。
		return closeErr
	}, nil
}

func cancelNetworkChanges(handle windows.Handle) error {
	if err := windows.CancelMibChangeNotify2(handle); err != nil {
		return i18n.Errorf("取消网络变化订阅失败: %w", "Failed to cancel network change notifications: %w", err)
	}
	return nil
}
