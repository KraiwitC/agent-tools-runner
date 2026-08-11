# Agent Tools Runner

Agent Tools Runner (`atr`) is a local CLI that lets an LLM inspect and make controlled changes to files through manually copied JSON requests.

## Requirements

- Go 1.26.5 or later

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

Run the compiled program from the project directory, or pass `--workspace` to select another project.

## Supported operations

- `tree`: List a bounded directory tree.
- `search`: Search supported text files using case-sensitive literal matching.
- `ranked_search`: Find confidence-ranked matches using exact, case-insensitive, identifier-aware, and fuzzy lexical matching.
- `read`: Read one or more complete text files and return a SHA-256 hash for each file.
- `read_range`: Read an inclusive one-based line range and return the complete-file SHA-256 hash.
- `inspect`: Return file metadata and SHA-256, or report whether a directory is empty.
- `edit`: Apply exact unique replacements when `expectedSha256` matches the current file.
- `create`: Create one new UTF-8 text file without overwriting an existing path.
- `copy`: Copy a hash-matched UTF-8 text file without overwriting the destination.
- `move`: Move or rename a hash-matched UTF-8 text file without overwriting the destination.
- `mkdir`: Create one directory whose parent already exists.
- `delete`: Delete a hash-matched file or an empty directory.
