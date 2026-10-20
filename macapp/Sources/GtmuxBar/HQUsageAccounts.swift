import SwiftUI

/// Platform limits and money retain their own units and provider reset windows.
struct HQUsageAccounts: View {
    @ObservedObject var l10n: L10n
    let report: HQUsageReport
    @State var detailsExpanded = false
    @Environment(\.colorScheme) private var scheme
    private var zh: Bool { l10n.lang == "zh" }
    private var current: [HQUsageWindowGroup] { hqUsageWindowGroups(report.limits?.windows ?? [], historical: false) }
    private var historical: [HQUsageWindowGroup] { hqUsageWindowGroups(report.limits?.windows ?? [], historical: true) }
    private var balances: [HQUsageBalance] { report.limits?.balances ?? [] }
    private var unknown: [HQUsageUnknownPlan] { report.limits?.unknown ?? [] }
    private var dshGap: Bool {
        report.sources?.contains(where: { $0.agent == "dsh" }) == true && !balances.contains(where: { $0.agent == "dsh" })
    }

    var body: some View {
        let p = Theme.Palette.of(scheme)
        let oldCount = historical.reduce(0) { $0 + $1.windows.count }
        if !current.isEmpty || !balances.isEmpty || oldCount > 0 || !unknown.isEmpty || dshGap {
            VStack(alignment: .leading, spacing: 8) {
                Text(l10n.tr("Limits & balances", "额度与余额")).font(HQTypography.rowTitle).foregroundStyle(p.fg)
                Text(l10n.tr("Reported by your providers", "平台报告的额度与账户余额"))
                    .font(HQTypography.caption).foregroundStyle(p.fg3)
                ForEach(current) { group in windows(group, p) }
                ForEach(balances, id: \.accountID) { balance in
                    VStack(alignment: .leading, spacing: 5) {
                        Text(balance.agent == "dsh" ? "DeepSeek Account" : balance.agentName)
                            .font(HQTypography.rowTitle).foregroundStyle(p.fg)
                        ForEach(Array(balance.wallets.enumerated()), id: \.offset) { _, wallet in
                            HStack {
                                Text(wallet.kind == "bonus" ? l10n.tr("Bonus", "赠送") : l10n.tr("Recharge", "充值"))
                                Spacer()
                                Text(wallet.currency + " " + wallet.amount).textSelection(.enabled)
                            }.font(HQTypography.caption).foregroundStyle(p.fg2)
                        }
                    }
                }
                if current.isEmpty && balances.isEmpty {
                    Text(l10n.tr("Current limits and balances are unavailable.", "暂时没有可用的当前额度或余额。"))
                        .font(HQTypography.caption).foregroundStyle(p.fg3)
                }
                Divider()
                DisclosureGroup(isExpanded: $detailsExpanded) {
                    VStack(alignment: .leading, spacing: 10) {
                        Text(l10n.tr(
                            "Limits and balances come from providers. Output tokens below cover conversations recorded by the connected machine; they are not a bill or a quota percentage.",
                            "额度和余额来自平台；下方输出 Token 只统计连接机器已记录的对话，不代表账单，也不能换算成额度百分比。"))
                            .font(HQTypography.caption).foregroundStyle(p.fg2)
                        ForEach(current) { group in
                            VStack(alignment: .leading, spacing: 3) {
                                Text(group.name).foregroundStyle(p.fg2)
                                ForEach(Array(group.windows.enumerated()), id: \.offset) { _, w in
                                    Text(metadata(w)).foregroundStyle(p.fg3)
                                }
                            }.font(HQTypography.caption)
                        }
                        ForEach(balances, id: \.accountID) { b in
                            Text(b.agentName + " · " + readAt(b.observedAt)).font(HQTypography.caption).foregroundStyle(p.fg3)
                        }
                        ForEach(historical) { group in windows(group, p) }
                        ForEach(unknown, id: \.agent) { gap in
                            Text((gap.agentName ?? gap.agent) + " · " + (gap.reason == "rolled-over"
                                ? l10n.tr("The last reported window has ended; a new session report can refresh it.", "上次报告的窗口已经过去，可由新的会话报告更新。")
                                : l10n.tr("Its limits are not readable right now.", "现在读不到额度。")))
                                .font(HQTypography.caption).foregroundStyle(p.fg3)
                        }
                        if dshGap {
                            Text("DeepSeek Account · " + hqUsageSourceNote(report.limits?.dshAccountStatus, zh: zh))
                                .font(HQTypography.caption).foregroundStyle(p.fg3)
                        }
                    }.padding(.top, 8)
                } label: {
                    Text(l10n.tr("History & sources", "历史与数据来源") + (oldCount > 0 ? l10n.tr(" · \(oldCount) historical", " · \(oldCount) 条旧读数") : ""))
                        .font(HQTypography.caption).foregroundStyle(p.fg2)
                }
                .accessibilityIdentifier("usage-account-details")
            }
        }
    }

