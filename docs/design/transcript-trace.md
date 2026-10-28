# Transcript trace: browser behavior and native App proposal

## Delivery boundary

The browser has a public-transcript trace. The native App currently offers Chat and
Terminal; adding Trace to the App is proposed here, not implemented or device-accepted.
The current App entry is `mobileapp/src/screens/DetailScreen.tsx` (`DetailMode`), with
conversation reading in `ui/ChatView.tsx` and native-session polling in
`state/useDesktopTranscript.ts`.

The basic App feature needs no new server endpoint. Both clients already receive
ordered prompts, assistant segments, session identity and recorded tool input through
the existing pane/session transcript routes. DSH's model-request timing, token totals,
private reasoning and tool output are not part of this contract. A useful trace must
describe the records it actually has.

## Browser interaction

Open a pane's Trace tab, search or filter its records, then select a message or call.
The list identifies agent, loaded turn and history boundaries. Call details preserve
recorded input whitespace, working directory and truncation notices. Prose offers
Preview and Source; Copy exports the displayed record's original text.

Wide views can keep list and details side by side. A compact panel uses the whole
reading area for details; Close returns to the selected row. The same rule applies to
small board tiles, independent of the browser window width. Expanding a tile or moving
to another pane preserves the current mode. Chat and Terminal retain their composers;
Trace keeps this space for reading.

The overview represents the entire filtered history, with a native range control for
touch and keyboard navigation. Search covers all loaded records. The list renders at
most 200 records per page, with explicit previous/next controls. Polling preserves
selection and disclosure state. Follow latest can be disabled by inspecting or scrolling;
selected text and focused detail controls hold their displayed revision until released.
Session state comes from the agent roster, not inferred tool completion. An older
session filter is labeled History.

## iPhone proposal

Extend the existing mode row to Chat / Trace / Terminal. Keep one row of mode controls,
using the existing display menu rather than adding a second font toolbar. A plain shell
still offers Terminal only. A readable external session offers Chat / Trace and stays
read-only; an unreadable native session remains status-only.

Trace opens a virtualized record list with search and a compact type filter. Selecting
a row pushes a detail page; Back restores the same row, scroll offset and filters.
Details use the existing type scale, selectable text and copy controls. Tool input keeps
indentation; long lines scroll within the code region. Copy and Close have 44pt touch
targets and accessible names. Agent/sender avatars reuse the App's
`ParticipantProfileModal` and its distinction between current and recorded facts.

Follow latest must not pull a person away from an older record or interrupt selection.
After stopping follow, offer a clear return to the newest record. Current activity and
historical records remain visually distinct. Trace does not duplicate the Chat composer.

## iPad proposal

Use the actual content-pane width, including Split View and Stage Manager. A pane wide
enough for a roughly 300pt list and a readable detail column can use two columns; around
820pt is an initial threshold to validate. Narrower panes use the same drill-in flow as
iPhone. Large accessibility text may require that compact flow at a wider window size.
Do not select the layout from the device name alone.

Closing details expands the list. Rotation and resizing preserve the selected record
and reading position. Native text selection and keyboard focus take precedence over
incoming records. Light/dark appearance follows existing App settings.

## Implementation and acceptance

Keep one canonical pure event-index model with a generated browser copy and shared
fixtures when adding the native consumer. Do not copy the browser DOM into a WebView
or create a second provider-log reader. `DetailScreen` should share its transcript
subscription between Chat and Trace, mount the trace lazily, and cancel reads when
the identity or credential changes. Keys include home/session and available prompt
identity; clockless identical records have an ordering limit that the API cannot
magically resolve. A later stable server record ID would improve that case.

Native acceptance must cover five providers, mixed HQ histories, same-ID/different-home
sessions, missing time/input, truncation, delayed responses, revocation, long histories,
selection during streaming, 320pt width, large text, iPad resizing and VoiceOver. The
browser's tests and screenshots do not substitute for those native checks.
