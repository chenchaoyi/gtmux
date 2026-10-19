import AppKit
import XCTest
@testable import GtmuxBar

final class MenuPopoverPresentationTests: XCTestCase {
    private func fixture() -> (NSWindow, NSPopover, MenuPopoverPresentation) {
        _ = NSApplication.shared
        let window = NSWindow(contentRect: NSRect(x: 100, y: 100, width: 500, height: 400),
                              styleMask: [.titled], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        let anchor = NSView(frame: NSRect(x: 20, y: 200, width: 40, height: 24))
        window.contentView?.addSubview(anchor)
        window.orderFront(nil)
        let popover = NSPopover()
        popover.animates = false
        let controller = NSViewController()
        controller.view = NSView(frame: NSRect(x: 0, y: 0, width: 240, height: 200))
        popover.contentViewController = controller
        let presentation = MenuPopoverPresentation(popover: popover)
        presentation.prepareToShow(restoring: false)
        popover.show(relativeTo: anchor.bounds, of: anchor, preferredEdge: .maxX)
        return (window, popover, presentation)
    }

    func testRestoreKeepsMenuUntilExplicitCloseAndNextOpeningIsTransient() {
        let (window, popover, presentation) = fixture()
        defer { popover.close(); window.close() }
        XCTAssertTrue(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
        presentation.restoreStarted()
        XCTAssertEqual(popover.behavior, .applicationDefined)
        NotificationCenter.default.post(name: NSApplication.didResignActiveNotification, object: NSApp)
        XCTAssertTrue(popover.isShown)
        presentation.restoreStarted() // duplicate action does not stack monitors
        popover.performClose(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
        presentation.restoreStarted() // late callback never reopens/protects a closed menu
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
        presentation.prepareToShow(restoring: true)
        XCTAssertEqual(popover.behavior, .applicationDefined, "reopening during Restore still shows progress")
        presentation.prepareToShow(restoring: false)
        XCTAssertEqual(popover.behavior, .transient)
    }

    func testAutomaticReaderDoesNotReplaceMenuAndItsClicksAreAllowed() {
        let (window, popover, presentation) = fixture()
        let reader = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 320, height: 240),
                              styleMask: [.titled, .closable], backing: .buffered, defer: false)
        reader.isReleasedWhenClosed = false
        defer { popover.close(); reader.close(); window.close() }
        XCTAssertTrue(presentation.presentAutomaticNotes {
            XCTAssertTrue(popover.isShown, "the reader must not replace the menu")
            XCTAssertEqual(popover.behavior, .applicationDefined, "protect before changing focus")
            reader.makeKeyAndOrderFront(nil)
            return reader
        })
        RunLoop.current.run(until: Date().addingTimeInterval(0.1))
        XCTAssertTrue(popover.isShown)
        presentation.prepareToShow(restoring: false) // an existing presentation must not lose its hold
        XCTAssertEqual(popover.behavior, .applicationDefined)
        XCTAssertFalse(presentation.dismissesForClick(in: reader))
        XCTAssertFalse(presentation.dismissesForClick(in: popover.contentViewController?.view.window))
        XCTAssertTrue(presentation.dismissesForClick(in: window))
        XCTAssertTrue(presentation.dismissesForClick(in: nil))
        popover.performClose(nil)
        XCTAssertFalse(presentation.presentAutomaticNotes { XCTFail("late notes must wait for next opening"); return reader })
        XCTAssertTrue(presentation.dismissesForClick(in: reader), "closed presentation must release its companion")
        XCTAssertEqual(popover.behavior, .transient)
    }

    func testEscapeInMenuClosesTheProtectedPresentation() {
        let (window, popover, presentation) = fixture()
        defer { popover.close(); window.close() }
        presentation.restoreStarted()
        let menuWindow = popover.contentViewController!.view.window!
        menuWindow.makeKey()
        let escape = NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: 0,
                                      windowNumber: menuWindow.windowNumber, context: nil,
                                      characters: "\u{1b}", charactersIgnoringModifiers: "\u{1b}", isARepeat: false, keyCode: 53)!
        NSApp.postEvent(escape, atStart: true)
        if let event = NSApp.nextEvent(matching: .keyDown, until: Date().addingTimeInterval(1), inMode: .default, dequeue: true) {
            NSApp.sendEvent(event)
        }
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
    }

