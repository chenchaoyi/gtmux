import Foundation

/// Paths is the one place the menu bar app names a file under gtmux's two roots, the
/// same split the CLI's `internal/state` makes: `~/.config/gtmux` holds what a person set
/// or would carry to a new machine, `~/.local/share/gtmux` what gtmux generates.
/// `check-design.sh` fails a hard-coded root anywhere else in the app, so a file that
/// moves (as `tunnel-url` did) moves here once.
enum Paths {
    static var configDir: String { NSHomeDirectory() + "/.config/gtmux" }
    static var dataDir: String { NSHomeDirectory() + "/.local/share/gtmux" }

    /// The local log store, shared with every gtmux process (see DiagLog).
    static var logsDir: String { dataDir + "/logs" }

    static func config(_ name: String) -> String { configDir + "/" + name }
    static func data(_ name: String) -> String { dataDir + "/" + name }

    /// The tunnel's recorded address. It is runtime state, rewritten on every start, so
    /// it moved from the config root to the data root; a CLI older than this app still
    /// writes the old place, so both are read, new first.
    static var tunnelURLCandidates: [String] { [data("tunnel-url"), config("tunnel-url")] }

    /// tunnelURL is the first non-empty recorded address, or nil.
    static func tunnelURL() -> String? {
        for p in tunnelURLCandidates {
            if let s = try? String(contentsOfFile: p, encoding: .utf8) {
                let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
                if !t.isEmpty { return t }
            }
        }
        return nil
    }
}
