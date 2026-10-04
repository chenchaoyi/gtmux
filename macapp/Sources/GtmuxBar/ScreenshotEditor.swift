import AppKit
import SwiftUI

/// What the editor's status line says.
enum ScreenshotStatus: Equatable {
    case idle
    case copied
    case saved(String)
    case sending
    case result(ScreenshotSendResult)
    case error(String)
}

/// Where a send went, fixed when Send is pressed: the picker may change while the send is in
/// flight, and the result must name the pane it was actually for.
struct ScreenshotSendTarget: Equatable {
    let paneID: String
    let session: String
    let name: String
}

/// The editor's state: the capture, the tool, the note, the target and the send.
final class ScreenshotEditorModel: ObservableObject {
    let doc: ScreenshotDocument
    let captureFile: URL
    @Published var tool: AnnotationTool = .rect
    @Published var color: AnnotationColor = .red
    @Published var width: AnnotationWidth = .medium
    /// A text mark being typed, at its point in image space.
    @Published var textAt: CGPoint?
    @Published var textValue = ""
    @Published var note = ""
    @Published var targetID: String?
    @Published var status: ScreenshotStatus = .idle
    @Published var sendingTarget: ScreenshotSendTarget?
    /// The agent pane a terminal showed most recently, when there is one: the picker says so.
    let recentPane: String?

    init(doc: ScreenshotDocument, captureFile: URL, target: String?, recentPane: String? = nil) {
        self.doc = doc
        self.captureFile = captureFile
        self.targetID = target
        self.recentPane = recentPane
    }

    var sending: Bool { status == .sending }
    /// Work that Esc or the close button would throw away.
    var hasWork: Bool { doc.hasEdits || !note.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
    /// A delivery that may or may not have happened: the next Send is a retry, said as one.
    var isRetry: Bool {
        if case .result(.notConfirmed) = status { return true }
        if case .result(.failed) = status { return true }
        return false
    }

    func commitText() {
        let s = textValue.trimmingCharacters(in: .whitespacesAndNewlines)
        if let at = textAt, !s.isEmpty {
            doc.add(Annotation(.text(s, at: at), color: color, width: width))
        }
        textAt = nil
        textValue = ""
    }

    /// A text mark still being typed is part of the picture: Copy, Save and Send all take it.
    func commitPendingText() {
        if textAt != nil { commitText() }
    }

    func cancelText() {
        textAt = nil
        textValue = ""
    }

    /// The one export: what Copy, Save and Send all use.
    func flattened() -> CGImage? { AnnotationRenderer.flatten(doc) }

    func copy(to pasteboard: NSPasteboard = .general) {
        commitPendingText()
        guard let cg = flattened(), AnnotationRenderer.copy(cg, pointSize: doc.pointSize, to: pasteboard) else {
            status = .error("could not render the image")
            return
        }
        status = .copied
    }

    func write(to url: URL) -> Bool {
        commitPendingText()
        guard let cg = flattened(), let png = AnnotationRenderer.pngData(cg, pointSize: doc.pointSize) else { return false }
        do {
            try png.write(to: url, options: .atomic)
            return true
        } catch {
            return false
        }
    }
}

/// The annotation window. A plain titled window, not the palette's hide-on-resign panel:
/// a colour menu, the target menu or the save sheet taking focus must not close it. Its
/// unified title bar carries the brand mark and title, the capture's size, and Copy and
/// Save as icons.
final class ScreenshotEditorController: NSObject, NSWindowDelegate, NSToolbarDelegate {
    static let shared = ScreenshotEditorController()

    private(set) var window: NSWindow?
    private(set) var model: ScreenshotEditorModel?

    /// Whether `w` is still this controller's editor — a late callback from an earlier
    /// editor must not act on a newer one.
    func isCurrent(_ w: NSWindow?) -> Bool { w != nil && window === w }

    /// Open means the window exists, minimised or not: a second ⌥⌘4 must bring the work
    /// back, never start a capture that throws it away.
    var isOpen: Bool { window != nil }

    func bringToFront() {
        guard let w = window else { return }
        if w.isMiniaturized { w.deminiaturize(nil) }
        NSApp.activate(ignoringOtherApps: true)
        w.makeKeyAndOrderFront(nil)
    }

