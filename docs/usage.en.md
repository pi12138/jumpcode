# Usage guide

[Project home](../README.en.md) · [Usage](usage.en.md) · [Architecture](architecture.en.md) · [Development](development.en.md) · [简体中文](usage.md)

Install the executable for each platform using the [project home](../README.en.md), then follow these steps for pairing and daily use. For source builds, see the [development guide](development.en.md).

## Requirements

- Windows: VS Code desktop, Remote-SSH extension, an OpenSSH client available as `ssh`, and the registered `vscode://` protocol handler.
- To open a remote Linux project, Windows can SSH into the VM. Test with `ssh me@192.168.56.10`; an SSH config alias such as `devvm` is optional shorthand.
- For Linux-initiated opens, the VM can reach the chosen Windows listen address and port. With a local VMware VM, a host-only network address can work.

## First use: open a Linux VM project

These examples use `192.168.56.1` for a Windows address reachable from the VM and `devvm` for the VM's SSH alias. Replace the addresses, user, and folder with yours. Put `jc.exe` in a stable local folder, then enter that folder in PowerShell as the signed-in user. The examples therefore use `.\jc.exe`; use `jc` if it is on `PATH`.

1. Check that Windows can SSH into the VM. You can define the alias in `%USERPROFILE%\.ssh\config`:

   ```sshconfig
   Host devvm
       HostName 192.168.56.10
       User me
   ```

   ```powershell
   ssh devvm
   ```

   SSH config is optional: you can instead run `ssh me@192.168.56.10` and use `--ssh-host me@192.168.56.10` below. Install VS Code Remote-SSH and confirm your chosen SSH host works before continuing. Exit the SSH session.

2. Start the Windows server in the background and inspect the bound addresses:

   ```powershell
   .\jc.exe s --listen auto -d
   .\jc.exe s status
   ```

   `auto` listens on every private IPv4 address of an active interface, using port `59684`. Choose an address the VM can reach from the status output; the remaining examples use `192.168.56.1:59684`. To limit exposure to one interface, use `--listen 192.168.56.1:59684` instead.

3. Pair the VM under the same Windows user account:

   ```powershell
   .\jc.exe pair create --name devvm --ssh-host devvm
   ```

   Keep the printed SHA-256 certificate fingerprint and the one-time pairing token. `--ssh-host` must match the Windows SSH host verified in step 1.

4. On the Linux VM, install the Linux executable from the current folder for this user, then pair using that fingerprint:

   ```sh
   mkdir -p "$HOME/.local/bin"
   install -m 755 ./jc "$HOME/.local/bin/jc"
   "$HOME/.local/bin/jc" pair join --server 192.168.56.1:59684 --fingerprint 'paste the certificate fingerprint printed on Windows'
   ```

   Paste the pairing token from step 3 when prompted and press Enter. It is not echoed. The pair command connects to the Windows server immediately to check the address, fingerprint, and token.

5. Open a project from Linux:

   ```sh
   cd /home/me/project
   "$HOME/.local/bin/jc" c .
   ```

   VS Code on Windows should attempt to connect to `devvm`; check the VS Code window for the final connection result. Use the same command later under the same Linux user. If `$HOME/.local/bin` is on `PATH`, use `jc c .`. The client remembers one Windows server address. Running `jc .` directly on Linux does not request a Windows launch.

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

Jumpcode uses a short-lived process without a console to open VS Code, so a newly launched VS Code process does not write its logs to the `jc server` terminal. VS Code may still record its own warnings. This does not change where an already running VS Code process writes its output.

## Server operation and management

To run the server in the foreground, start it in a terminal belonging to the signed-in Windows user and keep that terminal open:

```powershell
jc server --listen 192.168.56.1:59684
```

Replace the listen option with `--listen auto`, or repeat it, for example `--listen 192.168.56.1:59684 --listen 192.168.1.10:59684`. With no `--listen`, the server uses `auto`: it binds every private IPv4 address on an active interface, using port `59684`. Use `--listen auto:59700` for another port; explicit addresses can be combined with `auto`. Automatic selection may include VPN and virtual adapters, but excludes loopback, link-local, and public addresses. Startup prints the bound addresses. If any bind fails, the server fails to start rather than listening on only some addresses. Restart the server after an interface address changes.

To run it in the background of the same Windows user session:

```powershell
jc s --listen 192.168.56.1:59684 -d
jc s status
jc s stop
```

`-d` means `--daemon`. The log is `jumpcode/server.log` under the user's config folder; `daemon.json` in the same folder stores runtime state. Logs record timestamps, PID, version, listen addresses, normal stops, failures, and accepted or failed open requests with failure reasons. Pairing tokens and project paths are excluded. Logs are appended and do not rotate automatically yet. `jc s status` and `jc s stop` manage the background server; use `Ctrl+C` for foreground mode. Start the server manually again after signing out or restarting Windows.

| Action | Command |
| --- | --- |
| Run in the foreground | `jc s --listen auto` |
| Run in the background | `jc s --listen auto -d` |
| Check the background server | `jc s status` |
| Stop the background server | `jc s stop` |

