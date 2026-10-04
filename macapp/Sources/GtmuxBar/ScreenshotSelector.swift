import AppKit
import ScreenCaptureKit

/// gtmux's own region selector, in place of `screencapture -i`: it has a magnifier.
///
/// The system's selector has no way to magnify, so the corner of a box could only be placed
/// as precisely as the pointer could be seen. Here every display is frozen first (Screen
/// Recording, the permission the system tool already needed), and the frozen picture is
/// shown full screen under a gtmux-cyan crosshair with a loupe beside the pointer: the
/// pixels under it, magnified, the one under the hot spot outlined. Drag selects a region on
/// one display; Space switches to picking a whole window, as in the system tool; Esc cancels.
///
/// The selection is cut from the frozen picture, so what was selected is exactly what was
/// seen. A window is captured on its own, without the windows over it, as the system does.
/// When the freeze cannot be made, the system selector runs instead.
enum ScreenshotSelector {
    /// The lit pane of the brand mark (DESIGN §12), not a status: the selector says "gtmux".
    static let brand = Theme.Status.workingNS

    static func run(output: URL, completion: @escaping (CaptureOutcome) -> Void) {
        Task { @MainActor in
            let frozen: Freeze
            do {
                frozen = try await Freeze.make()
            } catch {
                DiagLog.act("act.screenshot.freeze", target: "", outcome: "failed", "fell back to the system selector",
                            ["error": error.localizedDescription])
                ScreenshotCapture.run(output: output, completion: completion)
                return
            }
            SelectionSession(freeze: frozen, output: output, completion: completion).begin()
        }
    }
}

// MARK: geometry — pure, so the tests can pin it

enum SelectionGeometry {
    /// The selection a drag makes, inside `bounds`, in the view's points (top-left origin).
    static func rect(from a: CGPoint, to b: CGPoint, in bounds: CGRect) -> CGRect {
        func clamp(_ p: CGPoint) -> CGPoint {
            CGPoint(x: min(max(p.x, bounds.minX), bounds.maxX), y: min(max(p.y, bounds.minY), bounds.maxY))
        }
        let p = clamp(a), q = clamp(b)
        return CGRect(x: min(p.x, q.x), y: min(p.y, q.y), width: abs(q.x - p.x), height: abs(q.y - p.y))
    }

    /// The pixels a selection covers: each edge rounded to the nearest pixel, inside the image.
    static func pixelRect(_ r: CGRect, scale: CGFloat, imageWidth: Int, imageHeight: Int) -> CGRect {
        let x0 = max(0, (r.minX * scale).rounded()), y0 = max(0, (r.minY * scale).rounded())
        let x1 = min(CGFloat(imageWidth), (r.maxX * scale).rounded())
        let y1 = min(CGFloat(imageHeight), (r.maxY * scale).rounded())
        return CGRect(x: x0, y: y0, width: max(0, x1 - x0), height: max(0, y1 - y0))
    }

    /// The pixel under a point.
    static func pixel(at p: CGPoint, scale: CGFloat) -> (x: Int, y: Int) {
        (Int((p.x * scale).rounded(.down)), Int((p.y * scale).rounded(.down)))
    }

    /// The `cells` × `cells` pixels the loupe shows, centred on the pixel under the pointer.
    /// It may reach past the image's edge; the loupe shows black there, so the centre cell
    /// is always the pixel under the hot spot.
    static func loupeSample(at p: CGPoint, scale: CGFloat, cells: Int) -> CGRect {
        let c = pixel(at: p, scale: scale), half = cells / 2
        return CGRect(x: c.x - half, y: c.y - half, width: cells, height: cells)
    }

    /// Where the loupe sits: below and right of the pointer, flipped to whichever side keeps
    /// it on the screen.
    static func loupeFrame(pointer p: CGPoint, size: CGSize, in bounds: CGSize, gap: CGFloat = 22) -> CGRect {
        var x = p.x + gap, y = p.y + gap
        if x + size.width > bounds.width { x = p.x - gap - size.width }
        if y + size.height > bounds.height { y = p.y - gap - size.height }
        return CGRect(x: max(0, x), y: max(0, y), width: size.width, height: size.height)
    }

