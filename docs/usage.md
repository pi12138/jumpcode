# 使用指南

[项目首页](../README.md) · [使用指南](usage.md) · [架构说明](architecture.md) · [开发指南](development.md) · [English](usage.en.md)

先按[项目首页](../README.md)的安装说明准备对应平台的可执行文件；以下步骤说明配对和日常使用。源码构建方法见[开发指南](development.md)。

## 使用前提

- Windows 已安装 VS Code 桌面版、Remote-SSH 扩展和 OpenSSH 客户端，且 `ssh` 可从终端执行。VS Code 需已注册 `vscode://` 协议。
- 打开 Linux 远程项目时，Windows 能通过 SSH 登录虚拟机。可用 `ssh me@192.168.56.10` 验证；SSH config 别名（如 `devvm`）只是可选的简写。
- 从 Linux 终端发起打开请求时，虚拟机能访问 Windows 的监听地址与端口。对于本机 VMware 虚拟机，可以使用主机可访问的专用虚拟网络地址。

## 首次使用：从 Linux 虚拟机打开项目

以下示例假设 Windows 主机在虚拟机可访问的网络上的地址是 `192.168.56.1`，虚拟机的 SSH 别名是 `devvm`。请替换为你自己的地址、用户名和目录。先把 `jc.exe` 放在固定的本地目录，并在当前登录用户的 PowerShell 中进入该目录；示例因此使用 `.\jc.exe`。如果已加入 `PATH`，可以直接写 `jc`。

1. 在 Windows 上确认可以通过 SSH 进入虚拟机。在 `%USERPROFILE%\.ssh\config` 中可配置别名：

   ```sshconfig
   Host devvm
       HostName 192.168.56.10
       User me
   ```

   ```powershell
   ssh devvm
   ```

   SSH config 不是必需的；也可直接运行 `ssh me@192.168.56.10`，后续把 `--ssh-host` 设为 `me@192.168.56.10`。先确认 VS Code 已安装 Remote-SSH，且所选 SSH 主机可以成功登录；退出 SSH 后继续。

2. 在 Windows 上启动后台服务，并查看它实际监听的地址：

   ```powershell
   .\jc.exe s --listen auto -d
   .\jc.exe s status
   ```

   `auto` 会使用端口 `59684` 监听所有启用网卡的私有 IPv4。请从状态输出中选一个**虚拟机能够访问**的地址；以下示例使用 `192.168.56.1:59684`。如果不希望监听其他网卡，改用 `--listen 192.168.56.1:59684`。

3. 在同一个 Windows 用户下创建配对：

   ```powershell
   .\jc.exe pair create --name devvm --ssh-host devvm
   ```

   记下输出的证书 SHA-256 指纹和仅显示一次的配对密钥。`--ssh-host` 要与第 1 步验证过的 Windows SSH 主机名一致。

4. 在 Linux 虚拟机中放好对应平台的 `jc`。例如将当前目录中的二进制文件安装到当前用户目录，再用上一步的指纹配对：

   ```sh
   mkdir -p "$HOME/.local/bin"
   install -m 755 ./jc "$HOME/.local/bin/jc"
   "$HOME/.local/bin/jc" pair join --server 192.168.56.1:59684 --fingerprint '粘贴 Windows 输出的证书指纹'
   ```

   命令提示输入配对密钥时，粘贴第 3 步得到的密钥并回车；终端输入不会回显。配对命令会立即连接 Windows 服务端验证地址、指纹和密钥。

5. 在 Linux 中打开项目：

   ```sh
   cd /home/me/project
   "$HOME/.local/bin/jc" c .
   ```

   Windows 上的 VS Code 应尝试连接 `devvm`，最终连接结果以 VS Code 窗口为准。此后在同一 Linux 用户下继续使用该命令即可；如果 `$HOME/.local/bin` 已在 `PATH`，可简写为 `jc c .`。客户端会记住本次配对使用的一个 Windows 地址。Linux 上直接执行 `jc .` 不会请求 Windows 打开目录。

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

