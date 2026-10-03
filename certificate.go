package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func ensureCertificate(dir string) (string, string, string, error) {
	certPath := filepath.Join(dir, "server-cert.pem")
	keyPath := filepath.Join(dir, "server-key.pem")
	certData, certErr := os.ReadFile(certPath)
	_, keyErr := os.Stat(keyPath)
	if certErr == nil && keyErr == nil {
		fingerprint, err := certificateFingerprint(certData)
		return certPath, keyPath, fingerprint, err
	}
	if (!errors.Is(certErr, os.ErrNotExist) && certErr != nil) || (!errors.Is(keyErr, os.ErrNotExist) && keyErr != nil) {
		return "", "", "", errors.New("读取服务端证书失败")
	}
	if certErr == nil || keyErr == nil {
		return "", "", "", errors.New("服务端证书或私钥缺失，请检查配置目录")
	}
	if err := ensureConfigDir(dir); err != nil {
		return "", "", "", err
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", "", err
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return "", "", "", err
	}
	certData = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	// 先写私钥，避免证书存在而私钥尚未写完时生成另一套密钥。
	if err := writePrivateFile(keyPath, keyData); err != nil {
		return "", "", "", fmt.Errorf("保存私钥失败: %w", err)
	}
	if err := writePrivateFile(certPath, certData); err != nil {
		return "", "", "", fmt.Errorf("保存证书失败: %w", err)
	}
	return certPath, keyPath, hex.EncodeToString(sha256sum(der)), nil
}

func certificateFingerprint(pemData []byte) (string, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", errors.New("服务端证书格式无效")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return "", fmt.Errorf("服务端证书无效: %w", err)
	}
	return hex.EncodeToString(sha256sum(block.Bytes)), nil
}

func sha256sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
