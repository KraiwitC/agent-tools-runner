# Agent Tools Runner

## Project Purpose

Agent Tools Runner is a small, human-controlled local program that allows a large language model (LLM) chatbot to inspect and make targeted changes to a local software project through structured copy-and-paste requests.

The LLM is responsible for understanding the user's requirement, planning the work, selecting actions, and interpreting results. Agent Tools Runner is responsible for executing supported local actions, enforcing basic safety rules, and returning structured results.

The initial use case is an enterprise environment where an LLM chatbot cannot directly access the developer's workstation. The design must not depend on one specific chatbot or model provider.

## Current Status

Agent Tools Runner version 0.2.0 is under implementation.

The implementation language is Go. The application is an interactive local command-line program.

Keep the codebase small enough that it can be read and understood by one developer who is learning Go.

## Initial Interaction Model

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

Direct integration with an LLM provider is not required for Version 1.

## Version 1 User Workflow

1. The user starts `atr` from the target project directory or supplies `--workspace <path>`.
2. Agent Tools Runner loads the workspace's `AGENTS.md` when present.
3. Agent Tools Runner generates an LLM bootstrap prompt containing the ATR workflow, supported operations, request format, error behavior, editing rules, and repository instructions.
4. Agent Tools Runner copies the bootstrap prompt to the system clipboard.
5. The user pastes the bootstrap prompt into an LLM chatbot.
6. The user describes a software-development requirement to the LLM chatbot.
7. The LLM restates the requirement and lists its assumptions.
8. The LLM asks clarifying questions when the requirement is ambiguous.
9. The LLM proposes a short investigation and implementation plan.
10. The user approves or adjusts the plan.
11. The LLM creates a structured request for Agent Tools Runner.
12. One request may contain multiple related read-only actions, such as searching code and reading several files.
13. The user pastes the request into the interactive ATR session.
14. Agent Tools Runner validates and executes the requested actions in order.
15. Agent Tools Runner displays a short human-readable summary and copies the complete JSON response to the clipboard.
16. The user pastes the response into the LLM chatbot.
17. The LLM uses the actual project contents to prepare one or more exact targeted edits.
18. The user pastes the edit request into Agent Tools Runner. Submitting the request authorizes those edits to be applied.
19. Agent Tools Runner validates all edits before writing.
20. Agent Tools Runner applies the edits only when every required validation succeeds.
21. Agent Tools Runner copies the structured result to the clipboard.
22. The user reviews the resulting changes with the editor's normal source-control or file-comparison tools, such as the VS Code Source Control view.

## Version 0.2.0 Scope

Version 0.2.0 supports these operations while retaining protocol version `1`:

- `tree`: Read a bounded project-directory tree without returning file contents.
- `search`: Search text files inside the selected workspace for a text value.
- `read`: Read one or more complete text files in a single request.
- `edit`: Apply one or more exact text replacements to one existing text file.
- `create`: Create one approved new UTF-8 text file without overwriting an existing path.

A request uses protocol version `1` because `tree` and `create` are backward-compatible additions. Existing `search`, `read`, and `edit` requests continue to use the same request shapes.

A request contains a non-empty ordered `actions` array. Every action has a unique `id` and an `operation`. A request may contain at most 100 actions, and one edit action may contain at most 100 replacements.

A single request may mix supported operations. The normal workflow uses an investigation request followed by one or more later edit or create requests after the LLM receives actual repository contents.

The complete request must pass structural validation before the first action executes. Runtime action failures preserve earlier successful results, include the failed action result, and stop before later actions execute. Batches are ordered but are not transactions. An earlier successful create or edit is not rolled back when a later action fails.


## Version 1 Interactive CLI

The installed executable name is `atr`.

Starting `atr` opens an interactive terminal session for the selected workspace. The current directory is the default workspace.

The session accepts either:

- a slash command beginning with `/`; or
- a JSON request beginning with `{`.