    func testLocalOutsideClickClosesWithoutConsumingTheClick() {
        let (window, popover, presentation) = fixture()
        defer { popover.close(); window.close() }
        presentation.restoreStarted()
        let click = NSEvent.mouseEvent(with: .leftMouseDown, location: .zero, modifierFlags: [],
                                       timestamp: 0, windowNumber: window.windowNumber,
                                       context: nil, eventNumber: 0, clickCount: 1, pressure: 1)!
        NSApp.postEvent(click, atStart: true)
        if let event = NSApp.nextEvent(matching: .leftMouseDown, until: Date().addingTimeInterval(1), inMode: .default, dequeue: true) {
            NSApp.sendEvent(event)
        } else { XCTFail("outside click was not delivered") }
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
    }

    func testScreenshotSelectionKeepsMenuAndCancellationRestoresOutsideDismissal() {
        let (window, popover, presentation) = fixture()
        defer { popover.close(); window.close() }
        presentation.screenshotStarted()
        XCTAssertTrue(popover.isShown)
        XCTAssertEqual(popover.behavior, .applicationDefined)
        func clickOutside() {
            let click = NSEvent.mouseEvent(with: .leftMouseDown, location: .zero, modifierFlags: [],
                                          timestamp: 0, windowNumber: window.windowNumber,
                                          context: nil, eventNumber: 0, clickCount: 1, pressure: 1)!
            NSApp.postEvent(click, atStart: true)
            if let event = NSApp.nextEvent(matching: .leftMouseDown, until: Date().addingTimeInterval(1), inMode: .default, dequeue: true) {
                NSApp.sendEvent(event)
            } else { XCTFail("selection click was not delivered") }
        }
        clickOutside() // the selector is another window; its click must reach it
        XCTAssertTrue(popover.isShown)
        presentation.screenshotFinished(.cancelled)
        XCTAssertTrue(popover.isShown)
        clickOutside()
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
    }

    func testScreenshotSuccessClosesAndLateCompletionDoesNotCloseANewOpening() {
        let (window, popover, presentation) = fixture()
        defer { popover.close(); window.close() }
        presentation.screenshotStarted()
        presentation.screenshotFinished(.failed("capture unavailable"))
        XCTAssertTrue(popover.isShown)
        presentation.screenshotStarted()
        presentation.screenshotFinished(.image(URL(fileURLWithPath: "/tmp/unused-shot.png")))
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
        presentation.screenshotStarted() // hotkey while closed must not reopen it
        XCTAssertEqual(popover.behavior, .transient)
        presentation.prepareToShow(restoring: false)
        let anchor = window.contentView!.subviews.first!
        popover.show(relativeTo: anchor.bounds, of: anchor, preferredEdge: .maxX)
        presentation.screenshotFinished(.image(URL(fileURLWithPath: "/tmp/unused-shot.png")))
        XCTAssertTrue(popover.isShown)
        XCTAssertEqual(popover.behavior, .transient)
    }

    func testCaptureFlowProtectsBeforeSelectorAndFinishesCancellationOnce() {
        let (window, popover, presentation) = fixture()
        defer { popover.close(); window.close() }
        let controller = ScreenshotController()
        controller.access.preflight = { true }
        let completed = expectation(description: "cancelled selection")
        var prepared = 0, finished = 0
        controller.capture = { _, completion in
            XCTAssertTrue(popover.isShown)
            XCTAssertEqual(popover.behavior, .applicationDefined)
            completion(.cancelled)
        }
        let prepare = { prepared += 1; presentation.screenshotStarted() }
        let finish: (CaptureOutcome) -> Void = { outcome in
            finished += 1
            presentation.screenshotFinished(outcome)
            completed.fulfill()
        }
        let store = AgentStore(readPoll: { _ in nil })
        controller.start(store: store, l10n: .shared, prepareSurfaces: prepare, finishSurfaces: finish)
        controller.start(store: store, l10n: .shared, prepareSurfaces: prepare, finishSurfaces: finish)
        wait(for: [completed], timeout: 3)
        XCTAssertEqual(prepared, 1, "reentry must not stack presentation protection")
        XCTAssertEqual(finished, 1)
        XCTAssertTrue(popover.isShown)
    }
}
