import AppKit
import Security
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
//
// The export is LOCKED (hq-export-passphrase, 2026-09-14). It carries the commander's
// project detail and whatever they told HQ to remember about themselves, and it is the
// copy that leaves the machine — over a USB stick, a synced folder, AirDrop. So the sheet
// asks for a passphrase once, confirms it, and can keep it in this Mac's keychain so the
// next export does not ask again: the lock is for the copy that travels, not a chore for
// the person at their own keyboard. The passphrase goes to the CLI over stdin, never argv.

/// What the CLI knows about the memory, for the row above the button.
struct HQMemoryState {
    var bytes: Int64 = 0
    var files: Int = 0
    var snapshots: Int = 0
    var offMachine: String = ""
    var exists: Bool = false
    /// The last export on this machine, when there has been one.
    var lastExportAt: Int64 = 0
    var lastExportEncrypted = false

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
    s.lastExportAt = (row["last_export_at"] as? NSNumber)?.int64Value ?? 0
    s.lastExportEncrypted = row["last_export_encrypted"] as? Bool ?? false
    return s
}

// MARK: the passphrase

/// The passphrase floor and ladder — the CLI's (`hq.PassphraseStrength`), so the sheet
/// refuses exactly what the command would refuse instead of finding out from stderr.
enum ExportPassphrase {
    static let minLength = 8

    enum Strength { case short, ok, good }

    static func strength(_ p: String) -> Strength {
        let n = p.count
        if n < minLength { return .short }
        if n < 12 { return .ok }
        return .good
    }
}

/// The passphrase, kept in this Mac's keychain when the commander asks — one generic
/// password item, read back to prefill the next export. gtmux's own service name, so it is
/// findable and deletable in Keychain Access like anything else.
enum ExportPassphraseStore {
    private static let service = "gtmux HQ export"
    private static let account = "passphrase"

    private static var query: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword,
         kSecAttrService as String: service,
         kSecAttrAccount as String: account]
    }

    static func read() -> String? {
        var q = query
        q[kSecReturnData as String] = true
        q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess,
              let d = out as? Data else { return nil }
        return String(data: d, encoding: .utf8)
    }

    static func write(_ p: String) {
        SecItemDelete(query as CFDictionary)
        var q = query
        q[kSecValueData as String] = Data(p.utf8)
        SecItemAdd(q as CFDictionary, nil)
    }

    static func delete() {
        SecItemDelete(query as CFDictionary)
    }
}

// MARK: the flow

/// The export, as a small state machine the sheet renders: ask → write → done or failed.
@MainActor
final class HQExportFlow: ObservableObject {
    enum Step: Equatable {
        case ask
        case exporting
        case done(path: String, size: String)
        case failed(String)
    }

    @Published var step: Step = .ask
    @Published var passphrase = ""
    @Published var confirm = ""
    @Published var reveal = false
    @Published var remember = true
    /// The keychain had one: the sheet opens on a single line instead of two fields,
    /// and "change" reopens them.
    @Published var remembered = false
    @Published var editing = false

    init() {
        if let saved = ExportPassphraseStore.read(), !saved.isEmpty {
            passphrase = saved
            confirm = saved
            remembered = true
        }
    }

    var strength: ExportPassphrase.Strength { ExportPassphrase.strength(passphrase) }
    var matches: Bool { !confirm.isEmpty && confirm == passphrase }
    /// The export may run: long enough, and typed the same twice (a remembered one
    /// already was).
    var ready: Bool { strength != .short && (remembered && !editing || matches) }

