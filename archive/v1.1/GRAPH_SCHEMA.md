# Codebase Flow Mapper --- Graph Schema Specification

**Document:** Canonical Graph Schema\
**Version:** 1.0\
**Status:** Proposed MVP contract\
**Format:** JSON

## 1. Purpose

The Graph Schema is the language-independent contract between language
analyzers, graph processing, persistence, APIs, export, and
visualization.

Go and TypeScript adapters must emit equivalent semantic structures for
equivalent source patterns. The frontend must not consume native Go AST
or TypeScript Compiler API objects.

The graph is a **static analysis model**. It is not an OpenTelemetry
trace and does not assert that a particular runtime execution occurred.

## 2. Design principles

1.  Stable IDs within a project and analysis identity.
2.  Explicit node and edge types.
3.  Source evidence on relationships.
4.  Confidence and resolution metadata separate from relationship type.
5.  Distinguish call relationships from control-flow transitions.
6.  Preserve unresolved and ambiguous references.
7.  Support cycles and shared callees.
8.  Version the schema.
9.  Separate canonical graph facts from UI layout and AI explanations.
10. Keep source contents out of the graph by default.

## 3. Top-level document

A graph snapshot has this shape:

``` json
{
  "schemaVersion": "1.0.0",
  "graphId": "graph:project_01:analysis_2026_01",
  "project": {
    "id": "project_01",
    "name": "payment-service",
    "rootFingerprint": "sha256:..."
  },
  "analysis": {
    "id": "analysis_2026_01",
    "status": "completed",
    "startedAt": "2026-11-12T09:00:00Z",
    "completedAt": "2026-11-12T09:00:12Z",
    "analyzerVersion": "0.1.0",
    "configurationFingerprint": "sha256:..."
  },
  "toolchains": {
    "goVersion": "1.x",
    "typescriptVersion": "x.y.z"
  },
  "nodes": [],
  "edges": [],
  "diagnostics": [],
  "statistics": {}
}
```

### Required top-level fields

  Field             Type       Required Description
  ----------------- -------- ---------- --------------------------------------
  `schemaVersion`   string          Yes Semantic schema version
  `graphId`         string          Yes Snapshot graph identity
  `project`         object          Yes Project identity and safe metadata
  `analysis`        object          Yes Analysis run metadata
  `toolchains`      object           No Relevant compiler/parser versions
  `nodes`           array           Yes Canonical nodes
  `edges`           array           Yes Canonical relationships
  `diagnostics`     array           Yes Warnings/errors/unsupported findings
  `statistics`      object           No Counts and analysis metrics

The exported graph must not contain absolute project paths unless the
user explicitly opts in. Source locations use project-relative paths.

## 4. Node model

### 4.1 Common node fields

``` json
{
  "id": "symbol:go:internal/service.CreateTransaction",
  "type": "function",
  "name": "CreateTransaction",
  "qualifiedName": "internal/service.CreateTransaction",
  "language": "go",
  "source": {
    "file": "internal/service/transaction.go",
    "startLine": 20,
    "startColumn": 1,
    "endLine": 55,
    "endColumn": 2
  },
  "status": "resolved",
  "metadata": {},
  "tags": []
}
```

  -----------------------------------------------------------------------------------
  Field             Type                          Required Description
  ----------------- ---------------- --------------------- --------------------------
  `id`              string                             Yes Stable node identifier

  `type`            enum                               Yes Node category

  `name`            string                             Yes Display name

  `qualifiedName`   string                              No Package/module-qualified
                                                           name

  `language`        enum/string                         No `go`, `typescript`,
                                                           `javascript`, etc.

  `source`          object/null                         No Project-relative source
                                                           span

  `status`          enum                               Yes Analysis resolution status

  `metadata`        object                             Yes Type-specific attributes

  `tags`            string\[\]                          No Search/filter labels
  -----------------------------------------------------------------------------------

### 4.2 Node types

