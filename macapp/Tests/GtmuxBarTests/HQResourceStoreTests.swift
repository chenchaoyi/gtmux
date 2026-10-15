import XCTest

@testable import GtmuxBar

/// All reads are synthetic; no CLI, HQ home, keychain or agent client is touched.
final class ResourceReadFake: @unchecked Sendable {
    private let lock = NSLock()
    private var commands: [[String]] = []
    private var _fail = false
    var delay: TimeInterval = 0
    var fail: Bool {
        get {
            lock.lock()
            defer { lock.unlock() }
            return _fail
        }
        set {
            lock.lock()
            defer { lock.unlock() }
            _fail = newValue
        }
    }
    var calls: [[String]] {
        lock.lock()
        defer { lock.unlock() }
        return commands
    }
    func read(_ args: [String]) -> Data? {
        lock.lock()
        commands.append(args)
        let fail = _fail
        lock.unlock()
        if delay > 0 { Thread.sleep(forTimeInterval: delay) }
        if fail { return nil }
        let json: String
        switch args {
        case ["hq", "--board", "--json"]:
            json = ###"{"exists":true,"text":"## Current work\nFixture board","updated_at":100}"###
        case ["knowledge", "list", "--json"]:
            json =
                ###"[{"id":"fixture/lesson","topic":"fixture","title":"Fixture lesson","body":"Knowledge detail","at":100}]"###
        case ["capture", "--list", "--json"], ["agents", "--json"]: json = "[]"
        case ["hq", "--records", "--json"]:
            json =
                ###"{"exists":true,"bytes":1200,"snapshots":2,"off_machine":"Fixture archive status"}"###
        case ["resource", "--json"]:
            json =
                ###"{"machine":{"tier":"green","mem_free_pct":60,"disk_free_gb":30,"disk_use_pct":80,"ncpu":8}}"###
        case ["usage", "--json"]:
            json =
                ###"{"sessions":[],"limits":{"windows":[{"label":"codex week","pct_used":35,"agent":"codex","kind":"week"}]}}"###
        default: return nil
        }
        return Data(json.utf8)
    }
}

@MainActor final class HQResourceStoreTests: XCTestCase {
    func settled(_ store: HQResourceStore) async throws {
        for _ in 0..<400 {
            if store.refreshingPages.isEmpty { return }
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTFail("resource read did not settle")
    }

    func testOnlySelectedDestinationReadsAndStartIsIdempotent() async throws {
        let fake = ResourceReadFake()
        let store = HQResourceStore(read: fake.read)
        for page in HQResourcePage.allCases {
            let before = fake.calls.count
            store.start(page: page)
            try await settled(store)
            let calls = Array(fake.calls.dropFirst(before))
            switch page {
            case .board: XCTAssertEqual(calls, [["hq", "--board", "--json"]])
            case .knowledge:
                XCTAssertEqual(
                    calls,
                    [
                        ["knowledge", "list", "--json"], ["capture", "--list", "--json"],
                        ["hq", "--records", "--json"],
                    ])
            case .machine: XCTAssertEqual(calls, [["resource", "--json"], ["agents", "--json"]])
            case .usage: XCTAssertEqual(calls, [["usage", "--json"]])
            }
            store.start(page: page)
            XCTAssertEqual(fake.calls.count, before + calls.count, "reopening must not duplicate polling")
            XCTAssertTrue(store.isPolling)
        }
        store.stop()
        let before = fake.calls.count
        store.refresh()
        XCTAssertEqual(fake.calls.count, before)
        XCTAssertNil(store.activePage)
        XCTAssertFalse(store.isPolling)
    }

    func testFailedReadRetainsSnapshotAndRetryClearsError() async throws {
        let fake = ResourceReadFake()
        let store = HQResourceStore(read: fake.read)
        store.start(page: .knowledge)
        try await settled(store)
        XCTAssertEqual(store.entries.map(\.id), ["fixture/lesson"])
        XCTAssertEqual(store.memory.snapshots, 2)
        fake.fail = true
        store.refresh()
        try await settled(store)
        XCTAssertEqual(store.entries.map(\.id), ["fixture/lesson"])
        XCTAssertEqual(store.memory.snapshots, 2)
        XCTAssertTrue(store.failedPages.contains(.knowledge))
        fake.fail = false
        store.refresh()
        try await settled(store)
        XCTAssertFalse(store.failedPages.contains(.knowledge))
        store.stop()
    }

    func testLatePriorReadCannotReplaceSelectedPageAndSamePageIsGated() async throws {
        let fake = ResourceReadFake()
        fake.delay = 0.03
        let store = HQResourceStore(read: fake.read)
        store.start(page: .board)
        store.refresh()
        store.start(page: .usage)
        try await settled(store)
        XCTAssertEqual(store.activePage, .usage)
        XCTAssertEqual(store.usage?.limits?.windows?.first?.pctUsed, 35)
        XCTAssertNotNil(store.board)
        XCTAssertEqual(fake.calls.filter { $0 == ["hq", "--board", "--json"] }.count, 1)
        store.stop()
    }
}
