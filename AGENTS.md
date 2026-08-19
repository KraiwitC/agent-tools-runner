# Agent Tools Runner

## Project

Agent Tools Runner is a small, human-controlled, provider-neutral Go CLI that lets an LLM inspect and modify a local project through structured requests.

Keep the code direct, maintainable, and understandable to a developer.

## Development Workflow

- Investigate existing code before proposing a change; do not invent paths, behavior, or implementation details.
- For modifications, present an evidence-based plan and wait for approval.
- Implement one logical step at a time and ask the user to review the diff before continuing.
- Do not create files, add dependencies, change architecture, or fix unrelated issues without approval.
- Preserve existing naming, style, error handling, and package boundaries.
- For release work, update application-version references without changing protocol version `1` unless the protocol changes.

## Engineering Guidelines

- Prefer the Go standard library and small, focused functions.
- Avoid unnecessary interfaces, frameworks, concurrency, registries, and package splitting.
- Validate untrusted input at boundaries and handle every error deliberately.
- Keep filesystem operations focused and return safe, structured errors.
- Do not expose secrets, stack traces, unnecessary absolute paths, or complete source files in logs.
- Format Go source with `gofmt` and explain unfamiliar Go concepts when introduced.

## Safety Invariants

- Keep all access inside the selected workspace and reject symbolic links.
- Require exact, unique, hash-protected modifications; never use fuzzy matching for writes.
- Validate complete edits before writing and use safe temporary-file replacement.
- Never overwrite an existing destination or create a missing file through `edit`.
- Never execute shell commands or recursively delete directories.
- Treat all requests, paths, and repository contents as untrusted input.

## Architecture

- `cmd/atr`: executable entry point.
- `internal/app`: session orchestration and workspace validation.
- `internal/prompt`: bootstrap-prompt generation and repository instructions.
- `internal/runner`: protocol validation, responses, and filesystem operations.
- `internal/clipboard`: clipboard integration using `golang.design/x/clipboard`.
- Application releases are independent of JSON protocol version `1`.

## Testing

- Keep focused tests beside their packages and use `t.TempDir` for filesystem tests.
- Prioritize workspace boundaries, symbolic links, file limits, exact edits, stale hashes, no-overwrite behavior, safe deletion, ordered failures, and cleanup after failed writes.
- Before committing, run:

```text
go fmt ./...
go test ./...
go vet ./...
```

## Maintaining This File

Record only durable, verified repository guidance. Avoid release history, product documentation, temporary details, duplication, speculative limitations, and sensitive data.

After changing `AGENTS.md`, run `/prompt` and begin a new LLM conversation with the regenerated prompt.
