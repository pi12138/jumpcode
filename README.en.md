# Jumpcode (`jc`)

[简体中文](README.md)

Jumpcode opens local Windows folders and Linux VM projects in VS Code. From a Linux VM terminal, it can ask VS Code on Windows to open the current folder. Remote folders use VS Code Remote-SSH and remain on the VM. VM-to-Windows requests use LAN HTTPS without a third-party relay.

## Requirements

- Windows: VS Code desktop, Remote-SSH extension, an OpenSSH client available as `ssh`, and the registered `vscode://` protocol handler.
- Windows can SSH into the VM. Test `ssh devvm` first, where `devvm` is an alias in the Windows user's SSH config.
- The VM can reach the chosen Windows LAN address and port. With a local VMware VM, a host-only network address can work.

## Build

Go 1.24 or newer is required. Build on the target platform:

```sh
go build -buildvcs=false -o jc .       # Linux
go build -buildvcs=false -o jc.exe .   # Windows
```

Or cross-compile for amd64 from Linux:

```sh
GOOS=windows GOARCH=amd64 go build -buildvcs=false -o jc.exe .
GOOS=linux GOARCH=amd64 go build -buildvcs=false -o jc .
```

Put the binary on `PATH`. The first build may download dependencies from `go.mod`. The `-buildvcs=false` option is used because this workspace lacks complete Git metadata; omit it in a normal Git checkout.

## Open from Windows

```powershell
jc .
jc .\project
jc C:\Users\me\project
jc devvm:/home/me/project
jc me@192.168.56.10:~/project
```

Arguments without a host prefix are local folders. Remote paths must start with `/` or `~/`; the latter is resolved through SSH. Hosts can be Windows SSH config aliases or `user@host`. On Linux, use `jc client .` or `jc c .` instead of `jc .`.

Jumpcode opens VS Code through its registered `vscode://` protocol handler. It uses `vscode://file/...` for local folders and `vscode://vscode-remote/ssh-remote+<host>/<folder>` for remote folders. It does not invoke `code.cmd`. VS Code may ask for confirmation when it receives an external URI. Jumpcode can confirm only that Windows accepted the launch request; check the VS Code window for the remote connection result.

## Open from a Linux VM

In a terminal belonging to the signed-in Windows user, start the server in the foreground:

```powershell
jc server --listen 192.168.56.1:59684
```

To run it in the background of the same Windows user session:

```powershell
jc s --listen 192.168.56.1:59684 -d
jc s status
jc s stop
```

`-d` means `--daemon`. The log is `jumpcode/server.log` under the user's config folder; `daemon.json` in the same folder stores runtime state. Background mode does not automatically restart after login. For automatic startup, configure Windows Task Scheduler to run it as the signed-in user. `status` and `stop` manage background mode only; use `Ctrl+C` for foreground mode.

In another Windows terminal, create a VM pairing. The command prints the certificate fingerprint and a token shown only once:

```powershell
jc server pair --name devvm --ssh-host devvm
```

On the Linux VM, enter the printed fingerprint and paste the pairing token when prompted:

```sh
jc client pair --server 192.168.56.1:59684 --fingerprint <fingerprint>
jc client .
```

You can also open another folder with `jc client /home/me/project`. The server uses the SSH host bound to the pairing; clients cannot choose another host.

Revoke a pairing on Windows:

```powershell
jc server revoke --name devvm
```

Revocation takes effect immediately because the server reads current pairing settings for each request. Settings are stored in the user's `jumpcode` config folder. The Linux client config is readable and writable only by the current user. Do not share pairing tokens or the server private key. If the VM cannot connect, check the Windows firewall and listen address; Jumpcode does not change firewall settings.

## Short commands and language

`server` has alias `s`, `client` has alias `c`, `pair` has alias `p`, `revoke` has alias `r`, and `status` has alias `st`. For example:

```powershell
jc s p --name devvm --ssh-host devvm
```

```sh
jc c p --server 192.168.56.1:59684 --fingerprint <fingerprint>
jc c .
```

Command names take precedence over local folders with the same name. On Windows, use `jc ./s` or `jc ./c` to open such a folder.

The CLI supports Simplified Chinese and English. It chooses a default from the system language. Override with `--lang zh-CN`, `--lang en-US`, or the `JC_LANG` environment variable. Example: `jc --lang en-US s --help`.

## Development checks

```sh
go test -buildvcs=false ./...
go test -race -buildvcs=false ./...
```
