package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type PairedClient struct {
	Name      string `json:"name"`
	SSHHost   string `json:"ssh_host"`
	TokenHash string `json:"token_hash"`
}

type Server struct {
	Clients []PairedClient `json:"clients"`
}

type Client struct {
	Server      string `json:"server"`
	Fingerprint string `json:"fingerprint"`
	Token       string `json:"token"`
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取用户配置目录失败: %w", err)
	}
	return filepath.Join(base, "jumpcode"), nil
}

func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	return nil
}

func ReadServer(dir string) (Server, error) {
	var cfg Server
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

func WriteServer(dir string, cfg Server) error {
	return WriteJSON(filepath.Join(dir, "server.json"), cfg)
}

func ReadClient(dir string) (Client, error) {
	var cfg Client
	data, err := os.ReadFile(filepath.Join(dir, "client.json"))
	if err != nil {
		return cfg, fmt.Errorf("读取客户端配置失败，请先运行 jc pair join: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析客户端配置失败: %w", err)
	}
	if cfg.Server == "" || cfg.Fingerprint == "" || cfg.Token == "" {
		return cfg, errors.New("客户端配置不完整，请重新配对")
	}
	return cfg, nil
}

func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writePrivateFile(path, data)
}

func writePrivateFile(path string, data []byte) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
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

func ServerDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := EnsureDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}
