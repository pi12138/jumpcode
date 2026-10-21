package protocol

import (
	"errors"
	"net"
	"strconv"
)

func ValidateAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("监听地址应为 局域网IPv4:端口")
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil || ip.IsUnspecified() {
		return errors.New("请指定一个明确的 IPv4 监听地址，不要使用 0.0.0.0")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("端口必须在 1 到 65535 之间")
	}
	return nil
}
