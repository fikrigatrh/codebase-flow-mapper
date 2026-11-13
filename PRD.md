# Codebase Flow Mapper — PRD (lean MVP)

**Version:** 2.0
**Supersedes:** `archive/v1.1/PRD.md`
**Product type:** Local CLI that writes a static HTML report

## 1. Problem

Engineers lose hours reading files to answer two questions about an unfamiliar Go service:

1. "What happens when `POST /transactions` is called?"
2. "If I change `repo.Save`, which endpoints are affected?"

In layered Go services, handlers call interfaces whose implementations are bound in DI code (Wire or hand-written constructors). Direct-call graphs stop at the interface, and runtime tracing needs instrumentation and traffic.

## 2. Hypothesis to validate

> If an engineer can click an endpoint and see handler → service → repository through interfaces, with branch structure and source lines, in a single offline HTML file, they will understand a request path faster than by reading code.

The MVP exists to test this hypothesis on 2 real repositories. Everything that doesn't serve it is deferred (§7).

## 3. Users

- **Backend engineer joining a project:** learns request paths.
- **Engineer changing shared code:** needs callers and the affected endpoints.
- **QA engineer:** sees the branches in an endpoint to find candidate test paths.

## 4. Principles (kept from v1.1)

1. Evidence first: every edge carries a source location.
2. Static, not runtime, and the UI says so.
3. Honest uncertainty: unresolved and ambiguous calls stay visible with a reason. A call edge is never made up from a name match.
4. Local-only: no network and no execution of target-project code.
5. Deterministic: the same input gives byte-identical JSON.

## 5. Scope

### 5.1 Input

A directory with a `go.mod`, or a `go.work`. Module dependencies must already be in the local module cache. CFM never downloads them (it runs with `GOPROXY=off`). If they are missing, CFM still produces a partial result plus a diagnostic.

### 5.2 Frameworks

| Framework | Import path | Recognized registrations |
|---|---|---|
| Gin | `github.com/gin-gonic/gin` | `GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS/Any/Handle/Match`, `Group`, `Use` on `*gin.Engine` / `*gin.RouterGroup` / `gin.IRouter`/`IRoutes` |
| net/http | `net/http` | `http.Handle/HandleFunc`, `(*http.ServeMux).Handle/HandleFunc`, Go 1.22 patterns (`"GET /items/{id}"`) |
| Fiber v2 | `github.com/gofiber/fiber/v2` | `Get/Post/Put/Patch/Delete/Head/Options/All/Add`, `Group`, `Use` on `*fiber.App` / `fiber.Router` |

Fiber v3 and other routers produce an `ROUTER_UNSUPPORTED` diagnostic.

### 5.3 Capabilities

