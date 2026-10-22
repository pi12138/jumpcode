package server

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"jumpcode/internal/config"
	"jumpcode/internal/remote"
	"jumpcode/internal/version"
)

func runLoggedServer(dir string, specs []string) error {
	return withServerLog(dir, specs, func(logger *log.Logger) error {
		return runServerWithLog(dir, specs, true, logger)
	})
}

// withServerLog 记录后台服务的生命周期，并保留服务原始错误。
func withServerLog(dir string, specs []string, run func(*log.Logger) error) error {
	if err := config.EnsureDir(dir); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("打开服务日志失败: %w", err)
	}
	defer f.Close()
	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("服务启动，PID: %d，版本: %s，监听配置: %s", os.Getpid(), version.String(), strings.Join(specs, ", "))
	if err := run(logger); err != nil {
		logger.Printf("服务退出，错误: %v", err)
		return err
	}
	logger.Print("服务已正常停止")
	return nil
}

func logOpenRequests(logger *log.Logger, opener func(remote.Target) error) func(remote.Target) error {
	return func(target remote.Target) error {
		err := opener(target)
		if err != nil {
			message := err.Error()
			// 保留失败原因，同时移除错误中可能附带的原始或 URI 编码后的项目路径。
			for _, value := range []string{target.Path, url.PathEscape(target.Path), (&url.URL{Path: target.Path}).EscapedPath()} {
				if value != "" {
					message = strings.ReplaceAll(message, value, "[项目路径]")
				}
			}
			logger.Printf("打开请求失败，SSH 主机 %s: %s", target.Host, message)
		} else {
			logger.Printf("已接受打开请求，SSH 主机 %s", target.Host)
		}
		return err
	}
}
