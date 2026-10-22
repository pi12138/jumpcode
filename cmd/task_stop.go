package cmd

import (
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newTaskStopCommand() *cobra.Command {
	return &cobra.Command{
		Use: "stop", Short: i18n.T("通知计划任务服务正常停止", "Request graceful shutdown of the task server"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return runTaskAction("stop", nil) },
	}
}
