package cmd

import (
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

func ensureLinuxPairCommand(command string) error {
	if runtime.GOOS != "linux" {
		return i18n.Errorf("请在 Linux 客户端执行 jc pair %s", "Run jc pair %s on the Linux client", command)
	}
	return nil
}

func ensureWindowsPairCommand(command string) error {
	if runtime.GOOS != "windows" {
		return i18n.Errorf("请在 Windows 服务端执行 jc pair %s", "Run jc pair %s on the Windows server", command)
	}
	return nil
}
