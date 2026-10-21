import AppKit
import SwiftUI

/// Fit measured prose inside the display, including the native title bar.
enum WhatsNewWindowLayout {
    static func contentHeight(desired: CGFloat, visible: CGFloat, titleBar: CGFloat) -> CGFloat {
        let ceiling = max(1, min(visible * 0.85, visible - 48) - titleBar)
        return min(ceiling, max(220, ceil(desired)))
    }
}

/// Escape also works before a selectable SwiftUI text element has focus.
private final class WhatsNewWindow: NSWindow {
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        if event.keyCode == 53, event.modifierFlags.intersection([.command, .option, .control, .shift]).isEmpty {
            performClose(nil)
            return true
        }
        return super.performKeyEquivalent(with: event)
    }
}

final class WhatsNewController: NSObject, NSWindowDelegate {
    static let shared = WhatsNewController()
    private var window: NSWindow?
    private let model: WhatsNewStore
    private var measuredHeight: CGFloat = 0
    private var fitScheduled = false
    private var manuallySized = false

    init(model: WhatsNewStore = .shared) { self.model = model }

    @discardableResult
    func show(l10n: L10n, automatically: Bool = false) -> NSWindow {
        if !automatically { model.showHistory() }
        if let window { window.makeKeyAndOrderFront(nil); NSApp.activate(ignoringOtherApps: true); return window }
        let visible = NSScreen.main?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1024, height: 768)
        let width = min(520, max(1, visible.width - 48))
        let w = WhatsNewWindow(contentRect: NSRect(x: 0, y: 0, width: width, height: 320),
                         styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
        let titleBar = w.frame.height - w.contentRect(forFrameRect: w.frame).height
        w.title = l10n.tr("What's new — gtmux", "更新内容 — gtmux")
        w.titleVisibility = .hidden
        w.titlebarAppearsTransparent = true
        w.contentMinSize = NSSize(width: min(420, width), height: WhatsNewWindowLayout.contentHeight(desired: 220, visible: visible.height, titleBar: titleBar))
        w.isReleasedWhenClosed = false; w.delegate = self
        manuallySized = false; measuredHeight = 0; fitScheduled = false
        let host = NSHostingController(rootView: WhatsNewView(model: model, l10n: l10n, close: { [weak self] in
            self?.window?.performClose(nil)
        }, contentHeightChanged: { [weak self, weak w] height in
            guard let self, let w, self.window === w else { return }
            self.scheduleFit(height, window: w)
        }))
        // The controller sizes the window; SwiftUI reports only its measured content.
        host.sizingOptions = []
        w.contentViewController = host
        window = w; w.center(); w.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        return w
    }

    private func scheduleFit(_ height: CGFloat, window: NSWindow) {
        guard height.isFinite, height > 0 else { return }
        measuredHeight = height
        guard !fitScheduled else { return }
        fitScheduled = true
        DispatchQueue.main.async { [weak self, weak window] in
            guard let self, let window, self.window === window else { return }
            self.fitScheduled = false
            guard !self.manuallySized else { return }
            let visible = window.screen?.visibleFrame ?? NSScreen.main?.visibleFrame ?? window.frame
            let titleBar = window.frame.height - window.contentRect(forFrameRect: window.frame).height
            let height = WhatsNewWindowLayout.contentHeight(desired: self.measuredHeight, visible: visible.height, titleBar: titleBar)
            guard abs(window.contentRect(forFrameRect: window.frame).height - height) > 1 else { return }
            var frame = window.frame
            let top = frame.maxY
            frame.size.height = height + titleBar
            frame.origin.y = min(visible.maxY - frame.height, max(visible.minY, top - frame.height))
            window.setFrame(frame, display: true)
        }
    }

    func windowWillStartLiveResize(_ notification: Notification) { manuallySized = true }
    func windowShouldZoom(_ window: NSWindow, toFrame newFrame: NSRect) -> Bool { manuallySized = true; return true }
    func windowWillClose(_ notification: Notification) {
        model.acknowledge()
        window = nil; fitScheduled = false
    }
}

struct WhatsNewView: View {
    @ObservedObject var model: WhatsNewStore
    @ObservedObject var l10n: L10n
    var close: () -> Void
    var contentHeightChanged: (CGFloat) -> Void
    @State private var expanded: Set<String> = []
    @State private var chromeHeight: CGFloat = 0
    @State private var bodyHeight: CGFloat = 0

    init(model: WhatsNewStore, l10n: L10n, close: @escaping () -> Void,
         contentHeightChanged: @escaping (CGFloat) -> Void = { _ in }) {
        self.model = model; self.l10n = l10n; self.close = close
        self.contentHeightChanged = contentHeightChanged
        _expanded = State(initialValue: Set(model.entries.prefix(1).map(\.version)))
    }

    private var grouped: Bool { model.entries.count > 1 }
    private var isUpgrade: Bool { if case .upgrade = model.scope { return true }; return false }