    /// A rectangle in global Quartz coordinates (top-left of the main display), in a display's
    /// own top-left points.
    static func local(_ r: CGRect, displayBounds d: CGRect) -> CGRect {
        r.offsetBy(dx: -d.minX, dy: -d.minY)
    }

    /// The frontmost ordinary window under a global Quartz point, from a front-to-back list.
    static func window(at p: CGPoint, in windows: [WindowInfo], excludingPID pid: pid_t) -> WindowInfo? {
        windows.first { $0.layer == 0 && $0.pid != pid && $0.bounds.width >= 40 && $0.bounds.height >= 40 && $0.bounds.contains(p) }
    }

    /// The selection cut from a frozen display, at its full resolution.
    static func crop(_ image: CGImage, rect r: CGRect, scale: CGFloat) -> CGImage? {
        let px = pixelRect(r, scale: scale, imageWidth: image.width, imageHeight: image.height)
        guard px.width >= 1, px.height >= 1 else { return nil }
        return image.cropping(to: px)
    }
}

struct WindowInfo: Equatable {
    let id: CGWindowID
    let bounds: CGRect // global Quartz points
    let layer: Int
    let pid: pid_t
}

// MARK: the freeze — every display, and the windows in front-to-back order

struct FrozenDisplay {
    let screen: NSScreen
    let displayID: CGDirectDisplayID
    /// The display's rectangle in global Quartz coordinates.
    let bounds: CGRect
    let scale: CGFloat
    let image: CGImage
}

struct Freeze {
    let displays: [FrozenDisplay]
    let windows: [WindowInfo]
    let shareable: SCShareableContent

    @MainActor static func make() async throws -> Freeze {
        // The window order first: it is the order the pointer will pick from.
        let windows = Self.windowList()
        let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
        var displays: [FrozenDisplay] = []
        for screen in NSScreen.screens {
            guard let id = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? CGDirectDisplayID,
                  let display = content.displays.first(where: { $0.displayID == id }) else { continue }
            let scale = screen.backingScaleFactor
            let config = SCStreamConfiguration()
            config.width = Int((screen.frame.width * scale).rounded())
            config.height = Int((screen.frame.height * scale).rounded())
            config.showsCursor = false
            config.captureResolution = .best
            let image = try await SCScreenshotManager.captureImage(
                contentFilter: SCContentFilter(display: display, excludingWindows: []), configuration: config)
            displays.append(FrozenDisplay(screen: screen, displayID: id, bounds: CGDisplayBounds(id), scale: scale, image: image))
        }
        guard !displays.isEmpty else { throw NSError(domain: "gtmux.screenshot", code: 1, userInfo: [NSLocalizedDescriptionKey: "no display could be captured"]) }
        return Freeze(displays: displays, windows: windows, shareable: content)
    }

    static func windowList() -> [WindowInfo] {
        let raw = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
        return raw.compactMap { w in
            guard let id = w[kCGWindowNumber as String] as? CGWindowID,
                  let b = w[kCGWindowBounds as String] as? [String: CGFloat],
                  let layer = w[kCGWindowLayer as String] as? Int,
                  let pid = w[kCGWindowOwnerPID as String] as? pid_t else { return nil }
            return WindowInfo(id: id, bounds: CGRect(x: b["X"] ?? 0, y: b["Y"] ?? 0, width: b["Width"] ?? 0, height: b["Height"] ?? 0),
                              layer: layer, pid: pid)
        }
    }
}

// MARK: the session — one overlay window per display

/// What an overlay reports; the session acts on it, a test records it.
protocol SelectionViewDelegate: AnyObject {
    func pointerEntered(_ view: SelectionView)
    func mode(_ windowMode: Bool)
    func cancel()
    func select(_ rect: CGRect, on display: FrozenDisplay)
    func select(window info: WindowInfo, scale: CGFloat)
}

