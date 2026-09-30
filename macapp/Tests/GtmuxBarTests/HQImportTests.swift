import AppKit
import Combine
import SwiftUI
import XCTest
@testable import GtmuxBar

private final class ImportCalls {
    private let lock = NSLock()
    private var calls: [([String], String?)] = []
    func append(_ args: [String], _ input: String?) { lock.lock(); defer { lock.unlock() }; calls.append((args, input)) }
    var values: [([String], String?)] { lock.lock(); defer { lock.unlock() }; return calls }
}

final class HQImportTests: XCTestCase {
    private let previewJSON = """
    {"encrypted":true,"source":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","entries":[{"id":"pitfalls/new","topic":"pitfalls","title":"Review this knowledge before using it on the new Mac","body":"A retained lesson.\\n\\nCheck paths and environment first.","status":"new","records":4},{"id":"pitfalls/conflict","topic":"pitfalls","title":"Conflicting history","status":"conflict","records":1}],"sensitive_count":0,"has_local":true,"local":"From the old Mac","current_local":"Current requirements","current_local_digest":"current-hash","tools":1}
    """
    private let manifestJSON = """
    {"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","source":"archive","created_at":1790752000,"files":{"knowledge/.ledger.jsonl":"digest","LOCAL.md":"digest","knowledge/tools/check.sh":"digest"},"path":"/synthetic/stage"}
    """
    private func preview(_ text: String? = nil) -> HQMigrationPreview {
        try! JSONDecoder().decode(HQMigrationPreview.self, from: Data((text ?? previewJSON).utf8))
    }
    private func manifest() -> HQMigrationManifest {
        try! JSONDecoder().decode(HQMigrationManifest.self, from: Data(manifestJSON.utf8))
    }

    @MainActor func testSelectionAndPlainAcknowledgementStartEmpty() {
        let f = HQImportFlow(purpose: .migrate)
        f.preview = preview()
        XCTAssertFalse(f.stageReady)
        XCTAssertFalse(f.knowledge)
        XCTAssertFalse(f.personal)
        XCTAssertFalse(f.tools)
        XCTAssertFalse(f.sensitive)
        f.knowledge = true
        XCTAssertTrue(f.stageReady)
        f.preview = preview(previewJSON.replacingOccurrences(of: "\"encrypted\":true", with: "\"encrypted\":false"))
        XCTAssertFalse(f.stageReady)
        f.allowPlain = true
        XCTAssertTrue(f.stageReady)
    }

    @MainActor func testKnowledgeMustBeReviewedAndCannotImportConflictOrUnknownRows() {
        let f = HQImportFlow(purpose: .migrate)
        f.preview = preview()
        f.selected = ["pitfalls/new"]
        XCTAssertFalse(f.knowledgeReady)
        f.reviewedKnowledge = true
        XCTAssertTrue(f.knowledgeReady)
        f.selected.insert("pitfalls/conflict")
        XCTAssertFalse(f.knowledgeReady)
        f.selected = ["unknown"]
        XCTAssertFalse(f.knowledgeReady)
        f.selected = ["pitfalls/new"]; f.busy = true
        XCTAssertFalse(f.knowledgeReady)
        f.busy = false; f.selected = []
        XCTAssertFalse(f.knowledgeReady)
    }

    @MainActor func testPersonalReplacementRequiresExplicitChoice() {
        let f = HQImportFlow(purpose: .migrate)
        f.preview = preview()
        XCTAssertFalse(f.personalReady)
        f.replacePersonal = true
        XCTAssertTrue(f.personalReady)
        f.busy = true
        XCTAssertFalse(f.personalReady)
    }

    @MainActor func testRestorePreviewUsesRecoveryModeAndGuardsSourceBeforePublication() async {
        let calls = ImportCalls(), previewJSON = previewJSON
        let f = HQImportFlow(purpose: .restore) { args, input in
            calls.append(args, input)
            return (0, args.contains("--import") ? "Restored" : previewJSON, "")
        }
        f.path = "/synthetic/backup.age"; f.passphrase = "synthetic-passphrase"
        let ready = expectation(description: "recovery preview")
        let observer = f.$step.dropFirst().filter { $0 == .preview }.sink { _ in ready.fulfill() }
        f.inspect(l10n: L10n.shared)
        await fulfillment(of: [ready], timeout: 3)
        observer.cancel()
        XCTAssertTrue(calls.values.first!.0.contains("--restore-preview"))
        f.confirmRestore = true
        let restored = expectation(description: "restore complete")
        let done = f.$step.dropFirst().filter { $0 == .done }.sink { _ in restored.fulfill() }
        f.restore(l10n: L10n.shared)
        await fulfillment(of: [restored], timeout: 3)
        done.cancel()
        XCTAssertTrue(calls.values.last!.0.contains("--expect-archive"))
        XCTAssertFalse(calls.values.last!.0.contains("synthetic-passphrase"))
    }