MVP node types:

  ------------------------------------------------------------------------------
  Type                    Meaning                        Typical metadata
  ----------------------- ------------------------------ -----------------------
  `project`               Analyzed project               language/framework
                                                         summary

  `package`               Go package or logical module   import path/module

  `file`                  Source file                    file hash, language

  `endpoint`              HTTP route entry point         method, path, framework

  `middleware`            Middleware/interceptor/guard   registration kind

  `handler`               Request handler/controller     HTTP binding
                          method                         

  `function`              Function or method             signature, receiver

  `class`                 Class/controller/provider      decorators, base types

  `interface`             Interface/contract             methods

  `condition`             Conditional decision           expression, construct

  `switch`                Switch/match decision          expression

  `case`                  Switch branch                  case expression/default

  `loop`                  Loop construct                 loop kind, condition

  `return`                Return statement/terminal      return category

  `external_symbol`       Call target outside analyzed   package/module
                          project                        

  `database`              Detected database              driver/technology if
                          system/target                  known

  `query`                 Extracted SQL/query operation  operation, tables if
                                                         known

  `external_service`      External service/API target    host/service only if
                                                         statically known

  `unresolved_symbol`     Call or reference without      call text, reason
                          resolved target                
  ------------------------------------------------------------------------------

Future node types may include `queue`, `event`, `cron_job`, `job`,
`topic`, `variable`, and `data_store`. Unknown future node types must be
safely ignored by older clients while preserving raw data where
possible.

### 4.3 Node status

Allowed values: - `resolved`: identity and relevant source declaration
resolved. - `partially_resolved`: node exists but some semantic
fields/targets are uncertain. - `unresolved`: intended symbol/target
could not be resolved. - `unsupported`: construct recognized but not
analyzed by current adapter.

Node status describes analysis completeness, not runtime health.

### 4.4 Type-specific metadata examples

Endpoint:

``` json
{
  "method": "POST",
  "path": "/api/v1/transactions",
  "framework": "gin",
  "registrationKind": "router_method",
  "routeGroup": "/api/v1"
}
```

Function:

``` json
{
  "package": "internal/service",
  "receiver": null,
  "parameters": [
    {"name": "ctx", "type": "context.Context"},
    {"name": "request", "type": "CreateRequest"}
  ],
  "returnTypes": ["error"],
  "isExported": true,
  "isAsync": false
}
```

Condition:

``` json
{
  "expression": "request.Amount <= 0",
  "construct": "if",
  "hasElse": true
}
```

Loop:

``` json
{
  "kind": "for",
  "condition": "i < len(items)"
}
```

Return:

``` json
{
  "returnKind": "error",
  "expression": "ErrInvalidAmount"
}
```

Do not store full source file contents in node metadata.

## 5. Edge model

### 5.1 Common edge fields

``` json
{
  "id": "edge:sha256:...",
  "source": "symbol:handler.CreateTransaction",
  "target": "symbol:service.CreateTransaction",
  "type": "calls",
  "confidence": "high",
  "resolution": {
    "method": "direct_symbol",
    "status": "resolved",
    "reason": "Call expression resolved to a unique declaration"
  },
  "evidence": [
    {
      "file": "internal/handler/transaction.go",
      "startLine": 35,
      "startColumn": 9,
      "endLine": 35,
      "endColumn": 44,
      "kind": "call_expression"
    }
  ],
  "metadata": {}
}
```

  -------------------------------------------------------------------------------
  Field            Type                          Required Description
  ---------------- ---------------- --------------------- -----------------------
  `id`             string                             Yes Stable edge identity

  `source`         string                             Yes Source node ID

  `target`         string                             Yes Target node ID

  `type`           enum                               Yes Relationship type

  `confidence`     enum                               Yes Evidence-based
                                                          certainty

  `resolution`     object                             Yes Resolution
                                                          method/status/reason

  `evidence`       array                              Yes Source evidence; may be
                                                          empty only for
                                                          explicitly synthetic
                                                          relations

  `metadata`       object                             Yes Relationship-specific
                                                          attributes
  -------------------------------------------------------------------------------

### 5.2 Relationship types

#### Structural relationships

-   `contains`: parent contains a declaration or control-flow construct.
-   `declared_in`: symbol is declared in a file/package.
-   `implements`: type implements an interface/contract.
-   `extends`: class/type extends another type.

#### Application flow relationships

-   `routes_to`: endpoint maps to handler.
-   `uses_middleware`: endpoint/route uses middleware.
-   `calls`: function invokes another function/method.
-   `calls_external`: function invokes an external symbol/service.
-   `depends_on`: component depends on another component/provider.

#### Control-flow relationships

-   `control_entry`: control enters a construct.
-   `branch_true`: true branch from a condition.
-   `branch_false`: false branch from a condition.
-   `case_match`: switch/match case branch.
-   `case_default`: default branch.
-   `loop_body`: loop enters body.
-   `loop_back`: body returns to loop condition.
-   `loop_exit`: loop exits.
-   `returns`: control reaches a return node.
-   `throws`: control raises/panics/throws an error where modeled.
-   `continues`: continue transition.
-   `breaks`: break transition.

