import SwiftUI

/// One leading group survives split-column and loading changes without relocation.
struct HQWorkspaceToolbar: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    @ObservedObject var resources: HQResourceStore
    @Binding var columns: NavigationSplitViewVisibility
    var openTerminal: () -> Void

    private var loading: Bool {
        if let page = store.selection.resourcePage { return resources.refreshingPages.contains(page) }
        return store.loading
    }
    private var refreshing: Bool {
        if let page = store.selection.resourcePage { return loading && resources.loadedPages.contains(page) }
        return loading && store.readPhase == .content
    }

    var body: some View {
        HStack(spacing: 4) {
            Button {
                columns = columns == .detailOnly ? .all : .detailOnly
            } label: {
                Image(systemName: "sidebar.left").frame(width: 32, height: 28).contentShape(Rectangle())
            }
            .help(l10n.tr(columns == .detailOnly ? "Show sidebar" : "Hide sidebar",
                          columns == .detailOnly ? "显示侧边栏" : "隐藏侧边栏"))
            .accessibilityIdentifier("hq-toolbar-sidebar")
            .accessibilityLabel(l10n.tr("Toggle sidebar", "切换侧边栏"))
            .accessibilityValue(l10n.tr(columns == .detailOnly ? "Hidden" : "Visible",
                                       columns == .detailOnly ? "已隐藏" : "已显示"))
            Button { store.refreshCurrent() } label: {
                ZStack {
                    if refreshing { BrandProgressMark(size: 16) }
                    else { Image(systemName: "arrow.clockwise") }
                }.frame(width: 32, height: 28).contentShape(Rectangle())
            }
            .help(l10n.tr(loading ? "Refreshing HQ…" : "Refresh HQ", loading ? "正在刷新 HQ…" : "重新读取 HQ"))
            .accessibilityIdentifier("hq-toolbar-refresh")
            .accessibilityLabel(l10n.tr(loading ? "Refreshing HQ" : "Refresh HQ", loading ? "正在刷新 HQ" : "重新读取 HQ"))
            .disabled(store.acting || loading)
            Button(action: openTerminal) {
                Image(systemName: "terminal").frame(width: 32, height: 28).contentShape(Rectangle())
            }
            .help(l10n.tr("Open HQ in Terminal", "在终端打开 HQ"))
            .accessibilityIdentifier("hq-toolbar-terminal")
            .accessibilityLabel(l10n.tr("Open HQ in Terminal", "在终端打开 HQ"))
        }
        .buttonStyle(.borderless)
        .fixedSize()
    }
}
