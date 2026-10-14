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
| Phone/iPad JS/TS source/tests | Policy/docs + RN typecheck/lint/Jest + Web + unsigned iOS build |
| iOS Swift/Objective-C source | Policy/docs + RN + unsigned iOS build |
| Push relay TS source/tests | Policy/docs + relay + complete Go + RN |
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

## Reading the result

`ci-required` is the stable aggregate. Scope, policy and all selected jobs must succeed;
required jobs that fail, cancel, disappear or skip make it fail. Unaffected jobs may
skip. Coverage is retained for seven days; rendered-browser and native-build failures
retain diagnostics. Obsolete runs are cancelled when a PR is updated.

Maintainers can configure `ci-required` as a required GitHub branch check. Adding the
job alone does **not** create branch protection; the repository's current policy remains
separate. No new mandatory human review or deployment approval is introduced here.

## Local verification and boundaries

Run `make check` before committing; it remains the complete core gate. Run relevant
platform suites too. For Web: `node --test internal/server/web/app.test.cjs`, then
`cd tests/web && npm ci && npx playwright install chromium && npm test`. An existing
compatible Chromium can be supplied through `GTMUX_TEST_CHROMIUM`.

The browser fixture uses real repository HTML/JS/CSS/xterm with synthetic HTTP data and
never sends to a real pane. It is not a real-server contract test. The unsigned native
build includes the app, Live Activity and notification extensions; it does not exercise
APNs, signing, installation or runtime accessibility. Safari, real phone/iPad layouts,
VoiceOver, terminal focus and tmux-resurrect acceptance remain separately recorded tests.
The mobile real-server contract suite still needs explicit `GTMUX_E2E_URL/TOKEN`; its
skips are not passes. Linux Go checks do not prove macOS-only runtime behavior.