    func show(doc: ScreenshotDocument, captureFile: URL, target: String?, recentPane: String? = nil,
              store: AgentStore, l10n: L10n) {
        close(discarding: true)
        let model = ScreenshotEditorModel(doc: doc, captureFile: captureFile, target: target, recentPane: recentPane)
        let screen = NSScreen.screens.first { NSMouseInRect(NSEvent.mouseLocation, $0.frame, false) } ?? NSScreen.main
        let visible = screen?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1280, height: 800)
        let display = ScreenshotLayout.displaySize(image: doc.pointSize, within: visible.size)
        let view = ScreenshotEditorView(model: model, store: store, l10n: l10n,
                                        displaySize: display,
                                        onSend: { [weak self] in self?.send() },
                                        onCancel: { [weak self] in self?.requestClose() },
                                        onShowPane: { pane in GtmuxCLI.spawn(["focus", pane]) })
        let host = NSHostingController(rootView: view)
        // The window's title, subtitle and tool bar are AppKit's here; SwiftUI bridging them
        // would replace the subtitle with an empty navigation subtitle.
        host.sceneBridgingOptions = []
        let content = ScreenshotLayout.windowSize(display: display)
        let w = NSWindow(contentRect: NSRect(origin: .zero, size: content),
                         styleMask: [.titled, .closable, .miniaturizable, .resizable],
                         backing: .buffered, defer: false)
        w.title = ScreenshotLayout.title(l10n)
        w.subtitle = ScreenshotLayout.subtitle(pointSize: doc.pointSize, scale: doc.scale)
        let toolbar = NSToolbar(identifier: "gtmux.screenshot")
        toolbar.delegate = self
        toolbar.displayMode = .iconOnly
        toolbar.allowsUserCustomization = false
        w.toolbar = toolbar
        w.toolbarStyle = .unified
        // The brand mark before the title, so the window reads as gtmux's at a glance.
        let mark = NSTitlebarAccessoryViewController()
        mark.view = NSHostingView(rootView: GtmuxLogo(size: 18).padding(.leading, 4).frame(height: 28))
        mark.view.frame = NSRect(x: 0, y: 0, width: 26, height: 28)
        mark.layoutAttribute = .leading
        w.addTitlebarAccessoryViewController(mark)
        w.contentViewController = host
        w.setContentSize(content)
        w.isReleasedWhenClosed = false
        w.delegate = self
        w.setFrameOrigin(NSPoint(x: visible.midX - w.frame.width / 2, y: visible.midY - w.frame.height / 2))
        doc.undoManager = w.undoManager
        self.window = w
        self.model = model
        NSApp.activate(ignoringOtherApps: true)
        w.makeKeyAndOrderFront(nil)
        // The canvas takes the keys first, so A / R / T and 1 2 3 work before any click.
        DispatchQueue.main.async { [weak w] in
            guard let w, let canvas = Self.canvas(in: w.contentView) else { return }
            w.makeFirstResponder(canvas)
        }
    }

    static func canvas(in view: NSView?) -> AnnotationCanvasView? {
        guard let view else { return nil }
        if let c = view as? AnnotationCanvasView { return c }
        for sub in view.subviews { if let c = canvas(in: sub) { return c } }
        return nil
    }

    // MARK: tool bar — Copy and Save, with their keys

    static let copyItem = NSToolbarItem.Identifier("gtmux.screenshot.copy")
    static let saveItem = NSToolbarItem.Identifier("gtmux.screenshot.save")

