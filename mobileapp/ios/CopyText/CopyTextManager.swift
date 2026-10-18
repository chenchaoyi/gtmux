import React

@objc(CopyTextViewManager)
final class CopyTextViewManager: RCTViewManager {
  override func view() -> UIView! {
    if #available(iOS 16.0, *) { return CopyTextView() }
    return UIView() // JS retains RN Text Copy on older iOS.
  }
  override static func requiresMainQueueSetup() -> Bool { true }
}
