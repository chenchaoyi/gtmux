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
}
