import AppKit
import SwiftUI
import XCTest

@testable import GtmuxBar

func choiceDecode<T: Decodable>(_ text: String) -> T {
    let decoder = JSONDecoder()
    decoder.keyDecodingStrategy = .convertFromSnakeCase
    return try! decoder.decode(T.self, from: Data(text.utf8))
}
final class ChoiceFixture: HQWorkspaceServing, @unchecked Sendable {
    private let lock = NSLock()
    private var value: HQWorkspaceSnapshot = choiceDecode(
        #"{"schema":1,"revision":1,"matters":[{"id":"choice-matter","title":"Synthetic manual request","state":"needs-input","created_at":100,"updated_at":100,"revision":1,"participants":[],"user_input":{"id":"choice-q","kind":"manual","prompt":"Confirm the synthetic sample","why":"Only a fixture is affected.","impact":"No file or session is changed.","choices":["Done","Keep"],"at":100}}],"conversations":[]}"#
    )
    var detailGate: DispatchSemaphore?
    var gate: DispatchSemaphore?
    var failSources = false
    var storedOnly = false
    private(set) var actions: [HQWorkspaceAction] = []
    func replace(_ transform: (inout HQWorkspaceSnapshot) -> Void) {
        lock.lock()
        defer { lock.unlock() }
        transform(&value)
    }
    func summary() throws -> HQWorkspaceSnapshot {
        lock.lock()
        defer { lock.unlock() }
        var result = value
        if storedOnly { result.sourcesIncluded = false }
        return result
    }
    func snapshot() throws -> HQWorkspaceSnapshot {
        var old = try summary()
        old.sourcesIncluded = true
        gate?.wait()
        if failSources { throw HQWorkspaceError.timeout }
        return old
    }
    func detail(_ scope: String, before: UInt64?, earlier: Int) throws -> HQWorkspaceDetail {
        detailGate?.wait()
        var result: HQWorkspaceDetail = choiceDecode(
            #"{"schema":1,"messages":[],"turns":[],"earlier_available":false,"changes":[],"events":[],"warnings":[]}"#
        )
        result.matter = try summary().matters.first { $0.id == scope }
        return result
    }
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail {
        try detail(matter, before: nil, earlier: 0)
    }
    func act(_ action: HQWorkspaceAction) throws {
        lock.lock()
        defer { lock.unlock() }
        actions.append(action)
    }
    func drain() throws {}
}

@MainActor enum HQChoiceTestSupport {
    static func waitUntil(_ condition: @escaping () -> Bool) async throws {
        for _ in 0..<400 {
            if condition() { return }
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTFail("synthetic HQ condition did not settle")
    }
    static func load(_ fake: ChoiceFixture) async throws -> HQWorkspaceStore {
        let store = HQWorkspaceStore(client: fake, resources: HQResourceStore(read: { _ in nil }))
        store.refresh()
        try await waitUntil { !store.loading }
        store.select(.matter("choice-matter"))
        try await waitUntil { !store.loading }
        return store
    }
}
