# gtmux — repo guide for Claude Code

## Repo map (which twin is the deployed one — don't edit the wrong side)

| Path | Role |
|---|---|
| `cmd/gtmux` + `internal/` | the Go CLI core (cgo-free) — the single data source |
| `macapp/` | macOS menu-bar app (Swift/AppKit), pure consumer of `agents --json` |
| `mobileapp/` | iOS app (bare RN, `com.gtmux.app`), third surface |
| `relay-worker/` | **DEPLOYED push relay** (Cloudflare Worker, `gtmux-relay.ccy.dev`) — changes need `npx wrangler deploy` |
| `relay/` | Go **self-host reference impl** of the relay — NEVER deployed; keep payload shape in sync with `relay-worker/` |
| `tunnel-worker/` | **DEPLOYED tunnel provisioner** (Cloudflare Worker, `api.gtmux.ccy.dev`) |
| `deploy/self-tunnel/` | self-host tunnel-server configs/docs (Caddy + chisel) |
| `api/contract.md` | v0 HTTP/SSE contract (serve ↔ apps) |
| `docs/design/` | the design authority (DESIGN/MOBILE/HANDOFF + research) |
| `openspec/` | `specs/` = what IS built · `changes/` = in-flight only |
| `scripts/check-design.sh` | CI design/architecture conformance gate |
| `ops/` | untracked, operator-private (secrets/runbooks) — never commit |

**gtmux** is a command center for tmux sessions and coding agents. Two surfaces
over one Go core (gtmux-core is the single data source):

