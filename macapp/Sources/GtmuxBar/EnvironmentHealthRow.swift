import SwiftUI

struct EnvironmentHealthRow: View {
    let row: EnvironmentReport.Row
    @ObservedObject var l10n: L10n
    @State private var detailsOpen = false

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: symbol)
                .font(.system(size: 16))
                .foregroundStyle(tone)
                .frame(width: 20)
                .padding(.top, 2)
                .accessibilityLabel(statusLabel)
            if !row.note.isEmpty || row.value.count > 70 {
                DisclosureGroup(isExpanded: $detailsOpen) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(row.value).font(Theme.Font.task)
                        if !row.note.isEmpty { Text(row.note).font(Theme.Font.task) }
                    }
                    .foregroundStyle(.secondary)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.top, 4)
                } label: {
                    summary
                }
                .font(Theme.Font.action)
                .help(l10n.tr("Show technical details", "查看技术详情"))
            } else {
                summary
            }
        }
        .padding(.vertical, 6)
        .accessibilityElement(children: .contain)
    }

    private var summary: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(alignment: .firstTextBaseline, spacing: 12) {
                Text(row.label).font(Theme.Font.session).foregroundStyle(.primary)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 0)
                Text(row.value).font(Theme.Font.task).foregroundStyle(.primary)
                    .multilineTextAlignment(.trailing)
                    .lineLimit(2)
                    .frame(maxWidth: 240, alignment: .trailing)
                    .help(row.value)
                    .accessibilityLabel(row.value)
            }
            if !row.purpose.isEmpty {
                Text(row.purpose).font(Theme.Font.task).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private var symbol: String {
        switch row.status {
        case "ok": return "checkmark.circle"
        case "blocking": return "xmark.octagon"
        case "recommended": return "exclamationmark.triangle"
        default: return "info.circle"
        }
    }
    private var tone: Color {
        switch row.status {
        case "ok": return Theme.Status.idle
        case "blocking": return Theme.Status.waiting
        case "recommended": return Theme.Status.errored
        default: return .secondary
        }
    }
    private var statusLabel: String {
        switch row.status {
        case "ok": return l10n.tr("Passed", "正常")
        case "blocking": return l10n.tr("Needs attention", "需要处理")
        case "recommended": return l10n.tr("Recommendation", "建议处理")
        default: return l10n.tr("Information", "提示")
        }
    }
}

struct EnvironmentHealthSection: View {
    let section: EnvironmentReport.DisplaySection
    @ObservedObject var l10n: L10n
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(section.title).font(Theme.Font.session)
                .foregroundStyle(.secondary)
                .padding(.bottom, 4)
            ForEach(section.rows) { item in
                Divider()
                EnvironmentHealthRow(row: item.row, l10n: l10n)
            }
        }
    }
}
