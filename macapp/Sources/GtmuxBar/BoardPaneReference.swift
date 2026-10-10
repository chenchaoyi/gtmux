import SwiftUI

/// Board prose may name a pane, but only an explicit standalone tmux identifier is
/// actionable. Never infer a target from a task title, project, or terminal name.
enum BoardPaneReference {
    private static let pattern = #"(?<![A-Za-z0-9_/%=&?])%(?:0|[1-9][0-9]{0,17})(?![A-Za-z0-9_%/]|\.[0-9])"#
    private static let regex = try! NSRegularExpression(pattern: pattern)

    static func attributed(_ text: String, enabled: Bool = true) -> AttributedString {
        var result = AttributedString(text)
        guard enabled else { return result }
        for match in regex.matches(in: text, range: NSRange(text.startIndex..., in: text)) {
            guard let range = Range(match.range, in: text),
                  let start = AttributedString.Index(range.lowerBound, within: result),
                  let end = AttributedString.Index(range.upperBound, within: result) else { continue }
            let pane = String(text[range])
            result[start..<end].link = URL(string: "gtmux-board://pane/" + pane.dropFirst())
            result[start..<end].underlineStyle = .single
            result[start..<end].foregroundColor = .accentColor
        }
        return result
    }

    /// Internal links cannot become arbitrary CLI arguments or external navigation.
    static func target(_ url: URL) -> String? {
        guard url.scheme == "gtmux-board", url.host == "pane" else { return nil }
        let digits = String(url.path.dropFirst())
        guard digits.range(of: #"^(0|[1-9][0-9]{0,17})$"#, options: .regularExpression) != nil,
              url.absoluteString == "gtmux-board://pane/" + digits else { return nil }
        return "%" + digits
    }

    static func failureMessage(pane: String, status: Int32, stderr: String, zh: Bool) -> String? {
        guard status != 0 else { return nil }
        let lead = zh ? "无法打开窗格 \(pane)。它可能已关闭，或终端无法切换到前台。"
            : "Could not open pane \(pane). It may have closed, or its terminal could not be brought forward."
        let detail = stderr.trimmingCharacters(in: .whitespacesAndNewlines)
        return detail.isEmpty ? lead : lead + "\n\n" + detail
    }
}