For a JSON request, Agent Tools Runner enters collection mode and accepts every pasted line until the user enters an empty line. It then validates the complete block exactly once. While collecting JSON, `/cancel` on its own line discards the current request.

Ordinary non-command input that does not begin with `{` is ignored with a short terminal message. It must not create a structured error response or replace the clipboard contents.

The minimum Version 1 slash commands are:

- `/help`: Show available commands.
- `/prompt`: Regenerate and copy the LLM bootstrap prompt.
- `/show-prompt`: Print the bootstrap prompt as plain text.
- `/copy`: Copy the last complete JSON response again.
- `/show`: Print the last complete JSON response as plain JSON.
- `/workspace`: Show the selected workspace.
- `/clear`: Clear the terminal without deleting the last response.
- `/exit`: End the session cleanly.

During JSON collection, `/cancel` discards the current request and returns to the normal prompt.

The terminal should display short human-readable summaries. The clipboard should contain the complete machine-readable prompt or JSON response without ANSI escape codes.

The session keeps only the current workspace, current bootstrap prompt, and last JSON response in memory. Persistent session storage is not required in Version 1.

## LLM Bootstrap Prompt

Agent Tools Runner must prepare the LLM before the first tool request.

At startup, it should generate and copy a bootstrap prompt that explains:

- the separate responsibilities of the LLM and Agent Tools Runner;
- the requirement-restatement, clarification, planning, and approval workflow;
- the supported `tree`, `search`, `read`, `edit`, and `create` operations;
- the exact JSON request format and batching rules;
- the structured response and error behavior;
- that edits use exact existing text and exact replacement text;
- that edit targets must be unique and include enough context to avoid ambiguity;
- that unrelated code and formatting must not be changed;
- that placeholders or ellipses must not be used inside edit text;
- that the LLM must not claim success until Agent Tools Runner returns success;
- that the user manually transfers requests and responses;
- the target repository's `AGENTS.md` contents when the file exists.

The bootstrap prompt must remain LLM-provider-neutral and must not include internal implementation details, future roadmap items, or Agent Tools Runner's own Go development history.

If the workspace has no `AGENTS.md`, prompt generation still succeeds and clearly states that no repository-specific instructions were found.

## Clipboard Behavior

Clipboard support is a core Version 1 usability feature.

Agent Tools Runner will use one small cross-platform Go clipboard library rather than operating-system shell commands. The library must support Windows and macOS, plain UTF-8 text, and an acceptable open-source license. Prefer a library that does not require CGO or external commands when practical.

At startup, Agent Tools Runner automatically copies the bootstrap prompt.

After every request, including failed requests, Agent Tools Runner automatically copies the complete structured JSON response.

A clipboard-copy failure does not change the success or failure of the underlying tool actions. The terminal must report the clipboard problem separately, retain the prompt or response in memory, and allow the user to retry with `/prompt` or `/copy` or print it with `/show-prompt` or `/show`.

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

One request may contain multiple related `search` and `read` actions.

Actions execute in their declared order.

By default, if a required action fails, Agent Tools Runner must stop the remaining actions and return:

- the failure status;
- a structured error;
- the index or identifier of the failed action;
- all successful results collected before the failure.

Version 1 does not need optional actions or continue-on-error behavior unless a real use case is identified during implementation.

### Project tree

Version 0.2.0 provides a basic recursive `tree` operation for inspecting repository structure without reading file contents.

A tree request contains one required workspace-relative directory path. Use `.` for the workspace root.

Tree results contain workspace-relative forward-slash paths and identify each entry as a file or directory. Results are deterministic, return at most 500 entries, and report `truncated=true` when additional entries exist.

Tree traversal skips symbolic links and uses the same built-in directory exclusions as search. It does not return file contents, sizes, timestamps, permissions, hashes, configurable depth, or glob-filtered results.

### Text search

Version 1 provides a basic case-sensitive literal text search implemented in Go.

It does not initially require regular expressions, fuzzy matching, external search programs, parallel searching, or advanced glob syntax.

