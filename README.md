# Agent Tools Runner

Agent Tools Runner (`atr`) is a human-in-the-loop agent harness that turns any chat-only LLM into a coding agent for your local workspace.

## Download

Download the latest prebuilt binary from the [GitHub Releases](https://github.com/KraiwitC/agent-tools-runner/releases) page.

Available platforms:

- Windows AMD64
- macOS ARM64
- Linux AMD64

Run ATR from the project directory you want it to access, or select another directory with `--workspace`.

## Safety boundaries

- ATR keeps filesystem access inside the selected workspace.
- Symbolic links are rejected.
- File operations are limited to files of at most 10 MiB.
- Existing destinations are not overwritten by create, copy, or move operations.
- Directory deletion is limited to empty directories.
- ATR does not execute shell commands.

## Supported operations

- `tree`: List a bounded directory tree.
- `search`: Find case-sensitive literal matches in folder paths, file paths, and supported text-file content.
- `ranked_search`: Find confidence-ranked matches in folder paths, file paths, and supported text-file content using exact, case-insensitive, identifier-aware, and fuzzy lexical matching.
- `read`: Read one or more complete text files and return a SHA-256 hash for each file.
- `read_range`: Read an inclusive one-based line range and return the complete-file SHA-256 hash.
- `inspect`: Return file metadata and SHA-256, or report whether a directory is empty.
- `edit`: Apply exact unique replacements when `expectedSha256` matches the current file.
- `create`: Create one new UTF-8 text file without overwriting an existing path.
- `copy`: Copy a hash-matched UTF-8 text file without overwriting the destination.
- `move`: Move or rename a hash-matched UTF-8 text file without overwriting the destination.
- `mkdir`: Create one directory whose parent already exists.
- `delete`: Delete a hash-matched file or an empty directory.

## Download dependencies

```sh
go mod download
```

## Run from source

Start ATR in the current project directory:

```sh
go run ./cmd/atr
```

Or select another workspace:

```sh
go run ./cmd/atr --workspace path/to/project
```

Start with clipboard monitoring enabled:

```sh
go run ./cmd/atr --auto
```

While clipboard monitoring is active, ATR displays `Command > ` and continues to accept slash commands. Use `/auto` to toggle clipboard monitoring on or off.

Show the application version:

```sh
go run ./cmd/atr --version
```

## Test

```sh
go fmt ./...
go test ./...
go vet ./...
```

## Build

Windows:

```powershell
go build -o dist/atr.exe ./cmd/atr
$env:GOOS="darwin"; $env:GOARCH="arm64"; go build -o dist/atr ./cmd/atr
```

macOS or Linux:

```sh
go build -o dist/atr ./cmd/atr
GOOS=windows GOARCH=amd64 go build -o dist/atr.exe ./cmd/atr
```
