# Codebase Flow Mapper — Sprint Backlog and Technical Task Breakdown

**Version:** 1.0  
**Status:** Proposed execution plan  
**Planning assumption:** 2-week sprints; one small team (2–4 engineers). Re-estimate after Sprint 0.  
**MVP languages:** Go + TypeScript  
**Product mode:** local-first static analysis

## 1. How to execute this backlog

- Work top-to-bottom within each sprint unless a dependency is explicitly satisfied.
- Each task must be a separate PR or a clearly isolated commit series.
- Do not start a task until its dependency tasks are merged or an approved interface stub exists.
- AI coding agents must read `AGENTS.md`, this backlog, the relevant ADR, `ARCHITECTURE.md`, and `GRAPH_SCHEMA.md` before editing.
- Never mark a task done based only on code generation. Run the listed tests and report exact commands/results.
- Keep PRs narrow. Do not ask an agent to implement an entire sprint in one prompt.
- `P0` blocks MVP; `P1` is important but can follow the first usable vertical slice.

## 2. Milestones

| Milestone | Target | Exit condition |
|---|---|---|
| M0 — Contracts | End Sprint 1 | Graph schema, API DTOs, worker protocol, repo skeleton and fixtures approved |
| M1 — First Go vertical slice | End Sprint 2 | One Go HTTP endpoint can be mapped through handler and direct calls to a repository function and displayed |
| M2 — Graph exploration | End Sprint 3 | Search, expansion, reverse traversal, path query and source locations work |
| M3 — TypeScript parity | End Sprint 4 | Supported TS project routes/calls produce canonical graph and display |
| M4 — MVP hardening | End Sprint 5 | Security, performance, export, diagnostics, packaging and release checks pass |

## 3. Sprint 0 — Project bootstrap and contract freeze

### CFM-0001 — Create monorepo skeleton
- **Priority:** P0
- **Owner:** Platform
- **Dependencies:** None
- **Target:** root, `cmd/cfm/`, `internal/`, `web/`, `workers/typescript/`, `schema/`, `testdata/`
- **Implementation:** Initialize Go module, frontend workspace, TypeScript worker package, lint/test scripts, Makefile or task runner. Add version policy and license decision placeholder.
- **Acceptance criteria:**
  - [ ] `go test ./...` runs successfully on clean checkout.
  - [ ] Frontend typecheck and test commands exist and run.
  - [ ] Worker typecheck and test commands exist and run.
  - [ ] No generated build output is committed.
- **Tests:** clean-checkout bootstrap CI.

### CFM-0002 — Publish machine-readable graph schema
- **Priority:** P0
- **Dependencies:** CFM-0001
- **Target:** `schema/graph.schema.json`, `internal/graph/model/`, `GRAPH_SCHEMA.md`
- **Implementation:** Convert the documented v1 graph contract to JSON Schema Draft 2020-12. Generate or validate Go structs against the same contract. Add schema version constants.
- **Acceptance criteria:**
  - [ ] Valid example graph passes schema validation.
  - [ ] Missing required fields, invalid enum values, and malformed source ranges fail validation.
  - [ ] Go model round-trips the canonical example without semantic loss.
  - [ ] Schema version is present in exports and API graph responses.
- **Tests:** schema positive/negative fixtures; Go JSON round-trip.

### CFM-0003 — Define adapter result and worker protocol
- **Priority:** P0
- **Dependencies:** CFM-0002
- **Target:** `schema/adapter-result.schema.json`, `schema/worker-protocol.schema.json`, `internal/analysis/adapters/`, `workers/typescript/src/protocol/`
- **Implementation:** Specify adapter metadata, partial-result semantics, diagnostics, NDJSON request/response envelopes, protocol versioning, and error shape.
- **Acceptance criteria:**
  - [ ] Protocol request and response examples validate against schema.
  - [ ] Unknown protocol version is rejected with a structured error.
  - [ ] Stdout carries protocol JSON only; worker logs use stderr.
  - [ ] Partial analysis is representable without fabricating missing nodes.
