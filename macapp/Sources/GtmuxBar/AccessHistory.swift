import AppKit
import SwiftUI

@MainActor final class AccessHistoryController {
    static let shared = AccessHistoryController()
    private var window: NSWindow?
    func show(l10n: L10n) {
        if window == nil {
            let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 720, height: 560),
                styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
            let host = NSHostingController(rootView: AccessHistoryView(l10n: l10n))
            host.sizingOptions = []
            w.contentViewController = host
            w.setContentSize(NSSize(width: 720, height: 560)); w.minSize = NSSize(width: 520, height: 380)
            w.isReleasedWhenClosed = false; w.center(); window = w
        }
        window?.title = l10n.tr("Access history", "访问历史")
        window?.makeKeyAndOrderFront(nil); NSApp.activate(ignoringOtherApps: true)
    }
}
struct AccessHistoryView: View {
    @ObservedObject var l10n: L10n
    @StateObject var store = AccessHistoryStore()
    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text(l10n.tr("External client activity", "外部客户端访问记录"))
                    .font(Theme.Font.session)
                Spacer()
                Text(store.query.groupBy == nil ? l10n.tr("\(store.entries.count) loaded", "已载入 \(store.entries.count) 条") : l10n.tr("\(store.groups.count) sources", "\(store.groups.count) 个来源"))
                    .font(Theme.Font.window).foregroundStyle(.secondary)
                Button(l10n.tr("Refresh", "刷新")) { store.load() }.disabled(store.loading)
            }
            AccessHistoryFilters(query: store.query, groups: store.known, operations: store.operations, zh: l10n.lang == "zh", change: store.select)
            if store.query.groupBy != nil {
                Text(l10n.tr("\(store.groups.reduce(0) { $0 + $1.count }) recorded activities" + (store.next == nil ? "" : " · Partial summary; continue below"),
                    "已汇总 \(store.groups.reduce(0) { $0 + $1.count }) 条已记录活动" + (store.next == nil ? "" : " · 部分结果，可继续汇总")))
                    .font(Theme.Font.window).foregroundStyle(.secondary)
            }
            if store.failed {
                HStack {
                    Text(store.unsupported ? l10n.tr("Update gtmux to view external access history.", "请更新 gtmux 后查看外部访问记录。") : l10n.tr("Could not load history. Loaded records are kept.", "无法载入访问历史，已载入的记录仍保留。"))
                        .font(Theme.Font.task).foregroundStyle(Theme.Status.errored)
                    Button(l10n.tr("Retry", "重试")) { store.load() }.disabled(store.loading)
                }
            }
            if store.loading && store.entries.isEmpty && store.groups.isEmpty {
                HQBrandLoading(label: l10n.tr("Loading access history…", "正在载入访问历史…"))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    if store.query.groupBy == "device" {
                        AccessHistoryGroups(groups: store.groups, zh: l10n.lang == "zh") { group in store.selectGroup(group) }
                    } else { AccessHistoryLog(entries: store.entries, zh: l10n.lang == "zh") }
                    if !store.failed && store.entries.isEmpty && store.groups.isEmpty {
                        Text(store.next == nil ? (store.query.hasFilters ? l10n.tr("No matching records. Clear filters to broaden the search.", "没有匹配记录，可清除筛选扩大范围。") : l10n.tr("No identified external access yet.", "暂无可识别的外部访问记录。"))
                            : l10n.tr("No matches in this page. Continue to earlier records.", "本页没有匹配记录，可继续载入更早记录。"))
                            .font(Theme.Font.task).foregroundStyle(.secondary).padding(.vertical, 28)
                    }
                    if store.next != nil {
                        Button(store.query.groupBy == nil ? l10n.tr("Load earlier", "载入更早记录") : l10n.tr("Continue summary", "继续汇总")) { store.load(earlier: true) }.disabled(store.loading)
                            .padding(.vertical, 8)
                    }
                }
            }
            HStack {
                Text(l10n.tr("Local time · Retained up to \(store.retainDays) days", "本地时间 · 最多保留 \(store.retainDays) 天"))
                Spacer()
                AccessHistoryHelp(zh: l10n.lang == "zh", retainDays: store.retainDays)
            }.font(Theme.Font.window).foregroundStyle(.secondary)
        }.padding(16).onAppear { store.load() }
    }
}
