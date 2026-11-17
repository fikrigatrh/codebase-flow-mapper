# Codebase Flow Mapper — Graph Schema (lean MVP)

**Schema version:** `2.1.0` (2.1 adds request bodies and body tests, §8; a 2.0 reader can ignore them)
**Supersedes:** `archive/v1.1/GRAPH_SCHEMA.md`

The viewer reads this JSON, and `--json` writes the same JSON. JSON fields are camelCase and enum values are snake_case. The Go types in `internal/graph` are the source of truth, and this document has to match them.

## 1. Top level

```json
{
  "schemaVersion": "2.1.0",
  "analyzerVersion": "0.3.0",
  "module": "github.com/acme/payment-service",
  "goVersion": "go1.23.4",
  "generatedAt": "2026-11-12T09:00:00Z",
  "editorRoot": null,
  "nodes": [],
  "edges": [],
  "diagnostics": [],
  "stats": { "packages": 42, "functions": 1810, "endpoints": 37, "edges": 6022, "unresolved": 14, "elapsedMs": 8400, "blockers": [] }
}
```

- `editorRoot` holds the absolute module root, but only when `--editor` is given. Otherwise it is `null`.
- `generatedAt` is the only field that isn't deterministic. Golden tests zero it.

## 2. Nodes

Common fields:

| Field | Type | Req | Notes |
|---|---|---|---|
| `id` | string | yes | §6 |
| `kind` | enum | yes | `endpoint`, `function`, `external`, `unresolved` |
| `name` | string | yes | display name |
| `pkg` | string | no | import path |
| `loc` | Loc | no | declaration / registration span |
| `status` | enum | yes | `resolved`, `partially_resolved`, `unresolved` |

`Loc` = `{ "file": "internal/handler/tx.go", "line": 20, "col": 1, "endLine": 55 }`. The file path is relative to the module root.

Kind-specific fields:

```jsonc
// endpoint
{ "id": "endpoint:gin:POST:/v1/transactions:internal/http/router.go:41",
  "kind": "endpoint", "name": "POST /v1/transactions", "status": "resolved",
  "framework": "gin", "method": "POST", "path": "/v1/transactions",
  "loc": { "file": "internal/http/router.go", "line": 41, "col": 2 } }

// function (also methods and func literals)
{ "id": "fn:github.com/acme/pay/internal/service.(*txService).Create",
  "kind": "function", "name": "(*txService).Create", "pkg": "github.com/acme/pay/internal/service",
  "receiver": "*txService", "signature": "func(ctx context.Context, in CreateInput) (Tx, error)",
  "loc": { "file": "internal/service/tx.go", "line": 30, "col": 1, "endLine": 71 },
  "status": "resolved",
  "flow": [ /* Step[] — §5 */ ],
  "body": { /* Body — §8, only when the function binds a request body */ } }

// external
{ "id": "external:database/sql.(*DB).ExecContext", "kind": "external",
  "name": "(*DB).ExecContext", "pkg": "database/sql", "category": "db", "status": "resolved" }

// unresolved
{ "id": "unresolved:internal/service/dispatch.go:44:12", "kind": "unresolved",
  "name": "h.Handle(...)", "reason": "no_concrete_type", "status": "unresolved",
  "loc": { "file": "internal/service/dispatch.go", "line": 44, "col": 12 } }
```

- `category` is one of `db`, `cache`, `http_client`, `queue`, `log`, `other`.
- `reason` is one of `no_concrete_type`, `func_value_unknown`, `reflection`, `excluded_target`.

## 3. Edges

| Field | Type | Req | Notes |
|---|---|---|---|
| `source` / `target` | node id | yes | |
| `kind` | enum | yes | `routes_to`, `uses_middleware`, `calls` |
| `confidence` | enum | yes | `high`, `medium`, `low`, `unknown` (no percentages) |
| `method` | enum | yes | `static`, `vta`, `framework_registration`, `wrapper_heuristic`, `unresolved` |
| `reason` | string | yes | short human sentence shown in the UI |
| `site` | Loc | yes | evidence: call / registration location |
| `siteId` | string | calls only | `siteKey`, joins the edge to the `call` step in the caller's `flow` |
| `order` | int | middleware only | 0-based position in the chain |
| `alternatives` | int | `low` vta only | number of sibling targets at this site |

