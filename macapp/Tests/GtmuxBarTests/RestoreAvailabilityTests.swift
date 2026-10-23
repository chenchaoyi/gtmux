import AppKit
import SwiftUI
import Combine
import Foundation
import XCTest
@testable import GtmuxBar

final class RestoreAvailabilityTests: XCTestCase {
    private let pending = Data(#"{"savePath":"/fixture/last","sessions":[{"name":"workspace","windows":2,"panes":3,"state":"missing"}]}"#.utf8)
    private let empty = Data(#"{"savePath":"","sessions":null}"#.utf8)

    private func refresh(_ store: AgentStore) {
        let published = expectation(description: "pending restore plan published")
        let subscription = store.$restorePlan.dropFirst().sink { _ in published.fulfill() }
        store.refreshRestorePlan()
        wait(for: [published], timeout: 2)
        subscription.cancel()
    }

    func testExternalHistoriesAndPartialFleetDoNotHideRestore() throws {
        for rows in [
            #"[{"agent":"DeepSeek Harness","source":"native","status":"idle","session_id":"history"}]"#,
            #"[{"agent":"Codex","source":"tmux","pane_id":"%fixture","status":"working"}]"#,
        ] {
            let store = AgentStore(readPoll: { args in
                XCTAssertEqual(args, ["restore", "--plan", "--pending", "--json"])
                return self.pending
            })
            store.setAgentsForTesting(try JSONDecoder().decode([Agent].self, from: Data(rows.utf8)))
            refresh(store)
            XCTAssertTrue(store.showsRestore)
            XCTAssertEqual(store.restorePlan?.sessions.map(\.name), ["workspace"])
            XCTAssertEqual(store.restorePlan?.sessions.first?.agents, [], "plain shells decode without agent entries")
        }
    }

    func testKnownEmptyHidesButRunningRestoreStaysVisible() {
        let store = AgentStore(readPoll: { _ in self.empty })
        XCTAssertTrue(store.showsRestore, "unknown keeps the quiet fallback")
        refresh(store)
        XCTAssertNotNil(store.restorePlan, "Go's null session slice is known empty")
        XCTAssertFalse(store.showsRestore)
        store.beginRestore()
        store.setAgentsForTesting([Agent(paneID: "%returned", agent: "Codex")])
        XCTAssertTrue(store.showsRestore, "restored rows must not hide progress")
        store.endRestore()
        XCTAssertFalse(store.showsRestore)
    }

    func testFailedReadBecomesUnknownThenRecovers() {
        let lock = NSLock()
        var result: Data? = empty
        let store = AgentStore(readPoll: { _ in
            lock.lock(); defer { lock.unlock() }
            return result
        })
        refresh(store)
        XCTAssertFalse(store.showsRestore)
        lock.lock(); result = nil; lock.unlock()
        refresh(store)
        XCTAssertNil(store.restorePlan, "failed topology query must not mean no recoverable sessions")
        XCTAssertTrue(store.showsRestore)
        lock.lock(); result = pending; lock.unlock()
        refresh(store)
        XCTAssertEqual(store.restorePlan?.sessions.count, 1)
    }

    func testRestoreReadNeverRunsOnFastRadarRefresh() {
        let store = AgentStore(readPoll: { args in
            XCTAssertEqual(args, ["agents", "--json"])
            return Data("[]".utf8)
        })
        let published = expectation(description: "radar refreshed")
        let subscription = store.$agents.dropFirst().sink { _ in published.fulfill() }
        store.refresh()
        wait(for: [published], timeout: 2)
        subscription.cancel()
        XCTAssertNil(store.restorePlan)
    }

    func testSubscriberCanImmediatelyRequestTheNextRead() {
        let store = AgentStore(readPoll: { _ in self.pending })
        let published = expectation(description: "both reads publish")
        published.expectedFulfillmentCount = 2
        var publications = 0
        let subscription = store.$restorePlan.dropFirst().sink { _ in
            publications += 1
            if publications == 1 { store.refreshRestorePlan() }
            published.fulfill()
        }
        store.refreshRestorePlan()
        wait(for: [published], timeout: 2)
        subscription.cancel()
    }

    func testSlowRestoreReadDoesNotOverlap() {
        let started = expectation(description: "restore read started")
        let published = expectation(description: "restore published")
        let overlap = expectation(description: "no overlapping restore read")
        overlap.isInverted = true
        let release = DispatchSemaphore(value: 0), lock = NSLock()
        var calls = 0
        let store = AgentStore(readPoll: { _ in
            lock.lock(); calls += 1; let index = calls; lock.unlock()
            if index == 1 { started.fulfill() } else { overlap.fulfill() }
            _ = release.wait(timeout: .now() + 2)
            return self.pending
        })
        let subscription = store.$restorePlan.dropFirst().sink { _ in published.fulfill() }
        store.refreshRestorePlan()
        wait(for: [started], timeout: 1)
        for _ in 0..<20 { store.refreshRestorePlan() }
        wait(for: [overlap], timeout: 0.15)
        release.signal()
        wait(for: [published], timeout: 2)
        subscription.cancel()
    }
}


@MainActor final class RestoreAvailabilityPresentationTests: XCTestCase {
    private func settle(_ host: NSView) async throws {
        host.layoutSubtreeIfNeeded()
        try await Task.sleep(nanoseconds: 120_000_000)
        host.layoutSubtreeIfNeeded()
    }

    func testPendingSectionExpandsAndStaysUntilRecoveryCompletes() async throws {
        NativeUIProbe.prepareAccessibility()
        let oldLanguage = L10n.shared.mode
        defer { L10n.shared.mode = oldLanguage }
        for language in [LangMode.en, .zh] {
            L10n.shared.mode = language
            for dark in [false, true] {
                let lock = NSLock()
                var empty = false
                let store = AgentStore(readPoll: { _ in
                    lock.lock(); defer { lock.unlock() }
                    return Data((empty ? #"{"savePath":"/fixture/last","sessions":null}"# : #"{"savePath":"/fixture/last","sessions":[{"name":"Research","windows":2,"panes":3,"state":"missing"},{"name":"Notes","windows":1,"panes":1,"state":"detached"}]}"#).utf8)
                })
                store.setAgentsForTesting([Agent(agent: "DeepSeek Harness", source: "native")])
                store.refreshRestorePlan()
                for _ in 0..<50 where store.restorePlan == nil { try await Task.sleep(nanoseconds: 10_000_000) }
                var restores = 0
                let root = RestoreSection(store: store, l10n: .shared, onRestore: { restores += 1 })
                    .frame(width: 420).fixedSize(horizontal: false, vertical: true)
                    .background(Color(nsColor: .windowBackgroundColor)).environment(\.colorScheme, dark ? .dark : .light)
                let host = NSHostingView(rootView: root)
                let window = NSWindow(contentRect: NSRect(x: 80, y: 80, width: 420, height: 160), styleMask: [.titled], backing: .buffered, defer: false)
                window.isReleasedWhenClosed = false
                window.title = "gtmux synthetic restore preview"
                window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
                window.contentView = host; window.orderBack(nil)
                defer { window.close() }
                try await settle(host)
                let action = try XCTUnwrap(NativeUIProbe.find("restore-action", in: host))
                XCTAssertTrue(action.accessibilityPerformPress())
                XCTAssertEqual(restores, 1, "preview only records intent; it never runs restore")
                XCTAssertNil(NativeUIProbe.find("restore-plan", in: host))
                XCTAssertTrue(try XCTUnwrap(NativeUIProbe.find("restore-expand", in: host)).accessibilityPerformPress())
                try await settle(host)
                XCTAssertNotNil(NativeUIProbe.find("restore-plan", in: host))
                if let path = ProcessInfo.processInfo.environment["GTMUX_RESTORE_PREVIEW_DIR"] {
                    let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                    host.cacheDisplay(in: host.bounds, to: bitmap)
                    try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: path).appendingPathComponent("restore-\(language.rawValue)-\(dark ? "dark" : "light").png"))
                }
                store.beginRestore()
                lock.withLock { empty = true }
                store.refreshRestorePlan()
                try await settle(host)
                XCTAssertNotNil(NativeUIProbe.find("restore-busy", in: host))
                XCTAssertNil(NativeUIProbe.find("restore-action", in: host))
                store.endRestore()
                try await settle(host)
                XCTAssertFalse(store.showsRestore)
                XCTAssertNil(NativeUIProbe.find("restore-busy", in: host))
            }
        }
    }
}
