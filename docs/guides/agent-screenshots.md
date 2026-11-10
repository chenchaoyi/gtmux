# Screenshots for local agents

Use `gtmux screenshot` from a local agent's command tool. It emits versioned JSON, image paths and SHA256 hashes. A second MCP service is not needed for this workflow. The native operations require a compatible **running** Gtmux macOS App; querying capabilities never launches it or opens a permission prompt. A new App can take up to two seconds to acquire the screenshot endpoint from an older instance.

## Observe, capture, inspect, mark, send

Keep images and sidecars in the current task's output directory. Do not use gtmux configuration or state directories as a project workspace.

```sh
mkdir -p /tmp/my-task-shots
gtmux screenshot capabilities
gtmux screenshot sources --app com.mitchellh.ghostty
gtmux screenshot capture --source-ref REF --rect 40,60,800,500 --unit pt --output /tmp/my-task-shots/terminal.png
```

Choose `REF` from the returned window inventory. To select a display explicitly, use `sources --displays`. References expire after 30 seconds. The window's owner, process birth, geometry, display mapping and backing scale are checked again before and after capture. A scale change with unchanged point bounds also expires the observation. A changed source is refused rather than silently widening to a whole display. Regions use source-local top-left **points**; a window can have a negative global display origin. Very large full sources and regions crossing different display scales are refused; select a smaller source or separate regions.

Read the returned `preview_path` through your agent's image viewer. The sidecar includes full/preview dimensions and hashes, a capture ID and the point-to-image transform. Use exactly that coordinate frame when marking. An existing PNG/JPEG/other supported ImageIO image can instead be normalized with:

```sh
gtmux screenshot import --input /tmp/existing.jpg --output /tmp/my-task-shots/imported.png
```

Imported images are identified as imported, without invented live-window provenance. Annotation JSON addresses an exact image:

```json
{
  "schema_version": 1,
  "capture_id": "COPY_FROM_SIDECAR",
  "image_sha256": "COPY_FROM_SIDECAR",
  "coordinate_space": "image_px",
  "annotations": [
    {"type": "rect", "x": 20, "y": 30, "width": 300, "height": 80, "color": "red"},
    {"type": "arrow", "from": [500, 200], "to": [320, 90], "color": "blue"},
    {"type": "text", "x": 20, "y": 130, "text": "Inspect this result"},
    {"type": "redact", "x": 20, "y": 220, "width": 200, "height": 40}
  ]
}
```

`preview_px` is also supported when the matching `preview_id` is supplied. Other marks are `ellipse`, `mosaic`, and `line` with `points: [[x,y], ...]`. Colors are red/blue/yellow; stroke styles are thin/medium/thick. Solid `redact` rounds coverage outward to intersected pixels, including in the downsampled preview, and removes covered pixels from the derived PNG; mosaic is visual obscuring and is not equivalent. The unredacted base still exists locally: keep it private, and send **only the derived file**.

```sh
gtmux screenshot annotate --capture /tmp/my-task-shots/terminal.capture.json --spec /tmp/my-task-shots/marks.json --output /tmp/my-task-shots/marked.png
gtmux screenshot edit --document /tmp/my-task-shots/marked.capture.json
```

Annotation uses the existing native renderer. Base image plus marks remain editable through the existing editor; an open draft is preserved and returns `editor-busy`. Capture, import, annotation and editing never send implicitly. Output files are not overwritten. A later annotation of an editable derived document is refused: use `edit`, or explicitly import the flattened image as a new base.

## Deliver to one exact conversation

```sh
gtmux screenshot target --pane %9 --session-id EXPECTED_SESSION_ID
gtmux screenshot send --image /tmp/my-task-shots/marked.png --target-ref TARGET_REF --request-id task-shot-00000001 --message-file /tmp/my-task-shots/note.txt
gtmux screenshot status task-shot-00000001
```

Target references last ten minutes and bind session, home, client, pane root and tmux server incarnation. This first version supports verified managed Codex and DeepSeek Harness native bindings. Other clients keep `gtmux send --attach`; no bare-terminal fallback is introduced. Resolve a reference on demand, rather than adding process inspections to normal agent-list polling.

A receipt records the **last observed** state and time:

| State | Meaning |
| --- | --- |
| `landed` | Input receipt confirms entry into the conversation. |
| `active_turn_accepted` | Current turn accepted the image input. |
| `queued` | Backend accepted it for a queue; entry into conversation is unconfirmed. |
| `blocked` / `failed` | Submission was refused or failed before a confirmed input receipt. |
| `unknown` | Acceptance could not be confirmed. Preserve the ID and investigate; do not retry automatically. |

None means the agent read the image or completed work. `status` reads the durable receipt; it does not resume a thread or claim a queued item is still queued now. The same request ID and payload never silently send twice; changing its image, note or target is a conflict. Resolve expired targets for **new** submissions; inspect an existing request by ID.

## Limits and storage

Requests have a ten-second maximum budget, 256 KiB JSON, 30 MiB/image, 32 million pixels, 500 marks and 10,000 path points. Preview long edge is at most 1600 pixels. The App accepts local same-user Unix connections with pinned host identity, a per-host nonce and opened file descriptors; requests cannot name arbitrary native output paths. No TCP, shell execution, clipboard changes or implicit Screen Recording prompt occurs. The App blocks on its listener when idle, with two concurrent request slots. Deadline expiry, caller disconnect and host stop revoke the request and close its transport. Image decoding, rendering and file IO run away from the UI thread. An OS callback or filesystem call may itself be uninterruptible: the host retains a bounded outstanding-work permit until it actually completes, rejects replacement work with `native-busy`, and discards late results instead of opening an editor or publishing an artifact. A stalled OS capture does not prevent a capabilities query; it does not authorize automatically retrying an uncertain send.

The caller owns PNG/preview/sidecar outputs. Product-managed `capture-bridge` holds only the host/socket lease and identity. `capture-requests` holds a private target-signing key and hashed delivery receipts, capped at 1000 directory entries; it stores no note or image body. A full receipt budget returns an explicit error and does not discard unresolved receipts. This increment does not automatically migrate or delete existing archives. Existing attachment delivery retains its established product-managed uploads behavior.

Linux can query CLI help and use supported delivery logic, but native image operations report `platform-unsupported`. Native recording permission, real window capture and signed installation are acceptance items distinct from synthetic renderer/socket tests.
