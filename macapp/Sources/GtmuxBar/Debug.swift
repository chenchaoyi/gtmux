import Foundation

/// dbg writes a line to stderr when GTMUXBAR_DEBUG is set — diagnostics for the
/// menu-bar app (status item / popover / hotkey), which are hard to observe
/// otherwise. The same line goes to the log store as a debug entry, so it is still there
/// after the terminal that started the app is gone.
func dbg(_ message: String) {
    guard ProcessInfo.processInfo.environment["GTMUXBAR_DEBUG"] != nil else { return }
    FileHandle.standardError.write(Data((message + "\n").utf8))
    DiagLog.debug("menubar.trace", message)
}
