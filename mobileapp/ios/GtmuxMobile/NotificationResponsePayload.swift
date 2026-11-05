import Foundation

/// One representation for a scene's cold response and the warm notification delegate.
/// The request ID lets JS suppress the same response delivered by both paths.
func notificationResponsePayload(
  userInfo: [AnyHashable: Any], requestID: String, actionID: String?, userText: String?
) -> [AnyHashable: Any] {
  var payload = userInfo
  payload["userInteraction"] = true
  payload["gtmuxResponseId"] = requestID
  payload.removeValue(forKey: "actionIdentifier")
  payload.removeValue(forKey: "userText")
  if let actionID = actionID { payload["actionIdentifier"] = actionID }
  if let userText = userText { payload["userText"] = userText }
  return payload
}

/// Responses can arrive before React subscribes. Keep a small in-memory queue
/// until JS drains or acknowledges them; never persist notification payloads.
enum NotificationResponseBuffer {
  private static let lock = NSLock()
  private static var pending: [[AnyHashable: Any]] = []
  private static let limit = 128

  static func enqueue(_ payload: [AnyHashable: Any]) {
    lock.lock(); defer { lock.unlock() }
    let id = payload["gtmuxResponseId"] as? String ?? ""
    let action = payload["actionIdentifier"] as? String ?? ""
    if !id.isEmpty && pending.contains(where: {
      ($0["gtmuxResponseId"] as? String ?? "") == id && ($0["actionIdentifier"] as? String ?? "") == action
    }) { return }
    if pending.count >= limit { pending.removeFirst() }
    pending.append(payload)
  }

  static func acknowledge(id: String, action: String) {
    guard !id.isEmpty else { return }
    lock.lock(); defer { lock.unlock() }
    pending.removeAll {
      ($0["gtmuxResponseId"] as? String ?? "") == id && ($0["actionIdentifier"] as? String ?? "") == action
    }
  }

  static func drain() -> [[AnyHashable: Any]] {
    lock.lock(); defer { lock.unlock() }
    // Delivery across the bridge is asynchronous. Keep each response until JS
    // explicitly acknowledges it, so a failed or suspended bridge can retry.
    return pending
  }
}
