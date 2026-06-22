import AppKit
import SwiftUI

enum MenuAction {
    case overview, watch, restore, newSession, preferences, quit
}

/// MenuView is the popover (DESIGN §3): a header (logo + waiting-only + search +
/// summary), the agents grouped Needs-you → Working → Idle → Running, and an
/// actions footer. A custom view (not NSMenu) so it can group, emphasize "needs
/// you", and stay calm elsewhere.
struct MenuView: View {
    @ObservedObject var store: AgentStore
    @ObservedObject var l10n: L10n
    @ObservedObject var remote = RemoteAccess.shared
    var onJump: (Agent) -> Void
    var onAction: (MenuAction) -> Void
    var onClose: () -> Void = {}

    @State private var waitingOnly = false
    @State private var searchActive = false
    @State private var searchText = ""
    @State private var selected = 0
    @FocusState private var rootFocused: Bool
    @FocusState private var searchFocused: Bool
    @Environment(\.colorScheme) private var scheme

    private var query: String { searchActive ? searchText : "" }
    private var sections: [(status: Status, agents: [Agent])] {
        store.sections(waitingOnly: waitingOnly, query: query)
    }
    private var flat: [Agent] { sections.flatMap { $0.agents } }

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            header(p)
            Divider().overlay(p.divider)
            content(p)
            Divider().overlay(p.divider)
            footer(p)
        }
        .frame(width: Theme.Size.popoverWidth)
        .background {
            // Vibrancy blur + the DESIGN §9 tint, so it's a proper frosted panel
            // (not the bare blur that let the terminal bleed through as gray).
            ZStack { VisualEffect(); p.bg }.ignoresSafeArea()
        }
        .focusable()
        .focused($rootFocused)
        .onKeyPress(.upArrow) { move(-1); return .handled }
        .onKeyPress(.downArrow) { move(1); return .handled }
        .onKeyPress(.return) { jumpSelected(); return .handled }
        .onKeyPress(.escape) { onEscape(); return .handled }
        .onAppear { selected = 0; if !searchActive { rootFocused = true }; remote.refresh() }
    }

    // MARK: header

    @ViewBuilder private func header(_ p: Theme.Palette) -> some View {
        VStack(spacing: 5) {
            HStack(spacing: 7) {
                GtmuxLogo(size: 15)
                Text("gtmux").font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg)
                Spacer()
                waitingOnlyButton(p)
                Button { toggleSearch() } label: {
                    Image(systemName: "magnifyingglass").font(.system(size: 11))
                        .foregroundStyle(searchActive ? Theme.Status.working : p.fg2)
                }.buttonStyle(.plain)
            }
            if searchActive {
                TextField(l10n.tr("Search agents…", "搜索 agent…"), text: $searchText)
                    .textFieldStyle(.plain).font(.system(size: 12)).foregroundStyle(p.fg)
                    .focused($searchFocused)
            } else {
                HStack { Text(summaryText).font(Theme.Font.summary).foregroundStyle(p.fg2); Spacer() }
            }
        }
        .padding(.horizontal, 12).padding(.top, 10).padding(.bottom, 7)
    }

    private func waitingOnlyButton(_ p: Theme.Palette) -> some View {
        Button { withAnimation(nil) { waitingOnly.toggle() } } label: {
            HStack(spacing: 3) {
                Image(systemName: waitingOnly ? "checkmark.circle.fill" : "circle")
                    .font(.system(size: 10)).foregroundStyle(waitingOnly ? Theme.Status.waiting : p.fg3)
                Text(l10n.tr("Waiting only", "仅等待")).font(.system(size: 11)).foregroundStyle(p.fg2)
            }
        }.buttonStyle(.plain)
    }

    private var summaryText: String {
        let n = store.total
        if n == 0 { return l10n.tr("no agents", "没有 agent") }
        var parts: [String] = []
        if store.waiting > 0 { parts.append(l10n.tr("\(store.waiting) awaiting input", "\(store.waiting) 待输入")) }
        parts.append(l10n.tr("\(store.working) working", "\(store.working) 运行中"))
        parts.append(l10n.tr("\(store.idleCount) completed", "\(store.idleCount) 已完成"))
        let agents = l10n.tr("\(n) agent\(n == 1 ? "" : "s")", "\(n) 个 agent")
        return agents + " · " + parts.joined(separator: " · ")
    }

    // MARK: content

    @ViewBuilder private func content(_ p: Theme.Palette) -> some View {
        if store.total == 0 {
            EmptyStateView(l10n: l10n, onNew: { onAction(.newSession) })
        } else if flat.isEmpty {
            Text(l10n.tr("No matches", "无匹配"))
                .font(.system(size: 12)).foregroundStyle(p.fg3)
                .frame(maxWidth: .infinity).padding(.vertical, 22)
        } else {
            ScrollView {
                // ONE flat ForEach (headers + rows) — nested ForEach left a row's
                // status badge stale when an agent moved between sections.
                LazyVStack(alignment: .leading, spacing: 1) {
                    ForEach(listItems) { item in
                        switch item {
                        case let .header(status, count):
                            SectionHeader(status: status, count: count, l10n: l10n)
                        case let .agent(agent, idx):
                            AgentRowView(agent: agent, selected: idx == selected, l10n: l10n)
                                .onHover { if $0 { selected = idx } }
                                .onTapGesture { onJump(agent) }
                        }
                    }
                }
                .padding(.vertical, 4)
            }
            .frame(maxHeight: Theme.Size.listMaxHeight)
        }
    }

    /// Flattened list of section headers + agent rows, each with a stable id, so
    /// SwiftUI reconciles a single identity space (no cross-section staleness).
    private enum ListItem: Identifiable {
        case header(Status, Int)
        case agent(Agent, Int) // agent + its flat index (for keyboard selection)
        var id: String {
            switch self {
            case let .header(s, _): return "h:" + s.rawValue
            // status is part of the identity → a status change rebuilds the row,
            // so the badge can never go stale (defends the working/waiting bug).
            case let .agent(a, _): return "a:" + a.id + ":" + a.status
            }
        }
    }

    private var listItems: [ListItem] {
        var items: [ListItem] = []
        var idx = 0
        for section in sections {
            items.append(.header(section.status, section.agents.count))
            for a in section.agents {
                items.append(.agent(a, idx))
                idx += 1
            }
        }
        return items
    }

    // MARK: footer

    @ViewBuilder private func footer(_ p: Theme.Palette) -> some View {
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                footerAction("square.grid.2x2", l10n.tr("Overview", "概览")) { onAction(.overview) }
                footerAction("waveform", l10n.tr("Watch", "实时")) { onAction(.watch) }
                footerAction("arrow.uturn.backward", l10n.tr("Restore", "接回")) { onAction(.restore) }
                footerAction("plus", l10n.tr("New", "新建")) { onAction(.newSession) }
            }
            .padding(.vertical, 6)
            Divider().overlay(p.divider)
            HStack(spacing: 5) {
                Button { onAction(.preferences) } label: {
                    Image(systemName: "gearshape").font(.system(size: 11)).foregroundStyle(p.fg2)
                }.buttonStyle(.plain)
                Text(l10n.tr("Preferences", "偏好设置")).font(.system(size: 10)).foregroundStyle(p.fg3)
                Spacer()
                if remote.isOn {
                    // Visible "remote access is on" indicator (a standing exposure
                    // should never be silent). Tap opens Preferences to turn off.
                    Button { onAction(.preferences) } label: {
                        HStack(spacing: 3) {
                            Image(systemName: "globe").font(.system(size: 9))
                            Text(l10n.tr("Remote on", "远程开启")).font(Theme.Font.footer)
                        }.foregroundStyle(p.fg2)
                    }.buttonStyle(.plain).help(remote.url ?? "")
                    Text("·").font(Theme.Font.footer).foregroundStyle(p.fg3)
                }
                Text("gtmux \(appVersion) · by ccy").font(Theme.Font.footer).foregroundStyle(p.fg3)
            }
            .padding(.horizontal, 12).padding(.vertical, 6)
        }
    }

    private func footerAction(_ symbol: String, _ title: String, _ act: @escaping () -> Void) -> some View {
        Button(action: act) {
            VStack(spacing: 3) {
                Image(systemName: symbol).font(.system(size: 13))
                Text(title).font(.system(size: 10))
            }
            .frame(maxWidth: .infinity)
            .foregroundStyle(Theme.Palette.of(scheme).fg2)
            .contentShape(Rectangle())
        }.buttonStyle(.plain)
    }

    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "dev"
    }

    // MARK: keyboard

    private func move(_ delta: Int) {
        guard !flat.isEmpty else { return }
        selected = max(0, min(flat.count - 1, selected + delta))
    }
    private func jumpSelected() {
        guard selected >= 0, selected < flat.count else { return }
        onJump(flat[selected])
    }
    private func toggleSearch() {
        searchActive.toggle()
        if searchActive { DispatchQueue.main.async { searchFocused = true } }
        else { searchText = ""; rootFocused = true }
    }
    private func onEscape() {
        if searchActive { searchActive = false; searchText = ""; rootFocused = true }
        else { onClose() }
    }
}

