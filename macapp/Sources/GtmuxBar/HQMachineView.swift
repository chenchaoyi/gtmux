import SwiftUI

struct HQMachineView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQResourceStore
    @Environment(\.colorScheme) private var scheme
    var body: some View { machineBody(Theme.Palette.of(scheme)) }

    // MARK: machine (menubar-hq-report)

    /// `gtmux resource`, laid out to read: the four readings, the core's own warning
    /// sentence, the per-agent table heaviest first, and the orphans it calls reclaimable
    /// with its own hint. READING ONLY — nothing here kills a process; that would be the
    /// first act in this window that touches the machine, and it gets its own change.
    @ViewBuilder private func machineBody(_ p: Theme.Palette) -> some View {
        if let r = store.resource {
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    readings(r.machine, p)
                    // The core's own words, or its silence: the tier is what the medallion
                    // reads, and this line is the reason behind it.
                    HStack(spacing: 6) {
                        Circle().fill(tierColor(r.machine.tier ?? "", p)).frame(width: 7, height: 7)
                        Text(machineSentence(r.machine)).font(HQTypography.caption).foregroundStyle(p.fg2)
                    }
                    Divider()
                    procTable(r, p)
                    if let orphans = r.orphans, !orphans.isEmpty {
                        Divider()
                        orphanList(orphans, p)
                    }
                }
                .padding(14)
            }
        } else {
            HQResourceEmpty(l10n.tr("Reading the machine…", "正在读取机器…"), p)
        }
    }

    @ViewBuilder private func readings(_ m: ResourceReport.Machine, _ p: Theme.Palette) -> some View {
        let mem = m.memFreePct ?? 0
        let disk = m.diskUsePct ?? 0
        let diskFree = m.diskFreeGB ?? 0
        let warn = m.warn ?? ""
        let loadText = String(format: "%.2f × %d", m.loadRatio ?? 0, m.ncpu ?? 0)
        HStack(spacing: 8) {
            readingCard(
                zh ? "内存 \(mem)% 空闲" : "memory \(mem)% free",
                memTierWord(m.memTier ?? ""), tone: memTierTone(m.memTier ?? ""), p)
            readingCard(
                zh ? "磁盘 \(disk)% 已用" : "disk \(disk)% used",
                zh ? "剩 \(diskFree) GB" : "\(diskFree) GB left",
                tone: warn.hasPrefix("disk") ? .amber : .plain, p)
            readingCard(
                (zh ? "负载 " : "load ") + loadText, zh ? "核" : "cores",
                tone: warn.hasPrefix("load") ? .amber : .plain, p)
            if let b = m.battery, b.present {
                readingCard(
                    zh ? "电量 \(b.percent)%" : "power \(b.percent)%", batteryWord(b),
                    tone: (!b.onAC && b.percent <= 20) ? .amber : .plain, p)
            }
        }
    }

    private func batteryWord(_ b: ResourceReport.Battery) -> String {
        if b.onAC { return l10n.tr("AC", "电源") }
        if let t = b.timeLeft, !t.isEmpty { return l10n.tr("battery \(t)", "电池 \(t)") }
        return l10n.tr("battery", "电池")
    }

    @ViewBuilder private func procTable(_ r: ResourceReport, _ p: Theme.Palette) -> some View {
        let rows = (r.agents ?? [:]).map { (pane: $0.key, rss: $0.value.rssMB, cpu: $0.value.cpu) }
            .sorted { $0.rss != $1.rss ? $0.rss > $1.rss : $0.pane < $1.pane }
        let heaviest = rows.first?.rss ?? 0
        let red = (r.machine.tier ?? "") == "red"
        if rows.isEmpty {
            Text(l10n.tr("No agent process to measure", "没有可测量的 agent 进程"))
                .font(HQTypography.caption).foregroundStyle(p.fg3)
        } else {
            procHeader(p)
            ForEach(rows, id: \.pane) { row in
                procRow(
                    pane: row.pane, name: store.paneNames[row.pane] ?? "", rss: row.rss, cpu: row.cpu,
                    hot: red && row.rss >= heaviest, p)
            }
        }
    }

    @ViewBuilder private func orphanList(_ orphans: [ResourceReport.Orphan], _ p: Theme.Palette)
        -> some View
    {
        Text(l10n.tr("Reclaimable: no live agent owns these", "可回收：没有活着的 agent 拥有它们"))
            .font(HQTypography.rowTitle).foregroundStyle(p.fg3)
        ForEach(orphans) { o in
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 8) {
                    Text("pid \(o.pid)").font(HQTypography.code).foregroundStyle(p.fg3)
                    Text(o.comm).font(HQTypography.caption).foregroundStyle(p.fg)
                    if let k = o.kind, !k.isEmpty {
                        Text(k).font(HQTypography.caption).foregroundStyle(p.fg3)
                            .padding(.horizontal, 5).padding(.vertical, 1)
                            .background(RoundedRectangle(cornerRadius: 4).fill(p.fg3.opacity(0.12)))
                    }
                    Spacer()
                    Text("\(o.rssMB) MB · " + String(format: "%.1f%%", o.cpu))
                        .font(HQTypography.code).foregroundStyle(p.fg2)
                }
                if let h = o.hint, !h.isEmpty {
                    Text(h).font(HQTypography.caption).foregroundStyle(p.fg3).textSelection(.enabled)
                }
            }
            .padding(.vertical, 3)
        }
    }

    private var zh: Bool { l10n.lang == "zh" }

    private func memTierWord(_ tier: String) -> String {
        switch tier {
        case "critical": return l10n.tr("critical", "临界")
        case "warn": return l10n.tr("warn", "警戒")
        default: return l10n.tr("normal", "正常")
        }
    }

    private func memTierTone(_ tier: String) -> HQRowTone {
        switch tier {
        case "critical": return .red
        case "warn": return .amber
        default: return .plain
        }
    }

    private func tierColor(_ tier: String, _ p: Theme.Palette) -> Color {
        switch tier {
        case "red": return Theme.Status.waiting
        case "amber": return Theme.Status.errored
        default: return Theme.Status.idle
        }
    }

    /// The sentence under the readings: the core's warning verbatim when it has one.
    private func machineSentence(_ m: ResourceReport.Machine) -> String {
        if let w = m.warn, !w.isEmpty {
            switch m.tier ?? "" {
            case "red":
                return l10n.tr(
                    "Red: \(w). This is what the ⚠ on the HQ badge means.", "红档：\(w)。HQ 徽章上的 ⚠ 说的就是它。")
            default:
                return l10n.tr(
                    "Amber: \(w). Worth knowing; nothing is blocked yet.", "琥珀：\(w)。提个醒，目前还没卡住什么。")
            }
        }
        return l10n.tr(
            "Nothing is short. These readings come from gtmux resource.", "没有短缺。这些读数来自 gtmux resource。")
    }

    @ViewBuilder private func readingCard(
        _ value: String, _ sub: String, tone: HQRowTone, _ p: Theme.Palette
    ) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(value).font(HQTypography.rowTitle).foregroundStyle(p.fg).lineLimit(1)
            Text(sub).font(HQTypography.caption)
                .foregroundStyle(
                    tone == .red ? Theme.Status.waiting : tone == .amber ? Theme.Status.errored : p.fg3
                )
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 10).padding(.vertical, 8)
        .background(
            RoundedRectangle(cornerRadius: 8, style: .continuous).fill(p.rowSelected.opacity(0.5))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 8, style: .continuous).strokeBorder(p.divider, lineWidth: 1))
    }

    @ViewBuilder private func procHeader(_ p: Theme.Palette) -> some View {
        HStack(spacing: 10) {
            Text("PANE").frame(width: 44, alignment: .leading)
            Text(l10n.tr("SESSION", "会话")).frame(maxWidth: .infinity, alignment: .leading)
            Text("RSS").frame(width: 72, alignment: .trailing)
            Text("CPU").frame(width: 60, alignment: .trailing)
        }
        .font(HQTypography.rowTitle).kerning(0.5).foregroundStyle(p.fg3)
    }

    @ViewBuilder private func procRow(
        pane: String, name: String, rss: Int, cpu: Double, hot: Bool, _ p: Theme.Palette
    ) -> some View {
        HStack(spacing: 10) {
            Text(pane).font(HQTypography.code).foregroundStyle(p.fg3).frame(width: 44, alignment: .leading)
            Text(name).font(HQTypography.caption).foregroundStyle(p.fg).lineLimit(1)
                .frame(maxWidth: .infinity, alignment: .leading)
            Text("\(rss) MB").font(HQTypography.code).foregroundStyle(hot ? Theme.Status.waiting : p.fg2)
                .frame(width: 72, alignment: .trailing)
            Text(String(format: "%.1f%%", cpu)).font(HQTypography.code).foregroundStyle(
                hot ? Theme.Status.waiting : p.fg2
            )
            .frame(width: 60, alignment: .trailing)
        }
        .padding(.vertical, 4)
        .overlay(alignment: .top) { Divider().overlay(p.divider) }
    }

}
