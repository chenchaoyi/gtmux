import ActivityKit
import Foundation

// Shared between the app (LiveActivityModule starts/updates it) and the widget
// extension (renders it). The dynamic ContentState is the live agent tally.
@available(iOS 16.1, *)
public struct GtmuxActivityAttributes: ActivityAttributes {
  public struct ContentState: Codable, Hashable {
    public var waiting: Int
    public var working: Int
    public var idle: Int
    // The waiting agent's prompt/task (the detail line), "" if none.
    public var waitingTitle: String
    // The waiting agent's tmux session name (the bold headline — WHERE to look),
    // "" if none. Decoded leniently so an old push without it still renders.
    public var waitingSession: String
    public init(waiting: Int, working: Int, idle: Int, waitingTitle: String = "", waitingSession: String = "") {
      self.waiting = waiting
      self.working = working
      self.idle = idle
      self.waitingTitle = waitingTitle
      self.waitingSession = waitingSession
    }

    public init(from decoder: Decoder) throws {
      let c = try decoder.container(keyedBy: CodingKeys.self)
      waiting = try c.decodeIfPresent(Int.self, forKey: .waiting) ?? 0
      working = try c.decodeIfPresent(Int.self, forKey: .working) ?? 0
      idle = try c.decodeIfPresent(Int.self, forKey: .idle) ?? 0
      waitingTitle = try c.decodeIfPresent(String.self, forKey: .waitingTitle) ?? ""
      waitingSession = try c.decodeIfPresent(String.self, forKey: .waitingSession) ?? ""
    }
  }

  public var title: String
  public init(title: String = "gtmux") { self.title = title }
}
