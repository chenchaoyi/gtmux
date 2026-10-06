import Foundation

/// dbg records a debug line for the menu-bar app (status item / popover / hotkey), which
/// is hard to observe otherwise, whenever its debug switch is on: GTMUXBAR_DEBUG,
/// GTMUX_DEBUG naming menubar or all, or `debug` in config.json (Diagnostics' Extra
/// detail). It goes to the log store as a debug entry, so it is still there after the
/// terminal that started the app is gone. It is echoed to stderr only when a shell
/// variable asked for it: a person watching that terminal set one. dbg used to answer to
/// GTMUXBAR_DEBUG alone, so neither GTMUX_DEBUG nor the config switch reached it.
func dbg(_ message: String) {
    guard DiagLog.debugOn else { return }
    if DiagLog.envDebugOn { FileHandle.standardError.write(Data((message + "\n").utf8)) }
    DiagLog.debug("menubar.trace", message)
}
