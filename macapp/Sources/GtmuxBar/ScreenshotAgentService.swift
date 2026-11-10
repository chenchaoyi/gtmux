import AppKit
import Darwin

struct ScreenshotAgentBudget {
    let end: TimeInterval
    init(end: TimeInterval) { self.end = end }
    init(deadlineMS: Int64, uptime: TimeInterval = ProcessInfo.processInfo.systemUptime, now: Date = Date()) {
        end = uptime + min(10, max(0, Double(deadlineMS)/1000-now.timeIntervalSince1970))
    }
    func check() throws {
        guard ProcessInfo.processInfo.systemUptime < end else { throw ScreenshotAgentFailure.refused("deadline-exceeded", "Screenshot request deadline expired") }
    }
}

@MainActor final class ScreenshotAgentService {
    let sources: ScreenshotAgentSources
    var editorOpen: () -> Bool = { ScreenshotEditorController.shared.isOpen }
    var openEditor: (ScreenshotDocument, Data) throws -> Void

    init(sources: ScreenshotAgentSources? = nil, openEditor: @escaping (ScreenshotDocument, Data) throws -> Void) {
        self.sources = sources ?? ScreenshotAgentSources()
        self.openEditor = openEditor
    }

    func perform(_ request: ScreenshotAgentRequest, files: [Int32], budget: ScreenshotAgentBudget) async throws -> Data {
        try budget.check()
        guard files.count == request.descriptorCount else { throw ScreenshotAgentFailure.refused("descriptor-count", "Wrong number of image/output descriptors") }
        switch request.operation {
        case "capabilities":
            return try JSONSerialization.data(withJSONObject: ["platform": "macOS", "schema_version": 1,
                "capture_available": sources.recordingAllowed(), "annotation_available": true,
                "operations": ["sources", "capture", "import", "annotate", "edit"],
                "screen_recording_prompted": false, "mcp_required": false])
        case "sources":
            let list = try await sources.inventory(app: request.app, displays: request.source_kind == "display")
            try budget.check()
            return try JSONSerialization.data(withJSONObject: ["sources": list])
        case "capture":
            guard let reference = request.source_ref else { throw ScreenshotAgentFailure.refused("source-required", "Choose a source reference first") }
            let (image, size, rect, scale, kind) = try await sources.capture(reference: reference, rect: request.rect)
            try budget.check()
            let result = try ScreenshotAgentRendering.encode(image: image, pointSize: size, kind: kind, rect: rect, sourceScale: scale)
            return try write(result, files: files, budget: budget)
        case "import", "annotate", "edit":
            let data = try Self.readImage(files[0], budget: budget)
            guard request.image_sha256 == ScreenshotAgentRendering.hash(data) else { throw ScreenshotAgentFailure.refused("image-changed", "Input image hash changed") }
            let (image, importedSize) = try ScreenshotAgentRendering.decode(data)
            if request.operation == "import" {
                return try write(ScreenshotAgentRendering.encode(image: image, pointSize: importedSize, kind: "imported"), files: Array(files.dropFirst()), budget: budget)
            }
            guard let frame = request.document, let spec = request.spec else { throw ScreenshotAgentFailure.refused("document-required", "Provide image metadata and its annotation specification") }
            guard frame.image_sha256 == request.image_sha256 else { throw ScreenshotAgentFailure.refused("image-changed", "Image and document hash differ") }
            let items = try spec.items(frame: frame, image: image)
            let size = CGSize(width: frame.point_width, height: frame.point_height)
            let doc = ScreenshotDocument(image: image, pointSize: size)
            items.forEach { doc.add($0) }
            if request.operation == "edit" {
                guard !editorOpen() else { throw ScreenshotAgentFailure.refused("editor-busy", "An existing screenshot draft is open; it was preserved") }
                try budget.check()
                try openEditor(doc, data)
                return try JSONSerialization.data(withJSONObject: ["editor_opened": true, "sent": false])
            }
            guard let flattened = AnnotationRenderer.flatten(doc) else { throw ScreenshotAgentFailure.refused("render-failed", "Could not render annotations") }
            var result = try ScreenshotAgentRendering.encode(image: flattened, pointSize: size, kind: frame.source_kind)
            result.2.captured_at = frame.captured_at
            result.2.source_rect_pt = frame.source_rect_pt
            result.2.source_to_image = frame.source_to_image
            return try write(result, files: Array(files.dropFirst()), budget: budget)
        default: throw ScreenshotAgentFailure.refused("unsupported-operation", "Unsupported screenshot operation")
        }
    }

    private func write(_ output: (Data, Data, ScreenshotAgentFrame), files: [Int32], budget: ScreenshotAgentBudget) throws -> Data {
        try budget.check()
        guard files.count == 2 else { throw ScreenshotAgentFailure.refused("descriptor-count", "Two output descriptors are required") }
        try Self.writeImage(output.0, to: files[0], budget: budget)
        try Self.writeImage(output.1, to: files[1], budget: budget)
        try budget.check()
        return try JSONEncoder().encode(output.2)
    }

    static func readImage(_ fd: Int32, budget: ScreenshotAgentBudget) throws -> Data {
        guard lseek(fd, 0, SEEK_SET) >= 0 else { throw ScreenshotAgentFailure.refused("image-unavailable", "Input is not a seekable image") }
        var result = Data(), buffer = [UInt8](repeating: 0, count: 64 << 10)
        while true {
            try budget.check()
            let n = Darwin.read(fd, &buffer, buffer.count)
            if n == 0 { return result }
            guard n > 0, result.count+n <= ScreenshotAgentRendering.maximumBytes else { throw ScreenshotAgentFailure.refused("image-budget", "Input read failed or exceeded its budget") }
            result.append(contentsOf: buffer.prefix(n))
        }
    }

    static func writeImage(_ data: Data, to fd: Int32, budget: ScreenshotAgentBudget) throws {
        try data.withUnsafeBytes { raw in
            var offset = 0
            while offset < raw.count {
                try budget.check()
                let n = Darwin.write(fd, raw.baseAddress!.advanced(by: offset), min(64 << 10, raw.count-offset))
                guard n > 0 else { throw ScreenshotAgentFailure.refused("output-write-failed", "Could not write screenshot output") }
                offset += n
            }
        }
        guard fsync(fd) == 0 else { throw ScreenshotAgentFailure.refused("output-write-failed", "Could not confirm screenshot output") }
    }
}
