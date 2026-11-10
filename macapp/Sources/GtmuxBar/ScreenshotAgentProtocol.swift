import AppKit
import CryptoKit
import ImageIO

enum ScreenshotAgentFailure: Error {
    case refused(String, String)
    var code: String { switch self { case let .refused(code, _): return code } }
    var message: String { switch self { case let .refused(_, message): return message } }
}

struct ScreenshotAgentRequest: Decodable {
    let schema_version: Int
    let request_id: String
    let host_nonce: String
    let operation: String
    let deadline_ms: Int64
    let app: String?
    let source_kind: String?
    let source_ref: String?
    let rect: [Double]?
    let image_sha256: String?
    let document: ScreenshotAgentFrame?
    let spec: ScreenshotAgentSpec?

    static func decode(_ data: Data) throws -> Self {
        let base: Set<String> = ["schema_version", "request_id", "host_nonce", "operation", "deadline_ms"]
        let fields: [String: Set<String>] = ["capabilities": [], "sources": ["app", "source_kind"], "capture": ["source_ref", "rect"], "import": ["image_sha256"], "annotate": ["image_sha256", "document", "spec"], "edit": ["image_sha256", "document", "spec"]]
        guard data.count <= 256 << 10,
              let object = try JSONSerialization.jsonObject(with: data) as? [String: Any],
              let operation = object["operation"] as? String, let allowed = fields[operation],
              Set(object.keys).isSubset(of: base.union(allowed)) else { throw ScreenshotAgentFailure.refused("invalid-request", "Invalid request fields or JSON budget") }
        let request = try JSONDecoder().decode(Self.self, from: data)
        guard request.schema_version == 1, request.request_id.count == 32,
              request.request_id.allSatisfy({ $0.isHexDigit && $0.isASCII }),
              ["capabilities", "sources", "capture", "import", "annotate", "edit"].contains(request.operation)
        else { throw ScreenshotAgentFailure.refused("unsupported-request", "Unsupported protocol version, request ID or operation") }
        return request
    }

    var descriptorCount: Int {
        switch operation {
        case "capture": return 2
        case "import", "annotate": return 3
        case "edit": return 1
        default: return 0
        }
    }
}

struct ScreenshotAgentFrame: Codable {
    var schema_version = 1
    var capture_id: String
    var source_kind: String
    var image_path = ""
    var image_sha256: String
    var pixel_width: Int
    var pixel_height: Int
    var point_width: Double
    var point_height: Double
    var preview_id: String
    var preview_path = ""
    var preview_sha256: String
    var preview_width: Int
    var preview_height: Int
    var source_rect_pt: [Double]?
    var source_to_image: [Double]?
    var captured_at: String

    func validate(image: CGImage) throws {
        guard schema_version == 1, !capture_id.isEmpty, image_sha256.count == 64,
              image.width == pixel_width, image.height == pixel_height,
              point_width.isFinite, point_height.isFinite, point_width > 0, point_height > 0,
              abs(Double(pixel_width)/point_width - Double(pixel_height)/point_height) <= 0.02,
              !preview_id.isEmpty, preview_width > 0, preview_height > 0,
              preview_width <= 1600, preview_height <= 1600
        else { throw ScreenshotAgentFailure.refused("invalid-document", "Image and document coordinate metadata disagree") }
    }
}

struct ScreenshotAgentSpec: Decodable {
    let schema_version: Int
    let capture_id: String
    let image_sha256: String
    let coordinate_space: String
    let preview_id: String?
    let annotations: [ScreenshotAgentAnnotationItem]

    func items(frame: ScreenshotAgentFrame, image: CGImage) throws -> [Annotation] {
        try frame.validate(image: image)
        guard schema_version == 1, capture_id == frame.capture_id, image_sha256 == frame.image_sha256,
              annotations.count <= 500, ["image_px", "preview_px"].contains(coordinate_space),
              coordinate_space != "preview_px" || preview_id == frame.preview_id else {
            throw ScreenshotAgentFailure.refused("annotation-frame-mismatch", "Annotations must address this exact image and preview")
        }
        let width = coordinate_space == "preview_px" ? Double(frame.preview_width) : Double(image.width)
        let height = coordinate_space == "preview_px" ? Double(frame.preview_height) : Double(image.height)
        let sx = frame.point_width/width, sy = frame.point_height/height
        var pointCount = 0
        return try annotations.map { mark in
            pointCount += mark.points?.count ?? 0
            guard pointCount <= 10000 else { throw ScreenshotAgentFailure.refused("annotation-budget", "Too many path points") }
            return try mark.item(width: width, height: height, sx: sx, sy: sy)
        }
    }
}

struct ScreenshotAgentAnnotationItem: Decodable {
    let type: String
    let x: Double?
    let y: Double?
    let width: Double?
    let height: Double?
    let from: [Double]?
    let to: [Double]?
    let points: [[Double]]?
    let text: String?
    let color: String?
    let width_style: String?

