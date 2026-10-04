import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotSelectorTests: XCTestCase {

    // MARK: geometry

    func testADragIsARectangleOnItsDisplay() {
        let bounds = CGRect(x: 0, y: 0, width: 1440, height: 900)
        XCTAssertEqual(SelectionGeometry.rect(from: CGPoint(x: 300, y: 200), to: CGPoint(x: 100, y: 50), in: bounds),
                       CGRect(x: 100, y: 50, width: 200, height: 150), "any direction")
        XCTAssertEqual(SelectionGeometry.rect(from: CGPoint(x: 1400, y: 880), to: CGPoint(x: 1600, y: 1000), in: bounds),
                       CGRect(x: 1400, y: 880, width: 40, height: 20), "a drag off the edge stops at it")
    }

    /// On Retina a point is two pixels; each edge lands on the nearest pixel.
    func testSelectionEdgesLandOnPixels() {
        let r = SelectionGeometry.pixelRect(CGRect(x: 10.3, y: 20.7, width: 50.4, height: 30.1), scale: 2, imageWidth: 4000, imageHeight: 3000)
        XCTAssertEqual(r, CGRect(x: 21, y: 41, width: 100, height: 61))
        XCTAssertEqual(SelectionGeometry.pixelRect(CGRect(x: 1990, y: 0, width: 100, height: 10), scale: 2, imageWidth: 4000, imageHeight: 3000).maxX,
                       4000, "never past the image")
    }

    /// The loupe is centred on the pixel under the hot spot, even at the image's edge.
    func testLoupeIsCentredOnThePointersPixel() {
        let s = SelectionGeometry.loupeSample(at: CGPoint(x: 10.3, y: 20.7), scale: 2, cells: 15)
        XCTAssertEqual(s, CGRect(x: 20 - 7, y: 41 - 7, width: 15, height: 15))
        let corner = SelectionGeometry.loupeSample(at: .zero, scale: 1, cells: 15)
        XCTAssertEqual(corner.midX, 0.5, accuracy: 0.001, "past the edge rather than shifted: the centre cell is still the pointer's")
    }

    func testLoupeStaysOnScreen() {
        let size = CGSize(width: 135, height: 135), screen = CGSize(width: 1440, height: 900)
        let mid = SelectionGeometry.loupeFrame(pointer: CGPoint(x: 400, y: 300), size: size, in: screen)
        XCTAssertGreaterThan(mid.minX, 400); XCTAssertGreaterThan(mid.minY, 300)
        let corner = SelectionGeometry.loupeFrame(pointer: CGPoint(x: 1430, y: 890), size: size, in: screen)
        XCTAssertLessThan(corner.maxX, 1430, "flipped left of the pointer")
        XCTAssertLessThan(corner.maxY, 890, "flipped above it")
        XCTAssertTrue(CGRect(origin: .zero, size: screen).contains(corner))
    }

    func testGlobalRectanglesBecomeLocal() {
        // A second display to the right of a 1440-wide main one.
        let local = SelectionGeometry.local(CGRect(x: 1500, y: 100, width: 300, height: 200),
                                            displayBounds: CGRect(x: 1440, y: 0, width: 1920, height: 1080))
        XCTAssertEqual(local, CGRect(x: 60, y: 100, width: 300, height: 200))
    }

    /// Space picks the frontmost ordinary window under the pointer — not a menu, not a
    /// sliver, never gtmux's own overlay.
    func testWindowModePicksTheFrontmostOrdinaryWindow() {
        let me: pid_t = 42
        let list = [
            WindowInfo(id: 1, bounds: CGRect(x: 0, y: 0, width: 2000, height: 1200), layer: 1000, pid: me),  // our overlay
            WindowInfo(id: 2, bounds: CGRect(x: 0, y: 0, width: 1440, height: 25), layer: 24, pid: 7),        // menu bar
            WindowInfo(id: 3, bounds: CGRect(x: 90, y: 90, width: 20, height: 20), layer: 0, pid: 8),          // a sliver
            WindowInfo(id: 4, bounds: CGRect(x: 50, y: 50, width: 600, height: 400), layer: 0, pid: 9),        // front
            WindowInfo(id: 5, bounds: CGRect(x: 0, y: 0, width: 1000, height: 800), layer: 0, pid: 10),        // behind
        ]
        XCTAssertEqual(SelectionGeometry.window(at: CGPoint(x: 100, y: 100), in: list, excludingPID: me)?.id, 4)
        XCTAssertEqual(SelectionGeometry.window(at: CGPoint(x: 900, y: 700), in: list, excludingPID: me)?.id, 5)
        XCTAssertNil(SelectionGeometry.window(at: CGPoint(x: 1300, y: 1100), in: list, excludingPID: me))
    }

    /// Four quadrants of four colours: a crop at 2× takes the right pixels and keeps the DPI.
    func testCropIsTheSelectionAtFullResolution() throws {
        let img = quadrants(width: 400, height: 200)
        let cut = try XCTUnwrap(SelectionGeometry.crop(img, rect: CGRect(x: 100, y: 0, width: 100, height: 50), scale: 2))
        XCTAssertEqual([cut.width, cut.height], [200, 100])
        XCTAssertEqual(color(cut, 10, 10), "B", "the top-right quadrant (top-left origin)")
        XCTAssertNil(SelectionGeometry.crop(img, rect: CGRect(x: 10, y: 10, width: 0.1, height: 0.1), scale: 2))
        let png = try XCTUnwrap(AnnotationRenderer.pngData(cut, pointSize: CGSize(width: 100, height: 50)))
        XCTAssertEqual(NSBitmapImageRep(data: png)?.size, CGSize(width: 100, height: 50), "reopens at its point size")
    }

    // MARK: the overlay, driven by real events

    private final class Recorder: SelectionViewDelegate {
        var selected: CGRect?
        var window: WindowInfo?
        var cancelled = 0
        var modes: [Bool] = []
        func pointerEntered(_ view: SelectionView) {}
        func mode(_ windowMode: Bool) { modes.append(windowMode) }
        func cancel() { cancelled += 1 }
        func select(_ rect: CGRect, on display: FrozenDisplay) { selected = rect }
        func select(window info: WindowInfo, scale: CGFloat) { window = info }
    }

    private func overlay(windows: [WindowInfo] = []) throws -> (SelectionView, NSWindow, Recorder) {
        guard let screen = NSScreen.screens.first else { throw XCTSkip("no display") }
        let size = CGSize(width: 800, height: 500)
        let display = FrozenDisplay(screen: screen, displayID: 1, bounds: CGRect(origin: .zero, size: size), scale: 2,
                                    image: quadrants(width: 1600, height: 1000))
        let rec = Recorder()
        let view = SelectionView(display: display, windows: windows, delegate: rec)
        view.frame = NSRect(origin: .zero, size: size)
        let w = NSWindow(contentRect: view.frame, styleMask: .borderless, backing: .buffered, defer: true)
        w.contentView = view
        return (view, w, rec)
    }

    /// A mouse event at a top-left point in the view.
    private func mouse(_ type: NSEvent.EventType, _ p: CGPoint, in w: NSWindow) -> NSEvent {
        NSEvent.mouseEvent(with: type, location: NSPoint(x: p.x, y: w.frame.height - p.y), modifierFlags: [], timestamp: 0,
                           windowNumber: w.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1)!
    }

    private func key(_ code: UInt16, _ chars: String, in w: NSWindow) -> NSEvent {
        NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: 0, windowNumber: w.windowNumber,
                         context: nil, characters: chars, charactersIgnoringModifiers: chars, isARepeat: false, keyCode: code)!
    }

    func testDraggingSelectsAndTheLoupeFollows() throws {
        let (v, w, rec) = try overlay()
        v.mouseMoved(with: mouse(.mouseMoved, CGPoint(x: 120, y: 80), in: w))
        XCTAssertTrue(v.loupeShowsPixels, "the loupe shows the pixels under the pointer")
        XCTAssertEqual(v.readoutText, "120, 80", "and where the pointer is")
        v.mouseDown(with: mouse(.leftMouseDown, CGPoint(x: 120, y: 80), in: w))
        v.mouseDragged(with: mouse(.leftMouseDragged, CGPoint(x: 420, y: 230), in: w))
        XCTAssertEqual(v.readoutText, "300 × 150", "while dragging it says the size")
        v.mouseUp(with: mouse(.leftMouseUp, CGPoint(x: 420, y: 230), in: w))
        XCTAssertEqual(rec.selected, CGRect(x: 120, y: 80, width: 300, height: 150))
    }

    func testAClickSelectsNothingAndEscCancels() throws {
        let (v, w, rec) = try overlay()
        v.mouseDown(with: mouse(.leftMouseDown, CGPoint(x: 50, y: 50), in: w))
        v.mouseUp(with: mouse(.leftMouseUp, CGPoint(x: 51, y: 51), in: w))
        XCTAssertNil(rec.selected, "a click is not a selection: still selecting")
        XCTAssertEqual(rec.cancelled, 0)
        v.keyDown(with: key(53, "\u{1b}", in: w))
        XCTAssertEqual(rec.cancelled, 1)
    }

    func testSpacePicksAWholeWindow() throws {
        let target = WindowInfo(id: 9, bounds: CGRect(x: 100, y: 100, width: 400, height: 300), layer: 0, pid: 1)
        let (v, w, rec) = try overlay(windows: [target])
        v.keyDown(with: key(49, " ", in: w))
        XCTAssertEqual(rec.modes, [true], "Space switches to window mode")
        v.windowMode = true // what the session does for every display
        v.mouseMoved(with: mouse(.mouseMoved, CGPoint(x: 200, y: 200), in: w))
        XCTAssertEqual(v.hoveredWindow, target)
        XCTAssertEqual(v.readoutText, "400 × 300")
        v.mouseDown(with: mouse(.leftMouseDown, CGPoint(x: 200, y: 200), in: w))
        v.mouseUp(with: mouse(.leftMouseUp, CGPoint(x: 200, y: 200), in: w))
        XCTAssertEqual(rec.window, target)
        XCTAssertNil(rec.selected)
    }

    func testTheCrosshairIsTheBrandsCyan() {
        XCTAssertEqual(ScreenshotSelector.brand, Theme.Status.workingNS, "the lit pane of the brand mark (DESIGN §12)")
    }

    // MARK: helpers

    /// Red, blue over green, yellow: four quadrants, top-left origin as a screen reads.
    private func quadrants(width: Int, height: Int) -> CGImage {
        let ctx = CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        let w = CGFloat(width) / 2, h = CGFloat(height) / 2
        // CGContext is bottom-left: the top row of the image is y = h...height here.
        ctx.setFillColor(CGColor(srgbRed: 1, green: 0, blue: 0, alpha: 1)); ctx.fill(CGRect(x: 0, y: h, width: w, height: h))
        ctx.setFillColor(CGColor(srgbRed: 0, green: 0, blue: 1, alpha: 1)); ctx.fill(CGRect(x: w, y: h, width: w, height: h))
        ctx.setFillColor(CGColor(srgbRed: 0, green: 1, blue: 0, alpha: 1)); ctx.fill(CGRect(x: 0, y: 0, width: w, height: h))
        ctx.setFillColor(CGColor(srgbRed: 1, green: 1, blue: 0, alpha: 1)); ctx.fill(CGRect(x: w, y: 0, width: w, height: h))
        return ctx.makeImage()!
    }

    private func color(_ img: CGImage, _ x: Int, _ y: Int) -> String {
        let c = NSBitmapImageRep(cgImage: img).colorAt(x: x, y: y)!.usingColorSpace(.sRGB)!
        switch (c.redComponent > 0.5, c.greenComponent > 0.5, c.blueComponent > 0.5) {
        case (true, false, false): return "R"
        case (false, false, true): return "B"
        case (false, true, false): return "G"
        case (true, true, false): return "Y"
        default: return "?"
        }
    }
}