| # | Capability | Requirement |
|---|---|---|
| FR-01 | Load | Load all packages in the module (no `_test.go`) with full type info, under the safety env (ARCHITECTURE §6) |
| FR-02 | Exclusions | Skip `vendor/`, and by default skip packages whose last path element matches `*fakes`, `*mocks`, `mock*`, `testutil*`. `--exclude` adds glob patterns |
| FR-03 | Functions | Every project function, method and function literal becomes a node with its source span |
| FR-04 | Calls | Call edges come from the VTA call graph. Static calls get `high` confidence. Interface or func-value calls get `medium` if VTA finds exactly 1 target, and `low` for each target when it finds ≥2, with all alternatives kept. Zero targets → an `unresolved` node plus a diagnostic |
| FR-05 | External calls | Calls leaving the module become `external` nodes, one per function, and are tagged by category (`db`, `http_client`, `cache`, `queue`, `log`, `other`) using a fixed prefix table. They are not traversed |
| FR-06 | Routes | Endpoint nodes have a method, a full path (including the group prefix) and the source location of the registration. Handler and middleware edges keep their order |
| FR-07 | Partial routes | A non-constant path or a group prefix that can't be resolved still produces the endpoint, with `?` segments, status `partially_resolved` and a diagnostic. Routes are never dropped silently |
| FR-08 | Flow outline | Each project function holds an ordered outline of `if/else`, `switch/case`, `for/range`, `select`, `go`, `defer`, `return` and `panic`, plus the call sites nested in them. Constructs that contain no call or return are pruned |
| FR-09 | Diagnostics | Load errors, type errors, unsupported routers, dynamic paths, unresolved calls and excluded packages are reported with a code, location and reason |
| FR-10 | Report | One HTML file with the graph JSON and the viewer JS/CSS inlined. It works offline from `file://` |
| FR-11 | JSON | `--json path` also writes the graph JSON (GRAPH_SCHEMA) |
| FR-12 | Endpoint view | An endpoint list with search. Selecting one shows the Call Tree and the Flowchart from its handler |
| FR-13 | Function view | Search any function and toggle **Callees** or **Callers**. Callers mode lists the endpoints that reach it |
| FR-14 | Call Tree | Collapsible tree layout of call edges, with depth 3 by default and expand on click. Cycles are shown as a back-reference node, never expanded forever |
| FR-15 | Flowchart | Top-down layout of the selected function's outline. Branch edges are labelled `true/false/case/default/loop/exit`. A call step can be expanded inline into the callee's outline |
| FR-16 | Trust UI | Confidence is shown by line style plus a text label (solid = high, dashed = medium, dotted = low), not colour alone. Clicking an edge shows the resolution method, reason and source location |
| FR-17 | Source link | Nodes show `relative/path.go:line`. With `--editor vscode\|idea\|zed`, they also link to the editor URL (this embeds the absolute root, which the user chose to do) |
| FR-18 | Filters | Hide or show external calls (hidden by default), low-confidence edges and unresolved nodes |
| FR-19 | Generated files | If `wire_gen.go` or an in-module generated package (swag `docs`) is missing, warn and show a blocker banner with the exact command to run. CFM never runs generators itself (M0 finding: 0 interface hops resolve without `wire_gen.go`) |
| FR-20 | Helper noise | Repeated callees are deduped per parent, and there's a package filter with shared/error/utils-style packages hidden by default (ARCHITECTURE §5) |

### 5.4 Non-functional

- **NFR-1 Safety:** never runs project code, `go generate`, cgo or scripts, and never makes network calls (ARCHITECTURE §6).
- **NFR-2 Determinism:** sorted nodes and edges, stable IDs, golden-file tests.
- **NFR-3 Performance:** to measure in M3, not guarantee. Initial target: a ~100k LOC service analyzed in < 60 s and < 4 GB RSS, and the report opens in < 3 s. M0 measured 10–30 s and **2.8–3.6 GB** on 68–300-file services. Memory is the risk, not time (`docs/spike-m0.md`).
- **NFR-4 Report privacy:** the report contains identifiers, paths relative to the root, and condition expression text. It contains no function bodies. The README warns against sharing reports of confidential code.
- **NFR-5 Accessibility:** the viewer works by keyboard (search, list navigation, expand) and confidence isn't shown by colour alone.

## 6. Acceptance / release gate

MVP is done when:

1. Golden fixtures pass for Gin, net/http, Fiber and the DI/interface fixture.
2. On **2 real repositories**, for 5 randomly chosen endpoints each, the handler → repository path through interfaces is correct, checked by hand against the code. ≥ 4/5 per repo must be correct, and every miss must have a diagnostic explaining it.
3. No fake/mock implementation shows up as a call target in the real-repo check.
4. The safety test passes: analyzing the `evil` fixture runs no code and makes no network call.
5. An engineer who doesn't know one of the real repos answers the "what does `X` do" questions for 3 endpoints faster with the report than without it. This is informal, but it is the hypothesis check. Record the result in `docs/validation.md`.

## 7. Deferred (with trigger)

| Feature | Revisit when |
|---|---|
| TypeScript (Express/NestJS/Fastify) | Go MVP passes the gate and there is a real TS repo to support |
| Echo, Chi, Fiber v3, gRPC, queue/cron entry points | A target repo needs them |
| Local server, SQLite, project registry, job lifecycle | Reports become too big for one HTML file, or incremental analysis is needed |
| AI explanation | Never inside the graph. Optional later, as a separate panel |
| Graph diff / git impact | Users ask "what changed in this PR" |
| SQL/table extraction | `db` tagging proves not to be enough |
| JSON Schema file for third-party consumers | Someone consumes the JSON besides the viewer |
