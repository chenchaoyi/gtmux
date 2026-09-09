import AppKit
import SwiftUI

// Exporting the supervisor's memory, from the surface that sits on top of it.
//
// The memory is the one thing gtmux holds that cannot be reproduced: a board it has
// rewritten for months, a curated knowledge base, and a `LOCAL.md` seeded once and never
// rewritten, so losing it does not self-heal. The CLI can export it and the phone can keep
// a copy — and the menu bar, which runs ON the machine where all of it lives and whose
// whole job is reading it, was the only surface that could not.
//
// The verb is EXPORT, not "back up". gtmux does not run a backup service and will not
// imply one: this writes a file where you point it, and what happens to that file after is
// yours. The row says the same thing the doctor row says, in the same words — how much is
// at risk, and whether anything at all carries it off this disk.

/// What the CLI knows about the memory, for the row above the button.
struct HQMemoryState {
    var bytes: Int64 = 0
    var files: Int = 0
    var snapshots: Int = 0
    var offMachine: String = ""
    var exists: Bool = false

    /// The size a person reads. Same shape as the CLI's, so the two surfaces agree.
    var sizeText: String {
        if bytes >= 1 << 20 { return String(format: "%.1f MB", Double(bytes) / Double(1 << 20)) }
        if bytes >= 1 << 10 { return "\(bytes / (1 << 10)) KB" }
        return "\(bytes) B"
    }
}

/// readHQMemoryState asks `gtmux hq --memory --json`.
///
/// Through the CLI, like every other fact this app shows. A second implementation of "how
/// big is the HQ home, and is anything protecting it" is how two surfaces start
/// disagreeing about the same number — and the off-machine sentence in particular has to
/// be the SAME sentence the doctor row prints, not a paraphrase of it.
func readHQMemoryState() -> HQMemoryState {
    var s = HQMemoryState()
    guard let d = GtmuxCLI.capture(["hq", "--memory", "--json"]),
          let row = (try? JSONSerialization.jsonObject(with: d)) as? [String: Any] else { return s }
    s.exists = row["exists"] as? Bool ?? false
    s.bytes = (row["bytes"] as? NSNumber)?.int64Value ?? 0
    s.files = (row["files"] as? NSNumber)?.intValue ?? 0
    s.snapshots = (row["snapshots"] as? NSNumber)?.intValue ?? 0
    s.offMachine = row["off_machine"] as? String ?? ""
    return s
}

/// exportHQMemory asks where to put it, then runs `gtmux hq --export`.
///
/// A save panel rather than a fixed location: the whole point is to put the file somewhere
/// this disk is not, and only the person at the keyboard knows where that is.
@MainActor
func exportHQMemory(l10n: L10n, completion: @escaping (String?) -> Void) {
    let panel = NSSavePanel()
    panel.title = l10n.tr("Export HQ's memory", "导出 HQ 的记忆")
    panel.nameFieldStringValue = "gtmux-hq-" + memoryDateStamp() + ".tar.gz"
    panel.allowedContentTypes = []
    panel.message = l10n.tr(
        "The board, the knowledge base and your LOCAL.md, as one file. It carries your project detail — keep it somewhere you would keep working notes.",
        "态势板、知识库和你的 LOCAL.md，写成一个文件。里面是你的项目细节，按工作笔记的标准找地方放。")
    guard panel.runModal() == .OK, let url = panel.url else {
        completion(nil)
        return
    }
    DispatchQueue.global(qos: .userInitiated).async {
        let r = GtmuxCLI.captureFull(["hq", "--export", url.path])
        DispatchQueue.main.async {
            if r.status == 0 {
                completion(nil)
                NSWorkspace.shared.activateFileViewerSelecting([url])
            } else {
                let msg = r.stderr.trimmingCharacters(in: .whitespacesAndNewlines)
                completion(msg.isEmpty ? l10n.tr("export failed", "导出失败") : msg)
            }
        }
    }
}

private func memoryDateStamp() -> String {
    let f = DateFormatter()
    f.dateFormat = "yyyyMMdd"
    return f.string(from: Date())
}
