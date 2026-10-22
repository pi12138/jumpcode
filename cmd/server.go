package cmd

import (
	"errors"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newServerCommand() *cobra.Command {
	var listen []string
	var daemon, child bool
	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"s"},
		Short:   i18n.T("在 Windows 桌面会话中运行局域网服务端", "Run the LAN server in a Windows desktop session"),
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := config.ServerDir()
			if err != nil {
				return err
			}
			if daemon && child {
				return errors.New(i18n.T("后台参数不能组合使用", "Daemon options cannot be combined"))
			}
			if daemon {
				return server.StartBackground(dir, listen)
			}
			return server.Run(dir, listen, child)
		},
	}
	cmd.Flags().StringArrayVar(&listen, "listen", nil, i18n.T("IPv4:端口（可重复）；auto 或 auto:端口监听所有私有 IPv4；默认 auto", "IPv4:port (repeatable); auto or auto:port listens on all private IPv4 addresses; default auto"))
	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, i18n.T("在当前 Windows 用户会话中后台运行", "Run in the current Windows user session in the background"))
	cmd.Flags().BoolVar(&child, "daemon-child", false, "internal")
	cmd.Flags().MarkHidden("daemon-child")
	cmd.AddCommand(newServerStatusCommand(), newServerStopCommand())
	return cmd
}
