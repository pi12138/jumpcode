package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/protocol"
)

func validateServerAddress(address string) error {
	if err := protocol.ValidateAddress(address); err != nil {
		return fmt.Errorf("服务端地址无效: %w", err)
	}
	host, _, _ := net.SplitHostPort(address)
	if net.ParseIP(host).IsUnspecified() {
		return errors.New("服务端地址不能使用 0.0.0.0")
	}
	return nil
}

// Join 在线验证地址、证书指纹和密钥，验证成功后保存客户端配置。
func Join(dir, address, fingerprint, token string) error {
	if err := validateServerAddress(address); err != nil {
		return err
	}
	fingerprint, err := protocol.NormalizeFingerprint(fingerprint)
	if err != nil {
		return err
	}
	if token == "" {
		return errors.New("配对密钥不能为空")
	}
	cfg := config.Client{Server: address, Fingerprint: fingerprint, Token: token}
	client, err := protocol.NewHTTPClient(fingerprint)
	if err != nil {
		return err
	}
	resp, err := authorizedRequest(client, cfg, http.MethodGet, "/v1/ping", nil)
	if err != nil {
		return fmt.Errorf("配对验证失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	return config.WriteJSON(filepath.Join(dir, "client.json"), cfg)
}

// Open 将 Linux 目录解析为绝对路径，并请求已配对的 Windows 服务端打开。
func Open(dir, folder string) error {
	if runtime.GOOS != "linux" {
		return errors.New("首版 jc client 仅支持 Linux 虚拟机")
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		return fmt.Errorf("解析目录失败: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("读取目录失败: %w", err)
	}
	if !info.IsDir() {
		return errors.New("目标必须是目录")
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return fmt.Errorf("解析目录符号链接失败: %w", err)
	}
	cfg, err := config.ReadClient(dir)
	if err != nil {
		return err
	}
	client, err := protocol.NewHTTPClient(cfg.Fingerprint)
	if err != nil {
		return err
	}
	data, err := json.Marshal(protocol.OpenRequest{Path: resolved})
	if err != nil {
		return err
	}
	resp, err := authorizedRequest(client, cfg, http.MethodPost, "/v1/open", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("连接 Windows 服务端失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return responseError(resp)
	}
	return nil
}

func authorizedRequest(client *http.Client, cfg config.Client, method, endpoint string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, "https://"+cfg.Server+endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	message := strings.TrimSpace(string(data))
	if message == "" {
		message = resp.Status
	}
	if i18n.Language == "en-US" {
		message = i18n.LocalizeError(errors.New(message))
	}
	return fmt.Errorf(i18n.T("服务端返回 %d: %s", "Server returned %d: %s"), resp.StatusCode, message)
}
