# Troubleshooting & footguns (living checklist)

## Working Codex pane becomes idle after another client's SessionEnd

**Evidence (2026-10-10).** `%19` started release work at event 71850 and continued through compaction (71856/71857). Event 71859 was `SessionEnd` for a different session, yet named `%19`; no matching Stop ended the release turn. The phone showed idle while the TUI still showed Working.

**Root cause.** The end-event identity guard ran only when `native.Load(endingSession)` succeeded. An unregistered client ending with an inherited or same-directory pane bypassed that guard and cleared the current turn. The fallback then depended on frame/CPU activity and could show idle during a quiet interval.

**Fix/check.** Every identified Codex end must agree with the candidate pane's saved session binding, regardless of native registration. Mismatches remain pane-less and write `codex.session_end.pane_mismatch`; lifecycle records are retained and native cleanup still uses the actual ending ID. `TestCodexSessionEndIgnoresInheritedPane` covers registered/unregistered, missing/same cwd and valid ownership. Do not repair this by forcing a working marker or guessing from the title.

Pitfalls we've actually hit during **development, debugging, and release** — with
the check that would have caught each one early. This is a **living document**:
when a new footgun costs real time, add an entry here (symptom → root cause → the
must-check / rule), so the next person (or Claude) trips a checklist instead of the
rake. Keep entries short and action-first.

> Related runbooks live next to their subsystem: remote-access / pairing debug in
> `docs/design/remote-access-tunnel.md`; deploy paths in `CLAUDE.md` (Deploy table).

---

## Two terminals display the same Codex conversation after restore (2026-10-09)

**Evidence:** `%18` in `check-hq-stop` was launched as `codex resume <dev-thread>`;
`%19` in `gtmux dev` had that same conversation binding. Both displayed the latest
user instruction and tool output. The 2026-10-08 restore trace explicitly recorded
`resume[cwd-fallback] ... ran=true AMBIGUOUS(2 cwd+position candidates)`.

**Cause:** fallback selected the newest same-directory/position record even when
there were several candidates. Its dedup set only covered commands sent during
that restore: conversations in already-running panes were not reserved. A record
whose original locator was still present was incorrectly treated as rename evidence.

**Rule:** reserve IDs in running panes; exclude fallback records whose original
locator remains in the saved or live topology; require one distinct remaining
conversation. A rejected fallback leaves the shell untouched and prints a skip
receipt, with candidate count in diagnostics. The preview uses the same saved-layout
ownership rules. A preview is a saved-workspace plan; already-running panes are
additionally protected at execution time.

**Check:** compare live command arguments, the per-location resume record and the
`restore.resume` trace before blaming title rendering. Do not deduplicate separate
live clients out of the radar or stop either client automatically. Existing shared
clients require an explicit decision about which terminal should retain the conversation.

---

## Spawned Codex worker appears twice and has no Chat (2026-10-03)

**Symptom:** one HQ-spawned worker appeared as both a tmux pane and a native
Codex row. Its rollout had nearly 1 MB of history, but the live tmux location had
no resume record; `/api/transcript` returned an empty array.

**Cause:** two Codex panes shared a cwd. The shared app-server's inherited pane
was not trustworthy, so the hook correctly refused cwd-only attribution. Nothing
provided independent evidence for the new worker's first binding. Native dedup
and Chat both depend on that binding, so they failed together.

**Rule:** fresh interactive Codex spawn carries a short-lived, exact-payload
delivery witness scoped to the live pane ID/location/shell PID/Codex client PID/cwd and incumbent.
Only a terminal-originator session with that submitted user message may bind.
`pane_pid` can name the shell, so verify the unique actual Codex client beneath
that root too; restarting Codex in the same shell must invalidate an old intent.
See `docs/design/CODEX.md` for expiry, bounded recovery and diagnostics. Do not
solve the bootstrap gap by trusting inherited `TMUX_PANE` or choosing the newest
same-directory log. Assistant/tool echoes and desktop rollouts are not evidence.

**Existing-worker repair:** the reported pane had four phone send fingerprints,
byte counts and timestamps uniquely matching one of 13 contemporaneous rollouts.
Its live locator/root PID/command/cwd were unchanged; the actual Codex client
started one second before that rollout. A missing resume record was
created exclusively, without overwriting another record or touching the worker.
The native duplicate disappeared and the authenticated Chat API returned seven
turns. This was a verified repair, not an automatic rule for old unbound workers.

---

## A Codex quota banner briefly shows an idle session as running, then done (2026-09-28)

**Confirmed:** `%17` wrote `task_complete` at 16:44:40 and no turn event followed;
`finished/%17` still read 16:44. The yellow weekly-limit banner only repainted the Codex
TUI; it did not start a task. The old radar treated any screen change as a brief
`working`, and when the screen went quiet and the status returned to `idle`, the
notification layer could push that false edge as a completion.

**Rule:** for a Codex pane that is bound to a session and has no current-turn or wait
marker, if that session's latest rollout boundary is still `task_complete`, a screen
repaint alone cannot start a new working state. A new `task_started`, a real hook marker,
and panes with no reliable binding keep the existing logic. If it happens again, check
`gtmux events --all --json`, the pane's markers and the last boundary in its **own**
rollout first.

---

## A Codex native session's end event lands on a tmux pane in the same repo (2026-09-28)

**Confirmed:** the session 「调查 SpringBoard 崩溃问题」 ("investigate the SpringBoard
crash") wrote `task_complete` at 16:43:20, but its native record still said `working` as
of 16:43:07. The `SessionEnd` at 16:47:31 carried this session's ID but was filed against
`%19`, which earlier that day had been bound to a different session. So the old code never
cleared the native record, and could also clear `%19`'s own running marker.

**Attribution path:** the hook reads `TMUX_PANE` first; without it, it also walks the
process ancestry. Codex's cwd-carrying path tries to match a pane in the same directory:
even when the payload's session ID differed from the pane's bound ID, the old logic could
pick `%19` as long as it was the only candidate in that directory. That hook's original
environment and cwd were not kept, so which step produced `%19` cannot be determined. The
shared app-server's environment currently says `%16`, which is no basis for claiming the
`%19` came from it at the time either.

**Rule:** a Codex `SessionEnd` for a session that already has a native record may act on a
pane only when that pane is bound to the same session ID; with or without a cwd, the native
record is cleared by session ID. When a wrong candidate is rejected, the diagnostic log
records the environment pane, the candidate pane, the bound session and whether a cwd was
present, so the next occurrence can confirm the source.

---

## Codex has finished but still shows working (2026-09-28)

**Symptom:** the top of the phone's session view still shows `working` after Codex has
printed `Worked for …` and its ready input box. Read the pane ID at the top first; a
neighboring pane button in a screenshot may show a different ID.

**Root cause:** this time `%16`'s `UserPromptSubmit` carried no session ID and wrote an
empty running marker; the `Stop` at 14:57:46 had no verifiable pane attribution either.
The old closing logic accepted only a marker that recorded a session ID, so it could not
use the `task_complete` in the same session's log to clear `working` in time.

**Diagnosis:** compare `gtmux events --all --json`, `gtmux agents --json`, the pane's
screen and the `task_started` / `task_complete` times in that Codex session's log. Close a
turn only with the current pane's bound session, a matching cwd and a completion record
later than the running marker; never clear another session on the strength of a
`TMUX_PANE` inherited from the shared app-server alone.

---

## An idle Codex input box shows up under `Your call` (2026-09-28)

**Symptom:** the phone lists under `Your call` a session that had finished answering and
was sitting at `Ask Codex to do anything`; a new `Waiting(permission)` in the event stream
points at that old pane, while the approval prompt was actually HQ's.

**Root cause:** the approval hook sent by Codex's shared app-server carried no cwd or
session ID, but inherited the first client's `TMUX_PANE`. Writing the waiting marker
straight from that variable filed someone else's approval against the old session.

**Diagnosis:** compare `gtmux events --since 20m --all --json`, `gtmux agents --json` and
both panes' current screens; check whether the event's `agent_session` and cwd can
establish attribution. A hook with no attribution is placed only through a unique session
binding; otherwise it stays pane-less and the radar senses the approval from the pane that
actually shows the approval menu. Never guess. A marker already written to the wrong pane
is cleared when Codex's ready input box and a quiet running state appear together.

---

## General: `| tail` swallows the exit code, so a failure looks like success

**Symptom:** a command failed and you believe it succeeded, because what you saw was its
**last few lines**, and the tail is often harmless wrap-up output. Worse,
`cmd | tail && echo success` **still prints "success"**.

**Root cause:** a pipeline's exit code is the exit code of its **last command**. `tail`
returns 0 as long as it read input, however badly the command before it blew up. The same
goes for `| head`, `| grep` and `| cut`.

**It bit twice on the same day, in completely different settings:**

- `fastlane release | tail -40`: the upload had actually **succeeded**, but the output
  ended with an unrelated Ruby error. I saw only those 40 lines and concluded "the upload
  failed", then queried ASC (a new build takes minutes to appear in the list), which
  "confirmed" the wrong call. The truth only came out when a re-upload made altool report
  *Redundant Binary Upload*.
- `git rebase main | tail -2 && echo "rebased"`: the rebase **never ran** because the
  working tree had conflicts, but "rebased" was printed anyway, and I took the next step
  on that basis.

**Rules:**

- **When success matters, never pipe the command into `| tail`/`| head`.** Redirect to a
  file, then grep: `cmd > /tmp/x.log 2>&1; echo "exit=$?"; grep -E '…' /tmp/x.log`
- If you must pipe, `set -o pipefail` first, or read `${PIPESTATUS[0]}`.
- **`&& echo success` is not verification.** It only proves the last stage of the pipeline
  didn't crash. To verify, look for evidence that **the action actually happened** (the
  file changed, the process is there, the endpoint answered), not that the command "seems"
  to have finished.

**The bigger lesson from the same root: a correct judgment ≠ the action done.** Probes lie
too. The same day I judged whether the menu-bar app was polling by "counting child
processes" and concluded "not at all"; when I later checked that probe against a control
that **definitely made a call every 1.5 seconds**, the control also showed "none".
**Before drawing a conclusion, validate your instrument against a control with a known
result.**

---

## A long Chinese sentence gets truncated on send: the space a line wrap inserts breaks the fingerprint match (2026-08-03)

**Symptom:** send a **fairly long Chinese** message from the phone and the app shows "Not
sent — the input box didn't confirm the full message", yet the agent **received a
truncated, duplicated** chunk (the beginning twice, the tail missing). Short messages and
long English-only messages work; only a **long single line of Chinese with no spaces**
triggers it. Earlier attempts never reproduced it (ASCII tests up to 37K characters were
fine).

**Root cause:** dispatch's paste confirmation, `draftHasDelivery`, requires **both** the
body's head fingerprint (first 40 runes) and tail fingerprint (last 40 runes) to appear in
the draft. A line of **Chinese with no spaces** **wraps** in the composer, and
`normalizeSpace` turns the wrap's `\n` into a **space**. So the draft holds "我们 正在"
while the tail fingerprint is "我们正在" (no space; the text means "we are"), and a tail
that straddles the wrap point can never match. Confirmation fails → the compensating
**clear-and-re-paste** logic fires → it clears a good draft and re-pastes on top of it →
truncation + duplication. English has spaces, so it wraps **at word boundaries** and the
tail stays contiguous on the next line, which is why English is not affected; Chinese has
no break points, so a single line wraps in the middle of the text.

**Fix (internal dispatch, `internal/dispatch/deliver.go`):** when the plain head+tail match
fails, run one more head+tail match with **all whitespace removed** (`containsSpaceless`).
The spaces the wrap inserted are erased and a fingerprint that straddles a wrap matches
again, the same whitespace-free trick the image path already used. A 40-rune fingerprint
that reappears intact once whitespace is removed is not a false match.
Regression test: `TestPasteAndSubmit_WrappedCJKLine_ConfirmsNotChurns`.

**MUST-CHECK:** verify any change to send/paste confirmation logic with a **long single
line of Chinese with no spaces** (not with English alone).

---

## The menu bar can't switch to Anywhere: a GUI process's PATH has no Homebrew prefix

**Symptom** — in the menu bar's preferences you click "Anywhere", the
confirmation dialog appears, you click Enable, and **the dialog simply vanishes, the
switch snaps back and nothing shows on screen**. The same command run in a terminal
(`gtmux tunnel --service --yes`) **succeeds completely**.

**Root cause (two of them; neither alone explains it)**

1. **A GUI process's PATH is not your PATH.** An app launched from Finder/LaunchServices
   inherits launchd's `PATH=/usr/bin:/bin:/usr/sbin:/sbin`, and **neither Homebrew prefix
   is on it**. `cloudflared` lives in `/usr/local/bin`, so `exec.LookPath` reports it as
   not installed, and the CLI goes on to say Homebrew isn't installed to fetch it either.
   Both statements are false; both tools were there all along. `internal/tmux` hit this
   long ago and hardcoded a fallback path for tmux, but cloudflared / brew never got the
   same lesson.
2. **The failure was swallowed.** `RemoteAccess.run()` always had a `lastError`, and the
   pairing panel always displayed it, **but the preferences row never rendered it**. So the
   failure looked like "the dialog vanished and nothing happened".

**Reproduce (no need to actually break anything)**
```sh
env -i HOME="$HOME" PATH="/usr/bin:/bin:/usr/sbin:/sbin" gtmux tunnel --service --yes
# → cloudflared isn't installed … Homebrew isn't installed to fetch it
```

**Must-check**
- When debugging anything that "fails in the app but works in a terminal", **reproduce it
  with the `env -i` line above first**. That step separates this class of bug from the
  rest; skip it and you will spend a long time looking in the wrong direction (network?
  token? permissions?).
- When gtmux gains a new tool to shell out to, use `toolpath.Look()` (`internal/toolpath`),
  not `exec.LookPath`. It originally lived in `internal/app`, and the next tool that needed
  it (`gh`, in the `internal/dispatch` leaf) **couldn't reach it**, so the same lesson was
  learned a second time, at the cost of `gtmux reap` reporting a merged branch as unmerged
  (see the reap entry below). **A shared lesson has to sit where every layer can reach it.**
- When adding a control that can fail, render its error **on the screen where that control
  lives**. Don't count on the user finding the reason in some other panel.


## `gtmux update` installs the x86 build on Apple Silicon (and keeps doing it)

**Symptom** — on an M-series Mac, `gtmux update` prints `[1/5] Host darwin-amd64` and
`~/.local/bin/gtmux` ends up a pure x86 binary. After that **every further update is amd64
too**, and `file` shows x86_64.

**Root cause** — `install.sh` decided the architecture with `uname -m`, and **`uname -m`
reports the architecture of the current process, not of the machine**. Under Rosetta on
Apple Silicon it returns `x86_64`. So:

- run the installer from a translated shell (`sysctl -n sysctl.proc_translated` = 1) → you
  get the amd64 package;
- **the installed x86 gtmux itself runs translated**, so when it runs `gtmux update` it sees
  x86_64 again → **a closed loop with no way out**. That is why one wrong install means
  every install after it is wrong too.

**The test** — `sysctl -n sysctl.proc_translated` returning `1` means "you are being
translated; the hardware is arm64". `install.sh` now corrects `uname -m` on that basis.

**Must-check**
- When you suspect an architecture problem, **don't trust `uname -m`**; run
  `sysctl -n sysctl.proc_translated` first.
- `file -b ~/.local/bin/gtmux` should say `arm64` (or universal), never pure `x86_64`.

## Install layout: which gtmux is authoritative

**The authoritative CLI is `~/.local/bin/gtmux`**, a **real binary** (not a symlink).
`install.sh` / `gtmux update` replace it atomically in place (`mv -f`), so **making it a
symlink is pointless: the next update overwrites the symlink with a file**.

- `~/Applications/Gtmux.app/Contents/MacOS/gtmux` is the app's **own private copy**, tied to
  the app's version. The two LaunchAgents (serve / selftunnel) point at it by **absolute
  path**, so cleaning up copies on PATH does not touch the services. **Don't** symlink
  anything to it: the app can be replaced or deleted.
- `/usr/local/bin/gtmux` **should not exist**. That is Homebrew cask territory (left over
  from the early 0.9.3 cask).
- The `bind g/a/J` lines in `~/.tmux.conf` hardcode `~/.local/bin/gtmux`, which is
  **correct**, because that is exactly the authoritative path.


## restore: four kinds of symptom, one shared cause (no executable contract)

In one day restore showed four kinds of symptom (a lost session / the same session lost
twice in a row / a changed pane layout / terminal window order), and each time only the one
somebody noticed got fixed. **These are not four independent bugs; they are the signature
of a subsystem with no executable contract.**

**Run the contract** (SKIPs by default and touches nothing of yours):
```sh
GTMUX_RESTORE_E2E=1 go test ./internal/app/ -run TestRestore -timeout 12m
```

It runs save → kill-server → restore → a per-dimension assertion inside a **private tmux
server** (`TMUX_TMPDIR`) + a **private HOME** (the resurrect save directory). **It does not
mock tmux**: the failures it exists to catch live precisely in the interaction of gtmux ×
resurrect × a real server, and a mock would delete them.

**What it caught on its first run**
- **The active window/pane had never been restored**, and **none of the four reported
  symptoms was this one**. resurrect places the active window with `tmux switch-client`,
  which **does nothing and reports no error when no client is attached**, and gtmux drives
  restore headless. Fix: replay it ourselves with `select-window`/`select-pane` (see
  `restoreactive.go`).
- **Recovering missing sessions was blocked by an inverted condition**: `shouldRecover`
  recovered only when **every** saved session was absent. After a reboot some terminal tab
  always brings up a session on its own → the condition fails → **the other sessions never
  come back**, and the next autosave records the fact that they don't exist.

**What it did not catch, stated just as plainly**
- **Pane layout is not lost on the clean path.** The first version of the test went red,
  and **my comparison was wrong**: restored panes are new panes, so of course the pane ids
  in the layout string change (`...,0,0,7` → `...,0,0,8`) while the geometry is identical.
  The comparison must strip the **checksum** and **the pane id at the end of each leaf**. I
  nearly reported it as a bug.
- **Terminal window order cannot be verified here** (it needs a real terminal + the
  accessibility tree). It is listed in the contract, marked "manual check".

**Must-check**
- Changed any restore-related code? Run the command above.
- When adding a restore behavior, **add an assertion dimension to the contract first**,
  then write the implementation.
- When asserting on a tmux layout string, **always normalize the pane ids first**, or you
  are testing pane numbering, not layout.


## restore injects `claude --resume` into a pane that never ran an agent (2026-08-04)

**Symptom**: after a reboot + restore, `{ cd -- '…'; } && claude --resume '<uuid>'` appears
out of nowhere in panes that had always been plain shells, stopped at the trust gate. One
reboot took the fleet from **10 agent panes to 16**; the extra sessions' goals were all
days-old business, and one of them carried 33.7M tokens, which promptly triggered
`usage·warn burn`. **The same pane gets hit again across several reboots.**