    func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        [.flexibleSpace, Self.copyItem, Self.saveItem]
    }

    func toolbarAllowedItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        toolbarDefaultItemIdentifiers(toolbar)
    }

    func toolbar(_ toolbar: NSToolbar, itemForItemIdentifier id: NSToolbarItem.Identifier,
                 willBeInsertedIntoToolbar flag: Bool) -> NSToolbarItem? {
        let l = L10n.shared
        let copy = id == Self.copyItem
        guard copy || id == Self.saveItem else { return nil }
        let title = copy ? l.tr("Copy Image", "拷贝图片") : l.tr("Save…", "存储…")
        let symbol = copy ? "doc.on.doc" : "square.and.arrow.down"
        // Icons only: the symbols are the system's own for copy and save, and the tooltip
        // names each with its key. The words beside them said the same thing twice.
        let image = NSImage(systemSymbolName: symbol, accessibilityDescription: title) ?? NSImage()
        let button = NSButton(image: image, target: self, action: copy ? #selector(copyImage) : #selector(saveImage))
        button.title = "" // before the position: setting a title resets it
        button.imagePosition = .imageOnly
        button.bezelStyle = .toolbar
        button.setAccessibilityLabel(title)
        // ⇧⌘C, not ⌘C: ⌘C has to keep copying text out of the note and a text mark. An
        // NSButton spells Shift as the capital letter; a .shift in the mask is ignored, so
        // "c" + [.command, .shift] answered plain ⌘C and took the text copy over (#1291 M1).
        button.keyEquivalent = copy ? "C" : "s"
        button.keyEquivalentModifierMask = [.command]
        let item = NSToolbarItem(itemIdentifier: id)
        item.view = button
        item.label = title
        item.toolTip = copy ? l.tr("Copy the marked image (⇧⌘C)", "拷贝标注后的图片（⇧⌘C）")
                            : l.tr("Save as PNG (⌘S)", "存成 PNG（⌘S）")
        button.toolTip = item.toolTip
        return item
    }

    /// Where Copy puts the image; a test swaps in a private pasteboard.
    var pasteboard: NSPasteboard = .general
    @objc func copyImage() { model?.copy(to: pasteboard) }
    /// Replaces the save panel in a test, which cannot answer a sheet.
    var saveForTesting: (() -> Void)?
    @objc func saveImage() { if let t = saveForTesting { t() } else { save() } }

    // MARK: actions

    private func save() {
        guard let window, let model else { return }
        let panel = NSSavePanel()
        panel.allowedContentTypes = [.png]
        panel.canCreateDirectories = true
        panel.nameFieldStringValue = ScreenshotLayout.saveName()
        panel.beginSheetModal(for: window) { response in
            guard response == .OK, let url = panel.url else { return }
            model.status = model.write(to: url) ? .saved(url.path) : .error(url.path)
        }
    }

    private func send() {
        guard let model, !model.sending, let pane = model.targetID else { return }
        model.commitPendingText()
        guard let cg = model.flattened(), let png = AnnotationRenderer.pngData(cg, pointSize: model.doc.pointSize) else {
            model.status = .error("could not render the image")
            return
        }
        // A file of its own for the send: --attach copies it into the uploads dir under a
        // name taken from its content, so the same picture is the same message on a retry.
        let file = model.captureFile.deletingLastPathComponent().appendingPathComponent("screenshot.png")
        do {
            try png.write(to: file, options: .atomic)
        } catch {
            model.status = .error(error.localizedDescription)
            return
        }
        let agent = AgentStoreSnapshot.agent(pane)
        let target = ScreenshotSendTarget(
            paneID: pane, session: agent?.session ?? "",
            name: agent.map { $0.agent.isEmpty ? $0.primary : $0.agent } ?? pane)
        model.sendingTarget = target
        model.status = .sending
        let sentFrom = window
        ScreenshotSender.send(pane: target.paneID, session: target.session, note: model.note, png: file) { [weak self, weak model] result in
            guard let model else { return }
            model.status = .result(result)
            guard result.isSuccess else { return }
            if !target.session.isEmpty {
                ScreenshotTarget(paneID: target.paneID, session: target.session).save()
            }
            // Close THIS editor, not whichever one is open 1.2 s from now.
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) { [weak self, weak model] in
                guard let self, self.isCurrent(sentFrom), let model, self.model === model else { return }
                self.close(discarding: true)
            }
        }
    }

    private func requestClose() {
        guard let window else { return }
        if windowShouldClose(window) { close(discarding: true) }
    }

    func windowShouldClose(_ sender: NSWindow) -> Bool {
        guard let model, model.hasWork, !(model.status == .sending) else { return model?.sending != true }
        if case .result(let r) = model.status, r.isSuccess { return true }
        let alert = NSAlert()
        alert.messageText = L10n.shared.tr("Discard this screenshot?", "放弃这张截图？")
        alert.informativeText = L10n.shared.tr("Its marks and note will be lost.", "上面的标注和说明都会丢掉。")
        alert.addButton(withTitle: L10n.shared.tr("Discard", "放弃"))
        alert.addButton(withTitle: L10n.shared.tr("Keep Editing", "继续编辑"))
        return alert.runModal() == .alertFirstButtonReturn
    }

    func windowWillClose(_ notification: Notification) {
        cleanUp()
    }

    private func close(discarding: Bool) {
        guard let w = window else { return }
        w.delegate = nil
        w.orderOut(nil)
        cleanUp()
    }

    /// The capture lives in the app's temporary directory; the uploads dir keeps its own
    /// copy of what was sent, pruned with everything else there.
    private func cleanUp() {
        if let model {
            try? FileManager.default.removeItem(at: model.captureFile)
            try? FileManager.default.removeItem(at: model.captureFile.deletingLastPathComponent().appendingPathComponent("screenshot.png"))
        }
        window = nil
        model = nil
    }
}

/// The agent list as the store last read it, for remembering a target after a send.
enum AgentStoreSnapshot {
    static weak var store: AgentStore?
    static func agent(_ pane: String) -> Agent? { store?.agents.first { $0.paneID == pane } }
}

