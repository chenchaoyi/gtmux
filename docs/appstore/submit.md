# Submitting gtmux to the App Store — runbook

The Fastfile automates the binary + metadata upload; the **Submit** itself is manual in
App Store Connect (ASC). This is the end-to-end checklist. App: `com.gtmux.app`
(ASC App ID `6791144062`).

Deep build/toolchain footguns are in [`../TROUBLESHOOTING.md`](../TROUBLESHOOTING.md)
("iOS 上架" + "arm-ruby migration"); this doc is the *what to run and click*.

---

## 0. Toolchain prerequisite (once per machine)

fastlane must run under **arm** Homebrew ruby (an x86/Rosetta ruby silently breaks the
archive). Verify:

```sh
which ruby      # → /opt/homebrew/... , NOT /usr/local/...
file $(which ruby) | grep arm64
```

If it's x86, see TROUBLESHOOTING's "arm-ruby migration". `.zshrc` should have:

```sh
export PATH="/opt/homebrew/opt/ruby/bin:/opt/homebrew/lib/ruby/gems/4.0.0/bin:$PATH"
```

ASC API creds live in `~/.zshrc` (`ASC_KEY_ID` / `ASC_ISSUER_ID` / `ASC_KEY_PATH`,
pointing at `~/.appstoreconnect/private_keys/AuthKey_*.p8`). An interactive shell has
them; a non-interactive one must source `.zshrc` first.

---

## 1. Version

The **public** App Store version = the pbxproj `MARKETING_VERSION`, which
`mobileapp/scripts/set-version.sh` syncs from the latest git tag. Bump the tag, then:

```sh
cd mobileapp && bash scripts/set-version.sh    # writes src/version.ts + pbxproj
```

Confirm: `grep MARKETING_VERSION ios/GtmuxMobile.xcodeproj/project.pbxproj` (all targets
equal, e.g. `0.41.0`). The build NUMBER auto-increments off ASC — don't set it.

---

## 2. Build + upload the binary

```sh
cd mobileapp
bundle exec fastlane release
```

This archives (Release, production `aps-environment`, team `2337SY8FRT`), exports the
signed ipa, and uploads to TestFlight/ASC. It does **not** submit for review
(`skip_submission: true`).

If it dies early with a one-line gym log, that's the formatter — the lane already sets
`xcodebuild_formatter: ""`; see TROUBLESHOOTING if it regresses.

---

## 3. Push listing text + screenshots

Screenshots live in `mobileapp/fastlane/screenshots/{en-US,zh-Hans}/` (3 each, 1320×2868).
Regenerate them from a v-current simulator build with the Appium harness — see
[Regenerating screenshots](#regenerating-screenshots) below.

```sh
bundle exec fastlane metadata                    # text (en+zh) + creates the version record
bundle exec fastlane metadata skip_metadata:true # screenshots only
```

The **two-step** is required for a NEW version: `deliver` uploads the text fine, then hits
a fastlane bug (`No data`, reading a not-yet-existing review detail) *before* screenshots.
The `skip_metadata:true` re-run pushes the screenshots. (Both steps are idempotent.)

---

## 4. Manual steps in ASC (the part fastlane can't do)

Go to the app's **X.Y.Z Prepare for Submission** version page.

### 4a. Build
Under **Build**, click **+** and select the build you just uploaded (wait a few min for
Apple to finish processing if it's not selectable yet).

### 4b. App Review Information
- **Sign-In Information → UNCHECK "Sign-in required".** gtmux has NO account/login; it
  pairs to the user's own Mac via a one-time code, so there is no username/password to
  give a reviewer. Leave User name / Password blank.
- **Notes** — the reviewer has no Mac to pair with, so point them at **Demo mode** (this
  is exactly why Demo mode exists). Paste:

  > gtmux is a companion app that pairs with the user's OWN Mac (running the open-source
  > `gtmux serve`) to monitor and reply to coding-agent sessions in their tmux — analogous
  > to an SSH / remote-terminal client for a machine you control. There is no account or
  > login: pairing is done via a one-time QR/code shown by the user's Mac.
  >
  > TO REVIEW WITHOUT A MAC: on the first "Servers" screen, tap **"No Mac? See a demo"**.
  > This opens a fully self-contained demo (sample data, marked with a DEMO badge) covering
  > the whole app — the agent radar, opening a session's terminal, the 1/2/3 approval card,
  > and the HQ command page. No server, network, or credentials required.
  >
  > Remote terminal input goes only to the user's own paired machine, gated by a bearer
  > token the user controls; nothing is sent to any third party.

- **Contact Information** — the developer's real name / phone / email (Apple contacts you
  about the review). Not app data.
