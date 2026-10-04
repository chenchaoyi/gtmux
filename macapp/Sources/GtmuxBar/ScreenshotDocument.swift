import AppKit
import CoreGraphics

enum AnnotationTool: String, CaseIterable, Identifiable {
    case arrow, rect, text
    var id: String { rawValue }
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
        case rect(CGRect)
        case text(String, at: CGPoint)
    }
    let id: UUID
    var kind: Kind
    var color: AnnotationColor

    init(_ kind: Kind, color: AnnotationColor, id: UUID = UUID()) {
        self.id = id
        self.kind = kind
        self.color = color
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
    func shape(_ tool: AnnotationTool, from a: CGPoint, to b: CGPoint, color: AnnotationColor) -> Annotation? {
        let p = clamp(a), q = clamp(b)
        guard hypot(q.x - p.x, q.y - p.y) >= Self.minimumDrag else { return nil }
        switch tool {
        case .arrow: return Annotation(.arrow(from: p, to: q), color: color)
        case .rect:
            let r = CGRect(x: min(p.x, q.x), y: min(p.y, q.y), width: abs(q.x - p.x), height: abs(q.y - p.y))
            guard r.width >= Self.minimumDrag, r.height >= Self.minimumDrag else { return nil }
            return Annotation(.rect(r), color: color)
        case .text: return nil
        }
    }

    func clamp(_ p: CGPoint) -> CGPoint {
        CGPoint(x: min(max(p.x, 0), pointSize.width), y: min(max(p.y, 0), pointSize.height))
    }
}

/// Draws the marks into the capture at its full pixel resolution. Copy, Save and Send all
/// go through `flatten`, so the three can never disagree about what was marked.
enum AnnotationRenderer {
    /// Stroke width in points; a Retina capture gets twice the pixels, the same look.
    static let lineWidth: CGFloat = 4
    static let fontSize: CGFloat = 18

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
        for a in items { draw(a, in: ctx) }
        return ctx.makeImage()
    }

    static func draw(_ a: Annotation, in ctx: CGContext) {
        let color = a.color.nsColor.cgColor
        ctx.saveGState()
        defer { ctx.restoreGState() }
        ctx.setStrokeColor(color)
        ctx.setFillColor(color)
        ctx.setLineWidth(lineWidth)
        ctx.setLineCap(.round)
        ctx.setLineJoin(.round)
        switch a.kind {
        case .rect(let r):
            ctx.stroke(r)
        case let .arrow(from, to):
            let head = arrowHead(from: from, to: to)
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
            drawText(s, at: at, color: a.color.nsColor, in: ctx)
        }
    }

    struct ArrowHead { let base, left, right: CGPoint }

    static func arrowHead(from: CGPoint, to: CGPoint) -> ArrowHead {
        let length = max(lineWidth * 4, 14)
        let angle = atan2(to.y - from.y, to.x - from.x)
        let spread: CGFloat = .pi / 7
        let left = CGPoint(x: to.x - length * cos(angle - spread), y: to.y - length * sin(angle - spread))
        let right = CGPoint(x: to.x - length * cos(angle + spread), y: to.y - length * sin(angle + spread))
        let base = CGPoint(x: (left.x + right.x) / 2, y: (left.y + right.y) / 2)
        return ArrowHead(base: base, left: left, right: right)
    }

    static func textAttributes(color: NSColor) -> [NSAttributedString.Key: Any] {
        [.font: NSFont.boldSystemFont(ofSize: fontSize), .foregroundColor: color]
    }

    /// The text's box in points, its top-left at `at`: a white backing makes it readable on
    /// any screenshot.
    static func textBox(_ s: String, at: CGPoint) -> CGRect {
        let size = (s as NSString).size(withAttributes: textAttributes(color: .black))
        return CGRect(x: at.x, y: at.y, width: ceil(size.width) + 12, height: ceil(size.height) + 6)
    }

    private static func drawText(_ s: String, at: CGPoint, color: NSColor, in ctx: CGContext) {
        let box = textBox(s, at: at)
        ctx.setFillColor(NSColor(white: 1, alpha: 0.88).cgColor)
        ctx.addPath(CGPath(roundedRect: box, cornerWidth: 5, cornerHeight: 5, transform: nil))
        ctx.fillPath()
        // Swap the current context in and back out rather than save/restore: there may be no
        // current context to save (an export off screen, a test), and the stack call raises then.
        let previous = NSGraphicsContext.current
        NSGraphicsContext.current = NSGraphicsContext(cgContext: ctx, flipped: true)
        (s as NSString).draw(at: CGPoint(x: box.minX + 6, y: box.minY + 3), withAttributes: textAttributes(color: color))
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
