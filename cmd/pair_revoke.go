package cmd

import (
	"fmt"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/pairing"

	"github.com/spf13/cobra"
)

func newPairRevokeCommand() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use: "revoke", Aliases: []string{"r"},
		Short: i18n.T("在 Windows 撤销虚拟机配对凭据", "Revoke VM pairing credentials on Windows"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ensureWindowsPairCommand(cmd.Name()); err != nil {
				return err
			}
			dir, err := config.ServerDir()
			if err != nil {
				return err
			}
			if err := pairing.Revoke(dir, name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), i18n.T("已撤销配对 %q\n", "Revoked pairing %q\n"), name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", i18n.T("要撤销的虚拟机名称", "VM name to revoke"))
	cmd.MarkFlagRequired("name")
	return cmd
}