    private func readAt(_ at: Int64) -> String {
        l10n.tr("Read ", "读取于 ") + Date(timeIntervalSince1970: TimeInterval(at)).formatted(date: .abbreviated, time: .shortened)
    }
    private func metadata(_ w: HQUsageWindow) -> String {
        [hqWindowTitle(w, zh: zh), w.planType.map { l10n.tr("Recorded plan: ", "记录的套餐：") + $0 }, w.observedAt.map(readAt)]
            .compactMap { $0 }.joined(separator: " · ")
    }
    private func quotaTone(_ w: HQUsageWindow) -> (fill: Color, track: Color, edge: Color?, text: Color?) {
        if w.stale == true { return (.secondary, .secondary.opacity(0.15), nil, .secondary) }
        let dark = scheme == .dark
        let wash = dark ? 0.24 : 0.22
        let line = dark ? 0.32 : 0.30
        switch hqQuotaTier(w) {
        case .full: return (Theme.Status.waiting, Theme.Status.waiting, nil, Theme.Status.waiting)
        case .warn:
            let a = Theme.Status.errored
            return (a, a.opacity(wash), a.opacity(line), dark ? a : Theme.Quota.amberTextLight)
        case .normal:
            let b = dark ? Theme.Quota.blueDark : Theme.Quota.blueLight
            return (b, b.opacity(wash), b.opacity(line), nil)
        }
    }
    @ViewBuilder private func windows(_ group: HQUsageWindowGroup, _ p: Theme.Palette) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(group.name).font(HQTypography.rowTitle).foregroundStyle(p.fg)
            ForEach(Array(group.windows.enumerated()), id: \.offset) { _, w in
                VStack(alignment: .leading, spacing: 4) {
                    HStack(spacing: 10) {
                        Text(hqWindowTitle(w, zh: zh).replacingOccurrences(of: "^week(?=$|[ (])|^本周(?=$|[（(])", with: zh ? "周额度" : "Weekly limit", options: .regularExpression))
                            .font(HQTypography.caption).foregroundStyle(p.fg2)
                            .frame(width: 150, alignment: .leading)
                        let tone = quotaTone(w)
                        GeometryReader { g in
                            ZStack(alignment: .leading) {
                                Capsule().fill(tone.track)
                                    .overlay(Capsule().strokeBorder(tone.edge ?? .clear, lineWidth: 0.5))
                                if w.pctUsed > 0 {
                                    Capsule().fill(tone.fill).frame(width: max(6, g.size.width * CGFloat(min(w.pctUsed, 100)) / 100))
                                }
                            }
                        }.frame(height: 6)
                        Text(l10n.tr("\(w.pctUsed)% used", "已用 \(w.pctUsed)%")).font(HQTypography.code)
                            .foregroundStyle(tone.text ?? p.fg).frame(width: 80, alignment: .trailing)
                    }
                    Text(w.stale == true
                         ? [l10n.tr("Historical reading", "旧读数"), w.observedAt.map(readAt)].compactMap { $0 }.joined(separator: " · ")
                         : hqUsageResetSummary(w, zh: zh))
                        .font(HQTypography.caption).foregroundStyle(p.fg3)
                }
            }
        }
    }
}
