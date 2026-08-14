import AppKit
import XCTest
@testable import GtmuxBar

/// Tests for the all-panes browser's parity with the phone's version of the same screen.
///
/// The labelling rules are the load-bearing part: each one is a defect the phone already
/// hit, and a Mac browser that labels rows its own way is exactly the drift that let the
/// two surfaces disagree about the same fleet.
final class PaneBrowserTests: XCTestCase {

    private func row(_ mutate: (inout PaneRow) -> Void) -> PaneRow {
        var r = try! JSONDecoder().decode(PaneRow.self, from: Data("{}".utf8))
        mutate(&r)
        return r
    }

    // MARK: labels — same rules as mobileapp/src/screens/PaneBrowserScreen.tsx

    /// A Claude 2.x pane's `pane_current_command` is its VERSION ("2.1.220", the #659
    /// fact). An agent row whose `agent` field arrives empty must not become a bare
    /// version number — the live radar join names it.
    func testAgentLabelPrefersTheJoinOverAVersionString() {
        let r = row { $0.tier = "agent"; $0.agent = ""; $0.command = "2.1.220"; $0.paneID = "%9" }
        var joined = Agent(); joined.agent = "Claude Code"
        XCTAssertEqual(PaneLabels.agent(row: r, joined: joined), "Claude Code")
        // With no join at all the command is the last resort, not the first choice.
        XCTAssertEqual(PaneLabels.agent(row: r, joined: nil), "2.1.220")
    }

    func testAgentLabelUsesItsOwnNameWhenItHasOne() {
        let r = row { $0.tier = "agent"; $0.agent = "Codex"; $0.command = "node" }
        var joined = Agent(); joined.agent = "Claude Code"
        XCTAssertEqual(PaneLabels.agent(row: r, joined: joined), "Codex",
                       "the row's own identity wins over the join")
    }

    /// Many shells set the pane title to the cwd, sometimes colon-prefixed. That is both
    /// ugly and redundant with the directory shown beside it, so a path falls back to the
    /// command.
    func testPlainLabelFallsBackWhenTheTitleIsJustAPath() {
        XCTAssertEqual(PaneLabels.plain(title: ":/Users/x/src", command: "bash"), "bash")
        XCTAssertEqual(PaneLabels.plain(title: "/Users/x/src", command: "bash"), "bash")
        XCTAssertEqual(PaneLabels.plain(title: "~/src", command: "zsh"), "zsh")
        XCTAssertEqual(PaneLabels.plain(title: "", command: "vim"), "vim")
        XCTAssertEqual(PaneLabels.plain(title: "   ", command: "vim"), "vim")
    }

    func testPlainLabelKeepsARealTitle() {
        XCTAssertEqual(PaneLabels.plain(title: "make check", command: "bash"), "make check")
        XCTAssertEqual(PaneLabels.plain(title: ": logs", command: "tail"), "logs",
                       "a colon prefix is stripped, the title still stands")
    }

    // MARK: fold state

    /// Folding is per SESSION and survives closing the window, so the browser opens the
    /// way the user left it.
    func testCollapseTogglesAndPersists() {
        let d = UserDefaults.standard
        let key = "panes.collapsedSessions"
        let saved = d.string(forKey: key)
        defer { if let s = saved { d.set(s, forKey: key) } else { d.removeObject(forKey: key) } }

        d.removeObject(forKey: key)
        let c = PaneCollapse.shared
        c.setAll([], collapsed: false) // known-empty starting point
        XCTAssertFalse(c.isCollapsed("HQ"))
        c.toggle("HQ")
        XCTAssertTrue(c.isCollapsed("HQ"))
        XCTAssertEqual(d.string(forKey: key), "HQ", "the choice is written, not just held")
        c.toggle("HQ")
        XCTAssertFalse(c.isCollapsed("HQ"))
    }

    /// A session name may legally contain a comma (`tmux new -s a,b`), so the stored form
    /// must not be comma-separated — it would read back as two sessions and fold one
    /// nobody asked to fold. This goes through the STORED form on purpose: an in-memory
    /// set never exercises the separator, so a test that only toggles passes either way.
    func testASessionNameWithACommaSurvivesTheStoredForm() {
        let round = PaneCollapse.decode(PaneCollapse.encode(["a,b", "HQ"]))
        XCTAssertEqual(round, ["a,b", "HQ"])
        XCTAssertFalse(round.contains("a"), "the name must not split into two")
        XCTAssertFalse(round.contains("b"))
    }

