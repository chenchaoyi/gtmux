import AppKit

// Programmatic entry point (an executable SwiftPM target, not @NSApplicationMain).
// .accessory == LSUIElement: a menu-bar app with no Dock icon or main window.
// Before the app can create a file: everything it writes under gtmux's roots is
// readable by its owner only, the same umask the CLI sets (openspec `diagnostics`).
umask(0o077)
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
