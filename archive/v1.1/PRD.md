# Codebase Flow Mapper --- Product Requirements Document

**Document:** PRD\
**Version:** 1.0\
**Status:** Proposed baseline for MVP\
**Product type:** Local-first developer tool\
**Primary users:** Backend engineers, QA engineers, software architects,
technical leads

## 1. Product overview

Codebase Flow Mapper (CFM) analyzes a source-code project and produces
an interactive, navigable representation of how the application is
structured and how statically discoverable execution paths connect.

An engineer selects a local project directory. CFM scans supported
source files, discovers supported entry points (initially HTTP routes),
resolves function and method relationships where possible, extracts
control-flow constructs, normalizes the results into a
language-independent graph, and opens a browser UI.

The product has two complementary analysis modes:

-   **Endpoint-driven:** start at an endpoint and explore middleware,
    handler, downstream calls, branches, data-access operations, and
    terminal returns.
-   **Function-driven:** start at any discovered function or method and
    explore callers, callees, containing control flow, and reachable
    entry points.

The same canonical graph powers two visualizations:

-   **Mind Map:** hierarchical, progressively expanded exploration.
-   **Flowchart:** directed execution/control-flow representation with
    branches, loops, and returns.

AI is optional. Static analysis and visualization must work without an
AI provider, account, or network access.

## 2. Problem statement

Engineers spend significant time reading multiple files to understand
unfamiliar or legacy systems. Existing documentation can be incomplete
or stale. Runtime tracing requires instrumentation and only shows paths
that have actually executed. Basic call graphs often omit framework
routing, middleware, conditions, and source evidence.

CFM aims to provide a source-grounded, local-first map that helps
engineers understand code without requiring OpenTelemetry
instrumentation or application execution.

## 3. Product principles

1.  **Evidence before explanation:** every discovered relationship must
    retain source evidence or an explicit reason why evidence is
    indirect.
2.  **Static analysis, not runtime tracing:** results describe
    statically discovered or inferred paths, not proof of runtime
    execution.
3.  **Honest uncertainty:** unresolved or ambiguous relationships remain
    visible and are never silently fabricated.
4.  **Local-first privacy:** source files stay local by default.
5.  **Language-independent core:** language adapters produce one
    normalized graph schema.
6.  **Progressive disclosure:** large graphs load and expand
    incrementally.
7.  **AI is optional:** AI can explain evidence but cannot invent or
    silently modify graph relationships.
8.  **Reproducibility:** analysis runs record analyzer, schema, and
    language-tool versions.

## 4. Goals and non-goals

### 4.1 Goals

-   Discover supported HTTP endpoints in supported Go and TypeScript
    frameworks.
-   Map endpoint-to-middleware-to-handler relationships.
-   Discover functions, methods, and statically resolvable call
    relationships.
-   Represent supported control-flow constructs: conditionals, switch
    statements, loops, and returns.
-   Support forward and reverse graph traversal.
-   Display Mind Map and Flowchart views from the same graph.
-   Navigate from graph nodes to the corresponding local source
    location.
-   Report unresolved calls, unsupported constructs, and analysis
    limitations.
-   Export a portable graph snapshot.
-   Operate without mandatory AI or cloud services.

### 4.2 Non-goals for MVP

-   Guaranteeing complete runtime execution traces.
-   Executing project code, tests, build scripts, generators, package
    scripts, or dependency installation.
-   Full interprocedural data-flow/taint analysis.
-   Complete SQL semantic analysis or database schema inference.
-   Complete support for every framework, language feature, reflection
    mechanism, or dynamic dispatch pattern.
-   Distributed/microservice-wide graph correlation.
-   Runtime instrumentation, OpenTelemetry collection, or production
    telemetry ingestion.
-   Team collaboration, cloud hosting, authentication, or multi-user
    permissions.
-   IDE extensions.
-   Automatic code modification or refactoring.
-   AI-generated graph edges without static evidence.

## 5. Target personas and primary jobs

### Persona A --- Backend engineer joining an existing project

Needs to understand request paths, service boundaries, repositories, and
error handling quickly.

### Persona B --- Engineer changing a shared function

Needs to identify callers and endpoint paths potentially affected by a
change.

### Persona C --- QA engineer

Needs to discover endpoints, shared logic, branching conditions, and
candidate test paths.

### Persona D --- Technical lead / architect

Needs a high-level map of application entry points, dependencies, and
major components.

## 6. MVP scope

### 6.1 Supported languages