// MARK: - Section header

private struct SectionHeader: View {
    let status: Status
    let count: Int
    @ObservedObject var l10n: L10n
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        HStack(spacing: 6) {
            Text(title.uppercased()).font(Theme.Font.section).kerning(0.5)
                .foregroundStyle(status == .waiting ? Theme.Status.waiting : p.fg3)
            Text("\(count)").font(.system(size: 9, weight: .bold)).foregroundStyle(p.fg3)
            Spacer()
        }
        .padding(.horizontal, 12).padding(.top, 7).padding(.bottom, 2)
    }

    private var title: String {
        switch status {
        case .waiting: return l10n.tr("Needs input", "需要输入")
        case .working: return l10n.tr("Working", "运行中")
        case .idle:    return l10n.tr("Completed", "已完成")
        case .running: return l10n.tr("Idle", "空闲")
        }
    }
}

// MARK: - Row

private struct AgentRowView: View {
    let agent: Agent
    let selected: Bool
    @ObservedObject var l10n: L10n
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        HStack(spacing: Theme.Size.gap) {
            AgentAvatar(agent: agent)
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 5) {
                    // line 1: the agent's own session name (bold).
                    Text(agent.primary).font(Theme.Font.session).foregroundStyle(p.fg)
                        .lineLimit(1).truncationMode(.tail).help(agent.primary)
                    if agent.isNative { tag(l10n.tr("native", "native"), p) }
                    if agent.latest { latestPill(p) }
                    Spacer(minLength: 0)
                }
                // line 2: where it lives — tmux session · pane (dim context).
                Text(agent.secondary)
                    .font(Theme.Font.window).foregroundStyle(p.fg3).lineLimit(1).truncationMode(.tail)
            }
            VStack(alignment: .trailing, spacing: 3) {
                Text(agent.relativeTimeLabel).font(Theme.Font.mono).foregroundStyle(p.fg3).monospacedDigit()
                Image(systemName: selected ? "return" : "chevron.right")
                    .font(.system(size: 10, weight: .semibold)).foregroundStyle(p.fg3)
            }
        }
        .padding(.horizontal, 12).padding(.vertical, 6)
        .frame(minHeight: Theme.Size.rowHeight)
        .background(rowBackground(p))
        .contentShape(Rectangle())
    }

    @ViewBuilder private func rowBackground(_ p: Theme.Palette) -> some View {
        ZStack {
            if agent.state == .waiting { p.waitingRowTint }
            if selected {
                p.rowSelected
            }
        }
        .clipShape(RoundedRectangle(cornerRadius: Theme.Size.radiusRow, style: .continuous))
        .padding(.horizontal, 4)
    }

    private func tag(_ text: String, _ p: Theme.Palette) -> some View {
        Text(text).font(.system(size: 8.5, weight: .medium)).foregroundStyle(p.fg3)
            .padding(.horizontal, 4).padding(.vertical, 1)
            .background(RoundedRectangle(cornerRadius: 3).fill(scheme == .dark ? Color.white.opacity(0.08) : Color.black.opacity(0.06)))
    }

    private func latestPill(_ p: Theme.Palette) -> some View {
        Text(l10n.tr("latest", "最近完成")).font(.system(size: 8.5, weight: .semibold))
            .foregroundStyle(Theme.Status.idle)
            .padding(.horizontal, 4).padding(.vertical, 1)
            .background(RoundedRectangle(cornerRadius: 3).fill(Theme.Status.idle.opacity(0.14)))
    }
}

// MARK: - Vibrancy background

private struct VisualEffect: NSViewRepresentable {
    func makeNSView(context: Context) -> NSVisualEffectView {
        let v = NSVisualEffectView()
        v.material = .popover
        v.blendingMode = .behindWindow
        v.state = .active
        return v
    }
    func updateNSView(_ nsView: NSVisualEffectView, context: Context) {}
}
