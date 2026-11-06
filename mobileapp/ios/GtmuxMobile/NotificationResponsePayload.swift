import Foundation

// RN's cold-link bridge reads both the activity and its type from this dictionary.
func sceneUserActivityPayload(_ activity: NSUserActivity, typeKey: String) -> [String: Any] {
  [typeKey: activity.activityType, "UIApplicationLaunchOptionsUserActivityKey": activity]
}

// A scene can arrive after a background response has already started React.
// Its launch options cannot be retroactively applied to that existing bridge.
let sceneLinksAvailableNotification = Notification.Name("gtmuxSceneLinksAvailable")

enum SceneLinkBuffer {
  private static let lock = NSLock()
  private static var pending: [[String: String]] = []
  private static let limit = 32

  static func enqueue(_ url: URL) {
    lock.lock()
    if pending.count >= limit { pending.removeFirst() }
    pending.append(["id": UUID().uuidString, "url": url.absoluteString])
    lock.unlock()
    NotificationCenter.default.post(name: sceneLinksAvailableNotification, object: nil)
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

// Usage links have one consumer and need acknowledgement if the RN listener is
// not yet observing. Other URLs stay on React Native's normal Linking route.
func retainWarmUsageSceneLink(_ url: URL) -> Bool {
  let value = url.absoluteString.lowercased()
  guard value == "gtmux://usage" || value == "gtmux://usage/" else { return false }
  SceneLinkBuffer.enqueue(url)
  return true
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

// UIKit's response callback must stay alive while an asynchronous quick reply
// is being sent. A unique token makes timeout and acknowledgement race safely.
enum NotificationResponseCompletions {
  private struct Entry {
    let id: String
    let action: String
    let completion: () -> Void
  }
  private static let lock = NSLock()
  private static var entries: [String: Entry] = [:]

  static func retain(id: String, action: String, completion: @escaping () -> Void) -> String {
    let token = UUID().uuidString
    lock.lock()
    entries[token] = Entry(id: id, action: action, completion: completion)
    lock.unlock()
    return token
  }

  static func finish(id: String, action: String) {
    lock.lock()
    let tokens = entries.filter { $0.value.id == id && $0.value.action == action }.map(\.key)
    let callbacks = tokens.compactMap { entries.removeValue(forKey: $0)?.completion }
    lock.unlock()
    callbacks.forEach { $0() }
  }

  static func expire(token: String) {
    lock.lock()
    let callback = entries.removeValue(forKey: token)?.completion
    lock.unlock()
    callback?()
  }
}