- **Tests:** protocol fixtures, malformed line, worker crash, unknown operation.

### CFM-0004 — Create language conformance fixtures
- **Priority:** P0
- **Dependencies:** CFM-0002
- **Target:** `testdata/go/`, `workers/typescript/src/fixtures/`
- **Implementation:** Add small, deterministic fixtures for direct calls, methods, branch, switch, loop, returns, unresolved calls, route registration, middleware, and repository calls.
- **Acceptance criteria:**
  - [ ] Every fixture has source files and an expected graph assertion file.
  - [ ] Fixtures include at least one intentionally unresolved relationship.
  - [ ] Tests assert semantic node/edge properties, not unstable generated IDs only.
- **Tests:** fixture manifest validation.

## 4. Sprint 1 — Runtime shell, project scanning, and job lifecycle

### CFM-0101 — Implement CLI and application lifecycle
- **Priority:** P0
- **Dependencies:** CFM-0001
- **Target:** `cmd/cfm/main.go`, `internal/app/`, `internal/config/`
- **Implementation:** Flags: `--project`, `--addr` (loopback only by default), `--port`, `--open-browser`, `--no-browser`, `--data-dir`, `--log-level`. Start/stop HTTP server gracefully.
- **Acceptance criteria:**
  - [ ] `cfm --project /path/to/repo` starts the local app.
  - [ ] Default bind address is `127.0.0.1`.
  - [ ] `--no-browser` prints the URL and does not launch a browser.
  - [ ] SIGINT/SIGTERM shuts down server and workers cleanly.
  - [ ] Invalid or missing project path produces actionable error.
- **Tests:** CLI unit tests; lifecycle integration test.

### CFM-0102 — Implement project registration and safe root validation
- **Priority:** P0
- **Dependencies:** CFM-0101
- **Target:** `internal/project/`, `internal/security/`
- **Implementation:** Canonicalize selected root, check directory access, assign project ID/fingerprint, enforce root boundary and symlink policy.
- **Acceptance criteria:**
  - [ ] Nonexistent path, regular file, and inaccessible directory are rejected.
  - [ ] Symlink resolving outside project root is excluded by default.
  - [ ] Root path is stored only in local metadata and omitted from graph export.
  - [ ] Project identity is stable enough to associate subsequent analysis runs.
- **Tests:** temp-directory tests; symlink escape tests on supported OS.

### CFM-0103 — Build deterministic project scanner
- **Priority:** P0
- **Dependencies:** CFM-0102
- **Target:** `internal/project/scanner.go`, `internal/project/ignore.go`
- **Implementation:** Discover Go and TS manifests/source files; apply default excludes and user include/exclude globs; sort output deterministically; enforce file count and size limits.
- **Acceptance criteria:**
  - [ ] Same tree/config produces same ordered file inventory.
  - [ ] `vendor`, `node_modules`, `.git`, build output, and configured generated directories are excluded by default.
  - [ ] Scanner reports skipped files and reasons.
  - [ ] Oversized and binary files are skipped with diagnostics.
- **Tests:** scanner fixtures, ignore precedence, limits.

### CFM-0104 — Implement analysis job state machine
- **Priority:** P0
- **Dependencies:** CFM-0101
- **Target:** `internal/analysis/jobs/`
- **Implementation:** Job states: queued, running, cancelling, completed, completed_with_warnings, failed, cancelled. Add cancellation context and stage progress.
- **Acceptance criteria:**
  - [ ] Invalid state transitions are rejected.
  - [ ] Cancellation is observable and idempotent.
  - [ ] Job state and timestamps survive application restart.
  - [ ] Long-running analysis does not block HTTP request handlers.
- **Tests:** state transition table tests; persistence integration.