- **Attachment** — leave empty (optional).

### 4c. General Information (first submission only — set once, persists)
On the app's **General Information** page:
- **Category → Primary: Developer Tools.** gtmux is a developer utility; this category is
  the right fit and far less crowded than Productivity (don't pick Productivity — it's a
  sea of notes/todo apps). Secondary (optional): Utilities, or leave blank.
- **Content Rights → "No, it does not contain, show, or access third-party content."**
  gtmux shows the user's OWN terminal sessions/code on their OWN machine — not
  third-party content the app distributes.
- **Primary Language: English (U.S.)** (matches the en-US-first metadata; zh-Hans is the
  localization). **License Agreement: Apple's Standard License Agreement** (no custom EULA).

### 4d. Age Ratings (required — "Set Up Age Ratings")
Answer every content category **None / No**. gtmux is a developer tool with no
objectionable content. The two that give pause:
- **Unrestricted Web Access → No.** It's a terminal to the user's own Mac (SSH-analogous),
  not a web browser.
- **User-Generated Content → No.** It shows the user's own agent sessions, not content
  other users share through the app.

Result: **4+** (same class as SSH/terminal apps).

### 4e. Sections that need NOTHING (verify, don't fill)
- **App Encryption Documentation** — already handled by the binary:
  `ITSAppUsesNonExemptEncryption = false` is in `ios/GtmuxMobile/Info.plist` (gtmux uses
  only standard OS TLS = exempt). No upload; you won't be asked at submission.
- **App Store Server Notifications** (Production/Sandbox URL) and **App-Specific Shared
  Secret** — these are for in-app purchases / subscriptions. gtmux has none. Leave blank.
- **Vietnam Game License** (not a game) and **Regulated Medical Devices** (not medical) —
  skip.

### 4f. Digital Services Act (EU only — "Set Up")
Declare your **trader status** (an individual releasing a free, non-commercial app is
typically a non-trader). Affects **EU App Store availability only**, not function. Apple
may require trader details for continued EU distribution; if you'd rather not deal with EU
trader requirements, you can exclude the EU from availability instead.

### 4g. Privacy
App **Privacy → "Data Not Collected"** (gtmux collects nothing; it talks only to the
user's own paired Mac). Confirm the **Privacy Policy URL** is
`https://ccy.dev/projects/gtmux/privacy` (resolves 200).

### 4h. Everything else
- Support URL `https://ccy.dev/projects/gtmux/support`, Marketing URL
  `https://ccy.dev/projects/gtmux` (both resolve).
- Screenshots / description / keywords are already pushed by step 3.

### 4i. Submit
Click **Add for Review** → **Submit to App Review**.

---

## Regenerating screenshots

Requires a booted 6.9" simulator (1320×2868) + the app built for it + Appium (Node **22**,
not 26 — 26's undici breaks webdriverio). One-time per release:

```sh
cd mobileapp
UDID=$(xcrun simctl list devices | grep "iPhone .* Pro Max" | grep -oE '[0-9A-F-]{36}' | head -1)
xcrun simctl boot "$UDID"
bash scripts/set-version.sh

# English
xcrun simctl spawn "$UDID" defaults write .GlobalPreferences AppleLanguages -array en
GTMUX_E2E_UDID=$UDID bash scripts/e2e-build-sim.sh
export PATH="$HOME/.nvm/versions/node/v22.22.0/bin:$PATH"     # node 22 for webdriverio
npm run e2e:appium &                                          # if not already running
GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID=$UDID npm run test:e2e -- -t "app store"

# Chinese: set the sim to zh-Hans, reboot, rebuild-install, re-run with GTMUX_SHOTS_LANG=zh
# Then copy .e2e-artifacts/appstore/{en,zh}/*.png → fastlane/screenshots/{en-US,zh-Hans}/
```

Keep ONE naming scheme (`01-radar.png`, `02-terminal-approval.png`, `03-hq.png`) — a
stray second set makes `deliver` upload a stale mix.
