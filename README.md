# Jumpcode (`jc`)

[English](README.en.md)

在 Windows 中打开本地目录或 Linux 虚拟机项目；也可以从 Linux 虚拟机的独立终端请求 Windows 上的 VS Code 打开当前目录。远程项目使用 VS Code Remote-SSH，文件仍保存在虚拟机中。虚拟机到 Windows 的请求走局域网 HTTPS，不使用第三方中转服务。

## 前提

- Windows 已安装 VS Code 桌面版、Remote-SSH 扩展和 OpenSSH 客户端，且 `ssh` 可从终端执行。VS Code 需已注册 `vscode://` 协议。
- Windows 能通过 SSH 登录 Linux 虚拟机。建议先在 Windows 终端执行 `ssh devvm` 验证；`devvm` 是 Windows 用户 SSH config 中的别名。
- Linux 虚拟机能访问 Windows 的局域网地址与选定端口。对于本机 VMware 虚拟机，可以使用主机可访问的专用虚拟网络地址。

## 构建

需要 Go 1.24 或更新版本。在目标系统上构建：

```sh
go build -buildvcs=false -o jc .       # Linux
go build -buildvcs=false -o jc.exe .   # Windows
```

也可以在 Linux 上交叉构建：

```sh
GOOS=windows GOARCH=amd64 go build -buildvcs=false -o jc.exe .
GOOS=linux GOARCH=amd64 go build -buildvcs=false -o jc .
```

将对应的可执行文件放入 `PATH`。首次构建需要下载 `go.mod` 中的依赖。
这里使用 `-buildvcs=false`，是因为当前工作区没有完整的 Git 元数据；在正常 Git 仓库中可以省略。

## 从 Windows 打开项目

```powershell
jc .
jc .\project
jc C:\Users\me\project
jc devvm:/home/me/project
jc me@192.168.56.10:~/project
```

不含主机前缀的参数会作为 Windows 本地目录打开，包括相对路径和盘符路径。远端目标只接受以 `/` 或 `~/` 开头的路径；`~/` 会通过 SSH 查询远端用户的主目录。主机部分可使用 Windows SSH config 别名或 `user@host`。如果在 Linux 上直接运行 `jc .`，程序会提示改用 `jc client .`。

Windows 通过系统注册的 `vscode://` 协议打开 VS Code：本地目录使用 `vscode://file/...`，远端目录使用 `vscode://vscode-remote/ssh-remote+<主机>/<目录>`。程序不调用 `code.cmd`，也不修改 Electron 环境变量。VS Code 首次接收外部 URI 时可能要求确认；`jc` 只能确认 Windows 已接受启动请求，远程连接结果以 VS Code 窗口中的提示为准。

打开 VS Code 时，`jc` 使用一个无控制台的短暂子进程，避免新启动的 VS Code 把自身日志写进 `jc server` 所在终端。VS Code 自身的警告仍可能出现在其日志中；已运行的 VS Code 进程若原本连接到其他终端，此操作不会改变它的输出位置。

## 从虚拟机打开项目

首先在 Windows 当前登录用户的终端启动服务端，保持该终端运行：

```powershell
jc server --listen 192.168.56.1:59684
```

也可以在当前登录用户会话中后台运行（`-d` 等价于 `--daemon`）：

```powershell
jc s --listen 192.168.56.1:59684 -d
jc s status
jc s stop
```

后台日志位于用户配置目录的 `jumpcode/server.log`，运行状态保存在同目录的 `daemon.json`。`-d` 不会在下次登录后自动启动。`jc s status` 和 `jc s stop` 管理后台及登录启动模式，前台模式用 `Ctrl+C` 停止。

要在当前 Windows 用户登录后自动运行，执行：

```powershell
jc s startup install --listen 192.168.56.1:59684
jc s startup status
jc s startup uninstall
```

安装命令只注册计划任务，下次登录时启动；如需立即启动，可在 Windows 任务计划程序中手动运行该任务。任务中的隐藏启动器会等待服务进程并传递退出码，失败后最多间隔 1 分钟重试 3 次，不会在预设时长后停止。任务以当前登录用户身份运行，`jc.exe` 必须留在安装时的路径。`server.log` 记录启动、停止、失败及已接受的打开请求，不记录配对密钥或项目路径。`jc s stop` 只停止当前服务，任务仍会在下次登录时启动；`startup uninstall` 会移除任务并停止由任务启动的服务。

在 Windows 的另一个终端创建配对。命令会显示证书指纹和仅显示一次的配对密钥：

```powershell
jc server pair --name devvm --ssh-host devvm
```

在 Linux 虚拟机中执行以下命令，使用上一步显示的指纹，并在提示时粘贴配对密钥：

```sh
jc client pair --server 192.168.56.1:59684 --fingerprint <证书指纹>
jc client .
```

`jc client .` 会把当前目录换成绝对路径并请求 Windows 打开。也可用 `jc client /home/me/project` 指定其他目录。配对密钥只用于这台虚拟机；服务端按配对记录中的 SSH 别名连接，不接受客户端指定其他主机。

撤销配对时在 Windows 执行：

```powershell
jc server revoke --name devvm
```

server 在处理请求时读取最新配对配置，因此撤销立即生效。配置保存在用户配置目录下的 `jumpcode` 目录；Linux 客户端配置文件权限为当前用户可读写。请勿分享配对密钥或服务端私钥。若虚拟机访问失败，先检查 Windows 防火墙和监听地址；程序不会自行修改防火墙。

## 短命令与语言

`server` 可简写为 `s`，`client` 可简写为 `c`；配对 `pair` 可简写为 `p`，撤销 `revoke` 可简写为 `r`，查看后台状态 `status` 可简写为 `st`。例如：

```powershell
jc s p --name devvm --ssh-host devvm
```

```sh
jc c p --server 192.168.56.1:59684 --fingerprint <证书指纹>
jc c .
```

命令名优先于同名本地目录。如果 Windows 当前目录下有 `s` 或 `c` 文件夹，请使用 `jc ./s` 或 `jc ./c`。

CLI 支持简体中文和英文，默认根据系统语言选择；可用 `--lang zh-CN` 或 `--lang en-US` 指定，也可设置环境变量 `JC_LANG`。例如 `jc --lang en-US s --help`。

## 开发验证

```sh
go test -buildvcs=false ./...
go test -race -buildvcs=false ./...
```
