import AppKit
import Combine
import SwiftUI

// PaneRow mirrors `gtmux panes --json` — EVERY tmux pane, tiered agent/plain
// (tiered-pane-control). The browser reaches ANY pane; agents get badged and link
// back to the radar, plain panes get focus/type/watch.
struct PaneRow: Codable, Equatable, Identifiable {
    var paneID = ""
    var loc = ""
    var session = ""
    var window = ""
    var pane = ""
    var cwd = ""
    var command = ""
    var title = ""
    var active = false
    var inMode = false
    var tier = "plain"
    var agent = ""
    var icon = "" // official-icon hint for an agent pane (.app/image path); "" = monogram
    // The window's STABLE id and its (drifting) name — tmux-id-surface. The window INDEX
    // cannot identify a window: measured on this fleet, most sessions' windows all sit at
    // index 0. `@N` is unique per server; the name is a gloss.
    var winID = ""
    var winName = ""
    /// The repo root's basename, on every tier ("" outside a repo). It is how a shell is
    /// referred to out loud, and it is stable across the repo's subdirectories.
    var project = ""
    var id: String { paneID }

    var isAgent: Bool { tier == "agent" }
    var wp: String { // window.pane, from the loc tail
        guard let i = loc.lastIndex(of: ":") else { return loc }
        return String(loc[loc.index(after: i)...])
    }
    /// Just the pane index — for rows sitting under a window line that already names the
    /// window.
    var paneIndexOnly: String {
        guard let i = wp.lastIndex(of: ".") else { return wp }
        return String(wp[wp.index(after: i)...])
    }
    // NO `label` here on purpose: what a row is called depends on the live radar join
    // (an agent's name can arrive empty and its command be a version string), so the rule
    // lives in PaneLabels, next to the phone's identical one. Two label rules in two
    // places is how the surfaces drifted in the first place.

    enum CodingKeys: String, CodingKey {
        case paneID = "pane_id"
        case loc, session, window, pane, cwd, command, title, active, tier, agent, icon, project
        case winID = "win_id"
        case winName = "win_name"
        case inMode = "in_mode"
    }

    // Custom decoder: `gtmux panes --json` OMITS empty fields (title/agent/cwd/in_mode
    // via omitempty). The synthesized Codable init calls `decode` (not decodeIfPresent)
    // for the non-optional properties, so a plain shell pane with no title/agent made
    // the WHOLE array fail to decode → an empty "0 panes" browser. decodeIfPresent
    // tolerates the omitted keys (defaults stand).
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        func s(_ k: CodingKeys, _ d: String = "") -> String { (try? c.decode(String.self, forKey: k)) ?? d }
        func b(_ k: CodingKeys) -> Bool { (try? c.decode(Bool.self, forKey: k)) ?? false }
        paneID = s(.paneID); loc = s(.loc); session = s(.session); window = s(.window); pane = s(.pane)
        cwd = s(.cwd); command = s(.command); title = s(.title)
        active = b(.active); inMode = b(.inMode)
        tier = s(.tier, "plain"); agent = s(.agent); icon = s(.icon)
        winID = s(.winID); winName = s(.winName); project = s(.project)
    }
}

/// PaneCollapse remembers which SESSIONS the user folded in the browser, so the choice
/// survives closing the window. Same idea as the popover's SectionCollapse, keyed by
/// session name rather than status.
final class PaneCollapse: ObservableObject {
    static let shared = PaneCollapse()
    @Published private var collapsed: Set<String>
    private let key = "panes.collapsedSessions"

    private init() {
        collapsed = Self.decode(UserDefaults.standard.string(forKey: key) ?? "")
    }

    /// The stored form, kept as a pair of pure functions so the round trip is testable
    /// without restarting the process — the separator is the whole risk here and an
    /// in-memory set never exercises it.
    ///
    /// A session name cannot contain a NEWLINE, so that is the safe separator. A comma is
    /// not: `tmux new -s a,b` is legal, and a comma-separated store would read it back as
    /// two sessions and fold a session nobody asked to fold.
    static func encode(_ set: Set<String>) -> String { set.sorted().joined(separator: "\n") }
    static func decode(_ raw: String) -> Set<String> {
        Set(raw.split(separator: "\n").map(String.init))
    }

    func isCollapsed(_ session: String) -> Bool { collapsed.contains(session) }

    func toggle(_ session: String) {
        if collapsed.contains(session) { collapsed.remove(session) } else { collapsed.insert(session) }
        persist()
    }

    /// setAll folds or unfolds everything currently listed. Unfolding CLEARS the whole
    /// set rather than removing just these: a session that is gone should not keep a
    /// folded state waiting for it to come back under the same name.
    func setAll(_ sessions: [String], collapsed want: Bool) {
        collapsed = want ? Set(sessions) : []
        persist()
    }

    private func persist() {
        UserDefaults.standard.set(Self.encode(collapsed), forKey: key)
    }
}

