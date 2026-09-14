import XCTest
@testable import GtmuxBar

/// The chief-of-staff report the HQ card expands into (menubar-hq-report). These pin the
/// judgment — what is on the card, in which order and colour, in both languages — so the
/// menu bar cannot drift from the phone's `hqHeaderModel` in silence.
final class HQCardReportTests: XCTestCase {
    private func machine(tier: String? = nil, mem: Int = 40, memTier: String = "normal",
                         disk: Int = 60, warn: String? = nil) -> ResourceReport.Machine {
        let json = """
        {"tier":\(tier.map { "\"\($0)\"" } ?? "null"),"disk_free_gb":58,"disk_use_pct":\(disk),
         "mem_free_pct":\(mem),"mem_tier":"\(memTier)","load_ratio":0.7,"ncpu":14
         \(warn.map { ",\"warn\":\"\($0)\"" } ?? "")}
        """
        return try! JSONDecoder().decode(ResourceReport.Machine.self, from: Data(json.utf8))
    }

    func testTheResourceReportDecodesTheWholeSnapshot() {
        // The medallion read only `tier`; the card reads the rest, and the Go side marks
        // most of it omitempty, so a sparse report must still decode.
        let full = """
        {"machine":{"disk_free_gb":58,"disk_use_pct":87,"mem_free_pct":38,"mem_tier":"warn",
          "load_ratio":0.72,"ncpu":14,"warn":"memory warn","tier":"amber",
          "battery":{"present":true,"percent":100,"on_ac":true,"state":"charged"}},
         "agents":{"%20":{"rss_mb":453,"cpu":14.8}},
         "orphans":[{"pid":3015,"rss_mb":100,"cpu":0,"comm":"node","kind":"dev-server","hint":"kill it"}]}
        """
        let r = try! JSONDecoder().decode(ResourceReport.self, from: Data(full.utf8))
        XCTAssertEqual(r.machine.tier, "amber")
        XCTAssertEqual(r.machine.memFreePct, 38)
        XCTAssertEqual(r.machine.battery?.onAC, true)
        XCTAssertEqual(r.agents?["%20"]?.rssMB, 453)
        XCTAssertEqual(r.orphans?.first?.hint, "kill it")
        let sparse = try! JSONDecoder().decode(ResourceReport.self, from: Data(#"{"machine":{}}"#.utf8))
        XCTAssertNil(sparse.machine.tier)
        XCTAssertNil(sparse.agents)
    }

    func testARedMachineLeadsAndMayWrap() {
        var i = HQReportInput(machine: machine(tier: "red", mem: 9, memTier: "critical", disk: 92, warn: "memory critical"))
        i.orphans = 4
        i.entries = 478
        i.boardUpdatedAt = Int64(Date().timeIntervalSince1970) - 180
        let rows = hqReportRows(i, zh: true)
        XCTAssertEqual(rows.map { $0.key }, [.machine, .knowledge, .board])
        XCTAssertEqual(rows[0].tone, .red)
        XCTAssertTrue(rows[0].wraps, "the reason the card opened may take two lines")
        XCTAssertEqual(rows[0].value, "内存 9% 空闲（临界） · 磁盘 92% 已用（剩 58 GB） · 负载 0.7×14 · 4 个孤儿进程可回收")
        XCTAssertEqual(rows[0].door, .machine)
        let en = hqReportRows(i, zh: false)
        XCTAssertEqual(en[0].label, "machine")
        XCTAssertEqual(en[0].value, "memory 9% free (critical) · disk 92% used (58 GB left) · load 0.7×14 · 4 orphans reclaimable")
    }

    func testAHealthyMachineSitsAfterTheDoorsInGrey() {
        var i = HQReportInput(machine: machine())
        i.entries = 12
        i.boardUpdatedAt = Int64(Date().timeIntervalSince1970) - 60
        let rows = hqReportRows(i, zh: false)
        XCTAssertEqual(rows.map { $0.key }, [.knowledge, .board, .machine])
        XCTAssertEqual(rows[2].tone, .plain)
        XCTAssertFalse(rows[2].wraps)
        // Amber is amber: a heads-up keeps its ordinary place, only its colour changes.
        i.machine = machine(tier: "amber", memTier: "warn", warn: "memory warn")
        let amber = hqReportRows(i, zh: false)
        XCTAssertEqual(amber.map { $0.key }, [.knowledge, .board, .machine])
        XCTAssertEqual(amber[2].tone, .amber)
    }

    func testTheKnowledgeRowCarriesTheDebtAndTurnsAmberOnlyPastTwoWeeks() {
        var i = HQReportInput()
        i.entries = 478
        i.owed = 7
        i.owedOldestSecs = 6 * 86400
        var row = hqReportRows(i, zh: true).first { $0.key == .knowledge }!
        XCTAssertEqual(row.value, "478 条 · 7 条待你带走 · 最久 6 天")
        XCTAssertEqual(row.tone, .plain, "work in the queue is normal")
        i.owedOldestSecs = 16 * 86400
        row = hqReportRows(i, zh: false).first { $0.key == .knowledge }!
        XCTAssertEqual(row.value, "478 entries · 7 waiting on you · oldest 16d")
        XCTAssertEqual(row.tone, .amber, "past doctor's line the debt is rotting")
        XCTAssertEqual(row.door, .knowledge)
    }

    func testARowWithNothingToSayIsAbsent() {
        // Nothing loaded: no rows, never a table of zeros.
        XCTAssertEqual(hqReportRows(HQReportInput(), zh: true), [])
        // A base that does not exist: no knowledge row; a board never written: no board row.
        var i = HQReportInput()
        i.entries = 0
        XCTAssertTrue(hqReportRows(i, zh: false).isEmpty)
        i.boardUpdatedAt = 0
        XCTAssertTrue(hqReportRows(i, zh: false).isEmpty)
    }

    func testTheDidRowIsTheLastDaysTallyInTheFixedOrder() {
        let now: Int64 = 1_000_000
        let day: Int64 = 24 * 3600
        let tally = hqActTally([
            (kind: "gtmux:audit:knowledge", ts: now - 10),
            (kind: "gtmux:audit:knowledge", ts: now - 20),
            (kind: "gtmux:audit:knowledge", ts: now - 30),
            (kind: "gtmux:audit:send", ts: now - 40),
            (kind: "gtmux:audit:wake-delivered", ts: now - 50), // plumbing, never work
            (kind: "Stop", ts: now - 60),                        // the fleet's, not HQ's
            (kind: "gtmux:audit:reap", ts: now - day - 1),       // yesterday's, outside the window
            (kind: "gtmux:self-check", ts: now - 70),
        ], now: now, windowSecs: day)
        // Dispatch before knowledge although knowledge is more numerous: the order is
        // consequence, not count, and it never reshuffles as the numbers move.
        XCTAssertEqual(tally, [HQActTally(kind: "gtmux:audit:send", n: 1),
                               HQActTally(kind: "gtmux:audit:knowledge", n: 3),
                               HQActTally(kind: "gtmux:self-check", n: 1)])
        let i = HQReportInput(acts: tally)
        XCTAssertEqual(hqReportRows(i, zh: true).first?.value, "24 小时：派活 1 · 记账 3 · 自审 1")
        XCTAssertEqual(hqReportRows(i, zh: false).first?.value, "24h: dispatched 1 · recorded 3 · self-audit 1")
        XCTAssertNil(hqReportRows(i, zh: false).first?.door, "the tally is a reading, not a door")
        XCTAssertEqual(hqActVerb("gtmux:audit:some-new-thing", zh: false), "some new thing")
    }

    func testTheCardOpensItselfOnEnteringRedAndOnlyThen() {
        XCTAssertTrue(hqCardShouldAutoOpen(from: .normal, to: .resource))
        XCTAssertTrue(hqCardShouldAutoOpen(from: .working, to: .needsYou))
        XCTAssertTrue(hqCardShouldAutoOpen(from: .absent, to: .hqCall))
        XCTAssertFalse(hqCardShouldAutoOpen(from: .needsYou, to: .resource), "already red: do not reopen what they closed")
        XCTAssertFalse(hqCardShouldAutoOpen(from: .resource, to: .normal), "leaving red does not touch the card")
        XCTAssertFalse(hqCardShouldAutoOpen(from: .normal, to: .working))
    }
}

/// The usage row (menubar-hq-usage): one window per plan, the tightest, labelled the way
/// the phone labels it, and absent when no plan is readable.
final class HQCardUsageTests: XCTestCase {
    private func win(_ label: String, _ pct: Int, agent: String? = nil, name: String? = nil) -> HQUsageWindow {
        HQUsageWindow(label: label, pctUsed: pct, resetAt: "Sep 18 at 11pm", agent: agent, agentName: name, resetUnix: nil)
    }

    func testOneWindowPerPlanTheTightest() {
        let ws = [win("claude session", 30, agent: "claude"), win("claude week (all models)", 29, agent: "claude"),
                  win("claude week (fable)", 49, agent: "claude"), win("codex week", 0, agent: "codex")]
        let t = hqTightestPerPlan(ws)
        XCTAssertEqual(t.map { $0.label }, ["claude week (fable)", "codex week"])
        XCTAssertEqual(hqPlanLabel(t[0], zh: false), "claude Fable")
        XCTAssertEqual(hqPlanLabel(t[1], zh: true), "codex 周")
        XCTAssertEqual(hqPlanLabel(win("claude session", 30, agent: "claude"), zh: false), "claude 5h")
        XCTAssertEqual(hqPlanLabel(win("claude week (all models)", 29, agent: "claude"), zh: false), "claude wk")
        var i = HQReportInput()
        i.windows = ws
        let row = hqReportRows(i, zh: false).first { $0.key == .usage }!
        XCTAssertEqual(row.value, "claude Fable 49% · codex wk 0%")
        XCTAssertEqual(row.door, .usage)
        XCTAssertTrue(hqReportRows(HQReportInput(), zh: true).isEmpty, "no plan: no row")
    }

    func testTheUsageRowSitsAfterTheDoorsAndBeforeTheMachine() {
        var i = HQReportInput(machine: nil)
        i.entries = 3
        i.boardUpdatedAt = Int64(Date().timeIntervalSince1970) - 60
        i.windows = [win("claude week (all models)", 12, agent: "claude")]
        XCTAssertEqual(hqReportRows(i, zh: false).map { $0.key }, [.knowledge, .board, .usage])
        XCTAssertEqual(hqCompactTok(2_851_826), "2.9M")
        XCTAssertEqual(hqCompactTok(830_400), "830k")
        XCTAssertEqual(hqCompactTok(412), "412")
    }
}
