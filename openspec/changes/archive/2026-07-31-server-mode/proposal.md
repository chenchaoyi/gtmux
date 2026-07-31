# server-mode

## Why

gtmux's whole remote story — `serve` + `tunnel` + the phone app — assumes the Mac is
awake. It isn't. Close a MacBook's lid and macOS sleeps the system; the tunnel drops,
`serve` stops answering, every agent's turn freezes mid-flight. So today "指挥合盖的
Mac" is a promise gtmux cannot keep: the user must leave the lid open (screen on,
desk occupied, machine exposed) for the remote surfaces to be worth anything.

Sleep *assertions* don't fix it. `caffeinate` / `IOPMAssertion` (and the agents
themselves — this Mac's `pmset -g` already shows `sleep prevented by caffeinate, …,
Claude`) suppress **idle** sleep only. Lid-close sleep is a separate path that
assertions do not override: without an external display + power, a closed clamshell
sleeps anyway. The only user-space switch that keeps a closed-lid Mac running is
`pmset disablesleep 1`, which needs **root**.

That single root requirement is the whole design problem, and it cuts both ways:

- **Getting root once is easy; giving it back reliably is the hard part.**
  `disablesleep` is written into `/Library/Preferences/com.apple.PowerManagement.plist`
  and **survives reboot**. If gtmux sets it and then crashes, is force-quit, is
  uninstalled, or the user reboots, the Mac is left silently unable to sleep — forever,
  with no gtmux running to explain why. A laptop in that state, in a bag, on battery,
  is a genuinely harmful outcome (flat battery + a hot machine in an enclosed space),
  and it is the *default* outcome of a naive implementation.
- **Enabling it is exactly the kind of action that must not be remotely triggerable.**
  A phone tap that makes an unattended laptop never sleep is an escalation with no
  human at the machine to notice the consequences.

So this change is not "shell out to `pmset`". It is a small, auditable **privilege +
guardrail design** around one bit, whose organizing invariant is asymmetry:

> **Escalation (stop sleeping) is local, interactive, and deliberately visible for as long
> as it lasts. De-escalation (sleep again) is free, automatic, unattended, and always
> possible — including when gtmux is dead.**

## What Changes

Introduce **server mode**: a state the user switches on, and which stays on until they
switch it off, during which the Mac keeps running with the lid closed so
`serve`/`tunnel`/the phone keep working.

**It does not expire** (decided 2026-07-29). A timed lease was considered and rejected as
the wrong shape for the job: the user turns this on because they are about to leave a
machine working, and a machine that stops working at hour 8 for no reason the user can
observe is a worse failure than one that keeps going. The weight the expiry would have
carried moves to the two places it belongs — an **informed-consent card before the system
authorization**, and a **persistent menu-bar indicator, in the shape of a recording
indicator**: present for exactly as long as the state is, unhideable, and itself the switch
that ends it. What ends server mode is the user, or a fault (unplug, gtmux gone) — never a
clock. See `design.md` §2 for the residual risk this accepts and why the other guardrails
still make the design fail-safe.

- **A new CLI front door** — `gtmux server-mode [on|off|status]` (the `gtmux quiet`
  shape), `--json` status, en+zh. CLI-first so Homebrew users get it, not app-only.
- **A two-tier keep-awake model** — `awake` (no privilege: gtmux holds a sleep
  assertion; lid must stay open) and `clamshell` (root: `pmset disablesleep`; lid may
  close). The status always says which tier is live, so the capability is never
  overstated.
- **One local admin authorization per enable** (`osascript … with administrator
  privileges`). Same authorization installs a tiny, root-owned **de-escalation-only
  guard** (a LaunchDaemon that can *only ever* restore sleep and remove itself — it has
  no code path that disables sleep). See `design.md` §1 for the four auth paths
  considered and why SMAppService/XPC and a `sudoers` drop-in were rejected.
- **Guardrails, in the order they bite**: prefer AC-scoped `pmset -c` so the kernel
  itself re-enables sleep on unplug · refuse to enable on battery (hard floor under a
  battery threshold, no override) · unplug → auto-exit within a grace window · a heartbeat
  whose staleness restores sleep when gtmux dies · boot reconcile *after a startup grace
  window*, so a reboot of the remote machine resumes server mode instead of killing it ·
  self-removing guard on `off`/uninstall · thermal + physical-siting advisories.
- **gtmux only reverts what gtmux set.** A `disablesleep` that gtmux did not stamp as
  its own is reported, never silently changed — the same report-only discipline
  `gtmux reap` uses for an unclean worktree.
- **Visibility that cannot be missed or hidden** — a dedicated menu-bar indicator for as
  long as server mode is on, in the shape of a recording indicator: unhideable (the
  hide-when-idle preference governs the agent status item, not this), its click action is
  "turn off", neutral while healthy and red **only** when a guardrail wants attention.
  Alongside it: a Preferences section next to 远程访问, a plain-language authorization card
  (DESIGN §5 tone), and an announced exit (notification + push carrying the reason).
- **Remote: read + revoke, never enable.** `GET /api/servermode` and a de-escalate-only
  `POST` are OWNER-scoped; `{"on":true}` is refused by the server with a reason.
  Guests never see server mode at all.

Deviation from the brief, stated up front: the brief said `pmset -a disablesleep 1`.
This design prefers **`-c` (AC profile only)** where the OS honours the scoping, because
it turns "unplug ⇒ sleep works again" from a thing our daemon must poll for into a thing
the kernel enforces. `-a` + polling remains the fallback if the empirical check (task
0.2) shows `disablesleep` is applied globally regardless of profile.

**Non-goals** (explicit): no remote *enable* (see `design.md` §4); no change to any other
`pmset` key (`sleep`/`displaysleep`/`hibernatemode`/`standby` are the user's); no touching
screen-lock, auto-login, or FileVault; no display-on / screensaver control; no silent
thermal auto-exit in v1 (advisory only); no Windows/Linux equivalent; no attempt to keep
the machine reachable *through* a sleep (that is Wake-on-LAN, a different feature).

## Capabilities

### New Capabilities
- `server-mode`: the on-until-switched-off state model, the escalation/de-escalation
  asymmetry, the de-escalation-only privileged guard, the power/crash/boot guardrails, the
  ownership stamp, the two tiers, the status contract, and the announced exit.

### Modified Capabilities
- `remote-access`: server mode is remotely READABLE (owner scope) and remotely
  REVOCABLE, but never remotely enablable — a requirement about which direction of a
  privileged machine state a remote client may move.
- `menu-bar-app`: a dedicated, unhideable server-mode indicator (never overlaid on the
  agent status item, never a radar row) plus the pre-authorization explainer card.
- `env-doctor`: doctor detects a left-behind `disablesleep` and offers a fix only for
  the one gtmux owns.

Feasibility is researched, not assumed: `docs/design/server-mode-research.md` separates
what was **measured** on this hardware (macOS 26.5.2 / M4 Pro) from what is only
**sourced**, names the one blocking physical test, and records the two findings that
changed the design — that a power-source transition is the documented Apple-Silicon
breakage point (hence the lapsed-state detector), and that server mode cannot survive a
reboot on a FileVault-locked Mac (a boundary to document, not a bug to fix).

## Impact

- **New CLI surface**: `gtmux server-mode [on|off|status] [--json] [--tier awake|clamshell]
  [--force]`. Needs the full command-docs treatment in the
  implementing PR: CLAUDE.md command list, `gtmux --help` (en+zh), `## gtmux server-mode`
  in `docs/cli.md`, `api/contract.md` for the endpoints.
