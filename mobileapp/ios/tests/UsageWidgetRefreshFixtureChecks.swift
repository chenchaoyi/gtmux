// Native coordinator acceptance checks for the standalone synthetic fixture.
import Foundation

@MainActor enum UsageWidgetRefreshFixtureChecks {
  static func run(base: String) async -> [String: Bool] {
    guard let url = URL(string: base), url.host == "127.0.0.1" else { return ["loopback-only": false] }
    let directory = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("UsageChecks")
    let store = UsageSnapshotStore(directory: directory)
    let defaults = UserDefaults(suiteName: "com.gtmux.widget-fixture.refresh")!
    var token = "old-fixture", locked = false, removed = false
    let coordinator = UsageWidgetRefresh(defaults: defaults, store: store, load: { selected in
      guard !locked, !removed, let selectedURL = URL(string: selected) else { return nil }
      return WidgetOwnerConnection(base: selectedURL, token: token, name: selectedURL.lastPathComponent)
    }, stillSelected: { _ in removed ? false : (locked ? nil : true) }, configurations: { $0(true) })
    defer { coordinator.configure(nil); defaults.removePersistentDomain(forName: "com.gtmux.widget-fixture.refresh") }
    func pause(_ ms: UInt64) async { try? await Task.sleep(nanoseconds: ms * 1_000_000) }
    func wait(_ check: () -> Bool) async -> Bool {
      for _ in 0..<120 { if check() { return true }; await pause(25) }
      return false
    }
    var results: [String: Bool] = [:]
    coordinator.configure(base + "/slow"); await pause(80); coordinator.refresh(); await pause(150)
    coordinator.configure(base + "/current"); await pause(50); coordinator.refresh()
    results["new-server-published"] = await wait { store.read()?.server == "current" }
    await pause(1100)
    results["old-server-cannot-overwrite"] = store.read()?.server == "current" && store.read()?.accounts.first?.windows.first?.percent == 24
    let beforeLock = store.read()
    locked = true; coordinator.refresh(); await pause(50)
    results["locked-storage-preserves-dated-reading"] = beforeLock != nil && store.read() == beforeLock
    removed = true; coordinator.refresh(); await pause(50)
    results["removed-pairing-clears"] = store.read() == nil
    locked = false; removed = false
    coordinator.configure(base + "/rotate"); await pause(80); coordinator.refresh(); await pause(150)
    token = "new-fixture"; coordinator.configure(base + "/rotate"); await pause(50); coordinator.refresh()
    results["rotated-token-published"] = await wait { store.read()?.accounts.first?.windows.first?.percent == 25 }
    await pause(1100)
    results["old-token-cannot-overwrite"] = store.read()?.accounts.first?.windows.first?.percent == 25
    let beforeRefusal = store.read()
    coordinator.configure(base + "/refused"); await pause(60); coordinator.refresh()
    if let beforeRefusal { try? store.write(beforeRefusal) }
    results["authorization-refusal-clears"] = await wait { store.read() == nil }
    coordinator.configure(base + "/slow"); await pause(80); coordinator.refresh(); await pause(150)
    coordinator.configure(nil); await pause(1100)
    results["disconnect-cancels-and-stays-empty"] = store.read() == nil
    return results
  }
}
