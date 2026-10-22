package i18n

import (
	"fmt"
	"os"
	"strings"
)

var Language = detectLanguage()

func detectLanguage() string {
	for _, key := range []string{"JC_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" {
			if strings.HasPrefix(strings.ToLower(value), "zh") {
				return "zh-CN"
			}
			return "en-US"
		}
	}
	return systemLanguage()
}

func SelectLanguage(args []string) {
	for i, arg := range args {
		if arg == "--lang" && i+1 < len(args) {
			Language = args[i+1]
		} else if strings.HasPrefix(arg, "--lang=") {
			Language = strings.TrimPrefix(arg, "--lang=")
		}
	}
}

func T(zh, en string) string {
	if Language == "en-US" {
		return en
	}
	return zh
}

func Errorf(zh, en string, args ...any) error {
	return fmt.Errorf(T(zh, en), args...)
}

// 协议和配置层保留中文原文，命令边界统一翻译展示给用户的错误。
var errorTranslations = [][2]string{
	{"监听地址应为 局域网IPv4:端口", "Listen address must be LAN-IPv4:port"},
	{"请指定一个明确的 IPv4 监听地址，不要使用 0.0.0.0", "Specify an explicit IPv4 listen address, not 0.0.0.0"},
	{"端口必须在 1 到 65535 之间", "Port must be between 1 and 65535"},
	{"服务端证书数量无效", "Invalid server certificate count"},
	{"服务端证书已过期或尚未生效", "Server certificate is expired or not yet valid"},
	{"服务端证书指纹不匹配", "Server certificate fingerprint mismatch"},
	{"不接受服务端重定向", "Server redirects are not accepted"},
	{"服务端地址无效", "Invalid server address"},
	{"服务端地址不能使用 0.0.0.0", "Server address cannot be 0.0.0.0"},
	{"配对密钥不能为空", "Pairing token cannot be empty"},
	{"请求超时时间必须大于 0", "Request timeout must be greater than 0"},
	{"配对验证失败", "Pairing verification failed"},
	{"首版 jc client 仅支持 Linux 虚拟机", "jc client currently supports Linux VMs only"},
	{"解析目录失败", "Failed to resolve folder"},
	{"读取目录失败", "Failed to read folder"},
	{"目标必须是目录", "Target must be a folder"},
	{"解析目录符号链接失败", "Failed to resolve folder symlink"},
	{"连接 Windows 服务端失败", "Failed to connect to Windows server"},
	{"服务端返回", "Server returned"},
	{"读取服务端证书失败", "Failed to read server certificate"},
	{"服务端证书或私钥缺失，请检查配置目录", "Server certificate or private key is missing; check the config folder"},
	{"保存私钥失败", "Failed to save private key"},
	{"保存证书失败", "Failed to save certificate"},
	{"服务端证书格式无效", "Invalid server certificate format"},
	{"服务端证书无效", "Invalid server certificate"},
	{"获取用户配置目录失败", "Failed to locate user config folder"},
	{"创建配置目录失败", "Failed to create config folder"},
	{"读取服务端配置失败", "Failed to read server config"},
	{"解析服务端配置失败", "Failed to parse server config"},
	{"读取客户端配置失败，请先运行 jc pair join", "Failed to read client config; run jc pair join first"},
	{"解析客户端配置失败", "Failed to parse client config"},
	{"客户端配置不完整，请重新配对", "Client config is incomplete; pair again"},
	{"保存配置失败", "Failed to save config"},
	{"证书指纹必须是 SHA-256 十六进制值", "Certificate fingerprint must be a SHA-256 hexadecimal value"},
	{"证书指纹不是有效的十六进制值", "Certificate fingerprint is not valid hexadecimal"},
	{"远程目标格式应为 主机:/绝对路径 或 主机:~/路径", "Remote target must be host:/absolute/path or host:~/path"},
	{"SSH 主机名无效", "Invalid SSH host"},
	{"SSH 主机名仅支持字母、数字、点、下划线、连字符和 @", "SSH host may contain only letters, digits, dots, underscores, hyphens and @"},
	{"远程路径包含非法字符", "Remote path contains invalid characters"},
	{"远程路径必须以 / 或 ~/ 开头", "Remote path must start with / or ~/"},
	{"查询远端主目录失败", "Failed to query remote home directory"},
	{"远端返回的主目录不是绝对路径", "Remote home directory is not absolute"},
	{"当前平台请使用 jc client . 请求桌面端打开目录", "On this platform, use jc client . to ask Windows to open the folder"},
	{"读取本地目录失败", "Failed to read local folder"},
	{"本地目标必须是目录", "Local target must be a folder"},
	{"VS Code URI 无效", "Invalid VS Code URI"},
	{"获取 jc 程序路径失败", "Failed to locate jc executable"},
	{"无法通过 vscode:// 打开 VS Code，请确认已安装桌面版并注册协议", "Failed to open VS Code through vscode://; check that the desktop app and protocol handler are installed"},
	{"首版仅支持在 Windows 桌面端打开 VS Code", "Opening VS Code is currently supported only on a Windows desktop"},
	{"不支持的请求方法", "Unsupported request method"},
	{"未授权", "Unauthorized"},
	{"服务端配置读取失败", "Failed to read server config"},
	{"请求格式无效", "Invalid request format"},
	{"目录必须是绝对路径", "Folder must be an absolute path"},
	{"启动 VS Code 失败", "Failed to launch VS Code"},
}

func LocalizeError(err error) string {
	message := err.Error()
	if Language == "zh-CN" {
		for _, item := range [][2]string{
			{"unknown command", "未知命令"},
			{"for \"jc\"", "对于 \"jc\""},
			{"accepts 1 arg(s), received", "需要 1 个参数，实际收到"},
			{"accepts 0 arg(s), received", "不接受参数，实际收到"},
			{"required flag(s)", "缺少必需选项"},
			{"not set", "未设置"},
			{"unknown flag:", "未知选项:"},
		} {
			message = strings.ReplaceAll(message, item[0], item[1])
		}
		return message
	}
	for _, item := range errorTranslations {
		message = strings.ReplaceAll(message, item[0], item[1])
	}
	return message
}