## 服务端运行与管理

若不使用后台模式，可在 Windows 当前登录用户的终端前台启动服务端，保持该终端运行：

```powershell
jc server --listen 192.168.56.1:59684
```

按需将监听参数换为 `--listen auto`，或重复指定，例如 `--listen 192.168.56.1:59684 --listen 192.168.1.10:59684`。省略时等价于 `--listen auto`，使用端口 `59684` 监听所有已启用网卡上的私有 IPv4 地址；`--listen auto:59700` 可指定端口，也可与明确的监听地址组合。自动选择可能包含 VPN 和虚拟网卡地址，不包含回环、链路本地或公网地址。程序启动时会显示实际监听列表；若任何地址绑定失败，启动整体失败，不会只监听部分地址。网卡地址变化后需要重新启动服务。

也可以在当前登录用户会话中后台运行（`-d` 等价于 `--daemon`）：

```powershell
jc s --listen 192.168.56.1:59684 -d
jc s status
jc s stop
```

后台日志位于用户配置目录的 `jumpcode/server.log`，运行状态保存在同目录的 `daemon.json`。日志记录时间、PID、版本、监听地址、正常停止、异常退出，以及打开请求的成功或失败原因；配对密钥和项目路径不写入日志。日志追加写入，当前没有自动轮转。`jc s status` 和 `jc s stop` 管理后台服务，前台模式用 `Ctrl+C` 停止。退出登录或重启后，需要再次手动启动服务。

| 操作 | 命令 |
| --- | --- |
| 前台运行 | `jc s --listen auto` |
| 后台运行 | `jc s --listen auto -d` |
| 查看后台服务 | `jc s status` |
| 停止后台服务 | `jc s stop` |

### 旧版任务清理

当前源码版已移除 `jc s startup`、`jc s start` 和计划任务支持。如果旧版曾安装过登录启动任务，请在替换程序前使用**旧版 jc** 执行：

```powershell
jc s startup uninstall
```

该命令会移除任务并停止任务启动的服务，保留配对凭据和证书。如果已经替换程序，可在 Windows 任务计划程序中删除当前用户的 `Jumpcode Server <用户 SID>` 任务，并执行 `jc s stop` 停止仍在运行的后台服务。升级本身不会删除 Windows 中已有的任务。之后使用 `jc s -d` 手动启动。

## 配对与凭据

在 Windows 的另一个终端创建配对。命令会显示证书指纹和仅显示一次的配对密钥：

```powershell
jc pair create --name devvm --ssh-host devvm
jc pair list
```

执行 `pair create` 时服务可以尚未启动；执行 `pair join` 时，Windows 服务必须已经运行且可从 Linux 访问。在 Linux 虚拟机中执行以下命令，使用上一步显示的指纹，并在提示时粘贴配对密钥：

```sh
jc pair join --server 192.168.56.1:59684 --fingerprint '粘贴证书指纹'
jc pair list
jc pair status
jc client .
```

`jc client .` 会把当前目录换成绝对路径并请求 Windows 打开。也可用 `jc client /home/me/project` 指定其他目录。配对密钥只用于这台虚拟机；服务端按配对记录中的 SSH 别名连接，不接受客户端指定其他主机。

服务端可以监听多个 IP，但每个 Linux 客户端只保存一个服务端地址；配对时请选择该虚拟机可访问的地址，地址变化后需要重新配对。

`jc pair list` 在两端均可执行：Windows 显示已签发的配对名称和绑定的 SSH 主机，Linux 显示本机保存的服务端地址和证书指纹；不显示配对密钥或密钥哈希。尚未配对时显示“暂无配对凭据”。Linux 上的 `jc pair status` 也显示同一份本地配对信息；这些查看命令不连接服务端，不能判断服务是否在线、密钥是否已被撤销。