enum ScreenshotLayout {
    /// The capture's edge: light on the dark backdrop, dark on the light one.
    static let edge = NSColor(name: nil) { appearance in
        appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            ? NSColor(white: 1, alpha: 0.16)
            : NSColor(white: 0, alpha: 0.12)
    }
    /// The quiet surface the capture sits on: a step darker than the window, in either
    /// appearance (the system's under-page grey is too heavy in light mode).
    static let backdrop = NSColor(name: nil) { appearance in
        appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            ? NSColor(srgbRed: 0.14, green: 0.14, blue: 0.15, alpha: 1)
            : NSColor(srgbRed: 0.89, green: 0.89, blue: 0.91, alpha: 1)
    }
    /// Room above the capture for the floating tools, below it for the key hints (right under
    /// the capture's edge), and beside it.
    static let stageTop: CGFloat = 56
    static let hintGap: CGFloat = 8
    static let stageBottom: CGFloat = 30
    static let stageSide: CGFloat = 32
    /// The "Send to" row, tall enough for the chip and for two lines of status beside it.
    static let targetRowHeight: CGFloat = 32
    /// Everything in the content area that is not the capture: the stage's margins, the
    /// composer (target, note and Send, status).
    static let chromeHeight: CGFloat = stageTop + stageBottom + 1 + composerHeight
    /// The composer at its one, fixed height: the target row (with the status in it) and a
    /// two-line note (longer notes scroll inside it). Fixed, so the window holds no slack.
    static let composerHeight: CGFloat = 112
    /// The unified title bar with Copy and Save, outside the content area.
    static let titleBarHeight: CGFloat = 52
    static let minWidth: CGFloat = 680

    /// The capture's size on screen: its real point size when it fits, scaled down to fit
    /// 85% of the screen otherwise. Never enlarged.
    static func displaySize(image: CGSize, within screen: CGSize) -> CGSize {
        guard image.width > 0, image.height > 0 else { return CGSize(width: 1, height: 1) }
        let maxW = screen.width * 0.85
        let maxH = max(screen.height * 0.85 - chromeHeight - titleBarHeight, 120)
        let k = min(1, maxW / image.width, maxH / image.height)
        return CGSize(width: floor(image.width * k), height: floor(image.height * k))
    }

    static func windowSize(display: CGSize) -> CGSize {
        CGSize(width: max(display.width + 2 * stageSide, minWidth), height: display.height + chromeHeight)
    }

    /// The window's subtitle: the capture's size in points, and its scale when not 1×.
    static func subtitle(pointSize: CGSize, scale: CGFloat) -> String {
        let size = "\(Int(pointSize.width.rounded())) × \(Int(pointSize.height.rounded()))"
        return scale > 1.01 ? "\(size) · @\(Int(scale.rounded()))x" : size
    }

    /// The window's title: the product's name for this tool, not a generic "Screenshot".
    static func title(_ l10n: L10n) -> String { l10n.tr("gtmux shot", "gtmux 截图") }

    /// The default name in the save panel: short, sortable, and plainly gtmux's —
    /// `gtmux-shot-1004-210046.png`, month and day then the time to the second.
    static func saveName(now: Date = Date()) -> String {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "MMdd-HHmmss"
        return "gtmux-shot-\(f.string(from: now)).png"
    }
}

struct ScreenshotEditorView: View {
    @ObservedObject var model: ScreenshotEditorModel
    @ObservedObject var store: AgentStore
    @ObservedObject var l10n: L10n
    let displaySize: CGSize
    let onSend: () -> Void
    let onCancel: () -> Void
    let onShowPane: (String) -> Void
    @FocusState private var textFocused: Bool
    @State private var pickingTarget = false

    var body: some View {
        VStack(spacing: 0) {
            stage
            Divider()
            composer
        }
        .background(Color(nsColor: .windowBackgroundColor))
        .onExitCommand { model.textAt != nil ? model.cancelText() : onCancel() }
    }

    // MARK: stage — the capture on a quiet backdrop, the tools floating over its top edge

    private var stage: some View {
        ZStack(alignment: .top) {
            Color(nsColor: ScreenshotLayout.backdrop)
            // Pinned to the top, not centred, with the key hints right under the capture's
            // edge: a window made taller adds room below them, never between them.
            VStack(alignment: .leading, spacing: ScreenshotLayout.hintGap) {
                canvas
                Text(Self.hints(l10n))
                    .font(.system(size: 11.5))
                    .foregroundStyle(.tertiary)
                    .lineLimit(1)
                    .truncationMode(.tail)
                    .frame(width: displaySize.width, alignment: .leading)
            }
            .padding(.top, ScreenshotLayout.stageTop)
            .padding(.horizontal, ScreenshotLayout.stageSide)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
            ScreenshotToolPill(model: model, doc: model.doc, l10n: l10n)
                .padding(.top, 12)
        }
    }

    static func hints(_ l10n: L10n) -> String {
        l10n.tr("A arrow · R box · O oval · M mosaic · T text · 1–3 colour · [ ] width · ⌘Z undo · Esc close",
                "A 箭头 · R 矩形 · O 椭圆 · M 马赛克 · T 文字 · 1–3 颜色 · [ ] 粗细 · ⌘Z 撤销 · Esc 关闭")
    }

