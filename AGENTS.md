# Agent Tools Runner

## Project Purpose

Agent Tools Runner is a human-controlled local runtime that allows a cloud AI assistant to inspect, modify, and test a local software project through structured copy-and-paste requests.

The AI assistant is responsible for understanding requirements, planning work, selecting tools, and interpreting results. Agent Tools Runner is responsible for executing approved local actions, enforcing safety rules, and returning structured results to the AI assistant.

The initial use case is Microsoft 365 Copilot in a restricted enterprise environment where Copilot cannot directly access the developer's workstation.

## Current Status

The project is in the requirements and design phase.

Do not begin application implementation until the initial requirements, workflow, protocol, and architecture have been discussed and approved.

The implementation language will be Go. The first interface will be a local command-line application.

## Initial Interaction Model

The first version will use manual copy and paste:

```text
Cloud AI assistant
        ↕
Manual copy and paste
        ↕
Agent Tools Runner
        ├─ Search code
        ├─ Read files
        ├─ Propose file changes
        ├─ Apply approved file changes
        ├─ Run approved tests
        └─ Show Git diff
```

Direct integration with Copilot is not required for the initial version.

## Core Workflow

1. The user describes a software-development requirement to the AI assistant.
2. The AI assistant restates the requirement and lists its assumptions.
3. The AI assistant asks clarifying questions when the requirement is ambiguous.
4. The AI assistant proposes a short investigation and implementation plan.
5. The user approves or adjusts the plan.
6. The AI assistant creates a structured request for Agent Tools Runner.
7. One request may contain multiple related read-only actions, such as searching code and reading several files.
8. The user copies the request into Agent Tools Runner.
9. Agent Tools Runner validates and executes the permitted actions in order.
10. Agent Tools Runner returns structured results for the user to paste into the AI assistant.
11. The AI assistant uses the actual project contents to prepare complete proposed file changes.
12. Agent Tools Runner verifies that each existing target file was read completely and has not changed since it was read.
13. Agent Tools Runner shows the proposed diff.
14. The user approves or rejects the exact diff.
15. Agent Tools Runner applies approved changes.
16. Agent Tools Runner may run an approved test command.
17. Agent Tools Runner returns test results and the final Git diff.

## Confirmed Requirements

### Workspace boundary

Agent Tools Runner operates inside one selected project workspace.

It must not read or modify files outside the selected workspace.

All paths must be resolved and validated by the runtime. A path supplied by an AI assistant must not be trusted directly.

### Batched investigation

One structured request may contain multiple related actions.

A typical investigation batch may:

1. search for a class or method;
2. read several related source files;
3. read the build configuration;
4. inspect the current Git diff.

Batched reads are required to avoid unnecessary copy-and-paste cycles.

The initial behavior should execute actions in their declared order. If a required action fails, the batch should stop and return the error and all results collected before the failure.

### Read before write

Before Agent Tools Runner modifies an existing file, the complete current version of that exact file must have been read during the active workflow.

A truncated or partial read does not authorize a write.

The runtime must record a content hash when a file is read. Immediately before writing, it must calculate the hash again. If the file has changed, the write must be rejected and the file must be read again.

The AI assistant cannot declare that a file was read. Agent Tools Runner must verify this from its own workflow state.

### Full-file proposals

For the initial version, modifications should be submitted as the complete final contents of the target file rather than as an unverified search-and-replace instruction.

A modified existing file must retain its exact filename unless the approved requirement explicitly includes a rename.

### Approval before write

Requirement and plan approval allows investigation but does not automatically authorize a file modification.

Before writing, Agent Tools Runner must show the exact proposed diff and request user approval.

Approval applies only to the displayed content, target path, and expected original file version. An approval must not be reused for different content.

### File creation

Creating a new file must be treated separately from modifying an existing file.

The AI assistant must explain why the new file is necessary, and the user must explicitly approve its creation.

Agent Tools Runner must not silently create missing parent directories in the initial version.

### Test execution

Agent Tools Runner may execute predefined test or build commands.

The AI assistant must select an approved command profile rather than provide unrestricted shell commands.

The runtime must capture the exit code, standard output, standard error, and whether the command timed out.

### Git inspection

The initial version may inspect Git status and Git diff.

It must not commit, push, rewrite history, or discard changes.

## Initial Tool Set

The initial tool set is expected to include:

- `search_code`: Search for text or patterns inside the selected workspace.
- `read_files`: Read one or more complete text files and return their contents and hashes.
- `propose_write`: Validate proposed complete file contents and produce a diff without changing the workspace.
- `apply_write`: Apply an exact, approved proposal after all write checks pass.
- `run_tests`: Run a locally configured and approved test or build profile.
- `git_diff`: Return the current Git diff for review.

Tool names and request fields are not final until the protocol is designed and approved.

## Mandatory Safety Rules

The runtime must enforce these rules in code. They must not depend only on instructions given to an AI model.