    func item(width canvasWidth: Double, height canvasHeight: Double, sx: Double, sy: Double) throws -> Annotation {
        func point(_ values: [Double]?) throws -> CGPoint {
            guard let values, values.count == 2, values.allSatisfy({ $0.isFinite }),
                  values[0] >= 0, values[1] >= 0, values[0] <= canvasWidth, values[1] <= canvasHeight else {
                throw ScreenshotAgentFailure.refused("invalid-annotation", "Annotation lies outside the selected image")
            }
            return CGPoint(x: values[0]*sx, y: values[1]*sy)
        }
        guard let tint = AnnotationColor(rawValue: color ?? "red"),
              let stroke = AnnotationWidth(rawValue: width_style ?? "medium") else {
            throw ScreenshotAgentFailure.refused("invalid-annotation", "Unsupported marking color or width")
        }
        let kind: Annotation.Kind
        switch type {
        case "rect", "ellipse", "mosaic", "redact":
            let origin = try point([x ?? 0, y ?? 0])
            guard let width, let height, width.isFinite, height.isFinite, width > 0, height > 0,
                  (x ?? 0) + width <= canvasWidth, (y ?? 0) + height <= canvasHeight else {
                throw ScreenshotAgentFailure.refused("invalid-annotation", "Invalid annotation rectangle")
            }
            let rect = CGRect(origin: origin, size: CGSize(width: width*sx, height: height*sy))
            switch type {
            case "rect": kind = .rect(rect)
            case "ellipse": kind = .ellipse(rect)
            case "mosaic": kind = .mosaic(rect)
            default: kind = .redact(rect)
            }
        case "arrow": kind = .arrow(from: try point(from), to: try point(to))
        case "line":
            guard let points, points.count >= 2 else { throw ScreenshotAgentFailure.refused("invalid-annotation", "A line requires at least two points") }
            kind = .path(try points.map { try point($0) })
        case "text":
            guard let text, !text.isEmpty, text.utf8.count <= 2048 else { throw ScreenshotAgentFailure.refused("invalid-annotation", "Invalid annotation text") }
            kind = .text(text, at: try point([x ?? 0, y ?? 0]))
        default: throw ScreenshotAgentFailure.refused("invalid-annotation", "Unsupported annotation type")
        }
        return Annotation(kind, color: tint, width: stroke)
    }
}

enum ScreenshotAgentRendering {
    static let maximumBytes = 30 << 20
    static let maximumPixels = 32_000_000
    static func hash(_ data: Data) -> String { SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined() }

    static func decode(_ data: Data) throws -> (CGImage, CGSize) {
        guard !data.isEmpty, data.count <= maximumBytes,
              let source = CGImageSourceCreateWithData(data as CFData, nil),
              let props = CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any],
              let w = props[kCGImagePropertyPixelWidth] as? NSNumber,
              let h = props[kCGImagePropertyPixelHeight] as? NSNumber,
              w.intValue > 0, h.intValue > 0, w.intValue <= maximumPixels, h.intValue <= maximumPixels,
              Int64(w.intValue)*Int64(h.intValue) <= maximumPixels,
              let image = CGImageSourceCreateThumbnailAtIndex(source, 0, [kCGImageSourceShouldCacheImmediately: true,
                kCGImageSourceCreateThumbnailFromImageAlways: true, kCGImageSourceCreateThumbnailWithTransform: true,
                kCGImageSourceThumbnailMaxPixelSize: max(w.intValue, h.intValue)] as CFDictionary)
        else { throw ScreenshotAgentFailure.refused("invalid-image", "Image exceeds its decode budget or cannot be decoded") }
        let dpi = (props[kCGImagePropertyDPIWidth] as? NSNumber)?.doubleValue ?? 72
        let scale = dpi.isFinite && dpi >= 72 && dpi <= 288 ? dpi/72 : 1
        return (image, CGSize(width: Double(image.width)/scale, height: Double(image.height)/scale))
    }

    static func preview(_ image: CGImage) throws -> CGImage {
        let ratio = min(1, 1600/Double(max(image.width, image.height)))
        let w = max(1, Int((Double(image.width)*ratio).rounded())), h = max(1, Int((Double(image.height)*ratio).rounded()))
        guard let space = CGColorSpace(name: CGColorSpace.sRGB),
              let ctx = CGContext(data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: 0, space: space, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
        else { throw ScreenshotAgentFailure.refused("render-failed", "Could not create the preview") }
        ctx.interpolationQuality = .high
        ctx.draw(image, in: CGRect(x: 0, y: 0, width: w, height: h))
        guard let output = ctx.makeImage() else { throw ScreenshotAgentFailure.refused("render-failed", "Could not render the preview") }
        return output
    }

    static func encode(image: CGImage, pointSize: CGSize, kind: String, rect: CGRect? = nil, sourceScale: CGFloat = 1) throws -> (Data, Data, ScreenshotAgentFrame) {
        guard image.width > 0, image.height > 0, Int64(image.width)*Int64(image.height) <= maximumPixels,
              let full = AnnotationRenderer.pngData(image, pointSize: pointSize) else { throw ScreenshotAgentFailure.refused("render-failed", "Could not encode the image") }
        let small = try preview(image)
        guard let png = AnnotationRenderer.pngData(small, pointSize: CGSize(width: small.width, height: small.height)),
              full.count <= maximumBytes, png.count <= maximumBytes else { throw ScreenshotAgentFailure.refused("image-too-large", "PNG exceeds the 30 MiB attachment budget") }
        var frame = ScreenshotAgentFrame(capture_id: UUID().uuidString.lowercased(), source_kind: kind, image_sha256: hash(full), pixel_width: image.width, pixel_height: image.height,
            point_width: pointSize.width, point_height: pointSize.height, preview_id: UUID().uuidString.lowercased(), preview_sha256: hash(png), preview_width: small.width, preview_height: small.height,
            captured_at: ISO8601DateFormatter().string(from: Date()))
        if let rect {
            frame.source_rect_pt = [rect.minX, rect.minY, rect.width, rect.height]
            frame.source_to_image = [sourceScale, 0, 0, sourceScale, -rect.minX*sourceScale, -rect.minY*sourceScale]
        }
        return (full, png, frame)
    }
}
