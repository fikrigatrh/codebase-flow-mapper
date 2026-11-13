# Codebase Flow Mapper — Engineering-Ready Implementation Package

**Package version:** 1.1  
**Status:** Proposed MVP implementation baseline  
**MVP languages:** Go + TypeScript  
**Execution model:** Local-first static analysis

## Document index

| Document | Purpose |
|---|---|
| `PRD.md` | Product scope, requirements, user stories, acceptance criteria and release gates |
| `ARCHITECTURE.md` | System context, modules, analysis pipeline, adapter contracts, API, persistence, UI and security |
| `GRAPH_SCHEMA.md` | Canonical graph nodes, edges, evidence, confidence, traversal and serialization |
| `ADR.md` | Accepted architectural decisions and rationale |
| `SPRINT_BACKLOG.md` | Ordered sprint plan, task IDs, dependencies, target files, implementation notes, acceptance criteria and tests |
| `IMPLEMENTATION_GUIDE.md` | Recommended vertical slice, implementation sequence, error policy and AI-agent prompt template |
| `CODING_STANDARDS.md` | Go, TypeScript, frontend, graph, diagnostics and dependency conventions |
| `TESTING_STRATEGY.md` | Fixture matrix, contract, integration, security, performance and CI strategy |
| `AGENTS.md` | Repository-wide instructions for AI coding agents and contributors |

## Baseline decisions

- Local-first application; browser UI served by a local Go process.
- Go backend/orchestrator.
- Go analysis through Go-native AST/type tooling and `golang.org/x/tools`.
- TypeScript analysis through a pinned TypeScript Compiler API running in a packaged Node worker.
- Tree-sitter is supplementary for future language expansion, not the sole MVP semantic analyzer.
- One canonical graph powers Mind Map and Flowchart.
- React + TypeScript UI, React Flow rendering, ELK.js layout.
- SQLite for metadata and versioned JSON graph snapshots.
- AI optional; graph facts originate from static analysis.
- No target-project code execution or dependency installation.
- Export paths are project-relative by default.

## Recommended implementation order

1. Review and approve PRD scope and accepted ADRs.
2. Freeze `GRAPH_SCHEMA.md`, publish `schema/graph.schema.json`, and establish adapter/worker contracts.
3. Build project scanner and analysis job lifecycle.
4. Deliver a Go endpoint → handler → service → repository vertical slice.
5. Implement graph validation, persistence, bounded traversal and API.
6. Build Flowchart and Mind Map as projections of the same canonical graph.
7. Implement TypeScript worker and analyzer parity.
8. Add security, performance, export, packaging and release gates.

## Important semantic warning

The graph represents statically discovered or inferred relationships. It is not a runtime trace and does not prove that a particular branch or call executes for a given request.

## Package status and assumptions

This is an implementation-ready baseline, not a claim that the product code already exists or that every framework pattern can be resolved perfectly. The sprint estimates and safety limits are initial planning values and must be validated against representative repositories. Platform support, licensing, exact toolchain versions, and distribution details must be finalized before release.
