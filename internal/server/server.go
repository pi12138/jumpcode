package server

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"runtime"
	"strings"
	"time"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/protocol"
	"jumpcode/internal/remote"
)

func runServer(dir string, specs []string) error {
	return Run(dir, specs, false)
}

func Run(dir string, specs []string, daemonChild bool) error {
	if daemonChild {
		return runLoggedServer(dir, specs)
	}
	return runServerWithLog(dir, specs, "", nil)
}

func runServerWithLog(dir string, specs []string, controlFile string, activityLog *log.Logger) error {
	if runtime.GOOS != "windows" {
		return errors.New(i18n.T("首版服务端仅支持 Windows", "The server currently supports Windows only"))
	}
	addresses, err := resolveListenAddresses(specs)
	if err != nil {
		return err
	}
	certPath, keyPath, fingerprint, err := config.EnsureCertificate(dir)
	if err != nil {
		return err
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return err
	}
	listeners, err := bindListenAddresses(addresses)
	if err != nil {
		return err
	}
	defer func() {
		for _, listener := range listeners {
			listener.Close()
		}
	}()
	opener := remote.Open
	if activityLog != nil {
		opener = logOpenRequests(activityLog, opener)
	}
	handler := newServerHandler(dir, opener)
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}},
		ErrorLog:          activityLog,
	}
	var cleanup func()
	if controlFile != "" {
		cleanup, err = serveControl(controlFile, addresses, server)
		if err != nil {
			return err
		}
		defer cleanup()
	}
	if activityLog != nil {
		activityLog.Printf("正在监听 %s，证书 SHA-256 指纹: %s", formatListenAddresses(addresses), fingerprint)
	} else {
		fmt.Printf(i18n.T("jc server 正在监听 %s\n证书 SHA-256 指纹: %s\n", "jc server listening at %s\nCertificate SHA-256 fingerprint: %s\n"), formatListenAddresses(addresses), fingerprint)
	}
	return serveHTTPSOnListeners(server, listeners)
}

func bindListenAddresses(addresses []string) ([]net.Listener, error) {
	var listeners []net.Listener
	for _, address := range addresses {
		listener, err := net.Listen("tcp4", address)
		if err != nil {
			for _, bound := range listeners {
				bound.Close()
			}
			return nil, i18n.Errorf("监听 %s 失败: %w", "Failed to listen on %s: %w", address, err)
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func serveHTTPSOnListeners(server *http.Server, listeners []net.Listener) error {
	results := make(chan error, len(listeners))
	for _, listener := range listeners {
		go func(listener net.Listener) {
			results <- server.Serve(tls.NewListener(listener, server.TLSConfig))
		}(listener)
	}
	var serveErr error
	for range listeners {
		err := <-results
		if err != nil && !errors.Is(err, http.ErrServerClosed) && serveErr == nil {
			serveErr = err
			server.Close()
		}
	}
	return serveErr
}

func newServerHandler(dir string, opener func(remote.Target) error) http.Handler {
	mux := http.NewServeMux()
	withClient := func(next func(http.ResponseWriter, *http.Request, config.PairedClient)) http.HandlerFunc {
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
			cfg, err := config.ReadServer(dir)
			if err != nil {
				http.Error(w, "服务端配置读取失败", http.StatusInternalServerError)
				return
			}
			given := protocol.HashToken(token)
			for _, client := range cfg.Clients {
				if subtle.ConstantTimeCompare([]byte(given), []byte(client.TokenHash)) == 1 {
					next(w, r, client)
					return
				}
			}
			http.Error(w, "未授权", http.StatusUnauthorized)
		}
	}
	mux.HandleFunc("/v1/ping", withClient(func(w http.ResponseWriter, r *http.Request, _ config.PairedClient) {
		if r.Method != http.MethodGet {
			http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("/v1/open", withClient(func(w http.ResponseWriter, r *http.Request, client config.PairedClient) {
		if r.Method != http.MethodPost {
			http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
			return
		}
		var req protocol.OpenRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "请求格式无效", http.StatusBadRequest)
			return
		}
		if err := remote.ValidatePath(req.Path); err != nil || !strings.HasPrefix(req.Path, "/") {
			http.Error(w, "目录必须是绝对路径", http.StatusBadRequest)
			return
		}
		if err := opener(remote.Target{Host: client.SSHHost, Path: path.Clean(req.Path)}); err != nil {
			http.Error(w, "启动 VS Code 失败: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	return mux
}

func CheckPlatform() error {
	if runtime.GOOS != "windows" {
		return errors.New(i18n.T("首版 jc server 命令仅支持 Windows", "jc server currently supports Windows only"))
	}
	return nil
}
