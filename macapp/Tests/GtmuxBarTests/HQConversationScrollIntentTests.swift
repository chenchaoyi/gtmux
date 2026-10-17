import AppKit
import XCTest
@testable import GtmuxBar

@MainActor final class HQConversationScrollIntentTests: XCTestCase {
    func testGestureAndLegacyMouseEventsInvalidateTheSameStableIntent() {
        let center = NotificationCenter()
        let intent = HQConversationScrollIntent(center: center)
        center.post(name: Notification.Name("unrelated"), object: nil)
        XCTAssertEqual(intent.generation, 0)
        center.post(name: NSScrollView.willStartLiveScrollNotification, object: nil)
        XCTAssertEqual(intent.generation, 1)
        center.post(name: NSScrollView.didLiveScrollNotification, object: nil)
        XCTAssertEqual(intent.generation, 2)
    }
    func testObservationDoesNotRetainAnExitedReader() {
        let center = NotificationCenter()
        var intent: HQConversationScrollIntent? = HQConversationScrollIntent(center: center)
        weak var reference = intent
        XCTAssertNotNil(reference)
        intent = nil
        XCTAssertNil(reference)
        center.post(name: NSScrollView.didLiveScrollNotification, object: nil)
    }
}
