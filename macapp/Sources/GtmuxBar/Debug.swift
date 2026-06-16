import Foundation

/// dbg writes a line to stderr when GTMUXBAR_DEBUG is set — diagnostics for the
/// menu-bar app (status item / popover / hotkey), which are hard to observe
/// otherwise.
func dbg(_ message: String) {
    guard ProcessInfo.processInfo.environment["GTMUXBAR_DEBUG"] != nil else { return }
    FileHandle.standardError.write(Data((message + "\n").utf8))
}
