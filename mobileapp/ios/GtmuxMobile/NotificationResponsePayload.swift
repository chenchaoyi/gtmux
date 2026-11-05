import Foundation

// RN's cold-link bridge reads both the activity and its type from this dictionary.
func sceneUserActivityPayload(_ activity: NSUserActivity, typeKey: String) -> [String: Any] {
  [typeKey: activity.activityType, "UIApplicationLaunchOptionsUserActivityKey": activity]
}

// A scene can arrive after a background response has already started React.
// Its launch options cannot be retroactively applied to that existing bridge.
enum SceneLinkBuffer {
  private static let lock = NSLock()
  private static var pending: [[String: String]] = []
  private static let limit = 32

  static func enqueue(_ url: URL) {
    lock.lock(); defer { lock.unlock() }
    if pending.count >= limit { pending.removeFirst() }
    pending.append(["id": UUID().uuidString, "url": url.absoluteString])
  }

  static func drain() -> [[String: String]] {
    lock.lock(); defer { lock.unlock() }
    return pending
  }

  static func acknowledge(_ id: String) {
    lock.lock(); defer { lock.unlock() }
    pending.removeAll { $0["id"] == id }
  }
}

// Called only when an existing headless bridge is attached to a new scene.
func retainHeadlessSceneLinks(urls: [URL], activities: [NSUserActivity]) {
  for url in urls { SceneLinkBuffer.enqueue(url) }
  for activity in activities where activity.activityType == NSUserActivityTypeBrowsingWeb {
    if let url = activity.webpageURL { SceneLinkBuffer.enqueue(url) }
  }
}

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
