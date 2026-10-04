import AppKit
import Combine
import SwiftUI

/// The drawing surface. AppKit rather than SwiftUI, for one reason: speed under the pointer.
///
/// The SwiftUI canvas kept the shape being dragged in a published property of the editor's
/// model. Every mouse event changed it, and every change re-rendered the whole window — the
/// tool bar, the send bar, and the capture itself, scaled down again from its full Retina
/// resolution — dozens of times a second. Drawing lagged behind the pointer.
///
/// Here the capture is a layer's contents, scaled to the screen once; the marks are a view of
/// their own, and a drag redraws that view and nothing else. Nothing the drag touches is
/// published, so SwiftUI does no work until the mark is added.
final class AnnotationCanvasView: NSView {
    let doc: ScreenshotDocument
    weak var model: ScreenshotEditorModel?
    var onCancel: () -> Void = {}
    let marks: MarksView
    private let imageLayer = CALayer()
    /// What the capture layer shows; the tests check a drag never replaces it.
    var imageContents: Any? { imageLayer.contents }
    private var dragStart: CGPoint?
    /// The points a line has passed so far.
    private var linePoints: [CGPoint] = []
    private var subscriptions: Set<AnyCancellable> = []
    private(set) var displaySize: CGSize
    /// The backing scale the capture was last scaled for.
    private var scaledFor: CGFloat = 0