**Root cause**: the resume record is written by the agent's hook and **never cleaned up**;
all it can prove is "this locator ran an agent at some point". restore treated it as "an
agent was running here at the time", so any pane that had ever run an agent became a
permanent injection target. And it **perpetuates itself**: the injected session rewrites
that record (and the next autosave writes it into `pane_full_command`), so the "evidence"
is stronger at the next reboot. No heuristic based on how old the record is can stop it,
because the record's timestamp is exactly what the previous injection refreshed.

**Fix (v0.45.x, `internal/app/restoresave.go`)**: the test is now the
`pane_current_command` / `pane_full_command` on that pane's line in the tmux-resurrect
save. The save is a snapshot from minutes before the reboot and the **only witness still
alive** (the processes are long gone). A pane the save says was a shell is never touched;
only "can't tell" (the save didn't record enough) is let through, and logged (better to
restore one too many than lose a session that was really running).

**⚠️ Must-check: a tmux-resurrect save has two field layouts (an empty pane title shifts the rest left by one)**
resurrect's `save.sh` reads back the lines it dumped with `while IFS=$d read …`, using
**TAB** as the delimiter, and tab is an IFS **whitespace** character, so bash **collapses
consecutive whitespace delimiters into one**. So **the line for a pane with an empty title
loses a field, and every field after it shifts left by one**: the "command" read at the
fixed column is really **the pane's pid**, and the trailing full command is garbage that
resurrect computed from the wrong pid. In the incident, 4 of the 6 phantom sessions were on
such lines (including the reported one): read by fixed column, the command for
`日常更新:0.0` (a session named "daily updates") is `77304`, not a shell, so it was let
through anyway. **Without handling this shift, the bug cannot be fixed.** How to tell: the
directory field carries a `:` prefix in the format, and sits at index 7 normally, index 6
after the shift; a shifted line's full command must be discarded. (The same shift also
leaves resurrect **unable to restore these panes' directories**; they come back at `/`.
That is upstream behavior.)

**Verify without rebooting**:
```sh
mkdir -p /tmp/probe/tmux/resurrect && cd /tmp/probe/tmux/resurrect
cp ~/.local/share/tmux/resurrect/tmux_resurrect_<stamp>.txt . && ln -sf tmux_resurrect_<stamp>.txt last
XDG_DATA_HOME=/tmp/probe gtmux restore --plan   # read-only; what it lists is what would be restored
# compare with the panes that were really running an agent in the save:
awk -F'\t' '/^pane/{ if (substr($8,1,1)==":") print $2":"$3"."$6" "$10" "$11; else print $2":"$3"."$6" "$9" (shifted)" }' \
  ~/.local/share/tmux/resurrect/tmux_resurrect_<stamp>.txt
```
End-to-end contract (real tmux + real resurrect, private server):
`GTMUX_RESTORE_E2E=1 go test ./internal/app/ -run TestRestoreResumesOnlyPanesThatWereRunningAnAgent`

**Zombie sessions that were already injected**: they really are running (and burning
quota); the gate only prevents new ones. `tmux kill-pane` them by hand or exit inside the
pane. The history records in `~/.local/share/gtmux/resume/` need no manual cleanup
(harmless once the gate is in).

## Three things after a reboot: a stale save, pane ids on the wrong panes, a broken layout nobody reported (2026-08-18)

One reboot exposed all three at once, and they **are three outlets of the same flaw**:
gtmux trusted what it was told (a line of config, a record, a name) instead of what it
could see for itself (a file's modification time, the panes alive right now, a window's
shape).

### ① The save sat untouched for hours, and the backstop save never fired once

**Symptom** — what comes back is the state from tens of minutes or even hours earlier;
every change made before shutdown is gone.
**Root cause** — the autosave command hangs off the **right side of the tmux status bar**,
so it runs only while a terminal is attached and the status bar is redrawing. Once the Mac
sleeps, nothing is saved, however correct the config. Measured over 3.5 days: **76 gaps,
the longest nearly 6 hours**, against a configured interval of 5 minutes. gtmux has had a
backstop for a long time, but its switch asked **whether that command is written in the
status bar** (`shouldBackstopSave(statusRight)`), and the problem is exactly that
**written ≠ running**, so the backstop never fired once.
**Fix** — the test is now **how long since the save file was last updated**
(`internal/app/resurrectsave.go`): untouched for 10 minutes → save it ourselves; with a
trigger in the status bar the limit widens to 20 minutes (let the autosave go first), and
right before actually saving it **waits a moment and checks again**, because both sides
starting a save the instant the Mac wakes is the only collision window left. The old lesson
against concurrent saves is kept, just expressed as evidence: **a saver that is running
keeps the file fresh, so the backstop never wakes up next to it**.
**Must-check** — the `resurrect autosave` row in `gtmux doctor` now reports **the save's
real age**. Seeing "armed, but idle 6h" is this
problem; don't take it as healthy just because it says armed.

### ② `%N` gets reissued, and a batch of state files lands on other panes

**Symptom** — after a reboot gtmux pins one session's goal on another session, and raises
a "stuck draft" alert for a pane that was never dispatched to.
**Root cause** — **a tmux pane id is a sequence number on the server, not an identity**.
When the server restarts it hands out ids from `%1` again, and old numbers go to other
panes. gtmux has a dozen-odd directories named by that number but cleaned only three.
Measured after the reboot: `enrolled` 50/52, `goal` 27/29, `sends` 31/32 and `hqwake`
93/103 were dead ids, the oldest from two weeks earlier, and live panes were wearing
exactly those ids.
**Fix** — `state.ReapDeadPaneState(live)`: when a restore completes (`afterRestore`) and
on the serve slow tick (every 5 minutes), it clears every pane-named record against the
current list of live panes. **Three safeguards**: it deletes only files whose names look
like pane ids, scans only the listed families, and **deletes nothing when the live set is
empty** (a failed `list-panes` read must mean "don't know", never "no panes"). `resume/`
(named by locator; restore reads it at exactly the moment every pane is gone) and `usage/`
(by conversation id) are **never touched**.
**Also** — a dispatch record that has gone undelivered for two weeks no longer drives
screen judgments (`dispatch.Task.StaleUndelivered`). The record itself stays and
`gtmux tasks` still tells the truth; it just no longer has a say about a reissued id.

### ③ The layout broke, and neither side said a word

**Symptom** — a window comes back with the wrong arrangement, and `restore.log` has nothing.
**Root cause** — gtmux only counted whether the session names came back and never looked at
what the windows looked like. On tmux-resurrect's side,
`restore_window_properties >/dev/null 2>&1` means that when `select-layout` fails
(typically `have 3 panes but need 2`: the restored window has one more pane than the save)
**the error is thrown away**, and the window stays in the default stacked arrangement. Both
sides are mute, so the only detector was a person noticing days later. It had already
broken this way twice (8/15, 8/18).
**Fix** — after a restore, compare each window's **pane count + arrangement** in the save
against reality; a mismatch is written to the log store (then `restore.log`, now read with
`gtmux logs --component restore`) and flagged in the terminal
(`internal/app/restorecheck.go`). Every restore also prints **the save's time and age**
("Restoring the layout saved at 09:57 (37m ago)"). The old staleness-alert threshold was 24 hours, while the save that actually
lost work was only 37 minutes old.

**⚠️ Normalize tmux layout strings before comparing them**: strip the leading 4-character
checksum + the pane number at the end of each leaf. Every pane number changes when the
server restarts, so without normalization **every window reports "changed"**. The
production `normalizeLayout` (`restorecheck.go`) and the end-to-end contract now share one
implementation, so the two can't drift.

**End-to-end check** (real tmux + real resurrect, private server, your sessions untouched):
```sh
GTMUX_RESTORE_E2E=1 go test ./internal/app/ -run TestRestoreContract -timeout 12m -v
```
The contract gains two dimensions: on a faithful restore the reconciliation must stay
**silent** (a check that cries wolf is worse than no check), and after an extra pane is
split into a window it must **name that window**.

## The release has no tag message (`{{ .TagBody }}` became the PR description)

**Symptom** — the tag clearly has a `user:` block (`git tag -l --format='%(contents:body)' vX`
shows it locally), but the GitHub Release body holds **that squash merge's commit body (that
is, the PR description)**, the `user:` block never appears, and so the "What changed"
section of `gtmux update` prints nothing.

