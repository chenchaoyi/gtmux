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
    public init(waiting: Int, working: Int, idle: Int) {
      self.waiting = waiting
      self.working = working
      self.idle = idle
    }
  }

  public var title: String
  public init(title: String = "gtmux") { self.title = title }
}
