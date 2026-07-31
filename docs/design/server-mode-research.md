# Server mode — closed-lid operation research (2026-07-30, updated 07-31)

Feasibility research for the `server-mode` change (`openspec/changes/server-mode/`):
can gtmux keep a MacBook working with the lid closed, so `serve` + `tunnel` + the phone
keep answering, and can it do that without ever leaving the machine unable to sleep?

Two kinds of claim are kept strictly apart below, because they carry different weight:

- **[MEASURED]** — run on the dev machine (Apple M4 Pro, macOS **26.5.2** / build 25F84)
  during this pass. Reproducible.
- **[SOURCED]** — from documentation or a credible third party. Good enough to design
  against, not good enough to ship on.

Anything that is neither is called out as **open** — see §7. One open item is blocking.

## 1. Verdict

**PROVEN on the target hardware (2026-07-31). The blocking test passed: a closed-lid M4
Pro with `disablesleep` on kept serving for the full test, with the kernel confirming zero
sleeps.** The approach is sound, has a decade-old precedent, and is now measured rather
than argued.

- The mechanism gtmux needs exists, is still recognized on macOS 26.5.2 **[MEASURED]**, and
  is the same mechanism the category leader (Amphetamine, on the Mac App Store) has used
  for years **[SOURCED]**.
- The reason a cheaper, unprivileged path won't do is not a gtmux limitation but a macOS
  architecture fact: lid-close sleep is a separate, hardware-triggered path that power
  assertions do not participate in — **confirmed on this hardware 2026-07-31**: a held
  `caffeinate` assertion did not survive a closed lid (§2.0). **[MEASURED]**
- The recommended authorization path (§4) is exactly what Amphetamine does, including from
  a sandboxed App Store binary **[SOURCED]** — so it is not an exotic choice.