```json
{ "source": "fn:…/handler.(*TxHandler).Create", "target": "fn:…/service.(*txService).Create",
  "kind": "calls", "confidence": "medium", "method": "vta",
  "reason": "interface call service.TxService.Create; 1 concrete type reaches this site",
  "site": { "file": "internal/handler/tx.go", "line": 52, "col": 31 },
  "siteId": "internal/handler/tx.go:52:31" }
```

Edges have no `id`. Their identity is `(source, target, kind, siteId|order)`.

## 4. Diagnostics

```json
{ "severity": "warning", "code": "ROUTE_PATH_DYNAMIC",
  "message": "Route path is not a constant; endpoint kept with '?' segment",
  "loc": { "file": "internal/http/router.go", "line": 77, "col": 9 },
  "nodeId": "endpoint:gin:GET:?:internal/http/router.go:77" }
```

- `severity` is `info`, `warning` or `error`.
- Codes: `GO_LOAD_ERROR`, `GO_TYPE_ERROR`, `PACKAGE_EXCLUDED`, `NO_COMPOSITION_ROOT`, `CALL_UNRESOLVED`, `ROUTER_UNSUPPORTED`, `ROUTE_PATH_DYNAMIC`, `ROUTE_PREFIX_UNKNOWN`, `ROUTE_PREFIX_DEPTH`, `HANDLER_UNRESOLVED`, `WIRE_GEN_MISSING`, `WIRE_GEN_STALE`, `GENERATED_PKG_MISSING`, `ANALYZER_PANIC`.
- `WIRE_GEN_MISSING`, `WIRE_GEN_STALE` and `GENERATED_PKG_MISSING` carry `details.fix` (e.g. `"wire ./..."`, `"swag init"`), and their indexes are listed in `stats.blockers` (an int array of indexes into `diagnostics`).

## 5. Flow steps (`function.flow`)

This is an ordered tree of steps. It describes structure inside one function, so it isn't stored as graph nodes (ADR-103).

```jsonc
{ "k": "call",   "site": "internal/handler/tx.go:52:31", "line": 52 }
{ "k": "if",     "cond": "err != nil", "line": 53, "then": [Step], "else": [Step] }   // else-if = else:[{k:"if",…}]
{ "k": "switch", "tag": "in.Type", "line": 60, "cases": [ { "label": "\"card\"", "body": [Step] }, { "label": "default", "body": [Step] } ] }
{ "k": "loop",   "head": "_, item := range items", "line": 70, "body": [Step] }
{ "k": "select", "line": 80, "cases": [ { "label": "<-ctx.Done()", "body": [Step] } ] }
{ "k": "go",     "site": "…", "line": 90 }      // also "defer"
{ "k": "return", "line": 95 }
{ "k": "panic",  "line": 99 }
```

- `call.site` matches `edge.siteId`. One site may have several edges (`low` alternatives) or one `unresolved` target.
- The pruning rule (ARCHITECTURE §4.3) applies: no step without a call, return or panic beneath it.
- An `if` step and a switch `case` may carry `test` (§8) when the condition reads a bound body type.
- `cond`/`tag`/`head`/`label` are expression text up to 120 runes. This is the only source text stored (PRD NFR-4).

## 6. IDs

Go has no overloading, so a qualified name is unique within a build and IDs leave out the line number. That keeps them stable when code moves.

| Kind | Format |
|---|---|
| function | `fn:<pkgPath>.<Name>` / `fn:<pkgPath>.(<*Recv>).<Name>` / literal: `fn:<parentId-without-prefix>$<n>` (SSA numbering) |
| external | `external:<pkgPath>.<Name>` (same receiver form) |
| endpoint | `endpoint:<framework>:<METHOD>:<path>:<file>:<line>` (location kept because duplicates are legal) |
| unresolved | `unresolved:<file>:<line>:<col>` |

