# Release preparation and timing

This is a maintainer workflow, not another product release command. Keep the normal
PR → latest candidate CI → merge → actual main CI → annotated tag sequence. Use the
same reviewed source for CLI, Mac and any selected mobile delivery. Signing,
notarization, private installer checks and actual installation are separate receipts.

## Prepare once, before the first release PR

Check the effective author and committer before publishing any task commit:

```sh
python3 mobileapp/scripts/release-preflight.py --identity-only
```

The check accepts the project's public `gtmux@ccy.dev` identity or a GitHub no-reply
identity, and prints neither configured address. If the repository inherits a work
email, select the public identity for the operation with `GIT_AUTHOR_NAME`,
`GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME` and `GIT_COMMITTER_EMAIL`; do not silently
change shared/global Git configuration. The platform preflight includes this check.
This prevents new disclosures; it does not erase metadata already published.

1. Compare the requested changes with the previous published release. Use
   `mobileapp/scripts/delivery-scope.py` for iOS inputs and review Android inputs
   independently. CLI/Mac-only changes do not require a phone package.
2. Write each selected platform's bilingual notes. Apple metadata and
   `mobileapp/release-notes/android.json` are independent histories.
3. Stamp the intended version without inventing a temporary tag:

   ```sh
   bash mobileapp/scripts/set-version.sh --version X.Y.Z
   python3 mobileapp/scripts/release-preflight.py --version X.Y.Z --platform ios --platform android
   ```

   Select only the platforms actually being delivered. For CLI/Mac-only scope use
   `--platform none`. Preflight reads stamps, native marketing versions, notes,
   generated Apple history and its source fingerprint. It writes nothing, creates
   no tag and does not replace design, CI, signing or delivery-scope gates.
4. Run the normal required checks after stamping. Commit the prepared notes and
   generated files together, then open the PR. Recheck remote version occupancy and
   active workflows immediately before creating the official annotated tag.

## Build once, verify independently

Android CI builds/lints/tests the QA APK once. Phone and tablet jobs download the
same run-scoped package, verify its exact source and checksum, and independently
run the complete existing suite. Both jobs are mandatory, with separate evidence;
one failure does not cancel the other. A failed-only rerun uses the original
successful build output rather than guessing an artifact name from the new attempt.
The Android delivery gate accepts either the complete older combined check or all
three new checks, so previously reviewed sources remain usable.

Mac signing/notarization and protected mobile signing remain mandatory.
Mac builds now overlap the CLI gate/build. A separate publisher waits for both,
checks the immutable ZIP against the build job's source/ref/run/attempt and SHA256,
and only then uploads it and updates the cask. The official mirror also waits for
that publisher. Failed-only reruns retain the successful build's artifact ID and
build attempt; they do not substitute a ZIP from a newer or older run.

Start selected iOS and Android delivery workflows immediately after the authorized
annotated tag push. iOS can prepare the exact green main source before the GitHub
Release exists; its existing protected-environment review is unchanged. Before
installer publication it rechecks the stable release, remote annotated tag, latest
main CI and latest successful CLI job. It waits up to fifteen minutes with GETs
only if CLI is still building, and stops without uploading on failure. The original
IPA/archive and failure diagnostics still go to encrypted recovery. No new tag,
release, mobile build or Store submission is authorized by this optimization.
Unsigned iOS CI retains its existing normal ARM host, explicitly compiles the ARM64
Simulator target, and checks the actual app and both embedded extension binaries without
signing secrets. Native compilation must actually pass on the candidate and main. A
standard Intel trial exceeded 40 minutes and was not adopted as a default or configured
recovery path because it did not meet the latency goal. Do not skip latest-main CI
or change protection rules, cache trust or signing identities to save time.

## Expected savings and remaining limits

Release 171 measured 6m07s for CLI, followed by 7m54s for Mac, then 39s for the
mirror: 14m47s overall. Overlapping the builds removes roughly six minutes from
that desktop path, less artifact transfer/publisher setup. iOS was dispatched
6m32s after the tag and took 15m43s; early preparation can overlap most of that
initial CLI wait. These savings overlap: do not add them into a twelve-minute
end-to-end claim. They are scheduling estimates, not a measured new signed release.

