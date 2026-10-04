import Foundation

/// Which agent pane a screenshot goes to, remembered between captures.
struct ScreenshotTarget: Equatable {
    let paneID: String
    let session: String

    private static let key = "screenshot.lastTarget"

    /// tmux ids are only stable for one server's life, so the session name rides along: a
    /// renumbered %N in another session is not the pane the user chose.
    static func load(_ defaults: UserDefaults = .standard) -> ScreenshotTarget? {
        guard let raw = defaults.string(forKey: key) else { return nil }
        let parts = raw.split(separator: "\t", maxSplits: 1, omittingEmptySubsequences: false)
        guard parts.count == 2, !parts[0].isEmpty else { return nil }
        return ScreenshotTarget(paneID: String(parts[0]), session: String(parts[1]))
    }

    func save(_ defaults: UserDefaults = .standard) {
        defaults.set(paneID + "\t" + session, forKey: Self.key)
    }
}

enum ScreenshotTargets {
    /// The default target: the agent pane a terminal showed most recently (what the user was
    /// looking at before the capture), then the last target used, then the most recently
    /// active agent. A pane waiting on the user is never the default.
    static func defaultTarget(candidates: [Agent], viewedAt: [String: Int64], last: ScreenshotTarget?) -> Agent? {
        let open = candidates.filter { $0.state != .waiting }
        if let viewed = open
            .filter({ (viewedAt[$0.paneID] ?? 0) > 0 })
            .max(by: { (viewedAt[$0.paneID] ?? 0) < (viewedAt[$1.paneID] ?? 0) }) {
            return viewed
        }
        if let last, let a = open.first(where: { $0.paneID == last.paneID && $0.session == last.session }) {
            return a
        }
        return open.max(by: { $0.activityAt < $1.activityAt }) ?? candidates.first
    }

    /// The agent pane a terminal showed most recently, waiting or not; nil when none was.
    static func recentPane(candidates: [Agent], viewedAt: [String: Int64]) -> String? {
        candidates
            .filter { (viewedAt[$0.paneID] ?? 0) > 0 }
            .max { (viewedAt[$0.paneID] ?? 0) < (viewedAt[$1.paneID] ?? 0) }?
            .paneID
    }

    /// `viewed_at` per pane id from `gtmux panes --json`; empty on anything unexpected.
    static func viewedAt(fromPanesJSON data: Data) -> [String: Int64] {
        struct Row: Decodable {
            let pane_id: String
            let viewed_at: Int64?
        }
        guard let rows = try? JSONDecoder().decode([Row].self, from: data) else { return [:] }
        var out: [String: Int64] = [:]
        for r in rows {
            if let v = r.viewed_at, v > 0 { out[r.pane_id] = v }
        }
        return out
    }
}

enum ScreenshotSendResult: Equatable {
    case delivered(queued: Bool)
    case refusedWaiting
    /// Refused as waiting AFTER the paste: the Enter was withheld, so the note and the image
    /// path may still be in the agent's input box.
    case heldAfterPaste
    case paneGone
    /// The fresh agent list could not be read, so nothing was checked or sent.
    case agentsUnreadable
    case refusedDraft(String)
    case duplicate
    case notConfirmed(String)
    case failed(String)

    var isSuccess: Bool {
        if case .delivered = self { return true }
        return false
    }
}

/// Sends the note and the image file through `gtmux send`, which verifies the landing,
/// guards an unsent draft and refuses an identical payload sent twice. The file is copied
/// into gtmux's uploads dir by `--attach`, named by its content, so a retry of the same
/// screenshot and note is the same message and the interlock can recognise it.
enum ScreenshotSender {
    static func arguments(pane: String, png: String) -> [String] {
        ["send", "--json", pane, "--message-file", "-", "--attach", png]
    }

    /// Checks the target against a FRESH agent list (nil when it could not be read). A
    /// waiting agent is refused: typed text and Enter could answer its permission prompt or
    /// question for the user. A pane id is only stable for one tmux server, so the session
    /// must match too: %N reused by another session is not the pane the user chose.
    static func preflight(target: String, session: String, agents: [Agent]?) -> ScreenshotSendResult? {
        guard let agents else { return .agentsUnreadable }
        guard let a = agents.first(where: { $0.paneID == target }),
              session.isEmpty || a.session == session else { return .paneGone }
        if a.state == .waiting { return .refusedWaiting }
        return nil
    }

    static func interpret(status: Int32, stdout: String, stderr: String) -> ScreenshotSendResult {
        struct Reply: Decodable {
            let delivered: Bool
            let state: String
            let evidence: String?
        }
        if let line = stdout.split(separator: "\n").last,
           let reply = try? JSONDecoder().decode(Reply.self, from: Data(line.utf8)) {
            let evidence = reply.evidence ?? ""
            switch reply.state {
            case "landed", "sent": return .delivered(queued: false)
            case "queued": return .delivered(queued: true)
            case "refused-draft": return .refusedDraft(evidence)
            case "refused-duplicate": return .duplicate
            case "refused-waiting": return pastedFirst(evidence) ? .heldAfterPaste : .refusedWaiting
            default: return reply.delivered ? .delivered(queued: false) : .notConfirmed(evidence.isEmpty ? reply.state : evidence)
            }
        }
        let message = stderr.isEmpty ? "gtmux send exited \(status)" : stderr
        if message.contains("pane not found") || message.contains("找不到该 pane") { return .paneGone }
        return .failed(message)
    }

    /// `gtmux send` starts the evidence of a refusal that came after the paste with one of
    /// these (dispatch.EvidenceHeldBeforeEnter / EvidenceHeldBeforeRetry).
    static let pastedPrefixes = ["stopped before Enter: ", "Enter not retried: "]

    static func pastedFirst(_ evidence: String) -> Bool {
        pastedPrefixes.contains { evidence.hasPrefix($0) }
    }

    /// Re-reads the agents, then sends, off the main thread; reports on the main queue.
    static func send(pane: String, session: String, note: String, png: URL,
                     completion: @escaping (ScreenshotSendResult) -> Void) {
        DispatchQueue.global(qos: .userInitiated).async {
            // A failed read is not "the pane is gone": say what actually happened.
            let agents = GtmuxCLI.capture(["agents", "--json"]).flatMap { try? JSONDecoder().decode([Agent].self, from: $0) }
            let result: ScreenshotSendResult
            if let refused = preflight(target: pane, session: session, agents: agents) {
                result = refused
            } else {
                let r = GtmuxCLI.captureFull(arguments(pane: pane, png: png.path), stdin: note)
                result = interpret(status: r.status, stdout: r.stdout, stderr: r.stderr)
            }
            DiagLog.act("act.screenshot.send", target: pane, outcome: result.isSuccess ? "ok" : "refused",
                        "sent a screenshot to a pane")
            DispatchQueue.main.async { completion(result) }
        }
    }
}
