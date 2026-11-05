import Foundation
import React
import UIKit

@objc(GtmuxNotificationResponses)
class GtmuxNotificationResponses: RCTEventEmitter {
  private var sceneObserver: NSObjectProtocol?
  override static func requiresMainQueueSetup() -> Bool { false }
  override func supportedEvents() -> [String]! { ["gtmuxSceneLinksAvailable"] }

  override func startObserving() {
    sceneObserver = NotificationCenter.default.addObserver(forName: sceneLinksAvailableNotification, object: nil, queue: nil) { [weak self] _ in
      self?.sendEvent(withName: "gtmuxSceneLinksAvailable", body: nil)
    }
  }

  override func stopObserving() {
    if let observer = sceneObserver { NotificationCenter.default.removeObserver(observer) }
    sceneObserver = nil
  }

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
    DispatchQueue.main.async {
      NotificationResponseCompletions.finish(id: identifier, action: action)
    }
  }

  @objc(pendingSceneLinks:rejecter:)
  func pendingSceneLinks(_ resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
    resolve(SceneLinkBuffer.drain())
  }

  @objc(acknowledgeSceneLink:)
  func acknowledgeSceneLink(_ identifier: String) {
    SceneLinkBuffer.acknowledge(identifier)
  }

  @objc(currentSceneMetrics:rejecter:)
  func currentSceneMetrics(_ resolve: @escaping RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
    DispatchQueue.main.async {
      guard let app = UIApplication.shared.delegate as? AppDelegate,
            let window = app.window, window.isKeyWindow, window.windowScene != nil,
            window.rootViewController?.view.window === window,
            window.bounds.width > 0, window.bounds.height > 0 else {
        resolve(nil)
        return
      }
      let bounds = window.bounds
      let insets = window.safeAreaInsets
      resolve([
        "frame": ["x": bounds.origin.x, "y": bounds.origin.y, "width": bounds.width, "height": bounds.height],
        "insets": ["top": insets.top, "right": insets.right, "bottom": insets.bottom, "left": insets.left],
      ])
    }
  }
}
