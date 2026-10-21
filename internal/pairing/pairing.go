package pairing

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/protocol"
	"jumpcode/internal/remote"
)

// Create 离线创建绑定 SSH 主机的凭据，返回服务端指纹和仅在此时提供的密钥。
func Create(dir, name, sshHost string) (string, string, error) {
	if name == "" || strings.ContainsAny(name, "\r\n\t") {
		return "", "", errors.New(i18n.T("虚拟机名称无效", "Invalid VM name"))
	}
	if err := remote.ValidateSSHHost(sshHost); err != nil {
		return "", "", err
	}
	_, _, fingerprint, err := config.EnsureCertificate(dir)
	if err != nil {
		return "", "", err
	}
	cfg, err := config.ReadServer(dir)
	if err != nil {
		return "", "", err
	}
	for _, existing := range cfg.Clients {
		if existing.Name == name {
			return "", "", i18n.Errorf("虚拟机 %q 已配对，请先撤销", "VM %q is already paired; revoke it first", name)
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	cfg.Clients = append(cfg.Clients, config.PairedClient{Name: name, SSHHost: sshHost, TokenHash: protocol.HashToken(token)})
	if err := config.WriteServer(dir, cfg); err != nil {
		return "", "", err
	}
	return fingerprint, token, nil
}

// Revoke 删除指定配对，服务端在下一次认证时读取更新后的配置。
func Revoke(dir, name string) error {
	cfg, err := config.ReadServer(dir)
	if err != nil {
		return err
	}
	for i, client := range cfg.Clients {
		if client.Name == name {
			cfg.Clients = append(cfg.Clients[:i], cfg.Clients[i+1:]...)
			return config.WriteServer(dir, cfg)
		}
	}
	return i18n.Errorf("未找到已配对虚拟机 %q", "Paired VM %q not found", name)
}