final class SelectionSession: SelectionViewDelegate {
    private let freeze: Freeze
    private let output: URL
    private let completion: (CaptureOutcome) -> Void
    private var windows: [NSWindow] = []
    private var views: [SelectionView] = []
    private var finished = false
    private let previousApp = NSWorkspace.shared.frontmostApplication
    /// Kept alive while the overlay is up.
    private static var current: SelectionSession?

    init(freeze: Freeze, output: URL, completion: @escaping (CaptureOutcome) -> Void) {
        self.freeze = freeze
        self.output = output
        self.completion = completion
    }

    func begin() {
        Self.current = self
        for d in freeze.displays {
            let w = SelectionWindow(contentRect: d.screen.frame, styleMask: .borderless, backing: .buffered, defer: false, screen: d.screen)
            w.setFrame(d.screen.frame, display: false)
            w.level = .screenSaver
            w.isOpaque = true
            w.backgroundColor = .black
            w.hasShadow = false
            w.acceptsMouseMovedEvents = true
            w.isReleasedWhenClosed = false
            // Over full-screen apps and every Space, as the system selector is.
            w.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .stationary, .ignoresCycle]
            let v = SelectionView(display: d, windows: freeze.windows, delegate: self)
            w.contentView = v
            windows.append(w)
            views.append(v)
        }
        NSApp.activate(ignoringOtherApps: true)
        for w in windows { w.orderFrontRegardless() }
        // The display under the pointer takes the keys (Esc, Space).
        let mouse = NSEvent.mouseLocation
        let start = windows.first { NSMouseInRect(mouse, $0.frame, false) } ?? windows.first
        start?.makeKey()
        for v in views { v.pointerMoved(toGlobal: mouse) }
    }

    /// The pointer crossed onto another display: it takes the keys, and the others forget it.
    func pointerEntered(_ view: SelectionView) {
        view.window?.makeKey()
        for v in views where v !== view { v.clearPointer() }
    }

    func mode(_ windowMode: Bool) {
        for v in views { v.windowMode = windowMode }
    }

    func cancel() {
        finish(.cancelled)
        previousApp?.activate()
    }

    func select(_ rect: CGRect, on display: FrozenDisplay) {
        guard let cut = SelectionGeometry.crop(display.image, rect: rect, scale: display.scale) else { return cancel() }
        let size = CGSize(width: CGFloat(cut.width) / display.scale, height: CGFloat(cut.height) / display.scale)
        write(cut, pointSize: size)
    }

    /// A whole window, on its own (no windows over it, no shadow), as the system's Space does.
    func select(window info: WindowInfo, scale: CGFloat) {
        close()
        guard let scWindow = freeze.shareable.windows.first(where: { $0.windowID == info.id }) else {
            return report(.failed("the window is gone"))
        }
        let config = SCStreamConfiguration()
        config.width = Int((info.bounds.width * scale).rounded())
        config.height = Int((info.bounds.height * scale).rounded())
        config.showsCursor = false
        config.ignoreShadowsSingleWindow = true
        config.captureResolution = .best
        Task { @MainActor in
            do {
                let image = try await SCScreenshotManager.captureImage(
                    contentFilter: SCContentFilter(desktopIndependentWindow: scWindow), configuration: config)
                self.write(image, pointSize: info.bounds.size)
            } catch {
                self.report(.failed(error.localizedDescription))
            }
        }
    }

    private func write(_ image: CGImage, pointSize: CGSize) {
        close()
        guard let png = AnnotationRenderer.pngData(image, pointSize: pointSize) else {
            return report(.failed("could not encode the capture"))
        }
        do {
            try png.write(to: output, options: .atomic)
            report(.image(output))
        } catch {
            report(.failed(error.localizedDescription))
        }
    }

    private func finish(_ outcome: CaptureOutcome) {
        close()
        report(outcome)
    }

    private func close() {
        for w in windows { w.orderOut(nil) }
        windows.removeAll()
        views.removeAll()
    }

    private func report(_ outcome: CaptureOutcome) {
        guard !finished else { return }
        finished = true
        Self.current = nil
        completion(outcome)
    }
}

