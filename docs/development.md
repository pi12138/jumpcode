# 开发指南

[项目首页](../README.md) · [使用指南](usage.md) · [架构说明](architecture.md) · [开发指南](development.md) · [English](development.en.md)

构建和验证命令均从仓库根目录执行。

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
这里使用 `-buildvcs=false`，使缺少 Git 元数据的源码目录也能构建。

## 项目结构

命令层采用 [Cobra 官方指南](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md)中的常见布局：根目录 `main.go` 调用 `cmd.Execute()`，命令定义按子命令拆分到 `cmd/`，业务实现放在 `internal/`。

```text
main.go              程序入口，仅调用 cmd.Execute()
docs/                使用、架构和开发文档
scripts/release/     双平台发布包、版本清单和 SHA-256 校验文件生成工具
.github/workflows/  Windows/Linux 持续集成和标签发布流程
cmd/
  root.go            根命令、语言初始化与统一错误处理
  client.go          客户端打开命令
  server*.go         服务和后台管理子命令
  task*.go           计划任务管理的五个子命令
  pair*.go           配对管理子命令
  token.go           配对密钥的终端输入
internal/
  client/            Linux 客户端配对验证与打开请求
  server/            HTTPS 服务、多 IP 监听、后台进程与原生计划任务管理
  pairing/           配对凭据的创建、SSH 主机绑定与撤销
  config/            用户配置、证书与私钥的持久化
  protocol/          共享请求结构、地址校验、密钥哈希与 TLS 指纹校验
  remote/            本地和 SSH 目录解析、VS Code URI 与 Windows 打开操作
  i18n/              语言检测、提示和错误翻译
  version/           构建时注入的版本号、提交号与构建时间
```

`cmd/` 负责命令注册、参数解析和终端交互，并调用各功能包。客户端和服务端通过 `protocol` 共享通信规则，通过 `config` 访问配置。配对凭据管理独立于服务进程，`pair create` 可在服务未启动时执行；`pair join` 会连接服务端验证凭据。各模块的测试放在对应目录中，Windows 专用实现使用 `_windows.go` 文件隔离。

## 验证

```sh
go test -buildvcs=false ./...
go test -race -buildvcs=false ./...
go vet -buildvcs=false ./...
```

`internal/server/task*.go` 实现任务生命周期、XML 定义、Windows COM 调用及任务进程脱离控制台，复用现有 `golang.org/x/sys/windows`，没有新增依赖。Linux 测试使用模拟调度器验证就绪等待、正常停止、错误传播和状态隔离；Windows 测试额外调用原生调度器验证 Unicode XML（仅验证，不注册任务）。控制台测试在隔离子进程中创建真实控制台，验证任务模式脱离后 PID 不变、服务仍然可用、标准输出和 panic 写入日志，避免修改测试主进程的控制台和标准流。跨平台编译不能验证 Windows 桌面的窗口表现，需使用实际产物测试 `jc task` 的安装、启动、停止、登录触发及瞬间闪窗情况。

句柄生命周期回归测试在同样的隔离子进程中模拟旧标准流句柄释放，并让事件对象复用这些编号。强制 GC 后，复用句柄及服务响应必须仍有效。任务进程会保留脱离控制台前的标准流对象，避免其 finalizer 误关被 Windows 复用的句柄。

`network_windows.go` 复用现有依赖中的 `NotifyUnicastIpAddressChange`、`NotifyIpInterfaceChange` 和 `CancelMibChangeNotify2`。`listen_dynamic.go` 串行处理通知、监听增删和失败重试。测试使用可控地址快照、事件通道和真实回环 HTTPS 验证网络切换、连接保留、断网恢复、绑定重试、空监听状态及停止行为；不会修改测试机器的网卡或 VPN 配置。实际 Windows 通知仍需在桌面环境切换 Wi-Fi、启停 VPN 验证，观察 PID、`jc task status` / `jc s status` 和 `server.log` 的监听变化。

## 版本与发布打包

`jc --version`（或 `jc -v`）显示版本号、提交号、UTC 构建时间和目标平台。普通 `go build` 的版本为 `dev`，提交号和构建时间为 `unknown`；发布工具会通过 `-ldflags` 注入实际信息，后台日志也记录这些信息。

发布工具仅依赖 Go 标准库和 Git，生成 Windows amd64 ZIP、Linux amd64 tar.gz、`release.json` 和 `SHA256SUMS`。Windows ZIP 的可执行文件为 `jc.exe`，Linux 包的可执行文件为 `jc`。两种压缩包都包含双语指南和对应版本的发布说明；Linux 可执行文件保留执行权限。本地配置、密钥和被忽略的验证草稿不会打包。

开发过程中可生成测试快照：

```sh
go run ./scripts/release --version v0.2.1 --snapshot
```

输出目录为 `dist/v0.2.1-snapshot/`。正式版本号追加 `-snapshot`，已有预发布后缀的版本号追加 `.snapshot`；存在未提交修改时提交号带 `-dirty`，清单记录 `snapshot` 和 `dirty` 状态。工具拒绝覆盖已有输出目录，构建失败时不会留下完整发布目录。

正式打包前，先完成对应版本的 `docs/releases/<版本>.md` 和 `.en.md`，提交所有源码修改，确保工作区干净，再创建指向当前提交的版本标签：

```sh
git tag -a v0.2.1 -m '发布 v0.2.1'
go run ./scripts/release --version v0.2.1
```

正式产物位于 `dist/v0.2.1/`；标签与当前提交不一致或工作区有改动时会拒绝打包。校验文件覆盖两个压缩包和版本清单。在 Linux 中验证：

```sh
cd dist/v0.2.1
sha256sum -c SHA256SUMS
```

打包工具只生成本地产物。稳定版发布前需使用候选产物完成 Windows 桌面会话中的打开和后台启停验收；测试版需在发布说明中明确实际验收范围。

## GitHub 发布流程

仓库使用 [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository) 分发版本，工作流位于 `.github/workflows/`。

提交并推送 `main` 后，CI 会分别在 Linux 和 Windows 运行格式检查、`go vet`、测试和构建；Linux 额外启用竞态检查。Unix 文件权限检查仅在支持该权限模型的平台执行，协议认证和配对撤销测试保留 Windows 覆盖。

推送版本标签触发发布：

```sh
git push origin main
git push origin refs/tags/v0.2.1
```

Release 工作流先对标签对应的源码执行两平台 CI，再生成并校验发布包。上传全部附件后才公开草稿；带 `-beta` 等预发布后缀的版本会标记为 Pre-release。四个附件为 Windows ZIP、Linux tar.gz、`release.json` 和 `SHA256SUMS`。

上传失败时可重新运行对应工作流，继续处理草稿；已有公开 Release 会拒绝覆盖。也可在 Actions 中手动运行 Release，输入一个已有版本标签。新的版本需先更新发布说明和 README 中的版本下载链接。
