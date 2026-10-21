package server

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"jumpcode/internal/protocol"
)

const defaultListenPort = "59684"

func normalizeListenSpecs(specs []string) ([]string, error) {
	if len(specs) == 0 {
		return []string{"auto"}, nil
	}
	for _, spec := range specs {
		if _, ok, err := autoListenPort(spec); ok {
			if err != nil {
				return nil, err
			}
			continue
		}
		if err := protocol.ValidateAddress(spec); err != nil {
			return nil, fmt.Errorf("监听地址 %q 无效: %w", spec, err)
		}
	}
	return append([]string(nil), specs...), nil
}

func autoListenPort(spec string) (string, bool, error) {
	if spec == "auto" {
		return defaultListenPort, true, nil
	}
	if !strings.HasPrefix(spec, "auto:") {
		return "", false, nil
	}
	port := strings.TrimPrefix(spec, "auto:")
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", true, fmt.Errorf("auto 端口必须在 1 到 65535 之间: %q", port)
	}
	return port, true, nil
}

func resolveListenAddresses(specs []string) ([]string, error) {
	specs, err := normalizeListenSpecs(specs)
	if err != nil {
		return nil, err
	}
	var candidates []string
	seen := make(map[string]bool)
	var addresses []string
	for _, spec := range specs {
		if port, ok, _ := autoListenPort(spec); ok {
			if candidates == nil {
				interfaces, err := net.Interfaces()
				if err != nil {
					return nil, fmt.Errorf("枚举网卡失败: %w", err)
				}
				candidates, err = autoIPv4Candidates(interfaces, func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() })
				if err != nil {
					return nil, err
				}
				if len(candidates) == 0 {
					return nil, fmt.Errorf("没有可用的私有 IPv4 地址，请使用 --listen 指定地址")
				}
			}
			for _, ip := range candidates {
				address := net.JoinHostPort(ip, port)
				if !seen[address] {
					addresses = append(addresses, address)
					seen[address] = true
				}
			}
			continue
		}
		if !seen[spec] {
			addresses = append(addresses, spec)
			seen[spec] = true
		}
	}
	return addresses, nil
}

func autoIPv4Candidates(interfaces []net.Interface, addrs func(net.Interface) ([]net.Addr, error)) ([]string, error) {
	seen := make(map[string]bool)
	var candidates []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := addrs(iface)
		if err != nil {
			return nil, fmt.Errorf("读取网卡 %q 的地址失败: %w", iface.Name, err)
		}
		for _, address := range addresses {
			var ip net.IP
			switch value := address.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
				continue
			}
			value := ip.To4().String()
			if !seen[value] {
				candidates = append(candidates, value)
				seen[value] = true
			}
		}
	}
	sort.Strings(candidates)
	return candidates, nil
}

func formatListenAddresses(addresses []string) string {
	urls := make([]string, len(addresses))
	for i, address := range addresses {
		urls[i] = "https://" + address
	}
	return strings.Join(urls, ", ")
}
