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
    print("notification response payload checks passed")
  }
}
