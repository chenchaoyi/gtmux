import SwiftUI

struct KnowledgeUsageSummary: View {
    let entry: KBEntry
    @ObservedObject var l10n: L10n
    let p: Theme.Palette

    var body: some View {
        let use = entry.usage(l10n)
        VStack(alignment: .leading, spacing: 5) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text(l10n.tr("Use", "使用方式")).foregroundStyle(p.fg3)
                Text(use.title).foregroundStyle(p.fg)
                if !use.state.isEmpty { Text(use.state).foregroundStyle(p.fg2) }
            }
            .font(HQTypography.navigation)
            Text(use.explanation).font(HQTypography.caption).foregroundStyle(p.fg2)
                .fixedSize(horizontal: false, vertical: true)
            if let repo = entry.audienceRepo, !repo.isEmpty {
                Text(repo).font(HQTypography.code).foregroundStyle(p.fg3).textSelection(.enabled)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("knowledge-use-summary")
    }
}

struct KnowledgeUsageGuide: View {
    @ObservedObject var l10n: L10n
    let p: Theme.Palette

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                Text(l10n.tr("Knowledge and instructions", "从经验到指令"))
                    .font(HQTypography.rowTitle).foregroundStyle(p.fg)
                Text(l10n.tr("Entries start as references HQ can look up. Add a standing instruction only when it should guide repeated work.",
                    "知识默认供 HQ 按需查阅。需要反复遵守的约定，再加入相应的固定指令。"))
                    .font(HQTypography.navigation).foregroundStyle(p.fg2)
                ForEach(KnowledgeAudience.allCases, id: \.self) { scope in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(scope.word(l10n)).font(HQTypography.navigation).foregroundStyle(p.fg)
                        Text(scope.explanation(l10n)).font(HQTypography.caption).foregroundStyle(p.fg2)
                    }
                }
                Text(l10n.tr("Select an entry to read it, check its use or record a result.",
                    "选择左侧条目，查看内容、使用方式和应用记录。"))
                    .font(HQTypography.caption).foregroundStyle(p.fg3)
            }
            .fixedSize(horizontal: false, vertical: true)
            .padding(24).frame(maxWidth: 600, alignment: .leading)
        }
    }
}