/// PaneLabels mirrors the phone's labelling rules (PaneBrowserScreen.plainLabel /
/// agentLabel). Both encode a real defect, so the two surfaces must agree.
/// The commands a pane id can be turned into. One place, so the three surfaces that
/// offer "copy this row's command" cannot drift apart — and so a test can assert the
/// exact string a user will paste, rather than a rebuilt lookalike.
enum PaneCommands {
    static func focus(paneID: String) -> String { "gtmux focus " + paneID }
}

enum PaneLabels {
    /// A PLAIN pane — a shell, an editor, a log tail — named for a reader choosing
    /// between rows.
    ///
    /// It used to end at the command, so a machine's shells all read "bash". That is true
    /// and it identifies nothing. The order below is "what does this say that the previous
    /// did not", and it is the same rule the phone's neighbour strip uses
    /// (mobileapp/src/api/types.ts paneLabel) so a pane is called the same thing on both.
    ///
    /// Many shells set the pane title to the cwd, sometimes prefixed with a colon
    /// (":/Users/…"). A whole path is not a name, so it is skipped and the rules below
    /// produce its meaningful part anyway.
    static func plain(title: String, winName: String, project: String, cwd: String, command: String) -> String {
        var t = title
        while t.hasPrefix(":") { t.removeFirst() }
        t = t.trimmingCharacters(in: .whitespaces)
        if !t.isEmpty, !t.hasPrefix("/"), !t.hasPrefix("~"), t != command { return t }

        // tmux's automatic-rename writes the command into the window name, which would
        // land us back on "bash".
        let w = winName.trimmingCharacters(in: .whitespaces)
        if !w.isEmpty, w != command { return w }

        // The repo it sits in: stable across its subdirectories, and how people refer to
        // a shell out loud ("the gtmux one").
        let p = project.trimmingCharacters(in: .whitespaces)
        if !p.isEmpty { return p }

        var dir = cwd.trimmingCharacters(in: .whitespaces)
        while dir.count > 1, dir.hasSuffix("/") { dir.removeLast() }
        if let leaf = dir.split(separator: "/").last, !leaf.isEmpty { return String(leaf) }

        return command
    }

    /// An AGENT pane. The raw command is NEVER a good first choice here: a Claude 2.x
    /// pane's `pane_current_command` is its VERSION string ("2.1.220" — the #659 fact),
    /// so a row whose `agent` field arrives empty showed a bare version number. Prefer
    /// the live radar join's name; the command stays only as the last resort.
    static func agent(row: PaneRow, joined: Agent?) -> String {
        if !row.agent.isEmpty { return row.agent }
        if let j = joined, !j.agent.isEmpty { return j.agent }
        return row.command
    }
}

/// PaneBrowserStore polls `gtmux panes --json` while the browser window is open,
/// and tracks the watched set (from `agents --json` watched rows via AgentStore) so
/// the browser can show a live watch toggle. Separate from AgentStore so the radar's
/// hot poll is untouched — this only runs while the window is visible.
final class PaneBrowserStore: ObservableObject {
    @Published private(set) var panes: [PaneRow] = []
    @Published private(set) var watched: Set<String> = []
    private var timer: Timer?

    func start() {
        refresh()
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: 1.5, repeats: true) { [weak self] _ in self?.refresh() }
    }

    func stop() { timer?.invalidate(); timer = nil }

    func refresh() {
        DispatchQueue.global(qos: .userInitiated).async {
            var rows: [PaneRow] = []
            if let d = GtmuxCLI.capture(["panes", "--json"]) {
                rows = (try? JSONDecoder().decode([PaneRow].self, from: d)) ?? []
            }
            var w = Set<String>()
            if let d = GtmuxCLI.capture(["panes", "--watched"]),
               let s = String(data: d, encoding: .utf8) {
                for line in s.split(separator: "\n") { w.insert(String(line).trimmingCharacters(in: .whitespaces)) }
            }
            DispatchQueue.main.async { self.panes = rows; self.watched = w }
        }
    }

    func toggleWatch(_ pane: String) {
        let verb = watched.contains(pane) ? "unwatch" : "watch"
        GtmuxCLI.spawn(["panes", verb, pane])
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { self.refresh() }
    }
}

/// PaneBrowserController hosts the sessions/panes browser in its OWN window — a
/// SEPARATE surface from the radar popover (tiered-pane-control's anti-dilution rule:
/// general pane management never merges into the agent radar). Modeled on
/// PreferencesController.
final class PaneBrowserController {
    static let shared = PaneBrowserController()
    /// Kept across closes so the browser reopens where the user left it — but its CONTENT
    /// is dropped on close, see the willClose handler. `private(set)` so a test can assert
    /// exactly that: window kept, graph gone.
    private(set) var window: NSWindow?
    private let browserStore = PaneBrowserStore()
    /// What the content wants to be — published by the view, consumed by fitWindow.
    private let contentSize = PanelSize()
    private var cancellables = Set<AnyCancellable>()
    /// Set once the user drags the window's edge. From then on the size is theirs and
    /// this stops touching it — a window is not a popover; the user is in charge of it.
    private var userSized = false
    /// How long after an opening the window still fits itself to its content.
    ///
    /// It cannot be a single shot: the pane list is fetched asynchronously, so at the
    /// moment the window appears the content measures a few points and a one-shot fit
    /// would lock that in (measured: `wanted=8`). It cannot be forever either — panes come
    /// and go on a 1.5s poll, and a window that resizes under a reader is worse than one
    /// that is the wrong size. So: fit while the window is still opening, then stop.
    private var fitUntil: Date?
    private static let fitWindow: TimeInterval = 2.5

