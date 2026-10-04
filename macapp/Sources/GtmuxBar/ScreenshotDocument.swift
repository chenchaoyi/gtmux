import AppKit
import CoreGraphics

enum AnnotationTool: String, CaseIterable, Identifiable {
    case arrow, line, rect, ellipse, mosaic, text
    var id: String { rawValue }
}

/// How heavy a mark is: the stroke of a line or an outline, the size of text, and the size
/// of a mosaic's blocks, all from one choice.
enum AnnotationWidth: String, CaseIterable, Identifiable {
    case thin, medium, thick
    var id: String { rawValue }
    /// Stroke width in image points; a Retina capture gets twice the pixels, the same look.
    var stroke: CGFloat {
        switch self {
        case .thin: return 2
        case .medium: return 4
        case .thick: return 7
        }
    }
    var fontSize: CGFloat {
        switch self {
        case .thin: return 14
        case .medium: return 18
        case .thick: return 24
        }
    }
    /// A mosaic block's side in image points. Even the smallest hides body text.
    var block: CGFloat {
        switch self {
        case .thin: return 8
        case .medium: return 12
        case .thick: return 18
        }
    }
    var thinner: AnnotationWidth { self == .thick ? .medium : .thin }
    var thicker: AnnotationWidth { self == .thin ? .medium : .thick }
}

/// Three marking colours. User content, not status: DESIGN's "colour means state" rule is
/// about the app's chrome, and a red box on a screenshot is the convention people expect.
enum AnnotationColor: String, CaseIterable, Identifiable {
    case red, yellow, blue
    var id: String { rawValue }
    var nsColor: NSColor {
        switch self {
        case .red: return NSColor(srgbRed: 0.94, green: 0.27, blue: 0.27, alpha: 1)
        case .yellow: return NSColor(srgbRed: 0.98, green: 0.80, blue: 0.08, alpha: 1)
        case .blue: return NSColor(srgbRed: 0.15, green: 0.47, blue: 0.96, alpha: 1)
        }
    }
}

struct Annotation: Equatable, Identifiable {
    enum Kind: Equatable {
        case arrow(from: CGPoint, to: CGPoint)
        /// A line drawn by hand: the pointer's path, or two points for a straight one.
        case path([CGPoint])
        case rect(CGRect)
        case ellipse(CGRect)
        /// The capture under the rectangle, in blocks: for what must not be read.
        case mosaic(CGRect)
        case text(String, at: CGPoint)
    }
    let id: UUID
    var kind: Kind
    var color: AnnotationColor
    var width: AnnotationWidth

    init(_ kind: Kind, color: AnnotationColor, width: AnnotationWidth = .medium, id: UUID = UUID()) {
        self.id = id
        self.kind = kind
        self.color = color
        self.width = width
    }
}

/// One capture and its marks. Marks are in the image's POINT space (pixels / scale), the
/// space the editor draws in; the renderer maps them to pixels on export.
final class ScreenshotDocument: ObservableObject {
    let image: CGImage
    /// The image's size in points, from its DPI (a Retina capture is 144 dpi: 2 pixels per point).
    let pointSize: CGSize
    @Published private(set) var items: [Annotation] = []
    /// The editor window's undo manager, so ⌘Z / ⇧⌘Z from the app's Edit menu work.
    weak var undoManager: UndoManager?

    var scale: CGFloat { pointSize.width > 0 ? CGFloat(image.width) / pointSize.width : 1 }
    var bounds: CGRect { CGRect(origin: .zero, size: pointSize) }
    var hasEdits: Bool { !items.isEmpty }

    init(image: CGImage, pointSize: CGSize) {
        self.image = image
        self.pointSize = pointSize
    }

