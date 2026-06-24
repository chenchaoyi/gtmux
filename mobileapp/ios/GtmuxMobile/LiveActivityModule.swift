import ActivityKit
import Foundation
import React

// RN bridge to start / update / end the gtmux Live Activity. Requests a push token
// so the relay can push-to-update the lock screen even when the app is killed; the
// token is emitted to JS via `onActivityPushToken` (and cached for a late getter).
@objc(LiveActivityModule)
class LiveActivityModule: RCTEventEmitter {

  private var lastToken: String?

  override static func requiresMainQueueSetup() -> Bool { false }
  override func supportedEvents() -> [String]! { ["onActivityPushToken"] }

  @objc(areEnabled:rejecter:)
  func areEnabled(_ resolve: @escaping RCTPromiseResolveBlock, rejecter _: @escaping RCTPromiseRejectBlock) {
    if #available(iOS 16.1, *) {
      resolve(ActivityAuthorizationInfo().areActivitiesEnabled)
    } else {
      resolve(false)
    }
  }

  // getPushToken returns the most recent activity push token (or "") — a fallback
  // for a JS listener that attaches after the token was first emitted.
  @objc(getPushToken:rejecter:)
  func getPushToken(_ resolve: @escaping RCTPromiseResolveBlock, rejecter _: @escaping RCTPromiseRejectBlock) {
    resolve(lastToken ?? "")
  }

  @objc(start:working:idle:title:session:resolver:rejecter:)
  func start(_ waiting: NSNumber, working: NSNumber, idle: NSNumber, title: NSString, session: NSString,
             resolver resolve: @escaping RCTPromiseResolveBlock,
             rejecter reject: @escaping RCTPromiseRejectBlock) {
    if #available(iOS 16.1, *) {
      guard ActivityAuthorizationInfo().areActivitiesEnabled else {
        reject("disabled", "Live Activities are disabled", nil)
        return
      }
      if let existing = Activity<GtmuxActivityAttributes>.activities.first {
        observeToken(existing)
        Task {
          await existing.update(using: state(waiting, working, idle, title, session))
          resolve(existing.id)
        }
        return
      }
      do {
        let act = try Activity.request(
          attributes: GtmuxActivityAttributes(),
          contentState: state(waiting, working, idle, title, session),
          pushType: .token)
        observeToken(act)
        resolve(act.id)
      } catch {
        reject("start_failed", error.localizedDescription, error)
      }
    } else {
      reject("unsupported", "iOS 16.1+ required", nil)
    }
  }

  @objc(update:working:idle:title:session:)
  func update(_ waiting: NSNumber, working: NSNumber, idle: NSNumber, title: NSString, session: NSString) {
    if #available(iOS 16.1, *) {
      let s = state(waiting, working, idle, title, session)
      Task {
        for act in Activity<GtmuxActivityAttributes>.activities {
          await act.update(using: s)
        }
      }
    }
  }

  @objc func end() {
    if #available(iOS 16.1, *) {
      Task {
        for act in Activity<GtmuxActivityAttributes>.activities {
          await act.end(dismissalPolicy: .immediate)
        }
      }
    }
  }

  @available(iOS 16.1, *)
  private func observeToken(_ act: Activity<GtmuxActivityAttributes>) {
    Task { [weak self] in
      for await data in act.pushTokenUpdates {
        let hex = data.map { String(format: "%02x", $0) }.joined()
        self?.lastToken = hex
        self?.sendEvent(withName: "onActivityPushToken", body: ["token": hex])
      }
    }
  }

  @available(iOS 16.1, *)
  private func state(_ w: NSNumber, _ wk: NSNumber, _ i: NSNumber, _ title: NSString, _ session: NSString) -> GtmuxActivityAttributes.ContentState {
    GtmuxActivityAttributes.ContentState(
      waiting: w.intValue, working: wk.intValue, idle: i.intValue,
      waitingTitle: title as String, waitingSession: session as String)
  }
}
