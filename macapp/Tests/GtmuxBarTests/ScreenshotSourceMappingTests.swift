import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotSourceMappingTests: XCTestCase {
    @MainActor func testASC011ChangedDisplayMappingAndScaleRefuseBeforeAndAfterCapture() throws {
        let sources = ScreenshotAgentSources()
        let bounds = CGRect(x: -80, y: -10, width: 80, height: 40)
        let displayBounds = CGRect(x: -100, y: -100, width: 200, height: 200)
        let birth = Date(timeIntervalSince1970: 100)
        var snapshot = [ScreenshotAgentSources.DisplayMapping(id: 1, bounds: displayBounds, scale: 1)]
        sources.displaySnapshot = { snapshot }; sources.displayActive = { _ in true }
        sources.windowObservation = { _ in .init(owner: 55, birth: birth, bounds: bounds) }
        let value = sources.insert(kind: "window", id: 99, pid: 55, birth: birth, bounds: bounds, scale: 1, title: "fixture")
        let source = try sources.resolve(XCTUnwrap(value["source_ref"] as? String))
        XCTAssertTrue(sources.unchanged(source))
        let roi = CGRect(origin: .zero, size: bounds.size)
        XCTAssertNoThrow(try sources.finishCapture(ScreenshotAgentRenderingTests.image(), source: source, roi: roi))
        for changed in [
            ScreenshotAgentSources.DisplayMapping(id: 1, bounds: displayBounds, scale: 2),
            .init(id: 2, bounds: displayBounds, scale: 1),
            .init(id: 1, bounds: displayBounds.offsetBy(dx: 1, dy: 0), scale: 1)
        ] {
            snapshot = [changed]
            XCTAssertFalse(sources.unchanged(source), "same point bounds must not authorize a changed display")
            XCTAssertThrowsError(try sources.finishCapture(ScreenshotAgentRenderingTests.image(), source: source, roi: roi)) {
                XCTAssertEqual(($0 as? ScreenshotAgentFailure)?.code, "source-changed")
            }
        }
        snapshot = []
        XCTAssertFalse(sources.unchanged(source), "missing display is not an implicit 1x mapping")
        snapshot = [.init(id: 1, bounds: displayBounds, scale: 1)]
        sources.windowObservation = { _ in .init(owner: 56, birth: birth, bounds: bounds) }
        XCTAssertFalse(sources.unchanged(source), "mapping validation must preserve owner checks")
        sources.windowObservation = { _ in .init(owner: 55, birth: birth.addingTimeInterval(1), bounds: bounds) }
        XCTAssertFalse(sources.unchanged(source), "process birth is still required")
        sources.windowObservation = { _ in .init(owner: 55, birth: birth, bounds: bounds) }
        let context = try XCTUnwrap(CGContext(data: nil, width: 160, height: 80, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        XCTAssertThrowsError(try sources.finishCapture(XCTUnwrap(context.makeImage()), source: source, roi: roi)) {
            XCTAssertEqual(($0 as? ScreenshotAgentFailure)?.code, "capture-geometry")
        }
        let display = sources.insert(kind: "display", id: 1, pid: 0, birth: nil, bounds: displayBounds, scale: 1, title: "fixture display")
        let displaySource = try sources.resolve(XCTUnwrap(display["source_ref"] as? String))
        XCTAssertTrue(sources.unchanged(displaySource))
        snapshot = [.init(id: 1, bounds: displayBounds, scale: 2)]
        XCTAssertFalse(sources.unchanged(displaySource), "display capture must also compare scale")
    }
}