**Root cause** — `actions/checkout` leaves the tag as a **lightweight ref**. On a lightweight
tag `%(contents:body)` (which is what GoReleaser's `{{ .TagBody }}` reads) **falls back to
the commit message**, and a squash merge's commit body is the PR description. Nothing in the
chain errors; the content is silently swapped.

**Fix** — add one step after checkout:
```yaml
- run: git fetch --force --tags
```

**Must-check**
- After a release, **confirm the release body has the `user:` block**; a green workflow is
  not enough.
- Any CI logic that depends on a tag message must run `git fetch --force --tags` first.


## iOS submission: the long chain of traps behind `fastlane release` failing to archive on this M4 (2026-07-24)

One `bundle exec fastlane release` hit five traps in a row before the build uploaded. All of
them stem from **the toolchain being Intel x86 (Rosetta)**: on this same machine `uname -m`
returns x86_64 under Rosetta, and ruby/cocoapods were both Intel builds.

**The chain (in the order we hit it)**
1. **`bundle` can't find bundler 4.0.8** — `/usr/bin/bundle` is the system ruby 2.6. The
   project's gems are in `vendor/bundle/ruby/4.0.0` and need ruby 4.x. Fix: use Homebrew
   ruby's bundle.
2. **gym's archive "fails instantly, with one line in the gym log"** — gym pipes xcodebuild
   into **xcpretty**, and under the new ruby xcpretty breaks xcodebuild's pipe with SIGPIPE
   and dies after one line. **Plain xcodebuild works.** Fix:
   `build_app(xcodebuild_formatter: "")` (xcpretty is deprecated). **This is what made every
   later real error visible.**
3. **`Signing … requires a development team`** — the release lane didn't pass
   `DEVELOPMENT_TEAM` to the archive (the reliable device build always did). Fix: add
   `DEVELOPMENT_TEAM=<TEAM> CODE_SIGN_STYLE=Automatic` to xcargs.
4. **`Build input file cannot be found … ReactCodegen/*-generated.mm`** — `clean: true` wipes
   `ios/build/generated` (where the RN New Architecture codegen writes), and the codegen
   script phase is not guaranteed to regenerate it before the compile that consumes it.
   Fix: `clean: false` + `pod install` (the device build never cleans either).
5. **`option '-authenticationKeyPath' may only be provided once`** — this version of gym
   applies `xcargs` to **both** the archive and the export, so adding `export_xcargs: auth`
   passes auth twice. Fix: drop `export_xcargs` and put auth only in `xcargs` (one copy
   reaches both).

**The real fix: switch to an arm ruby**
- `arch -arm64 /opt/homebrew/bin/brew install ruby` (arm 4.0.6).
- `.zshrc`: `export PATH="/opt/homebrew/opt/ruby/bin:/opt/homebrew/lib/ruby/gems/4.0.0/bin:$PATH"`
  (the ruby bin + the gem-exec bin); remove RVM and `/usr/local/opt/ruby`.
- `gem install cocoapods` (arm); `bundle install` (rebuilds native gems for arm);
  `Gemfile.lock`'s `BUNDLED WITH` follows the arm ruby's bundler (4.0.16).
- Remove RVM: `rvm implode` + `rm -rf ~/.rvm` + delete the rvm lines from your rc files.

**Must-check**
- When debugging an iOS archive error, **turn `xcodebuild_formatter` off first**, then read
  the log: once the formatter crashes, every real error is swallowed.
- `bundle`/`ruby`/`pod` must all be arm (`file $(which ruby) | grep arm64`).
- On a version's first submission, `fastlane metadata` used to hit fastlane's `No data` bug
  after uploading the text and before uploading the screenshots. Since #1535 the lane
  uploads text only: screenshots go through `asc-asset-library.rb place-screenshots`
  (`docs/appstore/submit.md` §4). The deliver fallback is
  `fastlane metadata skip_metadata:true deliver_screenshots:true` (see the Fastfile comment).


## "My Mac stopped sleeping": `disablesleep` is an invisible switch (2026-07-31)

**Symptom** — closing the lid doesn't put the Mac to sleep, the battery drains for no clear
reason, the machine gets hot in your bag. Nothing looks wrong anywhere in System Settings ›
Battery, and `pmset -g` looks normal too.

**Root cause** — `sudo pmset -a disablesleep 1` sets the kernel flag `SleepDisabled`, which:

1. **survives reboots** (it is written to `/Library/Preferences/com.apple.PowerManagement.plist`);
2. **is invisible in every `pmset` reporting command**: none of `-g` / `-g custom` /
   `-g live` shows it, whether it is on or off. So "no unusual output" proves nothing about
   it being off.

So any tool that set it and never restored it (or a command you ran by hand) leaves the Mac
**never sleeping, with nobody noticing**.

**Must-check (most trustworthy first)**

```sh
# ① live kernel state: the only authority, real-time, no root needed
ioreg -r -c IOPMrootDomain -d 1 -w0 | grep SleepDisabled     # = Yes means it will not sleep

# ② on-disk setting: answers "does it still apply after a reboot", but LAGS a change you just made
plutil -extract SystemPowerSettings.SleepDisabled raw -o - \
  /Library/Preferences/com.apple.PowerManagement.plist

# ③ restore
sudo pmset -a disablesleep 0
```

**Three details that bite**

- **Don't trust `pmset`'s exit code.** Run without root, it prints "must be run as root" and
  then **exits 0**. After writing, read it back with ① above, or you will believe it has
  been restored.
- **The off state is `false`, not "key absent".** A machine that was restored still has the
  key, with value false; only a machine that never set it lacks the key. Judging by whether
  the key exists gives the opposite answer.
- **This setting ignores `pmset -c` (AC power only)**: the value lands at the top level of
  the plist's `SystemPowerSettings`, is global, and applies on battery too. So don't count
  on the kernel sleeping again once you unplug.

`gtmux doctor` now reports this state (the row appears only on a machine that has actually
touched it), and `gtmux awake` shows both the live state and the on-disk value. **gtmux
rolls back only the setting it made itself**: a setting without a gtmux ownership stamp is
only reported, with the manual commands above; gtmux never changes it for you.

---

## `make check` green locally, CI's `test` red: the Go tests run on **Linux** (2026-07-31)

**Symptom** — `make check` passes in full on your Mac, but the PR's `test` job fails on an
assertion along the lines of "this machine should support X".

**Root cause** — gtmux is a macOS product, but CI's `test` job is `runs-on: ubuntu-latest`
(only `menu-bar app build` runs on macos-latest). Any test that **reads the real system**
(`ioreg`, `pmset`, `sw_vers`, `/Library/...`, `runtime.GOOS`) behaves completely
differently there than on your Mac, and you can never catch that locally.

**Must-check / how to write it**

- A test that reads the system either **branches on `runtime.GOOS`** or uses a pure
  function + fixture (preferred: split the parsing into a pure function and feed it strings
  of real output).
- When branching, **assert both directions** instead of calling `t.Skip` off macOS. CI's
  Linux environment is a free "unsupported platform" sample, exactly what proves your
  compatibility gate really **refuses** instead of limping on. The test for
  `servermode.Supported()` is written this way.
- To check before opening a PR: `GOOS=linux go vet ./...` catches compile-time problems;
  behavioral differences can only be prevented by writing tests the two ways above.

---

## A generated shell script's content must travel as base64 — another backtick incident (2026-07-31)

**Symptom** — in the menu bar you click "Turn on server mode?", enter
your password, and get "authorization declined — nothing was changed". But the user **did**
enter the password and did click OK.

**Root cause (two, compounding)** — the privileged payload is "a shell script that writes
out another shell script". The inner content was inserted into shell double quotes with
Go's `%q`:

1. **`%q` is Go syntax, not shell syntax.** It writes a newline as the two characters `\n`,
   and shell double quotes **do not interpret** `\n`, so the guard script it writes out
   becomes a single line full of literal `\n`, completely broken.
2. **The script being written had backticks in a comment** (`` `gtmux awake on` ``). Inside
   double quotes the shell **executes** whatever is between backticks. With `set -e`, the
   moment that line fails the whole install aborts.

So osascript returned non-zero, and the caller classified **every** non-zero result as
"authorization declined", sending the investigation in entirely the wrong direction.

**Rules**

- **Pass file content into a shell as base64, always**: `echo <base64> | base64 -d > file`.
  The base64 alphabet contains no shell-special characters, so this class of bug becomes
  structurally impossible. Never assemble shell with `%q` or hand-written quoting.
- **Tell "the user declined" apart from "the script failed"**: only osascript's `-128` /
  "User canceled" is a refusal; anything else is an execution failure, and the real error
  must be shown. Collapsing the two into one error sends people to look in the wrong place.
- **Test the round trip, not the quoting**: actually run the payload, then compare the file
  it wrote with the original byte for byte (`TestInstallPayloadReproducesTheGuardExactly`).
  No quoting that merely "looks right" gets past that.

This is a relative of the `--body "$(…)"` backtick footgun in CLAUDE.md, the same trap
stepped on again in a different setting. The test is the same: **whenever a piece of text
has to pass through a shell, ask whether it contains backticks, newlines or quotes; if it
does, base64 it.**

---

## Release / git-ops

### Never inline backtick-containing prose into a shell-substituted string
**Symptom:** `gh pr create` / `git commit` prints `foo: command not found`, the PR
body comes out mangled, and — worse — a random process (once a rogue `gtmux serve`)
is now running and squatting a port.
**Root cause:** backticks and `$(…)` written directly inside a **double-quoted**
shell string are command substitution. Prose placed there can execute its fenced
identifiers instead of passing them as text. Do not rely on a quoted heredoc
(`<<'EOF'`) wrapped inside `"$(…)"` to protect prose on macOS: the bundled
`/bin/bash` 3.2 misparses this form. An unpaired `)` in the body can close the
substitution early and cause backticks to execute; a single quote can produce an
unexpected-EOF error. zsh preserved the body literally in these same probes, but
that does not make the form portable. Command substitution also strips trailing
newlines. Write the body into a file and pass its path in every shell.
**Rules:**
- Write PR/issue/commit bodies to a **file**, then `gh pr create --body-file <path>`
  / `git commit -F <path>`. Never `--body "$(…)"` or `-m "$(…)"` on text with backticks.
- After any PR-create that warned or errored, inspect whether that invocation
  started an unintended process. Stop only a process you can attribute to it;
  a matching name alone does not identify a stray.

### GoReleaser deletes every apostrophe from the release notes (2026-08-09)
**Symptom:** `gtmux update` prints the release notes with apostrophes missing —
"gtmux HQs own pull", "a real non-tmux agents work" — while the tag message reads
correctly. Only `'` is affected: em dashes, quotes, `·` and `%` all survive.
**Root cause:** GoReleaser reads `{{ .TagBody }}` with
`git tag -l --format='%(contents:body)'`, passing those quotes as **literal characters**
(it execs git, no shell to strip them), then removes `'` from the output to undo them —
which takes the body's own apostrophes with it. The mangled text lands in the GitHub
release body, which is exactly what `gtmux update` / `gtmux whatsnew` read back.
**Fix (shipped):** `release.yml` REPAIRS the notes after GoReleaser runs — the true tag
body (shell quotes this time, so git gets a clean format string) + GoReleaser's generated
`## Changelog` section, written back with `gh release edit --notes-file`. Deliberately
`continue-on-error`: the header is already there and merely mangled, so a failed repair
degrades to the old behavior instead of failing a release whose assets already shipped.
**Must-check:** this is upstream behavior we work around, not something we control — if
the workflow step is ever removed or GoReleaser's header stops being `{{ .TagBody }}`,
apostrophes go missing again. To repair a release by hand:
`git tag -l --format='%(contents:body)' vX.Y.Z` → prepend to the `## Changelog` section →
`gh release edit vX.Y.Z --notes-file <file>`.

### A code change isn't shipped until the right delivery path runs
Four artifacts, **three** paths (git tag ≠ device build ≠ `wrangler deploy`). Editing
`relay-worker/` or `tunnel-worker/` and merging changes **nothing live** until you
redeploy the Worker. See the Deploy table in `CLAUDE.md` and
[[relay-redeploy-footgun]]. Quick check when push behaves oddly:
`cd relay-worker && npx wrangler deployments list` vs. `git log -1 -- relay-worker/`.

### Release tag gate
Tagging `vX.Y.Z` runs the **full `make check`** (not a weaker `go test`), then
goreleaser + the macOS app build. CI can't see the menu bar — smoke-test the app on
real macOS before trusting a tag.

### Menu-bar "click to update" loops — the app reinstalls its OWN version
**Symptom:** the popover shows `New version X — click to update`; clicking it "finishes"
(no error), the app relaunches, and the SAME banner reappears. The CLI + app both stay
on the old version. `~/…/T/gtmux-update.log` shows `Release v<OLD>` / `Installed gtmux
v<OLD>` even though Go logged `Updating <OLD> → <NEW>`. Running `gtmux update` **by hand
in a normal shell works** (installs `<NEW>`).
**Root cause:** `install.sh`'s `open -n "…/Gtmux.app"` used to launch the app with the
installer's env still set, **leaking `GTMUX_VERSION=<OLD>` into the long-lived app
process**. The in-menu update runs `gtmux update`, which inherits that pin; Go honors a
pre-set `GTMUX_VERSION` (`if !LookupEnv(...)`) instead of resolving the latest, so
install.sh reinstalls `<OLD>` — forever. A manual shell has no `GTMUX_VERSION`, so it
resolves `<NEW>` and works. (After a re-login the login LaunchAgent starts the app with
a clean env, which is why a reboot "fixes" it.)
**Fix:** `install.sh` now strips it (`env -u GTMUX_VERSION open -n …`) so the app never
inherits the pin, and `Updater.spawnDetachedUpdate` runs `env -u GTMUX_VERSION gtmux
update` as a belt. **Diagnose** with `ps eww <GtmuxBar-pid> | tr ' ' '\n' | grep GTMUX_`
— a `GTMUX_VERSION=` there is the smell. **Unstick a machine now:** `gtmux update` from
a plain terminal, or just click update twice (the first click relaunches with a clean
env via the fixed install.sh).

---

### `gtmux doctor --fix` / `gtmux update` hangs right after "menu-bar app launched"
**Symptom:** the app-install step finishes (`[5/5] Menu bar … ✓`, "menu-bar app launched",
the PATH hint all print), then the command NEVER returns to the prompt — no "Restarted
the remote serve" / "Done". The app IS installed and running; only the command is stuck.
**Root cause:** `runInstaller` ends with `restartServeAgents()`, which ran
`launchctl kickstart -k gui/<uid>/com.gtmux.serve` UNBOUNDED. On some Macs that
`kickstart -k` blocks indefinitely, freezing the synchronous `doctor --fix` / `update`
forever. install.sh itself already completed (its final line printed) — the hang is the
best-effort serve-restart, not the install.
**Fix:** every `launchctl` call in `restartServeAgents` is now hard-bounded by a 6s
timeout (`runBounded`); on timeout it skips the restart (the serve refreshes on next
login) instead of hanging. **Unstick a machine now:** press **Ctrl-C** — the app is
already installed; only the trailing restart stalled. (Needs a release to reach an old
`gtmux`.)

---

### HQ's startup briefing ends up as an unsubmitted draft in its input box
**Symptom:** `gtmux hq` in the HQ pane prints "restarting it in the window it already had",
then "the startup briefing was not delivered (failed)", and once the agent is up its input box holds
`» gtmux·startup │ …` unsent. Sometimes the same line appears two or three times
concatenated in the scrollback first. It looks like a delivery bug; nothing was ever
delivered.
**Root cause:** `gtmux hq` was running IN the pane it was handing to the agent (a revive of
the pane you are sitting in, or `--here`). gtmux holds that terminal, so:
① everything it typed was buffered by the tty and handed to the agent as stdin the moment
the shell started it, which is how the briefing became a draft;
② the ready gate read the pane's foreground command, saw `gtmux` (itself), treated that as
"the agent took the pane over", and the dead session's leftover composer row passed the
screen test — so it pasted into a terminal nobody was reading;
③ `--here` additionally refused with "has unsent text on its line", because the draft guard
read the `gtmux hq` command line the user had just submitted as someone's half-typed line.
**Fix:** the self-pane case types nothing. `gtmux hq` starts a detached watcher that waits
for the composer from outside and delivers the briefing through the normal verified path,
then REPLACES its own process with the agent (`exec`, so the shell does not stay as the
pane's foreground process group leader — tmux would report `bash` and the watcher would
wait forever). The ready gate no longer reads gtmux itself as the agent, and the draft
guard is skipped on our own pane. `gtmux logs --event act.hq.brief` records each outcome.
**Must-check:** any new path that starts an agent in a pane must ask whether that pane is
`$TMUX_PANE` first; on our own pane, typing and screen-reading both lie.

---

### After `gtmux update`, the Direct tunnel still runs the old version
**Symptom:** `gtmux update` finishes and says it restarted serve, but `ps -eo lstart,command |
grep tunnel-client` shows the Direct client started before the update, and anything the new
client should do is missing. On v1.0.34 that was `status/tunnel.json`: doctor and the menu
bar had no tunnel state to read.
**Root cause:** `restartServeAgents` kickstarted only `com.gtmux.serve`. Its comment said the
tunnel agent "reconnects on its own", which was true while the only tunnel agent was
cloudflared. The Direct client (`com.gtmux.selftunnel`) runs `gtmux tunnel-client`, the same
binary the update replaced, so it kept the old code until the next login.
**Fix:** the update restarts every loaded agent that runs the gtmux binary, serve and the
Direct client (`restartAgents`, with a test). **Unstick a machine now:** `launchctl kickstart
-k gui/$(id -u)/com.gtmux.selftunnel`. That restarts the one client launchd owns; never start
a second `gtmux tunnel-client` by hand, it fights the live one for the reverse port.
**Must-check:** a new LaunchAgent that runs `gtmux …` belongs in `restartAgents`' list.

---

### `brew upgrade --cask gtmux-app` fails: "App source '/Applications/Gtmux.app' is not there"
**Symptom:** `brew install/upgrade --cask chenchaoyi/tap/gtmux-app` downloads + verifies
the zip, then errors `It seems the App source '/Applications/Gtmux.app' is not there.`
(often on a machine that previously ran `gtmux update`).
**Root cause:** the app has **two install channels that targeted different dirs** — the
Homebrew cask installs to `/Applications/Gtmux.app`, but `install.sh` / `gtmux update`
installed to `~/Applications/Gtmux.app`. If a user did both, `/Applications/Gtmux.app`
goes missing (only the `~/Applications` copy is current), and Homebrew's cask uninstall
step can't find the app it recorded at `/Applications` → the error. NOT a bad zip or
cask stanza (`ditto --keepParent` + `app "Gtmux.app"` are correct).
**Fix:** `install.sh` now **co-locates** — if `/Applications/Gtmux.app` exists (a cask
install) and `~/Applications/Gtmux.app` doesn't, it updates the `/Applications` bundle
in place instead of making a second copy, so the two channels stay on one app.
**Unstick a machine now:** `brew uninstall --cask gtmux-app --force` (forgets the broken
state) then `brew install --cask chenchaoyi/tap/gtmux-app` — or just switch to the curl
installer: `curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash`.
(The separate deprecation *warning* `depends_on macos: ">= :ventura"` is cosmetic; the
cask generator now emits `depends_on macos: :ventura`.)

---

## Remote access / pairing / push

### Menu-bar Off / Wi-Fi picker "won't change" from Anywhere — on the Direct backend
**Symptom:** on `Anywhere`, tapping `Off` or `Wi-Fi` in the menu-bar Remote-access picker
snaps straight back to `Anywhere`. Reproduces only when the tunnel backend is **Direct**
(self-hosted); on Standard/Cloudflare the picker works.
**Root cause:** the picker's mode is DERIVED from which LaunchAgents exist
(`groundTruth()`: `cfOn || selfOn ? .anywhere : …`). `serviceRemoveAll()` (Off) and
`serveServiceInstall()` (Wi-Fi) tore down `com.gtmux.serve` + `com.gtmux.tunnel`
(Cloudflare) but **skipped `com.gtmux.selftunnel`** (the Direct agent) — so on Direct it
stayed loaded, `selfOn` stayed true, and the mode re-derived to `.anywhere`.
**Fix:** both teardown paths now remove ALL three agents (serve + tunnel +
**selftunnel**), matching `tunnelServiceRemove` (`gtmux tunnel --unservice`). Pinned by
`TestServiceRemoveAllDropsSelfTunnel`.

### "Pairing code expired" that never clears — check for a DUPLICATE serve on :8765
**Symptom:** menubar "refresh code" → phone scans → *invalid or expired enroll code*,
no matter how fresh the code, across app reinstalls and `gtmux update`.
**Root cause:** two `gtmux serve` processes on 8765. The menubar mints via
`POST 127.0.0.1:8765` (IPv4 → serve A); the tunnel ingress `http://localhost:8765`
resolves to `::1` (IPv6 → serve B). **Enroll codes are in-memory per process**, so a
code minted on A is absent on B → "expired". (The same split corrupts push-token state.)
**Must-check (run this FIRST when pairing/push misbehaves):**
```
lsof -nP -iTCP:8765 -sTCP:LISTEN     # MUST show exactly one PID
ps aux | grep 'gtmux serve' | grep -v grep
```
Expect ONE serve — the app's `com.gtmux.serve` LaunchAgent
(`/…/Gtmux.app/Contents/MacOS/gtmux serve --bind 127.0.0.1 --port 8765`). Any second
`gtmux serve` (especially bare, binding `*:8765`) is a squatter → kill it. With only
`127.0.0.1` listening, cloudflared's `localhost` falls back to IPv4 and hits the same
serve the menubar mints on.

### Don't restart `gtmux serve` between mint and scan
Enroll codes (TTL 5 min) live only in memory; a serve restart (incl.
`launchctl kickstart`, and the `launchctl unload/load` that `gtmux tunnel --service`
does) wipes every pending code → a just-minted QR reads as "expired". Mint → scan
without bouncing serve in between.

### Tunnel silently offline on a corp network — QUIC is blocked
**Symptom:** phone gets Cloudflare **1033 / HTTP 530**; `tunnel.log` loops
`failed to dial to edge with quic: timeout` / `no free edge addresses left to resolve to`.
**Root cause:** cloudflared defaults to QUIC (UDP/7844); many corp/campus nets block it.
**Fix:** `--protocol http2` (TCP/443) — now the gtmux default for all cloudflared
launch paths (override with `GTMUX_TUNNEL_PROTOCOL`). An **old** service plist keeps
QUIC, so after `gtmux update` re-run `gtmux tunnel --service` to regenerate it.
Diagnose with `gtmux doctor` (the tunnel row reads `status/tunnel.json`: backend,
state, since when, last error) and `gtmux logs --component tunnel --since 1d` for the
transitions. cloudflared's own text is in `~/.local/share/gtmux/logs/cloudflared.stderr`
(`tunnel.log` in the data root under a plist written before v1.0.35) for the edge
messages, but nothing in gtmux reads it for state any more. See
`docs/design/remote-access-tunnel.md`.

### Corp-DNS hijack ≠ dead tunnel
`ccy.dev` answers come back as `172.19.x` IPs, so the Mac's own reachability probe fails
on a *healthy* tunnel (returns HTTP 530). On the maintainer's Mac those are **Clash
Verge's TUN fake IPs** (gateway `172.19.0.1` on a `utun`), not the office network: on
2026-10-05 a query sent straight to `1.1.1.1` from a home network came back `172.19.x`
too. See "The tunnel dies with one proxy node" below. Verify the
last hop from a **phone on cellular**, not from the office LAN. `api.cloudflare.com` is
also intermittently TLS-reset here — retry `wrangler`.

### The app classifies enroll failures — read the phone's message
Since the enroll-error split, the phone names the failure class: *can't reach* /
*tunnel offline* / *code expired* / *no token*. Use that to jump straight to the right
section above instead of guessing.

---

## HQ attention system / perception feed

### `gtmux hq` said it focused the supervisor but the HQ session is dead
**Symptom:** you quit the HQ agent but left its tmux window open (a bare shell). Later
`gtmux hq` said it had focused the running supervisor and jumped to that window — which
held only a shell prompt, no agent. Confusing.
**Root cause:** `findHQPane()` detects HQ by a pane STAMP that survives the agent
exiting, so `gtmux hq` treated a stamped-but-dead pane as "running" and focused it.
**Fix:** `gtmux hq` now checks the pane's foreground command (`hqAgentAlive` →
`pane_current_command`): a shell means the agent exited, so it RELAUNCHES the agent in
that same pane instead of focusing a dead prompt (`agentAliveByCmd`, pinned by
`TestAgentAliveByCmd`).

### A dispatched worker shows `done` in `gtmux tasks` but never ran
**Symptom:** you `gtmux spawn` a task; `gtmux tasks` (and HQ/the digest) show it `done`,
but the worker's tmux pane is actually sitting at the "Do you trust the files in this
folder?" startup gate (or holds the goal UNSUBMITTED in the composer — a long paste
swallowed the Enter). Not one step ran.
**Root cause:** `waiting` (needs-you) was HOOK-marker-driven ONLY. The startup gate and
an unsubmitted composer fire NO gtmux hook, so the radar read the pane `idle`, and
`taskStatusFor("idle")` mapped idle → `done` unconditionally — no `waiting` wake either.
**Fix (v0.28.9, stuck-dispatch-waiting):** a narrow screen-content guard — for a TRACKED
dispatch whose capture shows a startup/permission gate (`prompt.IsStartupGate`, per-agent)
or a structured non-empty draft (`dispatch.DraftOfColored`) — reclassifies it `waiting` (kind
`startup`/`draft`), never `done`. The serve slow-tick writes the marker + fires a
`waiting` wake so HQ unblocks it; `wakeDone` also skips `done` when the post-Stop screen
is a gate/draft. All other waiting stays hook-driven. **Unstick now:** answer the gate /
press Enter in the pane.

### `gtmux spawn` starts a session but the goal never arrives — and `tasks` says `done`
**Symptom:** `gtmux spawn --goal-file …` prints what reads like a normal startup log,
the session is up, and the composer is **empty** — the goal was never entered. `gtmux
tasks` shows the entry green `✳ done` with the full goal text beside it. `digest`'s
goal/last for that pane are blank. `gtmux send --message-file <the same file>` into that
pane lands first try. Hit on 2026-08-01, 08-03, 08-06, and twice on 08-09.
**Root cause (two independent defects, and you need both fixes):**
1. The readiness gate listed `MCP servers need authentication` as a **boot banner**. On a
   Mac that permanently carries `⚠ 10 MCP servers need authentication · run /mcp`, that
   line is not startup noise — it never clears — so `hasBootBanner` matched forever, the
   gate could never settle, and every spawn timed out into `NOT delivered`. (#652
   narrowed the MATCHING but left the phrase in, fixing only the "transcript mentions it"
   variant — which is why "✅ fixed in v0.44.10" was wrong and cost two more incidents.)
2. `gtmux tasks` derived status from the pane ALONE (`taskStatusFor`: idle → `done`). A
   ready-timeout leaves a live, empty, **idle** agent pane, so the failed dispatch
   rendered as finished work. The ledger recorded `delivered:false` the whole time —
   nothing but the resume lookup ever read it.
**Fix (spawn-readiness-persistent-banner):** a **standing notice** — a bottom line naming
an action only the user can take — is no longer a boot banner; only chrome that RESOLVES
BY WAITING (`Connecting…`/`Loading…`) holds the gate, with the two-frame settle covering
the transient window. The ledger stores the dispatch `state`, and `tasks`/`digest` read
it: a never-landed dispatch is `✗ undelivered`, sorted first, never `done`. A `gtmux
send` that lands in that pane back-fills the entry.
**Must-check when a spawn looks wrong:** read the FIRST line of the failure, not the
screen dump under it — it now says `blocked by: <the line that said no>`. spawn DOES
report this on **stderr with exit 1**; the reason it reads as "no error" is that the old
evidence appended the full capture (measured: 224 lines / 11.8 KB), so the verdict was
the head of a wall that looks like a boot log. **Rule:** a spawn whose `tasks` row is
`undelivered` was never dispatched — re-send with `gtmux send %N --message-file <path>`,
or fix the blocker the evidence names (`/mcp` to authenticate, answer the trust gate, …).

### `gtmux reap` kills the session and worktree but leaves the branch behind
**Symptom:** you squash-merge a PR, run `gtmux reap`, and it reclaims the session and the
worktree but reports the branch as **not merged** and keeps it. Meanwhile `gh pr view
<branch>` from your terminal says `MERGED`. Hit 2026-08-09 on `fix/app-hq-history`.
**Root cause — TWO faults that compound, which is why it looked like "squash detection is
broken":**
1. `defaultBranch` stripped the `origin/` prefix and returned the LOCAL branch name, so
   both git-side probes judged against a local `main` that nothing had pulled — `gh pr
   merge` doesn't fetch, and under a worktree layout the local `main` belongs to another
   checkout that may be pinned for other work. Measured on the branch that hit this: the
   squash-equivalence scan had an **empty commit range** against local `main` and said no;
   against `origin/main` the branch tip's tree matched the squash commit exactly.
2. `prMerged` folded every `gh` failure into `false`. `gh` lives in `/opt/homebrew/bin`;
   a gtmux started by launchd inherits `/usr/bin:/bin:/usr/sbin:/sbin`. So the last
   remaining probe was invisible, and its silence became a confident "not merged".
**Fix (reap-merged-detection):** the base is the remote-tracking ref, `gtmux reap`
refreshes it before judging (bounded + best-effort; the hook-driven reap-suggest sweep
still does no network), `gh` resolves through the shared `internal/toolpath` search, and
the result is three-way — "could not establish" fails the gate CLOSED but says so and
names the remedy instead of asserting unmerged commits. Verified end-to-end on the two
real leftover branches: both judge merged, **and still do with `gh` hidden** — the single
point of failure is gone.
**Must-check:** `gh` being on YOUR PATH proves nothing about the PATH the failing process
had. Before concluding a shelled-out tool is "not installed", check it through
`internal/toolpath` (or `env -i PATH=/usr/bin:/bin sh -c 'command -v <tool>'`), and never
let a tool's *unavailability* stand in as its *answer*.