    func show(l10n: L10n, radar: AgentStore) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: PanelMetrics.browserWidth, height: 620),
                styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
            w.contentViewController = NSHostingController(
                rootView: PaneBrowserView(l10n: l10n, store: browserStore, size: contentSize,
                                          radar: radar))
            w.isReleasedWhenClosed = false
            w.center()
            window = w
            NotificationCenter.default.addObserver(
                forName: NSWindow.willCloseNotification, object: w, queue: .main) { [weak self] _ in
                self?.browserStore.stop()
                MenuVisibility.shared.setVisible(false)
            }
            NotificationCenter.default.addObserver(
                forName: NSWindow.didEndLiveResizeNotification, object: w, queue: .main) { [weak self] _ in
                self?.userSized = true
            }
            contentSize.$height
                .receive(on: RunLoop.main)
                .sink { [weak self] h in self?.fitWindow(to: h) }
                .store(in: &cancellables)
        }
        window?.title = l10n.tr("gtmux · All panes", "gtmux · 所有 pane")
        fitUntil = Date().addingTimeInterval(Self.fitWindow)
        browserStore.start()
        MenuVisibility.shared.setVisible(true)
        window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    /// fitWindow opens the browser at the height its sessions actually need, instead of a
    /// fixed 620 that showed a handful of rows on a machine with dozens of panes.
    ///
    /// It keeps the window's TOP-LEFT corner and grows downward, because a window that
    /// re-centres itself as it resizes appears to jump. If growing down would run past the
    /// bottom of the usable area, it slides up to stay whole.
    private func fitWindow(to wanted: CGFloat) {
        guard !userSized, wanted > 1, let w = window,
              let until = fitUntil, Date() < until else { return }
        let screen = w.screen ?? NSScreen.main
        let visible = screen?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1440, height: 900)
        let titleBar = w.frame.height - w.contentLayoutRect.height
        let height = PanelMetrics.windowContentHeight(desired: wanted,
                                                      visible: visible.height,
                                                      titleBar: titleBar)
        var f = w.frame
        let top = f.maxY
        f.size.height = height + titleBar
        f.origin.y = top - f.size.height
        if f.minY < visible.minY { f.origin.y = visible.minY }
        if f.maxY > visible.maxY { f.origin.y = visible.maxY - f.size.height }
        w.setFrame(f, display: true)
        dbg(String(format: "panes: wanted=%.0f → content=%.0f window=%.0fx%.0f@y%.0f (visible=%.0f@y%.0f)",
                   wanted, height, f.width, f.height, f.minY, visible.height, visible.minY))
    }
}

