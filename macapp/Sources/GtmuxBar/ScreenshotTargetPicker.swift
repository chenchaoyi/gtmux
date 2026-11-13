import AppKit
import SwiftUI

enum ScreenshotTargetSearch {
    static func context(_ agent: Agent) -> String {
        [agent.agent, agent.secondary].filter { !$0.isEmpty }.joined(separator: " · ")
    }

    static func results(_ agents: [Agent], query: String) -> [Agent] {
        guard query.count <= 256 else { return [] }
        let tokens = normalized(query).split(whereSeparator: { $0.isWhitespace }).map(String.init)
        guard !tokens.isEmpty else { return agents }
        guard tokens.count <= 16 else { return [] }
        return agents.enumerated().compactMap { index, agent -> (Int, Int, Agent)? in
            let fields = [agent.primary, agent.session, agent.paneID, agent.project, agent.agent].map(normalized)
            var total = 0
            for token in tokens {
                let best = fields.enumerated().compactMap { offset, field in
                    score(token, in: field).map { $0 + offset * 3 }
                }.min()
                guard let best else { return nil }
                total += best
            }
            return (total, index, agent)
        }.sorted { $0.0 == $1.0 ? $0.1 < $1.1 : $0.0 < $1.0 }.map { $0.2 }
    }

    private static func normalized(_ value: String) -> String {
        value.folding(options: [.caseInsensitive, .diacriticInsensitive, .widthInsensitive],
                      locale: Locale(identifier: "en_US_POSIX"))
    }

    private static func score(_ token: String, in value: String) -> Int? {
        if value == token { return 0 }
        if value.hasPrefix(token) { return 10 }
        if value.contains(token) { return 25 }
        // Ordered subsequence matching supports abbreviations without fabricating titles.
        var next = token.startIndex, first: Int?, last = 0
        for (index, character) in value.enumerated() where next < token.endIndex {
            if character == token[next] {
                if first == nil { first = index }
                last = index
                next = token.index(after: next)
            }
        }
        guard next == token.endIndex, let first else { return nil }
        return 60 + min(100, last - first + 1 - token.count) + min(50, first)
    }
}

struct ScreenshotTargetList: View {
    let agents: [Agent]
    let selected: String?
    let recent: String?
    @ObservedObject var l10n: L10n
    let dismiss: () -> Void
    let pick: (String) -> Void
    @State private var query = ""
    @State private var highlighted: String?

    private var matches: [Agent] { ScreenshotTargetSearch.results(agents, query: query) }

