package cmd

import (
	"fmt"
	"io"
	"runtime"
	"text/tabwriter"

	"jumpcode/internal/config"
	"jumpcode/internal/i18n"

	"github.com/spf13/cobra"
)

func newPairListCommand() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: i18n.T("列出本机保存的配对凭据", "List pairings saved on this machine"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
				return i18n.Errorf("jc pair list 仅支持 Windows 和 Linux", "jc pair list supports Windows and Linux only")
			}
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			if runtime.GOOS == "linux" {
				return pairStatus(cmd.OutOrStdout(), dir)
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
