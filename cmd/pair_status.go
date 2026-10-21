package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newPairStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: i18n.T("查看 Linux 客户端本地保存的配对", "Show the pairing saved locally on the Linux client"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ensureLinuxPairCommand(cmd.Name()); err != nil {
				return err
			}
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			return pairStatus(cmd.OutOrStdout(), dir)
		},
	}
}

func pairStatus(output io.Writer, dir string) error {
	cfg, err := config.ReadClient(dir)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(output, i18n.T("暂无配对凭据", "No pairings"))
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(output, i18n.T("服务端: %s\n证书 SHA-256 指纹: %s\n", "Server: %s\nCertificate SHA-256 fingerprint: %s\n"), cfg.Server, cfg.Fingerprint)
	return nil
}
