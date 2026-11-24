# Workspace and durable responsibility

## Reconcile an incoming item

At startup, after reset, at a reconciliation tick and before ending work on a matter, read `gtmux hq workspace list --json` and `gtmux hq workspace reconcile --json`. Compare the board, task ledger and original conversations. Check unassigned or overlapping worker evidence, user-direct goals, replies and the `goals` and `intent_attempts` fields, even if unread is empty. Observe only sessions followed by HQ. Use the exact matter ID, current revision and participant's agent, home, session and verified time interval.

Menu-bar input carries a durable message ID and matter or global scope. Inspect the current record with `gtmux hq workspace show` and deduplicate by that ID. Register a verified ongoing goal once without replaying it. Use `gtmux hq workspace act --stdin --json` with an immutable source reference and stable ID, for example:

```json
{"op":"register","id":"STABLE-GOAL-ID","title":"Verify import","body":"Verified import request; validation is open.","state":"open","origin":{"kind":"board","ref":"notes/board.md#verify-import"}}
```

Use the actual board section, task ID or exact conversation and message. Register records an already-assigned goal. It neither sends work nor resumes it. Do not bulk-enroll historical or unfollowed goals. Bind new dispatch by `--matter`; pending-binding needs verification. Update existing participant intervals using the current revision and exact association, then read back. Follow explicit merge/split records when ownership changes.

Claim exact observations, then assess using the receipt's generation, current matter revision and a stable submission ID. A `noted` observation does not clear unassigned triage or next-responsibility debt. For unassigned evidence, verify the source and authority, record a `review_ref`, and choose a justified `handoff`, `no-followup` or `deferred` with a future check. Use a current matching revision for transitions. Record old observations as examined and recheck before a separate update. Look up payload fields in installed workspace help.

## Give the goal a next owner

Claim the affected goal's `followup_id` and assess `decision:responsibility` with exact generation, matter revision, evidence and the next step. Use a stable step ID, specific action and original `authority_ref`. Record either verified driver evidence, a user input request, a sourced hold, or explicit HQ follow-up. A deferred external dependency requires a future check and owner. Pause and refusal stay in effect until a verified `resume_ref`. At expiry of an external check reassess; time alone grants no authority.

For a new authorized action, save an intent with unique ID, source, exact participant and time before `op:drive`. Attempt once through the tracked identity and input gates. For queued, submitting or uncertain delivery, reconcile the original attempt, never force or replay through another route. Verify existing native or background work against the exact step, action, reference, participant and interval. An authorized worker can supply evidence through relay using the `gtmux.goal-drive/v1` schema. Verify the source before treating it as a driver.

For relay inbox/fetched and later approvals, read the exact durable request, original user instruction, actual acceptance/work and interval covering both request and reply. Use `message:ID` for a verified workspace instruction or cleared answer, or `events:N` and the matching `verified_user_event` for independently verified direct user authorization. An available reply, hint or read receipt does not prove a handoff. Use `op:resume-reply` once with the current revision and exact saved reply for already-authorized and unclaimed work, after checking prior delivery. See `reply_observations`, `intent_attempts` and `next_responsibility` for receipts and remaining debt.

## User input, reporting and closure

Use `request-input` for one genuinely user-owned decision, manual step or missing fact. State reason, specific effect, options and your recommendation. Verify and clear the exact question ID against current revision when answered or superseded on any channel. Keep open work open. Use `gtmux tasks --pending` for the separate task ledger and update its pending state as appropriate. Recorded replies still need delivery and execution checks. After restoration, recheck old driver proof and preserve user pause/refusal.

Use `report` for progress and results, with stable retry ID, actual matter and `reply_to` as appropriate. Read back the receipt before citing its ID. A report records an observation. Change lifecycle explicitly using the current revision, after lead evidence and pending questions are reconciled. Use completed only when every required result has been verified. Distinguish stage completion, CI, merge, publication, installation and real acceptance. Report remaining work and its owner first. An engineering blocker stays with HQ or a worker. Do not manufacture a user question to dispose of it.

Before marking a diagnosis reviewed, check what remains in scope, verify the original user authorization and check who is already handling it. In the same HQ turn, record a confirmed, authorized defect with no verified owner as a separate open matter using `register` and a stable ID. Cite the diagnosis and original matter, read back the registration, and use its `goal-followup` to record HQ's next step with the original `authority_ref`. If the follow-up row is still pending creation, the saved open matter leaves verification with HQ. Reuse an existing repair matter on repeat evidence and check its owner before any dispatch. Keep the original matter's external hold, condition and check time. Apply any user pause or refusal that covers the repair and use the usual dispatch gates. If the diagnosis or question is fully answered and leaves no work, record why in the assessment.

For followed DSH terminal conversations, read pages from the saved review cursor via `gtmux transcript`. Review each returned public message and use the CLI's `--review-through` acknowledgement. Handle gaps and interval boundaries separately. A notification, event read or accepted user input does not mean the conversation was reviewed. Current terminal coverage does not retroactively cover other clients or earlier periods.
