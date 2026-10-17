import XCTest
@testable import GtmuxBar

@MainActor final class AccessHistoryTests: XCTestCase {
    private func settle(_ store: AccessHistoryStore) async throws {
        for _ in 0..<200 { if !store.loading { return }; try await Task.sleep(nanoseconds: 5_000_000) }
        XCTFail("history read did not finish")
    }
    func testSharedPageDecodesAndEarlierReadsPreserveUniqueRecords() async throws {
        let page = #"{"entries":[{"id":"one","at":"2026-10-16T10:00:00+08:00","actor":"phone:a","name":"My iPhone","scope":"device","label_en":"Read terminal","label_zh":"读取终端画面","outcome":"ok","device_id":"device-a","source_ip":"203.0.113.1","target":"%21","status":200}],"next":"earlier","retain_days":30}"#
        let store = AccessHistoryStore { args in
            XCTAssertEqual(Array(args.prefix(5)), ["logs", "--access", "--json", "--limit", "100"])
            return (0, page, "")
        }
        store.load(); try await settle(store)
        XCTAssertEqual(store.entries.first?.sourceIp, "203.0.113.1")
        XCTAssertEqual(store.entries.first?.labelZh, "读取终端画面")
        store.load(earlier: true); try await settle(store)
        XCTAssertEqual(store.entries.count, 1)
        XCTAssertEqual(store.next, "earlier")
        XCTAssertFalse(store.failed)
    }
    func testFailedReadDoesNotBecomeEmptyHistory() async throws {
        let store = AccessHistoryStore { _ in (1, "", "unavailable") }
        store.load(); try await settle(store)
        XCTAssertTrue(store.failed)
        XCTAssertTrue(store.entries.isEmpty)
    }
}

final class AccessHistoryPresentationTests: XCTestCase {
    private func record(_ id: String, _ at: String) -> AccessHistoryEntry {
        AccessHistoryEntry(id: id, at: at, actor: "phone:device-a", name: "My iPhone", scope: "device",
            labelEn: "Read conversation", labelZh: "读取对话", outcome: "ok", deviceId: "device-full-identity",
            platform: "iOS 26.6", sourceIp: "203.0.113.8", method: "GET", target: "%21", status: 200, durationMs: 18)
    }
    func testDateLanguageAndLocalBoundaryAreIndependentOfSystemLocale() {
        let entries = [record("a", "2026-10-16T16:00:01.123Z"), record("b", "2026-10-16T16:00:01Z"), record("c", "2026-10-16T15:59:59Z")]
        let lines = AccessHistoryPresentation.lines(entries, zh: true, timeZone: TimeZone(secondsFromGMT: 8 * 3600)!)
        XCTAssertEqual(lines.map(\.clock), ["00:00:01", "00:00:01", "23:59:59"])
        XCTAssertEqual(lines.map(\.day), ["2026年10月17日", nil, "2026年10月16日"])
        XCTAssertEqual(lines.map(\.id), ["a", "b", "c"])
        XCTAssertEqual(AccessHistoryPresentation.lines(entries, zh: false, timeZone: TimeZone(secondsFromGMT: 0)!)[0].day, "Oct 16, 2026")
        let invalid = AccessHistoryPresentation.lines([record("x", "bad-clock")], zh: true)
        XCTAssertEqual(invalid[0].clock, "bad-clock"); XCTAssertEqual(invalid[0].day, "时间未知")
    }
    func testMetadataAndCopyRetainFullIdentityWithoutDisclosure() {
        let e = record("a", "2026-10-17T10:00:00+08:00")
        XCTAssertEqual(AccessHistoryPresentation.metadata(e, zh: false), "iOS 26.6 · 203.0.113.8 · Device device-full-identity · GET · 18 ms")
        let copy = AccessHistoryPresentation.copyText(e, zh: true)
        for value in [e.at, "My iPhone", "读取对话", "%21", "device-full-identity", "200"] { XCTAssertTrue(copy.contains(value)) }
        let failed = AccessHistoryEntry(id: "failed", at: e.at, actor: "owner", name: "Owner credential", scope: "master", labelEn: "Download HQ archive", labelZh: "下载 HQ 档案", outcome: "failed", status: 200)
        XCTAssertEqual(AccessHistoryPresentation.outcome(failed, zh: true), "失败 · 200")
        XCTAssertEqual(AccessHistoryPresentation.name(failed, zh: true), "所有者凭据")
    }
}

