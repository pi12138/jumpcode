# 使用指南

[项目首页](../README.md) · [使用指南](usage.md) · [架构说明](architecture.md) · [开发指南](development.md) · [English](usage.en.md)

先按[项目首页](../README.md)的安装说明准备对应平台的可执行文件；以下步骤说明配对和日常使用。源码构建方法见[开发指南](development.md)。

## 使用前提

- Windows 已安装 VS Code 桌面版、Remote-SSH 扩展和 OpenSSH 客户端，且 `ssh` 可从终端执行。VS Code 需已注册 `vscode://` 协议。
- 打开 Linux 远程项目时，Windows 能通过 SSH 登录对应主机。可用 `ssh me@192.168.56.10` 验证；SSH config 别名（如 `devvm`）只是可选的简写。
- 从 Linux 终端发起打开请求时，Linux 能直接或通过隧道访问 Windows 的监听地址与端口。本机虚拟机可使用专用虚拟网络；公网云服务器可使用下文的 SSH 反向转发。

## 按使用场景查找

以下场景均使用现有命令，两端先完成[安装](../README.md#安装)。后续示例假设 `jc` 已加入 `PATH`，地址、用户名和目录均需替换为实际值。

| 使用场景 | 操作指南 |
| --- | --- |
| 在本机 Linux 虚拟机终端工作，用 Windows VS Code 打开目录 | [首次使用：从 Linux 虚拟机打开项目](#首次使用从-linux-虚拟机打开项目) |
| 在 Windows 直接打开本地目录或远程项目 | [从 Windows 打开项目](#从-windows-打开项目)，无需配对或启动服务端 |
| 在局域网中的独立 Linux 主机上工作 | [局域网中的独立 Linux 主机](#局域网中的独立-linux-主机) |
| SSH 登录云服务器后，从远端终端打开当前目录 | [通过 SSH 反向转发访问云服务器](#通过-ssh-反向转发访问云服务器) |
| Windows 和 Linux 位于不同网络，已使用 Tailscale | [通过 Tailscale 连接异地设备](#通过-tailscale-连接异地设备) |
| 多台 Linux 主机分别请求同一台 Windows | [多台 Linux 主机分别配对](#多台-linux-主机分别配对) |
| 切换 Wi-Fi、启停 VPN 后无法打开项目 | [网络变化后恢复连接](#网络变化后恢复连接) |
| 当前用户登录后自动启动，或日常启停与升级 | [计划任务管理](#计划任务管理)、[日常启停与查看日志](#日常启停与查看日志)、[升级与修改任务配置](#升级与修改任务配置) |

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

   也可改用[计划任务管理](#计划任务管理)，实现当前用户登录后自动启动；两种运行方式选一种即可，后续配对步骤相同。

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

通过 `jc c .` 或 Windows 上的 `jc 主机:目录` 打开远程项目时，`jc` 明确请求新窗口，避免替换当前窗口中的其他项目。同一 SSH 主机的同一目录已打开时，VS Code 会激活已有窗口。窗口识别由 VS Code 负责，`jc` 不维护窗口列表，也无需调整 VS Code 的全局窗口设置。

## 从 Windows 打开项目

```powershell
jc .
jc .\project
jc C:\Users\me\project
jc devvm:/home/me/project
jc me@192.168.56.10:~/project
```

不含主机前缀的参数会作为 Windows 本地目录打开，包括相对路径和盘符路径。远端目标只接受以 `/` 或 `~/` 开头的路径；`~/` 会通过 SSH 查询远端用户的主目录。主机部分可使用 Windows SSH config 别名或 `user@host`。如果在 Linux 上直接运行 `jc .`，程序会提示改用 `jc client .`。

Windows 通过系统注册的 `vscode://` 协议打开 VS Code：本地目录使用 `vscode://file/...`，远端目录使用 `vscode://vscode-remote/ssh-remote+<主机>/<目录>/?windowId=_blank`。程序不调用 `code.cmd`，也不修改 Electron 环境变量。VS Code 首次接收外部 URI 时可能要求确认；`jc` 只能确认 Windows 已接受启动请求，远程连接结果以 VS Code 窗口中的提示为准。

打开 VS Code 时，`jc` 使用一个无控制台的短暂子进程，避免新启动的 VS Code 把自身日志写进 `jc server` 所在终端。VS Code 自身的警告仍可能出现在其日志中；已运行的 VS Code 进程若原本连接到其他终端，此操作不会改变它的输出位置。

## 服务端运行与管理

服务端支持以下三种运行方式。手动后台与计划任务分别使用自己的状态、停止命令；切换前先停止原来的服务，避免监听端口冲突。

| 运行方式 | 启动 | 查看状态 | 停止 | 下次登录自动启动 |
| --- | --- | --- | --- | --- |
| 前台 | `jc s --listen auto` | 当前终端输出 | `Ctrl+C` | 否 |
| 手动后台 | `jc s --listen auto -d` | `jc s status` | `jc s stop` | 否 |
| 计划任务 | 先安装，再 `jc task start` | `jc task status` | `jc task stop` | 是 |

若不使用后台模式，可在 Windows 当前登录用户的终端前台启动服务端，保持该终端运行：

```powershell
jc server --listen 192.168.56.1:59684
```

按需将监听参数换为 `--listen auto`，或重复指定，例如 `--listen 192.168.56.1:59684 --listen 192.168.1.10:59684`。省略时等价于 `--listen auto`，使用端口 `59684` 监听所有已启用网卡上的私有 IPv4 地址；`--listen auto:59700` 可指定端口，也可与明确的监听地址组合。自动选择可能包含 VPN 和虚拟网卡地址，不包含回环、链路本地或公网地址。程序会显示实际监听列表；明确指定的地址在启动时必须全部绑定成功，自动候选地址绑定失败则记录日志并重试，其他地址继续服务。

也可以在当前登录用户会话中后台运行（`-d` 等价于 `--daemon`）：

```powershell
jc s --listen 192.168.56.1:59684 -d
jc s status
jc s stop
```

后台日志位于用户配置目录的 `jumpcode/server.log`，运行状态保存在同目录的 `daemon.json`。日志记录时间、PID、版本、监听地址、正常停止、异常退出，以及打开请求的成功或失败原因；配对密钥和项目路径不写入日志。日志追加写入，当前没有自动轮转。`jc s status` 和 `jc s stop` 管理后台服务，前台模式用 `Ctrl+C` 停止。退出登录或重启后，需要再次手动启动服务。

### 网络变化与自动监听

从 v0.2.0 起，`auto` 会订阅 Windows 的 IPv4 地址和网络接口变化通知，前台、手动后台、计划任务三种模式均支持。v0.1.1 只在启动时识别地址，地址变化后仍需手动重启。

切换 Wi-Fi、启停 VPN 或插拔网线后，服务收到系统通知，会合并约 500 毫秒内的重复事件，再重新识别候选地址：新增 IP 开始监听，消失 IP 移除监听，未变化的监听和已有连接保留。过程在原服务进程内完成，证书指纹和配对凭据保持不变。只有明确 IP、没有 `auto` 的配置继续使用固定地址。

启动时没有可用地址，或运行中暂时断网，服务都会保持运行、等待后续网络通知。枚举或绑定失败时，每 3 秒重试一次；相同的连续错误只记录一次，恢复后不再定时检查网卡。日志记录监听增删和失败原因，`jc s status`、`jc task status` 显示当前实际监听列表。

没有可用监听时，状态会提示正在等待网络恢复或绑定重试，仍可执行对应的停止命令。`jc task start` 继续最多等待 10 秒确认有监听；超时后进程会保持运行，网络恢复后自动继续服务。`jc s -d` 可以报告进程已启动并正在等待网络。

这项功能只调整服务端监听。Linux 客户端仍保存一个固定服务端地址：如果该地址变化，需要更新配对；若一直使用地址稳定的虚拟网卡，则无需随 Wi-Fi 或 VPN 的变化调整客户端。

### 旧版任务清理

`jc s startup` 和 `jc s start` 已移除，新版管理入口为 `jc task`。如果旧版曾安装过登录启动任务，请在替换程序前使用**旧版 jc** 执行：

```powershell
jc s startup uninstall
```

该命令会移除任务并停止任务启动的服务，保留配对凭据和证书。如果已经替换程序，请在 Windows 任务计划程序中先结束、再删除当前用户的 `Jumpcode Server <用户 SID>` 任务，并执行 `jc s stop` 停止旧版仍在运行的后台服务。升级本身不会更新已有任务；新版安装遇到同名任务会报错，请清理旧任务后重新安装。

### 计划任务管理

`jc task` 从 v0.2.0 起提供，升级旧版任务前先按上文清理。在 Windows 当前登录用户的终端执行，先将 `jc.exe` 放在固定安装目录，任务会保存它的绝对路径。如果之前使用 `jc s -d`，先执行 `jc s stop`；前台服务则先按 `Ctrl+C` 停止。然后首次安装并启动任务：

```powershell
jc task install --listen auto
jc task start
jc task status
```

`task` 可简写为 `t`，`status` 可简写为 `st`，例如 `jc t st`。只有安装命令接受 `--listen`，可重复指定，默认 `auto`。安装时保存配置，服务实际启动时再识别网卡地址。

| 命令 | 行为 |
| --- | --- |
| `jc task install` | 注册当前用户的登录自启任务，不立即启动；已有同名任务时拒绝覆盖。 |
| `jc task start` | 请求任务计划程序启动，最多等待 10 秒确认服务就绪；已经就绪则直接成功。 |
| `jc task stop` | 通过带令牌认证的本机控制接口请求正常退出，最多等待 10 秒；已停止则成功。 |
| `jc task status` | 显示安装情况、调度器状态、上次结果、服务就绪情况、PID、监听地址和日志位置。 |
| `jc task uninstall` | 先停止服务再删除任务；停止失败时保留任务并报错，凭据、证书和日志保留。 |

任务名为 `Jumpcode Server <当前用户 SID>`，可在 Windows“任务计划程序”中查看。任务使用当前用户的交互登录会话和普通权限，在该用户登录后启动；重复启动不会创建新实例，没有最长运行时间限制，切换到电池供电也不会自动停止。异常退出会以 1 分钟间隔重试，最多 3 次。

任务动作直接执行安装目录中的 `jc.exe --lang ... server --listen ... --task-run`。任务模式先打开日志，再调用 Windows `FreeConsole` 脱离控制台，随后在同一个 PID 中运行服务并提供停止接口。原控制台关闭后，服务继续运行；使用 `jc task stop` 正常停止。管理命令临时运行后退出，服务运行期间没有额外常驻 launcher 或监控进程。

任务模式的标准输入连接到空设备，标准输出、错误输出及 Go runtime 的 panic 输出写入 `server.log`。控制台关联在程序启动后解除，启动瞬间仍可能短暂闪窗，但服务运行不需要保持终端窗口。普通 CLI 命令和前台 `jc server` 保留终端交互。

任务服务运行状态保存在 `%APPDATA%\jumpcode\task.json`，手动 `jc s -d` 的状态保存在 `daemon.json`；分别使用 `jc task status/stop` 和 `jc s status/stop` 管理。两种模式默认使用相同端口，切换前请先停止另一种模式。日志统一追加到 `%APPDATA%\jumpcode\server.log`，包括管理请求和失败原因。

启动超时表示尚未确认就绪，任务可能仍在启动或自动重试，请查看 `jc task status` 和日志。正常停止失败时可在任务计划程序中检查任务，必要时手动结束。任务管理功能仅支持 Windows。

### 日常启停与查看日志

已安装任务后，日常使用 `jc task start` 和 `jc task status`。需要重启时，先停止，确认命令成功后再启动：

```powershell
jc task stop
jc task start
jc task status
```

`stop` 只停止本次运行，保留登录自启；下次登录仍会启动。要关闭登录自启，执行 `jc task uninstall`，它会先停止服务、再移除任务，保留配对凭据、证书和日志。以后可重新 `install`、`start`。

在 PowerShell 中查看最近 50 行日志；需要持续跟踪时，在末尾加 `-Wait`：

```powershell
Get-Content (Join-Path $env:APPDATA 'jumpcode\server.log') -Tail 50
```

`jc task status` 会显示任务状态、服务是否就绪和日志路径。调度器的“就绪（未运行）”表示任务可以被启动，服务当前尚未运行；应以“服务已就绪”和实际监听地址判断服务能否使用。

### 升级与修改任务配置

同一套 `jc task` 实现之间升级，安装路径和监听配置不变时，**无需重新安装任务**：

1. 用原版本执行 `jc task stop`，确认停止成功。
2. 用新版覆盖原路径的 `jc.exe`。不要删除 `%APPDATA%\jumpcode`，配对凭据和证书都保存在其中。
3. 执行 `jc task start`、`jc task status`，确认新服务就绪。

手动后台模式在替换前用 `jc s stop` 停止，替换后用 `jc s -d` 启动；前台模式先按 `Ctrl+C`。从旧版 `startup` 机制迁移时，先按[旧版任务清理](#旧版任务清理)操作。

修改监听地址需要先卸载再安装。以下示例仅监听指定网卡，地址请按实际情况替换：

```powershell
jc task uninstall
jc task install --listen 192.168.56.1:59684
jc task start
jc task status
```

每一步成功后再执行下一步。移动安装目录时，先用原位置的 `jc.exe task uninstall`，再移动程序，从新位置执行 `task install`、`task start`。安装参数会写入任务，之后的 `start` 不接受 `--listen`。若客户端保存的服务端地址也发生变化，需要在 Linux 重新执行 `jc pair join`。

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

## 常用场景

### 局域网中的独立 Linux 主机

Linux 不必是本机虚拟机，也可以是同一网络中的开发机或服务器。假设 Windows 地址为 `192.168.1.20`，Linux SSH 地址为 `me@192.168.1.30`：

1. 在 Windows 用 `ssh me@192.168.1.30` 确认 SSH 可用，退出 SSH 后启动 jc 服务。若已有服务，先按原运行模式停止；下例使用手动后台：

   ```powershell
   jc server --listen 192.168.1.20:59684 --daemon
   jc server status
   jc pair create --name lan-dev --ssh-host me@192.168.1.30
   ```

2. 在 Linux 使用输出的指纹和配对密钥：

   ```sh
   jc pair join --server 192.168.1.20:59684 --fingerprint '粘贴 Windows 输出的证书指纹'
   cd /home/me/project
   jc client .
   ```

Windows 需要能访问 Linux 的 SSH 端口，Linux 也需要能访问 Windows 的 `59684/TCP`；处在同一 Wi-Fi 并不保证可达，访客网络或客户端隔离可能阻止互访。按实际网络配置防火墙入站规则，jc 不会自动修改防火墙。Windows 地址经常变化时，可改用 `--listen auto`，客户端地址的更新方法见[网络变化后恢复连接](#网络变化后恢复连接)。

### 通过 SSH 反向转发访问云服务器

适合从 Windows 的 OpenSSH 客户端登录云服务器后，在远端终端执行 `jc client .`。云服务器无需直接连接 Windows 的公网地址；SSH 把回调请求带回发起该连接的 Windows。这里只转发 jc 的 HTTPS 请求，VS Code 仍通过自己的 Remote-SSH 连接读取项目。

#### 1. 配置 Windows 回环监听

`auto` 不包含 `127.0.0.1`，因此必须显式加入回环监听。按已有运行方式选择一种配置：

- 手动后台：已有手动后台服务先执行 `jc server stop`；前台服务先按 `Ctrl+C`；已有计划任务先停止任务。然后执行：

  ```powershell
  jc server --listen 127.0.0.1:59684 --daemon
  jc server status
  ```

- 计划任务：若已有任务，先执行 `jc task uninstall`，确认成功后重新安装；首次安装跳过卸载。若有手动服务，先停止它。然后执行：

  ```powershell
  jc task install --listen 127.0.0.1:59684
  jc task start
  jc task status
  ```

状态应包含 `127.0.0.1:59684`。如果还要服务局域网客户端，在上述启动或安装命令中再添加 `--listen auto`。卸载任务会保留配对、证书和日志；后续的 `jc task start` 不接受监听参数。

#### 2. 准备 SSH 目标并签发凭据

可以直接使用 `user@host`。如果需要固定保存转发参数，推荐在 Windows 的 `%USERPROFILE%\.ssh\config` 中使用两个别名。以下 `203.0.113.10` 是文档示例地址，请替换为真实云服务器地址，端口和用户名也按实际修改：

```sshconfig
Host cloud cloud-jc
    HostName 203.0.113.10
    User me
    Port 22

Host cloud-jc
    RemoteForward 127.0.0.1:59684 127.0.0.1:59684
    ExitOnForwardFailure yes
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

`cloud` 供 VS Code 使用，不配置反向转发；`cloud-jc` 供登录终端使用，建立回调通道。不要把 `RemoteForward` 放进 `Host *` 或 VS Code 使用的别名，否则 VS Code 建立额外 SSH 连接时可能重复占用云端端口。保活参数用于发现失效连接，不会自动重连。

在 Windows 先用 `ssh cloud` 确认能登录并接受正确的 SSH 主机指纹，退出后创建配对：

```powershell
jc pair create --name cloud --ssh-host cloud
```

保存输出的 jc 证书指纹和配对密钥。`--ssh-host` 指向普通的 `cloud` 别名；名称已存在时，可沿用仍持有的凭据，或按[配对与凭据](#配对与凭据)撤销后重新创建。

#### 3. 建立隧道，在 Linux 配对并打开项目

从 Windows 发起连接，并保持这个会话在线：

```powershell
ssh cloud-jc
```

不使用 SSH config 时，可用下列命令建立同样的转发，并在创建凭据时使用 `--ssh-host me@203.0.113.10`：

```powershell
ssh -o ExitOnForwardFailure=yes -R 127.0.0.1:59684:127.0.0.1:59684 me@203.0.113.10
```

进入云服务器后，在对应 Linux 用户下执行：

```sh
jc pair join --server 127.0.0.1:59684 --fingerprint '粘贴 Windows 输出的 jc 证书指纹'
cd /home/me/project
jc client .
```

按提示输入配对密钥。这里的 `127.0.0.1` 是**云服务器自身**：SSH 在云端回环地址接收请求，再将 HTTPS 原样转发给 Windows 的回环监听。jc 的证书指纹仍是 Windows 的指纹，配对认证继续生效。后续保持隧道可用，直接执行 `jc client .` 即可，无需每次配对。

#### 4. 日常使用与故障排查

- 云端 `sshd` 必须允许远程 TCP 转发，且没有其他策略禁止该监听；云端 `59684` 端口也不能已被占用。无需为回调端口开放云安全组的公网入站，也无需开启 `GatewayPorts`。
- 同一台云服务器只保持一个使用该端口的 `cloud-jc` 隧道。其他终端可用 `ssh cloud`；普通连接下的 `jc client .` 也能使用已建立的回环入口。
- SSH 会话断开后，转发随之关闭，重新执行 `ssh cloud-jc` 恢复。Windows 登录自启任务只启动 jc 服务，不会替你建立 SSH 隧道；从其他设备登录这台云服务器也不会自动建立回到 Windows 的通道。
- 云端可用 `ss -ltn 'sport = :59684'` 检查回环监听是否存在；它只能证明入口存在。`ExitOnForwardFailure` 也只检查转发建立，不保证 Windows 的 jc 已运行。请求失败时，继续核对 Windows 服务状态中的 `127.0.0.1:59684` 及对应日志。
- 配对成功但 VS Code 连不上时，在 Windows 单独检查 `ssh cloud` 和 Remote-SSH。回调入口 `127.0.0.1:59684` 与配对绑定的 SSH 目标 `cloud` 用途不同，不能互换。

转发及策略说明见 [OpenSSH `ssh -R`](https://man.openbsd.org/ssh#R)、[`sshd` 转发配置](https://man.openbsd.org/sshd_config#AllowTcpForwarding)和 [`ExitOnForwardFailure`](https://man.openbsd.org/ssh_config#ExitOnForwardFailure)。本指南依据现有实现与 OpenSSH 语义编写，尚未完成 Windows 与真实云服务器的完整端到端实测。

### 通过 Tailscale 连接异地设备

适合两端已安装 Tailscale、位于不同网络的情况。先确认两台设备加入可互访的网络，访问策略与主机防火墙允许 Linux 访问 Windows 的 jc 端口，以及 Windows 访问 Linux 的 SSH 端口。组网只提供连通性，SSH 与 jc 仍需各自运行。

假设 Windows 的 Tailscale IPv4 为 `100.101.102.103`，Linux 为 `100.104.105.106`。请用实际地址替换，并在 Windows 先验证 `ssh me@100.104.105.106`。停止已有 jc 服务后，手动后台示例为：

```powershell
jc server --listen 100.101.102.103:59684 --daemon
jc server status
jc pair create --name tailscale-dev --ssh-host me@100.104.105.106
```

在 Linux 执行：

```sh
jc pair join --server 100.101.102.103:59684 --fingerprint '粘贴 Windows 输出的证书指纹'
jc client /home/me/project
```

Tailscale 默认地址来自 `100.64.0.0/10`；当前 jc 的 `auto` 只选私有 IPv4，不包含这个范围，因此必须显式配置该地址。可以附加 `--listen auto` 继续服务其他网卡，但显式地址在启动时必须已存在且可绑定。若使用计划任务，把相同监听参数传给 `jc task install`；登录自启时也需要 Tailscale 网卡已就绪，不能依靠 `auto` 自动发现它。

当前 jc 的 `--listen` 和配对 `--server` 仅接受明确的 IPv4，不接受 IPv6 或域名；以上示例应填写 Tailscale IPv4。访问策略或网卡尚未就绪时，先恢复组网，再重新启动 jc。设备地址及连接方式见 [Tailscale 地址说明](https://tailscale.com/docs/concepts/tailscale-ip-addresses)和[设备连接指南](https://tailscale.com/docs/how-to/connect-to-devices)。本指南尚未完成双端 Tailscale 环境实测。

### 多台 Linux 主机分别配对

Windows 可以为不同 Linux 主机分别签发凭据。在 Windows 顺序执行以下命令，并分别保管每次输出的密钥：

```powershell
jc pair create --name devvm --ssh-host me@192.168.56.10
jc pair create --name lan-dev --ssh-host me@192.168.1.30
jc pair list
```

在每台 Linux 上，用**属于该主机的密钥**分别执行 `jc pair join`，服务端地址选择该主机可达的 Windows 监听。同一 Windows 用户配置下的配对共用证书指纹，密钥各不相同；服务端依据密钥选择绑定的 SSH 主机。不要把一份 `client.json` 复制给不同主机，否则它们会被当成同一配对，打开同一个 SSH 目标。

停用某台主机时，在 Windows 执行 `jc pair revoke --name lan-dev`，其他配对继续有效。创建和撤销请顺序执行，当前没有跨进程写入锁。

Linux 客户端配置按当前用户保存，每个用户当前只保留一份配对。再次成功执行 `pair join` 会替换该用户之前的客户端配置；这不会撤销 Windows 上的旧凭据。日常以完成配对的同一个 Linux 用户执行 `jc client`，不要额外使用 `sudo` 切换到另一份用户配置。

### 网络变化后恢复连接

如果只是 Wi-Fi 或 VPN 变化，但客户端仍能访问保存的 Windows 地址，继续执行 `jc client .` 即可。服务使用 `auto` 时会调整监听；仅配置固定 IP 时，需要按新地址修改监听并重启服务。

连接失败时，按以下顺序检查：

1. 在 Windows 用 `jc task status`（计划任务）或 `jc server status`（手动后台）查看实际监听；前台模式看终端输出。日志查看方法见[日常启停与查看日志](#日常启停与查看日志)。
2. 在 Linux 用 `jc pair status` 查看保存的地址。它只读本地记录，不能检测服务在线或凭据是否有效。
3. 使用 SSH 隧道时，先重新建立隧道；只要入口仍是 `127.0.0.1:59684`，无需因为 Windows 物理 IP 变化而重新配对。
4. 直接连接时，如果旧地址已不可达，选择新的可达监听，并在仍持有原配对密钥时执行：

   ```sh
   jc pair join --server 192.168.1.21:59684 --fingerprint '原 Windows 证书指纹'
   jc client .
   ```

   `192.168.1.21` 是新地址示例；按提示输入原密钥。只更换地址、未更换 Windows 证书时，指纹不变。

如果没有另行保留原密钥，`pair list/status` 无法显示它；Windows 仅保存密钥哈希。可在 Windows 按原配对名称先 `pair revoke`，再用相同 SSH 目标 `pair create`，然后在 Linux 使用新密钥配对。无需删除整个配置目录，也无需为了地址变化重新生成证书。服务端自动监听不会把新地址推送给客户端。

## 常见问题

| 现象 | 检查方法 |
| --- | --- |
| `auto` 显示暂无监听或部分地址不可用 | 检查 Windows 是否有已启用的私有 IPv4 网卡，以及日志中的绑定错误。无可用地址时等待网络通知，绑定失败会重试；明确指定的地址在启动时必须可用。若提示订阅网络通知失败，服务会报错退出。 |
| `task start` 提示未安装，或 `task install` 提示已存在 | 先用 `jc task status` 查看安装情况；未安装时执行 `install`。已有任务需要修改配置时先 `uninstall`；旧版任务按上文清理。 |
| 任务启动超时或启动后退出 | 查看 `jc task status` 的调度器状态、上次结果和日志；检查是否有手动后台或前台服务占用端口。超时后任务可能仍在启动或重试。 |
| 任务启动后终端窗口一直存在，关闭后服务也停止 | 核对任务动作和 `jc --version`，确认运行的是当前版本且已按迁移说明清理旧任务。当前任务模式会脱离控制台，日志应出现“已脱离控制台”；短暂闪窗仍可能发生。 |
| Linux 配对时报“连接失败” | 计划任务模式执行 `jc task status`，手动后台模式执行 `jc s status`，前台模式查看终端；确认选择的是实际监听且虚拟机可达的地址，再检查 Windows 防火墙。`auto` 选出的其他网卡地址不一定对这台虚拟机可达。 |
| 指纹不匹配或返回 401 | 核对 Windows 配对输出中的证书指纹和密钥，不要把两个值混用。如果密钥丢失，在 Windows 用 `jc pair revoke --name devvm` 撤销，再创建新配对并在 Linux 重新配对。 |
| Linux 显示请求成功，但 VS Code 未连上项目 | 在 Windows 再次运行 `ssh devvm`，检查 Remote-SSH 扩展、`vscode://` 协议注册和 VS Code 窗口中的报错。请求成功只表示 Windows 接受了打开请求。 |

## 短命令与语言

`server` 可简写为 `s`，`client` 可简写为 `c`，顶层 `pair` 可简写为 `p`，`task` 可简写为 `t`，撤销 `revoke` 可简写为 `r`，`status` 可简写为 `st`，例如 `jc s st`、`jc t st`。配对凭据统一通过 `jc pair` 管理。例如：

```powershell
jc p create --name devvm --ssh-host devvm
```

```sh
jc p join --server 192.168.56.1:59684 --fingerprint '粘贴证书指纹'
jc c .
```

命令名优先于同名本地目录。如果 Windows 当前目录下有 `pair`、`task`、`p`、`t`、`s` 或 `c` 文件夹，请使用 `jc ./目录名` 打开。

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
