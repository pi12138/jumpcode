package server

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

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
)

type daemonState struct {
	PID       int      `json:"pid"`
	Address   string   `json:"address"`
	Addresses []string `json:"addresses,omitempty"`
	Control   string   `json:"control"`
	Token     string   `json:"token"`
}

func statePath(dir string) string { return filepath.Join(dir, "daemon.json") }

func readDaemonState(dir string) (daemonState, error) {
	return readControlState(statePath(dir))
}

func readControlState(file string) (daemonState, error) {
	var state daemonState
	data, err := os.ReadFile(file)
	if err != nil {
		return state, err
	}
	if err = json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	if state.PID <= 0 || state.Address == "" || state.Control == "" || state.Token == "" {
		return state, errors.New(i18n.T("后台状态文件无效", "Invalid daemon state file"))
	}
	host, port, err := net.SplitHostPort(state.Control)
	if err != nil || host != "127.0.0.1" {
		return state, errors.New(i18n.T("后台控制地址无效", "Invalid daemon control address"))
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return state, errors.New(i18n.T("后台控制端口无效", "Invalid daemon control port"))
	}
	return state, nil
}

func daemonRequest(state daemonState, method string) error {
	return controlRequest(context.Background(), state, method)
}

func controlRequest(ctx context.Context, state daemonState, method string) error {
	client := &http.Client{Timeout: time.Second}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+state.Control+"/control", nil)
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

func Status(dir string) error {
	if err := CheckPlatform(); err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New(i18n.T("后台服务未运行", "Background server is not running"))
	}
	if err != nil {
		return err
	}
	if err := daemonRequest(state, http.MethodGet); err != nil {
		return i18n.Errorf("后台服务未运行（状态文件可能已过期）: %w", "Background server is not running (state file may be stale): %w", err)
	}
	fmt.Printf(i18n.T("后台服务运行中：PID %d，%s\n", "Background server running: PID %d, %s\n"), state.PID, formatListenAddresses(stateListenAddresses(state)))
	return nil
}

func Stop(dir string) error {
	if err := CheckPlatform(); err != nil {
		return err
	}
	state, err := readDaemonState(dir)
	if err != nil {
		return err
	}
	if err := daemonRequest(state, http.MethodPost); err != nil {
		return i18n.Errorf("停止后台服务失败: %w", "Failed to stop background server: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(statePath(dir)); errors.Is(err, os.ErrNotExist) {
			fmt.Println(i18n.T("后台服务已停止", "Background server stopped"))
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New(i18n.T("停止请求已发送，但服务未在 5 秒内退出", "Stop requested, but the server did not exit within 5 seconds"))
}

func stateListenAddresses(state daemonState) []string {
	if len(state.Addresses) > 0 {
		return state.Addresses
	}
	return []string{state.Address}
}

func serveDaemonControl(dir string, addresses []string, mainServer *http.Server) (func(), error) {
	return serveControl(statePath(dir), addresses, mainServer)
}

func serveControl(file string, addresses []string, mainServer *http.Server) (func(), error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		listener.Close()
		return nil, err
	}
	state := daemonState{PID: os.Getpid(), Address: addresses[0], Addresses: addresses, Control: listener.Addr().String(), Token: base64.RawURLEncoding.EncodeToString(secret)}
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
	if err := config.WriteJSON(file, state); err != nil {
		control.Close()
		return nil, err
	}
	return func() {
		control.Close()
		current, err := readControlState(file)
		if err == nil && current.Token == state.Token {
			os.Remove(file)
		}
	}, nil
}
