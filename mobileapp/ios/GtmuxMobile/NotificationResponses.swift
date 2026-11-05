import Foundation
import React

@objc(GtmuxNotificationResponses)
class GtmuxNotificationResponses: NSObject {
  @objc static func requiresMainQueueSetup() -> Bool { false }

  @objc(drain:rejecter:)
  func drain(_ resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
    let pending = NotificationResponseBuffer.drain()
    traceNotification("native-drain-\(pending.count)")
    resolve(pending)
  }

  @objc(acknowledge:action:)
  func acknowledge(_ identifier: String, action: String) {
    NotificationResponseBuffer.acknowledge(id: identifier, action: action)
  }
}
