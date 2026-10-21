import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class WhatsNewTests: XCTestCase {
    private func defaults() -> UserDefaults {
        let suite = "gtmux-whatsnew-tests-" + UUID().uuidString
        let prefs = UserDefaults(suiteName: suite)!
        addTeardownBlock { prefs.removePersistentDomain(forName: suite) }
        return prefs
    }
    private func receipt(_ entries: [MacReleaseNote]? = nil) -> String {
        let notes = entries ?? [
            MacReleaseNote(version: "4.0.3", en: ["Newest change"], zh: ["最新更新"]),
            MacReleaseNote(version: "4.0.2", en: [], zh: ["只有中文"]),
            MacReleaseNote(version: "4.0.1", en: ["Already installed"], zh: [])]
        return String(data: try! JSONEncoder().encode(MacReleaseNotes(schema: 1, since: "0.0.0", until: "4.0.3", limit: 30,
                        publishedThrough: "4.0.4", oldestAvailable: "4.0.0", entries: notes)), encoding: .utf8)!
    }
    @MainActor private func settle(_ model: WhatsNewStore) async {
        for _ in 0..<200 { if !model.loading { break }; try? await Task.sleep(nanoseconds: 10_000_000) }
        XCTAssertFalse(model.loading)
    }

    func testVersionOrderingAndLanguageFallback() {
        XCTAssertLessThan(ReleaseVersion("v1.0.9")!, ReleaseVersion("1.0.10")!)
        XCTAssertEqual(ReleaseVersion("v1.0.9"), ReleaseVersion("1.0.9"))
        for value in ["dev", "", "1.2", "1.2.3-rc", "1.2.-3", "1..3", "1.2.3.4", "１.2.3"] { XCTAssertNil(ReleaseVersion(value), value) }
        let note = MacReleaseNote(version: "4.0.2", en: [], zh: ["只有中文"])
        XCTAssertEqual(note.lines(lang: "en"), ["只有中文"])
        XCTAssertEqual(note.lines(lang: "zh"), ["只有中文"])
    }

    @MainActor func testFreshSameVersionAndDowngradeNeverPopOrFetch() {
        let prefs = defaults()
        let fresh = WhatsNewStore(defaults: prefs, version: "v4.0.3", run: { _ in XCTFail("fresh install must be quiet"); return (-1,"","") })
        fresh.autoCheck { XCTFail("fresh popup") }
        XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.3")
        let same = WhatsNewStore(defaults: prefs, version: "4.0.3", run: { _ in XCTFail("repeat fetch"); return (-1,"","") })
        same.autoCheck { XCTFail("repeat popup") }
        let older = WhatsNewStore(defaults: prefs, version: "4.0.2", run: { _ in XCTFail("downgrade fetch"); return (-1,"","") })
        older.autoCheck { XCTFail("downgrade popup") }
        older.acknowledge()
        XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.3")
    }

    @MainActor func testUpgradeMigratesLaunchMarkerShowsSkippedVersionsAndAcknowledgesOnlyOnDismissal() async {
        let prefs = defaults(); prefs.set("v4.0.1", forKey: "environmentHealth.v1.attempt")
        let ready = expectation(description: "upgrade notes"), json = receipt()
        let model = WhatsNewStore(defaults: prefs, version: "4.0.3", run: { args in
            XCTAssertEqual(args, ["whatsnew", "--all", "--until", "4.0.3", "--json"])
            return (0,json,"")
        })
        model.prepareLaunch()
        prefs.set("4.0.3", forKey: "environmentHealth.v1.attempt")
        model.autoCheck { ready.fulfill() }
        await fulfillment(of: [ready], timeout: 3)
        XCTAssertEqual(model.entries.map(\.version), ["4.0.3", "4.0.2"])
        XCTAssertTrue(model.upgradeDue)
        XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.1")
        model.acknowledge()
        XCTAssertFalse(model.upgradeDue)
        XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.3")
        let reopen = WhatsNewStore(defaults: prefs, version: "4.0.3", run: { _ in XCTFail("saved notes should open offline"); return (-1,"","") })
        reopen.autoCheck { XCTFail("already acknowledged") }
        reopen.showHistory()
        XCTAssertEqual(reopen.entries.map(\.version), ["4.0.3", "4.0.2", "4.0.1"])
        XCTAssertEqual(reopen.entries[1].lines(lang: "en"), ["只有中文"])
        XCTAssertFalse(reopen.failed)
    }

    @MainActor func testFailuresAndBadReceiptsPreserveUnreadVersionAndAllowManualRetry() async {
        let json = receipt()
        for output in [(-2,"", "timeout"), (1,json,"network failure"), (0,"{}",""),
                       (0,json.replacingOccurrences(of: "\"schema\":1", with: "\"schema\":2"),""),
                       (0,json.replacingOccurrences(of: "4.0.4", with: "4.0.2"),"")] {
            let prefs = defaults(); prefs.set("4.0.1", forKey: WhatsNewStore.seenKey)
            var calls = 0
            let failed = WhatsNewStore(defaults: prefs, version: "4.0.3", run: { _ in
                calls += 1
                return calls == 1 ? (Int32(output.0), output.1, output.2) : (0,json,"")
            })
            failed.autoCheck { XCTFail("failed read opened popup") }
            await settle(failed)
            failed.acknowledge()
            XCTAssertTrue(failed.failed)
            XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.1")
            failed.retry(); await settle(failed)
            XCTAssertFalse(failed.failed); XCTAssertEqual(failed.entries.count, 2)
            failed.showHistory(); XCTAssertEqual(failed.entries.count, 3)
            failed.acknowledge()
            XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.3")
        }
    }

    @MainActor func testAutomaticFailureRecoversOnLaterMenuOpeningWithoutAdvancingSeenOrRepeatingSuccess() async {
        let json = receipt()
        for output in [(Int32(1), ""), (Int32(-2), ""), (Int32(0), "{}") ] {
            let prefs = defaults(); prefs.set("4.0.1", forKey: WhatsNewStore.seenKey)
            var date = Date(timeIntervalSince1970: 1_000), calls = 0, presentations = 0
            let model = WhatsNewStore(defaults: prefs, version: "4.0.3", now: { date }, run: { _ in
                calls += 1
                return calls == 1 ? (output.0, output.1, "") : (0, json, "")
            })
            model.autoCheck { presentations += 1 }; await settle(model)
            XCTAssertTrue(model.failed)
            XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.1")
            date.addTimeInterval(59)
            model.autoCheck { presentations += 1 }
            XCTAssertEqual(calls, 1)
            date.addTimeInterval(1)
            model.autoCheck { presentations += 1 }; await settle(model)
            XCTAssertFalse(model.failed)
            XCTAssertEqual(calls, 2); XCTAssertEqual(presentations, 1)
            XCTAssertEqual(model.entries.map(\.version), ["4.0.3", "4.0.2"])
            XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.1")
            date.addTimeInterval(60)
            model.autoCheck { presentations += 1 }
            XCTAssertEqual(calls, 2); XCTAssertEqual(presentations, 1)
            model.acknowledge()
            XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.3")
        }
    }

    @MainActor func testAutomaticReadsDoNotOverlapAndManualHistoryCancelsFutureAutomaticRetry() async {
        let prefs = defaults(); prefs.set("4.0.1", forKey: WhatsNewStore.seenKey)
        let started = expectation(description: "automatic read started")
        let gate = DispatchSemaphore(value: 0), json = receipt()
        var date = Date(timeIntervalSince1970: 1_000), calls = 0
        let model = WhatsNewStore(defaults: prefs, version: "4.0.3", now: { date }, run: { _ in
            calls += 1
            if calls == 1 { started.fulfill(); _ = gate.wait(timeout: .now() + 3); return (1, "", "") }
            return (0, json, "")
        })
        model.autoCheck { XCTFail("failed read presented") }
        await fulfillment(of: [started], timeout: 2)
        date.addTimeInterval(60)
        model.autoCheck { XCTFail("overlapping read") }
        XCTAssertEqual(calls, 1)
        gate.signal(); await settle(model)
        model.showHistory(); await settle(model)
        date.addTimeInterval(60)
        model.autoCheck { XCTFail("manual history must not be replaced by automatic notes") }
        XCTAssertEqual(calls, 2); XCTAssertEqual(model.scope, .history)
    }

    @MainActor func testNoUserNotesIsQuietButMissingOlderHistoryIsDisclosed() async {
        let empty = receipt([])
        let prefs = defaults(); prefs.set("4.0.1", forKey: WhatsNewStore.seenKey)
        let model = WhatsNewStore(defaults: prefs, version: "4.0.3", run: { _ in (0,empty,"") })
        model.autoCheck { XCTFail("no notes popup") }; await settle(model)
        XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.3")
        let olderPrefs = defaults(); olderPrefs.set("3.9.0", forKey: WhatsNewStore.seenKey)
        let limited = WhatsNewStore(defaults: olderPrefs, version: "4.0.3", run: { _ in (0,empty,"") })
        let ready = expectation(description: "limited history notice")
        limited.autoCheck { ready.fulfill() }; await fulfillment(of: [ready], timeout: 3)
        XCTAssertTrue(limited.historyLimited)
        XCTAssertEqual(olderPrefs.string(forKey: WhatsNewStore.seenKey), "3.9.0")
    }

    func testReceiptRejectsFutureVersionsAndDuplicateOrUnorderedEntries() throws {
        let valid = try JSONDecoder().decode(MacReleaseNotes.self, from: Data(receipt().utf8))
        XCTAssertTrue(valid.valid(for: ReleaseVersion("4.0.3")!))
        let future = MacReleaseNote(version: "4.0.4", en: ["Not installed"], zh: [])
        for entries in [[future] + valid.entries, valid.entries + [valid.entries[0]], Array(valid.entries.reversed())] {
            let report = MacReleaseNotes(schema: 1, since: valid.since, until: valid.until, limit: 30,
                          publishedThrough: valid.publishedThrough, oldestAvailable: valid.oldestAvailable, entries: entries)
            XCTAssertFalse(report.valid(for: ReleaseVersion("4.0.3")!))
        }
    }

    @MainActor func testManualHistoryOutsideSourceRangeIsNotReportedAsNoChanges() async {
        let report = MacReleaseNotes(schema: 1, since: "0.0.0", until: "3.9.0", limit: 30,
                       publishedThrough: "4.0.4", oldestAvailable: "4.0.0", entries: [])
        let json = String(data: try! JSONEncoder().encode(report), encoding: .utf8)!
        let model = WhatsNewStore(defaults: defaults(), version: "3.9.0", run: { _ in (0,json,"") })
        model.showHistory(); await settle(model)
        XCTAssertFalse(model.failed); XCTAssertTrue(model.entries.isEmpty); XCTAssertTrue(model.historyLimited)
    }

    @MainActor func testClosingDuringReadDoesNotLoseNotesAndStaleAutomaticReadCannotReplaceHistory() async {
        let prefs = defaults(); prefs.set("4.0.1", forKey: WhatsNewStore.seenKey)
        let first = expectation(description: "first read"), returned = expectation(description: "old read returned")
        let gate = DispatchSemaphore(value: 0), lock = NSLock()
        let old = receipt([MacReleaseNote(version: "4.0.3", en: ["Old response"], zh: [])])
        let new = receipt([MacReleaseNote(version: "4.0.3", en: ["Current response"], zh: [])])
        var calls = 0
        let model = WhatsNewStore(defaults: prefs, version: "4.0.3", run: { _ in
            lock.lock(); calls += 1; let call = calls; lock.unlock()
            if call == 1 {
                first.fulfill(); _ = gate.wait(timeout: .now() + 3); returned.fulfill(); return (0,old,"")
            }
            return (0,new,"")
        })
        model.autoCheck { XCTFail("superseded automatic read showed popup") }
        await fulfillment(of: [first], timeout: 2)
        model.acknowledge()
        XCTAssertEqual(prefs.string(forKey: WhatsNewStore.seenKey), "4.0.1")
        model.showHistory(); await settle(model)
        XCTAssertEqual(model.scope, .history)
        XCTAssertEqual(model.entries.first?.en, ["Current response"])
        gate.signal(); await fulfillment(of: [returned], timeout: 2)
        // Let the old worker's main-queue completion run too.
        try? await Task.sleep(nanoseconds: 50_000_000)
        XCTAssertEqual(model.entries.first?.en, ["Current response"])
        XCTAssertEqual(model.scope, .history)
    }

    @MainActor func testNotesRenderInCompactResizableHostWithLongChineseAndBothThemes() async {
        let long = String(repeating: "会话更新说明支持折行。", count: 16)
        let json = receipt([MacReleaseNote(version: "4.0.3", en: [String(repeating: "A wrapped release note. ", count: 20)], zh: [long])])
        let model = WhatsNewStore(defaults: defaults(), version: "4.0.3", run: { _ in (0,json,"") })
        model.showHistory(); await settle(model)
        for scheme in [ColorScheme.light, .dark] {
            let host = NSHostingView(rootView: WhatsNewView(model: model, l10n: .shared, close: {}).environment(\.colorScheme, scheme))
            for width in [CGFloat(420), 640] {
                host.frame = NSRect(x: 0, y: 0, width: width, height: 360)
                host.layoutSubtreeIfNeeded()
                XCTAssertLessThanOrEqual(host.fittingSize.width, width + 1)
                XCTAssertGreaterThan(host.fittingSize.height, 0)
            }
            if let path = ProcessInfo.processInfo.environment["GTMUX_WHATSNEW_PREVIEW_DIR"] {
                let host = NSHostingView(rootView: WhatsNewView(model: model, l10n: .shared, close: {})
                    .environment(\.colorScheme, scheme).background(Color(nsColor: .windowBackgroundColor)))
                let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 520, height: 420),
                                      styleMask: [.borderless], backing: .buffered, defer: false)
                window.isReleasedWhenClosed = false
                window.appearance = NSAppearance(named: scheme == .light ? .aqua : .darkAqua)
                window.contentView = host
                host.frame = NSRect(x: 0, y: 0, width: 520, height: 420)
                host.layoutSubtreeIfNeeded()
                try? await Task.sleep(nanoseconds: 20_000_000)
                host.layoutSubtreeIfNeeded()
                let bitmap = try! XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                window.appearance?.performAsCurrentDrawingAppearance { host.cacheDisplay(in: host.bounds, to: bitmap) }
                let file = URL(fileURLWithPath: path).appendingPathComponent("whatsnew-\(scheme == .light ? "light" : "dark").png")
                try! bitmap.representation(using: .png, properties: [:])!.write(to: file)
                window.close()
            }
        }
    }

    @MainActor private func settleLayout(_ window: NSWindow) async {
        for _ in 0..<15 {
            window.contentView?.layoutSubtreeIfNeeded()
            try? await Task.sleep(nanoseconds: 10_000_000)
        }
    }

    @MainActor private func savePreview(_ window: NSWindow, _ name: String) throws {
        guard let path = ProcessInfo.processInfo.environment["GTMUX_WHATSNEW_PREVIEW_DIR"] else { return }
        let root = try XCTUnwrap(window.contentView?.superview)
        window.displayIfNeeded()
        root.displayIfNeeded()
        let bitmap = try XCTUnwrap(root.bitmapImageRepForCachingDisplay(in: root.bounds))
        (window.appearance ?? NSApp.effectiveAppearance).performAsCurrentDrawingAppearance { root.cacheDisplay(in: root.bounds, to: bitmap) }
        try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: path).appendingPathComponent(name + ".png"))
    }

    func testMeasuredHeightFitsProseAndRespectsSmallDisplays() {
        XCTAssertEqual(WhatsNewWindowLayout.contentHeight(desired: 900, visible: 1200, titleBar: 28), 900)
        XCTAssertEqual(WhatsNewWindowLayout.contentHeight(desired: 40, visible: 1200, titleBar: 28), 220)
        let small = WhatsNewWindowLayout.contentHeight(desired: 900, visible: 180, titleBar: 28)
        XCTAssertLessThanOrEqual(small + 28, 180 * 0.85)
        XCTAssertGreaterThan(small, 0)
    }

    @MainActor func testDelayedNotesResizeToActualWrappedContentInBothLanguagesAndThemes() async throws {
        NativeUIProbe.prepareAccessibility()
        let previous = L10n.shared.mode
        defer { L10n.shared.mode = previous }
        let notes = [MacReleaseNote(version: "4.0.3", en: [
            "Activity calls show a readable task purpose, while recorded arguments remain available to inspect and copy.",
            "Provider information keeps recorded account plans distinct from conversation modes.",
            "Browser copy preserves the original command and returns focus to the control.",
            "Long commands scroll inside the details without widening the whole conversation.",
            "Read-only sessions show their input capability before any action is available.",
            "The last update remains visible when the measured content fits the display."
        ], zh: [
            "工具调用显示易读的任务和等待摘要，原始参数记录仍可展开检查和复制。",
            "参与者资料明确标注已记录套餐，用量摘要与明细统一周额度和重置时间文案。",
            "浏览器复制保留原始命令，操作后仍可继续使用键盘。",
            "长命令在详情内滚动，不会把整个对话页面撑宽。",
            "只读会话先说明输入权限，再显示可用操作。",
            "最后一条更新也能完整显示，不会被底栏挡住。"
        ])]
        let json = receipt(notes)
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for dark in [false, true] {
                let gate = DispatchSemaphore(value: 0)
                let model = WhatsNewStore(defaults: defaults(), version: "4.0.3", run: { _ in
                    _ = gate.wait(timeout: .now() + 3); return (0,json,"")
                })
                let controller = WhatsNewController(model: model)
                let window = controller.show(l10n: .shared)
                window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
                defer { window.close() }
                await settleLayout(window)
                let waitingHeight = window.contentRect(forFrameRect: window.frame).height
                gate.signal(); await settle(model); await settleLayout(window)
                XCTAssertGreaterThan(window.contentRect(forFrameRect: window.frame).height, waitingHeight + 30)
                let last = try XCTUnwrap(NativeUIProbe.find("whatsnew-line-4.0.3-5", in: window))
                let scroll = try XCTUnwrap(NativeUIProbe.find("whatsnew-scroll", in: window))
                XCTAssertGreaterThan(last.accessibilityFrame().height, 0)
                XCTAssertGreaterThanOrEqual(last.accessibilityFrame().minY, scroll.accessibilityFrame().minY - 1)
                XCTAssertLessThanOrEqual(last.accessibilityFrame().maxY, scroll.accessibilityFrame().maxY + 1)
                XCTAssertNotNil(NativeUIProbe.find("whatsnew-header-version", in: window))
                XCTAssertNil(NativeUIProbe.find("whatsnew-version-4.0.3", in: window), "a single release must not repeat its version")
                let visible = window.screen?.visibleFrame ?? NSScreen.main!.visibleFrame
                XCTAssertTrue(visible.insetBy(dx: -1, dy: -1).contains(window.frame))
                try savePreview(window, "measured-\(mode.rawValue)-\(dark ? "dark" : "light")")
            }
        }
    }

    @MainActor func testLongHistoryRemainsScrollableAndKeepsOlderDisclosureAcrossRefresh() async throws {
        NativeUIProbe.prepareAccessibility()
        let lines = (0..<20).map { "Recorded change \($0 + 1): " + String(repeating: "A long update stays selectable and wraps within its window. ", count: 3) }
        let json = receipt([MacReleaseNote(version: "4.0.3", en: lines, zh: []), MacReleaseNote(version: "4.0.2", en: ["An older change remains accessible."], zh: [])])
        let model = WhatsNewStore(defaults: defaults(), version: "4.0.3", run: { _ in (0,json,"") })
        let controller = WhatsNewController(model: model), window = controller.show(l10n: .shared)
        defer { window.close() }
        await settle(model); await settleLayout(window)
        XCTAssertNil(NativeUIProbe.find("whatsnew-header-version", in: window), "grouped notes only label each version once")
        XCTAssertNil(NativeUIProbe.find("whatsnew-line-4.0.2-0", in: window))
        let old = try XCTUnwrap(NativeUIProbe.find("whatsnew-version-4.0.2", in: window))
        XCTAssertTrue(old.accessibilityPerformPress()); await settleLayout(window)
        model.retry(); await settle(model); await settleLayout(window)
        XCTAssertNotNil(NativeUIProbe.find("whatsnew-line-4.0.2-0", in: window), "same-version refresh retains the reader's disclosure")
        let scroll = try XCTUnwrap(NativeUIProbe.views(NSScrollView.self, in: window.contentView!).first)
        let document = try XCTUnwrap(scroll.documentView)
        XCTAssertGreaterThan(document.bounds.height, scroll.contentView.bounds.height)
        scroll.contentView.scroll(to: NSPoint(x: 0, y: max(0, document.bounds.height - scroll.contentView.bounds.height)))
        scroll.reflectScrolledClipView(scroll.contentView); await settleLayout(window)
        let last = try XCTUnwrap(NativeUIProbe.find("whatsnew-line-4.0.2-0", in: window))
        let clip = try XCTUnwrap(NativeUIProbe.find("whatsnew-scroll", in: window))
        XCTAssertGreaterThan(last.accessibilityFrame().height, 0)
        XCTAssertGreaterThan(clip.accessibilityFrame().height, 0)
        XCTAssertGreaterThanOrEqual(last.accessibilityFrame().minY, clip.accessibilityFrame().minY - 1)
        XCTAssertLessThanOrEqual(last.accessibilityFrame().maxY, clip.accessibilityFrame().maxY + 1)
        try savePreview(window, "history-scrolled-to-last")
    }

    @MainActor func testManualSizeWinsWhenNotesAreRefreshed() async {
        var calls = 0
        let short = receipt([MacReleaseNote(version: "4.0.3", en: ["Short note"], zh: [])])
        let long = receipt([MacReleaseNote(version: "4.0.3", en: Array(repeating: "More recorded updates after refresh.", count: 20), zh: [])])
        let model = WhatsNewStore(defaults: defaults(), version: "4.0.3", run: { _ in
            calls += 1; return (0, calls == 1 ? short : long, "")
        })
        let controller = WhatsNewController(model: model), window = controller.show(l10n: .shared)
        defer { window.close() }
        await settle(model); await settleLayout(window)
        controller.windowWillStartLiveResize(Notification(name: NSWindow.willStartLiveResizeNotification, object: window))
        window.setContentSize(NSSize(width: 640, height: 360))
        model.retry(); await settle(model); await settleLayout(window)
        XCTAssertEqual(window.contentRect(forFrameRect: window.frame).height, 360, accuracy: 1)
        XCTAssertEqual(window.contentRect(forFrameRect: window.frame).width, 640, accuracy: 1)
    }

    @MainActor func testPlainDismissActionRetainsReturnAndEscapeShortcuts() async throws {
        NativeUIProbe.prepareAccessibility()
        let json = receipt([MacReleaseNote(version: "4.0.3", en: ["A readable update."], zh: [])])
        for (characters, keyCode) in [("\r", UInt16(36)), ("\u{1b}", UInt16(53))] {
            let model = WhatsNewStore(defaults: defaults(), version: "4.0.3", run: { _ in (0,json,"") })
            let controller = WhatsNewController(model: model), window = controller.show(l10n: .shared)
            defer { withExtendedLifetime(controller) { window.close() } }
            await settle(model); await settleLayout(window)
            window.makeFirstResponder(window.contentView)
            let event = try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: 0,
                windowNumber: window.windowNumber, context: nil, characters: characters,
                charactersIgnoringModifiers: characters, isARepeat: false, keyCode: keyCode))
            // XCTest does not own a key window. Route through this window's native
            // key-equivalent and responder dispatch, as AppKit does for its key window.
            if !window.performKeyEquivalent(with: event) { window.sendEvent(event) }
            await settleLayout(window)
            XCTAssertFalse(window.isVisible, "Key code \(keyCode) must dismiss the plain action")
        }
    }

}
