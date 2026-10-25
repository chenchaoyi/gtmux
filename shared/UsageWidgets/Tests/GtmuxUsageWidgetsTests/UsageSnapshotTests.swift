import Foundation
import XCTest
@testable import GtmuxUsageWidgets

final class UsageSnapshotTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 2_000_000_000)
    private func snapshot(_ json: String) throws -> UsageWidgetSnapshot {
        try .project(report: Data(json.utf8), server: "Studio", at: now)
    }

    func testOnlyCurrentProviderCapacityReachesSnapshot() throws {
        let value = try snapshot(#"{"limits":{"windows":[{"agent":"codex","agent_name":"Codex","label":"codex session","kind":"session","pct_used":42,"reset_unix":2000000600,"observed_at":1999999940},{"agent":"claude","label":"claude week","pct_used":98,"stale":true},{"agent":"custom","agent_name":"Custom agent","label":"custom day","pct_used":19,"reset_unix":2000086400},{"agent":"old","label":"old window","pct_used":100,"reset_unix":1999999999}]},"sessions":[{"prompt":"PRIVATE CONVERSATION"}],"token":"PRIVATE TOKEN","url":"https://private.example"}"#)
        XCTAssertEqual(value.accounts.map(\.agent), ["codex", "custom"])
        XCTAssertEqual(value.accounts[0].windows[0].percent, 42)
        let encoded = String(decoding: try JSONEncoder().encode(value), as: UTF8.self)
        for forbidden in ["PRIVATE", "private.example", "sessions", "token", "url"] {
            XCTAssertFalse(encoded.contains(forbidden), "Unexpected shared field: \(forbidden)")
        }
    }

    func testBalanceUnitsAndSeparateAccountsArePreserved() throws {
        let value = try snapshot(#"{"limits":{"balances":[{"agent":"dsh","agent_name":"DeepSeek Harness","account_id":"PRIVATE_ID","observed_at":1999999990,"wallets":[{"kind":"bonus","currency":"CNY","amount":"12.3400"},{"kind":"recharge","currency":"USD","amount":"0"}]},{"agent":"dsh","account_id":"OTHER_ID","observed_at":1999999990,"wallets":[{"kind":"recharge","currency":"CNY","amount":"3.1"}]}]}}"#)
        XCTAssertEqual(value.accounts.count, 2)
        XCTAssertEqual(value.accounts[0].wallets.map(\.amount), ["12.3400", "0"])
        XCTAssertEqual(value.accounts[0].wallets.map(\.currency), ["CNY", "USD"])
        XCTAssertTrue(value.accounts.allSatisfy { $0.windows.isEmpty })
        XCTAssertTrue(value.transitionDates(after: now).contains(Date(timeIntervalSince1970: 1999999990 + 900)), "Balances become stale from their observation, not the later cache write")
        let data = String(decoding: try JSONEncoder().encode(value), as: UTF8.self)
        XCTAssertFalse(data.contains("PRIVATE_ID")); XCTAssertFalse(data.contains("OTHER_ID"))
    }

    func testExpiredWindowNeverBecomesAZeroReading() throws {
        let value = try snapshot(#"{"limits":{"windows":[{"agent":"codex","label":"5h","pct_used":87,"reset_unix":2000000060}]}}"#)
        let window = try XCTUnwrap(value.accounts.first?.windows.first)
        XCTAssertFalse(window.expired(at: now))
        XCTAssertTrue(window.expired(at: now.addingTimeInterval(60)))
        XCTAssertEqual(window.percent, 87)
        XCTAssertTrue(value.transitionDates(after: now).contains(now.addingTimeInterval(60)))
        XCTAssertTrue(value.transitionDates(after: now).contains(now.addingTimeInterval(900)))
    }

    func testOldReadingAndFutureClockAreNotFresh() throws {
        let value = try snapshot(#"{"limits":{"windows":[{"agent":"codex","label":"5h","pct_used":10,"observed_at":1999998000}]}}"#)
        XCTAssertFalse(value.isStale(at: now))
        XCTAssertTrue(try XCTUnwrap(value.accounts.first?.windows.first).stale(at: now))
        XCTAssertTrue(value.isStale(at: now.addingTimeInterval(900)))
        XCTAssertTrue(value.isStale(at: now.addingTimeInterval(-301)))
        XCTAssertTrue(value.selected("unavailable").isEmpty)
        let future = try snapshot(#"{"limits":{"windows":[{"agent":"codex","label":"5h","pct_used":10,"observed_at":2000000900}]}}"#)
        XCTAssertTrue(future.accounts.isEmpty, "An implausible source clock cannot be laundered into a fresh local observation")
    }

    func testAutomaticPrioritizesCurrentReadingsOverOldHighUsage() throws {
        let value = try snapshot(#"{"limits":{"windows":[{"agent":"old","label":"week","pct_used":99,"observed_at":1999998000},{"agent":"current","label":"week","pct_used":19,"observed_at":1999999990}]}}"#)
        XCTAssertEqual(value.accounts.map(\.agent), ["current", "old"])
        XCTAssertEqual(value.accounts[1].windows[0].percent, 99, "Keep the dated fallback without ranking it above current capacity")
    }

    func testInvalidAndOversizedInputCannotReplaceStoredSnapshot() throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let store = UsageSnapshotStore(directory: directory)
        let valid = try snapshot(#"{"limits":{"windows":[{"agent":"codex","label":"5h","pct_used":42}]}}"#)
        try store.write(valid)
        XCTAssertThrowsError(try UsageWidgetSnapshot.project(report: Data(repeating: 32, count: 2 * 1024 * 1024 + 1), server: "Studio", at: now))
        XCTAssertEqual(store.read(), valid)
        var invalid = valid; invalid.accounts[0].windows[0].percent = .nan
        XCTAssertThrowsError(try store.write(invalid))
        XCTAssertEqual(store.read(), valid)
        try Data(repeating: 32, count: UsageWidgetSnapshot.maximumBytes + 1).write(to: directory.appendingPathComponent("usage-v1.json"))
        XCTAssertNil(store.read())
        store.clear(); XCTAssertFalse(FileManager.default.fileExists(atPath: directory.appendingPathComponent("usage-v1.json").path))
    }

    func testZeroCapacityIsDataAndMissingCapacityIsEmpty() throws {
        let zero = try snapshot(#"{"limits":{"windows":[{"agent":"future_provider","agent_name":"New provider","label":"quota","pct_used":0}]}}"#)
        XCTAssertEqual(zero.accounts.first?.windows.first?.percent, 0)
        XCTAssertTrue(try snapshot(#"{"limits":{}}"#).accounts.isEmpty)
        XCTAssertThrowsError(try snapshot(#"{"limits":{"windows":[{"agent":"codex","label":"quota","pct_used":true}]}}"#))
        var invalid = zero; invalid.schema = 99
        XCTAssertNil(invalid.validated())
    }

    func testConfigurationUsesNonemptyNamespacedProviderIdentity() throws {
        let value = try snapshot(#"{"limits":{"windows":[{"agent":"automatic","agent_name":"Custom agent","label":"quota","pct_used":10},{"agent":"dsh","label":"quota","pct_used":2}],"balances":[{"agent":"dsh","observed_at":1999999990,"wallets":[{"kind":"recharge","currency":"CNY","amount":"3.1"}]}]}}"#)
        let choices = UsageWidgetSelection.choices(in: value, automaticName: "Automatic")
        XCTAssertEqual(choices.count, 3, "Multiple wallets do not duplicate the provider selector")
        XCTAssertEqual(Set(choices.map(\.id)).count, choices.count)
        XCTAssertTrue(choices.allSatisfy { !$0.id.isEmpty })
        XCTAssertEqual(UsageWidgetSelection.provider(for: choices[0].id), "")
        XCTAssertEqual(UsageWidgetSelection.provider(for: choices[1].id), "automatic")
        XCTAssertEqual(value.selected(UsageWidgetSelection.provider(for: "provider:dsh")).count, 2)
        XCTAssertTrue(value.selected(UsageWidgetSelection.provider(for: "provider:absent")).isEmpty)
    }
}
