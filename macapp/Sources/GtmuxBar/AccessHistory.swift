import AppKit
import SwiftUI

struct AccessHistoryEntry: Decodable, Identifiable {
    let id: String
    let at: String
    let actor: String
    let name: String
    let scope: String
    let labelEn: String
    let labelZh: String
    let outcome: String
    var deviceId: String?
    var platform: String?
    var sourceIp: String?
    var method: String?
    var target: String?
    var status: Int?
    var durationMs: Int64?
    var operation: String?
}
struct AccessHistoryPage: Decodable {
    let entries: [AccessHistoryEntry]
    var next: String?
    let retainDays: Int
}

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
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top) {
                Text(l10n.tr("Device and browser requests to this server. Repeated reads are grouped over five minutes; changes are recorded individually.",
                    "此服务器收到的设备和浏览器请求。同类读取每 5 分钟记录一次，更改操作逐次记录。"))
                    .font(Theme.Font.task).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                Spacer()
                Button(l10n.tr("Refresh", "刷新")) { store.load() }.disabled(store.loading)
            }
            if store.failed {
                HStack {
                    Text(l10n.tr("Could not load history. Refresh to start again; existing records are kept below.", "无法载入访问历史。请刷新重试；已载入记录仍保留。"))
                        .font(Theme.Font.task).foregroundStyle(Theme.Status.errored)
                    Button(l10n.tr("Retry", "重试")) { store.load() }.disabled(store.loading)
                }
            }
            if store.loading && store.entries.isEmpty {
                HQBrandLoading(label: l10n.tr("Loading access history…", "正在载入访问历史…"))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 10) {
                        ForEach(store.entries) { entry in
                            VStack(alignment: .leading, spacing: 5) {
                                HStack(alignment: .firstTextBaseline) {
                                    Text(entry.scope == "master" ? l10n.tr("Owner credential", "所有者凭据") : entry.scope == "anonymous" ? l10n.tr("Unknown credential", "未知凭据") : entry.name).font(Theme.Font.session)
                                    Spacer()
                                    Text(timestamp(entry.at)).font(Theme.Font.window).foregroundStyle(.secondary)
                                }
                                HStack {
                                    Text(l10n.tr(entry.labelEn, entry.labelZh) + (entry.target.map { " · " + $0 } ?? ""))
                                    Spacer()
                                    Text(outcome(entry)).foregroundStyle(entry.outcome == "ok" ? Theme.Status.idle : Theme.Status.errored)
                                }.font(Theme.Font.task)
                                if let platform = entry.platform, !platform.isEmpty {
                                    Text(platform).font(Theme.Font.window).foregroundStyle(.secondary)
                                }
                                if let ip = entry.sourceIp, !ip.isEmpty {
                                    Text(l10n.tr("Source: ", "来源地址：") + ip).font(Theme.Font.window).foregroundStyle(.secondary).textSelection(.enabled)
                                }
                                if let id = entry.deviceId, !id.isEmpty {
                                    DisclosureGroup(l10n.tr("Device ID", "设备标识")) {
                                        Text(id).font(Theme.Font.window).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading)
                                    }.font(Theme.Font.window).foregroundStyle(.secondary)
                                }
                            }.padding(12).frame(maxWidth: .infinity, alignment: .leading)
                                .background(Color.primary.opacity(0.035), in: RoundedRectangle(cornerRadius: 9))
                        }
                        if store.entries.isEmpty {
                            Text(store.next == nil ? l10n.tr("No access history yet. Recording starts with this version.", "暂无访问历史，记录从此版本开始。")
                                : l10n.tr("No access records in this page. Continue to earlier records.", "本页未找到访问记录，可继续载入更早记录。"))
                                .font(Theme.Font.task).foregroundStyle(.secondary).padding(.vertical, 28)
                        }
                        if store.next != nil {
                            Button(l10n.tr("Load earlier", "载入更早记录")) { store.load(earlier: true) }.disabled(store.loading)
                        }
                    }
                }
            }
            Text(l10n.tr("Retained with server logs (\(store.retainDays) days, subject to the size limit). HTTP success means accepted, not agent completion. Owner credentials do not identify an individual device.",
                "随服务器日志保留 \(store.retainDays) 天，并受容量上限限制。HTTP 成功表示请求已接受，不表示 agent 已完成。所有者凭据无法区分具体设备。"))
                .font(Theme.Font.window).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
        }.padding(16).onAppear { store.load() }
    }
    private func timestamp(_ raw: String) -> String {
        let parser = ISO8601DateFormatter()
        parser.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        var date = parser.date(from: raw)
        if date == nil { parser.formatOptions = [.withInternetDateTime]; date = parser.date(from: raw) }
        return date?.formatted(date: .abbreviated, time: .standard) ?? raw
    }
    private func outcome(_ entry: AccessHistoryEntry) -> String {
        let text = entry.outcome == "ok" ? l10n.tr("Accepted", "已接受") : entry.outcome == "refused" ? l10n.tr("Refused", "已拒绝") : l10n.tr("Failed", "失败")
        return text + (entry.status.map { " · HTTP \($0)" } ?? "")
    }
}
