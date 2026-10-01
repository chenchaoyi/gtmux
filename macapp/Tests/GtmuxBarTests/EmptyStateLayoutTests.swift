import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar
final class EmptyStateLayoutTests: XCTestCase {
    @MainActor func testBothLanguagesFitCompactAndExpanded() throws {
        let original = L10n.shared.mode
        defer { L10n.shared.mode = original }
        XCTAssertEqual(EmptyStateView.startCommand, "tmux new -s work")
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for scheme in [ColorScheme.light, .dark] {
                var heights: [CGFloat] = []
                for expanded in [false, true] {
                    let view = EmptyStateView(l10n: .shared, showManual: expanded)
                        .frame(width: 380).environment(\.colorScheme, scheme)
                        .background(Theme.Palette.of(scheme).bg)
                    let renderer = ImageRenderer(content: view)
                    renderer.scale = 2
                    let image = try XCTUnwrap(renderer.nsImage)
                    XCTAssertEqual(image.size.width, 380, accuracy: 1)
                    XCTAssertLessThan(image.size.height, expanded ? 300 : 190)
                    heights.append(image.size.height)
                    if let directory = ProcessInfo.processInfo.environment["GTMUX_UI_EVIDENCE_DIR"] {
                        let bitmap = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(image.tiffRepresentation)))
                        let png = try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
                        try png.write(to: URL(fileURLWithPath: directory).appendingPathComponent("empty-\(mode.rawValue)-\(scheme)-\(expanded ? "expanded" : "compact").png"))
                    }
                }
                XCTAssertGreaterThan(heights[1], heights[0])
            }
        }
    }
}
