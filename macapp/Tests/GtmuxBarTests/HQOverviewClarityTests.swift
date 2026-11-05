import Foundation
import XCTest

@testable import GtmuxBar

func overviewClaritySnapshot(count: Int = 26) throws -> HQWorkspaceSnapshot {
    let now = Int64(Date().timeIntervalSince1970)
    var matters: [[String: Any]] = (0..<count).map { i in
        [
            "id": "open-\(i)", "title": "Synthetic ongoing goal \(i)", "state": "open",
            "assessment": "Engineering is checking the remaining installation issue. HQ owns the next step.",
            "created_at": now - 3600, "updated_at": now - Int64(i), "revision": i + 1, "participants": [],
        ]
    }
    for state in ["completed", "closed"] {
        matters.append([
            "id": state, "title": "Delivered synthetic goal", "state": state,
            "created_at": now - 3600, "updated_at": now + 1, "revision": 100, "participants": [],
        ])
    }
    matters.append([
        "id": "merged", "title": "Merged synthetic goal", "state": "open", "merged_into": "open-0",
        "created_at": now - 3600, "updated_at": now + 2, "revision": 101, "participants": [],
    ])
    let item: [String: Any] = [
        "matter_id": "open-0",
        "preview": ["kind": "result", "body": "Cleanup stage completed", "at": now - 30],
    ]
    let value: [String: Any] = [
        "schema": 1, "revision": 101, "matters": matters, "conversations": [],
        "overview": [
            "generated_at": now, "since": now - 86400, "human_input": [], "awaiting_hq": [],
            "needs_input": [],
            "following": [["matter_id": "completed"], item, item, ["matter_id": "missing"]], "recent": [],
            "recent_total": 0,
        ],
    ]
    let decoder = JSONDecoder()
    decoder.keyDecodingStrategy = .convertFromSnakeCase
    return try decoder.decode(HQWorkspaceSnapshot.self, from: JSONSerialization.data(withJSONObject: value))
}

final class HQOverviewClarityTests: XCTestCase {
    func testCanonicalGroupsAndHiddenCountIgnoreEndedMergedStaleAndDuplicateRows() throws {
        var snapshot = try overviewClaritySnapshot()
        let matters = HQMatterGroup.following.matters(in: snapshot)
        XCTAssertEqual(matters.count, 26)
        XCTAssertTrue(matters.allSatisfy { $0.state == "open" && $0.mergedInto == nil })
        let items = HQOverviewModel.items(.following, in: snapshot)
        XCTAssertEqual(items.map(\.matterId), ["open-0", "open-1", "open-2"])
        XCTAssertEqual(items.first?.preview?.body, "Cleanup stage completed")
        XCTAssertEqual(matters.first?.state, "open", "A completed stage cannot finish its continuing goal")
        XCTAssertEqual(HQOverviewModel.remaining(.following, in: snapshot), 23)
        XCTAssertTrue(
            HQMatterGroup.needsInput.matters(in: snapshot).isEmpty,
            "Engineering blockers are not invented human questions")
        snapshot.overview?.following = []
        XCTAssertEqual(HQOverviewModel.items(.following, in: snapshot).count, 3)
        snapshot = try overviewClaritySnapshot(count: 2)
        XCTAssertEqual(HQOverviewModel.items(.following, in: snapshot).count, 2)
        XCTAssertEqual(HQOverviewModel.remaining(.following, in: snapshot), 0)
    }
    func testCurrentAssessmentTakesPriorityWithoutBorrowingReceiptOrMetadataTime() throws {
        let snapshot = try overviewClaritySnapshot()
        var matter = try XCTUnwrap(snapshot.matters.first)
        let item = try XCTUnwrap(HQOverviewModel.items(.following, in: snapshot).first)
        let preview = try XCTUnwrap(HQOverviewModel.currentPreview(item, matter: matter))
        XCTAssertTrue(preview.body.contains("remaining installation issue"))
        XCTAssertFalse(preview.body.contains("Cleanup stage completed"))
        XCTAssertNil(preview.at, "A rename/update cannot date an assessment")
        XCTAssertNil(preview.messageId, "A receipt must not be attributed to different text")
        matter.assessment = "   "
        XCTAssertEqual(HQOverviewModel.currentPreview(item, matter: matter)?.body, item.preview?.body)
        XCTAssertEqual(HQOverviewModel.currentPreview(item, matter: matter)?.at, item.preview?.at)
    }
}
