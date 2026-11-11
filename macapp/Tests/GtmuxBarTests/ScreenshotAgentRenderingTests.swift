import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotAgentRenderingTests: XCTestCase {
    static func image() -> CGImage {
        let c = CGContext(data: nil, width: 80, height: 40, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        c.setFillColor(CGColor(red: 1, green: 0, blue: 0, alpha: 1)); c.fill(CGRect(x: 0, y: 0, width: 80, height: 40))
        return c.makeImage()!
    }
    static func spec(frame: ScreenshotAgentFrame, space: String = "image_px", marks: [[String: Any]]) throws -> ScreenshotAgentSpec {
        let data = try JSONSerialization.data(withJSONObject: ["schema_version": 1, "capture_id": frame.capture_id,
            "image_sha256": frame.image_sha256, "coordinate_space": space, "preview_id": frame.preview_id, "annotations": marks])
        return try JSONDecoder().decode(ScreenshotAgentSpec.self, from: data)
    }
    func testASC003RedactionUsesSharedRetinaRendererAndExactFrame() throws {
        let source = Self.image(), size = CGSize(width: 40, height: 20)
        let encoded = try ScreenshotAgentRendering.encode(image: source, pointSize: size, kind: "imported")
        let marks: [[String: Any]] = [["type": "redact", "x": 10, "y": 4, "width": 30, "height": 20]]
        let spec = try Self.spec(frame: encoded.2, marks: marks)
        let items = try spec.items(frame: encoded.2, image: source)
        let result = try XCTUnwrap(AnnotationRenderer.flatten(image: source, pointSize: size, items: items))
        let pixel = try XCTUnwrap(NSBitmapImageRep(cgImage: result).colorAt(x: 20, y: 10)?.usingColorSpace(.sRGB))
        XCTAssertLessThan(pixel.redComponent, 0.01); XCTAssertLessThan(pixel.greenComponent, 0.01); XCTAssertLessThan(pixel.blueComponent, 0.01)
        XCTAssertEqual(NSBitmapImageRep(cgImage: source).colorAt(x: 20, y: 10)?.redComponent, 1, "original remains unchanged")
        let decoded = try ScreenshotAgentRendering.decode(encoded.0)
        XCTAssertEqual(decoded.0.width, 80); XCTAssertEqual(decoded.1.width, 40, accuracy: 0.01)
        var wrong = encoded.2; wrong.capture_id = "other"
        XCTAssertThrowsError(try spec.items(frame: wrong, image: source))
        let outside = try Self.spec(frame: encoded.2, marks: [["type": "rect", "x": 79, "y": 0, "width": 2, "height": 4]])
        XCTAssertThrowsError(try outside.items(frame: encoded.2, image: source))
        let preview = try Self.spec(frame: encoded.2, space: "preview_px", marks: marks)
        XCTAssertEqual(try preview.items(frame: encoded.2, image: source).count, 1)
        XCTAssertThrowsError(try ScreenshotAgentRendering.decode(Data("invalid".utf8)))
    }
    @MainActor func testASC006EditorRoundtripPreservesBusyDraftAndDoesNotSend() async throws {
        let encoded = try ScreenshotAgentRendering.encode(image: Self.image(), pointSize: CGSize(width: 40, height: 20), kind: "imported")
        let spec = try Self.spec(frame: encoded.2, marks: [["type": "arrow", "from": [0, 0], "to": [60, 20]]])
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true); defer { try? FileManager.default.removeItem(at: dir) }
        let input = dir.appendingPathComponent("image.png"); try encoded.0.write(to: input)
        let fd = open(input.path, O_RDONLY); XCTAssertGreaterThanOrEqual(fd, 0); defer { close(fd) }
        var opens = 0
        let service = ScreenshotAgentService { doc, data in opens += 1; XCTAssertEqual(doc.items.count, 1); XCTAssertEqual(data, encoded.0) }
        let request = ScreenshotAgentRequest(schema_version: 1, request_id: String(repeating: "a", count: 32), host_nonce: "fixture", operation: "edit", deadline_ms: 0,
            app: nil, source_kind: nil, source_ref: nil, rect: nil, image_sha256: encoded.2.image_sha256, document: encoded.2, spec: spec)
        service.editorOpen = { true }
        do { _ = try await service.perform(request, files: [fd], budget: ScreenshotAgentBudget(end: ProcessInfo.processInfo.systemUptime+2)); XCTFail("busy draft replaced") } catch { XCTAssertEqual((error as? ScreenshotAgentFailure)?.code, "editor-busy") }
        XCTAssertEqual(opens, 0)
        service.editorOpen = { false }
        let data = try await service.perform(request, files: [fd], budget: ScreenshotAgentBudget(end: ProcessInfo.processInfo.systemUptime+2))
        let receipt = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(opens, 1); XCTAssertEqual(receipt["sent"] as? Bool, false)
    }
}
