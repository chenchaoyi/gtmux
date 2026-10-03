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
    /// The shape under the pointer while dragging, drawn but not yet a mark.
    @Published var draft: Annotation?
    /// A text mark being typed, at its point in image space.
    @Published var textAt: CGPoint?
    @Published var textValue = ""
    @Published var note = ""
    @Published var targetID: String?
    @Published var status: ScreenshotStatus = .idle
    @Published var sendingTarget: ScreenshotSendTarget?

    init(doc: ScreenshotDocument, captureFile: URL, target: String?) {
        self.doc = doc
        self.captureFile = captureFile
        self.targetID = target
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
            doc.add(Annotation(.text(s, at: at), color: color))
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
/// a colour menu, the target menu or the save sheet taking focus must not close it.
final class ScreenshotEditorController: NSObject, NSWindowDelegate {
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

    func show(doc: ScreenshotDocument, captureFile: URL, target: String?, store: AgentStore, l10n: L10n) {
        close(discarding: true)
        let model = ScreenshotEditorModel(doc: doc, captureFile: captureFile, target: target)
        let screen = NSScreen.screens.first { NSMouseInRect(NSEvent.mouseLocation, $0.frame, false) } ?? NSScreen.main
        let visible = screen?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1280, height: 800)
        let display = ScreenshotLayout.displaySize(image: doc.pointSize, within: visible.size)
        let view = ScreenshotEditorView(model: model, store: store, l10n: l10n,
                                        displaySize: display,
                                        onSave: { [weak self] in self?.save() },
                                        onSend: { [weak self] in self?.send() },
                                        onCancel: { [weak self] in self?.requestClose() })
        let host = NSHostingController(rootView: view)
        let content = ScreenshotLayout.windowSize(display: display)
        let w = NSWindow(contentRect: NSRect(origin: .zero, size: content),
                         styleMask: [.titled, .closable, .miniaturizable, .resizable],
                         backing: .buffered, defer: false)
        w.title = l10n.tr("Screenshot", "截图")
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
    }

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
    static let chromeHeight: CGFloat = 176 // tool bar + target, note and buttons
    static let minWidth: CGFloat = 620

    /// The capture's size on screen: its real point size when it fits, scaled down to fit
    /// 85% of the screen otherwise. Never enlarged.
    static func displaySize(image: CGSize, within screen: CGSize) -> CGSize {
        guard image.width > 0, image.height > 0 else { return CGSize(width: 1, height: 1) }
        let maxW = screen.width * 0.85
        let maxH = max(screen.height * 0.85 - chromeHeight, 120)
        let k = min(1, maxW / image.width, maxH / image.height)
        return CGSize(width: floor(image.width * k), height: floor(image.height * k))
    }

    static func windowSize(display: CGSize) -> CGSize {
        CGSize(width: max(display.width + 32, minWidth), height: display.height + chromeHeight)
    }

    /// The default name in the save panel. It has spaces on purpose, like the system's own.
    static func saveName(now: Date = Date()) -> String {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyy-MM-dd 'at' HH.mm.ss"
        return "gtmux screenshot \(f.string(from: now)).png"
    }
}

struct ScreenshotEditorView: View {
    @ObservedObject var model: ScreenshotEditorModel
    @ObservedObject var store: AgentStore
    @ObservedObject var l10n: L10n
    let displaySize: CGSize
    let onSave: () -> Void
    let onSend: () -> Void
    let onCancel: () -> Void
    @FocusState private var textFocused: Bool

    var body: some View {
        VStack(spacing: 0) {
            toolbar
            Divider()
            canvas
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(Color(nsColor: .windowBackgroundColor))
            Divider()
            sendBar
        }
        .onExitCommand { model.textAt != nil ? model.cancelText() : onCancel() }
    }

    // MARK: tool bar

