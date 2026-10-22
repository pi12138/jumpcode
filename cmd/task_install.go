package cmd

import (
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newTaskInstallCommand() *cobra.Command {
	var listen []string
	cmd := &cobra.Command{
		Use: "install", Short: i18n.T("安装当前用户的登录自启任务（不立即启动）", "Install the current user's sign-in task without starting it"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return runTaskAction("install", listen) },
	}
	cmd.Flags().StringArrayVar(&listen, "listen", nil, i18n.T("IPv4:端口（可重复）；auto 或 auto:端口；默认 auto，运行时识别地址", "IPv4:port (repeatable); auto or auto:port; default auto, resolved at runtime"))
	return cmd
}
