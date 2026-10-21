package cmd

import (
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newServerStartupStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: i18n.T("查看登录启动任务状态", "Show sign-in task status"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return server.StartupStatus() },
	}
}