-   Go, using Go-native parser/type tooling and `golang.org/x/tools`
    where appropriate.
-   TypeScript, using the TypeScript Compiler API and TypeChecker.

JavaScript projects may be supported when they are parseable by the
TypeScript compiler, but the initial acceptance target is TypeScript
projects. The analyzer must label JavaScript-specific or untyped
resolution limitations.

### 6.2 Initial framework adapters

Go: - `net/http` common registration patterns. - Gin. - Echo. - Fiber. -
Chi.

TypeScript/Node.js: - Express. - NestJS. - Fastify.

Framework support is pattern-specific. The UI must disclose which
adapter and registration patterns were recognized. A dependency
appearing in a manifest does not by itself prove that a route or
middleware was discovered.

### 6.3 Required capabilities

  Capability            MVP requirement
  --------------------- -------------------------------------------------
  Project selection     Register and analyze a local directory
  File scanning         Respect ignore rules and safe path boundaries
  Language detection    Identify Go and TypeScript source/configuration
  Framework detection   Detect supported framework candidates
  Endpoint discovery    Extract supported HTTP method/path/handler
  Middleware mapping    Extract recognized middleware associations
  Symbol inventory      Functions, methods, relevant types/classes
  Call graph            Direct calls and supported static resolution
  Control flow          If/else, switch, loops, returns
  Traversal             Forward, reverse, path, endpoint reachability
  Visualizations        Mind Map and Flowchart
  Source navigation     File and line/column references
  Confidence/evidence   Relationship certainty and evidence
  Warnings              Unresolved and unsupported findings
  Export                Versioned JSON graph snapshot
  Local operation       No mandatory network or AI

## 7. Functional requirements

### FR-001 --- Project registration

The user can select a local directory and register it as a project. The
application stores a canonical project root and a display name. The user
can list, inspect, and remove registered projects.

### FR-002 --- Safe scanning

The scanner traverses supported source files while excluding
configurable directories such as `.git`, `node_modules`, `vendor`, build
output, generated output, and configured ignore patterns. It must not
follow symlinks outside the registered project root by default.

### FR-003 --- Language and framework detection

The analyzer identifies languages and framework candidates from
manifests, imports, source patterns, and route registration syntax.
Detection results include evidence and confidence. Detection must not be
treated as proof that a framework is actively used.

### FR-004 --- Analysis job lifecycle

The user can start an analysis and inspect progress, status, timestamps,
counts, and warnings. Jobs expose phases such as scanning, parsing,
resolving, extracting, graph construction, validation, and persistence.

### FR-005 --- Endpoint discovery

For supported framework patterns, the analyzer extracts HTTP method,
route path, source location, handler target, and known middleware chain.
Unknown/dynamic route registration must be reported as unresolved or
partially resolved.

### FR-006 --- Symbol discovery

The analyzer discovers functions, methods, receivers/classes,
parameters, return types when available, source locations, and stable
symbol identifiers.

### FR-007 --- Call relationship extraction

The analyzer extracts call expressions and resolves targets using
language-specific type/symbol information. Each edge records its
resolution method, evidence location, and confidence. Multiple possible
targets must be represented as alternatives or unresolved candidates,
not collapsed into a fabricated single target.

### FR-008 --- Control-flow extraction

The analyzer represents supported conditional branches, switch/case
branches, loops, returns, and relevant error-handling paths. The graph
must distinguish control-flow structure from ordinary function-call
relationships.

### FR-009 --- Endpoint-driven exploration

The user selects an endpoint and sees its known middleware, handler, and
downstream call graph. The user can expand nodes and inspect branches.
The view must label static reachability separately from runtime
execution.

### FR-010 --- Function-driven exploration

The user searches/selects a function or method and sees its callers and
callees. The user can switch between forward and reverse exploration and
navigate to endpoint roots that can reach the selected symbol.

### FR-011 --- Mind Map visualization

The UI displays a hierarchical, expandable view. The view supports
progressive expansion, collapse, search focus, zoom, pan, path
highlighting, and node detail inspection.

### FR-012 --- Flowchart visualization

The UI displays directed edges and explicit control-flow branches.
True/false, case/default, loop-back, and return semantics must be
visually distinguishable. The graph layout must remain usable for cyclic
graphs.

### FR-013 --- Shared graph model

Both visualizations consume the same canonical graph data. Switching
views must not change the underlying analysis result.

### FR-014 --- Source navigation

Selecting a node or edge opens the relevant file and line/column in a
read-only source viewer. The application validates that the resolved
path remains within the registered project root.