    @MainActor func testStageUsesOnlySelectedContentAndPassphraseStaysOffArgv() async {
        let calls = ImportCalls(), manifestJSON = manifestJSON, previewJSON = previewJSON
        let f = HQImportFlow(purpose: .migrate) { args, input in
            calls.append(args, input)
            return (0, args.contains("--stage") ? manifestJSON : previewJSON, "")
        }
        f.preview = preview(); f.path = "/synthetic/export.age"; f.passphrase = "synthetic-passphrase"; f.knowledge = true
        let done = expectation(description: "staged review ready")
        let observer = f.$step.dropFirst().filter { $0 == .staged }.sink { _ in done.fulfill() }
        f.stage(l10n: L10n.shared)
        await fulfillment(of: [done], timeout: 3)
        observer.cancel()
        let stage = calls.values.first!
        XCTAssertEqual(stage.1, "synthetic-passphrase")
        XCTAssertFalse(stage.0.contains("synthetic-passphrase"))
        let selection = stage.0.firstIndex(of: "--include")!
        XCTAssertEqual(stage.0[selection+1], "knowledge")
        XCTAssertTrue(stage.0.contains("--expect-source"))
        XCTAssertFalse(stage.0.contains("--include-sensitive"))
        XCTAssertTrue(f.passphrase.isEmpty)
        XCTAssertTrue(f.selected.isEmpty)
        XCTAssertEqual(calls.values.last?.0, ["hq", "migrate", "--review", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--json"])
    }

    @MainActor func testPersonalApplyCarriesFreshReviewDigestAndStaysOnPersonalTab() async {
        let calls = ImportCalls(), previewJSON = previewJSON
        let f = HQImportFlow(purpose: .migrate) { args, input in
            calls.append(args, input)
            if args.contains("--apply") { return (0, "{\"operation_id\":\"op\",\"imported\":1,\"skipped\":0,\"committed\":true}", "") }
            return (0, previewJSON, "")
        }
        f.manifest = manifest(); f.preview = preview(); f.step = .staged; f.reviewTab = 1; f.replacePersonal = true
        let done = expectation(description: "review refreshed")
        let observer = f.$preview.dropFirst().sink { _ in done.fulfill() }
        f.applyPersonal(l10n: L10n.shared)
        await fulfillment(of: [done], timeout: 3)
        observer.cancel()
        XCTAssertEqual(f.reviewTab, 1)
        XCTAssertFalse(f.replacePersonal)
        let args = calls.values.first!.0
        XCTAssertTrue(args.contains("--apply-local"))
        XCTAssertTrue(args.contains("--reviewed"))
        XCTAssertTrue(args.contains("current-hash"))
        XCTAssertFalse(args.contains("--entries"))
    }

    @MainActor func testCommittedFailureRefreshesRowsAndRetainsRepairMessage() async {
        let fresh = previewJSON.replacingOccurrences(of: "\"status\":\"new\"", with: "\"status\":\"identical\"")
        let f = HQImportFlow(purpose: .migrate) { args, _ in
            if args.contains("--apply") { return (1, "{\"operation_id\":\"op\",\"imported\":1,\"skipped\":0,\"committed\":true,\"error\":\"committed; repair with knowledge render\"}", "") }
            return (0, fresh, "")
        }
        f.manifest = manifest(); f.preview = preview(); f.step = .staged; f.selected = ["pitfalls/new"]; f.reviewedKnowledge = true
        let done = expectation(description: "committed rows refreshed")
        let observer = f.$preview.dropFirst().sink { _ in done.fulfill() }
        f.applyKnowledge(l10n: L10n.shared)
        await fulfillment(of: [done], timeout: 3)
        observer.cancel()
        XCTAssertEqual(f.preview?.entries.first?.status, "identical")
        XCTAssertTrue(f.selected.isEmpty)
        XCTAssertFalse(f.knowledgeReady)
        XCTAssertEqual(f.error, "committed; repair with knowledge render")
    }

    func testReviewResolvesExistingLanguageHalfWithoutTranslating() throws {
        let json = """
        {"id":"pitfalls/a","topic":"pitfalls","title":"Source title","body":"Source body","status":"new","records":2,"lang":"en","alt":{"lang":"zh","title":"中文标题","body":"中文正文"}}
        """
        let entry = try JSONDecoder().decode(HQMigrationEntry.self, from: Data(json.utf8))
        XCTAssertEqual(entry.resolved("zh").title, "中文标题")
        XCTAssertEqual(entry.resolved("zh").body, "中文正文")
        XCTAssertEqual(entry.resolved("en").title, "Source title")
        XCTAssertEqual(entry.resolved("fr").tag, "en")
    }

    @MainActor func testArchiveHeaderDrivesPasswordAndSelectionResetsSecrets() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        let encrypted = dir.appendingPathComponent("renamed-backup.bin")
        try Data("age-encryption.org/v1\nsynthetic".utf8).write(to: encrypted)
        let plain = dir.appendingPathComponent("misleading.age")
        try Data([0x1f, 0x8b, 0x08]).write(to: plain)
        let f = HQImportFlow(purpose: .restore)
        XCTAssertFalse(f.previewReady)
        try f.selectArchive(encrypted)
        XCTAssertTrue(f.archiveEncrypted)
        XCTAssertFalse(f.previewReady)
        f.passphrase = "secret"
        XCTAssertTrue(f.previewReady)
        f.confirmRestore = true; f.allowPlain = true; f.knowledge = true
        try f.selectArchive(plain)
        XCTAssertFalse(f.archiveEncrypted)
        XCTAssertTrue(f.passphrase.isEmpty)
        XCTAssertFalse(f.confirmRestore)
        XCTAssertFalse(f.allowPlain)
        XCTAssertFalse(f.knowledge)
        XCTAssertTrue(f.previewReady)
        XCTAssertThrowsError(try f.selectArchive(dir))
        XCTAssertEqual(f.path, plain.path)
    }

    @MainActor func testEncryptedBackupCannotPreviewUntilPasswordIsProvided() throws {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try Data("age-encryption.org/v1".utf8).write(to: file)
        defer { try? FileManager.default.removeItem(at: file) }
        let calls = ImportCalls()
        let f = HQImportFlow(purpose: .restore) { args, input in calls.append(args, input); return (1, "", "invalid") }
        try f.selectArchive(file)
        f.inspect(l10n: L10n.shared)
        XCTAssertTrue(calls.values.isEmpty)
        XCTAssertFalse(f.busy)
    }

    @MainActor func testHostedLayoutsAreCompactUntilReviewInBothLanguagesAndAppearances() throws {
        let oldMode = L10n.shared.mode
        defer { L10n.shared.mode = oldMode }
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        let encrypted = dir.appendingPathComponent("HQ-backup-2026-09-30.tar.gz.age")
        try Data("age-encryption.org/v1".utf8).write(to: encrypted)
        let plain = dir.appendingPathComponent("HQ-backup.tar.gz")
        try Data([0x1f, 0x8b, 0x08]).write(to: plain)
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for dark in [false, true] {
                for scenario in ["empty", "encrypted", "plain", "restore", "preview", "knowledge", "personal", "tools", "stages", "done", "error", "busy"] {
                    let f = HQImportFlow(purpose: scenario == "restore" || scenario == "empty" ? .restore : .migrate) { _, _ in (0, "[]", "") }
                    if scenario != "empty" { try f.selectArchive(scenario == "plain" ? plain : encrypted) }
                    f.preview = preview(); f.manifest = manifest(); f.inspected = "pitfalls/new"
                    switch scenario {
                    case "restore", "preview": f.step = .preview
                    case "knowledge", "personal", "tools": f.step = .staged; f.reviewTab = scenario == "knowledge" ? 0 : scenario == "personal" ? 1 : 2
                    case "stages": f.step = .stages; f.stages = [manifest()]
                    case "done": f.step = .done; f.result = "Restored HQ backup. Previous records saved separately."
                    case "error": f.error = "Unable to open this backup. Check the password and try again."
                    case "busy": f.busy = true; f.progress = "Validating backup…"
                    default: break
                    }
                    let host = NSHostingView(rootView: HQImportSheet(l10n: L10n.shared, flow: f, onClose: {}).background(Color(nsColor: .windowBackgroundColor)).environment(\.colorScheme, dark ? .dark : .light))
                    let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1, height: 1), styleMask: [.borderless], backing: .buffered, defer: false)
                    window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
                    window.contentView = host
                    let size = host.fittingSize
                    window.setContentSize(size)
                    host.layoutSubtreeIfNeeded()
                    RunLoop.main.run(until: Date().addingTimeInterval(0.02))
                    host.layoutSubtreeIfNeeded()
                    XCTAssertEqual(size.width, f.step == .staged ? 820 : 560, accuracy: 1, scenario)
                    XCTAssertGreaterThan(size.height, 200, scenario)
                    XCTAssertLessThan(size.height, f.step == .staged ? 650 : f.step == .preview ? 620 : 490, scenario)
                    // Native SecureField must not steal focus or space before an encrypted file is chosen.
                    func secureFields(_ view: NSView) -> Int {
                        (view is NSSecureTextField ? 1 : 0) + view.subviews.reduce(0) { $0 + secureFields($1) }
                    }
                    XCTAssertEqual(secureFields(host), f.step == .choose && f.archiveEncrypted ? 1 : 0, scenario)
                    let rep = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                    host.cacheDisplay(in: host.bounds, to: rep)
                    let png = try XCTUnwrap(rep.representation(using: .png, properties: [:]))
                    try png.write(to: URL(fileURLWithPath: "/private/tmp/gtmux-import-ui-\(mode == .zh ? "zh" : "en")-\(dark ? "dark" : "light")-\(scenario).png"))
                }
            }
        }
    }
}