/// A borderless window that still takes the keyboard.
final class SelectionWindow: NSWindow {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }
}

// MARK: the overlay — layers only, so a pointer move redraws nothing on the CPU

final class SelectionView: NSView {
    let display: FrozenDisplay
    private let windows: [WindowInfo]
    private weak var session: SelectionViewDelegate?

    static let loupeCells = 15
    static let loupeSize = CGSize(width: 135, height: 135) // 9 points per pixel cell

    private let picture = CALayer()
    private let dim = CAShapeLayer()
    private let guides = CAShapeLayer()
    private let outline = CAShapeLayer()
    private let loupe = CALayer()
    private let loupePixels = CALayer()
    private let loupeGrid = CAShapeLayer()
    private let loupeCenter = CAShapeLayer()
    private let readout = CATextLayer()
    /// What the readout says, and what the loupe shows; the tests read them.
    var readoutText: String { (readout.string as? String) ?? "" }
    var loupeShowsPixels: Bool { !loupe.isHidden && loupePixels.contents != nil }

    private(set) var pointer: CGPoint?
    private(set) var dragStart: CGPoint?
    private(set) var selection: CGRect?
    private(set) var hoveredWindow: WindowInfo?
    var windowMode = false { didSet { refresh() } }

    init(display: FrozenDisplay, windows: [WindowInfo], delegate: SelectionViewDelegate?) {
        self.display = display
        self.windows = windows
        self.session = delegate
        super.init(frame: NSRect(origin: .zero, size: display.screen.frame.size))
        let root = CALayer()
        root.isGeometryFlipped = true // sublayers in the view's top-left points; contents stay upright
        layer = root
        wantsLayer = true
        let scale = display.scale
        picture.contents = display.image
        picture.contentsScale = scale
        picture.frame = bounds
        picture.contentsGravity = .resize
        dim.fillRule = .evenOdd
        dim.fillColor = NSColor(white: 0, alpha: 0.32).cgColor
        guides.strokeColor = ScreenshotSelector.brand.withAlphaComponent(0.75).cgColor
        guides.lineWidth = 1
        outline.strokeColor = ScreenshotSelector.brand.cgColor
        outline.fillColor = nil
        outline.lineWidth = 1.5
        // The loupe: a dark rounded square, the pixels without smoothing, a faint grid, and
        // the centre pixel in brand cyan.
        loupe.backgroundColor = NSColor.black.cgColor
        loupe.cornerRadius = 10
        loupe.masksToBounds = true
        loupe.borderColor = NSColor(white: 1, alpha: 0.9).cgColor
        loupe.borderWidth = 1.5
        loupePixels.magnificationFilter = .nearest
        loupeGrid.strokeColor = NSColor(white: 1, alpha: 0.10).cgColor
        loupeGrid.lineWidth = 0.5
        loupeCenter.strokeColor = ScreenshotSelector.brand.cgColor
        loupeCenter.fillColor = nil
        loupeCenter.lineWidth = 1.5
        loupe.addSublayer(loupePixels)
        loupe.addSublayer(loupeGrid)
        loupe.addSublayer(loupeCenter)
        readout.fontSize = 11
        readout.font = NSFont.monospacedDigitSystemFont(ofSize: 11, weight: .medium)
        readout.foregroundColor = NSColor.white.cgColor
        readout.backgroundColor = NSColor(white: 0, alpha: 0.72).cgColor
        readout.cornerRadius = 5
        readout.alignmentMode = .center
        readout.contentsScale = scale
        for l in [picture, dim, guides, outline, loupe, readout] as [CALayer] {
            l.contentsScale = scale
            root.addSublayer(l)
        }
        for l in [loupePixels, loupeGrid, loupeCenter] as [CALayer] { l.contentsScale = scale }
        buildLoupeGrid()
        refresh()
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override var isFlipped: Bool { true }
    override var acceptsFirstResponder: Bool { true }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        for t in trackingAreas { removeTrackingArea(t) }
        addTrackingArea(NSTrackingArea(rect: bounds, options: [.mouseMoved, .mouseEnteredAndExited, .activeAlways, .inVisibleRect, .cursorUpdate],
                                       owner: self, userInfo: nil))
    }

