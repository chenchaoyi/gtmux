import Foundation
import React

// DebugSettings — a launch-time debug channel for UI automation, gated entirely
// by `GTMUX_DEBUG_*` launch environment variables (Appium passes them via
// `mobile: launchApp` environment / processArguments). A normal launch sets
// none, so production behavior is unchanged.
//
// JS reads the flags as constants (NativeModules.DebugSettings.flags) and writes
// structured events with `record(...)`, which append to a JSONL file in the
// app's Documents dir — an e2e test reads it post-run via:
//   xcrun simctl get_app_container booted com.gtmux.app data → Documents/gtmux-debug.jsonl
@objc(DebugSettings)
class DebugSettings: NSObject {

  @objc static func requiresMainQueueSetup() -> Bool { false }

  // Export debug flags to JS as `flags`. Two sources, merged (env wins): a JSON
  // file the e2e harness writes before launch (deterministic — XCUITest's
  // launchEnvironment is cached per-session and unreliable on relaunch), then
  // GTMUX_DEBUG_* launch env. Keys are the full GTMUX_DEBUG_* names.
  @objc func constantsToExport() -> [AnyHashable: Any]! {
    var flags: [String: String] = [:]
    if let url = flagsURL, let data = try? Data(contentsOf: url),
       let obj = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] {
      for (k, v) in obj { flags[k] = "\(v)" }
    }
    for (k, v) in ProcessInfo.processInfo.environment where k.hasPrefix("GTMUX_DEBUG_") {
      flags[k] = v
    }
    return ["flags": flags]
  }

  private var docs: URL? {
    FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
  }
  private var logURL: URL? { docs?.appendingPathComponent("gtmux-debug.jsonl") }
  private var flagsURL: URL? { docs?.appendingPathComponent("gtmux-debug-flags.json") }

  // Append one line (a JSON object) to the debug log.
  @objc(record:)
  func record(_ line: NSString) {
    guard let url = logURL, let data = ((line as String) + "\n").data(using: .utf8) else { return }
    if let fh = try? FileHandle(forWritingTo: url) {
      defer { try? fh.close() }
      fh.seekToEndOfFile()
      fh.write(data)
    } else {
      try? data.write(to: url)
    }
  }

  // Truncate the debug log (call once at app start when logging is on).
  @objc func reset() {
    guard let url = logURL else { return }
    try? FileManager.default.removeItem(at: url)
  }
}