### Legacy task cleanup

The current source version removes `jc s startup`, `jc s start`, and Task Scheduler support. If an older version installed a sign-in task, run this command with **the old jc version** before replacing the executable:

```powershell
jc s startup uninstall
```

This removes the task and stops its server while preserving pairing credentials and certificates. If you already replaced the executable, delete the current user's `Jumpcode Server <user SID>` task in Windows Task Scheduler and run `jc s stop` to stop a background server that is still running. Upgrading alone does not remove existing Windows tasks. Use `jc s -d` to start the server manually afterward.

## Pairing and credentials

In another Windows terminal, create a VM pairing. The command prints the certificate fingerprint and a token shown only once:

```powershell
jc pair create --name devvm --ssh-host devvm
jc pair list
```

The server can be stopped when running `pair create`. For `pair join`, the Windows server must be running and reachable from Linux. On the Linux VM, enter the printed fingerprint and paste the pairing token when prompted:

```sh
jc pair join --server 192.168.56.1:59684 --fingerprint 'paste the certificate fingerprint'
jc pair list
jc pair status
jc client .
```

You can also open another folder with `jc client /home/me/project`. The server uses the SSH host bound to the pairing; clients cannot choose another host.

The server may listen on multiple IPs, but each Linux client stores one server address. Pair with an address reachable from that VM, and pair again if that address changes.

`jc pair list` works on both platforms: Windows shows issued pairing names and bound SSH hosts, while Linux shows the locally saved server address and certificate fingerprint. Tokens and token hashes are never displayed. An unpaired machine shows "No pairings". On Linux, `jc pair status` shows the same locally saved information. These commands do not contact the server, so they do not verify availability or revocation.

Run `pair create/revoke` on the Windows server and `pair join/status` on the Linux client. Using the wrong platform reports where to run the specific command. Revoke a pairing on Windows:

```powershell
jc pair revoke --name devvm
```

Revocation takes effect immediately because the server reads current pairing settings for each request. Windows pairings are stored in `jumpcode/server.json` under the user's config folder and contain only token hashes. The Linux pairing is stored in `jumpcode/client.json`, contains the token, and is readable and writable only by the current user. `jc pair revoke` does not delete the saved Linux record, but its token can no longer access the Windows server. Do not share pairing tokens or the server private key. If the VM cannot connect, check the Windows firewall and listen address; Jumpcode does not change firewall settings.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| `--listen auto` fails to start | Check that Windows has an active interface with a private IPv4 address and read the bind error. Use an explicit `--listen IP:port` if needed. One failed bind prevents startup on all addresses. |
| Pairing from Linux reports a connection failure | Run `jc s status` on Windows and choose a bound address reachable from the VM. Check the Windows firewall for that port. Other addresses selected by `auto` may be unreachable from this VM. |
| Fingerprint mismatch or HTTP 401 | Check the fingerprint and token printed during Windows pairing. If the token was lost, revoke with `jc pair revoke --name devvm`, then pair again on Windows and Linux. |
| The request succeeds but VS Code does not connect | Retry `ssh devvm` on Windows and check Remote-SSH, the registered `vscode://` handler, and the error in the VS Code window. A successful request only confirms that Windows accepted the launch. |

Read the background log from Windows PowerShell:

```powershell
Get-Content (Join-Path $env:APPDATA 'jumpcode\server.log') -Tail 50
```

## Short commands and language

`server` has alias `s`, `client` has alias `c`, top-level `pair` has alias `p`, `revoke` has alias `r`, and server background status has alias `s st`. All pairing credentials are managed through `jc pair`. For example:

```powershell
jc p create --name devvm --ssh-host devvm
```

```sh
jc p join --server 192.168.56.1:59684 --fingerprint 'paste the certificate fingerprint'
jc c .
```

Command names take precedence over local folders with the same name. On Windows, use `jc ./foldername` to open a folder named `pair`, `p`, `s`, or `c`.

The CLI supports Simplified Chinese and English. It chooses a default from the system language. Override with `--lang zh-CN`, `--lang en-US`, or the `JC_LANG` environment variable. Example: `jc --lang en-US s --help`.

Run `jc --version` or `jc -v` to check the version, commit, build time, and target platform. Include this output when reporting a problem.

## Request timeouts

`jc client` and `jc pair join` default to a **10-second** timeout for the entire HTTP request, including the TCP connection, TLS handshake, and server response. The TCP connection and TLS handshake each have a **3-second** limit, also bounded by the total request timeout.

Use `--timeout` to change the total timeout for one command. The value must include a unit and be greater than zero:

```sh
jc c . --timeout 5s
jc pair join --server 192.168.56.1:59684 --fingerprint 'paste the certificate fingerprint' --timeout 15s
```

This option is not saved in pairing settings. Connection and handshake limits remain 3 seconds each. Timing starts when the request is sent, excludes the time spent entering a pairing token, and does not control subsequent SSH connections made by VS Code.
