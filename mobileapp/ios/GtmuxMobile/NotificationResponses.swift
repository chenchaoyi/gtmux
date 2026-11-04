import Foundation
import React

@objc(GtmuxNotificationResponses)
class GtmuxNotificationResponses: NSObject {
  @objc static func requiresMainQueueSetup() -> Bool { false }

  @objc(drain:rejecter:)
  func drain(_ resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
    resolve(NotificationResponseBuffer.drain())
  }

  @objc(acknowledge:action:)
  func acknowledge(_ identifier: String, action: String) {
    NotificationResponseBuffer.acknowledge(id: identifier, action: action)
  }
}
