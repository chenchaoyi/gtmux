# Release preparation and timing

This is a maintainer workflow, not another product release command. Keep the normal
PR → latest candidate CI → merge → actual main CI → annotated tag sequence. Use the
same reviewed source for CLI, Mac and any selected mobile delivery. Signing,
notarization, private installer checks and actual installation are separate receipts.

## Prepare once, before the first release PR

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

Mac CLI and App publication, protected iOS signing and production Android signing
remain unchanged. Do not skip latest-main CI to save time. Do not change runner
architecture, protection rules, cache trust or signing identities based only on a
queue timeout.

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
