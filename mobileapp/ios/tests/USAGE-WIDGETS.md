# Usage widget acceptance fixture

Run from `mobileapp` using the repository's Ruby and locked bundle:

```sh
bundle exec ruby ios/tests/build-usage-widget-fixture.rb /private/tmp/gtmux-usage-fixture
```

The output is a standalone simulator App, `com.gtmux.widget-fixture`, with a real
Widget extension. It builds the production display files and Widget configuration,
substituting only `group.com.gtmux.widget-fixture.usage` for the production container.
It does not install itself, alter an existing App, read real pairings, or contact a
server. Install it only on an authorized dedicated simulator. Keep fixture output
outside the checkout. Existing output can be reused for incremental builds.

Use Agent and Reading to exercise current, old, expired, empty and long content.
The native previews include small/medium and all three Lock Screen families. Seed
widget gallery writes only the synthetic display snapshot to the fixture's container.
Add its widget through the system gallery; select DeepSeek in Edit Widget and verify
the Home Screen actually shows its CNY balance. A selected row in the editor alone
is not proof that WidgetKit saved the choice. Check light/dark appearance and the
largest accessibility size. Fixed widget bounds must retain the agent, main reading
and source/read time; stale information must be identified and expired capacity must
never look like a fresh zero. Native family previews are separate from installing
that family on an actual Lock Screen.

For parent refresh races, run the fixture with `--refresh-checks LOOPBACK_BASE`.
The synthetic server must answer `/slow/api/share`, `/current/api/share` and
`/rotate/api/share` with `{"all":true}`; `/refused/api/share` returns 401.
Their `/api/usage` responses contain Codex percentages 99, 24 and 25 respectively.
Delay slow usage and old-token rotate usage; a synthetic `new-fixture` bearer gets
25 immediately. The checked-in `UsageWidgetRefreshFixtureChecks.swift` records eight
results in the fixture's own `Documents/usage-refresh-checks.json`, covering server
switch, token rotation, cancellation, removal, protected storage and refusal. Require
all results to be true. These checks inject credential reads; they do not prove a
real Keychain protection class or OS background scheduling.

Run `bundle exec ruby ios/tests/usage_widget_targets_test.rb` for actual source-target,
entitlement and canonical-icon boundaries. It accepts a private copied project as
an argument: adding a host-only credential reader to that copy's Widget target must
make it fail. Swift package tests use real bounded loopback HTTP, including redirects.

Stop only the test App, services and automation created for this run. Restore changed
appearance, text size and orientation, then shut down dedicated simulators that were
originally off. Do not reset devices or remove unrelated apps or simulator data.
Unsigned simulator success is not signed-device, provisioning-profile, VoiceOver
speech or background refresh acceptance.