    func testAnEmptyStoreDecodesToNothingFolded() {
        XCTAssertTrue(PaneCollapse.decode("").isEmpty, "a fresh install starts expanded")
    }

    /// Expanding everything clears the whole set rather than removing only what is on
    /// screen now: a session that has gone away should not come back folded.
    func testExpandAllClearsEverything() {
        let d = UserDefaults.standard
        let key = "panes.collapsedSessions"
        let saved = d.string(forKey: key)
        defer { if let s = saved { d.set(s, forKey: key) } else { d.removeObject(forKey: key) } }

        let c = PaneCollapse.shared
        c.setAll(["gone", "here"], collapsed: true)
        c.setAll(["here"], collapsed: false)
        XCTAssertFalse(c.isCollapsed("gone"))
        XCTAssertFalse(c.isCollapsed("here"))
    }
}

// MARK: window level (tmux-id-surface phase 1)

private func row(_ id: String, _ session: String, _ win: String, _ winID: String, _ winName: String) -> PaneRow {
    let json = """
    {"pane_id":"\(id)","session":"\(session)","window":"\(win)","pane":"0",
     "loc":"\(session):\(win).0","command":"bash","tier":"plain",
     "win_id":"\(winID)","win_name":"\(winName)"}
    """
    return try! JSONDecoder().decode(PaneRow.self, from: Data(json.utf8))
}

final class PaneWindowTests: XCTestCase {

    /// The window INDEX cannot group windows. Measured on the fleet this was built for:
    /// every session's windows sat at index 0..n but MOST sessions had all their windows at
    /// 0, and two different windows sharing an index would merge into one group. `@id` is
    /// the anchor; grouping keys on it.
    func testTwoWindowsAtTheSameIndexDoNotMerge() {
        let a = PaneWindow(winID: "@4", winName: "cc dev", rows: [row("%9", "HSS", "0", "@4", "cc dev")])
        let b = PaneWindow(winID: "@5", winName: "cc dev", rows: [row("%31", "HSS", "0", "@5", "cc dev")])
        XCTAssertNotEqual(a.id, b.id, "same index AND same name — only the @id separates them")
    }

    /// The label leads with the id because the id is the anchor: a name drifts with
    /// `automatic-rename` and two windows may share one.
    func testWindowLabelLeadsWithTheId() {
        XCTAssertEqual(PaneWindow(winID: "@7", winName: "multipilot", rows: []).label, "@7 multipilot")
        // No name yet — the id alone still identifies it.
        XCTAssertEqual(PaneWindow(winID: "@7", winName: "", rows: []).label, "@7")
        // An older core sends no id: fall back to the name rather than showing nothing.
        XCTAssertEqual(PaneWindow(winID: "", winName: "multipilot", rows: []).label, "multipilot")
    }

    /// A window line is only worth a row when there is more than one window. With one, the
    /// levels coincide and its id rides on the session header — measured motivation: 6 of
    /// 11 sessions on the real fleet have exactly one window, so always drawing it would
    /// add a line saying nothing to more than half of them.
    func testSingleWindowShowsNoExtraRowButKeepsItsId() {
        let one = PaneGroup(session: "HQ",
                            windows: [PaneWindow(winID: "@3", winName: "hq", rows: [row("%4", "HQ", "0", "@3", "hq")])],
                            agentCount: 1, roll: [:])
        XCTAssertFalse(one.showsWindowRows)
        XCTAssertEqual(one.windowIDs, "@3", "the id must not disappear just because the row does")

        let many = PaneGroup(session: "MP",
                             windows: [PaneWindow(winID: "@7", winName: "a", rows: [row("%12", "MP", "0", "@7", "a")]),
                                       PaneWindow(winID: "@8", winName: "b", rows: [row("%10", "MP", "1", "@8", "b")])],
                             agentCount: 2, roll: [:])
        XCTAssertTrue(many.showsWindowRows)
        // A multi-window session must name ALL of them: collapsed, this line is the only
        // thing saying which windows are in here, and it used to say nothing at all.
        XCTAssertEqual(many.windowIDs, "@7 @8")
    }