### CFM-0105 — Implement SQLite metadata repository
- **Priority:** P0
- **Dependencies:** CFM-0104
- **Target:** `internal/persistence/sqlite/`
- **Implementation:** Migrations for projects, analyses, diagnostics, settings. Use parameterized queries and transaction boundaries.
- **Acceptance criteria:**
  - [ ] Migrations are repeatable and versioned.
  - [ ] Project and analysis metadata can be created/read/updated.
  - [ ] Latest successful analysis can be queried.
  - [ ] Database corruption/open errors produce actionable startup errors.
- **Tests:** repository tests against temporary SQLite DB.

## 5. Sprint 2 — Go analyzer and first vertical slice

### CFM-0201 — Implement Go package loader and diagnostics
- **Priority:** P0
- **Dependencies:** CFM-0004, CFM-0103
- **Target:** `internal/analysis/goanalyzer/loader.go`
- **Implementation:** Load Go packages using `go/packages` with explicit mode/env/build flags. Record package errors, Go version, partial loading.
- **Acceptance criteria:**
  - [ ] Valid Go module loads and reports package count.
  - [ ] Missing dependencies/build failures produce diagnostics rather than panic.
  - [ ] Build tags and workspace mode are surfaced in analysis metadata.
  - [ ] No `go get`, `go mod tidy`, or source mutation is performed.
- **Tests:** valid module, missing module, build tags, workspace fixtures.

### CFM-0202 — Index Go declarations and source locations
- **Priority:** P0
- **Dependencies:** CFM-0201
- **Target:** `internal/analysis/goanalyzer/symbols.go`
- **Implementation:** Index packages, files, functions, methods, receivers, types, and source ranges; derive deterministic symbol IDs.
- **Acceptance criteria:**
  - [ ] Functions and methods have stable IDs across unchanged runs.
  - [ ] Source ranges are 1-based and project-relative.
  - [ ] Duplicate names in different packages remain distinct.
  - [ ] Generated files are tagged or excluded according to scanner policy.
- **Tests:** symbol index fixtures.

### CFM-0203 — Implement Go direct-call resolution
- **Priority:** P0
- **Dependencies:** CFM-0202
- **Target:** `internal/analysis/goanalyzer/calls.go`
- **Implementation:** Resolve direct function calls and statically resolvable method calls using type information; emit unresolved/ambiguous references with evidence.
- **Acceptance criteria:**
  - [ ] Direct same-package and imported calls resolve.
  - [ ] Method calls resolve when receiver/type information is available.
  - [ ] Unresolved external/dynamic calls remain explicit diagnostics or unresolved nodes per schema.
  - [ ] No relationship is marked resolved solely from matching function names.
- **Tests:** direct/imported/method/ambiguous fixtures.

### CFM-0204 — Extract Go control-flow constructs
- **Priority:** P0
- **Dependencies:** CFM-0202
- **Target:** `internal/analysis/goanalyzer/controlflow.go`
- **Implementation:** Represent if/else, switch/type switch, for/range loops, return, panic, and supported early exits as control-flow nodes/edges according to schema.
- **Acceptance criteria:**
  - [ ] Branch alternatives are distinct and labeled where condition text is safe.
  - [ ] Loop back-edges are explicit and cycle-safe.
  - [ ] Return/terminal nodes are distinguishable from ordinary calls.
  - [ ] Unsupported constructs generate diagnostics.
- **Tests:** control-flow fixture assertions.

### CFM-0205 — Detect Go HTTP endpoints and middleware
- **Priority:** P0
- **Dependencies:** CFM-0201, CFM-0202
- **Target:** `internal/analysis/goanalyzer/frameworks/`
- **Implementation:** Implement recognizers for `net/http`, Gin, Echo, Fiber, Chi. Normalize routes and registration-to-handler evidence.
- **Acceptance criteria:**
  - [ ] Each supported framework fixture discovers expected method/path/handler.
  - [ ] Middleware/handler ordering is preserved when statically known.
  - [ ] Group/prefix composition is represented where supported.
  - [ ] Unknown patterns are not guessed; diagnostics identify unsupported registration.
