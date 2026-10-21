package cmd

import (
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newServerStartupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "startup", Short: i18n.T("管理登录后自动启动", "Manage startup after sign-in"), Args: cobra.NoArgs,
	}
	cmd.AddCommand(newServerStartupInstallCommand(), newServerStartupStatusCommand(), newServerStartupUninstallCommand())
	return cmd
}