    /// Ask where, then write. The save panel comes AFTER the passphrase: a file the
    /// commander has already named and placed, then refused for a short passphrase, is
    /// the wrong order — decide the lock first, then the place.
    func run(l10n: L10n) {
        guard ready else { return }
        let panel = NSSavePanel()
        panel.title = l10n.tr("Export HQ's memory", "导出 HQ 的记忆")
        panel.nameFieldStringValue = "gtmux-hq-" + memoryDateStamp() + ".tar.gz.age"
        panel.allowedContentTypes = []
        panel.message = l10n.tr(
            "One locked file: the board, the knowledge base and your LOCAL.md. Put it somewhere this disk is not.",
            "一个上了锁的文件：态势板、知识库和你的 LOCAL.md。放到这块盘以外的地方。")
        guard panel.runModal() == .OK, let url = panel.url else { return }
        if remember {
            ExportPassphraseStore.write(passphrase)
        } else if remembered {
            ExportPassphraseStore.delete()
        }
        step = .exporting
        let pass = passphrase
        DispatchQueue.global(qos: .userInitiated).async {
            let r = GtmuxCLI.captureFull(["hq", "--export", url.path, "--passphrase-stdin"], stdin: pass)
            // The CLI adds .age when the name lacks it; read the size from the file it says
            // it wrote rather than the one we asked for.
            let written = url.path.hasSuffix(".age") ? url.path : url.path + ".age"
            let size = (try? FileManager.default.attributesOfItem(atPath: written)[.size] as? Int64)
                .map { HQMemoryState(bytes: $0).sizeText } ?? ""
            DispatchQueue.main.async {
                if r.status == 0 {
                    self.step = .done(path: written, size: size)
                } else {
                    let msg = r.stderr.trimmingCharacters(in: .whitespacesAndNewlines)
                    self.step = .failed(msg.isEmpty ? l10n.tr("export failed", "导出失败") : msg)
                }
            }
        }
    }
}

// MARK: the sheet

