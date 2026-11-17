# Codebase Flow Mapper — Architecture (lean MVP)

**Version:** 2.0
**Supersedes:** `archive/v1.1/ARCHITECTURE.md`, `CODING_STANDARDS.md`, `TESTING_STRATEGY.md`, `IMPLEMENTATION_GUIDE.md`

## 1. Shape

```
cfm analyze <dir> -o report.html [--json graph.json] [--exclude glob]... [--editor vscode|idea|zed]
cfm render <graph.json> -o report.html   # graph from any analyzer (ADR-108)

 ┌────────┐   ┌──────────┐   ┌───────────┐   ┌────────┐   ┌──────────┐
 │  load  │──▶│ callgraph│──▶│  routes   │──▶│  flow  │──▶│  graph   │──▶ report.html / graph.json
 │packages│   │  (VTA)   │   │gin/http/  │   │outline │   │ build +  │
 │ + SSA  │   │          │   │  fiber    │   │ (AST)  │   │ validate │
 └────────┘   └──────────┘   └───────────┘   └────────┘   └──────────┘
```

It is one process with one pass and no persistence. Every stage reads the loaded `*packages.Package` set and the SSA program, and nothing else.

## 2. Repository layout

```
cmd/cfm/main.go            flag parsing, run pipeline, write outputs (stdlib flag; no CLI framework)
internal/load/             go/packages load under safety env, exclusions, SSA build
internal/callgraph/        VTA call graph → call edges, confidence, unresolved, external tagging
internal/routes/           route extraction: router.go (shared prefix/middleware tracking), gin.go, nethttp.go, fiber.go
internal/flow/             per-function flow outline from AST
internal/graph/            types (GRAPH_SCHEMA), IDs, sort, validate, JSON encode
internal/report/           html/template + go:embed of web/
web/                       viewer.js, viewer.css, vendor/ (cytoscape, dagre, cytoscape-dagre + LICENSE files)
testdata/fixtures/<name>/  one Go module per fixture + expected.json (golden)
```

There is no `util` package and no interfaces with one implementation. Stages are plain functions:

```go
func Load(ctx context.Context, opts load.Options) (*load.Program, []graph.Diagnostic, error)
func Calls(p *load.Program) callgraph.Result            // edges, unresolved, externals
func Routes(p *load.Program) routes.Result              // endpoints, routesTo, middleware edges
func Outline(p *load.Program) map[graph.NodeID][]graph.Step
func Build(in graph.BuildInput) (*graph.Graph, error)   // merge, sort, validate
```

## 3. Load (`internal/load`)

- `packages.Load` with `NeedName|NeedFiles|NeedSyntax|NeedTypes|NeedTypesInfo|NeedImports|NeedDeps|NeedModule`, `Tests: false`, pattern `./...`, `Dir: <root>`.
- Safety env is set on `packages.Config.Env` (§6).
- Exclusions apply **after** loading but **before** SSA. Excluded packages are dropped from the SSA input set, and each one gets an `PACKAGE_EXCLUDED` info diagnostic.
- `ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)`, then `Build()` on the project packages only (CFM-302). Dependency functions have no bodies: their calls are leaves anyway, and skipping them is 2.5–6.5x faster with 30–40% less RSS. The cost is that VTA no longer sees types created inside dependencies. Measured on 3 repos, that only drops low-confidence `error.Error` fan-out (253 candidates on service A) and gin's own `ResponseWriter`. Project nodes, flows, diagnostics and unresolved counts are unchanged.
- Package and type errors become `GO_LOAD_ERROR` / `GO_TYPE_ERROR` diagnostics. The analysis continues on whatever loaded.
- **Generated-file detection (ADR-107).** The M0 spike showed that real checkouts gitignore these files.
  - `WIRE_GEN_MISSING` (warning): a package's `IgnoredFiles` include a file with a `wireinject` build constraint, and the package has no `wire_gen.go`. The fix is `wire ./...`.
  - `GENERATED_PKG_MISSING` (warning): an import of an **in-module** path has no Go files (`could not import … (invalid package name: "")`). If the path ends in `/docs`, the fix is `swag init`. Otherwise it's `go generate` for that package.
  - Either warning also goes into the `stats.blockers` list. The viewer shows that list as a banner at the top, with the exact command to run, because without it most interface hops come out unresolved.
- Build tags use the defaults, so `wire_gen.go` (`!wireinject`) is in and `wire.go` (`wireinject`) is out. That is what we want.

## 4. Analysis rules

### 4.1 Call graph (`internal/callgraph`)