    /// Reads a capture file; the point size comes from the PNG's DPI, falling back to the
    /// pixel size when there is none.
    convenience init?(contentsOf url: URL) {
        guard let data = try? Data(contentsOf: url),
              let rep = NSBitmapImageRep(data: data),
              let cg = rep.cgImage else { return nil }
        var size = rep.size
        if size.width <= 0 || size.height <= 0 {
            size = CGSize(width: cg.width, height: cg.height)
        }
        self.init(image: cg, pointSize: size)
    }

    func add(_ a: Annotation) {
        items.append(a)
        undoManager?.registerUndo(withTarget: self) { $0.remove(a) }
    }

    func remove(_ a: Annotation) {
        guard let i = items.lastIndex(where: { $0.id == a.id }) else { return }
        items.remove(at: i)
        undoManager?.registerUndo(withTarget: self) { $0.add(a) }
    }

    // MARK: geometry shared by the editor and the tests

    /// The smallest drag that counts as a mark; a click is not a zero-size box.
    static let minimumDrag: CGFloat = 4

    /// A shape from a drag, clamped to the image; nil when the drag was too small.
    /// `square` (Shift held) makes a rectangle a square and an oval a circle.
    func shape(_ tool: AnnotationTool, from a: CGPoint, to b: CGPoint, color: AnnotationColor,
               width: AnnotationWidth = .medium, square: Bool = false) -> Annotation? {
        let p = clamp(a)
        var q = clamp(b)
        if square, tool == .rect || tool == .ellipse {
            // The shorter side wins, so the square never leaves the image.
            let side = min(abs(q.x - p.x), abs(q.y - p.y))
            q = CGPoint(x: p.x + (q.x < p.x ? -side : side), y: p.y + (q.y < p.y ? -side : side))
        }
        guard hypot(q.x - p.x, q.y - p.y) >= Self.minimumDrag else { return nil }
        let r = CGRect(x: min(p.x, q.x), y: min(p.y, q.y), width: abs(q.x - p.x), height: abs(q.y - p.y))
        let big = r.width >= Self.minimumDrag && r.height >= Self.minimumDrag
        switch tool {
        case .arrow: return Annotation(.arrow(from: p, to: q), color: color, width: width)
        case .rect: return big ? Annotation(.rect(r), color: color, width: width) : nil
        case .ellipse: return big ? Annotation(.ellipse(r), color: color, width: width) : nil
        case .mosaic: return big ? Annotation(.mosaic(r), color: color, width: width) : nil
        case .line: return Annotation(.path([p, q]), color: color, width: width)
        case .text: return nil
        }
    }

    /// A drawn line from the points the pointer passed, clamped to the image; `straight`
    /// (Shift held) keeps only the first and the last. Nil for a click: a line has to go
    /// somewhere.
    func line(_ points: [CGPoint], color: AnnotationColor, width: AnnotationWidth = .medium,
              straight: Bool = false) -> Annotation? {
        var pts = points.map(clamp)
        if straight, let first = pts.first, let last = pts.last { pts = [first, last] }
        guard let first = pts.first,
              pts.contains(where: { hypot($0.x - first.x, $0.y - first.y) >= Self.minimumDrag }) else { return nil }
        return Annotation(.path(pts), color: color, width: width)
    }

    func clamp(_ p: CGPoint) -> CGPoint {
        CGPoint(x: min(max(p.x, 0), pointSize.width), y: min(max(p.y, 0), pointSize.height))
    }
}

/// Draws the marks into the capture at its full pixel resolution. Copy, Save and Send all
/// go through `flatten`, so the three can never disagree about what was marked.
enum AnnotationRenderer {
    /// The capture a mosaic is cut from, with its pixels per point.
    struct Source {
        let image: CGImage
        let scale: CGFloat
    }

    static func flatten(_ doc: ScreenshotDocument) -> CGImage? {
        flatten(image: doc.image, pointSize: doc.pointSize, items: doc.items)
    }