    var body: some View {
        VStack(spacing: 6) {
            ScreenshotTargetSearchField(text: $query, placeholder: l10n.tr("Search sessions, projects or pane IDs", "搜索会话、项目或窗格编号"),
                                        move: move, submit: chooseHighlighted, dismiss: dismiss)
                .frame(height: 28).padding(.horizontal, 8).padding(.top, 8)
            if matches.isEmpty {
                Text(l10n.tr("No matching sessions", "没有匹配的会话"))
                    .foregroundStyle(.secondary).frame(maxWidth: .infinity, minHeight: 80)
            } else {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(spacing: 2) {
                            ForEach(matches) { agent in
                                Button { pick(agent.paneID) } label: { row(agent) }
                                    .buttonStyle(.plain).id(agent.paneID)
                                    .onHover { over in if over { highlighted = agent.paneID } }
                            }
                        }.padding(6)
                    }
                    .onChange(of: highlighted) { value in
                        if let value { proxy.scrollTo(value, anchor: .center) }
                    }
                }.frame(height: min(310, CGFloat(matches.count) * 50 + 12))
            }
            Text(l10n.tr("↑↓ Select · Return Confirm · Esc Close", "↑↓ 选择 · 回车确认 · Esc 关闭"))
                .font(.system(size: 11)).foregroundStyle(.secondary).padding(.bottom, 6)
        }
        .frame(width: 440)
        .onAppear { highlighted = matches.first(where: { $0.paneID == selected })?.paneID ?? matches.first?.paneID }
        .onChange(of: query) { _ in highlighted = matches.first?.paneID }
        .onChange(of: agents.map(\.paneID)) { _ in
            if !matches.contains(where: { $0.paneID == highlighted }) { highlighted = matches.first?.paneID }
        }
    }

    private func move(_ delta: Int) {
        guard !matches.isEmpty else { highlighted = nil; return }
        let index = matches.firstIndex { $0.paneID == highlighted } ?? (delta > 0 ? -1 : matches.count)
        highlighted = matches[max(0, min(matches.count - 1, index + delta))].paneID
    }

    private func chooseHighlighted() {
        guard let agent = matches.first(where: { $0.paneID == highlighted }) else { return }
        pick(agent.paneID)
    }

    private func row(_ agent: Agent) -> some View {
        HStack(spacing: 10) {
            ScreenshotAgentMark(agent: agent, size: 24)
            VStack(alignment: .leading, spacing: 3) {
                Text(agent.primary).font(.system(size: 13, weight: .semibold)).lineLimit(1)
                HStack(spacing: 6) {
                    Text(ScreenshotTargetSearch.context(agent)).lineLimit(1)
                    if agent.paneID == recent { Text(l10n.tr("Recently viewed", "最近查看")).fixedSize() }
                }.font(.system(size: 11.5)).foregroundStyle(.secondary)
            }
            Spacer(minLength: 6)
            if agent.paneID == selected { Image(systemName: "checkmark").accessibilityLabel(l10n.tr("Selected", "已选择")) }
            StatusBadge(status: agent.state, size: 13, errored: agent.errored)
        }
        .padding(.horizontal, 10).frame(height: 48)
        .background(RoundedRectangle(cornerRadius: 8).fill(Color.accentColor.opacity(agent.paneID == highlighted ? 0.16 : 0)))
        .contentShape(Rectangle())
        .help(agent.primary + " · " + ScreenshotTargetSearch.context(agent))
        .accessibilityAddTraits(agent.paneID == selected ? .isSelected : [])
    }
}

/// Only navigation commands are intercepted; IME composition belongs to the field editor.
private struct ScreenshotTargetSearchField: NSViewRepresentable {
    @Binding var text: String
    let placeholder: String
    let move: (Int) -> Void
    let submit: () -> Void
    let dismiss: () -> Void

    func makeCoordinator() -> Coordinator { Coordinator(self) }
    func makeNSView(context: Context) -> NSSearchField {
        let field = ScreenshotSearchInput()
        field.delegate = context.coordinator
        field.sendsSearchStringImmediately = true
        field.placeholderString = placeholder
        field.setAccessibilityLabel(placeholder)
        return field
    }
    func updateNSView(_ field: NSSearchField, context: Context) {
        context.coordinator.parent = self
        field.placeholderString = placeholder
        if field.stringValue != text, (field.currentEditor() as? NSTextView)?.hasMarkedText() != true { field.stringValue = text }
    }
    final class Coordinator: NSObject, NSSearchFieldDelegate {
        var parent: ScreenshotTargetSearchField
        init(_ parent: ScreenshotTargetSearchField) { self.parent = parent }
        func controlTextDidChange(_ notification: Notification) {
            guard let field = notification.object as? NSSearchField else { return }
            parent.text = field.stringValue
        }
        func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
            guard !textView.hasMarkedText() else { return false }
            switch selector {
            case #selector(NSResponder.moveDown(_:)): parent.move(1)
            case #selector(NSResponder.moveUp(_:)): parent.move(-1)
            case #selector(NSResponder.insertNewline(_:)): parent.submit()
            case #selector(NSResponder.cancelOperation(_:)): parent.dismiss()
            default: return false
            }
            return true
        }
    }
}

private final class ScreenshotSearchInput: NSSearchField {
    private var focusedOnce = false
    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        guard let window, !focusedOnce else { return }
        DispatchQueue.main.async { [weak self, weak window] in
            guard let self, let window, self.window === window, !self.focusedOnce else { return }
            self.focusedOnce = window.makeFirstResponder(self)
        }
    }
}
