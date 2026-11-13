# Codebase Flow Mapper --- Architecture Decision Records

**Document:** ADR collection\
**Version:** 1.0\
**Status:** Proposed decisions for MVP

This document records the architectural decisions that shape the first
implementation. Each ADR includes context, decision, alternatives,
consequences, and validation.

------------------------------------------------------------------------

## ADR-001 --- Local-first application architecture

**Status:** Accepted for MVP

### Context

The product analyzes potentially sensitive source code. The core
experience should work without cloud services, account setup, or
mandatory external APIs.

### Decision

Build CFM as a local-first application: - Go local
server/orchestrator. - React web UI served locally. - Default bind
address `127.0.0.1`. - Local SQLite and versioned JSON graph
snapshots. - Optional future Tauri packaging. - No source upload by
default.

### Alternatives considered

-   Cloud-hosted SaaS analyzer.
-   Desktop-only native UI.
-   Browser-only app with no local process.

### Consequences

Positive: - Source remains local. - Works offline for core analysis. -
Easier to distribute a free developer tool. - Local filesystem access is
explicit and controlled.

Negative: - Must secure local HTTP API against browser-origin abuse. -
Installation and OS-specific packaging require work. - Local resource
consumption must be managed.

### Validation

Verify no network calls occur during core scan, confirm loopback
binding, and test path authorization.

------------------------------------------------------------------------

## ADR-002 --- Modular monolith for MVP

**Status:** Accepted

### Context

MVP is a single-user local tool. Microservices would add deployment,
networking, and operational complexity without a demonstrated need.

### Decision

Use a modular Go application with clear internal boundaries: `project`,
`scanner`, `detector`, `analyzer`, `adapters`, `resolver`, `graph`,
`traversal`, `storage`, `ai`, and `api`.

The frontend is a separate React package in a monorepo. Analyzer
processes may be isolated as subprocesses later if resource isolation or
language tooling requires it.

### Alternatives considered

-   Microservices from day one.
-   One unstructured backend package.
-   Separate distributed analyzer services per language.

### Consequences

Positive: - Simple local deployment. - Clear module contracts. - Easier
integration testing. - Language adapters can evolve independently.

Negative: - Requires discipline to prevent package coupling. - A
long-running analysis can compete with API responsiveness unless jobs
are managed correctly.

### Validation

Enforce package dependency rules and test analysis job
cancellation/status behavior.

------------------------------------------------------------------------

## ADR-003 --- Language adapters emit a canonical Intermediate Representation

**Status:** Accepted; foundational

### Context

Go and TypeScript have different ASTs, type systems, and framework
conventions. Frontend and graph traversal must not depend on
language-specific parser structures.

### Decision

Each language adapter emits a canonical, versioned intermediate
representation (IR) consisting of: - Symbols and source locations. -
Entry points and framework metadata. - Calls and resolution
candidates. - Control-flow structures. - Evidence, confidence, and
diagnostics.

The graph builder consumes this IR and produces the canonical Graph
Schema.

### Alternatives considered

-   Frontend consumes native AST-derived JSON per language.
-   One generic parser with no language-specific resolution.
-   Separate graph model for each language.

### Consequences

Positive: - One frontend and traversal engine. - Future language support
is additive. - Consistent export format.

Negative: - IR must model the intersection of language features without
flattening meaningful semantics. - Schema evolution and compatibility
need explicit versioning.

### Validation

Run identical graph-schema conformance tests against Go and TypeScript
adapters.

------------------------------------------------------------------------

## ADR-004 --- Use Go-native analysis tooling for Go projects

**Status:** Accepted

### Context

Go provides native parsing and type information. Accurate call
resolution needs more than syntax parsing.

### Decision

Use: - `go/parser` and `go/ast` for syntax. - `go/packages` for package
loading and type information. - `go/types` for semantic/type
resolution. - `golang.org/x/tools/go/ssa` for SSA where useful. -
`golang.org/x/tools/go/callgraph` for supported call graph algorithms. -
`golang.org/x/tools/go/cfg` or equivalent analysis for control flow
where appropriate.

