import SwiftUI

struct HQWorkspaceSidebar: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore

    var body: some View {
        VStack(spacing: 0) {
            Button {
                store.select(.newTask)
            } label: {
                Label(l10n.tr("New task", "交代任务"), systemImage: "plus").frame(maxWidth: .infinity)
            }.controlSize(.large).padding(12).keyboardShortcut("n", modifiers: [.command])
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 12) {
                    VStack(spacing: 2) {
                        navigationRow(l10n.tr("Overview", "总览"), symbol: "square.grid.2x2", selection: .overview)
                        navigationRow(l10n.tr("Talk to HQ", "与 HQ 对话"), symbol: "bubble.left", selection: .conversation)
                        navigationRow(l10n.tr("HQ activity", "HQ 运行记录"), symbol: "clock", selection: .activity)
                    }
                    Divider()
                    ForEach(HQMatterGroup.allCases) { group in
                        let matters = group.matters(in: store.snapshot)
                        if !matters.isEmpty {
                            category(group.title(l10n), count: matters.count, section: .matters(group)) {
                                ForEach(matters) { matter in
                                    Button {
                                        store.select(.matter(matter.id))
                                    } label: {
                                        HStack(spacing: 8) {
                                            StatusBadge(status: group.status, size: 16).accessibilityHidden(true)
                                            Text(matter.title).font(HQTypography.sidebarItem).lineLimit(2)
                                                .multilineTextAlignment(.leading)
                                            Spacer(minLength: 0)
                                        }.padding(9).frame(maxWidth: .infinity, minHeight: 36, alignment: .leading)
                                            .background(
                                                selectionBackground(.matter(matter.id)),
                                                in: RoundedRectangle(cornerRadius: 7))
                                    }.buttonStyle(.plain)
                                        .accessibilityIdentifier("hq-sidebar-matter-\(matter.id)")
                                        .accessibilityLabel("\(matter.title), \(matter.stateLabel(l10n))")
                                        .accessibilityAddTraits(
                                            store.selection == .matter(matter.id) ? .isSelected : [])
                                }
                            }
                        }
                    }
                    Divider()
                    category(
                        l10n.tr("Resources", "资料与用量"), count: HQResourcePage.sidebarPages.count, section: .resources
                    ) {
                        ForEach(HQResourcePage.sidebarPages, id: \.self) { page in
                            navigationRow(page.title(l10n), symbol: page.symbol, selection: .resource(page))
                                .accessibilityIdentifier("hq-sidebar-resource-\(page.rawValue)")
                        }
                    }
                }.padding(10)
            }.background(Color(nsColor: .controlBackgroundColor))
        }
    }

    private func category<Content: View>(
        _ title: String, count: Int, section: HQSidebarSection,
        @ViewBuilder content: () -> Content
    ) -> some View {
        let expanded = !store.collapsedSections.contains(section)
        return VStack(alignment: .leading, spacing: 2) {
            Button {
                store.toggleSection(section)
            } label: {
                HStack(spacing: 6) {
                    Image(systemName: expanded ? "chevron.down" : "chevron.right")
                        .font(HQTypography.rowTitle).frame(width: 16)
                    Text(title).font(HQTypography.sidebarGroup).foregroundStyle(.primary)
                    Spacer(minLength: 0)
                    Text("\(count)").font(HQTypography.caption.monospacedDigit())
                }.foregroundStyle(.secondary).padding(.horizontal, 8)
                    .frame(maxWidth: .infinity, minHeight: 36).contentShape(Rectangle())
            }.buttonStyle(.plain)
                .accessibilityIdentifier("hq-sidebar-section-\(section.id)")
                .accessibilityLabel("\(title), \(count)")
                .accessibilityValue(l10n.tr(expanded ? "Expanded" : "Collapsed", expanded ? "已展开" : "已收起"))
            if expanded {
                VStack(alignment: .leading, spacing: 2) { content() }
                    .padding(.leading, 20)
                    .accessibilityElement(children: .contain)
            }
        }
    }

    private func selectionBackground(_ selection: HQWorkspaceSelection) -> Color {
        store.selection == selection ? Color.primary.opacity(0.08) : .clear
    }
    private func navigationRow(_ title: String, symbol: String, selection: HQWorkspaceSelection) -> some View {
        Button {
            store.select(selection)
        } label: {
            Label(title, systemImage: symbol).font(HQTypography.navigation)
                .frame(maxWidth: .infinity, minHeight: 18, alignment: .leading).padding(9)
                .background(selectionBackground(selection), in: RoundedRectangle(cornerRadius: 7))
        }.buttonStyle(.plain).accessibilityAddTraits(store.selection == selection ? .isSelected : [])
    }
}
