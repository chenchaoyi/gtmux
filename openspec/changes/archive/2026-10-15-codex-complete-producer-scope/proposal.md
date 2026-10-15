# Share Codex discovery across complete producers

## Why

After gather-level indexing, digest content and usage still discover rollout paths per
session. Two fixture sessions incur four additional discoveries after their gather.
Ordered HQ sensors also repeat discovery while correctly resampling mutable state.

## What changes

Create a short-lived collection owning a lazy path scope. Digest/usage carry that scope
through content and accounting; ordered HQ sensors reuse it while collecting fresh rows
at each step. Public direct readers remain fresh and driver content switches still apply.
This caches paths only, never status, transcript facts, consent or authority decisions.

## Surfaces

- 终端 (terminal / attach): digest/usage keep their output and matching contracts with fewer discoveries.
- 菜单栏 (menubar): shared core producers benefit without UI changes.
- 手机 (phone): shared HTTP producers benefit without protocol changes.
- iPad: same shared producers as phone.
- Web: same shared producers as phone.

## Non-goals

No cross-request cache, global status snapshot, new HTTP surface, altered usage file
identity, or reordering of state-changing HQ sensors. Limits and daily-history catalog
updates retain their separate bounded/cache policies.
