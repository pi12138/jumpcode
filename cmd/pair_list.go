package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"
	"jumpcode/internal/server"

	"github.com/spf13/cobra"
)

func newPairListCommand() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: i18n.T("列出 Windows 上的配对", "List pairings on Windows"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := server.CheckPlatform(); err != nil {
				return err
			}
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			return listPairs(cmd.OutOrStdout(), dir)
		},
	}
}

func listPairs(output io.Writer, dir string) error {
	cfg, err := config.ReadServer(dir)
	if err != nil {
		return err
	}
	if len(cfg.Clients) == 0 {
		fmt.Fprintln(output, i18n.T("暂无配对凭据", "No pairings"))
		return nil
	}
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, i18n.T("名称\tSSH 主机", "Name\tSSH host"))
	for _, client := range cfg.Clients {
		fmt.Fprintf(w, "%s\t%s\n", client.Name, client.SSHHost)
	}
	return w.Flush()
}
