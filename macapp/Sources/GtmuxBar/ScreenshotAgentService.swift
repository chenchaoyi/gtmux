import AppKit
import Darwin

struct ScreenshotAgentBudget {
    let end: TimeInterval
    let cancellation: ScreenshotAgentCancellation?
    init(end: TimeInterval, cancellation: ScreenshotAgentCancellation? = nil) { self.end = end; self.cancellation = cancellation }
    init(deadlineMS: Int64, uptime: TimeInterval = ProcessInfo.processInfo.systemUptime, now: Date = Date()) {
        cancellation = nil
        end = uptime + min(10, max(0, Double(deadlineMS)/1000-now.timeIntervalSince1970))
    }
    func check() throws {
        try cancellation?.check()
        guard ProcessInfo.processInfo.systemUptime < end else { throw ScreenshotAgentFailure.refused("deadline-exceeded", "Screenshot request deadline expired") }
    }
}

@MainActor final class ScreenshotAgentService {
    let sources: ScreenshotAgentSources
    var editorOpen: () -> Bool = { !ScreenshotEditors.shared.hasCapacity }
    var openEditor: (ScreenshotDocument, Data) throws -> Void

    init(sources: ScreenshotAgentSources? = nil, openEditor: @escaping (ScreenshotDocument, Data) throws -> Void) {
        self.sources = sources ?? ScreenshotAgentSources()
        self.openEditor = openEditor
    }

    func perform(_ request: ScreenshotAgentRequest, files: [Int32], budget: ScreenshotAgentBudget) async throws -> Data {
        let context = ScreenshotAgentCancellation(end: budget.end)
        defer { context.finish() }
        let resources = ScreenshotAgentFiles(files, owned: false)
        defer { resources.close() }
        return try await perform(request, resources: resources, context: context)
    }

    func perform(_ request: ScreenshotAgentRequest, resources: ScreenshotAgentFiles, context: ScreenshotAgentCancellation) async throws -> Data {
        try context.check()
        guard resources.count == request.descriptorCount else { throw ScreenshotAgentFailure.refused("descriptor-count", "Wrong number of image/output descriptors") }
        switch request.operation {
        case "capabilities":
            return try JSONSerialization.data(withJSONObject: ["platform": "macOS", "schema_version": 1,
                "capture_available": sources.recordingAllowed(), "annotation_available": true,
                "operations": ["sources", "capture", "import", "annotate", "edit"],
                "screen_recording_prompted": false, "mcp_required": false])
        case "sources":
            let list = try await sources.inventory(app: request.app, displays: request.source_kind == "display", context: context)
            try context.check()
            return try JSONSerialization.data(withJSONObject: ["sources": list])
        case "capture":
            guard let reference = request.source_ref else { throw ScreenshotAgentFailure.refused("source-required", "Choose a source reference first") }
            let (image, size, rect, scale, kind) = try await sources.capture(reference: reference, rect: request.rect, context: context)
            return try await ScreenshotAgentWork.run(context: context, files: resources, indices: [0, 1]) { files, budget in
                let result = try ScreenshotAgentRendering.encode(image: image, pointSize: size, kind: kind, rect: rect, sourceScale: scale)
                return try Self.write(result, files: files, budget: budget)
            }
        case "import", "annotate":
            return try await ScreenshotAgentWork.run(context: context, files: resources, indices: [0, 1, 2]) { files, budget in
                let (image, size, items, _) = try Self.prepare(request, fd: files[0], budget: budget)
                let result: (Data, Data, ScreenshotAgentFrame)
                if request.operation == "import" {
                    result = try ScreenshotAgentRendering.encode(image: image, pointSize: size, kind: "imported")
                } else {
                    guard let flattened = AnnotationRenderer.flatten(image: image, pointSize: size, items: items), let frame = request.document else { throw ScreenshotAgentFailure.refused("render-failed", "Could not render annotations") }
                    var annotated = try ScreenshotAgentRendering.encode(image: flattened, pointSize: size, kind: frame.source_kind, annotations: items)
                    annotated.2.captured_at = frame.captured_at
                    annotated.2.source_rect_pt = frame.source_rect_pt
                    annotated.2.source_to_image = frame.source_to_image
                    result = annotated
                }
                return try Self.write(result, files: Array(files.dropFirst()), budget: budget)
            }
        case "edit":
            let (image, size, items, data) = try await ScreenshotAgentWork.run(context: context, files: resources, indices: [0]) { files, budget in
                try Self.prepare(request, fd: files[0], budget: budget)
            }
            return try context.commit {
                guard !editorOpen() else { throw ScreenshotAgentFailure.refused("editor-busy", "Screenshot window limit reached; existing drafts were preserved") }
                let document = ScreenshotDocument(image: image, pointSize: size)
                items.forEach { document.add($0) }
                try openEditor(document, data)
                return try JSONSerialization.data(withJSONObject: ["editor_opened": true, "sent": false])
            }
        default: throw ScreenshotAgentFailure.refused("unsupported-operation", "Unsupported screenshot operation")
        }
    }

    nonisolated private static func prepare(_ request: ScreenshotAgentRequest, fd: Int32, budget: ScreenshotAgentBudget) throws -> (CGImage, CGSize, [Annotation], Data) {
        let data = try readImage(fd, budget: budget)
        guard request.image_sha256 == ScreenshotAgentRendering.hash(data) else { throw ScreenshotAgentFailure.refused("image-changed", "Input image hash changed") }
        let (image, importedSize) = try ScreenshotAgentRendering.decode(data)
        try budget.check()
        if request.operation == "import" { return (image, importedSize, [], data) }
        guard let frame = request.document, let spec = request.spec else { throw ScreenshotAgentFailure.refused("document-required", "Provide image metadata and its annotation specification") }
        guard frame.image_sha256 == request.image_sha256 else { throw ScreenshotAgentFailure.refused("image-changed", "Image and document hash differ") }
        return (image, CGSize(width: frame.point_width, height: frame.point_height), try spec.items(frame: frame, image: image), data)
    }

    nonisolated private static func write(_ output: (Data, Data, ScreenshotAgentFrame), files: [Int32], budget: ScreenshotAgentBudget) throws -> Data {
        try budget.check()
        guard files.count == 2 else { throw ScreenshotAgentFailure.refused("descriptor-count", "Two output descriptors are required") }
        try writeImage(output.0, to: files[0], budget: budget)
        try writeImage(output.1, to: files[1], budget: budget)
        try budget.check()
        return try JSONEncoder().encode(output.2)
    }

    nonisolated static func readImage(_ fd: Int32, budget: ScreenshotAgentBudget) throws -> Data {
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

    nonisolated static func writeImage(_ data: Data, to fd: Int32, budget: ScreenshotAgentBudget) throws {
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
