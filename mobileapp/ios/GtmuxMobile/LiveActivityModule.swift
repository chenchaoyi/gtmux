import ActivityKit
import Foundation
import React

// RN bridge to start / update / end the gtmux Live Activity from JS. Local
// (foreground/background-runtime) updates; remote push-to-update can be layered
// on later by requesting a pushType and forwarding the token to the relay.
@objc(LiveActivityModule)
class LiveActivityModule: NSObject {

  @objc static func requiresMainQueueSetup() -> Bool { false }

  // areEnabled resolves whether the user has Live Activities turned on.
  @objc(areEnabled:rejecter:)
  func areEnabled(_ resolve: @escaping RCTPromiseResolveBlock, rejecter _: @escaping RCTPromiseRejectBlock) {
    if #available(iOS 16.1, *) {
      resolve(ActivityAuthorizationInfo().areActivitiesEnabled)
    } else {
      resolve(false)
    }
  }

  @objc(start:working:idle:title:resolver:rejecter:)
  func start(_ waiting: NSNumber, working: NSNumber, idle: NSNumber, title: NSString,
             resolver resolve: @escaping RCTPromiseResolveBlock,
             rejecter reject: @escaping RCTPromiseRejectBlock) {
    if #available(iOS 16.1, *) {
      guard ActivityAuthorizationInfo().areActivitiesEnabled else {
        reject("disabled", "Live Activities are disabled", nil)
        return
      }
      // Reuse a running activity (just update it) so we never stack duplicates.
      if let existing = Activity<GtmuxActivityAttributes>.activities.first {
        Task {
          await existing.update(using: state(waiting, working, idle, title))
          resolve(existing.id)
        }
        return
      }
      do {
        let act = try Activity.request(
          attributes: GtmuxActivityAttributes(),
          contentState: state(waiting, working, idle, title),
          pushType: nil)
        resolve(act.id)
      } catch {
        reject("start_failed", error.localizedDescription, error)
      }
    } else {
      reject("unsupported", "iOS 16.1+ required", nil)
    }
  }

  @objc(update:working:idle:title:)
  func update(_ waiting: NSNumber, working: NSNumber, idle: NSNumber, title: NSString) {
    if #available(iOS 16.1, *) {
      let s = state(waiting, working, idle, title)
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
  private func state(_ w: NSNumber, _ wk: NSNumber, _ i: NSNumber, _ title: NSString) -> GtmuxActivityAttributes.ContentState {
    GtmuxActivityAttributes.ContentState(waiting: w.intValue, working: wk.intValue, idle: i.intValue, waitingTitle: title as String)
  }
}
