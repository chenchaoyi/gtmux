# HQ choice response and independent reads

Selecting a question choice prepares a draft in that same matter. A checkmark and prepared message make the result visible, and the existing native multiline field receives a new focus request. It never sends the answer, marks work completed or executes the described action. User-edited text is preserved; stale questions show a reload message.

The matter header has its own bounded scroll area (up to 42% of the available height). Header and conversation share an explicit vertical container, and the split view is constrained to the actual window bounds. The composer stays visible. The native harness uses the production hosting configuration at the supported 900-point minimum width and a wide window; a bare card test cannot verify this layout.

`gtmux hq workspace list --overview --stored --json` reuses the existing saved-state projection without discovering live or historical sessions. It declares `sources_included:false`. Ordinary list/participant reads keep their full semantics. The Mac reads the stored summary first and refreshes selected conversation history independently. Quiet stored polls still read selected history, since a stored revision alone cannot prove an agent transcript is unchanged.

Optional live/historical source metadata uses its own queue, at most once per minute plus explicit retries. It merges only conversation/reference/supervisor fields into the current snapshot, never older questions/revisions or the local draft. Failure is local to source status; unknown metadata must not mean a session is gone or offer a false Start HQ action. Existing source readers and the multiline field are reused.

Older CLIs may fall back once to the full read only for the known usage response to an unsupported stored flag. Timeouts, denied reads and malformed successful responses are not retried this way. Request timeout text does not assert that a submission succeeded or failed.

The local read-only diagnosis observed a full overview at 8.09s, roster discovery exceeding a 10s bound and selected matter data at 0.20s. A development binary stored-only read was about 0.94s. These are individual nonexclusive-machine observations, not an end-to-end UI timing guarantee. No production question was answered or cleanup command executed. Tests use synthetic stores/windows and distinguish AX activation/geometry from actual mouse/VoiceOver or installed-app acceptance.