Search results identify the workspace-relative file path, one-based line number, and matching line text. A search returns at most 100 matches and reports `truncated=true` when additional matches exist.

Version 1 uses one centralized built-in directory exclusion set in `search.go`: `.git`, `.idea`, `node_modules`, `target`, `build`, `dist`, and `vendor`. `.vscode` remains searchable because it may contain useful project configuration. Configurable exclusions are deferred until a real use case requires them.

Search skips symbolic links, files larger than the 1 MiB Version 1 limit, invalid UTF-8 files, binary-looking files containing null bytes, and files that cannot be opened. A workspace traversal failure returns a structured `SEARCH_FAILED` error.

### File reading

A `read` request may include multiple files.

Version 1 supports complete UTF-8 text files only.

The runtime should reject binary files, invalid UTF-8 files, symbolic links, and files above the configured size limit.

A failed file read must return a structured error. It must not return invented, partial, or misleading content as a successful result.

### Safe file creation

Version 0.2.0 adds a separate `create` operation. Create must never be simulated through an edit with an empty or invented `oldText`.

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

The Go standard library does not provide one simple cross-platform primitive that both performs a no-overwrite rename and guarantees atomic final-file visibility. Version 0.2.0 prioritizes the mandatory no-overwrite guarantee by using exclusive final-target creation. Another process could briefly observe the newly created target while its prepared content is copied into it.

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
- `UNKNOWN_OPERATION`
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
- `WRITE_FAILED`
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
10. Use exclusive final-target creation to prevent a create race from overwriting another file.
11. Never delete a user file. Removing an incomplete target created by the current failed create action is required cleanup, not a delete operation exposed to the LLM.
12. Never execute shell commands.
13. Treat all LLM requests and repository contents as untrusted input.
14. Return structured errors for failed operations.
15. Do not expose secrets, stack traces, or unnecessary absolute paths in responses.
16. Do not log complete source-file contents by default.

## Deliberately Excluded from Version 1

The following features are moved out of Version 1 to keep the codebase small and understandable:

- runtime-enforced read-before-write tracking;
- file hashes and stale-file detection;
- complete-file replacement;
- built-in diff generation;
- interactive diff approval;
- persistent workflow state;
- workflow identifiers and expiration;
- request deduplication;
- Git status or Git diff execution;
- test or build execution;
- arbitrary shell execution;
- structured audit files;
- concurrency and parallel action execution;
- advanced secret detection;
- encoding detection and conversion;
- binary file processing;
- editing multiple files atomically;
- generic plugin or tool registry architecture.

## Phase 2 Candidates

Phase 2 may add features that improve the developer workflow after Version 1 is working and understood:

- Git status and Git diff operations;
- approved test and build command profiles;
- command timeouts and output capture;
- runtime-enforced read-before-write tracking;
- file hashes and stale-file detection;
- built-in change preview and approval;
- persistent workflow state;
- editing multiple files in one request;
- better search filters and exclusions;
- structured audit logging;
- improved sensitive-file warnings.

Phase 2 requirements must be discussed before implementation. Inclusion in this list is not automatic approval to build the feature.

## Later Features

The following features remain outside the near-term scope:

- SQL queries and database connectivity;
- database writes;
- Git commit, push, reset, or history rewriting;
- file deletion;
- direct integration with a specific LLM provider;
- MCP or local HTTP integration;
- graphical user interface;
- background autonomous execution;
- plugin systems;
- multi-agent orchestration.

## Development Workflow

Follow this workflow while developing Agent Tools Runner.

### Understand first

- Restate the requested change in your own words.
- List assumptions.
- Ask clarifying questions when the intent is ambiguous.
- Do not invent existing file contents, function signatures, imports, configuration, or behavior.
- Request the exact current file when it is needed and has not been provided.

### Plan before coding

- Propose a short implementation plan.
- Identify which files will be inspected or changed.
- Explain the purpose of each change and the implementation order.
- Wait for user approval before writing code.

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

Tests use root-level `package main`, use `t.TempDir`, and must not modify the real repository.

