package cmd

import (
	"fmt"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/pairing"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newPairCreateCommand() *cobra.Command {
	var name, sshHost string
	cmd := &cobra.Command{
		Use:   "create",
		Short: i18n.T("创建虚拟机配对凭据并绑定 SSH 主机", "Pair a VM with a fixed SSH host"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := server.CheckPlatform(); err != nil {
				return err
			}
			dir, err := config.ServerDir()
			if err != nil {
				return err
			}
			fingerprint, token, err := pairing.Create(dir, name, sshHost)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), i18n.T("已创建配对 %q，SSH 主机: %s\n证书 SHA-256 指纹: %s\n配对密钥（仅显示一次）: %s\n", "Paired %q, SSH host: %s\nCertificate SHA-256 fingerprint: %s\nPairing token (shown once): %s\n"), name, sshHost, fingerprint, token)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", i18n.T("虚拟机名称", "VM name"))
	cmd.Flags().StringVar(&sshHost, "ssh-host", "", i18n.T("SSH config 别名或 user@host", "SSH config alias or user@host"))
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("ssh-host")
	return cmd
}
