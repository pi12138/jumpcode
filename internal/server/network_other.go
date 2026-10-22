//go:build !windows

package server

import "errors"

func watchNetworkChanges() (<-chan struct{}, func() error, error) {
	return nil, nil, errors.New("网络变化通知仅支持 Windows")
}