Use version-pinned and tested versions of Go and `x/tools`.

### Alternatives considered

-   Tree-sitter only.
-   Regex and source-text heuristics.
-   Build a custom Go compiler front-end.

### Consequences

Positive: - Strong Go semantic information. - Better symbol and method
resolution. - Less custom compiler work.

Negative: - Package loading may depend on module/workspace
configuration. - Build tags, generated code, cgo, and unavailable
dependencies can cause partial analysis. - `x/tools` analysis
capabilities have specific assumptions and need fixture-based
validation.

### Validation

Create fixtures for direct calls, methods, interfaces, generics, build
tags, and unresolved dependencies.

------------------------------------------------------------------------

## ADR-005 --- Use TypeScript Compiler API for TypeScript projects

**Status:** Accepted

### Context

TypeScript's AST and TypeChecker provide syntax and semantic symbol
information needed to resolve calls and declarations.

### Decision

Use the TypeScript Compiler API: - Create a `Program` from the project
configuration. - Use AST traversal for declarations, calls, and control
flow. - Use `TypeChecker` for symbol/type resolution. - Keep compiler
integration behind a versioned adapter boundary. - Pin a tested
TypeScript version in the analyzer package.

### Alternatives considered

-   Tree-sitter only.
-   Babel parser without TypeScript type checking.
-   Regex extraction.
-   Custom TypeScript parser.

### Consequences

Positive: - Access to project compiler configuration and semantic
symbols. - Better resolution of imports, methods, and types. - Native
alignment with TS language semantics.

Negative: - Compiler API behavior and version compatibility require
testing. - Project configuration and module resolution can be complex. -
JavaScript, decorators, path aliases, and generated declaration files
require explicit support rules.

### Validation

Fixtures cover Express, NestJS DI, overloads, inheritance, path aliases,
async calls, and unresolved imports.

------------------------------------------------------------------------

## ADR-006 --- Tree-sitter is supplementary, not the sole semantic analyzer

**Status:** Accepted

### Context

Tree-sitter is useful for multi-language parsing and incremental syntax
trees. Parsing syntax alone does not resolve symbols, types, framework
registration, or dynamic dispatch.

### Decision

Use Tree-sitter as a future multi-language parsing foundation or
fallback where native semantic tooling is unavailable. Do not use it as
the sole Go or TypeScript analyzer for MVP.

### Alternatives considered

-   Tree-sitter as the only parser/resolver for all languages.
-   One custom parser per future language.
-   Regex extraction.

### Consequences

Positive: - Future language expansion can reuse parsing
infrastructure. - Native analyzers retain semantic accuracy for MVP
languages.

Negative: - Additional parser integrations increase maintenance. - A
Tree-sitter adapter still needs symbol resolution and language-specific
semantic logic.

### Validation

Evaluate each future language based on parser coverage, resolver
availability, framework patterns, and conformance tests.

------------------------------------------------------------------------

## ADR-007 --- One canonical graph powers both visualizations

**Status:** Accepted

### Context

Mind Map and Flowchart are two presentations of the same underlying code
facts. Separate models would drift and produce inconsistent results.

### Decision

Store nodes and typed edges in one canonical graph.
Visualization-specific layout and grouping are computed as view models,
never persisted as new semantic facts.

### Alternatives considered

-   Separate Mind Map and Flowchart data models.
-   Generate Mind Map from AI and Flowchart from parser output.
-   Persist layout-specific edge structures as canonical data.

### Consequences

Positive: - Consistent node identity and selection. - Easier view
switching. - Reusable search, filters, and traversal.

Negative: - The graph schema must distinguish call, control-flow,
containment, and framework relationships. - Layout code must avoid
confusing hierarchy with execution order.

### Validation

Switching views preserves selected node IDs and the underlying node/edge
set.

------------------------------------------------------------------------

## ADR-008 --- React Flow for interactive graph UI, ELK for layout

