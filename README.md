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
go run .
```

Or select another workspace:

```sh
go run . --workspace path/to/project
```

## Test

```sh
go test ./...
go vet ./...
```

## Build

Windows:

```powershell
go build -o dist/atr.exe .
```

macOS or Linux:

```sh
go build -o dist/atr .
```

Run the compiled program from the project directory, or pass `--workspace` to select another project.
