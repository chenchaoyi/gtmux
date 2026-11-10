import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotPreviewAspectTests: XCTestCase {
    private func image(width: Int, height: Int, color: CGColor) throws -> CGImage {
        let context = try XCTUnwrap(CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(color); context.fill(CGRect(x: 0, y: 0, width: width, height: height))
        return try XCTUnwrap(context.makeImage())
    }

    @MainActor private func annotate(_ base: (Data, Data, ScreenshotAgentFrame), rect: CGRect, space: String) async throws -> CGImage {
        let frame = base.2
        let sx = space == "preview_px" ? Double(frame.preview_width)/frame.point_width : Double(frame.pixel_width)/frame.point_width
        let sy = space == "preview_px" ? Double(frame.preview_height)/frame.point_height : Double(frame.pixel_height)/frame.point_height
        let spec = try ScreenshotAgentRenderingTests.spec(frame: frame, space: space, marks: [["type": "redact",
            "x": rect.minX*sx, "y": rect.minY*sy, "width": rect.width*sx, "height": rect.height*sy]])
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let input = directory.appendingPathComponent("base.png"), output = directory.appendingPathComponent("result.png"), preview = directory.appendingPathComponent("preview.png")
        try base.0.write(to: input)
        let original = open(input.path, O_RDONLY|O_CLOEXEC)
        let full = open(output.path, O_CREAT|O_EXCL|O_WRONLY|O_CLOEXEC, 0o600)
        let small = open(preview.path, O_CREAT|O_EXCL|O_WRONLY|O_CLOEXEC, 0o600)
        XCTAssertGreaterThanOrEqual(original, 0); XCTAssertGreaterThanOrEqual(full, 0); XCTAssertGreaterThanOrEqual(small, 0)
        defer { close(original); close(full); close(small) }
        let service = ScreenshotAgentService { _, _ in XCTFail("annotation opened an editor") }
        let request = ScreenshotAgentRequest(schema_version: 1, request_id: String(repeating: "c", count: 32), host_nonce: "fixture", operation: "annotate", deadline_ms: 0,
            app: nil, source_kind: nil, source_ref: nil, rect: nil, image_sha256: frame.image_sha256, document: frame, spec: spec)
        let result = try await service.perform(request, files: [original, full, small], budget: .init(end: ProcessInfo.processInfo.systemUptime+3))
        let metadata = try JSONDecoder().decode(ScreenshotAgentFrame.self, from: result)
        XCTAssertEqual(metadata.source_rect_pt, frame.source_rect_pt)
        XCTAssertEqual(metadata.source_to_image, frame.source_to_image)
        XCTAssertEqual(try Data(contentsOf: input), base.0, "original bytes are not rewritten")
        return try ScreenshotAgentRendering.decode(Data(contentsOf: preview)).0
    }

    private func assertCoverage(_ image: CGImage, frame: ScreenshotAgentFrame, rect: CGRect, file: StaticString = #filePath, line: UInt = #line) throws {
        let sx = CGFloat(image.width)/frame.point_width, sy = CGFloat(image.height)/frame.point_height
        let bitmap = NSBitmapImageRep(cgImage: image)
        for y in Int(floor(rect.minY*sy))..<Int(ceil(rect.maxY*sy)) {
            for x in Int(floor(rect.minX*sx))..<Int(ceil(rect.maxX*sx)) {
                let pixel = try XCTUnwrap(bitmap.colorAt(x: x, y: y)?.usingColorSpace(.sRGB), file: file, line: line)
                XCTAssertEqual(pixel.redComponent, 0, accuracy: 0.001, "pixel=\(x),\(y)", file: file, line: line)
                XCTAssertEqual(pixel.greenComponent, 0, accuracy: 0.001, file: file, line: line)
                XCTAssertEqual(pixel.blueComponent, 0, accuracy: 0.001, file: file, line: line)
                XCTAssertEqual(pixel.alphaComponent, 1, accuracy: 0.001, file: file, line: line)
            }
        }
    }

    @MainActor func testASC012OddAspectImageAndPreviewCoordinatesCoverBothAxes() async throws {
        let rect = CGRect(x: 2.25, y: 15.75, width: 0.5, height: 0.15)
        for (width, height) in [(3200, 33), (33, 3200)] {
            for color in [CGColor(red: 1, green: 0, blue: 0, alpha: 1), CGColor(red: 0, green: 0, blue: 1, alpha: 1)] {
                let source = try image(width: width, height: height, color: color)
                let base = try ScreenshotAgentRendering.encode(image: source, pointSize: CGSize(width: width, height: height), kind: "imported")
                for space in ["image_px", "preview_px"] {
                    let result = try await annotate(base, rect: rect, space: space)
                    XCTAssertEqual(result.width, base.2.preview_width); XCTAssertEqual(result.height, base.2.preview_height)
                    try assertCoverage(result, frame: base.2, rect: rect)
                }
            }
        }
    }

    @MainActor func testASC012CroppedSourceRegionRetainsOpaquePreviewCoverageAndTransforms() async throws {
        let bounds = CGRect(x: -1200, y: -40, width: 3202, height: 35)
        let sources = ScreenshotAgentSources(), birth = Date(timeIntervalSince1970: 100)
        sources.displaySnapshot = { [.init(id: 7, bounds: CGRect(x: -1400, y: -100, width: 4000, height: 1000), scale: 1)] }
        sources.windowObservation = { _ in .init(owner: 55, birth: birth, bounds: bounds) }
        let entry = sources.insert(kind: "window", id: 19, pid: 55, birth: birth, bounds: bounds, scale: 1, title: "synthetic window")
        let source = try sources.resolve(XCTUnwrap(entry["source_ref"] as? String))
        let roi = try ScreenshotAgentSources.region([1, 1, 3200, 33], in: bounds.size)
        for color in [CGColor(red: 1, green: 0, blue: 0, alpha: 1), CGColor(red: 0, green: 0, blue: 1, alpha: 1)] {
            let original = try image(width: 3202, height: 35, color: color)
            let (cropped, size, actual, scale, kind) = try sources.finishCapture(original, source: source, roi: roi)
            let base = try ScreenshotAgentRendering.encode(image: cropped, pointSize: size, kind: kind, rect: actual, sourceScale: scale)
            XCTAssertEqual(base.2.source_rect_pt, [1, 1, 3200, 33])
            let rect = CGRect(x: 2.25, y: 15.75, width: 0.5, height: 0.15)
            let result = try await annotate(base, rect: rect, space: "preview_px")
            try assertCoverage(result, frame: base.2, rect: rect)
        }
    }
}
