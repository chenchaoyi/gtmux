import AppKit
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
    init(store: AgentStore) { self.store = store }

    var results: [Agent] { store.ordered(waitingOnly: false, query: query) }

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
        center(panel)
        installMonitor()
        NSApp.activate(ignoringOtherApps: true)
        panel.makeKeyAndOrderFront(nil)
    }

    private func build(store: AgentStore, l10n: L10n, onJump: @escaping (Agent) -> Void) {
        let model = PaletteModel(store: store)
        self.model = model
        self.jump = onJump
        let view = CommandPaletteView(
            model: model, l10n: l10n,
            onJump: { [weak self] a in self?.jump?(a); self?.hide() })
        let panel = KeyablePanel(
            contentRect: NSRect(x: 0, y: 0, width: 560, height: 460),
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

// MARK: - View

struct CommandPaletteView: View {
    @ObservedObject var model: PaletteModel
    @ObservedObject var l10n: L10n
    var onJump: (Agent) -> Void
    @Environment(\.colorScheme) private var scheme
    @FocusState private var searchFocused: Bool

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            search(p)
            Divider().overlay(p.divider)
            results(p)
            Divider().overlay(p.divider)
            shortcutBar(p)
        }
        .frame(width: 560)
        .background { ZStack { VisualEffectWindow(); p.bg } }
        .clipShape(RoundedRectangle(cornerRadius: 13, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 13, style: .continuous).stroke(p.divider, lineWidth: 1))
        .onAppear { searchFocused = true; model.selected = 0 }
    }

    private func search(_ p: Theme.Palette) -> some View {
        HStack(spacing: 10) {
            Image(systemName: "magnifyingglass").font(.system(size: 16)).foregroundStyle(p.fg3)
            TextField(l10n.tr("Jump to an agent…", "跳到某个 agent…"), text: $model.query)
                .textFieldStyle(.plain).font(Theme.Font.title).foregroundStyle(p.fg)
                .focused($searchFocused)
                .onChange(of: model.query) { _, _ in model.selected = 0 }
        }
        .padding(.horizontal, 16).padding(.vertical, 13)
    }

    @ViewBuilder private func results(_ p: Theme.Palette) -> some View {
        let r = model.results
        if r.isEmpty {
            VStack(spacing: 5) {
                Text(l10n.tr("No matching agents", "没有匹配的 agent"))
                    .font(.system(size: 12)).foregroundStyle(p.fg2)
            }
            .frame(maxWidth: .infinity).frame(height: 90)
        } else {
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: 2) {
                        ForEach(Array(r.enumerated()), id: \.element.id) { i, agent in
                            PaletteRow(agent: agent, index: i, selected: i == model.selected, l10n: l10n)
                                .id(i)
                                .onHover { if $0 { model.selected = i } }
                                .onTapGesture { onJump(agent) }
                        }
                    }
                    .padding(8)
                }
                .frame(height: min(CGFloat(r.count) * 50 + 16, 360))
                .onChange(of: model.selected) { _, s in proxy.scrollTo(s) }
            }
        }
    }

    private func shortcutBar(_ p: Theme.Palette) -> some View {
        HStack(spacing: 12) {
            hint("↑↓", l10n.tr("select", "选择"), p)
            hint("⏎", l10n.tr("jump", "跳转"), p)
            hint("⌘1–9", l10n.tr("direct", "直达"), p)
            Spacer()
            hint("esc", l10n.tr("close", "关闭"), p)
        }
        .padding(.horizontal, 16).padding(.vertical, 8)
    }

    private func hint(_ key: String, _ label: String, _ p: Theme.Palette) -> some View {
        HStack(spacing: 4) {
            Text(key).font(.system(size: 10, weight: .medium, design: .rounded))
                .padding(.horizontal, 4).padding(.vertical, 1)
                .background(RoundedRectangle(cornerRadius: 4).fill(scheme == .dark ? Color.white.opacity(0.08) : Color.black.opacity(0.06)))
            Text(label).font(.system(size: 10))
        }.foregroundStyle(p.fg3)
    }
}

private struct PaletteRow: View {
    let agent: Agent
    let index: Int
    let selected: Bool
    @ObservedObject var l10n: L10n
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        HStack(spacing: 11) {
            AgentAvatar(agent: agent)
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 5) {
                    Text(agent.primary).font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg).lineLimit(1)
                    if !agent.secondary.isEmpty {
                        Text("· \(agent.secondary)").font(Theme.Font.window).foregroundStyle(p.fg3).lineLimit(1)
                    }
                    if agent.isNative {
                        Text("native").font(.system(size: 8.5)).foregroundStyle(p.fg3)
                    }
                }
                Text(agent.task.isEmpty ? "—" : agent.task).font(Theme.Font.task).foregroundStyle(p.fg2).lineLimit(1)
            }
            Spacer(minLength: 6)
            if index < 9 {
                Text("⌘\(index + 1)").font(.system(size: 10, weight: .medium, design: .rounded)).foregroundStyle(p.fg3)
            }
        }
        .padding(.horizontal, 10).padding(.vertical, 7)
        .frame(minHeight: 44)
        .background(RoundedRectangle(cornerRadius: Theme.Size.radiusRow, style: .continuous)
            .fill(selected ? p.rowSelected : .clear))
        .contentShape(Rectangle())
    }
}
