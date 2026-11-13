# Design direction (viewer report)

Transcribed from the project owner's brief (2026-11-13). The agent only formatted it.

- **Theme:** dark by default, with a toggle to light. Both modes must work.
- **Character / mood:** IDE-like: dense like an editor panel, lots of monospace, high contrast; it should feel like part of the toolchain.
- **Typography:** system fonts only: `system-ui` for text, `ui-monospace` for paths and code. No embedded fonts (zero extra bytes, native feel on each OS).
- **Accent:** amber, one accent only. Edge confidence is shown by line style, not colour (FR-16).

Dial: ENERGY 1 / RHYTHM 1 / MOTION 1 (agent's reading of "IDE-like" for a single-screen tool; not part of the brief).
