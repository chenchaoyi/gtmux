import SwiftUI

struct HQWorkspaceComposer: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            Text(scopeLabel).font(Theme.Font.window).foregroundStyle(.secondary)
            HStack(alignment: .bottom, spacing: 10) {
                ZStack(alignment: .topLeading) {
                    if store.draft.isEmpty {
                        Text(l10n.tr("Describe your request…", "输入要求或问题…"))
                            .font(Theme.Font.session.weight(.regular)).foregroundStyle(.tertiary)
                            .padding(.horizontal, 12).padding(.top, 7).allowsHitTesting(false)
                    }
                    MultilineMessageField(
                        text: $store.draft, label: scopeLabel,
                        help: l10n.tr("Return for a new line; ⌘Return to send", "回车换行，⌘回车发送"),
                        canSend: { store.canSend }, onSend: { store.send() },
                        focusKey: store.selection.scope ?? "new-task")
                }.frame(height: 76).background(
                    Color(nsColor: .textBackgroundColor), in: RoundedRectangle(cornerRadius: 8)
                )
                .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(Color.primary.opacity(0.12)))
                Button(l10n.tr("Send to HQ", "发送给 HQ")) { store.send() }
                    .keyboardShortcut(.return, modifiers: [.command]).buttonStyle(.borderedProminent)
                    .disabled(!store.canSend)
            }
            if store.draft.utf8.count > 32768 {
                Text(
                    l10n.tr(
                        "This message is too long. Split it into shorter instructions.", "内容较长，请分成几条指令发送。")
                ).font(Theme.Font.window).foregroundStyle(Theme.Status.errored)
            }
            if store.currentMatter?.state == "closed" {
                Text(
                    l10n.tr("Continue following this matter to send another instruction.", "选择“继续跟进”后可发送新指令。")
                ).font(Theme.Font.window).foregroundStyle(.secondary)
            }
            if store.acting {
                Text(l10n.tr("Saving instruction…", "正在保存指令…")).font(Theme.Font.window).foregroundStyle(
                    .secondary)
            }
        }.padding(12)

    }
    private var scopeLabel: String {
        if let matter = store.currentMatter {
            return l10n.tr("Current matter: \(matter.title)", "当前事项：\(matter.title)")
        }
        if case .matter = store.selection {
            return l10n.tr("Matter unavailable · reload to continue", "事项暂不可用 · 请重新读取")
        }
        return store.selection == .newTask
            ? l10n.tr("New matter · Enter for a newline, ⌘Return to send", "新事项 · 回车换行，⌘回车发送")
            : l10n.tr("Global HQ conversation", "与 HQ 对话 · 不关联具体事项")
    }
}
