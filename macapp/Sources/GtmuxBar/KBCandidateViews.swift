import SwiftUI

/// Raw captures can contain whole prompts, paths and line breaks. The sidebar is an
/// index, so it shows a bounded preview without selectable text or inline actions.
struct KBCandidateRow: View {
    let candidate: KBCandidateGroup
    let selected: Bool
    let p: Theme.Palette
    let select: () -> Void

    var body: some View {
        Button(action: select) {
            VStack(alignment: .leading, spacing: 3) {
                Text(candidate.lesson)
                    .font(.system(size: 12))
                    .foregroundStyle(p.fg)
                    .lineLimit(2)
                    .multilineTextAlignment(.leading)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                HStack(spacing: 6) {
                    Text(candidate.topic).font(.system(size: 10)).foregroundStyle(p.fg3)
                    Spacer(minLength: 4)
                    Text("›").font(.system(size: 12)).foregroundStyle(p.fg3)
                }
            }
            .padding(.horizontal, 12).padding(.vertical, 7)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(alignment: .leading) { KBSelectionMark(selected: selected, p: p) }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("kb-candidate-\(candidate.key)")
        .overlay(alignment: .bottom) {
            Rectangle().fill(p.divider).frame(height: 0.5).padding(.leading, 12)
        }
    }
}

/// The original lead is readable and selectable in the wider pane. Keep its wording
/// intact; filing it as knowledge is HQ's job. Actions reuse the reader's reason sheet.
struct KBCandidateDetail<Actions: View>: View {
    let candidate: KBCandidateGroup
    let l10n: L10n
    let p: Theme.Palette
    @ViewBuilder let actions: Actions

    var body: some View {
        ScrollView {
            content
        }
        .accessibilityIdentifier("kb-candidate-detail")
    }

    /// The same full document, separately renderable to check its intrinsic layout.
    var content: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Awaiting HQ review", "待 HQ 整理的线索"))
                .font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg)
            Text(l10n.tr(
                "HQ checks these leads and adds useful ones to the knowledge base. You can dismiss a lead if it is incorrect or redundant.",
                "HQ 会核实这些线索，将有价值的内容整理为知识。有误或重复的线索可以驳回。"))
                .font(.system(size: 11)).foregroundStyle(p.fg2)
                .fixedSize(horizontal: false, vertical: true)

            Divider()

            Text(candidate.lesson)
                .font(.system(size: 13)).foregroundStyle(p.fg)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityIdentifier("kb-candidate-full-text")

            Text(candidate.key)
                .font(.system(size: 10, design: .monospaced)).foregroundStyle(p.fg3)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: 8) {
                Text(candidate.topic)
                if candidate.count > 1 {
                    Text(l10n.tr("\(candidate.count) captures", "\(candidate.count) 条记录"))
                }
                if candidate.family > 0 {
                    Text(l10n.tr("≈ family \(candidate.family)", "≈ 同一件事 \(candidate.family)"))
                }
            }
            .font(.system(size: 10)).foregroundStyle(p.fg3)

            Divider()
            HStack(spacing: 8) {
                actions
                Spacer(minLength: 0)
            }
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}
