package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"jumpcode/internal/client"
	"jumpcode/internal/config"
	"jumpcode/internal/pairing"
	"jumpcode/internal/protocol"
	"jumpcode/internal/remote"
)

func TestPairedClientCanOnlyOpenBoundHost(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()
	certPath, keyPath, fingerprint, err := config.EnsureCertificate(serverDir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := pairing.Create(serverDir, "vm1", "fixed-host")
	if err != nil {
		t.Fatal(err)
	}
	var opened []remote.Target
	server := httptest.NewUnstartedServer(newServerHandler(serverDir, func(target remote.Target) error {
		opened = append(opened, target)
		return nil
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "https://")

	if err := client.Join(clientDir, address, fingerprint, token, protocol.DefaultRequestTimeout); err != nil {
		t.Fatalf("正确凭据应能配对: %v", err)
	}
	cfg, err := config.ReadClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	httpClient, err := protocol.NewHTTPClient(cfg.Fingerprint, protocol.DefaultRequestTimeout)
	if err != nil {
		t.Fatal(err)
	}
	workspace := "/home/me/project"
	openProject := func() error {
		data, _ := json.Marshal(protocol.OpenRequest{Path: workspace})
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/open", bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return nil
	}
	if runtime.GOOS == "linux" {
		workspace = t.TempDir()
		openProject = func() error { return client.Open(clientDir, workspace, protocol.DefaultRequestTimeout) }
	}
	if err := openProject(); err != nil {
		t.Fatalf("正确凭据应能打开目录: %v", err)
	}
	if len(opened) != 1 || opened[0].Host != "fixed-host" || opened[0].Path != workspace {
		t.Fatalf("请求必须绑定固定主机和目录: %#v", opened)
	}
	badRequest, _ := json.Marshal(map[string]string{"path": "/tmp/another", "host": "attacker"})
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/open", bytes.NewReader(badRequest))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || len(opened) != 1 {
		t.Fatalf("服务端应拒绝自选主机: status=%d opened=%#v", resp.StatusCode, opened)
	}

	if err := pairing.Revoke(serverDir, "vm1"); err != nil {
		t.Fatal(err)
	}
	if err := openProject(); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("撤销后必须拒绝请求，得到: %v", err)
	}
}

func TestWrongCertificateFingerprintFails(t *testing.T) {
	serverDir := t.TempDir()
	certPath, keyPath, _, err := config.EnsureCertificate(serverDir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(newServerHandler(serverDir, func(remote.Target) error { return nil }))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "https://")
	_, token, err := pairing.Create(serverDir, "vm1", "fixed-host")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Join(t.TempDir(), address, strings.Repeat("0", 64), token, protocol.DefaultRequestTimeout); err == nil || !strings.Contains(err.Error(), "指纹不匹配") {
		t.Fatalf("错误证书指纹必须拒绝，得到: %v", err)
	}
}
