package cmd

import (
	"fmt"
	"time"

	"jumpcode/internal/client"
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/protocol"

	"github.com/spf13/cobra"
)

func newPairJoinCommand() *cobra.Command {
	var address, fingerprint string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "join",
		Short: i18n.T("在 Linux 使用 Windows 服务端提供的指纹和密钥配对", "Pair on Linux using the fingerprint and token from Windows"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ensureLinuxPairCommand(cmd.Name()); err != nil {
				return err
			}
			if err := protocol.ValidateRequestTimeout(timeout); err != nil {
				return err
			}
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			token, err := readPairToken(cmd.InOrStdin(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if err := client.Join(dir, address, fingerprint, token, timeout); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("配对成功", "Pairing succeeded"))
			return nil
		},
	}
	cmd.Flags().StringVar(&address, "server", "", i18n.T("Windows 服务端的局域网 IPv4 地址和端口", "Windows server LAN IPv4 address and port"))
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", i18n.T("Windows 服务端的 SHA-256 证书指纹", "Windows server SHA-256 certificate fingerprint"))
	cmd.Flags().DurationVar(&timeout, "timeout", protocol.DefaultRequestTimeout, i18n.T("请求总超时（如 5s、1m）；连接超时固定为 3s", "Total request timeout (e.g. 5s, 1m); connection timeout is fixed at 3s"))
	cmd.MarkFlagRequired("server")
	cmd.MarkFlagRequired("fingerprint")
	return cmd
}