    override func cursorUpdate(with event: NSEvent) { Self.cursor.set() }
    override func resetCursorRects() { addCursorRect(bounds, cursor: Self.cursor) }

    /// A thin crosshair in brand cyan with a dark edge, readable on any background.
    static let cursor: NSCursor = {
        let side: CGFloat = 23, mid = side / 2
        let img = NSImage(size: NSSize(width: side, height: side), flipped: false) { _ in
            for (color, width) in [(NSColor(white: 0, alpha: 0.6), CGFloat(3)), (ScreenshotSelector.brand, CGFloat(1))] {
                color.setStroke()
                let p = NSBezierPath()
                p.lineWidth = width
                p.move(to: NSPoint(x: mid, y: 0)); p.line(to: NSPoint(x: mid, y: mid - 3))
                p.move(to: NSPoint(x: mid, y: mid + 3)); p.line(to: NSPoint(x: mid, y: side))
                p.move(to: NSPoint(x: 0, y: mid)); p.line(to: NSPoint(x: mid - 3, y: mid))
                p.move(to: NSPoint(x: mid + 3, y: mid)); p.line(to: NSPoint(x: side, y: mid))
                p.stroke()
            }
            return true
        }
        return NSCursor(image: img, hotSpot: NSPoint(x: mid, y: mid))
    }()

    // MARK: pointer

    private func local(_ event: NSEvent) -> CGPoint { convert(event.locationInWindow, from: nil) }

    /// The pointer at a global Cocoa location (the session's first frame).
    func pointerMoved(toGlobal g: NSPoint) {
        guard let w = window, NSMouseInRect(g, w.frame, false) else { return clearPointer() }
        let inWindow = w.convertPoint(fromScreen: g)
        track(convert(inWindow, from: nil))
    }

    func clearPointer() {
        pointer = nil
        refresh()
    }

    override func mouseMoved(with event: NSEvent) { track(local(event)) }
    override func mouseEntered(with event: NSEvent) {
        session?.pointerEntered(self)
        track(local(event))
    }

    private func track(_ p: CGPoint) {
        pointer = p
        if windowMode { hoveredWindow = windowUnder(p) }
        refresh()
    }

    override func mouseDown(with event: NSEvent) {
        window?.makeKey()
        let p = local(event)
        pointer = p
        if windowMode { return }
        dragStart = p
        selection = nil
        refresh()
    }

    override func mouseDragged(with event: NSEvent) {
        let p = local(event)
        pointer = p
        if let s = dragStart { selection = SelectionGeometry.rect(from: s, to: p, in: bounds) }
        refresh()
    }

    override func mouseUp(with event: NSEvent) {
        if windowMode {
            if let w = hoveredWindow ?? windowUnder(local(event)) { session?.select(window: w, scale: display.scale) }
            return
        }
        defer { dragStart = nil }
        guard let r = selection, r.width >= 4, r.height >= 4 else {
            // A click without a drag selects nothing: still selecting, as the system does.
            selection = nil
            refresh()
            return
        }
        session?.select(r, on: display)
    }

    override func keyDown(with event: NSEvent) {
        switch event.keyCode {
        case 53: session?.cancel() // Esc
        case 49 where dragStart == nil: session?.mode(!windowMode) // Space: region ↔ window
        default: super.keyDown(with: event)
        }
    }

    override func cancelOperation(_ sender: Any?) { session?.cancel() }

