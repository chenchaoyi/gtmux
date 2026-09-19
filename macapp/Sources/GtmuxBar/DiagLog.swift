import Foundation
import os

/// DiagLog is the menu bar app's writer for gtmux's local log store, the one every gtmux
/// process shares (openspec change `diagnostics`; the CLI side is `internal/diag`). The
/// app used to keep no record at all unless it was started by hand with GTMUXBAR_DEBUG.
///
/// An entry is the same JSON line the CLI writes, appended to today's file under
/// `~/.local/share/gtmux/logs/` in one `write` of at most 4 KB, which a local file system
/// appends atomically beside the other writers. Each entry is also sent to the unified log
/// under subsystem `com.gtmux.menubar`, so Console.app and `log show` see the app beside
/// the rest of macOS. Retention is the CLI's job: whichever gtmux process writes the first
/// entry of a day cleans the store, and serve's sweep does too.
///
/// Logging never throws and never blocks the caller on failure.
enum DiagLog {
    static let maxEntry = 4096
    private static let queue = DispatchQueue(label: "com.gtmux.menubar.diaglog")
    private static let unified = Logger(subsystem: "com.gtmux.menubar", category: "gtmux")

    /// Records something the app saw.
    static func info(_ event: String, _ msg: String, _ attrs: [String: Any] = [:]) {
        write(level: "info", kind: "diag", event: event, msg: msg, attrs: attrs)
    }

    static func warn(_ event: String, _ msg: String, _ attrs: [String: Any] = [:]) {
        write(level: "warn", kind: "diag", event: event, msg: msg, attrs: attrs)
    }

    /// Records a debug line, only when debug is on for the app (GTMUXBAR_DEBUG, or
    /// GTMUX_DEBUG naming `menubar` or `all`).
    static func debug(_ event: String, _ msg: String) {
        guard debugOn else { return }
        write(level: "debug", kind: "diag", event: event, msg: msg, attrs: [:])
    }

    /// Records something the app did. A refused act is a warning and a failed one an
    /// error, as in the CLI, so `gtmux logs --level warn` lists every act that did not
    /// happen.
    static func act(_ event: String, target: String, outcome: String, _ msg: String,
                    _ attrs: [String: Any] = [:]) {
        let level = outcome == "failed" ? "error" : (outcome == "refused" ? "warn" : "info")
        write(level: level, kind: "act", event: event, msg: msg, attrs: attrs,
              actor: "menubar", target: target, outcome: outcome)
    }

    /// A test points the store at a temporary directory; nil is the real one.
    static var dirOverride: String?

    private static let underTest = NSClassFromString("XCTestCase") != nil

    /// Waits for the entries already handed to the writer.
    static func flush() { queue.sync {} }

    static var debugOn: Bool {
        let env = ProcessInfo.processInfo.environment
        if env["GTMUXBAR_DEBUG"] != nil { return true }
        let names = (env["GTMUX_DEBUG"] ?? "").split(separator: ",")
            .map { $0.trimmingCharacters(in: .whitespaces) }
        return names.contains("menubar") || names.contains("all") || names.contains("1")
    }

    // MARK: - writing

    private static func write(level: String, kind: String, event: String, msg: String,
                              attrs: [String: Any], actor: String? = nil,
                              target: String? = nil, outcome: String? = nil) {
        let now = Date()
        queue.async {
            var e: [String: Any] = [
                "ts": timestamp(now), "level": level, "component": "menubar",
                "kind": kind, "event": event,
            ]
            if !msg.isEmpty { e["msg"] = redact(msg) }
            if let actor { e["actor"] = actor }
            if let target, !target.isEmpty { e["target"] = redact(target) }
            if let outcome { e["outcome"] = outcome }
            var clean: [String: Any] = [:]
            for (k, v) in attrs {
                switch v {
                case let s as String: clean[k] = isCredentialKey(k) ? redacted : redact(s)
                case let b as Bool: clean[k] = b
                case let i as Int: clean[k] = i
                case let d as Double: clean[k] = d
                default: clean[k] = redact(String(describing: v))
                }
            }
            if !clean.isEmpty { e["attrs"] = clean }
            guard var line = encode(e) else { return }
            if line.count > maxEntry {
                // Too long for one atomic append: keep the envelope, cut the message, mark it.
                e["msg"] = String((e["msg"] as? String ?? "").prefix(512))
                e["attrs"] = nil
                e["truncated"] = true
                guard let short = encode(e), short.count <= maxEntry else { return }
                line = short
            }
            append(line, day: now)
            mirror(level: level, event: event, msg: msg, target: target, outcome: outcome)
        }
    }

