# App Store submission runbook — gtmux (`com.gtmux.app`)

End-to-end process to ship the gtmux iOS app to the App Store. Ported from the
Rodi/StreetEye runbook (same Apple team `2337SY8FRT`, same fastlane shape).

**Legend — where each step runs:**

- 🖥️ **You, in GUI Terminal.app** — needs the login keychain for signing; an
  agent shell can't do these. Run them yourself.
- 🌐 **You, in App Store Connect** (web) — Apple won't let these be scripted.
- 🤖 **Already prepared in this repo** — fastlane lanes, metadata, config. You
  just run the lane.

The only things that are truly "you-only" are the 🖥️ and 🌐 steps; everything
else is committed.

---

## 0. One-time setup (do once on this Mac)

### 0a. 🖥️ Ruby toolchain (system ruby 2.6 is too old for fastlane)

```sh
brew install ruby                                   # Homebrew ruby (>= 3.x)
echo 'export PATH="/opt/homebrew/opt/ruby/bin:$PATH"' >> ~/.zshrc   # Apple Silicon
# (Intel: /usr/local/opt/ruby/bin). If rvm noise appears on cd:
echo 'rvm_project_rvmrc=0' >> ~/.rvmrc
exec $SHELL -l
ruby -v                                             # should be 3.x / 4.x, not 2.6
cd mobileapp && bundle install                      # installs fastlane + cocoapods
```

### 0b. 🖥️ App Store Connect API key (Team key)

Download your Team key .p8 to, e.g., `~/Downloads/AuthKey_XXXXXXXXXX.p8`.
**Confirm it is a _Team_ key** at
<https://appstoreconnect.apple.com/access/integrations/api> (Individual keys hit
fastlane#26949 → "credentials missing or invalid"). If it's Individual, click
**Request Access** and generate a **Team** key (App Manager role).

Export in your shell profile (never commit the `.p8`):

```sh
export ASC_KEY_ID=XXXXXXXXXX
export ASC_ISSUER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export ASC_KEY_PATH=$HOME/Downloads/AuthKey_XXXXXXXXXX.p8
```

### 0c. 🌐 Create the app record (from scratch — gtmux is not on ASC yet)

1. <https://appstoreconnect.apple.com> → **Apps → + → New App**.
2. Platform **iOS**; Name **gtmux** (if taken, try "gtmux — agent radar"); Primary
   Language **English (U.S.)**; Bundle ID **com.gtmux.app** (must already exist in
   the [Developer portal](https://developer.apple.com/account/resources/identifiers/list)
   under team 2337SY8FRT — it does, from the dev device builds); SKU e.g. `gtmux-ios`.
3. Create. The record's numeric **Apple ID is `6791144062`** (shown under App
   Information; also in the app-page URL). `fastlane` + uploads reference it.

> The three targets (`com.gtmux.app`, `.widget`, `.notificationservice`) each
> need an App ID in the portal with their capabilities (Push, App Groups). The
> dev device builds already created/used them; `fastlane release`'s
> `-allowProvisioningUpdates` will fetch/refresh the App Store profiles. If the
> FIRST archive fails on a missing profile for an extension, open the workspace
> in Xcode once (Signing & Capabilities → let it provision all three), then
> re-run the lane.

---

## 1. Release runbook (every release, top to bottom)

1. **Version** — first release ships **MARKETING_VERSION 1.0** (already set).
   For a later release, bump it in `ios/GtmuxMobile.xcodeproj/project.pbxproj`
   (both configs) on a branch → PR → CI → squash-merge → tag. The **build
   number is automatic** — `fastlane release` picks `latest ASC build + 1`.
2. 🖥️ **Auth check** — `cd mobileapp && bundle exec fastlane verify`
   (prints the latest build number; proves the key before a long build).
3. 🖥️ **Pods** (first time / after a dep change) — `cd mobileapp/ios && bundle exec pod install`.
4. 🖥️ **Build + upload** — `cd mobileapp && bundle exec fastlane release`.
   Builds Release, signs (Distribution), archives, exports an `app-store` ipa,
   uploads to ASC. ~15 min. Handles the rsync + export-auth foot-guns. If the
   upload leg fails but `build/GtmuxMobile.ipa` exists, don't rebuild — run
   `bundle exec fastlane upload`.
5. 🖥️ **Metadata + screenshots** — `cd mobileapp && bundle exec fastlane metadata`
   (needs screenshots staged first — see §5). Pushes en-US + zh-Hans copy +
   screenshots; never auto-submits.