    private func windowUnder(_ p: CGPoint) -> WindowInfo? {
        let global = CGPoint(x: p.x + display.bounds.minX, y: p.y + display.bounds.minY)
        return SelectionGeometry.window(at: global, in: windows, excludingPID: ProcessInfo.processInfo.processIdentifier)
    }

    // MARK: drawing — layer properties only

    private func buildLoupeGrid() {
        let n = Self.loupeCells, size = Self.loupeSize, cell = size.width / CGFloat(n)
        let grid = CGMutablePath()
        for i in 1..<n {
            let v = CGFloat(i) * cell
            grid.move(to: CGPoint(x: v, y: 0)); grid.addLine(to: CGPoint(x: v, y: size.height))
            grid.move(to: CGPoint(x: 0, y: v)); grid.addLine(to: CGPoint(x: size.width, y: v))
        }
        loupeGrid.path = grid
        loupeGrid.frame = CGRect(origin: .zero, size: size)
        loupeCenter.frame = CGRect(origin: .zero, size: size)
        let c = CGFloat(n / 2) * cell
        loupeCenter.path = CGPath(rect: CGRect(x: c, y: c, width: cell, height: cell), transform: nil)
    }

    func refresh() {
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        defer { CATransaction.commit() }
        picture.frame = bounds
        let hole: CGRect? = windowMode
            ? hoveredWindow.map { SelectionGeometry.local($0.bounds, displayBounds: display.bounds) }
            : selection
        let dimPath = CGMutablePath()
        dimPath.addRect(bounds)
        if let h = hole { dimPath.addRect(h) }
        dim.path = dimPath
        dim.frame = bounds
        outline.frame = bounds
        outline.path = hole.map { CGPath(rect: $0, transform: nil) }
        guard let p = pointer else {
            guides.path = nil
            loupe.isHidden = true
            readout.isHidden = true
            return
        }
        // Guides across the whole display through the pointer, in region mode.
        let g = CGMutablePath()
        if !windowMode {
            g.move(to: CGPoint(x: 0, y: p.y.rounded() + 0.5)); g.addLine(to: CGPoint(x: bounds.width, y: p.y.rounded() + 0.5))
            g.move(to: CGPoint(x: p.x.rounded() + 0.5, y: 0)); g.addLine(to: CGPoint(x: p.x.rounded() + 0.5, y: bounds.height))
        }
        guides.path = g
        guides.frame = bounds
        // The loupe, beside the pointer.
        loupe.isHidden = windowMode
        let frame = SelectionGeometry.loupeFrame(pointer: p, size: Self.loupeSize, in: bounds.size)
        loupe.frame = frame
        let sample = SelectionGeometry.loupeSample(at: p, scale: display.scale, cells: Self.loupeCells)
        let img = CGRect(x: 0, y: 0, width: display.image.width, height: display.image.height)
        let visible = sample.intersection(img)
        let cell = Self.loupeSize.width / CGFloat(Self.loupeCells)
        if !visible.isNull, let crop = display.image.cropping(to: visible) {
            loupePixels.contents = crop
            loupePixels.frame = CGRect(x: (visible.minX - sample.minX) * cell, y: (visible.minY - sample.minY) * cell,
                                       width: visible.width * cell, height: visible.height * cell)
        } else {
            loupePixels.contents = nil
        }
        // The readout under the loupe: where the pointer is, or how big the selection is.
        readout.isHidden = false
        let text: String
        if let s = selection, dragStart != nil {
            text = "\(Int(s.width.rounded())) × \(Int(s.height.rounded()))"
        } else if windowMode, let w = hoveredWindow {
            text = "\(Int(w.bounds.width)) × \(Int(w.bounds.height))"
        } else {
            text = "\(Int(p.x.rounded())), \(Int(p.y.rounded()))"
        }
        readout.string = text
        let width = max(64, CGFloat(text.count) * 7.2 + 14)
        var label = CGRect(x: frame.midX - width / 2, y: frame.maxY + 6, width: width, height: 18)
        if label.maxY > bounds.height { label.origin.y = frame.minY - 24 }
        readout.frame = label
    }
}