    private var factor: CGFloat { AnnotationCanvasView.factor(doc: model.doc, display: displaySize) }

    private var canvas: some View {
        ZStack(alignment: .topLeading) {
            AnnotationCanvas(model: model, displaySize: displaySize, onCancel: onCancel)
            if let at = model.textAt {
                TextField(l10n.tr("Type, then Return", "输入后按回车"), text: $model.textValue)
                    .textFieldStyle(.roundedBorder)
                    .font(.system(size: model.width.fontSize * factor, weight: .bold))
                    .frame(width: max(160, min(displaySize.width - at.x * factor, 360)))
                    .offset(x: at.x * factor, y: at.y * factor)
                    .focused($textFocused)
                    .onSubmit { model.commitText() }
                    .onExitCommand { model.cancelText() }
                    .onAppear { textFocused = true }
            }
        }
        .frame(width: displaySize.width, height: displaySize.height)
        // The shadow sits on a shape behind the canvas: an AppKit view does not take one.
        .background(
            RoundedRectangle(cornerRadius: 8, style: .continuous)
                .fill(Color.black)
                .shadow(color: .black.opacity(0.32), radius: 16, y: 8))
        // A hairline just outside the capture, behind it: a dark terminal screenshot on the dark
        // backdrop needs an edge, and a line outside can never cover a mark or move the canvas.
        .background(
            RoundedRectangle(cornerRadius: 9, style: .continuous)
                .strokeBorder(Color(nsColor: ScreenshotLayout.edge), lineWidth: 1)
                .padding(-1))
    }

    // MARK: composer — where it goes, what to say, Send

