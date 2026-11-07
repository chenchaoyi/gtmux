import AppKit
import Combine

/// Keep user-scroll observation stable across SwiftUI body updates. AppKit posts
/// both notifications on the main thread; didLive also covers legacy mouse events.
@MainActor final class HQConversationScrollIntent: ObservableObject {
    @Published private(set) var generation: UInt64 = 0
    private var observation: AnyCancellable?
    init(center: NotificationCenter = .default) {
        observation = center.publisher(for: NSScrollView.willStartLiveScrollNotification)
            .merge(with: center.publisher(for: NSScrollView.didLiveScrollNotification))
            .sink { [weak self] notification in
                MainActor.assumeIsolated {
                    guard let self else { return }
                    let before = self.generation
                    self.generation &+= 1
                    HQConversationTrace.record("scroll-intent", ["owner": HQConversationTrace.identity(self),
                        "notification": notification.name.rawValue,
                        "objectType": notification.object.map { String(describing: type(of: $0)) } ?? "nil",
                        "scroller": HQConversationTrace.identity(notification.object as? NSScrollView),
                        "window": HQConversationTrace.identity((notification.object as? NSScrollView)?.window),
                        "before": String(before), "after": String(self.generation)])
                }
            }
    }
}
