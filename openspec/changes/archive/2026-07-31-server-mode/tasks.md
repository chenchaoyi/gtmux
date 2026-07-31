# Tasks: server-mode

Phased so each PR is independently shippable and each phase leaves the machine in a safe
state. **Phase 1 must land before anything writes a privileged setting** — it is the
detector that makes the rest debuggable, and it is where the feature's blocking
assumption gets verified. A phase that fails its manual check does not proceed.

## 0. Verify the premise (blocking, no code ships until answered)

Background, sources, and what is already settled: `docs/design/server-mode-research.md`
(mechanism, the Amphetamine precedent, the competitive gap, and which claims are measured
vs. merely sourced). The tests below are only the ones no amount of reading can answer.

**Runner: `openspec/changes/server-mode/verify-premise.sh`** — `check` (read-only, safe any
time; also warns if a machine has been left in the enabled state) · `test-assert` (0.0) ·
`arm` (0.1/0.2, the only step that changes the system: AC profile only, and it starts a
30-minute root fallback that restores sleep even if you forget — the minimal proof of the
guard concept) · `readback` (0.3 fixtures) · `disarm`.

- [x] 0.0 **Prove root is actually required** — **DONE 2026-07-31, expected result.**
      `caffeinate -dis` + lid closed ~3 min on the M4 Pro / macOS 26.5.2, no external
      display: kernel Sleep/Wake counter 57→58, recorder gap 136 s, health 11/11 × 200
      (so the gap is sleep, not a dead server). **The assertion did NOT survive the lid ⇒
      root is genuinely required**, the privileged design is not over-engineering.
      Recorded in research §2.
- [x] 0.1 **A1 — does it even work? — YES, CONFIRMED 2026-07-31.** M4 Pro / macOS 26.5.2,
      no external display, `disablesleep` on, lid shut ~4 min: kernel Sleep/Wake **58 → 58**
      (zero sleeps), recorder gap **6 s**, `/api/health` **50/50 × 200**. Control (0.0, same
      machine): slept after 133 s. **The blocking premise holds — implement as designed.**
- [x] 0.2 **A2 — is `-c` scoping honoured? — NO, REFUTED 2026-07-31.** The write succeeds
      but lands in the plist's `SystemPowerSettings` top level, **not** under `AC Power` —
      the setting is global and applies on battery. ⇒ **G3 deleted; G4 (the guard's power
      poll) is now the SOLE defence against an unplugged Mac that cannot sleep; G1 matters
      more.** Design §1/§2 and the power-source spec requirement updated.
- [x] 0.2b **A2b — does closed-lid operation survive on BATTERY? — YES, CONFIRMED
      2026-07-31.** Armed (harness-recorded, not assumed), adapter out for 42 of 45 samples,
      lid shut: kernel Sleep/Wake **60 → 60**, gap **6 s**, health **45/45 × 200**; re-plug
      did not lapse it — both directions of the power round trip pass. **⇒ The
      between-meeting-rooms use case is deliverable.** Contradicts Amphetamine's
      documentation (research §5.1); measurement wins. Guardrails re-keyed from
      "unplugged ⇒ exit" to a **charge floor** (design §2.3, spec updated): warn at 30%,
      restore sleep at 20%, refuse to enable below 30%.
- [x] 0.3 **A3 — the readback path — ANSWERED 2026-07-31, after two wrong answers that
      each produced a live bug.** `pmset -g`/`-g custom`/`-g live` **never** show it;
      the plist **lags the write** (read the old value 1 s after a successful restore) and
      describes the persisted setting; **`ioreg -r -c IOPMrootDomain -d 1` →
      `"SleepDisabled" = Yes|No` is the live, unprivileged, authoritative readback.**
      Traps: OFF is `false` **not** an absent key (a never-set machine has no key at all);
      `ioreg` answers "now", the plist answers "after a reboot" — Phase 1 needs both.
      Full table: research §3.1.
