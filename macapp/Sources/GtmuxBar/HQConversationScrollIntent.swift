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
            .sink { [weak self] _ in
                MainActor.assumeIsolated { self?.generation &+= 1 }
            }
    }
}