#### Data/access relationships (limited MVP/future)

-   `reads`: operation reads a variable/resource.
-   `writes`: operation writes a variable/resource.
-   `queries`: function issues a query.
-   `calls_service`: function invokes a detected external service.

Avoid overloading `calls` to represent control-flow or data
dependencies. Edge type semantics must remain precise.

### 5.3 Edge direction

Edges are directed from the initiating/source entity to the target
entity.

Examples: - Caller `calls` callee. - Endpoint `routes_to` handler. -
Condition `branch_true` branch block. - Function `queries` query node.

For `contains`, source is the containing parent and target is the
contained child.

## 6. Confidence and resolution

### 6.1 Confidence enum

-   `high`
-   `medium`
-   `low`
-   `unknown`

Do not expose a numeric percentage in MVP. Categories are not calibrated
probabilities.

### 6.2 Resolution methods

Recommended values: - `direct_symbol`: direct call resolved to a unique
declaration. - `type_checker`: resolved using compiler/type
information. - `framework_registration`: resolved from known framework
registration pattern. - `interface_dispatch`: candidate target(s)
inferred/resolved through interface implementation. -
`static_inference`: inferred from statically available evidence. -
`name_heuristic`: name-based heuristic; low confidence. - `unresolved`:
no target resolved. - `synthetic`: structural edge generated by graph
normalization, not a direct source call.

### 6.3 Assignment guidance

  --------------------------------------------------------------------------
  Scenario                Confidence              Method
  ----------------------- ----------------------- --------------------------
  Direct call to unique   high                    `direct_symbol`
  local function                                  

  TypeChecker resolves    high                    `type_checker`
  unique TS method                                

  Recognized route        high                    `framework_registration`
  registration maps to                            
  handler                                         

  Interface dispatch has  medium/high,            `interface_dispatch`
  one valid known         adapter-specific        
  implementation under                            
  analyzed config                                 

  Multiple possible       medium                  `interface_dispatch`
  implementations                                 

  Name-only guess         low                     `name_heuristic`

  Dynamic target unknown  unknown                 `unresolved`
  --------------------------------------------------------------------------

Confidence must be assigned by documented analyzer rules. AI output must
not increase edge confidence.

## 7. Evidence model

Evidence records the source span supporting a node or edge.

``` json
{
  "file": "internal/service/transaction.go",
  "startLine": 40,
  "startColumn": 5,
  "endLine": 40,
  "endColumn": 28,
  "kind": "call_expression",
  "snippetHash": "sha256:..."
}
```

Allowed evidence kinds may include: - `declaration` -
`call_expression` - `route_registration` - `middleware_registration` -
`condition_expression` - `branch_body` - `return_statement` -
`type_implementation` - `query_literal` - `configuration`

Store source snippets only when explicitly required and safe. Hashes can
help detect stale evidence without persisting source content.

## 8. Diagnostics

Each diagnostic:

``` json
{
  "id": "diag:001",
  "severity": "warning",
  "code": "CALL_TARGET_UNRESOLVED",
  "message": "Unable to resolve dynamic method target",
  "source": {
    "file": "internal/service/dispatcher.go",
    "startLine": 44,
    "startColumn": 3,
    "endLine": 44,
    "endColumn": 21
  },
  "relatedNodeIds": ["unresolved:dispatcher.Execute"],
  "details": {
    "reason": "dynamic_dispatch"
  }
}
```

Severity: - `info` - `warning` - `error`

Diagnostics must distinguish parse failure, missing dependencies/type
information, unsupported framework patterns, ambiguous targets, dynamic
dispatch, excluded/generated files, and invalid configuration.

## 9. Identity and stable IDs

IDs must be deterministic for the same project identity, source symbol,
and analyzer identity where feasible.

Suggested forms: - `project:<projectKey>` - `file:<relativePath>` -
`endpoint:<framework>:<method>:<path>:<sourceLocation>` -
`symbol:<language>:<qualifiedName>:<sourceLocation>` -
`control:<symbolId>:<constructKind>:<sourceLocation>` -
`external:<module>:<qualifiedName>` -
`unresolved:<language>:<sourceLocation>:<callTextHash>`

Do not use display names alone as IDs; overloaded methods and same-named
functions can collide.

If source movement changes line numbers, IDs may change in MVP. A later
identity layer may use symbol fingerprints and syntax hashes to improve
continuity.

## 10. Traversal semantics

The graph engine supports these traversals.

### 10.1 Forward traversal

Starting at a node, follow selected outgoing edge types.

