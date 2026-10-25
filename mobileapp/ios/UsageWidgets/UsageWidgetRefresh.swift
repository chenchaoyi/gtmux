import BackgroundTasks
import UIKit
import WidgetKit

/// Refresh stays in the parent app and uses its existing Keychain protection.
/// A locked/unavailable credential leaves a dated reading, never a weaker copy.
final class UsageWidgetRefresh {
  static let shared = UsageWidgetRefresh()
  static let taskID = "com.gtmux.app.usage-refresh"
  static let kind = "com.gtmux.app.usage"
  private let defaults: UserDefaults
  private let store: UsageSnapshotStore
  private let load: (String) -> WidgetOwnerConnection?
  private let stillSelected: (String) -> Bool?
  private let configurations: (@escaping (Bool) -> Void) -> Void
  private var generation = UUID()
  private var request: WidgetUsageRequest?
  private var timer: Timer?
  private var observers: [NSObjectProtocol] = []
  private var registered = false
  private var configured = false
  private var lastReload: Date?
  private var lastContents: UsageWidgetSnapshot?
  private var background: BGAppRefreshTask?
  private let preference = "usageWidget.selectedServer"

  init(defaults: UserDefaults = .standard, store: UsageSnapshotStore = .shared,
       load: @escaping (String) -> WidgetOwnerConnection? = { WidgetOwnerConnection.load(selected: $0) },
       stillSelected: @escaping (String) -> Bool? = { WidgetOwnerConnection.stillSelectedOwner($0) },
       configurations: @escaping (@escaping (Bool) -> Void) -> Void = { done in
         WidgetCenter.shared.getCurrentConfigurations { result in
           done((try? result.get().contains { $0.kind == UsageWidgetRefresh.kind }) ?? false)
         }
       }) {
    self.defaults = defaults; self.store = store; self.load = load; self.stillSelected = stillSelected; self.configurations = configurations
  }

  func register() {
    guard !registered else { return }; registered = true
    BGTaskScheduler.shared.register(forTaskWithIdentifier: Self.taskID, using: .main) { [weak self] task in
      guard let self, let task = task as? BGAppRefreshTask else { task.setTaskCompleted(success: false); return }
      self.background = task
      task.expirationHandler = { [weak self, weak task] in
        DispatchQueue.main.async {
          guard let self, self.background === task else { return }
          self.generation = UUID(); self.request?.cancel(); self.request = nil; self.finishBackground(false)
        }
      }
      self.checkConfiguration { [weak self] in
        guard let self, self.background === task else { return }
        self.schedule(); self.refresh()
      }
    }
    observers.append(NotificationCenter.default.addObserver(forName: UIApplication.didBecomeActiveNotification, object: nil, queue: .main) { [weak self] _ in
      self?.checkConfiguration { [weak self] in self?.startForeground() }
    })
    observers.append(NotificationCenter.default.addObserver(forName: UIApplication.willResignActiveNotification, object: nil, queue: .main) { [weak self] _ in
      guard let self else { return }
      self.generation = UUID(); self.request?.cancel(); self.request = nil
      self.timer?.invalidate(); self.timer = nil; self.schedule()
    })
  }

  /// Called after the app has resolved its selected pairing. The URL is private
  /// parent state; no token or address enters the shared widget container.
  func configure(_ selected: String?) {
    generation = UUID(); request?.cancel(); request = nil
    timer?.invalidate(); timer = nil; finishBackground(false)
    let next = selected.flatMap { WidgetOwnerConnection.validatedURL($0) == nil ? nil : $0 }
    // A token can rotate at the same address. Configure is a pairing-context
    // transition, not a render callback; verify anew before exposing a reading.
    store.clear(); lastContents = nil; reload(force: true)
    if let next { defaults.set(next, forKey: preference) }
    else {
      defaults.removeObject(forKey: preference); store.clear(); lastContents = nil
      BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: Self.taskID); reload(force: true)
    }
    checkConfiguration { [weak self] in
      guard let self else { return }
      if UIApplication.shared.applicationState == .active { self.startForeground() }
      else { self.schedule() }
    }
  }

  private func checkConfiguration(_ completion: @escaping () -> Void) {
    let current = generation
    configurations { [weak self] configured in
      DispatchQueue.main.async {
        guard let self, current == self.generation else { return }
        self.configured = configured
        completion()
      }
    }
  }

  private func startForeground() {
    timer?.invalidate(); timer = nil
    guard configured, defaults.string(forKey: preference) != nil else { return }
    refresh()
    timer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in self?.refresh() }
    schedule()
  }

  private func schedule() {
    guard registered else { return }
    guard configured, defaults.string(forKey: preference) != nil else {
      BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: Self.taskID); return
    }
    let task = BGAppRefreshTaskRequest(identifier: Self.taskID)
    task.earliestBeginDate = Date(timeIntervalSinceNow: 15 * 60)
    // Disabled background refresh and simulator limitations are not permission to
    // weaken Keychain storage or spin a substitute timer in the background.
    try? BGTaskScheduler.shared.submit(task)
  }

  func refresh() {
    guard configured, let selected = defaults.string(forKey: preference) else {
      finishBackground(false); return
    }
    guard request == nil else { return }
    guard let connection = load(selected) else {
      if stillSelected(selected) == false { store.clear(); lastContents = nil; reload(force: true) }
      finishBackground(false); return
    }
    let current = generation
    let next = WidgetUsageRequest(connection: connection) { [weak self] result in
      guard let self, self.generation == current else { return }
      self.request = nil
      // The Keychain may have changed while the request was in flight, even if
      // the JS pairing screen has not mounted yet (cold background execution).
      guard self.defaults.string(forKey: self.preference) == selected,
            self.load(selected) == connection else {
        if self.stillSelected(selected) == false { self.store.clear(); self.lastContents = nil; self.reload(force: true) }
        self.finishBackground(false); return
      }
      switch result {
      case .success(let data):
        do {
          let snapshot = try UsageWidgetSnapshot.project(report: data, server: connection.name, at: Date())
          try self.store.write(snapshot)
          var comparable = snapshot; comparable.capturedAt = 0
          let changed = self.lastContents != comparable
          self.lastContents = comparable; self.reload(force: changed)
          self.finishBackground(true)
        } catch { self.finishBackground(false) }
      case .failure(.refused):
        self.store.clear(); self.lastContents = nil; self.reload(force: true); self.finishBackground(false)
      case .failure:
        self.finishBackground(false)
      }
    }
    request = next; next.start()
  }

  private func reload(force: Bool) {
    let now = Date()
    if force || lastReload == nil || now.timeIntervalSince(lastReload!) >= 5 * 60 {
      lastReload = now; WidgetCenter.shared.reloadTimelines(ofKind: Self.kind)
    }
  }

  private func finishBackground(_ success: Bool) {
    let task = background; background = nil; task?.setTaskCompleted(success: success)
  }
}
