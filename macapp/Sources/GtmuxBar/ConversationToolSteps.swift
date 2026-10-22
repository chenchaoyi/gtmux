import SwiftUI

/// Public recorded calls, not execution-success receipts. The inspector preserves
/// whitespace and shares the existing native copy feedback and reading typography.
struct ConversationToolSteps: View {
    let steps: [DesktopChatStep]
    let l10n: L10n
    @State private var open = false
    @State private var selected: Int?
    @Environment(\.colorScheme) private var scheme
    private var p: Theme.Palette { .of(scheme) }
    private var heading: String {
        if !open, let first = steps.first, Set(steps.map(\.action)).count == 1 { return first.actionLabel(l10n) }
        return l10n.tr(open ? "Tool calls" : "Activity", open ? "工具调用" : "执行过程")
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Button {
                open.toggle()
                if !open { selected = nil }
            } label: {
                HStack(spacing: 6) {
                    Image(systemName: open ? "chevron.down" : "chevron.right").font(.system(size: 10, weight: .semibold))
                    Text("\(heading) · \(steps.count)")
                }.frame(minHeight: 24).contentShape(Rectangle())
            }.buttonStyle(.plain).font(HQTypography.caption).foregroundStyle(p.fg2)
                .accessibilityIdentifier("desktop-tool-group")
                .accessibilityValue(l10n.tr(open ? "Expanded" : "Collapsed", open ? "已展开" : "已折叠"))
            if open {
                VStack(alignment: .leading, spacing: 0) {
                    ForEach(Array(steps.enumerated()), id: \.offset) { index, step in
                        VStack(alignment: .leading, spacing: 6) {
                            if step.hasDetails {
                                Button { selected = selected == index ? nil : index } label: {
                                    row(step, selected: selected == index)
                                }.buttonStyle(.plain).accessibilityIdentifier("desktop-tool-\(index)")
                                    .accessibilityLabel("\(step.actionLabel(l10n)): \(step.preview(l10n))")
                                    .accessibilityValue(l10n.tr(selected == index ? "Expanded" : "Collapsed", selected == index ? "已展开" : "已折叠"))
                            } else { row(step, selected: false) }
                            if selected == index { ConversationToolInput(step: step, l10n: l10n) }
                        }.padding(.vertical, 6)
                        if index < steps.count - 1 { Divider().opacity(0.6) }
                    }
                }.padding(.leading, 12).overlay(alignment: .leading) { Rectangle().fill(p.divider).frame(width: 1) }
            }
        }
    }
    private func row(_ step: DesktopChatStep, selected: Bool) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
            Image(systemName: step.symbol).frame(width: 16).accessibilityHidden(true)
            Text(step.actionLabel(l10n)).font(HQTypography.caption)
            let preview = step.preview(l10n)
            if preview.lowercased() != step.title.lowercased() && preview != step.actionLabel(l10n) {
                Text(preview).font(step.action == "agent" || step.action == "wait" ? HQTypography.caption : HQTypography.code)
                    .foregroundStyle(p.fg2).lineLimit(2).frame(maxWidth: .infinity, alignment: .leading)
            } else { Spacer(minLength: 0) }
            if step.hasDetails { Image(systemName: selected ? "chevron.down" : "chevron.right").font(.system(size: 10)) }
        }.foregroundStyle(p.fg2).frame(minHeight: 28).contentShape(Rectangle())
    }
}

private struct ConversationToolInput: View {
    let step: DesktopChatStep
    let l10n: L10n
    @StateObject private var copy = HQChatCopyFeedback()
    @Environment(\.colorScheme) private var scheme
    private var height: CGFloat { CGFloat(min(12, max(2, step.recordedInput.split(separator: "\n", omittingEmptySubsequences: false).count))) * 18 + 20 }
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text(step.title).font(HQTypography.code).textSelection(.enabled)
                Spacer()
                if !step.recordedInput.isEmpty {
                    Button { copy.copy(step.recordedInput) } label: { Image(systemName: copy.state == .copied ? "checkmark" : "doc.on.doc") }
                        .buttonStyle(.plain)
                        .help(l10n.tr("Copy recorded input", "复制已记录输入"))
                        .accessibilityLabel(l10n.tr("Copy recorded input", "复制已记录输入"))
                }
            }
            if let cwd = step.workdir, !cwd.isEmpty {
                Text(l10n.tr("Working directory: ", "工作目录：") + cwd).font(HQTypography.caption).textSelection(.enabled)
            }
            if !step.recordedInput.isEmpty {
                ScrollView([.horizontal, .vertical]) {
                    Text(step.recordedInput).font(HQTypography.code).textSelection(.enabled)
                        .fixedSize(horizontal: true, vertical: true).padding(10).frame(maxWidth: .infinity, alignment: .leading)
                }.frame(height: height).background(Theme.Palette.of(scheme).rowSelected, in: RoundedRectangle(cornerRadius: 6))
                    .accessibilityLabel(l10n.tr("Recorded input; scroll to inspect", "已记录输入，可滚动查看"))
                    .accessibilityIdentifier("desktop-tool-input")
            }
            if step.detailsTruncated == true {
                Label(l10n.tr("Only the recorded excerpt is available", "仅能查看已记录的片段"), systemImage: "info.circle")
                    .font(HQTypography.caption)
            }
        }.padding(10).foregroundStyle(Theme.Palette.of(scheme).fg2)
            .onChange(of: step.recordedInput) { _, _ in copy.clear() }.onDisappear { copy.clear() }
    }
}
