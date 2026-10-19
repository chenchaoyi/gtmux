import AppKit
import XCTest
@testable import GtmuxBar

final class AgentIconLayoutTests: XCTestCase {
    func testBuiltinWhaleHasBalancedWhitespaceAndCustomIconsAreUnchanged() throws {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let source = root.appendingPathComponent("assets/agent-icons/dsh.png")
        let original = try XCTUnwrap(NSImage(contentsOf: source))
        let shown = try XCTUnwrap(AgentIcons.image(icon: source.path, name: "DeepSeek Harness"))
        let pixels = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(shown.tiffRepresentation)))
        XCTAssertEqual(shown.size, original.size)
        var minX = pixels.pixelsWide, maxX = 0
        for y in 0..<pixels.pixelsHigh {
            for x in 0..<pixels.pixelsWide {
                let color = try XCTUnwrap(pixels.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB))
                if color.redComponent < 0.2 && color.greenComponent < 0.2 && color.blueComponent < 0.2 {
                    minX = min(minX, x); maxX = max(maxX, x)
                }
            }
        }
        let inkWidth = Double(maxX - minX + 1) / Double(pixels.pixelsWide)
        XCTAssertGreaterThan(inkWidth, 0.75)
        XCTAssertLessThan(inkWidth, 0.8, "the original whale fills 97% of its canvas")
        let custom = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".png")
        try FileManager.default.copyItem(at: source, to: custom)
        defer { try? FileManager.default.removeItem(at: custom) }
        let customImage = try XCTUnwrap(AgentIcons.image(icon: custom.path, name: "DeepSeek Harness"))
        XCTAssertEqual(customImage.tiffRepresentation, original.tiffRepresentation)
    }
}