**Status:** Accepted for MVP, subject to prototype

### Context

The UI needs custom nodes, interactive edges, zoom/pan, selection,
expansion, and graph layouts. Hierarchical and directed flow layouts are
different problems.

### Decision

Use: - React + TypeScript for UI. - React Flow for graph interaction and
custom node rendering. - ELK.js for hierarchical/directed graph
layout. - A layout adapter so the rendering library can be replaced if
needed.

Use progressive subgraph loading and expansion rather than rendering
every node in a large repository at once.

### Alternatives considered

-   Cytoscape.js.
-   D3 from scratch.
-   Mermaid-only diagrams.
-   Static SVG.

### Consequences

Positive: - Custom node UX and interaction model. - Separation between
graph semantics and layout. - Supports two visualization modes.

Negative: - Large graphs require careful rendering and layout
performance work. - Layout algorithms can produce crowded graphs and
need tuning.

### Validation

Prototype a graph with branches, cycles, shared callees, and several
hundred visible nodes. Measure interaction and layout responsiveness.

------------------------------------------------------------------------

## ADR-009 --- SQLite metadata plus JSON graph snapshots

**Status:** Accepted for MVP

### Context

The local application needs durable project metadata and graph
persistence, but does not yet need a dedicated graph database server.

### Decision

Use SQLite for project, analysis, file, symbol index, warnings, and
snapshot metadata. Store canonical graph snapshots as versioned JSON
blobs or files referenced by SQLite.

### Alternatives considered

-   Neo4j.
-   PostgreSQL.
-   JSON files only.
-   In-memory graph only.

### Consequences

Positive: - Minimal operational footprint. - Portable local storage. -
Easy backup/export. - Graph data remains inspectable.

Negative: - Very large snapshots may need chunking or indexed tables
later. - Graph traversal may need in-memory indexing or dedicated
optimized structures.

### Validation

Test atomic snapshot commits, schema migration, export/import, and large
fixture behavior.

------------------------------------------------------------------------

## ADR-010 --- Static analysis is the source of graph facts; AI is optional explanation

**Status:** Accepted; non-negotiable

### Context

AI can summarize business logic but can hallucinate function
relationships or overstate runtime behavior. Core product trust depends
on source-grounded relationships.

### Decision

-   Canonical graph nodes and edges originate from deterministic
    language/framework analyzers.
-   Every relationship carries evidence and resolution metadata.
-   AI explanations are stored separately from graph facts.
-   AI never creates or silently edits canonical edges.
-   External AI calls require explicit user action and disclosure of
    provider/context.
-   Core product works with AI disabled.

### Alternatives considered

-   LLM reads the repository and generates the whole graph.
-   AI-generated graph edges are treated as confirmed.
-   Mandatory hosted AI.

### Consequences

Positive: - Verifiable graph. - Predictable offline mode. - Lower
mandatory cost and better privacy.

Negative: - Static analysis cannot resolve all dynamic behavior. - AI
explanations may be less comprehensive when only limited source context
is available.

### Validation

Test that disabling AI does not change graph output. Ensure AI output is
labeled and isolated.

------------------------------------------------------------------------

## ADR-011 --- Confidence is evidence-based and categorical in MVP

**Status:** Accepted

### Context

A numeric confidence percentage would imply calibration that the product
does not yet possess.

### Decision

Use categorical confidence values: - `high` - `medium` - `low` -
`unknown`

Store the resolution method and evidence separately. Do not present
uncalibrated percentages.

### Alternatives considered

-   Arbitrary percentage score.
-   Binary resolved/unresolved only.
-   Hide uncertain relationships.

### Consequences

Positive: - More honest interpretation. - UI can explain the basis for a
relationship. - Future calibration remains possible.

Negative: - Categories require documented assignment rules. - Different
adapters must follow consistent rules.

### Validation

Create conformance fixtures for direct calls, type-resolved calls,
inferred dispatch, and unresolved dynamic calls.