    /// The rollup counts every pane in the session, across its windows — folding a session
    /// must still report what is inside all of it.
    func testGroupRowsSpanAllWindows() {
        let g = PaneGroup(session: "S",
                          windows: [PaneWindow(winID: "@1", winName: "a", rows: [row("%1", "S", "0", "@1", "a"), row("%2", "S", "0", "@1", "a")]),
                                    PaneWindow(winID: "@2", winName: "b", rows: [row("%3", "S", "1", "@2", "b")])],
                          agentCount: 0, roll: [:])
        XCTAssertEqual(g.rows.count, 3)
        XCTAssertEqual(g.rows.map(\.paneID), ["%1", "%2", "%3"], "window order preserved")
    }

    /// The decoder must tolerate a core that does not send the fields at all.
    func testRowDecodesWithoutWindowFields() {
        let json = #"{"pane_id":"%1","session":"S","window":"0","pane":"0","loc":"S:0.0","command":"bash","tier":"plain"}"#
        let r = try! JSONDecoder().decode(PaneRow.self, from: Data(json.utf8))
        XCTAssertEqual(r.winID, "")
        XCTAssertEqual(r.winName, "")
    }
}


// A session with many windows must not push its own name off the row.
final class PaneWindowIDsTests: XCTestCase {
    private func win(_ id: String) -> PaneWindow { PaneWindow(winID: id, winName: "w", rows: []) }

    func testWindowIDsAreCappedNotEndless() {
        let g = PaneGroup(session: "S", windows: (1...9).map { win("@\($0)") }, agentCount: 0, roll: [:])
        XCTAssertEqual(g.windowIDs, "@1 @2 @3 @4 @5 +4")
    }

    func testSixFitWithoutACounter() {
        let g = PaneGroup(session: "S", windows: (1...6).map { win("@\($0)") }, agentCount: 0, roll: [:])
        XCTAssertEqual(g.windowIDs, "@1 @2 @3 @4 @5 @6")
    }

    /// An older core sends no ids — the header then says nothing rather than a row of blanks.
    func testNoIdsMeansNoLine() {
        let g = PaneGroup(session: "S", windows: [win(""), win("")], agentCount: 0, roll: [:])
        XCTAssertEqual(g.windowIDs, "")
    }
}

// The search box must find a pane by the token everyone actually holds: its `%N`. That is
// what a tab title shows, what `gtmux focus %N` takes, and what HQ says — and it was the
// one thing the haystack did not contain, so typing a row's own leading label found
// nothing. Same for a window's `@N`.
final class PaneSearchTests: XCTestCase {
    /// Mirrors the filter's haystack. Kept beside the tests so a field dropped from the
    /// real one shows up here as a failure rather than as silence.
    private func haystack(_ r: PaneRow) -> [String] {
        [r.session, r.window, r.command, r.title, r.cwd, r.agent, r.loc, r.paneID, r.winID, r.winName]
    }
    private func matches(_ r: PaneRow, _ q: String) -> Bool {
        haystack(r).contains { $0.lowercased().contains(q.lowercased()) }
    }
    private func sample() -> PaneRow {
        let json = #"{"pane_id":"%23","session":"gtmux dev","window":"0","pane":"1","loc":"gtmux dev:0.1","command":"bash","tier":"plain","win_id":"@17","win_name":"gtmux"}"#
        return try! JSONDecoder().decode(PaneRow.self, from: Data(json.utf8))
    }

    func testFindsByPaneId() {
        XCTAssertTrue(matches(sample(), "%23"))
        XCTAssertTrue(matches(sample(), "23"), "typing the digits without the sigil still finds it")
    }

    func testFindsByWindowIdAndName() {
        XCTAssertTrue(matches(sample(), "@17"))
        XCTAssertTrue(matches(sample(), "gtmux"))
    }

    func testStillFindsByTheOldFields() {
        XCTAssertTrue(matches(sample(), "bash"))
        XCTAssertTrue(matches(sample(), "gtmux dev"))
    }

    func testDoesNotMatchEverything() {
        XCTAssertFalse(matches(sample(), "%99"))
        XCTAssertFalse(matches(sample(), "zsh"))
    }
}
