import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class EnvironmentFindingsTests: XCTestCase {
    private func report(lang: String = "en") -> EnvironmentReport {
        let tr: (String, String) -> String = { lang == "zh" ? $1 : $0 }
        let row: (String, String, String) -> EnvironmentReport.Row = { status, label, value in
            EnvironmentReport.Row(status: status, label: label, value: value,
                                  note: tr("Review this condition; some actions must be performed separately.", "查看此项状态；部分事项需另行处理。"),
                                  description: tr("Checks the reported prerequisite.", "检查相关功能的必要条件。"))
        }
        return EnvironmentReport(schema: 1, version: "1.2.3", checkedAt: 1791819000,
                                 sections: [.init(title: tr("Logs", "日志"), rows: [row("ok", "healthy", "installed"), row("recommended", tr("Shared check", "同名检查"), tr("Recent errors", "近期出现报错")), row("info", "optional", "Not configured")]),
                                            .init(title: "HQ", rows: [row("blocking", tr("Shared check", "同名检查"), tr("Missing prerequisite", "缺少必要配置")), row("recommended", tr("Conversation capacity", "对话容量"), tr("Context is high", "上下文用量较高"))])],
                                 counts: ["ok": 1, "recommended": 2, "blocking": 1, "info": 1])
    }
    func testFindingsExcludePassedAndInformationAndRetainTargets() {
        let r = report()
        XCTAssertTrue(r.valid)
        XCTAssertEqual(r.attentionItems.map(\.row.status), ["blocking", "recommended", "recommended"])
        XCTAssertEqual(r.attentionItems.map(\.section), ["HQ", "Logs", "HQ"])
        XCTAssertEqual(Set(r.attentionItems.map(\.id)).count, 3)
        let targets = Set(r.displaySections.flatMap(\.rows).map(\.id))
        XCTAssertTrue(r.attentionItems.allSatisfy { targets.contains($0.target) })
        XCTAssertNotEqual(r.attentionItems[0].target, r.attentionItems[1].target)
    }
    func testCompletedRecommendationReportHasNoPendingCheckCopy() {
        let r = report()
        XCTAssertTrue(r.summary(lang: "zh").hasPrefix("检查已完成"))
        XCTAssertTrue(r.summary(lang: "zh").contains("2 项建议处理"))
        XCTAssertTrue(r.summary(lang: "zh").contains("1 项影响功能"))
        XCTAssertTrue(r.summary(lang: "zh").contains("1 项信息"))
        XCTAssertFalse(r.summary(lang: "zh").contains("待检查"))
        XCTAssertTrue(r.summary(lang: "en").contains("1 feature issue"))
        XCTAssertTrue(r.summary(lang: "en").contains("1 informational item"))
    }
    @MainActor func testFocusedFindingOpensItsFullDetails() throws {
        let item = report().attentionItems[0]
        let collapsed = ImageRenderer(content: EnvironmentHealthRow(row: item.row, l10n: .shared).frame(width: 500))
        let focused = ImageRenderer(content: EnvironmentHealthRow(row: item.row, l10n: .shared, focused: true).frame(width: 500))
        let a = try XCTUnwrap(collapsed.nsImage), b = try XCTUnwrap(focused.nsImage)
        XCTAssertGreaterThan(b.size.height, a.size.height, "Finding navigation must expose full guidance, not only highlight the same collapsed row")
    }
    @MainActor func testBilingualSummaryAndClassificationHelpRenderAtNarrowWidth() throws {
        let prior = L10n.shared.mode
        defer { L10n.shared.mode = prior }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let r = report(lang: L10n.shared.lang)
            let content = VStack(alignment: .leading, spacing: 16) {
                Text(r.summary(lang: L10n.shared.lang)).font(Theme.Font.session)
                EnvironmentFindingsSummary(findings: r.attentionItems, l10n: .shared, open: { _ in })
            }.padding(16).frame(width: 560).background(Color(nsColor: .windowBackgroundColor))
            let renderer = ImageRenderer(content: content.environment(\.colorScheme, .light))
            renderer.scale = 2
            let image = try XCTUnwrap(renderer.nsImage)
            XCTAssertEqual(image.size.width, 560, accuracy: 1)
            XCTAssertGreaterThan(image.size.height, 180)
            XCTAssertLessThan(image.size.height, 500)
            let help = ImageRenderer(content: EnvironmentResultClasses(l10n: .shared))
            XCTAssertNotNil(help.nsImage)
            if let dir = ProcessInfo.processInfo.environment["GTMUX_UI_EVIDENCE_DIR"], let tiff = image.tiffRepresentation,
               let bitmap = NSBitmapImageRep(data: tiff), let png = bitmap.representation(using: .png, properties: [:]) {
                try FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)
                try png.write(to: URL(fileURLWithPath: dir).appendingPathComponent("doctor-findings-\(mode.rawValue).png"))
            }
        }
    }
    @MainActor func testNarrowNativeWindowMaintainsContentBounds() throws {
        let prior = L10n.shared.mode
        defer { L10n.shared.mode = prior }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let suite = "gtmux-doctor-window-" + UUID().uuidString
            let prefs = UserDefaults(suiteName: suite)!
            defer { prefs.removePersistentDomain(forName: suite) }
            prefs.set(try JSONEncoder().encode(report(lang: L10n.shared.lang)), forKey: "environmentHealth.v1.report")
            let model = EnvironmentHealth(defaults: prefs, version: "1.2.3", run: { _ in
                XCTFail("Rendering a saved result must not run repairs or a background check")
                return (-1, "", "")
            })
            let host = NSHostingView(rootView: EnvironmentHealthView(model: model, l10n: .shared)
                .environment(\.colorScheme, .light))
            host.sizingOptions = []
            let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 560, height: 600),
                                  styleMask: [.titled, .closable], backing: .buffered, defer: false)
            window.isReleasedWhenClosed = false
            window.contentView = host
            window.setContentSize(NSSize(width: 560, height: 600))
            window.orderFront(nil)
            defer { window.close() }
            RunLoop.main.run(until: Date(timeIntervalSinceNow: 0.15))
            host.layoutSubtreeIfNeeded()
            XCTAssertEqual(host.bounds.width, 560, accuracy: 1)
            XCTAssertEqual(host.bounds.height, 600, accuracy: 1)
        }
    }

}
