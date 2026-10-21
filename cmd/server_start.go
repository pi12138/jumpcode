package cmd

import (
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newServerStartCommand() *cobra.Command {
	return &cobra.Command{
		Use: "start", Short: i18n.T("立即启动已安装的登录启动任务", "Start the installed sign-in task now"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := server.CheckPlatform(); err != nil {
				return err
			}
			dir, err := config.ServerDir()
			if err != nil {
				return err
			}
			return server.Start(dir)
		},
	}
}
