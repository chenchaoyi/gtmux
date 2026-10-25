import React

@objc(ConversationTextViewManager)
final class ConversationTextViewManager: RCTViewManager {
  override func view() -> UIView! { ConversationTextView() }
  override static func requiresMainQueueSetup() -> Bool { true }
}
