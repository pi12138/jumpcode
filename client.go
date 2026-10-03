package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
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
	"time"

	"golang.org/x/term"
)

func newPinnedHTTPClient(fingerprint string) (*http.Client, error) {
	fingerprint, err := normalizeFingerprint(fingerprint)
	if err != nil {
		return nil, err
	}
	want, _ := hex.DecodeString(fingerprint)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			// 自签名证书由用户提供的 SHA-256 指纹直接固定。
			InsecureSkipVerify: true,
			VerifyConnection: func(state tls.ConnectionState) error {
				if len(state.PeerCertificates) != 1 {
					return errors.New("服务端证书数量无效")
				}
				cert := state.PeerCertificates[0]
				now := time.Now()
				if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
					return errors.New("服务端证书已过期或尚未生效")
				}
				sum := sha256.Sum256(cert.Raw)
				if subtle.ConstantTimeCompare(want, sum[:]) != 1 {
					return errors.New("服务端证书指纹不匹配")
				}
				return nil
			},
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("不接受服务端重定向")
		},
	}, nil
}

func validateServerAddress(address string) error {
	if err := validateListenAddress(address); err != nil {
		return fmt.Errorf("服务端地址无效: %w", err)
	}
	host, _, _ := net.SplitHostPort(address)
	if net.ParseIP(host).IsUnspecified() {
		return errors.New("服务端地址不能使用 0.0.0.0")
	}
	return nil
}

func readPairToken(input io.Reader, output io.Writer) (string, error) {
	fmt.Fprint(output, tr("配对密钥: ", "Pairing token: "))
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		data, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(output)
		return strings.TrimSpace(string(data)), err
	}
	data, err := io.ReadAll(io.LimitReader(input, 256))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func pairWithServer(dir, address, fingerprint, token string) error {
	if err := validateServerAddress(address); err != nil {
		return err
	}
	fingerprint, err := normalizeFingerprint(fingerprint)
	if err != nil {
		return err
	}
	if token == "" {
		return errors.New("配对密钥不能为空")
	}
	cfg := clientConfig{Server: address, Fingerprint: fingerprint, Token: token}
	client, err := newPinnedHTTPClient(fingerprint)
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
	return writeJSON(filepath.Join(dir, "client.json"), cfg)
}

func openViaClient(dir, folder string) error {
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
	cfg, err := readClientConfig(dir)
	if err != nil {
		return err
	}
	client, err := newPinnedHTTPClient(cfg.Fingerprint)
	if err != nil {
		return err
	}
	data, err := json.Marshal(openRequest{Path: resolved})
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

func authorizedRequest(client *http.Client, cfg clientConfig, method, endpoint string, body io.Reader) (*http.Response, error) {
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
	if language == "en-US" {
		for _, item := range errorTranslations {
			message = strings.ReplaceAll(message, item[0], item[1])
		}
	}
	return fmt.Errorf(tr("服务端返回 %d: %s", "Server returned %d: %s"), resp.StatusCode, message)
}
