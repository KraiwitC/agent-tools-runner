# Agent Tools Runner

## Project Purpose

Agent Tools Runner is a small, human-controlled local program that allows a large language model (LLM) chatbot to inspect and make targeted changes to a local software project through structured copy-and-paste requests.

The LLM is responsible for understanding the user's requirement, planning the work, selecting actions, and interpreting results. Agent Tools Runner is responsible for executing supported local actions, enforcing basic safety rules, and returning structured results.

The initial use case is an enterprise environment where an LLM chatbot cannot directly access the developer's workstation. The design must not depend on one specific chatbot or model provider.

## Current Status

Agent Tools Runner version 0.6.0 implements the current protocol version 1 feature set.

The implementation language is Go. The application is an interactive local command-line program.

Keep the codebase small enough that it can be read and understood by one developer who is learning Go.

## Interaction Model

Version 1 uses an interactive terminal session and manual copy and paste:

```text
LLM chatbot
     ↕
Manual copy and paste
     ↕
Agent Tools Runner (`atr`)
     ├─ Prepare the LLM bootstrap prompt
     ├─ Inspect the project tree
     ├─ Search code
     ├─ Read files
     ├─ Apply targeted edits
     └─ Create approved new files
```

The user normally starts Agent Tools Runner from the target project directory:

```text
atr
```

An explicit workspace may also be provided:

```text
atr --workspace <path>
```

The application version may be displayed with `atr --version` without starting an interactive session.

Direct integration with an LLM provider is not required for Version 1.

## Version 1 User Workflow

1. The user starts `atr` from the target project directory or supplies `--workspace <path>`.
2. ATR loads the workspace's `AGENTS.md` when present, prepares the bootstrap prompt, and copies it to the clipboard.
3. The LLM infers whether the request is exploration, debugging, review, or modification and keeps its response and planning depth proportional to the task.
4. Before repository access, the LLM proposes a concise read-only investigation and waits for approval. A simple lookup may use a one-sentence proposal.
5. When modification is intended, the LLM uses verified investigation results to propose a concise implementation plan and waits for separate implementation approval.
6. The user transfers approved JSON requests to ATR and returns structured responses to the LLM.
7. ATR validates and executes actions in order. Pasting a modifying request authorizes that exact request.
8. The user reviews resulting changes with the editor's source-control or file-comparison tools.

## Version 0.6.0 Scope

Version 0.6.0 supports these operations while retaining protocol version `1`:

- `tree`: Read a bounded project-directory tree without returning file contents.
- `search`: Search text files inside the selected workspace using case-sensitive literal matching.
- `ranked_search`: Find confidence-ranked matches using exact, case-insensitive, identifier-aware, and fuzzy lexical matching.
- `read`: Read one or more complete text files and return a complete-file SHA-256 hash.
- `read_range`: Read an inclusive one-based line range and return the complete-file SHA-256 hash.
- `inspect`: Return file metadata and SHA-256, or report whether a directory is empty.
- `edit`: Apply exact replacements when `expectedSha256` matches the current file.
- `create`: Create one approved new UTF-8 text file without overwriting an existing path and return its SHA-256 hash.
- `copy`: Copy one hash-matched UTF-8 text file without overwriting the destination and return its SHA-256 hash.
- `move`: Move or rename one hash-matched UTF-8 text file without overwriting the destination and return its SHA-256 hash.
- `mkdir`: Create one directory whose parent already exists.
- `delete`: Delete a hash-matched regular file or an empty directory.

A request uses protocol version `1`. Existing compatible request shapes remain valid, except that `edit` now requires `expectedSha256` for stale-file protection.

A request contains a non-empty ordered `actions` array. Every action has a unique `id` and an `operation`. A request may contain at most 100 actions, and one edit action may contain at most 100 replacements.

The optional request-level `maxTransferChars` must be between 1,000 and 120,000 characters. Its default is 100,000. The limit applies to the complete serialized JSON response. One `read_range` action may request at most 1,000 lines.

A single request may mix supported operations. The normal workflow uses an investigation request followed by one or more later modifying requests after the LLM receives actual repository contents and hashes.

