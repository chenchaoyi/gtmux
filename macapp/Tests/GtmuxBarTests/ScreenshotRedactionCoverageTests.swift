import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotRedactionCoverageTests: XCTestCase {
    private func render(color: CGColor, deviceScale: CGFloat, rect: CGRect) throws -> (CGImage, CGImage) {
        let size = Int(16 * deviceScale)
        let context = try XCTUnwrap(CGContext(data: nil, width: size, height: size, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(color)
        context.fill(CGRect(x: 0, y: 0, width: size, height: size))
        let original = try XCTUnwrap(context.makeImage())
        context.translateBy(x: 0, y: CGFloat(size))
        context.scaleBy(x: deviceScale, y: -deviceScale)
        // The source scale can differ from the current editor preview's CTM.
        AnnotationRenderer.draw(Annotation(.redact(rect), color: .yellow), in: context, source: .init(image: original, scale: 2))
        return (original, try XCTUnwrap(context.makeImage()))
    }

    func testASC008FractionalRedactionOverwritesEveryIntersectedDevicePixel() throws {
        let rect = CGRect(x: 2.25, y: 4.25, width: 0.5, height: 0.5)
        for scale: CGFloat in [1, 2, 0.5] {
            let redImages = try render(color: CGColor(red: 1, green: 0, blue: 0, alpha: 1), deviceScale: scale, rect: rect)
            let blueImages = try render(color: CGColor(red: 0, green: 0, blue: 1, alpha: 1), deviceScale: scale, rect: rect)
            let red = NSBitmapImageRep(cgImage: redImages.1), redOriginal = NSBitmapImageRep(cgImage: redImages.0)
            let blue = NSBitmapImageRep(cgImage: blueImages.1), blueOriginal = NSBitmapImageRep(cgImage: blueImages.0)
            let x0 = Int(floor(rect.minX * scale)), x1 = Int(ceil(rect.maxX * scale))
            let y0 = Int(floor(rect.minY * scale)), y1 = Int(ceil(rect.maxY * scale))
            for y in 0..<red.pixelsHigh {
                for x in 0..<red.pixelsWide {
                    let a = try XCTUnwrap(red.colorAt(x: x, y: y)?.usingColorSpace(.sRGB))
                    let b = try XCTUnwrap(blue.colorAt(x: x, y: y)?.usingColorSpace(.sRGB))
                    if x >= x0 && x < x1 && y >= y0 && y < y1 {
                        for c in [a, b] {
                            XCTAssertEqual(c.redComponent, 0, accuracy: 0.001, "scale=\(scale), pixel=\(x),\(y)")
                            XCTAssertEqual(c.greenComponent, 0, accuracy: 0.001)
                            XCTAssertEqual(c.blueComponent, 0, accuracy: 0.001)
                            XCTAssertEqual(c.alphaComponent, 1, accuracy: 0.001)
                        }
                    } else {
                        for (actual, baseline) in [(a, redOriginal), (b, blueOriginal)] {
                            let expected = try XCTUnwrap(baseline.colorAt(x: x, y: y)?.usingColorSpace(.sRGB))
                            XCTAssertEqual(actual.redComponent, expected.redComponent, accuracy: 0.001)
                            XCTAssertEqual(actual.greenComponent, expected.greenComponent, accuracy: 0.001)
                            XCTAssertEqual(actual.blueComponent, expected.blueComponent, accuracy: 0.001)
                            XCTAssertEqual(actual.alphaComponent, expected.alphaComponent, accuracy: 0.001)
                        }
                    }
                }
            }
        }
    }
    @MainActor func testASC008DerivedArtifactPreviewKeepsSmallRedactionsOpaque() async throws {
        for color in [CGColor(red: 1, green: 0, blue: 0, alpha: 1), CGColor(red: 0, green: 0, blue: 1, alpha: 1)] {
            let ctx = try XCTUnwrap(CGContext(data: nil, width: 3200, height: 32, bitsPerComponent: 8, bytesPerRow: 0,
                space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
            ctx.setFillColor(color); ctx.fill(CGRect(x: 0, y: 0, width: 3200, height: 32))
            let encoded = try ScreenshotAgentRendering.encode(image: XCTUnwrap(ctx.makeImage()), pointSize: CGSize(width: 3200, height: 32), kind: "imported")
            let spec = try ScreenshotAgentRenderingTests.spec(frame: encoded.2, marks: [["type": "redact", "x": 2.25, "y": 4.25, "width": 0.5, "height": 0.5]])
            let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            defer {try? FileManager.default.removeItem(at: dir)}
            let input = dir.appendingPathComponent("input.png"), preview = dir.appendingPathComponent("preview.png")
            try encoded.0.write(to: input)
            let source = open(input.path, O_RDONLY)
            let full = open(dir.appendingPathComponent("full.png").path, O_CREAT|O_EXCL|O_WRONLY, 0o600)
            let small = open(preview.path, O_CREAT|O_EXCL|O_WRONLY, 0o600)
            XCTAssertGreaterThanOrEqual(source, 0); XCTAssertGreaterThanOrEqual(full, 0); XCTAssertGreaterThanOrEqual(small, 0)
            defer {close(source); close(full); close(small)}
            let service = ScreenshotAgentService { _, _ in XCTFail("annotation opened an editor") }
            let request = ScreenshotAgentRequest(schema_version: 1, request_id: String(repeating: "a", count: 32), host_nonce: "fixture", operation: "annotate", deadline_ms: 0,
                app: nil, source_kind: nil, source_ref: nil, rect: nil, image_sha256: encoded.2.image_sha256, document: encoded.2, spec: spec)
            _ = try await service.perform(request, files: [source, full, small], budget: .init(end: ProcessInfo.processInfo.systemUptime+3))
            let result = try ScreenshotAgentRendering.decode(Data(contentsOf: preview)).0
            XCTAssertEqual(result.width, 1600)
            let pixel = try XCTUnwrap(NSBitmapImageRep(cgImage: result).colorAt(x: 1, y: 2)?.usingColorSpace(.sRGB))
            XCTAssertEqual(pixel.redComponent, 0, accuracy: 0.001)
            XCTAssertEqual(pixel.greenComponent, 0, accuracy: 0.001)
            XCTAssertEqual(pixel.blueComponent, 0, accuracy: 0.001)
            XCTAssertEqual(pixel.alphaComponent, 1, accuracy: 0.001)
        }
    }

}