- **Tests:** one or more fixtures per framework, route nesting and middleware tests.

### CFM-0206 — Normalize and validate Go graph fragment
- **Priority:** P0
- **Dependencies:** CFM-0002, CFM-0203, CFM-0204, CFM-0205
- **Target:** `internal/analysis/normalize/`, `internal/graph/validate/`
- **Implementation:** Map Go-specific findings to canonical nodes/edges; merge evidence; validate references, enums, ranges, and stable IDs.
- **Acceptance criteria:**
  - [ ] Go fixture graph passes JSON Schema and semantic validation.
  - [ ] All edges reference existing nodes or schema-defined unresolved targets.
  - [ ] Every resolved call edge includes source evidence.
  - [ ] Diagnostics survive normalization.
- **Tests:** graph conformance suite.

### CFM-0207 — Implement snapshot persistence and first endpoint-to-repository graph
- **Priority:** P0
- **Dependencies:** CFM-0105, CFM-0206
- **Target:** `internal/persistence/snapshots/`, `internal/analysis/pipeline/`
- **Implementation:** Persist graph atomically and link snapshot metadata to analysis job. Create one end-to-end Go fixture: route → handler → service → repository.
- **Acceptance criteria:**
  - [ ] Snapshot is readable after process restart.
  - [ ] Failed write does not publish a partial snapshot.
  - [ ] One fixture yields endpoint-to-repository path with evidence at each edge.
  - [ ] Partial adapter result can complete with warnings.
- **Tests:** end-to-end pipeline and atomic-write failure simulation.

## 6. Sprint 3 — Graph engine, API, and visualization

### CFM-0301 — Implement graph traversal primitives
- **Priority:** P0
- **Dependencies:** CFM-0207
- **Target:** `internal/graph/query/`
- **Implementation:** Forward/reverse/bidirectional bounded traversal, cycle detection, path search, max-depth/node/edge guards.
- **Acceptance criteria:**
  - [ ] Traversal returns deterministic results.
  - [ ] Cycles terminate and are represented without infinite expansion.
  - [ ] Depth and size limits are enforced server-side.
  - [ ] Relationship filters can distinguish call and control-flow edges.
- **Tests:** graph algorithm fixtures including cycles and diamonds.

### CFM-0302 — Implement local HTTP API and security middleware
- **Priority:** P0
- **Dependencies:** CFM-0105, CFM-0207
- **Target:** `internal/httpapi/`, `internal/security/`
- **Implementation:** Implement health, projects, analyses, graph, diagnostics, and export endpoints. Add local token/session, Host/Origin checks, request IDs, validation, error envelope.
- **Acceptance criteria:**
  - [ ] API contract matches `ARCHITECTURE.md`.
  - [ ] State-changing requests without valid local session/token are rejected.
  - [ ] Invalid Host/Origin is rejected.
  - [ ] Long-running analysis returns 202 and job ID.
  - [ ] API never accepts arbitrary filesystem paths except project registration.
- **Tests:** handler integration, auth/origin/path traversal tests.

### CFM-0303 — Build frontend shell and project workflow
- **Priority:** P0
- **Dependencies:** CFM-0302
- **Target:** `web/src/app/`, `web/src/features/projects/`, `web/src/api/`
- **Implementation:** Project selection/registration, analysis start, progress and failure state, analysis history.
- **Acceptance criteria:**
  - [ ] User can register a project path through the local app workflow.
  - [ ] User can start and cancel analysis.
  - [ ] UI distinguishes completed, completed-with-warnings, failed, and cancelled.
  - [ ] API errors are shown with actionable messages.
- **Tests:** component tests and browser-level happy path.

