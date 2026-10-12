# Codex Desktop and CLI lifecycle

**English** · [中文](codex-client-lifecycle.zh.md)

**Proposed; researched on 2026-10-12 against Codex 0.162.1.** This is a design and
implementation plan, not a shipped handoff. Current behavior remains documented in
[Desktop follow](desktop-follow.md), [conversation reading](desktop-conversation.md)
and the capability specs. Implementation is tracked in
`openspec/changes/codex-client-lifecycle/`.

## 1. Findings and evidence

| Evidence | Established conclusion | Boundary |
|---|---|---|
| Live tmux CLI conversation appears in Desktop with an external-app lock | Desktop visibility is compatible with a CLI owning the conversation | The screenshot does not identify the backend or expose a transfer API |
| Reaped HQ worker remains in Desktop history with an enabled composer | Reaping a tmux resource does not delete the conversation | A composer alone does not prove a running turn or fresh writer |
| Worker has an early pane-less SessionStart, verified tmux binding, Stop and successful reap; the old PID-less native record remains unchanged | A provisional record was suppressed during pane life, then resurfaced after reap | Confirmed gtmux lifecycle defect, separate from Desktop retaining history |
| Local generated schema describes `originator` as creation metadata | Creation source cannot reliably identify the current client | Do not substitute cwd, title or rollout `source` |
| `WriterLockCoordinator::acquire` rejects competing process writers in one Codex home | Cross-process storage write exclusion exists | A lock filename alone does not prove it is locked or name the frontend |
| `resume_running_thread` rejoins a loaded thread and attaches a connection | Shared-server rejoin differs from acquiring a writer in another process | Storage exclusion does not establish one exclusive frontend |
| Current CLI owns an alternate screen; normal tmux history is frozen and current grid has 48 rows | Terminal frames and stored conversation history are different data | Older normal-buffer output cannot restore overwritten frames |

