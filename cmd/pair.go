package cmd

import (
	"errors"
	"runtime"

	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newPairCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "pair", Aliases: []string{"p"},
		Short: i18n.T("管理配对凭据", "Manage pairing credentials"), Args: cobra.NoArgs,
	}
	cmd.AddCommand(newPairCreateCommand(), newPairJoinCommand(), newPairListCommand(), newPairRevokeCommand(), newPairStatusCommand())
	return cmd
}

func ensureLinuxPairCommand() error {
	if runtime.GOOS != "linux" {
		return errors.New(i18n.T("首版 jc client 仅支持 Linux 虚拟机", "jc client currently supports Linux VMs only"))
	}
	return nil
}
