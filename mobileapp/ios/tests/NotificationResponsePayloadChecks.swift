import Foundation

@main
struct NotificationResponsePayloadChecks {
  static func main() {
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
    precondition(NotificationResponseBuffer.drain().isEmpty)
    for n in 0..<140 {
      NotificationResponseBuffer.enqueue(notificationResponsePayload(userInfo: [:], requestID: "bounded-\(n)", actionID: nil, userText: nil))
    }
    let bounded = NotificationResponseBuffer.drain()
    precondition(bounded.count == 128 && bounded.first?["gtmuxResponseId"] as? String == "bounded-12")
    print("notification response payload and queue checks passed")
  }
}