- **A1 — closed-lid operation: CONFIRMED [MEASURED]** (§7). Lid shut ~4 min with
  `disablesleep` on: kernel Sleep/Wake counter unchanged (58 → 58), recorder gap 6 s
  (i.e. none), `/api/health` 50/50 × 200. Against the 0.0 control on the same machine —
  which slept within 133 s (`pmset -g log`: *"Entering Sleep state due to 'Clamshell
  Sleep'"*) — the mechanism is doing exactly what it claims.
- **A2 — AC-only scoping: REFUTED [MEASURED]** (§7). `pmset -c` is accepted but the value
  lands in the plist's `SystemPowerSettings` top level, i.e. **globally, battery included**.
  There is no kernel-enforced recovery on unplug, so the guard's power poll is the *sole*
  defence, not a backup. This makes the guard more load-bearing than the first draft assumed.
- **A2b — battery operation: CONFIRMED [MEASURED]** (§5.1). Unplugged, lid shut, 42 of 45
  samples on battery: zero sleeps, served throughout, and re-plugging did not lapse it. The
  "carry it between meeting rooms" use case works. This **contradicts the incumbent's own
  documentation**, which says the adapter cannot be pulled — measurement wins.
- **A3 — how to read the state: ANSWERED, after two wrong answers** (§3.1). `ioreg`'s
  `IOPMrootDomain.SleepDisabled` is the only correct readback; `pmset` never shows it and
  the plist lags. Both wrong answers were reproduced as live bugs, one of which reported
  a Mac as "will sleep normally" while it could not sleep.

One finding materially hardened the design (§5): on Apple Silicon, **plugging or
unplugging power is the known-fragile moment for closed-lid operation**, not a theoretical
edge. Amphetamine shipped a bug-class and then a user-facing failure alert for exactly this.

## 2. Why an unprivileged path cannot work

macOS has two independent sleep paths, and only one of them listens to assertions:

| path | trigger | can user space suppress it? |
|---|---|---|
| idle sleep | inactivity past the `sleep` timer | **yes** — an IOKit power assertion (`caffeinate`) |
| lid-close (clamshell) sleep | the lid sensor | **no** — assertions do not participate |

- Closing the lid "is not ordinary inactivity — it is a separate hardware-triggered sleep
  path", so a busy process *and* an idle-sleep assertion can both be live while the machine
  sleeps the instant the lid shuts **[SOURCED]**.
- `caffeinate -s` ("prevent the system from sleeping") is explicitly qualified in its own
  man page: *"This assertion is valid only when system is running on AC power."*
  **[MEASURED — `man caffeinate`]**. It says nothing about the lid, and the assertion type it
  raises (`PreventSystemSleep`) is an idle-path concept.
- Live assertion types on this machine are all idle/display-path: `PreventUserIdleSystemSleep`,
  `PreventUserIdleDisplaySleep`, `NoDisplaySleepAssertion`, `SystemIsActive`,
  `PreventSystemSleep` **[MEASURED — `pmset -g assertions`]**. None of them names the lid.
- What *does* work: `pmset disablesleep 1` sets a kernel flag (`SleepDisabled`) that
  `IOPMrootDomain` treats as a veto on sleep which **persists through the lid-close event**
  **[SOURCED]**. That is a different layer from assertions, and it is why root is required.

### 0.0 — measured on the target hardware (2026-07-31)

The claim above is no longer only sourced. Run on this Mac (M4 Pro, macOS 26.5.2, **no
external display**): hold `caffeinate -dis`, close the lid ~3 minutes, reopen.

| signal | result |
|---|---|
| kernel `Sleep/Wakes since boot` | **57 → 58** — the kernel counted a sleep |
| recorder log gap | **136 s** — user space was frozen for the whole closed-lid period |
| `/api/health` samples | 11/11 × HTTP 200 — `serve` was healthy before and after, so the gap is sleep, not a dead server |

Two independent signals agree, and the third rules out the obvious confound. **A held
`caffeinate` assertion does not survive a closed lid on Apple Silicon.** So root is not a
design preference here — it is the only remaining mechanism, exactly as §2 predicted.
`[MEASURED]`

**Consequence for gtmux:** the two-tier model in the design is not padding. The `awake`
tier (assertion) is honestly incapable of surviving a closed lid, and any UI that implies
otherwise would be lying. Only the `clamshell` tier (root) does the thing users want.

Independent confirmation that this is a real, live pain point rather than a hypothetical:
the Claude-Code-on-your-phone ecosystem has published workarounds for it, and every one of
them is an idle-path assertion or a manual Energy-Saver change — i.e. none of them actually
solves the lid **[SOURCED]**. See §6.

## 3. What is true about `disablesleep` (the risky part)

**It is undocumented.** `disablesleep` appears **nowhere** in `man pmset` on macOS 26.5.2
**[MEASURED]** — not in `SETTINGS`, not anywhere in the page — and it is absent from the
widely-used third-party pmset references too **[SOURCED]**. Everything gtmux would build on
it is therefore load-bearing on an interface Apple has never promised. That is a real risk
(§7/R5), not a footnote.

**It is still recognized on macOS 26.5.2 [MEASURED].** Probed without root, so nothing was
changed:

```
$ pmset -c disablesleep 1          → "'pmset' must be run as root..."   (parsed OK)
$ pmset -a disablesleep 1          → "'pmset' must be run as root..."   (parsed OK)
$ pmset -a totallybogussetting 1   → "Usage: pmset <options>"           (rejected)
```

Argument parsing happens before the privilege check, so reaching "must be run as root" is
positive evidence the setting name is live, and the bogus-name control proves the probe
discriminates. It also shows **`-c` is accepted syntactically** — though acceptance is not
the same as the scoping being *honoured* (§7/A2).

**Implementation trap [MEASURED]:** that failure returned **exit status 0**. `pmset` prints
"must be run as root" and exits 0. So a caller **must not** infer success from the exit
code — verify by reading the setting back. Missing this would produce the worst possible
bug in this feature: gtmux believing sleep is disabled when it isn't, or believing it
restored sleep when it didn't.

**It persists across reboot [SOURCED + consistent with MEASURED].** pmset settings live in
a system-wide plist and survive restarts. This is the property that makes a left-behind `1`
dangerous, and it is why the design has a boot-time reconcile at all.

**Path correction [MEASURED].** Third-party references (and an earlier draft of our own
design) give `/Library/Preferences/SystemConfiguration/com.apple.PowerManagement.plist`.
On macOS 26.5.2 that file **does not exist**; the real one is
`/Library/Preferences/com.apple.PowerManagement.plist` (root:wheel, world-readable).

### 3.1 How to actually read the state — A3, answered the hard way [MEASURED 2026-07-31]

Three candidate sources; **two of them produced live, reproduced bugs** in the verification
harness before the right one was found. This table is normative for the implementation:

| source | verdict |
|---|---|
| `pmset -g` / `-g custom` / `-g live` | ❌ **never prints `disablesleep`, in either state.** The harness used this and told the operator "sleep is restored, the Mac will sleep normally" while `SleepDisabled` was true — the exact worst-case failure this feature is designed to avoid, reproduced by accident. |
| plist → `SystemPowerSettings.SleepDisabled` | ⚠️ real and world-readable, but **lags the write**: one second after a successful `pmset -a disablesleep 0` it still read `true`, so the harness reported "restore FAILED" when the restore had in fact worked. It also describes the *persisted* setting, not current kernel behaviour. |
| **`ioreg -r -c IOPMrootDomain -d 1` → `"SleepDisabled" = Yes\|No`** | ✅ **the kernel's live state. Immediate, unprivileged, authoritative.** |

Two traps worth stating explicitly:

- **Off is `false`, not absent.** A machine that has never had this set has **no**
  `SleepDisabled` key; a machine that had it and restored holds `SleepDisabled => false`.
  Both mean "off", so presence-of-key is the wrong test — parse the value.
- **The two live sources answer different questions, and both are needed.** `ioreg` =
  "is sleep disabled *right now*" (what every surface shows). plist = "will it still be
  disabled *after a reboot*" (what boot-reconcile and `doctor` care about, since the
  setting persists). Neither substitutes for the other.