------------------------------------------------------------------------

## ADR-012 --- No automatic project code execution

**Status:** Accepted; security baseline

### Context

Analyzing untrusted repositories by executing their build scripts,
generators, or package scripts could expose the user's machine to
arbitrary code.

### Decision

The analyzer performs static reads and compiler/package metadata loading
only. It does not automatically: - Install dependencies. - Run package
scripts. - Run build/test commands. - Run code generators. - Execute
application code.

Any future optional command execution must be separately designed,
explicitly approved, sandboxed where possible, and disabled by default.

### Alternatives considered

-   Automatically run project build to improve resolution.
-   Automatically install missing dependencies.
-   Execute test suite to discover routes.

### Consequences

Positive: - Safer default behavior. - Predictable offline operation. -
No surprise side effects.

Negative: - Some projects may have incomplete type information or
generated sources. - User may need to provide pre-generated artifacts or
explicit configuration.

### Validation

Use process-level tests to assert that analysis does not spawn
project-defined scripts or package lifecycle commands.

------------------------------------------------------------------------

## ADR-013 --- Local API and filesystem boundary are security-sensitive

**Status:** Accepted

### Context

A local web UI communicating with a local server can still be exposed to
malicious browser origins or unsafe path inputs.

### Decision

-   Bind to `127.0.0.1` by default.
-   Use per-session authorization for local API requests.
-   Validate Origin/Host and apply CSRF protections.
-   Restrict filesystem reads to registered project roots.
-   Canonicalize paths and reject traversal and escaping symlinks.
-   Do not expose arbitrary file read endpoints.
-   Redact likely secrets from logs and optional AI context.

### Alternatives considered

-   Bind to all interfaces.
-   Trust localhost as sufficient authentication.
-   Accept arbitrary filesystem paths from UI requests.

### Consequences

Positive: - Reduced local attack surface. - Explicit filesystem
boundary.

Negative: - More setup and browser integration complexity. - Must test
platform-specific path and symlink behavior.

### Validation

Security tests cover DNS rebinding/host validation, origin checks, path
traversal, symlink escapes, and local API authorization.

------------------------------------------------------------------------

## ADR-014 --- Full analysis first; incremental analysis later

**Status:** Accepted for MVP

### Context

Incremental analysis can improve responsiveness but requires dependency
invalidation, symbol-level change propagation, and cache correctness.

### Decision

MVP runs full analysis per requested run, while recording file hashes
and analysis metadata to prepare for later incremental analysis.

### Alternatives considered

-   Incremental analysis from the first release.
-   Re-scan only changed files without dependency invalidation.

### Consequences

Positive: - Simpler correctness model. - Easier reproducibility and
debugging. - Lower risk of stale graph edges.

Negative: - Slower for large projects. - Repeated analysis consumes more
resources.

### Validation

Benchmark full scans on fixtures and representative projects before
deciding when incremental analysis becomes necessary.

------------------------------------------------------------------------

## ADR-015 --- Framework support is adapter-based and evidence-driven

**Status:** Accepted

### Context

Frameworks register routes and dependencies through different
conventions, decorators, functions, generated code, and runtime
mechanisms.

### Decision

Each supported framework has a dedicated extractor/adapter that emits
normalized entry-point and middleware relationships with source
evidence. Framework detection alone does not imply complete route
support.

Initial targets: - Go: `net/http`, Gin, Echo, Fiber, Chi. - TypeScript:
Express, NestJS, Fastify.

### Alternatives considered

-   Generic regex route discovery.
-   Infer all frameworks from package dependencies.
-   Ask AI to infer route registration.

### Consequences

Positive: - Clear support boundaries. - Easier fixture-based
validation. - Framework-specific behavior does not pollute graph core.

Negative: - Each adapter adds maintenance. - Dynamic registration and
custom wrappers may remain unresolved.

### Validation

Each adapter must have fixtures for direct route registration,
grouped/prefixed routes, middleware, handler references, and unsupported
dynamic patterns.