    private var composer: some View {
        VStack(alignment: .leading, spacing: 8) {
            // The status lives in this row, beside the target it is about, so the composer
            // keeps no empty band for it (it used to hold two blank lines under the note).
            HStack(spacing: 10) {
                Text(l10n.tr("Send to", "发给"))
                    .font(.system(size: 12.5))
                    .foregroundStyle(.secondary)
                targetChip
                if !statusText.isEmpty {
                    statusInline.frame(maxWidth: .infinity, alignment: .leading)
                } else {
                    if let hint = targetHint {
                        Text(hint).font(.system(size: 12)).foregroundStyle(.tertiary).lineLimit(1)
                    }
                    Spacer(minLength: 0)
                }
            }
            .frame(height: ScreenshotLayout.targetRowHeight)
            HStack(alignment: .bottom, spacing: 10) {
                TextField(l10n.tr("Say what to look at (optional)", "说明要看哪里（可选）"), text: $model.note, axis: .vertical)
                    .lineLimit(2, reservesSpace: true)
                    .textFieldStyle(.plain)
                    .font(.system(size: 13))
                    .padding(.horizontal, 12)
                    .padding(.vertical, 9)
                    .background(RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .fill(Color(nsColor: .textBackgroundColor)))
                    .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .strokeBorder(Color.primary.opacity(0.12)))
                Button(action: onSend) {
                    HStack(spacing: 8) {
                        Text(sendTitle).fontWeight(.semibold)
                        Text("⌘↩").opacity(0.7)
                    }
                    .padding(.horizontal, 4)
                }
                .keyboardShortcut(.return, modifiers: .command)
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .disabled(model.sending || model.targetID == nil || sentOK)
            }
        }
        .padding(.horizontal, 16)
        .padding(.top, 10)
        .frame(height: ScreenshotLayout.composerHeight, alignment: .top)
    }

    private var target: Agent? { store.shareablePanes.first { $0.paneID == model.targetID } }

    private var targetChip: some View {
        Button { pickingTarget = true } label: {
            HStack(spacing: 8) {
                if let a = target {
                    ScreenshotAgentMark(agent: a, size: 20)
                    Text(a.agent.isEmpty ? a.primary : a.agent).fontWeight(.semibold)
                    Text(a.secondary).foregroundStyle(.secondary)
                    StatusBadge(status: a.state, size: 13, errored: a.errored)
                } else {
                    Text(store.shareablePanes.isEmpty ? l10n.tr("No agent panes", "没有 agent pane")
                                                      : l10n.tr("Choose an agent", "选一个 agent"))
                        .foregroundStyle(.secondary)
                }
                Image(systemName: "chevron.down")
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(.tertiary)
            }
            .font(.system(size: 13))
            .padding(.leading, target == nil ? 10 : 5)
            .padding(.trailing, 9)
            .frame(height: 30)
            .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(Color.primary.opacity(0.06)))
            .overlay(RoundedRectangle(cornerRadius: 8, style: .continuous).strokeBorder(Color.primary.opacity(0.1)))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(model.sending || store.shareablePanes.isEmpty)
        .accessibilityLabel(l10n.tr("Send to", "发给") + " " + (target.map(targetLabel) ?? ""))
        .popover(isPresented: $pickingTarget, arrowEdge: .top) {
            ScreenshotTargetList(agents: store.shareablePanes, selected: model.targetID,
                                 recent: model.recentPane, l10n: l10n) { pane in
                model.targetID = pane
                pickingTarget = false
            }
        }
    }

    /// Why this target is the default, while it still is.
    private var targetHint: String? {
        guard let t = model.targetID, t == model.recentPane else { return nil }
        return l10n.tr("the pane you were just typing in", "你刚才在打字的 pane")
    }

    private var sendTitle: String {
        if model.isRetry { return l10n.tr("Send Again", "再发一次") }
        let name = target.map { $0.agent.isEmpty ? $0.primary : $0.agent } ?? ""
        return name.isEmpty ? l10n.tr("Send", "发送") : l10n.tr("Send to \(name)", "发给 \(name)")
    }

    private var sentOK: Bool {
        if case .result(let r) = model.status { return r.isSuccess }
        return false
    }

    private func targetLabel(_ a: Agent) -> String {
        let who = a.agent.isEmpty ? a.primary : a.agent
        let waiting = a.state == .waiting ? l10n.tr(" · waiting on you", " · 正在等你") : ""
        return "\(who) · \(a.session) \(a.paneID)\(waiting)"
    }

    // MARK: status — one line, said once

    private var statusText: String {
        ScreenshotStatusText.text(model.status, target: model.sendingTarget?.name ?? targetName, l10n: l10n)
    }

    /// Up to two lines in the target row's height; a longer one is cut short with its whole
    /// text in the tooltip. Nothing grows, so a status appearing never moves the capture.
    private var statusInline: some View {
        HStack(alignment: .center, spacing: 8) {
            statusIcon.frame(width: 14, height: 16)
            Text(statusText)
                .font(.system(size: 12))
                .foregroundStyle(ScreenshotStatusText.isProblem(model.status) ? Theme.Status.waiting : .secondary)
                .lineLimit(2)
                .truncationMode(.tail)
                .help(statusText)
            Spacer(minLength: 0)
            if let pane = ScreenshotStatusText.paneToShow(model.status, target: model.sendingTarget) {
                Button(l10n.tr("Show the pane", "去看那个 pane")) { onShowPane(pane) }
                    .controlSize(.small)
            }
        }
    }

    @ViewBuilder private var statusIcon: some View {
        switch model.status {
        case .sending:
            ProgressView().controlSize(.mini)
        case .copied, .saved, .result(.delivered):
            Image(systemName: "checkmark").font(.system(size: 12, weight: .semibold)).foregroundStyle(.secondary)
        case .result(.refusedWaiting), .result(.heldAfterPaste):
            StatusBadge(status: .waiting, size: 13)
        case .idle:
            EmptyView()
        default:
            Image(systemName: "exclamationmark.triangle.fill").font(.system(size: 12)).foregroundStyle(Theme.Status.waiting)
        }
    }

    private var targetName: String {
        target.map { $0.agent.isEmpty ? $0.primary : $0.agent } ?? ""
    }
}

/// The tools, floating over the top edge of the capture. It watches the document as well as
/// the model, so Undo and Redo follow the marks.
struct ScreenshotToolPill: View {
    @ObservedObject var model: ScreenshotEditorModel
    @ObservedObject var doc: ScreenshotDocument
    @ObservedObject var l10n: L10n

