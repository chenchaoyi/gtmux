## Context

**What the OS actually does.** macOS has two distinct sleep paths:

| path | trigger | suppressed by |
|---|---|---|
| idle sleep | no input / no activity for `sleep` minutes | a power assertion (`caffeinate`, `IOPMAssertion`) — *and already suppressed today*: this Mac's `pmset -g` reads `sleep 0 (sleep prevented by caffeinate, …, Claude)` |
| lid-close (clamshell) sleep | lid shuts | **nothing in user space except `pmset disablesleep 1` (root)**, unless an external display + power + input device are attached |

So the feature's entire root requirement traces to one row of that table. An
assertion-only "server mode" would be a lie: the lid closes and everything stops.

**What `disablesleep` costs.** It is written to
`/Library/Preferences/com.apple.PowerManagement.plist` and **persists across reboot**.
Measured on the dev Mac (Apple M4 Pro, macOS **26.5.2** / 25F84): the key is absent while
unset, and `pmset -g` prints no `disablesleep` line — i.e. *the off state is invisible*,
which is exactly how a left-behind `1` goes unnoticed for months. The harm case is
concrete: laptop in a bag, lid closed, on battery, never sleeping → flat battery and a hot
enclosure. (Note for implementers: the third-party-documented path
`…/SystemConfiguration/com.apple.PowerManagement.plist` **does not exist** on macOS 26 —
the real file is `/Library/Preferences/com.apple.PowerManagement.plist`, and per §3 it is
the ONLY readback, because `pmset` never reports this setting in either state.)

**Two measured traps, both capable of producing the worst bug this feature has.**
`disablesleep` is **undocumented** — it appears nowhere in `man pmset` on macOS 26.5.2,
though it is still recognized (a non-root probe reaches "must be run as root", while a
bogus setting name is rejected outright). And `pmset` **exits 0 when it refuses for lack of
privilege**, printing the error to output. So success must NEVER be inferred from an exit
code: every write is confirmed by reading the setting back, or gtmux will eventually claim
it restored sleep when it did not.

**Full feasibility research — mechanism, precedent, competitive picture, and what is still
unproven — is in `docs/design/server-mode-research.md`.** It separates measured facts from
sourced ones; the one blocking item (does a closed lid actually keep serving on this
hardware) is `tasks.md` 0.1.

**Existing machinery this leans on** (do not rebuild):
`internal/resource` already samples the machine cgo-free by parsing CLI tools, and
already owns the *tick-driven, single-writer, hysteresis + confirmation-window + minimum
restate-interval* discipline for warnings — server mode's power/thermal watching is the
same shape and must reuse it rather than invent a second flapping alarm. `gtmux quiet
[on|off|status]` is the CLI shape to copy. `gtmux reap`'s "gate satisfied → act, else
report only" is the discipline for touching state gtmux may not own.

## Goals / Non-Goals

**Goals**
- A closed-lid Mac keeps serving `serve`/`tunnel`/the phone, on purpose, for as long as the
  user wants it to.
- **Restoring sleep never depends on gtmux being alive, sane, installed, or authorized.**
- The privileged footprint is one small, root-owned, de-escalation-only component whose
  worst-case misuse is "your Mac sleeps".
- Every surface tells the truth about which tier is live, and the state is impossible to
  forget while it is on.
- Works for Homebrew/CLI-only users, not just menu-bar-app users.

**Non-Goals**
- Remote *enable* (§4). Remote *disable* is in scope — it is the safe direction.
- Touching any other power/lock setting. gtmux owns exactly one bit: `disablesleep`.
- Silent thermal auto-exit in v1 (advisory only — a wrong auto-exit kills the remote
  session the user is mid-way through).
- Reachability during actual sleep (Wake-on-LAN / `tcpkeepalive` tuning).
- Reverting a `disablesleep` gtmux did not set.

## 1. Decision — how we get root (the brief's ①)

Four candidate paths, scored against what this feature actually needs. Note the column
that decides it: **"restores sleep when gtmux is dead"** — a crash, a force-quit, a
`brew uninstall`, a reboot. That is the failure mode with a real victim.

