package cmd

import (
	"fmt"
	"os"

	"jumpcode/internal/client"
	"jumpcode/internal/config"
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newPairJoinCommand() *cobra.Command {
	var address, fingerprint string
	cmd := &cobra.Command{
		Use:   "join",
		Short: i18n.T("使用 Windows 服务端提供的指纹和密钥完成配对", "Pair using the fingerprint and token from Windows"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ensureLinuxPairCommand(); err != nil {
				return err
			}
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			token, err := readPairToken(os.Stdin, os.Stderr)
			if err != nil {
				return err
			}
			if err := client.Join(dir, address, fingerprint, token); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("配对成功", "Pairing succeeded"))
			return nil
		},
	}
	cmd.Flags().StringVar(&address, "server", "", i18n.T("Windows 服务端的局域网 IPv4 地址和端口", "Windows server LAN IPv4 address and port"))
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", i18n.T("Windows 服务端的 SHA-256 证书指纹", "Windows server SHA-256 certificate fingerprint"))
	cmd.MarkFlagRequired("server")
	cmd.MarkFlagRequired("fingerprint")
	return cmd
}
