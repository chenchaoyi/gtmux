import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotAgentSourceTests: XCTestCase {
    @MainActor func testASC003SourcesExpireAndNeverWidenChangedOwnershipOrRegion() throws {
        let s = ScreenshotAgentSources(); var clock = 100.0; s.uptime = { clock }
        let birth = Date(timeIntervalSince1970: 100)
        let bounds = CGRect(x: -1200, y: -50, width: 800, height: 600)
        let result = s.insert(kind: "window", id: 99, pid: 55, birth: birth, bounds: bounds, scale: 2, title: "synthetic")
        let ref = try XCTUnwrap(result["source_ref"] as? String); let source = try s.resolve(ref)
        XCTAssertTrue(ScreenshotAgentSources.matches(source, owner: 55, birth: birth, bounds: bounds))
        XCTAssertFalse(ScreenshotAgentSources.matches(source, owner: 56, birth: birth, bounds: bounds))
        XCTAssertFalse(ScreenshotAgentSources.matches(source, owner: 55, birth: birth.addingTimeInterval(1), bounds: bounds))
        XCTAssertFalse(ScreenshotAgentSources.matches(source, owner: 55, birth: birth, bounds: bounds.offsetBy(dx: 1, dy: 0)))
        XCTAssertEqual(try ScreenshotAgentSources.region([0, 0, 20, 30], in: bounds.size), CGRect(x: 0, y: 0, width: 20, height: 30))
        XCTAssertThrowsError(try ScreenshotAgentSources.region([-1, 0, 20, 30], in: bounds.size))
        XCTAssertThrowsError(try ScreenshotAgentSources.region([799, 0, 20, 30], in: bounds.size))
        XCTAssertThrowsError(try ScreenshotAgentSources.region([Double.nan, 0, 20, 30], in: bounds.size))
        let c = ScreenshotCapture.configuration(pointSize: bounds.size, scale: 2, window: true)
        XCTAssertEqual(c.width, 1600); XCTAssertEqual(c.height, 1200); XCTAssertTrue(c.ignoreShadowsSingleWindow)
        clock = 130; XCTAssertThrowsError(try s.resolve(ref))
    }
    @MainActor func testASC003MissingPermissionDoesNotPromptOrListOtherApps() async {
        let sources = ScreenshotAgentSources(); sources.recordingAllowed = { false }
        do { _ = try await sources.inventory(app: "synthetic.app", displays: false); XCTFail("permission fabricated") }
        catch { XCTAssertEqual((error as? ScreenshotAgentFailure)?.code, "screen-recording-unavailable") }
    }
}