### CFM-0304 — Implement graph search and node detail
- **Priority:** P0
- **Dependencies:** CFM-0302, CFM-0303
- **Target:** `web/src/features/graph/`
- **Implementation:** Search endpoints/functions, show node metadata, evidence, confidence, resolution, diagnostics, source location.
- **Acceptance criteria:**
  - [ ] Search supports endpoint path, symbol name, and qualified name.
  - [ ] Selecting a result opens node details.
  - [ ] Evidence and confidence are visible and not hidden behind AI text.
  - [ ] Empty and no-match states are explicit.
- **Tests:** API query tests and UI component tests.

### CFM-0305 — Implement Flowchart view
- **Priority:** P0
- **Dependencies:** CFM-0301, CFM-0304
- **Target:** `web/src/features/flowchart/`
- **Implementation:** React Flow nodes/edges; ELK layout; branch, loop, call, return styling; progressive expansion.
- **Acceptance criteria:**
  - [ ] Flowchart uses canonical graph facts only.
  - [ ] Branches and loops are visually distinguishable.
  - [ ] Edge type and confidence can be inspected.
  - [ ] Expand/collapse does not duplicate nodes or lose graph identity.
  - [ ] Large result is bounded and displays truncation notice.
- **Tests:** view-model tests and browser interaction test.

### CFM-0306 — Implement Mind Map view
- **Priority:** P0
- **Dependencies:** CFM-0301, CFM-0304
- **Target:** `web/src/features/mindmap/`
- **Implementation:** Hierarchical projection rooted at endpoint or selected function; group by middleware/handler/service/repository where evidence supports it.
- **Acceptance criteria:**
  - [ ] Mind Map uses the same canonical graph as Flowchart.
  - [ ] Shared callees are not falsely duplicated as separate canonical entities.
  - [ ] User can expand/collapse branches and inspect source evidence.
  - [ ] Cycles are indicated and do not create infinite hierarchy.
- **Tests:** projection tests for shared callees and cycles.

### CFM-0307 — Implement source navigation
- **Priority:** P0
- **Dependencies:** CFM-0302, CFM-0304
- **Target:** `internal/httpapi/`, `web/src/features/source/`
- **Implementation:** Open local file location through a safe source-preview API and line/column anchors. Do not expose arbitrary file read.
- **Acceptance criteria:**
  - [ ] Clicking evidence opens the correct project-relative file and line.
  - [ ] Path traversal and symlink escape attempts fail.
  - [ ] Source is rendered as escaped text.
  - [ ] Missing/deleted file produces a clear stale-source message.
- **Tests:** API security and UI source panel tests.

## 7. Sprint 4 — TypeScript analyzer parity

### CFM-0401 — Build Node worker process manager
- **Priority:** P0
- **Dependencies:** CFM-0003, CFM-0104
- **Target:** `internal/analysis/tsworker/`
- **Implementation:** Launch packaged worker with fixed executable/args, bounded environment, stdin/stdout pipes, stderr logging, timeout and cancellation.
- **Acceptance criteria:**
  - [ ] Worker handshake validates protocol version.
  - [ ] Worker stdout is parsed as NDJSON only.
  - [ ] Timeout, crash, malformed JSON, and cancellation become structured failures.
  - [ ] Project-controlled values are never interpolated into shell commands.
- **Tests:** fake worker tests for each failure mode.

### CFM-0402 — Implement TS project discovery and Program creation
- **Priority:** P0
- **Dependencies:** CFM-0003, CFM-0103, CFM-0401
- **Target:** `workers/typescript/src/project/`
- **Implementation:** Locate tsconfig files, parse compiler options, create TypeScript Program with pinned compiler, avoid runtime imports/execution.
- **Acceptance criteria:**
  - [ ] Single and multi-tsconfig projects are represented.
  - [ ] Compiler diagnostics are preserved.
  - [ ] Path aliases and project references are handled where supported.
  - [ ] No project scripts or application modules are executed.
- **Tests:** tsconfig fixtures, path aliases, project references, invalid config.

