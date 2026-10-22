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

   You can also use [Task management](#task-management) to start automatically when the current user signs in. Choose one mode; the remaining pairing steps are the same.

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

The server supports three modes. Manual background servers and scheduled servers have separate status and stop commands. Stop the previous mode before switching to avoid port conflicts.

| Mode | Start | Status | Stop | Starts at next sign-in |
| --- | --- | --- | --- | --- |
| Foreground | `jc s --listen auto` | Terminal output | `Ctrl+C` | No |
| Manual background | `jc s --listen auto -d` | `jc s status` | `jc s stop` | No |
| Scheduled task | Install first, then `jc task start` | `jc task status` | `jc task stop` | Yes |

To run the server in the foreground, start it in a terminal belonging to the signed-in Windows user and keep that terminal open:

```powershell
jc server --listen 192.168.56.1:59684
```

Replace the listen option with `--listen auto`, or repeat it, for example `--listen 192.168.56.1:59684 --listen 192.168.1.10:59684`. With no `--listen`, the server uses `auto`: it binds every private IPv4 address on an active interface, using port `59684`. Use `--listen auto:59700` for another port; explicit addresses can be combined with `auto`. Automatic selection may include VPN and virtual adapters, but excludes loopback, link-local, and public addresses. The server prints the actual listeners. All explicitly configured addresses must bind at startup. Failed automatic candidates are logged and retried while other listeners remain available.

To run it in the background of the same Windows user session:

```powershell
jc s --listen 192.168.56.1:59684 -d
jc s status
jc s stop
```

`-d` means `--daemon`. The log is `jumpcode/server.log` under the user's config folder; `daemon.json` in the same folder stores runtime state. Logs record timestamps, PID, version, listen addresses, normal stops, failures, and accepted or failed open requests with failure reasons. Pairing tokens and project paths are excluded. Logs are appended and do not rotate automatically yet. `jc s status` and `jc s stop` manage the background server; use `Ctrl+C` for foreground mode. Start the server manually again after signing out or restarting Windows.

### Network changes and automatic listeners

Starting with v0.2.0, `auto` subscribes to Windows IPv4 address and interface change notifications in foreground, manual background, and task modes. The v0.1.1 release discovers addresses only at startup and still requires a restart after address changes.

After switching Wi-Fi, starting or stopping a VPN, or connecting or disconnecting Ethernet, the server receives a system notification. It combines repeated events over about 500 milliseconds, discovers candidates again, adds new listeners, and removes listeners for addresses that disappeared. Unchanged listeners and existing connections stay in place. The process, certificate fingerprint, and pairing credentials stay the same. Configurations containing only explicit IPs continue to use fixed addresses.

If no addresses are available at startup or the network is temporarily lost, the server stays running and waits for network notifications. Enumeration or bind failures trigger retries every three seconds. Identical consecutive errors are logged once; periodic checks stop after recovery. Logs record listener changes and failures. `jc s status` and `jc task status` show the actual listener list.

With no listeners, status reports that the server is waiting for the network or a bind retry, and the corresponding stop command still works. `jc task start` waits up to 10 seconds for a listener; after a timeout the process stays running and resumes service when the network recovers. `jc s -d` can report that the process has started and is waiting for the network.

This feature adjusts server listeners. Each Linux client still stores one fixed server address. Pair again if that address changes. A client using a stable virtual adapter address needs no changes when Wi-Fi or VPN addresses change.

### Legacy task cleanup

`jc s startup` and `jc s start` have been removed; use `jc task` for the new interface. If an older version installed a sign-in task, run this command with **the old jc version** before replacing the executable:

```powershell
jc s startup uninstall
```

This removes the task and stops its server while preserving pairing credentials and certificates. If you already replaced the executable, end and then delete the current user's `Jumpcode Server <user SID>` task in Windows Task Scheduler, and run `jc s stop` to stop any remaining background server from the older version. Upgrading does not update existing tasks. The new installer rejects an existing task with the same name; remove the old task first.

### Task management

`jc task` is available starting with v0.2.0. Follow the cleanup instructions above before migrating an old task. Run these commands as the signed-in Windows user. Put `jc.exe` in a permanent installation folder first: the task saves its absolute path. If you used `jc s -d`, run `jc s stop` first; stop a foreground server with `Ctrl+C`. Then install and start the task for the first time:

```powershell
jc task install --listen auto
jc task start
jc task status
```

Aliases are `t` for `task` and `st` for `status`, for example `jc t st`. Only `install` accepts `--listen`, which is repeatable and defaults to `auto`. Installation saves the configuration; interfaces are discovered when the server actually starts.

| Command | Behavior |
| --- | --- |
| `jc task install` | Register a sign-in task for the current user without starting it; reject an existing task with the same name. |
| `jc task start` | Ask Task Scheduler to start the server and wait up to 10 seconds for readiness; succeed immediately if already ready. |
| `jc task stop` | Request graceful shutdown through the authenticated local control endpoint and wait up to 10 seconds; succeed if already stopped. |
| `jc task status` | Show installation, scheduler state, last result, server readiness, PID, listeners, and log location. |
| `jc task uninstall` | Stop the server before deleting the task; keep the task on stop failure. Retain credentials, certificates, and logs. |

The task is named `Jumpcode Server <current user SID>` and is visible in Windows Task Scheduler. It uses the current user's interactive sign-in session with ordinary privileges. Repeated starts do not create another instance. Execution has no time limit and continues on battery power. Failed runs are retried up to three times at one-minute intervals.

The action directly runs the installed `jc.exe --lang ... server --listen ... --task-run`. Task mode opens the log, calls Windows `FreeConsole` to detach from the console, and then runs the server and stop endpoint in the same PID. The server continues running after its former console closes; use `jc task stop` for graceful shutdown. Management commands exit after use; there is no additional resident launcher or monitor process.

Task mode connects standard input to the null device and sends standard output, standard error, and Go runtime panic output to `server.log`. Detachment happens after process creation, so a window may still flash briefly during startup. The running server does not require an open terminal window. Ordinary CLI commands and foreground `jc server` retain terminal interaction.

Task runtime state is `%APPDATA%\jumpcode\task.json`; manual `jc s -d` state is `daemon.json`. Manage them with `jc task status/stop` and `jc s status/stop` respectively. Both default to the same port, so stop one mode before switching to the other. Management requests and failures append to the shared `%APPDATA%\jumpcode\server.log`.

A start timeout means readiness was not confirmed; the task may still be starting or retrying. Inspect `jc task status` and the log. If graceful stopping fails, inspect the task in Task Scheduler and end it manually if necessary. Task management is Windows only.

### Daily operation and logs

After installation, use `jc task start` and `jc task status`. To restart, stop the server and confirm success before starting it again:

```powershell
jc task stop
jc task start
jc task status
```

`stop` only stops the current run; the server will still start at the next sign-in. To disable sign-in startup, run `jc task uninstall`. It stops the server and removes the task while retaining pairing credentials, certificates, and logs. You can run `install` and `start` again later.

Read the latest 50 log lines in PowerShell. Append `-Wait` to follow new entries:

```powershell
Get-Content (Join-Path $env:APPDATA 'jumpcode\server.log') -Tail 50
```

`jc task status` shows task state, server readiness, and the log path. The scheduler state "Ready (stopped)" means the task can be started, but its server is currently stopped. Check "Server ready" and the bound addresses to confirm the server is available.

### Upgrades and task configuration

When upgrading between versions using the same `jc task` mechanism, **no task reinstall is needed** if the installation path and listeners stay the same:

1. Use the old executable to run `jc task stop` and confirm success.
2. Replace `jc.exe` at its installed path with the new version. Keep `%APPDATA%\jumpcode`, which holds pairing credentials and certificates.
3. Run `jc task start` and `jc task status` to confirm readiness.

For manual background mode, stop with `jc s stop` before replacing the executable, then start with `jc s -d`. For foreground mode, stop with `Ctrl+C`. To migrate from the old `startup` mechanism, follow [Legacy task cleanup](#legacy-task-cleanup) first.

To change listeners, uninstall and reinstall the task. This example listens on one interface; replace the address with yours:

```powershell
jc task uninstall
jc task install --listen 192.168.56.1:59684
jc task start
jc task status
```

Confirm each command succeeds before continuing. To move the installation, run `jc.exe task uninstall` from the original location first, then move the executable and run `task install` and `task start` from the new location. Installation saves the arguments; later `start` commands do not accept `--listen`. If the server address saved by the client changes, run `jc pair join` again on Linux.

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
| `auto` shows no listeners or some addresses are unavailable | Check for an active interface with a private IPv4 address and read the bind errors in the log. The server waits for notifications when no addresses exist and retries failed binds. Explicit addresses must be available at startup. Failure to subscribe to network notifications causes startup to fail. |
| `task start` says the task is missing, or `task install` says it exists | Check installation with `jc task status`. Run `install` if missing. To change an existing task's configuration, run `uninstall` first. Use the legacy cleanup instructions for old tasks. |
| Task startup times out or the server exits | Check scheduler state, last result, and the log using `jc task status`. Check whether a manual background or foreground server owns the port. A timed-out task may still be starting or retrying. |
| A terminal stays open after task startup, and closing it stops the server | Check the task action and `jc --version`. Confirm the task uses the current version and old tasks were removed as described above. Current task mode detaches from the console and logs "已脱离控制台". A brief startup flash is still possible. |
| Pairing from Linux reports a connection failure | Use `jc task status` for task mode, `jc s status` for manual background mode, or terminal output for foreground mode. Choose a bound address reachable from the VM and check the Windows firewall. Other addresses selected by `auto` may be unreachable from this VM. |
| Fingerprint mismatch or HTTP 401 | Check the fingerprint and token printed during Windows pairing. If the token was lost, revoke with `jc pair revoke --name devvm`, then pair again on Windows and Linux. |
| The request succeeds but VS Code does not connect | Retry `ssh devvm` on Windows and check Remote-SSH, the registered `vscode://` handler, and the error in the VS Code window. A successful request only confirms that Windows accepted the launch. |

## Short commands and language

`server` has alias `s`, `client` has alias `c`, top-level `pair` has alias `p`, `task` has alias `t`, `revoke` has alias `r`, and `status` has alias `st`, as in `jc s st` or `jc t st`. All pairing credentials are managed through `jc pair`. For example:

```powershell
jc p create --name devvm --ssh-host devvm
```

```sh
jc p join --server 192.168.56.1:59684 --fingerprint 'paste the certificate fingerprint'
jc c .
```

Command names take precedence over local folders with the same name. On Windows, use `jc ./foldername` to open a folder named `pair`, `task`, `p`, `t`, `s`, or `c`.

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
