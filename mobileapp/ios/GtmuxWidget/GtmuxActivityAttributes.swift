import ActivityKit
import Foundation

// Shared between the app (LiveActivityModule starts/updates it) and the widget
// extension (renders it). The dynamic ContentState is the live agent tally.
@available(iOS 16.1, *)
public struct GtmuxActivityAttributes: ActivityAttributes {
  // One listed session: its name/task, status, and the epoch its state started.
  // The widget renders the relative time LOCALLY from `since` (SwiftUI's
  // auto-updating date Text), so the lock screen stays current without a push.
  public struct Item: Codable, Hashable {
    public var title: String
    public var status: String // waiting | working
    public var since: Int // epoch seconds the state started; 0 if unknown
    public init(title: String, status: String, since: Int) {
      self.title = title
      self.status = status
      self.since = since
    }

    public init(from decoder: Decoder) throws {
      let c = try decoder.container(keyedBy: CodingKeys.self)
      title = try c.decodeIfPresent(String.self, forKey: .title) ?? ""
      status = try c.decodeIfPresent(String.self, forKey: .status) ?? ""
      since = try c.decodeIfPresent(Int.self, forKey: .since) ?? 0
    }
  }

  public struct ContentState: Codable, Hashable {
    public var waiting: Int
    public var working: Int
    public var idle: Int
    // The waiting agent's prompt/task (the detail line), "" if none.
    public var waitingTitle: String
    // The waiting agent's tmux session name (the bold headline — WHERE to look),
    // "" if none. Decoded leniently so an old push without it still renders.
    public var waitingSession: String
    // The top in-flight sessions to LIST concretely (waiting first), + how many
    // active sessions aren't shown. Decoded leniently (old push → empty list).
    public var items: [Item]
    public var more: Int
    public init(waiting: Int, working: Int, idle: Int, waitingTitle: String = "", waitingSession: String = "", items: [Item] = [], more: Int = 0) {
      self.waiting = waiting
      self.working = working
      self.idle = idle
      self.waitingTitle = waitingTitle
      self.waitingSession = waitingSession
      self.items = items
      self.more = more
    }

    public init(from decoder: Decoder) throws {
      let c = try decoder.container(keyedBy: CodingKeys.self)
      waiting = try c.decodeIfPresent(Int.self, forKey: .waiting) ?? 0
      working = try c.decodeIfPresent(Int.self, forKey: .working) ?? 0
      idle = try c.decodeIfPresent(Int.self, forKey: .idle) ?? 0
      waitingTitle = try c.decodeIfPresent(String.self, forKey: .waitingTitle) ?? ""
      waitingSession = try c.decodeIfPresent(String.self, forKey: .waitingSession) ?? ""
      items = try c.decodeIfPresent([Item].self, forKey: .items) ?? []
      more = try c.decodeIfPresent(Int.self, forKey: .more) ?? 0
    }
  }

  public var title: String
  // The paired Mac's name (which server this activity tracks) — static for the
  // activity's life, so it's shown even before the first push and never wiped by a
  // push-to-update (those only replace ContentState). Switching servers ends this
  // activity and starts a fresh one with the new name.
  public var server: String
  public init(title: String = "gtmux", server: String = "") {
    self.title = title
    self.server = server
  }
}
