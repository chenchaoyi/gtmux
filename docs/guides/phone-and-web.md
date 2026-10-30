---
title: Use your phone, iPad and browser
description: Connect your own devices with Pair, reply to agents from anywhere, and give others access to selected panes with Share.
order: 3
---

**English** · [中文](phone-and-web.zh.md)

Use an iPhone, iPad or browser to view agent status, read output and reply. The app also provides access to HQ. This guide covers pairing your devices, everyday controls and sharing panes.

## Choose Pair or Share

| What you want | Use | Access |
|---|---|---|
| Connect your own iPhone, iPad or browser | **Pair** | Owner access to the Mac, including HQ and sharing controls |
| Let someone else watch or help with a task | **Share** | Only the panes you select; view-only or allowed to type |

**Pair is for your own devices. Send collaborators a Share link.**

## Connect your device

Complete [the three-step setup](../../README.md#get-connected) first. It covers the host installation, doctor, network choice and the App Store app's **Add a server → Scan pairing QR** controls. The same app works on iPhone and iPad. A successful pairing saves the host in the server list; open it to see the radar.

![Server list showing paired hosts and a separate guest connection](img/phone-and-web-servers-en.jpg)

To add another device or browser, run `gtmux pair` on the host for a fresh code. Each code works once and expires after five minutes. Open its browser link to pair your own browser. Keep the host awake; use [connection troubleshooting](../phone.md#troubleshooting) if the radar cannot load.

## Use the app

On **iPhone**, tap an agent to read its conversation or terminal. When it needs a reply,
choose an offered answer or type your own. You can also send control keys or a screenshot.

Open the display icon in Chat or Terminal to adjust text size. Both modes use the same controls and size setting.
Terminal also offers **Wrap lines** (default) / **Original width**.
Wrap lines fits rows to your screen. Original width keeps the Mac's row widths and allows horizontal scrolling,
which helps with tables and diffs. A wide tmux pane on an external monitor produces wide rows; older history can also remain
wide after the pane shrinks. The app does not resize the Mac pane. Exit full-screen to change display settings.
For conversation prose, use Chat; Codex's truncated pinned prompt has its own full-text reader.

An alternate-screen terminal provides its current frame; earlier conversation is available in Chat. gtmux does not prepend unrelated output from before that screen. Direct Codex sessions created or normally restored by gtmux use inline mode to preserve future terminal history. For a manually started session, use `codex --no-alt-screen` at its next launch; do not start a second client for the same conversation. Previously unrecorded terminal redraws cannot be recovered.

![iPhone terminal with the Needs your reply card and its three answers](img/phone-and-web-approval-en.jpg)

On **iPad**, a wide window keeps the radar in a sidebar and the selected session beside
it. Tap another row to switch sessions. A narrow window uses the iPhone layout.

![iPad with the radar sidebar and All panes in the main area](img/watch-a-fleet-panes-en.jpg)

Open **HQ** for its conversation, situation board and knowledge base. Tap the server name
to switch between Macs. To create a shell session on a paired Mac, tap **New session**;
you can start your agent in the terminal it opens.

Allow iOS notifications if you want alerts when an agent needs you or finishes. The Mac
must stay awake with serve running; delivery also depends on network and notification
settings. A guest connection does not receive these notifications.

## Open one pane to a collaborator

In the Mac menu-bar app, open **Preferences → Sharing → New share…**. Enter a label,
select which panes the guest can **See**, and tick **Type** only where they may send input.
Create the link and send it to them. They open it in a browser or redeem it in the iOS app.

![New share link: a label field and a See and Type checkbox for each pane](img/phone-and-web-share-en.png)

For input to work, also enable guest typing in Sharing. A view-only link needs no input
permission. Guests do not see the HQ page or owner controls.

From the terminal, a view-only link that expires in 24 hours is:

```sh
gtmux share new --label alice --view %7 --expires 24h
```

Replace `%7` with the pane you want to share (`gtmux panes` lists the IDs). To allow
replies in that pane, add `--type %7` when creating the link and enable `gtmux share on`.
A guest who can type can operate the programs in that pane, with those programs' permissions.

## End access

Revoke a guest link in Sharing when the task is done, or use `gtmux share revoke <id>`.
You can also manage guest links from a paired phone under **Settings → Sharing & pairing**.

If your own device is lost, revoke it **on the Mac**: `gtmux devices` lists its ID, and
`gtmux devices revoke <id>` removes its access. Keep pairing codes and share links private.

---

Connection choices or a connection problem? [Remote access reference](../phone.md).
Want a full terminal on another computer? [Attach from any computer](attach-from-anywhere.md).
Full parameters: [Pair](../cli.md#gtmux-pair-enroll-your-own-devices-full-control) ·
[Share](../cli.md#gtmux-share-scoped-revocable-access-for-a-collaborator).

## Identify paired devices

In Mac **Preferences → My devices · Pairing**, use the pencil beside a device to
edit its display name. For example, distinguish your work iPhone from your personal
one. This is a gtmux label; it does not change the device's system name or require
pairing again. The same label appears in the app's paired-device list.

iOS normally supplies a generic iPhone/iPad label. Reading a user-assigned name such
as “ccy's iPhone” requires an entitlement granted by Apple; gtmux does not assume it
has this permission. Browsers appear as **Browser**, with the browser family, major
version and OS when available. These details are approximate client information,
not a hardware model. An old entry showing **Client details unavailable** updates
when that device reconnects; a new pairing is unnecessary.

### Chat formatting and reply actions

Chat renders Markdown in both user messages and agent replies on iPhone, iPad and Web. Long user messages on the app keep a compact preview; expand one to read its formatting. Text remains selectable.

On iPhone and iPad, **Copy reply** below a reply copies the whole turn's text, including folded segments, as Markdown. Tool output and your prompt are excluded. **Share** opens the system share sheet with the same text. For an unfinished turn, these actions use the text received so far.

### Browser controls

On a wide screen, drag a tmux pane from the sidebar onto the board, or double-click to add it. Sessions outside tmux are marked **Status only** and cannot be opened as terminal panes. On a narrow screen, the toolbar wraps to keep its controls visible, including for ordinary shells.

In the Chat and Terminal composers, Enter starts a new line and Ctrl/Cmd+Enter sends. You can enable Return-to-send in Settings. When an input method is composing text, Enter confirms the word without sending. Chat refreshes intermediate reply text and tool details as they arrive.

Chat shows agent avatars and the HQ role mark, with Copy icons on messages (hover for labels; a checkmark confirms copying) and expandable tool call summaries. Board tiles and focused conversations use the same presentation. Unsent drafts survive transcript refreshes and switching between Chat and Terminal; text drafts are saved separately for each pane in this browser, scoped to the server and access identity.

Copy icons sit below the message background with recorded prompt time and sender attribution. Desktop controls use 36px targets and touch controls use 44px targets. Layout actions and session rows support keyboard access; Escape closes layout and settings panels.

While an agent is working, Chat shows a **Working** strip above the composer. Expand **Live output** to see the latest screen content, or collapse it to keep reading. It remembers your choice per pane and disappears when work stops. Runtime is shown when the server provides a known start time. Live updates preserve your draft and reading position; unavailable captures show a retry message.

In the browser, select **Trace** beside Chat / Terminal; in the App, open **Session tools → Trace** to inspect the recorded sequence of user prompts, agent replies and tool calls. Search the contents, filter by event type, then select an event to read or copy its detail. The overview marks follow event order. Tool details are recorded argument summaries; this view does not contain model request timing, token counts, raw arguments or tool output.

On macOS, choose **Open local workspace** at the bottom of the menu bar panel. It opens `http://127.0.0.1:8765` directly, using a one-time pairing code. No tunnel is needed. If serve is not running, the app starts a loopback-only child that stops when the app quits; an existing remote service is reused without changing its settings. Switching remote access in the app releases that child before starting the managed service. Non-tmux sessions remain visible as status-only entries.

If access is revoked, the browser returns to the access page and stops polling the old sessions. Enter a new pairing or share code to reconnect; only panes authorized by that credential are restored.

### Codex terminal history and titles

For a manually launched Codex, start the next session with `codex --no-alt-screen` (two leading hyphens). To resume an existing conversation, first exit its old client, then run `codex --no-alt-screen resume <SESSION_ID>` with the same model, effort and permission options. This preserves future terminal scrollback; it cannot recover old overwritten frames. `/raw` changes presentation and does not by itself disable alternate-screen mode. See [Codex handling](../design/CODEX.md) and the [official CLI reference](https://learn.chatgpt.com/docs/developer-commands?surface=cli).

A tmux session's displayed title comes from its terminal title. If Codex includes a project or directory (for example `Review changes | project`), gtmux preserves it. The app does not append that suffix. Codex's `tui.terminal_title` controls title components; native sessions use their recorded conversation title when available. A `|` in a title is not sufficient evidence to strip text. See the [official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).

The composer supports a larger writing area, terminal control keys, input history and snippets. Photos, files, pasted images and dropped files are staged before sending; images can be annotated. Send uploads the staged files and shows progress. A failed send keeps the draft and attachments. Reloading keeps text, but files must be selected again. History and snippets insert text for review without sending it.

Click a message's user, Agent or HQ avatar to inspect its identity and available session information. Historical messages show recorded identity; unavailable personal details are not inferred. Writable conversations show the input field without a separate writable badge.

### Appearance and trace details

Open the **Settings** gear in the browser's top-right corner, then **Appearance**, to select **System**, **Light** or **Dark**. System is the default and updates live; your choice is saved in this browser. Terminal captures retain the server terminal's colors. Close the trace inspector to give the event list its full width and longer previews. Selecting an event or **Show details** reopens it; polling does not reopen a closed inspector.

Both macOS pairing entries now show the same reachability, app scan guidance, full browser link, terminal command and code renewal/refresh controls.
