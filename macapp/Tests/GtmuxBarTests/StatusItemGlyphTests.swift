import AppKit
import XCTest
@testable import GtmuxBar

final class StatusItemGlyphTests: XCTestCase {
    // Ignore RGB entirely: this catches a return to color-only state encoding.
    private func alphaMask(_ status: Status, dark: Bool, awake: Bool = false, empty: Bool = false) throws -> [Bool] {
        let image = StatusItemGlyph.image(mostUrgent: status, empty: empty, dark: dark, awake: awake)
        let bitmap = try XCTUnwrap(NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 72, pixelsHigh: 72,
            bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0))
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
        image.draw(in: CGRect(x: 0, y: 0, width: 72, height: 72))
        NSGraphicsContext.restoreGraphicsState()
        return (0..<72).flatMap { y in (0..<72).map { x in bitmap.colorAt(x: x, y: y)!.alphaComponent > 0.2 } }
    }

    func testKeyStatesHaveDifferentGeometryWithoutUsingColor() throws {
        for dark in [false, true] {
            let masks = try Status.allCases.map { try alphaMask($0, dark: dark) }
            for i in masks.indices { for j in masks.indices where j > i { XCTAssertNotEqual(masks[i], masks[j]) } }
            XCTAssertEqual(try alphaMask(.working, dark: dark, empty: true), try alphaMask(.running, dark: dark))
        }
    }

    func testAwakeModifierDoesNotEraseTheStatusDifference() throws {
        for dark in [false, true] {
            let masks = try [Status.waiting, .working, .idle].map { try alphaMask($0, dark: dark, awake: true) }
            XCTAssertNotEqual(masks[0], masks[1]); XCTAssertNotEqual(masks[1], masks[2]); XCTAssertNotEqual(masks[0], masks[2])
            XCTAssertNotEqual(try alphaMask(.working, dark: dark), masks[1])
        }
    }
}
