// make-icon.swift — build macapp/AppIcon.icns from the iOS App Store art.
//
// The Mac icon used to be a hand-made binary with no source, and it drifted: a 2x2 grid
// with the cyan pane top-LEFT, the App Store icon mirrored, with opaque WHITE corners
// that showed as a white frame on a dark Dock (found 2026-09-21). Deriving it from the
// one piece of art both platforms share means there is nothing left to drift.
//
//   cd macapp
//   swift scripts/make-icon.swift ../mobileapp/ios/GtmuxMobile/Images.xcassets/AppIcon.appiconset/icon-1024.png /tmp/AppIcon.iconset
//   iconutil -c icns /tmp/AppIcon.iconset -o AppIcon.icns
//
// BrandMarkTests reads the result: which pane is lit, and that the corners are clear.

import AppKit

// Build the macOS app icon from the iOS App Store art, so the two are the same drawing.
// macOS wants the artwork inside a rounded square with transparent margins (Apple's grid:
// an 824pt square on a 1024pt canvas, corner radius ~185) and a soft shadow under it.
let src = CommandLine.arguments[1]
let outDir = CommandLine.arguments[2]
guard let art = NSImage(contentsOfFile: src) else { print("no art"); exit(1) }

func render(_ px: Int) -> Data? {
    let s = CGFloat(px)
    guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: px, pixelsHigh: px,
                                     bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true,
                                     isPlanar: false, colorSpaceName: .deviceRGB,
                                     bytesPerRow: 0, bitsPerPixel: 0) else { return nil }
    rep.size = NSSize(width: s, height: s)
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    NSGraphicsContext.current?.imageInterpolation = .high
    NSColor.clear.set()
    NSRect(x: 0, y: 0, width: s, height: s).fill()

    let side = s * 824 / 1024
    let rect = NSRect(x: (s - side) / 2, y: (s - side) / 2 + s * 0.006, width: side, height: side)
    let radius = side * 0.225
    let shape = NSBezierPath(roundedRect: rect, xRadius: radius, yRadius: radius)

    // Shadow first, drawn by the shape itself so it follows the rounded corners.
    NSGraphicsContext.saveGraphicsState()
    let sh = NSShadow()
    sh.shadowColor = NSColor.black.withAlphaComponent(0.30)
    sh.shadowOffset = NSSize(width: 0, height: -s * 0.010)
    sh.shadowBlurRadius = s * 0.022
    sh.set()
    NSColor.black.setFill()
    shape.fill()
    NSGraphicsContext.restoreGraphicsState()

    // Then the art, clipped to the same shape.
    NSGraphicsContext.saveGraphicsState()
    shape.addClip()
    art.draw(in: rect, from: .zero, operation: .copy, fraction: 1.0)
    NSGraphicsContext.restoreGraphicsState()

    NSGraphicsContext.restoreGraphicsState()
    return rep.representation(using: .png, properties: [:])
}

try? FileManager.default.createDirectory(atPath: outDir, withIntermediateDirectories: true)
for (name, px) in [("16x16", 16), ("16x16@2x", 32), ("32x32", 32), ("32x32@2x", 64),
                   ("128x128", 128), ("128x128@2x", 256), ("256x256", 256), ("256x256@2x", 512),
                   ("512x512", 512), ("512x512@2x", 1024)] {
    guard let d = render(px) else { print("render failed \(name)"); exit(1) }
    try! d.write(to: URL(fileURLWithPath: "\(outDir)/icon_\(name).png"))
}
print("ok")
