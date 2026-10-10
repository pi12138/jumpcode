package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           tr("jc [目录或主机:目录]", "jc [folder or host:folder]"),
		Short:         tr("在 VS Code 中打开本地或远程项目", "Open local or remote projects in VS Code"),
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if runtime.GOOS != "windows" {
				return errors.New(tr("当前平台请使用 jc client . 请求 Windows 打开目录", "On this platform, use jc client . to ask Windows to open the folder"))
			}
			if !isRemoteInput(args[0]) {
				return openLocal(args[0])
			}
			target, err := parseRemoteTarget(args[0])
			if err != nil {
				return err
			}
			return openRemote(target)
		},
	}
	root.PersistentFlags().StringVar(&language, "lang", language, tr("界面语言：zh-CN 或 en-US", "Interface language: zh-CN or en-US"))
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if language != "zh-CN" && language != "en-US" {
			return errorf("不支持的语言 %q（可选 zh-CN、en-US）", "Unsupported language %q (choose zh-CN or en-US)", language)
		}
		return nil
	}
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		usage := cmd.UsageString()
		if language == "zh-CN" {
			usage = strings.NewReplacer("Usage:", "用法:", "Available Commands:", "可用命令:", "Flags:", "选项:", "Global Flags:", "全局选项:", "Aliases:", "别名:", "Use \"jc", "使用 \"jc", "for more information about a command.", "查看命令详情。", "help for", "查看帮助：").Replace(usage)
		}
		fmt.Fprintln(cmd.OutOrStdout(), cmd.Short)
		fmt.Fprint(cmd.OutOrStdout(), usage)
	})
	root.AddCommand(newServerCommand(), newClientCommand())
	root.SetHelpCommand(&cobra.Command{
		Use: "help [command]", Short: tr("查看命令帮助", "Help about a command"),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			cmd, _, err := root.Find(args)
			if err != nil {
				return err
			}
			if cmd == root {
				return errorf("未知命令 %q", "Unknown command %q", strings.Join(args, " "))
			}
			return cmd.Help()
		},
	})
	root.InitDefaultCompletionCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "completion" {
			cmd.Short = tr("生成指定 shell 的自动补全脚本", "Generate shell completion scripts")
			for _, shell := range cmd.Commands() {
				shell.Short = tr("生成 "+shell.Name()+" 自动补全脚本", "Generate the autocompletion script for "+shell.Name())
			}
		}
	}
	var setHelpFlags func(*cobra.Command)
	setHelpFlags = func(cmd *cobra.Command) {
		cmd.InitDefaultHelpFlag()
		if flag := cmd.Flags().Lookup("help"); flag != nil {
			flag.Usage = tr("显示帮助", "Show help")
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

func newServerCommand() *cobra.Command {
	var listen string
	var daemon, child, taskRun bool
	server := &cobra.Command{
		Use:     "server",
		Aliases: []string{"s"},
		Short:   tr("在 Windows 桌面会话中运行局域网服务端", "Run the LAN server in a Windows desktop session"),
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := serverConfigDir()
			if err != nil {
				return err
			}
			if (daemon && child) || (daemon && taskRun) || (child && taskRun) {
				return errors.New(tr("后台参数不能组合使用", "Daemon options cannot be combined"))
			}
			if taskRun {
				return runStartupServer(dir, listen)
			}
			if daemon {
				return startDaemon(dir, listen)
			}
			return runServerMode(dir, listen, child)
		},
	}
	server.Flags().StringVar(&listen, "listen", "", tr("明确的局域网 IPv4 地址和端口", "Explicit LAN IPv4 address and port"))
	server.Flags().BoolVarP(&daemon, "daemon", "d", false, tr("在当前 Windows 用户会话中后台运行", "Run in the current Windows user session in the background"))
	server.Flags().BoolVar(&child, "daemon-child", false, "internal")
	server.Flags().MarkHidden("daemon-child")
	server.Flags().BoolVar(&taskRun, "task-run", false, "internal")
	server.Flags().MarkHidden("task-run")
	server.MarkFlagRequired("listen")
	server.AddCommand(&cobra.Command{
		Use: "status", Aliases: []string{"st"}, Short: tr("查看后台服务状态", "Show background server status"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := serverConfigDir()
			if err != nil {
				return err
			}
			return daemonStatus(dir)
		},
	})
	server.AddCommand(&cobra.Command{
		Use: "stop", Short: tr("停止后台服务", "Stop the background server"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := serverConfigDir()
			if err != nil {
				return err
			}
			return stopDaemon(dir)
		},
	})
	startup := &cobra.Command{
		Use: "startup", Short: tr("管理登录后自动启动", "Manage startup after sign-in"), Args: cobra.NoArgs,
	}
	var startupListen string
	install := &cobra.Command{
		Use: "install", Short: tr("安装当前用户的登录启动任务", "Install a sign-in task for the current user"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return installStartupTask(startupListen)
		},
	}
	install.Flags().StringVar(&startupListen, "listen", "", tr("明确的局域网 IPv4 地址和端口", "Explicit LAN IPv4 address and port"))
	install.MarkFlagRequired("listen")
	startup.AddCommand(install)
	startup.AddCommand(&cobra.Command{
		Use: "status", Short: tr("查看登录启动任务状态", "Show sign-in task status"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return startupTaskStatus() },
	})
	startup.AddCommand(&cobra.Command{
		Use: "uninstall", Short: tr("卸载登录启动任务", "Remove the sign-in task"), Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return uninstallStartupTask() },
	})
	server.AddCommand(startup)

	var name, sshHost string
	pair := &cobra.Command{
		Use:     "pair",
		Aliases: []string{"p"},
		Short:   tr("创建虚拟机配对凭据并绑定 SSH 主机", "Pair a VM with a fixed SSH host"),
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := ensureWindowsServerCommand(); err != nil {
				return err
			}
			dir, err := serverConfigDir()
			if err != nil {
				return err
			}
			fingerprint, token, err := pairClient(dir, name, sshHost)
			if err != nil {
				return err
			}
			fmt.Printf(tr("已创建配对 %q，SSH 主机: %s\n证书 SHA-256 指纹: %s\n配对密钥（仅显示一次）: %s\n", "Paired %q, SSH host: %s\nCertificate SHA-256 fingerprint: %s\nPairing token (shown once): %s\n"), name, sshHost, fingerprint, token)
			return nil
		},
	}
	pair.Flags().StringVar(&name, "name", "", tr("虚拟机名称", "VM name"))
	pair.Flags().StringVar(&sshHost, "ssh-host", "", tr("Windows SSH config 中的主机别名或 user@host", "Windows SSH config alias or user@host"))
	pair.MarkFlagRequired("name")
	pair.MarkFlagRequired("ssh-host")
	server.AddCommand(pair)

	var revokeName string
	revoke := &cobra.Command{
		Use:     "revoke",
		Aliases: []string{"r"},
		Short:   tr("撤销虚拟机配对凭据", "Revoke VM pairing credentials"),
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := ensureWindowsServerCommand(); err != nil {
				return err
			}
			dir, err := serverConfigDir()
			if err != nil {
				return err
			}
			if err := revokeClient(dir, revokeName); err != nil {
				return err
			}
			fmt.Printf(tr("已撤销配对 %q\n", "Revoked pairing %q\n"), revokeName)
			return nil
		},
	}
	revoke.Flags().StringVar(&revokeName, "name", "", tr("要撤销的虚拟机名称", "VM name to revoke"))
	revoke.MarkFlagRequired("name")
	server.AddCommand(revoke)
	return server
}

