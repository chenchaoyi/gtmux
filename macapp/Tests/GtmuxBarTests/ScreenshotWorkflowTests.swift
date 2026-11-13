import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

@MainActor final class ScreenshotWorkflowTests: XCTestCase {
    private func image(_ red: Bool = true) throws -> CGImage {
        let context = try XCTUnwrap(CGContext(data: nil, width: 16, height: 16, bitsPerComponent: 8,
            bytesPerRow: 0, space: CGColorSpace(name: CGColorSpace.sRGB)!,
            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(CGColor(red: red ? 1 : 0, green: 0, blue: red ? 0 : 1, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: 16, height: 16))
        return try XCTUnwrap(context.makeImage())
    }
    private func document(_ red: Bool = true) throws -> ScreenshotDocument {
        ScreenshotDocument(image: try image(red), pointSize: CGSize(width: 16, height: 16))
    }
    private func folder() throws -> URL {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("screenshot-workflow-\(UUID())")
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        return url
    }
    private func agent(_ pane: String, _ title: String, session: String = "fixture", type: String = "Codex") -> Agent {
        var a = Agent(); a.paneID = pane; a.task = title; a.session = session; a.agent = type; a.status = "working"
        a.project = "gtmux"; return a
    }
    private func clean(_ pool: ScreenshotEditors) {
        for controller in Array(pool.documents.values) {
            let window = controller.window
            window?.delegate = nil; window?.orderOut(nil)
            controller.windowWillClose(Notification(name: NSWindow.willCloseNotification, object: window))
            window?.close()
        }
    }
    private func until(_ condition: () -> Bool) async throws {
        let end = Date().addingTimeInterval(3)
        while !condition(), Date() < end { try await Task.sleep(nanoseconds: 5_000_000) }
        XCTAssertTrue(condition(), "bounded UI transition did not finish")
    }

    func testSCW001WindowsKeepDraftsFilesAndLateReceiptsIndependent() throws {
        let pool = ScreenshotEditors(), store = AgentStore()
        let dir = try folder(); defer { clean(pool); try? FileManager.default.removeItem(at: dir) }
        store.setForTesting([agent("%one", "First session"), agent("%two", "Second session")])
        let firstDoc = try document(), secondDoc = try document(false)
        XCTAssertTrue(pool.open(doc: firstDoc, captureFile: dir.appendingPathComponent("one.png"), target: "%one", store: store, l10n: .shared))
        XCTAssertTrue(pool.open(doc: secondDoc, captureFile: dir.appendingPathComponent("two.png"), target: "%two", store: store, l10n: .shared))
        let first = try XCTUnwrap(pool.documents.values.first { $0.model?.doc === firstDoc })
        let second = try XCTUnwrap(pool.documents.values.first { $0.model?.doc === secondDoc })
        let one = try XCTUnwrap(first.model), two = try XCTUnwrap(second.model)
        one.note = "keep first"; two.note = "keep second"
        XCTAssertFalse(first.window === second.window)
        XCTAssertFalse(firstDoc.undoManager === secondDoc.undoManager)
        let undo = try XCTUnwrap(firstDoc.undoManager); undo.groupsByEvent = false
        undo.beginUndoGrouping(); firstDoc.add(Annotation(.rect(CGRect(x: 1, y: 1, width: 3, height: 3)), color: .yellow)); undo.endUndoGrouping()
        undo.undo(); XCTAssertTrue(firstDoc.items.isEmpty); XCTAssertTrue(secondDoc.items.isEmpty)
        undo.redo(); XCTAssertEqual(firstDoc.items.count, 1); XCTAssertTrue(secondDoc.items.isEmpty)
        var finishOne: ((ScreenshotSendResult) -> Void)?, finishTwo: ((ScreenshotSendResult) -> Void)?
        var closeOne: (() -> Void)?
        first.deliver = { pane, session, note, _, done in
            XCTAssertEqual(pane, "%one"); XCTAssertEqual(session, "fixture"); XCTAssertEqual(note, "keep first"); finishOne = done
        }
        second.deliver = { pane, _, note, _, done in
            XCTAssertEqual(pane, "%two"); XCTAssertEqual(note, "keep second"); finishTwo = done
        }
        first.scheduleDeliveryClose = { _, action in closeOne = action }; first.rememberTarget = { _ in }
        first.send(); second.send()
        XCTAssertEqual(one.sendingTarget?.name, "First session")
        let firstFile = try XCTUnwrap(first.renderedFile), secondFile = try XCTUnwrap(second.renderedFile)
        XCTAssertNotEqual(firstFile, secondFile, "same parent directory cannot mean one shared screenshot.png")
        let oneData = try Data(contentsOf: firstFile), twoData = try Data(contentsOf: secondFile)
        let red = try XCTUnwrap(NSBitmapImageRep(data: oneData)?.colorAt(x: 10, y: 10)?.usingColorSpace(.sRGB))
        let blue = try XCTUnwrap(NSBitmapImageRep(data: twoData)?.colorAt(x: 10, y: 10)?.usingColorSpace(.sRGB))
        XCTAssertGreaterThan(red.redComponent, 0.9); XCTAssertLessThan(red.blueComponent, 0.1)
        XCTAssertGreaterThan(blue.blueComponent, 0.9); XCTAssertLessThan(blue.redComponent, 0.1)
        finishTwo?(.refusedUnavailable("codex-frontend-unavailable"))
        XCTAssertFalse(two.editingBlocked); XCTAssertEqual(two.note, "keep second")
        finishOne?(.delivered(queued: false)); try XCTUnwrap(closeOne)()
        XCTAssertEqual(pool.documents.count, 1); XCTAssertTrue(second.isOpen)
        XCTAssertFalse(FileManager.default.fileExists(atPath: firstFile.path))
        XCTAssertEqual(try Data(contentsOf: secondFile), twoData)
        finishOne?(.failed("late old callback"))
        XCTAssertEqual(two.status, .result(.refusedUnavailable("codex-frontend-unavailable")))
        XCTAssertEqual(two.note, "keep second"); XCTAssertEqual(two.targetID, "%two")
    }

    func testSCW001CapacityPreservesEveryExistingDocument() throws {
        let pool = ScreenshotEditors(), store = AgentStore(), dir = try folder()
        defer { clean(pool); try? FileManager.default.removeItem(at: dir) }
        for i in 0..<ScreenshotEditors.maximumCount {
            XCTAssertTrue(pool.open(doc: try document(), captureFile: dir.appendingPathComponent("\(i).png"), target: nil, store: store, l10n: .shared))
            pool.documents.values.first { $0.model?.captureFile.lastPathComponent == "\(i).png" }?.model?.note = "draft \(i)"
        }
        let before = Set(pool.documents.keys)
        XCTAssertFalse(pool.hasCapacity)
        XCTAssertFalse(pool.open(doc: try document(), captureFile: dir.appendingPathComponent("extra.png"), target: nil, store: store, l10n: .shared))
        XCTAssertEqual(Set(pool.documents.keys), before)
        XCTAssertEqual(Set(pool.documents.values.compactMap { $0.model?.note }).count, ScreenshotEditors.maximumCount)
    }

    func testSCW002SearchUsesTitlesLocationsAndStableFuzzyMatches() {
        let agents = [agent("%1", "构建服务后端", session: "build"), agent("%2", "Fix Session Links", session: "review"),
                      agent("%3", "Café Parser", session: "parser"), agent("%4", "Fix Session Links", session: "review", type: "Claude Code")]
        func ids(_ query: String) -> [String] { ScreenshotTargetSearch.results(agents, query: query).map(\.paneID) }
        XCTAssertEqual(ids("构后"), ["%1"])
        XCTAssertEqual(ids("fsl"), ["%2", "%4"])
        XCTAssertEqual(ids("ＦＩＸ"), ["%2", "%4"])
        XCTAssertEqual(ids("cafe"), ["%3"])
        XCTAssertEqual(ids("gtmux review"), ["%2", "%4"])
        XCTAssertEqual(ids("%4"), ["%4"])
        XCTAssertEqual(ids("Claude"), ["%4"])
        XCTAssertEqual(ids("  "), agents.map(\.paneID)); XCTAssertTrue(ids("missing-target").isEmpty)
        XCTAssertTrue(ids(String(repeating: "x", count: 257)).isEmpty)
        XCTAssertTrue(ids(Array(repeating: "x", count: 17).joined(separator: " ")).isEmpty)
        XCTAssertEqual(agents[1].primary, "Fix Session Links")
        XCTAssertTrue(ScreenshotTargetSearch.context(agents[1]).contains("review · %2"))
    }

    func testSCW002ActualSearchFieldNavigationCompositionAndEmptyResults() async throws {
        NativeUIProbe.prepareAccessibility()
        let agents = [agent("%1", "构建服务后端"), agent("%2", "Review Capture")]
        var picked: [String] = [], dismissed = 0
        let view = ScreenshotTargetList(agents: agents, selected: "%1", recent: "%1", l10n: .shared,
            dismiss: { dismissed += 1 }, pick: { picked.append($0) })
        let host = NSHostingView(rootView: view)
        let window = NSWindow(contentRect: NSRect(x: 20, y: 20, width: 440, height: 180), styleMask: [.titled], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false; window.contentView = host; window.makeKeyAndOrderFront(nil)
        defer { window.close() }
        try await until { !NativeUIProbe.views(NSSearchField.self, in: host).isEmpty }
        let field = try XCTUnwrap(NativeUIProbe.views(NSSearchField.self, in: host).first)
        window.makeFirstResponder(field)
        try await until { field.currentEditor() != nil }
        let editor = try XCTUnwrap(field.currentEditor() as? NSTextView)
        XCTAssertTrue(field.delegate?.control?(field, textView: editor, doCommandBy: #selector(NSResponder.moveDown(_:))) == true)
        XCTAssertTrue(field.delegate?.control?(field, textView: editor, doCommandBy: #selector(NSResponder.insertNewline(_:))) == true)
        XCTAssertEqual(picked, ["%2"])
        let marked = MarkedSearchTextView()
        XCTAssertFalse(field.delegate?.control?(field, textView: marked, doCommandBy: #selector(NSResponder.insertNewline(_:))) ?? true)
        XCTAssertEqual(picked.count, 1, "IME Return must not choose a session")
        field.stringValue = "missing-target"
        field.delegate?.controlTextDidChange?(Notification(name: NSControl.textDidChangeNotification, object: field))
        _ = field.delegate?.control?(field, textView: editor, doCommandBy: #selector(NSResponder.insertNewline(_:)))
        XCTAssertEqual(picked.count, 1, "a hidden previous row cannot be submitted")
        XCTAssertTrue(field.delegate?.control?(field, textView: editor, doCommandBy: #selector(NSResponder.cancelOperation(_:))) == true)
        XCTAssertEqual(dismissed, 1)
    }

    func testSCW003RefusalCategoriesNeverInventHumanApprovalOrDelivery() throws {
        let old = L10n.shared.mode; defer { L10n.shared.mode = old }
        for reason in ["codex-frontend-unavailable", "expected-conversation-changed", "input-state-unavailable", "input-metadata-unavailable"] {
            let result = ScreenshotSender.interpret(status: 1, stdout: "{\"state\":\"refused-waiting\",\"delivered\":false,\"evidence\":\"\(reason)\"}", stderr: "")
            XCTAssertEqual(result, .refusedUnavailable(reason)); XCTAssertFalse(result.isSuccess)
            XCTAssertEqual(result.reasonCode, reason)
            for language in [LangMode.en, .zh] {
                L10n.shared.mode = language
                let text = ScreenshotStatusText.text(.result(result), target: "Fixture", l10n: .shared)
                XCTAssertFalse(text.contains("待处理请求")); XCTAssertFalse(text.contains("pending confirmation"))
                XCTAssertTrue(text.contains(language == .zh ? "未发送" : "Not sent"))
            }
        }
        for reason in ["input-awaiting-confirmation", "a choice menu is on the screen", "a choice menu is open: synthetic"] {
            let json = "{\"state\":\"refused-waiting\",\"delivered\":false,\"evidence\":\"\(reason)\"}"
            XCTAssertEqual(ScreenshotSender.interpret(status: 1, stdout: json, stderr: ""), .refusedWaiting)
        }
        let unknown = ScreenshotSender.interpret(status: 1, stdout: #"{"state":"refused-waiting","delivered":false,"evidence":"new-unknown-reason"}"#, stderr: "")
        XCTAssertEqual(unknown, .notConfirmed("new-unknown-reason"))
        XCTAssertEqual(ScreenshotSender.interpret(status: 1, stdout: #"{"state":"refused-waiting","delivered":true,"evidence":"codex-frontend-unavailable"}"#, stderr: ""), .notConfirmed("inconsistent send receipt"))
        L10n.shared.mode = .zh
        XCTAssertFalse(ScreenshotStatusText.text(.result(.duplicate), target: "Fixture", l10n: .shared).contains("已发送至"))
        let model = ScreenshotEditorModel(doc: try document(), captureFile: URL(fileURLWithPath: "/unused"), target: "%fixture")
        model.note = "preserve"; model.status = .result(.refusedUnavailable("codex-frontend-unavailable"))
        XCTAssertFalse(model.editingBlocked); XCTAssertEqual(model.note, "preserve")
    }

    func testSCW004NewCaptureIsSerializedThroughHandoffAndKeepsOldDraft() async throws {
        let pool = ScreenshotEditors.shared, store = AgentStore(), dir = try folder()
        XCTAssertTrue(pool.documents.isEmpty)
        defer { clean(pool); try? FileManager.default.removeItem(at: dir) }
        XCTAssertTrue(pool.open(doc: try document(), captureFile: dir.appendingPathComponent("old.png"), target: nil, store: store, l10n: .shared))
        let previous = try XCTUnwrap(pool.documents.values.first?.model); previous.note = "old draft"
        let controller = ScreenshotController(), releaseLookup = DispatchSemaphore(value: 0)
        let lookupEntered = expectation(description: "lookup entered"), captured = expectation(description: "new selection captured")
        defer { releaseLookup.signal() }
        controller.access.preflight = { true }
        controller.readPanes = { lookupEntered.fulfill(); releaseLookup.wait(); return Data("[]".utf8) }
        controller.presentAlert = { title, detail in XCTFail("unexpected alert \(title): \(detail)") }
        let png = try XCTUnwrap(AnnotationRenderer.pngData(try image(false), pointSize: CGSize(width: 16, height: 16)))
        var captures = 0
        controller.capture = { file, done in
            captures += 1; try! png.write(to: file); done(.image(file)); captured.fulfill()
        }
        controller.start(store: store, l10n: .shared, prepareSurfaces: {}, finishSurfaces: { _ in })
        await fulfillment(of: [lookupEntered, captured], timeout: 3)
        controller.start(store: store, l10n: .shared, prepareSurfaces: {}, finishSurfaces: { _ in })
        XCTAssertEqual(captures, 1, "selection remains serialized until target lookup hands off the document")
        releaseLookup.signal()
        try await until { pool.documents.count == 2 }
        XCTAssertEqual(previous.note, "old draft")
        let cancelled = expectation(description: "second selection cancelled")
        controller.readPanes = { Data("[]".utf8) }
        controller.capture = { _, done in captures += 1; done(.cancelled); cancelled.fulfill() }
        controller.start(store: store, l10n: .shared, prepareSurfaces: {}, finishSurfaces: { _ in })
        await fulfillment(of: [cancelled], timeout: 3)
        XCTAssertEqual(captures, 2); XCTAssertEqual(pool.documents.count, 2); XCTAssertEqual(previous.note, "old draft")
    }

    func testSCW004CLIEditCanOpenBesideDraftAndResolveItsOwnStore() async throws {
        let pool = ScreenshotEditors.shared, store = AgentStore(), dir = try folder()
        XCTAssertTrue(pool.documents.isEmpty)
        defer { clean(pool); try? FileManager.default.removeItem(at: dir) }
        store.setForTesting([agent("%cli", "CLI session", session: "fixture-cli")])
        XCTAssertTrue(pool.open(doc: try document(), captureFile: dir.appendingPathComponent("old.png"), target: nil, store: store, l10n: .shared))
        let previous = try XCTUnwrap(pool.documents.values.first?.model); previous.note = "keep old"
        let encoded = try ScreenshotAgentRendering.encode(image: image(false), pointSize: CGSize(width: 16, height: 16), kind: "imported")
        let input = dir.appendingPathComponent("input.png"); try encoded.0.write(to: input)
        let fd = open(input.path, O_RDONLY); XCTAssertGreaterThanOrEqual(fd, 0); defer { close(fd) }
        let spec = try JSONDecoder().decode(ScreenshotAgentSpec.self, from: JSONSerialization.data(withJSONObject: ["schema_version": 1, "capture_id": encoded.2.capture_id, "image_sha256": encoded.2.image_sha256, "coordinate_space": "image_px", "annotations": []]))
        let service = ScreenshotAgentService { doc, bytes in
            let file = dir.appendingPathComponent("cli.png"); try bytes.write(to: file)
            XCTAssertTrue(pool.open(doc: doc, captureFile: file, target: "%cli", store: store, l10n: .shared))
        }
        let request = ScreenshotAgentRequest(schema_version: 1, request_id: String(repeating: "a", count: 32), host_nonce: "fixture", operation: "edit", deadline_ms: 0, app: nil, source_kind: nil, source_ref: nil, rect: nil, image_sha256: encoded.2.image_sha256, document: encoded.2, spec: spec)
        let receipt = try await service.perform(request, files: [fd], budget: ScreenshotAgentBudget(end: ProcessInfo.processInfo.systemUptime + 3))
        XCTAssertEqual((try JSONSerialization.jsonObject(with: receipt) as? [String: Bool])?["sent"], false)
        XCTAssertEqual(pool.documents.count, 2); XCTAssertEqual(previous.note, "keep old")
        let cli = try XCTUnwrap(pool.documents.values.first { $0.model?.targetID == "%cli" })
        var sends = 0
        cli.deliver = { pane, session, _, _, _ in XCTAssertEqual(pane, "%cli"); XCTAssertEqual(session, "fixture-cli"); sends += 1 }
        cli.send(); XCTAssertEqual(sends, 1, "no earlier human capture is required to initialize a global store")
        // A reused pane in a different session cannot silently become this draft's destination.
        cli.model?.status = .idle
        store.setForTesting([agent("%cli", "Replacement", session: "other-session")])
        cli.send(); XCTAssertEqual(sends, 1); XCTAssertEqual(cli.model?.status, .result(.paneGone))
    }

    func testSCW004PermissionAndCapacityRefusalsPreserveWorkWithoutCapture() throws {
        let pool = ScreenshotEditors.shared, controller = ScreenshotController(), store = AgentStore(), dir = try folder()
        XCTAssertTrue(pool.documents.isEmpty)
        defer { clean(pool); try? FileManager.default.removeItem(at: dir) }
        var requests = 0, alerts = 0
        controller.access.preflight = { false }
        controller.requestPermission = { _ in requests += 1 }
        controller.capture = { _, _ in XCTFail("denied capture must not start") }
        controller.start(store: store, l10n: .shared, prepareSurfaces: { XCTFail("permission denied") }, finishSurfaces: { _ in XCTFail("no capture") })
        XCTAssertEqual(requests, 1); XCTAssertTrue(pool.documents.isEmpty)
        for i in 0..<ScreenshotEditors.maximumCount {
            XCTAssertTrue(pool.open(doc: try document(), captureFile: dir.appendingPathComponent("\(i).png"), target: nil, store: store, l10n: .shared))
        }
        let identities = Set(pool.documents.keys)
        controller.access.preflight = { XCTFail("capacity must be checked before TCC"); return true }
        controller.presentAlert = { _, _ in alerts += 1 }
        controller.start(store: store, l10n: .shared, prepareSurfaces: { XCTFail("no capacity") }, finishSurfaces: { _ in XCTFail("no capture") })
        XCTAssertEqual(alerts, 1); XCTAssertEqual(requests, 1); XCTAssertEqual(Set(pool.documents.keys), identities)
    }
}

private final class MarkedSearchTextView: NSTextView {
    override func hasMarkedText() -> Bool { true }
}
