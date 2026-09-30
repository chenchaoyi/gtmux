import AppKit
import Foundation

struct HQMigrationEntry: Decodable, Identifiable {
    let id: String
    let topic: String
    let title: String
    let body: String?
    let sensitive: Bool?
    let status: String
    let records: Int
    let lang: String?
    let alt: KBAlt?

    func resolved(_ readerLang: String) -> (title: String, body: String, tag: String) {
        guard let lang, !lang.isEmpty, lang != readerLang else { return (title, body ?? "", "") }
        if let alt, alt.lang == readerLang, !alt.title.isEmpty { return (alt.title, alt.body ?? body ?? "", "") }
        return (title, body ?? "", lang)
    }
}

struct HQMigrationPreview: Decodable {
    let encrypted: Bool
    let source: String
    let entries: [HQMigrationEntry]
    let sensitive_count: Int
    let has_local: Bool
    let local: String?
    let current_local: String?
    let current_local_digest: String
    let tools: Int
}

struct HQMigrationManifest: Decodable, Identifiable {
    let id: String
    let source: String
    let created_at: Int64
    let files: [String: String]
    let path: String?
}

struct HQMigrationReceipt: Decodable {
    let operation_id: String
    let imported: Int
    let skipped: Int
    let committed: Bool
    let backup: String?
    let error: String?
}

@MainActor
final class HQImportFlow: ObservableObject {
    enum Purpose { case restore, migrate, resume }
    enum Step { case choose, preview, staged, stages, done }
    typealias Runner = ([String], String?) -> (status: Int32, stdout: String, stderr: String)

    let purpose: Purpose
    private let runner: Runner
    @Published var step: Step = .choose
    @Published var path = ""
    @Published var passphrase = ""
    @Published private(set) var archiveEncrypted = false
    @Published var preview: HQMigrationPreview?
    @Published var manifest: HQMigrationManifest?
    @Published var stages: [HQMigrationManifest] = []
    @Published var busy = false
    @Published var progress = ""
    @Published var error: String?
    @Published var result = ""
    @Published var knowledge = false
    @Published var personal = false
    @Published var tools = false
    @Published var sensitive = false
    @Published var allowPlain = false
    @Published var confirmRestore = false
    @Published var reviewedKnowledge = false
    @Published var replacePersonal = false
    @Published var selected: Set<String> = []
    @Published var inspected: String?
    @Published var reviewTab = 0

    init(purpose: Purpose, runner: @escaping Runner = { GtmuxCLI.captureFull($0, stdin: $1) }) {
        self.purpose = purpose
        self.runner = runner
        if purpose == .resume { step = .stages }
    }

    var previewReady: Bool { !busy && !path.isEmpty && (!archiveEncrypted || !passphrase.isEmpty) }

    var stageReady: Bool {
        guard let p = preview else { return false }
        return !busy && (knowledge || personal || tools) && (p.encrypted || allowPlain)
            && (!knowledge || !p.entries.isEmpty || sensitive && p.sensitive_count > 0)
            && (!personal || p.has_local) && (!tools || p.tools > 0)
    }
    var knowledgeReady: Bool { !busy && reviewedKnowledge && !selected.isEmpty && selected.allSatisfy { id in preview?.entries.contains { $0.id == id && $0.status == "new" } == true } }
    var personalReady: Bool { !busy && replacePersonal && preview?.has_local == true }
    var entry: HQMigrationEntry? { preview?.entries.first { $0.id == inspected } }

    func changeArchive() {
        step = .choose
        preview = nil
        knowledge = false; personal = false; tools = false; sensitive = false
        allowPlain = false; confirmRestore = false
        error = nil; result = ""
    }

    func chooseFile(l10n: L10n) {
        let panel = NSOpenPanel()
        panel.title = l10n.tr("Choose HQ archive", "选择 HQ 档案")
        panel.canChooseDirectories = false
        panel.allowsMultipleSelection = false
        guard panel.runModal() == .OK, let url = panel.url else { return }
        do { try selectArchive(url) }
        catch { self.error = l10n.tr("Cannot read this backup. Choose a readable HQ backup file.", "无法读取此备份，请选择可读取的 HQ 备份文件。") }
    }

    // A bounded header hint for the form only. The CLI still validates the whole archive.
    func selectArchive(_ url: URL) throws {
        guard try url.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile == true else {
            throw CocoaError(.fileReadUnsupportedScheme)
        }
        let file = try FileHandle(forReadingFrom: url)
        defer { try? file.close() }
        let ageHeader = Data("age-encryption.org/v1".utf8)
        let header = try file.read(upToCount: ageHeader.count) ?? Data()
        changeArchive()
        passphrase = ""
        archiveEncrypted = header == ageHeader
        path = url.path
    }

    private func run(_ args: [String], input: String? = nil, label: String,
                     completion: @escaping (Int32, String, String) -> Void) {
        guard !busy else { return }
        busy = true; progress = label; error = nil
        let runner = runner
        DispatchQueue.global(qos: .userInitiated).async {
            let r = runner(args, input)
            DispatchQueue.main.async {
                self.busy = false
                completion(r.status, r.stdout, r.stderr)
            }
        }
    }

