# Agent Tools Runner

## Project Purpose

Agent Tools Runner is a small, human-controlled local program that allows a large language model (LLM) chatbot to inspect and make targeted changes to a local software project through structured copy-and-paste requests.

The LLM is responsible for understanding the user's requirement, planning the work, selecting actions, and interpreting results. Agent Tools Runner is responsible for executing supported local actions, enforcing basic safety rules, and returning structured results.

The initial use case is an enterprise environment where an LLM chatbot cannot directly access the developer's workstation. The design must not depend on one specific chatbot or model provider.

## Current Status

The project is ready to move from requirements discussion into initial implementation planning.

The implementation language is Go. The first interface will be a local command-line application.

Keep Version 1 small enough that the complete codebase can be read and understood by one developer who is learning Go.

## Initial Interaction Model

Version 1 uses an interactive terminal session and manual copy and paste:

```text
LLM chatbot
     ↕
Manual copy and paste
     ↕
Agent Tools Runner (`atr`)
     ├─ Prepare the LLM bootstrap prompt
     ├─ Search code
     ├─ Read files
     └─ Apply targeted edits
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

## Version 1 Scope

Version 1 supports only these operations:

- `search`: Search text files inside the selected workspace for a text value.
- `read`: Read one or more complete text files in a single request.
- `edit`: Apply one or more exact text replacements to one existing text file.

A request uses protocol version `1` and contains a non-empty ordered `actions` array. Every action has a unique `id` and an `operation`.

A single request may mix `search`, `read`, and `edit` actions, although the normal workflow uses one investigation request followed by a later edit request after the LLM receives the project contents.

The complete request must pass structural validation before the first action executes. Runtime action failures stop the batch while preserving earlier successful results.

The exact protocol field names are not final until the request and response format is approved.


## Version 1 Interactive CLI

The installed executable name is `atr`.

Starting `atr` opens an interactive terminal session for the selected workspace. The current directory is the default workspace.

The session accepts either:

- a slash command beginning with `/`; or
- one complete JSON request, including multiline pasted JSON.

Agent Tools Runner should collect multiline input until one complete JSON object is available. It must distinguish incomplete input from malformed completed input and must not execute malformed requests.

The minimum Version 1 slash commands are:

- `/help`: Show available commands.
- `/prompt`: Regenerate and copy the LLM bootstrap prompt.
- `/show-prompt`: Print the bootstrap prompt as plain text.
- `/copy`: Copy the last complete JSON response again.
- `/show`: Print the last complete JSON response as plain JSON.
- `/workspace`: Show the selected workspace.
- `/clear`: Clear the terminal without deleting the last response.
- `/exit`: End the session cleanly.

The terminal should display short human-readable summaries. The clipboard should contain the complete machine-readable prompt or JSON response without ANSI escape codes.

The session keeps only the current workspace, current bootstrap prompt, and last JSON response in memory. Persistent session storage is not required in Version 1.

## LLM Bootstrap Prompt

Agent Tools Runner must prepare the LLM before the first tool request.

At startup, it should generate and copy a bootstrap prompt that explains:

- the separate responsibilities of the LLM and Agent Tools Runner;
- the requirement-restatement, clarification, planning, and approval workflow;
- the supported `search`, `read`, and `edit` operations;
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

### Text search

Version 1 provides a basic text search implemented in Go.

It does not initially require regular expressions, fuzzy matching, external search programs, parallel searching, or advanced glob syntax.

Search results should identify the relative file path and matching line information while applying reasonable result limits.

### File reading

A `read` request may include multiple files.

Version 1 supports complete UTF-8 text files only.

The runtime should reject binary files, invalid UTF-8 files, symbolic links, and files above the configured size limit.

A failed file read must return a structured error. It must not return invented, partial, or misleading content as a successful result.

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

After validation, all replacements are applied in memory and the completed result is written once.

Support for editing multiple files in one request is deferred until a real need is demonstrated.

### Safe writing

Agent Tools Runner should avoid leaving a partially written target file.

The initial implementation should write the completed content to a temporary file in the same directory and then replace the target file using the safest simple approach supported by the target operating system.

The implementation must report a failure if it cannot complete the write safely.

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

Initial error codes may include:

- `INVALID_REQUEST`
- `UNKNOWN_OPERATION`
- `PATH_OUTSIDE_WORKSPACE`
- `SYMLINK_NOT_SUPPORTED`
- `FILE_NOT_FOUND`
- `UNSUPPORTED_FILE`
- `FILE_TOO_LARGE`
- `READ_FAILED`
- `EDIT_TARGET_NOT_FOUND`
- `EDIT_TARGET_NOT_UNIQUE`
- `WRITE_FAILED`
- `INTERNAL_ERROR`

An internal error may be logged locally with its cause, but the structured response must remain safe and understandable.

## Mandatory Version 1 Safety Rules

These rules must be enforced by the Go program rather than relying only on instructions given to an LLM.

1. Never access a path outside the selected workspace.
2. Reject symbolic links in Version 1.
3. Never modify a file through fuzzy or ambiguous matching.
4. Require every edit target to match exactly once.
5. Validate every replacement in an edit request before writing anything.
6. Never create a missing file through the `edit` operation.
7. Never delete a file.
8. Never execute shell commands.
9. Treat all LLM requests and repository contents as untrusted input.
10. Return structured errors for failed operations.
11. Do not expose secrets, stack traces, or unnecessary absolute paths in responses.
12. Do not log complete source-file contents by default.

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

Version 1 tests should focus on the behavior most likely to damage a project or produce misleading results:

1. paths outside the workspace are rejected;
2. symbolic links are rejected;
3. multiple files can be read in one request;
4. unsupported and oversized files return errors;
5. an edit target that is not found is rejected;
6. an edit target that occurs more than once is rejected;
7. multiple replacements are all validated before writing;
8. a failed multi-replacement request leaves the file unchanged;
9. a successful edit changes only the exact targeted text;
10. tool failures produce structured error results;
11. error responses do not expose stack traces or unnecessary absolute paths.

Exact build and test commands will be added after the Go module and initial source structure are approved.

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
- Session persistence: In memory only
- Project instruction file: `AGENTS.md`
- Assistant terminology: LLM-neutral
- Version 1 operations: `search`, `read`, and `edit`
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

## Remaining Implementation Decisions

The project is ready for code-writing preparation. Resolve these decisions during the relevant small implementation step rather than designing the entire program in advance:

1. Select and review the cross-platform Go clipboard library before adding the dependency.
2. Finalize the exact JSON request and response structs while implementing protocol validation.
3. Choose small fixed Version 1 limits for search matches and file size.
4. Finalize built-in search exclusions such as `.git`, `target`, `build`, `dist`, `node_modules`, and `vendor`.
5. Confirm the minimum Go version from the development environment before creating `go.mod`.
6. Keep the initial source layout minimal and add files only when their responsibility is clear.

Do not resolve these items by inventing environment details or adding speculative architecture.

## Repository-Specific Knowledge

When Agent Tools Runner is used with another repository, the LLM should inspect that repository's `AGENTS.md` when one exists.

Repository instructions guide the LLM's planning and code generation. They do not replace the runtime's safety checks.

Language-specific conventions, such as the DCService Java and Quarkus conventions, belong in the target repository's own `AGENTS.md`. They are not general Go coding rules for Agent Tools Runner.