    var body: some View {
        HStack(spacing: 2) {
            tool(.arrow, "arrow.up.right", l10n.tr("Arrow", "箭头"), key: "A")
            tool(.rect, "rectangle", l10n.tr("Box", "矩形"), key: "R", hint: l10n.tr("hold ⇧ for a square", "按住 ⇧ 画正方形"))
            tool(.ellipse, "circle", l10n.tr("Oval", "椭圆"), key: "O", hint: l10n.tr("hold ⇧ for a circle", "按住 ⇧ 画圆"))
            tool(.mosaic, "checkerboard.rectangle", l10n.tr("Mosaic", "马赛克"), key: "M",
                 hint: l10n.tr("hides what is under it", "遮住下面的内容"))
            tool(.text, "textformat", l10n.tr("Text", "文字"), key: "T")
            divider
            ForEach(Array(AnnotationColor.allCases.enumerated()), id: \.element) { i, c in
                swatch(c, key: i + 1)
            }
            divider
            ForEach(AnnotationWidth.allCases) { w in weight(w) }
            divider
            icon("arrow.uturn.backward", help: l10n.tr("Undo (⌘Z)", "撤销（⌘Z）"),
                 enabled: doc.undoManager?.canUndo ?? false) { doc.undoManager?.undo() }
            icon("arrow.uturn.forward", help: l10n.tr("Redo (⇧⌘Z)", "重做（⇧⌘Z）"),
                 enabled: doc.undoManager?.canRedo ?? false) { doc.undoManager?.redo() }
        }
        .padding(5)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).strokeBorder(Color.primary.opacity(0.08)))
        .shadow(color: .black.opacity(0.2), radius: 10, y: 4)
    }

    private var divider: some View {
        Rectangle().fill(Color.primary.opacity(0.12)).frame(width: 1, height: 18).padding(.horizontal, 5)
    }

    private func tool(_ t: AnnotationTool, _ symbol: String, _ name: String, key: String, hint: String? = nil) -> some View {
        let on = model.tool == t
        return Button { model.tool = t } label: {
            Image(systemName: symbol)
                .font(.system(size: 14, weight: .medium))
                .foregroundStyle(on ? Color.primary : Color.secondary)
                .frame(width: 34, height: 28)
                .background(RoundedRectangle(cornerRadius: 7, style: .continuous)
                    .fill(Color.primary.opacity(on ? 0.14 : 0)))
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .help("\(name) (\(key))" + (hint.map { " — \($0)" } ?? ""))
        .accessibilityLabel(name)
        .accessibilityAddTraits(on ? .isSelected : [])
    }

    /// One of the three widths, drawn as a stroke that heavy.
    private func weight(_ w: AnnotationWidth) -> some View {
        let on = model.width == w
        return Button { model.width = w } label: {
            Capsule()
                .fill(on ? Color.primary : Color.secondary)
                .frame(width: 14, height: [AnnotationWidth.thin: 2, .medium: 3.5, .thick: 6][w] ?? 3)
                .frame(width: 26, height: 28)
                .background(RoundedRectangle(cornerRadius: 7, style: .continuous)
                    .fill(Color.primary.opacity(on ? 0.14 : 0)))
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .help("\(widthName(w)) ([ ])")
        .accessibilityLabel(widthName(w))
        .accessibilityAddTraits(on ? .isSelected : [])
    }

    private func widthName(_ w: AnnotationWidth) -> String {
        switch w {
        case .thin: return l10n.tr("Thin", "细")
        case .medium: return l10n.tr("Medium", "中")
        case .thick: return l10n.tr("Thick", "粗")
        }
    }

    private func swatch(_ c: AnnotationColor, key: Int) -> some View {
        let on = model.color == c
        return Button { model.color = c } label: {
            Circle().fill(Color(nsColor: c.nsColor))
                .frame(width: 15, height: 15)
                .overlay(Circle().strokeBorder(Color.primary.opacity(on ? 0.85 : 0), lineWidth: 1.5).padding(-3.5))
                .frame(width: 28, height: 28)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .help("\(colorName(c)) (\(key))")
        .accessibilityLabel(colorName(c))
        .accessibilityAddTraits(on ? .isSelected : [])
    }

    private func icon(_ symbol: String, help: String, enabled: Bool, _ action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: symbol)
                .font(.system(size: 13, weight: .medium))
                .frame(width: 30, height: 28)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .foregroundStyle(enabled ? Color.secondary : Color.secondary.opacity(0.4))
        .disabled(!enabled)
        .help(help)
    }

    private func colorName(_ c: AnnotationColor) -> String {
        switch c {
        case .red: return l10n.tr("Red", "红")
        case .yellow: return l10n.tr("Yellow", "黄")
        case .blue: return l10n.tr("Blue", "蓝")
        }
    }
}

/// An agent's mark at a small size: its icon, or its monogram.
struct ScreenshotAgentMark: View {
    let agent: Agent
    let size: CGFloat

    var body: some View {
        if let icon = AgentIcons.image(for: agent) {
            Image(nsImage: icon).resizable().interpolation(.high).scaledToFit()
                .frame(width: size, height: size)
                .clipShape(RoundedRectangle(cornerRadius: size * 0.25, style: .continuous))
        } else {
            RoundedRectangle(cornerRadius: size * 0.25, style: .continuous)
                .fill(Color.primary.opacity(0.08))
                .frame(width: size, height: size)
                .overlay(Text(agentMonogram(agent.agent))
                    .font(.system(size: size * 0.45, weight: .semibold, design: .rounded))
                    .foregroundStyle(.secondary))
        }
    }
}

/// The target picker: every agent pane with its status. The pane a terminal showed most
/// recently says so; a pane waiting on the user says so too, and can still be chosen.
struct ScreenshotTargetList: View {
    let agents: [Agent]
    let selected: String?
    let recent: String?
    @ObservedObject var l10n: L10n
    let pick: (String) -> Void

    var body: some View {
        ScrollView {
            VStack(spacing: 2) {
                ForEach(agents) { a in
                    Button { pick(a.paneID) } label: { row(a) }.buttonStyle(.plain)
                }
            }
            .padding(6)
        }
        .frame(width: 420)
        .frame(maxHeight: 360)
        .fixedSize(horizontal: false, vertical: true)
    }

    private func row(_ a: Agent) -> some View {
        let on = a.paneID == selected
        return HStack(spacing: 10) {
            ScreenshotAgentMark(agent: a, size: 22)
            Text(a.agent.isEmpty ? a.primary : a.agent).font(.system(size: 13, weight: .semibold))
            Text(a.secondary).font(.system(size: 12.5)).foregroundStyle(.secondary).lineLimit(1)
            Spacer(minLength: 8)
            if a.state == .waiting {
                Text(l10n.tr("waiting on you", "正在等你")).font(.system(size: 11.5)).foregroundStyle(Theme.Status.waiting)
            } else if a.paneID == recent {
                Text(l10n.tr("last typed in", "刚才在这里打字")).font(.system(size: 11.5)).foregroundStyle(.secondary)
            }
            StatusBadge(status: a.state, size: 13, errored: a.errored)
        }
        .padding(.horizontal, 10)
        .frame(height: 40)
        .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(Color.accentColor.opacity(on ? 0.18 : 0)))
        .contentShape(Rectangle())
        .accessibilityAddTraits(on ? .isSelected : [])
    }
}