### HQ's startup briefing typed into the input box but never sent
**Symptom:** `gtmux hq` starts the agent, a long "Startup briefing — make this your very
first output…" prompt sits in the input box UNSENT, and HQ stalls waiting.
**Root cause:** the briefing used to be a huge multi-line prompt PASTED into the pane and
submitted — fragile (a long paste + a single Enter can land as typed-but-not-submitted,
especially on a just-started agent) and Claude-Code-specific.
**Fix (v0.28.8, playbook v6):** the briefing CONTENT + format now live in the seeded
playbook (`AGENTS.md` "## First turn"), read by any agent via its own convention file;
gtmux injects only a MINIMAL one-line trigger — `» gtmux·startup` — which submits
reliably and is agent-agnostic. (Unstick a stalled one: just press Enter in that pane.)

### An `event-sequence gap` warning on a pull — events rotated away unread
**Symptom:** `gtmux events --since-seq <n>` prints a CRITICAL warning about a sequence
gap.
**Root cause:** the journal retains a bounded window (8 MB × 2 generations); events
between HQ's watermark and the retained tail were rotated away before being read —
usually after HQ was down or silent for a long stretch.
**Must-check / fix:** rebuild from `gtmux digest --json` FIRST, then
`gtmux events --ack <latest>` — acking over the gap without the snapshot forgives the
loss silently. If gaps recur, check that `gtmux serve` is running (without it the wake
and unread knocks that keep HQ consuming never fire).

### HQ went quiet — is it the feed or the surfacing threshold?
**Symptom:** HQ stopped printing routine updates.
**Root cause:** by design. The feed is SILENT (gtmux no longer types low-value receipt
nudges into the pane); HQ only PRINTS CRITICAL/NORMAL and ledger-records QUIET. Quiet
mode raises the bar to CRITICAL-only.
**Must-check:** `gtmux quiet status` (the resolved threshold). QUIET items are in
`gtmux tasks --verbose`, not lost. A read-time gap CRITICAL is never quieted, so
silence there means perception is healthy, not broken.

### The playbook upgrades itself — never hand-edit AGENTS.md
The supervisor's charter (`~/.config/gtmux/hq/AGENTS.md`) is a MANAGED file: it carries a
version+language marker, and `gtmux hq` regenerates it automatically whenever the shipped
version is newer OR `GTMUX_LANG` names a different language than the installed edition —
the prior file is backed up beside it (`AGENTS.md.bak-v<N>-<lang>`) first. There is
nothing to re-seed by hand, and edits to AGENTS.md are displaced to a backup on the next
upgrade; your customizations belong in `LOCAL.md`, which no upgrade ever touches.

---

## Reclaiming a dispatch (`gtmux reap`)

### `reap` removed the session + worktree, left the branch, and **said not a word** (2026-08-09)
**Symptom:** `gtmux reap <id>` prints two `✓ reaped:` lines (killed session / removed
worktree), **no `deleted branch` line and no explanation at all**, exit code 0. The branch
stays where it was. This happened on v0.48.1, that is, after PR #746 ("a merged branch stops
being reported as unmerged") was already on the live path.
**Root cause — three defects stacked together, all downstream of the gate:**
1. `planAndReap` ran `removeWorktree` first, then called `deleteBranch(t.Worktree, …)`.
   `DeleteBranch` finds the main repo with `mainRepo(wt)`, which **asks git from inside the
   wt directory**, and that directory had just been deleted, so
   `git -C <deleted dir> branch -d` → `fatal: cannot change to '<path>'` → exit 128.
   (`spawn`'s `rollbackWorktree` had this right all along: `MainRepo` was exported **for
   exactly this ordering**, with a comment that says "resolve BEFORE removal". reap never
   adopted it.)
