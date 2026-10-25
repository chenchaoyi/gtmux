# Conversation document acceptance fixture

This standalone UIKit app compiles the production preview transport/controller and
rich-text renderer without React Native or real pairing data. It is not a product target.
The React Native bridge still needs a full app build and separate integrated acceptance.

Use a private output directory and an **already authorized dedicated simulator**:

```sh
python3 mobileapp/ios/tests/conversation-fixture-server.py --output PRIVATE_OUTPUT
# Read PRIVATE_OUTPUT/fixture-server-port.txt for the loopback server port.
mobileapp/ios/tests/build-conversation-fixture.sh PRIVATE_OUTPUT
xcrun simctl install SIMULATOR_ID PRIVATE_OUTPUT/ConversationFixture.app
xcrun simctl launch SIMULATOR_ID com.gtmux.conversation-fixture \
  --server http://127.0.0.1:PORT --preview --transport-checks
```

The fixture only uses the synthetic `fixture-only` header. It tests partial Unicode
copy, streaming pin/resume, HTML inline rendering and absence of an app bridge.
`--transport-checks` also checks denied, unpublished, old-route, redirect and digest
failures. Read its own `Documents/fixture-results.json` in the simulator data container.
`GET /stats` on the fake server should show only metadata/content requests and **no
`/beacon/` requests**. This checks actual WebKit/network behavior, not just policy strings.

Manually long-press the text, drag either handle across paragraphs and Copy. Check the
clipboard contains the chosen substring rather than the full message. Check iPhone/iPad,
light/dark, portrait/landscape, accessibility text size, Close/Info/Share and retry states.
HTML starts static. Cancel its native Enable interaction confirmation and verify scripts
remain disabled; then confirm and check details expand. Closing/reopening must be static
again. An optional `--rtc-probe-port` on the fake server targets only loopback UDP: static
mode must send zero packets; interactive mode may send synthetic STUN after explicit
consent and must not be called a network sandbox. Request flags cannot authorize scripts.
Native controls remain outside the document. The share
popover must anchor to Share on iPad. Quick Look and text formats require separate cases;
HTML success does not establish PDF or image acceptance.

Stop only the fixture server/automation and apps started for this run. Restore any
appearance, orientation or text-size settings changed on the dedicated simulators.
Do not reclaim unrelated simulator data, pairings, caches or processes.

## Script-consent regression

`conversation-probe-server.py --output FRESH_PRIVATE_OUTPUT` starts the same synthetic
HTTP fixture and a loopback-only UDP recorder, and stops both within three minutes.
Launch the already-built fixture app against the port in
`FRESH_PRIVATE_OUTPUT/fixture-server-port.txt`. Then run:

```sh
node mobileapp/ios/tests/verify-script-consent.cjs \
  FRESH_PRIVATE_OUTPUT AUTHORIZED_SIMULATOR_ID DEDICATED_APPIUM_PORT \
  DEDICATED_WDA_PORT PRIVATE_WDA_BUILD_DIR
```

Use an existing dedicated Appium server. The runner never boots devices, resets apps,
or reclaims other automation. It closes its own session in `finally`. It verifies actual
static rendering/zero UDP before consent and after Cancel, script/UDP execution only
after native confirmation, shutdown after disabling interaction, and static mode in a
new viewer despite a caller-supplied script flag. The counter records packet lengths,
never private document contents. Inspect the saved screenshots and result JSON.