/// The status line's words, in one place so the tests read the same strings.
enum ScreenshotStatusText {
    static func text(_ s: ScreenshotStatus, target: String, l10n: L10n) -> String {
        let who = target.isEmpty ? l10n.tr("the agent", "agent") : target
        switch s {
        case .idle: return ""
        case .copied: return l10n.tr("Copied to the clipboard.", "已拷贝到剪贴板。")
        case .saved(let path): return l10n.tr("Saved to \(path)", "已存到 \(path)")
        case .sending: return l10n.tr("Sending…", "正在发送…")
        case .error(let m): return l10n.tr("Something went wrong: \(m)", "出错了：\(m)")
        case .result(let r):
            switch r {
            case .delivered(queued: false): return l10n.tr("Sent to \(who).", "已发给 \(who)。")
            case .delivered(queued: true):
                return l10n.tr("Queued: \(who) reads it after its current turn.", "已排队：\(who) 当前这轮结束后会读到。")
            case .refusedWaiting:
                return l10n.tr("Not sent: \(who) is waiting for your decision. Answer it first.",
                               "没有发送：\(who) 正在等你做决定，先去回应它。")
            case .heldAfterPaste:
                return l10n.tr("Not submitted: \(who) started waiting for your decision. The note and image path may already be in its input box. Check it before sending again, or they go twice.",
                               "没有提交：\(who) 开始等你做决定了。说明和图片路径可能已经在它的输入框里，再发之前先看一眼，免得发两遍。")
            case .paneGone:
                return l10n.tr("Not sent: that pane is gone. Pick another.", "没有发送：那个 pane 已经不在了，换一个。")
            case .agentsUnreadable:
                return l10n.tr("Not sent: could not read the agent list, so the target was not checked. Try again.",
                               "没有发送：读不到 agent 列表，没法核对目标。再试一次。")
            case .refusedDraft:
                return l10n.tr("Not sent: \(who)'s input box has unsent text. Clear it, then send.",
                               "没有发送：\(who) 的输入框里有没提交的内容，清空后再发。")
            case .duplicate:
                return l10n.tr("Already sent: this screenshot and note reached \(who).",
                               "已经发过了：同样的截图和说明 \(who) 已收到。")
            case .notConfirmed(let e):
                return l10n.tr("Not confirmed. Look at the pane before sending again; it may have arrived. \(e)",
                               "未能确认送达。再发之前先看一眼 pane，可能已经到了。\(e)")
            case .failed(let m):
                return l10n.tr("Not sent: \(m)", "没有发送：\(m)")
            }
        }
    }

    /// The pane worth a look after this result, for the status line's "Show the pane": one that
    /// is asking the user, holds text that may not have been submitted, or was not confirmed.
    static func paneToShow(_ s: ScreenshotStatus, target: ScreenshotSendTarget?) -> String? {
        guard case .result(let r) = s, let target else { return nil }
        switch r {
        case .refusedWaiting, .heldAfterPaste, .notConfirmed: return target.paneID
        default: return nil
        }
    }

    static func isProblem(_ s: ScreenshotStatus) -> Bool {
        switch s {
        case .error: return true
        case .result(let r): return !r.isSuccess
        default: return false
        }
    }
}