    private func decode<T: Decodable>(_ type: T.Type, _ output: String) -> T? {
        do { return try JSONDecoder().decode(type, from: Data(output.utf8)) }
        catch { self.error = error.localizedDescription; return nil }
    }

    func inspect(l10n: L10n) {
        guard previewReady else { return }
        var args = ["hq", "migrate", "--from", path, "--passphrase-stdin", "--json"]
        if purpose == .restore { args.append("--restore-preview") }
        run(args, input: passphrase,
            label: l10n.tr("Validating archive…", "正在校验档案…")) { status, output, stderr in
            guard status == 0 else { self.error = stderr; return }
            guard let p = self.decode(HQMigrationPreview.self, output) else { return }
            self.preview = p
            self.step = .preview
        }
    }

    func stage(l10n: L10n) {
        guard stageReady, let p = preview else { return }
        let choices = [(knowledge, "knowledge"), (personal, "local"), (tools, "tools")]
            .filter { $0.0 }.map { $0.1 }.joined(separator: ",")
        var args = ["hq", "migrate", "--from", path, "--stage", "--include", choices,
                    "--expect-source", p.source, "--passphrase-stdin", "--json"]
        if sensitive && knowledge { args.append("--include-sensitive") }
        if allowPlain { args.append("--allow-plain") }
        run(args, input: passphrase, label: l10n.tr("Staging selected content…", "正在暂存所选内容…")) { status, output, stderr in
            guard status == 0 else { self.error = stderr; return }
            guard let m = self.decode(HQMigrationManifest.self, output) else { return }
            self.manifest = m
            self.passphrase = ""
            self.loadReview(l10n: l10n)
        }
    }

    func loadStages(l10n: L10n) {
        run(["hq", "migrate", "--list", "--json"], label: l10n.tr("Loading staged migrations…", "正在读取暂存记录…")) { status, output, stderr in
            guard status == 0 else { self.error = stderr; return }
            self.stages = self.decode([HQMigrationManifest].self, output) ?? []
        }
    }

    func loadReview(l10n: L10n, preservingError: String? = nil) {
        guard let m = manifest else { return }
        run(["hq", "migrate", "--review", m.id, "--json"], label: l10n.tr("Preparing review…", "正在准备核对内容…")) { status, output, stderr in
            guard status == 0 else { self.error = stderr; return }
            guard let p = self.decode(HQMigrationPreview.self, output) else { return }
            self.preview = p
            self.selected = []
            self.inspected = p.entries.first?.id
            self.reviewedKnowledge = false
            self.replacePersonal = false
            if self.step != .staged { self.reviewTab = m.files["knowledge/.ledger.jsonl"] != nil ? 0 : m.files["LOCAL.md"] != nil ? 1 : 2 }
            self.step = .staged
            self.error = preservingError
        }
    }

    func applyKnowledge(l10n: L10n) {
        guard knowledgeReady, let m = manifest else { return }
        apply(["hq", "migrate", "--apply", m.id, "--entries", selected.sorted().joined(separator: ","), "--reviewed", "--json"], l10n: l10n)
    }

    func applyPersonal(l10n: L10n) {
        guard personalReady, let m = manifest, let p = preview else { return }
        apply(["hq", "migrate", "--apply", m.id, "--apply-local", "--expect-local", p.current_local_digest, "--reviewed", "--json"], l10n: l10n)
    }

    private func apply(_ args: [String], l10n: L10n) {
        run(args, label: l10n.tr("Importing reviewed content…", "正在导入已核对的内容…")) { status, output, stderr in
            guard let receipt = self.decode(HQMigrationReceipt.self, output) else {
                if !stderr.isEmpty { self.error = stderr }
                return
            }
            self.result = l10n.tr("Imported \(receipt.imported), skipped \(receipt.skipped).", "已导入 \(receipt.imported) 项，跳过 \(receipt.skipped) 项。")
            if let backup = receipt.backup { self.result += "\n" + l10n.tr("Previous records: ", "原内容备份：") + backup }
            if status != 0 {
                self.error = receipt.error ?? stderr
                // A committed render/sync failure must not leave its old selectable rows active.
                if receipt.committed { self.loadReview(l10n: l10n, preservingError: receipt.error ?? stderr) }
                return
            }
            self.loadReview(l10n: l10n)
        }
    }

    func restore(l10n: L10n) {
        guard confirmRestore, let p = preview, p.encrypted || allowPlain else { return }
        run(["hq", "--import", path, "--expect-archive", p.source, "--passphrase-stdin"], input: passphrase,
            label: l10n.tr("Restoring backup…", "正在恢复备份…")) { status, output, stderr in
            guard status == 0 else { self.error = stderr; return }
            self.passphrase = ""
            self.result = output
            self.step = .done
        }
    }
}
