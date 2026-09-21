import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

/// The gtmux mark, wherever the app draws it.
///
/// The mark is two panes across the top with the RIGHT one lit cyan, and one wide pane
/// beneath them. That is the App Store icon and the phone's `BrandMark`. The Mac drew it
/// MIRRORED — a 2×2 grid with the cyan cell top-left — in the popover header, the command
/// palette, the HQ medallion and the centre of every pairing QR 「跟app store的logo蓝色的
/// 小方块是反的」(2026-09-21).
///
/// Nothing could catch that: which corner is lit is not a string, a size or a count, and
/// the only reader was a person looking at a screenshot. So it is asked of the PIXELS —
/// where the cyan actually lands in the drawn image.
final class BrandMarkTests: XCTestCase {
    /// The centre of mass of everything cyan, in unit coordinates from the top-left
    /// (0,0 = top-left, 1,1 = bottom-right), or nil when nothing cyan was drawn.
    private func cyanCentre(_ img: NSImage) -> CGPoint? {
        guard let tiff = img.tiffRepresentation, let rep = NSBitmapImageRep(data: tiff) else { return nil }
        return cyanCentre(rep)
    }

    private func cyanCentre(_ rep: NSBitmapImageRep) -> CGPoint? {
        var sx = 0.0, sy = 0.0, n = 0.0
        for y in 0..<rep.pixelsHigh {
            for x in 0..<rep.pixelsWide {
                guard let c = rep.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB) else { continue }
                // Cyan is the only saturated colour in either drawing: strong blue and
                // green, little red. The exact shade is Theme.Status.working.
                if c.blueComponent > 0.6, c.greenComponent > 0.5, c.redComponent < 0.4, c.alphaComponent > 0.5 {
                    sx += Double(x); sy += Double(y); n += 1
                }
            }
        }
        guard n > 0 else { return nil }
        return CGPoint(x: sx / n / Double(rep.pixelsWide), y: sy / n / Double(rep.pixelsHigh))
    }

    func testTheQRCentreMarkLightsTheTopRightPane() throws {
        guard let qr = Pairing.qrImage("https://x.dev/p1#code=GM4W-HCCQ", size: 240) else {
            XCTFail("the QR did not render"); return
        }
        guard let c = cyanCentre(qr) else {
            XCTFail("the QR centre mark drew nothing cyan"); return
        }
        XCTAssertGreaterThan(c.x, 0.5, "the lit pane is on the LEFT — the mark is mirrored")
        XCTAssertLessThan(c.y, 0.5, "the lit pane is on the BOTTOM row")
    }

    @MainActor func testTheDrawnLogoLightsTheTopRightPane() throws {
        let r = ImageRenderer(content: GtmuxLogo(size: 64).environment(\.colorScheme, .dark))
        r.scale = 2
        guard let img = r.nsImage else { XCTFail("the logo did not render"); return }
        guard let c = cyanCentre(img) else {
            XCTFail("the logo drew nothing cyan"); return
        }
        XCTAssertGreaterThan(c.x, 0.5, "the lit pane is on the LEFT — the mark is mirrored")
        XCTAssertLessThan(c.y, 0.5, "the lit pane is on the BOTTOM row")
    }

    /// The bottom is ONE pane spanning both columns, not two. A wide pane's ink reaches
    /// the middle of the row; two panes leave a gutter there.
    @MainActor func testTheBottomIsOneWidePane() throws {
        let r = ImageRenderer(content: GtmuxLogo(size: 64).environment(\.colorScheme, .dark))
        r.scale = 2
        guard let img = r.nsImage, let tiff = img.tiffRepresentation,
              let rep = NSBitmapImageRep(data: tiff) else {
            XCTFail("the logo did not render"); return
        }
        // Sample the vertical seam, three quarters of the way down.
        let x = rep.pixelsWide / 2
        let y = rep.pixelsHigh * 3 / 4
        guard let c = rep.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB) else {
            XCTFail("no pixel at the seam"); return
        }
        // The neutral pane is white at 32% over the mark's dark plate; the gutter between
        // two panes would leave the plate showing through, which reads near-black.
        XCTAssertGreaterThan(c.greenComponent, 0.25,
                             "the bottom row has a gutter down the middle — it is two panes, not one")
    }

    // MARK: the shipped icon

    /// The Mac's own app icon, the file `build.sh` copies into the bundle. It was the one copy
    /// of the mark no test could reach: a hand-made binary, mirrored like the drawings, with
    /// opaque white corners that framed it on a dark Dock. It is generated from the iOS art
    /// now (`scripts/make-icon.swift`), and these read the file that actually ships.
    private func shippedIcon() -> NSBitmapImageRep? {
        let path = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("AppIcon.icns").path
        guard let img = NSImage(contentsOfFile: path) else { return nil }
        return img.representations.compactMap { $0 as? NSBitmapImageRep }
            .max { $0.pixelsWide < $1.pixelsWide }
    }

    func testTheShippedIconLightsTheTopRightPane() throws {
        guard let rep = shippedIcon() else { XCTFail("AppIcon.icns did not load"); return }
        guard let c = cyanCentre(rep) else { XCTFail("the icon has nothing cyan in it"); return }
        XCTAssertGreaterThan(c.x, 0.5, "the icon's lit pane is on the LEFT — it is mirrored")
        XCTAssertLessThan(c.y, 0.5, "the icon's lit pane is on the BOTTOM row")
    }

    func testTheShippedIconHasClearCorners() throws {
        guard let rep = shippedIcon() else { XCTFail("AppIcon.icns did not load"); return }
        let w = rep.pixelsWide, h = rep.pixelsHigh
        for (x, y) in [(2, 2), (w - 3, 2), (2, h - 3), (w - 3, h - 3)] {
            let a = rep.colorAt(x: x, y: y)?.alphaComponent ?? 1
            XCTAssertLessThan(a, 0.05, "corner (\(x),\(y)) is painted — it shows as a frame on the Dock")
        }
    }
}
