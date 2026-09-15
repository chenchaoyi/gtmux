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
        // With tokens by day the row leads with today and this week (usage-door-tokens).
        i.history = HQUsageHistory(days: nil, todayOut: 2_802_190, weekOut: 16_216_165, byAgent: nil)
        XCTAssertEqual(hqReportRows(i, zh: false).first { $0.key == .usage }!.value, "today 2.8M · week 16.2M · claude Fable 49% · codex wk 0%")
        XCTAssertEqual(hqReportRows(i, zh: true).first { $0.key == .usage }!.value, "今天 2.8M · 本周 16.2M · claude Fable 49% · codex 周 0%")
        var noPlan = HQReportInput()
        noPlan.history = i.history
        XCTAssertEqual(hqReportRows(noPlan, zh: false).first { $0.key == .usage }!.value, "today 2.8M · week 16.2M", "tokens alone still earn the row")
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

/// Tokens by day (usage-daily-totals): the bars carry today and the tallest day's labels
/// and nothing else, and decode from the CLI's field names.
final class HQUsageHistoryTests: XCTestCase {
    func testSevenBarsLabelTodayAndTheTallest() {
        let json = """
        {"history":{"days":[{"date":"2026-09-08","out":4000000,"in":1},{"date":"2026-09-09","out":12000000,"in":1},
          {"date":"2026-09-10","out":9000000,"in":1},{"date":"2026-09-11","out":0,"in":0},{"date":"2026-09-12","out":20000000,"in":1},
          {"date":"2026-09-13","out":11600000,"in":1},{"date":"2026-09-14","out":12400000,"in":1}],
          "today_out":12400000,"week_out":69000000,
          "by_agent":[{"agent_key":"claude","agent_name":"Claude Code","today_out":12000000,"week_out":60000000}]}}
        """
        let u = try! JSONDecoder().decode(HQUsageReport.self, from: Data(json.utf8))
        let h = u.history!
        XCTAssertEqual(h.todayOut, 12_400_000)
        XCTAssertEqual(h.byAgent?.first?.agentName, "Claude Code")
        let bars = hqDayBars(h.days!, zh: false)
        XCTAssertEqual(bars.count, 7)
        XCTAssertEqual(bars.map { $0.labelled }, [false, false, false, false, true, false, true])
        XCTAssertEqual(bars[4].frac, 1)
        XCTAssertEqual(bars[3].frac, 0)
        XCTAssertTrue(bars[6].today)
        XCTAssertEqual(bars.map { $0.weekday }.joined(), "TWTFSSM")
        XCTAssertEqual(hqDayBars(h.days!, zh: true).map { $0.weekday }.joined(), "二三四五六日一")
        let old = try! JSONDecoder().decode(HQUsageReport.self, from: Data(#"{"sessions":[]}"#.utf8))
        XCTAssertNil(old.history, "an older CLI carries no history and the block is absent")
    }
}

final class HQUsageWindowTitleTests: XCTestCase {
    // The window's identity arrives as data so each surface words it in its own
    // language; the label the agent printed stays the English form and the fallback.
    func testWordsAWindowByKindInChinese() {
        let w = HQUsageWindow(label: "claude week (all models)", pctUsed: 41, resetAt: "Sep 18 at 10:59pm", agent: "claude", agentName: "Claude Code", resetUnix: 1_789_743_540, kind: "week-all", model: nil)
        XCTAssertEqual(hqWindowTitle(w, zh: true), "本周（全部模型）")
        XCTAssertEqual(hqWindowTitle(w, zh: false), "week (all models)")
        let m = HQUsageWindow(label: "claude week (fable)", pctUsed: 58, resetAt: "", agent: "claude", agentName: nil, resetUnix: nil, kind: "week-model", model: "Fable")
        XCTAssertEqual(hqWindowTitle(m, zh: true), "本周（Fable）")
        // An older CLI sends no kind: the label is all there is, in either language.
        let old = HQUsageWindow(label: "codex week", pctUsed: 1, resetAt: "Sep 7 at 3pm", agent: "codex", agentName: nil, resetUnix: nil, kind: nil, model: nil)
        XCTAssertEqual(hqWindowTitle(old, zh: true), "week")
        XCTAssertEqual(hqResetTitle(old, zh: true), "Sep 7 at 3pm")
    }

    func testResetIsALocalDateInChineseWhenTheEpochIsKnown() {
        let w = HQUsageWindow(label: "claude session", pctUsed: 1, resetAt: "Sep 18 at 10:59pm", agent: "claude", agentName: nil, resetUnix: 1_789_743_540, kind: "session", model: nil)
        let s = hqResetTitle(w, zh: true)
        XCTAssertTrue(s.contains("月") && s.contains("日") && s.contains(":"), s)
        XCTAssertEqual(hqResetTitle(w, zh: false), "Sep 18 at 10:59pm")
    }
}

final class HQActivityTests: XCTestCase {
    // The year at a glance: the phone's activityView ported, pinned on the same calendar
    // (Tuesday 2026-09-15) so the two surfaces agree cell for cell.
    private func history() -> HQUsageHistory {
        let json = """
        {"days":[{"date":"2026-09-08","out":3500000,"in":0,"by_agent":{"claude":{"out":3300000,"in":0},"codex":{"out":200000,"in":0}}}],
         "today_out":450000,"week_out":3950000,
         "activity":{"since":"2026-08-01","series":[{"date":"2026-08-12","out":4100000},{"date":"2026-09-08","out":3500000},{"date":"2026-09-14","out":3500000},{"date":"2026-09-15","out":450000}],
           "all_out":11550000,"peak_out":4100000,"peak_date":"2026-08-12","streak":2,"best_streak":2,"active_days":4,"days_known":46}}
        """
        return try! JSONDecoder().decode(HQUsageHistory.self, from: json.data(using: .utf8)!)
    }
    private var today: Date {
        var c = DateComponents(); c.year = 2026; c.month = 9; c.day = 15; c.hour = 10
        return Calendar.current.date(from: c)!
    }

    func testLevelsAgainstThePeak() {
        XCTAssertEqual(hqActivityLevel(0, peak: 100), 0)
        XCTAssertEqual(hqActivityLevel(25, peak: 100), 1)
        XCTAssertEqual(hqActivityLevel(26, peak: 100), 2)
        XCTAssertEqual(hqActivityLevel(75, peak: 100), 3)
        XCTAssertEqual(hqActivityLevel(100, peak: 100), 4)
    }

    func testLaysTheWeeksOutMondayDownAndEndsOnToday() {
        let g = hqActivity(history(), weeks: 20, today: today, zh: false)!
        XCTAssertEqual(g.rows.count, 7)
        XCTAssertEqual(g.rows[0].count, 20)
        let tue = g.rows[1][19]
        XCTAssertEqual(tue.date, "2026-09-15"); XCTAssertTrue(tue.today); XCTAssertEqual(tue.level, 1)
        XCTAssertEqual(g.rows[2][19].level, -1)
        XCTAssertEqual(g.rows[1][18].date, "2026-09-08"); XCTAssertEqual(g.rows[1][18].level, 4)
        XCTAssertEqual(g.months.map { $0.1 }, ["May", "Jun", "Jul", "Aug", "Sep"])
        XCTAssertEqual(g.figs.map { $0.1 }, ["today", "this week", "since Aug 1"])
        XCTAssertEqual(g.figs[0].0, "450k"); XCTAssertEqual(g.figs[2].0, "11.6M")
        XCTAssertEqual(g.stats, ["peak 4.1M · Aug 12", "streak 2d · best 2d", "251k a day", "4 of 46 days active"])
    }

    func testWeeksAndTheReadoutInChinese() {
        let g = hqActivity(history(), weeks: 20, today: today, zh: true)!
        XCTAssertEqual(g.stats[0], "峰值 4.1M · 8月12日")
        XCTAssertEqual(g.rowLabels.joined(), "一三五日")
        XCTAssertTrue(g.weekBars[19].current)
        XCTAssertEqual(g.weekBars[19].out, 3_950_000)
        XCTAssertEqual(g.weekBars[18].level, 4)
        XCTAssertEqual(g.cumulative.last, 1)
        XCTAssertEqual(hqDayReadout(history(), date: "2026-09-08", zh: true), "9月8日 · 3.5M · claude 3.3M · codex 200k")
        XCTAssertNil(hqActivity(HQUsageHistory(days: [], todayOut: 0, weekOut: 0, byAgent: nil, activity: nil), weeks: 20, today: today, zh: true))
    }
}
