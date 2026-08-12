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
