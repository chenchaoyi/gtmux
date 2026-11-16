import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class BrandProgressMarkTests: XCTestCase {
    @MainActor private func render<V: View>(_ view: V, scheme: ColorScheme, name: String) throws -> NSBitmapImageRep {
        let renderer = ImageRenderer(content: view.environment(\.colorScheme, scheme)
            .background(scheme == .dark ? Color.black : Color.white))
        renderer.scale = 2
        let image = try XCTUnwrap(renderer.nsImage)
        let data = try XCTUnwrap(image.tiffRepresentation)
        let pixels = try XCTUnwrap(NSBitmapImageRep(data: data))
        if let directory = ProcessInfo.processInfo.environment["GTMUX_BRAND_LOADING_RESULTS"] {
            let url = URL(fileURLWithPath: directory, isDirectory: true)
            try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
            try XCTUnwrap(pixels.representation(using: .png, properties: [:]))
                .write(to: url.appendingPathComponent(name + ".png"))
        }
        return pixels
    }

    private func cyan(_ pixels: NSBitmapImageRep, x: Double, y: Double) -> Bool {
        guard let color = pixels.colorAt(x: Int(x * Double(pixels.pixelsWide)),
                                        y: Int(y * Double(pixels.pixelsHigh)))?.usingColorSpace(.deviceRGB) else { return false }
        return color.redComponent < 0.4 && color.greenComponent > 0.5 && color.blueComponent > 0.6
    }

    @MainActor func testMBL001RenderedFocusMovesWithoutMovingPanes() throws {
        let points = [(0.28, 0.28), (0.72, 0.28), (0.5, 0.72)]
        for scheme in [ColorScheme.light, .dark] {
            for size in [CGFloat(16), CGFloat(36)] {
                for (phase, active) in [(0.0, 1), (0.6, 2), (1.2, 0)] {
                    let pixels = try render(BrandProgressFrame(size: size, elapsed: phase), scheme: scheme,
                                            name: "mac-\(scheme)-\(Int(size))-\(Int(phase * 1000))")
                    for (index, point) in points.enumerated() {
                        XCTAssertEqual(cyan(pixels, x: point.0, y: point.1), index == active,
                                       "cyan focus must follow top-right, bottom, top-left")
                    }
                    XCTAssertEqual(pixels.pixelsWide, Int(size * 2))
                    XCTAssertEqual(pixels.pixelsHigh, Int(size * 2))
                    let corner = try XCTUnwrap(pixels.colorAt(x: 0, y: 0)?.usingColorSpace(.deviceRGB))
                    XCTAssertEqual(corner.redComponent, scheme == .dark ? 0 : 1, accuracy: 0.01)
                }
            }
        }
        let panes = BrandMark.loadingPanes(side: 100)
        XCTAssertEqual(panes, [CGRect(x: 8, y: 8, width: 40, height: 40),
                               CGRect(x: 52, y: 8, width: 40, height: 40),
                               CGRect(x: 8, y: 52, width: 84, height: 40)])
        let midpoint = BrandProgressFrame.highlights(elapsed: 0.495)
        XCTAssertEqual(midpoint[1], 0.5, accuracy: 0.0001)
        XCTAssertEqual(midpoint[2], 0.5, accuracy: 0.0001)
        XCTAssertEqual(BrandProgressFrame.highlights(elapsed: 1.8), [0, 1, 0])
    }

    @MainActor func testMBL001ReducedMotionAndInactiveUseStaticTopRight() throws {
        for reduced in [false, true] {
            for phase in [ScenePhase.active, .inactive, .background] where reduced || phase != .active {
                let pixels = try render(BrandProgressMark(size: 36)
                    .environment(\.accessibilityReduceMotion, reduced).environment(\.scenePhase, phase),
                    scheme: .dark, name: "mac-static-\(reduced)-\(phase)")
                XCTAssertTrue(cyan(pixels, x: 0.72, y: 0.28))
                XCTAssertFalse(cyan(pixels, x: 0.28, y: 0.28))
                XCTAssertFalse(cyan(pixels, x: 0.5, y: 0.72))
            }
        }
    }
}