@MainActor final class AccessHistoryFilterTests: XCTestCase {
    private let grouped = #"{"entries":[],"groups":[{"key":"device:a","actor":"phone:a","device_id":"a","name":"Phone","scope":"device","count":3,"accepted":2,"refused":0,"failed":1,"first_at":"2026-10-16T10:00:00Z","last_at":"2026-10-17T10:00:00Z"}],"query":{"group_by":"device"},"next":"older","retain_days":30}"#
    private func settle(_ store: AccessHistoryStore) async throws {
        for _ in 0..<200 { if !store.loading { return }; try await Task.sleep(nanoseconds: 5_000_000) }
        XCTFail("read did not finish")
    }
    func testGroupPagesSumAndDrillDownUsesExactID() async throws {
        let response = grouped
        let store = AccessHistoryStore { args in
            if args.contains("--group-by") { return (0, response, "") }
            XCTAssertTrue(args.contains("--device-id")); XCTAssertTrue(args.contains("a")); XCTAssertFalse(args.contains("--before"))
            return (0, #"{"entries":[],"groups":[],"query":{"device_id":"a"},"retain_days":30}"#, "")
        }
        store.select(.init(groupBy: "device")); try await settle(store)
        store.load(earlier: true); try await settle(store)
        XCTAssertEqual(store.groups[0].count, 6)
        store.selectGroup(store.groups[0]); try await settle(store)
        XCTAssertEqual(store.query.deviceId, "a"); XCTAssertNil(store.query.groupBy); XCTAssertFalse(store.failed)
    }
    func testUnsupportedQueryDoesNotPresentUnfilteredResults() async throws {
        let store = AccessHistoryStore { _ in (0, #"{"entries":[],"retain_days":30}"#, "") }
        store.select(.init(deviceId: "full-id")); try await settle(store)
        XCTAssertTrue(store.failed); XCTAssertTrue(store.unsupported); XCTAssertTrue(store.entries.isEmpty)
    }
    func testGroupingPreservesLatestMetadataAndSeparatesSameNames() throws {
        let decoder = JSONDecoder(); decoder.keyDecodingStrategy = .convertFromSnakeCase
        let g = try decoder.decode(AccessHistoryPage.self, from: Data(grouped.utf8)).groups![0]
        var older = g; older.name = "Older name"; older.lastAt = "2026-10-16T11:00:00Z"
        let result = AccessHistoryGrouping.merge([g], [older], sum: true)
        XCTAssertEqual(result[0].name, "Phone"); XCTAssertEqual(result[0].count, 6)
        XCTAssertEqual(AccessHistoryGrouping.merge([g], [older], sum: false)[0].count, 3)
        let other = try decoder.decode(AccessHistoryPage.self, from: Data(grouped.replacingOccurrences(of: "device:a", with: "device:b").replacingOccurrences(of: "\"device_id\":\"a\"", with: "\"device_id\":\"b\"").utf8)).groups![0]
        XCTAssertEqual(AccessHistoryGrouping.merge([g], [other], sum: true).map(\.key), ["device:a", "device:b"])
    }
}

@MainActor final class AccessHistoryQueryRaceTests: XCTestCase {
    func testLateOldQueryCannotReplaceNewFilter() async throws {
        let started = DispatchSemaphore(value: 0), release = DispatchSemaphore(value: 0)
        defer { release.signal() }
        let store = AccessHistoryStore { args in
            let old = args.contains("send")
            if old { started.signal(); _ = release.wait(timeout: .now() + 3) }
            let op = old ? "send" : "terminal"
            let name = old ? "Stale result" : "Filtered result"
            return (0, "{\"entries\":[{\"id\":\"one\",\"at\":\"2026-10-17T10:00:00Z\",\"actor\":\"a\",\"name\":\"\(name)\",\"scope\":\"device\",\"label_en\":\"Read\",\"label_zh\":\"读取\",\"outcome\":\"ok\"}],\"query\":{\"operation\":\"\(op)\"},\"retain_days\":30}", "")
        }
        store.select(.init(operation: "send"))
        XCTAssertEqual(started.wait(timeout: .now() + 1), .success)
        store.select(.init(operation: "terminal"))
        for _ in 0..<200 { if !store.loading { break }; try await Task.sleep(nanoseconds: 5_000_000) }
        release.signal(); try await Task.sleep(nanoseconds: 50_000_000)
        XCTAssertEqual(store.entries.first?.name, "Filtered result")
        XCTAssertEqual(store.query.operation, "terminal")
    }
}
