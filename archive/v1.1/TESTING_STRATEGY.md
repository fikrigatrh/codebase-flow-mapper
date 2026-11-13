# Codebase Flow Mapper — Testing Strategy

**Version:** 1.0  
**Status:** MVP baseline

## 1. Quality goals

Testing must prove that CFM:
1. produces schema-valid, source-grounded graph facts;
2. preserves uncertainty instead of inventing relationships;
3. behaves consistently across Go and TypeScript adapters;
4. bounds resource use on large or malformed repositories;
5. protects local filesystem and browser/API boundaries;
6. renders both views from one canonical graph without changing graph semantics.

## 2. Test pyramid

| Layer | Purpose | Examples |
|---|---|---|
| Unit | Pure functions and small components | stable IDs, path normalization, edge filters, branch extraction |
| Contract | Shared schema/protocol compatibility | JSON Schema, NDJSON worker envelope, adapter output |
| Fixture/integration | Real source patterns end-to-end | Go route → handler → service → repo |
| API/UI | User workflows and API boundaries | register project, start analysis, expand graph, source navigation |
| Security | Adversarial inputs and boundary checks | symlink escape, path traversal, hostile source text |
| Performance | Bounded behavior | large graph traversal, many files, worker timeout |

## 3. Golden fixture matrix

### Go fixtures
- Direct same-package function call.
- Imported function call.
- Method call with concrete receiver.
- Interface method call (resolved, ambiguous, or unresolved according to analyzer capability).
- Generic function and instantiated call.
- `if/else`, `switch`, `for`, `range`, `return`, `panic`.
- `net/http`, Gin, Echo, Fiber, Chi routes.
- Middleware and nested route groups.
- Build tags, workspace, generated source, missing dependency.
- Recursive calls and shared callees.

### TypeScript fixtures
- Local and imported function calls.
- Class methods, inheritance, overloads, aliases.
- `async`/`await`, promises, try/catch/finally.
- if/conditional, switch, loops, return, throw.
- Express, Fastify, NestJS controllers/decorators.
- Path aliases and multiple tsconfig projects.
- Dynamic import, `any`, unresolved package, generated declaration.
- Dependency injection and decorator patterns with partial-resolution expectations.

Each fixture should contain:
- source files;
- a fixture manifest declaring language/framework/toolchain assumptions;
- expected semantic assertions;
- expected diagnostics where uncertainty is intentional.

Avoid full serialized golden snapshots when generated IDs include irrelevant implementation details. Prefer assertions on node types, edge types, endpoints, evidence, and resolution status.

## 4. Contract tests

- Validate all graph examples against `schema/graph.schema.json`.
- Validate adapter result and worker protocol messages against their schemas.
- Round-trip canonical graph JSON through Go structs without dropping fields.
- Verify unknown schema/protocol versions fail clearly.
- Verify every edge endpoint exists or uses the schema-defined unresolved-target representation.
- Verify every resolved call edge includes evidence.
- Verify project-relative source paths and 1-based locations.
- Run the same semantic assertions for Go and TS patterns where the source patterns are equivalent.

## 5. Graph algorithm tests

Required graph shapes:
- linear chain;
- branching tree;
- diamond/shared callee;
- direct cycle;
- mutual recursion;
- disconnected components;
- unresolved target;
- branch + loop control-flow graph;
- very high fan-out node.

Assertions:
- traversal is deterministic;
- forward and reverse traversals agree with edge direction;
- cycles terminate;
- path search respects max paths/depth;
- node/edge limits are enforced;
- filtering call edges versus control-flow edges works;
- Mind Map projection does not mutate canonical graph;
- Flowchart projection preserves branch and loop semantics.

## 6. API and UI workflow tests

Minimum happy path:
1. Start CFM on loopback.
2. Register fixture project.
3. Start analysis and receive `202`.
4. Poll job until completed or completed-with-warnings.
5. Load graph summary.
6. Search endpoint and function.
7. Expand endpoint to handler and downstream calls.
8. Switch Mind Map / Flowchart and verify same node identities.
9. Inspect evidence and confidence.
10. Open source location.
11. Export graph and validate schema.

Negative workflows:
- invalid project root;
- cancellation during analysis;
- malformed or oversized project file;
- missing compiler/toolchain;
- worker crash or timeout;
- deleted source file after analysis;
- no endpoints discovered;
- partial package load.

## 7. Security test matrix

| Threat | Test | Expected result |
|---|---|---|
| Symlink escape | Project file symlink points outside root | Excluded/rejected by default |
| Path traversal | `../../etc/passwd` in source endpoint | Rejected |
| Host header attack | Unexpected Host | Request rejected |
| Cross-origin mutation | Untrusted Origin POST | Request rejected |
| Secret leakage | `.env` and key fixtures | Not indexed/exported by default |
| XSS | Source contains script/HTML payload | Displayed as escaped text |
| Shell injection | Project path contains shell metacharacters | No shell interpretation |
| Worker protocol injection | Malformed stdout lines | Worker result rejected safely |
| Resource exhaustion | Huge file / many files / deep graph | Bounded and diagnostic |
| Snapshot corruption | Invalid JSON snapshot | Error surfaced; app remains recoverable |

Security tests should run in CI on supported operating systems where filesystem semantics differ.

## 8. Performance tests

Create deterministic synthetic projects/graphs at several sizes (for example 1k, 10k, 100k nodes). Measure:
- scanner throughput;
- Go package load and analysis duration;
- TS Program creation and analysis duration;
- graph normalization/validation;
- snapshot write/read;
- traversal and path search;
- view-model projection and UI expansion.

Do not set hard latency SLAs until baseline measurements exist on documented reference hardware. Initially enforce memory/time ceilings and detect major regressions relative to CI baseline.

## 9. CI gates

Required on every pull request:
- Go format and lint;
- Go unit tests;
- frontend format/lint/typecheck/tests;
- worker format/lint/typecheck/tests;
- schema validation;
- fixture tests affected by the change.

Required before release:
- full integration suite;
- security suite;
- race tests where supported;
- performance smoke tests;
- clean install/package smoke test;
- supported OS matrix;
- dependency/license and vulnerability review.

## 10. Test data and privacy

- Use synthetic repositories only; never commit proprietary source code or credentials.
- Keep fixture size small and deterministic.
- Avoid network-dependent tests in core CI.
- Do not send test source files to external AI services.
- Redact local absolute paths from golden outputs unless a test specifically validates local metadata.