**The off state is invisible to the casual observer [MEASURED].** Nothing in the default
`pmset` output ever hints at this setting, so a stray enabled state on someone's machine is
genuinely undiscoverable unless you know to look in `ioreg` or the plist. That is the
strongest argument for the `doctor` check in this change — without it, the failure mode is
silent by construction.

## 4. Precedent for the authorization path

**Amphetamine** (Mac App Store, the best-known tool in this niche) implements
Closed-Display Mode with `pmset disablesleep`, escalating via
`do shell script "pmset disablesleep 1" with administrator privileges` **[SOURCED]** —
the same path the change recommends (option A/E in `design.md` §1). It is distributed
through the App Store, so it handles admin authentication in-app with no sudoers changes
and no Gatekeeper friction **[SOURCED]**.

That is a strong precedent: the mechanism plus the escalation path have survived years of
macOS releases in a widely-installed, Apple-reviewed app.

**Counter-pressure to record honestly.** Apple's own position on that escalation path is
lukewarm: `do shell script … with administrator privileges` is regarded as equivalent to
`sudo` and intended for administrators rather than as an app API, and the underlying
`AuthorizationExecuteWithPrivileges` has been formally deprecated for years while
continuing to work — "should not be used in a widely distributed product" **[SOURCED]**.
The modern replacement, `SMAppService` (macOS 13+), hosts a daemon **inside the app
bundle**, so it is removed when the `.app` is deleted **[SOURCED]**.

This sharpens — but does not reverse — the change's rejection of the privileged-helper
route:

- `SMAppService` is *bundle-scoped by construction*. gtmux's CLI ships via Homebrew and a
  tarball with no `.app` at all, so a helper route would give app users the feature and
  brew users nothing. That split is the disqualifier, and it is now confirmed rather than
  assumed.
- It would also introduce the persistent root RPC the design's core asymmetry forbids: an
  interface that exists to be *asked to escalate*, reachable while nobody is at the machine.
- Mitigation for the deprecation risk is a fallback ladder, not a rewrite: if the AppleScript
  path is ever closed, fall back to the narrow `sudoers` drop-in (`design.md` §1 option C) or
  to guiding the user through one manual `sudo`. Both keep the asymmetry; neither needs a
  helper.

## 5. Apple Silicon: power transitions are the fragile moment

This is the finding that changed the design rather than just confirming it.

- Before Amphetamine 5.3, Closed-Display Mode on Apple Silicon **failed when the Mac was
  connected to or disconnected from external power** (charger, or a display with power
  delivery) **[SOURCED]**.
- Even after the fix, the vendor still warns Closed-Display Mode "may not work as expected"
  across a power-source change on Apple Silicon laptops **[SOURCED]**.
- Amphetamine 5.2 added a user-facing alert for *failed* closed-display sessions, plus
  optional auto-termination **[SOURCED]** — i.e. the incumbent concluded that the right
  answer to this class of failure is **to tell the user**, not to pretend it worked.
- And on the unplug direction specifically, the vendor states plainly that OS-level
  limitations mean it **cannot** keep the Mac awake in clamshell after the power cord is
  pulled **[SOURCED]**.

### 5.1 Measured 2026-07-31: the unplug claim does NOT hold here

The vendor documentation above says OS limitations prevent closed-display operation once
the adapter is pulled. **On macOS 26.5.2 / M4 Pro that is false**, and the test was run
precisely because a user requirement depended on it (carrying a closed laptop between
meeting rooms on battery):

