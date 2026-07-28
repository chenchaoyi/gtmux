import AppKit
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
    var id: String { paneID }

    var isAgent: Bool { tier == "agent" }
    var wp: String { // window.pane, from the loc tail
        guard let i = loc.lastIndex(of: ":") else { return loc }
        return String(loc[loc.index(after: i)...])
    }
    var label: String {
        if isAgent { return agent.isEmpty ? command : agent }
        return command
    }

    enum CodingKeys: String, CodingKey {
        case paneID = "pane_id"
        case loc, session, window, pane, cwd, command, title, active, tier, agent
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
        tier = s(.tier, "plain"); agent = s(.agent)
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
    private var window: NSWindow?
    private let browserStore = PaneBrowserStore()

    func show(l10n: L10n) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 480, height: 620),
                styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
            w.contentViewController = NSHostingController(
                rootView: PaneBrowserView(l10n: l10n, store: browserStore))
            w.isReleasedWhenClosed = false
            w.center()
            window = w
            NotificationCenter.default.addObserver(
                forName: NSWindow.willCloseNotification, object: w, queue: .main) { [weak self] _ in
                self?.browserStore.stop()
            }
        }
        window?.title = l10n.tr("gtmux · All panes", "gtmux · 所有 pane")
        browserStore.start()
        window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}

/// PaneBrowserView — the session → window → pane tree. Agent panes are badged and
/// jump to the radar's context; plain panes offer focus + a watch toggle. Reaching
/// any pane lives HERE, not on the radar (kept clean, agent-first).
struct PaneBrowserView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: PaneBrowserStore
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text(l10n.tr("All tmux panes", "所有 tmux pane"))
                    .font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg)
                Spacer()
                Text(l10n.tr("\(store.panes.count) panes · ▸ agents", "\(store.panes.count) 个 pane · ▸ agent"))
                    .font(.system(size: 10)).foregroundStyle(p.fg3)
            }
            .padding(.horizontal, 14).padding(.top, 12).padding(.bottom, 8)
            Divider().overlay(p.divider)

            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1) {
                    ForEach(sessionGroups, id: \.0) { sess, rows in
                        Text(sess).font(.system(size: 11, weight: .semibold)).foregroundStyle(p.fg2)
                            .padding(.horizontal, 14).padding(.top, 10).padding(.bottom, 2)
                        ForEach(rows) { row in
                            PaneBrowserRow(row: row, watched: store.watched.contains(row.paneID),
                                           l10n: l10n,
                                           onFocus: { GtmuxCLI.spawn(["focus", row.paneID]) },
                                           onToggleWatch: { store.toggleWatch(row.paneID) })
                        }
                    }
                }
                .padding(.vertical, 4)
            }
            Divider().overlay(p.divider)
            Text(l10n.tr("Click a pane to jump. Watch a plain pane to pin it onto the radar.",
                         "点击 pane 跳转。关注普通 pane 可把它钉到雷达上。"))
                .font(.system(size: 10)).foregroundStyle(p.fg3)
                .padding(.horizontal, 14).padding(.vertical, 8)
        }
        .frame(minWidth: 420, minHeight: 400)
        .background { ZStack { VisualEffectWindow(); p.bg }.ignoresSafeArea() }
    }

    // Group by session (first-seen order), preserving the pane order within.
    private var sessionGroups: [(String, [PaneRow])] {
        var order: [String] = []
        var seen = Set<String>()
        var byS: [String: [PaneRow]] = [:]
        for r in store.panes {
            if !seen.contains(r.session) { seen.insert(r.session); order.append(r.session) }
            byS[r.session, default: []].append(r)
        }
        return order.map { ($0, byS[$0] ?? []) }
    }
}

private struct PaneBrowserRow: View {
    let row: PaneRow
    let watched: Bool
    @ObservedObject var l10n: L10n
    var onFocus: () -> Void
    var onToggleWatch: () -> Void
    @Environment(\.colorScheme) private var scheme
    @State private var hovering = false

    var body: some View {
        let p = Theme.Palette.of(scheme)
        HStack(spacing: 8) {
            Text(row.isAgent ? "▸" : " ")
                .font(.system(size: 11, weight: .bold)).foregroundStyle(Theme.Status.working)
                .frame(width: 10)
            Text(row.wp).font(Theme.Font.mono).foregroundStyle(p.fg3).frame(width: 34, alignment: .leading)
            VStack(alignment: .leading, spacing: 1) {
                HStack(spacing: 5) {
                    Text(row.label).font(.system(size: 12, weight: row.isAgent ? .medium : .regular))
                        .foregroundStyle(p.fg).lineLimit(1).truncationMode(.tail)
                    if row.active {
                        Text(l10n.tr("active", "活动"))
                            .font(.system(size: 8)).foregroundStyle(p.fg3)
                    }
                }
                if row.isAgent, !row.title.isEmpty {
                    Text(row.title).font(.system(size: 10)).foregroundStyle(p.fg3).lineLimit(1).truncationMode(.tail)
                }
            }
            Spacer(minLength: 6)
            Text(row.paneID).font(Theme.Font.mono).foregroundStyle(p.fg3)
            // A PLAIN pane can be watched (pinned onto the radar). An agent pane is
            // already on the radar — no watch toggle.
            if !row.isAgent, hovering || watched {
                Button(action: onToggleWatch) {
                    Image(systemName: watched ? "eye.fill" : "eye")
                        .font(.system(size: 10)).foregroundStyle(watched ? Theme.Status.idle : p.fg3)
                        .frame(width: 20, height: 18).contentShape(Rectangle())
                }.buttonStyle(.plain)
                .help(watched ? l10n.tr("Watching — click to stop", "关注中 · 点击取消")
                              : l10n.tr("Watch this pane (pin to radar)", "关注这个 pane（钉到雷达）"))
            }
        }
        .padding(.horizontal, 14).padding(.vertical, 5)
        .frame(minHeight: 26)
        .background(hovering ? p.fg.opacity(0.05) : .clear)
        .contentShape(Rectangle())
        .onTapGesture { onFocus() }
        .onHover { hovering = $0 }
    }
}
