import XCTest
@testable import GtmuxBar

final class HQActivityTimelineTests: XCTestCase {
    private func detail(changes: [HQWorkspaceChange], events: [HQWorkspaceEvent] = []) -> HQWorkspaceDetail {
        HQWorkspaceDetail(matterTitles: ["a": "Investigate storage", "b": "Review release"], schema: 1,
            messages: [], turns: [], earlierAvailable: false, changes: changes, events: events, warnings: [])
    }
    func testSourcesInterleaveByClockAndKeepDuplicateLegacyEventsAndFailures() {
        let event = HQWorkspaceEvent(ts: 15, event: "future-event", summary: "Unconfirmed", pane: "%22", outcome: "refused")
        let items = HQActivityTimeline.items(detail(changes: [
            .init(seq: 1, at: 10, kind: "associate", matter: "a", detail: "execution"),
            .init(seq: 2, at: 20, kind: "hq-result", matter: "b")
        ], events: [event, event]))
        XCTAssertEqual(items.map(\.at), [20, 15, 15, 10])
        XCTAssertEqual(Set(items.map(\.id)).count, 4)
        XCTAssertEqual(items[0].context, "Review release")
        XCTAssertNil(items[1].contextID, "A journal pane cannot invent a matter association")
        XCTAssertTrue(items[1].presentation.needsAttention)
        let filtered = items.filter { $0.presentation.includes(in: .other) }
        XCTAssertEqual(filtered.count, 2)
        XCTAssertEqual(HQActivityTimeline.days(filtered).flatMap(\.episodes).flatMap(\.items).map(\.id), filtered.map(\.id))
    }
    func testOnlyAdjacentSameMatterActionsWithinFiveMinutesFormAnEpisode() {
        let items = HQActivityTimeline.items(detail(changes: [
            .init(seq: 1, at: 100, kind: "associate", matter: "a"),
            .init(seq: 2, at: 101, kind: "register", matter: "b"),
            .init(seq: 3, at: 102, kind: "hq-progress", matter: "a"),
            .init(seq: 4, at: 103, kind: "associate", matter: "a"),
            .init(seq: 5, at: 404, kind: "hq-result", matter: "a")
        ]))
        let episodes = HQActivityTimeline.days(items).flatMap(\.episodes)
        XCTAssertEqual(episodes.map { $0.items.count }, [1, 2, 1, 1])
        XCTAssertEqual(episodes.flatMap(\.items).map(\.id), items.map(\.id), "Grouping cannot drop or reorder source records")
    }
    func testCalendarBoundariesAndUnknownTimesAreExplicit() {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(secondsFromGMT: 8 * 3600)!
        let before: Int64 = 1792166399 // 2026-10-16 15:59:59 UTC
        let items = HQActivityTimeline.items(detail(changes: [
            .init(seq: 1, at: before, kind: "update", matter: "a"),
            .init(seq: 2, at: before + 2, kind: "update", matter: "a"),
            .init(seq: 3, at: 0, kind: "future-action")
        ]))
        let days = HQActivityTimeline.days(items, calendar: calendar)
        XCTAssertEqual(days.count, 3)
        XCTAssertEqual(days.last?.id, "unknown")
        XCTAssertEqual(HQActivityTimeline.clock(before, seconds: true, timeZone: calendar.timeZone), "23:59:59")
        XCTAssertEqual(HQActivityTimeline.clock(0), "—")
        XCTAssertEqual(HQActivityTimeline.dateLabel(0, zh: true), "时间未知")
        XCTAssertTrue(HQActivityTimeline.dateLabel(before + 2, zh: true, timeZone: calendar.timeZone).contains("2026年10月17日"))
        XCTAssertTrue(HQActivityTimeline.dateLabel(before + 2, zh: false, timeZone: calendar.timeZone).contains("Oct 17, 2026"))
    }
    func testFilteringCannotJoinUnrelatedEpisodesOrChangeLoadedCounts() {
        let items = HQActivityTimeline.items(detail(changes: [
            .init(seq: 1, at: 100, kind: "hq-progress", matter: "a"),
            .init(seq: 2, at: 101, kind: "associate", matter: "b"),
            .init(seq: 3, at: 102, kind: "hq-result", matter: "a")
        ]))
        let episodes = HQActivityTimeline.days(items, category: .updates).flatMap(\.episodes)
        XCTAssertEqual(episodes.count, 2, "A hidden unrelated record still separates these episodes")
        XCTAssertEqual(episodes.flatMap(\.items).map(\.id), items.filter { $0.presentation.category == .updates }.map(\.id))
        XCTAssertTrue(HQActivityTimeline.days(items, category: .maintenance).isEmpty)
    }
    func testSameSecondTiesUseSequencesOnlyWithinTheirSourceAndMissingTitleStaysUnknown() {
        var d = detail(changes: [
            .init(seq: 2, at: 100, kind: "update", matter: "missing"),
            .init(seq: 1, at: 100, kind: "update", matter: "a")
        ], events: [.init(ts: 100, seq: 999, event: "future-event")])
        let expected = HQActivityTimeline.items(d)
        d.changes.reverse()
        XCTAssertEqual(expected.map(\.id), HQActivityTimeline.items(d).map(\.id))
        XCTAssertEqual(expected.filter { $0.source == .workspace }.map(\.sequence), [2, 1])
        XCTAssertNil(expected.first { $0.contextID == "missing" }?.context)
        XCTAssertEqual(expected.count, 3)
    }
}