/// PaneBrowserView — the session → window → pane tree. Agent panes are badged and
/// jump to the radar's context; plain panes offer focus + a watch toggle. Reaching
/// any pane lives HERE, not on the radar (kept clean, agent-first).
struct PaneBrowserView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: PaneBrowserStore
    /// What the window would have to be to show every session without scrolling. The
    /// controller opens the window at it; see PaneBrowserController.fitWindow.
    @ObservedObject var size = PanelSize()
    /// The live radar, joined by pane id so an agent row can show its REAL status — the
    /// phone's browser has always done this; the Mac's showed identity only.
    @ObservedObject var radar: AgentStore
    @ObservedObject private var collapse = PaneCollapse.shared
    @Environment(\.colorScheme) private var scheme
    @State private var query = ""
    @State private var chromeHeight: CGFloat = 0
    @State private var listHeight: CGFloat = 0

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 0) {
                header(p)
                searchField(p)
                Divider().overlay(p.divider)
            }
            .background(GeometryReader { g in
                Color.clear.preference(key: ChromeHeight.self, value: g.size.height)
            })

            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1) {
                    if groups.isEmpty {
                        Text(store.panes.isEmpty
                             ? l10n.tr("No tmux panes", "没有 tmux pane")
                             : l10n.tr("No panes match", "没有匹配的 pane"))
                            .font(.system(size: 12)).foregroundStyle(p.fg2)
                            .frame(maxWidth: .infinity).padding(.vertical, 30)
                    }
                    ForEach(groups) { g in
                        SessionHeader(group: g, collapsed: collapse.isCollapsed(g.session),
                                      l10n: l10n) { collapse.toggle(g.session) }
                        if !collapse.isCollapsed(g.session) {
                            ForEach(g.windows) { win in
                                // The window line appears only when there is more than one
                                // window to tell apart (see showsWindowRows).
                                if g.showsWindowRows {
                                    WindowHeader(winID: win.winID, winName: win.winName, count: win.rows.count)
                                }
                                ForEach(win.rows) { row in
                                    PaneBrowserRow(row: row, watched: store.watched.contains(row.paneID),
                                                   joined: byPane[row.paneID],
                                                   indented: g.showsWindowRows,
                                                   l10n: l10n,
                                                   onFocus: { GtmuxCLI.spawn(["focus", row.paneID]) },
                                                   onToggleWatch: { store.toggleWatch(row.paneID) })
                                }
                            }
                        }
                    }
                }
                .padding(.vertical, 4)
                // What the list WANTS. A ScrollView has no intrinsic content height, so
                // without this the window has nothing to size itself from.
                .background(GeometryReader { g in
                    Color.clear.preference(key: ListContentHeight.self, value: g.size.height)
                })
            }
            VStack(alignment: .leading, spacing: 0) {
                Divider().overlay(p.divider)
                Text(l10n.tr("Click a pane to jump. Watch a plain pane to pin it onto the radar.",
                             "点击 pane 跳转。关注普通 pane 可把它钉到雷达上。"))
                    .font(.system(size: 10)).foregroundStyle(p.fg2)
                    .padding(.horizontal, 14).padding(.vertical, 8)
            }
            .background(GeometryReader { g in
                Color.clear.preference(key: ChromeHeight.self, value: g.size.height)
            })
        }
        // The MINIMUM is the design width, because that is what an NSHostingController
        // sizes its window to. The window was created 480 wide and measured 420 — the
        // old minimum — and stating idealWidth did not change that (measured). Below 480
        // these rows truncate anyway, so the design width is the right floor.
        .frame(minWidth: PanelMetrics.browserWidth, minHeight: 400)
        .background { ZStack { VisualEffectWindow(); p.bg }.ignoresSafeArea() }
        // The root here FILLS the window, so measuring it would only report the window
        // back to itself. The wanted height is chrome + what the list wants.
        .onPreferenceChange(ChromeHeight.self) { h in
            let r = h.rounded()
            if abs(r - chromeHeight) >= 1 { chromeHeight = r; publishWanted() }
        }
        .onPreferenceChange(ListContentHeight.self) { h in
            let r = h.rounded()
            if abs(r - listHeight) >= 1 { listHeight = r; publishWanted() }
        }
    }

    // MARK: chrome

    @ViewBuilder private func header(_ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            VStack(alignment: .leading, spacing: 2) {
                Text(l10n.tr("All tmux panes", "所有 tmux pane"))
                    .font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg)
                // Panes · sessions · who needs you — the same count line the phone
                // carries. "Needs you" is the only part in the waiting color: it is the
                // one number that is a call to act.
                HStack(spacing: 0) {
                    Text(countLine).font(.system(size: 10)).foregroundStyle(p.fg2)
                    if needsYou > 0 {
                        Text(l10n.tr(" · \(needsYou) need you", " · \(needsYou) 个等你"))
                            .font(.system(size: 10)).foregroundStyle(Theme.Status.waiting)
                    }
                }
            }
            Spacer()
            // Fold everything at once. Only worth showing when there is more than one
            // session to fold.
            if groups.count > 1 {
                Button {
                    collapse.setAll(groups.map(\.session), collapsed: !allCollapsed)
                } label: {
                    Image(systemName: allCollapsed ? "rectangle.expand.vertical" : "rectangle.compress.vertical")
                        .font(.system(size: 12)).foregroundStyle(p.fg2)
                        .frame(width: 22, height: 22).contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .help(allCollapsed ? l10n.tr("Expand all", "展开全部") : l10n.tr("Collapse all", "折叠全部"))
            }
        }
        .padding(.horizontal, 14).padding(.top, 12).padding(.bottom, 8)
    }

    @ViewBuilder private func searchField(_ p: Theme.Palette) -> some View {
        HStack(spacing: 6) {
            // An SF Symbol, not the "⌕" CHARACTER. A text glyph's ink has nothing to do
            // with its point size: ⌕ at 13pt read smaller than the 12pt placeholder
            // beside it. See DESIGN §16.
            Image(systemName: "magnifyingglass")
                .font(.system(size: 13, weight: .medium)).foregroundStyle(p.fg3)
            TextField(l10n.tr("Search %pane · @window · session · command · dir",
                              "搜索 %pane · @窗口 · 会话 / 命令 / 目录"), text: $query)
                .textFieldStyle(.plain).font(.system(size: 12)).foregroundStyle(p.fg)
            if !query.isEmpty {
                Button { query = "" } label: {
                    Image(systemName: "xmark.circle.fill").font(.system(size: 12)).foregroundStyle(p.fg3)
                }.buttonStyle(.plain)
            }
        }
        .padding(.horizontal, 8).padding(.vertical, 5)
        .background(RoundedRectangle(cornerRadius: Theme.Size.radiusChip, style: .continuous)
            .fill(p.fg.opacity(0.06)))
        .padding(.horizontal, 14).padding(.bottom, 8)
    }

    private func publishWanted() {
        let want = (chromeHeight + listHeight).rounded()
        if abs(want - size.height) >= 1 { size.height = want }
    }

    // MARK: model

    /// The live radar row behind each pane id, so an agent-tier row can show its REAL
    /// status instead of only its identity. The radar is already polling for the popover.
    private var byPane: [String: Agent] {
        Dictionary(radar.agents.map { ($0.paneID, $0) }, uniquingKeysWith: { a, _ in a })
    }

    /// Sessions in first-seen order (stable, so the layout is learnable), filtered by the
    /// query, each carrying the rollup its header shows. Independent of the fold state —
    /// a folded session still counts, which is the point of the rollup.
    private var groups: [PaneGroup] {
        let needle = query.trimmingCharacters(in: .whitespaces).lowercased()
        let join = byPane
        var order: [String] = []
        var byS: [String: [PaneRow]] = [:]
        for r in store.panes {
            if !needle.isEmpty {
                // The IDs are searchable, and they are the reason this box is worth
                // opening now: `%23` is the token the tab title, `gtmux focus %23` and HQ
                // all use, so it is what someone arrives here holding. It was not in the
                // haystack at all — typing the row's own leading label found nothing.
                // `@4` likewise, so a tab's window id lands on that window's panes.
                let hay = [r.session, r.window, r.command, r.title, r.cwd, r.agent, r.loc,
                           r.paneID, r.winID, r.winName]
                guard hay.contains(where: { $0.lowercased().contains(needle) }) else { continue }
            }
            if byS[r.session] == nil { order.append(r.session) }
            byS[r.session, default: []].append(r)
        }
        return order.map { s in
            let rows = byS[s] ?? []
            var roll: [Status: Int] = [:]
            var agents = 0
            for r in rows where r.isAgent {
                agents += 1
                if let st = join[r.paneID]?.state { roll[st, default: 0] += 1 }
            }
            // Windows in first-seen order too, keyed by the STABLE `@id` — not the index,
            // which is `0` for most windows on a real fleet and would merge them.
            var wOrder: [String] = []
            var byW: [String: [PaneRow]] = [:]
            for r in rows {
                let key = r.winID.isEmpty ? "idx:" + r.window : r.winID
                if byW[key] == nil { wOrder.append(key) }
                byW[key, default: []].append(r)
            }
            let windows = wOrder.map { k -> PaneWindow in
                let rs = byW[k] ?? []
                return PaneWindow(winID: rs.first?.winID ?? "", winName: rs.first?.winName ?? "", rows: rs)
            }
            return PaneGroup(session: s, windows: windows, agentCount: agents, roll: roll)
        }
    }

    private var countLine: String {
        let total = store.panes.count
        let shown = groups.reduce(0) { $0 + $1.rows.count }
        let n = query.isEmpty ? "\(total)" : "\(shown)/\(total)"
        return l10n.tr("\(n) panes · \(groups.count) sessions", "\(n) 个 pane · \(groups.count) 个会话")
    }

    private var needsYou: Int { groups.reduce(0) { $0 + ($1.roll[.waiting] ?? 0) } }
    private var allCollapsed: Bool {
        !groups.isEmpty && groups.allSatisfy { collapse.isCollapsed($0.session) }
    }
}

