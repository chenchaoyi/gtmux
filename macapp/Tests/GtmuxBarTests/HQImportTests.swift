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

    @MainActor func testSheetRendersAllStepsAndReviewTabsInBothLanguages() throws {
        let oldMode = L10n.shared.mode
        defer { L10n.shared.mode = oldMode }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for step in [HQImportFlow.Step.choose, .preview, .staged, .stages, .done] {
                for tab in 0...2 {
                    let f = HQImportFlow(purpose: .migrate) { _, _ in (0, "[]", "") }
                    f.preview = preview(); f.manifest = manifest(); f.step = step; f.reviewTab = tab
                    f.inspected = "pitfalls/new"; f.path = "/synthetic/archive.age"; f.result = "Completed"
                    let host = NSHostingView(rootView: HQImportSheet(l10n: L10n.shared, flow: f, onClose: {}))
                    let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 800, height: 640), styleMask: [.borderless], backing: .buffered, defer: false)
                    window.contentView = host
                    host.frame = NSRect(x: 0, y: 0, width: 800, height: 640)
                    host.layoutSubtreeIfNeeded()
                    RunLoop.main.run(until: Date().addingTimeInterval(0.02))
                    host.layoutSubtreeIfNeeded()
                    let rep = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                    host.cacheDisplay(in: host.bounds, to: rep)
                    let image = NSImage(size: host.bounds.size)
                    image.addRepresentation(rep)
                    XCTAssertEqual(image.size.width, 800, accuracy: 1)
                    XCTAssertEqual(image.size.height, 640, accuracy: 1)
                    if mode == .zh && step == .staged && tab < 2 {
                        let bitmap = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(image.tiffRepresentation)))
                        let png = try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
                        try png.write(to: URL(fileURLWithPath: "/private/tmp/gtmux-migration-\(tab == 0 ? "knowledge" : "personal").png"))
                    }
                }
            }
        }
    }
}
