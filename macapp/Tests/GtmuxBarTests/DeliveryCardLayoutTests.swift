import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

/// The delivery card has to FIT, and it has to SHOW what it hands over.
///
/// It did not fit: a 168pt QR with the browser URL, the terminal one-liner and a read-out
/// line crammed into the 240pt column beside it. The read-out line was 15pt monospace
/// with no line limit, so on a real link it was cut, and what got cut was the code itself
/// (「这个UI都展示不全」, 2026-09-21).
///
/// Rebuilt as three cards it fit, but the two text cards then said only "Copy link" and
/// "Copy command", so the command was written nowhere on the screen 「这里能否把具体的链
/// 接与命令也展示出来」(2026-09-21). A caption that names a medium is not the value, and
/// a verb on a button is not the value either — neither of them goes red when the value
/// stops being drawn.
///
/// So the assertions are: the card holds its width in either language, a value too long
/// for the card grows it DOWNWARD instead of being clipped, and each value is actually on
/// the card — pinned by rendering the same card twice with only that value changed and
/// requiring the two images to differ. A card that stopped drawing one would render
/// identically both times.
///
/// All of it through ImageRenderer, which needs no screen and no permission.
final class DeliveryCardLayoutTests: XCTestCase {
    private let short = "https://x.dev/p1#code=GM4W-HCCQ"
    private let long = "https://gtmux.a-rather-long-self-hosted-domain.example.dev/p35047#code=GM4W-HCCQ"
    /// Long enough that its wrapped value outgrows the QR beside it, which is what sets
    /// the card's floor height.
    private let huge = "https://gtmux.a-very-long-label-a-very-long-label-a-very-long-label"
        + "-a-very-long-label-a-very-long-label.example.dev/p35047#code=GM4W-HCCQ"

    @MainActor private func render(link: String, terminal: String? = nil,
                                   qr: String? = nil, width: CGFloat = 560) -> NSImage? {
        let v = CodeDeliveryBlock(
            l10n: L10n.shared,
            qrText: qr ?? link,
            linkValue: link,
            terminalValue: terminal ?? "gtmux attach '\(link)'")
            .padding(18)
            .frame(width: width)
        let r = ImageRenderer(content: v)
        r.scale = 1
        return r.nsImage
    }

    /// PNG bytes, so two renders can be compared for "is this value drawn at all".
    @MainActor private func pixels(link: String, terminal: String? = nil, qr: String? = nil) -> Data? {
        guard let img = render(link: link, terminal: terminal, qr: qr),
              let tiff = img.tiffRepresentation,
              let rep = NSBitmapImageRep(data: tiff) else { return nil }
        return rep.representation(using: .png, properties: [:])
    }

    @MainActor func testTheCardHoldsItsWidthAndGrowsDownward() throws {
        guard let a = render(link: short), let b = render(link: huge) else {
            XCTFail("the card did not render"); return
        }
        // Neither link may push the card wider than the sheet that holds it.
        XCTAssertEqual(a.size.width, 560, accuracy: 0.5)
        XCTAssertEqual(b.size.width, 560, accuracy: 0.5)
        // A value too long for the space beside the QR WRAPS and takes the card with it.
        // Equal heights would mean the overflow went somewhere invisible, which is the
        // defect this card was rebuilt for.
        XCTAssertGreaterThan(b.size.height, a.size.height,
                             "a long link did not grow the card — it is being clipped again")
    }

    /// The terminal one-liner is the value that was nowhere on the old card, and it is the
    /// one nothing else on the screen repeats.
    @MainActor func testTheCommandIsOnTheCard() throws {
        let withShort = pixels(link: short, terminal: "gtmux attach '\(short)'", qr: short)
        let withLong = pixels(link: short, terminal: "gtmux attach '\(long)'", qr: short)
        XCTAssertNotNil(withShort)
        XCTAssertNotEqual(withShort, withLong,
                          "changing the command changed nothing on the card — it is not being shown")
    }

    /// And the link, which the headline above the doors used to be responsible for.
    @MainActor func testTheLinkIsOnTheCard() throws {
        let terminal = "gtmux attach '\(short)'" // held still, so only the link differs
        let a = pixels(link: short, terminal: terminal, qr: short)
        let b = pixels(link: long, terminal: terminal, qr: short)
        XCTAssertNotNil(a)
        XCTAssertNotEqual(a, b,
                          "changing the link changed nothing on the card — it is not being shown")
    }

    /// Chinese labels are wider than their English counterparts, and the captions share a
    /// row with a copy button, so the language must not change the card's width.
    @MainActor func testChineseDoesNotWidenIt() throws {
        let en = render(link: short)
        L10n.shared.mode = .zh
        defer { L10n.shared.mode = .en }
        let zh = render(link: short)
        XCTAssertEqual(en?.size.width, zh?.size.width)
    }
}
