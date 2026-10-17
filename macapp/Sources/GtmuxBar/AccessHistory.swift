import AppKit
import SwiftUI

@MainActor final class AccessHistoryStore: ObservableObject {
    @Published private(set) var entries: [AccessHistoryEntry] = []
    @Published private(set) var loading = false
    @Published private(set) var failed = false
    @Published private(set) var next: String?
    @Published private(set) var retainDays = 30
    private let run: ([String]) -> (status: Int32, stdout: String, stderr: String)
    init(run: @escaping ([String]) -> (status: Int32, stdout: String, stderr: String) = { GtmuxCLI.captureBounded($0, timeout: 20) }) { self.run = run }
    func load(earlier: Bool = false) {
        guard !loading, !earlier || next != nil else { return }
        loading = true; failed = false
        var args = ["logs", "--access", "--json", "--limit", "100"]
        if earlier, let next { args += ["--before", next] }
        let run = run
        Task.detached(priority: .userInitiated) {
            let result = run(args)
            let decoder = JSONDecoder(); decoder.keyDecodingStrategy = .convertFromSnakeCase
            let page = result.status == 0 ? try? decoder.decode(AccessHistoryPage.self, from: Data(result.stdout.utf8)) : nil
            await MainActor.run {
                if let page {
                    let prior = earlier ? self.entries : []
                    var ids = Set(prior.map(\.id))
                    self.entries = prior + page.entries.filter { ids.insert($0.id).inserted }
                    self.next = page.next; self.retainDays = page.retainDays
                } else { self.failed = true }
                self.loading = false
            }
        }
    }
}

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
                Text(l10n.tr("Device and browser activity", "设备与浏览器的访问记录"))
                    .font(Theme.Font.session)
                Spacer()
                Text(l10n.tr("\(store.entries.count) loaded", "已载入 \(store.entries.count) 条"))
                    .font(Theme.Font.window).foregroundStyle(.secondary)
                Button(l10n.tr("Refresh", "刷新")) { store.load() }.disabled(store.loading)
            }
            if store.failed {
                HStack {
                    Text(l10n.tr("Could not load history. Loaded records are kept.", "无法载入访问历史，已载入的记录仍保留。"))
                        .font(Theme.Font.task).foregroundStyle(Theme.Status.errored)
                    Button(l10n.tr("Retry", "重试")) { store.load() }.disabled(store.loading)
                }
            }
            if store.loading && store.entries.isEmpty {
                HQBrandLoading(label: l10n.tr("Loading access history…", "正在载入访问历史…"))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    AccessHistoryLog(entries: store.entries, zh: l10n.lang == "zh")
                    if store.entries.isEmpty {
                        Text(store.next == nil ? l10n.tr("No access history yet.", "暂无访问记录。")
                            : l10n.tr("No matches in this page. Continue to earlier records.", "本页没有匹配记录，可继续载入更早记录。"))
                            .font(Theme.Font.task).foregroundStyle(.secondary).padding(.vertical, 28)
                    }
                    if store.next != nil {
                        Button(l10n.tr("Load earlier", "载入更早记录")) { store.load(earlier: true) }.disabled(store.loading)
                            .padding(.vertical, 8)
                    }
                }
            }
            HStack {
                Text(l10n.tr("Local time · Retained up to \(store.retainDays) days", "本地时间 · 最多保留 \(store.retainDays) 天"))
                Spacer()
                Text(l10n.tr("About these records", "记录说明"))
                    .help(l10n.tr("Reads are sampled every five minutes; changes and archive downloads are recorded individually. Capacity limits may shorten retention. HTTP acceptance is not agent completion. Shared owner credentials cannot identify a device.",
                        "读取每 5 分钟采样，更改操作及档案下载逐次记录。容量上限可能缩短保留时间。HTTP 成功不代表 agent 已完成；共用的所有者凭据无法区分具体设备。"))
            }.font(Theme.Font.window).foregroundStyle(.secondary)
        }.padding(16).onAppear { store.load() }
    }
}
