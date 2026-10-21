# Development guide

[Project home](../README.en.md) · [Usage](usage.en.md) · [Architecture](architecture.en.md) · [Development](development.en.md) · [简体中文](development.md)

Run all build and verification commands from the repository root.

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

Put the binary on `PATH`. The first build may download dependencies from `go.mod`. The `-buildvcs=false` option also permits builds from source directories without Git metadata.

## Project layout

The command layer follows the common layout in the [Cobra user guide](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md): root-level `main.go` calls `cmd.Execute()`, command definitions live in individual files under `cmd/`, and implementation packages live under `internal/`.

```text
main.go              Entry point; calls cmd.Execute()
docs/                Usage, architecture, and development documentation
scripts/release/     Two-platform archives, version manifest, and SHA-256 checksums
.github/workflows/  Windows/Linux CI and tag-triggered releases
cmd/
  root.go            Root command, language initialization, and error handling
  client.go          Client open command
  server*.go         Server, background management, and sign-in subcommands
  pair*.go           Pairing management subcommands
  token.go           Pairing token input from the terminal
internal/
  client/            Linux client pairing verification and open requests
  server/            HTTPS server, multiple listeners, background process, and sign-in task
  pairing/           Pairing credentials, SSH host binding, and revocation
  config/            User settings, certificate, and private key persistence
  protocol/          Shared requests, address validation, token hashes, and TLS fingerprint checks
  remote/            Local and SSH paths, VS Code URIs, and Windows launch operations
  i18n/              Language detection, messages, and error translations
  version/           Version, commit, and build time injected during builds
```

`cmd/` registers commands, parses arguments, handles terminal interaction, and calls the feature packages. The client and server share communication rules through `protocol` and access settings through `config`. Pairing credential management is independent of the server process: `pair create` works before the server starts, while `pair join` connects to the server to verify credentials. Tests live alongside their modules, and Windows implementations use `_windows.go` files.

## Verification

```sh
go test -buildvcs=false ./...
go test -race -buildvcs=false ./...
go vet -buildvcs=false ./...
```

## Version and release packaging

`jc --version` (or `jc -v`) prints the version, commit, UTC build time, and target platform. A regular `go build` reports `dev` with an `unknown` commit and build time. The release tool injects these values through `-ldflags`; background logs include them too.

The release tool uses the Go standard library and Git. It produces a Windows amd64 ZIP, a Linux amd64 tar.gz, `release.json`, and `SHA256SUMS`. Each archive includes the executable, bilingual guides, and release notes for that version. Linux executable permissions are preserved. Local configuration, credentials, and the ignored validation draft are excluded.

Create a test snapshot during development:

```sh
go run ./scripts/release --version v0.1.0-beta.1 --snapshot
```

Output goes to `dist/v0.1.0-beta.1.snapshot/`. Snapshot versions include `.snapshot`; uncommitted changes add `-dirty` to the commit. The manifest records both `snapshot` and `dirty`. Existing output directories are never overwritten, and a failed build does not leave a completed release directory.

For a release, prepare `docs/releases/<version>.md` and `.en.md`, commit all source changes, ensure the working tree is clean, and create a version tag pointing to the current commit:

```sh
git tag -a v0.1.0-beta.1 -m 'Release v0.1.0-beta.1'
go run ./scripts/release --version v0.1.0-beta.1
```

Output goes to `dist/v0.1.0-beta.1/`. Packaging fails if the working tree is dirty or the tag does not point to the current commit. Checksums cover both archives and the manifest. Verify them on Linux:

```sh
cd dist/v0.1.0-beta.1
sha256sum -c SHA256SUMS
```

The packaging tool only generates local artifacts. Before a stable release, use the candidate binaries to verify project opens, background start/stop, sign-in tasks, and failure recovery in a Windows desktop session. Beta release notes must state the actual acceptance scope.

## GitHub release workflow

The repository distributes versions through [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository). Workflows live in `.github/workflows/`.

Pushing `main` runs formatting checks, `go vet`, tests, and builds on both Linux and Windows. Linux also runs the race detector. Unix permission checks run only on platforms with that permission model; protocol authentication and pairing revocation remain tested on Windows.

Push a version tag to trigger publication:

```sh
git push origin main
git push origin refs/tags/v0.1.0-beta.1
```

The Release workflow first runs both platform checks on the tagged source, then builds and verifies the archives. It uploads all assets to a draft before publishing it. Versions with prerelease suffixes such as `-beta` are marked as pre-releases. The four assets are the Windows ZIP, Linux tar.gz, `release.json`, and `SHA256SUMS`.

If an upload fails, rerun the workflow to resume the draft. Published releases are never overwritten. Release can also be dispatched manually from Actions with an existing version tag. Prepare release notes and update the versioned README download links for each new version.
