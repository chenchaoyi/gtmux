import AppKit

// Programmatic entry point (an executable SwiftPM target, not @NSApplicationMain).
// .accessory == LSUIElement: a menu-bar app with no Dock icon or main window.
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