| # | path | one-tap after first time | restores sleep when gtmux is **dead** | works for brew-only CLI users | privileged surface | verdict |
|---|---|---|---|---|---|---|
| A | `osascript … with administrator privileges` per action | ✕ (password each enable) | **✕** — nothing left running to revert | ✓ | none persistent | **keep, for ENABLE only** |
| B | `SMAppService` / `SMJobBless` privileged helper + XPC | ✓ | ✓ | **✕** — needs a signed .app bundle; CLI users excluded | **large** — persistent root daemon exposing an RPC that *can be asked to escalate*; must pin client code-signing; versioning/upgrade dance | **rejected** (revisit only if remote enable is ever wanted, §4) |
| C | `/etc/sudoers.d/gtmux` NOPASSWD, restricted to `pmset … disablesleep 0` | ✓ (revert only) | **✕** — someone still has to invoke `sudo` | ✓ | medium — editing `/etc`; sudoers arg-matching is a classic footgun | **rejected**, kept as documented fallback |
| D | print the command, user runs `sudo pmset …` | ✕ | ✕ | ✓ | none | **doc + `doctor` hint only** |
| **E** | **A for enable + a root-owned, de-escalation-only guard installed in the same authorization** | ✕ by design (see below) | **✓** | ✓ | **minimal — the guard has no code path that disables sleep** | **RECOMMENDED** |

**E is A plus a deadman.** The single admin authorization that enables server mode also
(idempotently) installs:

- `/Library/Application Support/gtmux/sleepguard.sh` — root:wheel `0755`, in a root-owned
  directory, a ~40-line `/bin/sh` script that only ever runs SIP-protected binaries
  (`/usr/bin/pmset`, `/bin/launchctl`, `/usr/bin/stat`, `/bin/rm`).
- `/Library/LaunchDaemons/com.gtmux.sleepguard.plist` — `RunAtLoad` + `StartInterval 30`.

Every 30 s (and at every boot) the guard asks three questions and, on any *yes*, runs
`pmset -a disablesleep 0`, writes a reason, then `launchctl bootout`s and deletes itself:

1. is there a revoke marker? (`gtmux server-mode off` writes it — unprivileged)
2. is the heartbeat missing or stale (> 120 s), outside the post-boot startup grace?
3. is the machine on battery for longer than the grace window?

**Where the guard writes its exit reason (a real gap, found while verifying).** The guard
runs as **root, with no user session** — at boot, or after the user logged out — so it
cannot assume it can write into `~/.local/share/gtmux/`. Its record therefore lives at
`/Library/Application Support/gtmux/last-exit.json` (root-owned, world-readable), and
gtmux merges it into `last_exit` on the next status read. Two properties are required, not
optional:

- **It must survive a reboot.** `reason:"boot-reconcile"` exists precisely to explain
  something that happened *before* the machine came back; a record written somewhere the OS
  clears on restart explains nothing. (This is not theoretical: the verification harness for
  this change wrote its only artifact into `$TMPDIR`, and a reboot mid-test destroyed the
  entire result — the one place a temp dir must never be used is for evidence about
  reboots.)
- **It must not be trusted for anything but display.** It is written by root and read by
  the user process; treat it as a message, never as authority over the current state, which
  always comes from the readback.

**Implementation note, learned the hard way during verification.** The guard stops and
removes *itself*, and gtmux stops the guard — both are "terminate a thing you started
earlier". Do that **by launchd label** (`launchctl bootout system/com.gtmux.sleepguard`),
**never by a pid read back from a file**. A recorded pid is only valid while that process
lives; once it exits, the OS recycles the number, and a later `kill` hits whatever
inherited it. This is not hypothetical — the verification script for this very change did
exactly that and killed the operator's terminal window, destroying the test result it was
about to print. Two rules follow, and they apply to the shipped code as much as to the
script: **(1)** identify before you signal — a label, or a pid re-validated against the
process's actual identity; **(2)** produce the outcome *before* the cleanup, so a failed
teardown can never swallow the result the user was waiting for.

Why this shape:

- **It cannot escalate.** The script contains no `disablesleep 1`. A compromised or
  spoofed heartbeat file can at most *delay* a restore — it can never turn sleep off, and
  it cannot conjure the state in the first place, because only the authorized enable path
  writes the setting. Compare B, whose XPC surface exists precisely to be asked to
  escalate.
- **It is the only component that can remove itself, and it will.** `off` and uninstall
  write an unprivileged marker; the guard does the root work. So the user is **never**
  asked for a password to make their Mac *safer* — a property C and D both fail.
- **It survives our death**, which is the whole point, and it self-cleans afterwards:
  a Mac whose gtmux was `rm`'d still returns to normal at the next 30 s tick.
- **No signing, no bundle, no XPC, no cgo.** Works identically for a Homebrew CLI and the
  notarized app.
- **A password per enable is a feature, not friction.** Enabling means "I am leaving this
  machine running unattended"; requiring a human at the keyboard is the cheapest possible
  enforcement of §4's no-remote-escalation rule. `osascript` admin auth also fails cleanly
  in a headless SSH session — which is the correct outcome, with an explicit error.