- **CLI** — `cmd/gtmux` (Go, **must stay cgo-free**). Commands: `agents`, `panes`,
  `digest`, `hq`, `quiet`, `capture`, `knowledge`, `usage`, `limits`, `events`, `resource`, `awake`, `overview`, `restore`, `focus`, `new`, `adopt`, `spawn`, `tasks`, `reap`, `send`, `share`, `pair`, `attach`, `status`, `config`, `hook`,
  `serve`, `tunnel`, `devices`, `doctor`, `update`, `whatsnew`, `install`, `uninstall`. `attach` = the remote terminal client: `gtmux attach <host|pair-link|share-link>
  [%pane]` bridges a remote tmux pane's PTY to your local terminal over a WebSocket
  (`GET /api/attach`, scope-gated), raw passthrough; owner or guest. See
  `openspec/changes/remote-terminal-client` + `docs/design/remote-attach-research.md`. Logic lives in `internal/`: the command layer is `internal/app` (CLI dispatch + thin command shims + spawn/send/serve/tunnel), over the extracted, compiler-enforced clusters — `internal/radar` (the pane-data KERNEL: the `agents`/`digest`/`usage` producers + their JSON shapes + `CurrentResource`/`PreflightResource`), `internal/hq` (the supervisor subsystem — the `hq`/`slowtick`/`selfcheck`/`distill`/`diskhygiene`/`tiergate`/`watchdog`/`tasks`/`events` files), `internal/dispatchbridge` (the tmux/events dispatch adapter), and the `internal/panefocus` pane-jump leaf. Import rule is strictly acyclic — `app → hq → {radar, dispatchbridge} → leaves`; **`hq` NEVER imports `app`**, nothing below `app` imports it (see openspec change `decompose-app-package`). `digest`+`hq` = the supervisor
  (中控) MVP: a deterministic per-agent digest (goal/last/ask, zero LLM tokens;
  also `GET /api/digest`) + a supervisor agent session at `~/.config/gtmux/hq/`
  (radar rows carry `role:"supervisor"`; the hook nudges it on waiting events —
  `hqNudge:false` disables). See `openspec/changes/supervisor-mvp`. The HQ is a
  **chief-of-staff** (参谋长), not just an event forwarder: its seed teaches a persistent
  situation board (`notes/board.md`, survives context resets), a severity-tagged event
  ledger (`events.jsonl` records carry `routine|notable|important`; THREE reads, not one
  — unfiltered `--since-seq` = the reconcile delta · `--severity notable` = fleet changes
  (incl. a user instruction reaching a session, `origin:"instruction"`) · `--severity
  important` = the escalation SUBSET. A filter is a triage shortcut, never HQ's model of
  the world — change `hq-attention-stream`), decision-authority tiers (reversible∧
  low-risk∧in-discussed-scope → HQ decides+dispatches; else escalate), graded escalation
  + reconcile-before-relay (kills stale needs-you), and a correction→charter learning
  loop (`corrections` topic; a CHARTER-LEVEL lesson exits mechanically — change
  `hq-promotion-exit`: `gtmux knowledge promote` → a brief under `knowledge/promotions/`
  → `land --ref` closes it; doctor flags a brief pending past ~2 weeks). See
  `openspec/changes/hq-chief-of-staff`.
  **HQ perception = the wake protocol** (`internal/hqwake`, spec `hq-wake-protocol`,
  change hq-perception-v2): decision-dense events — `waiting·kind / resolved / asks /
  done(unattended) / crash(StopFailure) / goal-changed / new-session / reap-suggest /
  wake-degraded / tick` — type ONE `» gtmux·<class> │ …` signal line into the HQ pane
  (draft-guarded, coalesced; done is rate-merged per pane, and a completion in the
  FOCUSED pane of an attached client defers to the tick instead — `hqWake.done`
  config). Everything else is pull-side: HQ wakes → `gtmux events --since-seq N` /
  `digest` → judges in a short turn, replying in the `⟣` signal register. A summary
  tick (10 min / burst 5, zero-change gate = zero cost) delivers the periodic brief;
  playbook v2 teaches enrollment (建联, goal-aware dossiers) and works on any agent
  (no background tail). `gtmux hq` also MIGRATES legacy CLAUDE.md-only homes now.
  **Those classes are PRIORITY, not coverage** (change `hq-watermark-wakes`,
  `internal/hqwake/watermark.go` + `internal/hq/unread.go`): deciding *which* events
  deserve a knock needs context only HQ has, so gtmux stopped deciding. It tracks HQ's
  CONSUMPTION WATERMARK and knocks `unread` (count + pull cursor, NO severity claim) for
  anything past it — 120s debounce, 300s repeat, PriorityStanding — so an event no class
  claims can no longer vanish (a `gtmux send`-driven session's turn-end did exactly that
  on 2026-08-01). **Only HQ consuming advances the watermark**: an UNFILTERED
  `events --since-seq` from the HQ home, or `gtmux events --ack <seq>`; a filtered or
  skip-ahead read does not — and a read from a SUBDIRECTORY of the home (the measured
  `cd`-drift after writing `notes/`) now WARNS on stderr instead of silently not counting
  (change `hq-unread-noise`). Excluded from the count (never from the stream): HQ's own
  pane records (else the knock feeds itself), a pane-less lifecycle BLINK — a
  `SessionStart` whose `SessionEnd` pairs within 10s — and gtmux's own `gtmux:audit:*`
  trail (change hq-action-journal: wake delivered/dropped, send, reap, rotate,
  hq-session records — acts the supervision performed, journaled for audit, never debt;
  non-audit `gtmux:*` triggers still count). **The blink rule keys on that
  PAIRING, never on the empty pane alone**: native (non-tmux) agents' turns and gtmux's own
  `gtmux:*` triggers are pane-less too, and since every CLASS wake is gated on `pane != ""`,
  `unread` is their only channel. The knock line names its composition
  (`7 unconsumed (%21 ×4 · control)`). **HQ's PULL shows that same set** — the supervisor's
  unfiltered `--since-seq` omits what the count omits (measured: 68.7% of a knock's pull was
  HQ's own echo), reports the withheld count on stderr, and `--all` restores the raw view;
  both still consume (neither shows LESS than the debt — that is what disqualifies a
  `--severity` read). Nothing leaves the log; a non-supervisor read is untouched.
  `gtmux doctor`'s `event consumption` row flags a lagging HQ.
  **HQ's OWN SESSION is watched too** (change `hq-self-rotate`,
  `internal/hq/selfrotate.go`): a long, near-full session degrades the boundary between what
  HQ produced and what reached it — on 2026-08-03 one read its own `Stop` output as the
  commander's `UserPromptSubmit` and withdrew a correct suspicion. HQ can neither self-detect
  it (the failing faculty is the one that would notice) nor self-schedule the check (no timer
  between wakes), so the serve slow tick senses **ctx / age (from the transcript's FIRST
  message, so a serve restart can't reset it) / turn count** and knocks `self-rotate` at
  standing priority when any crosses its line (`hqWake.selfRotate{Ctx 0.75, Hours 12, Turns
  300, RepeatSec 1800, CheckSec 300}`; a 0 THRESHOLD disables that criterion alone). Same
  shape as the watermark — **only the act clears the debt**: a NEW agent session id, never
  the knock. Playbook v16 makes it unattended (board+KB current → hand off → `gtmux hq
  --rotate`, which types the agent's own `/clear`|`/new` so HQ never touches tmux and the
  role whitelist stands). Deliberately NOT folded into `self-check`, which audits HQ's
  PRODUCTS silently at ≤1/h; this audits the JUDGE and must be heard. `gtmux doctor`'s
  `HQ session health` row shows the figures.
  **Wake DELIVERY is acked** (change `hq-wake-reliability`, `internal/hqnudge`): paste
  + Enter as separate steps, and a claim (`.txt` → `.sending` rename) is deleted ONLY
  on confirmation. Every terminal outcome is journaled (hq-action-journal): a confirmed
  batch as `gtmux:audit:wake-delivered` (id + full payload, once per batch), every drop
  as `gtmux:audit:wake-dropped` with its reason (evicted / unconfirmed / superseded);
  `gtmux send`/`reap`/`--rotate` and the sensor-observed HQ session chain journal
  likewise, and degradation emitters append to the JOURNAL — #647's fix, finished
  (retire-perception-spool then removed the spool copy layer entirely: the journal is
  the single stream, and `gtmux events --since-seq` warns about a sequence gap at read
  time). The ack is three layers (agent-drivers P2): the DRIVER RECEIPT
  first — the HQ session's own `UserPromptSubmit` carrying the batch `#id` (the hook
  records a wake batch's id as its event Summary; `hqwake.BatchID`) — then the screen
  read (id in history, not draft); an id still in the DRAFT is the precise
  swallowed-Enter verdict: the claim parks as `.stuck` and the next drain re-sends
  ONLY Enter (draft must still hold the batch intact; bounded, never a re-paste, same
  id). Any error or missed ack requeues, a claim stranded >60s by a dead drainer is
  reclaimed, 3 unconfirmed deliveries raise a CRITICAL `wake-degraded` (control
  record + desktop notification — the alarm can't ride the broken channel). Delivery is therefore at-least-once: each
  batch ends with `#<id>`, stable across a re-send, and playbook v3 tells HQ to ignore
  a repeated id. The queue drains by class priority (decision > outcome > standing),
  caps at 8 lines per batch + 200 entries, and is flushed by a 3s serve fast tick
  (`OnFastTick`), not the 20s sampler. The HQ pane itself resolves via
  `internal/hqpane` — the `@gtmux_hq_home` pane option, then symlink-NORMALIZED
  cwd/start-path (a symlinked `~/.config` silently ate every wake before this); an
  unresolvable HQ seen within 2h HOLDS the wake instead of dropping it.
  A self-check sensor raises a `self-check` trigger (idle/threshold/daily, ≤1/h)
  HQ acts on (`internal/hqfeed` keeps only the surfacing tiers + control-record
  names). `gtmux tasks` doubles as the **attention
  ledger** (free-text disposition + the `--pending` plate view; `--verbose` adds the
  disposition detail — the unwired tier/priority/surfaced/archive verbs were removed
  by slim-attention-ledger); `gtmux quiet
  [on|off|status]` tunes the surfacing threshold (a read-time gap CRITICAL is never
  quieted). HQ gates its OWN prints by the tier — CRITICAL/NORMAL print, QUIET is
  ledger-only.
  **HQ playbook is VERSION-TRACKED** (`openspec/changes/versioned-hq-playbook`): the seeded
  `AGENTS.md` is gtmux-OWNED, carries a `<!-- gtmux-hq-playbook vN -->` marker, and `gtmux
  hq` REGENERATES it (backing up the prior to `AGENTS.md.bak-v<old>`) when the shipped
  `hqPlaybookVersion` (in `hq.go`) is newer — so `gtmux update` + next `gtmux hq` upgrades
  the charter, no manual re-seed. **Any change to `hqInstructions` MUST bump
  `hqPlaybookVersion`** or it won't reach existing homes. User personalization lives in a
  seed-once, never-overwritten `LOCAL.md` (imported by AGENTS.md); the situation board +
  knowledge base stay untouched by upgrades.
  `spawn`+`tasks`+`reap` = **verified dispatch** (`internal/dispatch`): `spawn`
  launches an agent (new session / `--pane` / `--worktree`), proxied by construction,
  and delivers a task with LAND-VERIFICATION (hook-event first via the #388 stream,
  hardened two-frame screen-read as fallback; a re-send interlock refuses a duplicate
  payload, `--force` overrides — but a delivery that ends `failed` DROPS its interlock
  record, so a send that never landed can be retried without `--force`). **No path ever
  writes into an unsubmitted draft**: every delivery reads the box first and refuses
  (`state:"refused-draft"`) when it holds someone else's text — a paste APPENDS, so
  delivering would submit their half-written line with your payload. The override is
  `Opts.ClobberDraft`, deliberately SEPARATE from the interlock's `Force`: serve sets Force
  on every `/api/send` (it has its own sendID idempotency), and merging them would leave the
  phone — a send from another device into a pane whose owner may be typing — unprotected.
  A fragment verdict on a hook-equipped pane where the paste DID land no longer clears the
  draft (the screen can't tell a short render from a short paste; the receipt judges).
  **The guard is scoped and fails OPEN** (its job is to protect a send, never block one):
  it runs ONLY when `Opts.HasComposer` — a KNOWN agent drives the pane (a vim/ssh/TUI pane
  has no composer, and reading one there returns the transcript as a "draft"); it reads the
  COLOR capture via `DraftOfColored` and needs TWO agreeing frames (a plain capture strips
  the SGR-faint markers, so Claude's ghost suggestion reads as a typed draft — shipped in
  v0.46.2, fixed v0.46.3, see TROUBLESHOOTING); and it PROCEEDS on everything it cannot
  judge — unreadable capture, copy-mode, no input region, our own text. Bounded at 2 reads
  + 1 poll interval, never loops. Full decision table in the `agent-dispatch` spec.
  `gtmux send` verifies by default (`--no-verify` opts out — that skips CONFIRMATION, never
  the draft guard); `POST /api/send` stays fast/unverified. `tasks` is the dispatch/needs-you
  ledger; `reap` safely reclaims a finished dispatch (worktree-clean + branch-merged
  gate, else report-only; `--snooze` to keep). See `openspec/changes/hq-dispatch`.
- **Native menu-bar app** — `macapp/` (Swift / AppKit + `NSStatusItem` +
  `NSPopover` + SwiftUI). A pure **consumer** of the CLI: polls
  `gtmux agents --json` and shells out to `gtmux focus`. It's also the
  notification click target (`com.gtmux.menubar`; reopen → `gtmux focus --last`).

## Build & verify

- CLI: `make build`. App: `make app` (Swift app + bundled CLI → `Gtmux.app`).
- **Run the gate before every commit** (same as CI): `make check`
  (= gofmt + `go vet` + staticcheck + `go test -race`). For the app:
  `cd macapp && swift build -c release`. For the **mobile app**, the equivalent
  one-command gate is `cd mobileapp && npm run check` (= `tsc --noEmit` + `eslint .`
  + `jest --ci`; 0 errors required, eslint warnings tolerated) — same three checks
  CI's `mobile` job runs. The release tag gate also runs `make check` (not a weaker
  `go test`), so a tag can't ship a regression a PR would have caught.
- The CLI MUST stay cgo-free — `CGO_ENABLED=0 go build ./cmd/gtmux` must pass.
  Only the Swift app is native; nothing in `internal/` may pull in cgo.
- Release: push a tag `vX.Y.Z` → goreleaser ships the CLI tarballs and a macOS
  job runs `macapp/build.sh` to ship `Gtmux-<v>-macos.zip`. CI builds the app
  but **can't see the menu bar** — smoke-test on real macOS before trusting a tag.
- **Signing & notarization:** `build.sh` signs ad-hoc by default (Gatekeeper-blocked
  on other Macs → needs `xattr -dr com.apple.quarantine`). Set `GTMUX_SIGN_ID=
  "Developer ID Application: …"` for a STABLE Developer ID signature (hardened
  runtime; TCC grants then persist across updates), and `GTMUX_NOTARY_KEY`/`_ID`/
  `_ISSUER` (App Store Connect API key) OR `GTMUX_NOTARY_PROFILE` (keychain) to
  **notarize + staple** — then it opens on any Mac with no quarantine dance. **CI
  does this automatically when the release secrets are set** (`release.yml` app job
  imports the cert into a throwaway keychain): `MACOS_CERT_P12` (base64 .p12),
  `MACOS_CERT_PASSWORD`, `MACOS_NOTARY_KEY_P8` (base64 .p8), `MACOS_NOTARY_KEY_ID`,
  `MACOS_NOTARY_ISSUER` (the signing identity is auto-derived from the cert). **This
  is the primary path** — a **tag push with the secrets missing FAILS the app job**
  (it won't silently ship ad-hoc or drop the app). `make app-release` from a Mac is a
  manual fallback only. One-time setup: `docs/release-signing.md`.
- **Mac App Store is NOT a viable target as built:** the app shells out to
  `gtmux`/`tmux`/`osascript` and reads `~/.local/share/gtmux`, `~/.tmux/…` etc.,
  none of which survive the App Sandbox MAS mandates (and there's no entitlement
  for "drive tmux / control arbitrary terminals"). Ship **Developer ID + notarized
  direct distribution**; MAS would require a sandbox-compatible rearchitect.
- Workflow: branch → PR → CI green → squash-merge → tag. Don't commit to `main`.
- **Every release tag MUST carry a `user:` block in its tag message** — the lines
  `gtmux update` prints after installing and `gtmux whatsnew` lists. Write them for the
  USER (what moved that they'd notice), not as commit subjects; goreleaser copies the tag
  body into the release, and the CLI reads it back **from the GitHub RELEASE body, not the
  local tag** — so a botched line can be repaired with `gh release edit <tag> --notes-file
  <path>` (run it INSIDE the repo; from anywhere else it fails with "not a git repository")
  without rewriting the tag. **Keep every bullet on ONE line**: the reader renders the block
  line-by-line, so a wrapped bullet ships as two bullets, the second one starting mid-sentence
  (v0.53.0 did exactly that). A release with no `user:` block shows
  nothing, which is correct only when nothing changed for users. A `user-zh:` twin block
  is **optional but encouraged** (tags used to flip language between releases): the CLI
  serves the block matching the reader's language (`GTMUX_LANG`) and falls back to the
  other when one is absent, so old single-block tags keep working. Blocks may appear in
  either order; each ends at a blank line, a `#` heading, or the other block's marker.

  ```
  git tag -a v0.40.0 -m "v0.40.0 — <one-line dev summary>

  user:
  - spawn --title now names the session
  - restore returns you to the window you were on

  user-zh:
  - spawn --title 现在会命名会话
  - restore 会把你带回原来的窗口
  "
  ```
- **git-ops footgun:** never build a `gh pr create` / `git commit` body via
  `--body "$(cat <<'EOF' … EOF)"` or `-m "$(…)"` when the text contains backticks —
  the `"$(…)"` re-enables command substitution, so `` `gtmux serve` `` in prose gets
  **executed** (this once spawned a rogue serve that squatted :8765). Use
  `--body-file <path>` / `git commit -F <path>` instead. After a PR-create that
  warned/errored, `ps aux | grep 'gtmux serve'` and kill strays.
- **Debug/release pitfalls live in `docs/TROUBLESHOOTING.md`** — a living checklist
  (duplicate-serve pairing bug, QUIC-blocked tunnel, relay-redeploy, etc.). Consult
  it when pairing/push/release misbehaves, and **append a new entry whenever a
  footgun costs real time** (symptom → root cause → must-check).

## Deploy — where each artifact ships (DON'T FORGET)

Four artifacts, **three different delivery paths**. A code change isn't "shipped"
until the right one runs:

| Artifact | Ships via | Command / notes |
|---|---|---|
| **CLI** (`gtmux`) | git tag `vX.Y.Z` → `release.yml` (goreleaser) | tarballs + Homebrew. Users get it with `gtmux update` / curl `install.sh`. |
| **macOS app** (`Gtmux.app`) | git tag → `release.yml`'s **app job** signs+notarizes+uploads+casks it in CI (the primary path, once the 5 signing secrets are set — see Signing below; a tag with the secrets missing now **fails the job** rather than shipping ad-hoc). `make app-release` from a Mac is a **manual fallback** (CI outage/hotfix). | `Gtmux-<v>-macos.zip` + Homebrew cask → `~/Applications`. |
| **Mobile app** (`com.gtmux.app`) | **NOT a git tag** — manual device build. `set-version.sh` ARCHIVES the store notes under the stamped version in `mobileapp/release-notes/` (only when they changed — most stamps cross a CLI-only version) and regenerates `src/releaseNotes.ts` from that archive, which is what the in-app What's New popup renders (it must show EVERY version a user skipped, so the per-release-overwritten store metadata cannot be the source). Edit the store notes, then re-run it; `check-design.sh` regenerates and diffs, so drift is a red build. | `cd mobileapp && bash scripts/set-version.sh` then `xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile -configuration Release -destination 'id=<device-udid>' -derivedDataPath ios/build/dd DEVELOPMENT_TEAM=2337SY8FRT CODE_SIGN_STYLE=Automatic MARKETING_VERSION=<v> APS_ENVIRONMENT=development -allowProvisioningUpdates build` → `xcrun devicectl device install app --device <devicectl-uuid> <app>`. Embeds the `GtmuxWidget` + `GtmuxNotificationService` app-extension targets (wired by the `ios/add_*.rb` xcodeproj scripts). **`APS_ENVIRONMENT` controls the aps-environment entitlement AND the reported push env: a dev device build MUST pass `=development` (→ sandbox APNs, matching the dev signing); an App Store/TestFlight ARCHIVE leaves it at the Release default `production`. The app reads it back (LiveActivityModule `apnsEnv` constant from `Info.plist APNS_ENV`) and reports it at push-register so ONE relay routes per token.** |
| **Push relay** (`gtmux-relay.ccy.dev`) | **the LIVE relay is the Cloudflare Worker `relay-worker/` (TS)** — NOT the Go `relay/` (that's the self-host reference impl) | `cd relay-worker && npx wrangler deploy` (wrangler is OAuth-logged-in). **Per-token APNs env:** each push intent carries `env` (`sandbox`/`production`) that the device reported at register (via serve's `DeviceToken.Env`/`PushIntent.Env`), so ONE relay serves dev + App Store; `APNS_ENV` is only the fallback for env-less (old) tokens. (The Go `relay/` reference is single-env — a self-host simplification.) **Keep `relay-worker/src/index.ts` and `relay/apns.go` in sync on payload SHAPE, and REDEPLOY the Worker** — editing only the Go one changes nothing live. |
| **Tunnel provisioner** (`api.gtmux.ccy.dev`) | Cloudflare Worker `tunnel-worker/` | `cd tunnel-worker && npx wrangler deploy`. See [[hosted-tunnel-a1]] / `docs/design/remote-access-tunnel.md`. |

Corp-network caveat: `api.cloudflare.com` is intermittently TLS-reset from the
office — wrangler calls may need a retry (see `docs/design/remote-access-tunnel.md`).

## Spec-driven development (OpenSpec)

This repo uses **OpenSpec** for spec-driven development. `openspec/specs/`
holds the current capability specs (the source of truth for what IS built);
`openspec/changes/` holds in-flight change proposals. For any non-trivial
feature, **propose a change first**, implement against it, then sync/archive:

- `/opsx:propose "<idea>"` — draft a change (proposal + tasks + spec deltas).
- `/opsx:apply-change` — implement an approved change's tasks.
- `/opsx:sync-specs` / `/opsx:archive-change` — fold the deltas into the main
  specs when done. `npx @fission-ai/openspec validate --specs --strict` must pass.

Keep specs aligned with the code: when behavior changes, update the relevant
`openspec/specs/<capability>/spec.md`. Capabilities are backfilled for the major
existing features (agent-radar, terminal-jump, notifications, menu-bar-app,
env-doctor, session-restore, remote-access, push-notifications, mobile-app).

### RULE — spec ⇄ code ⇄ test consistency (REQUIRED; part of "done")

A 2026-07-12 audit found docs/specs/tests drifting from the code (e.g. the
`self-hosted-tunnel` change was fully implemented — `internal/app/tunnelself.go` —
yet its `tasks.md` sat at 0/14 and it was never archived). To stop that drift, a PR
that changes the **observable behavior of a spec'd capability is NOT done** until,
in the **same PR**:

1. **Spec updated** — edit the relevant `openspec/specs/<capability>/spec.md` (small
   change) or land it via an `openspec/changes/<id>` proposal (non-trivial). A
   behavior change with no spec delta is incomplete.
2. **Tests updated** — add/adjust the test(s) that pin the new behavior (Go
   `*_test.go` / mobile jest / e2e). "It builds" is not coverage.
3. **Docs/memory corrected** — any doc, memory, or `CLAUDE.md` line that cites the
   changed file / flag / behavior is fixed in the same PR. No stale references.
4. **CLI surface documented (usage/docs drift)** — a NEW or RENAMED command must be
   reflected everywhere a user or reader looks, in the same PR: the CLAUDE.md command
   list (**enforced** — `check-design.sh` fails a dispatched command that isn't listed;
   add genuinely-internal ones to its `HIDDEN` allowlist), the top-level `gtmux --help`
   usage (`internal/app/help.go`, en+zh) when it's a user-facing command, a
   `## gtmux <cmd>` section in `docs/cli.md`, and — if it adds/changes an HTTP surface —
   `api/contract.md`. (This rule exists because `attach` once shipped absent from the
   usage + `docs/cli.md`.)
5. **Docs conformance (enforced, with a stated boundary)** — `check-design.sh` now also
   checks the docs claims that HAVE a machine-readable source: every wake class must be
   taught in BOTH `docs/cli.md`'s class table and the seeded playbook (a class taught in
   neither is a knock HQ's charter never mentions — `usage·warn` and `stuck·waiting` did
   exactly that for months); retired tokens must not return (each denylist entry names the
   change that retired it; `openspec/changes/**` is excluded so a proposal can quote what
   it retires). Doc examples of code-generated formats are marked
   `<!-- gtmux:rendered <id> -->` and compared against the real builder by
   `internal/docs` — `make docs-fix` rewrites them; never hand-copy a rendered line.
   **The boundary is part of the rule:** none of this checks whether prose is TRUE. "The
   `done` wake fires for any session" is a sentence no grep can falsify, and a green gate
   is NOT a reviewed doc — that judgment stays a reviewer's, exactly as before.

**Historical consistency (the spec lifecycle is not optional).** propose →
implement → **sync-specs + archive-change**. The moment a change in
`openspec/changes/` is implemented + merged, ARCHIVE it (same PR, or the very next),
and keep its `tasks.md` checkboxes truthful as you go. Invariant: `specs/` = what IS
built · `changes/` = ONLY truly in-flight work · `changes/archive/` = the audit
trail. An implemented change left in `changes/` (or unchecked tasks over shipped
code) is exactly the drift we are eliminating.

**Enforced:** `scripts/check-design.sh` (CI's "design + architecture conformance"
step) runs `openspec validate --specs --strict` (a malformed/broken spec fails the
build like a red test) AND the **command-docs drift check** — every command dispatched
in `internal/app/app.go` must appear in the CLAUDE.md command list (minus the `HIDDEN`
allowlist), and `docs/cli.md` must not document a command that no longer exists.
Validation only proves the spec is well-formed and the command registry is complete;
the spec-matches-behavior, "worth a curated usage/cli.md entry", and archive-hygiene
judgments above stay a **review-gate checklist** (a reviewer confirms them before
squash-merge — they can't be fully automated).

## Conventions / invariants

- **Contracts (don't break):** the `gtmux agents --json` schema (incl. the
  additive optional `role` field) + the `gtmux digest --json`/`GET /api/digest`
  shape (incl. the additive `sense: driver|partial|screen` perception tier,
  agent-drivers) + `gtmux usage --json`/`GET /api/usage` (usage-watch); state paths
  `~/.local/share/gtmux/{active/<pane>, waiting/<pane>, last-finished,
  notify-icon.png, notify/<id>.json}`; hook events
  `Stop`/`Notification`/`UserPromptSubmit`; bundle id `com.gtmux.menubar`. The
  hook→app **notify queue** (`internal/notify` writes JSON; macapp
  `NotificationManager` drains & posts) is the notification channel — there is no
  terminal-notifier/osascript fallback (notifications need the app running).
- **i18n:** every user-facing string is en+zh via `internal/i18n` and `GTMUX_LANG`.
- **Scope (decided):** gtmux focuses on the **tmux + agent** workflow. Its rich
  view/control surface is **tmux-only** (`agents` scans `tmux list-panes`; focus &
  send need a pane). **Non-tmux ("native") agent sessions are now SENSED**
  (read-only): a hook that fires with no `$TMUX_PANE` records the session by id in
  `internal/native`, and the radar shows it as a `source:"native"` row — sense-only
  (no view/jump/send), in the menu-bar's "Elsewhere / 不在 tmux" category. Resumable
  ones can be **adopted into tmux** (`gtmux adopt <session_id>` resumes the
  conversation in a fresh tmux session). See `openspec/changes/native-agent-sessions`
  + memory `native-agent-sessions`. Still OUT of scope: a live screen/preview or
  in-place input for native sessions, and detecting agents that install **no** gtmux
  hook (no hook = no signal). The per-terminal tab-title scanner (needed for native
  *jump*) remains deferred — `source/terminal/tab` + `focus --terminal/--tab` are
  latent groundwork for it.
- **Terminal coupling** goes through the `internal/terminal.Terminal` interface
  (`FocusTab`/`IsViewing`/`OpenWindow`/`SpawnTabs`); `internal/ghostty.Driver`,
  `terminal.iterm2`, and `terminal.warp` are the impls (Warp is BEST-EFFORT: no
  AppleScript dictionary — focus = `warp://session/<uuid>` when gtmux's own
  attach recorded the uuid into the tmux session env, else app-activate;
  restore/new via launch configs; its macOS process name is `stable`). `terminal.Active()` resolves the host
  driver via `detect.go` (`GTMUX_TERMINAL` override → `$TERM_PROGRAM` → tmux
  client process ancestry → Ghostty fallback). Callers
  (`focus`/`restore`/`new`/`hook`) use `terminal.Active()`, never a terminal
  package directly (except the still-deferred native `ghostty.FocusTerminalTab`).
  The radar side (`agents`/`overview`/notify) is tmux-only and terminal-agnostic.
  **iTerm2 gotchas (verified on real iTerm2):** the AppleScript target is
  `"iTerm"` (NOT `"iTerm2"` — that loads no scripting dictionary), the macOS
  *process* name is `"iTerm2"`, and the iTerm session `name` carries the tmux
  title (often suffixed `" (tmux)"` → drivers prefix-match). iTerm2's AX window
  title is empty, so `IsViewing` asks iTerm directly (`frontmost` + current
  session `name`) instead of System Events. Add new terminals as drivers
  (kitty/WezTerm/Apple Terminal feasible; Alacritty not — no tabs/scripting) — see
  `docs/design/multi-agent-multi-terminal.md`.
- **Verifying the status item / popover on macOS** (screen capture is
  permission-blocked): query the accessibility tree, e.g. `osascript -e 'tell
  application "System Events" to get count of menu bar items of menu bar 1 of
  (first process whose name is "GtmuxBar")'`; or run the binary directly with
  `GTMUXBAR_DEBUG=1` (logs to stderr).

---

## gtmux 设计规范（必读 —— 后续每次 UI 迭代都必须遵循）

gtmux 是一个产品、三块屏（CLI · 菜单栏 · 手机），共用同一套状态语言。**改动任何 UI 前，先读对应
权威设计规范，并严格遵循；不得擅自偏离。**

- 改 **菜单栏 app**（`NSStatusItem + NSPopover + SwiftUI`）→ 先读 `docs/design/DESIGN.md`。
- 改 **移动端 app**（bare React Native，`mobileapp/`）→ 先读 `docs/design/MOBILE.md`。
- **接入一个新 coding agent**（或迭代已有的）→ 先读 `docs/design/agent-onboarding.md`（支持分层、身份唯一来源 `internal/agents` 注册表、逐步流程 + 踩坑清单）。
- 落地总入口 / 顺序 / 验收 → `docs/design/HANDOFF.md`；可视参照 `docs/design/mockup/`。

要点（两块屏统一）：

- **「等你输入」= 仅 `waiting`（红）；`working`（蓝）永不等输入。** 结构化 `1/2/3` 回应只挂 waiting。
- **状态语言三重编码**（色+形+字形），全表面统一：waiting=红方块·双竖线 / working=青圆·静态加载环 /
  idle=绿圆·✓ / running=灰圆·点。颜色**只**表达状态，状态色用权威值（见下方「代码位置对照」/
  `mobileapp/src/ui/theme.ts`）。
- **层级**：waiting 响、idle 静。分区顺序 needs-you→working→idle→running。
- **agent 身份**图标:官方图标**内置在 `assets/agent-icons/<key>.png`**(嵌入二进制,`gtmux update` 随发,三端统一从这里取,开箱即显示),仅用于**标识对应 agent**(nominative use),来源与授权逐条记在 `assets/agent-icons/SOURCES.md`;已装官方桌面 app 时仍可用系统真图标,都取不到才回退中性单字标。**gtmux 自身不绘制任何第三方商标**;这是「仓库不放第三方商标」的**明确例外**(见下方 §6 细则)。
- **双语** en/zh（跟随 `GTMUX_LANG` / 设备语言），CJK 不换行用省略号；语言三态（跟随系统/EN/中文）即时生效。
- **动效最小**：只允许 idle→waiting 一次脉冲；加载环不旋转；空闲零动画。
- **视觉克制**：无彩虹渐变、无彩色发光阴影；文案平实、禁止营销腔（尤其首次运行/权限卡）。
- **支持原生终端**（无 tmux）：行与跳转按 `source: tmux|native` 泛化（DESIGN §7 / MOBILE §2）。
- **连接指示**用 server 名 + 状态点（已连接绿 / 重连琥珀 / 离线红），不用 "live" 字样；离线不清屏、留缓存置灰。
- **移动端**：监控 + focus + 推送 + **终端输入**（`POST /api/send` → `tmux send-keys`，仅
  bearer token 把关，默认开启 —— token 泄露即可在 Mac 上执行命令,务必当密码看待）。语音输入仍属 P3。
- **命名**统一小写 `gtmux`。
- **与 `DESIGN.md` / `MOBILE.md` 冲突的改动，先提出、不擅自偏离；改了设计就同步更新这两份规范。**

### 代码位置对照（重要：DESIGN.md 写于原生化迁移之前）

`docs/design/DESIGN.md` / `HANDOFF.md` 里引用的 Go 文件在迁移到原生 Swift app（v0.0.11）后
已删除。按下表换算到现状，**实现以现状为准、设计意图以 DESIGN.md 为准**：

| DESIGN.md 引用 | 现状位置 |
|---|---|
| `internal/menubar/icon.go`（状态色权威值 `#EF4444/#06B6D4/#22C55E/#8E8E93`） | `macapp/Sources/GtmuxBar/Theme.swift` 的 `Theme.Status.waiting/working/idle/none`——**已用 DESIGN 的精确 hex，且有一致性测试断言其匹配**；`AgentStore.swift` 的 `Status.color`/`nsColor`（枚举名为 `Status`，非 `AgentState`）只是委托给 `Theme.Status.*`。 |
| `internal/menubar/model.go`（`agents --json` 契约 / Agent shape） | 产出端 `internal/radar/agents.go`（`agentJSON`）；消费端 `macapp/Sources/GtmuxBar/AgentStore.swift`（`Agent`）。 |
| `cmd/gtmux-menubar/`（cgo systray 入口） | 已废弃；菜单栏 app 现为 `macapp/`（Swift）。systray 不再使用。 |

agent 官方图标 —— **主来源现为仓库内置**（`assets/agent-icons/<key>.png`,经
`assets` 包 `//go:embed` 嵌入 CLI）：`GET /api/icon`(serve)优先返回内置图标,
手机/浏览器**开箱即显示**,无需装桌面 app、无需运行时联网。菜单栏 app `AgentIcons`
仍解析 `agents --json` 的 `icon`:`.app` 路径走 `NSWorkspace` 取**已装应用真图标**、
图片路径直接加载、`~/.config/gtmux/icons/<agent-key>.png` 免配置投放,都取不到才回退
中性单字标。内置图标**仅用于标识对应 agent**、来源记于 `assets/agent-icons/SOURCES.md`
(见 §6);gtmux 自身不绘制第三方商标。内置默认:Claude/Cursor 指向各自
`/Applications/*.app`,**Codex 用内置 `assets/agent-icons/codex.png`**(Codex 的官方
mark —— 不是 ChatGPT 的 logo;codex CLI 无独立 .app,曾错抽 ChatGPT.app 的图标)。
内置图标现在**三端都能取到**:`radar.IconFor` 会把内置 PNG 落盘到
`~/.local/share/gtmux/agent-icons/<key>.png` 并把**该路径**作为 `icon` 提示返回(内容变了才
重写,写入走 rename)。所以菜单栏 app 无需改 Swift —— 它本来就是「把 hint 当文件打开」;
以前返回的是 `builtin:<key>` 这种不透明 token,手机/浏览器只看非空所以正常,菜单栏打不开就
回退单字标(Codex 的非 tmux 行显示 "Cx" 即此)。`~/.config/gtmux/icons/<slug>.png` 仍是手动覆盖。