Use cases: - Endpoint to downstream calls. - Function to callees. -
Service to repositories/external dependencies.

Parameters: - `startNodeId` - `edgeTypes` - `maxDepth` -
`includeUnresolved` - `includeExternal` - `nodeTypeFilter` - `maxNodes`

### 10.2 Reverse traversal

Starting at a node, follow selected incoming edge types.

Use cases: - Function to callers. - Repository to services and
endpoints. - Shared function impact discovery.

### 10.3 Path traversal

Find one or more graph paths between source and target nodes.

Parameters: - `sourceNodeId` - `targetNodeId` - `edgeTypes` -
`maxDepth` - `maxPaths` - `includeLowConfidence`

Path results must preserve edge evidence and confidence. Cycles must be
bounded.

### 10.4 Endpoint reachability

Starting from an endpoint, traverse: 1. `uses_middleware` 2. `routes_to`
3. `calls` 4. relevant control-flow edges 5. optional `queries` /
`calls_external`

The result is a statically reachable subgraph, not a runtime trace.

### 10.5 Impact traversal

Starting from a changed symbol, traverse reverse `calls` and relevant
dependency relationships to identify potentially affected callers and
endpoint roots.

Impact is a candidate set, not proof that every caller's behavior
changes.

### 10.6 Cycle handling

-   Track visited node IDs and edge IDs.
-   Support strongly connected components or cycle markers.
-   Never recursively expand a cycle indefinitely.
-   Render recursive calls and dependency cycles explicitly.
-   Respect `maxDepth` and `maxNodes`.

### 10.7 Traversal defaults

Suggested MVP defaults: - `maxDepth`: 8 - `maxNodes`: 300 - `maxPaths`:
20 - Exclude low-confidence edges unless user enables them. - Include
unresolved nodes with a visible unresolved marker. - Collapse
repeated/shared subgraphs rather than duplicating semantic node
identity.

Defaults are configurable and should be tuned through performance
testing.

## 11. Visualization view models

Visualization layout is not part of canonical graph facts.

### Mind Map view model

May include: - Root node. - Parent/child display relationships. -
Expanded node IDs. - Collapsed node IDs. - Layout coordinates. -
Grouping and display labels.

The view model may derive a tree from a graph, but must preserve
canonical node IDs and indicate when a node has multiple
parents/callers.

### Flowchart view model

May include: - Canonical nodes/edges. - Layout coordinates. - Edge
routing. - Branch labels. - Loop-back rendering. - Grouping/collapse
boundaries.

Neither view may invent semantic edges.

## 12. API representation and pagination

Graph APIs should return bounded subgraphs rather than always returning
the entire graph.

Suggested request:

``` http
GET /api/v1/analyses/{analysisId}/graph/subgraph?rootNodeId=endpoint:...&direction=forward&depth=8&maxNodes=300
```

Suggested response:

``` json
{
  "graphId": "graph:...",
  "schemaVersion": "1.0.0",
  "rootNodeId": "endpoint:...",
  "nodes": [],
  "edges": [],
  "truncated": false,
  "nextCursor": null,
  "diagnosticsSummary": {
    "unresolved": 2,
    "warnings": 1
  }
}
```

Search and traversal APIs must return stable node/edge IDs so the UI can
merge subgraphs without duplicates.

## 13. Schema compatibility and evolution

Use semantic versioning: - Major: breaking field/semantic changes. -
Minor: backward-compatible additions. - Patch: clarifications/fixes
without contract break.

Rules: - Never silently change the meaning of an existing edge type. -
Unknown fields should be tolerated. - Unknown node/edge types should be
preserved or safely ignored by clients. - Export includes
`schemaVersion`. - Analyzer records the schema version it emitted. -
Migration tools are required before removing or renaming fields.

## 14. Validation rules

A graph snapshot is valid only if: 1. Every node has a unique non-empty
ID. 2. Every edge has a unique non-empty ID. 3. Every edge endpoint
references an existing node. 4. Node and edge types belong to supported
enums or are namespaced extensions. 5. Confidence and resolution values
are valid. 6. Source paths are project-relative and normalized. 7.
Line/column values are positive when present. 8. Evidence references
valid project-relative locations. 9. No exported absolute path or source
content is included unless explicitly allowed. 10. `schemaVersion`,
`graphId`, and analysis metadata are present.

## 15. Example: endpoint to service to repository

