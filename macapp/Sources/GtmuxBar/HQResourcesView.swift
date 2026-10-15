import SwiftUI

/// Feature content hosted by the workspace; navigation and lifecycle belong to its owner.
struct HQResourcesView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQResourceStore
    let page: HQResourcePage
    var onOverview: (() -> Void)?
    @Environment(\.colorScheme) private var scheme
    var body: some View {
        VStack(spacing: 0) {
            HStack {
                if page == .board, let onOverview {
                    Button(action: onOverview) { Image(systemName: "chevron.left") }
                        .help(l10n.tr("Back to Overview", "返回总览"))
                        .accessibilityLabel(l10n.tr("Back to Overview", "返回总览"))
                }
                Label(page.title(l10n), systemImage: page.symbol).font(Theme.Font.title)
                Spacer()
                if store.refreshingPages.contains(page), store.loadedPages.contains(page) {
                    HQBrandLoading(label: l10n.tr("Refreshing", "正在刷新"), compact: true)
                }
            }.padding(16)
            Divider()
            if store.failedPages.contains(page) {
                HStack {
                    Label(
                        l10n.tr("Couldn’t refresh. Try again.", "读取失败，请重试。"),
                        systemImage: "exclamationmark.triangle"
                    )
                    .font(Theme.Font.task).foregroundStyle(Theme.Status.errored)
                    Spacer()
                    Button(l10n.tr("Retry", "重试")) { store.refresh() }
                }.padding(12)
            }
            if !store.loadedPages.contains(page), !store.failedPages.contains(page) || store.refreshingPages.contains(page) {
                HQBrandLoading(label: l10n.tr("Loading \(page.title(l10n))…", "正在载入\(page.title(l10n))…"))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if store.failedPages.contains(page), !store.loadedPages.contains(page) {
                HQResourceEmpty(l10n.tr("Data unavailable", "暂无可读取的数据"), Theme.Palette.of(scheme))
            } else {
                switch page {
                case .board: HQBoardView(l10n: l10n, store: store)
                case .knowledge:
                    HQKnowledgeView(l10n: l10n, store: store, navigation: store.knowledgeNavigation)
                case .machine: HQMachineView(l10n: l10n, store: store)
                case .usage: HQUsageView(l10n: l10n, store: store, navigation: store.usageNavigation)
                }
            }
        }.frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

struct HQResourceEmpty: View {
    let text: String
    let p: Theme.Palette
    init(_ text: String, _ p: Theme.Palette) {
        self.text = text
        self.p = p
    }
    var body: some View {
        VStack {
            Spacer()
            Text(text).font(Theme.Font.task).foregroundStyle(p.fg3)
                .multilineTextAlignment(.center).padding(24)
            Spacer()
        }.frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
