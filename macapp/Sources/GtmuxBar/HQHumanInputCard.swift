import SwiftUI

/// The actionable question stays outside the technical conversation history.
struct HQHumanInputCard: View {
    let l10n: L10n
    let matter: HQMatter
    var preparedChoice: String? = nil
    let prepare: (HQHumanInput, String) -> Void
    var body: some View {
        if matter.state == "needs-input" {
            VStack(alignment: .leading, spacing: 8) {
                if let q = matter.userInput {
                    Label(q.answerId == nil ? title(q.kind) : l10n.tr("Reply saved", "答复已保存"),
                          systemImage: q.answerId == nil ? "bubble.left.and.bubble.right" : "checkmark.circle")
                        .font(HQTypography.caption).foregroundStyle(.secondary)
                    Text(q.prompt).font(HQTypography.section).textSelection(.enabled)
                    if q.answerId == nil {
                        Text(q.why).font(HQTypography.body).fixedSize(horizontal: false, vertical: true)
                        DisclosureGroup(l10n.tr("Scope and impact", "范围与影响")) {
                            Text(q.impact).font(HQTypography.body).textSelection(.enabled)
                        }
                        LazyVGrid(columns: [GridItem(.adaptive(minimum: 150))], alignment: .leading, spacing: 6) {
                            ForEach(q.choices ?? [], id: \.self) { choice in
                                Button { prepare(q, choice) } label: {
                                    HStack {
                                        if preparedChoice == choice { Image(systemName: "checkmark") }
                                        Text(choice)
                                    }
                                }.buttonStyle(.bordered)
                                    .tint(preparedChoice == choice ? .accentColor : nil)
                                    .accessibilityValue(preparedChoice == choice ? l10n.tr("Prepared", "已填入输入框") : "")
                                    .accessibilityIdentifier("hq-human-choice-\(q.choices?.firstIndex(of: choice) ?? 0)")
                            }
                            Button(l10n.tr("Write a reply", "自行答复")) { prepare(q, "") }.buttonStyle(.bordered)
                                .accessibilityIdentifier("hq-human-write-reply")
                        }
                        Text(preparedChoice == nil
                             ? l10n.tr("Choose to prepare a reply, then send it below.", "选择后会填入下方输入框，由你确认发送。")
                             : l10n.tr("Reply prepared below. Review it, then send.", "已填入下方输入框，确认后发送。"))
                            .font(HQTypography.caption).foregroundStyle(.secondary)
                    } else {
                        Text(l10n.tr("HQ will confirm receipt and follow up. Saving a reply does not mean the action has run.", "接下来由 HQ 确认并跟进；答复已保存不代表操作已执行。"))
                            .font(HQTypography.body).foregroundStyle(.secondary)
                    }
                } else {
                    Label(l10n.tr("HQ needs to clarify", "待 HQ 明确问题"), systemImage: "info.circle")
                        .font(HQTypography.section)
                    Text(l10n.tr("This older record has no specific question or choices. HQ needs to check whether it still needs your input.", "这条旧记录没有具体问题和选项，需要 HQ 先核对是否仍需你处理。"))
                        .font(HQTypography.body).foregroundStyle(.secondary)
                }
            }.padding(12).frame(maxWidth: .infinity, alignment: .leading)
                .background(.quaternary.opacity(0.4), in: RoundedRectangle(cornerRadius: 10))
                .accessibilityIdentifier("hq-current-human-request")
        }
    }
    private func title(_ kind: String) -> String {
        switch kind {
        case "manual": return l10n.tr("Your action needed", "请你操作")
        case "information": return l10n.tr("A detail is missing", "请你补充")
        default: return l10n.tr("Your decision needed", "请你选择")
        }
    }
}
