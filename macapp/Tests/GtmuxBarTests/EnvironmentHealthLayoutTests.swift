import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class EnvironmentHealthLayoutTests: XCTestCase {
    private func row(_ label: String, _ value: String = "Installed", description: String? = nil) -> EnvironmentReport.Row {
        EnvironmentReport.Row(status: "ok", label: label, value: value, note: "Technical configuration path", description: description)
    }
    private func report(_ sections: [EnvironmentReport.Section]) -> EnvironmentReport {
        EnvironmentReport(schema: 1, version: "1.2.3", checkedAt: 1_791_819_000, sections: sections,
                          counts: ["ok": sections.flatMap(\.rows).count, "info": 0, "recommended": 0, "blocking": 0])
    }
    func testRowIdentityBelongsToItsSectionAndSurvivesInsertions() {
        let first = report([.init(title: "Install", rows: [row("CLI"), row("App")]),
                            .init(title: "HQ", rows: [row("Home"), row("Board")])])
        XCTAssertEqual(Set(first.displaySections.flatMap(\.rows).map(\.id)).count, 4)
        XCTAssertEqual(first.displaySections[1].rows.map(\.row.label), ["Home", "Board"])
        let inserted = report([.init(title: "Install", rows: [row("New check"), row("CLI"), row("App")]),
                               .init(title: "HQ", rows: [row("Home"), row("Board")])])
        XCTAssertEqual(first.displaySections[0].rows[0].id, inserted.displaySections[0].rows[1].id)
        XCTAssertEqual(first.displaySections[1].rows[1].id, inserted.displaySections[1].rows[1].id)
    }
    func testRepeatedLabelsAndHeadingsStillHaveDistinctIdentities() {
        let r = report([.init(title: "Group", rows: [row("Same"), row("Same")]),
                        .init(title: "Group", rows: [row("Same")])])
        XCTAssertEqual(Set(r.displaySections.map(\.id)).count, 2)
        XCTAssertEqual(Set(r.displaySections.flatMap(\.rows).map(\.id)).count, 3)
    }
    func testOldReportNotesAndLocalizedTime() throws {
        let json = #"{"schema":1,"version":"1.2.3","checked_at":1791819000,"sections":[{"title":"Tools","rows":[{"status":"ok","label":"CLI","value":"installed","note":"original note"}]}],"counts":{"ok":1,"recommended":0,"blocking":0,"info":0}}"#
        let r = try JSONDecoder().decode(EnvironmentReport.self, from: Data(json.utf8))
        XCTAssertTrue(r.valid)
        XCTAssertEqual(r.sections[0].rows[0].purpose, "original note")
        XCTAssertEqual(r.summary(lang: "zh"), "1 项正常")
        XCTAssertNotEqual(r.checkedTime(lang: "zh"), r.checkedTime(lang: "en"))
        XCTAssertTrue(r.checkedTime(lang: "zh").contains("年"))
        XCTAssertFalse(r.checkedTime(lang: "zh").contains("Oct"))
    }
    @MainActor func testPurposeWrapsRatherThanClips() throws {
        let short = ImageRenderer(content: EnvironmentHealthRow(row: row("Terminal history", "50000", description: "Checks retained output."), l10n: .shared).frame(width: 500))
        let long = ImageRenderer(content: EnvironmentHealthRow(row: row("Terminal history", "50000", description: String(repeating: "This checks retained terminal output and its configured limit. ", count: 5)), l10n: .shared).frame(width: 500))
        let a = try XCTUnwrap(short.nsImage), b = try XCTUnwrap(long.nsImage)
        XCTAssertGreaterThan(b.size.height, a.size.height)
        XCTAssertEqual(b.size.width, 500, accuracy: 1)
    }
    @MainActor func testGroupsRenderAtNarrowAndWideWidthsInBothLanguages() throws {
        let prior = L10n.shared.mode
        defer { L10n.shared.mode = prior }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let tr = L10n.shared.tr
            let r = report([
                .init(title: "gtmux", rows: [row(tr("Command-line tool", "命令行工具"), "v1.2.3", description: tr("Reports the version and location of the tool running these checks.", "确认执行本次检查的 gtmux 命令行版本及位置。")), row(tr("Menu-bar version", "菜单栏版本"), "v1.2.3", description: tr("Checks that app and command-line versions match.", "检查菜单栏应用与命令行工具的版本是否一致。"))]),
                .init(title: "tmux", rows: [row(tr("Terminal history", "终端历史长度"), "50000", description: tr("Checks the number of terminal output lines retained by tmux.", "检查 tmux 可保留的终端输出行数。"))]),
                .init(title: "HQ", rows: [row(tr("Situation board", "态势板"), tr("Updated 5 minutes ago", "5 分钟前更新"), description: tr("Checks board freshness and overly long cells.", "检查态势板更新时间，以及单元格内容是否过长。"))])])
            for width: CGFloat in [560, 760] {
                let content = VStack(alignment: .leading, spacing: 16) {
                    Text(tr("Environment check", "运行环境检查")).font(Theme.Font.title)
                    Text(r.summary(lang: L10n.shared.lang)).font(Theme.Font.task).foregroundStyle(.secondary)
                    ForEach(r.displaySections) { section in
                        EnvironmentHealthSection(section: section, l10n: .shared)
                    }
                }.padding(20).frame(width: width).background(Color(nsColor: .windowBackgroundColor))
                let renderer = ImageRenderer(content: content.environment(\.colorScheme, .light))
                renderer.scale = 2
                let image = try XCTUnwrap(renderer.nsImage)
                XCTAssertEqual(image.size.width, width, accuracy: 1)
                XCTAssertGreaterThan(image.size.height, 300)
                XCTAssertLessThan(image.size.height, 500, "Four checks should fit a compact reading window")
                if let dir = ProcessInfo.processInfo.environment["GTMUX_UI_EVIDENCE_DIR"],
                   let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff),
                   let png = bitmap.representation(using: .png, properties: [:]) {
                    try FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)
                    try png.write(to: URL(fileURLWithPath: dir).appendingPathComponent("environment-\(mode.rawValue)-\(Int(width)).png"))
                }
            }
        }
    }
}
