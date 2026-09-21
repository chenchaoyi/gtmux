import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

/// One setting, one answer.
///
/// `gtmux` printed Chinese because `~/.config/gtmux/config.json` says `"lang": "zh"`,
/// while the menu-bar window stayed English because macOS's first preferred language is
/// `en-CN`. A setting one half of a product obeys and the other ignores is not a
/// preference. It reached the commander as "why is the knowledge base all in English":
/// the reader's language decides which half of a bilingual entry he is shown, so the
/// disagreement changed what he could READ, not just the chrome.
final class ConfigLanguageTests: XCTestCase {
    private func withConfig(_ body: String?, _ run: () -> Void) {
        let dir = Paths.configDir
        let path = Paths.config("config.json")
        let fm = FileManager.default
        let saved = fm.contents(atPath: path)
        defer {
            if let saved = saved { fm.createFile(atPath: path, contents: saved) } else { try? fm.removeItem(atPath: path) }
        }
        try? fm.createDirectory(atPath: dir, withIntermediateDirectories: true)
        if let body = body {
            fm.createFile(atPath: path, contents: Data(body.utf8))
        } else {
            try? fm.removeItem(atPath: path)
        }
        run()
    }

    /// The ORDER, not just the pieces: a test on configLang() alone passes even when
    /// nothing calls it, which is how the setting came to be ignored in the first place.
    func testTheConfigDecidesWhenTheEnvironmentIsSilent() {
        // en-CN is this machine: the locale says English while the config says Chinese.
        XCTAssertEqual(L10n.resolveLang(env: nil, config: "zh", locale: "en-CN"), "zh")
        XCTAssertEqual(L10n.resolveLang(env: nil, config: "en", locale: "zh-Hans-CN"), "en")
        // The environment gtmux was launched with still wins over the file.
        XCTAssertEqual(L10n.resolveLang(env: "en", config: "zh", locale: "zh-Hans"), "en")
        // No config: the locale decides, as before.
        XCTAssertEqual(L10n.resolveLang(env: nil, config: nil, locale: "zh-Hans-CN"), "zh")
        XCTAssertEqual(L10n.resolveLang(env: nil, config: nil, locale: "en-CN"), "en")
    }

    func testTheConfigsLanguageIsRead() {
        withConfig(#"{"lang":"zh"}"#) { XCTAssertEqual(L10n.configLang(), "zh") }
        withConfig(#"{"lang":"en"}"#) { XCTAssertEqual(L10n.configLang(), "en") }
    }

    /// "auto", an unknown value, no key, a malformed file and no file at all all mean
    /// "no answer here" — the locale decides, rather than the app forcing English.
    func testAnAbsentAnswerFallsThrough() {
        for body in [#"{"lang":"auto"}"#, #"{"lang":"fr"}"#, #"{"notLang":1}"#, "{ not json", ""] {
            withConfig(body) { XCTAssertNil(L10n.configLang(), "body \(body) should not answer") }
        }
        withConfig(nil) { XCTAssertNil(L10n.configLang()) }
    }
}

/// The knowledge list and its detail sit side by side, and the row carried no mark of
/// being the selected one — so answering "which entry is filling the right half" meant
/// reading both and matching titles.
final class KnowledgeSelectionTests: XCTestCase {
    func testAPaneEqualsOnlyItsOwnEntry() {
        let a = KnowledgePane.entry(id: "pitfalls/one")
        XCTAssertEqual(a, .entry(id: "pitfalls/one"))
        XCTAssertNotEqual(a, .entry(id: "pitfalls/two"))
        XCTAssertNotEqual(a, .index)
    }

    /// The mark must be DRAWN, not merely computable: selected and unselected cannot
    /// render to the same pixels.
    @MainActor func testTheSelectedRowRendersDifferently() {
        func png(_ selected: Bool) -> Data? {
            let v = KBSelectionMark(selected: selected, p: Theme.Palette.of(.light))
                .frame(width: 292, height: 48)
            let r = ImageRenderer(content: v)
            r.scale = 1
            guard let img = r.nsImage, let t = img.tiffRepresentation,
                  let b = NSBitmapImageRep(data: t) else { return nil }
            return b.representation(using: .png, properties: [:])
        }
        guard let off = png(false), let on = png(true) else { XCTFail("no render"); return }
        XCTAssertNotEqual(off, on, "the selected row looks exactly like an unselected one")
    }
}
