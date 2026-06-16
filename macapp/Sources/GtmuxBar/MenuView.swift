import SwiftUI

enum MenuAction {
    case overview, watch, restore, newSession, quit
}

/// MenuView is the SwiftUI popover: a summary header, the agents grouped by
/// urgency (waiting → working → idle), and an actions footer. A real custom
/// view (vs the old text-only NSMenu) so it can group, emphasize "needs you",
/// and grow richer over time.
struct MenuView: View {
    @ObservedObject var store: AgentStore
    var onJump: (Agent) -> Void
    var onAction: (MenuAction) -> Void

    private var version: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "dev"
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(store.summary)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(.secondary)
                .padding(.horizontal, 12).padding(.top, 10).padding(.bottom, 6)

            Divider()

            if store.agents.isEmpty {
                emptyState
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 1) {
                        section("Needs you", "waiting")
                        section("Working", "working")
                        section("Idle", "idle", includeRunning: true)
                    }
                    .padding(.vertical, 4)
                }
                .frame(maxHeight: 340)
            }

            Divider()
            footer
        }
        .frame(width: 320)
    }

    @ViewBuilder
    private func section(_ title: String, _ status: String, includeRunning: Bool = false) -> some View {
        let rows = store.agents.filter {
            $0.status == status || (includeRunning && $0.status == "running")
        }
        if !rows.isEmpty {
            Text(title.uppercased())
                .font(.system(size: 9, weight: .bold))
                .foregroundStyle(.tertiary)
                .padding(.horizontal, 12).padding(.top, 6).padding(.bottom, 2)
            ForEach(rows) { agent in
                AgentRow(agent: agent) { onJump(agent) }
            }
        }
    }

    private var emptyState: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("No agents running").font(.system(size: 12, weight: .medium))
            Text("Start an agent in a tmux pane (e.g. claude) and it'll show up here.")
                .font(.system(size: 11)).foregroundStyle(.secondary)
        }
        .padding(.horizontal, 12).padding(.vertical, 14)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var footer: some View {
        VStack(spacing: 0) {
            actionRow("Overview", "rectangle.3.group") { onAction(.overview) }
            actionRow("Live watch…", "waveform.path.ecg") { onAction(.watch) }
            actionRow("Restore detached", "arrow.uturn.backward") { onAction(.restore) }
            actionRow("New session", "plus.rectangle") { onAction(.newSession) }
            Divider().padding(.vertical, 4)
            HStack {
                Text("gtmux \(version)").font(.system(size: 10)).foregroundStyle(.tertiary)
                Spacer()
                Button("Quit") { onAction(.quit) }
                    .buttonStyle(.plain)
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
            }
            .padding(.horizontal, 12).padding(.vertical, 6)
        }
        .padding(.vertical, 4)
    }

    private func actionRow(_ title: String, _ symbol: String, _ act: @escaping () -> Void) -> some View {
        HoverButton(action: act) {
            HStack(spacing: 8) {
                Image(systemName: symbol).frame(width: 16)
                Text(title).font(.system(size: 12))
                Spacer()
            }
            .padding(.horizontal, 12).padding(.vertical, 5)
        }
    }
}

/// AgentRow — one agent: status glyph, session (emphasis), task (dimmed).
private struct AgentRow: View {
    let agent: Agent
    let onTap: () -> Void

    private var glyph: String {
        switch agent.status {
        case "working": return "⠿"
        case "waiting": return "⏸"
        case "idle":    return "✳"
        default:        return "●"
        }
    }

    var body: some View {
        HoverButton(action: onTap) {
            HStack(spacing: 8) {
                Text(glyph).font(.system(size: 12)).frame(width: 14)
                VStack(alignment: .leading, spacing: 1) {
                    Text(agent.session.isEmpty ? agent.loc : agent.session)
                        .font(.system(size: 12, weight: .medium))
                        .lineLimit(1)
                    if !agent.task.isEmpty {
                        Text(agent.task).font(.system(size: 10))
                            .foregroundStyle(.secondary).lineLimit(1)
                    }
                }
                Spacer()
                if agent.latest {
                    Text("✓").font(.system(size: 10)).foregroundStyle(.secondary)
                }
            }
            .padding(.horizontal, 12).padding(.vertical, 4)
            .contentShape(Rectangle())
        }
    }
}

/// HoverButton — a plain button that highlights its row on hover (menu feel).
private struct HoverButton<Label: View>: View {
    let action: () -> Void
    @ViewBuilder var label: () -> Label
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            label()
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(hovering ? Color.accentColor.opacity(0.18) : Color.clear)
                .clipShape(RoundedRectangle(cornerRadius: 5))
        }
        .buttonStyle(.plain)
        .onHover { hovering = $0 }
        .padding(.horizontal, 4)
    }
}
