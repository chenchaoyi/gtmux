# Transcript trace: browser and native readers

## Delivery boundary

The browser and native App read the same transcript index. Owner/full-scope paired readers also receive explicitly recorded harness context; guest links retain their public-message scope. Chat and Terminal
remain presentation modes; Trace and Diff open from session tools. Native implementation
and automated acceptance do not establish physical-device or VoiceOver acceptance.
The current App entry is `mobileapp/src/screens/DetailScreen.tsx` (`DetailMode`), with
conversation reading in `ui/ChatView.tsx` and native-session polling in
`state/useDesktopTranscript.ts`.

The basic App feature needs no new server endpoint. Both clients already receive
ordered prompts, assistant segments, session identity and recorded tool input through
the existing pane/session transcript routes. DSH's model-request timing, token totals,
private reasoning and tool output are not part of this contract. A useful trace must
describe the records it actually has.

## Browser interaction

Open Trace from a pane's session-tools menu, search or filter its records, then select a message or call.
The list identifies agent, loaded turn and history boundaries. Call details preserve
recorded input whitespace, working directory and truncation notices. Details offer Overview, Preview, Raw content and Source. Preview formats prose and recorded JSON; Raw content preserves the recorded text. Source shows actual transcript provenance. Copy exports original content, or the displayed provenance JSON on Source.

Wide views can keep list and details side by side. A compact panel uses the whole
reading area for details; Close returns to the selected row. The same rule applies to
small board tiles, independent of the browser window width. Expanding a tile or moving
to another pane preserves the current mode. Chat and Terminal retain their composers;
Trace keeps this space for reading.

The overview represents the entire filtered history, with a native range control for
touch and keyboard navigation. Search covers all loaded records. The list renders at
most 200 projected rows per page, including disclosure headers, with explicit previous/next controls. Opening Trace locates the latest loaded record, including idle sessions. There is no follow-mode toggle. Polling preserves
selection, disclosure and the visible row anchor. Readers at the tail continue following; inspecting or scrolling away pauses it. A floating one-shot action returns to the latest record;
selected text and focused detail controls hold their displayed revision until released.
Session state comes from the agent roster, not inferred tool completion. An older
session filter is labeled History.

## iPhone interaction

Keep Chat / Terminal as presentation modes. Open Trace from session tools, alongside Diff. Keep one row of mode controls,
using the existing display menu rather than adding a second font toolbar. A plain shell
still offers Terminal only. A readable external session offers Chat / Trace and stays
read-only; an unreadable native session remains status-only.

Trace opens at the latest loaded activity with a compact progress summary. The virtualized timeline groups exact sessions and stable turns, expands the latest turn, and leaves older turns as readable request summaries. Consecutive calls compact into a counted group; expanding it exposes every individual input. An ordered icon rail distinguishes requests, public updates and calls without claiming tool success. Search opens from the header and covers all loaded records, including folded groups; one filter menu holds record types and readable session labels. Selecting a record opens full-width details over the mounted list; Close restores the same pixel position and filters.
Details use the existing type scale, selectable text and copy controls. Tool input keeps
indentation; long lines scroll within the code region. Copy and Close have 44pt touch
targets and accessible names. Agent/sender avatars reuse the App's
`ParticipantProfileModal` and its distinction between current and recorded facts.

Implicit tail following must not pull a person away from an older record or interrupt selection. Appends, streaming and prepended history retain the visible row anchor. Returning to the tail resumes following; the one-shot latest action appears only while away. Expansions survive refresh by stable turn identity, rather than a loaded turn number. Current activity and
historical records remain visually distinct. Trace does not duplicate the Chat composer.

## iPad interaction

Use the actual content-pane width, including Split View and Stage Manager. A pane wide
enough for a roughly 300pt list and a readable detail column can use two columns; the
reader uses 820pt with a font scale below 1.6. Narrower panes use the same drill-in flow as
iPhone. Large accessibility text may require that compact flow at a wider window size.
Do not select the layout from the device name alone.

Closing details expands the list. Rotation and resizing preserve the selected record
and reading position. Native text selection and keyboard focus take precedence over
incoming records. Light/dark appearance follows existing App settings.

## Implementation and acceptance

