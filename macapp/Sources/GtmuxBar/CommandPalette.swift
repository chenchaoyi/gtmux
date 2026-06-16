import AppKit
import Combine
import SwiftUI

/// A borderless floating panel that can become key (for the search field).
final class KeyablePanel: NSPanel {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }
}

/// PaletteModel drives the command palette (DESIGN §4 B): the query + selection
/// over a fuzzy-filtered agent list.
final class PaletteModel: ObservableObject {
    @Published var query = ""
    @Published var selected = 0
    private let store: AgentStore
    private var cancellable: AnyCancellable?
    init(store: AgentStore) {
        self.store = store
        // Re-render the palette when the agent list refreshes while it's open, so
        // status/grouping stay live (and never show a stale row).
        cancellable = store.objectWillChange.sink { [weak self] in self?.objectWillChange.send() }
    }

    var results: [Agent] { store.ordered(waitingOnly: false, query: query) }
    var sections: [(status: Status, agents: [Agent])] { store.sections(waitingOnly: false, query: query) }

    func move(_ delta: Int) {
        let n = results.count
        guard n > 0 else { selected = 0; return }
        selected = (selected + delta + n) % n // wrap, command-palette style
    }
    func reset() { query = ""; selected = 0 }
}

/// CommandPaletteController owns the screen-centered Raycast-style panel
/// (DESIGN §4 B), summoned by the global hotkey. Keyboard-first: ↑↓ select,
/// ⏎ jump, ⌘1–9 direct, ⎋ close.
final class CommandPaletteController {
    static let shared = CommandPaletteController()

    private var panel: KeyablePanel?
    private var model: PaletteModel?
    private var monitor: Any?
    private var jump: ((Agent) -> Void)?

    func toggle(store: AgentStore, l10n: L10n, onJump: @escaping (Agent) -> Void) {
        if panel == nil { build(store: store, l10n: l10n, onJump: onJump) }
        guard let panel = panel else { return }
        if panel.isVisible { hide(); return }
        store.refresh()
        model?.reset()
        sizeToFit(panel)
        center(panel)
        installMonitor()
        NSApp.activate(ignoringOtherApps: true)
        panel.makeKeyAndOrderFront(nil)
        dbg("palette shown visible=\(panel.isVisible) key=\(panel.isKeyWindow) frame=\(panel.frame)")
    }

    /// Lay the SwiftUI content out NOW and adopt its fitting size. `.preferredContentSize`
    /// auto-sizing is async and leaves the panel 0×0 on first order-front.
    private func sizeToFit(_ panel: NSPanel) {
        guard let host = panel.contentViewController else { return }
        host.view.layoutSubtreeIfNeeded()
        var s = host.view.fittingSize
        if s.width < 100 { s.width = 620 }
        if s.height < 100 { s.height = 420 }
        panel.setContentSize(s)
    }

    private func build(store: AgentStore, l10n: L10n, onJump: @escaping (Agent) -> Void) {
        let model = PaletteModel(store: store)
        self.model = model
        self.jump = onJump
        let view = CommandPaletteView(
            model: model, l10n: l10n,
            onJump: { [weak self] a in self?.jump?(a); self?.hide() })
        let panel = KeyablePanel(
            contentRect: NSRect(x: 0, y: 0, width: 620, height: 480),
            styleMask: [.borderless, .fullSizeContentView], backing: .buffered, defer: false)
        let host = NSHostingController(rootView: view)
        host.sizingOptions = [.preferredContentSize]
        panel.contentViewController = host
        panel.isFloatingPanel = true
        panel.level = .floating
        panel.backgroundColor = .clear
        panel.isOpaque = false
        panel.hasShadow = true
        panel.hidesOnDeactivate = true
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient]
        panel.isMovableByWindowBackground = true
        self.panel = panel
    }

    private func center(_ panel: NSPanel) {
        guard let screen = NSScreen.main else { return }
        let vf = screen.visibleFrame
        let size = panel.frame.size
        panel.setFrameOrigin(NSPoint(
            x: vf.midX - size.width / 2,
            y: vf.maxY - size.height - vf.height * 0.18))
    }

    private func hide() { panel?.orderOut(nil) }

    private func installMonitor() {
        guard monitor == nil else { return }
        monitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            guard let self = self, let panel = self.panel, panel.isKeyWindow,
                  let model = self.model else { return event }
            let r = model.results
            switch event.keyCode {
            case 126: model.move(-1); return nil            // up
            case 125: model.move(1); return nil             // down
            case 36, 76:                                    // return / keypad enter
                if model.selected < r.count { self.jump?(r[model.selected]); self.hide() }
                return nil
            case 53: self.hide(); return nil                // escape
            default: break
            }
            if event.modifierFlags.contains(.command),
               let s = event.charactersIgnoringModifiers, let n = Int(s), (1...9).contains(n) {
                if n - 1 < r.count { self.jump?(r[n - 1]); self.hide() }
                return nil
            }
            return event
        }
    }
}

