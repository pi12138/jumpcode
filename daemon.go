package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type daemonState struct {
	PID     int    `json:"pid"`
	Address string `json:"address"`
	Control string `json:"control"`
	Token   string `json:"token"`
}

func statePath(dir string) string { return filepath.Join(dir, "daemon.json") }

func readDaemonState(dir string) (daemonState, error) {
	var state daemonState
	data, err := os.ReadFile(statePath(dir))
	if err != nil {
		return state, err
	}
	if err = json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	if state.PID <= 0 || state.Address == "" || state.Control == "" || state.Token == "" {
		return state, errors.New(tr("后台状态文件无效", "Invalid daemon state file"))
	}
	host, port, err := net.SplitHostPort(state.Control)
	if err != nil || host != "127.0.0.1" {
		return state, errors.New(tr("后台控制地址无效", "Invalid daemon control address"))
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return state, errors.New(tr("后台控制端口无效", "Invalid daemon control port"))
	}
	return state, nil
}

func daemonRequest(state daemonState, method string) error {
	client := &http.Client{Timeout: time.Second}
	req, err := http.NewRequest(method, "http://"+state.Control+"/control", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+state.Token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func daemonStatus(dir string) error {
	if err := ensureWindowsServerCommand(); err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New(tr("后台服务未运行", "Background server is not running"))
	}
	if err != nil {
		return err
	}
	if err := daemonRequest(state, http.MethodGet); err != nil {
		return errorf("后台服务未运行（状态文件可能已过期）: %w", "Background server is not running (state file may be stale): %w", err)
	}
	fmt.Printf(tr("后台服务运行中：PID %d，https://%s\n", "Background server running: PID %d, https://%s\n"), state.PID, state.Address)
	return nil
}

func stopDaemon(dir string) error {
	if err := ensureWindowsServerCommand(); err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if err != nil {
		return err
	}
	if err := daemonRequest(state, http.MethodPost); err != nil {
		return errorf("停止后台服务失败: %w", "Failed to stop background server: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(statePath(dir)); errors.Is(err, os.ErrNotExist) {
			fmt.Println(tr("后台服务已停止", "Background server stopped"))
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New(tr("停止请求已发送，但服务未在 5 秒内退出", "Stop requested, but the server did not exit within 5 seconds"))
}

func serveDaemonControl(dir, address string, mainServer *http.Server) (func(), error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		listener.Close()
		return nil, err
	}
	state := daemonState{PID: os.Getpid(), Address: address, Control: listener.Addr().String(), Token: base64.RawURLEncoding.EncodeToString(secret)}
	control := &http.Server{ReadHeaderTimeout: 3 * time.Second}
	control.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+state.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				mainServer.Shutdown(ctx)
			}()
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	go control.Serve(listener)
	if err := writeJSON(statePath(dir), state); err != nil {
		control.Close()
		return nil, err
	}
	return func() {
		control.Close()
		current, err := readDaemonState(dir)
		if err == nil && current.Token == state.Token {
			os.Remove(statePath(dir))
		}
	}, nil
}
