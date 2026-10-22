package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"jumpcode/internal/i18n"
	"jumpcode/internal/remote"
	"jumpcode/internal/version"

	"github.com/spf13/cobra"
)

// Execute 初始化语言并执行命令，统一处理进程退出和错误输出。
func Execute() {
	remote.HandleOpenChild(os.Args[1:])
	i18n.SelectLanguage(os.Args[1:])
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "jc:", i18n.LocalizeError(err))
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           i18n.T("jc [目录或主机:目录]", "jc [folder or host:folder]"),
		Short:         i18n.T("在 VS Code 中打开本地或远程项目", "Open local or remote projects in VS Code"),
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if runtime.GOOS != "windows" {
				return errors.New(i18n.T("当前平台请使用 jc client . 请求 Windows 打开目录", "On this platform, use jc client . to ask Windows to open the folder"))
			}
			if !isRemoteInput(args[0]) {
				return remote.OpenLocal(args[0])
			}
			target, err := remote.ParseTarget(args[0])
			if err != nil {
				return err
			}
			return remote.Open(target)
		},
	}
	root.SetVersionTemplate("jc {{.Version}}\n")
	root.Flags().BoolP("version", "v", false, i18n.T("显示版本信息", "Show version information"))
	root.PersistentFlags().StringVar(&i18n.Language, "lang", i18n.Language, i18n.T("界面语言：zh-CN 或 en-US", "Interface language: zh-CN or en-US"))
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if i18n.Language != "zh-CN" && i18n.Language != "en-US" {
			return i18n.Errorf("不支持的语言 %q（可选 zh-CN、en-US）", "Unsupported language %q (choose zh-CN or en-US)", i18n.Language)
		}
		return nil
	}
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		usage := cmd.UsageString()
		if i18n.Language == "zh-CN" {
			usage = strings.NewReplacer("Usage:", "用法:", "Available Commands:", "可用命令:", "Flags:", "选项:", "Global Flags:", "全局选项:", "Aliases:", "别名:", "Use \"jc", "使用 \"jc", "for more information about a command.", "查看命令详情。", "help for", "查看帮助：").Replace(usage)
		}
		fmt.Fprintln(cmd.OutOrStdout(), cmd.Short)
		fmt.Fprint(cmd.OutOrStdout(), usage)
	})
	root.AddCommand(newServerCommand(), newClientCommand(), newPairCommand(), newTaskCommand())
	root.SetHelpCommand(&cobra.Command{
		Use: "help [command]", Short: i18n.T("查看命令帮助", "Help about a command"),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			cmd, _, err := root.Find(args)
			if err != nil {
				return err
			}
			if cmd == root {
				return i18n.Errorf("未知命令 %q", "Unknown command %q", strings.Join(args, " "))
			}
			return cmd.Help()
		},
	})
	root.InitDefaultCompletionCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "completion" {
			cmd.Short = i18n.T("生成指定 shell 的自动补全脚本", "Generate shell completion scripts")
			for _, shell := range cmd.Commands() {
				shell.Short = i18n.T("生成 "+shell.Name()+" 自动补全脚本", "Generate the autocompletion script for "+shell.Name())
			}
		}
	}
	var setHelpFlags func(*cobra.Command)
	setHelpFlags = func(cmd *cobra.Command) {
		cmd.InitDefaultHelpFlag()
		if flag := cmd.Flags().Lookup("help"); flag != nil {
			flag.Usage = i18n.T("显示帮助", "Show help")
		}
		for _, sub := range cmd.Commands() {
			setHelpFlags(sub)
		}
	}
	setHelpFlags(root)
	return root
}

func isRemoteInput(value string) bool {
	if len(value) >= 2 && value[1] == ':' && (value[0] >= 'a' && value[0] <= 'z' || value[0] >= 'A' && value[0] <= 'Z') {
		return false // Windows 盘符路径，例如 C:\project 或 C:project。
	}
	return strings.Contains(value, ":")
}
