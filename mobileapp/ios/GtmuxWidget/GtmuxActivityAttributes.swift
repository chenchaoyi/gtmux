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
    // Name of the agent that needs you (the most relevant waiting one), "" if none
    // — shown as the headline so you know WHO to attend to at a glance.
    public var waitingTitle: String
    public init(waiting: Int, working: Int, idle: Int, waitingTitle: String = "") {
      self.waiting = waiting
      self.working = working
      self.idle = idle
      self.waitingTitle = waitingTitle
    }
  }

  public var title: String
  public init(title: String = "gtmux") { self.title = title }
}