The complete request must pass structural validation before the first action executes. Runtime action failures preserve earlier successful results, include the failed action result, and stop before later actions execute. Batches are ordered but are not transactions. An earlier successful modifying action is not rolled back when a later action fails.


## Interactive CLI

The executable is `atr`, and the current directory is the default workspace. The session accepts slash commands or a JSON request beginning with `{` or a supported JSON code fence. ATR collects request lines until an empty line and then validates the complete request once. `/cancel` on its own line discards a request during collection. Other ordinary input is ignored without replacing the last response.

Supported commands:

- `/help`: Show available commands.
- `/prompt`: Copy the bootstrap prompt.
- `/show-prompt`: Print the bootstrap prompt.
- `/copy`: Copy the last JSON response.
- `/show`: Print the last JSON response.
- `/workspace`: Show the selected workspace.
- `/clear`: Clear the terminal without deleting the last response.
- `/exit`: End the session.

The session keeps the workspace and last JSON response in memory only. `/prompt` and `/show-prompt` regenerate the bootstrap prompt from the current workspace instructions when invoked.

## LLM Bootstrap Prompt

At startup, ATR generates and copies a provider-neutral bootstrap prompt describing the planning and approval workflow, supported operations, request and response formats, safety rules, exact editing requirements, and repository instructions. The LLM must use actual ATR results and must not claim success before ATR reports it.

When the selected workspace contains `AGENTS.md`, its contents are included. If it is absent, prompt generation still succeeds and states that no repository-specific instructions were found. `/prompt` and `/show-prompt` reload `AGENTS.md` and regenerate the prompt without restarting ATR. A new LLM conversation is still required for refreshed instructions to take full effect.

## Clipboard Behavior

ATR uses `golang.design/x/clipboard` for plain-text clipboard access without shell commands. Successful initialization enables automatic copying of the bootstrap prompt and every structured response, including error responses. When clipboard initialization is unavailable, the prompt or response remains available through `/show-prompt` or `/show`.

## Confirmed Version 1 Requirements

### Workspace boundary

Agent Tools Runner operates inside one selected project workspace.

It must not read or modify files outside that workspace.

All requested paths must be resolved and validated by the runtime. Paths supplied by an LLM are untrusted input.

Symbolic links should be rejected in Version 1 rather than supported with complex cross-platform resolution rules.

### LLM-neutral terminology

Project documentation, protocol names, and runtime messages should use neutral terms such as `LLM`, `LLM chatbot`, or `assistant` instead of depending on Microsoft Copilot or another specific product.

Provider-specific integration may be added later without changing the core runtime.

### Batched read-only actions

One request may contain multiple related `search`, `ranked_search`, and `read` actions.

Actions execute in their declared order.

By default, if a required action fails, Agent Tools Runner must stop the remaining actions and return:

- the failure status;
- a structured error;
- the index or identifier of the failed action;
- all successful results collected before the failure.

Version 1 does not need optional actions or continue-on-error behavior unless a real use case is identified during implementation.

### Project tree

Version 0.5.0 introduced a basic recursive `tree` operation for inspecting repository structure without reading file contents.

A tree request contains one required workspace-relative directory path. Use `.` for the workspace root.

Tree results contain workspace-relative forward-slash paths and identify each entry as a file or directory. Results are deterministic, return at most 500 entries, and report `truncated=true` when additional entries exist.

Tree traversal skips symbolic links and uses the same built-in directory exclusions as search. It does not return file contents, sizes, timestamps, permissions, hashes, configurable depth, or glob-filtered results.

### Text search

Version 1 provides a basic case-sensitive literal text search implemented in Go.

The `search` operation remains case-sensitive literal matching. It does not require regular expressions, external search programs, parallel searching, or advanced glob syntax.

The read-only `ranked_search` operation combines exact, case-insensitive, identifier-aware, and fuzzy lexical matching. Its query must contain at least two letters or digits. Fuzzy matching is disabled when the query contains fewer than three letters or digits.