// MARK: - View (matches docs/design/mockup §4 B)

struct CommandPaletteView: View {
    @ObservedObject var model: PaletteModel
    @ObservedObject var l10n: L10n
    var onJump: (Agent) -> Void
    @Environment(\.colorScheme) private var scheme
    @FocusState private var searchFocused: Bool

    var body: some View {
        VStack(spacing: 0) {
            search
            Rectangle().fill(divider).frame(height: 0.5)
            results
            bottomBar
        }
        .frame(width: 620)
        .background { ZStack { VisualEffectWindow(); bg }.ignoresSafeArea() }
        .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous)
            .stroke(Color.white.opacity(scheme == .dark ? 0.12 : 0.10), lineWidth: 0.5))
        .onAppear { searchFocused = true; model.selected = 0 }
    }

    // MARK: search row — logo + field + hotkey keycap

    private var search: some View {
        HStack(spacing: 12) {
            GtmuxLogo(size: 18)
            TextField(l10n.tr("Jump to agent", "跳到某个 agent"), text: $model.query)
                .textFieldStyle(.plain).font(.system(size: 18)).foregroundStyle(fg)
                .focused($searchFocused)
                .onChange(of: model.query) { _, _ in model.selected = 0 }
            Text("⌘⌥G").font(.system(size: 11, design: .monospaced)).foregroundStyle(fg3)
                .padding(.horizontal, 6).padding(.vertical, 2)
                .overlay(RoundedRectangle(cornerRadius: 5).stroke(Color.white.opacity(0.15), lineWidth: 0.5))
        }
        .padding(.horizontal, 18).padding(.vertical, 16)
    }

    // MARK: results — grouped, single flat ForEach

    @ViewBuilder private var results: some View {
        let r = model.results
        if r.isEmpty {
            Text(l10n.tr("No matching agents", "没有匹配的 agent"))
                .font(.system(size: 12)).foregroundStyle(fg2)
                .frame(maxWidth: .infinity).frame(height: 84)
        } else {
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 1) {
                        ForEach(items) { item in
                            switch item {
                            case let .header(st): sectionHeader(st)
                            case let .agent(a, i): row(a, i)
                            }
                        }
                    }
                    .padding(.horizontal, 10).padding(.top, 2).padding(.bottom, 10)
                }
                .frame(height: min(CGFloat(r.count) * 54 + 120, 380))
                .onChange(of: model.selected) { _, s in
                    if s < r.count { proxy.scrollTo(PItem.agent(r[s], s).id, anchor: .center) }
                }
            }
        }
    }

    private enum PItem: Identifiable {
        case header(Status)
        case agent(Agent, Int)
        var id: String {
            switch self {
            case let .header(s): return "h\(s.rawValue)"
            case let .agent(a, _): return "a\(a.id):\(a.status)"
            }
        }
    }

    private var items: [PItem] {
        var out: [PItem] = []
        var idx = 0
        for s in model.sections {
            out.append(.header(s.status))
            for a in s.agents { out.append(.agent(a, idx)); idx += 1 }
        }
        return out
    }

    private func sectionHeader(_ st: Status) -> some View {
        Text(sectionTitle(st).uppercased())
            .font(.system(size: 10.5, weight: .bold)).kerning(0.6)
            .foregroundStyle(st == .waiting ? Theme.Status.waiting : fg3)
            .padding(.horizontal, 12).padding(.top, 8).padding(.bottom, 5)
    }

    private func sectionTitle(_ st: Status) -> String {
        switch st {
        case .waiting: return l10n.tr("Needs you", "需要你")
        case .working: return l10n.tr("Working", "运行中")
        case .idle:    return l10n.tr("Idle", "空闲")
        case .running: return l10n.tr("Running", "待命")
        }
    }

    private func row(_ a: Agent, _ i: Int) -> some View {
        let selected = i == model.selected
        return HStack(spacing: 13) {
            PaletteStatusIcon(status: a.state)
            VStack(alignment: .leading, spacing: 2) {
                Text(a.primary).font(.system(size: 15, weight: .semibold)).foregroundStyle(fg).lineLimit(1)
                Text(a.task.isEmpty ? "—" : a.task).font(.system(size: 12.5)).foregroundStyle(fg2).lineLimit(1)
            }
            Spacer(minLength: 8)
            Text(meta(a)).font(.system(size: 11, design: .monospaced)).foregroundStyle(fg3).lineLimit(1)
            if selected {
                Text(l10n.tr("⏎ jump", "⏎ 跳转")).font(.system(size: 12)).foregroundStyle(fg)
                    .padding(.horizontal, 9).padding(.vertical, 4)
                    .background(RoundedRectangle(cornerRadius: 6).fill(Color.white.opacity(0.14)))
            }
        }
        .padding(.horizontal, 12).padding(.vertical, 10)
        .background(RoundedRectangle(cornerRadius: 10, style: .continuous).fill(selected ? rowSel : .clear))
        .contentShape(Rectangle())
        .onHover { if $0 { model.selected = i } }
        .onTapGesture { onJump(a) }
    }

    private func meta(_ a: Agent) -> String {
        let id = a.isNative ? a.terminal : a.paneID
        return a.agent.isEmpty ? id : "\(a.agent) · \(id)"
    }

    // MARK: bottom bar

    private var bottomBar: some View {
        HStack(spacing: 16) {
            kbd("↑↓ " + l10n.tr("select", "选择"))
            kbd("⏎ " + l10n.tr("jump", "跳转"))
            kbd("⌘1–9 " + l10n.tr("direct", "直达"))
            Spacer()
            kbd("gtmux focus")
        }
        .padding(.horizontal, 16).padding(.vertical, 9)
        .background(scheme == .dark ? Color.white.opacity(0.04) : Color.black.opacity(0.03))
        .overlay(Rectangle().fill(divider).frame(height: 0.5), alignment: .top)
    }

    private func kbd(_ s: String) -> some View {
        Text(s).font(.system(size: 11, design: .monospaced)).foregroundStyle(fg3)
    }

    // MARK: palette tokens (DESIGN §9 / mockup — more opaque than the popover)

    private var bg: Color {
        scheme == .dark ? Color(hex: 0x18181B, opacity: 0.86) : Color(hex: 0xF4F4F6, opacity: 0.90)
    }
    private var fg: Color { scheme == .dark ? Color(white: 1, opacity: 0.95) : Color(hex: 0x1D1D1F) }
    private var fg2: Color {
        scheme == .dark ? Color(red: 235/255, green: 235/255, blue: 245/255, opacity: 0.60)
                        : Color(red: 60/255, green: 60/255, blue: 67/255, opacity: 0.62)
    }
    private var fg3: Color {
        scheme == .dark ? Color(red: 235/255, green: 235/255, blue: 245/255, opacity: 0.45)
                        : Color(red: 60/255, green: 60/255, blue: 67/255, opacity: 0.45)
    }
    private var divider: Color { scheme == .dark ? Color(white: 1, opacity: 0.10) : Color(black: 0, opacity: 0.10) }
    private var rowSel: Color { scheme == .dark ? Color(white: 1, opacity: 0.12) : Color(black: 0, opacity: 0.07) }
}

