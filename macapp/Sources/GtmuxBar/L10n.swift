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

    /// The `GTMUX_LANG` this process was LAUNCHED with, captured before anything is written
    /// back. recompute() mirrors every answer into that same variable for the CLI processes
    /// the app spawns, so reading it live afterwards reads the app's own last answer: pick
    /// Chinese once, go back to "follow the setting", and it followed the pick (2026-09-22).
    private let launchEnvLang: String?

    private init() {
        launchEnvLang = ProcessInfo.processInfo.environment["GTMUX_LANG"]
        let raw = UserDefaults.standard.string(forKey: "lang.mode") ?? LangMode.system.rawValue
        mode = LangMode(rawValue: raw) ?? .system
        recompute()
    }

    private func recompute() {
        switch mode {
        case .en: lang = "en"
        case .zh: lang = "zh"
        case .system:
            lang = systemLang()
        }
        // Mirror to the CLI processes we spawn (focus/restore/new chrome).
        setenv("GTMUX_LANG", lang, 1)
    }

    /// "Follow the setting" — the SAME setting the CLI follows.
    ///
    /// The config step was missing, so the two halves of one product answered the same
    /// question differently: `gtmux` printed Chinese because `~/.config/gtmux/config.json`
    /// says `"lang": "zh"`, while this window stayed English because macOS's first
    /// preferred language is `en-CN`. A setting one surface obeys and another ignores is
    /// not a preference, and the commander met it as "why is the knowledge base all in
    /// English" (2026-09-21) — the reader's language decides which half of a bilingual
    /// entry he is shown, so ignoring it changed what he could READ, not just the chrome.
    ///
    /// `.en` / `.zh` still override this: choosing in the app is a stronger statement than
    /// a file, and the app's setting is the one the person is looking at.
    private func systemLang() -> String {
        L10n.resolveLang(env: launchEnvLang,
                         config: L10n.configLang(),
                         locale: Locale.preferredLanguages.first ?? "en")
    }

    /// The precedence, pure, so the ORDER is testable and not just its pieces: the
    /// environment gtmux was launched with, then the machine config, then the locale.
    /// Mirrors the CLI's `resolveLang`.
    static func resolveLang(env: String?, config: String?, locale: String) -> String {
        if env == "zh" || env == "en" { return env! }
        if config == "zh" || config == "en" { return config! }
        return locale.lowercased().hasPrefix("zh") ? "zh" : "en"
    }

    /// The machine config's `lang`, when it names one. "auto", an unknown value and an
    /// unreadable file all mean "no answer here" — they fall through to the locale rather
    /// than forcing English, exactly as the CLI reads it.
    static func configLang() -> String? {
        guard let data = FileManager.default.contents(atPath: Paths.config("config.json")),
              let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let l = obj["lang"] as? String else { return nil }
        return l == "zh" || l == "en" ? l : nil
    }

    /// Pick the English or Chinese variant.
    func tr(_ en: String, _ zh: String) -> String { lang == "zh" ? zh : en }
}