2. Even with the right path, `git branch -d` runs **its own** merge test, which accepts only
   "is an ancestor of HEAD/upstream". It is structurally blind to a **squash merge**, which
   is the shape this repo (and GitHub's default) produces every day. So the gate says
   merged and git still refuses.
3. All three steps were `if op(…) == nil { record an action }`: **success was recorded,
   failure passed in total silence**. So the only trace of a failure was one missing line,
   still with `✓` + exit 0.
**Why #746 didn't catch it** (worth more than the bug itself): #746 fixed the **judgment**
and verified the **judgment function** (`BranchMerged`, run against real branches, green).
But every reap test in `internal/app/reap_test.go` is driven through injected ops, with
`deleteBranch` stubbed as `func(...) error { return nil }`, **unable to fail by
construction**, so the assertions could reach no further than "this step was called". Two
of the defects sat exactly in the gap **below the injection seam and above the functions
`git_test.go` tests**, where neither side's green light reaches.
**Rule:** a fix to a command must be verified at the **command level** (real repo + real
git), not only at the function level. A correct judgment is not the action done. For any
"gate passes → perform a side effect" structure, the test asserts that **the world changed**
(the branch is really gone), not that the stub was called. See
`internal/app/reap_live_test.go`.
**Fix (PR #748):** ① resolve the repo **before** removing the worktree (new
`reapOps.mainRepo`); ② once the gate has confirmed the merge, `deleteBranch` uses `-D` (the
gate is strictly stronger than `-d`; paths that didn't run the gate still use `-d`); ③ new
`reapResult.Failed` + a `⚠ but these steps failed` block + a non-zero exit when any step
failed, and `gitRunLoud` folds git's stderr into the error (`exit status 1` told the user
nothing).

## Driving a pane (dispatch / `gtmux send`)

### An "is there a draft?" check MUST read the COLOR capture (2026-08-09)
**Symptom:** `gtmux send` refuses with "that pane has unsent text in its input box" against
a pane whose box is visibly EMPTY. Shipped in v0.46.2, fixed in v0.46.3.
**Root cause:** Claude Code renders its suggested-next-command as FAINT (SGR 2) ghost text
inside the input box. `tmux capture-pane` WITHOUT `-e` strips the SGR markers, so the ghost
comes back as ordinary text and every "is the box empty?" caller reads it as a half-typed
draft. Measured live on pane `%7`: the plain read returned `把评论里 273 改成 265` ("change
273 to 265 in the comment"), the color read correctly returned nothing.
**Rule:** a caller asking *"is there an unsubmitted draft?"* uses `dispatch.DraftOfColored`
on a `tmux.CaptureFullColor` capture — never plain `SplitInputRegion`. This is written at
`internal/dispatch/region.go`'s `DraftOfColored` doc comment, which names the exact failure
("a stuck `waiting`, a suppressed `done`, a held HQ nudge") — the draft guard became its
fourth instance. Callers matching a SPECIFIC payload (Deliver's verify, the wake-ack `#id`)
are exempt: a ghost can never equal their target.
**Also:** refuse only on TWO agreeing frames. One frame mid-repaint can show a phantom, and
a refusal — unlike the wake channel's queue-and-retry — cannot be taken back.
**Must-check when adding any such gate:** run it against a LIVE Claude pane that is showing
its ghost suggestion, not just against test fixtures. Fixtures carry no SGR, so
`DraftOfColored` and `SplitInputRegion` agree on them and the bug is invisible in CI.
**Second false positive, same gate (fixed in the same round):** a pane running neither a
known agent nor a bare shell (vim, ssh, a TUI — routing fails SAFE to the agent pipeline for
those) has NO composer, so `SplitInputRegion`'s no-box degrade path returns the TRANSCRIPT as
a draft — measured live at 347 characters of log output on a pane running `worker`. Any
"is there a draft?" gate must therefore ALSO be scoped to panes a known agent drives
(`Opts.HasComposer`, from `radar.AgentDriverKey`), not merely to "not a shell".
**And the shape of the gate matters as much as its inputs:** it must fail OPEN on every
condition it cannot judge (unreadable capture, copy-mode, no input region) and be bounded —
a pre-check on the send path may cost a known amount, it may never loop or hang. The live
probe to reproduce all of this: build `DeliverOpts` for a pane, call the guard read-only,
and print `driverKey / hasComposer / refused` across an agent pane, a TUI pane and a shell.

### A screen check matched what the pane was QUOTING, not what it was drawing (2026-08-10)
**Symptom:** HQ is knocked `» ◆ gtmux·waiting … stuck before running — startup` for a pane
that is working normally, over and over. Measured: 36 deliveries for one pane (`%74`) across
13.6 hours, all false.
**Root cause:** `prompt.IsStartupGate` matched its signatures with `strings.Contains` over
the WHOLE capture — and a capture is `capture-pane -S -200`, i.e. 200 lines of scrollback.
That pane's own Edit diff had rendered this repo's gate table, `"": {"Do you trust the
files"}`, onto its screen at 16:51:23; the first false knock landed at 16:56:25 and they ran
until 06:34. gtmux was reading the pane's CONTENT as the pane's CHROME.
**This is a repeat.** #652 fixed exactly this for boot banners ("a pane whose scrollback
merely MENTIONED those words") by anchoring them to the bottom region — and left the sibling
function unanchored. When you narrow one screen predicate, check every predicate reading the
same capture.
**It costs twice, and the second cost is silent.** The same `IsStartupGate` call suppresses
the `done` wake (`internal/hook/nudge.go`), so a pane that merely quoted the phrase would
also have had a real completion withheld from HQ — a false positive here manufactures noise
in one direction and swallows signal in the other.
**Rule:** a predicate that asks *"is the agent DRAWING this right now?"* reads only the
bottom region (`bottomLines`) of a faint-stripped capture. Whole-capture `Contains` answers
a different question — *"does this text appear anywhere in 200 lines of history?"* — and
that question is never the one being asked.
**Second fix in the same call:** a "stuck BEFORE running" claim is about the DISPATCH, not
the screen, so the ledger decides it — `Task.Undelivered()`. A dispatch whose goal landed was
accepted by the agent and is past both a launch gate and an unsent goal; it is not
screen-classified at all. Screen evidence is the fallback for a question the records can't
answer, never the first source when they can.
**Third, found while fixing:** the per-agent gate map is keyed by registry KEY (`codex`) but
the radar and the HQ slow tick pass the display LABEL (`Codex`), so codex's gates — added the
day before precisely to see a stuck codex worker — resolved to nothing on exactly the paths
that watch a live fleet. A per-agent table lookup must accept whichever identity its callers
actually carry; `agents.KeyForLabel` is the bridge.
**Live repro (a real frame, not a fixture):** print the quoted phrase into a scratch tmux
pane, then push it up past the bottom region with ~30 more lines. `grep` still finds it in
`capture-pane -S -200` (what the old code saw) while the predicate now answers false.

### One instruction pasted 2–3× and submitted in pieces
**Symptom:** a dispatched message appears in the agent's box twice or three times, is
submitted line by line (the tail lines land as "queued messages"), the Enter looks
swallowed and needs a manual re-press — and `gtmux send` still reports `NOT delivered`.
**Root cause:** two, and they compound.
1. `paste-buffer` was **not bracketed** (`-p`), so the payload went in raw and every
   `\n` reached the TUI as a bare Return — submitting each line as its own message.
2. The fragment retry called `ClearDraft` (C-u) and re-pasted **without checking the
   clear worked**. C-u kills only the line the cursor is on; against a multi-line draft
   a second C-u (and Escape) do nothing at all. So the retry pasted onto the leftover
   and concatenated a copy — `PasteRetries: 2` → up to three copies.
**Rules:**
- Any tmux paste into an agent TUI is `paste-buffer -p`. Test with a **multi-line**
  payload — single-line text hides both bugs completely.
- Never re-paste into a box you have not SEEN go empty. Clearing a draft is not
  reliable; failing loudly with evidence beats duplicating an instruction.
- The frame right after a paste is not evidence — the TUI redraws on its own schedule.
  Let a paste settle before judging it a fragment (a stale frame read as a fragment is
  what triggered the destructive retry).
**Must-check:** reproduce against a real agent pane, not a fake — `tmux new-session -d
-s lab; tmux send-keys -t lab claude Enter`, then send a 3-line instruction and read
the box. A unit test with single-line fixtures passes either way.

### A dispatched goal's backticks were executed by the shell, and spawn died on the spot (2026-08-01)

**Symptom** — HQ runs `gtmux spawn … "<a long Chinese goal with code identifiers wrapped in backticks>"`,
the shell reports `command substitution: syntax error near unexpected token 'done'`, and
spawn never gets going at all.
**The side effects are worse**: the worktree and branch had already been created, but no
session started; on retry `git worktree add` reports `exit status 128`; the two attempts
left two empty sessions (neither one got its goal).

**Root cause** — the goal travels as **argv**, so it necessarily passes through the
caller's shell first: backticks inside double quotes are **executed** and `$x` is expanded;
an unquoted newline ends the command, while a newline inside quotes stays part of the
argument. A long enough natural-language instruction will contain these characters sooner
or later, so "be careful with quotes every time" is not a property a system can hold. The
record bears this out: this trap was already in HQ's knowledge base **twice**, had been
promoted to a general rule that same morning, and was stepped on again a few hours later.
This is an interface problem, not a memory problem.

**Fix (this time)**

- `gtmux spawn --goal-file <path|->` / `gtmux send --message-file <path|->`: the caller
  writes a file, gtmux reads the bytes, and there is no shell on the path. One stated
  normalization only: at most one trailing newline is removed (every heredoc adds one).
- `gtmux spawn --oneshot`'s goal now goes through a staging file too (it used to be
  shell-quoted with whitespace collapsed, which squashed multiple lines into one).
- **Failure is re-entrant**: an existing worktree is reused (no more 128), a session from an
  earlier attempt whose goal was never delivered is taken over, and a worktree/branch that
  this attempt created but didn't use is rolled back. **Re-running the same command
  converges**; don't clean up by hand.

**Rule (same root as the base64 one)** — whenever a piece of text has to pass through a
shell, ask whether it contains backticks, newlines, quotes or `$`; if it does, keep it off
argv. Giving the tool a **file channel** is more reliable than giving callers a discipline.
Tests must assert a **byte-for-byte round trip**, not just that the command didn't error:
quoting that "looks right" fools people.

---

## Disk / storage

### gtmux state dir balloons to GB (disk red line)
**Symptom:** `~/.local/share/gtmux` grows to hundreds of MB or GB; a disk-space alarm
fires. `gtmux doctor`'s `Storage` row shows red (`✗ very large`).
**Root cause:** it is almost never the event log — `events.jsonl` (20 MB) and the HQ
spool (8 MB) already self-rotate. The culprit is an **unrotated launchd log**:
the launchd captures (`logs/<component>.stderr`; `serve.log` / `tunnel.log` /
`selftunnel.log` in the data root under a plist written before v1.0.35) are plain
`StandardOutPath`/`StandardErrorPath` redirects launchd never rotates, and the gtmux
process can't `SetOutput` a redirect it doesn't own. A chatty daemon — classically
`cloudflared` retrying forever against a **QUIC-blocked** corp network — writes with no
ceiling. Secondary: the `uploads/` dir (phone images) and the per-pane churn markers
(`frame/`, `cpu/`, `goalchanged/`, `sends/`) that never cleaned up a dead pane's leftover.
**Fix / must-check:**
- `gtmux doctor`'s `Logs` section names the store's size, a runaway writer (a day of
  `logs/` past 20 MB, with the component and event that filled it) and every other store
  over its bound; `gtmux doctor --fix` trims them. Otherwise
  `du -ah ~/.local/share/gtmux | sort -rh | head` finds the big file. A multi-hundred-MB
  `logs/cloudflared.stderr` (or legacy `tunnel.log`) confirms cloudflared churn (check the tunnel is actually up; see the
  QUIC-blocked entry).
- The slow-tick hygiene sweep (`internal/hq/diskhygiene.go` `diskHygieneSweep`) caps each
  log to its recent tail (8 MB → last 2 MB), age-prunes + LRU-trims `uploads/`, and ages
  out dead-pane churn markers, every 30 min while `gtmux serve` runs. If serve isn't
  running, nothing trims — start it, or manually `: > ~/.local/share/gtmux/logs/cloudflared.stderr`.
- `events.seq` is a single monotonic integer — never delete it to reclaim space; a reset
  would break every consumer's durable cursor.

---

## Radar / process sampling

### Ghost native rows: "two codex sessions running" that nobody started (agent-internal helper calls)
**Symptom:** `gtmux digest --json` / `agents --json` grow `source:"native"` rows —
seen twice as a working+idle Codex pair, ~1 min apart — for sessions nobody started.
The `working` one never resolves; "kill it" has no target. Fingerprint of a fake: no
pane/session/window, no PID, zero tokens in `gtmux usage`, only 5 digest fields
(agent/source/status/since/sense — no goal/last/cwd), record `cwd:"/"`, and the
session id is absent from the agent's own session index (`~/.codex/session_index.jsonl`).
**Root cause:** an agent's INTERNAL helper calls — Codex's ambient-suggestions
generator (`~/.codex/ambient-suggestions`, prompt head `# Overview Generate 0 to 3
hyperpersonal…`) and its auto-mode safety classifier (`You are an expert at upholding
safety…`) — run as full agent sessions: they read `~/.codex/hooks.json` and fire
pane-less SessionStart/UserPromptSubmit/Stop through `gtmux hook --agent codex`, which
sensed them as native sessions. The "Codex" attribution is thus FACT, not a fallback
bug — the helpers really are codex processes. They surfaced only after #703 made codex
hooks fire at all. `working` sticks because the helper's Stop carries no session id to
pair back; no PID because the codex hook path re-execs detached (`setsid`), severing
the ancestry walk.
**Fix (fix/codex-ghost-rows):** two layers, both on positive evidence — never "pane is
empty" alone (real native sessions are pane-less too). ① `internal/hook/helper.go`: a
pane-less UserPromptSubmit whose prompt head matches the known-helper list erases the
session (record dropped, id marked, later events swallowed; the streamed SessionStart
gets its pairing SessionEnd so the unread-debt blink exclusion covers it). ② radar
`nativePanes`: a record with no live PID AND no on-disk conversation is withheld from
every surface.
**Must-check when it recurs:** pull the pane-less UserPromptSubmit's summary from
`gtmux events` — a new helper prompt head means the list in
`internal/hook/helper.go` needs that fingerprint (copy it verbatim from the event
summary; same normalization).
**Symptom:** right after changing networks (office ↔ home, VPN up/down) the menu bar
stops updating and the phone shows the server connected but **0 agents**. `ps aux` in a
shell ALSO hangs. `pgrep -f "gtmux agents"` shows dozens/hundreds piled up.
**Root cause:** a background system process, classically a **corporate VPN or endpoint-security
agent**, wedges in **uninterruptible kernel sleep (`U`/"stuck")** during the
network transition. The radar samples processes with a full-table `ps -axo …command=`,
which reads every process's argv (`KERN_PROCARGS2`); reading the wedged process's args
blocks **forever**, and a `ps` stuck in `U` can't be killed (SIGKILL/SIGALRM stay pending)
nor reaped. The menu bar shells out to `gtmux agents` every poll, so each poll spawns
another undying `ps` — observed **137+ stuck `ps`**, a frozen bar, and the serve (which
samples through the same call) returning an empty agent list to the phone.
**Diagnose (do NOT use full `ps` — it wedges too; each call adds another stuck `ps`):**
- `top -l 1 -stats pid,state,command -n 800 | awk '$2=="stuck"'` — `top` uses libproc and
  does NOT block. The **only non-`ps` "stuck" row is the culprit** (`ps` reading its argv
  is why everything else hangs). A **targeted** `ps -p <pid>` still works — it's only the
  full-table `-axo …command=` walk that blocks.
- The wedged process usually self-heals when its stuck kernel op (network) completes; it
  may flap stuck↔running as the network settles. You can't `kill -9` it while it's `U`.
**Fix (shipped v0.43.11, `internal/radar/agents.go` `boundedOutput`):** the full-table
`ps` runs with a hard timeout that **truly abandons** a hung child — read+`Wait` in a
goroutine, `select` against a timer, on timeout best-effort `Kill` and **return** a
degraded (empty) snapshot. A degraded snapshot loses only the CPU "working" signal +
bare-`node` (idle Codex) detection; **title-identified agents still show**, so the radar
keeps working through a wedge. ⚠️ **`exec.CommandContext` + `WaitDelay` is NOT sufficient**
— `WaitDelay` closes the I/O pipes but `Cmd.Wait` still blocks in `wait4()` on the
unreapable child (v0.43.10 shipped this and still hung). You must abandon the `Wait`.
**Recover a live wedge on an OLD build:** `gtmux update` restarts the serve+menu bar with
the fixed binary — `gtmux agents` then returns in ~4s even while the process is still
wedged. The leftover stuck `ps` drain when the wedged agent finally unblocks.

---

## Clicking a waiting session in the menu bar does nothing (v0.50.0–v0.51.0, `tab-alert` on)

**Symptom.** The menu-bar row for a session that needs you does not jump to its terminal
tab. Every OTHER row jumps fine. `gtmux focus <pane>` prints *"No Ghostty tab is showing
session 'X' (it may be detached)"* while the tab is right there.

**Root cause.** `tab-alert` (v0.50.0, #764) PREPENDS a `● ` to the tab title, and the jump
matched the raw title: the AppleScript asked `tn starts with "<session> — "`, which
`"● MP — api-service"` never satisfies. Because tab-alert marks **only waiting sessions**,
the failure landed exactly on the rows a user clicks — the ones that need them.

**Must-check when a jump misses.** Read the tab's REAL title before theorising:

```sh
osascript -e 'tell application "Ghostty" to return name of tab 3 of window 1'
# and to see which tab is actually selected — NOT `front tab`, which is always tab 1:
osascript -e 'tell application "Ghostty" to return name of selected tab of front window'
```

**Fix (v0.51.1).** Matching tolerates leading decoration — `ghostty.TitleMatchesSession` /
`StripDecoration`, used by Ghostty + Warp, with iTerm2's script accepting the marked form.
The general point, and why the fix is not about `●`: **the terminal decorates titles too**
(Ghostty prefixes a background tab that rang the bell), so a tab-to-session match must be
decoration-tolerant by construction. `SessionsFromTitles` already did this for the bell
glyph; the focus path did not.

**Two AppleScript traps found while fixing it.**
- `select tab 8 of window 1` is INVALID in Ghostty's dictionary — *"Can't get 8 of window
  1. Access not allowed. (-1723)"*. The only reference `select` accepts is the loop
  variable from `repeat with t in tabs of w`. So match in Go, then select by the tab's
  exact raw title through that loop.
- `name of front tab of front window` returns **tab 1**, not the selected tab. Verifying a
  jump with it reports failure on a jump that worked. Use `selected tab`.

## App Store upload "failed" when it actually landed (2026-08-18, 0.66.1)

**Symptom.** `bundle exec fastlane release` ends with a Ruby crash —
`uninitialized constant Gem::Resolver::APISet::GemParser (NameError)` followed by
*"RubyGems is not listed as your Gem source"* — and an immediate App Store Connect query
still shows the PREVIOUS build. Everything points at a failed upload.

**It had already uploaded.** Re-running the `upload` lane on the same ipa is what proved it:
altool answered *"Redundant Binary Upload. You've already uploaded a build with build number
'12' for version number '0.66.1'"*. Two independent things had lined up to look like failure:

- **The crash is teardown noise.** Homebrew ruby is at 4.0.6; the pinned fastlane (2.237.0)
  predates its Ruby 4 support (`[core] Support for Ruby 4` ships in a later release). A
  subprocess spawned after the transfer dies on it — after the binary is on Apple's side.
- **A fresh build is not in the builds list yet.** ASC takes minutes to surface an upload,
  so "the list still shows build 11" is not evidence of anything in the first few minutes.

**Must check.**
- Never judge a lane by an immediate ASC query. Ask again a few minutes later, or re-run
  `upload` — a *Redundant Binary Upload* error is positive proof the first one landed.
- Never run a lane through `| tail -N`: the pipe hands you `tail`'s exit code (always 0) and
  hides every line above the window. Redirect to a log file and grep it.
- The fix for the noise itself is `bundle update fastlane` (or a Ruby 3.x for this
  toolchain); until then, expect the crash and read past it.

## A pane goes blind: the agent moved, the hook lost its pane (2026-08-18, %13)

**Symptom.** The phone shows a calm, complete conversation whose newest turn is hours
old. The radar shows the pane `idle`. HQ's picture of it is frozen at the same moment.
Nothing errors anywhere — and the pane is working the whole time.

**What happened.** Claude Code 2.1.234 moved the session into a background host:

    34175  the pane's interactive client   TMUX=…  TMUX_PANE=%13
    77257    claude daemon run --origin transient --spawned-by {pid:34175}
    77301      --bg-pty-host
    78152        the conversation process  (no tty, no TMUX_PANE)

The hooks kept firing — `hookErrors` was empty and `stop_hook_summary` listed `gtmux
hook` every time — but from a process with no `$TMUX_PANE`. gtmux identified panes by
that one env var, so every event was filed as a NATIVE (non-tmux) session under
`native/<sessionId>.json`. The pane's resume binding never updated, so `/api/transcript`
kept serving a log whose last message was 17:47 while the conversation ran on until
22:58 in a session gtmux had never heard of.

**Diagnosing it (in this order — three of these contradict each other).**
- `~/.local/share/gtmux/resume/<base64 of session:window.pane>.json` — what the chat is
  being served FROM. Compare its `sessionId` against what is actually growing.
- `ls -lt ~/.claude/projects/<cwd-slug>/` — **mtime lies here.** The dead log had the
  NEWER mtime (Claude appends `permission-mode` records to it long after the
  conversation left). Read the last `"timestamp"` in the file, not the mtime.
- `ps -o ppid=,tty=,command= -p <pid>` up the chain from the agent process, and
  `ps eww -p <pid> | tr ' ' '\n' | grep TMUX` — the decisive evidence: which process
  actually runs the conversation, and whether it can see the pane.
- `~/.local/share/gtmux/native/` — a record here whose cwd matches a tmux pane is the
  fingerprint of this failure.
- `grep '"%13"' ~/.local/share/gtmux/events.jsonl | tail` — the moment the events stop
  is the moment the session moved.

**Fixed (hook-pane-identity).** A hook with no `$TMUX_PANE` now walks its process
ancestry and matches it against tmux's pane table (`pane_pid`, then `pane_tty`) before
concluding it is native. `gtmux doctor`'s `chat binding` row reports a pane whose
directory holds a newer, live, unclaimed session. The mobile chat polls while open
(conditional on an ETag) instead of refreshing only when the status flips — a pane whose
status is stuck cannot flip, which is why the reader saw nothing new for five hours.

**Must-check when something like this recurs:** the events stopping is not evidence the
hook stopped running. Check where the hook RAN before assuming it failed — and never
judge which log is live by its mtime.

### `gtmux hq` moved my terminal and said nothing (2026-08-20)

**Symptom:** you run `gtmux hq` expecting a supervisor to start. Instead the terminal
jumps to some other window, and as far as you can tell nothing was explained.

**What was actually happening:** a supervisor was already running, so the command
focused it rather than starting a second one — which is right, since two supervisors
would both be driving the same panes. It did print a line. But it printed it AFTER the
jump, so the words stayed in the window you were just taken out of, and the wording
("Focused the running supervisor") confirmed an action nobody had asked for instead of
answering the question you actually had.

**Fix:** the line now comes BEFORE the jump, names where the supervisor is
(`HQ:0.0 · %6`), and says plainly that nothing new was started — and the same fact is
put on tmux's status bar in the window you land in (`noteAtPane`), because that is where
you are looking once the jump has happened.

**The general rule this is an instance of:** say something when what happened differs
from what was asked. A jump you asked for needs no words; a jump you did not ask for
needs them, and they have to arrive where your eyes are. `gtmux focus` already worked
this way (it speaks only when the pane you land on is no longer running an agent).

## A permission prompt with the wrong app's name on it (2026-08-23)

**Symptom:** macOS asks *"Gtmux.app would like to access data from other apps"*, now and
then, while gtmux is doing nothing that would need it.

**It is not gtmux.** It is one of your own agents — clearing disk space, reading a cache,
walking `~/Library` — and the prompt is putting gtmux's name on it.

**Root cause:** macOS attributes a file access to the **responsible process**, the
application at the root of the process tree, and fixes that when a process is created. It
survives being re-parented to launchd, so nothing in `ps` reveals it — the tmux server
shows `ppid=1` and looks unowned. `gtmux restore` run from the menu-bar app creates the
tmux server as the app's child, and from then on every pane in that server, and every
command every agent runs in it, answers as Gtmux.app for as long as the server lives.

**How to check it yourself:**

```bash
log show --last 1h --predicate 'process == "tccd" AND eventMessage CONTAINS "AUTHREQ_PROMPTING"' \
  --style compact | grep -oE "responsible_path=[^,]*|binary_path=[^}]*"
```

`responsible_path` is the name on the dialog; `binary_path` is what actually asked. Seeing
`responsible_path=…/GtmuxBar` beside `binary_path=/usr/bin/du` is this exact case.

**Fixed (tmux-server-own-identity):** a restore started by the menu-bar app now creates
the server through launchd, which hands it an identity of its own — measured, same read
both ways: started directly → `GtmuxBar`, started by launchd → `tmux`. A restore from a
terminal is unchanged: it is already attributed to that terminal, which IS the app the
user launched.

**Two things worth knowing:**
- **It cannot fix a server already running.** Identity is set at creation, so an existing
  session keeps answering as Gtmux.app until tmux is next restarted.
- `setsid` does nothing here. Responsibility is neither the process group nor the parent,
  and survives both — tried first, measured, discarded.

## Reinstalling an agent's hooks silences the sessions already running (2026-08-29)

**Symptom:** an agent stops being seen entirely — no waiting, no done, no digest — while
its pane visibly works. `gtmux doctor` says its hooks are installed. They are.

**What happened:** `gtmux install hooks --agent codex` rewrote `~/.codex/hooks.json`
under a Codex that was already running. That session kept using the hooks it started
with for another two hours, then went silent across EVERY Codex pane at once, because
Codex re-read the changed file and stopped trusting the entries. Nothing said so: no
prompt on screen, nothing in 3000 lines of scrollback.

Measured: reinstall at 23:57 → last Codex event at 02:13 → zero Codex events for the next
six hours, during which an approval sat unanswered and the radar showed the session
`working`.

**Why the existing checks could not see it:** they ask about the FILE — is it installed,
is it complete. Both were true the whole time. The channel was dead anyway.

**Fixed (this change):** `gtmux doctor` gains a `hook traffic` row that compares two
clocks gtmux already keeps — the pane is painting, and no event has arrived:

```
⚠  hook traffic   %60 (6h)   these panes are busy but their agent has sent nothing
```

*(2026-09-14: the "painting" clock was retired — see "`hook traffic` named fourteen idle
sessions after a reboot" below. The row now judges by the hook's own last word.)*

**Must-check when installing hooks into a live fleet:** the sessions already running are
NOT left alone. Restart them (Codex asks you to trust the hooks again after a change —
press `t`), and check the `hook traffic` row afterwards rather than assuming.

## `hook traffic` named fourteen idle sessions after a reboot (2026-09-14)

**Symptom:** after a reboot at 09:16 and `gtmux restore` at 09:33, `gtmux doctor` listed
fourteen panes — `%1 (8h) · %3 (8h) · …` — as "busy but their agent has sent nothing;
hooks installed and not firing; restart the agent". Every one of them was a Claude session
`--resume`d by restore and not touched since: one `SessionStart` at 09:33, nothing after,
because nobody had typed a prompt. Meanwhile the three panes that WERE working (`%20`,
`%6`, `%29`) had every event.

**Root cause, two layers:** (1) the row judged "busy" by tmux's `window_activity`, and on
this machine something bumps EVERY window's activity every few minutes — the bash panes
too — so an eight-hour idle read as eight hours of painting with no word. Activity is a
claim about the terminal, not about the agent. (2) The row called `events.Read(now-3h, now)`
where `Read` takes a DURATION as its first argument, so it scanned the whole log; that is
how it could print "8h" at all against a 90-minute grace.

**Fix:** the row now judges by the hook's own last word (`hookSilentPanes`, pure and
tested on the day's data): a pane whose newest event opened a turn (`working`) and which
has said nothing since for longer than the grace is silent — the exact shape of the
2026-08-29 Codex incident. A pane whose last word was a start, a stop or a wait is idle
or waiting on a person, and is never named. `Read` gets its duration.

**Must-check:** when a doctor row reads "X is happening and Y is not", ask what each clock
actually measures. `window_activity` is per WINDOW and moves on any repaint; it can never
stand in for "the agent did something". The hook's state can.

## A session came back as a bare shell after a reboot (2026-08-29, Codex + `agentwrap`)

**Symptom.** After a machine restart, `gtmux restore` brought the tmux layout back but
one Codex pane was an empty shell — no conversation, no error, nothing in the plan
marked `×`.

**Two different things look identical here, and only one is a bug.**

1. **The save says the pane was a shell.** Then restore is CORRECT: the agent had
   already exited when the layout was saved, and injecting a resume into a pane that
   never ran an agent is exactly the phantom #688 removed. Check it before anything
   else:

   ```sh
   # what the save recorded for each pane — locator, command, full command line
   awk -F'\t' '/^pane/{print $2":"$3"."$6, $10, $11}' ~/.local/share/tmux/resurrect/last
   gtmux restore --plan          # ↻ = will come back · × = transcript gone
   ```

   A pane whose command is a shell and whose full command line is empty (a bare `:`)
   was a shell in the save, and no record anywhere overrides that.

2. **The agent was started through a wrapper.** `agentwrap`, an alias, an `npx`-style
   launcher — anything that is not the agent's own binary name. Until the launcher was recorded, the
   resume record held `{agent, sessionId, cwd}` and nothing about the launch, so the
   conversation was resumed as `codex resume <id>`: no wrapper, and none of the
   configuration, credentials or network the wrapper exists to provide. Claude hides
   this class of bug, because tmux-resurrect saves its whole command line and replays
   it verbatim; gtmux's own resume path is the one that assumed the launcher was the
   agent's name.

**Must-check when a wrapper is involved.** The launcher is read from the pane at hook
time, so a session running since before the upgrade has no launcher recorded:

```sh
# what gtmux will type to bring this pane's conversation back
cat "$HOME/.local/share/gtmux/resume/$(printf %s 'session:0.0' | base64 | tr '+/' '-_' | tr -d '=').json"
# → {"agent":"codex", …, "launcher":"agentwrap"}   ← present only after a prompt/session-start
tmux display -p -t %NN '#{pane_current_command}'  # the wrapper must be visible here
```

If `pane_current_command` shows the wrapper but the record has no `launcher`, the
session simply has not submitted a prompt since the upgrade — one more turn records it.

## Your half-typed line got submitted to HQ — and HQ acted on it (2026-08-29)

**Symptom.** You are mid-sentence in the HQ pane. Without you pressing Enter, what you had
typed is submitted — sometimes joined to text you never wrote — and HQ treats it as an
instruction. On 2026-08-29 a half-typed `%11 ` became `%11 /clear`; HQ read that as an
order and ran `gtmux send %11 /clear`, clearing a live session. Not recoverable.

**It is not always the wake nudge.** The nudge path has had a draft guard since
hq-nudge-hardening, and it holds correctly. Every path that types into a pane is a
suspect, and the fastest way to name the right one is the journal, because each writer
leaves its own audit record at the moment it typed:

```sh
# what typed into HQ, and what HQ submitted, in the same seconds
gtmux events --since-seq <N> --all | grep -E 'audit:(rotate|rotate-requested|rotate-failed|wake-delivered|send)|UserPromptSubmit'
```

Older releases wrote `gtmux:audit:rotate` when they pasted a reset, even if the agent
rejected it. In current releases, `rotate-requested` means queued, `rotate` names the
observed old and new session IDs, and `rotate-failed` explains an attempt that did not
settle. The historical bug here was `RotateHQ` pasting `/clear` and pressing Enter with
no draft check, because the guard had been written for the other writers first.

**Must-check when this class recurs.** The guard now lives in one place
(`dispatch.BoxEmpty`) and `scripts/check-design.sh` fails the build when a NEW file types
into a pane without being declared. So the question is not "was the guard added again"
but which of these it is:

- the guard ran and misread the box — reproduce with `dispatch.DraftOfColored` against
  the pane's real COLOR capture (`tmux capture-pane -e -p -t %NN`), not a plain one; a
  plain capture reads Claude's faint ghost suggestion as a typed draft, and an agent's
  input-box border changes shape between versions (see the TUI border entry above)
- a writer bypassed the seam — `bash scripts/check-design.sh` names it
- the text was submitted by something that is not gtmux at all (the agent's own paste
  handling, a terminal keybinding)

## An event type gtmux registers has never once arrived (2026-08-29, PreCompact)

**Symptom.** A whole class of hook event is absent from the stream — not rare, *zero* —
while the agent is plainly doing the thing that fires it.

```sh
# every non-gtmux event name actually received, with counts
gtmux events --since-seq 0 --all --json 2>/dev/null | \
  python3 -c 'import sys,json,collections
c=collections.Counter(json.loads(l).get("event","") for l in sys.stdin if l.strip())
[print(f"{n:6d}  {e}") for e,n in c.most_common() if not e.startswith("gtmux:")]'
```

Measured here against the raw log (`~/.local/share/gtmux/events.jsonl`): 28291 events
over 48 days, 11106 prompts — and `PreCompact` / `PostCompact` count **0**, while
`grep -c compact_boundary` over the four largest Claude transcripts found 61 real
compactions.

**The order to check things in, because the obvious first guess is wrong.**

1. **Is gtmux actually registered for it?** Not "is the event in gtmux's list" — is there
   a gtmux command in the agent's config, under that event:

   ```sh
   python3 -c 'import json,os;d=json.load(open(os.path.expanduser("~/.claude/settings.json")))
   [print(k, ["gtmux" if "gtmux" in h.get("command","") else "other" for g in v for h in g.get("hooks",[])]) for k,v in d["hooks"].items()]'
   ```

   This was the answer: the key existed with ANOTHER tool's hook under it, and no gtmux
   entry at all. `gtmux doctor` now reports this ("N events missing"), but only since
   v0.76.0 — before that the row read a green "installed".

2. **Does the agent support the event?** `strings -a "$(readlink -f "$(which claude)")" |
   grep -c PreCompact` — a name that appears nowhere in the binary was never going to fire.

3. **Only then**: the agent is installed but silent (see the entries above — codex's
   `async: true`, or a reinstall under a running session).

**Why the gap opens.** gtmux adds an event to its list; nothing rewrites the agent's
config on update; the file ages in place. The fix is `gtmux install hooks` (it merges —
each event's gtmux entry is replaced, every other tool's is preserved), and the price is
that some agents re-prompt for trust and running sessions may need a restart.

## HQ went deaf and three checks blamed three different things (2026-09-02)

**Symptom.** The supervisor stops responding to anything the fleet does. `gtmux doctor`
flags `hook traffic` and `event consumption`, a `wake-degraded` alarm fires every so
often, and none of it points anywhere useful.

**One cause, three descriptions.** All three were true and none named it: the HQ pane was
scrolled into tmux **copy-mode**, where injected keys are eaten as navigation commands
(`f` jumps, `/` searches). The wake guard held every knock there — correctly; injecting
would be worse — so 22 of them queued for 1h40m.

```sh
tmux display -p -t %NN '#{pane_in_mode}'    # 1 = scrolled; nothing gtmux types arrives
```

**The fix is one keypress**: `q` (or Escape) in that pane. The queue drains on the next
3-second tick — measured: 22 → 0 in six seconds.

To release it without touching the composer (a scrolled pane still has a draft you must
not disturb), send the copy-mode COMMAND rather than a key:

```sh
tmux send-keys -X -t %NN cancel
```

**What was checked and ruled out first**, in case the next occurrence is not this:

- **the tick loop wedged.** `h.tick()`, the slow tick and the wake drain share ONE
  goroutine and one `select` in `internal/server/events.go`, so a hung child process
  stops all three (this has happened — see the radar/`ps` entry). Ruled out by mtimes:
  `~/.local/share/gtmux/hqwake/{selfrotate-state,tick-seq}` were seconds old.
- **the input-box detector lost the box.** The agent was rendering a plain-rule box, not
  Claude's rounded one. Ruled out by feeding the pane's real COLOR capture to
  `dispatch.DraftOfColored`: `structured=true, draft=""`.
- **the HQ pane stopped resolving.** Ruled out by `hqwake/last-seen-hq`, which
  `hqpane.resolve()` touches on every hit — it was current.

**Must-check.** Since v0.77.0 the guard returns WHY it refused (`dispatch.Reach`) and all
three surfaces render it, so a recurrence of this particular cause names itself and the
key that clears it. If a future outage still reads as a generic "not landing", the cause
is one `Reach` does not model — start with `pane_in_mode`, then the three checks above.

## HQ's identity stamp disappears after a reboot (2026-09-02, silent for 4 days)

**Symptom.** None — that is the point. `gtmux hq --rotate`, the wake channel and the
digest all keep working, because HQ still resolves by PATH. What is gone is the lock that
makes the resolution unambiguous.

```sh
# every pane on the server, and which one claims to be HQ
tmux list-panes -a -F '#{pane_id}\t#{@gtmux_hq_home}\t#{pane_current_path}'
```

An empty second column on the HQ pane means the stamp is missing.

**Cause.** The stamp is a tmux PANE OPTION, and a pane option dies with its pane. It was
written in exactly one place — `gtmux hq` at spawn — so an HQ brought back any other way
(a `restore` after a reboot, a hand-restarted agent) never had one. Compare the tmux
server's start time with the HQ session's:

```sh
tmux display -p '#{t:start_time}'                 # server up
tmux display -p -t %NN '#{t:session_created}'     # HQ session created
```

One second apart is a restore, not a spawn.

**Why it matters even though nothing broke.** The stamp exists (hq-home-quarantine)
because the path criteria are AMBIGUOUS: a worker parked in the HQ home matches them too,
and first-line-wins let one steal the supervisor identity — wakes delivered to a worker,
the real HQ silent. Running on the fallback alone means that accident is one stray `cd`
away.

**Since v0.77.0** a path hit re-stamps itself, so this heals on the next resolve (seconds).
It re-stamps only a UNIQUE hit: stamping an ambiguous one would freeze a first-line-wins
guess into the authoritative answer, which is precisely the accident being prevented.

**Must-check when HQ identity misbehaves:** whether TWO panes match the home. If so, the
fallback is picking one by list order and no stamp will be written until that is resolved.

## A failing `claude -p /usage` spawned 94 headless sessions in 47 minutes (2026-09-07)

**Symptom.** The event stream filled with pane-less Claude Code `SessionStart`/`SessionEnd`
pairs — 12–50s each, zero work recorded, often two in the same second, ~6 per minute,
70+ within the hour. Nothing a person was doing.

**What they were.** `gtmux serve`'s limits refresher running `claude -p /usage`. Each
one leaves a 6-line transcript under `~/.claude/projects/-/<session>.jsonl` carrying
`entrypoint: "sdk-cli"`, `cwd: "/"` (launchd has no working directory, hence no
`$TMUX_PANE`), and the `/usage` output.

**Root cause.** `limits.Get` refreshes when the cache is stale and, on failure,
deliberately does NOT save — so a bad reading is never cached as fresh. Correct, but it
had no partner rule: the cache therefore stays stale by construction, and the *next*
caller refreshes again. With serve, the menu-bar app and the phone all asking, a
persistent failure turned "refresh every 15 minutes" into "refresh on every poll". The
trigger was an upstream outage (16 `server_error` StopFailures on the HQ pane in the
same 08:45–09:47 window); it ended by itself when the API recovered.

**Fixed** by recording the last ATTEMPT (`try_at`/`fails` in `limits.json`) separately
from the last SUCCESS (`at`), backing off 1 → 2 → 5 → TTL minutes, and bounding each run
with `limitsTimeoutSec` (default 60s; it had no timeout at all).

**Must-check when it recurs.** Compare the hourly rate against the TTL — 4/hour is
healthy, anything near 6/minute is the retry loop. `stat -f %Sm ~/.local/share/gtmux/limits.json`
shows the last SUCCESS; `fails` in that file now shows whether it is backing off. And
note the diagnostic trap: those transcripts all contain valid `/usage` text, because the
slash command reads local files — the text proves nothing about the process's exit code,
which is what gtmux actually judges on.

## An incremental device build can install last version's app

**Symptom.** `xcodebuild … MARKETING_VERSION=1.0.6` exits 0, `devicectl install` reports
success, and the app on the phone behaves exactly as it did before — none of the release's
changes are there. Settings shows the PREVIOUS version number.

**What it actually was (2026-09-08).** The incremental build reused two products from the
previous build: `Info.plist` and, worse, `main.jsbundle`. The native binary was rebuilt
(fresh timestamp) while the JavaScript — where nearly every mobile change lives — was the
old one. So the installed app was a new binary running last version's app code.

**Root cause.** With `-derivedDataPath ios/build/dd` reused across versions, Xcode judged
the plist-processing and RN bundle phases up to date. `MARKETING_VERSION` on the command
line changes the build setting but does not, on its own, invalidate a plist Xcode already
considers current.

**Must-check, before trusting any device install:**

```bash
APP=mobileapp/ios/build/dd/Build/Products/Release-iphoneos/gtmux.app
ls -l "$APP/Info.plist" "$APP/main.jsbundle" "$APP/gtmux"   # all three must be from THIS build
/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$APP/Info.plist"
```

If any is stale, `rm -rf mobileapp/ios/build/dd/Build/Products/Release-iphoneos` and build
again. Deleting only the products directory is enough; the module cache is what makes an
incremental build fast and it stays.

**The verification trap that hid it.** Confirming the version by grepping
`mobileapp/src/releaseNotes.ts` proves the SOURCE is current, which was never in doubt.
The artifact that ships is `main.jsbundle` inside the .app. Check the thing you are about
to install, not the thing you built it from.

## A test overwrote the real situation board (five times)

**Symptom.** `~/.config/gtmux/hq/notes/board.md` is replaced by a small test fixture.
Five times between 2026-09-08 and 2026-09-09.

**Why "write to a temp dir" did not fix it.** Every one of those tests DID redirect — they
set `XDG_CONFIG_HOME` or `XDG_DATA_HOME`. Nothing in gtmux reads either. `state.Dir()` and
`state.HQHome()` were built from `$HOME` alone, so the redirect was a no-op and the write
landed on the real path. The tests were not careless; the override that looks right is not
the one that works, so four rounds of fixing the offending test fixed a symptom.

**The fix (do not undo it).** `state.home()` is the single chokepoint every gtmux path goes
through, and under `go test` it PANICS unless HOME points somewhere under the temp tree.
The real path is unreachable from a test rather than discouraged. The panic names the fix
and says explicitly that the XDG variables are not read.

**Writing a test that touches gtmux state:**

```go
t.Setenv("HOME", t.TempDir())   // the ONLY redirect that works
```

**If you add another path root**, build it from `state.home()`. A root that reads `$HOME`
directly is outside the guard, which is exactly the shape of the original bug.

## A release tag landed on the wrong branch (2026-09-10)

**Symptom.** `v1.0.10` was cut, the release built fine, and then `git merge-base
--is-ancestor v1.0.10 origin/main` said NO. Every earlier tag sits on main.

**Root cause.** The tag was created by a command that began with `git checkout main`,
and the whole command was refused by the permission layer before any of it ran. The
refusal is not a failure of one step: **nothing in the chain executes**, so the checkout
never happened and the tag was written on the feature branch that was still checked out.
The same refusal, one command earlier, is why the tag body file "did not exist" — the
heredoc that would have written it was in the refused command too.

**Must-check.** After any refused command, verify the state you assumed it produced —
`git branch --show-current` before tagging, and `ls` the file you were about to read.
Never chain a branch switch with the action that depends on it.

**Recovery, when the release is already building.** Do not re-point a pushed tag. Put the
tag's commit onto main instead: open the branch as a PR and merge it with `--merge`, not
`--squash`. A squash gives the commit a new sha, so the tag would stay off main's history,
`git describe --tags` on main would keep answering with the PREVIOUS version, and
`mobileapp/scripts/set-version.sh` would stamp the next device build with it.

## A layout change that unit tests and the artifact both called fine (2026-09-10)

**Symptom.** The Detail screen's top chrome was invisible. Reported as "it folds and
never comes back", which is what it looks like — but it had never appeared at all.

**Root cause.** The chrome was moved out of the layout flow into a floating overlay, a
sibling of the body. The body's visible mode layer carries `zIndex: 1` (that is how the
Chat and Terminal layers stack), the chrome carried none, and the terminal painted over
it. The bands were laid out correctly the whole time — the e2e page dump put the back
button at y=65 — and a second defect rode along: floating over scrollback, the chrome
needs its own background, or the title and controls sit on top of terminal output.

**Why nothing caught it.** 886 unit tests passed, `check-design.sh` passed, and the
shipped bundle was verified to contain the new code and not the old. None of that can
see a paint order. **A change to layout, stacking, or anything else whose result is
"what is on the screen" is not verified until the screen has been looked at.**

**How to look, in about four minutes:**

```sh
(cd mobileapp && \
 GTMUX_E2E_UDID="${AUDIT_SIM_UDID:?set the owned simulator UDID}" \
 npm run e2e:build)                         # build and install on that simulator
(cd mobileapp && \
 GTMUX_E2E_UDID="${AUDIT_SIM_UDID:?set the owned simulator UDID}" \
 npm run test:e2e -- terminal-scroll-collapse)
```

Current harness: use Node 22.11+ in the CI's 22.x line. `test:e2e` starts Appium
itself, and this suite starts its own fake serve; it needs no real Mac token or
working pane. See [the e2e runbook](../mobileapp/e2e/README.md) for setup and limits.

`terminal-scroll-collapse` drags through scrollback and taps jump-to-bottom, and leaves
screenshots plus an XCUITest element dump under `.e2e-artifacts/latest/`. **Read the
screenshots** — the first failing run here reported only "could not reach Detail", and
the picture was the whole answer. The element dump gives frames, which is how you tell
"laid out in the wrong place" from "laid out correctly and painted underneath".

## A Live Activity never starts in the simulator (2026-09-10)

**Symptom.** The app runs, the radar fills, the Dynamic Island stays an empty notch, and
nothing appears anywhere. No error reaches the app: `LiveActivity.sync` calls `start()`,
the promise rejects, `started` resets, and the next refresh tries again — so it retries
quietly forever.

**Root cause.** ActivityKit refuses the request:

```
liveactivitiesd: [com.apple.activitykit:requestResolver]
  com.gtmux.app does not specify an APS environment name
```

The simulator package tested in this incident (`mobileapp/scripts/e2e-build-sim.sh`) passed
`CODE_SIGNING_ALLOWED=NO`, so no entitlements are embedded, so there is no
`aps-environment`, and a Live Activity cannot be created. Nothing to do with the widget's
code — the same binary's widget compiles and its views are fine.

**This incident does not establish that every simulator package behaves this way.**
Device acceptance is still needed for real push delivery and lock-screen presentation;
the simulator scripts' rendered lock-screen images are fixtures, not that acceptance.

**How to see it went wrong, next time, in one command:**

```sh
xcrun simctl spawn <udid> log show --last 5m \
  --predicate 'subsystem CONTAINS "activitykit"' | grep -iE "error|denied"
```

## Nothing in the release pipeline points the version at your new build (2026-09-10)

**Symptom.** `fastlane release` uploads, `fastlane metadata` pushes the listing, both say
they finished, and the version in App Store Connect is carrying the PREVIOUS build. On
1.0.12 that was build 14 sitting on the version while build 15 — the one with the release's
actual work — was processed and idle beside it. Nothing warns you; submitting there ships
the wrong binary.

**Why.** `deliver` does not select a build. The upload lane hands ASC a binary; the metadata
lane pushes text and screenshots. Choosing which build the version carries is a separate
act, and by default that is whatever was chosen last.

**So the release is not done until you have READ it back.** Three checks, in this order,
each with a script in `mobileapp/scripts/`:

```sh
eval "$(grep -E '^export ASC_(KEY_ID|ISSUER_ID|KEY_PATH)=' ~/.zshrc)"
bundle exec ruby scripts/asc-attach-build.rb          # attaches the newest processed build
bundle exec ruby scripts/asc-prune-dup-screenshots.rb # deliver double-uploads; prune to 6 (removed 2026-10-08, see below)
bundle exec ruby scripts/asc-attach-build.rb --list   # read it back: version + build
```

The screenshot one is not optional either: on this same release deliver left **10
screenshots per locale, four of them duplicates**, which is what it does on essentially
every run (see the entry above).

**Superseded (2026-10-08).** Screenshots no longer go through deliver: App Store Connect API
4.5.1 deprecated the set-based resources it uses. `scripts/asc-asset-library.rb
place-screenshots` places them through the App Asset Library, reusing images by content and
leaving a group alone when it already shows the same files in order, so nothing duplicates
and the dedupe script is gone. The read-back is `asc-asset-library.rb screenshot-status`.

## Removing a parameter from a positional list of numbers (2026-09-10)

**Symptom.** The HQ page's header folded once and never came back. Nothing in the app
looked broken; the header was simply gone for the rest of the session.

**Root cause.** `chromeDecision` had lost a parameter from the MIDDLE of its positional
list two days earlier (the chrome height, when folding stopped resizing anything). The
Detail screen was updated with it. The HQ page was not — and it kept passing four numbers:

```
chromeDecision(chrome.current, gap, chromeH.current, Date.now())
                                    ^ landed on `now`  ^ landed on `animMs`
```

Every number went to a number, so **nothing failed to compile**, every unit test of the
rule stayed green, and the mistake lived entirely at a call site nobody tested. After the
first fold, `settledAt` was a header height plus a timestamp — about 1.8e12 — and each
later reading arrived with `now` ≈ 180, which the rule reads as "still animating, ignore".
Frozen, permanently.

**The fix is the shape, not the call.** The reading is a NAMED object now
(`{gap, now, animMs?}`), so a call site left behind stops compiling — verified by putting
the old four-argument call back and watching tsc reject it. Naming cannot stop a wrong
value in a right-named field, and nothing could; it stops the stale-arity mistake, which
is the one that happened.

**Must-check when you change a signature:** if the removed parameter has the SAME TYPE as
its neighbours, the compiler is not your net. Grep every call site by name, and prefer a
named object over three-plus positional numbers.

**And the guard lives at the call site.** `e2e/hq-header-collapse` scrolls the page and
looks for the header, because the rule's own tests were green throughout. Drive the ACTS
tab there: the page's two kinds of zone fold on opposite gestures — a top-anchored list
folds as you scroll down into it, the console is pinned to its tail and folds as you scroll
away from it — so a test that lands on whichever tab was last open is a coin flip.

## Installing on the phone asks for it to be unlocked (2026-09-15, second time)

**Symptom.** `xcrun devicectl device install app --device <uuid> gtmux.app` fails with
`kAMDMobileImageMounterDeviceLocked: The device is locked.` while the phone sits locked on
the desk. The 2026-09-03 note said `devicectl install` works on a locked phone; it did then,
and the instruction to go unlock it went out again anyway.

**Root cause.** `devicectl` installs through CoreDevice, which needs the developer disk
image mounted, and mounts it on demand — and a mount needs the phone unlocked. On
2026-09-03 the image happened to be mounted already (the phone had been unlocked with Xcode
attached earlier that day). After an iOS update or a reboot it is not:
`xcrun devicectl device info details --device <uuid>` shows `ddiServicesAvailable: false`.
So "works on a locked phone" was true of a state, not of the tool.

**Must-check.** Install through the installation service instead, which never needs the
image: `ideviceinstaller -u "$(idevice_id -l)" install <app>` (libimobiledevice 1.2:
`install PATH`, not `-i`). It installed 1.0.25 on the locked phone in one go. Keep
`devicectl` for `list devices`; never for the install. Never ask the commander to unlock
the phone for an install.

## An idle pane reads "working" after a typed `/compact` (2026-09-15)

**Symptom.** Someone types `/compact` at an idle Claude prompt. The pane flips to
`working` on the phone and in the menu bar and stays there — thirteen minutes on %20,
until its user's next prompt — with no turn running.

**Root cause.** The hook's `PostCompact` rule re-armed the turn marker unconditionally.
It was written for the AUTOMATIC compaction (the context fills mid-turn, a `SessionStart`
arrives with the same session id, the turn carries on), where "compaction finished"
does mean "the turn continues". A typed `/compact` runs no turn: nothing starts after it,
so nothing ever sends the `Stop` that clears the marker. Claude's payload says which
case it is (`trigger: "manual" | "auto"`); gtmux was not reading it.

**Fix.** `decide` reads the trigger: `manual` touches no marker, `auto` (or an agent
that does not say) re-arms as before.

**What did NOT catch it, and why it is left alone.** The radar does have a staleness
guard on the turn marker (`activeQuietGrace`, 10 minutes of the pane's tmux window going
quiet), so a lone pane would have corrected itself after ten minutes. It keys on WINDOW
activity because that is the only clock tmux keeps, so a pane sharing its window with a
busy neighbour is never quiet by that measure. The pane-level signals the radar has
(screen-frame change, subtree CPU) need a poll every few seconds to mean anything — a
baseline older than 6s reads as "not working" — so using them to disbelieve a marker
would turn every long turn on a slowly polled serve into `idle`. The guard stays as it
is; the honest fix for a stuck marker is at the source, as here.

**Must-check when it recurs.** `gtmux events --all --since-seq <n> --json | grep '"%N"'`:
a `PostCompact` with `state:"working"` and no later `Stop` is this shape. If the trigger
field is present and `manual`, the hook is old; if it is absent, the agent's hook payload
changed.

## Wakes pasted into the HQ shell before the agent started (2026-09-16)

**Symptom.** `gtmux hq --here` (or any start of HQ into a pane) prints its lines, and
before the agent draws its first frame a wall of `» ◆ gtmux·goal-changed …` lines lands
on the bash prompt. The journal shows them as `gtmux:audit:wake-dropped … unconfirmed`,
seconds before the pane's `SessionStart`.

**Root cause.** The wake drainer's only gate was the draft guard (is the input box
empty?). A shell prompt has no input box, so the guard fails open, by design: its job is
to protect a send, never to block one. The pane was stamped as HQ the moment `gtmux hq`
chose it, the 3s tick found the stamp and an "empty box", and drained the backlog into
bash. Nothing could confirm those batches (no agent, no receipt), so they were dropped.

**Fix.** `hqnudge` now asks the pane's foreground command first (`agentUp`): a bare
shell holds every entry queued until the agent is up. An unknown foreground proceeds.

**Must-check.** After starting HQ into a pane with a backlog, `gtmux events --all
--since-seq <n> --json | grep wake-` should show `wake-delivered` records AFTER the
pane's `SessionStart`, and no `unconfirmed` drops in the seconds before it.

## Design history moved out of the CLI reference (2026-09-16)

`docs/cli.md` was rewritten as a plain reference: what each command does now and how to
use it. The paragraphs below were moved out of it because they explain why a rule exists
(an incident, a measurement, a rejected design) rather than what the rule is. Each
subsection opens with what the reference now says, then holds the moved text. Numbers
and dates are as they were.

**Current reference (2026-10-06):** the material below records the 2026-09-16
design history. For today's rules, use [the CLI reference](cli.md): reads from
subdirectories of the HQ home count too; `--severity`/`--acts`, skip-ahead and gap
reads do not. The owner HTTP knowledge door now offers `land`, `retire`, `carry`
and `withdraw`. `awake off` needs no password while the guard is installed; without
it, restoring sleep asks for local administrator authorization. These later rules
do not rewrite the incidents below.

### The consumption watermark

The reference says: the wake classes are priority labels; a consumption watermark
guarantees HQ hears about everything, and the `unread` knock reports what sits past it.

Every wake class is gtmux deciding an event is worth a knock, and that decision needs
context gtmux does not have: only HQ knows it is waiting on the pane that just finished.
So the classes are priority labels, not coverage; they say what to read first. What
guarantees HQ hears about something at all is the watermark. The everyday
pull-on-wake (an unfiltered `gtmux events --since-seq <n>` from the HQ home) is what
advances it, which is why the mechanism needed no new habit from HQ. A read that
detected a sequence gap does not advance it either: the warning would otherwise get one
chance to be seen before the loss was forgiven.

Three kinds of record are excluded from the count. HQ's own lines, or the channel would
feed itself forever. A pane-less lifecycle blink. And gtmux's own audit trail
(`gtmux:audit:*`), which documents acts HQ already knows about, so counting it would
mint fresh debt per delivered knock.

HQ's pull was then scoped to that same set. Measured on a week of one fleet, 68.7 % of
what a knock sent HQ to read was its own echo, so a knock about one new fact cost a whole
turn to read. The count and the read now name the same records; `--all` gets the raw
view back, and both still count as consumption (one is exactly what HQ owes, the other a
superset).

This is what makes perception complete rather than well-guessed. Before the watermark,
an event no class claimed simply never arrived: a session finishing work nobody had
dispatched through gtmux was neither `done` (no ledger entry) nor `asks` (no question),
so the turn-end sat in the stream, correct and unread, until the user asked in person.
`gtmux doctor`'s `event consumption` row exists because the failure is otherwise silent
in both directions. The journaled delivery outcomes (`gtmux:audit:wake-delivered`,
`gtmux:audit:wake-dropped`) mean "what was HQ told at 14:02, and what was it never told"
are `gtmux events --all` queries rather than reconstructions.

The subdirectory rule in `gtmux events` has its own history: a read from `notes/` or
`knowledge/` inside the HQ home (where HQ lands after writing its board) never counted,
and that used to be silent, so HQ read the delta, believed it had consumed, and watched
the same cursor re-knock. It now warns on stderr, naming the home to run from.

The grade glyphs (`◆` `▸` `·`) clear the same encoding bar as `»` and `│`: no emoji,
nothing outside the blocks the grammar already uses, because colour is an addition on
surfaces that own their rendering, never the only carrier.

### Self-rotation

The reference says: `gtmux serve` senses HQ's context fraction, session age and turn
count, knocks `self-rotate` when any crosses its line, and only a new agent session id
clears the debt.

The class exists because of a specific failure: in a long, near-full session HQ starts
to read its own output as input that came from outside. On 2026-08-03 one did exactly
that. It found a line saying "that message was from me, don't worry", took it for the
user's reassurance, and dropped a suspicion it had raised correctly. The line was its own
previous turn. The event stream is what settles it, because the two acts carry different
types: on HQ's pane `UserPromptSubmit` is you, `Stop` is HQ.

HQ cannot catch this itself (the faculty that would notice is the one that degraded)
and it cannot schedule the check either, since it is not running between wakes. So serve
watches from outside. The thresholds (`selfRotateCtx` 0.75, `selfRotateHours` 12,
`selfRotateTurns` 300) are deliberately conservative: a knock you don't believe is worse
than no knock.

The floor (`hqWake.selfRotateFloorSec`, 12 h) was added after a measured night: an age
breach, which can never recover, knocked every half hour forever. One night cost 17
knocks against a completely still fleet, with context climbing largely on the
answer-the-knock turns themselves. Past `selfRotateRepeatSec` a repeat now fires only
when the breach set or the fleet has changed.

### Where HQ runs, `--board` and `--home`

The reference says: `gtmux hq --board` prints the situation board; `gtmux hq --home`
prints the HQ home path.

`--board` exists so a surface can show HQ's synthesis without knowing where the HQ home
lives: that path is relocatable and is reached through a symlink on at least one real
machine, so resolving it belongs in the CLI rather than in every consumer. The menu-bar
app's board reader was the first caller.

`--home` serves the caller that has to act rather than read. It does not widen the cwd
gate by one inch: the verb is still judged by cwd exactly as before, and the path was
never a secret, since every refusal message prints it. A machine with no HQ home still
prints the path and exits non-zero, because the chdir that was about to happen would
have failed with less to go on.

### `gtmux capture` and the distill queue

The reference says: `capture` drops a one-line candidate into the pending-distill spool;
`--list` shows when the queue was last drained; five candidates pull the next distill
forward.

Writing a polished knowledge-base entry mid-work is expensive and gets skipped, so
`capture` decouples noticing (one line, in the moment) from writing it up well (batched,
at HQ's distill pass). It is public by design because opening the input is safe: the
distill pass is the quality gate, and the worst case is a candidate dropped at distill
time. This is layer ② of the capture loop; see
`openspec/changes/archive/2026-07-29-hq-capture-loop`.

`--list` heads the queue with the last-drained time because the depth alone can't tell
you whether the loop is alive: an empty queue reads the same whether distill drained it
yesterday or has never run. `--list --json` carries the dedup key and the text form
omits it, so a GUI could show the queue and never act on it; the key is the unit of
action, not the line.

### Knowledge base: migration and the phone door

The reference says: `gtmux knowledge` mutations are accepted only from the HQ home;
`gtmux serve` accepts `land` and `retire` from an owner-authenticated client; a
hand-written topic file is moved to `knowledge/legacy/<topic>.md` on the first mutation.

The phone door (change `hq-knowledge-on-phone`) is not a hole in the cwd rule: the cwd
gate keeps workers out of the quality gate, while this door is the commander, who
outranks HQ, and `land` in particular is a fact only they hold (HQ can judge a lesson
charter-level; only the person who carried it knows it arrived). `add`/`supersede`
stay off it because they carry prose.

Migration is incremental: the first mutation touching a topic moves its pre-ledger
hand-written file verbatim to `knowledge/legacy/<topic>.md` (an untouched seeded
placeholder is simply replaced), the render links to it, and the dispatch-time knowledge
echo consults both, so nothing loses reach while HQ migrates lessons by use.

The three axes (`kind` / `provenance` / `audience`) landed with change
`hq-knowledge-engine`; sensitive entries with `kb-sensitive-entries`; the `orphan-tool` /
`broken-tool` lint pairing with `kb-tools-in-knowledge`. The `sense` field on a digest
row came with `agent-drivers`, and the pane tiers behind `gtmux panes` with
`tiered-pane-control`.

### Spawn: the goal travels as a file

The reference says: anything longer than one short line goes through `--goal-file` /
`--message-file`.

The reason is structural, not stylistic: a goal passed as a command-line argument is
parsed by your shell before gtmux ever sees it. Inside `"…"` a backticked span is
executed and `$foo` is expanded; a newline inside the quotes is preserved, while an
unquoted newline ends the command. A goal placed in unsafe quoting and containing
`for f in *; do echo $f; done` dies with `command substitution: syntax error near
unexpected token 'done'` and dispatches nothing. Any sufficiently long natural-language
instruction eventually contains one of those characters, which is why "quote it
carefully each time" is not a property you can rely on. Passing both a file and a
positional goal is an error rather than a precedence rule.

### Spawn: re-run convergence

The reference says: re-running the identical `gtmux spawn` lands on one worktree, one
session, one ledger entry.

A spawn that died partway used to leave a worktree the retry then tripped over
(`exit status 128`) and an empty session per attempt. Now `--worktree` reuses a worktree
that already serves that branch; spawn adopts its own previous attempt (a ledger entry
that owns its session, never got its goal delivered, and still has a live pane) instead
of parking a second pane beside it; and a worktree or branch this invocation created is
rolled back when a step fails with nothing resumable.

### Spawn: the readiness gate and the standing notice

The reference says: a boot banner holds the gate, a standing notice does not, and a
timeout names the line that blocked it.

A standing notice such as `⚠ N MCP servers need authentication · run /mcp` used to hold
the gate, which made spawn impossible on any machine carrying one (see the MCP-banner
entry above). The gate now distinguishes chrome that resolves by waiting
(`Connecting…`, `Loading…`) from a notice that names an action only you can take. On
timeout the failure prints the pane's bottom region, not its whole scrollback.

### Spawn and send: how a landing is judged

The reference says: a hook-equipped agent's own `UserPromptSubmit` event is the receipt;
otherwise a two-frame screen read; `judged_by` says which.

The receipt rides the session-events stream introduced in #388. The event's recorded
head and the verifier's needle come from one shared normalization pipeline, so a genuine
submit event always matches. Arbitration is positive-monotonic: a stream-confirmed
landing is final and a screen read can never overturn it, and before any
`delivered:false` the stream is re-read once more so a confirmation arriving at the
deadline is never lost to the timeout. `judged_by` exists so a misjudgment can be
attributed instead of reconstructed from timelines.

### Send: no box-confirm on a plain shell

The reference says: a plain terminal pane is typed into directly, with no input-box
confirm and no re-send interlock.

There is no agent composer to verify against, and running the box-confirm against a
shell false-failed whenever stale box-drawing sat in the pane's scrollback (a pane that
previously ran an agent). Running the same shell command twice is normal usage, not a
double-dispatch, so the interlock is off there too.

### Tasks: `undelivered` and the pending plate

The reference says: `undelivered` leads `gtmux tasks`; `--pending` prints a stable,
absolute-stamped plate.

A dispatch that dies at the ready gate leaves a live, empty, idle agent pane,
indistinguishable from one that just finished a turn, so a status derived from the pane
alone rendered a task that never started as green `done`, with the goal you intended
printed beside it. The ledger's delivery verdict now wins. A `gtmux send` that lands the
same goal closes the record so the workaround does not leave a permanently wrong row.

`--pending` reads the ledger only and prints an absolute stamp because two reads of an
unchanged plate must be byte-identical: that is what lets a brief point at it
("everything else as before") instead of re-printing the list every time.
`gtmux reap` names every failed step under `⚠ but these steps failed` because a branch that survived a reap must
never be left to be inferred from a line that isn't printed.

### Usage: two log shapes

The reference says: Claude totals are a running sum, Codex totals are the last reading.

Codex records the session's running totals after every turn, so summing those would
multiply a session's burn by its turn count. Codex is the better-informed of the two in
one respect: it states `model_context_window` outright (258,400 on a live session). The
daily ledger (change `usage-daily-totals`) attributes each message to the day it
happened, so a session three weeks old no longer reads as this week's spend. An agent
whose log carries no usage still gets a row with those fields empty, the same
degradation as before.

### Events: `--acts`

The reference says: `--acts` keeps the supervision's own acts and drops the wake plumbing.

Measured on a real machine the plumbing outnumbers the acts about forty to one, so "what
did HQ do today" is `gtmux events --since 24h --acts`, not a scroll.

### Limits: the footer, the prefix and the dropped window

The reference says: `gtmux limits` lists every window; every other place shows one per
plan, the tightest; every window says whose plan it is; a window whose reset has passed
is dropped, and a Codex with no readable window but recent use gets its own line.

`gtmux usage`'s footer ran past 100 characters once Codex added its own windows, and the
phone's header row truncated mid-number ("Fable 11…"), which is the one thing a
percentage must never do. The tightest is the right one to keep, since the question a
summary answers is "where do I stand".

An unprefixed label next to a prefixed one (`session` beside `codex session`) reads as
the general case with a special case beside it, which is the opposite of true, so the
first agent's windows are prefixed too.

A dropped window's percentage is unknown, not low. An agent that reports through its log
goes quiet on its own the moment you stop using it, which looks exactly like gtmux having
broken; hence the `○ codex …` line when Codex was used in the past week. An operator who
does not run Codex should not be told about Codex. The cache file is
`state/limits.json`.

### Awake: the asymmetry and the state table

The reference says: on costs a password, off costs nothing; a root-owned guard restores
sleep; the state is read from `ioreg`.

Before this feature, "command your Mac from your phone" only worked while the lid stayed
open. The command shipped as `gtmux server-mode` in v0.44.0; the feature kept the name,
the command got shorter.

That asymmetry is the whole design: escalation is local, interactive and deliberately
visible for as long as it lasts; de-escalation is free, automatic and always possible,
including when gtmux is dead, which is exactly when it matters most. Battery is a
supported case, not a hazard: measured unplugged, lid shut, zero sleeps.

Where the state is read from is the subtle part, and getting it wrong is the feature's
worst failure (announcing "sleep restored" on a Mac that cannot sleep): `pmset -g`
never reports `disablesleep` in either state, the power-management plist lags a write,
and only `ioreg -r -c IOPMrootDomain` → `SleepDisabled` is the live truth. The two
boundaries (a per-user LaunchAgent cannot survive an unattended FileVault reboot; the
setting is undocumented by Apple) are stated rather than papered over. The phone can see
the state but not change it: a remote switch that could only turn it off would send you
back to the laptop anyway.

### Restore: phantom agents and silent layout failures

The reference says: one restore at a time; only panes that were running an agent at save
time get one back; the layout that came back is compared with the save.

The lock exists because a restore opens the whole working set, so two in parallel open
it twice, and the way people trigger a second one is the way slow actions invite: asking
again because nothing visibly happened. `--plan` and `--dry-run` are exempt because
refusing to show the plan while a restore runs would be the guard blocking the answer.

Which panes get an agent back is read from the save's own record of each pane's command,
not from "an agent lived here once". It used to come back with `claude --resume …` typed
into it: the resume records are written by the agent's hooks and never pruned, so every
pane that had ever hosted a conversation was a permanent target. One reboot turned 10
live agent panes into 16 (the 2026-08-04 entry above).

The save's timestamp is printed because the thing that quietly costs work is not an
ancient save, it is a recent-looking one. `gtmux doctor`'s `resurrect autosave` row flags
an armed trigger that has not saved for hours rather than calling it OK.

The post-restore comparison matters because both halves used to be silent: gtmux only
counted session names, and tmux-resurrect discards its own layout errors. So when a
window came back with one pane more than the save recorded, tmux refused to apply the
layout ("have 3 panes but need 2") and that window silently kept a default stacked
arrangement with nothing, anywhere, saying so.

### Focus: a session with no window

The reference says: `focus` on a session with no window open opens a tab and attaches.

It used to select the pane inside tmux and then search for a tab that could not exist,
which looked exactly like a broken jump. The test is the client count
(`session_attached`), not how the session was started.

### Pane ids in tab titles

The reference says: `#{P:…}` lists every pane in the window; the `pane-exited` hook is
required.

An active-pane id was the first design and measurement killed it: tmux re-evaluates the
format on its own schedule, so the name lagged the real active pane, a stale pointer
dressed as a live one. Without the hook, a tab keeps advertising a pane that is gone,
which is worse than showing no ids at all. `pane-border-status` is not suggested by
`doctor` because a permanent screen row per pane in every split is a real price for
something the window name already carries.

### Tab alert, share status, whatsnew

`tab-alert` marks only `waiting` for the same reason red is reserved for decisions: a
marker that appears on most tabs most of the time is one nobody reads. It is a glyph and
not a colour because a coloured tab would not travel across terminals. HQ is not in the
loop; the marker is a mechanical projection, not a judgment.

`share status --json` reports `last_seen` / `platform` / `last_ip`, all absent until
someone has used the link, which is itself the answer. A link is a standing grant handed
to someone else, so "has anyone walked through it, and from where" is the question worth
answering; what it permits is already on the row above.

A release with no `user:` block contributes nothing to `whatsnew`, deliberately: a
version where nothing changed for users should say nothing rather than paraphrase a
commit subject into developer vocabulary.

### HQ records: the export format and the import rule

The reference says: the export is an age-locked tar.gz; `--import` moves the existing
records aside; daily snapshots live outside the HQ home.

"Records" is what "memory" used to name (`--memory` remains as an alias). The export is
locked in the age format and not a format of gtmux's own, because the case it exists for
is the case where gtmux may not be there to read it back. `--import` never overwrites in
place because restoring is done in a hurry and usually on the wrong assumption, and it
must never turn "I restored last week's board" into "and I destroyed today's".
Snapshots live beside the state and not inside the HQ home because the likeliest loss is
that directory going away, and a backup stored inside it goes too. A `gtmux quiet`
setting can never silence a read-time gap because a setting that could would make every
other reading untrustworthy.

### Smaller rationale trimmed in the same pass

The grade glyph on a wake line is a projection of the class, not a second opinion about
it: what fires and at what severity is decided elsewhere, and the grade only says how
loudly it should read. Standing classes ask two questions before each repeat that a
first knock never has to (does the premise still hold, and has anything changed since I
last said this), and a queued wake whose premise died while it waited is dropped rather
than delivered as a claim about a world that has moved on. `notes/board.md` and the
knowledge base are brought current before a rotation because they are the successor's
entire briefing.

Codex needs no full scan and no counter file for `gtmux usage`, since its log carries
running totals. Codex's own `/usage` is an activity heatmap rather than remaining quota,
so the `claude -p /usage` command route has no counterpart there.

`gtmux awake`'s pulsing red dot exists because the risk this feature actually has is
being forgotten. The FileVault reboot boundary is the right fail-safe. The report-only
discipline for a `disablesleep` gtmux did not stamp is the same one `gtmux reap` applies
to an unclean worktree.

`gtmux tasks --pending` reads the ledger only (no radar scan). The `undelivered` status
is the one the pane cannot tell you. A restore's plan is printed up front so you see what
is being restored as it happens and have a review checklist afterward; the doctor row
for the autosave flags a stale trigger rather than calling it OK.

`gtmux send` on a plain shell runs no interlock because running the same shell command
twice is normal usage, not a double-dispatch. The draft guard's job is to protect a send,
never to block one, which is why everything it cannot judge lets the send through, and
why it costs at most two reads and one poll interval: it can slow a send by a known
amount but cannot hang one.

`adopt` resumes by session id because that is the only way a sensed native session can
become a full row; agents whose CLI cannot resume by id are listed and left alone rather
than half-adopted. `panes` is a separate command because `gtmux agents --json` is a
locked contract meaning "coding agents", and a browser that reaches any pane needs the
superset; the agent radar is not diluted, a plain pane appears only on opt-in.

## The phone build breaks overnight after Xcode updates itself (2026-09-17)

**Symptom.** A device build that worked the evening before fails with `** BUILD FAILED **`
and, near the end of the log, `Pods.xcodeproj: error: The iOS deployment target
'IPHONEOS_DEPLOYMENT_TARGET' is set to 13.0, but the range of supported deployment target
versions is 15.0 to 27.0.x` for a few pod targets (AsyncStorage resources 13.0, RNSVG
filters 12.4, image-picker privacy info 9.0). Earlier in the same log: `CoreSimulator is out
of date` and `No locator class for device extension 'Xcode.Device.CoreDevice'`.

**Root cause.** The App Store updated Xcode to 27.0 at 18:10, three minutes after the last
good build. Xcode 27 turns a pod deployment target below iOS 15 from a warning into an
error, and a few pods still declare old floors. The CoreSimulator/CoreDevice lines mean
Xcode's additional system components were not installed after the update; they did not stop
the device build (installing goes through `ideviceinstaller`, not Xcode).

**Fix.** The Podfile's `post_install` raises every pod target's deployment target to the
app's own floor (15.1), then `pod install`. On this machine `pod` lives in Homebrew Ruby's gem
bin, which `arch` does not search, and it needs a UTF-8 locale:

```sh
cd mobileapp/ios
export LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 PATH="/opt/homebrew/lib/ruby/gems/4.0.0/bin:$PATH"
arch -arm64 env PATH="$PATH" pod install
```

(Superseded 2026-10-03: run `bundle exec pod install` under the rbenv Ruby from
`mobileapp/.ruby-version` — see "Setting up a Mac to build the phone app".)

**Must-check.** `xcodebuild -version` before blaming the code, and
`grep -o 'IPHONEOS_DEPLOYMENT_TARGET = [0-9.]*' Pods/Pods.xcodeproj/project.pbxproj | sort | uniq -c`
should show nothing below 15.1. For simulator builds, run `xcodebuild -runFirstLaunch` once
(needs an admin password) so the CoreSimulator version matches the new Xcode.

## Installing a Direct server onto a box that already serves a site (2026-09-23)

**Symptom.** `install-server.sh` with `FRONT=nginx` ended three times with
`the gtmux site was REMOVED again and nginx left as it was`, each time for a different
reason, and one of those reasons was a file that had already been fixed.

**Root causes, in the order they appeared.**

1. `unknown directive "http2"`. `http2 on;` is nginx 1.25 syntax; Ubuntu 24.04 ships 1.24 and
   refuses the whole file. The template no longer enables HTTP/2 at all: it installs into
   whatever nginx a box already runs, and the tunnel is one WebSocket, which is HTTP/1.1
   either way.
2. `pcre2_compile() failed: missing closing parenthesis`, with the pattern cut off mid-way.
   An unquoted `{` in a location regex starts a config BLOCK, so `[0-9]{4}` ends the
   directive. Location regexes with a repetition count must be quoted.
3. The fix for (1) was in the repo and the box kept failing on it. `scp -r deploy/self-tunnel
   root@host:/tmp/gtmux-self-tunnel` when that directory ALREADY EXISTS copies INTO it, so the
   box ran the old `/tmp/gtmux-self-tunnel/install-server.sh` while the new one sat in
   `/tmp/gtmux-self-tunnel/self-tunnel/`.

**Must-check.** After copying, `grep` the file ON THE BOX for the thing you just changed
before re-running, or `rm -rf` the destination first. And when a config-time failure repeats
unchanged, suspect the copy before the fix.

**What worked as intended.** `nginx -t` caught all three before any reload, the installer
withdrew its own site each time, and the site already on that box served without interruption
throughout.

## An attachment over 1 MB never arrives, and the phone's send button spins forever

**Symptom.** You attach a file in the phone's Chat composer and tap send. The send button
turns into a spinner and stays there. Small photos work. A PDF or anything over a megabyte
does not, and the composer never comes back, so the message cannot be sent or cancelled.

**Root cause, two layers.** nginx caps a request body at 1 MB unless `client_max_body_size`
says otherwise, and the Direct tunnel's site config did not say otherwise, while serve
itself accepts 30 MB. Every attachment over a megabyte earned a `413`. A small one got that
`413` back and failed cleanly; a big one had its connection dropped while the body was
still going out, which fires neither `onload` nor `onerror` on an XMLHttpRequest. The
phone's upload promise never settled, so the composer's `sending` flag never reset.

**Must-check.**
- `curl -s -o /dev/null -w '%{http_code}' -X POST -H "Authorization: Bearer $TOKEN" -F "file=@<a 2MB file>" "$TUNNEL_URL/api/upload"` — a `413` means the front end, not serve.
- On the tunnel host: `nginx -T | grep client_max_body_size`. Nothing printed means the 1 MB default is in force.
- Straight at the Mac (`http://127.0.0.1:8765/api/upload`) the same file returns `200`, which is how you tell the two layers apart.

**Fixed.** `deploy/self-tunnel/nginx-site.conf` sets `client_max_body_size 32m`, and the
phone's upload gives up on its own when an upload stops moving (20s while the body is going
out, 60s while waiting for the Mac to answer), so a dropped connection can no longer wedge
the composer. An existing tunnel host does NOT pick the config up by itself: copy the site
file, `nginx -t`, then `systemctl reload nginx`.


## A device IPA fails installation on `Payload/._gtmux.app`

**Symptom (2026-10-02).** `ideviceinstaller` copies the IPA successfully, then returns
`APIInternalError` / `IXErrorDomain Code=10`: it cannot create a CFBundle from
`Extracted/Payload/._gtmux.app`. The previous app remains installed. This happened
with a local 1.0.71 (23) device package; compiling and code-signing had succeeded.

**Root cause.** The local `ditto -c -k --keepParent Payload` packaging step included
macOS AppleDouble metadata. The archive contained 103 entries with `._` path
components, including the extra `._gtmux.app` next to the actual application. The
installation service tried to treat that sidecar as another app bundle. This is
a packaging failure, not a locked-phone or developer-disk-image problem.

**Must-check.** Inspect ZIP entry names for `__MACOSX` or components starting with
`._` before installing a locally assembled IPA. Prefer the repository's documented
installation of the built `.app` directory, or package an IPA without resource-fork
sidecars. Keep the actual app files, `_CodeSignature`, provisioning profile and
extensions intact. Verify the parent and both extensions' versions, then query
`ideviceinstaller list -b com.gtmux.app --xml` after installation; a successful
copy alone is not evidence that the app was updated.

**Verified recovery.** Repacked the same IPA, preserving every non-metadata entry
and its file bytes, while excluding those sidecars. Installation reached
`InstallComplete (100%)`; a separate device query returned 1.0.71 (23), replacing
1.0.67 (1). No device reset, app uninstall or signing change was needed. This proves
installation, not physical accessibility or layout acceptance.

## Setting up a Mac to build the phone app (2026-10-03)

**Symptom.** Getting the Intel Home MacBook to build and install the phone app hit four
walls in a row. `brew install` refused with `Xcode alone is not sufficient on Tahoe. Install
the Command Line Tools`. `rbenv install 4.0.7` failed in `ext/psych` with `yaml.h not found`.
`xcodebuild … -destination 'generic/platform=iOS'` refused every destination with `iOS 26.2 is
not installed. Please download and install the platform from Xcode > Settings > Components`,
although `xcodebuild -showsdks` listed the `iphoneos26.2` SDK. And `Podfile.lock` said
`COCOAPODS: 1.17.0` while `Gemfile.lock` pinned cocoapods 1.15.2, so `bundle exec pod
install` rewrote the lock on every run.

**Root cause.** Four separate gaps:

1. Homebrew no longer bottles for Intel, and on Tahoe it requires the Command Line Tools
   for anything it builds; a full Xcode does not count.
2. ruby-build compiles Ruby against Homebrew's libyaml and does not fetch it itself.
3. Xcode 26 checks for the iOS *platform* component (the simulator runtime) before it accepts
   any iOS destination, a generic device build included. The SDK alone is not enough.
4. The other Mac ran a global `pod` 1.17.0, which Bundler could not reach: the Gemfile still
   carried the React Native template's `xcodeproj < 1.26.0` cap, and cocoapods 1.17 needs
   xcodeproj >= 1.28.1. Two Macs, two CocoaPods versions, one lock file.

**Fix (once per Mac).**

```sh
xcode-select --install                     # Command Line Tools; a GUI prompt
brew install rbenv ruby-build libyaml openssl@3
#   (Intel: Homebrew's rbenv works, but a git checkout of rbenv + ruby-build does not
#    depend on Homebrew's support for the platform; both read .ruby-version)
rbenv install "$(cat mobileapp/.ruby-version)"
rbenv global  "$(cat mobileapp/.ruby-version)"
cd mobileapp && bundle install && (cd ios && bundle exec pod install)
xcodebuild -downloadPlatform iOS           # ~10 GB, again after each Xcode major
```

The Gemfile now locks cocoapods 1.17 (xcodeproj cap removed), and `mobileapp/.ruby-version`
pins the Ruby, so every Mac resolves the same tools.

**Must-check.** After `bundle exec pod install`, `git diff mobileapp/ios/Podfile.lock` must be
empty except possibly the `hermes-engine:` checksum: that podspec embeds the absolute path of
`node_modules/hermes-compiler/.../hermesc`, so a checkout at any other path (a scratch
worktree) gets a different checksum. Don't commit that line from such a checkout.

**Verified on this setup.** A Release device build (`generic/platform=iOS`) with the rbenv Ruby
4.0.7 first on PATH ran every CocoaPods/React Native script phase cleanly; the RubyGems
`GemParser` NameError that `fastlane/Fastfile`'s FOOT-GUN 3 works around (Homebrew Ruby 4.0.2)
did not reproduce. The fastlane archive path itself was not re-run. Without
`ideviceinstaller`, `xcrun devicectl device install app --device <id> <app>` installs while the
phone is unlocked.

## The phone's terminal shows only one screen of a Claude session (2026-10-03)

**Symptom.** The Terminal tab for a Claude Code pane suddenly had almost no history: scrolling
up stopped after one screen. `tmux list-panes -a -F '#{pane_id} #{pane_current_command}
alt=#{alternate_on} history=#{history_size}'` showed every Claude pane at `alt=1` with 0–19
lines of history. Codex panes had always looked like this.

**Root cause.** Claude Code 2.1.285 has a fullscreen renderer that runs in the ALTERNATE screen
and virtualizes its own scrollback, so tmux holds one screen. When `tui` is unset, Claude picks
it by itself on a fresh install (`fresh_install_on`). A reinstall reset `~/.claude/settings.json`
that afternoon, so every Claude session started after it came up fullscreen. Not a gtmux
regression, and nothing in gtmux said why until the doctor row below.

**Fix.** `"tui": "default"` in `~/.claude/settings.json` (or `gtmux doctor --fix`, which writes
it after a backup). Running sessions keep their renderer until restarted, or until `/tui default`
is typed in each (it resumes the conversation). History a fullscreen session never gave tmux is
gone; it does not come back.

**Must-check.** `gtmux doctor` → "Claude Code renderer". A Claude pane at `alt=1` while that row
says classic is most likely a session started before the setting (a project or managed setting,
or the env it was launched with, can also do it). Env beats `tui`: a true
`CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN` forces classic; `CLAUDE_CODE_NO_FLICKER` forces fullscreen
when true and classic when false. Claude reads booleans as `1/true/yes/on` and `0/false/no/off`.
`--fix` backs the file up to `settings.json.gtmux-tui.bak` and stops if it cannot.

## macOS keeps asking for Photos, Music, Calendar… in gtmux's name (2026-10-04)

**Symptom.** Every so often a system prompt says gtmux (or Gtmux) would like to access
Photos, Music, Downloads, Desktop, Documents, Calendar, Contacts or data from other apps,
with nothing on screen to explain why. It reads as gtmux rummaging through the Mac.

**Root cause.** The plan-limits probe, `claude -p /usage`, starts a real Claude Code
session every 15 minutes, and it ran in the caller's working directory: `/` for both
`gtmux serve` (a LaunchAgent with no WorkingDirectory) and the menu-bar app. Claude Code
looks through the directory it starts in, so from `/` it reached the protected folders.
macOS attributes a child's access to the process that started it, so every prompt named
gtmux. Measured: the TCC log held 44 such `claude` runs in three days, 15 minutes apart,
responsible = `gtmux serve` or `com.gtmux.menubar`. The CLI's unstable code identity makes
macOS forget a "Don't Allow" at each update, so the prompts came back.

**Fix.** The probe runs in an empty `~/.local/share/gtmux/probe` (`internal/limits`).

**Must-check.** `log show --last 1h --predicate 'subsystem == "com.apple.TCC" AND
eventMessage CONTAINS "AUTHREQ_ATTRIBUTION"' | grep claude-code | grep gtmux` is empty after
a few probe cycles. Anything gtmux runs without a pane has to choose its working directory:
`/` is the whole disk.

## The screenshot hotkey asks for Screen Recording again after a rebuild (2026-10-04)

**Symptom.** ⌥⌘4 shows the "Turn on Screen Recording" alert although Gtmux was allowed
before, or the capture shows only the desktop picture.

**Root cause.** Screen Recording is granted to the app's code identity. An ad-hoc build
(`make app` without `GTMUX_SIGN_ID`) gets a new identity on every build, so the grant does not
carry over. The selector freezes the displays with ScreenCaptureKit inside the app, and falls
back to `screencapture -i` (a child attributed to Gtmux) when that fails; both need the same
grant. macOS often applies a new grant only after the app is reopened. A fallback is logged
as `act.screenshot.freeze` (`gtmux logs --event act.screenshot.freeze`).

**Fix.** Turn Gtmux on again in System Settings → Privacy & Security → Screen & System Audio
Recording, then quit and reopen Gtmux. A Developer ID build keeps the grant across updates.

**Must-check.** The editor never opens without the grant: the app checks
`CGPreflightScreenCaptureAccess` before every capture.

## Codex deliveries confirm only by screen; HQ wakes "dropped" that had arrived (2026-10-04)

**Symptom.** `gtmux send` to a Codex pane says `failed` although Codex answered it, and
`gtmux doctor` lists `act.wake.dropped … unconfirmed` for wakes HQ (on Codex) did receive —
each of them three times. Codex `UserPromptSubmit` events in the journal have no session id,
cwd or summary.

**Root cause.** Three things. (1) A Codex hook re-execs itself detached and exits at once;
its payload went through os/exec's stdin copy goroutine, which died with it, so the worker
read nothing and no Codex delivery ever had its receipt (11 of 413 carried a prompt).
(2) Without a receipt the screen decides, and a long Chinese prompt wrapped in a narrow pane
never matched the 40-rune head. (3) On HQ's screen, a Claude input box that HQ had printed
from a captured pane was taken for HQ's own input box, so the wake id was in neither region.

**Fix.** The payload rides in an unlinked temp file; the history match is wrap-tolerant; a
box with a prompt line below it is transcript; a requeued wake is checked again before it is
re-pasted.

**Must-check.** `gtmux logs --component hook` shows `hook.payload.empty` if a detached worker
ever reads nothing again. A Codex `UserPromptSubmit` in `gtmux events --json` should carry a
session id and a summary.

## The tunnel dies with one proxy node, and serve calls it back up every minute (2026-10-05)

**Symptom:** the phone cannot reach the Mac; `gtmux serve` is healthy. HQ is woken with
`tunnel up` ("connected 20s ago") and `tunnel down` 20–40s later, over and over.
`cloudflared.stderr` loops `TLS handshake with edge error: EOF` against `172.19.x`
addresses and has no `Registered tunnel connection` line since the outage began.

**Root causes (two):**
- **One rule.** Under a TUN proxy (Clash Verge / mihomo here) every connection
  cloudflared makes to the edge goes through the proxy's rules. On 2026-10-03 a Codex
  session fixing the tunnel had prepended `argotunnel.com`, `cftunnel.com` and the
  tunnel host to the user's Clash rules, pinned to ONE node. The user did not remember
  them. When that node broke, the tunnel broke with it, while everything else (on
  another node) kept working.
- **The probe.** serve read cloudflared's `cloudflared_tunnel_ha_connections` gauge. It
  counts a connection while its handshake is in flight: 2 for the ~5s each doomed
  handshake took, so every retry read as "up". It now reads `/ready`
  (`readyConnections` = connections the edge has registered).

**Must-check:**
- `curl 127.0.0.1:49317/ready`: `readyConnections` 0 means not connected, whatever
  the gauge says.
- Ask the proxy which rule takes the edge dial. Connections live milliseconds, so
  polling `/connections` misses them; stream mihomo's log instead
  (`/logs?level=debug` on its controller socket) while you probe
  `h2.cftunnel.com:7844`. Look for `dial <node> (match …argotunnel.com)`.
- Compare with a direct path (`curl --interface en0 --resolve
  h2.cftunnel.com:7844:198.41.192.27 -k https://h2.cftunnel.com:7844/`). A completed
  TLS handshake there means the edge is reachable and the proxy path is what fails.
- `PROCESS-NAME` rules do nothing when the proxy's process lookup is off (every
  connection's `process` is empty). Route by domain.
- The proxy's config is the user's. An agent changes it only on their explicit word,
  with a backup kept outside `/tmp` (the 2026-10-03 backups were in `/private/tmp` and
  gone by the time they were needed).
