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
HTML can expand its details; native controls remain outside the document. The share
popover must anchor to Share on iPad. Quick Look and text formats require separate cases;
HTML success does not establish PDF or image acceptance.

Stop only the fixture server/automation and apps started for this run. Restore any
appearance, orientation or text-size settings changed on the dedicated simulators.
Do not reclaim unrelated simulator data, pairings, caches or processes.