Literal search results identify the workspace-relative file path, one-based line number, and matching line text. A search returns at most 100 matches and reports `truncated=true` when additional matches exist.

Ranked search results additionally return `matchType` and a deterministic confidence `score`. Results are ordered by score, match-type priority, path, and line. A ranked search returns at most 20 matches and reports `truncated=true` when additional candidates exist. Ranked matches are discovery suggestions only; the LLM must read the selected file before editing, and edits remain exact and hash-protected.

Version 1 uses one centralized built-in directory exclusion set in `search.go`: `.git`, `.idea`, `node_modules`, `target`, `build`, `dist`, and `vendor`. `.vscode` remains searchable because it may contain useful project configuration. Configurable exclusions are deferred until a real use case requires them.

Search skips symbolic links, files larger than the 1 MiB Version 1 limit, invalid UTF-8 files, binary-looking files containing null bytes, and files that cannot be opened. A workspace traversal failure returns a structured `SEARCH_FAILED` error.

### File reading

A `read` request may include multiple files.

Version 1 supports complete UTF-8 text files only.

The runtime should reject binary files, invalid UTF-8 files, symbolic links, and files above the configured size limit.

A failed file read must return a structured error. It must not return invented, partial, or misleading content as a successful result.

### Safe file creation

Version 0.5.0 introduced separate `create`, `copy`, and `move` operations. Create must never be simulated through an edit with an empty or invented `oldText`.

A create request has this action shape:

```json
{
  "id": "create-file",
  "operation": "create",
  "path": "relative/path/to/file.txt",
  "content": "complete non-empty file content"
}
```

A successful create result returns the workspace-relative forward-slash path and the number of bytes written. It does not return the complete created content.

Create requires:

- one non-blank workspace-relative target path;
- complete non-empty content;
- valid UTF-8 text without null bytes;
- content no larger than the shared 1 MiB file-size limit;
- every parent directory to already exist;
- every parent path component to be a real directory and not a symbolic link;
- the target path not to exist.

Create does not make parent directories and never overwrites or modifies an existing path.

The implementation first writes and flushes the complete content to a temporary file in the target directory. It then claims the final target with exclusive creation so that a competing file cannot be silently overwritten. The target is removed if final writing, flushing, or closing fails. Temporary files are removed after success and failure.

The Go standard library does not provide one simple cross-platform primitive that both performs a no-overwrite rename and guarantees atomic final-file visibility. Version 0.5.0 prioritized the mandatory no-overwrite guarantee by using exclusive final-target creation, and the current implementation retains that design. Another process could briefly observe the newly created target while its prepared content is copied into it.

### Create and edit distinction

Use `create` only for a target file that does not exist. Use `edit` for an existing target file.

Edit continues to require one or more exact replacements with a non-empty `oldText`. Edit must not create a missing file. Do not use placeholder conventions such as `PLACEHOLDER` and do not invent edit targets.

### Targeted edits

Version 1 does not replace an entire existing file merely to change one part of it.

An edit contains:

- the target relative path;
- the exact existing text to find;
- the replacement text.

The runtime must require the existing text to occur exactly once.

The edit must fail when the existing text:

- is not found;
- occurs more than once;
- belongs to a file outside the workspace;
- targets a symbolic link;
- targets an unsupported file;
- cannot be written safely.

The runtime must not use fuzzy matching, infer the intended location, or choose one of several matches.

### Multiple edits to one file

One `edit` request may contain multiple targeted replacements for one existing file.

Agent Tools Runner must load the file and validate every requested replacement before writing anything.

If any replacement fails validation, the entire edit request fails and the file remains unchanged.

After validation, all replacements are applied in memory from the end of the file toward the beginning so earlier byte positions remain stable. The completed result is written once.

Overlapping replacement ranges are rejected with `EDIT_TARGET_OVERLAP`. Duplicate or otherwise ambiguous edit targets are never guessed.

Support for editing multiple files in one request is deferred until a real need is demonstrated.

### Safe writing

Agent Tools Runner should avoid leaving a partially written target file.

