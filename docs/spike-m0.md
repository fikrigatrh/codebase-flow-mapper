# M0 Spike Results (CFM-001)

**Date:** 2026-11-12
**Code:** `spike/main.go` (throwaway). Toolchain: go1.27.3, `golang.org/x/tools v0.52.0`.
**Verdict:** **Gate passed, with one blocker that changes the product: generated files.**

## Setup

- `go run ./spike -dir <module> [-summary] [-func <substr>] [-depth N] [-lean] [-scope prefix]`
- Roots are picked automatically: every function with a `*gin.Context`, `*fiber.Ctx` or `http.ResponseWriter` parameter. This also picks up middleware and response helpers. The real product roots from routes (CFM-002, which hasn't been done yet).
- The safety env from ADR-105 was used on every run. No network call was made, and no target repo was modified. For `service B`, `wire`/`swag` were run on a **copy** in the scratchpad.

## Results

| Target | Composition root | Handlers reaching `/repo` | Dynamic (interface) edges | Time | Max RSS |
|---|---|---|---|---|---|
| `spike/fixture-di` (stdlib) | `wire_gen.go` present | 1/1 | 2 (both single-target) | 0.4 s | 354 MB |
| `service B` as checked out | **`wire_gen.go` gitignored → absent** | 0 interface hops (stops at usecase) | **0** | 14.7 s | 2.7 GB |
| `service B` copy + `swag init` + `wire` | present | 31 summaries mention `/repo` | 95 | 10.4 s | 2.8 GB |
| `service A` | present | 42 summaries mention `/repo` | 2242 | 16–30 s | 3.3–3.6 GB |
| `service A` (root module) | — | 0 roots | — | 1.9 s | 1.2 GB |

**Hand-checked trees** (each line was checked against the source):
- Service A: a list handler → its usecase [vta x1] → its repository [vta x1]. Correct.
- Service B: a top-up handler → its usecase [static] → 4 calls on the repository interface field → the one repository implementation [vta x1]. All 4 sites are correct.
- fixture: the unwired `memRepo` and the `repofakes` package never appear. That is the CHA problem avoided.

**Gate (≥4/5 handlers per repo reach repo through ≥1 interface hop, no fakes):**
- Service A: all 5 sampled handlers reach `*/repo`. **5/5**
- Service B (with generated files): all 5 sampled handlers reach `internal/repo`. **5/5**
- No fake or mock type appeared in any run.

The tally for the other handlers is based on package reach in the summaries, not on a hand check of each tree. CFM-301 does the full hand check.

## Findings that change the docs

1. **Generated files are missing in real checkouts. This is the #1 blocker.** Both repos gitignore `wire_gen.go` and the swag `docs/` package. Without `wire_gen.go`, VTA finds **zero** interface targets, so the product would look broken. CFM must not run `wire` or `swag` (ADR-105). It has to detect the situation and say exactly what to run.
   → New diagnostics `WIRE_GEN_MISSING` and `GENERATED_PKG_MISSING`, a run summary banner, and a README prerequisite. See ADR-107.
2. **Noise from shared error/response helpers.** In one service A list handler, 16 of 18 dynamic edges are `internal/shared` / `internal/error` helpers repeated at every `if err != nil`. The domain path is 2 edges.
   → Viewer: dedupe repeated callees per parent, plus a package filter that hides `shared`/`error`/`utils`-style packages. Most of these calls sit inside `if err != nil` branches, which the Flowchart already isolates.
3. **Dead handlers.** A legacy handler type isn't constructed in `wire_gen.go` or routed, so its usecase calls have no concrete type and come out unresolved. That is correct, and route-rooting hides them anyway. The spike drops unresolved sites, but the product must show them (FR-04 already says so).
4. **Memory is the real performance risk, not time.** Full mode on 68–300-file services uses 2.8–3.6 GB RSS, which is close to the NFR-3 limit of 4 GB. Larger services (service A has 586 files) may exceed it.
   - Lean mode (SSA bodies only for module packages, plus the router packages) is 7× faster with 40% less RSS. It is **lossy** for an unknown reason: service A repo reach drops from 42 to 7, while service B is identical. Keeping the router packages' bodies did **not** fix it, and an org-wide scope didn't either.
   - → Full mode stays the default. Lean mode is an M3 investigation (CFM-302), not an MVP feature.
     - Resolved in CFM-302: the loss came from the spike's `inModule` function filter, which drops synthetic pointer wrappers. Building bodies only for module packages is lossless for the call paths cfm shows, and it is now the default (`docs/validation.md`, Performance).
5. **Multi-module repos.** The `service A` root module is broken (it imports a package that doesn't exist), and the live code is in the `v2/` module. CFM analyzes one module directory, and the README must say to point it at the right one.
6. **x/tools v0.52.0 requires `go 1.26` in CFM's own go.mod.** CFM isn't affected by the target's Go version, as long as the local toolchain can load it (`GOTOOLCHAIN=local`).

## Not done in M0

- **CFM-002 (route extraction spike) wasn't run.** Its pattern risks are already specified in ARCHITECTURE §4.2, and the M1 fixtures test them directly. Recommendation: fold it into CFM-105 instead of spiking it separately.
