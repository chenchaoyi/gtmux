import Foundation
import React

@objc(UsageWidgetsModule)
final class UsageWidgetsModule: NSObject {
  @objc static func requiresMainQueueSetup() -> Bool { true }

  @objc(configure:resolver:rejecter:)
  func configure(_ selected: String?, resolver resolve: @escaping RCTPromiseResolveBlock,
                 rejecter reject: @escaping RCTPromiseRejectBlock) {
    DispatchQueue.main.async {
      UsageWidgetRefresh.shared.configure(selected)
      resolve(nil)
    }
  }
}