The initial implementation writes the completed content to a temporary file in the same directory, preserves the original file permissions, flushes and closes the temporary file, and then renames it over the target file. Temporary files are removed when a failure occurs.

The implementation must report `WRITE_FAILED` if it cannot complete the write safely. It must not report success until the final replacement succeeds.

### User authorization and review

Version 1 does not include a built-in diff preview or a second interactive approval prompt.

Pasting an `edit` request into Agent Tools Runner means the user authorizes that exact request to be applied.

The user reviews resulting changes in VS Code or another editor after execution.

Version 1 should normally be used in a Git working tree so changes can be reviewed and reverted, but Agent Tools Runner does not need to execute Git commands.

### Tool failures and structured errors

Every failed action must return an error result.

A failure must never be represented as a successful result with empty or fabricated data.

An error result should contain at least:

- a stable error code;
- a short human-readable message;
- the failed action index or identifier when applicable;
- safe contextual details, such as a relative file path;
- no stack trace, secret, or unnecessary absolute path.

Stable error codes include:

- `INVALID_REQUEST`
- `PATH_OUTSIDE_WORKSPACE`
- `SYMLINK_NOT_SUPPORTED`
- `FILE_NOT_FOUND`
- `FILE_ALREADY_EXISTS`
- `PARENT_DIRECTORY_NOT_FOUND`
- `UNSUPPORTED_FILE`
- `FILE_TOO_LARGE`
- `READ_FAILED`
- `SEARCH_FAILED`
- `TREE_FAILED`
- `EDIT_TARGET_NOT_FOUND`
- `EDIT_TARGET_NOT_UNIQUE`
- `EDIT_TARGET_OVERLAP`
- `FILE_CHANGED`
- `RANGE_OUT_OF_BOUNDS`
- `DIRECTORY_NOT_EMPTY`
- `WRITE_FAILED`
- `DELETE_FAILED`
- `TRANSFER_LIMIT_EXCEEDED`
- `INTERNAL_ERROR`

Create errors use `FILE_ALREADY_EXISTS` when the target already exists, `PARENT_DIRECTORY_NOT_FOUND` when a required parent is missing, and `WRITE_FAILED` when safe creation cannot be completed. Error responses include the action ID, zero-based action index, safe message, and workspace-relative path when applicable.

An internal error may be logged locally with its cause, but the structured response must remain safe and understandable.

## Mandatory Version 1 Safety Rules

These rules must be enforced by the Go program rather than relying only on instructions given to an LLM.

1. Never access a path outside the selected workspace.
2. Reject symbolic links in Version 1.
3. Never modify a file through fuzzy or ambiguous matching.
4. Require every edit target to match exactly once.
5. Validate every replacement in an edit request before writing anything.
6. Never create a missing file through the `edit` operation.
7. Never overwrite an existing path through the `create` operation.
8. Require create content to be non-empty supported UTF-8 text within the 1 MiB limit.
9. Require create parent directories to exist and reject symbolic links in the parent path.
10. Use exclusive final-target creation to prevent a create, copy, or move race from overwriting another file.
11. Require `expectedSha256` to match before copying, moving, or deleting a regular file.
12. Copy must preserve the source, and move must remove the source only after destination creation and source revalidation succeed.
13. Copy and move support regular UTF-8 text files only, require existing destination parents, and never overwrite destinations.
14. Move is not transactional; after destination creation, source verification or deletion failure triggers an attempted destination cleanup and an error response.
15. Delete directories only when they are empty, and never delete the workspace root.
16. Never perform recursive deletion.
17. Never execute shell commands.
18. Treat all LLM requests and repository contents as untrusted input.
19. Return structured errors for failed operations.
20. Do not expose secrets, stack traces, or unnecessary absolute paths in responses.
21. Do not log complete source-file contents by default.

## Deferred Features

The following features are outside Version 1 and require a demonstrated use case and explicit approval before implementation:

