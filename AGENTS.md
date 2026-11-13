# AGENTS.md — Codebase Flow Mapper

## Before changing code

1. Read `PRD.md`, `ARCHITECTURE.md`, `GRAPH_SCHEMA.md`, `ADR.md` and `SPRINT_BACKLOG.md`. `archive/v1.1/` is reference only and is **not** the spec.
2. Identify the exact backlog task ID and implement only that task, plus its explicit dependencies.
3. If the docs conflict or don't specify something, stop and ask one focused question instead of inventing a contract.

## Invariants

- Go-only MVP. Frameworks: Gin, net/http, Fiber v2.
- Static analysis only. Never execute target code. Keep the safety env from ARCHITECTURE §6 and ADR-105.
- Never create a call edge from a name match. Unresolved stays visible, with a reason and a diagnostic.
- No server, database, AI or network (ADR-101). Proposing one requires a new ADR.
- Output is deterministic and goldens are byte-stable.
- Graph strings are untrusted. The viewer uses `textContent` only and never `innerHTML` with them.
- Portable paths are relative to the module root.

## Contract changes

A schema change must update `GRAPH_SCHEMA.md`, `internal/graph` types, `Validate`, the affected goldens and `viewer.js` in the same change. Changing an accepted ADR requires a new ADR that supersedes it.

## Validation

Run `gofmt -l .`, `go vet ./...`, `staticcheck ./...` and `go test -race ./...`. If a command can't run, say so and why. Never claim tests pass without running them.

## Completion report

Include the task ID, the files changed, the AC mapping (pass/fail/not run), the commands run with their exact results, any known limitations, and the impact on schema and ADRs.