### FR-015 --- Search and filters

The user can search endpoints, symbols, files, and node IDs. Filters
include node type, relationship type, language, confidence, and
resolution status.

### FR-016 --- Analysis warnings

The application surfaces unresolved calls, unsupported constructs,
ambiguous targets, parse errors, excluded files, and partial analysis.
Warnings link to source evidence where available.

### FR-017 --- Graph export

The user can export a versioned JSON graph snapshot containing schema
version, analyzer version, project metadata that is safe to include,
nodes, edges, and analysis warnings. Export must not include source
contents or secrets by default.

### FR-018 --- Optional AI explanation

If configured, the user can request an explanation of a selected
function or flow. The request includes only relevant source snippets and
graph context. The UI must disclose the provider and data being sent. AI
output is stored separately from canonical graph facts and is labeled as
generated explanation.

### FR-019 --- Project removal

The user can remove a project from the local application. The UI must
clearly state whether this removes only CFM metadata/cache or also any
exported files. It must not delete the original project directory.

## 8. User stories and acceptance criteria

### Epic A --- Project onboarding

**US-A01:** As an engineer, I want to select a local project folder so
that CFM can analyze it.

Acceptance criteria: - Given a readable directory, when I register it,
then the project appears in the project list. - The canonical root path
is stored. - An invalid or unreadable directory produces an actionable
error. - The application does not copy or upload the source tree by
default.

**US-A02:** As an engineer, I want scanning to ignore irrelevant
directories.

Acceptance criteria: - Default exclusions are applied. - User-configured
ignore patterns are respected. - Symlinks escaping the project root are
not followed by default. - The scan summary reports scanned, ignored,
and unsupported file counts.

### Epic B --- Analysis

**US-B01:** As an engineer, I want to start an analysis and see
progress.

Acceptance criteria: - The analysis returns a unique run ID. - The
status endpoint reports current phase and progress counters. - Failures
include a phase and actionable diagnostic. - A partial result can be
retained when safe and useful, with status clearly marked partial.

**US-B02:** As an engineer, I want the analyzer to discover supported
endpoints.

Acceptance criteria: - Recognized route registrations produce endpoint
nodes. - Each endpoint records method, path, source location, and
handler relationship where resolvable. - Unsupported/dynamic
registrations are reported as unresolved rather than omitted without
notice. - Duplicate route declarations are preserved with distinct
source evidence.

**US-B03:** As an engineer, I want function calls resolved to their
declarations.

Acceptance criteria: - Direct calls are linked to their declaration when
the language resolver can identify it. - Unresolved and ambiguous calls
remain represented. - Every resolved call edge has source evidence. -
External/library calls can be represented without pretending they are
project-local symbols.

**US-B04:** As an engineer, I want conditions and loops represented.

Acceptance criteria: - Supported if/else and switch constructs have
explicit branch edges. - Loop constructs have loop entry/body/exit
semantics. - Return statements are represented with source location and
return category. - Unsupported control-flow constructs produce warnings.

### Epic C --- Exploration

**US-C01:** As an engineer, I want to open an endpoint flow.

Acceptance criteria: - Selecting an endpoint opens its known
middleware/handler/downstream graph. - Expansion is progressive and does
not load the entire project graph unnecessarily. - A node can be
expanded and collapsed. - Unresolved edges are visibly marked.

**US-C02:** As an engineer, I want to explore a function in both
directions.

Acceptance criteria: - Forward view lists callees. - Reverse view lists
callers. - The UI can show endpoint roots that reach the function when
statically known. - Cycles do not cause infinite expansion.

**US-C03:** As an engineer, I want to switch between Mind Map and
Flowchart.

Acceptance criteria: - Both views represent the same node and edge
identities. - Mind Map emphasizes hierarchy. - Flowchart emphasizes
direction and control flow. - Switching views preserves the selected
node and relevant filters.

**US-C04:** As an engineer, I want to inspect source code from a node.

Acceptance criteria: - Node selection reveals file and line/column. -
Open-source action loads the correct source range. - Missing or changed
source is reported gracefully. - Path traversal outside the registered
project root is rejected.

**US-C05:** As an engineer, I want to filter and search large graphs.

Acceptance criteria: - Search supports symbol name, endpoint path, and
file path. - Filters can constrain node and edge types. - Search
focuses/highlights matches. - The UI indicates when results are filtered
or truncated.

### Epic D --- Trust and export

