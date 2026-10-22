package cmd

import (
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newTaskStartCommand() *cobra.Command {
	return &cobra.Command{
		Use: "start", Short: i18n.T("通过计划任务启动服务并等待就绪", "Start the server through Task Scheduler and wait for readiness"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return runTaskAction("start", nil) },
	}
}
