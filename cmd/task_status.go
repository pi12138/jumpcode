package cmd

import (
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newTaskStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use: "status", Aliases: []string{"st"}, Short: i18n.T("查看任务状态、服务就绪情况和日志位置", "Show scheduler state, server readiness, and log location"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return runTaskAction("status", nil) },
	}
}
