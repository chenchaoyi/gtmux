# Mac desktop notifications

The hook writes a private request to the local queue; the menu-bar app submits a native notification to macOS. Queueing, system acceptance and a visible banner are separate facts. A successful submission is logged as **submitted to macOS**. Focus, system authorization, banner style and sound settings still control presentation.

## Timing and form

- Keep native banners and Notification Center, with the session name and pane/agent context already supplied by the hook. Do not add a custom toast window.
- A genuine permission, question or plan-review wait may request sound with normal active priority. Respect Focus rather than classifying every wait as Time Sensitive.
- A successful turn ending gets a silent banner. Say **Turn finished**, not that the entire task is complete. Cancellation/failure must not earn a success notification. This change does not add a separate error-alert class.
- Keep existing hook rules: viewing the session suppresses its alert; HQ's routine completion stays silent; resolved waits withdraw. Do not start notifying for working/progress/tool chatter or replay an old backlog.
- Existing reply/jump actions retain their current targets. Test notifications have no agent target or reply/jump action and cannot send input.

Each distinct queue file is an event, including repeated successful turns on the same pane. A stable native identifier replaces the preceding banner for that pane/kind; it is not a lifetime dedup key. Submission failures cannot poison later events. Start the watcher before draining, wait for an authorization verdict and use normal polling as a recovery path. Reconcile even when the status item hides while idle. Requests older than 30 seconds still expire.

## Compact diagnostics

Preferences groups **Desktop notifications** with its explanation, then a compact system-permission row and **Test notification** action. Test feedback stays beside that action; **System settings…** sits beside permission status. The **Tab marker** setting is in a separate **Terminal alerts** group. Sound and Focus guidance is a section footer, not another settings row. Refresh on opening/returning to preferences and during the existing poll. Do not change system settings automatically or claim that Focus is off. A test result reports system acceptance, not proof of a banner; the person checks the banner or Notification Center.

This controls the Mac queue only. Phone/iPad APNs, external-conversation notification grants, and browser behavior retain their existing rules. Native Codex/DSH conversations that route through the consent-aware server poller are not silently opted into this hook queue by this fix.

## Evidence and limits

An injected notification-center adapter and temporary queues verify repeated turns, failure recovery, startup/permission races, resolved waits, stale/off/denied settings and an explicit test with no session target. These tests do not establish live macOS authorization, Focus behavior, banner appearance or VoiceOver acceptance.

The design follows Apple's guidance for [concise, informative notifications](https://developer.apple.com/design/human-interface-guidelines/notifications), reading [current authorization and feature settings](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter/getnotificationsettings(completionhandler:)), and the [local notification request API](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter/add(_:withcompletionhandler:)).

Unconfirmed native controllers do not imply ChatGPT Desktop. Phone notification copy directs the user to the original app, without inventing an origin or offering terminal reply actions. Only an explicitly confirmed ChatGPT Desktop origin uses its name.