    private static func encode(_ e: [String: Any]) -> Data? {
        guard var d = try? JSONSerialization.data(withJSONObject: e, options: [.sortedKeys, .withoutEscapingSlashes])
        else { return nil }
        d.append(0x0A)
        return d
    }

    /// Appends to the day's current file: the last in-day segment when the store has
    /// started one (the CLI rotates a day past 20 MB), else `<day>.jsonl`.
    private static func append(_ line: Data, day: Date) {
        // A test run never writes to the real store: the CLI's tests panic on it, and a
        // stray test entry in someone's log is the same defect from this side.
        if dirOverride == nil, underTest { return }
        let dir = dirOverride ?? Paths.logsDir
        try? FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true,
                                                 attributes: [.posixPermissions: 0o700])
        let path = currentSegment(dir: dir, day: dayString(day))
        let fd = open(path, O_WRONLY | O_APPEND | O_CREAT, 0o600)
        guard fd >= 0 else { return }
        defer { close(fd) }
        _ = line.withUnsafeBytes { Darwin.write(fd, $0.baseAddress, line.count) }
    }

    static func currentSegment(dir: String, day: String) -> String {
        var best = 0
        let prefix = day + "."
        for name in (try? FileManager.default.contentsOfDirectory(atPath: dir)) ?? []
        where name.hasPrefix(prefix) && name.hasSuffix(".jsonl") {
            let mid = name.dropFirst(prefix.count).dropLast(".jsonl".count)
            if let n = Int(mid), n > best { best = n }
        }
        return dir + "/" + (best == 0 ? "\(day).jsonl" : "\(day).\(best).jsonl")
    }

    private static func mirror(level: String, event: String, msg: String, target: String?,
                               outcome: String?) {
        var text = event
        if let target, !target.isEmpty { text += " → " + redact(target) }
        if let outcome { text += " " + outcome }
        if !msg.isEmpty { text += " · " + redact(msg) }
        switch level {
        case "error": unified.error("\(text, privacy: .public)")
        case "warn": unified.warning("\(text, privacy: .public)")
        case "debug": unified.debug("\(text, privacy: .public)")
        default: unified.info("\(text, privacy: .public)")
        }
    }

    // MARK: - format

    private static let tsFormatter: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyy-MM-dd'T'HH:mm:ss.SSSxxxxx"
        return f
    }()

    private static let dayFormatter: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyy-MM-dd"
        return f
    }()

    static func timestamp(_ d: Date) -> String { tsFormatter.string(from: d) }
    static func dayString(_ d: Date) -> String { dayFormatter.string(from: d) }

    // MARK: - redaction

    /// The marker a credential is replaced with, the same one the CLI writes.
    static let redacted = "‹redacted›"
    private static var secrets = Set<String>()
    private static let secretsLock = NSLock()

    /// Makes every later entry replace s wherever it appears: the app registers the serve
    /// token when it reads it. Values shorter than 8 characters are ignored.
    static func registerSecret(_ s: String) {
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        guard t.count >= 8 else { return }
        secretsLock.lock(); secrets.insert(t); secretsLock.unlock()
    }

    private static let shapes: [NSRegularExpression] = [
        #"#[cg]=[^\s"&]+"#, // pairing and share fragments
        #"(?i)(authorization:?\s*)(bearer\s+|basic\s+)?\S+"#,
        #"(?i)bearer\s+[A-Za-z0-9._~+/=-]+"#,
    ].compactMap { try? NSRegularExpression(pattern: $0) }

    static func redact(_ s: String) -> String {
        var out = s
        secretsLock.lock(); let known = secrets; secretsLock.unlock()
        for sec in known where out.contains(sec) {
            out = out.replacingOccurrences(of: sec, with: redacted)
        }
        for re in shapes {
            out = re.stringByReplacingMatches(in: out, range: NSRange(out.startIndex..., in: out),
                                              withTemplate: redacted)
        }
        return out
    }

    static func isCredentialKey(_ k: String) -> Bool {
        let k = k.lowercased()
        if ["token", "secret", "password", "auth", "authorization", "cookie", "code"].contains(k) { return true }
        return k.hasSuffix("token") || k.hasSuffix("_secret") || k.hasSuffix("_password")
    }
}