`pair create/revoke` 需要在 Windows 服务端执行，`pair join/status` 需要在 Linux 客户端执行；误用时会提示对应命令的执行位置。撤销配对时在 Windows 执行：

```powershell
jc pair revoke --name devvm
```

server 在处理请求时读取最新配对配置，因此撤销立即生效。Windows 配对记录位于用户配置目录的 `jumpcode/server.json`，只保存密钥哈希；Linux 配对记录位于 `jumpcode/client.json`，包含配对密钥，文件权限为当前用户可读写。`jc pair revoke` 不会删除 Linux 上保存的记录，但该密钥已无法访问 Windows 服务端。请勿分享配对密钥或服务端私钥。若虚拟机访问失败，先检查 Windows 防火墙和监听地址；程序不会自行修改防火墙。

## 常见问题

| 现象 | 检查方法 |
| --- | --- |
| `--listen auto` 启动失败 | 检查 Windows 是否有已启用的私有 IPv4 网卡，以及输出中的具体绑定错误；必要时改用明确的 `--listen IP:端口`。任一候选地址绑定失败会使整个启动失败。 |
| Linux 配对时报“连接失败” | 在 Windows 执行 `jc s status`，确认选择的是实际监听且虚拟机可达的地址；检查 Windows 防火墙是否允许虚拟机访问该端口。`auto` 选出的其他网卡地址不一定对这台虚拟机可达。 |
| 指纹不匹配或返回 401 | 核对 Windows 配对输出中的证书指纹和密钥，不要把两个值混用。如果密钥丢失，在 Windows 用 `jc pair revoke --name devvm` 撤销，再创建新配对并在 Linux 重新配对。 |
| Linux 显示请求成功，但 VS Code 未连上项目 | 在 Windows 再次运行 `ssh devvm`，检查 Remote-SSH 扩展、`vscode://` 协议注册和 VS Code 窗口中的报错。请求成功只表示 Windows 接受了打开请求。 |

后台日志可在 Windows PowerShell 中查看：

```powershell
Get-Content (Join-Path $env:APPDATA 'jumpcode\server.log') -Tail 50
```

## 短命令与语言

`server` 可简写为 `s`，`client` 可简写为 `c`，顶层 `pair` 可简写为 `p`，撤销 `revoke` 可简写为 `r`，服务后台状态 `server status` 可简写为 `s st`。配对凭据统一通过 `jc pair` 管理。例如：

```powershell
jc p create --name devvm --ssh-host devvm
```

```sh
jc p join --server 192.168.56.1:59684 --fingerprint '粘贴证书指纹'
jc c .
```

命令名优先于同名本地目录。如果 Windows 当前目录下有 `pair`、`p`、`s` 或 `c` 文件夹，请使用 `jc ./目录名` 打开。

CLI 支持简体中文和英文，默认根据系统语言选择；可用 `--lang zh-CN` 或 `--lang en-US` 指定，也可设置环境变量 `JC_LANG`。例如 `jc --lang en-US s --help`。

使用 `jc --version` 或 `jc -v` 查看版本、提交号、构建时间和目标平台；反馈问题时可附上该输出。

## 请求超时

`jc client` 和 `jc pair join` 默认的整个 HTTP 请求超时为 **10 秒**，包含 TCP 连接、TLS 握手和等待服务端响应；TCP 连接和 TLS 握手各有 **3 秒**的时限，同时受请求总超时约束。

可用 `--timeout` 调整单次请求的总超时，必须带单位且大于 0，例如：

```sh
jc c . --timeout 5s
jc pair join --server 192.168.56.1:59684 --fingerprint '粘贴证书指纹' --timeout 15s
```

该参数不保存到配对配置，连接和握手时限仍各为 3 秒。计时从发送请求开始，不包含输入配对密钥的时间，也不控制 VS Code 后续的 SSH 连接。
