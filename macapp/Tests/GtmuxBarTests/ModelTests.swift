import AppKit
import XCTest
@testable import GtmuxBar

final class ModelTests: XCTestCase {

    // MARK: design conformance (the "design-follow" automated check)

    /// Status colors MUST equal DESIGN.md §9's authoritative hex. This is the
    /// guardrail that the implementation didn't drift from the spec palette.
    func testStatusColorsMatchDesignHex() {
        XCTAssertEqual(hex(Theme.Status.waitingNS), "EF4444", "waiting")
        XCTAssertEqual(hex(Theme.Status.workingNS), "06B6D4", "working")
        XCTAssertEqual(hex(Theme.Status.idleNS), "22C55E", "idle")
        XCTAssertEqual(hex(Theme.Status.noneNS), "8E8E93", "none/running")
    }

    /// Every status maps to a color (color is the status-only channel, DESIGN §1).
    func testEveryStatusHasColor() {
        for s in Status.allCases { XCTAssertNotNil(s.nsColor) }
    }

    /// Section ordering is fixed: waiting → working → idle → running (DESIGN §3).
    func testStatusRankOrder() {
        XCTAssertLessThan(Status.waiting.rank, Status.working.rank)
        XCTAssertLessThan(Status.working.rank, Status.idle.rank)
        XCTAssertLessThan(Status.idle.rank, Status.running.rank)
    }

    // MARK: relative time (DESIGN §3)

    func testRelativeTime() {
        let now = 1_000_000
        XCTAssertEqual(relativeTime(0, now: now), "")
        XCTAssertEqual(relativeTime(now - 10, now: now), "now")
        XCTAssertEqual(relativeTime(now - 120, now: now), "2m")
        XCTAssertEqual(relativeTime(now - 7200, now: now), "2h")
        XCTAssertEqual(relativeTime(now - 172800, now: now), "2d")
    }

    // MARK: agent identity (DESIGN §6) — neutral monogram, no logos

    func testAgentMonogram() {
        XCTAssertEqual(agentMonogram("Claude Code"), "C")
        XCTAssertEqual(agentMonogram("Codex"), "Cx")
        XCTAssertEqual(agentMonogram("Gemini"), "G")
        XCTAssertEqual(agentMonogram("Something Else"), "S")
        XCTAssertEqual(agentMonogram(""), "·")
    }

    // MARK: model decode (DESIGN §14) — tolerant of older JSON

    func testDecodeTmuxAgent() throws {
        let json = """
        [{"pane_id":"%5","session":"work","window":"1","loc":"work:1.0",
          "agent":"Claude Code","status":"waiting","task":"do it","latest":true,
          "activity_at":1700000000}]
        """
        let agents = try JSONDecoder().decode([Agent].self, from: Data(json.utf8))
        let a = agents[0]
        XCTAssertEqual(a.paneID, "%5")
        XCTAssertEqual(a.state, .waiting)
        XCTAssertEqual(a.source, "tmux") // default when absent
        XCTAssertFalse(a.isNative)
        XCTAssertEqual(a.primary, "work")
        XCTAssertEqual(a.jumpArgs(), ["focus", "%5"])
    }

    func testDecodeNativeAgent() throws {
        let json = """
        [{"source":"native","project":"diting","terminal":"Ghostty","tab":"diting — zsh",
          "agent":"Gemini","status":"idle","task":""}]
        """
        let agents = try JSONDecoder().decode([Agent].self, from: Data(json.utf8))
        let a = agents[0]
        XCTAssertTrue(a.isNative)
        XCTAssertEqual(a.primary, "diting")      // project, not session (DESIGN §7)
        XCTAssertEqual(a.secondary, "Ghostty")   // terminal
        XCTAssertEqual(a.jumpArgs(), ["focus", "--terminal", "Ghostty", "--tab", "diting — zsh"])
    }

    // MARK: store — counts, badge, grouping, filter, search

    private func store(_ statuses: [(String, String)]) -> AgentStore {
        // statuses: (session, status)
        let arr = statuses.map { #"{"pane_id":"%\#($0.0)","session":"\#($0.0)","status":"\#($0.1)"}"# }
        let json = "[" + arr.joined(separator: ",") + "]"
        let s = AgentStore()
        s.setForTesting(try! JSONDecoder().decode([Agent].self, from: Data(json.utf8)))
        return s
    }

    func testCountsAndBadge() {
        let s = store([("a", "waiting"), ("b", "working"), ("c", "working"), ("d", "idle"), ("e", "running")])
        XCTAssertEqual(s.total, 5)
        XCTAssertEqual(s.waiting, 1)
        XCTAssertEqual(s.working, 2)
        XCTAssertEqual(s.idleCount, 2) // idle + running
        XCTAssertEqual(s.badge, "1")   // waiting wins
        XCTAssertEqual(s.mostUrgent, .waiting)
    }

    func testBadgeFallsBackToWorking() {
        let s = store([("b", "working"), ("d", "idle")])
        XCTAssertEqual(s.badge, "1")
        XCTAssertEqual(s.mostUrgent, .working)
    }

    func testSectionsOrderAndNonEmpty() {
        let s = store([("d", "idle"), ("a", "waiting"), ("b", "working")])
        let secs = s.sections(waitingOnly: false, query: "")
        XCTAssertEqual(secs.map { $0.status }, [.waiting, .working, .idle])
    }

    func testWaitingOnlyFilter() {
        let s = store([("a", "waiting"), ("b", "working"), ("c", "idle")])
        let secs = s.sections(waitingOnly: true, query: "")
        XCTAssertEqual(secs.count, 1)
        XCTAssertEqual(secs[0].status, .waiting)
    }

    func testFuzzySearch() {
        XCTAssertTrue(AgentStore.fuzzy("pca", in: "pica"))
        XCTAssertTrue(AgentStore.fuzzy("auth", in: "refactor auth"))
        XCTAssertFalse(AgentStore.fuzzy("zzz", in: "pica"))
    }

    // MARK: command palette (DESIGN §4 B)

    func testPaletteWrapNavigation() {
        let m = PaletteModel(store: store([("a", "waiting"), ("b", "working"), ("c", "idle")]))
        XCTAssertEqual(m.results.count, 3)
        m.move(-1) // up from 0 wraps to last
        XCTAssertEqual(m.selected, 2)
        m.move(1)  // down from last wraps to first
        XCTAssertEqual(m.selected, 0)
    }

    // MARK: helpers

    private func hex(_ c: NSColor) -> String {
        let s = c.usingColorSpace(.sRGB) ?? c
        return String(format: "%02X%02X%02X",
                      Int((s.redComponent * 255).rounded()),
                      Int((s.greenComponent * 255).rounded()),
                      Int((s.blueComponent * 255).rounded()))
    }
}
