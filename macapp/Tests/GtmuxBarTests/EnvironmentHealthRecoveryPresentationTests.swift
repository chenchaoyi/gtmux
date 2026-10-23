import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

@MainActor final class EnvironmentHealthRecoveryPresentationTests: XCTestCase {
    func testHistoryAndOptionalFeaturesDoNotOfferUnnecessaryRepairs() async throws {
        NativeUIProbe.prepareAccessibility()
        let old = L10n.shared.mode
        defer { L10n.shared.mode = old }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let cn = mode == .zh
            let rows: [[String: String]] = [
                ["status":"ok", "label":cn ? "DeepSeek Harness 状态连接":"DeepSeek Harness state connection", "value":cn ? "已装":"Installed", "note":cn ? "检查状态连接组件；已有进程下次启动时加载。":"State components load on the next normal launch."],
                ["status":"info", "label":cn ? "DeepSeek Harness 终端会话":"DeepSeek Harness terminal sessions", "value":cn ? "当前没有接入 gtmux 的终端会话":"No terminal session is connected to gtmux", "note":cn ? "这是可选能力；读取 Web 历史不需要终端客户端。":"Optional; Web history does not need a terminal client."],
                ["status":"info", "label":cn ? "近期失败记录":"Recent failure records", "value":cn ? "24 小时内 3 次操作未成功 · 最近 08:30":"3 failed operations in 24h · last at 08:30", "note":cn ? "这是操作历史，不代表当前仍有故障。HQ 工作台操作 ×2，知识库操作 ×1。":"Operation history, not a current health failure. HQ workspace ×2, Knowledge operation ×1."]
            ]
            let object: [String: Any] = ["schema":1,"version":"9.0.1","checked_at":1792722000,"sections":[["title":cn ? "Agent 与通知":"Agents and notifications","rows":rows]],"counts":["ok":1,"info":2,"recommended":0,"blocking":0]]
            let json = String(data: try JSONSerialization.data(withJSONObject: object), encoding: .utf8)!
            for dark in [false, true] {
                let prefs = UserDefaults(suiteName: "gtmux-health-preview-" + UUID().uuidString)!
                let model = EnvironmentHealth(defaults: prefs, version: "9.0.1", run: { args in
                    if args == EnvironmentHealth.bridgeRefreshArguments { return (0, #"{"schema":1,"agent":"dsh","status":"updated"}"#, "") }
                    return (0,json,"")
                })
                model.autoCheck()
                for _ in 0..<100 where model.checking { try await Task.sleep(nanoseconds: 10_000_000) }
                XCTAssertFalse(model.failed)
                let host = NSHostingView(rootView: EnvironmentHealthView(model: model, l10n: .shared).frame(width:720,height:600).background(Color(nsColor:.windowBackgroundColor)).environment(\.colorScheme, dark ? .dark : .light))
                let window = NSWindow(contentRect: NSRect(x: 80,y:80,width:720,height:600), styleMask:[.titled,.resizable], backing:.buffered, defer:false)
                window.isReleasedWhenClosed = false
                window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
                window.contentView = host; window.orderBack(nil)
                defer {window.close()}
                host.layoutSubtreeIfNeeded()
                try await Task.sleep(nanoseconds:120_000_000)
                host.layoutSubtreeIfNeeded()
                XCTAssertNotNil(NativeUIProbe.find("environment-bridge-updated",in:host))
                XCTAssertNil(NativeUIProbe.find("environment-review-repairs",in:host))
                if let path=ProcessInfo.processInfo.environment["GTMUX_HEALTH_PREVIEW_DIR"] {
                    let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in:host.bounds))
                    host.cacheDisplay(in:host.bounds,to:bitmap)
                    try XCTUnwrap(bitmap.representation(using:.png,properties:[:])).write(to:URL(fileURLWithPath:path).appendingPathComponent("health-\(mode.rawValue)-\(dark ? "dark":"light").png"))
                }
            }
        }
    }
}
