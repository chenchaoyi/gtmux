# Local build storage workflow

[中文](development-storage.zh.md)

Use this workflow for iOS/macOS development, simulator verification and release
preparation. Reuse compatible caches and close the storage lifecycle of each task.
This is an operator workflow; it does not install a cleanup daemon or grant wider
deletion authority. Existing task authorization applies; do not ask again for an
already authorized cleanup scope.

## 1. Register and measure before building

Record a small private task manifest: owner/session, repository and source commit,
Xcode/SDK/architecture, dependency lockfile hashes, each output path, its purpose,
rebuild command, expected size, retention reason and active users. Use distinct paths
for rebuildable output, evidence and signed release/recovery material.

- Put rebuildable output in a named task/lane below a local build-cache root, such
  as `~/Library/Caches/gtmux-dev/<repository>/<task>/build/`. Keep screenshots,
  failure/success logs and receipts in the task's private evidence directory.
- Keep signed IPA, matching `.xcarchive`/dSYMs, publication receipts and signing
  recovery material in private durable storage. Do not mix them with disposable
  DerivedData. Do not place Pods, dependencies or build trees in HQ knowledge/notes.
- Save timestamped `df -k /System/Volumes/Data /System/Volumes/VM` readings and exact
  available bytes. Preserve earlier samples. APFS volumes share capacity: pool
  usage from `statvfs` alone does not distinguish Data growth from VM growth.
- Measure non-overlapping candidate roots with `du -skx`. A directory total,
  sparse file, clone or mounted simulator image is not an exact reclaim estimate.

The following are conservative project review thresholds, not vendor limits or
permission to delete. Review them against the actual task and recent measurements:

| Check | Default action |
| --- | --- |
| Fresh local iOS archive/dependency export | Plan for 25 GiB free; prefer the existing hosted delivery if unavailable. An evidence-based smaller estimate must still retain a 2 GiB safety floor. |
| Simulator verification with fresh dependencies | Budget 8 GiB for the task; measure again before exceeding it or switching toolchains. |
| Less than 10 GiB free, or continuing VM growth | Reassess the next write, reuse an existing validated artifact or use hosted builds. Below 2 GiB, stop large writes and report the exact blocker. |
| Shared Go build cache above 12 GiB, or a completed task above 1 GiB | Review ownership and reuse; report a bounded cleanup plan. These are alerts, not automatic eviction. |

## 2. Reuse by compatibility, isolate concurrent writers

| Storage | Reuse and ownership |
| --- | --- |
| npm, CocoaPods and gem download caches | Prefer the existing package manager's cache. Do not create a full private download cache for every worktree unless isolation requires it; register any override and its retirement plan. |
| `node_modules`, `Pods`, installed gems | Reuse inside a compatible maintained worktree. Install from its lockfiles. Do not symlink one mutable dependency installation into unrelated or concurrently changing worktrees. |
| Xcode/SwiftPM/WDA products and indexes | Keep an active lane warm during iteration. Compatibility includes repository/workspace path, Xcode build, SDK/platform, architecture, lockfiles and build settings. Let the build tool validate reuse; a matching app version alone is insufficient. |
| Go build/test cache | Reuse the tool's shared cache and normal garbage collection. Do not routinely run `go clean -cache` or delete individual hash files to meet a disk target. A separately authorized global reset discards useful entries and requires a rebuild allowance. |
| An already verified IPA or simulator build | Reuse the same bytes for matching source/configuration. Distribution retries and switching between compatible test devices do not justify another build. |

Avoid a new worktree for a documentation-only follow-up when an owned, clean,
compatible worktree can be repurposed. Do not take over another task's worktree.
Changing toolchains or dependency locks invalidates any assumed reuse; record the
new compatibility boundary rather than silently accumulating another build tree.

## 3. Verify cleanup scope before removing anything

1. Confirm task ownership and completion from current receipts/CI; preserve both
   failed and passing evidence. Age, a familiar prefix and a successful build alone
   do not establish disposability.
2. Check the exact directory with bounded `lsof -nP +D <candidate>`, and check active
   builds/devices and the tool's current project/DerivedData settings. A timeout or
   permission error means unknown; an empty handle result alone is not approval.
3. Before removing a worktree, verify a clean status, inspect ignored files for
   local settings/secrets, and verify actual merge delivery. A squash candidate
   need not be an ancestor of main: use the PR's merge commit/tree evidence.
4. Preserve small local settings and hash the retained evidence, signed packages
   and recovery files. Recheck the candidate's resolved path, inode/device, owner,
   symlinks and nested mounts immediately before the authorized operation.
5. Remove an owned completed worktree through normal `git worktree remove`; never
   remove its shared Git clone/objects as a side effect. If Git refuses, inspect the
   reason; do not add `--force` or use blanket `git clean -fdx` to bypass it.

Do not delete swap, snapshots, SDKs, simulator runtimes, global caches, personal
files, another task's material or signing/recovery data under a routine task cleanup.
Do not kill/restart other processes to make a candidate appear unused. Broader
cleanup requires its own actual scope and authority.

## 4. Close the task before calling it complete

- Stop only owned temporary test drivers and release their settings/leases. Preserve
  the selected devices and original app data; do not erase simulators as cleanup.
- After evidence is retained and authorization is established, remove completed
  task-local intermediates and dependency/worktree copies. A warm cache kept for a
  concrete follow-up needs an owner, compatibility key, byte budget and next review.
- Default to no unowned completed build tree. Shared caches and formal recovery
  artifacts remain managed stores, not abandoned task folders. Review old releases
  only after acceptance and verified recovery/retention requirements; version count
  or age alone is not permission to remove them.
- Independently verify path absence, retained-file hashes and remaining Git roots.
  Save final Data/VM readings, failures/skips and every retained path/reason. Report
  directory allocation removed separately from observed pool-free-space change.
- Finish with a cleanup receipt and a small knowledge entry/pointer. If a candidate
  is still active or unknown, hand off that exact exception and next action; do not
  silently leave gigabytes or a background promise behind.

See [iOS delivery](../mobileapp/DISTRIBUTION.md) for the signed-package boundary.
The private manifest/cleanup receipt must never enter Git, public CI artifacts or
a knowledge entry with credentials, device identities or private installation URLs.

## References

- [Apple: project-generated file locations](https://help.apple.com/xcode/mac/current/en.lproj/deva52afe8a4.html)
- [Go: build and test caching](https://pkg.go.dev/cmd/go#hdr-Build_and_test_caching)
- [npm: package cache](https://docs.npmjs.com/cli/v11/commands/npm-cache/)