/// PaneWindow is one tmux window inside a session: its STABLE `@id`, its name (a gloss),
/// and its panes.
struct PaneWindow: Identifiable {
    let winID: String
    let winName: String
    let rows: [PaneRow]
    var id: String { winID.isEmpty ? (winName + "/" + (rows.first?.paneID ?? "")) : winID }
    /// What the window line says: `@7 multipilot`. The id leads because it is the anchor —
    /// the name can drift or be shared by two windows.
    var label: String {
        let n = winName.isEmpty ? "" : " " + winName
        return winID.isEmpty ? winName : winID + n
    }
}

/// PaneGroup is one session's worth of the browser: its windows (each with its panes),
/// plus the rollup its header shows even when it is folded.
struct PaneGroup: Identifiable {
    let session: String
    let windows: [PaneWindow]
    let agentCount: Int
    let roll: [Status: Int]
    var id: String { session }
    /// Every pane in the session, window order preserved — the rollup counts over this.
    var rows: [PaneRow] { windows.flatMap(\.rows) }
    /// EVERY session draws its window rows, including a session that holds exactly one.
    ///
    /// The band used to appear only when there were several windows to tell apart, on the
    /// grounds that one window costs a line saying nothing new. That reasoning was about
    /// the LINE and missed the TREE: with it, a single-window session put its panes
    /// directly under the session header at the same indent that elsewhere means "window",
    /// so the same shape meant two different things one row apart. A reader had to work out
    /// which kind of session they were looking at before they could read the indent. One
    /// predictable three-level tree is worth the line.
    var showsWindowRows: Bool { !windows.isEmpty }
    /// EVERY window id in this session, for the header to carry — `@4 @5 @6`.
    ///
    /// It used to be the lone window's id only, which meant a multi-window session showed
    /// NOTHING when collapsed. Collapsed is exactly when the ids are needed: you are
    /// scanning the list to find which session owns the `@18` your tab is showing, and the
    /// sessions that own several were the ones staying silent.
    ///
    /// Listed whether folded or not, so the header does not change content as you fold it.
    /// Capped, because a session with a dozen windows would push its own name off the row.
    var windowIDs: String {
        let ids = windows.map(\.winID).filter { !$0.isEmpty }
        if ids.isEmpty { return "" }
        if ids.count <= 6 { return ids.joined(separator: " ") }
        return ids.prefix(5).joined(separator: " ") + " +\(ids.count - 5)"
    }
}

