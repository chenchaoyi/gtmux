import AppKit

/// openGhosttyWindow opens a new Ghostty window running a shell command — used
/// for the "Overview" and "Live watch" actions, which need a terminal surface.
/// Mirrors the CLI's internal/ghostty.OpenWindow.
func openGhosttyWindow(running command: String) {
    let escaped = command
        .replacingOccurrences(of: "\\", with: "\\\\")
        .replacingOccurrences(of: "\"", with: "\\\"")
    let source = """
    tell application "Ghostty"
      activate
      set cfg to new surface configuration
      set command of cfg to "\(escaped)"
      new window with configuration cfg
    end tell
    """
    DispatchQueue.global(qos: .userInitiated).async {
        var err: NSDictionary?
        NSAppleScript(source: source)?.executeAndReturnError(&err)
    }
}
