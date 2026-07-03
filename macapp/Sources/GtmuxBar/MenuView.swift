import AppKit
import SwiftUI

enum MenuAction {
    case restore, newSession, preferences, pairPhone, quit
}

/// MenuView is the popover (DESIGN §3): a header (logo + waiting-only + search +
/// summary), the agents grouped Needs-you → Working → Idle → Running, and an
/// actions footer. A custom view (not NSMenu) so it can group, emphasize "needs
/// you", and stay calm elsewhere.
struct MenuView: View {
    @ObservedObject var store: AgentStore
    @ObservedObject var l10n: L10n
    @ObservedObject var remote = RemoteAccess.shared
    @ObservedObject private var updater = Updater.shared
    @ObservedObject private var collapse = SectionCollapse.shared
    var onJump: (Agent) -> Void
    var onAction: (MenuAction) -> Void
    var onSend: (Agent, Int) -> Void = { _, _ in }
    var onClose: () -> Void = {}

    @State private var waitingOnly = false
    @State private var searchActive = false
    @State private var searchText = ""
    @State private var selected = 0
    @State private var expanded: String? // paneID of the waiting row showing 1/2/3
    @FocusState private var rootFocused: Bool
    @FocusState private var searchFocused: Bool
    @Environment(\.colorScheme) private var scheme

