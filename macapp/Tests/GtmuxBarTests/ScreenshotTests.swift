import AppKit
import SwiftUI
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
        XCTAssertTrue(url.path.contains(" "))
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

    /// Refused before the paste: nothing was typed. Refused before the Enter or its retry:
    /// the note and path may be in the input box, and the user is told to look first.
    func testRefusedWaitingSaysWhetherTheTextWasPasted() {
        func r(_ evidence: String) -> ScreenshotSendResult {
            let reply: [String: Any] = ["delivered": false, "state": "refused-waiting", "evidence": evidence]
            let json = String(data: try! JSONSerialization.data(withJSONObject: reply), encoding: .utf8)!
            return ScreenshotSender.interpret(status: 1, stdout: json, stderr: "")
        }
        XCTAssertEqual(r("the agent is waiting on a decision (a permission prompt or a question)"), .refusedWaiting)
        XCTAssertEqual(r("a choice menu is on the screen"), .refusedWaiting)
        XCTAssertEqual(r("stopped before Enter: a choice menu is on the screen"), .heldAfterPaste)
        XCTAssertEqual(r("Enter not retried: a choice menu is on the screen"), .heldAfterPaste)
        // A prefix that only appears later in the text is not the stage.
        XCTAssertEqual(r("could not read the pane: stopped before Enter: x"), .refusedWaiting)

        let l = L10n.shared
        let was = l.mode
        defer { l.mode = was }
        for mode in [LangMode.en, .zh] {
            l.mode = mode
            let before = ScreenshotStatusText.text(.result(.refusedWaiting), target: "Codex", l10n: l)
            let after = ScreenshotStatusText.text(.result(.heldAfterPaste), target: "Codex", l10n: l)
            let box = l.tr("input box", "输入框")
            XCTAssertFalse(before.contains(box), "nothing was pasted: \(before)")
            XCTAssertTrue(after.contains(box), "the text may be waiting there: \(after)")
            XCTAssertTrue(after.contains(l.tr("Check it before sending again", "再发之前先看一眼")))
        }
        // Neither is a retry: the user decides, after looking.
        let doc = ScreenshotDocument(image: blankImage(width: 10, height: 10), pointSize: CGSize(width: 10, height: 10))
        let m = ScreenshotEditorModel(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/none.png"), target: "%1")
        m.status = .result(.heldAfterPaste)
        XCTAssertFalse(m.isRetry)
        XCTAssertTrue(ScreenshotStatusText.isProblem(.result(.heldAfterPaste)))
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
        let own: [ScreenshotSendResult] = [.agentsUnreadable, .refusedWaiting, .heldAfterPaste, .paneGone, .duplicate, .delivered(queued: false)]
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

    // MARK: canvas — the drag path the user felt lag on

    /// A drag used to set a published property on every mouse event, and every one re-rendered
    /// the whole editor and rescaled the capture. Now a drag touches the marks view only: the
    /// model publishes nothing, the capture layer keeps its contents, and the shape lands as
    /// one undoable mark at the end.
    func testDragTouchesOnlyTheMarks() throws {
        let doc = ScreenshotDocument(image: blankImage(width: 2000, height: 1200), pointSize: CGSize(width: 1000, height: 600))
        let model = ScreenshotEditorModel(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/none.png"), target: nil)
        let canvas = AnnotationCanvasView(model: model, displaySize: CGSize(width: 500, height: 300))
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 500, height: 300), styleMask: [.borderless],
                              backing: .buffered, defer: true)
        window.contentView = canvas
        let image = try XCTUnwrap(canvas.imageContents as AnyObject?)
        var published = 0
        let watch = model.objectWillChange.sink { published += 1 }
        defer { watch.cancel() }
        var docChanges = 0
        let watchDoc = doc.objectWillChange.sink { docChanges += 1 }
        defer { watchDoc.cancel() }

        for i in 0..<120 {
            canvas.drag(from: CGPoint(x: 100, y: 100), to: CGPoint(x: 140 + CGFloat(i), y: 160 + CGFloat(i) / 2),
                        tool: .rect, color: .red)
        }
        XCTAssertEqual(published, 0, "a drag must not re-render the editor")
        XCTAssertEqual(docChanges, 0, "a drag must not change the document")
        XCTAssertNotNil(canvas.marks.draft)
        XCTAssertTrue(canvas.marks.needsDisplay)
        // Render the marks view on its own: the draft is drawn there, in red, where it was dragged.
        let rep = try XCTUnwrap(canvas.marks.bitmapImageRepForCachingDisplay(in: canvas.marks.bounds))
        canvas.marks.cacheDisplay(in: canvas.marks.bounds, to: rep)
        XCTAssertGreaterThan(canvas.marks.draws, 0)
        // The draft's left edge, x = 100 image points = 50 view points, halfway down it.
        let k = rep.pixelsWide / Int(canvas.marks.bounds.width)
        let edge = try XCTUnwrap(rep.colorAt(x: 50 * k, y: 70 * k))
        XCTAssertGreaterThan(edge.redComponent, 0.8)
        XCTAssertLessThan(edge.greenComponent, 0.5)
        XCTAssertTrue(canvas.imageContents as AnyObject? === image, "the capture is never rescaled by a drag")

        canvas.finish(from: CGPoint(x: 100, y: 100), to: CGPoint(x: 300, y: 220), tool: .rect, color: .blue)
        XCTAssertNil(canvas.marks.draft)
        XCTAssertEqual(doc.items.count, 1)
        XCTAssertEqual(doc.items.first?.kind, .rect(CGRect(x: 100, y: 100, width: 200, height: 120)))
        XCTAssertEqual(canvas.marks.items.count, 1, "the marks view follows the document")
        XCTAssertEqual(published, 0)
    }

    /// The capture is resampled once to the pixels it is shown at, never enlarged.
    func testCanvasScalesTheCaptureOnce() {
        let big = blankImage(width: 4000, height: 2000)
        let small = AnnotationCanvasView.scaled(big, to: CGSize(width: 1000, height: 500))
        XCTAssertEqual(small.width, 1000)
        XCTAssertEqual(small.height, 500)
        let tiny = blankImage(width: 200, height: 100)
        XCTAssertTrue(AnnotationCanvasView.scaled(tiny, to: CGSize(width: 400, height: 200)) === tiny)
    }

    func testSingleKeysChooseToolsAndColours() {
        XCTAssertEqual(AnnotationCanvasView.keyAction("a"), .tool(.arrow))
        XCTAssertEqual(AnnotationCanvasView.keyAction("r"), .tool(.rect))
        XCTAssertEqual(AnnotationCanvasView.keyAction("t"), .tool(.text))
        XCTAssertEqual(AnnotationCanvasView.keyAction("o"), .tool(.ellipse))
        XCTAssertEqual(AnnotationCanvasView.keyAction("m"), .tool(.mosaic))
        XCTAssertEqual(AnnotationCanvasView.keyAction("["), .thinner)
        XCTAssertEqual(AnnotationCanvasView.keyAction("]"), .thicker)
        XCTAssertEqual(AnnotationWidth.thin.thinner, .thin, "the widths stop at their ends")
        XCTAssertEqual(AnnotationWidth.thick.thicker, .thick)
        XCTAssertEqual(AnnotationWidth.thin.thicker.thicker, .thick)
        XCTAssertEqual(AnnotationCanvasView.keyAction("1"), .color(.red))
        XCTAssertEqual(AnnotationCanvasView.keyAction("2"), .color(.yellow))
        XCTAssertEqual(AnnotationCanvasView.keyAction("3"), .color(.blue))
        XCTAssertNil(AnnotationCanvasView.keyAction("x"))
    }

    /// Copy and Save live in the title bar, and their keys work from there.
    func testTitleBarCarriesCopyAndSave() throws {
        let c = ScreenshotEditorController()
        let pb = NSPasteboard(name: NSPasteboard.Name("gtmux-test-\(UUID().uuidString)"))
        c.pasteboard = pb
        defer { pb.releaseGlobally() }
        let doc = ScreenshotDocument(image: blankImage(width: 120, height: 80), pointSize: CGSize(width: 60, height: 40))
        c.show(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/gtmux-test-3.png"), target: nil, store: AgentStore(), l10n: L10n.shared)
        defer { if let w = c.window { w.delegate = nil; w.close() } }
        let w = try XCTUnwrap(c.window)
        XCTAssertEqual(w.subtitle, "60 × 40 · @2x")
        // The title names gtmux, and its mark sits before it.
        XCTAssertEqual(w.title, ScreenshotLayout.title(L10n.shared))
        XCTAssertTrue(w.title.hasPrefix("gtmux "))
        XCTAssertTrue(w.titlebarAccessoryViewControllers.contains { $0.layoutAttribute == .leading })
        let ids = w.toolbar?.items.map(\.itemIdentifier) ?? []
        XCTAssertTrue(ids.contains(ScreenshotEditorController.copyItem))
        XCTAssertTrue(ids.contains(ScreenshotEditorController.saveItem))
        // Icons only: no words beside them; each says what it is to VoiceOver and in its tooltip.
        for id in [ScreenshotEditorController.copyItem, ScreenshotEditorController.saveItem] {
            let b = try XCTUnwrap(w.toolbar?.items.first { $0.itemIdentifier == id }?.view as? NSButton)
            XCTAssertEqual(b.title, "")
            XCTAssertEqual(b.imagePosition, .imageOnly)
            XCTAssertFalse((b.accessibilityLabel() ?? "").isEmpty)
            XCTAssertTrue((b.toolTip ?? "").contains("⌘"), "the tooltip names the key")
        }

        // The keys as the keyboard sends them, through the window's own dispatch: what the
        // button's properties say is not what AppKit does with them (#1291 M1).
        var saves = 0
        c.saveForTesting = { saves += 1 }
        func key(_ chars: String, _ flags: NSEvent.ModifierFlags, _ code: UInt16) -> NSEvent {
            NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: flags, timestamp: 0,
                             windowNumber: w.windowNumber, context: nil, characters: chars,
                             charactersIgnoringModifiers: chars, isARepeat: false, keyCode: code)!
        }
        // ⌘C stays text copy: the image button must not take it.
        _ = w.performKeyEquivalent(with: key("c", [.command], 8))
        XCTAssertNotEqual(c.model?.status, .copied, "⌘C copied the image over the user's text copy")
        XCTAssertNil(pb.data(forType: .png))
        // ⇧⌘C copies the image.
        XCTAssertTrue(w.performKeyEquivalent(with: key("C", [.command, .shift], 8)))
        XCTAssertEqual(c.model?.status, .copied)
        XCTAssertNotNil(pb.data(forType: .png))
        // ⌘S saves.
        XCTAssertTrue(w.performKeyEquivalent(with: key("s", [.command], 1)))
        XCTAssertEqual(saves, 1)
    }

    /// A note grown to its four lines takes room from below the capture; the capture stays
    /// where it was, whole (#1291 L1).
    func testALongNoteDoesNotMoveTheCapture() throws {
        let display = CGSize(width: 800, height: 500)
        let doc = ScreenshotDocument(image: blankImage(width: 1600, height: 1000), pointSize: display)
        let model = ScreenshotEditorModel(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/none.png"), target: nil)
        let view = ScreenshotEditorView(model: model, store: AgentStore(), l10n: L10n.shared, displaySize: display,
                                        onSend: {}, onCancel: {}, onShowPane: { _ in })
        let host = NSHostingView(rootView: view)
        host.frame = NSRect(origin: .zero, size: ScreenshotLayout.windowSize(display: display))
        let w = NSWindow(contentRect: host.frame, styleMask: [.borderless], backing: .buffered, defer: true)
        w.contentView = host
        func canvasFrame() throws -> NSRect {
            host.layoutSubtreeIfNeeded()
            RunLoop.main.run(until: Date().addingTimeInterval(0.2))
            host.layoutSubtreeIfNeeded()
            let canvas = try XCTUnwrap(ScreenshotEditorController.canvas(in: host))
            return canvas.convert(canvas.bounds, to: nil)
        }
        let empty = try canvasFrame()
        XCTAssertEqual(empty.size, display)
        model.note = (1...6).map { "line \($0) of a long note about what to look at" }.joined(separator: "\n")
        let long = try canvasFrame()
        XCTAssertEqual(long, empty, "the note pushed the capture")
        model.status = .result(.heldAfterPaste)
        model.sendingTarget = ScreenshotSendTarget(paneID: "%5", session: "dev", name: "Codex")
        XCTAssertEqual(try canvasFrame(), empty, "two lines of status pushed the capture")
    }

    func testShowThePaneOnlyWhereALookHelps() {
        let t = ScreenshotSendTarget(paneID: "%5", session: "dev", name: "Codex")
        XCTAssertEqual(ScreenshotStatusText.paneToShow(.result(.heldAfterPaste), target: t), "%5")
        XCTAssertEqual(ScreenshotStatusText.paneToShow(.result(.refusedWaiting), target: t), "%5")
        XCTAssertEqual(ScreenshotStatusText.paneToShow(.result(.notConfirmed("")), target: t), "%5")
        XCTAssertNil(ScreenshotStatusText.paneToShow(.result(.delivered(queued: false)), target: t))
        XCTAssertNil(ScreenshotStatusText.paneToShow(.result(.duplicate), target: t))
        XCTAssertNil(ScreenshotStatusText.paneToShow(.result(.heldAfterPaste), target: nil))
        XCTAssertNil(ScreenshotStatusText.paneToShow(.sending, target: t))
    }

    func testRecentPaneIsTheOneLastTypedIn() {
        let a = [agent("%1", "w"), agent("%2", "x", status: "waiting"), agent("%3", "y")]
        XCTAssertEqual(ScreenshotTargets.recentPane(candidates: a, viewedAt: ["%1": 10, "%2": 30, "%3": 20]), "%2")
        XCTAssertNil(ScreenshotTargets.recentPane(candidates: a, viewedAt: [:]))
        XCTAssertEqual(ScreenshotLayout.subtitle(pointSize: CGSize(width: 1440, height: 900), scale: 1), "1440 × 900")
    }

    // MARK: layout

    /// The key hints sit right under the capture and the composer right under them: no band
    /// of empty backdrop in between (it was the room a four-line note kept, 2026-10-04).
    func testTheEditorIsCompact() throws {
        let display = CGSize(width: 800, height: 400)
        let doc = ScreenshotDocument(image: blankImage(width: 1600, height: 800), pointSize: display)
        let model = ScreenshotEditorModel(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/none.png"), target: nil)
        let view = ScreenshotEditorView(model: model, store: AgentStore(), l10n: L10n.shared, displaySize: display,
                                        onSend: {}, onCancel: {}, onShowPane: { _ in })
        let host = NSHostingView(rootView: view)
        host.frame = NSRect(origin: .zero, size: ScreenshotLayout.windowSize(display: display))
        let w = NSWindow(contentRect: host.frame, styleMask: [.borderless], backing: .buffered, defer: true)
        w.contentView = host
        host.layoutSubtreeIfNeeded()
        RunLoop.main.run(until: Date().addingTimeInterval(0.2))
        host.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(ScreenshotEditorController.canvas(in: host))
        let frame = canvas.convert(canvas.bounds, to: nil) // window space: origin bottom-left
        XCTAssertEqual(frame.size, display)
        XCTAssertEqual(host.frame.height - frame.maxY, ScreenshotLayout.stageTop, accuracy: 0.5)
        // Below the capture: the hint line, then the composer — nothing else.
        XCTAssertEqual(frame.minY, ScreenshotLayout.composerHeight + 1 + ScreenshotLayout.stageBottom, accuracy: 0.5)
        XCTAssertLessThanOrEqual(ScreenshotLayout.stageBottom, 32)
        // The composer is the target row and the note: no band held for a status line, which
        // sits in the target row (an empty band at the bottom of the window, 2026-10-04).
        XCTAssertLessThanOrEqual(ScreenshotLayout.composerHeight, ScreenshotLayout.targetRowHeight + 80)
    }

    func testSaveNameIsShortAndPlainlyGtmux() {
        var c = DateComponents()
        c.year = 2026; c.month = 10; c.day = 4; c.hour = 21; c.minute = 0; c.second = 46
        let date = Calendar.current.date(from: c)!
        XCTAssertEqual(ScreenshotLayout.saveName(now: date), "gtmux-shot-1004-210046.png")
    }

    // MARK: oval, mosaic, widths

    func testOvalsAndSquares() {
        let doc = ScreenshotDocument(image: blankImage(width: 200, height: 200), pointSize: CGSize(width: 200, height: 200))
        guard case let .ellipse(r)? = doc.shape(.ellipse, from: CGPoint(x: 10, y: 10), to: CGPoint(x: 90, y: 50), color: .red)?.kind else {
            return XCTFail("a drag with the oval tool is an oval")
        }
        XCTAssertEqual(r, CGRect(x: 10, y: 10, width: 80, height: 40))
        // Shift: the shorter side wins, in the drag's own direction, and stays on the image.
        guard case let .ellipse(c)? = doc.shape(.ellipse, from: CGPoint(x: 100, y: 100), to: CGPoint(x: 40, y: 180),
                                                color: .red, square: true)?.kind else { return XCTFail("a circle") }
        XCTAssertEqual(c, CGRect(x: 40, y: 100, width: 60, height: 60))
        guard case let .rect(sq)? = doc.shape(.rect, from: CGPoint(x: 10, y: 10), to: CGPoint(x: 50, y: 90),
                                              color: .red, square: true)?.kind else { return XCTFail("a square") }
        XCTAssertEqual(sq.width, sq.height)
        XCTAssertNil(doc.shape(.ellipse, from: CGPoint(x: 10, y: 10), to: CGPoint(x: 90, y: 11), color: .red), "a flat drag is no oval")
        XCTAssertNil(doc.shape(.mosaic, from: CGPoint(x: 10, y: 10), to: CGPoint(x: 12, y: 12), color: .red))
        XCTAssertEqual(doc.shape(.arrow, from: .zero, to: CGPoint(x: 50, y: 50), color: .red, width: .thick)?.width, .thick)
    }

    /// Fine black-and-white stripes, too fine to survive a mosaic.
    private func stripes(width: Int, height: Int) -> CGImage {
        let ctx = CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                            space: CGColorSpace(name: CGColorSpace.sRGB)!,
                            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        ctx.setFillColor(CGColor(gray: 1, alpha: 1))
        ctx.fill(CGRect(x: 0, y: 0, width: width, height: height))
        ctx.setFillColor(CGColor(gray: 0, alpha: 1))
        for x in stride(from: 0, to: width, by: 2) { ctx.fill(CGRect(x: x, y: 0, width: 1, height: height)) }
        return ctx.makeImage()!
    }

    /// Inside a mosaic, each block is one colour and the stripes are gone; outside, nothing
    /// changed. At 2× the blocks are twice the pixels, the same size on screen.
    func testMosaicHidesWhatIsUnderIt() throws {
        let img = stripes(width: 400, height: 200)
        let area = CGRect(x: 20, y: 20, width: 96, height: 48) // points; 8 × 4 blocks at medium (12pt)
        let out = try XCTUnwrap(AnnotationRenderer.flatten(image: img, pointSize: CGSize(width: 200, height: 100),
                                                           items: [Annotation(.mosaic(area), color: .red, width: .medium)]))
        // One block: pixels 40..63 across, 40..63 down (12pt × 2). All one value, near mid grey.
        let first = pixel(out, 41, 41)
        for x in stride(from: 41, to: 63, by: 3) {
            for y in stride(from: 41, to: 63, by: 3) {
                let p = pixel(out, x, y)
                XCTAssertEqual(Int(p.r), Int(first.r), accuracy: 2, "a block is one colour at (\(x), \(y))")
            }
        }
        XCTAssertTrue((60...200).contains(Int(first.r)), "stripes average to grey, got \(first.r)")
        // Outside the mosaic the stripes are untouched.
        XCTAssertNotEqual(pixel(out, 300, 150).r, pixel(out, 301, 150).r)
        // A thick mosaic has bigger blocks: fewer of them across the same area.
        let tile = try XCTUnwrap(AnnotationRenderer.mosaicTile(.init(image: img, scale: 2), rect: area, block: AnnotationWidth.thick.block))
        XCTAssertLessThan(tile.width, 8)
    }

    /// The editor draws a mosaic with the export's own code, from the same capture: on screen
    /// it is opaque blocks over the capture, not a tint the stripes show through.
    func testMosaicIsOpaqueBlocksOnScreen() throws {
        let img = stripes(width: 200, height: 100)
        let doc = ScreenshotDocument(image: img, pointSize: CGSize(width: 200, height: 100))
        doc.add(Annotation(.mosaic(CGRect(x: 12, y: 12, width: 60, height: 36)), color: .red))
        let marks = MarksView(factor: 1, source: .init(image: img, scale: 1))
        marks.frame = NSRect(x: 0, y: 0, width: 200, height: 100)
        marks.items = doc.items
        let rep = try XCTUnwrap(marks.bitmapImageRepForCachingDisplay(in: marks.bounds))
        marks.cacheDisplay(in: marks.bounds, to: rep)
        let k = CGFloat(rep.pixelsWide) / 200
        func at(_ x: CGFloat, _ y: CGFloat) -> NSColor { rep.colorAt(x: Int(x * k), y: Int(y * k))! }
        XCTAssertEqual(at(15, 15).alphaComponent, 1, accuracy: 0.01, "a block covers the capture")
        XCTAssertEqual(at(15, 15).redComponent, at(22, 22).redComponent, accuracy: 0.02, "one block, one colour")
        XCTAssertEqual(at(150, 80).alphaComponent, 0, accuracy: 0.01, "outside the mosaic the capture shows")
    }

    func testWidthsChangeStrokesAndText() throws {
        func ink(_ width: AnnotationWidth) throws -> Int {
            let out = try XCTUnwrap(AnnotationRenderer.flatten(
                image: blankImage(width: 200, height: 100), pointSize: CGSize(width: 200, height: 100),
                items: [Annotation(.rect(CGRect(x: 20, y: 20, width: 160, height: 60)), color: .red, width: width)]))
            var n = 0
            for y in 10..<30 where pixel(out, 100, y).g < 128 { n += 1 }
            return n
        }
        let thin = try ink(.thin), thick = try ink(.thick)
        XCTAssertLessThan(thin, thick)
        XCTAssertGreaterThan(AnnotationRenderer.textBox("gtmux", at: .zero, size: AnnotationWidth.thick.fontSize).height,
                             AnnotationRenderer.textBox("gtmux", at: .zero, size: AnnotationWidth.thin.fontSize).height)
    }

    func testEditorNeverEnlargesACapture() {
        XCTAssertEqual(ScreenshotLayout.displaySize(image: CGSize(width: 300, height: 200), within: CGSize(width: 1440, height: 900)),
                       CGSize(width: 300, height: 200))
        let big = ScreenshotLayout.displaySize(image: CGSize(width: 3000, height: 2000), within: CGSize(width: 1440, height: 900))
        XCTAssertLessThanOrEqual(big.width, 1440 * 0.85)
        XCTAssertLessThanOrEqual(big.height, 900 * 0.85 - ScreenshotLayout.chromeHeight)
        XCTAssertEqual(big.width / big.height, 1.5, accuracy: 0.01, "the aspect ratio is kept")
    }
}