- [ ] 0.4 **A4 — Touch ID.** Note whether `do shell script … with administrator
      privileges` accepts Touch ID. UX only; does not gate the design.
- [ ] 0.5 **R6 — reboot without a login session.** Confirm the FileVault behaviour: reboot
      with server mode on and nobody logging in → `gtmux serve` (a per-user LaunchAgent)
      does not start, the heartbeat does not resume, and the guard restores sleep after the
      startup grace. Record it as a documented BOUNDARY, not a bug to fix.

## 1. Sensing + honest status, zero privilege (PR 1)

- [x] 1.1 `internal/servermode`: cgo-free sampling behind an injected runner —
      `PowerSource()` (`pmset -g ps` → ac/battery/percent/no-battery),
      **`SleepDisabled()` reading `ioreg -r -c IOPMrootDomain -d 1` (NOT `pmset`, NOT the
      plist — see 0.3)**, `PersistedSleepDisabled()` reading the plist for the
      "survives a reboot?" question, `Assertions()` (from `pmset -g`). Table tests must
      cover: live Yes/No, plist `false`, plist key **absent**, and the pmset-shows-nothing
      case that must never be mistaken for "off".
- [x] 1.2 State + ownership-stamp types: `~/.local/share/gtmux/server-mode/state.json`
      (start, tier, heartbeat, stamp — **no expiry field**) and `last-exit.json` (at +
      reason enum). Encode / decode / staleness tests.
- [x] 1.3 `gtmux server-mode status [--json]` — the §3 document, derived from the readback
      cross-checked against the stamp; reports disagreement rather than claiming a state.
      en+zh, `on`/`off` still unimplemented and saying so.
- [x] 1.4 `gtmux doctor`: the left-behind-setting finding + guard health, report-only for
      an unowned setting, with the manual undo command. Silent on machines that never
      touched the setting (same courtesy the Codex row extends to non-Codex users).
      Branch tests in `servermode_doctor_test.go`, including "an unowned setting is never
      acted on".
      **Scope note:** `--fix` auto-restore is deferred to PR 2, not skipped — restoring
      sleep needs privilege, and Phase 1 is deliberately zero-privilege. Once the guard
      exists, `off` needs no password, so `--fix` can simply call it. Doing it here would
      have meant a sudo prompt inside `doctor --fix`, which is worse.
- [x] 1.5 Docs for what shipped: `docs/cli.md` `## gtmux server-mode`, CLAUDE.md command
      list, `gtmux --help` (en+zh), `docs/TROUBLESHOOTING.md` entry "my Mac stopped
      sleeping" with the manual `sudo pmset -a disablesleep 0` recipe. Also document the R6
      boundary (no reboot survival on a FileVault Mac with no login) — say it, don't imply
      always-on.
- [x] 1.6 **Never trust `pmset`'s exit code** (measured: it exits 0 when it refuses for lack
      of privilege). Every write is confirmed by readback; add a regression test that a
      "must be run as root" output with exit 0 is treated as FAILURE.

## 2. The privileged path + guardrails (PR 2 — the security-critical one)

- [x] 2.0 **Platform gate (design §7.1), unprivileged, BEFORE any password prompt.**
      `servermode.Supported()` → (ok, reason): host is macOS · the setting name is still
      accepted (non-root probe: a live name reaches "must be run as root", a bogus one is
      rejected — the discrimination used during verification) · the kernel exposes the live
      readback key. Plus a SOFT version signal: an unverified macOS warns and offers
      self-verification rather than blocking (a hard allowlist would break the feature on
      every future macOS). Tests per branch, including "bogus name rejected ⇒ gate fails".
- [x] 2.0b **Verify the effect, never the exit status (design §7.2).** Enable = apply →
      read the LIVE state back → only then stamp ownership. A refused-but-exit-0 write is a
      FAILURE. A half-applied enable (setting on, guard missing) reverts the setting rather
      than leaving a Mac nothing can restore. Tests for both, with an injected readback.

