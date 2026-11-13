# Codebase Flow Mapper — Implementation Guide

**Version:** 1.0  
**Audience:** Engineers and AI coding agents implementing the MVP.

## 1. Source-of-truth order

When documents appear to conflict, use this order and raise a decision rather than guessing:

1. `GRAPH_SCHEMA.md` for canonical graph facts and serialization.
2. `PRD.md` for product scope and user-visible behavior.
3. `ARCHITECTURE.md` for component boundaries, API and runtime contracts.
4. `ADR.md` for accepted technology decisions.
5. `SPRINT_BACKLOG.md` for task sequencing and acceptance criteria.
6. This guide for implementation workflow.

If a conflict cannot be resolved from these documents, stop the affected task and create an issue/ADR proposal.

## 2. Recommended build sequence

1. Freeze graph schema and publish JSON Schema.
2. Define adapter and worker protocol.
3. Build a tiny Go vertical slice: scanner → one `net/http` route → handler → direct service call → repository function → JSON graph.
4. Validate/persist graph snapshots.
5. Add bounded graph queries and API.
6. Build Flowchart, then Mind Map as a separate projection of the same graph.
7. Add Go framework recognizers.
8. Implement TS worker and TypeScript Compiler API adapter.
9. Add hardening, export, packaging, and release gates.

Do not build the complete UI before the first graph can be generated and validated from a real fixture.

## 3. First vertical slice

Use the following deliberately small fixture:

```go
package main

import "net/http"

func main() {
    http.HandleFunc("/payments", paymentHandler)
}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    createPayment()
}

func createPayment() { savePayment() }
func savePayment() {}
```

Expected high-level graph:
- endpoint node for `POST /payments` only if method narrowing is statically supported; otherwise represent route registration and handler separately and do not infer method restriction;
- handler function node;
- branch/control-flow node for method condition;
- `createPayment` call edge;
- `savePayment` call edge;
- return/terminal paths;
- source evidence for each relationship.

The exact node/edge shape must follow `GRAPH_SCHEMA.md`; this example is a test target, not permission to add schema fields ad hoc.

## 4. Development commands

The exact commands are finalized in Sprint 0 when tool versions are pinned. Recommended root-level interface:

```bash
make bootstrap
make format
make lint
make test
make test-go
make test-web
make test-worker
make test-integration
make build
```

Each command must be deterministic and documented. Avoid scripts that install project dependencies implicitly. Dependency installation is a developer setup step, never part of analysis of a target repository.

## 5. Adapter implementation checklist

For every language/framework adapter:

- Detect supported project/framework with explicit evidence.
- Report toolchain and adapter versions.
- Build a symbol index before resolving calls.
- Emit stable symbol identities.
- Attach source locations to declarations and relationships.
- Keep unresolved and ambiguous calls explicit.
- Separate call edges from control-flow edges.
- Return partial graph plus diagnostics when possible.
- Pass the common graph schema validator.
- Add positive, negative, and ambiguous fixtures.
- Document unsupported patterns.

## 6. Static-analysis correctness policy

Static analysis describes possible or statically supported relationships. It does not prove a runtime path occurred.

- A direct call with a resolved symbol may be labeled resolved.
- Interface dispatch, virtual dispatch, dynamic imports, reflection, dependency injection, decorators, generated code, and runtime route registration may be partial.
- A branch is a possible static branch; do not claim it was taken.
- A repository-like method name is not proof that a function is a repository.
- Framework role labels require framework registration evidence or an explicit user annotation.
- Do not turn a diagnostic into a fake node or edge merely to make the graph look complete.

## 7. Error and partial-result handling

Prefer typed errors internally and stable diagnostic codes externally.

Fatal:
- invalid project root;
- unsupported graph schema;
- unrecoverable persistence failure;
- worker protocol corruption that prevents safe continuation.

Recoverable:
- one package fails to load;
- one TS config is invalid;
- unresolved symbol;
- unsupported framework route pattern;
- file exceeds configured size limit.

A recoverable issue should not discard successful findings from other files/packages. Mark the analysis `completed_with_warnings` when useful results exist but some work was incomplete.

## 8. API implementation rules

- Validate every request at the HTTP boundary.
- Keep HTTP DTOs separate from persistence models and analyzer-native structures.
- Return stable error codes, not raw stack traces.
- Apply server-side depth and result limits.
- Make cancellation explicit.
- Include request IDs in logs and error envelopes.
- Do not expose arbitrary file-read endpoints.

## 9. Frontend implementation rules

- Treat the API graph as immutable data.
- Keep layout state separate from graph facts.
- Use stable node IDs as React keys and graph identities.
- Keep selected node, active analysis, view mode, and filters in URL/state where useful.
- Make uncertainty visible in both views.
- Provide a diagnostics panel and explain truncation.
- Never imply that a static path is a runtime trace.

## 10. AI coding-agent task prompt template

Use one task per prompt:

```text
You are implementing one task in Codebase Flow Mapper.

Read first:
- AGENTS.md
- SPRINT_BACKLOG.md (the exact task)
- ARCHITECTURE.md
- GRAPH_SCHEMA.md
- relevant ADRs

Task: CFM-XXXX — <exact title>

Rules:
1. Implement only this task and its explicit dependencies.
2. Do not change canonical graph semantics without updating GRAPH_SCHEMA.md,
   schema/graph.schema.json, fixtures, and proposing an ADR/schema decision.
3. Do not execute target project code or install target project dependencies.
4. Preserve unresolved relationships and diagnostics; do not guess.
5. Add tests for acceptance criteria and failure cases.
6. Run relevant format, lint, typecheck, and test commands.
7. Report changed files, design choices, commands run, results, limitations,
   and any follow-up task required.
8. If requirements are ambiguous or contradictory, stop and ask instead of inventing behavior.

Before coding, summarize the task and its acceptance criteria in your own words.
After coding, map each acceptance criterion to code/tests and report pass/fail.
```

## 11. Pull request template

```markdown
## Summary
<!-- What changed and why -->

## Task
- [ ] CFM-XXXX

## Contract impact
- [ ] No API/schema/ADR impact
- [ ] API changed (describe)
- [ ] Graph schema changed (version/compatibility plan)
- [ ] ADR proposed/updated

## Acceptance criteria
- [ ] AC 1
- [ ] AC 2

## Tests
| Command | Result |
|---|---|
| | |

## Limitations / follow-ups
<!-- Explicitly list uncertainty and unsupported cases -->
```