**US-D01:** As an engineer, I want to understand how certain a
relationship is.

Acceptance criteria: - Every edge has a confidence classification and
resolution method. - The UI explains why the edge has that
classification. - Inferred relationships are visually distinct from
direct relationships. - Unknown is not displayed as a confirmed
relationship.

**US-D02:** As an engineer, I want to export analysis results.

Acceptance criteria: - Export conforms to the published schema
version. - Export can be parsed by the schema validator. - Source
contents and secrets are excluded by default. - Export includes analyzer
and schema version metadata.

**US-D03:** As an engineer, I want AI explanation to be optional and
transparent.

Acceptance criteria: - Core analysis works when no AI provider is
configured. - AI is never invoked automatically by default. - Before
external transmission, the user sees provider and context scope. - AI
explanation is stored separately and cannot silently add graph edges.

## 9. Non-functional requirements

### NFR-001 --- Privacy

Source code remains local by default. No source content is transmitted
without explicit user action.

### NFR-002 --- Network

The local server binds to loopback (`127.0.0.1`) by default. Remote
access is not enabled by default.

### NFR-003 --- Security

-   Validate all file paths against the registered root.
-   Do not execute project code or scripts during analysis.
-   Do not install dependencies automatically.
-   Redact likely secrets from logs and optional AI context.
-   Apply request origin/CSRF protections appropriate for a local web
    application.
-   Protect local API endpoints with a per-session token or equivalent
    local authorization mechanism.
-   Avoid exposing arbitrary filesystem read APIs.

### NFR-004 --- Reliability

Analysis failures must not corrupt prior successful snapshots. A new
snapshot is committed atomically after validation.

### NFR-005 --- Performance

The UI uses progressive loading and graph expansion. Performance targets
must be benchmarked against a published fixture corpus before being
treated as release guarantees.

Initial engineering targets: - UI remains responsive while analysis
runs. - Search and expansion operate on bounded subgraphs. - Large
projects are not rendered as one unrestricted canvas. - Analysis reports
elapsed time and resource counters.

### NFR-006 --- Reproducibility

Each analysis stores analyzer version, schema version, parser/compiler
version, configuration fingerprint, and timestamp.

### NFR-007 --- Accessibility

Keyboard navigation, visible focus, readable node labels, and
non-color-only confidence indicators are required for core workflows.

## 10. Analysis semantics and limitations

CFM produces a **static code map**, not a runtime trace.

-   A call edge means the source contains a call expression that the
    analyzer resolved or inferred.
-   A control-flow edge means the source structure permits a statically
    modeled transition.
-   Reachability means the graph contains a path under the analyzer's
    model; it does not prove that runtime data will take that path.
-   Dynamic dispatch, reflection, runtime registration, generated code,
    build tags, plugins, and configuration-driven behavior may be
    partially or not resolved.
-   Framework support is adapter- and pattern-specific.
-   External dependency calls may be represented as external symbols
    without analyzing their implementation.
-   Concurrency and asynchronous scheduling are not equivalent to a
    single sequential execution path.

The UI and exported schema must preserve these distinctions.

## 11. MVP release gates

MVP is releasable when: 1. Go and TypeScript fixture projects pass
expected graph tests. 2. Supported routes are discovered with source
evidence. 3. Direct calls and supported control flow are represented. 4.
Both exploration modes work. 5. Both visualizations work from the same
graph. 6. Source navigation is safe and correct. 7.
Unresolved/unsupported analysis is visible. 8. Core functionality works
with AI disabled and offline. 9. JSON export validates against the
published schema. 10. Security review confirms local path and API
protections.

## 12. Deferred roadmap

Post-MVP candidates: - Incremental analysis and file watching. - More Go
and Node framework patterns. - Python, Java, PHP, C# adapters. - SQL
query extraction and database table mapping. - Interprocedural data-flow
and taint analysis. - Queue/event/cron entry-point discovery. -
Cross-repository and microservice mapping. - Git diff impact analysis. -
Optional runtime trace import for comparison, not as a requirement. -
IDE integration and team collaboration.

## 13. Open product questions

These questions should be resolved with implementation evidence rather
than assumed: - Which exact route-registration patterns are supported in
each framework adapter? - Should JavaScript be a first-class MVP target
or best-effort through the TypeScript compiler? - What maximum graph
size should trigger automatic subgraph mode? - Should project settings
support custom ignore files and framework adapter overrides in MVP? -
Which local AI runtimes, if any, will be officially supported at first
release?
