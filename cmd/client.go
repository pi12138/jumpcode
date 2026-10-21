package cmd

import (
	"time"

	"jumpcode/internal/client"
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/protocol"

	"github.com/spf13/cobra"
)

func newClientCommand() *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:     i18n.T("client <目录>", "client <folder>"),
		Aliases: []string{"c"},
		Short:   i18n.T("请求已配对的 Windows 服务端打开虚拟机目录", "Ask the paired Windows server to open a VM folder"),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := protocol.ValidateRequestTimeout(timeout); err != nil {
				return err
			}
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			return client.Open(dir, args[0], timeout)
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", protocol.DefaultRequestTimeout, i18n.T("请求总超时（如 5s、1m）；连接超时固定为 3s", "Total request timeout (e.g. 5s, 1m); connection timeout is fixed at 3s"))
	return cmd
}
