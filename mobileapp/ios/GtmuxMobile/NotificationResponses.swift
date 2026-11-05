import Foundation
import React

@objc(GtmuxNotificationResponses)
class GtmuxNotificationResponses: NSObject {
  @objc static func requiresMainQueueSetup() -> Bool { false }

  @objc(drain:rejecter:)
  func drain(_ resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
    let pending = NotificationResponseBuffer.drain()
    traceNotification("native-drain-\(pending.count)")
    traceNotification("native-drain-json-\(JSONSerialization.isValidJSONObject(pending) ? "valid" : "invalid")")
    resolve(pending)
    traceNotification("native-drain-resolve-called")
  }

  @objc(acknowledge:action:)
  func acknowledge(_ identifier: String, action: String) {
    NotificationResponseBuffer.acknowledge(id: identifier, action: action)
  }

  @objc(pendingSceneLinks:rejecter:)
  func pendingSceneLinks(_ resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
    resolve(SceneLinkBuffer.drain())
  }

  @objc(acknowledgeSceneLink:)
  func acknowledgeSceneLink(_ identifier: String) {
    SceneLinkBuffer.acknowledge(identifier)
  }
}