```go
cg := vta.CallGraph(ssautil.AllFunctions(prog), cha.CallGraph(prog))
```

- **Project function:** an `*ssa.Function` whose package (or, for closures, whose parent's package) has its module path under the root module. Everything else is **external**.
- For each project caller and each call site (`cg.Nodes[fn].Out` grouped by `edge.Site`):

| Site kind | Targets found | Confidence | Method |
|---|---|---|---|
| static callee (`site.Common().StaticCallee() != nil`) | 1 | high | `static` |
| dynamic (interface or func value) | 1 | medium | `vta` |
| dynamic | ≥2 | low (each) | `vta`; edges share `metadata.siteId` and list `alternatives` |
| dynamic | 0 | unknown | `unresolved` → `unresolved` node + `CALL_UNRESOLVED` diagnostic |

- **Composition root needed:** VTA only sees concrete types that flow into a call site. If `main` / `wire_gen.go` is not in the loaded packages (for example, when analyzing a library), interface sites come out unresolved. That's correct and honest behaviour. Emit `NO_COMPOSITION_ROOT` (info) once when no `main` package is loaded.
- **Callsite key:** `file:line:col` of `site.Pos()`. For `ssa.Call` that is the `CallExpr.Lparen`. For `ssa.Go` / `ssa.Defer` it is the `go` / `defer` keyword. `internal/flow` must use the same rule (`flow.siteKey`). It is defined once in `internal/graph` and used by both stages.
- **Externals:** one node per external function (`external:<pkgPath>.<Func>`). The category comes from a fixed prefix table in `callgraph/external.go`:
  - `db`: `database/sql`, `gorm.io/`, `github.com/jackc/pgx`, `github.com/jmoiron/sqlx`, `go.mongodb.org/`
  - `cache`: `github.com/redis/`, `github.com/go-redis/`
  - `http_client`: `net/http.(*Client)`, `net/http.Get/Post/Do`, `github.com/go-resty/`
  - `queue`: `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, `github.com/rabbitmq/`, `cloud.google.com/go/pubsub`
  - `log`: `log`, `log/slog`, `go.uber.org/zap`, `github.com/sirupsen/logrus`, `github.com/rs/zerolog`
  - otherwise `other`
- Synthetic SSA functions (wrappers, bound/thunk methods) are followed through to their real targets and never appear as nodes.
- **External callees (CFM-104 finding).** A dynamic site whose callee is declared outside the module (an interface method such as `context.Context.Err`, `error.Error`, `http.ResponseWriter.WriteHeader`, or a named func type such as `context.CancelFunc`) and that reaches no project function gets **one** `high`/`static` edge to that declaration (`external:context.(Context).Err`, `external:context.CancelFunc`), not one `low` edge per stdlib implementation. On service B every `low` edge was a `defer cancel()` before this rule. If a project type does reach the site, the table above applies.
- `alternatives` is the total number of targets at the site.
- Nodes exist only for functions SSA builds (`ssautil.AllFunctions`): a method of a type that never reaches an interface or a call is dead code and gets no node.
- Generic instances map to their origin (one node per source function; targets are unioned per site). Calls inside a range-over-func loop body belong to the enclosing function. SSA gives the iterator call no position, so its site is the range statement's `for` keyword, and `internal/flow` must use the same.

### 4.2 Routes (`internal/routes`)

This walks the AST of every project file and uses `types.Info` to decide whether a call is a router method. It never matches on names alone.

**Router values.** A router value is any expression whose type is one of:
- Gin: `*gin.Engine`, `*gin.RouterGroup`, `gin.IRouter`, `gin.IRoutes`
- Fiber: `*fiber.App`, `fiber.Router`, `*fiber.Group`
- net/http: `*http.ServeMux`, plus the package-level `http.Handle/HandleFunc`, which use the implicit `DefaultServeMux`

**Prefix and middleware state.** State is kept per `types.Object` (a local var, parameter or struct field):

1. Roots (`gin.New/Default`, `fiber.New`, `http.NewServeMux`, `DefaultServeMux`) start with prefix `""` and an empty chain.
2. `x := r.Group("/v1", mw...)` gives `x` the prefix `r.prefix + "/v1"` and the chain `r.chain + mw`.
3. `r.Use(mw...)` appends to `r.chain` for registrations that come **later in source order** in the same function.
4. **Router passed as a parameter** (the common `func RegisterUserRoutes(g *gin.RouterGroup)` pattern): collect the state of the argument at every static call site of that function, and run the function body once per distinct incoming state. Iterate to a fixed point, capped at depth 8, and emit `ROUTE_PREFIX_DEPTH` if the cap is hit. One body reached with 2 prefixes legitimately yields 2 endpoints.
5. A struct field router (`s.router`) uses the state from the field's single assignment in the package. If it is assigned more than once, or never, the prefix is `?` and `ROUTE_PREFIX_UNKNOWN` is emitted.

**Registration.** For a registration (`GET`, `Get`, `Handle`, `HandleFunc`, …):
- The path must be a string constant (`types.Info.Types[expr].Value`). Otherwise it is `?`, with `ROUTE_PATH_DYNAMIC`, and the endpoint status is `partially_resolved`.
- Gin/Fiber `handlers ...`: the last handler is the **handler** and the earlier ones are route-level **middleware**. The final chain is group chain + route middleware.
- net/http: the method comes from a Go 1.22 pattern prefix (`"GET /x"`) and is otherwise `ANY`. For the handler argument:
  - a func/method value or a func literal → the handler;
  - `http.HandlerFunc(f)` → `f`;
  - a value of a named type with `ServeHTTP` → that type's `ServeHTTP`;
  - a call `w(inner)` to a project function → `w` becomes middleware (confidence medium) and the rule recurses on `inner`;
  - anything else → an unresolved handler with `HANDLER_UNRESOLVED`.
  - (CFM-106) Conversions are stripped first. The ServeHTTP rule applies only to non-func types. An interface-typed variable is `no_concrete_type`, while a dependency call returning `http.Handler` (`http.StripPrefix(...)`) becomes an `external` target through the factory rule. The wrapper's inner handler is its first argument that is a func or has `ServeHTTP`. Wrappers are added after the route middleware, outermost first. In a pattern the host stays in the path, and net/http has no prefixes, so a router passed as a parameter just keeps `""`.
- `Any`/`All`/`Match`/`Handle(method, …)` expand to one endpoint per method when the methods are constant, and to `ANY` otherwise.
- Duplicate method+path registrations are kept as separate endpoints, each with its own `registration` location.
- **Middleware factories (CFM-105).** A handler or middleware written as a call (`mw.Logger()`, `mw.RateLimit(10)`) to a project function whose only `return` returns a func literal targets that literal (`Logger$1`); otherwise it targets the factory. Both are `medium` / `wrapper_heuristic`. A dependency's function (`gin.Recovery()`, `ginSwagger.WrapHandler(...)`) becomes an `external` target.
- A path or prefix that cannot be computed shows as `?`: `/?` for a dynamic path under the Gin root, `?/orphan` for an unknown prefix.
- **Fiber v2 (CFM-107).** Verbs, `All` → `ANY`, `Add(method, …)`, `Group(prefix, mw…)`, and `Route(prefix, func(r Router){…})`, where the literal's parameter is the group. `Use("/api", mw)` scopes `mw` to paths that start with that prefix, using a plain string prefix test as Fiber does, so `/apix` matches too. A chained `app.Get(…).Get(…)` registers both, because a registration call evaluates to its receiver's router (all frameworks).
- **Unsupported routers.** The first import of Fiber v3, Echo, Chi, gorilla/mux or httprouter gives one `ROUTER_UNSUPPORTED` warning, even when that package failed to load.
- Known limits (ponytail): middleware that `gin.Default()` adds internally is not shown; a `Use` inside a callee does not flow back to the caller's router; a handler held in a local func variable is unresolved. Fiber: the state model is gin-like (a group copies its chain when it is created), so an `app.Use` written after `app.Group(…)` does not reach that group's routes, although Fiber's ordered stack would apply it. `Mount`, `Static`, a named func passed to `Route`, and `[]string` prefixes in `Use` (shown unscoped) are not modelled.

### 4.3 Flow outline (`internal/flow`)

This is an AST walk over each project function body (named funcs, methods and func literals). It produces a `[]graph.Step` tree (GRAPH_SCHEMA §5):

- Kept: `if` (cond text, then, else / else-if chain), `switch`/type switch (tag, cases with label text, default), `for`/`range` (header text), `select` (cases), `go`, `defer`, `return`, `panic(...)`, and call sites (by `siteKey`).
- **Pruning:** a construct is kept only if its subtree contains a call site or a `return`/`panic`. This keeps outlines proportional to what matters.
- Cond, tag and header text is `go/printer` output of the expression, truncated to 120 runes with `…`.
- Not call sites: builtins (`len`, `append`, …), type conversions (`int(x)`, `http.HandlerFunc(f)`), and anything `types.Info` doesn't classify as a call. That keeps validation rule GRAPH_SCHEMA §7.3 exact. A call whose target lives in an excluded package becomes an `unresolved` node with `reason: excluded_target`.
- Func literals nested in a body are **not** inlined. The literal is its own function node, and the parent outline holds a `call` step only where the literal is called or passed to `go`/`defer`.

## 5. Viewer (`web/`, inlined by `internal/report`)

- **Template:** `html/template` puts `viewer.css` and `viewer.js` inline and puts the graph in `<script type="application/json" id="cfm-graph">`. The JSON is encoded with `json.Marshal`, which escapes `<`, `>` and `&`, so `</script>` inside cond text is safe.
- **Library:** Cytoscape.js with `cytoscape-dagre`, vendored with pinned versions and LICENSE files. There is no build step, no npm and no React (ADR-104).
- **Layout:** a left panel with Endpoints / Functions tabs, a search box, and keyboard-navigable lists (`<ul>` with roving focus). The main area has **Call Tree | Flowchart** tabs. A right panel shows details of the selected node or edge: kind, location, confidence, method, reason, alternatives, and the editor link.
- **Call Tree:** a dagre left-to-right layout over `calls` edges from the root (handler, or the selected function). It starts at depth 3. Clicking a node expands one more level. A node already on the current path renders as a `↺ name` back-reference leaf. In **Callers** mode, the reverse edges are walked up to endpoints.
- **Noise control (M0 finding).** Shared error and response helpers made up 16 of 18 dynamic edges in a typical handler.
  - The same callee is shown once per parent, with a `×N` badge for the number of sites.
  - A **package filter** lists every package in the current tree with a checkbox. Packages whose last path element is `shared`, `error*`, `errors`, `utils`, `response` or `logger` start unchecked. Hidden packages collapse into a single `… N helper calls` leaf, so you can still see they exist.
- **Flowchart:** dagre top-to-bottom over the selected function's outline. `if` becomes a diamond with `true`/`false` edges, `switch` becomes a diamond with `case <label>`/`default` edges, and a loop gets `loop`/`exit` edges plus a back edge. `return` and `panic` are terminal pills. A call step is a box. Clicking it expands the callee's outline inline as a sub-cluster, one level per click.
- **Shared selection:** both views read the same in-memory graph object, and switching tabs keeps the selected node ID (ADR-007 kept).
- (CFM-201) The page carries a `Content-Security-Policy` meta: `default-src 'none'`, inline styles allowed, and `script-src` limited to the sha256 hashes of the three inline scripts. Even a string that escaped the JSON encoding could not run or fetch anything. `report.Write` also refuses any inlined asset containing `</script`, `</style` or `<!--`. `cytoscape-dagre` 4.x bundles dagre and graphlib, so there is no separate dagre file. The visual direction lives in `DESIGN.md`.
- (CFM-203) A tree node is an instance (`r/0/2`), not a function ID, so the same function can appear under several parents. Grouped edges take the strongest confidence for line style and label. High edges are unlabelled because the solid line already says it, and a legend in the toolbar explains the styles. Callers mode has no depth limit, since AC needs it to reach endpoints; the 400-node cap bounds it.
- (CFM-204) The flowchart is rebuilt from `function.flow` on every draw: each step returns its open exits, and the next step connects to them. Expanded callees are compound nodes, but edges only ever touch leaf nodes, because dagre does not route edges to clusters. Only `call` steps expand; `go`/`defer` do not, since inlining them would suggest they run in place. Expansion keys are the site path from the root, so the same callee opened at two sites is tracked separately.
- **XSS rule:** every label goes in through Cytoscape data or `textContent`. `innerHTML` is never used with graph strings, because target repos are untrusted input.

## 6. Safety

CFM reads source. It never executes project code. `go/packages` shells out to `go list`, so its environment is pinned:

```
GOFLAGS=-mod=readonly     # never rewrite go.mod/go.sum
GOPROXY=off               # never download; missing modules → diagnostics
GOTOOLCHAIN=local         # never fetch a toolchain named in go.mod
CGO_ENABLED=0             # never invoke the C toolchain on project cgo files
GOWORK=<inherited>        # respect workspaces (user's own setup)
```

- `go list` does not run `go generate`, `init()`, tests or scripts. With the env above, it also runs no C compiler and makes no network call.
- Outputs are written only to the `-o`/`--json` paths. CFM never writes inside the analyzed root unless the user points `-o` there.
- Portable paths are relative to the module root. An absolute root appears in the report only with `--editor`.
- Logs go to stderr, contain no source text, and print paths relative to the root.
- (CFM-108) Calls in an `if`/`switch` init or condition, a `switch` case expression, a `select` comm and a `range` expression come **before** their construct step. Calls in a `for` condition or post statement go inside the loop body. Calls within one statement are listed in evaluation order: arguments come before the call. A range-over-func loop has a `call` step at its `for` keyword just before the `loop` step. `defer`/`go` of a builtin (`defer close(ch)`) is not a step. Literal IDs follow SSA numbering, which also counts the synthetic range-over-func yield closure, so the first source literal after such a loop can be `$2`. A panic while outlining one function becomes `ANALYZER_PANIC` for that function only.

- (CFM-109) Exit codes: 0 ok; 1 the report was written but the analysis was degraded (any `error` diagnostic, or `GO_LOAD_ERROR`/`GO_TYPE_ERROR`); 2 graph validation failed (CFM bug, nothing written); 3 bad usage or unusable input (not a module, no `go`, unwritable output). Flags may come before or after `<dir>`. Nodes from callgraph and routes are deduped by ID (first wins), and flow is attached to function nodes. `stats.blockers` index `WIRE_GEN_MISSING`/`GENERATED_PKG_MISSING`. Callgraph sites are emitted in sorted order so diagnostics are byte-stable. The `--editor` scheme is a `data-editor` attribute on the report's `<html>`, not a graph field: the graph only carries `editorRoot`.

## 7. Testing

- **Golden fixtures:** for each `testdata/fixtures/<name>`, run the pipeline and compare against `expected.json` (byte-for-byte, since output is deterministic). `go test ./... -update` rewrites the goldens. Review every golden diff in the PR.
- **Fixtures** (minimum):

| Fixture | Proves |
|---|---|
| `gin-basic` | verbs, groups, nested groups, `Use` ordering, route middleware, closures as handlers |
| `gin-split-routes` | router passed as a parameter across files, one function reached with 2 prefixes |
| `nethttp` | `HandleFunc`, `Handle` with a `ServeHTTP` type, `HandlerFunc(f)`, wrapper middleware, Go 1.22 patterns |
| `fiber` | verbs, `Group`, `Use`, `All`/`Add` |
| `di-interface` | handler → service iface → repo iface with constructors in `main`, a Wire-style `wire_gen.go`, and a counterfeiter-style `*fakes` package that must **not** appear as a target |
| `dynamic` | non-constant path, router in a map, func value in a map (→ unresolved with diagnostics, never dropped) |
| `wire-missing` | `wire.go` (`wireinject`) with no `wire_gen.go`, and an import of an empty in-module `docs` package → both ADR-107 warnings plus `stats.blockers` |
| `wire-stale` | `wire_gen.go` that no longer type-checks (a provider gained a parameter, and it imports a deleted in-module package) → `WIRE_GEN_STALE` blocker with `wire .`, and no `GENERATED_PKG_MISSING` for the import only `wire_gen.go` needs (CFM-301 finding) |
| `missing-dep` | import that isn't in the module cache → `GO_LOAD_ERROR`, partial graph, no download |
| `flow` | every step kind of GRAPH_SCHEMA §5, pruning, builtins/conversions skipped, 120-rune truncation (hand-written `flow.expected.txt`) |
| `evil` | cgo file, `//go:generate touch PWNED`, `init()` writing a marker, `</script><img onerror=…>` in an `if` condition → assert no marker file, no `PWNED`, and a report that contains no unescaped `</script>` |

- **Unit tests** cover the pure pieces only: `siteKey`, ID formatting, external category table, Go 1.22 pattern parsing, truncation. There are no mocks, and the pipeline is tested through fixtures.
- **Viewer:** one smoke test (`web/smoke_test.go` with `chromedp`) is *optional* and gets added only if viewer regressions start happening. Until then, check by hand against the fixtures' reports.
- **CI:** `gofmt -l`, `go vet`, `staticcheck`, `go test -race ./...`. Fixtures need their deps in the module cache, so CI runs `go mod download` inside each fixture module first. That step is CI-only, and CFM itself never downloads.

## 8. Coding rules

- Standard Go: `gofmt`, wrap errors with `%w`, `context.Context` first on `Load`, no panics on malformed input (recover per package walk and turn it into a diagnostic).
- At most 4 parameters per function. Use an options struct beyond that.
- Deterministic output: sort nodes by ID and edges by (source, target, type, siteKey). No map iteration order may leak into output.
- JSON fields are camelCase, enum values are snake_case, and diagnostic codes are UPPER_SNAKE.
- Dependencies: `golang.org/x/tools` (pinned) and the stdlib. Gin and Fiber are needed **only as fixture deps**, never by CFM itself, because routers are matched by package path and type name.
