import XCTest
@testable import GtmuxBar

final class PairStoreTests: XCTestCase {
    // parseDevices keeps OWNER entries only (guests live in ShareStore) and maps
    // the /api/devices wire shape.
    func testParseDevicesFiltersGuests() throws {
        let json = """
        {"devices":[
          {"id":"d1","name":"Demo iPhone","enrolledAt":100,"lastSeen":200},
          {"id":"g1","name":"Alice","enrolledAt":150,"scope":"guest","viewPanes":["%1"]},
          {"id":"d2","name":"work-laptop","enrolledAt":300}
        ]}
        """
        let devices = try XCTUnwrap(PairStore.parseDevices(Data(json.utf8)))
        XCTAssertEqual(devices.map(\.id), ["d1", "d2"])
        XCTAssertEqual(devices[0].lastSeen, 200)
    }

    // kind guesses the row icon from the device name — chrome only, best-effort.
    func testDeviceKind() {
        XCTAssertEqual(PairedDevice(id: "1", name: "Demo iPhone 15", enrolledAt: 0, lastSeen: 0, platform: "", lastIP: "").kind, "iphone")
        XCTAssertEqual(PairedDevice(id: "2", name: "Safari · macOS", enrolledAt: 0, lastSeen: 0, platform: "", lastIP: "").kind, "globe")
        XCTAssertEqual(PairedDevice(id: "3", name: "work-mbp", enrolledAt: 0, lastSeen: 0, platform: "", lastIP: "").kind, "laptopcomputer")
    }
}

// The roster's job is letting you tell YOUR devices apart well enough to revoke the right
// one. Every entry used to read `gtmux • iPhone`: a "gtmux" prefix inside gtmux's own
// roster (nothing in that list is not a gtmux device) over a word true of every iPhone.
final class PairedDeviceNameTests: XCTestCase {
    private func dev(_ name: String) -> PairedDevice {
        PairedDevice(id: "d1", name: name, enrolledAt: 0, lastSeen: 0, platform: "", lastIP: "")
    }

    func testLegacyPrefixIsStrippedForDisplay() {
        // Entries paired before the rename tidy themselves up — no re-pairing asked.
        XCTAssertEqual(dev("gtmux • iPhone").displayName, "iPhone")
        XCTAssertEqual(dev("gtmux · iPad").displayName, "iPad")
        XCTAssertEqual(dev("gtmux iPhone").displayName, "iPhone")
        XCTAssertEqual(dev("GTMUX • iPhone · iOS 18.5").displayName, "iPhone · iOS 18.5")
    }

    func testANameWithoutThePrefixIsUntouched() {
        XCTAssertEqual(dev("iPhone · iOS 18.5").displayName, "iPhone · iOS 18.5")
        XCTAssertEqual(dev("dev-mbp.local").displayName, "dev-mbp.local")
        // A device legitimately NAMED after the tool keeps something to show, rather
        // than being stripped to an empty row.
        XCTAssertEqual(dev("gtmux").displayName, "gtmux")
    }

    func testTheIconStillResolvesFromTheNewNaming() {
        XCTAssertEqual(dev("iPhone · iOS 18.5").kind, "iphone")
        XCTAssertEqual(dev("iPad · iOS 18.5").kind, "iphone")
        XCTAssertEqual(dev("Safari").kind, "globe")
        XCTAssertEqual(dev("dev-mbp.local").kind, "laptopcomputer")
    }
}

/// The row's second line, and the icon, now come from what the device IS.
///
/// The phone app had been showing the platform for months off the same `/api/devices`
/// payload the menu bar simply did not decode: two surfaces, one dataset, different
/// answers. These pin the shared builder so they cannot drift apart again.
final class PairedDeviceSubtitleTests: XCTestCase {
    private func tr(_ en: String, _ zh: String) -> String { en }

    func testSubtitleJoinsWhatIsKnown() {
        let d = PairedDevice(id: "1", name: "ccy", enrolledAt: 0, lastSeen: 1_000,
                             platform: "iOS 26.6", lastIP: "192.168.1.23")
        let s = d.subtitle(now: 1_060, tr: tr)
        XCTAssertTrue(s.contains("iOS 26.6"), s)
        XCTAssertTrue(s.contains("192.168.1.23"), "the address is the point — an unexpected one is what you act on")
        XCTAssertTrue(s.contains("last seen"), s)
    }

    /// A device the Mac has not heard from since it started recording platforms has
    /// neither — the line must still say the one thing it knows.
    func testSubtitleSurvivesAnEmptyRecord() {
        let d = PairedDevice(id: "1", name: "browser", enrolledAt: 5, lastSeen: 0, platform: "", lastIP: "")
        XCTAssertEqual(d.subtitle(now: 100, tr: tr), "never connected")
    }

