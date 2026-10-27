import XCTest
import MacUsageWidgetSupport
import GtmuxUsageWidgets
import SwiftUI
import WidgetKit

final class MacUsageWidgetSupportTests: XCTestCase {
    @MainActor
    func testNativeWidgetFamiliesRenderWithRealSharedContent() throws {
        let now = Date(timeIntervalSince1970: 1_792_998_000)
        let snapshot = UsageWidgetSnapshot(server: "This Mac", capturedAt: now.timeIntervalSince1970, accounts: [
            UsageWidgetAccount(id: "codex", agent: "codex", name: "Codex", windows: [
                UsageWidgetWindow(label: "Weekly", kind: "week", percent: 72, observedAt: now.timeIntervalSince1970)
            ], wallets: []),
            UsageWidgetAccount(id: "dsh", agent: "dsh", name: "DeepSeek Harness", windows: [], wallets: [],
                               observedAt: now.timeIntervalSince1970, output: UsageWidgetOutput(day: "2026-10-27", today: 12000, week: 64000))
        ])
        let output = ProcessInfo.processInfo.environment["GTMUX_WIDGET_RENDER_DIR"].map { URL(fileURLWithPath: $0) }
        if let output { try FileManager.default.createDirectory(at: output, withIntermediateDirectories: true) }
        for (name, family, width) in [("small", WidgetFamily.systemSmall, 170.0), ("medium", .systemMedium, 364.0)] {
            for dark in [false, true] {
                for chinese in [false, true] {
                    let view = UsageWidgetContent(snapshot: snapshot, provider: "", date: now, family: family, chinese: chinese)
                        .padding(16).frame(width: width, height: 170)
                        .background(dark ? Color.black : Color.white)
                        .environment(\.colorScheme, dark ? .dark : .light)
                    let renderer = ImageRenderer(content: view)
                    renderer.scale = 2
                    let image = try XCTUnwrap(renderer.nsImage)
                    XCTAssertEqual(image.size.width, width)
                    XCTAssertEqual(image.size.height, 170)
                    if let output, let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff),
                       let png = bitmap.representation(using: .png, properties: [:]) {
                        try png.write(to: output.appendingPathComponent("\(name)-\(dark ? "dark" : "light")-\(chinese ? "zh" : "en").png"))
                    }
                }
            }
        }
    }
    func testOnlySignerScopedMacContainerIsAccepted() {
        XCTAssertEqual(MacUsageWidgetContainer.groupIdentifier(team: "SYNTH12345", group: "SYNTH12345.com.gtmux.menubar.usage"), "SYNTH12345.com.gtmux.menubar.usage")
        for (team, group) in [("", ""), ("SYNTH12345", "group.com.gtmux.app.usage"), ("OTHER12345", "SYNTH12345.com.gtmux.menubar.usage"), ("../invalid", "../invalid.com.gtmux.menubar.usage"), ("SYNTH12345\n", "SYNTH12345\n.com.gtmux.menubar.usage")] {
            XCTAssertNil(MacUsageWidgetContainer.groupIdentifier(team: team, group: group))
        }
    }

    func testUsageShortcutCannotCarryAnAlternateActionOrCredentials() {
        for text in ["gtmux://usage", "gtmux://usage/"] { XCTAssertTrue(MacUsageWidgetContainer.opensUsage(URL(string: text)!)) }
        for text in ["https://usage", "gtmux://other", "gtmux://usage/send", "gtmux://usage?token=x", "gtmux://usage#x", "gtmux://user@usage", "gtmux://usage:80", "gtmux://usage/%2e%2e/send"] {
            XCTAssertFalse(MacUsageWidgetContainer.opensUsage(URL(string: text)!))
        }
    }

    func testNoPollingWithoutWidgetNoOverlapAndBoundedReloads() {
        var policy = MacUsageRefreshPolicy()
        let t = Date(timeIntervalSince1970: 1_000)
        XCTAssertFalse(policy.begin(configured: false, at: t))
        XCTAssertTrue(policy.begin(configured: true, at: t))
        XCTAssertFalse(policy.begin(configured: true, at: t.addingTimeInterval(90)))
        XCTAssertTrue(policy.finish(published: true, at: t))
        XCTAssertFalse(policy.begin(configured: true, at: t.addingTimeInterval(59)))
        XCTAssertTrue(policy.begin(configured: true, at: t.addingTimeInterval(60)))
        XCTAssertFalse(policy.finish(published: true, at: t.addingTimeInterval(60)))
        XCTAssertTrue(policy.begin(configured: true, at: t.addingTimeInterval(300)))
        XCTAssertFalse(policy.finish(published: false, at: t.addingTimeInterval(300)))
        XCTAssertTrue(policy.begin(configured: true, at: t.addingTimeInterval(360)))
        XCTAssertTrue(policy.finish(published: true, at: t.addingTimeInterval(360)))
    }
}
