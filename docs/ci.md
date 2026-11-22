# Pull request checks

Every PR to `main` starts CI, including outside contributions. The **Checks selected**
summary lists the selected suites and the changed-file reasons. No release or signing
secret is used. GitHub can still require approval for a first contribution from a new
GitHub account; ordinary PR tests do not use the protected delivery environment.

## Which checks run

| Change | Checks |
| --- | --- |
| Recognized docs, translations, spec Markdown and release-note text only | Policy/docs gate |
| Menu-bar Swift source/tests | Policy/docs + Swift build/tests |
| Embedded Web assets or browser tests | Policy/docs + complete Go + Web logic/browser |
| Proven Jest-only `mobileapp/src/**/*.test.ts(x)` excluded by the pinned Metro resolver | Policy/docs + complete RN + Web; unavailable proof retains native checks |
| Other phone/iPad JS/TS source/tests | Policy/docs + RN typecheck/lint/Jest + Web + unsigned iOS build + Android QA/native + phone/tablet emulator |
| iOS Swift/Objective-C source | Policy/docs + RN + unsigned iOS build |
| Push relay TS source/tests | Policy/docs + relay + complete Go + RN + Android QA/native + phone/tablet emulator |
| Go test files only | Policy/docs + complete Go |
| Core/API, dependencies, build configuration, CI/scripts, tunnel Worker, unknown paths | All checks |

The allowlist lives in `scripts/ci.py`. Mixed changes take the union; an unknown path
makes the whole run full. Renames include both names; deletions retain their old scope.
Missing/invalid revisions, unreadable/oversized diffs and symlink changes also run all.
There is no workflow-level path filter. Manual **ci → Run workflow** runs all suites.

The always-on policy/docs gate validates every workflow, the selection/aggregate/scanner
guards, strict specs, bilingual pairs, source design rules, generated docs examples and
delivery helpers with synthetic credentials. `check-design.sh --source-only` omits CLI
compilation and transitive-import resolution; complete Go/platform gates enforce those.
It checks mechanically verifiable claims, not whether every sentence is accurate.

## Private disclosure policy

Exact operator-specific names and device identifiers must not be written into the
public checker itself. `check_private_source.py` reads a JSON list from the
`GTMUX_PRIVATE_TERMS` Actions secret, or locally from `GTMUX_PRIVATE_TERMS_FILE`
pointing to an ignored/private file. It checks every tracked text file, including
checker code and fixtures, and logs only matching paths/line numbers. Policy values
and matching source lines are never printed. Configuration/read failures fail the
check; the design wrapper removes policy variables before running other helpers.

The always-on policy job requires this configuration for repository-owned PRs,
main pushes and manual runs. Fork PRs receive no private policy: CI explicitly
reports that exact-term coverage was skipped. Before merging an outside
contribution, a maintainer runs the checker against its checkout with the private
policy. This is a targeted current-source guard, not general secret detection,
image inspection or a Git-history scrub. GitHub secret scanning remains separate.

## Reading the result

`ci-required` is the stable aggregate. Scope, policy and all selected jobs must succeed;
required jobs that fail, cancel, disappear or skip make it fail. Unaffected jobs may
skip. Coverage is retained for seven days; rendered-browser and native-build failures
retain diagnostics. Obsolete runs are cancelled when a PR is updated. Manual runs
have a separate cancellation group, so they cannot cancel a main-push check that
test delivery relies on.

When Android is selected, the phone runs the full suite and the tablet runs two
parallel shards. All three emulator jobs use one verified QA APK. The existing
phone and tablet checks each require every registered case to pass once. The
tablet summary resolves each shard's original execution attempt, so a rerun can
reuse an earlier successful shard from the same run. Missing or conflicting
receipts, incomplete coverage, or a shard that did not succeed fail the check.

### Recover expired Android CI artifacts

Tablet shard coverage receipts are retained for 35 days, which covers GitHub's 30-day
rerun window. The larger QA APK and diagnostics are retained for seven days.
The new retention period applies to receipts created by this workflow revision.

If a coverage download fails, check the artifact's exact name and `expires_at`
on the same run. Retry the tablet summary for a transient download error when
the artifact is still available. If the coverage artifact is missing or expired
and the original QA APK is available, choose **Re-run job** for the affected
`Android tablet emulator shard <number>` producer. Repeat for the other shard if
its receipt is also unavailable. GitHub reruns its dependent checks. If a phone
job or shard needs the APK and it is missing or expired, rerun
`Android QA build and native policies` and its dependents, or rerun the whole
workflow when the affected producer cannot be identified reliably in that run.
If the run cannot be identified or is outside the rerun window, start a fresh
CI run for the intended revision.
Each recovery must keep the receipts bound to the tested source and build.

[GitHub rerun behavior](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/re-run-workflows-and-jobs)

Maintainers can configure `ci-required` as a required GitHub branch check. Adding the
job alone does **not** create branch protection; the repository's current policy remains
separate. No new mandatory human review or deployment approval is introduced here.

## Review language

English is the default language for PR reviews. Write review summaries, inline comments
and follow-up review replies in English, even when the user conversation is in Chinese.
Quoted UI strings, user reports or logs may retain their original language; explain the
finding in English. Documentation changes still include their English and Chinese
counterparts in the same PR. This is a contributor/reviewer rule, not a CI language
detector.

## Local verification and boundaries

The systemd acceptance job creates its own temporary Linux account. After stopping
its user manager, cleanup waits up to 15 seconds for that UID's processes to exit
before removing the account. A failed test or account removal still fails CI.
This prevents an asynchronous shutdown from racing `userdel`; it does not stop
processes belonging to other users.

Run `make check` before committing; it remains the complete core gate. Run relevant
platform suites too. For Web: `node --test internal/server/web/app.test.cjs`, then
`cd tests/web && npm ci && npx playwright install chromium && npm test`. An existing
compatible Chromium can be supplied through `GTMUX_TEST_CHROMIUM`.

The browser fixture uses real repository HTML/JS/CSS/xterm with synthetic HTTP data and
never sends to a real pane. Separate hermetic contracts execute real Go HTTP handlers and independently compare both client fakes; see [contract and execution checks](test/CONTRACTS-EXECUTION.md). The unsigned native
build uses the runner's simulator architecture and includes the app, Live Activity
and notification extensions; it does not exercise
APNs, signing, installation or runtime accessibility. Safari, real phone/iPad layouts,
VoiceOver, terminal focus and tmux-resurrect acceptance remain separately recorded tests.
The mobile real-server contract suite still needs explicit `GTMUX_E2E_URL/TOKEN`; its
skips are not passes. Linux Go checks do not prove macOS-only runtime behavior.

## Test layers and reviewed cases

See [the test system](test/TEST-PLAN.md) for layer definitions, the complete case index and PR/main/release/delivery matrix. [Test design review](test/DESIGN-WORKFLOW.md) must precede test edits; quality validates source hashes, review seals, mappings and PR commit order. Native iOS CI now runs the production HTML/Markdown Foundation oracles before compiling the app. These remain separate from device UI acceptance.
