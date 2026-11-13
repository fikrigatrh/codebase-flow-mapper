# Codebase Flow Mapper — Coding Standards

**Version:** 1.0  
**Status:** MVP baseline

## 1. General principles

- Prefer explicit, readable code over clever abstractions.
- Keep language-specific parsing/resolution inside analyzer adapters.
- Keep canonical graph semantics independent from UI and framework details.
- Make uncertainty and partial analysis explicit.
- Avoid hidden network calls, implicit dependency installation, and project-code execution.
- Keep public interfaces small and versioned.
- Use deterministic ordering and stable identifiers wherever output is persisted or tested.

## 2. Go standards

### Structure and errors
- Follow standard Go formatting (`gofmt`) and idiomatic package naming.
- Keep packages cohesive; avoid generic `util`, `common`, or `helpers` packages.
- Accept `context.Context` as the first parameter for operations that can block or traverse large inputs.
- Wrap errors with operation context using `%w`; do not discard causes.
- Use typed/sentinel errors for expected domain failures; map them to stable API diagnostic codes at boundaries.
- Avoid panics for malformed projects, parser errors, worker failures, and user input.
- Do not call `os/exec` with shell command strings. Use fixed executables and argument arrays.

### Concurrency and resources
- Bound worker concurrency and package/file processing.
- Check cancellation between packages/files and inside expensive traversals.
- Close files, pipes, DB rows, and subprocess handles deterministically.
- Avoid shared mutable graph structures across goroutines unless protected.
- Run `go test -race ./...` in CI where practical.

### Types and serialization
- Use explicit JSON tags and stable enum constants.
- Keep API DTOs separate from canonical graph model and database records.
- Do not serialize internal absolute paths in portable exports by default.
- Validate graph invariants before persistence/export.

## 3. TypeScript standards

- Enable `strict` and `noImplicitAny`.
- Use `unknown` at untrusted boundaries and validate before narrowing.
- Avoid `any`; if compiler API types require escape hatches, isolate and comment them.
- Prefer named exports and explicit return types for public functions.
- Keep compiler API traversal visitors small and testable.
- Do not `require()` or dynamically import target project modules.
- Do not evaluate project source or decorators at runtime.
- Keep worker protocol stdout clean; log to stderr only.
- Handle cancellation and worker shutdown explicitly.

## 4. Frontend standards

- Use TypeScript strict mode.
- Separate API DTOs, canonical graph types, and view-model types.
- Avoid deriving graph facts from display labels or node positions.
- Keep React components focused; extract hooks for API/state behavior.
- Use stable IDs for graph nodes and edges.
- Make loading, empty, partial, error, and truncated states explicit.
- Render source text as text, never trusted HTML.
- Ensure keyboard-accessible controls and visible focus states.

## 5. Naming conventions

| Item | Convention | Example |
|---|---|---|
| Go package | short lowercase | `graph`, `project` |
| Go exported type | PascalCase | `AnalysisRequest` |
| Go unexported symbol | camelCase | `stableNodeID` |
| TS file | camelCase or kebab-case, consistent per folder | `controlFlow.ts` |
| TS type/interface | PascalCase | `WorkerResponse` |
| API fields | camelCase | `analysisId` |
| JSON enum | lowercase snake_case or schema-defined literal | `completed_with_warnings` |
| Graph ID | schema-defined deterministic prefix | `symbol:go:...` |
| Diagnostic code | uppercase snake case | `GO_PACKAGE_LOAD_FAILED` |
| Backlog task | `CFM-####` | `CFM-0203` |

## 6. Graph-specific rules

- Node IDs and edge IDs must be deterministic for unchanged source and configuration.
- A node identity must include enough scope to avoid collisions between same-named symbols.
- Edge identity must include source, target, type, and relevant discriminator/evidence policy.
- Source line and column values are 1-based.
- Paths in graph snapshots are project-relative and slash-normalized.
- Every resolved relationship must have evidence.
- Confidence and resolution are separate fields; do not use confidence as a substitute for resolution.
- Do not merge semantically different edge types.
- Preserve multiple evidence locations when multiple registrations/call sites support one relationship.
- Cycles are valid and must be handled by all traversal and projection code.

## 7. Diagnostics and logs

- Diagnostic codes are stable API contracts; document new codes.
- Messages should explain what happened and how to proceed.
- Do not include source contents, secrets, access tokens, or full absolute paths in routine logs.
- Logs should include request/job IDs and stage names.
- Do not swallow errors. If continuing with partial results, emit a diagnostic.
- Avoid logging the same error at every layer; log once at the boundary with context.

## 8. Dependency policy

- Pin direct dependencies and use lockfiles for Node packages.
- Review license, maintenance, transitive dependency footprint, and security advisories before adding dependencies.
- Do not add runtime dependencies solely for convenience if the standard library suffices.
- Do not auto-upgrade parser/compiler toolchains without fixture and schema compatibility tests.
- Do not use dependencies that require cloud accounts or network access for core analysis.

## 9. Required checks before merge

- Formatting passes.
- Static lint/typecheck passes.
- Unit tests pass.
- Relevant integration/fixture tests pass.
- Graph schema validation passes if graph behavior changed.
- Security tests pass if filesystem, API, worker, or rendering behavior changed.
- Documentation and backlog status are updated.