    private var toolbar: some View {
        HStack(spacing: 12) {
            Picker("", selection: $model.tool) {
                Image(systemName: "arrow.up.right").help(l10n.tr("Arrow", "箭头")).tag(AnnotationTool.arrow)
                Image(systemName: "rectangle").help(l10n.tr("Rectangle", "矩形")).tag(AnnotationTool.rect)
                Image(systemName: "textformat").help(l10n.tr("Text", "文字")).tag(AnnotationTool.text)
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .frame(width: 132)
            HStack(spacing: 6) {
                ForEach(AnnotationColor.allCases) { c in
                    Button { model.color = c } label: {
                        Circle().fill(Color(nsColor: c.nsColor)).frame(width: 16, height: 16)
                            .overlay(Circle().stroke(Color.primary.opacity(model.color == c ? 0.8 : 0), lineWidth: 2).padding(-3))
                    }
                    .buttonStyle(.plain)
                    .help(colorName(c))
                }
            }
            Button { model.doc.undoManager?.undo() } label: { Image(systemName: "arrow.uturn.backward") }
                .help(l10n.tr("Undo (⌘Z)", "撤销（⌘Z）"))
                .disabled(!model.doc.hasEdits && model.doc.undoManager?.canUndo != true)
            Button { model.doc.undoManager?.redo() } label: { Image(systemName: "arrow.uturn.forward") }
                .help(l10n.tr("Redo (⇧⌘Z)", "重做（⇧⌘Z）"))
            Spacer()
            // ⇧⌘C, not ⌘C: ⌘C has to keep copying text out of the note and the text field.
            Button(l10n.tr("Copy Image", "拷贝图片")) { model.copy() }
                .keyboardShortcut("c", modifiers: [.command, .shift])
                .help(l10n.tr("Copy the marked image (⇧⌘C)", "拷贝标注后的图片（⇧⌘C）"))
            Button(l10n.tr("Save…", "存储…")) { onSave() }
                .keyboardShortcut("s", modifiers: .command)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 8)
    }

    private func colorName(_ c: AnnotationColor) -> String {
        switch c {
        case .red: return l10n.tr("Red", "红")
        case .yellow: return l10n.tr("Yellow", "黄")
        case .blue: return l10n.tr("Blue", "蓝")
        }
    }

    // MARK: canvas

    /// Image points → view points.
    private var factor: CGFloat { model.doc.pointSize.width > 0 ? displaySize.width / model.doc.pointSize.width : 1 }

    private var canvas: some View {
        ZStack(alignment: .topLeading) {
            Image(decorative: model.doc.image, scale: model.doc.scale)
                .resizable()
                .interpolation(.high)
                .frame(width: displaySize.width, height: displaySize.height)
            // The same drawing code as the export, so what you see is what is copied or sent.
            Canvas { ctx, _ in
                ctx.withCGContext { cg in
                    cg.scaleBy(x: factor, y: factor)
                    for a in model.doc.items { AnnotationRenderer.draw(a, in: cg) }
                    if let d = model.draft { AnnotationRenderer.draw(d, in: cg) }
                }
            }
            .frame(width: displaySize.width, height: displaySize.height)
            .allowsHitTesting(false)
            if let at = model.textAt {
                TextField(l10n.tr("Type, then Return", "输入后按回车"), text: $model.textValue)
                    .textFieldStyle(.roundedBorder)
                    .font(.system(size: AnnotationRenderer.fontSize * factor, weight: .bold))
                    .frame(width: max(160, min(displaySize.width - at.x * factor, 360)))
                    .offset(x: at.x * factor, y: at.y * factor)
                    .focused($textFocused)
                    .onSubmit { model.commitText() }
                    .onExitCommand { model.cancelText() }
                    .onAppear { textFocused = true }
            }
        }
        .frame(width: displaySize.width, height: displaySize.height)
        .contentShape(Rectangle())
        .gesture(drag)
        .padding(16)
    }

    private var drag: some Gesture {
        DragGesture(minimumDistance: 0, coordinateSpace: .local)
            .onChanged { v in
                guard model.tool != .text else { return }
                let p = point(v.startLocation), q = point(v.location)
                model.draft = model.doc.shape(model.tool, from: p, to: q, color: model.color)
            }
            .onEnded { v in
                if model.tool == .text {
                    if model.textAt != nil { model.commitText() }
                    model.textAt = model.doc.clamp(point(v.location))
                    return
                }
                if let shape = model.doc.shape(model.tool, from: point(v.startLocation), to: point(v.location), color: model.color) {
                    model.doc.add(shape)
                }
                model.draft = nil
            }
    }

    private func point(_ p: CGPoint) -> CGPoint { CGPoint(x: p.x / factor, y: p.y / factor) }

    // MARK: send bar

    private var sendBar: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Text(l10n.tr("Send to", "发给")).foregroundStyle(.secondary)
                Picker("", selection: $model.targetID) {
                    if store.shareablePanes.isEmpty {
                        Text(l10n.tr("No agent panes", "没有 agent pane")).tag(String?.none)
                    }
                    ForEach(store.shareablePanes) { a in
                        Text(targetLabel(a)).tag(String?.some(a.paneID))
                    }
                }
                .labelsHidden()
                .frame(maxWidth: 360)
                .disabled(model.sending)
                if let a = store.shareablePanes.first(where: { $0.paneID == model.targetID }) {
                    StatusBadge(status: a.state, size: 14, errored: a.errored)
                }
                Spacer()
            }
            TextField(l10n.tr("Say what to look at (optional)", "说明要看哪里（可选）"), text: $model.note, axis: .vertical)
                .lineLimit(1...4)
                .textFieldStyle(.roundedBorder)
            HStack(spacing: 10) {
                statusLine
                Spacer()
                Button(l10n.tr("Cancel", "取消")) { onCancel() }
                Button(model.isRetry ? l10n.tr("Send Again", "再发一次") : l10n.tr("Send", "发送")) { onSend() }
                    .keyboardShortcut(.return, modifiers: .command)
                    .buttonStyle(.borderedProminent)
                    .disabled(model.sending || model.targetID == nil || sentOK)
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
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

    @ViewBuilder private var statusLine: some View {
        let s = ScreenshotStatusText.text(model.status, target: model.sendingTarget?.name ?? targetName, l10n: l10n)
        if !s.isEmpty {
            Text(s)
                .font(.system(size: 12))
                .foregroundStyle(ScreenshotStatusText.isProblem(model.status) ? Theme.Status.waiting : .secondary)
                .lineLimit(2)
                .help(s)
        }
    }

    private var targetName: String {
        store.shareablePanes.first { $0.paneID == model.targetID }.map { $0.agent.isEmpty ? $0.primary : $0.agent } ?? ""
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

    static func isProblem(_ s: ScreenshotStatus) -> Bool {
        switch s {
        case .error: return true
        case .result(let r): return !r.isSuccess
        default: return false
        }
    }
}
