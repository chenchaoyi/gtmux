import AppKit
import SwiftUI

final class WhatsNewController: NSObject, NSWindowDelegate {
    static let shared = WhatsNewController()
    private var window: NSWindow?

    func show(l10n: L10n, automatically: Bool = false) {
        let model = WhatsNewStore.shared
        if !automatically { model.showHistory() }
        if let window { window.makeKeyAndOrderFront(nil); NSApp.activate(ignoringOtherApps: true); return }
        let height = model.loading && model.report == nil ? 420 : min(520, max(300, 180 + model.entries.prefix(1).reduce(0) { $0 + $1.lines(lang: l10n.lang).count * 42 } + max(0, model.entries.count - 1) * 38))
        let frame = NSScreen.main?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1024, height: 768)
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: min(520, frame.width - 40), height: min(CGFloat(height), frame.height - 80)),
                         styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
        w.title = "gtmux"
        w.minSize = NSSize(width: 420, height: 260)
        w.isReleasedWhenClosed = false; w.delegate = self
        w.contentViewController = NSHostingController(rootView: WhatsNewView(model: model, l10n: l10n) { [weak self] in self?.window?.performClose(nil) })
        window = w; w.center(); w.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
    func windowWillClose(_ notification: Notification) {
        WhatsNewStore.shared.acknowledge()
        window = nil
    }
}

struct WhatsNewView: View {
    @ObservedObject var model: WhatsNewStore
    @ObservedObject var l10n: L10n
    var close: () -> Void
    @State private var expanded: Set<String> = []

    init(model: WhatsNewStore, l10n: L10n, close: @escaping () -> Void) {
        self.model = model; self.l10n = l10n; self.close = close
        _expanded = State(initialValue: Set(model.entries.prefix(1).map(\.version)))
    }

    private var grouped: Bool { model.entries.count > 1 || model.entries.first?.version != model.version }
    private var isUpgrade: Bool { if case .upgrade = model.scope { return true }; return false }

    var body: some View {
        GeometryReader { geometry in
        VStack(spacing: 0) {
            HStack(spacing: 10) {
                GtmuxLogo(size: 22)
                Text(l10n.tr("What's new", "更新内容")).font(Theme.Font.title)
                Spacer(minLength: 8)
                Text("v" + model.version).font(Theme.Font.mono).foregroundStyle(.secondary)
                    .help(l10n.tr("Installed version", "已安装版本"))
                Button { model.retry() } label: { Image(systemName: "arrow.clockwise") }
                    .buttonStyle(.borderless).disabled(model.loading)
                    .accessibilityLabel(l10n.tr("Refresh release notes", "刷新更新内容"))
                    .help(l10n.tr("Refresh release notes", "刷新更新内容"))
            }.padding(18)
            Divider()
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 14) {
                    if model.loading {
                        HStack(spacing: 8) {
                            ProgressView().controlSize(.small)
                            Text(l10n.tr("Loading release notes…", "正在加载更新内容…"))
                        }.foregroundStyle(.secondary)
                    }
                    if model.failed {
                        HStack(alignment: .top, spacing: 12) {
                            Text(model.report == nil ? l10n.tr("Release notes are unavailable. Please retry.", "暂时无法加载更新内容，请重试。") : l10n.tr("Could not refresh. Showing saved notes.", "更新内容未刷新，以下为缓存内容。"))
                                .foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                            Spacer(minLength: 0)
                            Button(l10n.tr("Retry", "重试")) { model.retry() }
                        }
                    }
                    if !model.loading && !model.failed && model.entries.isEmpty && !model.historyLimited {
                        Text(l10n.tr("No release notes for this version.", "此版本暂无更新说明。"))
                            .foregroundStyle(.secondary)
                    }
                    ForEach(model.entries) { note in
                        VStack(alignment: .leading, spacing: 8) {
                            if grouped {
                                Button {
                                    if expanded.contains(note.version) { expanded.remove(note.version) }
                                    else { expanded.insert(note.version) }
                                } label: {
                                    HStack(spacing: 8) {
                                        Text("v" + note.version).font(Theme.Font.mono)
                                        Spacer(minLength: 8)
                                        if !expanded.contains(note.version) {
                                            let count = note.lines(lang: l10n.lang).count
                                            Text(l10n.tr("\(count) \(count == 1 ? "change" : "changes")", "\(count) 项更新"))
                                                .font(Theme.Font.task).foregroundStyle(.secondary)
                                        }
                                        Image(systemName: expanded.contains(note.version) ? "chevron.down" : "chevron.right")
                                            .font(.system(size: 13, weight: .medium)).foregroundStyle(.secondary)
                                    }.frame(minHeight: 32).contentShape(Rectangle())
                                }.buttonStyle(.plain)
                                    .accessibilityLabel(l10n.tr("Release \(note.version)", "版本 \(note.version)"))
                                    .accessibilityValue(expanded.contains(note.version) ? l10n.tr("Expanded", "已展开") : l10n.tr("Collapsed", "已折叠"))
                            }
                            if !grouped || expanded.contains(note.version) {
                                ForEach(Array(note.lines(lang: l10n.lang).enumerated()), id: \.offset) { _, line in
                                    HStack(alignment: .firstTextBaseline, spacing: 10) {
                                        Text("•").foregroundStyle(.tertiary)
                                        Text(line).fixedSize(horizontal: false, vertical: true).textSelection(.enabled)
                                    }.frame(maxWidth: .infinity, alignment: .leading)
                                }
                            }
                        }
                    }
                    if model.historyLimited {
                        Text(l10n.tr("Only the latest 30 releases are available here. Earlier changes are in the release archive.", "此处仅展示最近 30 个发行版范围内的更新。更早的内容请查看发布记录。"))
                            .foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    }
                }.frame(width: max(0, geometry.size.width - 36), alignment: .leading).padding(18)
            }
            Divider()
            HStack(spacing: 12) {
                if isUpgrade {
                    Button(l10n.tr("Recent updates", "最近更新")) { model.showHistory() }.buttonStyle(.link)
                } else {
                    Link(l10n.tr("Release archive", "发布记录"), destination: URL(string: "https://github.com/chenchaoyi/gtmux/releases")!)
                        .help(l10n.tr("This window shows the latest 30 releases up to your installed version.", "此窗口展示最近 30 个发行版中，截至已安装版本的更新。"))
                }
                Spacer(minLength: 0)
                Button(isUpgrade ? l10n.tr("Got it", "知道了") : l10n.tr("Close", "关闭"), action: close)
                    .keyboardShortcut(.defaultAction)
            }.padding(.horizontal, 18).padding(.vertical, 12)
        }.font(Theme.Font.task)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
            .onAppear { expandNewest() }
            .onChange(of: model.entries) { _, _ in expandNewest() }
            .onExitCommand(perform: close)
    }
    private func expandNewest() { expanded = Set(model.entries.prefix(1).map(\.version)) }
}
