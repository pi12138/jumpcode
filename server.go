package main

import (
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type openRequest struct {
	Path string `json:"path"`
}

func validateListenAddress(address string) error {
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

func runServer(dir, address string) error {
	return runServerMode(dir, address, false)
}

func runServerMode(dir, address string, daemonChild bool) error {
	return runServerModeWithLog(dir, address, daemonChild, nil)
}

func runServerModeWithLog(dir, address string, daemonChild bool, activityLog *log.Logger) error {
	if runtime.GOOS != "windows" {
		return errors.New(tr("首版服务端仅支持 Windows", "The server currently supports Windows only"))
	}
	if err := validateListenAddress(address); err != nil {
		return err
	}
	certPath, keyPath, fingerprint, err := ensureCertificate(dir)
	if err != nil {
		return err
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return errorf("监听 %s 失败: %w", "Failed to listen on %s: %w", address, err)
	}
	defer listener.Close()
	opener := openRemote
	if activityLog != nil {
		opener = func(target remoteTarget) error {
			err := openRemote(target)
			if err != nil {
				activityLog.Printf("打开请求失败，SSH 主机 %s: %v", target.Host, err)
			} else {
				activityLog.Printf("已接受打开请求，SSH 主机 %s", target.Host)
			}
			return err
		}
	}
	handler := newServerHandler(dir, opener)
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}},
	}
	var cleanup func()
	if daemonChild {
		mode := "daemon"
		if activityLog != nil {
			mode = "task"
		}
		cleanup, err = serveDaemonControlMode(dir, address, server, mode)
		if err != nil {
			return err
		}
		defer cleanup()
	}
	if activityLog != nil {
		activityLog.Printf("正在监听 https://%s，证书 SHA-256 指纹: %s", address, fingerprint)
	} else {
		fmt.Printf(tr("jc server 正在监听 https://%s\n证书 SHA-256 指纹: %s\n", "jc server listening at https://%s\nCertificate SHA-256 fingerprint: %s\n"), address, fingerprint)
	}
	err = server.Serve(tls.NewListener(listener, server.TLSConfig))
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newServerHandler(dir string, opener func(remoteTarget) error) http.Handler {
	mux := http.NewServeMux()
	withClient := func(next func(http.ResponseWriter, *http.Request, pairedClient)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodPost {
				http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
				return
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" || token == r.Header.Get("Authorization") {
				http.Error(w, "未授权", http.StatusUnauthorized)
				return
			}
			cfg, err := readServerConfig(dir)
			if err != nil {
				http.Error(w, "服务端配置读取失败", http.StatusInternalServerError)
				return
			}
			given := hashToken(token)
			for _, client := range cfg.Clients {
				if subtle.ConstantTimeCompare([]byte(given), []byte(client.TokenHash)) == 1 {
					next(w, r, client)
					return
				}
			}
			http.Error(w, "未授权", http.StatusUnauthorized)
		}
	}
	mux.HandleFunc("/v1/ping", withClient(func(w http.ResponseWriter, r *http.Request, _ pairedClient) {
		if r.Method != http.MethodGet {
			http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("/v1/open", withClient(func(w http.ResponseWriter, r *http.Request, client pairedClient) {
		if r.Method != http.MethodPost {
			http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
			return
		}
		var req openRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "请求格式无效", http.StatusBadRequest)
			return
		}
		if err := validateRemotePath(req.Path); err != nil || !strings.HasPrefix(req.Path, "/") {
			http.Error(w, "目录必须是绝对路径", http.StatusBadRequest)
			return
		}
		if err := opener(remoteTarget{Host: client.SSHHost, Path: path.Clean(req.Path)}); err != nil {
			http.Error(w, "启动 VS Code 失败: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	return mux
}

func pairClient(dir, name, sshHost string) (string, string, error) {
	if name == "" || strings.ContainsAny(name, "\r\n\t") {
		return "", "", errors.New(tr("虚拟机名称无效", "Invalid VM name"))
	}
	if err := validateSSHHost(sshHost); err != nil {
		return "", "", err
	}
	_, _, fingerprint, err := ensureCertificate(dir)
	if err != nil {
		return "", "", err
	}
	cfg, err := readServerConfig(dir)
	if err != nil {
		return "", "", err
	}
	for _, existing := range cfg.Clients {
		if existing.Name == name {
			return "", "", errorf("虚拟机 %q 已配对，请先撤销", "VM %q is already paired; revoke it first", name)
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	cfg.Clients = append(cfg.Clients, pairedClient{Name: name, SSHHost: sshHost, TokenHash: hashToken(token)})
	if err := writeServerConfig(dir, cfg); err != nil {
		return "", "", err
	}
	return fingerprint, token, nil
}

func revokeClient(dir, name string) error {
	cfg, err := readServerConfig(dir)
	if err != nil {
		return err
	}
	for i, client := range cfg.Clients {
		if client.Name == name {
			cfg.Clients = append(cfg.Clients[:i], cfg.Clients[i+1:]...)
			return writeServerConfig(dir, cfg)
		}
	}
	return errorf("未找到已配对虚拟机 %q", "Paired VM %q not found", name)
}

func ensureWindowsServerCommand() error {
	if runtime.GOOS != "windows" {
		return errors.New(tr("首版 jc server 命令仅支持 Windows", "jc server currently supports Windows only"))
	}
	return nil
}

func serverConfigDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := ensureConfigDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}