    static func flatten(image: CGImage, pointSize: CGSize, items: [Annotation]) -> CGImage? {
        let w = image.width, h = image.height
        guard w > 0, h > 0, pointSize.width > 0,
              let space = CGColorSpace(name: CGColorSpace.sRGB),
              let ctx = CGContext(data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: 0,
                                  space: space, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
        else { return nil }
        ctx.draw(image, in: CGRect(x: 0, y: 0, width: w, height: h))
        // Top-left origin, in points, like the editor: flip and scale once for every mark.
        let scale = CGFloat(w) / pointSize.width
        ctx.translateBy(x: 0, y: CGFloat(h))
        ctx.scaleBy(x: scale, y: -scale)
        let source = Source(image: image, scale: scale)
        for a in items { draw(a, in: ctx, source: source) }
        return ctx.makeImage()
    }

    /// Draws one mark into a top-left-origin context in image points. A mosaic needs the
    /// capture it hides; the editor's marks view passes the same one the export uses.
    static func draw(_ a: Annotation, in ctx: CGContext, source: Source?, tile: CGImage? = nil) {
        let color = a.color.nsColor.cgColor
        ctx.saveGState()
        defer { ctx.restoreGState() }
        ctx.setStrokeColor(color)
        ctx.setFillColor(color)
        ctx.setLineWidth(a.width.stroke)
        ctx.setLineCap(.round)
        ctx.setLineJoin(.round)
        switch a.kind {
        case .rect(let r):
            ctx.stroke(r)
        case .ellipse(let r):
            ctx.strokeEllipse(in: r)
        case .path(let pts):
            ctx.addPath(smoothPath(pts))
            ctx.strokePath()
        case .mosaic(let r):
            guard let t = tile ?? source.flatMap({ mosaicTile($0, rect: r, block: a.width.block) }) else { return }
            // The context is flipped (y down); an image draws upright only in an unflipped
            // space, so flip about the rectangle. No interpolation: the blocks stay blocks.
            ctx.translateBy(x: r.minX, y: r.maxY)
            ctx.scaleBy(x: 1, y: -1)
            ctx.interpolationQuality = .none
            ctx.draw(t, in: CGRect(origin: .zero, size: r.size))
        case let .arrow(from, to):
            let head = arrowHead(from: from, to: to, width: a.width.stroke)
            // The shaft stops at the head's base so the round cap does not poke through the tip.
            ctx.move(to: from)
            ctx.addLine(to: head.base)
            ctx.strokePath()
            ctx.move(to: to)
            ctx.addLine(to: head.left)
            ctx.addLine(to: head.right)
            ctx.closePath()
            ctx.fillPath()
        case let .text(s, at):
            drawText(s, at: at, size: a.width.fontSize, color: a.color.nsColor, in: ctx)
        }
    }

    /// The capture under `rect` (image points) at one sample per block: drawn back over the
    /// rectangle without interpolation, it is the mosaic. Downsampling averages each block,
    /// so nothing of the text under it survives.
    static func mosaicTile(_ source: Source, rect: CGRect, block: CGFloat) -> CGImage? {
        let s = source.scale
        let px = CGRect(x: rect.minX * s, y: rect.minY * s, width: rect.width * s, height: rect.height * s)
            .integral.intersection(CGRect(x: 0, y: 0, width: source.image.width, height: source.image.height))
        let cols = max(1, Int((rect.width / block).rounded(.up)))
        let rows = max(1, Int((rect.height / block).rounded(.up)))
        guard !px.isEmpty, let crop = source.image.cropping(to: px),
              let space = CGColorSpace(name: CGColorSpace.sRGB),
              let ctx = CGContext(data: nil, width: cols, height: rows, bitsPerComponent: 8, bytesPerRow: 0,
                                  space: space, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
        else { return nil }
        ctx.interpolationQuality = .high
        ctx.draw(crop, in: CGRect(x: 0, y: 0, width: cols, height: rows))
        return ctx.makeImage()
    }

    /// The pointer's points joined by curves through their midpoints, so a hand-drawn line
    /// reads as one stroke rather than a chain of segments.
    static func smoothPath(_ pts: [CGPoint]) -> CGPath {
        let path = CGMutablePath()
        guard let first = pts.first else { return path }
        path.move(to: first)
        if pts.count < 3 {
            for p in pts.dropFirst() { path.addLine(to: p) }
            return path
        }
        for i in 1..<(pts.count - 1) {
            let mid = CGPoint(x: (pts[i].x + pts[i + 1].x) / 2, y: (pts[i].y + pts[i + 1].y) / 2)
            path.addQuadCurve(to: mid, control: pts[i])
        }
        path.addLine(to: pts[pts.count - 1])
        return path
    }

    struct ArrowHead { let base, left, right: CGPoint }

    static func arrowHead(from: CGPoint, to: CGPoint, width: CGFloat = AnnotationWidth.medium.stroke) -> ArrowHead {
        let length = max(width * 4, 14)
        let angle = atan2(to.y - from.y, to.x - from.x)
        let spread: CGFloat = .pi / 7
        let left = CGPoint(x: to.x - length * cos(angle - spread), y: to.y - length * sin(angle - spread))
        let right = CGPoint(x: to.x - length * cos(angle + spread), y: to.y - length * sin(angle + spread))
        let base = CGPoint(x: (left.x + right.x) / 2, y: (left.y + right.y) / 2)
        return ArrowHead(base: base, left: left, right: right)
    }

    static func textAttributes(size: CGFloat, color: NSColor) -> [NSAttributedString.Key: Any] {
        [.font: NSFont.boldSystemFont(ofSize: size), .foregroundColor: color]
    }

    /// The text's box in points, its top-left at `at`: a white backing makes it readable on
    /// any screenshot.
    static func textBox(_ s: String, at: CGPoint, size: CGFloat) -> CGRect {
        let m = (s as NSString).size(withAttributes: textAttributes(size: size, color: .black))
        return CGRect(x: at.x, y: at.y, width: ceil(m.width) + 12, height: ceil(m.height) + 6)
    }

    private static func drawText(_ s: String, at: CGPoint, size: CGFloat, color: NSColor, in ctx: CGContext) {
        let box = textBox(s, at: at, size: size)
        ctx.setFillColor(NSColor(white: 1, alpha: 0.88).cgColor)
        ctx.addPath(CGPath(roundedRect: box, cornerWidth: 5, cornerHeight: 5, transform: nil))
        ctx.fillPath()
        // Swap the current context in and back out rather than save/restore: there may be no
        // current context to save (an export off screen, a test), and the stack call raises then.
        let previous = NSGraphicsContext.current
        NSGraphicsContext.current = NSGraphicsContext(cgContext: ctx, flipped: true)
        (s as NSString).draw(at: CGPoint(x: box.minX + 6, y: box.minY + 3), withAttributes: textAttributes(size: size, color: color))
        NSGraphicsContext.current = previous
    }

    // MARK: export

    /// PNG bytes that keep the capture's DPI, so a Retina image opens at its real size.
    static func pngData(_ cg: CGImage, pointSize: CGSize) -> Data? {
        let rep = NSBitmapImageRep(cgImage: cg)
        rep.size = pointSize
        return rep.representation(using: .png, properties: [:])
    }

    /// Puts the image on a pasteboard as PNG and TIFF: terminals, chat apps and image
    /// editors each read one or the other.
    @discardableResult
    static func copy(_ cg: CGImage, pointSize: CGSize, to pb: NSPasteboard = .general) -> Bool {
        guard let png = pngData(cg, pointSize: pointSize) else { return false }
        let rep = NSBitmapImageRep(cgImage: cg)
        rep.size = pointSize
        pb.clearContents()
        pb.declareTypes([.png, .tiff], owner: nil)
        let okPNG = pb.setData(png, forType: .png)
        let okTIFF = rep.tiffRepresentation.map { pb.setData($0, forType: .tiff) } ?? false
        return okPNG && okTIFF
    }
}
