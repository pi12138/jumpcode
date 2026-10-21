# Architecture

[Project home](../README.en.md) · [Usage](usage.en.md) · [Architecture](architecture.en.md) · [Development](development.en.md) · [简体中文](architecture.md)

```mermaid
flowchart LR
    Client["Linux terminal<br/>jc client ."] -->|"HTTPS: folder and pairing credential"| Server["Windows user session<br/>jc server"]
    Server -->|"vscode:// open request"| VSCode["Windows desktop<br/>VS Code + Remote-SSH"]
    VSCode -->|"SSH: project file access"| VM["Linux VM<br/>project folder"]
    WindowsCLI["Windows terminal<br/>jc folder or host:folder"] -->|"direct vscode:// request"| VSCode
```

There are two distinct connections. Linux-to-Windows HTTPS carries control information such as the folder path and authentication credential, not project files. VS Code on Windows then connects to Linux over SSH to read and edit project files. For Linux-initiated opens, Windows must be able to SSH into the VM, and the VM must be able to reach the Windows server's listen address. Running `jc devvm:/home/me/project` on Windows takes the direct path in the diagram without `jc server`; opening a local Windows folder needs no SSH connection either.

## Components

| Component | Location and responsibility |
| --- | --- |
| `jc server` | Runs in the signed-in Windows user's session, listens on HTTPS, verifies client credentials, and asks Windows to open VS Code through its registered `vscode://` handler. It can run in the foreground, background, or after sign-in. |
| `jc client` | Runs briefly on the Linux VM, resolves the folder to an absolute path, and requests an open from the paired Windows server. It does not stay running. |
| VS Code + Remote-SSH | Runs on the Windows desktop and establishes the actual SSH connection to Linux. Jumpcode can only confirm that the launch request was accepted; check the VS Code window for the final result. |
| Windows Task Scheduler | Optional. Once a sign-in task is installed, it starts `jc server` the next time the current user signs in, or immediately with `jc s start`. `jc s stop` stops the current server gracefully while keeping automatic startup. It is not a Windows Service running before sign-in. |

## Pairing and identity verification

Initial use requires pairing. Windows issues a pairing token shown only once and binds it to a fixed SSH host. Linux stores the server address, certificate fingerprint, and token. Subsequent requests authenticate with the token, and the client pins the server certificate fingerprint. The server opens only the SSH host associated with the pairing; clients cannot select another host per request. Each Linux user's client config stores one Windows server address, while the Windows server may listen on multiple IPs.

Each pairing has its own token so it can be revoked independently and identifies its bound SSH host. The certificate fingerprint belongs to the Windows server. Pairings under the same user config folder share a fingerprint; restarting the server or changing listen IPs does not change it. Keep tokens private; certificate fingerprints can be public. This fingerprint is separate from the Linux SSH host fingerprint.
