import SwiftUI

final class HQUsageNavigation: ObservableObject {
    @Published var mode = 0
    @Published var picked = ""
}

struct HQUsageView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQResourceStore
    @ObservedObject var navigation: HQUsageNavigation
    @Environment(\.colorScheme) private var scheme
    private var zh: Bool { l10n.lang == "zh" }
    var body: some View { usageBody(Theme.Palette.of(scheme)) }

    // MARK: usage (menubar-hq-usage)

    /// The phone's usage sheet, on the Mac, in the CLI's order: the quota (the number no
    /// local arithmetic can produce) → who is burning it (per-agent totals, then the
    /// sessions, sorted by trouble) — the machine has its own tab here. A card at the top
    /// says the one thing the reader came for: which window is tightest.
    @ViewBuilder private func usageBody(_ p: Theme.Palette) -> some View {
        if let u = store.usage {
            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    usageLead(u, p)
                    HQUsageAccounts(l10n: l10n, report: u)
                    if let h = u.history, let days = h.days, !days.isEmpty {
                        Divider()
                        Text(l10n.tr("Output tokens", "输出 Token")).font(HQTypography.rowTitle).foregroundStyle(p.fg)
                        Text(l10n.tr("Recorded conversations · local days", "已记录的对话 · 按当地日期"))
                            .font(HQTypography.caption).foregroundStyle(p.fg3)
                        usageTokens(h, days, p)
                    }
                    usageBurn(u, p)
                }
                .padding(14)
            }
        } else {
            HQResourceEmpty(l10n.tr("Reading usage…", "正在读取用量…"), p)
        }
    }

    /// The tightest non-session window, said in one sentence. Session windows reset in
    /// hours, so a high one is a working day, not news (the phone's rule).
    @ViewBuilder private func usageLead(_ u: HQUsageReport, _ p: Theme.Palette) -> some View {
        let all = (u.limits?.windows ?? []).filter { $0.stale != true }
        let weekly = all.filter { ($0.kind ?? hqWindowName($0).lowercased()) != "session" }
        let pool = weekly.isEmpty ? all : weekly
        if hqUsageShowsLead(all), let t = pool.max(by: { $0.pctUsed < $1.pctUsed }) {
            let name = t.agentName ?? t.agent ?? ""
            HStack(spacing: 8) {
                Text("\(t.pctUsed)%").font(HQTypography.title).foregroundStyle(p.fg)
                VStack(alignment: .leading, spacing: 2) {
                    Text(
                        l10n.tr(
                            "Highest usage: \(name) \(hqWindowTitle(t, zh: zh))",
                            "用量最高：\(name) \(hqWindowTitle(t, zh: zh))")
                    )
                    .font(HQTypography.rowTitle).foregroundStyle(p.fg)
                    let r = hqResetTitle(t, zh: zh)
                    if !r.isEmpty {
                        Text(l10n.tr("resets \(r)", "重置于 \(r)")).font(HQTypography.caption).foregroundStyle(p.fg3)
                    }
                }
                Spacer()
            }
            .padding(10)
            .background(
                RoundedRectangle(cornerRadius: 8, style: .continuous).fill(p.rowSelected.opacity(0.5))
            )
            .overlay(
                RoundedRectangle(cornerRadius: 8, style: .continuous).strokeBorder(p.divider, lineWidth: 1))
        }
    }

    /// Tokens by day (usage-daily-totals): today and the last seven days across every agent, seven
    /// bars — one neutral series, today's in the stronger ink, direct labels on today and
    /// the tallest day, weekday initials beneath — then the week's split per agent.
    @ViewBuilder private func usageTokens(
        _ h: HQUsageHistory, _ days: [HQUsageDay], _ p: Theme.Palette
    ) -> some View {
        if let grid = hqActivity(h, weeks: 44, today: Date(), zh: zh) {
            usageActivity(h, grid, p)
        } else {
            usageDayBars(h, days, p)
        }
    }

    /// The year at a glance (usage-activity, 2026-09-15): three figures, the stats a
    /// reader asks of a year, then one of three pictures of the same series — the phone's
    /// block with 44 columns. The greens are GitHub's contribution ramp (the commander's
    /// choice, so the picture reads the same everywhere); a chart, not a status.
    @ViewBuilder private func usageActivity(
        _ h: HQUsageHistory, _ grid: HQActivityGrid, _ p: Theme.Palette
    ) -> some View {
        let ramp = (scheme == .dark ? hqActivityRampDark : hqActivityRampLight).map {
            Color(hex: $0.0, opacity: $0.1)
        }
        let todayKey = grid.rows.flatMap { $0 }.first { $0.today }?.date ?? ""
        let readout = hqDayReadout(
            h, date: navigation.picked.isEmpty ? todayKey : navigation.picked, zh: zh)
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline, spacing: 18) {
                ForEach(Array(grid.figs.enumerated()), id: \.offset) { _, f in
                    VStack(alignment: .leading, spacing: 1) {
                        Text(f.0).font(HQTypography.title).foregroundStyle(p.fg)
                        Text(f.1).font(HQTypography.caption).foregroundStyle(p.fg3)
                    }
                }
                Spacer()
                Picker("", selection: $navigation.mode) {
                    Text(l10n.tr("daily", "按天")).tag(0)
                    Text(l10n.tr("weekly", "按周")).tag(1)
                    Text(l10n.tr("cumulative", "累计")).tag(2)
                }
                .pickerStyle(.segmented).labelsHidden().frame(width: 200)
            }
            HStack(spacing: 12) {
                ForEach(grid.stats, id: \.self) { s in
                    Text(s).font(HQTypography.caption).foregroundStyle(p.fg2)
                }
                Spacer()
                Text(grid.range).font(HQTypography.caption).foregroundStyle(p.fg3)
            }
            if navigation.mode == 0 {
                VStack(alignment: .leading, spacing: 3) {
                    ZStack(alignment: .topLeading) {
                        ForEach(Array(grid.months.enumerated()), id: \.offset) { _, m in
                            Text(m.1).font(HQTypography.caption).foregroundStyle(p.fg3).offset(
                                x: CGFloat(26 + m.0 * 14))
                        }
                    }
                    .frame(height: 13, alignment: .topLeading)
                    ForEach(Array(grid.rows.enumerated()), id: \.offset) { ri, row in
                        HStack(spacing: 3) {
                            Text(grid.rowLabels[ri]).font(HQTypography.caption).foregroundStyle(p.fg3)
                                .lineLimit(1).minimumScaleFactor(0.75).frame(
                                width: 23, alignment: .trailing)
                            ForEach(Array(row.enumerated()), id: \.offset) { _, c in
                                if c.level < 0 {
                                    Color.clear.frame(width: 11, height: 11)
                                } else {
                                    let ringed = navigation.picked.isEmpty ? c.today : navigation.picked == c.date
                                    RoundedRectangle(cornerRadius: 2.5, style: .continuous)
                                        .fill(ramp[c.level])
                                        .overlay(
                                            RoundedRectangle(cornerRadius: 2.5, style: .continuous).stroke(
                                                ringed ? (navigation.picked.isEmpty ? p.fg2 : p.fg) : .clear,
                                                lineWidth: 1.5
                                            )
                                        )
                                        .frame(width: 11, height: 11)
                                        .help("\(c.date) · \(hqCompactTok(c.out))")
                                        .onTapGesture { navigation.picked = c.date }
                                }
                            }
                        }
                    }
                    HStack(spacing: 3) {
                        Text(readout).font(HQTypography.code).foregroundStyle(p.fg2).lineLimit(1)
                        Spacer()
                        Text(l10n.tr("Less", "少")).font(HQTypography.caption).foregroundStyle(p.fg3)
                        ForEach(Array(ramp.enumerated()), id: \.offset) { _, c in
                            RoundedRectangle(cornerRadius: 2).fill(c).frame(width: 9, height: 9)
                        }
                        Text(l10n.tr("More", "多")).font(HQTypography.caption).foregroundStyle(p.fg3)
                    }
                    .padding(.top, 4).padding(.leading, 26)
                }
            } else if navigation.mode == 1 {
                VStack(alignment: .leading, spacing: 3) {
                    HStack(alignment: .bottom, spacing: 3) {
                        ForEach(Array(grid.weekBars.enumerated()), id: \.offset) { _, b in
                            RoundedRectangle(cornerRadius: 2, style: .continuous)
                                .fill(ramp[b.level])
                                .overlay(
                                    RoundedRectangle(cornerRadius: 2, style: .continuous).stroke(
                                        b.current ? p.fg2 : .clear, lineWidth: 1.5)
                                )
                                .frame(height: max(2, 60 * b.frac))
                                .frame(maxWidth: .infinity)
                                .help("\(b.start) · \(hqCompactTok(b.out))")
                        }
                    }
                    .frame(height: 64)
                    HStack(spacing: 3) {
                        ForEach(Array(grid.weekBars.enumerated()), id: \.offset) { _, b in
                            Text(b.month.isEmpty ? " " : b.month).font(.system(size: 9)).foregroundStyle(p.fg3)
                                .lineLimit(1).frame(maxWidth: .infinity)
                        }
                    }
                    if let top = grid.weekBars.max(by: { $0.out < $1.out }) {
                        Text(
                            l10n.tr(
                                "the week of \(top.start) was the highest · \(hqCompactTok(top.out)) · the ringed bar is this week, still running",
                                "\(top.start) 那周最高 · \(hqCompactTok(top.out)) · 描边的是本周，还没过完")
                        )
                        .font(HQTypography.caption).foregroundStyle(p.fg2).lineLimit(1).padding(.top, 4)
                    }
                }
            } else {
                VStack(alignment: .leading, spacing: 4) {
                    GeometryReader { g in
                        Path { path in
                            let n = grid.cumulative.count
                            guard n > 1 else { return }
                            for (i, v) in grid.cumulative.enumerated() {
                                let pt = CGPoint(
                                    x: g.size.width * CGFloat(i) / CGFloat(n - 1), y: g.size.height * (1 - CGFloat(v))
                                )
                                if i == 0 { path.move(to: pt) } else { path.addLine(to: pt) }
                            }
                        }
                        .stroke(ramp[4], style: StrokeStyle(lineWidth: 2, lineCap: .round, lineJoin: .round))
                    }
                    .frame(height: 64)
                    HStack {
                        Text(grid.range).font(HQTypography.caption).foregroundStyle(p.fg3)
                        Spacer()
                        Text(grid.cumulativeLabel).font(HQTypography.rowTitle).foregroundStyle(
                            p.fg)
                    }
                }
            }
            usageAgentSplit(h, p)
        }
    }

    /// A table within token history, with one shared reporting period per column.
    @ViewBuilder private func usageAgentSplit(_ h: HQUsageHistory, _ p: Theme.Palette) -> some View {
        if let agents = h.byAgent, !agents.isEmpty {
            VStack(alignment: .leading, spacing: 6) {
                Divider()
                HStack {
                    Text(l10n.tr("By agent", "按 Agent 统计"))
                    Spacer()
                    Text(l10n.tr("Today", "今天")).frame(width: 100, alignment: .trailing)
                    Text(l10n.tr("Last 7 days", "近 7 天")).frame(width: 100, alignment: .trailing)
                }.font(HQTypography.caption).foregroundStyle(p.fg3)
                ForEach(agents, id: \.agentKey) { a in
                    HStack {
                        Text(a.agentName ?? a.agentKey).font(HQTypography.caption).foregroundStyle(p.fg2)
                        Spacer()
                        Text(hqCompactTok(a.todayOut)).font(HQTypography.code).foregroundStyle(p.fg2)
                            .frame(width: 100, alignment: .trailing)
                        Text(hqCompactTok(a.weekOut)).font(HQTypography.code).foregroundStyle(p.fg)
                            .frame(width: 100, alignment: .trailing)
                    }
                }
            }.padding(.top, 4)
        }
    }

    /// An older CLI (no `activity`): the seven-day bars as before.
    @ViewBuilder private func usageDayBars(
        _ h: HQUsageHistory, _ days: [HQUsageDay], _ p: Theme.Palette
    ) -> some View {
        let bars = hqDayBars(days, zh: zh)
        let today = h.todayOut ?? days.last?.out ?? 0
        let week = h.weekOut ?? days.reduce(0) { $0 + $1.out }
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline, spacing: 18) {
                VStack(alignment: .leading, spacing: 1) {
                    Text(hqCompactTok(today)).font(HQTypography.title).foregroundStyle(p.fg)
                    Text(l10n.tr("today", "今天")).font(HQTypography.caption).foregroundStyle(p.fg3)
                }
                VStack(alignment: .leading, spacing: 1) {
                    Text(hqCompactTok(week)).font(HQTypography.title).foregroundStyle(p.fg)
                    Text(l10n.tr("last 7 days", "近 7 天")).font(HQTypography.caption).foregroundStyle(p.fg3)
                }
                Spacer()
                Text(l10n.tr("Output tokens", "输出 Token"))
                    .font(HQTypography.caption).foregroundStyle(p.fg3)
            }
            HStack(alignment: .bottom, spacing: 8) {
                ForEach(Array(bars.enumerated()), id: \.offset) { _, b in
                    VStack(spacing: 3) {
                        Text(b.labelled ? hqCompactTok(b.out) : " ")
                            .font(HQTypography.code).foregroundStyle(b.today ? p.fg : p.fg2)
                        RoundedRectangle(cornerRadius: 2, style: .continuous)
                            .fill(b.today ? p.fg2 : p.fg3.opacity(0.55))
                            .frame(height: max(2, 56 * b.frac))
                            .frame(maxWidth: .infinity)
                            .help("\(b.date) · \(hqCompactTok(b.out))")
                        Text(b.weekday).font(HQTypography.caption).foregroundStyle(b.today ? p.fg : p.fg3)
                    }
                }
            }
            .frame(height: 84)
            usageAgentSplit(h, p)
        }
    }

    /// Who is burning it: sessions by trouble — the alerted first, then by burn rate,
    /// then by context share; the parked fold into one count row.
    @ViewBuilder private func usageBurn(_ u: HQUsageReport, _ p: Theme.Palette) -> some View {
        Divider()
        let ranked = (u.sessions ?? []).sorted { a, b in
            let aw = (a.usageWarn ?? "").isEmpty ? 0 : 1
            let bw = (b.usageWarn ?? "").isEmpty ? 0 : 1
            if aw != bw { return aw > bw }
            if a.rate != b.rate { return a.rate > b.rate }
            if (a.ctx ?? 0) != (b.ctx ?? 0) { return (a.ctx ?? 0) > (b.ctx ?? 0) }
            return a.identity < b.identity
        }
        let shown = ranked.filter { !($0.usageWarn ?? "").isEmpty || $0.rate > 0 }
        let rest = ranked.filter { ($0.usageWarn ?? "").isEmpty && $0.rate <= 0 }
        if !shown.isEmpty {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(shown, id: \.identity) { s in
                    HStack(spacing: 10) {
                        Text(s.paneID ?? "—").font(HQTypography.code).foregroundStyle(p.fg3).frame(
                            width: 44, alignment: .leading)
                        VStack(alignment: .leading, spacing: 1) {
                            Text(s.displayLocation).font(HQTypography.caption).foregroundStyle(p.fg).lineLimit(1)
                            if let w = s.usageWarn, !w.isEmpty {
                                Text(w).font(HQTypography.caption).foregroundStyle(Theme.Status.errored).lineLimit(1)
                            }
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        Text(hqCompactTok(s.tok)).font(HQTypography.code).foregroundStyle(p.fg2).frame(
                            width: 60, alignment: .trailing)
                        Text(String(format: "%.0f/s", s.rate)).font(HQTypography.code).foregroundStyle(p.fg3)
                            .frame(width: 56, alignment: .trailing)
                        Text("ctx \(Int(((s.ctx ?? 0) * 100).rounded()))%").font(HQTypography.code)
                            .foregroundStyle(p.fg3).frame(width: 64, alignment: .trailing)
                    }
                    .padding(.vertical, 4)
                    .overlay(alignment: .top) { Divider().overlay(p.divider) }
                }
            }
        }
        if !rest.isEmpty {
            Text(
                l10n.tr(
                    "\(rest.count) parked sessions · \(hqCompactTok(rest.reduce(0) { $0 + $1.tok })) so far",
                    "\(rest.count) 个停着的会话 · 累计 \(hqCompactTok(rest.reduce(0) { $0 + $1.tok }))")
            )
            .font(HQTypography.caption).foregroundStyle(p.fg3)
        }
    }

}