Version 0.2.0 tests focus on behavior most likely to damage a project or produce misleading results:

1. paths outside the workspace are rejected;
2. symbolic links are rejected;
3. multiple files can be read in one request;
4. unsupported and oversized files return errors;
5. edit targets must be exact, unique, non-overlapping, and non-empty;
6. all replacements are validated before writing;
7. failed edits leave the file unchanged;
8. create writes complete UTF-8 content exactly and reports the normalized path and byte count;
9. create rejects existing targets without changing them;
10. create rejects missing parents, unsafe paths, symbolic links, invalid UTF-8, null bytes, and oversized content;
11. successful and failed creates leave no `.atr-create-*` temporary files;
12. tree returns deterministic forward-slash paths, honors exclusions, skips symbolic links, and reports truncation;
13. failed actions preserve earlier results and prevent later actions from running;
14. tool failures produce structured error results;
15. error responses do not expose stack traces or unnecessary absolute paths;
16. existing search, read, edit, and protocol behavior remains compatible.

Before version 0.2.0 is declared complete, run:

```text
go fmt ./...
go test ./...
go vet ./...
```

## Confirmed Project Decisions

- Project name: Agent Tools Runner
- Repository name: `agent-tools-runner`
- Implementation language: Go
- Initial application type: Interactive local command-line application
- Executable name: `atr`
- Default workspace: Current directory
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
- Version 0.2.0 operations under protocol version `1`: `tree`, `search`, `read`, `edit`, and `create`
- Maximum actions per request: 100
- Maximum replacements per edit action: 100
- Maximum tree entries per result: 500
- Create behavior: One new non-empty UTF-8 text file, no overwrite, existing parent directories only
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

## Version 0.2.0 Implementation Decisions

- Clipboard dependency: `golang.design/x/clipboard`, kept behind focused clipboard functions.
- Go module version: Go 1.26.5.
- JSON protocol version: `1`.
- Maximum file size: 1 MiB.
- Maximum search matches: 100.
- Maximum tree entries: 500.
- Maximum actions per request: 100.
- Maximum replacements per edit action: 100.
- Built-in traversal exclusions: `.git`, `.idea`, `node_modules`, `target`, `build`, `dist`, and `vendor`.
- `.vscode` remains visible to search and tree operations.
- Source layout remains a flat root-level `package main` structure.
- `create.go` and `tree.go` contain the separate filesystem responsibilities introduced in version 0.2.0.

Future decisions must be based on a demonstrated use case and must not introduce speculative architecture.

## Repository-Specific Knowledge

When Agent Tools Runner is used with another repository, the LLM should inspect that repository's `AGENTS.md` when one exists.

Repository instructions guide the LLM's planning and code generation. They do not replace the runtime's safety checks.

Durable repository knowledge may include coding conventions, architecture boundaries, build or test commands, logging conventions, validation rules, transaction rules, file-layout rules, recurring workflow requirements, and important constraints future tasks must follow.

Do not add temporary task details, one-time implementation notes, status updates, duplicate guidance, unverified guesses, secrets, credentials, tokens, customer data, production values, or other sensitive information to `AGENTS.md`.

When `AGENTS.md` exists, read it first and use a targeted edit that preserves its organization and terminology. Avoid duplication and change only the section affected by verified durable knowledge.

When `AGENTS.md` does not exist, do not create it automatically merely because it is missing. If the current task reveals useful durable repository instructions, propose a concise file during planning, explain why it is useful, and wait for user approval unless its creation was already explicitly approved. Use `create` only after approval and include only verified repository-specific instructions.

After a target repository's `AGENTS.md` is created or changed, the current ATR session still contains the old in-memory bootstrap prompt. Exit and restart ATR, start a new LLM conversation, and paste the newly generated bootstrap prompt. Automatic `AGENTS.md` reloading is outside version 0.2.0.

Language-specific and project-specific conventions belong in the target repository's own `AGENTS.md`. They are not general rules for Agent Tools Runner.