func newClientCommand() *cobra.Command {
	client := &cobra.Command{
		Use:     tr("client <目录>", "client <folder>"),
		Aliases: []string{"c"},
		Short:   tr("请求已配对的 Windows 服务端打开虚拟机目录", "Ask the paired Windows server to open a VM folder"),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := configDir()
			if err != nil {
				return err
			}
			return openViaClient(dir, args[0])
		},
	}
	var address, fingerprint string
	pair := &cobra.Command{
		Use:     "pair",
		Aliases: []string{"p"},
		Short:   tr("使用 Windows 服务端提供的指纹和密钥完成配对", "Pair using the fingerprint and token from Windows"),
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if runtime.GOOS != "linux" {
				return errors.New(tr("首版 jc client 仅支持 Linux 虚拟机", "jc client currently supports Linux VMs only"))
			}
			dir, err := configDir()
			if err != nil {
				return err
			}
			token, err := readPairToken(os.Stdin, os.Stderr)
			if err != nil {
				return err
			}
			if err := pairWithServer(dir, address, fingerprint, token); err != nil {
				return err
			}
			fmt.Println(tr("配对成功", "Pairing succeeded"))
			return nil
		},
	}
	pair.Flags().StringVar(&address, "server", "", tr("Windows 服务端的局域网 IPv4 地址和端口", "Windows server LAN IPv4 address and port"))
	pair.Flags().StringVar(&fingerprint, "fingerprint", "", tr("Windows 服务端的 SHA-256 证书指纹", "Windows server SHA-256 certificate fingerprint"))
	pair.MarkFlagRequired("server")
	pair.MarkFlagRequired("fingerprint")
	client.AddCommand(pair)
	return client
}