/// PaletteStatusIcon — the 32pt status-forward leading icon (mockup §4 B):
/// waiting is loud (solid red + white pause); the rest are quiet (translucent
/// tint + the status-colored glyph).
struct PaletteStatusIcon: View {
    let status: Status

    var body: some View {
        ZStack {
            if status == .waiting {
                RoundedRectangle(cornerRadius: 9, style: .continuous).fill(Theme.Status.waiting)
            } else {
                Circle().fill(status.color.opacity(0.16))
            }
            glyph
        }
        .frame(width: 32, height: 32)
    }

    @ViewBuilder private var glyph: some View {
        switch status {
        case .waiting:
            HStack(spacing: 2.2) {
                Capsule().fill(.white).frame(width: 2.6, height: 12)
                Capsule().fill(.white).frame(width: 2.6, height: 12)
            }
        case .idle:
            Image(systemName: "checkmark").font(.system(size: 13, weight: .bold)).foregroundStyle(status.color)
        case .working:
            Circle().trim(from: 0.08, to: 0.92)
                .stroke(status.color, style: StrokeStyle(lineWidth: 2.2, lineCap: .round))
                .frame(width: 15, height: 15).rotationEffect(.degrees(-80))
        case .running:
            Circle().fill(status.color).frame(width: 6, height: 6)
        }
    }
}
