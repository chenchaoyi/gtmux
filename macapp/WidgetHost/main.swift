#if GTMUX_WIDGET_PREVIEW
// Isolated system-gallery acceptance host: no polling, notifications or real CLI.
import AppKit
import WidgetKit
let app = NSApplication.shared
app.setActivationPolicy(.regular)
let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 400, height: 150),
                      styleMask: [.titled, .closable], backing: .buffered, defer: false)
window.title = "gtmux Widget Preview"
let label = NSTextField(labelWithString: "Isolated widget preview\nSynthetic usage only; no session access.")
label.frame = NSRect(x: 24, y: 40, width: 350, height: 60)
window.contentView?.addSubview(label)
window.center()
window.makeKeyAndOrderFront(nil)
app.activate(ignoringOtherApps: true)
WidgetCenter.shared.reloadTimelines(ofKind: "com.gtmux.menubar.usage")
app.run()
#else
import GtmuxBar
GtmuxBarRuntime.run()
#endif