Native compilation (iOS archive/export 10m28s, Android release build 7m33s in 171),
full phone/tablet automation, runner capacity and environment review remain real
costs. Keep candidate and actual-main checks. Do not promote a QA APK as a
production build or reuse signed archives across changed source/version/profile
inputs. Any compilation cache needs explicit input and trust boundaries before
adoption; a faster warm sample alone does not establish equivalence.

## Retrieve encrypted results consistently

Create an empty owner-only directory under task storage (`umask 077; mkdir ...`).
For a successful delivery whose workflow and delivery use the same exact commit:

```sh
python3 mobileapp/scripts/fetch-delivery.py --platform ios \
  --run RUN_ID --attempt ATTEMPT --artifact ARTIFACT_ID \
  --source FULL_COMMIT --output PRIVATE_RESULT_DIR
```

Use `--platform android` for Android. Authentication comes from the existing
`GH_TOKEN` or `gh auth token`; do not put credentials in arguments. The command
checks latest exact-source main CI, workflow/run/attempt, complete successful jobs,
artifact identity/expiry, published length/SHA256 and a single regular age-encrypted
ZIP member. It writes `VERIFIED.json` and encrypted bytes only. It does not decrypt,
sign, publish, install, issue links or retry a mutation. A delivery that intentionally
builds a different commit from its workflow checkout needs separate provenance
review; this command refuses to infer that relationship.

Only transient GET errors and HTTP 429/502/503/504 are retried, at most three times.
Normal TLS certificate and hostname checks stay enabled. If current evidence shows
an address-family problem, `--ipv4` changes resolution only in that process. The
GitHub credential is never forwarded to the artifact storage redirect. No global
proxy, DNS, TLS or authentication settings are changed.

Rerunning with the same output directory revalidates metadata and cached bytes,
avoiding another binary download. An incomplete or corrupted cache fails without
overwriting it; inspect it and select a new private directory. Do not mix decrypted
results into the transport directory. Decrypt and validate package signatures,
entitlements, source, native symbols and private publication receipts separately,
using the existing platform runbooks.

## Measure the delay before changing the process

Record these separately: preparation/rework, PR/main critical path, runner queue,
protected-environment review, build/export/notarization, transport/retries,
installation and evidence/cleanup. A job with runner ID zero and no steps did not
spend that time compiling. API job `started_at` can also include approval wait;
compare review and runner-acquisition events before calling it execution time.

Release 169 provides a concrete baseline:

| Stage | Observed time | Interpretation |
| --- | --- | --- |
| Final main CI | about 31 min | Android build 8m19s, then phone 9m58s and tablet 10m51s serially |
| Notes correction CI | about 27 min candidate + 31 min main | Android notes were omitted from the first preparation; early preflight prevents this omission |
| CLI release | 6m19s | Includes the full release gate; retained |
| Signed Mac job after acquisition | 6m17s | One earlier attempt never acquired a runner and waited 15m02s |
| iOS job after acquisition | 15m56s | One earlier attempt never acquired a runner; review and queue wait are separate |
| Android production build | 9m12s | Independent delivery, not emulator test time |

The parallel emulator path removes one serial leg (roughly ten minutes in this
sample), offset by artifact transfer, dependency setup and actual runner queue.
Measure new candidate and main runs before claiming an end-to-end speedup. It does
not promise to eliminate hosted-runner capacity or intermittent TLS failures.

If a job was never acquired, inspect its annotation and retry only failed jobs
after confirming the previous attempt has ended. GitHub documents
[failed-job reruns](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/re-run-workflows-and-jobs).
Keep successful build receipts; a rerun stays on the original ref and SHA. Any
future runner migration needs a separate unsigned/native and signing-chain
validation against the official [runner inventory](https://github.com/actions/runner-images).

Transport copies and build intermediates remain task-owned. Preserve only unique
recovery material and concise evidence under the existing retention policy;
configuration changes never authorize deletion of older protected archives.

For browser changes, run `node --test internal/server/web/*.test.cjs` before `npm test --prefix tests/web`. The VM logic harness must load the same generated shared models as the real page; rendered-browser success does not cover a missing harness dependency. `make check` is the core gate and does not run either separate browser layer.

Before opening the first PR, audit both current user guides and design documents for the changed interactions. Keeping only the design authority current can leave obsolete instructions in the published website and cause another CI cycle after the implementation is otherwise ready.
