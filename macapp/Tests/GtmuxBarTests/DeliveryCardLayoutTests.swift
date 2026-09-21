import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

/// The delivery card has to FIT.
///
/// It did not: a 168pt QR with the browser URL, the terminal one-liner and a read-out
/// line crammed into the 240pt column beside it. The read-out line was 15pt monospace
/// with no line limit, so on a real link it was cut, and what got cut was the code
/// itself (「这个UI都展示不全」, 2026-09-21).
///
/// So the assertion is the property that failed: the card holds its width, and a link too
/// long for one line grows the card DOWNWARD instead of being clipped. Rendered for real
/// through ImageRenderer, which needs no screen and no permission.
final class DeliveryCardLayoutTests: XCTestCase {
    private let short = "https://x.dev/p1#code=GM4W-HCCQ"
    private let long = "https://gtmux.a-rather-long-self-hosted-domain.example.dev/p35047#code=GM4W-HCCQ"

    @MainActor private func render(_ link: String, width: CGFloat = 560) -> NSImage? {
        let v = CodeDeliveryBlock(
            l10n: L10n.shared,
            qrText: link,
            linkValue: link,
            terminalValue: "gtmux attach '\(link)'")
            .padding(18)
            .frame(width: width)
        let r = ImageRenderer(content: v)
        r.scale = 1
        return r.nsImage
    }

    @MainActor func testTheCardHoldsItsWidthAndGrowsDownward() throws {
        guard let a = render(short), let b = render(long) else {
            XCTFail("the card did not render"); return
        }
        // Neither link may push the card wider than the sheet that holds it.
        XCTAssertEqual(a.size.width, 560, accuracy: 0.5)
        XCTAssertEqual(b.size.width, 560, accuracy: 0.5)
        // A link too long for one line WRAPS: the card gets taller. Equal heights would
        // mean the long one was truncated, which is the defect this card was rebuilt for.
        XCTAssertGreaterThan(b.size.height, a.size.height,
                             "a long link did not wrap — it is being clipped again")
    }

    /// Chinese labels are wider than their English counterparts, and three doors share
    /// one row, so the language must not change the card's width either.
    @MainActor func testChineseDoesNotWidenIt() throws {
        let en = render(short)
        L10n.shared.mode = .zh
        defer { L10n.shared.mode = .en }
        let zh = render(short)
        XCTAssertEqual(en?.size.width, zh?.size.width)
    }
}