    var body: some View {
        GeometryReader { geometry in
        VStack(spacing: 0) {
            HStack(spacing: 9) {
                GtmuxLogo(size: 19)
                Text(l10n.tr("What's new", "更新内容")).font(.system(size: 16, weight: .semibold))
                Spacer(minLength: 8)
                if !grouped, let note = model.entries.first {
                    Text("v" + note.version).font(Theme.Font.mono).foregroundStyle(.secondary)
                        .help(l10n.tr("Release notes for version \(note.version)", "版本 \(note.version) 的更新说明"))
                        .accessibilityIdentifier("whatsnew-header-version")
                }
                Button { model.retry() } label: { Image(systemName: "arrow.clockwise") }
                    .buttonStyle(.borderless).controlSize(.small).foregroundStyle(.secondary).disabled(model.loading)
                    .accessibilityLabel(l10n.tr("Refresh release notes", "刷新更新内容"))
                    .help(l10n.tr("Refresh release notes", "刷新更新内容"))
            }.padding(.horizontal, 18).padding(.vertical, 12)
                .background(GeometryReader { Color.clear.preference(key: ChromeHeight.self, value: $0.size.height) })
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
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
                        VStack(alignment: .leading, spacing: 10) {
                            if grouped {
                                Button {
                                    if expanded.contains(note.version) { expanded.remove(note.version) }
                                    else { expanded.insert(note.version) }
                                } label: {
                                    HStack(spacing: 8) {
                                        Text("v" + note.version).font(Theme.Font.mono)
                                        Rectangle().fill(.quaternary).frame(height: 0.5)
                                        if !expanded.contains(note.version) {
                                            let count = note.lines(lang: l10n.lang).count
                                            Text(l10n.tr("\(count) \(count == 1 ? "change" : "changes")", "\(count) 项更新"))
                                                .font(Theme.Font.mono).foregroundStyle(.tertiary)
                                        }
                                        Image(systemName: expanded.contains(note.version) ? "chevron.down" : "chevron.right")
                                            .font(.system(size: 12, weight: .medium)).foregroundStyle(.tertiary)
                                    }.frame(minHeight: 24).contentShape(Rectangle())
                                }.buttonStyle(.plain).foregroundStyle(.secondary)
                                    .accessibilityIdentifier("whatsnew-version-" + note.version)
                                    .accessibilityLabel(l10n.tr("Release \(note.version)", "版本 \(note.version)"))
                                    .accessibilityValue(expanded.contains(note.version) ? l10n.tr("Expanded", "已展开") : l10n.tr("Collapsed", "已折叠"))
                            }
                            if !grouped || expanded.contains(note.version) {
                                ForEach(Array(note.lines(lang: l10n.lang).enumerated()), id: \.offset) { index, line in
                                    HStack(alignment: .top, spacing: 9) {
                                        RoundedRectangle(cornerRadius: 1).fill(.tertiary).frame(width: 4, height: 4).padding(.top, 7)
                                        Text(line).fixedSize(horizontal: false, vertical: true).textSelection(.enabled)
                                            .frame(maxWidth: .infinity, alignment: .leading)
                                            .accessibilityIdentifier("whatsnew-line-" + note.version + "-" + String(index))
                                    }.frame(maxWidth: .infinity, alignment: .leading)
                                }
                            }
                        }
                    }
                    if model.historyLimited {
                        Text(l10n.tr("Only the latest 30 releases are available here. Earlier changes are in the release archive.", "此处仅展示最近 30 个发行版范围内的更新。更早的内容请查看发布记录。"))
                            .foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    }
                }.frame(width: max(0, geometry.size.width - 36), alignment: .leading).padding(.horizontal, 18).padding(.vertical, 14)
                    .background(GeometryReader { Color.clear.preference(key: ListContentHeight.self, value: $0.size.height) })
            }.accessibilityIdentifier("whatsnew-scroll")
            HStack(spacing: 12) {
                if isUpgrade {
                    Button(l10n.tr("Recent updates", "最近更新")) { model.showHistory() }.buttonStyle(.plain)
                } else {
                    Link(l10n.tr("Release archive", "发布记录"), destination: URL(string: "https://github.com/chenchaoyi/gtmux/releases")!)
                        .help(l10n.tr("This window shows the latest 30 releases up to your installed version.", "此窗口展示最近 30 个发行版中，截至已安装版本的更新。"))
                }
                Spacer(minLength: 0)
                Button(isUpgrade ? l10n.tr("Got it", "知道了") : l10n.tr("Close", "关闭"), action: close)
                    .buttonStyle(.plain).font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(.primary)
                    .frame(minWidth: 64, minHeight: 30)
                    .accessibilityIdentifier("whatsnew-dismiss")
                    .keyboardShortcut(.defaultAction)
            }.font(Theme.Font.action).foregroundStyle(.secondary)
                .padding(.horizontal, 18).padding(.vertical, 6)
                .overlay(alignment: .top) { Divider() }
                .background(GeometryReader { Color.clear.preference(key: ChromeHeight.self, value: $0.size.height) })
        }.font(.system(size: 13)).lineSpacing(3)
            .background(Color(nsColor: .windowBackgroundColor))
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
            .onPreferenceChange(ChromeHeight.self) { height in
                chromeHeight = height; contentHeightChanged(height + bodyHeight)
            }
            .onPreferenceChange(ListContentHeight.self) { height in
                bodyHeight = height; contentHeightChanged(height + chromeHeight)
            }
            .onAppear { expandNewest() }
            .onChange(of: model.entries.first?.version) { _, _ in expandNewest() }
            .onExitCommand(perform: close)
    }
    private func expandNewest() { expanded = Set(model.entries.prefix(1).map(\.version)) }
}
