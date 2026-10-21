package cmd

import (
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newServerStartupInstallCommand() *cobra.Command {
	var listen []string
	cmd := &cobra.Command{
		Use: "install", Short: i18n.T("安装当前用户的登录启动任务", "Install a sign-in task for the current user"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return server.InstallStartup(listen)
		},
	}
	cmd.Flags().StringArrayVar(&listen, "listen", nil, i18n.T("IPv4:端口（可重复）；auto 或 auto:端口监听所有私有 IPv4；默认 auto", "IPv4:port (repeatable); auto or auto:port listens on all private IPv4 addresses; default auto"))
	return cmd
}
