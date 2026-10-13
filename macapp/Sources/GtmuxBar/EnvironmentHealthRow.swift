import SwiftUI

struct EnvironmentHealthRow: View {
    let row: EnvironmentReport.Row
    @ObservedObject var l10n: L10n
    @State private var detailsOpen = false

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: symbol)
                .font(.system(size: 16))
                .foregroundStyle(tone)
                .frame(width: 20)
                .padding(.top, 2)
                .accessibilityLabel(statusLabel)
            VStack(alignment: .leading, spacing: 6) {
                HStack(alignment: .firstTextBaseline, spacing: 16) {
                    Text(row.label).font(Theme.Font.session)
                        .fixedSize(horizontal: false, vertical: true)
                    Spacer(minLength: 0)
                    Text(row.value).font(Theme.Font.task)
                        .foregroundStyle(.primary)
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
                if !row.note.isEmpty || row.value.count > 70 {
                    DisclosureGroup(isExpanded: $detailsOpen) {
                        VStack(alignment: .leading, spacing: 6) {
                            Text(row.value).font(Theme.Font.task)
                            if !row.note.isEmpty {
                                Text(row.note).font(Theme.Font.task)
                            }
                        }
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.top, 4)
                    } label: {
                        Text(l10n.tr("Details", "技术详情")).font(Theme.Font.action)
                    }
                    .font(Theme.Font.action)
                    .foregroundStyle(.secondary)
                }
            }
        }
        .padding(.vertical, 12)
        .accessibilityElement(children: .contain)
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
                .padding(.bottom, 6)
            ForEach(section.rows) { item in
                Divider()
                EnvironmentHealthRow(row: item.row, l10n: l10n)
            }
        }
    }
}
