# Design

The core parses release bodies once and returns schema-1 JSON with both languages,
stable versions, source bounds and entries. Swift handles version selection, local
acknowledgement and presentation only. Fetch runs off-main with a 15-second process
bound; a failed/invalid receipt never advances acknowledgement. Successful receipts
are cached for offline reading. Request identities prevent an old automatic read
from replacing a manually opened history view.

Before the startup environment check runs, `prepareLaunch` seeds a missing seen
version from the existing environment-check attempt marker. No marker means first
install, recorded quietly. Versions at or below the seen version never trigger an
automatic popup, and downgrade/manual dismissal cannot lower that version.

One compact, resizable native window owns a SwiftUI reader. Reading starts on the
first user menu opening after an upgrade. Hidden launches do not activate it; a
response arriving after the menu closes waits for the next opening. The latest version is
expanded, older groups are folded, prose wraps to the actual window width, and the
body scrolls. Enter, Escape and the title-bar close control share dismissal logic.
Automatic view shows unseen entries; manual entries show recent installed releases.
Crossing more than the available source range earns a short coverage notice.

The permanent entry is available even when an automatic read fails. Startup failure
does not raise an update-failed alert; a manually opened window exposes Retry.
