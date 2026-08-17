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

    /// Popover width MUST equal DESIGN §3's size table (420, a companion-app baseline).
    /// Pins the single width token so a drift from the spec fails the build.
    func testPopoverWidthMatchesDesign() {
        XCTAssertEqual(Theme.Size.popoverWidth, 420, "popover width (DESIGN §3, companion-app baseline)")
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
        [{"pane_id":"%22","session":"Team Eval Framework","status":"idle",
          "task":"优化配置加载性能"}]
        """.utf8))[0]
        XCTAssertEqual(withTitle.primary, "优化配置加载性能") // agent session name
        XCTAssertEqual(withTitle.secondary, "Team Eval Framework · %22")    // tmux location

        let noTitle = try JSONDecoder().decode([Agent].self, from: Data("""
        [{"pane_id":"%1","session":"Aurora","status":"idle","task":""}]
        """.utf8))[0]
        XCTAssertEqual(noTitle.primary, "Aurora") // falls back to the tmux session
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

    func testBadgeIdleHasNoCount() {
        // Done (idle) carries NO count — HANDOFF P0.4 (reverts the earlier done-count).
        let s = store([("d", "idle"), ("e", "idle"), ("f", "running")])
        XCTAssertEqual(s.badge, "")
        XCTAssertEqual(s.mostUrgent, .idle)
    }

    func testSectionsOrderAndNonEmpty() {
        let s = store([("d", "idle"), ("a", "waiting"), ("b", "working")])
        let secs = s.sections(query: "")
        XCTAssertEqual(secs.map { $0.status }, [.waiting, .working, .idle])
    }

    // MARK: HQ medallion state resolver (parity with the mobile disc — MOBILE §17)

    private func sup(_ status: String) -> Agent {
        try! JSONDecoder().decode([Agent].self,
            from: Data(#"[{"pane_id":"%h","session":"HQ","status":"\#(status)","role":"supervisor"}]"#.utf8))[0]
    }

    /// The resolver is the SAME six-state priority as the mobile `discState`: HQ's own
    /// call > a worker waiting > a resource bottleneck > working > normal; absent when no
    /// supervisor. A rename of a state or a reordering here silently desyncs the surfaces.
    /// CONFORMANCE with the core (hq-verdict-single-source). The fleet verdict is decided
    /// in Go — `internal/radar.hqVerdict`, pinned by `TestHQVerdict_Priority` — and served
    /// on the digest's supervisor row, which is what the phone renders.
    ///
    /// The menu bar still resolves locally, deliberately: it polls `agents --json` on a
    /// fast tick, the verdict rides the DIGEST, and sampling the machine on every fast
    /// tick is exactly the cost its separate slow resource timer exists to avoid. Its
    /// answer must therefore MATCH the core's, case for case — that is what this test is.
    /// If the Go ordering ever changes, this goes red and the two cannot drift apart in
    /// silence, which is how the phone came to disagree with this surface in the first
    /// place.
    func testHQStateMatchesCoreVerdictOrdering() {
        // Same eight cases as internal/radar/hqverdict_test.go, same order.
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("idle"), waiting: 0, resourceCritical: false), .normal)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("working"), waiting: 0, resourceCritical: false), .working)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("idle"), waiting: 1, resourceCritical: false), .needsYou)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("waiting"), waiting: 0, resourceCritical: false), .hqCall)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("idle"), waiting: 0, resourceCritical: true), .resource)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("waiting"), waiting: 1, resourceCritical: false), .hqCall)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("idle"), waiting: 1, resourceCritical: true), .needsYou)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("working"), waiting: 0, resourceCritical: true), .resource)
    }

    func testHQStateResolverPriority() {
        XCTAssertEqual(AgentStore.hqState(supervisor: nil, waiting: 0, resourceCritical: false), .absent)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("waiting"), waiting: 3, resourceCritical: true), .hqCall)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("idle"), waiting: 2, resourceCritical: true), .needsYou)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("idle"), waiting: 0, resourceCritical: true), .resource)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("working"), waiting: 0, resourceCritical: false), .working)
        XCTAssertEqual(AgentStore.hqState(supervisor: sup("idle"), waiting: 0, resourceCritical: false), .normal)
    }

    /// The HQ headline must AGREE with the medallion color: a red `.resource` tier (idle
    /// HQ, no worker waiting) must NOT read "all normal" — it reddens the row, so the
    /// sentence has to explain WHY (the resource condition). Pins the fix for the reported
    /// "red icon + red text next to 'all normal — nothing needs you'" contradiction.
    func testFleetHeadlineResourceSpeaksInsteadOfAllNormal() {
        let hq = sup("idle")
        XCTAssertEqual(AgentStore.fleetHeadline(state: .resource, agents: [hq]), .resource)
        // A genuinely quiet fleet (no red tier) still reads all-normal.
        XCTAssertEqual(AgentStore.fleetHeadline(state: .normal, agents: [hq]), .allNormal)
        // HQ's own call and a waiting worker still take priority and name the situation.
        XCTAssertEqual(AgentStore.fleetHeadline(state: .hqCall, agents: [hq]), .call)
        let worker = try! JSONDecoder().decode([Agent].self,
            from: Data(#"[{"pane_id":"%9","session":"api","status":"waiting"}]"#.utf8))[0]
        XCTAssertEqual(AgentStore.fleetHeadline(state: .needsYou, agents: [hq, worker]), .worker(name: "api", others: 0))
    }

    /// A soft "amber" resource tier must NOT redden the medallion — only "red" reaches
    /// the resource state (低噪, matching the disc). Also pins the store's tier wiring.
    func testHQStateSoftAmberStaysGreen() {
        let s = AgentStore()
        s.setForTesting([sup("idle")])
        s.setTierForTesting("amber")
        XCTAssertEqual(s.hqState, .normal)
        s.setTierForTesting("red")
        XCTAssertEqual(s.hqState, .resource)
    }

    /// workerWaiting (the `needsYou` driver) counts non-supervisor, non-native waiting
    /// sessions only — a waiting HQ or a waiting native session must not inflate it.
    func testWorkerWaitingExcludesSupervisorAndNative() {
        let s = AgentStore()
        s.setForTesting(try! JSONDecoder().decode([Agent].self, from: Data("""
        [{"pane_id":"%h","status":"waiting","role":"supervisor"},
         {"pane_id":"%w","status":"waiting"},
         {"pane_id":"%n","status":"waiting","source":"native"}]
        """.utf8)))
        XCTAssertEqual(s.workerWaiting, 1)   // only %w
        XCTAssertEqual(s.hqState, .hqCall)   // the waiting supervisor outranks the worker
    }

    // The Shared-input allowlist (Preferences) renders from `shareablePanes`: real
    // tmux panes only (native/hook-less rows can't be typed into), ordered like the
    // radar (state rank → session title), and identified by the SAME `primary`
    // session name shown in the popover — not an indistinguishable "Claude Code · %N".
    func testShareablePanesFilterOrderAndIdentity() throws {
        let json = """
        [{"pane_id":"%45","session":"gtmux","agent":"Claude Code","status":"idle","task":"reap gate"},
         {"pane_id":"%37","session":"gtmux","agent":"Claude Code","status":"waiting","task":"share picker"},
         {"pane_id":"%14","session":"gtmux","agent":"Claude Code","status":"working","task":"digest tables"},
         {"pane_id":"%20","session":"gtmux","agent":"Claude Code","status":"working","task":"apple briefing"},
         {"pane_id":"%9","session":"mob","agent":"Claude Code","status":"working","source":"native","task":"native one"},
         {"session":"nopane","agent":"Claude Code","status":"working","task":"ghost"}]
        """
        let s = AgentStore()
        s.setForTesting(try JSONDecoder().decode([Agent].self, from: Data(json.utf8)))
        let panes = s.shareablePanes
        // native (%9) and the no-pane row are excluded.
        XCTAssertEqual(panes.count, 4)
        XCTAssertFalse(panes.contains { $0.isNative || $0.paneID.isEmpty })
        // ordered by state rank (waiting → working → idle), ties by session title.
        XCTAssertEqual(panes.map { $0.paneID }, ["%37", "%20", "%14", "%45"])
        // each row carries the agent's OWN session name, not the generic agent label.
        XCTAssertEqual(panes.first?.primary, "share picker")
        XCTAssertEqual(panes.first?.secondary, "gtmux · %37")
        XCTAssertNotEqual(panes.first?.primary, "Claude Code")
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
        {"kind":"input","title":"Aurora","subtitle":"Claude Code",
         "body":"Needs your input","pane":"%12","session":"Aurora",
         "icon":"/tmp/icon.png","ts":1700000000}
        """
        let r = try JSONDecoder().decode(NotificationManager.Request.self, from: Data(json.utf8))
        XCTAssertEqual(r.kind, "input")
        XCTAssertEqual(r.title, "Aurora")
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
        [{"pane_id":"%1","session":"Team Eval Framework","status":"working","task":"eval"},
         {"pane_id":"%2","session":"ccy_dev","status":"idle","task":"ccy.dev"},
         {"pane_id":"%3","session":"dev-workspace","status":"idle","task":"workspace"}]
        """
        let s = AgentStore()
        s.setForTesting(try JSONDecoder().decode([Agent].self, from: Data(json.utf8)))
        let hit = s.ordered(query: "dev")
        XCTAssertEqual(hit.map { $0.session }, ["ccy_dev", "dev-workspace"]) // both idle, sorted
        XCTAssertFalse(hit.contains { $0.session == "Team Eval Framework" }) // non-match excluded
    }

    func testPaletteWrapNavigation() {
        let m = PaletteModel(store: store([("a", "waiting"), ("b", "working"), ("c", "idle")]))
        XCTAssertEqual(m.results.count, 3)
        m.move(-1) // up from 0 wraps to last
        XCTAssertEqual(m.selected, 2)
        m.move(1)  // down from last wraps to first
        XCTAssertEqual(m.selected, 0)
    }

    /// A keyboard move requests a scroll-into-view; a direct (hover-style) selection
    /// does NOT — else a stationary cursor over a scrolling list re-selects the row
    /// under it and the scroll yanks back to the cursor (the reported jitter).
    func testPaletteFollowScrollOnlyOnKeyboardMove() {
        let m = PaletteModel(store: store([("a", "idle"), ("b", "idle"), ("c", "idle")]))
        XCTAssertFalse(m.followScroll)          // default: don't fight the mouse
        m.move(1)
        XCTAssertTrue(m.followScroll)           // keyboard nav → scroll the row into view
        m.followScroll = false                  // (the view's onChange clears it after scrolling)
        m.selected = 2                          // the hover path sets selected directly…
        XCTAssertFalse(m.followScroll)          // …and never asks for a scroll
    }

    // MARK: self-update — exit:0 self-heal (no stuck "Updating…" spinner)

    /// Within the grace window, keep waiting to be killed regardless of versions —
    /// a real relaunch kills us in ~1–2s, so we mustn't jump the gun.
    func testPostExitZeroWaitsWithinGrace() {
        XCTAssertEqual(
            postExitZeroAction(secondsSinceExitZero: 5, grace: 12,
                               runningVersion: "0.18.1", installedVersion: "0.18.2"), .wait)
        // Even if on-disk already looks equal, still wait — the kill may be in flight.
        XCTAssertEqual(
            postExitZeroAction(secondsSinceExitZero: 11.9, grace: 12,
                               runningVersion: "0.18.1", installedVersion: "0.18.1"), .wait)
    }

    /// Grace elapsed + on-disk bundle differs (⇒ newer) → the swap landed but the
    /// relaunch was missed: relaunch the installed bundle ourselves.
    func testPostExitZeroRelaunchesWhenNewerOnDisk() {
        XCTAssertEqual(
            postExitZeroAction(secondsSinceExitZero: 13, grace: 12,
                               runningVersion: "0.18.1", installedVersion: "0.18.2"), .relaunch)
    }

    /// Grace elapsed + on-disk equals running → the app step was skipped (defect 2):
    /// offer a retry rather than "relaunch" the same version or spin forever.
    func testPostExitZeroFailsWhenSameVersion() {
        XCTAssertEqual(
            postExitZeroAction(secondsSinceExitZero: 20, grace: 12,
                               runningVersion: "0.18.1", installedVersion: "0.18.1"), .fail)
    }

    /// Grace elapsed + on-disk version unreadable/absent → treat as no swap: retry.
    func testPostExitZeroFailsWhenInstalledUnreadable() {
        XCTAssertEqual(
            postExitZeroAction(secondsSinceExitZero: 20, grace: 12,
                               runningVersion: "0.18.1", installedVersion: nil), .fail)
        XCTAssertEqual(
            postExitZeroAction(secondsSinceExitZero: 20, grace: 12,
                               runningVersion: "0.18.1", installedVersion: ""), .fail)
    }

    // MARK: self-update — never forward a leaked GTMUX_VERSION pin

    func testUpdateCommandStripsGtmuxVersion() {
        // install.sh's `open -n` leaks GTMUX_VERSION into the app's environment; if the
        // menu-bar self-update forwarded it, Go would reinstall the CURRENT version
        // forever and the "new version" banner would never clear. The command must
        // strip it so an update always resolves the latest release.
        let cmd = Updater.updateCommand(cli: "/Users/x/.local/bin/gtmux", binDir: nil)
        XCTAssertTrue(cmd.contains("env -u GTMUX_VERSION "), "must strip GTMUX_VERSION: \(cmd)")
        XCTAssertTrue(cmd.hasSuffix("'/Users/x/.local/bin/gtmux' update"), "runs the resolved CLI: \(cmd)")
    }

    func testUpdateCommandPinsBinDirWhenGiven() {
        let cmd = Updater.updateCommand(cli: "/App/Gtmux.app/Contents/MacOS/gtmux", binDir: "/Users/x/.local/bin")
        XCTAssertTrue(cmd.contains("env -u GTMUX_VERSION "))
        XCTAssertTrue(cmd.contains("GTMUX_BIN_DIR='/Users/x/.local/bin'"), "pins BIN_DIR: \(cmd)")
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

/// An idle session whose turn ended on a FAILURE gets its own section — it is not
/// "finished". The commander found one sitting under a green ✓ among the completed ones:
/// 这种情况挺严重的，也需要用户及时介入。
///
/// It is NOT folded into "needs you": that section means an agent is asking something you
/// can answer, and an error is not a question. Amber, between needs-you and working.
final class ErroredSectionTests: XCTestCase {
    private func agent(_ id: String, _ status: String, errored: Bool = false) -> Agent {
        let json = #"{"pane_id":"\#(id)","session":"s","status":"\#(status)","agent":"Claude Code","source":"tmux","error":\#(errored)}"#
        return try! JSONDecoder().decode(Agent.self, from: Data(json.utf8))
    }

    func testErroredIdleGetsItsOwnSectionAfterNeedsYou() {
        let store = AgentStore()
        store.setAgentsForTesting([
            agent("%1", "waiting"), agent("%2", "idle", errored: true),
            agent("%3", "working"), agent("%4", "idle"),
        ])
        let secs = store.sections(query: "")
        XCTAssertEqual(secs.map { $0.errored }, [false, true, false, false])
        XCTAssertEqual(secs[1].agents.map { $0.paneID }, ["%2"], "the errored row is its own group")
        XCTAssertEqual(secs[3].agents.map { $0.paneID }, ["%4"], "and is gone from idle")
    }

    func testNoSectionOnAHealthyFleet() {
        let store = AgentStore()
        store.setAgentsForTesting([agent("%1", "idle"), agent("%2", "working")])
        XCTAssertFalse(store.sections(query: "").contains { $0.errored })
    }
}
