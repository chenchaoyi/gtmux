# Desktop conversation follow

**English** · [中文](desktop-follow.zh.md)

Detecting a ChatGPT desktop Codex conversation makes its status visible. It does not
ask HQ to supervise it. This decision belongs to the user, per conversation.

## Permissions

| Mode / choice | Effect |
|---|---|
| Status only (default) | Separate Desktop apps section; no managed count, HQ unread/digest, push, Live Activity or content mining |
| HQ follow | Read the conversation, analyze and report; include lifecycle events after the next Unix second in HQ attention |
| Notifications (initially off) | Allow this conversation's future waiting/completion notifications; continue in ChatGPT desktop |
| Knowledge capture (initially off) | Mine reusable experience from activity after consent; HQ still verifies candidates before adding knowledge |
| Stop following | End observation, disable both choices; retain existing records and knowledge |

Follow does not give gtmux a desktop input, approval, focus or adoption capability.
The policy applies only to native Codex rows verified by the rollout's exact
`session_meta.originator` (`codex_work_desktop` / `Codex Desktop`), never the directory,
thread title or rollout `source`. Terminal Codex and other agents are unchanged.

## Authority and storage

`internal/sessionpolicy` owns the policy; the UIs consume it. Each document under
`~/.local/share/gtmux/session-follow/` has the verified session ID, three permissions,
consent timestamps, a revision and a distinct knowledge-interval revision. Filenames hash the agent/ID, directories are private
and writes are locked, synced and atomically renamed. Missing policy is status-only;
unreadable/corrupt state is fail-closed with diagnostics, and a failed save returns an
error. Saves compare the read revision; a stale device receives a conflict.

The CLI `gtmux follow`, menu-bar utility window and paired phone/iPad all change the
same document. The owner HTTP endpoint refuses guest links before reaching the writer.
Successful settings changes append a metadata-only `gtmux:audit:session-follow` receipt.
Policy lives outside HQ archives: new-Mac migration must not re-enable permissions for
old-Mac conversations. HQ is instructed not to opt in conversations on its own.

## Collection boundaries

- Radar reads identity/status metadata to keep detection working. Default desktop rows
  are excluded before digest's conversation-content reader or usage summary is invoked.
- Lifecycle records stamp the verified client and `knowledge_allowed`; raw metadata is
  retained for diagnosis. HQ debt, default HQ pulls, attention/self-check and HQ history
  share the observation gate. Explicit `events --all` remains a diagnostic view.
- The hub excludes status-only rows from managed tally and alerts. Enabling a setting
  does not replay the current waiting/done state. Notification delivery rechecks policy
  so queued work respects revocation; native alerts have session-specific collapse IDs.
- Mining checks the metadata header before opening the conversation reader. Knowledge
  consent starts at the next Unix second, conservatively excluding activity in the save's
  current second. Every content record, including shell errors, is time-fenced. A changed
  consent interval resets offset/carry (even for a same-second stop/re-enable) so no previous assistant context leaks into a
  new candidate. Incremental carry retains identity; older Codex ledgers recover identity
  from their metadata header without replaying old messages. Codex records without an ordinal
  use file identity and byte offset for stable, distinct provenance IDs.
- Existing pending candidates and filed knowledge are retained, not deleted or silently
  rewritten. A current observation-only event is not permission to file its content.

## Surfaces and verification

CLI and Web offer diagnostic visibility; CLI can edit. Web has a separate desktop section
and policy badge, without a guest edit control. Mac, phone and iPad offer a per-conversation
form with the saved state, explicit choices, one context-sensitive Save action and error/
conflict feedback. On iOS, the row menu finishes its native dismissal before opening
the settings form; changing conversations cancels a pending handoff. iPad shares the phone component, bounded in width and safe-area-aware.

Desktop-only launches still show the desktop list and follow controls; list visibility is
independent of managed counts. Header status counts exclude status-only desktop rows.

Regression coverage: `internal/sessionpolicy/policy_test.go` (default, per-ID, revocation,
conflict, malformed/I/O failures), `internal/radar/desktop_policy_test.go` (wire policy and
HQ identity), HQ debt/pull tests, server owner/guest and hub tests, mining consent tests,
Swift `SessionFollowTests`, and mobile `SessionFollowSheet.test.tsx` (drafts, save, conflict,
double taps and late responses). Device VoiceOver and real phone/iPad layout acceptance
requires connected unlocked devices; automated component tests do not claim that acceptance.

## Build gate note

PR CI selected Go 1.27.2, but pinned Staticcheck v0.8.1 could not decode its version-5
export data. `make lint` therefore runs only that analyzer with Go 1.27.1; CI calls
the same target. Builds, race tests and vulnerability scanning still use the current
CI Go patch. Revisit the analysis pin when Staticcheck supports the new format.
The toolchain selection uses the documented [GOTOOLCHAIN mechanism](https://go.dev/doc/toolchain).