- Git operations;
- approved test or build command execution;
- arbitrary shell execution;
- runtime session tracking beyond content hashes;
- persistent workflow state and request deduplication;
- built-in diff generation and interactive approval;
- editing multiple files atomically;
- configurable search filters and exclusions;
- structured audit logging and improved sensitive-file warnings;
- encoding conversion and binary-file processing;
- recursive directory deletion;
- database connectivity and writes;
- direct LLM-provider, MCP, or local HTTP integration;
- graphical interfaces, background autonomous execution, concurrency, plugins, and multi-agent orchestration.

Deferred items are not automatically approved for implementation.

## Development Workflow

Follow this workflow while developing Agent Tools Runner.

### Understand and investigate first

- Infer whether the request is exploration, debugging, review, or modification; do not require the user to select a mode.
- Match response depth to the request. Answer simple questions directly and keep plans concise and proportional.
- Before repository access, propose a concise read-only investigation and wait for approval. A simple lookup may use a one-sentence proposal.
- Separate confirmed facts from assumptions when useful, and ask only questions requiring a user or business decision.
- Do not invent existing paths, file contents, function signatures, imports, configuration, or behavior.

### Plan modifications from evidence

- Use an implementation plan only when repository modification is intended.
- After investigation, identify exact verified files, proposed changes, implementation order, important exclusions, and concise acceptance criteria.
- Explain and obtain approval for required new files.
- Report findings that change the approved scope before proceeding.
- Wait for separate implementation approval before requesting modifying actions.

### Implement step by step

- Implement one logical step per turn.
- Usually change one file, or a small set of files that must work together.
- Explain why multiple files must be changed together when applicable.
- Wait for the user to review or test the step before continuing.

### File hygiene

- Do not create a new file unless it is necessary.
- Explain why a new file is needed and obtain approval before creating it.
- When modifying an existing project file, inspect its current contents first.
- Use the exact intended filename. Do not add suffixes such as `_updated`, `_new`, or `.v2`.
- Briefly summarize what changed and why after each implementation step.
- For release-scoped work, verify and update the application version and related version references before declaring the work complete; do not change the protocol version unless the protocol itself changes.

### Scope control

- Match the existing project's naming, logging, error-handling, and return conventions.
- Do not introduce a new library, abstraction, pattern, or framework without discussing why it is necessary.
- Mention bugs or design concerns discovered during the work, but do not fix unrelated issues without approval.
- Do not rename or correct existing paths, identifiers, or apparent typos until their callers and impact are understood.

## Go Development Guidelines

The Go implementation must remain small, direct, and idiomatic.

- Prefer the Go standard library where it adequately meets the requirement.
- Do not add a framework.
- Do not add a web server in Version 1.
- Use one small cross-platform clipboard dependency because clipboard transfer is part of the primary user experience.
- Keep the clipboard package behind a small focused function so the rest of the program does not depend directly on the external library.
- Do not introduce concurrency in Version 1.
- Avoid interfaces when there is only one implementation and no clear testing or design benefit.
- Do not create a generic tool registry in Version 1.
- Keep packages focused, but do not split the project into many small packages prematurely.
- Handle every error deliberately. Do not ignore errors.
- Add useful context when returning an error while preserving its underlying cause for local diagnostics.
- Validate untrusted input at the program boundary.
- Keep request orchestration understandable and keep direct filesystem operations in small, focused functions.
- Use explicit request and result structs for the JSON protocol.
- Do not log secrets or complete source-file contents by default.
- Format Go source with `gofmt`.
- Explain unfamiliar Go concepts when they are introduced so the project also supports learning Go.

## Testing Expectations

Keep tests beside their focused packages. Use the same package name when direct testing of private helpers is useful, use `t.TempDir`, and never modify the real repository.

Prioritize tests for workspace boundaries, symbolic links, file limits, exact edits, stale hashes, no-overwrite creation, safe deletion, ordered batch failures, structured errors, and cleanup after failed writes.

Run before committing:

```text
go fmt ./...
go test ./...
go vet ./...
```

## Architecture and Project Decisions