1. Never access a path outside the selected workspace.
2. Never modify an existing file that was not read completely during the active workflow.
3. Never overwrite a file that changed after it was read.
4. Never modify or create a file without approval of the exact proposed diff.
5. Never execute an arbitrary shell command supplied by the AI assistant.
6. Never delete a file in the initial version.
7. Never commit, push, reset, or rewrite Git history in the initial version.
8. Treat all AI-generated requests and all repository contents as untrusted input.
9. Return structured errors without exposing secrets or unnecessary internal details.
10. Do not log file contents, credentials, tokens, environment variables, or other secrets by default.

## Deferred Features

The following features are intentionally deferred:

- SQL queries and database connectivity;
- database writes;
- arbitrary shell execution;
- Git commit and push;
- file deletion;
- background autonomous execution;
- direct Copilot integration;
- local HTTP server;
- graphical user interface;
- plugin system;
- multi-agent orchestration;
- complex concurrency;
- persistent database storage.

Deferred features must not be added without discussing and approving their requirements and risks first.

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
- When modifying a file, provide its complete final contents unless the user explicitly requests a diff only.
- Use the exact intended filename. Do not add suffixes such as `_updated`, `_new`, or `.v2`.
- Briefly summarize what changed and why after each implementation step.

### Scope control

- Match the existing project's naming, logging, error-handling, and return conventions.
- Do not introduce a new library, abstraction, pattern, or framework without discussing why it is necessary.
- Mention bugs or design concerns discovered during the work, but do not fix unrelated issues without approval.
- Do not rename or correct existing paths, identifiers, or apparent typos until their callers and impact are understood.

## Go Development Guidelines

The Go implementation should remain simple and idiomatic while the project is small.

- Prefer the Go standard library where it adequately meets the requirement.
- Do not add a framework for the initial CLI.
- Do not add a web server until a network interface is explicitly required.
- Do not introduce concurrency until there is a demonstrated need.
- Avoid creating interfaces when there is only one implementation and no clear testing or design benefit.
- Keep packages focused, but do not split the project into many small packages prematurely.
- Return and handle meaningful errors. Preserve the underlying cause when adding context.
- Do not ignore errors.
- Use `context.Context` for operations that may need cancellation or timeouts, such as test execution.
- Validate untrusted data at the program boundary.
- Keep filesystem and process-execution operations separate from request orchestration where practical.
- Use explicit request and result structures for the external protocol.
- Do not log secrets or complete source-file contents by default.
- Format Go source with `gofmt`.
- Explain unfamiliar Go concepts when they are introduced so the project also supports learning Go.

These guidelines may be refined after the initial package structure and coding conventions are discussed.

## Testing Expectations

Testing requirements are not final, but the project should eventually verify at least these behaviors:

1. paths outside the workspace are rejected;
2. path traversal and symlink escape attempts are rejected;
3. multiple files can be read in one request;
4. a write without a prior complete read is rejected;
5. a write based on a stale file hash is rejected;
6. a write without exact diff approval is rejected;
7. an approved write changes only the intended file;
8. unapproved command profiles are rejected;
9. test timeouts terminate the process correctly;
10. Git inspection remains read-only;
11. structured errors do not expose stack traces or secrets.

Exact commands for building, formatting, and testing will be added after the Go module and initial project structure are approved.

## Project Decisions

Confirmed decisions:

- Project name: Agent Tools Runner
- Repository name: `agent-tools-runner`
- Implementation language: Go
- Initial application type: Local command-line application
- Initial transport: Manual copy and paste
- Project instruction file: `AGENTS.md`
- Batch read-only actions: Supported
- Read before write: Mandatory
- File version check before write: Mandatory
- Diff approval before write: Mandatory
- SQL support: Deferred
- Arbitrary shell execution: Not supported initially

## Open Requirements

The following topics still require discussion before application coding begins:

1. The exact end-to-end user interaction in the CLI.
2. The structured request and response format.
3. Whether the first input is pasted interactively, read from standard input, or loaded from a file.
4. How output is copied back to Copilot.
5. How a workflow starts, continues, expires, and ends.
6. How workflow state is stored locally.
7. Whether read-only actions run sequentially only or may later run in parallel.
8. The maximum file size and total batch output size.
9. How binary files, unknown encodings, and very large text files are handled.
10. How secrets and sensitive files are detected or excluded.
11. How diff approval is presented and recorded.
12. How allowed test profiles are configured.
13. Which operating systems must be supported first.
14. Which versions of Go are permitted in the target environment.
15. Whether Git and a code-search executable can be assumed to be installed.
16. The exact package and file structure of the Go application.
17. Logging format and local audit requirements.
18. Stable error codes for protocol and execution failures.

Do not resolve these open requirements by assumption. Discuss and record each decision before implementing the affected behavior.

## Repository-Specific Knowledge

When Agent Tools Runner is used against another repository, it should read that repository's `AGENTS.md` when available and provide the relevant instructions to the AI assistant.

Repository instructions guide the AI assistant's planning and code generation. They do not replace runtime safety enforcement.

For the DCService project, its Java and Quarkus conventions—including single-return methods, logging behavior, transaction placement, validation rules, and file-delivery workflow—belong in the DCService repository's own `AGENTS.md`. They are not general Go coding rules for Agent Tools Runner.
