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

    func testRecordedOutputDoesNotBecomeBalanceOrQuota() throws {
        let value = try snapshot(#"{"limits":{"dsh_account_status":"unsupported"},"history":{"days":[{"date":"2033-05-18"}],"scanned_at":1999999990,"by_agent":[{"agent_key":"dsh","agent_name":"DeepSeek Harness","today_out":0,"week_out":179534,"today_in":999,"account_id":"PRIVATE_ACCOUNT"},{"agent_key":"custom","agent_name":"Custom","today_out":23,"week_out":42}]}}"#)
        let dsh = try XCTUnwrap(value.selected("dsh").first)
        XCTAssertEqual(dsh.output, UsageWidgetOutput(day: "2033-05-18", today: 0, week: 179534))
        XCTAssertEqual(dsh.observedAt, 1999999990)
        XCTAssertTrue(dsh.wallets.isEmpty); XCTAssertTrue(dsh.windows.isEmpty)
        XCTAssertEqual(value.selected("custom").first?.output?.today, 23, "History support must not be hard-coded to DSH")
        XCTAssertEqual(UsageWidgetSelection.choices(in: value, automaticName: "Auto").count, 3)
        let encoded = String(decoding: try JSONEncoder().encode(value), as: UTF8.self)
        XCTAssertFalse(encoded.contains("PRIVATE_ACCOUNT")); XCTAssertFalse(encoded.contains("today_in"))
        XCTAssertEqual(try JSONDecoder().decode(UsageWidgetSnapshot.self, from: JSONEncoder().encode(value)), value)
    }

    func testOutputFallbackPreservesCapacityAndDoesNotInventMissingData() throws {
        let capacity = try snapshot(#"{"limits":{"windows":[{"agent":"codex","label":"session","pct_used":40}],"balances":[{"agent":"dsh","observed_at":1999999990,"wallets":[{"kind":"recharge","currency":"CNY","amount":"3.1"}]}]},"history":{"days":[{"date":"2033-05-18"}],"scanned_at":1999999990,"by_agent":[{"agent_key":"codex","today_out":99,"week_out":999},{"agent_key":"dsh","today_out":2,"week_out":20}]}}"#)
        XCTAssertEqual(capacity.accounts.count, 2)
        XCTAssertTrue(capacity.accounts.allSatisfy { $0.output == nil })
        XCTAssertEqual(capacity.selected("codex").first?.windows.first?.percent, 40)
        XCTAssertEqual(capacity.selected("dsh").first?.wallets.first?.amount, "3.1")
        for json in [#"{}"#, #"{"history":{"scanned_at":1999999990,"days":[{"date":"2033-05-18"}]}}"#,
                     #"{"history":{"scanned_at":1999999990,"days":[{"date":"2033-05-18"}],"by_agent":[{"agent_key":"dsh","week_out":99}]}}"#] {
            XCTAssertTrue(try snapshot(json).accounts.isEmpty, "Missing history must not become zero output")
        }
    }

    func testOutputDatesAndFreshnessKeepTheirSourceMeaning() throws {
        let value = try snapshot(#"{"history":{"days":[{"date":"2033-05-18"}],"scanned_at":1999998000,"by_agent":[{"agent_key":"dsh","today_out":5,"week_out":1234}]}}"#)
        let account = try XCTUnwrap(value.accounts.first)
        XCTAssertEqual(account.observedAt, 1999998000, "Writing a new snapshot cannot refresh an old ledger scan")
        var calendar = Calendar(identifier: .gregorian); calendar.timeZone = TimeZone(secondsFromGMT: 0)!
        let start = calendar.date(from: DateComponents(year: 2033, month: 5, day: 18))!
        let output = try XCTUnwrap(account.output)
        XCTAssertEqual(UsageWidgetWords.outputDay(output, at: start, chinese: true, calendar: calendar), "今日")
        XCTAssertEqual(UsageWidgetWords.outputDay(output, at: start.addingTimeInterval(86400), chinese: true, calendar: calendar), "2033-05-18")
        calendar.timeZone = TimeZone(secondsFromGMT: -4 * 3600)!
        XCTAssertEqual(UsageWidgetWords.outputDay(output, at: start, chinese: false, calendar: calendar), "2033-05-18",
                       "A server day ahead of the device must retain its date instead of being called today")
        XCTAssertEqual(UsageWidgetWords.count(0), "0")
        XCTAssertEqual(UsageWidgetWords.count(179534), "179.5K")
        let fresh = try snapshot(#"{"history":{"days":[{"date":"2033-05-18"}],"scanned_at":1999999990,"by_agent":[{"agent_key":"dsh","today_out":5,"week_out":1234}]}}"#)
        XCTAssertTrue(fresh.transitionDates(after: now).contains(Date(timeIntervalSince1970: 1999999990 + 900)))
        let midnight = Calendar.current.date(byAdding: .day, value: 1, to: Calendar.current.startOfDay(for: now))!
        XCTAssertTrue(fresh.transitionDates(after: now).contains(midnight))
    }

    func testInvalidOutputCannotMasqueradeAsAReading() throws {
        for (day, today, week, scanned) in [("2033-02-30", 1, 2, 1999999990), ("2033-05-18", -1, 2, 1999999990),
                                          ("2033-05-18", 3, 2, 1999999990), ("2033-05-18", 1, 2, 2000000900),
                                          ("2033-05-18", 1, 2, 0)] {
            let value = try snapshot("{\"history\":{\"days\":[{\"date\":\"\(day)\"}],\"scanned_at\":\(scanned),\"by_agent\":[{\"agent_key\":\"dsh\",\"today_out\":\(today),\"week_out\":\(week)}]}}")
            XCTAssertTrue(value.accounts.isEmpty)
        }
        var invalid = UsageWidgetSnapshot(server: "Demo", capturedAt: now.timeIntervalSince1970, accounts: [
            UsageWidgetAccount(id: "dsh", agent: "dsh", name: "DeepSeek", windows: [], wallets: [],
                               output: UsageWidgetOutput(day: "2033-05-18", today: 1, week: 2))])
        XCTAssertNil(invalid.validated(), "Output without its scan time is not a valid cached reading")
        invalid.accounts[0].observedAt = now.timeIntervalSince1970
        invalid.accounts[0].output?.day = "2033-02-30"
        XCTAssertNil(invalid.validated())
    }

    func testOutputIdentityCannotCollideWithACustomProvider() throws {
        let value = try snapshot(#"{"limits":{"windows":[{"agent":"dsh-output","label":"quota","pct_used":20}]},"history":{"days":[{"date":"2033-05-18"}],"scanned_at":1999999990,"by_agent":[{"agent_key":"dsh","today_out":0,"week_out":42}]}}"#)
        XCTAssertEqual(value.accounts.count, 2)
        XCTAssertNotNil(value.validated())
        XCTAssertEqual(Set(value.accounts.map(\.id)).count, 2)
    }
}
