package cmd

import (
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newTaskCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "task", Aliases: []string{"t"},
		Short: i18n.T("管理 Windows 登录自启计划任务", "Manage the Windows sign-in task"),
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newTaskInstallCommand(), newTaskUninstallCommand(), newTaskStartCommand(), newTaskStopCommand(), newTaskStatusCommand())
	return cmd
}

func runTaskAction(action string, listen []string) error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	return server.ManageTask(dir, action, listen)
}
