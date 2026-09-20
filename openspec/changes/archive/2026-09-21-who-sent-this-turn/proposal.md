# who-sent-this-turn — the conversation says who spoke

## Why

The chat view gives every prompt the person-battery, the reader's own avatar. It is not
always the reader. On 2026-09-20 at 23:46 a message arrived in the commander's pane
reading 「先停一下，版本对不上，查清楚再做别的。」 and appeared under his avatar. HQ wrote it.
He asked for the fix in one line: 「对话里应该增加hq的角色，对于hq发的信息用hq自己的头像标识」.

A conversation that attributes someone else's words to you is not a cosmetic problem. The
commander reads his own history to remember what he decided, and a supervisor's relay
sitting under his face makes his own instruction indistinguishable from an agent's.

gtmux already knows. Every delivery into a pane is recorded with its sender, in the actor
vocabulary the action log uses. From that same machine, that same minute:

```
23:46:13  act.send  actor: hq                target: %18
23:49:46  act.send  actor: phone:c3bb6e4c    target: %18
```

The first is HQ's relay, the second is the commander from his phone. The transcript never
carried the distinction, so the chat could not draw it.

## What changes

**The journal records who sent.** `gtmux:audit:send` gains the sender, taken from the same
`diag.Caller()` the action log already stamps: `hq`, `agent:%N`, `user`. The record already
carries the target pane and a head of the payload; the sender is the one missing column.

**The served transcript attributes a turn.** `GET /api/transcript` joins each turn's prompt
against the audit sends for that pane, by a normalized 60-rune head, and stamps `from` when
the sender was NOT the person reading:

```json
{"prompt": "先停一下…", "from": {"kind": "hq", "label": "HQ"}}
```

A turn with no match carries no `from`, which is the overwhelming case and renders exactly
as it does today. That is deliberate: the reader's own channels (typing in the pane, the
phone, the browser) are all the reader, and only a delivery gtmux itself performed can be
attributed at all.

**The chat draws the sender.** HQ's messages wear the HQ mark the disc already uses, an
agent's dispatch wears that agent's icon. Everything else keeps the person-battery.

The scope was the commander's call: mark anything that was not him, not only HQ (asked and
answered 2026-09-20). A task another agent dispatched has the same defect as HQ's relay.

## Surfaces

- **终端 (terminal)** — not applicable: the CLI has no conversation view. `gtmux events`
  already shows the audit send, and now shows its sender in the same line.
- **菜单栏 (menubar)** — not applicable: the HQ card reads the situation board and the
  knowledge base, never a chat transcript. Nothing there renders a prompt.
- **手机 (phone)** — done: `ChatView` draws the sender's avatar beside a marked prompt,
  in place of the person-battery. The HQ screen gets it for free (same view).
- **iPad** — done by the same implementation, which the phone and iPad share.
- **Web (web)** — done: the browser mirror's chat and its tile chat draw the same mark.

## Risk

The join is by payload head, not by identity, so it can in principle mis-attribute: two
different senders delivering the same first 60 runes to the same pane inside the window.
The failure is a wrong avatar on a message, never a wrong message, and nothing acts on the
field. Where it cannot tell, it says nothing and the turn renders as it does today.

A guest typing through a shared link is recorded as a device, not as the owner, and is
therefore NOT marked today. That is a gap, named here rather than hidden: it needs serve's
own send path to journal, which this change does not touch.
