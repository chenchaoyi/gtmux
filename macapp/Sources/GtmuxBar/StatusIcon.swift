import AppKit

/// dotImage renders the status-bar indicator: a filled colored circle. Used
/// non-template so the state color (red waiting / teal working / green idle /
/// gray none) shows in both light and dark menu bars.
func dotImage(_ color: NSColor, diameter: CGFloat = 9) -> NSImage {
    let pad: CGFloat = 3 // vertical breathing room in the menu bar
    let size = NSSize(width: diameter, height: diameter + pad * 2)
    let image = NSImage(size: size)
    image.lockFocus()
    color.setFill()
    let rect = NSRect(x: 0, y: pad, width: diameter, height: diameter)
    NSBezierPath(ovalIn: rect).fill()
    image.unlockFocus()
    image.isTemplate = false
    return image
}
