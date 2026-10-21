package cmd

import (
	"jumpcode/internal/client"
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newClientCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     i18n.T("client <目录>", "client <folder>"),
		Aliases: []string{"c"},
		Short:   i18n.T("请求已配对的 Windows 服务端打开虚拟机目录", "Ask the paired Windows server to open a VM folder"),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			return client.Open(dir, args[0])
		},
	}
	return cmd
}