    /// The icon follows the platform, not the name a device chose for itself: a phone
    /// paired as "ccy" used to get a laptop.
    func testIconPrefersThePlatform() {
        XCTAssertEqual(PairedDevice(id: "1", name: "ccy", enrolledAt: 0, lastSeen: 0,
                                    platform: "iOS 26.6", lastIP: "").kind, "iphone")
        XCTAssertEqual(PairedDevice(id: "2", name: "ccy", enrolledAt: 0, lastSeen: 0,
                                    platform: "Chrome 141 · macOS", lastIP: "").kind, "globe")
        // No platform yet → the old name guess still applies rather than nothing.
        XCTAssertEqual(PairedDevice(id: "3", name: "Demo iPhone", enrolledAt: 0, lastSeen: 0,
                                    platform: "", lastIP: "").kind, "iphone")
    }
}

// The pairing QR used to carry the code minted when its window opened, for as long as
// the window stayed open. A code expires after 5 minutes, works once, and lives only in
// the serve's memory, so any of those three left a QR that looked fine and failed every
// scan. codeNeedsRenewal is the rule that replaces it in time.
final class PairingCodeRenewalTests: XCTestCase {
    private let t0 = Date(timeIntervalSince1970: 1_000_000)

    private func code(boot: String? = "b1", ttl: TimeInterval = 300) -> Pairing.EnrollCode {
        Pairing.EnrollCode(code: "c0ffee", mintedAt: t0, expiresAt: t0.addingTimeInterval(ttl), boot: boot)
    }

    func testAFreshCodeIsKept() {
        XCTAssertFalse(Pairing.codeNeedsRenewal(code(), boot: "b1", newestEnrolledAt: nil,
                                                now: t0.addingTimeInterval(30)))
    }

    func testNoCodeMeansMint() {
        XCTAssertTrue(Pairing.codeNeedsRenewal(nil, boot: "b1", newestEnrolledAt: nil, now: t0))
    }

    // Renewed a minute early, so the QR never shows a code with seconds left; the old
    // one keeps working until its own expiry for a phone that caught it just before.
    func testRenewedAMinuteBeforeItExpires() {
        XCTAssertFalse(Pairing.codeNeedsRenewal(code(), boot: "b1", newestEnrolledAt: nil,
                                                now: t0.addingTimeInterval(239)))
        XCTAssertTrue(Pairing.codeNeedsRenewal(code(), boot: "b1", newestEnrolledAt: nil,
                                               now: t0.addingTimeInterval(240)))
    }

    // What happened on 2026-09-19: `gtmux update` restarted the serve, every code went
    // with it, and the window kept showing one.
    func testARestartedServeRetiresTheCode() {
        XCTAssertTrue(Pairing.codeNeedsRenewal(code(boot: "b1"), boot: "b2", newestEnrolledAt: nil,
                                               now: t0.addingTimeInterval(10)))
    }

    // An older serve reports no boot; the clock is then all there is, and it still works.
    func testAServeWithoutABootFallsBackToTheClock() {
        XCTAssertFalse(Pairing.codeNeedsRenewal(code(boot: nil), boot: nil, newestEnrolledAt: nil,
                                                now: t0.addingTimeInterval(10)))
        XCTAssertTrue(Pairing.codeNeedsRenewal(code(boot: nil), boot: nil, newestEnrolledAt: nil,
                                               now: t0.addingTimeInterval(299)))
    }

    // Pairing the iPhone spends the code; the iPad scanning the same window next needs
    // another one.
    func testADeviceEnrollingAfterTheMintSpendsTheCode() {
        let minted = Int(t0.timeIntervalSince1970)
        XCTAssertFalse(Pairing.codeNeedsRenewal(code(), boot: "b1", newestEnrolledAt: minted - 50,
                                                now: t0.addingTimeInterval(20)),
                       "a device paired before this code existed says nothing about it")
        XCTAssertTrue(Pairing.codeNeedsRenewal(code(), boot: "b1", newestEnrolledAt: minted + 12,
                                               now: t0.addingTimeInterval(20)))
    }

    func testParseMintKeepsTheExpiryAndBoot() throws {
        let json = #"{"enrollCode":"abc123","expiresInSec":300,"boot":"f00d"}"#
        let c = try XCTUnwrap(Pairing.parseMint(Data(json.utf8), now: t0))
        XCTAssertEqual(c.code, "abc123")
        XCTAssertEqual(c.expiresAt, t0.addingTimeInterval(300))
        XCTAssertEqual(c.boot, "f00d")
    }

    func testParseMintFromAnOlderServe() throws {
        let c = try XCTUnwrap(Pairing.parseMint(Data(#"{"enrollCode":"abc123"}"#.utf8), now: t0))
        XCTAssertEqual(c.expiresAt, t0.addingTimeInterval(300), "every serve has used 5 minutes")
        XCTAssertNil(c.boot)
        XCTAssertNil(Pairing.parseMint(Data(#"{"enrollCode":""}"#.utf8), now: t0))
    }

    func testParseBoot() {
        XCTAssertEqual(Pairing.parseBoot(Data(#"{"status":"ok","service":"gtmux","boot":"f00d"}"#.utf8)), "f00d")
        XCTAssertNil(Pairing.parseBoot(Data(#"{"status":"ok","service":"gtmux"}"#.utf8)))
    }
}
