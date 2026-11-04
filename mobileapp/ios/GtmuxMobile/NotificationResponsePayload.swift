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