- **New package** `internal/servermode` (cgo-free; all sensing via `pmset -g …`
  parsing, like `internal/resource` parses `df`/`memory_pressure`). Consumed by
  `internal/app` (command), `internal/server` (endpoints), and the serve tick
  (heartbeat + guardrail evaluation, reusing the single-writer tick discipline
  `resource·warn` already established).
- **New machine state**, the only privileged footprint gtmux has ever had:
  `/Library/LaunchDaemons/com.gtmux.sleepguard.plist` + a root-owned guard script under
  `/Library/Application Support/gtmux/`. Both are removable by the guard itself, driven
  by an unprivileged marker — so `off` and uninstall never need a second password.
- **New user state**: `~/.local/share/gtmux/server-mode/{state.json,last-exit.json}` (the
  heartbeat and the ownership stamp; no expiry field).
- **Surfaces**: menu-bar (dedicated indicator + Preferences section + auth card),
  mobile (read-only status + "turn off" + push on auto-exit), web (deferred, phase 5).
- **Docs**: `docs/design/DESIGN.md` gains a server-mode section; `docs/design/MOBILE.md`
  §7/§8 gain the read-only + revoke-only rule; `docs/TROUBLESHOOTING.md` gains the
  "my Mac won't sleep any more" entry (the failure this design exists to prevent).
- **Untouched**: the `agents --json` / `digest` / `usage` contracts, the radar, HQ, the
  status-item glyph, dispatch, and every other `pmset` key.