``` json
{
  "schemaVersion": "1.0.0",
  "graphId": "graph:payment:analysis-001",
  "project": {
    "id": "payment",
    "name": "payment-service",
    "rootFingerprint": "sha256:project-fingerprint"
  },
  "analysis": {
    "id": "analysis-001",
    "status": "completed",
    "analyzerVersion": "0.1.0"
  },
  "nodes": [
    {
      "id": "endpoint:gin:POST:/transactions",
      "type": "endpoint",
      "name": "POST /transactions",
      "language": "go",
      "source": {
        "file": "internal/routes/transaction.go",
        "startLine": 18,
        "startColumn": 2,
        "endLine": 18,
        "endColumn": 65
      },
      "status": "resolved",
      "metadata": {
        "method": "POST",
        "path": "/transactions",
        "framework": "gin"
      }
    },
    {
      "id": "symbol:go:handler.CreateTransaction",
      "type": "handler",
      "name": "CreateTransaction",
      "language": "go",
      "source": {
        "file": "internal/handler/transaction.go",
        "startLine": 20,
        "startColumn": 1,
        "endLine": 42,
        "endColumn": 2
      },
      "status": "resolved",
      "metadata": {}
    },
    {
      "id": "symbol:go:service.CreateTransaction",
      "type": "function",
      "name": "CreateTransaction",
      "language": "go",
      "source": {
        "file": "internal/service/transaction.go",
        "startLine": 15,
        "startColumn": 1,
        "endLine": 60,
        "endColumn": 2
      },
      "status": "resolved",
      "metadata": {}
    },
    {
      "id": "symbol:go:repository.Transaction.Create",
      "type": "function",
      "name": "Create",
      "language": "go",
      "source": {
        "file": "internal/repository/transaction.go",
        "startLine": 10,
        "startColumn": 1,
        "endLine": 28,
        "endColumn": 2
      },
      "status": "resolved",
      "metadata": {
        "receiver": "TransactionRepository"
      }
    }
  ],
  "edges": [
    {
      "id": "edge:route-handler-001",
      "source": "endpoint:gin:POST:/transactions",
      "target": "symbol:go:handler.CreateTransaction",
      "type": "routes_to",
      "confidence": "high",
      "resolution": {
        "method": "framework_registration",
        "status": "resolved",
        "reason": "Gin route registration references this handler"
      },
      "evidence": [
        {
          "file": "internal/routes/transaction.go",
          "startLine": 18,
          "startColumn": 2,
          "endLine": 18,
          "endColumn": 65,
          "kind": "route_registration"
        }
      ],
      "metadata": {}
    },
    {
      "id": "edge:handler-service-001",
      "source": "symbol:go:handler.CreateTransaction",
      "target": "symbol:go:service.CreateTransaction",
      "type": "calls",
      "confidence": "high",
      "resolution": {
        "method": "direct_symbol",
        "status": "resolved",
        "reason": "Direct call resolves to a unique local method"
      },
      "evidence": [
        {
          "file": "internal/handler/transaction.go",
          "startLine": 34,
          "startColumn": 9,
          "endLine": 34,
          "endColumn": 48,
          "kind": "call_expression"
        }
      ],
      "metadata": {}
    },
    {
      "id": "edge:service-repository-001",
      "source": "symbol:go:service.CreateTransaction",
      "target": "symbol:go:repository.Transaction.Create",
      "type": "calls",
      "confidence": "high",
      "resolution": {
        "method": "type_checker",
        "status": "resolved",
        "reason": "Receiver method resolved through Go type information"
      },
      "evidence": [
        {
          "file": "internal/service/transaction.go",
          "startLine": 52,
          "startColumn": 9,
          "endLine": 52,
          "endColumn": 40,
          "kind": "call_expression"
        }
      ],
      "metadata": {}
    }
  ],
  "diagnostics": [],
  "statistics": {
    "nodeCount": 4,
    "edgeCount": 3,
    "unresolvedCount": 0
  }
}
```

## 16. Implementation checklist

-   [ ] Publish JSON Schema for top-level graph document.
-   [ ] Publish JSON Schema for node.
-   [ ] Publish JSON Schema for edge.
-   [ ] Publish enums for node/edge types and confidence.
-   [ ] Implement graph validator in backend.
-   [ ] Implement TypeScript shared types from schema.
-   [ ] Implement Go IR structs and serialization tests.
-   [ ] Add Go adapter conformance fixtures.
-   [ ] Add TypeScript adapter conformance fixtures.
-   [ ] Add schema compatibility tests.
-   [ ] Add export/import round-trip tests.
-   [ ] Add traversal cycle and truncation tests.
-   [ ] Add source path safety validation.
