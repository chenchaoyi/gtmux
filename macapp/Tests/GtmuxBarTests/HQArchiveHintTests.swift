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

final class HQAutomaticBackupContractTests: XCTestCase {
    @MainActor func testLegacyArchivesAreNotPresentedAsVerifiedAutomaticPoints() throws {
        let data = Data(#"{"exists":true,"snapshots":17,"automatic_backup":{"policy":{"enabled":true,"keep":3,"maxMB":64},"managed_points":3,"legacy_archives":14}}"#.utf8)
        let state = try XCTUnwrap(decodeHQMemoryState(data))
        XCTAssertEqual(state.automatic?.points, 3)
        XCTAssertEqual(state.automatic?.legacy, 14)
        XCTAssertEqual(state.automatic?.keep, 3)
        XCTAssertEqual(state.automatic?.maxMB, 64)
    }
    @MainActor func testOlderCoreKeepsUnknownValidity() throws {
        let state = try XCTUnwrap(decodeHQMemoryState(Data(#"{"exists":true,"snapshots":14}"#.utf8)))
        XCTAssertNil(state.automatic)
        XCTAssertEqual(state.snapshots, 14)
    }
}
