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
