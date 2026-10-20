import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class HQUsagePresentationTests: XCTestCase {
    private func report() throws -> HQUsageReport {
        try JSONDecoder().decode(HQUsageReport.self, from: Data("""
        {"sources":[{"agent":"dsh","name":"DeepSeek Harness"}],"limits":{"dsh_account_status":"signed-out","windows":[
        {"agent":"claude","agent_name":"Claude Code","label":"claude week","pct_used":100,"stale":true,"observed_at":100},
        {"agent":"codex","agent_name":"Codex","label":"codex week","pct_used":12,"observed_at":200,"plan_type":"prolite"},
        {"agent":"codex","agent_name":"Codex","label":"codex session","pct_used":90,"stale":true,"observed_at":100}],
        "unknown":[{"agent":"kimi","agent_name":"Kimi Code","reason":"rolled-over"}]}}
        """.utf8))
    }

    func testMixedFreshnessKeepsCurrentCapacitySeparateWithoutLosingHistory() throws {
        let u = try report()
        let current = hqUsageWindowGroups(u.limits!.windows!, historical: false)
        let history = hqUsageWindowGroups(u.limits!.windows!, historical: true)
        XCTAssertEqual(current.map(\.id), ["codex"])
        XCTAssertEqual(current.first?.windows.map(\.pctUsed), [12])
        XCTAssertEqual(history.map(\.id), ["claude", "codex"])
        XCTAssertEqual(history.flatMap(\.windows).map(\.pctUsed), [100, 90])
        XCTAssertEqual(u.limits?.unknown?.first?.agentName, "Kimi Code")
        XCTAssertFalse(hqUsageShowsLead(u.limits!.windows!))
        var warning = current[0].windows[0]
        warning.tier = "warn"
        XCTAssertTrue(hqUsageShowsLead([warning]))
        XCTAssertTrue(hqUsageShowsLead([warning, warning]))
        XCTAssertEqual(hqUsageSourceNote(u.limits?.dshAccountStatus, zh: true), "未登录 DeepSeek Account")
    }

    func testEpochOnlyResetRemainsReadableWithoutParsingProviderText() throws {
        var window = try report().limits!.windows![1]
        window.resetUnix = 86400 * 3
        XCTAssertTrue(hqUsageResetSummary(window, zh: false, now: Date(timeIntervalSince1970: 0)).contains("resets in 3d"))
        window.resetUnix = nil
        window.resetAt = "provider reset text"
        XCTAssertEqual(hqUsageResetSummary(window, zh: true), "provider reset text")
    }

    @MainActor func testHistoricalDetailsAreAvailableWithoutDominatingTheDefaultView() throws {
        let u = try report()
        let mode = L10n.shared.mode
        defer { L10n.shared.mode = mode }
        for language in [LangMode.en, .zh] {
            L10n.shared.mode = language
            for scheme in [ColorScheme.light, .dark] {
                var heights: [CGFloat] = []
                for expanded in [false, true] {
                    let view = HQUsageAccounts(l10n: .shared, report: u, detailsExpanded: expanded)
                        .padding(14).frame(width: 600).environment(\.colorScheme, scheme)
                        .background(Theme.Palette.of(scheme).bg)
                    let renderer = ImageRenderer(content: view)
                    renderer.scale = 2
                    let image = try XCTUnwrap(renderer.nsImage)
                    heights.append(image.size.height)
                    if let path = ProcessInfo.processInfo.environment["GTMUX_UI_EVIDENCE_DIR"] {
                        let bitmap = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(image.tiffRepresentation)))
                        try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
                            .write(to: URL(fileURLWithPath: path).appendingPathComponent("usage-\(language.rawValue)-\(scheme)-\(expanded ? "expanded" : "compact").png"))
                    }
                }
                XCTAssertGreaterThan(heights[1], heights[0], "Historical and source details must remain available on expansion")
            }
        }
    }
}
