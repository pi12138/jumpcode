package protocol

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
)

// NewHTTPClient 固定服务端证书指纹并拒绝重定向，避免凭据发送到其他服务端。
func NewHTTPClient(fingerprint string) (*http.Client, error) {
	fingerprint, err := NormalizeFingerprint(fingerprint)
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

func NormalizeFingerprint(value string) (string, error) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
	if len(value) != sha256.Size*2 {
		return "", errors.New("证书指纹必须是 SHA-256 十六进制值")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", errors.New("证书指纹不是有效的十六进制值")
	}
	return value, nil
}