    private var query: String { searchActive ? searchText : "" }
    private var sections: [(status: Status, agents: [Agent])] {
        store.sections(waitingOnly: waitingOnly, query: query)
    }
    // Keyboard navigation only walks rows in EXPANDED sections (A4): a collapsed
    // section's agents are hidden, so ↑/↓ skip them.
    private var flat: [Agent] {
        sections.filter { !collapse.isCollapsed($0.status) }.flatMap { $0.agents }
    }

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
        .onKeyPress(.rightArrow) { expandSelectedWaiting() ? .handled : .ignored }
        .onKeyPress(.leftArrow) { if expanded != nil { expanded = nil; return .handled }; return .ignored }
        .onKeyPress(.return) { jumpSelected(); return .handled }
        .onKeyPress(.escape) { onEscape(); return .handled }
        // 1/2/3 reply when a waiting row is expanded (A1) — only digits, others pass through.
        .onKeyPress { press in
            guard let n = Int(press.characters), (1...9).contains(n),
                  let pane = expanded, let a = flat.first(where: { $0.paneID == pane })
            else { return .ignored }
            onSend(a, n); expanded = nil
            return .handled
        }
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
        } else if sections.isEmpty {
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
                            SectionHeader(status: status, count: count,
                                          collapsed: collapse.isCollapsed(status), l10n: l10n) {
                                collapse.toggle(status)
                                selected = 0
                            }
                        case let .agent(agent, idx):
                            VStack(spacing: 0) {
                                AgentRowView(agent: agent, selected: idx == selected,
                                             expanded: expanded == agent.paneID,
                                             canReply: agent.state == .waiting, l10n: l10n,
                                             onReply: { toggleExpand(agent) })
                                    .onHover { if $0 { selected = idx } }
                                    .onTapGesture { onJump(agent) }
                                if agent.state == .waiting, expanded == agent.paneID {
                                    WaitingReplyView(agent: agent, l10n: l10n,
                                                     onSend: { n in onSend(agent, n); expanded = nil },
                                                     onJump: { onJump(agent) })
                                }
                            }
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
            if collapse.isCollapsed(section.status) { continue } // count stays; rows hidden
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
            updateBanner(p)
            HStack(spacing: 0) {
                footerAction("arrow.uturn.backward", l10n.tr("Restore", "接回")) { onAction(.restore) }
                footerAction("plus", l10n.tr("New", "新建")) { onAction(.newSession) }
                footerAction("qrcode", l10n.tr("Pair", "配对")) { onAction(.pairPhone) }
            }
            .padding(.vertical, 6)
            Divider().overlay(p.divider)
            HStack(spacing: 10) {
                // Preferences sits at fg2 (interactive tone); the version meta at
                // fg3. Single tappable icon+label; fixedSize so nothing wraps.
                Button { onAction(.preferences) } label: {
                    HStack(spacing: 4) {
                        Image(systemName: "gearshape").font(.system(size: 12))
                        Text(l10n.tr("Preferences", "偏好设置")).font(.system(size: 11, weight: .medium))
                    }.foregroundStyle(p.fg2)
                }.buttonStyle(.plain).fixedSize()
                Spacer(minLength: 8)
                if remote.isOn {
                    // ONE compact remote indicator (avoids the footer overflowing the
                    // 320pt popover — showing "Remote on" + a separate phone cluster +
                    // two separators clipped the version on the right). A live client
                    // (green phone, the connection-indicator convention) is a stronger
                    // exposure signal than "remote on", so it supersedes the globe
                    // label when present; otherwise show "Remote on" (a standing
                    // exposure should never be silent). Tap opens Preferences.
                    Button { onAction(.preferences) } label: {
                        HStack(spacing: 3) {
                            if remote.remoteClients > 0 {
                                Image(systemName: "iphone.radiowaves.left.and.right").font(.system(size: 9))
                                if remote.remoteClients > 1 {
                                    Text("\(remote.remoteClients)").font(Theme.Font.footer)
                                }
                            } else {
                                Image(systemName: "globe").font(.system(size: 9))
                                Text(l10n.tr("Remote on", "远程开启")).font(Theme.Font.footer)
                            }
                        }
                        .foregroundStyle(remote.remoteClients > 0 ? Theme.Status.idle : p.fg2)
                    }.buttonStyle(.plain)
                        .help(remoteViewersHelp)
                        .fixedSize()
                    Text("·").font(Theme.Font.footer).foregroundStyle(p.fg3)
                }
                // The version doubles as a tap-to-"check for updates" affordance and
                // shows the check's result inline. Lowest layout priority so a longer
                // locale truncates here instead of clipping at the popover edge.
                Button { updater.check() } label: {
                    Text(versionLabel)
                        .font(Theme.Font.footer).foregroundStyle(p.fg3)
                        .lineLimit(1).truncationMode(.tail)
                }.buttonStyle(.plain).layoutPriority(-1)
                    .help(l10n.tr("Check for updates", "检查更新"))
            }
            .padding(.horizontal, 12).padding(.vertical, 6)
        }
    }

    // A prominent, tappable "new version" banner above the footer actions — the
    // one-click path to install an update (same effect as `gtmux update`). Shows a
    // working state while updating; hidden when up to date.
    @ViewBuilder private func updateBanner(_ p: Theme.Palette) -> some View {
        switch updater.state {
        case .available(let v):
            Button { updater.install() } label: {
                HStack(spacing: 7) {
                    Image(systemName: "arrow.down.circle.fill").font(.system(size: 13))
                    Text(l10n.tr("New version \(v) — click to update", "新版本 \(v) · 点此更新"))
                        .font(.system(size: 11.5, weight: .semibold))
                    Spacer(minLength: 4)
                    Image(systemName: "chevron.right").font(.system(size: 10, weight: .semibold))
                }
                .foregroundStyle(.white)
                .padding(.horizontal, 12).padding(.vertical, 7)
                .frame(maxWidth: .infinity)
                .background(Theme.Status.working)
                .contentShape(Rectangle())
            }.buttonStyle(.plain)
                .help(l10n.tr("Update gtmux (CLI + app) to \(v)", "把 gtmux（CLI + app）更新到 \(v)"))
        case .updating:
            HStack(spacing: 7) {
                ProgressView().controlSize(.small)
                Text(l10n.tr("Updating gtmux… it will relaunch when done",
                             "正在更新 gtmux…完成后会自动重启"))
                    .font(.system(size: 11)).foregroundStyle(p.fg2)
            }
            .padding(.horizontal, 12).padding(.vertical, 7)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.Status.working.opacity(0.12))
        default:
            EmptyView()
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

    // The footer version line, suffixed with the check's transient result. The
    // `.available` case is intentionally NOT shown here — the prominent banner above
    // owns that state — so this stays "gtmux X · by ccy" until the user checks.
    private var versionLabel: String {
        switch updater.state {
        case .checking: return l10n.tr("gtmux \(appVersion) · checking…", "gtmux \(appVersion) · 检查中…")
        case .upToDate: return l10n.tr("gtmux \(appVersion) · up to date", "gtmux \(appVersion) · 已是最新")
        case .failed: return l10n.tr("gtmux \(appVersion) · check failed", "gtmux \(appVersion) · 检查失败")
        default: return "gtmux \(appVersion) · by ccy"
        }
    }

    // Tooltip for the live-viewer indicator: enumerate WHO is connected (phone
    // names / browser platforms), falling back to the tunnel URL when nobody is.
    private var remoteViewersHelp: String {
        let list = remote.remoteClientList
        if list.isEmpty {
            return remote.remoteClients > 0
                ? l10n.tr("A device is viewing this Mac right now", "有设备正在查看本机")
                : (remote.url ?? "")
        }
        let header = l10n.tr("Viewing this Mac right now:", "正在查看本机：")
        let rows = list.map { c -> String in
            let icon = c.isPhone ? "📱" : "🌐"
            return "\(icon) \(c.title(l10n.tr))"
        }
        return ([header] + rows).joined(separator: "\n")
    }

    // MARK: keyboard

    private func move(_ delta: Int) {
        guard !flat.isEmpty else { return }
        expanded = nil // arrowing away closes any open reply
        selected = max(0, min(flat.count - 1, selected + delta))
    }

    /// Expand the selected row's in-place 1/2/3 reply — only for waiting (A1 /
    /// cardinal rule: working never offers a reply). Returns false otherwise.
    private func expandSelectedWaiting() -> Bool {
        guard selected >= 0, selected < flat.count, flat[selected].state == .waiting else { return false }
        expanded = flat[selected].paneID
        return true
    }

    private func toggleExpand(_ agent: Agent) {
        expanded = (expanded == agent.paneID) ? nil : agent.paneID
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
    let collapsed: Bool
    @ObservedObject var l10n: L10n
    var onToggle: () -> Void
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        // Whole header is a fold toggle (A4): count stays visible when collapsed;
        // a Hide/Show label + a chevron that rotates ▶ (folded) ↔ ▼ (open).
        Button(action: onToggle) {
            HStack(spacing: 6) {
                Text(title.uppercased()).font(Theme.Font.section).kerning(0.5)
                    .foregroundStyle(status == .waiting ? Theme.Status.waiting : p.fg3)
                Text("\(count)").font(.system(size: 9, weight: .bold)).foregroundStyle(p.fg3)
                Spacer()
                Text(collapsed ? l10n.tr("Show", "展开") : l10n.tr("Hide", "收起"))
                    .font(Theme.Font.footer).foregroundStyle(p.fg3)
                Image(systemName: "chevron.right")
                    .font(.system(size: 8, weight: .semibold)).foregroundStyle(p.fg3)
                    .rotationEffect(.degrees(collapsed ? 0 : 90))
                    .animation(.easeInOut(duration: 0.15), value: collapsed)
            }
            .padding(.horizontal, 12).padding(.top, 7).padding(.bottom, 2)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
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

/// SectionCollapse remembers which popover sections the user folded (A4), so the
/// choice survives reopen. The count stays visible when a section is collapsed.
final class SectionCollapse: ObservableObject {
    static let shared = SectionCollapse()
    @Published private var collapsed: Set<String>
    private let key = "popover.collapsedSections"

    private init() {
        let raw = UserDefaults.standard.string(forKey: key) ?? ""
        collapsed = Set(raw.split(separator: ",").map(String.init))
    }

    func isCollapsed(_ s: Status) -> Bool { collapsed.contains(s.rawValue) }

    func toggle(_ s: Status) {
        if collapsed.contains(s.rawValue) { collapsed.remove(s.rawValue) } else { collapsed.insert(s.rawValue) }
        UserDefaults.standard.set(collapsed.sorted().joined(separator: ","), forKey: key)
    }
}

// MARK: - Row

private struct AgentRowView: View {
    let agent: Agent
    let selected: Bool
    var expanded = false
    var canReply = false
    @ObservedObject var l10n: L10n
    var onReply: () -> Void = {}
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
                // Waiting rows offer an in-place "Reply" affordance (A1); others
                // just show the jump chevron / ⏎.
                if canReply, selected {
                    Button(action: onReply) {
                        HStack(spacing: 3) {
                            Image(systemName: "arrowshape.turn.up.left.fill").font(.system(size: 8))
                            Image(systemName: expanded ? "chevron.down" : "chevron.right").font(.system(size: 8, weight: .semibold))
                        }
                        .foregroundStyle(Theme.Status.waiting)
                        .padding(.horizontal, 5).padding(.vertical, 2)
                        .background(RoundedRectangle(cornerRadius: 4).fill(Theme.Status.waiting.opacity(0.14)))
                        .contentShape(Rectangle())
                    }.buttonStyle(.plain).help(l10n.tr("Reply here", "就地回应"))
                } else {
                    Image(systemName: selected ? "return" : "chevron.right")
                        .font(.system(size: 10, weight: .semibold)).foregroundStyle(p.fg3)
                }
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

// MARK: - In-place reply (A1)

struct ReplyOption: Identifiable, Decodable {
    let n: Int
    let label: String
    var id: Int { n }
}

/// WaitingReplyView shows a waiting agent's own 1/2/3 choices as full-row buttons
/// (A1/A3 / mockup §09). Options come from the shared parser via `gtmux options
/// <pane>`; tapping one (or pressing its digit) sends it. Shared by the popover
/// (A1) and the command palette (A3). Falls back to "jump to reply" when nothing
/// parses.
struct WaitingReplyView: View {
    let agent: Agent
    @ObservedObject var l10n: L10n
    var onSend: (Int) -> Void
    var onJump: () -> Void
    @State private var options: [ReplyOption]? // nil = still loading
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 4) {
            if let opts = options, !opts.isEmpty {
                ForEach(opts) { o in
                    Button { onSend(o.n) } label: { optionLabel(o, p) }.buttonStyle(.plain)
                }
                Text(l10n.tr("1/2/3 send · ⏎ jump · ← close", "1/2/3 发送 · ⏎ 跳转 · ← 收起"))
                    .font(.system(size: 9)).foregroundStyle(p.fg3).padding(.top, 1)
            } else if options == nil {
                HStack(spacing: 6) {
                    ProgressView().controlSize(.small)
                    Text(l10n.tr("Reading options…", "正在读取选项…")).font(.system(size: 11)).foregroundStyle(p.fg3)
                }.padding(.vertical, 3)
            } else {
                Button(action: onJump) {
                    Text(l10n.tr("Jump to reply →", "去终端回应 →"))
                        .font(.system(size: 12, weight: .medium)).foregroundStyle(Theme.Status.waiting)
                }.buttonStyle(.plain).padding(.vertical, 3)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.leading, 53).padding(.trailing, 16).padding(.bottom, 7)
        .task(id: agent.paneID) { await load() }
    }

    private func optionLabel(_ o: ReplyOption, _ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            Text("\(o.n)").font(.system(size: 12, weight: .bold)).foregroundStyle(Theme.Status.waiting)
                .frame(width: 18, height: 18)
                .background(RoundedRectangle(cornerRadius: 4).fill(Theme.Status.waiting.opacity(0.16)))
            Text(o.label).font(.system(size: 12)).foregroundStyle(p.fg)
                .lineLimit(2).multilineTextAlignment(.leading).fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 8).padding(.vertical, 6)
        .background(RoundedRectangle(cornerRadius: 6)
            .fill(scheme == .dark ? Color.white.opacity(0.06) : Color.black.opacity(0.04)))
        .contentShape(Rectangle())
    }

    private func load() async {
        let pane = agent.paneID
        let data = await Task.detached { GtmuxCLI.capture(["options", pane]) ?? Data("[]".utf8) }.value
        options = (try? JSONDecoder().decode([ReplyOption].self, from: data)) ?? []
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