Generic instantiations collapse onto the generic function's ID.

## 7. Validation (`graph.Validate`, run before writing)

1. Every edge's `source`/`target` refers to an existing node.
2. `routes_to` goes endpoint → function, external or unresolved. `uses_middleware` goes endpoint → function, external or unresolved (external: e.g. `gin.Recovery()`, a dependency's handler). `calls` goes function → function, external or unresolved.
3. Every `calls` edge's `siteId` appears as a `call`/`go`/`defer` step in the source function's `flow`, and vice versa.
4. IDs are unique, and nodes and edges are sorted.
5. No absolute path appears anywhere unless `editorRoot` is set.

6. Body field `kind`s and test `op`s are from the enums in §8, and each op has the operands it needs.

A validation failure is a CFM bug: the run exits with code 2 and doesn't write a report.

## 8. Request bodies and body tests (2.1)

A function that binds a request body gets `body`. Binders: Gin `ShouldBindJSON`, `BindJSON`, `ShouldBind` and `Bind`; Fiber v2 `BodyParser`; and `json.NewDecoder(r.Body).Decode(&x)` with `r` an `*http.Request`. Only the first bind in a function counts.

```jsonc
{ "type": "example.com/app/dto.TopupReq", "binder": "ShouldBindJSON", "line": 21,
  "fields": [
    { "name": "source", "kind": "object", "type": "example.com/app/dto.Amount",
      "fields": [ { "name": "currency", "kind": "string" }, { "name": "value", "kind": "number" } ] },
    { "name": "targetCurrency", "kind": "string", "nullable": true },
    { "name": "tags", "kind": "array", "nullable": true, "elem": { "kind": "string" } } ] }
```

- `kind` is one of `string`, `integer`, `number`, `boolean`, `object`, `array`, `any`.
  - `time.Time` is `string`.
  - A type with its own `UnmarshalJSON` is `any`.
  - A map is an `object` with no `fields`.
- `name` is the json tag name, or the Go field name if the field has no tag. `json:"-"` and unexported fields are left out, and an untagged embedded struct is flattened.
- `nullable` means JSON `null` leaves Go `nil`: pointers, slices, maps and interfaces.
- `type` is set on named types, so a test can find a nested struct inside a body.

A **test** describes a condition that compares a field of any bound type (the root or a nested struct) with a constant. It is set in every project function, because DTOs are passed on to usecases.

```jsonc
{ "op": "and", "args": [ { "op": "truthy", "ref": { "type": "…/dto.TopupReq", "path": ["express"] } },
                         { "op": "gt", "ref": { "type": "…/dto.TopupReq", "path": ["source", "value"] }, "value": 1000 } ] }
{ "op": "eq", "ref": { "type": "…/dto.TopupReq", "path": ["source", "currency"], "fold": ["trim", "upper"] }, "value": "HKD" }
```

- **`op` values:**
  - `and`, `or`: one or more `args`.
  - `not`: one arg.
  - `eq`, `ne`, `lt`, `le`, `gt`, `ge`: a `ref` and a constant `value`, which is any JSON constant (`null` for `nil`).
  - `truthy`: a bool `ref`.
  - `unknown`: a part CFM can't evaluate, such as a field compared with a variable, or a non-body operand of `&&`.
- `ref.path` starts at the outermost bound type the expression reaches.
- `ref.fold` lists the string functions applied, in order: `strings.TrimSpace` → `trim`, `ToUpper` → `upper`, `ToLower` → `lower`. `strings.EqualFold(x, "c")` becomes `eq` with `lower` and a lower-cased value.
- **Switch cases:**
  - In a tagged switch on a body field, each `case` gets `eq`, or `or` over `eq`s.
  - In a tagless switch, each case gets its own expression.
  - `default` gets `and` over `not(case)` for every other case, including cases pruned from the outline.
- A value copied into another struct or read from a database is not traced and has no test.

The viewer evaluates tests against the body the user types in, using Go semantics (a missing field is its zero value, and keys match case-insensitively). The result is true, false or can't tell. CFM itself never evaluates anything.