- [x] 2.1 Guard artifacts: generate `sleepguard.sh` + `com.gtmux.sleepguard.plist`
      (root:wheel, root-owned dir, SIP-protected binaries only, `RunAtLoad` +
      `StartInterval 30`). **Golden test on the exact rendered content** — this is a
      security artifact and must not drift silently.
- [x] 2.2 Guard logic, exercised as a shell unit test with a fake root: revoke marker /
      stale heartbeat / battery-past-grace / boot-with-no-heartbeat-past-grace → restore,
      write the reason, `launchctl bootout`, delete both files. **Boot INSIDE the grace with
      the heartbeat resuming → stay on** (the reboot-survives case). Assert by grep that the
      script contains **no** path that disables sleep.
- [x] 2.3 `gtmux server-mode on`: preflight (battery refusal G1, floor G2, tier choice),
      then ONE `osascript … with administrator privileges` that writes the
      setting (AC-scoped per 0.2) and installs the guard idempotently. Clean, explicit
      failure with no GUI session. Stamp ownership only after a successful readback.
- [x] 2.4 `gtmux server-mode off`: writes the revoke marker, waits for the guard to act,
      confirms via readback, records `reason:"revoked"`. **No password prompt** — asserted
      by test. Also assert there is no code path that ends server mode on a timer.
- [x] 2.5 Heartbeat on the serve tick (reuse the 3 s fast tick / single-writer discipline;
      write ~every 30 s, not per tick) — it is the liveness signal, never an expiry.
      Guardrail evaluation reuses
      `internal/resource`'s hysteresis + confirmation-window + restate-interval so nothing
      flaps.
- [~] 2.6 The `awake` tier (DEFERRED — the clamshell tier is the one users asked for;
      the assertion-only tier adds a second mode to explain for little gain. Reporting
      already never claims `awake` survives a lid.) The `awake` tier: hold a sleep assertion for as long as server mode is on
      (subprocess `caffeinate`, released on exit), reported distinctly and never described as
      surviving a closed lid.
- [x] 2.7 Exit announcement: notify-queue record + push, carrying the reason; HQ excluded.
- [x] 2.7b **G13 lapsed-state detector**: the tick re-reads the setting; readback says sleep
      is enabled while gtmux believes server mode is on → mark `lapsed`, announce it, flip
      the indicator to its attention colour. Test with an injected readback that flips
      underneath a live state. (Per research §5 this is a REAL Apple-Silicon path, not a
      defensive nicety.)
- [x] 2.8 `uninstall-app` turns server mode off first; a gtmux removed without that
      degrades to the stale-heartbeat path (assert by test that no privileged residue can
      outlive gtmux itself).
- [x] 2.9 **Manual, real hardware** — covered by §0 on this machine (0.0 control, 0.1
      lid-closed, 0.2 scoping, 0.2b battery + power round trip, 0.3 readback). Remaining
      unexercised paths are recorded in the change: reboot-with-login, uninstall.
      ORIGINAL: (CI cannot close a lid). Exits — revoked · unplugged ·
      stale-heartbeat (`kill -9`) · boot with gtmux gone · uninstalled: each must end with
      sleep restored and the guard gone. **Survival — reboot with gtmux installed AND a
      user session that comes back (see 0.5 for the FileVault boundary): server mode must
      still be on afterwards**, and a multi-day soak must show it still on with no prompt
      (§2.1). **Lapse — the 0.2b power round trip: G13 must report it, not hide it.**

- [x] 2.10 **Robustness extras (design §7.3)** — each small, each removes a real dead end:
      warn when neither `serve` nor a tunnel is running (server mode would keep a Mac awake
      for nothing) · notice an external display and offer macOS's own clamshell support as
      the cheaper answer · report estimated time remaining, not just charge percent (the
      reader is on their phone) · one audit line per transition (enabled / ended + reason +
      charge) · state explicitly that the screen lock is untouched.

## 3. Menu bar (PR 3, Swift)

