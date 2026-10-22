# Session interaction inside conversations

Chat records messages; temporary permissions, questions and pickers can exist only on the live terminal screen. A waiting managed tmux conversation therefore keeps an interaction card visible even when no numbered choices parse. The card appears only while waiting; there is no separate Interact mode. If a wait is not recognized, the existing composer Keys and Terminal remain the fallback without changing agent status.

## Evidence and delivery

`GET /api/interaction?id=%N` reads the current visible screen through the core's tmux adapter, never scrollback. Plain text is bounded to the final 80 lines and 16,000 Unicode characters, with an explicit truncation flag. This is a live observation, kept separate from recorded messages. It is not a structured semantic description of every agent's prompt.

Numbered choices require both the agent-declared pending ask and the canonical live-menu parser. Rich/multi-select menus, startup pickers, truncated screens and unsafe labels produce no digit buttons. Nothing guesses Yes/Always/No or a default. Existing hooks remain the status authority; the screen is the input context and fallback, not a replacement state detector.

The existing composer owns one keyboard controller; the card only reuses it for safe numbered choices. Navigation, Space, Return, arbitrary advertised keys and contextual “View question” actions stay in the composer and reuse the existing capability catalogue, binding/proof and outcome queue. A tap sends only that event/digit, never a trailing Enter. Unknown outcomes block subsequent keys until explicitly resolved; there is no automatic replay. Older servers retain validated legacy keys and existing options, while explaining that live-screen observation is unavailable.

## Surfaces and layout

- iPhone/iPad: a compact card outside transcript scrolling, bounded body with vertical scrolling, horizontally scrollable screen text, collapse after a numbered response. A different numbered question opens again; ordinary refresh does not reopen the answered question. The card disappears when work resumes. Keyboard controls remain in the existing composer. Waiting remains discoverable when collapsed. The input box stays directly below the prompt; keys, question shortcuts and delivery feedback are not duplicated inside the card. Full-screen reading still exposes the exceptional interaction card.
- Web: the same snapshot, read-only guards and fallback keys; the stable shared composer is available in Chat and preserves typed text when switching modes. The card stays outside transcript scrolling.
- CLI and menubar: existing options/terminal operations remain. No second parser or new CLI command.

Requests stop outside the active wait, pause in the background and reject late results from another pane/server. Failed reads remove old choices and expose retry; they do not erase conversation history. The snapshot is a polling observation, not an atomic promise that a menu cannot change before a tap. Server input policy and binding validation remain authoritative.

External/native conversations without a verified tmux input route remain read-only. Shared links can read only granted panes; view permission never grants input. No real user prompt, device identifier or screenshot belongs in source fixtures. Synthetic UI checks, native compilation, independent review, physical-device and VoiceOver acceptance are reported separately.

Native external conversations keep human messages on the right with their avatar and bubble, without a redundant “You” heading. HQ and agent source labels remain visible on the left.
