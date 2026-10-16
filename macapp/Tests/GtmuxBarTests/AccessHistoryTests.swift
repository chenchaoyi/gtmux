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