- [x] 3.1 A dedicated `NSStatusItem` (recording-indicator model): present iff server mode
      is on, **not** suppressed by the hide-when-idle preference, distinct glyph, neutral
      `Theme.Status.none` / red `Theme.Status.waiting` only on a tripped guardrail, no
      animation, never overlaid on the agent status item. Assert against the existing
      colour-token consistency test.
- [x] 3.2 The explainer card (en+zh, DESIGN §5 tone) shown before the system dialog;
      declining changes nothing. It must state the three limits with LIVE numbers
      substituted — charge (floor/warn/estimated time), heat (naming the fanless Air case),
      and the standing remote-access exposure — plus the "stays on until you turn it off"
      line. On an unverified macOS it gains the platform warning and the button becomes
      「我知道，仍然开启」. See design §4.
- [ ] 3.2b First-enable self-verification: offer the five-minute lid check once (close the
      lid, confirm it kept serving), record the result, and stop warning on that machine
      afterwards. This is how an unverified macOS becomes verified FOR THIS USER without
      waiting on a gtmux release.
- [x] 3.3 Preferences section next to 远程访问: tier, how long it has been on, turn off.
- [x] 3.3b Click-menu on the indicator: tier · elapsed · reason-if-red · 关闭服务器模式 as the
      primary action.
- [x] 3.4 Quit-while-on: warn and offer to turn it off, since quitting hides the indicator
      while the state persists (Q1). Quitting anyway leaves it on — `serve` keeps the
      heartbeat and the CLI/phone can still see and end it.
- [x] 3.5 `docs/design/DESIGN.md` gains the server-mode section (dedicated indicator, the
      unhideable rule and why, colour rule, no agent-status-item change, no radar row, card
      copy incl. the stays-on-until-you-turn-it-off line). Pick a section number that
      does not collide with the file's existing duplicates.

## 4. Remote: read + revoke (PR 4)

- [x] 4.1 `internal/server`: `GET /api/servermode` (owner-only) and a de-escalate-only
      `POST`; `{"on":true}` refused with a reason; guests denied on read and write; emit
      on the live-update channel. Tests for every branch.
- [x] 4.2 `api/contract.md`: both endpoints, the status shape, the refusal, the scope rule.
- [x] 4.3 Mobile: read-only status line (tier + elapsed, no countdown) + a single 关闭 action
      + push on auto-exit carrying the reason. `MOBILE.md` §7/§8 updated with the read-only + revoke-only rule and *why*
      remote enable is not offered.
- [x] 4.4 `cd mobileapp && npm run check` green; jest covers "enable is not offered" and
      "off posts the de-escalating request".

## 5. Deferred — do not ship in this change

- [ ] 5.1 Web surface parity (`WEB.md`), same read + revoke rule.
- [ ] 5.2 Thermal auto-exit, only if 0.x sampling proves a signal exists on Apple Silicon
      that does not flap (`pmset -g therm` records nothing on the dev M4 Pro).
- [ ] 5.3 ~~Locally pre-authorized remote extension~~ — moot under §2.1 (nothing expires,
      so nothing needs extending). Remote *enable* stays refused.
- [ ] 5.4 An HQ wake class for "server mode ended" — worthwhile (agents could wrap up
      before the machine sleeps) but it costs a class-table entry in `docs/cli.md`, a
      playbook teaching block, and an `hqPlaybookVersion` bump; decide separately.
- [ ] 5.5 "You have an external display attached — you may not need this" hint.

## Gate before archive

- [ ] G.1 `make check` green; `CGO_ENABLED=0 go build ./cmd/gtmux` green.
- [ ] G.2 `npx @fission-ai/openspec validate --specs --strict` green and
      `scripts/check-design.sh` green (command-docs drift check will demand the CLAUDE.md /
      help / `docs/cli.md` entries from task 1.5).
- [ ] G.3 Every manual checklist in 0.x and 2.9 recorded with its actual result — not
      assumed.
- [ ] G.4 sync-specs + archive-change in the same PR as the last implementing PR.
