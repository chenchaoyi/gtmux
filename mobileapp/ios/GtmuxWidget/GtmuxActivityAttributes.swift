import ActivityKit
import Foundation

// Shared between the app (LiveActivityModule starts/updates it) and the widget
// extension (renders it). The dynamic ContentState is the live agent tally.
@available(iOS 16.1, *)
public struct GtmuxActivityAttributes: ActivityAttributes {
  // One listed session: its name/task, status, and a compact relative time ("2m").
  public struct Item: Codable, Hashable {
    public var title: String
    public var status: String // waiting | working
    public var time: String
    public init(title: String, status: String, time: String) {
      self.title = title
      self.status = status
      self.time = time
    }

    public init(from decoder: Decoder) throws {
      let c = try decoder.container(keyedBy: CodingKeys.self)
      title = try c.decodeIfPresent(String.self, forKey: .title) ?? ""
      status = try c.decodeIfPresent(String.self, forKey: .status) ?? ""
      time = try c.decodeIfPresent(String.self, forKey: .time) ?? ""
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
  public init(title: String = "gtmux") { self.title = title }
}