`mobileapp/src/trace/model.ts` is the canonical pure event index. The generator
`scripts/gen-trace-web.cjs` writes the browser asset; shared synthetic fixtures and a
generation check guard both consumers. Do not copy the browser DOM into a WebView
or create a second provider-log reader. `DetailScreen`, the HQ console and readable external sessions share their existing
transcript subscriptions with Trace. Opening the tool in Chat does not abort or restart
its read. The reader mounts lazily; identity, home and credential changes retire old
records, selections and profiles. External-session 401/403 responses clear cached records
and close the tool. Transient network failures retain the same-scope cache with Retry. Keys include home/session and available prompt
identity; clockless identical records have an ordering limit that the API cannot
magically resolve. A later stable server record ID would improve that case.

Native acceptance covers or must explicitly account for five providers, mixed HQ histories, same-ID/different-home
sessions, missing time/input, truncation, delayed responses, revocation, long histories,
selection during streaming, 320pt width, large text, iPad resizing and VoiceOver. The
browser's tests and screenshots do not substitute for those native checks.

Native details freeze the selected text and historical speaker together. Streaming adds an
explicit Update details action; it never silently replaces a selected revision. Historical
files use that record’s session and the selected home through the existing document bridge.
The status badge describes the roster session, not a tool result. Missing clocks are omitted.
The record-scope disclosure explains missing output/timing/token fields only when opened.

HQ also offers Load earlier sessions through its existing history reader. Loading is
explicit and disables repeat taps; history joins the same session filters and frozen
selection without a second subscription.

Compact details temporarily hide search, filters and the browser overview so recorded
input can use the full reading height. Close restores those controls and their state.

## Design references

This iteration uses Langfuse's [session grouping](https://langfuse.com/docs/observability/features/sessions) and [aggregation of repeated observations](https://langfuse.com/docs/observability/features/agent-graphs), and Chainlit's [message/step distinction](https://docs.chainlit.io/concepts/step). It retains gtmux's own icons, palette and recorded-input boundary. OpenCode's [scroll-anchor defect report](https://github.com/anomalyco/opencode/issues/17996) illustrates why explicit navigation and user scrolling must take precedence over refresh compensation. These are source-grounded references, not a live competitor audit.

## Independent disclosures and recorded context

Turns and Calls are independent bulk disclosures on browser, iPhone and iPad. Turns
hide or reveal each whole turn; Calls compact every consecutive run, including one or
two calls, into a counted row with tool names. A run can be opened separately. Explicit
bulk choices apply to new turns/runs; per-group choices survive same-scope refresh.
Filtering temporarily reveals matches without changing these choices. Folding pauses
implicit following even if the shorter list produces a programmatic tail scroll. Only
manual scrolling or Jump to latest may resume it. Closing a hidden record's details
returns keyboard focus to its visible parent disclosure.

Context appears in source order, with prefix instructions before the real prompt and
runtime/background records between updates. It never becomes a human message or an
empty Chat bubble. The four detail tabs share the same selected revision and original
copy boundary. Source contains recorded provider/record kind, exact session, turn and
available timestamps; a turn start is not an individual call time. It does not claim a
complete model request, RPC ID, token count, tool result or private reasoning.

| Provider | Recorded context used |
| --- | --- |
| Codex | Known workspace/environment input and complete system-reminder/task-notification envelopes in user records |
| Claude Code | Complete harness envelopes in user text and explicit compact summaries |
| Kimi | Known injection, skill activation, system trigger, hook result and compaction-summary inputs |
| DeepSeek Harness | Public injection append records and known harness envelopes; no replay/private thinking |
| OpenCode | No context reconstructed from its current hook log; an empty Context filter explains that none was recorded |

Each record keeps at most 8 KiB of UTF-8 text; each turn keeps at most 32 context
records and 64 KiB total. Retained partial text is marked. Existing transcript response
and cache budgets also apply. Compatible readers explicitly request `context=1`; older clients keep their original
message shape, without context-only segments. Context is additive in `segments[].context`
for opted-in owners and full-scope paired devices. The server strips it, including empty context-only
segments, for guest links and separates conditional-cache revisions by audience.
Existing public prompts/replies are unchanged; this does not widen share permissions.
Older clients may ignore the field. No source logs or hidden model requests are copied
into tests: all fixtures are synthetic.

The DSH Web screenshots supplied by the user informed the independent disclosures,
counted call runs, four detail modes and settings navigation. Their timing/token fields
are deliberately not inferred from gtmux's more limited records.
