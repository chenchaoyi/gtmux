import XCTest
@testable import GtmuxBar

// The menu bar renders `gtmux tunnel --servers --json` (openspec/changes/direct-server-choice).
// It is a CONSUMER of that command, so the only thing that can break quietly here is the
// decoding: a Go field with omitempty that Swift declared non-optional empties the whole
// panel and says nothing (the "0 panes" footgun). These pin the shape the CLI emits.

final class DirectServersTests: XCTestCase {
    private let sample = """
    {
      "servers": [
        {
          "id": "sh",
          "url": "https://sh.example.test",
          "region": "cn-shanghai",
          "name": "上海",
          "current": true,
          "accepting": true,
          "answering": true,
          "rtt_ms": 24
        },
        {
          "id": "la",
          "url": "https://la.example.test",
          "name": "la",
          "current": false,
          "accepting": false,
          "answering": false
        }
      ],
      "current": "sh"
    }
    """

    func testDecodesWhatTheCLIPrints() throws {
        let servers = try XCTUnwrap(DirectServerStore.decode(Data(sample.utf8)))
        XCTAssertEqual(servers.count, 2)
        XCTAssertEqual(servers[0].id, "sh")
        XCTAssertEqual(servers[0].name, "上海")
        XCTAssertTrue(servers[0].current)
        XCTAssertEqual(servers[0].rttMS, 24)
        // A server that did not answer carries no round trip at all, and must still decode.
        XCTAssertNil(servers[1].rttMS)
        XCTAssertNil(servers[1].region)
        XCTAssertFalse(servers[1].accepting)
    }

    func testAServerThatDidNotAnswerSaysSoInsteadOfShowingATime() {
        let l10n = L10n.shared
        l10n.mode = .en
        defer { l10n.mode = .en }
        let quiet = DirectServer(id: "la", url: "https://la.example.test", region: nil,
                                 name: "la", current: false, accepting: true,
                                 answering: false, rttMS: nil)
        XCTAssertEqual(quiet.roundTrip(l10n), "no answer")
        l10n.mode = .zh
        XCTAssertEqual(quiet.roundTrip(l10n), "没有回应")
        l10n.mode = .en

        let live = DirectServer(id: "sh", url: "https://sh.example.test", region: "cn-shanghai",
                                name: "上海", current: true, accepting: true,
                                answering: true, rttMS: 24)
        XCTAssertEqual(live.roundTrip(l10n), "24 ms")
    }

    func testNonsenseFromTheCLIIsNoListRatherThanAWrongOne() {
        XCTAssertNil(DirectServerStore.decode(Data("not json".utf8)))
        XCTAssertNil(DirectServerStore.decode(Data("{}".utf8)))
    }

    func testMovingSaysWhatItCostsBeforeItHappens() {
        L10n.shared.mode = .zh
        defer { L10n.shared.mode = .en }
        let s = DirectServer(id: "la", url: "https://la.example.test", region: "us-west",
                             name: "洛杉矶", current: false, accepting: true,
                             answering: true, rttMS: 186)
        let alert = directMoveConfirmation(s, l10n: L10n.shared)
        XCTAssertTrue(alert.messageText.contains("洛杉矶"))
        // The two things a reader must know before saying yes.
        XCTAssertTrue(alert.informativeText.contains("重新扫"))
        XCTAssertTrue(alert.informativeText.contains("分享链接"))
    }
}

// The pairing window after a move (openspec/changes/direct-server-choice). Moving takes a
// few seconds during which the address really is not answering; saying "can't reach it
// yet" there sends the reader looking for a fault that is not happening. On 2026-09-23 the
// window did exactly that, with the OLD address still on screen.
final class PairingAfterMoveTests: XCTestCase {
    func testNotAnsweringIsTheOnlyStateAMoveCanBeMistakenFor() {
        // These are the verdicts that mean "the address said nothing", and only these may
        // be re-read as "reconnecting" inside the window after a move.
        XCTAssertTrue(ReachVerdict.cannotReachYet.isNotReachable)
        XCTAssertTrue(ReachVerdict.tunnelDown("boom").isNotReachable)
        // A reachable address is reachable, whatever just happened.
        XCTAssertFalse(ReachVerdict.reachable.isNotReachable)
        // Still checking is not a failure, and neither is "this Mac cannot see its own
        // address but the tunnel is up" — a phone connects in that state.
        XCTAssertFalse(ReachVerdict.checking.isNotReachable)
        XCTAssertFalse(ReachVerdict.tunnelUpMacCannotSee.isNotReachable)
    }

    func testTheWindowNamesWhereItMovedTo() {
        let store = DirectServerStore()
        store.servers = [
            DirectServer(id: "sh", url: "https://sh.example.test", region: "cn-shanghai",
                         name: "上海", current: true, accepting: true, answering: true, rttMS: 135),
            DirectServer(id: "la", url: "https://la.example.test", region: "us-west",
                         name: "United States (West)", current: false, accepting: true,
                         answering: true, rttMS: 360),
        ]
        XCTAssertEqual(store.currentName(L10n.shared), "上海")
    }

    func testWithNoServerInUseThereIsNoPlaceToName() {
        // The sentence then has to work without one, so the store must say so rather than
        // hand back an id.
        XCTAssertEqual(DirectServerStore().currentName(L10n.shared), "")
    }
}

// The window's own honesty checks (the 2026-09-23 design pass): the row in use must read
// loudest rather than faded, a measurement has to say when it was taken, and a code that
// expires has to count down rather than repeat "5 minutes" until it dies.
final class PairingWindowHonestyTests: XCTestCase {
    func testTheRowInUseIsNotTheDimmestOne() throws {
        // The row in use takes no clicks — it is where you already are — but it must not be
        // DISABLED, which is what greyed it out. The distinction lives in the view; what
        // this pins is the rule the view follows: current means "not clickable", and
        // nothing else about the row changes to say so.
        let current = DirectServer(id: "sh", url: "https://sh.example.test", region: nil, name: "上海",
                                   current: true, accepting: true, answering: true, rttMS: 137)
        let other = DirectServer(id: "la", url: "https://la.example.test", region: nil, name: "美国西部",
                                 current: false, accepting: true, answering: true, rttMS: 428)
        XCTAssertFalse(pickableRoute(current), "the row you are on is not a choice")
        XCTAssertTrue(pickableRoute(other))
    }

    func testACountdownIsNotAConstant() {
        let expires = Date().addingTimeInterval(272) // 4:32
        XCTAssertEqual(codeLeft(expires, now: expires.addingTimeInterval(-272)), "4:32")
        XCTAssertEqual(codeLeft(expires, now: expires.addingTimeInterval(-59)), "0:59")
        XCTAssertEqual(codeLeft(expires, now: expires.addingTimeInterval(-5)), "0:05")
        // Past its end there is no time left to show; the window says it is renewing.
        XCTAssertNil(codeLeft(expires, now: expires.addingTimeInterval(1)))
    }
}
