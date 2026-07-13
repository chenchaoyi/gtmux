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

    /// Popover width MUST equal DESIGN §3's size table (420, calibrated to MPBar).
    /// Pins the single width token so a drift from the spec fails the build.
    func testPopoverWidthMatchesDesign() {
        XCTAssertEqual(Theme.Size.popoverWidth, 420, "popover width (DESIGN §3, MPBar baseline)")
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
        XCTAssertEqual(relativeTime(now - 7, now: now), "7s")   // seconds granularity
        XCTAssertEqual(relativeTime(now - 120, now: now), "2m")
        XCTAssertEqual(relativeTime(now - 7200, now: now), "2h")
        XCTAssertEqual(relativeTime(now - 172800, now: now), "2d")
    }

    /// The duration anchors to `since` (state start) when present, else activity.
    func testDurationAnchorsToSince() throws {
        let a = try JSONDecoder().decode([Agent].self, from: Data("""
        [{"pane_id":"%1","agent":"Claude Code","status":"working",
          "activity_at":1700000000,"since":1700000300}]
        """.utf8))[0]
        XCTAssertEqual(a.since, 1_700_000_300)
        XCTAssertEqual(relativeTime(a.since, now: 1_700_000_360), "1m") // 60s since state start
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
        XCTAssertEqual(a.primary, "do it")        // the agent's own session name (its title)
        XCTAssertEqual(a.secondary, "work · %5")  // dim location: tmux session · pane
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

    /// Row identity: the agent's own session name (its pane title) leads; the tmux
    /// session is the dim location. When the agent set no title, fall back to the
    /// tmux session so the row is never blank.
    func testAgentSessionNameLeadsRow() throws {
        let withTitle = try JSONDecoder().decode([Agent].self, from: Data("""
        [{"pane_id":"%22","session":"HSS Eval Framework","status":"idle",
          "task":"评估智能化能力评测框架的整体方案"}]
        """.utf8))[0]
        XCTAssertEqual(withTitle.primary, "评估智能化能力评测框架的整体方案") // agent session name
        XCTAssertEqual(withTitle.secondary, "HSS Eval Framework · %22")     // tmux location

        let noTitle = try JSONDecoder().decode([Agent].self, from: Data("""
        [{"pane_id":"%1","session":"Diting","status":"idle","task":""}]
        """.utf8))[0]
        XCTAssertEqual(noTitle.primary, "Diting") // falls back to the tmux session
    }

    // MARK: agent identity icon (DESIGN §6)

    func testDecodeIconField() throws {
        let a = try JSONDecoder().decode([Agent].self, from: Data("""
        [{"pane_id":"%1","agent":"Claude Code","status":"idle","icon":"/Applications/Claude.app"}]
        """.utf8))[0]
        XCTAssertEqual(a.icon, "/Applications/Claude.app")
    }

    /// No icon hint + no installed app + no drop-in file → nil, so the avatar
    /// falls back to the neutral monogram.
    func testAgentIconsNilWhenUnavailable() throws {
        let a = try JSONDecoder().decode([Agent].self, from: Data("""
        [{"pane_id":"%9","agent":"ZzzNoSuchAgent","status":"idle","icon":""}]
        """.utf8))[0]
        XCTAssertNil(AgentIcons.image(for: a))
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
        let secs = s.sections(query: "")
        XCTAssertEqual(secs.map { $0.status }, [.waiting, .working, .idle])
    }

    func testSupervisorExcludedFromSections() throws {
        let json = #"[{"pane_id":"%1","session":"a","status":"working"},"#
            + #"{"pane_id":"%9","session":"HQ","status":"working","role":"supervisor"}]"#
        let s = AgentStore()
        s.setForTesting(try JSONDecoder().decode([Agent].self, from: Data(json.utf8)))
        // The supervisor renders as the HQ card, never inside the sections.
        let rows = s.sections(query: "").flatMap { $0.agents }
        XCTAssertFalse(rows.contains { $0.isSupervisor })
        XCTAssertEqual(rows.count, 1)
        XCTAssertEqual(s.supervisor?.paneID, "%9")
    }

    func testFuzzySearch() {
        XCTAssertTrue(AgentStore.fuzzy("pca", in: "pica"))
        XCTAssertTrue(AgentStore.fuzzy("auth", in: "refactor auth"))
        XCTAssertFalse(AgentStore.fuzzy("zzz", in: "pica"))
    }

    // MARK: notification queue — decode the hook's request (contract with internal/notify)

    func testNotifyRequestDecode() throws {
        let json = """
        {"kind":"input","title":"Diting","subtitle":"Claude Code",
         "body":"Needs your input","pane":"%12","session":"Diting",
         "icon":"/tmp/icon.png","ts":1700000000}
        """
        let r = try JSONDecoder().decode(NotificationManager.Request.self, from: Data(json.utf8))
        XCTAssertEqual(r.kind, "input")
        XCTAssertEqual(r.title, "Diting")
        XCTAssertEqual(r.pane, "%12")
        XCTAssertEqual(r.icon, "/tmp/icon.png")
        XCTAssertEqual(r.ts, 1_700_000_000)
    }

    /// Tolerates older/sparse JSON: missing fields fall back, kind defaults to "done".
    func testNotifyRequestDecodeTolerant() throws {
        let r = try JSONDecoder().decode(
            NotificationManager.Request.self, from: Data(#"{"title":"X","body":"done"}"#.utf8))
        XCTAssertEqual(r.kind, "done")   // default
        XCTAssertEqual(r.title, "X")
        XCTAssertEqual(r.pane, "")       // missing → empty
        XCTAssertEqual(r.ts, 0)
    }

    // MARK: command palette (DESIGN §4 B)

    /// Search must find IDLE (done-this-turn) agents and exclude non-matches —
    /// regression for the palette showing a stale, non-matching row at a reused
    /// index instead of the real matches.
    func testSearchFindsIdleAgents() throws {
        let json = """
        [{"pane_id":"%1","session":"HSS Eval Framework","status":"working","task":"eval"},
         {"pane_id":"%2","session":"ccy_dev","status":"idle","task":"ccy.dev"},
         {"pane_id":"%3","session":"ccy-workspace","status":"idle","task":"workspace"}]
        """
        let s = AgentStore()
        s.setForTesting(try JSONDecoder().decode([Agent].self, from: Data(json.utf8)))
        let hit = s.ordered(query: "ccy")
        XCTAssertEqual(hit.map { $0.session }, ["ccy_dev", "ccy-workspace"]) // both idle, sorted
        XCTAssertFalse(hit.contains { $0.session == "HSS Eval Framework" })  // non-match excluded
    }

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
