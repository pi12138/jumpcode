package cmd

import (
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newServerStopCommand() *cobra.Command {
	return &cobra.Command{
		Use: "stop", Short: i18n.T("停止后台服务", "Stop the background server"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := config.ServerDir()
			if err != nil {
				return err
			}
			return server.Stop(dir)
		},
	}
}