/// The export sheet: the lock first, then the place, then what was written.
struct HQExportSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var flow: HQExportFlow
    var onClose: () -> Void
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 14) {
            switch flow.step {
            case .ask: ask(p)
            case .exporting:
                HStack(spacing: 8) {
                    ProgressView().controlSize(.small)
                    Text(l10n.tr("Writing and locking…", "正在写入并上锁…")).font(.system(size: 12.5))
                }
                .frame(maxWidth: .infinity, minHeight: 80)
            case let .done(path, size): done(path: path, size: size, p)
            case let .failed(msg): failed(msg, p)
            }
        }
        .padding(20)
        .frame(width: 440)
        .background(p.bg)
    }

    @ViewBuilder private func ask(_ p: Theme.Palette) -> some View {
        Text(l10n.tr("Export HQ's memory", "导出 HQ 的记忆")).font(.system(size: 15, weight: .semibold))
        Text(l10n.tr(
            "The board, the knowledge base and your LOCAL.md — your project detail, and whatever you told HQ to remember. The file is locked with a passphrase before it leaves this Mac.",
            "态势板、知识库和你的 LOCAL.md —— 你的项目细节，还有你让 HQ 记住的事。文件离开这台 Mac 之前先用口令上锁。"))
            .font(.system(size: 12)).foregroundStyle(p.fg2)
            .fixedSize(horizontal: false, vertical: true)

        if flow.remembered && !flow.editing {
            // The keychain has one. A line, not a form: the decision was made last time.
            HStack(spacing: 8) {
                Image(systemName: "key.fill").font(.system(size: 12)).foregroundStyle(p.fg2)
                Text(l10n.tr("Using the passphrase kept in this Mac's keychain", "用这台 Mac 钥匙串里记着的口令"))
                    .font(.system(size: 12))
                Spacer()
                Button(l10n.tr("Change…", "换一个…")) {
                    flow.editing = true
                    flow.passphrase = ""
                    flow.confirm = ""
                }
                .buttonStyle(.link).font(.system(size: 12))
            }
            .padding(10)
            .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(p.rowSelected.opacity(0.5)))
        } else {
            VStack(alignment: .leading, spacing: 8) {
                field(l10n.tr("Passphrase", "口令"), text: $flow.passphrase, p)
                field(l10n.tr("Once more", "再输一次"), text: $flow.confirm, p)
                HStack(spacing: 10) {
                    hint(p)
                    Spacer()
                    Toggle(l10n.tr("Show", "显示"), isOn: $flow.reveal).toggleStyle(.checkbox).font(.system(size: 11.5))
                }
            }
        }

        Toggle(l10n.tr("Remember it in this Mac's keychain — the next export will not ask",
                       "记在这台 Mac 的钥匙串里 —— 下次导出不再问"), isOn: $flow.remember)
            .toggleStyle(.checkbox).font(.system(size: 12))

        HStack {
            Text(l10n.tr("Lose the passphrase and the file stays shut. gtmux keeps no copy.",
                         "口令丢了文件就打不开。gtmux 不留副本。"))
                .font(.system(size: 11)).foregroundStyle(p.fg3)
                .fixedSize(horizontal: false, vertical: true)
            Spacer()
            Button(l10n.tr("Cancel", "取消"), action: onClose).keyboardShortcut(.cancelAction)
            Button(l10n.tr("Choose where and export…", "选位置并导出…")) { flow.run(l10n: l10n) }
                .keyboardShortcut(.defaultAction)
                .disabled(!flow.ready)
        }
    }

    @ViewBuilder private func field(_ label: String, text: Binding<String>, _ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            Text(label).font(.system(size: 12)).foregroundStyle(p.fg2).frame(width: 64, alignment: .trailing)
            if flow.reveal {
                TextField("", text: text).textFieldStyle(.roundedBorder).font(.system(size: 12.5))
            } else {
                SecureField("", text: text).textFieldStyle(.roundedBorder).font(.system(size: 12.5))
            }
        }
    }

    /// One line under the fields that says the one thing to fix, or that there is nothing.
    @ViewBuilder private func hint(_ p: Theme.Palette) -> some View {
        let (text, color): (String, Color) = {
            if flow.passphrase.isEmpty { return ("", p.fg3) }
            switch flow.strength {
            case .short:
                return (l10n.tr("Too short — \(ExportPassphrase.minLength) characters at least", "太短 —— 至少 \(ExportPassphrase.minLength) 位"), Theme.Status.waiting)
            case .ok, .good:
                if !flow.confirm.isEmpty && !flow.matches {
                    return (l10n.tr("The two differ", "两次不一样"), Theme.Status.waiting)
                }
                if flow.strength == .good {
                    return (l10n.tr("Good passphrase", "口令强度：好"), Theme.Status.idle)
                }
                return (l10n.tr("Fine — 12 characters or more is better", "可以 —— 12 位以上更好"), p.fg2)
            }
        }()
        Text(text).font(.system(size: 11)).foregroundStyle(color).padding(.leading, 72)
    }

    @ViewBuilder private func done(path: String, size: String, _ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            Image(systemName: "lock.fill").font(.system(size: 14)).foregroundStyle(Theme.Status.idle)
            Text(l10n.tr("Exported and locked", "已导出并上锁")).font(.system(size: 15, weight: .semibold))
        }
        VStack(alignment: .leading, spacing: 5) {
            Text(path).font(Theme.Font.mono).foregroundStyle(p.fg).textSelection(.enabled)
                .lineLimit(2).truncationMode(.middle)
            if !size.isEmpty {
                Text(size).font(.system(size: 12)).foregroundStyle(p.fg2)
            }
        }
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(p.rowSelected.opacity(0.5)))
        Text(flow.remember
             ? l10n.tr("Opens with the passphrase in this Mac's keychain, `gtmux hq --import`, or any age tool.",
                       "用这台 Mac 钥匙串里的口令、`gtmux hq --import`，或任何 age 工具解开。")
             : l10n.tr("Opens with your passphrase: `gtmux hq --import`, or any age tool. gtmux keeps no copy of it.",
                       "用你的口令解开：`gtmux hq --import`，或任何 age 工具。gtmux 不留副本。"))
            .font(.system(size: 12)).foregroundStyle(p.fg2)
            .fixedSize(horizontal: false, vertical: true)
        HStack {
            Button(l10n.tr("Show in Finder", "在访达中显示")) {
                NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
            }
            Spacer()
            Button(l10n.tr("Done", "完成"), action: onClose).keyboardShortcut(.defaultAction)
        }
    }

    @ViewBuilder private func failed(_ msg: String, _ p: Theme.Palette) -> some View {
        Text(l10n.tr("Export failed", "导出失败")).font(.system(size: 15, weight: .semibold))
        // The CLI's own words, unedited — they already say what to do next.
        Text(msg).font(Theme.Font.mono).foregroundStyle(Theme.Status.waiting).textSelection(.enabled)
            .fixedSize(horizontal: false, vertical: true)
        HStack {
            Spacer()
            Button(l10n.tr("Back", "返回")) { flow.step = .ask }
            Button(l10n.tr("Close", "关闭"), action: onClose).keyboardShortcut(.cancelAction)
        }
    }
}

private func memoryDateStamp() -> String {
    let f = DateFormatter()
    f.dateFormat = "yyyyMMdd"
    return f.string(from: Date())
}
