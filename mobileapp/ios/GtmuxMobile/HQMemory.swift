import Foundation
import React

// HQMemory — the phone holds a copy of the supervisor's memory.
//
// HQ's memory is the one thing gtmux keeps that cannot be reproduced: a situation board
// it has rewritten for months, a curated knowledge base, and a LOCAL.md the operator
// personalised once and which is never rewritten, so losing it does not self-heal. On the
// machine this was built for that was 6.1 MB with nothing protecting it — no export, no
// snapshot, no Time Machine destination, no cloud folder.
//
// The Mac now snapshots locally, which covers the deletion. It does not cover the disk,
// and asking someone to set up a cloud folder is asking them to configure a thing they
// will not configure. The phone is the answer that needs no setup: it is already paired,
// and a file in this app's Documents directory is INCLUDED IN THE iPHONE'S OWN BACKUP.
// The operator configures nothing and needs no account beyond the Apple ID they use
// anyway; replacing a phone restores the copy along with everything else.
//
// What this deliberately does NOT do is claim more than it can see. iOS exposes no API
// for "when did my last iCloud backup run", so the app can say a file is included in the
// backup and cannot say the backup happened. That is why `save` also returns a path the
// UI can hand to the share sheet: placing it in Files or iCloud Drive yourself is the
// version you can actually verify, and it is one tap.

@objc(HQMemory)
class HQMemory: NSObject {

  @objc static func requiresMainQueueSetup() -> Bool { false }

  /// Where the copy lives. Documents, not Caches: Caches is excluded from the backup
  /// and can be evicted under storage pressure, which is the one thing this must not be.
  private var fileURL: URL? {
    FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)
      .first?.appendingPathComponent("gtmux-hq-memory.tar.gz")
  }

  /// Downloads the archive from a paired Mac and replaces the stored copy.
  ///
  /// Downloaded to a temp file and MOVED into place, so an interrupted transfer cannot
  /// replace a good backup with half of one — the failure this exists to prevent is
  /// exactly "I had a copy and now I have a broken copy".
  @objc(save:token:resolver:rejecter:)
  func save(_ urlString: String, token: String,
            resolver resolve: @escaping RCTPromiseResolveBlock,
            rejecter reject: @escaping RCTPromiseRejectBlock) {
    guard let url = URL(string: urlString), let dst = fileURL else {
      reject("bad_url", "not a usable URL", nil)
      return
    }
    var req = URLRequest(url: url)
    req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
    req.timeoutInterval = 120

    URLSession.shared.downloadTask(with: req) { tmp, response, err in
      if let err = err {
        reject("download_failed", err.localizedDescription, err)
        return
      }
      guard let http = response as? HTTPURLResponse, http.statusCode == 200, let tmp = tmp else {
        let code = (response as? HTTPURLResponse)?.statusCode ?? 0
        reject("download_failed", "the Mac answered \(code)", nil)
        return
      }
      do {
        // Replace atomically. A partial file must never become the stored copy.
        if FileManager.default.fileExists(atPath: dst.path) {
          _ = try FileManager.default.replaceItemAt(dst, withItemAt: tmp)
        } else {
          try FileManager.default.moveItem(at: tmp, to: dst)
        }
        resolve(self.describe(dst))
      } catch {
        reject("save_failed", error.localizedDescription, error)
      }
    }.resume()
  }

  /// What is stored, if anything: size, when it was taken, and the path to share it.
  @objc(state:rejecter:)
  func state(_ resolve: @escaping RCTPromiseResolveBlock, rejecter _: RCTPromiseRejectBlock) {
    guard let url = fileURL, FileManager.default.fileExists(atPath: url.path) else {
      resolve(nil)
      return
    }
    resolve(describe(url))
  }

  /// Deletes the stored copy. The operator's data, so it must be removable from the
  /// device that is holding it.
  @objc(forget:rejecter:)
  func forget(_ resolve: @escaping RCTPromiseResolveBlock, rejecter _: RCTPromiseRejectBlock) {
    if let url = fileURL { try? FileManager.default.removeItem(at: url) }
    resolve(nil)
  }

  private func describe(_ url: URL) -> [String: Any] {
    let attrs = try? FileManager.default.attributesOfItem(atPath: url.path)
    return [
      "path": url.path,
      "url": url.absoluteString,
      "bytes": (attrs?[.size] as? NSNumber)?.int64Value ?? 0,
      "at": Int((attrs?[.modificationDate] as? Date)?.timeIntervalSince1970 ?? 0),
    ]
  }
}