// CONTRAST RULE for this window (2026-08-14): `fg3` is 0.34 alpha — calibrated in
// DESIGN §9 for DECORATION, and it was being used here for content. In a dense list of 26
// small rows that reads as illegible grey. So: glyphs, chevrons and controls may sit at
// fg3; anything a reader has to READ — ids, window labels, tallies, hints, empty states —
// is fg2 or better. The palette is unchanged; only its misuse was.

/// WindowHeader — one tmux window inside a session: `@id name` and its pane count.
///
/// Quieter than the session header on purpose: this is the MIDDLE of the tree, and the two
/// ends carry the work (which session, which pane). It exists so that a tab reading
/// "HSS AI Workspace — cc dev %9 %31" can be traced to the right group of panes when a
/// session has several windows.
private struct WindowHeader: View {
    let winID: String
    let winName: String
    let count: Int
    @Environment(\.colorScheme) private var scheme
    var body: some View {
        let p = Theme.Palette.of(scheme)
        HStack(spacing: 7) {
            // The ID is mono — it is an identifier, and mono makes the column of them
            // line up. The NAME is UI-font semibold at the primary tone: it is what a
            // person actually reads. Two typefaces because they are two kinds of thing —
            // the anchor and its gloss.
            Text(winID).font(Theme.Font.mono).foregroundStyle(p.fg2)
            if !winName.isEmpty {
                Text(winName).font(.system(size: 11.5, weight: .semibold)).foregroundStyle(p.fg)
                    .lineLimit(1).truncationMode(.middle)
            }
            Spacer(minLength: 4)
            Text("\(count)").font(.system(size: 10)).foregroundStyle(p.fg3)
        }
        .padding(.leading, 22).padding(.trailing, 14).padding(.vertical, 5)
        // A tinted BAND, not another line of text. It was mono grey at the same tone as
        // the ids and sublabels beneath it, so it read as one more row rather than as the
        // thing those rows belong to. A header is recognised by being a different KIND of
        // element, which is cheaper than out-weighting the content it heads — and the
        // content here (what each agent is doing) should stay the heaviest thing on screen.
        .background(p.fg.opacity(scheme == .dark ? 0.06 : 0.045))
        // The gap sits OUTSIDE the tint, so the band separates groups instead of padding
        // itself.
        .padding(.top, 7)
    }
}

/// SessionHeader — the fold control AND the session's panoramic signal.
///
/// It carries the rollup deliberately: a folded session must still be able to say a pane
/// inside it is waiting on you, or folding would hide exactly what the browser exists to
/// surface. Waiting is red, so a blocked session stands out with everything closed.
private struct SessionHeader: View {
    let group: PaneGroup
    let collapsed: Bool
    @ObservedObject var l10n: L10n
    var onToggle: () -> Void
    @Environment(\.colorScheme) private var scheme
    @State private var hovering = false

    var body: some View {
        let p = Theme.Palette.of(scheme)
        HStack(spacing: 6) {
            Image(systemName: collapsed ? "chevron.right" : "chevron.down")
                .font(.system(size: 12, weight: .semibold)).foregroundStyle(p.fg2)
                .frame(width: 10)
            Text(group.session).font(.system(size: 12, weight: .semibold)).foregroundStyle(p.fg)
                .lineLimit(1).truncationMode(.tail)
            // The session names its windows — all of them. With one, this IS the window
            // line (no separate row is drawn); with several it is how a collapsed session
            // still says what it holds.
            if !group.windowIDs.isEmpty {
                Text(group.windowIDs).font(Theme.Font.mono).foregroundStyle(p.fg2)
                    .lineLimit(1).truncationMode(.tail).layoutPriority(-1)
            }
            Spacer(minLength: 6)
            Text(group.agentCount > 0
                 ? l10n.tr("\(group.rows.count) · \(group.agentCount) agents",
                           "\(group.rows.count) · \(group.agentCount) 个 agent")
                 : "\(group.rows.count)")
                .font(.system(size: 10)).foregroundStyle(p.fg2)
            // One pip per non-zero agent state, in the radar's own order.
            ForEach([Status.waiting, .working, .idle], id: \.self) { st in
                if let n = group.roll[st], n > 0 {
                    HStack(spacing: 2) {
                        StatusBadge(status: st, size: 9)
                        Text("\(n)").font(.system(size: 10, weight: .medium)).foregroundStyle(st.color)
                    }
                }
            }
        }
        .padding(.horizontal, 14).padding(.top, 9).padding(.bottom, 4)
        .background(hovering ? p.fg.opacity(0.05) : .clear)
        .contentShape(Rectangle())
        .onTapGesture { onToggle() }
        .onHover { hovering = $0 }
    }
}

