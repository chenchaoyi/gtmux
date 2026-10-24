import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class KnowledgeUsageLayoutTests: XCTestCase {
    private func entry(_ audience: String?, repo: String? = nil, landed: Bool = false) -> KBEntry {
        KBEntry(id: "workflows/example", topic: "workflows", title: "A synthetic instruction", at: 1,
                promotedAt: audience == nil ? nil : 2, landedAt: landed ? 3 : nil,
                promoteWhy: nil, promoteTarget: nil, landedRef: nil, body: nil,
                audience: audience, audienceRepo: repo)
    }

    @MainActor private func render(_ entry: KBEntry, width: CGFloat, scheme: ColorScheme) throws -> NSImage {
        let p = Theme.Palette.of(scheme)
        let content = KnowledgeUsageSummary(entry: entry, l10n: L10n.shared, p: p)
            .padding(16).frame(width: width, alignment: .leading).background(p.bg)
            .environment(\.colorScheme, scheme)
        let renderer = ImageRenderer(content: content)
        renderer.scale = 1
        return try XCTUnwrap(renderer.nsImage)
    }

    @MainActor func testRecordedUseFitsNarrowAndWideLayoutsInBothLanguagesAndThemes() throws {
        let mode = L10n.shared.mode
        defer { L10n.shared.mode = mode }
        for language in [LangMode.en, .zh] {
            L10n.shared.mode = language
            for scheme in [ColorScheme.light, .dark] {
                for width: CGFloat in [300, 580] {
                    for audience in [nil, "hq", "machine", "repo", "everyone"] as [String?] {
                        let image = try render(entry(audience), width: width, scheme: scheme)
                        XCTAssertEqual(image.size.width, width, accuracy: 1)
                        XCTAssertGreaterThan(image.size.height, 45)
                        XCTAssertLessThan(image.size.height, 260, "Use guidance must remain a compact information list")
                        if let dir = ProcessInfo.processInfo.environment["GTMUX_KNOWLEDGE_RENDER_DIR"] {
                            let folder = URL(fileURLWithPath: dir, isDirectory: true)
                            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
                            let bitmap = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(image.tiffRepresentation)))
                            let png = try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
                            let name = "\(language.rawValue)-\(scheme)-\(Int(width))-\(audience ?? "reference").png"
                            try png.write(to: folder.appendingPathComponent(name), options: .atomic)
                        }
                    }
                }
            }
        }
    }

    @MainActor func testProjectPathTailContributesToTheDrawing() throws {
        let path = "/workspace/example/project-with-a-long-name/isolated-working-copy/instructions-alpha"
        let first = try render(entry("repo", repo: path), width: 300, scheme: .light)
        let second = try render(entry("repo", repo: path.replacingOccurrences(of: "alpha", with: "omega")),
                                width: 300, scheme: .light)
        XCTAssertNotEqual(first.tiffRepresentation, second.tiffRepresentation,
                          "The destination must remain drawn, not silently clipped at the end")
        let short = try render(entry("repo", repo: "/workspace/example"), width: 300, scheme: .light)
        XCTAssertGreaterThan(first.size.height, short.size.height,
                             "Long destinations must wrap instead of overlaying the following content")
    }

    @MainActor func testPendingAndRecordedStatesAreVisiblyDifferent() throws {
        let pending = try render(entry("machine"), width: 580, scheme: .light)
        let recorded = try render(entry("machine", landed: true), width: 580, scheme: .light)
        XCTAssertNotEqual(pending.tiffRepresentation, recorded.tiffRepresentation)
    }
}