6. 🌐 **In App Store Connect** (Apple won't script these):
   - Wait ~10–30 min for the build to finish *processing*, then **select build N**
     on the 1.0 version.
   - **Encryption** — nothing to do (`ITSAppUsesNonExemptEncryption=false` in
     Info.plist auto-skips the prompt). If it ever reappears: **"None of the
     algorithms mentioned above."**
   - Confirm **App Privacy**, **Age Rating**, **Category**, **Availability**,
     **Review Notes** (§3–§4).
   - **Add for Review → Submit** (the one intentional manual click).

---

## 2. 🖥️ Demo for App Review (do this right before you Submit — REQUIRED)

gtmux does nothing live without a real `gtmux serve`. A reviewer with no demo
almost always rejects under **2.1** ("unable to review — needs a server"). Give
them a scoped, revocable **guest link**:

```sh
# On a Mac that will STAY ONLINE for the whole review window (can be days):
gtmux serve                          # if not already running
gtmux tunnel                         # public https URL the reviewer can reach
# have a few real sessions in tmux (a couple of agents / panes), then:
gtmux share --view %A --view %B --input %A     # allow viewing A+B, typing into A only
# → prints a guest link:  https://<tunnel-host>/#t=<token>
```

Paste that link into **App Review Information → Notes** (below). It's guest-scoped
(reviewer sees only A+B, types only into A) and you can `gtmux share revoke` it
after approval. **Keep the demo Mac + tunnel up until the app is approved** —
reviewers often test a day or two later.

> Fallback that's always available even if your tunnel drops mid-review: the app
> has a built-in **Demo mode** — pairing screen → "No Mac handy? See a demo →".
> Mention it in the notes so a broken tunnel doesn't cause a hard reject.

---

## 3. 🌐 App Store Connect set-once fields

- **App Information → Category:** Primary **Developer Tools** (no secondary needed).
- **Age Rating:** answer every question **None** → **4+**.
- **App Privacy → Data Collection:** **"Data Not Collected."** (Verified: no
  analytics/tracking SDKs; tokens live in the iOS Keychain; nothing is uploaded
  to us.) `PrivacyInfo.xcprivacy` already declares empty collection + no tracking.
- **Export Compliance:** exempt — standard HTTPS/TLS only, no proprietary crypto.
  `ITSAppUsesNonExemptEncryption=false` auto-skips the upload prompt.
- **Pricing:** Free.
- **Availability:** **All countries EXCEPT mainland China** for the first
  release. (China requires an ICP filing for the app + a 备案'd domain, like Rodi
  did — defer it; add China later once备案 is done. Nothing else is blocked.)
- **Sign-in required:** No (there is no account; pairing is to the user's own Mac).

---

## 4. 🌐 Review Notes (copy-paste into App Review Information → Notes)

```
gtmux is a client for "gtmux serve", a small server the user runs on their OWN
Mac. It monitors the user's tmux sessions and coding-agent sessions, and can send
keystrokes to that Mac's terminal — conceptually the same as an SSH / terminal
client (cf. Termius, Blink Shell, Prompt). NO code is downloaded or executed on
iOS; input is sent to the user's own machine over the user's own network, VPN, or
tunnel. Access is gated by a bearer token the user controls and can revoke.

There is no account and no data collection. Camera = scan a pairing QR code;
Photo Library = attach an image to send to an agent; Push = agent status alerts.
Guests (shared links) are scoped: view is limited to an allowlist and typing is
OFF by default and limited to an allowlist (input ⊆ view), enforced server-side.

TO REVIEW THE LIVE APP:
Open the app → "Add a Mac" → paste this guest link into the host field
(the app auto-detects a guest link):
    <PASTE YOUR gtmux share GUEST LINK HERE>
It is scoped to a couple of demo sessions; you can view them and type into the
one input-allowed pane.

If that link is unreachable, tap "No Mac handy? See a demo →" on the pairing
screen for a built-in sample tour of the UI.
```

---

## 5. 🖥️/🤖 Screenshots (iPhone-only for the first release)

Required: one iPhone set, **6.7″ 1290×2796** OR **6.9″ 1320×2868** (either
covers modern iPhones), **3–10 images**. No iPad set needed (we ship iPhone-only).

Easiest source — the app's built-in **Demo mode** (no server needed):

```sh
# Boot an iPhone 16 Pro Max (6.9") or 15 Pro Max (6.7") simulator, run the app,
# tap "See a demo", then capture a few screens:
xcrun simctl io booted screenshot ~/Desktop/gtmux-01.png
# repeat for radar / detail / chat / diff / push views
```

Drop the PNGs into `mobileapp/fastlane/screenshots/en-US/` (and `zh-Hans/` for a
localized set — optional; en set is used for both if zh is absent). fastlane
`metadata` uploads whatever is there.

> Ask Claude to drive the simulator's Demo mode and capture a clean 5-shot set if
> you'd rather not do it by hand — no keychain needed for screenshots.

---

## 6. Pre-submit checklist

- [ ] 🌐 `https://ccy.dev/projects/gtmux` **and** `https://ccy.dev/projects/gtmux/privacy`
      resolve (Apple fetches the **privacy URL** — a 404 = metadata reject). Create
      those pages if they don't exist yet.
- [ ] 🖥️ `AuthKey_XXXXXXXXXX.p8` confirmed a **Team** key.
- [ ] 🖥️ `fastlane verify` prints a build number (key wiring OK).
- [ ] 🖥️ `fastlane release` uploaded a build; it finished processing in ASC.
- [ ] 🖥️ screenshots staged; `fastlane metadata` pushed copy + shots.
- [ ] 🌐 build selected, Age Rating 4+, Privacy = Data Not Collected, Category =
      Developer Tools, Availability excludes mainland China, Review Notes pasted
      with a live guest link.
- [ ] 🌐 demo Mac + `gtmux tunnel` will stay online through review.
- [ ] 🌐 **Add for Review → Submit.**

Typical review turnaround ~24–48h. Watch for "Metadata Rejected" (usually a URL
or a missing demo) vs "Binary Rejected".

---

## Notes / gotchas

- **The dev device build ≠ the store binary.** Device builds are Apple
  Development-signed with `APS_ENVIRONMENT=development` (sandbox APNs). The store
  archive is Distribution-signed and, because it uses the Release config whose
  `APS_ENVIRONMENT=production`, reports production APNs — do NOT pass an
  `APS_ENVIRONMENT` override to `fastlane release`.
- **iPad is intentionally off** for the first release
  (`TARGETED_DEVICE_FAMILY="1"`). The iPad UI code is still present; to re-enable
  later, set it back to `"1,2"`, test on iPad, and add an iPad screenshot set.
- **First archive + three targets:** if signing an extension fails headlessly,
  open `ios/GtmuxMobile.xcworkspace` in Xcode once to let it provision all three,
  then re-run `fastlane release`.