private struct PaneBrowserRow: View {
    let row: PaneRow
    let watched: Bool
    /// The live radar row at this pane id, when there is one.
    var joined: Agent?
    /// Indented when a window line sits above it, so the tree's depth is visible.
    var indented = false
    @ObservedObject var l10n: L10n
    var onFocus: () -> Void
    var onToggleWatch: () -> Void
    @Environment(\.colorScheme) private var scheme
    @State private var hovering = false
    @State private var copied = false

    /// Put the row's jump command on the pasteboard. `PaneCommands.focus` is shared with
    /// the test, so the string a user pastes is the one that is asserted.
    private func copyFocusCommand() {
        let pb = NSPasteboard.general
        pb.clearContents()
        pb.setString(PaneCommands.focus(paneID: row.paneID), forType: .string)
        copied = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) { copied = false }
    }

    var body: some View {
        let p = Theme.Palette.of(scheme)
        HStack(spacing: 8) {
            // Identity: an agent pane shows its OFFICIAL icon (like the radar), not a
            // "▸" glyph; a plain pane shows a dim $_ terminal mark (matches phone/web).
            paneIcon(p).frame(width: 20, height: 20)
            // The pane id LEADS. It is the one stable, unique thing on the row — the
            // token a tab title, `gtmux focus %N` and HQ all use — so it sits where the
            // eye starts, not trailing off the right edge.
            //
            // The window.pane INDEX that used to be here is gone: the window is already
            // named above, and the pane index means nothing on its own. A coordinate that
            // changes when panes are reordered was never worth the column.
            //
            // It is also the row's one COPYABLE token: clicking it puts
            // `gtmux focus %N` on the pasteboard, so the thing you can see becomes a
            // thing you can run. The tap is scoped to the id (the row still jumps), and
            // the label confirms in place — a copy with no feedback is indistinguishable
            // from a missed click.
            Text(copied ? l10n.tr("copied", "已复制") : row.paneID)
                .font(Theme.Font.mono)
                .foregroundStyle(copied ? Theme.Status.idle : p.fg2)
                .frame(width: 34, alignment: .leading)
                .contentShape(Rectangle())
                .onTapGesture { copyFocusCommand() }
                .help(l10n.tr("Copy `gtmux focus \(row.paneID)`", "复制 `gtmux focus \(row.paneID)`"))
            VStack(alignment: .leading, spacing: 1) {
                HStack(spacing: 5) {
                    Text(label).font(.system(size: 12, weight: row.isAgent ? .medium : .regular))
                        .foregroundStyle(p.fg).lineLimit(1).truncationMode(.tail)
                    // An agent pane's REAL state, from the radar join. A plain pane shows
                    // nothing — waiting/working/idle are agent concepts.
                    if let st = joined?.state, row.isAgent {
                        StatusBadge(status: st, size: 10)
                    }
                    // tmux's `pane_active`: the pane its WINDOW currently has selected —
                    // where you land if you switch to that window. A position, not a state.
                    //
                    // It used to read "active", which collided with the status language on
                    // the same row: next to a green ✓ meaning IDLE, "active" reads as "this
                    // agent is busy" — the opposite of what the badge says, and the opposite
                    // of what the word means here. The commander had to ask what it was.
                    if row.active {
                        Text(l10n.tr("focus", "焦点"))
                            .font(.system(size: 8, weight: .medium))
                            .foregroundStyle(p.fg2)
                            .padding(.horizontal, 4).padding(.vertical, 1)
                            .background(RoundedRectangle(cornerRadius: 3, style: .continuous)
                                .fill(p.fg.opacity(0.07)))
                            .help(l10n.tr("the pane this window has selected — switching to the window lands here",
                                          "这个窗口当前选中的 pane —— 切到该窗口会落在这里"))
                    }
                }
                if !sublabel.isEmpty {
                    Text(sublabel).font(.system(size: 10)).foregroundStyle(p.fg2).lineLimit(1).truncationMode(.tail)
                }
            }
            Spacer(minLength: 6)
            // A PLAIN pane can be watched (pinned onto the radar). An agent pane is
            // already on the radar — no watch toggle.
            if !row.isAgent, hovering || watched {
                Button(action: onToggleWatch) {
                    Image(systemName: watched ? "eye.fill" : "eye")
                        .font(.system(size: 12)).foregroundStyle(watched ? Theme.Status.idle : p.fg3)
                        .frame(width: 22, height: 22).contentShape(Rectangle())
                }.buttonStyle(.plain)
                .help(watched ? l10n.tr("Watching — click to stop", "关注中 · 点击取消")
                              : l10n.tr("Watch this pane (pin to radar)", "关注这个 pane（钉到雷达）"))
            }
        }
        .padding(.leading, indented ? 26 : 14).padding(.trailing, 14).padding(.vertical, 5)
        .frame(minHeight: 26)
        .background(hovering ? p.fg.opacity(0.05) : .clear)
        .contentShape(Rectangle())
        .onTapGesture { onFocus() }
        .onHover { hovering = $0 }
        .help(rowHelp)
    }

    /// What the row is CALLED.
    ///
    /// An agent row leads with what it is DOING, taken from the radar's already-derived
    /// task — not from the raw `pane_title`. Two reasons, both visible on a real fleet:
    ///
    ///  • The agent NAME is not an identity here. Six panes all read "Claude Code" while
    ///    the thing that tells them apart sat below in grey. The avatar already carries
    ///    the identity; the headline should carry the work.
    ///  • The raw title still wears the agent's status glyph (`✳`, `◐`…), which the radar
    ///    strips and whose alphabet changes without notice (Claude moved braille →
    ///    half-circles in 2.1.228). Reusing the radar's task means never re-deriving —
    ///    and never re-learning — that.
    ///
    /// Falls back to the name only when there is no radar row to join (an agent the radar
    /// has not classified yet).
    private var label: String {
        if row.isAgent {
            let task = joined?.task.trimmingCharacters(in: .whitespaces) ?? ""
            return task.isEmpty ? PaneLabels.agent(row: row, joined: joined) : task
        }
        return PaneLabels.plain(title: row.title, winName: row.winName, project: row.project,
                                cwd: row.cwd, command: row.command)
    }

    /// No second line. It used to name the agent under the work — "Claude Code" beneath
    /// every one of six rows — which the official icon at the head of the row already says,
    /// in less space and without repeating. The name stays reachable in the row's tooltip
    /// for the case the icon cannot be resolved and falls back to a monogram.
    private var sublabel: String { "" }

    /// What the row is, spelled out for the tooltip: the work, then who is doing it.
    private var rowHelp: String {
        guard row.isAgent else { return label }
        let name = PaneLabels.agent(row: row, joined: joined)
        return name.isEmpty || name == label ? label : label + " — " + name
    }

    // The leading identity tile: an agent's real icon (monogram fallback), or a plain
    // pane's dim $_ terminal mark. Reuses the radar's AgentIcons resolver so the browser
    // shows the same official logos.
    @ViewBuilder private func paneIcon(_ p: Theme.Palette) -> some View {
        if row.isAgent, let img = AgentIcons.image(icon: row.icon, name: row.agent) {
            Image(nsImage: img).resizable().interpolation(.high).scaledToFit()
                .clipShape(RoundedRectangle(cornerRadius: 5, style: .continuous))
        } else if row.isAgent {
            RoundedRectangle(cornerRadius: 5, style: .continuous)
                .fill(scheme == .dark ? Color.white.opacity(0.09) : Color.black.opacity(0.05))
                .overlay(Text(agentMonogram(row.agent))
                    .font(.system(size: 9, weight: .semibold, design: .rounded)).foregroundStyle(p.fg2))
        } else {
            // A plain pane wears the SAME neutral tile as an agent with no icon (DESIGN
            // §6's 中性单字标), with a shell prompt where the monogram goes.
            //
            // It used to be bare `$_` text with no tile — the only row type without one —
            // so next to an agent's solid coloured icon it read as a smudge rather than a
            // mark, and the leading column stopped being a column. This is not a new
            // symbol: it is the tile that already existed, applied to the row that was
            // skipped.
            //
            // It looks like a TERMINAL — a dark screen with a light prompt — rather than
            // like a faint grey chip. A grey-on-grey tile still lost next to an agent's
            // vivid icon, and the two rules that box this in leave no colour to borrow:
            // colour is the status channel (DESIGN §1) and brand belongs to the agent
            // (§6). Representation is the way out: this is not a colour CODE, it is a
            // picture of the thing, so it earns presence without claiming a state.
            //
            // Same #1C1C1F the notification badge uses for its tile. The hairline border
            // is what keeps the shape readable in DARK mode, where a dark tile would
            // otherwise dissolve into the sheet behind it.
            RoundedRectangle(cornerRadius: 5, style: .continuous)
                .fill(Color(red: 0x1C / 255, green: 0x1C / 255, blue: 0x1F / 255))
                .overlay(RoundedRectangle(cornerRadius: 5, style: .continuous)
                    .strokeBorder(Color.white.opacity(scheme == .dark ? 0.18 : 0.0), lineWidth: 0.5))
                .overlay(Text("$_")
                    .font(.system(size: 9, weight: .bold, design: .monospaced))
                    .foregroundStyle(Color.white.opacity(0.88)))
        }
    }
}