### CFM-0403 — Index TS declarations and resolve calls
- **Priority:** P0
- **Dependencies:** CFM-0402
- **Target:** `workers/typescript/src/analyzer/symbols.ts`, `calls.ts`
- **Implementation:** Index functions, methods, classes, imports/exports; resolve call expressions through TypeChecker symbols; represent unresolved dynamic calls.
- **Acceptance criteria:**
  - [ ] Direct local/imported function calls resolve.
  - [ ] Class methods and supported inherited methods resolve.
  - [ ] Overloads and aliases do not create duplicate false symbols.
  - [ ] Dynamic `any`, reflection, and unresolved module calls are marked uncertain.
- **Tests:** symbol/call fixtures.

### CFM-0404 — Extract TS control flow
- **Priority:** P0
- **Dependencies:** CFM-0402
- **Target:** `workers/typescript/src/analyzer/controlFlow.ts`
- **Implementation:** Extract if/conditional, switch, loops, try/catch/finally, return, throw, and await/call relationships supported by schema.
- **Acceptance criteria:**
  - [ ] Branches, loop back-edges, returns, and throws are distinct.
  - [ ] Async call relationship is represented without claiming runtime ordering beyond static semantics.
  - [ ] Unsupported syntax is diagnosed.
- **Tests:** control-flow fixture suite.

### CFM-0405 — Detect Express, Fastify, and NestJS routes
- **Priority:** P0
- **Dependencies:** CFM-0402, CFM-0403
- **Target:** `workers/typescript/src/analyzer/frameworks/`
- **Implementation:** Route recognizers for Express, Fastify, NestJS decorators/controllers; map registration to handler and middleware where statically known.
- **Acceptance criteria:**
  - [ ] Framework fixture routes are discovered with HTTP method/path.
  - [ ] NestJS controller prefix + method route composition works for supported patterns.
  - [ ] Middleware/guards/interceptors are not conflated with ordinary calls.
  - [ ] Unrecognized dynamic route registration produces diagnostics.
- **Tests:** one fixture per framework plus nested route/controller tests.

### CFM-0406 — Normalize TS results and run cross-language conformance
- **Priority:** P0
- **Dependencies:** CFM-0002, CFM-0403, CFM-0404, CFM-0405
- **Target:** `workers/typescript/src/`, `internal/analysis/normalize/`
- **Implementation:** Emit canonical adapter result and compare semantic output to expected fixtures.
- **Acceptance criteria:**
  - [ ] TS output passes the same graph schema validator as Go.
  - [ ] Equivalent Go/TS fixture patterns map to equivalent canonical node/edge semantics.
  - [ ] Evidence locations remain language-specific but conform to shared source schema.
  - [ ] Worker partial results and diagnostics are preserved.
- **Tests:** adapter conformance suite and Go-worker integration.

## 8. Sprint 5 — Hardening, export, packaging, and MVP release

### CFM-0501 — Add export and path-redaction policy
- **Priority:** P0
- **Dependencies:** CFM-0302, CFM-0207
- **Target:** `internal/export/`, `web/src/features/`
- **Implementation:** Export schema-versioned JSON, diagnostics, summary, and optional absolute-path opt-in.
- **Acceptance criteria:**
  - [ ] Export validates against published schema.
  - [ ] Default export contains project-relative paths only.
  - [ ] Explicit absolute-path export requires visible user confirmation.
  - [ ] Export is deterministic for unchanged graph snapshot.
- **Tests:** export golden tests and secret/path scan.

### CFM-0502 — Add security and adversarial repository tests
- **Priority:** P0
- **Dependencies:** CFM-0302, CFM-0307, CFM-0401
- **Target:** `internal/security/`, integration tests
- **Implementation:** Test symlink escape, path traversal, malicious filenames, oversized files, hostile source strings, malformed worker output, browser-origin/host attacks.
- **Acceptance criteria:**
  - [ ] All listed attacks are blocked or safely bounded.
  - [ ] No source content is executed or rendered as HTML.
  - [ ] No network listener is exposed beyond configured loopback.
  - [ ] Security failures are diagnosable without leaking secrets.
