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

  @objc(start:working:idle:title:session:items:server:resolver:rejecter:)
  func start(_ waiting: NSNumber, working: NSNumber, idle: NSNumber, title: NSString, session: NSString, items: NSString, server: NSString,
             resolver resolve: @escaping RCTPromiseResolveBlock,
             rejecter reject: @escaping RCTPromiseRejectBlock) {
    if #available(iOS 16.1, *) {
      guard ActivityAuthorizationInfo().areActivitiesEnabled else {
        reject("disabled", "Live Activities are disabled", nil)
        return
      }
      let srv = server as String
      // Reuse the running activity ONLY if it tracks the SAME server. After a
      // server switch (end + start can race), a leftover activity for the OLD Mac
      // must be replaced — its static server name can't be updated in place.
      if let existing = Activity<GtmuxActivityAttributes>.activities.first {
        if existing.attributes.server == srv {
          observeToken(existing)
          Task {
            await existing.update(using: state(waiting, working, idle, title, session, items))
            resolve(existing.id)
          }
          return
        }
        Task { for act in Activity<GtmuxActivityAttributes>.activities { await act.end(dismissalPolicy: .immediate) } }
      }
      do {
        let act = try Activity.request(
          attributes: GtmuxActivityAttributes(server: srv),
          contentState: state(waiting, working, idle, title, session, items),
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

  @objc(update:working:idle:title:session:items:)
  func update(_ waiting: NSNumber, working: NSNumber, idle: NSNumber, title: NSString, session: NSString, items: NSString) {
    if #available(iOS 16.1, *) {
      let s = state(waiting, working, idle, title, session, items)
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
  private func state(_ w: NSNumber, _ wk: NSNumber, _ i: NSNumber, _ title: NSString, _ session: NSString, _ itemsJson: NSString) -> GtmuxActivityAttributes.ContentState {
    var items: [GtmuxActivityAttributes.Item] = []
    var more = 0
    if let data = (itemsJson as String).data(using: .utf8),
       let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
      if let arr = obj["items"] as? [[String: Any]] {
        items = arr.map {
          GtmuxActivityAttributes.Item(
            title: $0["title"] as? String ?? "",
            status: $0["status"] as? String ?? "",
            time: $0["time"] as? String ?? "")
        }
      }
      more = obj["more"] as? Int ?? 0
    }
    return GtmuxActivityAttributes.ContentState(
      waiting: w.intValue, working: wk.intValue, idle: i.intValue,
      waitingTitle: title as String, waitingSession: session as String,
      items: items, more: more)
  }
}
