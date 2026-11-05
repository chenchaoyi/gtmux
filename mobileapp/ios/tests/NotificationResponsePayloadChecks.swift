import Foundation

final class SignalCounter: @unchecked Sendable {
  private let lock = NSLock()
  private var count = 0
  func increment() { lock.lock(); count += 1; lock.unlock() }
  func value() -> Int { lock.lock(); defer { lock.unlock() }; return count }
}

@main
struct NotificationResponsePayloadChecks {
  static func main() {
    let activity = NSUserActivity(activityType: "NSUserActivityTypeBrowsingWeb")
    activity.webpageURL = URL(string: "https://example.test/path")
    let typeKey = "UIApplicationLaunchOptionsUserActivityTypeKey"
    let activityPayload = sceneUserActivityPayload(activity, typeKey: typeKey)
    precondition(activityPayload[typeKey] as? String == "NSUserActivityTypeBrowsingWeb")
    precondition((activityPayload["UIApplicationLaunchOptionsUserActivityKey"] as? NSUserActivity)?.webpageURL == activity.webpageURL)
    let sceneURL = URL(string: "gtmux://usage")!
    retainHeadlessSceneLinks(urls: [sceneURL], activities: [activity])
    let links = SceneLinkBuffer.drain()
    precondition(links.count == 2 && links[0]["url"] == sceneURL.absoluteString)
    precondition(links[1]["url"] == activity.webpageURL?.absoluteString)
    precondition(SceneLinkBuffer.drain() == links) // retained until JS acknowledges
    SceneLinkBuffer.acknowledge(links[0]["id"]!)
    precondition(SceneLinkBuffer.drain().count == 1)
    SceneLinkBuffer.acknowledge(links[1]["id"]!)
    precondition(SceneLinkBuffer.drain().isEmpty)
    let counter = SignalCounter()
    let observer = NotificationCenter.default.addObserver(forName: sceneLinksAvailableNotification, object: nil, queue: nil) { _ in counter.increment() }
    precondition(!retainWarmUsageSceneLink(URL(string: "gtmux://usage?token=private")!))
    precondition(!retainWarmUsageSceneLink(URL(string: "https://example.test")!))
    precondition(retainWarmUsageSceneLink(sceneURL))
    precondition(retainWarmUsageSceneLink(sceneURL)) // distinct user intents
    let warmLinks = SceneLinkBuffer.drain()
    precondition(warmLinks.count == 2 && warmLinks[0]["id"] != warmLinks[1]["id"] && counter.value() == 2)
    for link in warmLinks { SceneLinkBuffer.acknowledge(link["id"]!) }
    NotificationCenter.default.removeObserver(observer)
    for n in 0..<40 { SceneLinkBuffer.enqueue(URL(string: "gtmux://usage/\(n)")!) }
    let boundedLinks = SceneLinkBuffer.drain()
    precondition(boundedLinks.count == 32 && boundedLinks.first?["url"] == "gtmux://usage/8")
    for link in boundedLinks { SceneLinkBuffer.acknowledge(link["id"]!) }
    precondition(SceneLinkBuffer.drain().isEmpty)
    let input: [AnyHashable: Any] = ["pane": "%7", "server": "fixture", "aps": ["alert": "test"]]
    let action = notificationResponsePayload(userInfo: input, requestID: "request-1", actionID: "2", userText: "hello")
    precondition(action["pane"] as? String == "%7")
    precondition(action["server"] as? String == "fixture")
    precondition(action["gtmuxResponseId"] as? String == "request-1")
    precondition(action["actionIdentifier"] as? String == "2")
    precondition(action["userText"] as? String == "hello")
    precondition(action["userInteraction"] as? Bool == true)
    let body = notificationResponsePayload(userInfo: input, requestID: "request-2", actionID: nil, userText: nil)
    precondition(body["actionIdentifier"] == nil && body["userText"] == nil)
    precondition(body["gtmuxResponseId"] as? String == "request-2")
    _ = NotificationResponseBuffer.drain()
    NotificationResponseBuffer.enqueue(action)
    NotificationResponseBuffer.enqueue(action)
    NotificationResponseBuffer.enqueue(body)
    NotificationResponseBuffer.acknowledge(id: "request-2", action: "")
    let pending = NotificationResponseBuffer.drain()
    precondition(pending.count == 1 && pending[0]["gtmuxResponseId"] as? String == "request-1")
    precondition(NotificationResponseBuffer.drain().count == 1) // retry until JS acknowledges
    NotificationResponseBuffer.acknowledge(id: "request-1", action: "2")
    precondition(NotificationResponseBuffer.drain().isEmpty)
    for n in 0..<140 {
      NotificationResponseBuffer.enqueue(notificationResponsePayload(userInfo: [:], requestID: "bounded-\(n)", actionID: nil, userText: nil))
    }
    let bounded = NotificationResponseBuffer.drain()
    precondition(bounded.count == 128 && bounded.first?["gtmuxResponseId"] as? String == "bounded-12")
    print("notification response payload and queue checks passed")
  }
}
