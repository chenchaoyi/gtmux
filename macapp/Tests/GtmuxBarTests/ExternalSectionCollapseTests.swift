import XCTest
import SwiftUI
@testable import GtmuxBar

final class ExternalSectionCollapseTests: XCTestCase {
    private func defaults() -> UserDefaults {
        let name = "gtmux-collapse-test-" + UUID().uuidString
        let defaults = UserDefaults(suiteName: name)!
        addTeardownBlock { defaults.removePersistentDomain(forName: name) }
        return defaults
    }

    func testExternalGroupsPersistIndependentlyOfStatusGroups() {
        let d = defaults()
        d.set("idle,errored", forKey: "popover.collapsedSections")
        let collapse = SectionCollapse(defaults: d)
        XCTAssertFalse(collapse.isCollapsed(.external))
        collapse.toggle(.external)
        let reopened = SectionCollapse(defaults: d)
        XCTAssertTrue(reopened.isCollapsed(.external))
        XCTAssertFalse(reopened.isCollapsed(.elsewhere))
        XCTAssertTrue(reopened.isCollapsed(.idle))
        XCTAssertTrue(reopened.isCollapsed(.idle, errored: true))
        reopened.toggle(.elsewhere)
        reopened.toggle(.external)
        let again = SectionCollapse(defaults: d)
        XCTAssertFalse(again.isCollapsed(.external))
        XCTAssertTrue(again.isCollapsed(.elsewhere))
        XCTAssertTrue(again.isCollapsed(.idle))
    }

    func testActualListOmitsFoldedRowsButRetainsCountsAndOtherGroups() {
        let collapse = SectionCollapse(defaults: defaults())
        let store = AgentStore()
        store.setAgentsForTesting([
            Agent(paneID: "%1", agent: "Codex", status: "idle"),
            Agent(agent: "Codex", status: "idle", source: "native", client: "chatgpt_desktop", sessionID: "external-a"),
            Agent(agent: "Codex", status: "working", source: "native", client: "chatgpt_desktop", sessionID: "external-b"),
            Agent(agent: "Claude Code", status: "idle", source: "native", sessionID: "elsewhere")
        ])
        let view = MenuView(store: store, l10n: .shared, collapse: collapse, onJump: { _ in }, onAction: { _ in })
        func rows() -> [String] {
            view.listItems.compactMap { item in
                if case let .native(a) = item { return a.sessionID }
                return nil
            }
        }
        XCTAssertEqual(Set(rows()), ["external-a", "external-b", "elsewhere"])
        collapse.toggle(.external)
        XCTAssertEqual(rows(), ["elsewhere"])
        XCTAssertTrue(view.listItems.contains { if case .desktopHeader(2) = $0 { return true }; return false })
        XCTAssertTrue(view.listItems.contains { if case let .agent(a, _) = $0 { return a.paneID == "%1" }; return false })
        collapse.toggle(.elsewhere)
        XCTAssertTrue(rows().isEmpty)
        XCTAssertTrue(view.listItems.contains { if case .nativeHeader(1) = $0 { return true }; return false })
        collapse.toggle(.external)
        XCTAssertEqual(Set(rows()), ["external-a", "external-b"])
    }

    @MainActor func testHeaderRendersBothChoicesInBothLanguagesAndAppearances() throws {
        let old = L10n.shared.mode
        defer { L10n.shared.mode = old }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for dark in [false, true] {
                for collapsed in [false, true] {
                    let renderer = ImageRenderer(content: NativeHeader(count: 12, l10n: .shared, desktop: true, collapsed: collapsed, onToggle: {}).frame(width: 400).environment(\.colorScheme, dark ? .dark : .light))
                    let image = try XCTUnwrap(renderer.nsImage)
                    XCTAssertEqual(image.size.width, 400, accuracy: 1)
                    if let dir = ProcessInfo.processInfo.environment["GTMUX_EXTERNAL_SNAPSHOT_DIR"], let data = image.tiffRepresentation {
                        let rep = NSBitmapImageRep(data: data)
                        try rep?.representation(using: .png, properties: [:])?.write(to: URL(fileURLWithPath: dir).appendingPathComponent("external-\(mode)-\(dark)-\(collapsed).png"))
                    }
                }
            }
        }
    }
}
