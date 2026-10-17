import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class AccessHistoryLayoutTests: XCTestCase {
    private func record(device: String = "device-identity-last-part") -> AccessHistoryEntry {
        AccessHistoryEntry(id: "one", at: "2026-10-17T10:45:12+08:00", actor: "phone:a", name: "iPhone", scope: "device",
            labelEn: "Read terminal", labelZh: "读取终端画面", outcome: "ok", deviceId: device,
            platform: "iOS 26.6", sourceIp: "203.0.113.8", method: "GET", target: "%21", status: 200, durationMs: 18)
    }
    @MainActor private func image(_ e: AccessHistoryEntry, width: CGFloat, zh: Bool) throws -> NSImage {
        let line = AccessHistoryLine(entry: e, clock: "10:45:12", day: nil)
        let renderer = ImageRenderer(content: AccessHistoryLogRow(line: line, zh: zh).frame(width: width))
        renderer.scale = 1
        return try XCTUnwrap(renderer.nsImage)
    }
    @MainActor func testLogRowsStayCompactAndFullIdentityIsDrawnWithoutExpansion() throws {
        for zh in [true, false] {
            for width: CGFloat in [488, 688] {
                let original = try image(record(), width: width, zh: zh)
                XCTAssertLessThanOrEqual(original.size.height, 90, "A normal log entry must not return to a tall card")
                XCTAssertGreaterThan(original.size.height, 28)
                let tail = try image(record(device: "device-identity-CHANGED"), width: width, zh: zh)
                XCTAssertNotEqual(original.tiffRepresentation, tail.tiffRepresentation, "Full metadata must be drawn before any interaction")
            }
        }
    }
}
