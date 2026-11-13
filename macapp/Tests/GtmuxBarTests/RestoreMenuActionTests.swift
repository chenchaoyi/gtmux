import AppKit
import XCTest
@testable import GtmuxBar
final class RestoreMenuActionTests: XCTestCase {
 func testRestoreDispatchPolicyKeepsActualProgressPopoverVisible() {
  _ = NSApplication.shared
  let window = NSWindow(contentRect: NSRect(x: 100,y: 100,width: 400,height: 300),styleMask: [.titled],backing: .buffered,defer: false)
  window.isReleasedWhenClosed = false
  let anchor = NSView(frame: NSRect(x: 20,y: 180,width: 32,height: 20));window.contentView?.addSubview(anchor);window.orderFront(nil)
  let popover = NSPopover();popover.animates = false
  let content = NSViewController();content.view = NSView(frame: NSRect(x: 0,y: 0,width: 240,height: 200));popover.contentViewController = content
  let presentation = MenuPopoverPresentation(popover: popover)
  defer {popover.close();window.close()}
  presentation.prepareToShow(restoring: false);popover.show(relativeTo: anchor.bounds,of: anchor,preferredEdge: .maxX)
  presentation.restoreStarted()
  // This is the exact closing decision called at the end of AppDelegate.perform.
  if MenuAction.restore.closesMenuAfterAction {popover.performClose(nil)}
  XCTAssertTrue(popover.isShown,"generic action close hid restore progress")
  XCTAssertEqual(popover.behavior,.applicationDefined)
  NotificationCenter.default.post(name: NSApplication.didResignActiveNotification,object: NSApp)
  XCTAssertTrue(popover.isShown)
  popover.performClose(nil);XCTAssertFalse(popover.isShown)
  presentation.prepareToShow(restoring: true);XCTAssertEqual(popover.behavior,.applicationDefined)
  XCTAssertTrue(MenuAction.startHQ.closesMenuAfterAction)
  for action in [MenuAction.quit,.preferences,.pairPhone,.newSession,.browsePanes,.screenshot] {XCTAssertFalse(action.closesMenuAfterAction)}
 }
}
