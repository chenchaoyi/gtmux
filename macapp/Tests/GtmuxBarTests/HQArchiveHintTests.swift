import XCTest
@testable import GtmuxBar

final class HQArchiveHintTests: XCTestCase {
    @MainActor func testBackupObservationSurvivesLegacyWireTextAndLanguageChanges() throws {
        let language = L10n.shared.mode
        defer { L10n.shared.mode = language }
        let state = try XCTUnwrap(decodeHQMemoryState(Data(#"{"exists":true,"off_machine":"Time Machine is configured"}"#.utf8)))
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            XCTAssertTrue(try XCTUnwrap(hqArchiveBackupHint(state.offMachine, .shared)).contains("Time Machine"))
            XCTAssertNotNil(hqArchiveBackupHint("有同步盘，导一份过去", .shared))
            XCTAssertNotNil(hqArchiveBackupHint("a synced folder exists; put an export there", .shared))
            XCTAssertEqual(hqArchiveBackupHint("A future backup observation", .shared), "A future backup observation")
            XCTAssertNil(hqArchiveBackupHint("", .shared), "Missing data cannot invent a backup conclusion")
            XCTAssertFalse(try XCTUnwrap(hqArchiveBackupHint("nothing carries it off this disk", .shared)).contains("carries"))
        }
    }
}
