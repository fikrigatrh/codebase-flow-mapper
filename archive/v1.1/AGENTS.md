# AGENTS.md — Codebase Flow Mapper

This file contains repository-wide instructions for AI coding agents and human contributors.

## 1. Before changing code

Read:
- `PRD.md`
- `ARCHITECTURE.md`
- `GRAPH_SCHEMA.md`
- `ADR.md`
- `SPRINT_BACKLOG.md`
- `IMPLEMENTATION_GUIDE.md`
- `CODING_STANDARDS.md`
- `TESTING_STRATEGY.md`

Then identify the exact backlog task ID. Do not implement an entire milestone unless explicitly requested.

## 2. Product invariants

- CFM is local-first.
- MVP languages are Go and TypeScript.
- Analysis is static; it is not runtime tracing and does not prove execution.
- No OpenTelemetry instrumentation is required.
- Do not execute target application code, project scripts, tests, generators, or package lifecycle hooks.
- Do not install target-project dependencies.
- AI is optional and cannot create or silently modify canonical graph facts.
- Mind Map and Flowchart must use the same canonical graph snapshot.
- Preserve unresolved and ambiguous relationships with diagnostics/evidence.
- Never fabricate a call edge from a name match alone.

## 3. Contract discipline

- `GRAPH_SCHEMA.md` and `schema/graph.schema.json` define graph contracts.
- `ARCHITECTURE.md` defines component and API boundaries.
- Keep adapter-native AST/compiler objects out of the API and frontend.
- Any schema change must update JSON Schema, Go types, TS types, fixtures, validators, both graph projections, and migration/versioning notes.
- Any API change must update API documentation and integration tests.
- Do not silently change accepted ADR decisions; propose an ADR amendment.

## 4. Implementation discipline

- Implement only the requested task and explicit dependencies.
- Prefer small, reviewable changes.
- Add tests before or alongside implementation.
- Use deterministic output and stable identifiers.
- Bound file sizes, graph expansion, concurrency, and analysis time.
- Respect cancellation.
- Keep source paths project-relative in portable graph data.
- Do not log source contents, secrets, or unnecessary absolute paths.
- Do not use shell command strings with project-controlled input.
- Never expose an arbitrary filesystem read API.

## 5. Required validation

Run all relevant commands available in the repository. At minimum:
- formatter;
- linter;
- typecheck;
- unit tests;
- fixture/contract tests for analyzer or graph changes;
- security tests for filesystem/API/worker/rendering changes.

If a command cannot run, state why. Do not claim tests passed unless they were actually run.

## 6. Completion report

At the end of each task, report:
1. Task ID and summary.
2. Files changed.
3. Acceptance criteria mapping (pass/fail/not run).
4. Commands run and exact results.
5. Known limitations or uncertainty.
6. Follow-up tasks/issues.
7. Schema/API/ADR impact.

If requirements conflict or are underspecified, stop and ask a focused question rather than inventing a contract.
