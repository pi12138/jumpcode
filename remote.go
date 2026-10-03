package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type remoteTarget struct {
	Host string
	Path string
}

func parseRemoteTarget(value string) (remoteTarget, error) {
	host, folder, ok := strings.Cut(value, ":")
	if !ok || host == "" || folder == "" {
		return remoteTarget{}, errors.New("远程目标格式应为 主机:/绝对路径 或 主机:~/路径")
	}
	if err := validateSSHHost(host); err != nil {
		return remoteTarget{}, err
	}
	if err := validateRemotePath(folder); err != nil {
		return remoteTarget{}, err
	}
	return remoteTarget{Host: host, Path: folder}, nil
}

func validateSSHHost(host string) error {
	if host == "" || strings.HasPrefix(host, "-") || strings.Count(host, "@") > 1 {
		return errors.New("SSH 主机名无效")
	}
	for _, char := range host {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._-@", char) {
			continue
		}
		return errors.New("SSH 主机名仅支持字母、数字、点、下划线、连字符和 @")
	}
	return nil
}

func validateRemotePath(folder string) error {
	if strings.ContainsAny(folder, "\x00\r\n") {
		return errors.New("远程路径包含非法字符")
	}
	if !strings.HasPrefix(folder, "/") && folder != "~" && !strings.HasPrefix(folder, "~/") {
		return errors.New("远程路径必须以 / 或 ~/ 开头")
	}
	return nil
}

func resolveRemotePath(host, folder string) (string, error) {
	if !strings.HasPrefix(folder, "~") {
		return path.Clean(folder), nil
	}
	// 命令文本是常量，远程路径不会进入 shell。
	out, err := exec.Command("ssh", "--", host, `printf '%s' "$HOME"`).Output()
	if err != nil {
		return "", fmt.Errorf("查询远端主目录失败: %w", err)
	}
	home := strings.TrimSpace(string(out))
	if !strings.HasPrefix(home, "/") {
		return "", errors.New("远端返回的主目录不是绝对路径")
	}
	if folder == "~" {
		return path.Clean(home), nil
	}
	return path.Join(home, strings.TrimPrefix(folder, "~/")), nil
}

func openLocal(folder string) error {
	if runtime.GOOS != "windows" {
		return errors.New("当前平台请使用 jc client . 请求桌面端打开目录")
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("读取本地目录失败: %w", err)
	}
	if !info.IsDir() {
		return errors.New("本地目标必须是目录")
	}
	return openURI(localVSCodeURL(filepath.ToSlash(abs)))
}

func openRemote(target remoteTarget) error {
	folder, err := resolveRemotePath(target.Host, target.Path)
	if err != nil {
		return err
	}
	return openURI(remoteVSCodeURL(target.Host, folder))
}

func localVSCodeURL(folder string) string {
	// vscode://file/ 是 VS Code 对本地目录公开的协议入口。
	return (&url.URL{Scheme: "vscode", Host: "file", Path: trailingSlash("/" + strings.TrimPrefix(folder, "/"))}).String()
}

func remoteVSCodeURL(host, folder string) string {
	// vscode://vscode-remote/ssh-remote+<host>/<path> 由 VS Code Remote Core 解析。
	return (&url.URL{Scheme: "vscode", Host: "vscode-remote", Path: trailingSlash("/ssh-remote+" + host + path.Clean(folder))}).String()
}

func trailingSlash(value string) string {
	if strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}