The official [app-server guide](https://learn.chatgpt.com/docs/app-server) distinguishes
stored reads, runtime status, resume, fork, unsubscribe, archive and delete.
[Desktop commands](https://learn.chatgpt.com/docs/app/commands) document
`codex://threads/<thread-id>` to open a local conversation. Neither establishes a gtmux
transfer command. No real resume, turn, fork, lock acquisition, archive, deletion or
client shutdown was performed in this research.

Version-specific implementation evidence inspected in official source tag `rust-v0.162.1`:

- `codex-rs/rollout/src/writer_lock.rs` and `writer_lock_tests.rs`: competing owners,
  release and stale unlocked files.
- `codex-rs/thread-store/src/local/live_writer.rs`: create/resume acquire a writer;
  shutdown finishes recorder work and removes the live recorder.
- `codex-rs/app-server/src/request_processors/thread_processor.rs`:
  `thread_resume_inner` and `resume_running_thread`; loaded rejoin preserves settings
  when conflicting overrides cannot safely rebuild the thread.
- `codex-rs/app-server/tests/suite/v2/thread_resume.rs`: separate-process writer
  refusal for legacy and paginated history, then resume after shutdown.
- `codex-rs/tui/src/app/tests/external_writer_fork_tests.rs`: a fork gets another ID,
  preserves the draft and leaves the original writer untouched.
- `codex-rs/tui/src/daemon_startup.rs`: shared-daemon and embedded launch paths,
  compatibility checks and exclusions. Shared storage does not prove shared backend.

These source tests were read, not executed. Static schema generation succeeded with
`codex app-server generate-json-schema --experimental --out <private-directory>`.
Real IDs, prompts, logs and screenshots stay outside the public repository.

## 2. Separate the identities

Conversation key: `(gtmux server identity, Codex home identity, thread.id)`. Home identity
is an opaque local identifier, not a path exposed to guests. Backend identity belongs
to runtime attachments: restarting a daemon must not duplicate history. Codex's
`thread.sessionId` groups a session tree; forks may share that root but remain distinct
threads. Existing gtmux `session_id` retains its verified thread-ID meaning through an
explicit compatibility adapter, never silently becoming the tree-root field.

| Object | Meaning | Must not stand in for |
|---|---|---|
| Conversation | Persisted ID, title, parent/fork relationship, history revision | Living client |
| Creation source | Original originator/source | Current controller or permission |
| Attachment | Backend, known frontend, exact process identity, pane, observation time | Durable conversation ID |
| Storage writer | Backend holding the writer lock | Single frontend or active turn |
| Controller | Verified input route and authority | Client guessed from originator |
| Turn | Turn ID, working/waiting/completed/failed outcome | Writer ownership or liveness |
| HQ task | Dispatch ID, resource ownership, role and authority scope | Permanent thread property |
| Follow policy | Observation, notification and knowledge grants | Input or takeover permission |

Idle writers can remain alive; loaded threads can be cache entries; saved conversations
can have neither. Missing evidence is **unknown**, not completed. `canAcceptDirectInput`
is a backend capability, including parent-owned subagent restrictions, not proof of this
client's input ownership. Process checks must distinguish backend/frontend and PID reuse.

## 3. Expected lifecycle

| Situation | Radar and action | History and HQ |
|---|---|---|
| Live tmux CLI also viewed in Desktop | One conversation with its verified pane; focus/send use that pane | External-app lock is expected; reading does not transfer control |
| Live native terminal CLI | Requires credible attachment evidence; moving requires a separate ownership check | Saved log enables reading, not indefinite liveness |
| CLI-created thread continues in Desktop | Controller is Desktop if verified, otherwise unknown; no unsafe move | Apply Desktop consent to new activity; creation source stays CLI |
| HQ worker successfully reaped | End dispatch resource attachment; retire only obsolete provisional native evidence | Retain thread; no automatic archive/delete |
| User later continues worker in Desktop | Fresh verified activity/attachment can re-enter Desktop section | Do not revive old dispatch, HQ role, authority, push or knowledge grants |
| Two actual attachments conflict | One conversation group with explicit conflict; pane browser keeps both locators | Do not silently select an owner or merge output by title |
| Fork shares title/root session | Separate thread with lineage | Never deduplicate by title, cwd or tree root |
| Backend disconnected/unsupported | Preserve data, mark observation unavailable, disable unverified controls | No fabricated working→done transition or completion push |

Reap success requires the owned resource to be closed. With a shared daemon, frontend
exit does not prove backend writer or background turn release. Report these separately.

## 4. Presentation and actions

Title first, one compact secondary line, existing status glyphs. Examples:
`Codex · tmux %N`, `Codex · ChatGPT Desktop`, `Codex · controller unconfirmed`.
Creation source belongs in details, not the current-client badge. Historical records
stay outside live counts. HQ badge requires verified current HQ binding. Do not add a
default stream of all Desktop history.

| Action | Preconditions and behavior |
|---|---|
| Read conversation | Verified thread, owner/paired access; no follow or input side effect |
| Go to pane | Exact live binding; existing focus path |
| Send/approve | Existing authorized pane route, revalidated before sending; never silently redirect |
| Open in Desktop | Verified local thread, URL handler and matching Codex home; Mac-local only. Opening is not handoff proof |
| Follow settings | Independent observation/push/knowledge choices; Desktop or unknown control cannot inherit automatic CLI supervision |
| Continue in tmux | Future explicit handoff, never ordinary spawn or second resume; unavailable until proven safe |
| Fork | Future explicit new conversation with lineage, never called a move |

Five surfaces consume Go core evidence/capabilities. Menubar has local focus/Desktop
actions. Phone/iPad read through the paired Mac and request existing authorized pane
actions, rather than opening a Mac-local URI on iOS. Web uses the same ordering/rules;
guests only access granted tmux panes, not arbitrary stored threads. CLI exposes
structured evidence/receipts. Missing capabilities remove actions; UIs do not guess.
Physical phone/iPad layout and VoiceOver still need device acceptance.

Chat shows public persisted dialogue and intermediate replies. Terminal shows actual
output: alternate mode may offer only the current frame; inline mode accumulates later
scrollback. Briefly explain limited history in that state and offer Chat. Never turn
transcript text into fabricated ANSI history. Existing clients need a separately approved
same-ID restart to change launch mode. `/tui` saves a next-launch choice; `/raw` has not
been established here as a safe alternate-screen exit.

## 5. Core adapter and evidence

Add a versioned Go leaf, cgo-free. Prefer validated capabilities on an already running,
trusted local backend. Do not discover/start a backend or resume a thread merely to
populate the list. Negotiate version/schema; bound calls, bytes, pagination and cancellation.
A read-only path permits metadata/history reads, not resume, turn, queue changes,
settings, shell, archive or delete. `thread/read` does not subscribe: live updates require
validated read-only observation or bounded reads, never resume disguised as subscription.
Use supported turns/items pagination; fall back to the existing verified bounded rollout
reader when needed, reporting omissions.

The inspected schema has no general current-frontend-owner/transfer method. Lock
existence, loaded list, direct-input capability and successful resume cannot fill that
gap. A backend's notLoaded state does not prove another process has no writer. Do not
manipulate Codex locks or SQLite. Unknown control permits reading, not takeover.

Reconcile attachment generations and turn IDs, not wall-clock recency alone. Exact tmux
binding may retire provisional evidence only with a conditional update protecting newer
independent native activity. Persist bounded retirement receipts for legacy reconciliation
and crash recovery. Inherited pane hints are not ownership proof. Corrupt state/write
failure must expose diagnostics and retryable state, never silent success. Reap receipts
carry verified thread, dispatch and closed-resource identities, without message bodies.

Diagnostics correlate server/home/backend, thread, attachment generation, verified
pane/process, turn and dispatch IDs; source/cause, before/after state, evidence time,
result/error. Trace binding→retirement→reap→fresh Desktop activity without credentials,
whole environment or prompts. Dedupe notifications by thread/turn/outcome; attachment
switching is not a new completion. Recheck follow and server-push grants on delivery.
Knowledge capture uses existing consent intervals and candidate ledger.

## 6. Batches and acceptance

| Batch | Delivery | Gate |
|---|---|---|
| A: lifecycle | Exact provisional retirement, reap correlation, bounded old-case reconciliation | Start→binding→Stop→reap has no ghost; fresh native activity survives; missing proof stays uncertain |
| B: identity/policy | Separate creation/controller, compatible fields/capabilities, uniform labels/menus | Desktop continuation cannot inherit HQ/input/mining grants; forks/hosts stay distinct; old clients degrade safely |
| C: reading | Read-only adapter, bounded history/updates, honest terminal limits | Viewing never resumes, claims, submits or changes permissions; late/error responses cannot affect another thread |
| D: optional handoff | Reviewed same-thread transfer and existing-client inline restart | Prove exclusive control; otherwise keep unavailable with a clear reason |

D first needs an isolated operation plan: natural idle, no queued/unsubmitted draft,
exact thread/pane/process, preserved model/effort/provider, approval/sandbox/workspace,
instructions and non-secret launch settings. Confirm old owner release before replacement;
never terminate a shared backend. Compare effective settings after resume; verify unique
controller, and for inline mode alternate_on=0 plus subsequent scrollback growth.
Timeout/conflict/partial failure retains a recovery record; rollback cannot spawn a
duplicate, broaden permissions or send queued input. Real switching requires the user's
explicit decision once that operation is reviewable.

Regression fixtures: both history formats, two backend processes, shared-server rejoin,
fork/root separation, identical titles/cwds, different hosts/homes, inherited pane hints,
reordered turns, PID reuse, interrupted binding/retirement writes, repeated reap, fresh
native activity racing reap, disconnected observation, old schemas, pagination omissions,
revocation and duplicate notifications. Break new guards to confirm they fail. Isolated
tests must not touch real Codex homes/HQ. UI acceptance includes short labels, disabled
actions, stable counts, safe areas, keyboard and actual-device VoiceOver.

## 7. Decisions and unknowns

Prioritize A, then B/C. D is a separate capability investigation and user-reviewed
operation; the public protocol does not establish safe general transfer. Do not archive
reaped workers by default. Future automatic archive needs its own policy for descendants
and user history.

Unverified: Desktop frontend control internals, installed-backend safe read-only
subscription, custom-home/old-version compatibility, Desktop tool/environment restoration
in CLI and end-to-end transfer. Screenshots/static sources do not establish these.
Post-update doctor UX is separate, tracked on its development branch in
`openspec/changes/post-update-environment-check/`; no automatic repair or existing-client
restart is included here.

## 8. Batch A implementation

The Go core now retires only exact PID-less minimal starts, using atomic native writes,
unique generations and a one-second store-lock deadline. Binding receipts are private,
store an opaque witness hash, and precede conditional removal. `Live` can finish a
receipt-backed interrupted removal; new activity remains visible. Legacy cases without
proof keep existing grace. This does not classify the current Desktop controller.

Reap records now distinguish resource closure and partial failure, correlate a thread
only against the same retained binding/process/pane incarnation, and preserve the task
ledger on failure. Failed resource closure stops worktree/branch cleanup. Closing a
frontend still does not prove shared-backend writer release. The recent historical
receipt cache is capped at 256; observations still on disk retain recovery receipts.

Tests: `internal/native/binding_test.go` (cross-process same-second writes, conditional
pruning, lock timeout, interrupted/corrupt receipts, bounded retention and identity),
`internal/resume/codexbinding_test.go`, `internal/hook/codexbinding_test.go` (failed receipt
retry without a false native turn), `internal/events/reap_result_test.go` and
`internal/app/reap_receipt_test.go` (closure failure, recovery ledger and correlation).
Four deliberate mutations failed at their behavior assertions: constant write generation,
omitted retirement, cleanup after failed kill, and premature failed-ledger deletion.
No real Codex/HQ client was stopped or resumed; B/C implementation follows below; D remains separately gated.

## 9. Identity, consent and read-only implementation

Native Codex rows now expose `creation_client` (with legacy `client` retained),
`current_client:"unconfirmed"`, opaque `home_id`, and `can_read_conversation`.
Originator remains creation metadata. The current public protocol does not prove which
frontend controls a shared backend, so no Desktop/Terminal ownership is invented.
Native Codex never inherits HQ role from cwd. Its external group defaults to status only;
HQ follow, notifications and future knowledge capture require per-conversation consent.
Actual external turn activity writes a durable consent boundary, which survives native
record removal. CLI-created threads therefore cannot bypass consent when later used
outside tmux. Existing tmux input remains addressed to its physical pane.

Native observations, binding receipts, resume records, follow policies and event metadata
carry an opaque canonical Codex-home identity. Symlink aliases share identity; equal
thread IDs in different homes do not share grants, rows, history revisions or alert keys.
Pre-scope follow grants are read only in the default home. Old unscoped observations are
not assigned to alternate homes. Readers operate on the server's configured Codex home;
other-home rows cannot expose that home's content through this server. Requests can pass
`home_id` (HTTP) or `--home-id` (CLI). Old ID-only requests fail when observations make the
scope ambiguous. Modern `session_meta.id` identifies the thread; a tree-root `sessionId`
cannot alias a fork. This batch does not expose a lineage tree or frontend-conflict UI.

Verified CLI-created conversations now use the same bounded public-history reader as
Desktop-created conversations. Mac, iPhone and iPad read serially and discard responses
from another server/home/thread. CLI reads the same data; Web remains status-only.
No existing-backend observer has been validated, so the stored-history fallback remains
in use. Reading does not resume a thread, start a daemon, acquire control or change consent.
Terminal still contains only captured terminal output; overwritten alternate-screen
frames cannot be reconstructed from this change.

Codex adoption is unavailable until exclusive frontend handoff can be established.
`SessionEnd` alone cannot delete a Codex observation: a shared backend PID is not a
frontend identity. Exact binding retirement, confirmed process death and existing grace
remain available. Other agents retain their existing adoption/lifecycle behavior.
General transfer and in-place inline restart remain unavailable (batch D). No real
client switch, release, installation, device layout or VoiceOver acceptance was performed.