    init(model: ScreenshotEditorModel, displaySize: CGSize) {
        self.doc = model.doc
        self.model = model
        self.displaySize = displaySize
        self.marks = MarksView(factor: AnnotationCanvasView.factor(doc: model.doc, display: displaySize),
                               source: AnnotationRenderer.Source(image: model.doc.image, scale: model.doc.scale))
        super.init(frame: NSRect(origin: .zero, size: displaySize))
        wantsLayer = true
        layer?.cornerRadius = 8
        layer?.masksToBounds = true
        imageLayer.frame = bounds
        imageLayer.contentsGravity = .resize
        layer?.addSublayer(imageLayer)
        marks.frame = bounds
        marks.autoresizingMask = [.width, .height]
        addSubview(marks)
        marks.items = doc.items
        // A mark added, undone or redone: redraw the marks, nothing else.
        doc.$items.sink { [weak self] items in self?.marks.items = items }.store(in: &subscriptions)
        model.$tool.sink { [weak self] _ in
            guard let self else { return }
            self.window?.invalidateCursorRects(for: self)
        }.store(in: &subscriptions)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override var isFlipped: Bool { true } // top-left origin, like the renderer and the export
    override var acceptsFirstResponder: Bool { true }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

    /// View points per image point.
    static func factor(doc: ScreenshotDocument, display: CGSize) -> CGFloat {
        doc.pointSize.width > 0 ? display.width / doc.pointSize.width : 1
    }
    var factor: CGFloat { Self.factor(doc: doc, display: displaySize) }

    func setDisplaySize(_ size: CGSize) {
        guard size != displaySize else { return }
        displaySize = size
        setFrameSize(size)
        marks.factor = factor
        scaledFor = 0
        rescaleIfNeeded()
    }

    override func layout() {
        super.layout()
        imageLayer.frame = bounds
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        rescaleIfNeeded()
        window?.invalidateCursorRects(for: self)
    }

    override func viewDidChangeBackingProperties() {
        super.viewDidChangeBackingProperties()
        rescaleIfNeeded()
    }

    /// The capture at the pixels this view occupies, made once per backing scale. Drawing it
    /// is then a composite of a texture, not a resample of a 5K image.
    private func rescaleIfNeeded() {
        let scale = window?.backingScaleFactor ?? NSScreen.main?.backingScaleFactor ?? 2
        guard scale != scaledFor else { return }
        scaledFor = scale
        imageLayer.contentsScale = scale
        imageLayer.contents = Self.scaled(doc.image, to: CGSize(width: displaySize.width * scale, height: displaySize.height * scale))
    }

    /// `image` resampled to `size` pixels with high-quality interpolation; the image itself
    /// when it is no larger.
    static func scaled(_ image: CGImage, to size: CGSize) -> CGImage {
        let w = Int(size.width.rounded()), h = Int(size.height.rounded())
        guard w > 0, h > 0, w < image.width || h < image.height,
              let space = CGColorSpace(name: CGColorSpace.sRGB),
              let ctx = CGContext(data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: 0,
                                  space: space, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
        else { return image }
        ctx.interpolationQuality = .high
        ctx.draw(image, in: CGRect(x: 0, y: 0, width: w, height: h))
        return ctx.makeImage() ?? image
    }

    // MARK: pointer

    override func resetCursorRects() {
        addCursorRect(bounds, cursor: model?.tool == .text ? .iBeam : .crosshair)
    }

    func imagePoint(_ event: NSEvent) -> CGPoint {
        let p = convert(event.locationInWindow, from: nil)
        return CGPoint(x: p.x / factor, y: p.y / factor)
    }

    override func mouseDown(with event: NSEvent) {
        window?.makeFirstResponder(self)
        guard let model, model.tool != .text else { return }
        dragStart = imagePoint(event)
        linePoints = [imagePoint(event)]
    }

    override func mouseDragged(with event: NSEvent) {
        guard let start = dragStart, let model else { return }
        if model.tool == .line {
            let p = imagePoint(event)
            if let last = linePoints.last, hypot(p.x - last.x, p.y - last.y) >= 0.5 / max(factor, 0.01) { linePoints.append(p) }
            marks.draft = doc.line(linePoints, color: model.color, width: model.width,
                                   straight: event.modifierFlags.contains(.shift))
            return
        }
        drag(from: start, to: imagePoint(event), tool: model.tool, color: model.color,
             width: model.width, square: event.modifierFlags.contains(.shift))
    }

    override func mouseUp(with event: NSEvent) {
        guard let model else { return }
        let p = imagePoint(event)
        if model.tool == .text {
            if model.textAt != nil { model.commitText() }
            model.textAt = doc.clamp(p)
            return
        }
        if model.tool == .line {
            if dragStart != nil {
                linePoints.append(p)
                if let a = doc.line(linePoints, color: model.color, width: model.width,
                                    straight: event.modifierFlags.contains(.shift)) { doc.add(a) }
            }
            dragStart = nil
            linePoints = []
            marks.draft = nil
            return
        }
        if let start = dragStart {
            finish(from: start, to: p, tool: model.tool, color: model.color,
                   width: model.width, square: event.modifierFlags.contains(.shift))
        }
    }

    /// One step of a drag: the shape so far, on the marks view only.
    func drag(from a: CGPoint, to b: CGPoint, tool: AnnotationTool, color: AnnotationColor,
              width: AnnotationWidth = .medium, square: Bool = false) {
        marks.draft = doc.shape(tool, from: a, to: b, color: color, width: width, square: square)
    }

    /// The end of a drag: the shape becomes a mark (an undoable change to the document).
    func finish(from a: CGPoint, to b: CGPoint, tool: AnnotationTool, color: AnnotationColor,
                width: AnnotationWidth = .medium, square: Bool = false) {
        if let shape = doc.shape(tool, from: a, to: b, color: color, width: width, square: square) { doc.add(shape) }
        dragStart = nil
        marks.draft = nil
    }

    // MARK: keys — single letters choose a tool, a colour or a width while the canvas has
    // focus; in the note or a text mark they type as usual.

    override func keyDown(with event: NSEvent) {
        guard let model, event.modifierFlags.intersection([.command, .control, .option]).isEmpty,
              let key = event.charactersIgnoringModifiers?.lowercased(),
              let action = Self.keyAction(key) else {
            super.keyDown(with: event)
            return
        }
        switch action {
        case .tool(let t): model.tool = t
        case .color(let c): model.color = c
        case .thinner: model.width = model.width.thinner
        case .thicker: model.width = model.width.thicker
        }
    }

    enum KeyAction: Equatable {
        case tool(AnnotationTool)
        case color(AnnotationColor)
        case thinner, thicker
    }

    static func keyAction(_ key: String) -> KeyAction? {
        switch key {
        case "a": return .tool(.arrow)
        case "l": return .tool(.line)
        case "r": return .tool(.rect)
        case "o": return .tool(.ellipse)
        case "m": return .tool(.mosaic)
        case "t": return .tool(.text)
        case "[": return .thinner
        case "]": return .thicker
        case "1": return .color(.red)
        case "2": return .color(.yellow)
        case "3": return .color(.blue)
        default: return nil
        }
    }

    override func cancelOperation(_ sender: Any?) {
        if model?.textAt != nil { model?.cancelText() } else { onCancel() }
    }
}

/// The marks over the capture, drawn by the export's own code so the screen and the file agree.
final class MarksView: NSView {
    var factor: CGFloat { didSet { needsDisplay = true } }
    var items: [Annotation] = [] { didSet { needsDisplay = true } }
    var draft: Annotation? { didSet { needsDisplay = true } }
    /// How many times the marks were drawn; the tests read it.
    private(set) var draws = 0
    /// The capture, for a mosaic's blocks.
    let source: AnnotationRenderer.Source?
    /// A finished mosaic's blocks, cut once: a mark never changes, and redrawing for a
    /// drag elsewhere must not resample the capture under every mosaic again.
    private var tiles: [UUID: CGImage] = [:]

    init(factor: CGFloat, source: AnnotationRenderer.Source? = nil) {
        self.factor = factor
        self.source = source
        super.init(frame: .zero)
    }

    private func tile(for a: Annotation) -> CGImage? {
        guard case .mosaic(let r) = a.kind, let source else { return nil }
        if let t = tiles[a.id] { return t }
        let t = AnnotationRenderer.mosaicTile(source, rect: r, block: a.width.block)
        tiles[a.id] = t
        return t
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override var isFlipped: Bool { true }
    override func hitTest(_ point: NSPoint) -> NSView? { nil } // the canvas takes the pointer

    override func draw(_ dirtyRect: NSRect) {
        draws += 1
        guard let cg = NSGraphicsContext.current?.cgContext else { return }
        cg.saveGState()
        cg.scaleBy(x: factor, y: factor)
        for a in items { AnnotationRenderer.draw(a, in: cg, source: source, tile: tile(for: a)) }
        // The draft changes on every drag event; its blocks are cut fresh each time.
        if let d = draft { AnnotationRenderer.draw(d, in: cg, source: source) }
        cg.restoreGState()
    }
}

/// The canvas in the SwiftUI window. It is made once; an update only passes a new size.
struct AnnotationCanvas: NSViewRepresentable {
    let model: ScreenshotEditorModel
    let displaySize: CGSize
    let onCancel: () -> Void

    func makeNSView(context: Context) -> AnnotationCanvasView {
        let v = AnnotationCanvasView(model: model, displaySize: displaySize)
        v.onCancel = onCancel
        return v
    }

    func updateNSView(_ v: AnnotationCanvasView, context: Context) {
        v.onCancel = onCancel
        v.setDisplaySize(displaySize)
    }
}