- **Tests:** security regression suite.

### CFM-0503 — Add performance benchmarks and bounded graph UX
- **Priority:** P1
- **Dependencies:** CFM-0301, CFM-0305, CFM-0306
- **Target:** `internal/graph/query/`, `web/src/features/graph/`
- **Implementation:** Benchmark graph traversal, snapshot load, and view projection; add truncation and loading states.
- **Acceptance criteria:**
  - [ ] Benchmarks run on deterministic synthetic graph fixtures.
  - [ ] Query limits are enforced under stress.
  - [ ] UI remains responsive by rendering bounded subgraphs.
  - [ ] Limit-induced truncation is visible and recoverable by narrowing/expanding.
- **Tests:** Go benchmarks, UI performance smoke tests.

### CFM-0504 — Implement diagnostics center and remediation guidance
- **Priority:** P1
- **Dependencies:** CFM-0206, CFM-0304, CFM-0406
- **Target:** `internal/diagnostics/`, `web/src/features/analysis/`
- **Implementation:** Aggregate diagnostics by severity, adapter, file, and code; show remediation hint and impact on completeness.
- **Acceptance criteria:**
  - [ ] User can inspect all diagnostics for an analysis.
  - [ ] Diagnostics link to source when location exists.
  - [ ] Partial analysis is distinguished from total failure.
  - [ ] No diagnostic is silently dropped between adapter and UI.
- **Tests:** diagnostic pipeline tests.

### CFM-0505 — Add release packaging and local install documentation
- **Priority:** P0
- **Dependencies:** CFM-0101, CFM-0401, CFM-0303
- **Target:** build scripts, CI, `README.md`
- **Implementation:** Embed frontend, package TS worker/runtime dependencies, build supported OS artifacts, checksum and SBOM.
- **Acceptance criteria:**
  - [ ] Clean machine can run packaged binary using documented steps.
  - [ ] Go-only project analysis does not require Node runtime if worker is not invoked (or bundled runtime is available according to release policy).
  - [ ] TS analysis reports a clear prerequisite/runtime error if unavailable.
  - [ ] Build artifacts include version metadata and checksums.
- **Tests:** smoke test each supported release platform.

### CFM-0506 — MVP release gate and reproducibility report
- **Priority:** P0
- **Dependencies:** all P0 tasks
- **Target:** `docs/release/`, CI
- **Implementation:** Run full fixture matrix, security suite, schema validation, install smoke tests, and produce known-limitations report.
- **Acceptance criteria:**
  - [ ] Every P0 acceptance criterion is met or explicitly waived.
  - [ ] All supported framework fixtures pass.
  - [ ] No known critical/high security issue remains open.
  - [ ] Release notes list limitations and supported versions.
  - [ ] Analysis output is reproducible for unchanged source/config/toolchain.
- **Tests:** full CI release pipeline.

## 9. Cross-sprint engineering rules

- Graph schema changes require a schema versioning decision and fixture updates.
- Any new edge type requires updates to schema, validators, traversal filters, both visualizations, and tests.
- Any framework recognizer requires positive and negative fixtures.
- Any new analyzer capability must expose uncertainty through diagnostics/confidence.
- Never use function-name string matching as the only evidence for a resolved call.
- Never serialize absolute paths into portable graph exports by default.
- Do not add AI-generated nodes or edges to canonical graph facts.
- Do not silently fall back from semantic resolution to a stronger confidence label.

## 10. Definition of Done (per task)

A task is done only when:
1. Implementation matches the task scope and relevant contracts.
2. Unit tests cover normal, edge, and failure cases.
3. Integration/fixture tests are updated where relevant.
4. Lint, format, typecheck, and tests pass.
5. Diagnostics and logs do not expose secrets or unnecessary absolute paths.
6. Documentation and examples are updated.
7. The PR describes behavior, limitations, test commands, and any schema/API impact.
8. No unrelated refactor or dependency upgrade is bundled into the task.
