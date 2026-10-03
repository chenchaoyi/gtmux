# Screenshot to an agent from the menu bar

## Why

Showing an agent what is on screen today takes several hops: a screenshot tool, a save or
a copy, switching to the terminal, finding the right pane, pasting a path or an image, and
typing what to look at. The commander asked for one smooth path from the menu bar: capture
a region, mark it, and either copy it or hand it, with a note, to the agent that should
look at it, without leaving what they were doing.

gtmux already has the pieces this needs on the Mac side: a global hotkey, the agent list,
the uploads dir with its age/size pruning, and `gtmux send` with verified delivery, the
draft guard and the re-send interlock. What it lacks is a capture, an annotator, an image
on the clipboard, and a way to attach a file to a send.

## What changes

- **Trigger:** a global hotkey, ⌥⌘4 (gtmux's ⌥⌘ family, next to ⌥⌘G, and shaped like the
  system's ⇧⌘4), and a camera button in the popover's header row. The hotkey class learns to
  carry more than one key: one Carbon handler dispatches by hotkey id, so ⌥⌘G is unchanged.
- **Permission first:** before capturing, `CGPreflightScreenCaptureAccess`. Without Screen
  Recording, the app asks once (`CGRequestScreenCaptureAccess`), then explains where to turn
  it on, with a button that opens that pane of System Settings. Nothing is captured until
  it is granted; a retry is the same hotkey.
- **Capture:** the popover and palette close first, then the system's own interactive
  capture (`/usr/sbin/screencapture -i -x -o -t png`). Apple's selection UI already handles
  several displays, Retina and mixed scales, the crosshair, Space for a window, and Esc;
  a cancelled capture leaves no file and nothing happens. The editor appears only after the
  capture, so it can never be in the picture.
- **Annotate:** a native editor window on the screen under the pointer: arrow, rectangle and
  text, three colours, undo/redo through the window's undo manager (⌘Z / ⇧⌘Z). Annotations
  live in the image's point space and are drawn at full pixel resolution on export, so
  Copy, Save and Send produce the same pixels.
- **Copy / Save:** Copy puts PNG and TIFF on the pasteboard. Save writes a PNG where the
  user chooses (the default name has spaces in it, on purpose).
- **Send to an agent:** a picker of the agent panes (the radar's shareable panes) and a
  note. The default target is the agent pane a terminal showed most recently
  (`gtmux panes --json` gains an additive `viewed_at`), then the last target used, then the
  most recently active agent. Send runs `gtmux send --json <pane> --message-file -
  --attach <png>`: the image is copied into the uploads dir (pruned as before) and its path
  follows the note on a line of its own. Nothing is sent until the user presses Send;
  capturing and annotating never touch a pane.
- **Safety at send:** the target is re-read just before sending. A pane that is gone is an
  error; a pane that is **waiting** on the user (a permission prompt or a question) is
  refused, because typed text and Enter could answer it. Delivery results come from
  `send --json`: landed and queued are success; a refused draft, a duplicate and an
  unconfirmed delivery keep the editor open with the reason. There is no automatic retry,
  and the retry button says to look at the pane first.
- **`gtmux send --attach FILE`** (repeatable, ≤30 MB): copies the file into the uploads dir
  under a name derived from its content and appends the path. The same image and note are
  the same message on a retry, so the existing re-send interlock refuses a second copy.

Not in this change: scrolling capture, video, OCR, blur, cloud links, history, sending to a
remote Mac over SSH, or a configurable hotkey. The path is meaningful on this Mac only.

## Surfaces

- **终端 / terminal / attach**: done in part — `gtmux send --attach` works from any shell or
  agent; there is no capture in the terminal. Attach is unchanged.
- **菜单栏 / menubar**: done — the hotkey, the header button, the editor and the send.
- **手机 / phone**: not applicable — the phone already attaches photos and screenshots
  through `POST /api/upload`; it reads `viewed_at` nowhere.
- **iPad**: not applicable, as the phone.
- **Web**: not applicable — the browser mirror keeps its upload button; capture there would
  need the browser's own screen-capture API and permission, left for later if wanted.
