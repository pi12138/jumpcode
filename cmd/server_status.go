package cmd

import (
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newServerStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use: "status", Aliases: []string{"st"}, Short: i18n.T("查看后台服务状态", "Show background server status"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := config.ServerDir()
			if err != nil {
				return err
			}
			return server.Status(dir)
		},
	}
}