| signal | result |
|---|---|
| server mode armed at start | **yes** — recorded by the harness, not assumed |
| samples taken on battery | **42 of 45** — the adapter was out for most of the run |
| kernel Sleep/Wakes | **60 → 60** — zero sleeps |
| recorder gap | **6 s** — none |
| `/api/health` | **45/45 × 200** — served throughout |

Unplugged, lid shut, walking: it kept working. Re-plugging afterwards did not lapse it
either, so **both directions of the power round trip passed**. This is consistent with A2
(the setting is stored globally, not per power profile) and inconsistent with the vendor's
note — which may describe an older OS, or the external-display clamshell mode rather than
the `disablesleep` path. Prefer the measurement; the citation stays so the disagreement is
visible rather than quietly dropped. `[MEASURED]`

Three consequences, all now folded into the change:

1. **"Unplugged ⇒ end the session" is REJECTED as a policy.** It is neither a system
   behaviour (§5.1 — the machine did NOT sleep when the adapter was pulled) nor a desirable
   one: unplugging is a normal part of the core use case. The guardrail is re-keyed to
   **remaining charge** — warn at a threshold, restore sleep at a floor — which is what
   actually predicts harm. See design §2.3.
2. **Re-plugging is a test case, not just unplugging.** Verifying "unplug → sleeps" is not
   enough; the plug-back-in path must be verified too, because that is the documented
   Apple-Silicon breakage. Added to the manual checklist (`tasks.md` 0.2b).
3. **A failed/lapsed state must be visible.** The incumbent needed an alert for this; so do
   we. The design's persistent indicator with an attention colour, and the exit
   announcement carrying a machine-readable reason, cover it — provided the indicator can
   also express "this silently stopped working", not only "on" and "off".

## 6. Competitive picture — nobody in this space has solved it

Remote-control-your-agent tools do not address the lid at all **[SOURCED]**:

- **VibeTunnel** exposes agent terminals via a local web UI — a *reachability* answer that
  assumes the machine stays awake.
- **Happy** is a mobile client for Claude Code / Codex — same assumption.
- **Claude Code's own Remote Control** continues local sessions from another device — same
  assumption.
- The community workarounds that circulate for "keep my Mac awake while Claude Code runs"
  are `caffeinate dims claude` (idle path — does not survive the lid, §2) or manually
  setting the Energy-Saver timer to Never and remembering to undo it.
- The tools that *do* solve the lid (Amphetamine, KeepingYouAwake, Caffeinated, and
  clamshell-focused utilities) are generic keep-awake apps with no notion of agents,
  sessions, or remote command.

So the gap is real and specific: **the remote-agent tools assume an awake Mac; the
keep-awake tools don't know what an agent is.** gtmux is the only product positioned to tie
"keep the machine serving" to "there are agents mid-turn and a phone on the other end" —
including the part the generic tools structurally cannot do: ending the state when the work
is done, and telling the operator on their phone that it ended.

Also worth noting for the safety copy: independent sources warn that a fanless MacBook Air
dissipates heat through the keyboard area, so extended closed-lid load can cost up to ~50%
sustained performance **[SOURCED]**. That is a stronger statement than the design's generic
"a closed lid dissipates heat worse", and argues for naming the Air case specifically in
the authorization card.

## 7. Open items and risks

**Closed — measured 2026-07-31 on the M4 Pro / macOS 26.5.2, no external display**

- **A1 — does a closed lid actually keep serving? YES.** With `disablesleep` on: kernel
  Sleep/Wake 58 → 58 (no sleep), recorder gap 6 s, `/api/health` 50/50 × 200 over ~4 min
  with the lid shut. Control (0.0, same machine, assertion only): slept after 133 s. **The
  feature's blocking premise holds; implementation may proceed as designed.**
- **A2 — is `-c` scoping honoured? NO.** The write succeeds but lands in
  `SystemPowerSettings` (global), not under `AC Power`. ⇒ G3 deleted, G4 (guard power poll)
  promoted to sole defence, G1 (refuse on battery) matters more. See design §1/§2.
- **A3 — readback: `ioreg` only.** See §3.1 for the full table and the two traps.

- **A2b — does it survive on battery? YES [MEASURED 2026-07-31].** Armed, adapter out for
  42 of 45 samples, lid shut: zero sleeps, no gap, 45/45 × 200; re-plugging did not lapse
  it. ⇒ **The between-rooms use case is deliverable.** Guardrails re-keyed from
  "unplugged ⇒ exit" to a charge floor (§5.1, design §2.3). Contradicts the vendor
  documentation in §5 — measurement wins, citation retained.

