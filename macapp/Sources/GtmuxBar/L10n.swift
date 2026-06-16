import AppKit
import SwiftUI

/// Language preference (DESIGN §8): follow system (default) / force English /
/// force Chinese. Persisted; changing it re-renders the UI instantly.
enum LangMode: String, CaseIterable {
    case system, en, zh
}

/// L10n resolves the active language and localizes UI strings (en/zh), mirroring
/// the CLI's i18n. The menu-bar app is a separate process, so it has its own copy.
final class L10n: ObservableObject {
    static let shared = L10n()

    @Published var mode: LangMode {
        didSet {
            UserDefaults.standard.set(mode.rawValue, forKey: "lang.mode")
            recompute()
        }
    }
    /// Resolved language: "en" or "zh".
    @Published private(set) var lang: String = "en"

    private init() {
        let raw = UserDefaults.standard.string(forKey: "lang.mode") ?? LangMode.system.rawValue
        mode = LangMode(rawValue: raw) ?? .system
        recompute()
    }

    private func recompute() {
        switch mode {
        case .en: lang = "en"
        case .zh: lang = "zh"
        case .system:
            lang = L10n.systemLang()
        }
        // Mirror to the CLI processes we spawn (focus/restore/new chrome).
        setenv("GTMUX_LANG", lang, 1)
    }

    private static func systemLang() -> String {
        if let env = ProcessInfo.processInfo.environment["GTMUX_LANG"] {
            if env == "zh" { return "zh" }
            if env == "en" { return "en" }
        }
        let pref = Locale.preferredLanguages.first ?? "en"
        return pref.lowercased().hasPrefix("zh") ? "zh" : "en"
    }

    /// Pick the English or Chinese variant.
    func tr(_ en: String, _ zh: String) -> String { lang == "zh" ? zh : en }
}
