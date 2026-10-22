package cmd

import (
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newTaskUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use: "uninstall", Short: i18n.T("停止服务并卸载登录自启任务", "Stop the server and uninstall the sign-in task"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return runTaskAction("uninstall", nil) },
	}
}