- Project name: Agent Tools Runner
- Repository name: `agent-tools-runner`
- Implementation language: Go
- Initial application type: Interactive local command-line application
- Executable name: `atr`
- Default workspace: Current directory
- Version flag: `--version`
- Optional workspace flag: `--workspace <path>`
- Initial transport: Manual copy and paste with automatic clipboard copying
- Clipboard implementation: One small cross-platform Go library
- Startup behavior: Generate and copy the LLM bootstrap prompt
- Repository instructions: Include workspace `AGENTS.md` in the bootstrap prompt when present
- Session commands: `/help`, `/prompt`, `/show-prompt`, `/copy`, `/show`, `/workspace`, `/clear`, and `/exit`
- JSON submission: Paste a JSON object beginning with `{`, then enter an empty line to execute it
- JSON cancellation: Enter `/cancel` on its own line while collecting a request
- Session persistence: In memory only
- Project instruction file: `AGENTS.md`
- Assistant terminology: LLM-neutral
- Current operations under protocol version `1`: `tree`, `search`, `ranked_search`, `read`, `read_range`, `inspect`, `edit`, `create`, `copy`, `move`, `mkdir`, and `delete`
- Maximum actions per request: 100
- Maximum replacements per edit action: 100
- Maximum tree entries per result: 500
- Create behavior: One new non-empty UTF-8 text file, no overwrite, existing parent directories only
- Copy behavior: One hash-matched UTF-8 text file, source preserved, no destination overwrite, existing parent directories only
- Move behavior: One hash-matched UTF-8 text file, destination created before source removal, no destination overwrite, existing parent directories only; rename uses the same operation
- Multiple read-only actions in one request: Supported
- Multiple targeted replacements to one file: Supported
- Exact unique matching: Mandatory
- Runtime read-before-write tracking: Deferred
- Built-in diff preview: Deferred
- User review: Performed in VS Code or another editor
- Git execution: Deferred
- Test execution: Deferred
- SQL support: Deferred
- Arbitrary shell execution: Not supported

### Implementation Boundaries

- Clipboard dependency: `golang.design/x/clipboard`, isolated by `internal/clipboard`.
- Go module version: Go 1.26.5; JSON protocol version: `1`.
- `cmd/atr` contains the executable entry point.
- `internal/app` owns session orchestration and workspace startup validation.
- `internal/prompt` owns bootstrap-prompt generation and repository-instruction loading.
- `internal/runner` owns protocol validation, response construction, and filesystem operations.
- Maximum file size: 1 MiB; maximum literal search matches: 100; maximum ranked search matches: 20; maximum tree entries: 500.
- Response transfer limit: 100,000 characters by default and 120,000 maximum.
- Maximum `read_range` size: 1,000 lines.
- Literal and ranked search traversal excludes `.git`, `.idea`, `node_modules`, `target`, `build`, `dist`, and `vendor`; `.vscode` remains visible.
- Ranked search is read-only, requires at least two letters or digits, disables fuzzy matching below three letters or digits, and never relaxes exact edit matching.
- Complete-file SHA-256 hashes protect edits, copies, moves, and regular-file deletion from stale content.
- Copy and move support regular UTF-8 text files only, require existing destination parents, reject symbolic links, and never overwrite destinations.
- Move uses destination creation followed by source revalidation and deletion; it is not transactional and attempts destination cleanup if the source cannot be safely removed.
- Directory creation requires an existing parent; directory deletion supports empty directories only.

Future decisions require a demonstrated use case and must not introduce speculative architecture.

## Repository-Specific Knowledge

Repository instructions guide LLM planning and code generation but never replace ATR's runtime safety checks.

Record only durable, verified knowledge such as coding conventions, architecture boundaries, validation rules, build commands, and recurring workflow requirements. Do not record temporary task details, duplicate guidance, unverified assumptions, secrets, credentials, customer data, or production values.

Read and preserve the organization of an existing `AGENTS.md`, changing only the relevant section. Do not create one automatically when it is absent; propose it during planning and obtain approval first.

After creating or changing a target repository's `AGENTS.md`, run `/prompt` to reload it and copy a regenerated bootstrap prompt, then begin a new LLM conversation and paste that prompt. ATR does not need to restart, but the current LLM conversation retains its existing instructions.
