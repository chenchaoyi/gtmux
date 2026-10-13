import SwiftUI

struct EnvironmentFindingsSummary: View {
    let findings: [EnvironmentReport.Finding]
    @ObservedObject var l10n: L10n
    let open: (EnvironmentReport.RowID) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(l10n.tr("Findings (\(findings.count))", "需要关注（\(findings.count) 项）"))
                .font(Theme.Font.session).padding(.bottom, 4)
            ForEach(findings) { finding in
                Divider()
                Button { open(finding.target) } label: {
                    HStack(alignment: .top, spacing: 10) {
                        Image(systemName: finding.row.status == "blocking" ? "xmark.octagon" : "exclamationmark.triangle")
                            .font(.system(size: 14)).frame(width: 18)
                            .foregroundStyle(finding.row.status == "blocking" ? Theme.Status.waiting : Theme.Status.errored)
                        VStack(alignment: .leading, spacing: 3) {
                            HStack(spacing: 12) {
                                Text(finding.row.label).font(Theme.Font.session)
                                Spacer(minLength: 0)
                                Text(finding.row.status == "blocking" ? l10n.tr("Affects features", "影响功能") : l10n.tr("Recommendation", "建议处理"))
                                    .font(Theme.Font.task).foregroundStyle(.secondary)
                            }
                            Text(finding.section + " · " + finding.row.value)
                                .font(Theme.Font.task).foregroundStyle(.secondary).lineLimit(2)
                            if !finding.row.note.isEmpty {
                                Text(finding.row.note).font(Theme.Font.task).foregroundStyle(.secondary).lineLimit(2)
                            }
                        }
                        Image(systemName: "chevron.right").font(.system(size: 12))
                            .foregroundStyle(.secondary).padding(.top, 2)
                    }
                    .foregroundStyle(.primary)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.vertical, 8)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .help(l10n.tr("Open this check and its full details", "定位到此检查项并展开完整说明"))
                .accessibilityHint(l10n.tr("Opens the original check", "定位到原检查项"))
            }
        }
    }
}

struct EnvironmentResultClasses: View {
    @ObservedObject var l10n: L10n
    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(l10n.tr("Check results", "检查结果分类")).font(Theme.Font.session)
            Text(l10n.tr("Passed: the checked prerequisite is satisfied.", "正常：已满足这项检查的条件。"))
            Text(l10n.tr("Recommendation: configuration or maintenance can be improved; some features may be limited.", "建议处理：配置或维护项可改进，部分功能可能受限。"))
            Text(l10n.tr("Affects features: the reported condition prevents the corresponding feature from working normally.", "影响功能：该问题会影响相应功能的正常使用。"))
            Text(l10n.tr("Information: optional or unverified status for reference, excluded from findings.", "信息：供参考的状态，可包含未启用或未核实项，不计入需要关注的事项。"))
            Divider()
            Text(l10n.tr("Checking again reads the current state. It does not apply repairs.", "重新检查只读取当前状态，不会自动修复。"))
                .foregroundStyle(.secondary)
        }
        .font(Theme.Font.task)
        .fixedSize(horizontal: false, vertical: true)
        .frame(width: 300, alignment: .leading)
        .padding(16)
    }
}
