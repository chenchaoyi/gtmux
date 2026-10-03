import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotTests: XCTestCase {

    // MARK: hotkeys

    /// Two keys, two actions: a press of one must not run the other. One shared handler
    /// used to run every instance's action on any press.
    func testHotkeyDispatchIsByID() {
        var palette = 0, shot = 0
        let a = GlobalHotkey.register { palette += 1 }
        let b = GlobalHotkey.register { shot += 1 }
        XCTAssertNotEqual(a, b)
        XCTAssertTrue(GlobalHotkey.dispatch(b))
        XCTAssertEqual([palette, shot], [0, 1])
        XCTAssertTrue(GlobalHotkey.dispatch(a))
        XCTAssertEqual([palette, shot], [1, 1])
        XCTAssertFalse(GlobalHotkey.dispatch(999_999))
    }

    // MARK: capture

    func testCaptureArgumentsKeepDPIAndStayQuiet() {
        let url = URL(fileURLWithPath: "/tmp/a b.png")
        XCTAssertEqual(ScreenshotCapture.arguments(output: url), ["-i", "-x", "-o", "-t", "png", "/tmp/a b.png"])
        XCTAssertFalse(ScreenshotCapture.arguments(output: url).contains("-r"), "-r would drop the DPI the editor sizes by")
    }

    func testEscLeavesNoFileAndIsNotAnError() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        let missing = dir.appendingPathComponent("none.png")
        XCTAssertEqual(ScreenshotCapture.outcome(fileAt: missing, exitStatus: 1), .cancelled)
        let empty = dir.appendingPathComponent("empty.png")
        FileManager.default.createFile(atPath: empty.path, contents: Data())
        XCTAssertEqual(ScreenshotCapture.outcome(fileAt: empty, exitStatus: 0), .cancelled)
        XCTAssertFalse(FileManager.default.fileExists(atPath: empty.path), "an empty capture is cleaned up")
        let full = dir.appendingPathComponent("shot.png")
        FileManager.default.createFile(atPath: full.path, contents: Data([1, 2, 3]))
        XCTAssertEqual(ScreenshotCapture.outcome(fileAt: full, exitStatus: 0), .image(full))
        if case .failed = ScreenshotCapture.outcome(fileAt: missing, exitStatus: 9) {} else {
            XCTFail("an unexpected exit without a file is a failure")
        }
    }

    // MARK: document and undo

    private func blankImage(width: Int, height: Int, gray: CGFloat = 1) -> CGImage {
        let ctx = CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                            space: CGColorSpace(name: CGColorSpace.sRGB)!,
                            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        ctx.setFillColor(CGColor(gray: gray, alpha: 1))
        ctx.fill(CGRect(x: 0, y: 0, width: width, height: height))
        return ctx.makeImage()!
    }

    func testMarksUndoAndRedoThroughTheWindowsUndoManager() {
        let doc = ScreenshotDocument(image: blankImage(width: 200, height: 100), pointSize: CGSize(width: 100, height: 50))
        let undo = UndoManager()
        undo.groupsByEvent = false
        doc.undoManager = undo
        let a = Annotation(.rect(CGRect(x: 10, y: 10, width: 20, height: 20)), color: .red)
        undo.beginUndoGrouping(); doc.add(a); undo.endUndoGrouping()
        XCTAssertEqual(doc.items, [a])
        undo.undo()
        XCTAssertEqual(doc.items, [])
        undo.redo()
        XCTAssertEqual(doc.items, [a])
        XCTAssertEqual(doc.scale, 2)
    }

    func testAClickIsNotAShapeAndDragsStayOnTheImage() {
        let doc = ScreenshotDocument(image: blankImage(width: 100, height: 100), pointSize: CGSize(width: 100, height: 100))
        XCTAssertNil(doc.shape(.rect, from: CGPoint(x: 5, y: 5), to: CGPoint(x: 6, y: 6), color: .red))
        XCTAssertNil(doc.shape(.text, from: .zero, to: CGPoint(x: 50, y: 50), color: .red))
        guard case let .rect(r)? = doc.shape(.rect, from: CGPoint(x: 90, y: 90), to: CGPoint(x: 150, y: -20), color: .red)?.kind else {
            return XCTFail("a drag off the edge is still a rectangle")
        }
        XCTAssertEqual(r, CGRect(x: 90, y: 0, width: 10, height: 90))
    }

    // MARK: rendering

    private func pixel(_ img: CGImage, _ x: Int, _ y: Int) -> (r: UInt8, g: UInt8, b: UInt8) {
        let rep = NSBitmapImageRep(cgImage: img)
        let c = rep.colorAt(x: x, y: y)!.usingColorSpace(.sRGB)!
        return (UInt8(c.redComponent * 255), UInt8(c.greenComponent * 255), UInt8(c.blueComponent * 255))
    }

    /// A 2× capture keeps its pixels, and a mark at a point lands at twice that pixel.
    func testRendererDrawsAtFullPixelResolution() throws {
        let img = blankImage(width: 400, height: 200)
        let rect = Annotation(.rect(CGRect(x: 20, y: 20, width: 60, height: 40)), color: .red)
        let out = try XCTUnwrap(AnnotationRenderer.flatten(image: img, pointSize: CGSize(width: 200, height: 100), items: [rect]))
        XCTAssertEqual([out.width, out.height], [400, 200])
        // The rectangle's top edge at y = 20pt is pixel row 40; its middle is still white.
        let edge = pixel(out, 100, 40)
        XCTAssertGreaterThan(edge.r, 200); XCTAssertLessThan(edge.g, 120)
        let inside = pixel(out, 100, 80)
        XCTAssertTrue([inside.r, inside.g, inside.b].allSatisfy { $0 >= 250 }, "still white inside: \(inside)")
        // 1×: the same mark at the same pixel as its point.
        let one = try XCTUnwrap(AnnotationRenderer.flatten(image: blankImage(width: 200, height: 100),
                                                           pointSize: CGSize(width: 200, height: 100), items: [rect]))
        let edge1 = pixel(one, 50, 20)
        XCTAssertGreaterThan(edge1.r, 200); XCTAssertLessThan(edge1.g, 120)
    }

    func testTextAndArrowLeaveInk() throws {
        let img = blankImage(width: 300, height: 120)
        let items = [
            Annotation(.text("GTMUX 42", at: CGPoint(x: 10, y: 10)), color: .blue),
            Annotation(.arrow(from: CGPoint(x: 20, y: 100), to: CGPoint(x: 280, y: 100)), color: .red),
        ]
        let out = try XCTUnwrap(AnnotationRenderer.flatten(image: img, pointSize: CGSize(width: 300, height: 120), items: items))
        var inked = 0
        for x in stride(from: 12, to: 120, by: 2) {
            for y in stride(from: 12, to: 34, by: 2) where Int(pixel(out, x, y).b) > Int(pixel(out, x, y).r) + 60 { inked += 1 }
        }
        XCTAssertGreaterThan(inked, 5, "the text was drawn")
        XCTAssertGreaterThan(pixel(out, 150, 100).r, 200, "the arrow shaft was drawn")
    }

    /// Copy and Save use the same export as Send: PNG keeps the DPI, the pasteboard gets PNG
    /// and TIFF. A private pasteboard keeps the test off the user's clipboard.
    func testExportsAgree() throws {
        let img = blankImage(width: 200, height: 100)
        let png = try XCTUnwrap(AnnotationRenderer.pngData(img, pointSize: CGSize(width: 100, height: 50)))
        let rep = try XCTUnwrap(NSBitmapImageRep(data: png))
        XCTAssertEqual([rep.pixelsWide, rep.pixelsHigh], [200, 100])
        XCTAssertEqual(rep.size, CGSize(width: 100, height: 50), "a Retina capture reopens at its point size")
        let pb = NSPasteboard(name: NSPasteboard.Name("gtmux-test-\(UUID().uuidString)"))
        defer { pb.releaseGlobally() }
        XCTAssertTrue(AnnotationRenderer.copy(img, pointSize: CGSize(width: 100, height: 50), to: pb))
        XCTAssertEqual(pb.data(forType: .png), png)
        XCTAssertNotNil(pb.data(forType: .tiff))
    }

    func testSaveWritesToAPathWithSpaces() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("gtmux test \(UUID().uuidString)")
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        let doc = ScreenshotDocument(image: blankImage(width: 40, height: 40), pointSize: CGSize(width: 20, height: 20))
        let model = ScreenshotEditorModel(doc: doc, captureFile: dir.appendingPathComponent("c.png"), target: nil)
        let url = dir.appendingPathComponent(ScreenshotLayout.saveName())
        XCTAssertTrue(url.lastPathComponent.contains(" "))
        XCTAssertTrue(model.write(to: url))
        XCTAssertNotNil(NSBitmapImageRep(data: try Data(contentsOf: url)))
    }

    // MARK: targets

    private func agent(_ pane: String, _ session: String, status: String = "idle", activity: Int = 0) -> Agent {
        var a = Agent()
        a.paneID = pane; a.session = session; a.status = status; a.activityAt = activity; a.agent = "Claude Code"
        return a
    }

    func testDefaultTargetPrefersWhatTheTerminalShowed() {
        let a = agent("%1", "work", activity: 900), b = agent("%2", "misc", activity: 100), c = agent("%3", "ask", status: "waiting")
        let all = [a, b, c]
        // What a terminal showed last wins.
        XCTAssertEqual(ScreenshotTargets.defaultTarget(candidates: all, viewedAt: ["%2": 50, "%1": 10], last: nil)?.paneID, "%2")
        // A waiting pane is never the default, even when it is on screen.
        XCTAssertEqual(ScreenshotTargets.defaultTarget(candidates: all, viewedAt: ["%3": 99], last: nil)?.paneID, "%1")
        // Then the last target, matched by pane AND session.
        XCTAssertEqual(ScreenshotTargets.defaultTarget(candidates: all, viewedAt: [:], last: ScreenshotTarget(paneID: "%2", session: "misc"))?.paneID, "%2")
        XCTAssertEqual(ScreenshotTargets.defaultTarget(candidates: all, viewedAt: [:], last: ScreenshotTarget(paneID: "%2", session: "other"))?.paneID, "%1")
        XCTAssertNil(ScreenshotTargets.defaultTarget(candidates: [], viewedAt: [:], last: nil))
    }

    func testViewedAtParsesPanesJSON() {
        let json = #"[{"pane_id":"%1","viewed_at":1791040500},{"pane_id":"%2"},{"pane_id":"%3","viewed_at":0}]"#
        XCTAssertEqual(ScreenshotTargets.viewedAt(fromPanesJSON: Data(json.utf8)), ["%1": 1791040500])
        XCTAssertEqual(ScreenshotTargets.viewedAt(fromPanesJSON: Data("not json".utf8)), [:])
    }

    func testLastTargetRoundTrips() {
        let defaults = UserDefaults(suiteName: "gtmux-test-\(UUID().uuidString)")!
        XCTAssertNil(ScreenshotTarget.load(defaults))
        ScreenshotTarget(paneID: "%5", session: "gtmux dev").save(defaults)
        XCTAssertEqual(ScreenshotTarget.load(defaults), ScreenshotTarget(paneID: "%5", session: "gtmux dev"))
    }

    // MARK: send

    func testSendArgumentsUseStdinAndAttach() {
        XCTAssertEqual(ScreenshotSender.arguments(pane: "%5", png: "/tmp/x y/screenshot.png"),
                       ["send", "--json", "%5", "--message-file", "-", "--attach", "/tmp/x y/screenshot.png"])
    }

    /// Never type into a pane that waits on the user, or one that is gone.
    func testPreflightRefusesWaitingAndGonePanes() {
        let agents = [agent("%1", "w", status: "waiting"), agent("%2", "x", status: "working")]
        XCTAssertEqual(ScreenshotSender.preflight(target: "%1", session: "w", agents: agents), .refusedWaiting)
        XCTAssertEqual(ScreenshotSender.preflight(target: "%9", session: "", agents: agents), .paneGone)
        XCTAssertNil(ScreenshotSender.preflight(target: "%2", session: "x", agents: agents), "a busy agent queues it")
        // The same %N in another session is not the pane that was chosen.
        XCTAssertEqual(ScreenshotSender.preflight(target: "%2", session: "other", agents: agents), .paneGone)
        // A list that could not be read is said to be that, not a vanished pane.
        XCTAssertEqual(ScreenshotSender.preflight(target: "%2", session: "x", agents: nil), .agentsUnreadable)
    }

    func testRefusedWaitingFromTheCLIMapsToWaiting() {
        XCTAssertEqual(ScreenshotSender.interpret(status: 1, stdout: #"{"delivered":false,"state":"refused-waiting"}"#, stderr: ""),
                       .refusedWaiting)
    }

    /// A text mark still being typed is in the picture for Copy and Save, as it is for Send.
    func testPendingTextIsExported() throws {
        let doc = ScreenshotDocument(image: blankImage(width: 300, height: 100), pointSize: CGSize(width: 300, height: 100))
        let m = ScreenshotEditorModel(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/none.png"), target: nil)
        m.textAt = CGPoint(x: 10, y: 10)
        m.textValue = "TYPED"
        let pb = NSPasteboard(name: NSPasteboard.Name("gtmux-test-\(UUID().uuidString)"))
        defer { pb.releaseGlobally() }
        m.copy(to: pb)
        XCTAssertNil(m.textAt, "committed")
        guard case .text("TYPED", _)? = doc.items.last?.kind else { return XCTFail("the typed text became a mark") }
        let copied = try XCTUnwrap(pb.data(forType: .png).flatMap { NSBitmapImageRep(data: $0) })
        let flat = try XCTUnwrap(AnnotationRenderer.flatten(doc))
        XCTAssertEqual(copied.pixelsWide, flat.width)
        // Save sees the same marks: a second pending text is committed by write too.
        m.textAt = CGPoint(x: 10, y: 50); m.textValue = "AGAIN"
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        XCTAssertTrue(m.write(to: dir.appendingPathComponent("s.png")))
        XCTAssertEqual(doc.items.count, 2)
    }

    /// Open means "exists": a hidden or minimised editor is brought back, and a late callback
    /// from an earlier editor does not act on a newer one.
    func testEditorLifecycle() throws {
        let c = ScreenshotEditorController()
        let store = AgentStore()
        let doc1 = ScreenshotDocument(image: blankImage(width: 80, height: 60), pointSize: CGSize(width: 80, height: 60))
        c.show(doc: doc1, captureFile: URL(fileURLWithPath: "/tmp/gtmux-test-1.png"), target: nil, store: store, l10n: L10n.shared)
        let first = try XCTUnwrap(c.window)
        XCTAssertTrue(c.isOpen)
        first.orderOut(nil) // out of sight, like a minimised window
        XCTAssertTrue(c.isOpen, "a window that is not visible still holds the work")
        c.bringToFront()
        XCTAssertTrue(first.isVisible)
        XCTAssertTrue(c.isCurrent(first))
        let doc2 = ScreenshotDocument(image: blankImage(width: 80, height: 60), pointSize: CGSize(width: 80, height: 60))
        c.show(doc: doc2, captureFile: URL(fileURLWithPath: "/tmp/gtmux-test-2.png"), target: nil, store: store, l10n: L10n.shared)
        XCTAssertFalse(c.isCurrent(first), "the first editor's late success must not close the second")
        XCTAssertTrue(c.isCurrent(c.window))
        c.windowWillClose(Notification(name: NSWindow.willCloseNotification))
        XCTAssertFalse(c.isOpen)
        XCTAssertFalse(c.isCurrent(nil))
    }

    func testSendResultsMapFromSendJSON() {
        func r(_ json: String, _ status: Int32 = 0, _ err: String = "") -> ScreenshotSendResult {
            ScreenshotSender.interpret(status: status, stdout: json, stderr: err)
        }
        XCTAssertEqual(r(#"{"delivered":true,"state":"landed","attachments":["/u/a.png"]}"#), .delivered(queued: false))
        XCTAssertEqual(r(#"{"delivered":true,"state":"sent"}"#), .delivered(queued: false))
        XCTAssertEqual(r(#"{"delivered":false,"state":"queued"}"#), .delivered(queued: true))
        XCTAssertEqual(r(#"{"delivered":false,"state":"refused-draft","evidence":"draft: hi"}"#, 1), .refusedDraft("draft: hi"))
        XCTAssertEqual(r(#"{"delivered":false,"state":"refused-duplicate"}"#, 1), .duplicate)
        XCTAssertEqual(r(#"{"delivered":false,"state":"failed","evidence":"no receipt"}"#, 1), .notConfirmed("no receipt"))
        XCTAssertEqual(r("", 1, "gtmux send: pane not found"), .paneGone)
        XCTAssertEqual(r("", 2, "gtmux send: --attach: open x: no such file"), .failed("gtmux send: --attach: open x: no such file"))
        XCTAssertTrue(ScreenshotSendResult.delivered(queued: true).isSuccess)
        XCTAssertFalse(ScreenshotSendResult.duplicate.isSuccess)
    }

    func testRetryIsOfferedOnlyForAnUnconfirmedDelivery() {
        let doc = ScreenshotDocument(image: blankImage(width: 10, height: 10), pointSize: CGSize(width: 10, height: 10))
        let m = ScreenshotEditorModel(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/none.png"), target: "%1")
        m.status = .result(.notConfirmed("x"))
        XCTAssertTrue(m.isRetry)
        m.status = .result(.duplicate)
        XCTAssertFalse(m.isRetry, "a duplicate already arrived")
        m.status = .result(.refusedWaiting)
        XCTAssertFalse(m.isRetry)
    }

    func testStatusLineSaysWhatHappened() {
        // L10n's language is the app's (not settable from a test); expect whichever is active.
        let l = L10n.shared
        XCTAssertTrue(ScreenshotStatusText.text(.result(.refusedWaiting), target: "Codex", l10n: l)
            .contains(l.tr("waiting for your decision", "正在等你做决定")))
        XCTAssertTrue(ScreenshotStatusText.text(.result(.notConfirmed("")), target: "Codex", l10n: l)
            .contains(l.tr("Look at the pane", "先看一眼 pane")))
        XCTAssertTrue(ScreenshotStatusText.text(.result(.refusedDraft("")), target: "Codex", l10n: l)
            .contains(l.tr("input box", "输入框")))
        XCTAssertEqual(ScreenshotStatusText.text(.idle, target: "Codex", l10n: l), "")
        XCTAssertTrue(ScreenshotStatusText.isProblem(.result(.paneGone)))
        XCTAssertTrue(ScreenshotStatusText.isProblem(.result(.agentsUnreadable)))
        XCTAssertFalse(ScreenshotStatusText.isProblem(.result(.delivered(queued: true))))
    }

    /// Every result the app produces itself is written in both languages; no English
    /// fragment rides into the Chinese window (review: "could not read the agent list").
    func testOwnResultsAreLocalised() {
        let l = L10n.shared
        let was = l.mode
        defer { l.mode = was }
        let own: [ScreenshotSendResult] = [.agentsUnreadable, .refusedWaiting, .paneGone, .duplicate, .delivered(queued: false)]
        for r in own {
            l.mode = .en
            let en = ScreenshotStatusText.text(.result(r), target: "Codex", l10n: l)
            l.mode = .zh
            let zh = ScreenshotStatusText.text(.result(r), target: "Codex", l10n: l)
            XCTAssertNotEqual(zh, en, "\(r) has no Chinese text")
            XCTAssertNotNil(zh.range(of: "\\p{Han}", options: .regularExpression), "\(r): \(zh)")
        }
        XCTAssertEqual(ScreenshotStatusText.text(.result(.agentsUnreadable), target: "", l10n: l),
                       "没有发送：读不到 agent 列表，没法核对目标。再试一次。")
        l.mode = .en
        XCTAssertTrue(ScreenshotStatusText.text(.result(.agentsUnreadable), target: "", l10n: l)
            .hasPrefix("Not sent: could not read the agent list"))
    }

    // MARK: layout

    func testEditorNeverEnlargesACapture() {
        XCTAssertEqual(ScreenshotLayout.displaySize(image: CGSize(width: 300, height: 200), within: CGSize(width: 1440, height: 900)),
                       CGSize(width: 300, height: 200))
        let big = ScreenshotLayout.displaySize(image: CGSize(width: 3000, height: 2000), within: CGSize(width: 1440, height: 900))
        XCTAssertLessThanOrEqual(big.width, 1440 * 0.85)
        XCTAssertLessThanOrEqual(big.height, 900 * 0.85 - ScreenshotLayout.chromeHeight)
        XCTAssertEqual(big.width / big.height, 1.5, accuracy: 0.01, "the aspect ratio is kept")
    }
}