Rejected-path notes worth keeping: B's real cost is not the code, it is that it splits
the product (app users get one-tap, brew users get nothing) and creates the exact RPC
that §4 says must not exist. C's `sudo -n pmset … disablesleep 0` is genuinely narrow
(fixed path on a SIP-protected binary, fixed args, no wildcards) and is a fine *manual*
recipe for `docs/TROUBLESHOOTING.md`, but it does not run itself.

**Precedent (research §4).** This is not an exotic route: **Amphetamine**, the
best-known tool in this niche and an App Store app, implements closed-display mode with
`do shell script "pmset disablesleep 1" with administrator privileges` — the same
mechanism and the same escalation path, sustained across years of macOS releases. Recorded
honestly alongside it: Apple regards that AppleScript path as equivalent to `sudo` rather
than an app API, and the underlying `AuthorizationExecuteWithPrivileges` has been
deprecated-but-working for years. The mitigation is a **fallback ladder, not a rewrite** —
if it is ever closed, drop to option C (narrow `sudoers` drop-in) or D (guided manual
`sudo`). Both preserve the asymmetry; neither needs a helper. Research §4 also confirms the
disqualifier for B empirically: `SMAppService` daemons are *bundle-scoped by construction*
(they live in the `.app` and die with it), so a helper route structurally cannot serve
Homebrew CLI users.

**AC-scoped write — attempted, and REFUTED by measurement (2026-07-31).** The plan was
`pmset -c disablesleep 1`, so that unplugging would re-enable lid sleep *by kernel policy*.
It does not work: `-c` is accepted, the write succeeds, but the value lands in the plist's
**`SystemPowerSettings`** top-level dictionary — not under `AC Power` — i.e. **the setting
is global and applies on battery too**. The fallback the design already specified is
therefore now the ONLY path: write `-a` and let the guard poll the power source.

The consequence is not cosmetic, and it cuts toward *more* safety machinery, not less:

- **G3 is deleted.** There is no kernel-enforced unplug recovery to lean on.
- **G4 (the guard's power poll) is promoted from belt-and-braces to the sole defence.**
  Without it, unplugging a server-mode Mac leaves it unable to sleep *on battery* — which
  is precisely the harm case this whole change exists to prevent (bag, battery, hot).
- **G1 (refuse on battery) matters more**, because there is no profile-level protection
  underneath it.

## 2. Decision — guardrails (the brief's ②)

Ordered by how early they bite. Each is independently sufficient to end server mode;
none depends on another.

| # | guardrail | mechanism | default |
|---|---|---|---|
| G1 | **charge-gated enable** | `on` reads `pmset -g ps`; on battery BELOW the warn threshold → refuse (a session that would end at once isn't worth an authorization). Above it: allowed, running on battery is a supported case (§2.3) | warn threshold 30 % |
| G2 | **charge floor → exit** | guard polls remaining charge; at the floor → restore sleep, `reason:"battery-low"` | 20 % |
| G3 | ~~AC-scoped setting~~ | **REFUTED 2026-07-31** — `-c` is not honoured; the setting is global (§1). No kernel-enforced unplug recovery exists. | n/a |
| G4 | ~~unplug auto-exit~~ | **REFUTED as a policy 2026-07-31** — unplugging is a normal part of the core use case (§2.3). Replaced by G2's charge floor. Warn (Mac + push) at the threshold, exit at the floor. | n/a |
| G5 | **heartbeat** | gtmux rewrites `state.json` every ~30 s; guard restores when stale > 120 s | covers crash / force-quit / logout / `kill -9` |
| G6 | *(none — no time limit)* | server mode ends when the user ends it or a fault ends it; see §2.1 | decided 2026-07-29 |
| G7 | **boot reconcile, after a startup grace** | `RunAtLoad` + a grace window for launchd to bring gtmux back; heartbeat resumes → stay on, otherwise restore + self-remove. Independently, `server-mode status` and `doctor` compare the readback against the ownership stamp | 5 min grace |
| G8 | **explicit exit** | `off` (local or remote) writes the revoke marker | always |
| G9 | **uninstall** | `uninstall-app` / a missing gtmux both degrade to G5 → guard restores + self-removes | always |
| G10 | **thermal** | advisory in v1: reuse `internal/resource`'s load/memory sampling and its hysteresis + confirm-window + restate-interval so it cannot flap; surface a warning, do not auto-exit | advisory |
| G11 | **siting advisory** | the auth card says, plainly: a closed lid dissipates heat worse — hard surface, vents clear, not in a bag | copy |
| G12 | **ownership stamp** | gtmux records that *it* set `disablesleep`; anything unstamped is report-only, never reverted | always |
| G13 | **lapsed-state detection** | the readback is re-checked on the tick: if the machine says sleep is enabled while gtmux thinks server mode is on, the state is LAPSED — announce it, do not pretend | always (see §2.2) |

### 2.1 Decided 2026-07-29 — server mode does not expire

An earlier draft made server mode a lease with a hard cap (8 h default / 24 h max). That
was reviewed and **rejected**: the user turns this on *because* they are leaving a machine
working, so an unobservable clock that stops the work is a worse failure than the work
continuing. Enable it, and it stays on until it is switched off.

What that costs, stated plainly, because it is a real trade and not a free one:

- **The hard expiry was the only guardrail triggered by time rather than by a fault.**
  Every other row of the table above fires because something *happened* — unplugged,
  crashed, rebooted, revoked. If nothing happens, nothing ends it. The "enabled it on
  Friday, still on Monday" path is now reachable by design.
- **R2 (§6) downgrades from bounded to accepted.** With no cap, a local process that keeps
  the heartbeat fresh keeps the machine awake indefinitely. It still cannot *escalate* —
  only the authorized enable path can create the state — so the worst case is a machine
  that stays awake, which is what the user asked for.

What carries that weight instead — and these are load-bearing, not decoration:

1. **Informed consent at the moment of authorization.** The card (§4) says, in the same
   breath as the password request, that this stays on until switched off. That sentence is
   now a requirement, not copy.
2. **A persistent indicator, modelled on the screen-recording indicator.** Present for
   exactly as long as the state is, its own click action is "turn off", and — critically —
   **not hideable**: the `hide-when-idle` preference governs the agent status item, which
   is about *fleet noise*; this is a *machine state* the user chose to leave running. macOS
   itself makes the same call for recording and for location use.
3. **Every fault-triggered guardrail stays.** Unplug, crash, uninstall, and a boot with no
   gtmux still restore sleep. The design is still fail-safe against *faults*; it is
   deliberately no longer fail-safe against *forgetting*, which is the indicator's job.

Consequence worth noting for implementation: with no expiry, the *only* thing keeping the
setting alive is the heartbeat, so the boot path needs a startup grace window (G7) —
otherwise every reboot of the remote machine would kill server mode before launchd had
brought `gtmux serve` back, which is exactly when the user is least able to fix it.

### 2.3 Battery is a use case, not only a hazard (requirement stated 2026-07-31)

The first draft's power guardrails rested on an assumption the user has now corrected:
that "on battery, lid shut, not sleeping" is always the harm case (bag, overnight, flat
battery, hot enclosure). It is also the **core scenario**: carrying the laptop between
meeting rooms — lid shut, on battery, minutes at a time — with agents expected to keep
working across the walk. Ending server mode the moment the cord is pulled would break the
feature precisely when the user most wants it.

The distinguishing variable is not *whether* it is on battery. It is **how long, and how
much charge is left**. So the guardrails re-cut around a floor, not around the power source:

| | first draft | corrected |
|---|---|---|
| G1 | refuse to enable on battery | **allow on battery** above a charge floor, with the state clearly shown; refuse below it |
| G4 | on battery past a 60 s grace ⇒ restore sleep | **charge below the floor ⇒ restore sleep**; being on battery is not by itself a reason to end |
| new | — | **warn before the floor** (Mac notification + push): "server mode is running on battery, N% left" — the user is away from the machine, so the warning has to reach the phone |

The harm case is still covered — a laptop forgotten in a bag drains toward the floor and
then restores sleep on its own — but it is covered by the thing that actually predicts harm
(remaining charge) rather than by a proxy that also kills the good case (power source).

**This is gated on A2b** (`tasks.md` 0.2b), which is now blocking: if closed-lid operation
does *not* survive unplugging on Apple Silicon, none of this matters and server mode must
be documented as an on-AC feature. A2's measurement (the setting is global, not
per-power-profile) predicts it *should* survive; Amphetamine's documentation predicts it
should not. Only the physical test settles it, and the honest outcome if it fails is to say
the between-rooms scenario is not deliverable this way — not to ship something that quietly
stops working in a corridor.

### 2.2 Apple Silicon: a power transition is the fragile moment (research §5)

Research turned up a failure mode neither the brief nor the first draft anticipated, and it
is not theoretical: on Apple Silicon, **connecting or disconnecting external power is the
documented breakage point for closed-display operation.** Amphetamine carried it as a
bug-class before 5.3, still warns that closed-display "may not work as expected" across a
power-source change, and — the instructive part — its answer in 5.2 was to add a *failed
session* alert rather than to pretend the session was healthy.

Three things follow, and they are already folded into the guardrails above:

1. **G4 (unplug) is a system behaviour we surface, not a policy we invented.** The vendor
   states plainly that OS limitations mean closed-lid operation cannot be maintained once
   the cord is pulled. gtmux's contribution is to *notice and announce* it, which is what
   the announced-exit requirement is for.
2. **Re-plugging is its own test case** (`tasks.md` 0.2b). "Unplug → sleeps" passing does
   not imply "plug back in → still works". That asymmetry is exactly where the incumbent
   broke.
3. **A silently-lapsed state must be expressible** — hence G13. The indicator has to be
   able to say "this stopped working", not just on/off, or gtmux would be reporting a
   closed-lid session that is quietly dead. This is the one place where the persistent
   indicator's attention colour is load-bearing rather than cosmetic.

Thermal, honestly: `pmset -g therm` returns *"No thermal warning level has been
recorded"* on the dev M4 Pro **[measured]**, so it is **not** a usable gate on Apple
Silicon. v1 reads it best-effort, otherwise leans on load/RSS from `internal/resource`, and
warns rather than acts — auto-killing a remote session on a noisy heuristic is worse than
the heat. The advisory copy should name the sharp case rather than generalise: a **fanless
MacBook Air dissipates heat through the keyboard area**, so extended closed-lid load can
cost roughly half its sustained performance (research §6). "Air, closed, under load" is the
configuration to warn about by name.

Security note (not a new exposure, but a longer window): server mode extends the period
in which an unattended Mac is remotely reachable and, via `POST /api/send`, remotely
*typeable*. The auth card must say so in one sentence and point at the existing
remote-access exposure model. gtmux does **not** touch screen lock: the display still
locks per system settings, and closing the lid still blanks the screen.

## 3. Decision — state, tiers, and the status contract

**Two tiers, never conflated.**

| tier | mechanism | privilege | lid | when it is the right answer |
|---|---|---|---|---|
| `awake` | gtmux holds a sleep assertion for as long as server mode is on | none | must stay **open** | desktops (no lid), or "don't idle-sleep while I step away" |
| `clamshell` | `pmset disablesleep` + the guard | one admin auth | may **close** | the actual feature: remote command of a shut laptop |

A machine with no internal battery (Mac mini/Studio) skips G1/G2/G4 and is told plainly
that `awake` is usually all it needs.

**Status shape** (`gtmux server-mode status --json`, and the body of `GET /api/servermode`):

```json
{
  "state": "on",                       // on | off | ended
  "tier": "clamshell",                 // clamshell | awake
  "since": 1753800000,
  "heartbeat_at": 1753803600,          // liveness, NOT an expiry
  "power": "ac",                       // ac | battery
  "battery_pct": 100,                  // omitted when no internal battery
  "guard": { "installed": true, "healthy": true },
  "system_disablesleep": true,         // raw readback
  "owned_by_gtmux": true,              // G12 stamp — false ⇒ report-only
  "last_exit": { "at": 1753790000, "reason": "unplugged" }
}
```

`reason` ∈ `revoked | unplugged | stale-heartbeat | boot-reconcile | thermal |
uninstalled | lapsed` — note the absence of `expired`, and the presence of `lapsed` (§2.2:
the setting stopped taking effect on its own). The reason is a first-class field, not a log
line: it is what the phone push says, and what `doctor` explains.

**Readback — measured 2026-07-31. Three candidate sources, only one is correct.** This
took two wrong answers to get right, and both wrong answers were live-reproduced bugs, so
the table is normative for `internal/servermode`:

| source | verdict |
|---|---|
| `pmset -g` / `-g custom` / `-g live` | ❌ **never shows `disablesleep`, in either state.** Using it reports "sleep restored" while the Mac cannot sleep — the worst failure this feature has. |
| `/Library/Preferences/com.apple.PowerManagement.plist` → `SystemPowerSettings.SleepDisabled` | ⚠️ real, world-readable, but **lags**: one second after a successful write it still read the old value, producing a false "restore failed". It also expresses the *persisted* setting, not what the kernel is doing right now. |
| **`ioreg -r -c IOPMrootDomain -d 1` → `"SleepDisabled" = Yes\|No`** | ✅ **the kernel's live state — immediate, unprivileged. This is the readback.** |

```
ioreg -r -c IOPMrootDomain -d 1 -w0 | grep -E '"SleepDisabled"[[:space:]]*=[[:space:]]*Yes'
```

Two corollaries that will otherwise bite the implementation:

- **Off is `false`, not absent.** After a restore the plist holds `SleepDisabled => false`
  — the key stays. A never-set machine has no key at all. So "is the key present" is NOT
  the question; the value is. Both states must parse.
- **Use both sources, for different questions.** `ioreg` answers "is sleep disabled right
  now" (what every surface shows). The plist answers "will it still be disabled after a
  reboot" — which is exactly what the boot-reconcile and `doctor` checks care about. They
  are not redundant, and neither substitutes for the other.

Hence the discipline: **state comes from the `ioreg` readback**, never from gtmux's own
record, never from a `pmset` exit code, and never from the plist alone.

## 4. Decision — surfaces, and why the phone cannot enable (the brief's ③)

**The asymmetry maps onto the surfaces directly:**

| action | local CLI | menu bar | phone / web | guest share |
|---|---|---|---|---|
| read status | ✓ | ✓ (persistent indicator) | ✓ (owner scope) | **✕ — invisible** |
| turn **off** | ✓ | ✓ | **✓** | ✕ |
| turn **on** | ✓ (admin dialog) | ✓ (admin dialog) | **✕ — server refuses** | ✕ |

Why remote enable is refused, concretely — three independent reasons, any one sufficient:

1. **Mechanism.** Enabling needs an interactive GUI authorization; there is no one at the
   machine to answer it. Making it work would require exactly the persistent root RPC §1
   rejected.
2. **Consequence asymmetry.** A wrong remote *off* costs a dropped remote session, and
   the user is holding the phone that caused it. A wrong remote *on* burns an unattended
   laptop's battery in a bag, possibly for days, with no one to notice — and it is the
   direction an attacker with a leaked bearer token would want.
3. **Consent shape.** "Keep my machine running while I'm away" is a decision made *before*
   leaving, at the machine. Remote *revocation* ("I'm done, stand down") is the natural
   remote half.

Remote **off** is therefore not a compromise, it is the same invariant: de-escalation is
always allowed, from anywhere. Guests never see server mode at all — it is a machine-level
control, owner-scoped like `/api/digest`.

(The "pre-authorized remote extension" idea an earlier draft deferred is now moot: with no
expiry there is nothing to extend. Remote *enable* stays refused for the three reasons
above — that is unchanged by §2.1.)

**Menu bar (DESIGN.md conformance, the brief's ④).** The reference the reviewer gave is
exact and decides the form: **a screen-recording indicator.** macOS puts recording in its
own menu-bar item, always visible while it lasts, clickable to stop, and unhideable —
because it is a state you must not be able to forget you are in. Server mode is the same
species of state, so it gets the same treatment. The existing gtmux design language
decides the rest:

- **Its own status item — not the agent one.** DESIGN §1/§2 make the agent status item's
  glyph mean *the fleet's most-urgent agent state*. Overlaying a machine state on it would
  break "颜色只表达状态" and dilute waiting-red. A second, dedicated item sits alongside it,
  appears only while server mode is on, and disappears the moment it ends — zero footprint
  when off.
- **Not hideable.** The `hide-when-idle` preference (§2 mode 3) exists so an idle *fleet*
  does not nag; it governs the agent item only. This indicator is the mechanism that
  replaces the expiry we removed (§2.1), so it is not subject to a "don't bother me"
  preference. Same call macOS makes for recording and location use.
- **Its click action is "turn off".** A small menu: tier, how long it has been on, why it
  is red if it is red, and 关闭服务器模式 as the primary item. The indicator *is* the switch.
- **Radar: untouched.** No row, no section. (§16's red line, generalized: server mode is
  not an agent.)
- **Shape carries presence, colour carries attention.** A distinct glyph reading as "this
  machine is kept awake"; neutral `#8E8E93` while healthy; the authoritative red `#EF4444`
  **only** when a guardrail wants the user (on battery under `--force`, guard unhealthy,
  thermal advisory). This follows the HQ medallion precedent — 红 = 需要注意 — and the
  deliberate correction in `b1272c4` that a soft amber must not read as red. **No new
  colour token, no gradient, no glow, and no orange** (macOS's recording orange is not in
  §9's palette and would collide with waiting-red at a glance).
- **Motion: none.** §10 allows exactly one animation in the product (idle→waiting pulse).
  A permanently-present indicator that pulses would be intolerable.
- **Preferences**: a section adjacent to 远程访问 (§13) — the toggle, the tier, how long it
  has been on, "requires one admin authorization", and 关闭. Same 460-wide grouped-form grid.
- **Auth card copy** follows §5's tone rule (平实、就事论事、禁止营销腔), en+zh. Draft:

  > **服务器模式需要一次管理员授权**
  > 开启后，合上盖子 Mac 也继续运行，`gtmux serve` 与隧道保持在线，你可以在手机上继续指挥。
  > 这会改动系统的一项电源设置（`disablesleep`），因此需要输入一次管理员密码。
  >
  > 1. 点「继续」，会弹出 macOS 系统对话框
  > 2. 授权后同时安装一个只负责**恢复睡眠**的守护程序 —— 它不能开启服务器模式，只会在电量见底、
  >    gtmux 退出或被卸载后把睡眠设置改回去，然后自我卸载
  > 3. **开启后会一直生效，直到你自己关闭** —— 不会自动到期。菜单栏会一直显示一个服务器模式
  >    标记，点它随时可以关闭；配对的手机上也能看到、也能关。
  >
  > **需要你知道的三件事：**
  > *· 电量* —— 用电池时不会自动停，会一路跑到 {floor}% 才恢复睡眠；到 {warn}% 会先提醒你
  >   （Mac 和手机各一次）。合盖跑满负载大约还能撑 {eta}。
  > *· 发热* —— 合着盖子散热更差，尤其是无风扇的 MacBook Air（持续高负载可能掉一半性能）。
  >   请放在硬质平面、别塞进包里或压在软垫上。
  > *· 敞口* —— 开启期间这台 Mac 会长时间无人值守地保持可远程访问；任何持有 token 的人都能
  >   在上面执行命令。屏幕锁不受影响，仍按你的系统设置照常锁。

  (en mirror in the implementing PR; both strings via `internal/i18n`. Note item 3 is a
  spec requirement, not decoration — it is what replaced the expiry; the three bullets are
  the "limitations" the user asked to be made explicit, with live numbers substituted.)

  **On an unverified macOS** the card gains one more line, before the button, and the
  button's label changes from 「继续」 to 「我知道，仍然开启」:

  > ⚠️ 这个开关依赖一项 Apple 未公开文档的系统设置。我们只在 macOS {tested} 上验证过，
  > **你的 {current} 没有验证过** —— 它可能不生效。开启后请务必做一次验证：合上盖子等两分钟，
  > 再打开看 gtmux 是不是一直在服务。验证通过后这条提示就不再出现。

**Mobile (MOBILE.md).** §7 连接与推送 gains the read-only status line (「服务器模式 · 已开启
3天2小时」) with a single 关闭 action, and §8 状态与边界 gains the row: 无 server mode → 不显示;
远程开启 → 不提供，并说明原因（需要在 Mac 上授权）. Push on auto-exit reuses the existing pipeline
and carries the `reason`; HQ stays out of it (#557 — HQ pushes nothing).

## 5. Testing strategy

Everything privileged is injected, never executed in tests:

- `internal/servermode` takes a runner interface; unit tests drive fixtures of real
  `pmset -g ps` / `pmset -g` / `pmset -g custom` output (including the *absent*
  `disablesleep` line, the state that hides).
- Golden test on the generated guard script + plist — the file is a security artifact;
  its content must not drift silently (the `internal/docs` rendered-block discipline).
- Table tests per guardrail: battery refusal, floor, unplug grace, stale heartbeat,
  boot reconcile inside vs. outside the startup grace, revoke marker, ownership stamp
  (unstamped ⇒ report-only).
- Server tests: `GET` owner-only; guest 403 on read *and* write; `POST {"on":false}` OK;
  `POST {"on":true}` refused with a reason; SSE emits on change.
- Menu-bar tests: the indicator is present iff the state is on, survives the
  `hide-when-idle` preference (the regression §2.1 depends on), and its colour matches the
  palette-consistency assertion that already guards `Theme.Status`.
- Manual, on real hardware, and non-negotiable before the tier ships (CI cannot close a
  lid): the §6 phase-2 checklist.

## 6. Risks & open questions

- **A1 (blocking, task 0.1): does `disablesleep` actually keep a lid-closed Apple-Silicon
  Mac awake with no external display?** The whole feature rests on it. If it does not, the
  honest outcome is to ship the `awake` tier only and say so — not to fake it.
- **A2: is `-c` scoping honoured for `disablesleep`?** Decides G3; fallback specified.
- **A3: readback path** for the on state on macOS 15+ (§3).
- **A4: does the admin dialog accept Touch ID** for `do shell script`? UX only.
- **R1: a root LaunchDaemon is a real footprint** for a tool installed by `brew`. Mitigated
  by: de-escalation-only, self-removing, tiny, SIP-protected binaries only, root-owned
  path, golden-tested content, listed by `doctor`, documented. Accepted — the alternative
  (no guard) is the harmful-default design this change exists to avoid.
- **R2: a user-writable heartbeat file feeding a root daemon.** Bounded by construction:
  it can only *delay* de-escalation, never cause escalation. With no hard expiry (§2.1) this
  is an accepted residual risk rather than a bounded one — a local process that keeps the
  heartbeat fresh keeps the machine awake. It gains nothing it did not already have: only
  the authorized enable path can create the state in the first place.
- **R3: fighting other assertions.** gtmux sets exactly one key and reads the rest.
  `server-mode status` should *report* who else is holding assertions (`pmset -g` already
  names them) rather than trying to win.
- **R4: scope creep into a power-management panel.** Explicit non-goal; one bit only.
- **R5 (from research §3/§4): the foundation is undocumented, and the escalation path is
  deprecated-but-working.** `disablesleep` is absent from `man pmset`; `do shell script …
  with administrator privileges` is not an app API in Apple's view. Accepted, because the
  fallback ladder (§1) survives losing either one, and the readback discipline turns "the
  mechanism stopped taking effect" into a *detected, announced* state (G13) rather than a
  silent lie. A decade of Amphetamine shipping on the same two foundations is the empirical
  argument that this is not reckless.
- **R6 (from research §7): server mode does not survive a reboot on a FileVault-locked
  Mac.** `gtmux serve` is a per-user **LaunchAgent**, so it starts only once someone logs
  in. Reboot a FileVault Mac with nobody there and the heartbeat never resumes → the guard
  restores sleep after the startup grace. That is the *correct* fail-safe (a machine nobody
  can reach should sleep), but the docs must say it plainly instead of implying always-on.
  gtmux must NOT "solve" this by touching FileVault or auto-login — explicit non-goal.
  Anyone who needs true unattended reboot survival should be pointed at a Mac that stays
  logged in, not at a weakened disk-encryption posture.
- **Q1: should the menu-bar app's quit end server mode?** Proposed **no** — server mode is
  machine state owned by the heartbeat + guard, not by the app. But note the interaction
  with §2.1: the indicator lives in the app, so quitting the app hides the indicator while
  the state persists. Proposed resolution: quitting the app while server mode is on warns
  and offers to turn it off; if the user quits anyway, `gtmux serve` keeps the heartbeat and
  the CLI/phone still show and can end it. Flagged for the reviewer.
- **Q2: RESOLVED 2026-07-29 — no time limit** (§2.1). Was: default 8 h cap.

## 7. Robustness & platform boundaries (requirement stated 2026-07-31)

Everything measured for this change was measured on **one** configuration: Apple M4 Pro,
macOS 26.5.2. The mechanism itself is undocumented by Apple. So "works on my machine" is
the literal, complete extent of what is known, and the product has to behave accordingly —
the failure to avoid is a user enabling server mode, closing the lid, walking away, and
finding a dead machine because the mechanism silently did nothing on their OS.

### 7.1 Three gates, in order, before anything is authorized

Each is unprivileged, so a refusal costs the user nothing — no password prompt, no change:

| gate | check | on failure |
|---|---|---|
| **host** | is this macOS at all? | refuse — the mechanism is macOS-only, say so |
| **mechanism** | is the setting name still accepted? (probe as non-root: a live name reaches "must be run as root", a bogus one is rejected outright — the exact discrimination used during verification) | refuse: this macOS no longer accepts the setting gtmux relies on |
| **readback** | does the kernel expose the live state key? | refuse: without it gtmux cannot tell whether sleep is on, and it will not manage a state it cannot see |

**Version is a fourth signal, but a softer one.** An unverified macOS is *unverified*, not
*unsupported* — a hard version allowlist would break the feature on every future macOS
until someone shipped a release, which is worse than an honest warning. So: name the
version, say it has not been verified by the project, tell the user how to verify it
themselves in five minutes (enable → close the lid → confirm it kept serving → the
`verify-premise.sh` flow), and let them proceed deliberately. Record the outcome so the
warning stops after their machine has proven itself.

### 7.2 Verify the effect, never the exit status

Two measured facts make this non-negotiable: the power tool **exits 0 when it refuses for
lack of privilege**, and the state is invisible to that tool's own reporting commands. So
an enable is: apply → read the live kernel state back → only then write the ownership
stamp. No readback, no stamp, no "server mode is on" anywhere in the UI.

The same rule kills the half-applied state: **sleep disabled but no guard installed** is
strictly worse than not enabling at all, because nothing would restore sleep if gtmux went
away. Either both land, or the setting is reverted and the failure explained.

### 7.3 What else is worth doing (beyond the stated ask)

- **Tell the user when server mode is pointless.** If neither `serve` nor a tunnel is
  running, enabling it keeps a Mac awake for nothing. Say so before the password prompt.
- **Notice an external display.** With one attached, macOS already supports lid-closed
  operation natively; offer that as the cheaper answer rather than taking an authorization.
- **Report time, not just percent.** "≈2h left at the current draw" is actionable in a way
  "47%" is not, and the user reading it is on their phone, away from the machine.
- **Keep an audit line per transition** (enabled / ended + reason + charge at the time).
  Cheap, and it turns "why did my Mac sleep" into a question with an answer.
- **Say what will and will not happen to the screen.** Server mode does not touch the
  lock: the display still sleeps and locks per the user's settings. Users assume otherwise,
  in both directions.
- **Make the first enable teach the limits.** Not a wall of text every time — the first
  time, offer the five-minute self-verification, then stop mentioning it.
