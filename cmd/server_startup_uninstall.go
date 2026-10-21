package cmd

import (
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newServerStartupUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use: "uninstall", Short: i18n.T("卸载登录启动任务", "Remove the sign-in task"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return server.UninstallStartup() },
	}
}