**Still open**

- **G13 (lapsed-state detection) keeps its place but loses its original justification.**
  It was motivated by the Apple-Silicon power-transition breakage, which did NOT reproduce
  here. Retain it — cheap, and the setting can still be changed by an MDM profile, an OS
  update, or another tool — but stop describing it as a known-live failure path.
- **A4 — Touch ID** for the admin dialog. UX only.
- **R5 (new) — `disablesleep` is undocumented (§3)** and the AppleScript escalation path is
  deprecated-but-working (§4). Mitigation: the fallback ladder in §4, plus `doctor` telling
  the user the truth if the mechanism ever stops taking effect (the readback discipline
  makes that detectable rather than silent).
- **R6 (new) — reboot without a login session.** `gtmux serve` runs as a **LaunchAgent**
  (per-user), so after a reboot it only starts once a user session exists. With FileVault on
  and no auto-login, nobody logs in until someone opens the lid — so the heartbeat never
  resumes, and the guard correctly restores sleep. **This is the right fail-safe** (an
  unreachable machine should sleep) but it means *server mode does not survive a reboot on a
  FileVault-locked Mac*, and the docs must say so instead of implying always-on. gtmux must
  not "fix" this by touching FileVault or auto-login — an explicit non-goal.

## 8. Sources

- [Why caffeinate does not work with the lid closed — clamshell.dev](https://clamshell.dev/guides/why-caffeinate-does-not-work-lid-closed)
- [Amphetamine & Closed-Display Mode — Toothpicks Support](https://iffy.freshdesk.com/support/solutions/articles/48001077199-amphetamine-closed-display-mode)
- [About Failed Closed-Display Mode Sessions — Toothpicks Support](https://iffy.freshdesk.com/support/solutions/articles/48001180528-about-failed-closed-display-mode-sessions)
- [Amphetamine: keep your MacBook awake in clamshell mode — TechPP](https://techpp.com/2021/06/18/macbook-clamshell-mode-keep-awake-amphetamine/)
- [How to use a MacBook with the lid closed — Macworld](https://www.macworld.com/article/673295/how-to-use-macbook-with-lid-closed-stop-closed-mac-sleeping.html)
- [How to keep your Mac awake, even when the lid is closed — 9to5Mac](https://9to5mac.com/2026/06/12/how-to-keep-your-mac-awake-even-when-your-macbook-lid-is-closed/)
- [How to Keep Your MacBook Running 24/7 for AI Agents (Even With the Lid Closed) — BlitzMetrics](https://blitzmetrics.com/how-to-keep-your-macbook-running-24-7-for-ai-agents-even-with-the-lid-closed/)
- [Running VoiceMode with MacBook Lid Closed](https://glama.ai/mcp/servers/@mbailey/voicemode/blob/61135913f46bd6d8612aa7d401c5c051e87b84e1/docs/guides/macbook-portable.md)
- [Keep Your Mac Awake While Claude Code Runs Locally — andrewbaker.ninja](https://andrewbaker.ninja/2026/04/11/keep-your-mac-awake-while-claude-code-works/)
- [Using VibeTunnel to control Claude Code instances remotely](https://www.andreagrandi.it/posts/using-vibetunnel-to-control-claude-code-instances-remotely/)
- [Happy — Remote Control for Claude Code & Codex](https://happy.engineering/)
- [Claude Code Remote Control — official docs](https://code.claude.com/docs/en/remote-control)
- [pmset reference — ss64](https://ss64.com/mac/pmset.html)
- [Power Management in detail: using pmset — The Eclectic Light Company](https://eclecticlight.co/2017/01/20/power-management-in-detail-using-pmset/)
- [pmset — disablesleep, Taylor Price](https://drpebcak.svbtle.com/pmset-disablesleep)
- [One-time privilege escalation — Apple Developer Forums](https://forums.developer.apple.com/forums/thread/768765)
- [How to perform actions as root from GUI apps on macOS? — Apple Developer Forums](https://developer.apple.com/forums/thread/773025)
- [Demystifying root on macOS, Part 3 — scriptingosx](https://scriptingosx.com/2018/04/demystifying-root-on-macos-part-3-root-and-scripting/)
- Local, this machine: `man pmset`, `man caffeinate`, `pmset -g` / `-g custom` / `-g ps` /
  `-g therm` / `-g assertions`, `sw_vers`, `csrutil status`, the non-root `disablesleep`
  probe, `/Library/Preferences/com.apple.PowerManagement.plist`.
