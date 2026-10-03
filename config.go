package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type pairedClient struct {
	Name      string `json:"name"`
	SSHHost   string `json:"ssh_host"`
	TokenHash string `json:"token_hash"`
}

type serverConfig struct {
	Clients []pairedClient `json:"clients"`
}

type clientConfig struct {
	Server      string `json:"server"`
	Fingerprint string `json:"fingerprint"`
	Token       string `json:"token"`
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取用户配置目录失败: %w", err)
	}
	return filepath.Join(base, "jumpcode"), nil
}

func ensureConfigDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	return nil
}

func readServerConfig(dir string) (serverConfig, error) {
	var cfg serverConfig
	data, err := os.ReadFile(filepath.Join(dir, "server.json"))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("读取服务端配置失败: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析服务端配置失败: %w", err)
	}
	return cfg, nil
}

func writeServerConfig(dir string, cfg serverConfig) error {
	return writeJSON(filepath.Join(dir, "server.json"), cfg)
}

func readClientConfig(dir string) (clientConfig, error) {
	var cfg clientConfig
	data, err := os.ReadFile(filepath.Join(dir, "client.json"))
	if err != nil {
		return cfg, fmt.Errorf("读取客户端配置失败，请先运行 jc client pair: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析客户端配置失败: %w", err)
	}
	if cfg.Server == "" || cfg.Fingerprint == "" || cfg.Token == "" {
		return cfg, errors.New("客户端配置不完整，请重新配对")
	}
	return cfg, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writePrivateFile(path, data)
}

func writePrivateFile(path string, data []byte) error {
	if err := ensureConfigDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".jc-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeFingerprint(value string) (string, error) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
	if len(value) != sha256.Size*2 {
		return "", errors.New("证书指纹必须是 SHA-256 十六进制值")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", errors.New("证书指纹不是有效的十六进制值")
	}
	return value, nil
}
