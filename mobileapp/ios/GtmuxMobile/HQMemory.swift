import Foundation
import React
import zlib

// HQMemory — the phone holds a copy of the supervisor's memory.
//
// HQ's memory is the one thing gtmux keeps that cannot be reproduced: a situation board
// it has rewritten for months, a curated knowledge base, and a LOCAL.md the operator
// personalised once and which is never rewritten, so losing it does not self-heal.
//
// Hardened to match HQMemoryModule.kt: strict URL/token validation, ephemeral URLSession
// that refuses redirects, 128 MiB compressed and uncompressed bounds, complete gzip stream
// verification without extraction, generation invalidation on forget(), and file protection.

private final class MemoryStreamDelegate: NSObject, URLSessionDataDelegate {
  private let limit: Int64
  private let handle: FileHandle
  private let isCurrent: () -> Bool
  private let onFinish: (Int, Int64, Error?) -> Void
  private var statusCode = 0
  private var received: Int64 = 0
  private var abortedReason: NSError?

  init(limit: Int64,
       handle: FileHandle,
       isCurrent: @escaping () -> Bool,
       onFinish: @escaping (Int, Int64, Error?) -> Void) {
    self.limit = limit
    self.handle = handle
    self.isCurrent = isCurrent
    self.onFinish = onFinish
  }

  func urlSession(_ session: URLSession,
                  task: URLSessionTask,
                  willPerformHTTPRedirection response: HTTPURLResponse,
                  newRequest request: URLRequest,
                  completionHandler: @escaping (URLRequest?) -> Void) {
    abortedReason = NSError(domain: "HQMemory", code: 301,
                            userInfo: [NSLocalizedDescriptionKey: "Redirects are not permitted"])
    task.cancel()
    completionHandler(nil)
  }

  func urlSession(_ session: URLSession,
                  dataTask: URLSessionDataTask,
                  didReceive response: URLResponse,
                  completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
    guard isCurrent() else {
      abortedReason = NSError(domain: "HQMemory", code: 499,
                              userInfo: [NSLocalizedDescriptionKey: "Download cancelled"])
      completionHandler(.cancel)
      return
    }
    let http = response as? HTTPURLResponse
    statusCode = http?.statusCode ?? 0
    guard statusCode == 200 else {
      abortedReason = NSError(
        domain: "HQMemory", code: statusCode,
        userInfo: [NSLocalizedDescriptionKey: "The server could not provide a memory archive (\(statusCode))"])
      completionHandler(.cancel)
      return
    }
    if response.expectedContentLength > limit {
      abortedReason = NSError(
        domain: "HQMemory", code: 413,
        userInfo: [NSLocalizedDescriptionKey: "The memory archive exceeds the import limit"])
      completionHandler(.cancel)
      return
    }
    completionHandler(.allow)
  }

  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
    guard isCurrent() else {
      abortedReason = NSError(domain: "HQMemory", code: 499,
                              userInfo: [NSLocalizedDescriptionKey: "Download cancelled"])
      dataTask.cancel()
      return
    }
    received += Int64(data.count)
    if received > limit {
      abortedReason = NSError(
        domain: "HQMemory", code: 413,
        userInfo: [NSLocalizedDescriptionKey: "The memory archive exceeds the import limit"])
      dataTask.cancel()
      return
    }
    do {
      try handle.write(contentsOf: data)
    } catch {
      abortedReason = error as NSError
      dataTask.cancel()
    }
  }

  func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
    session.finishTasksAndInvalidate()
    onFinish(statusCode, received, abortedReason ?? error)
  }
}

@objc(HQMemory)
class HQMemory: NSObject {

  private static let limitBytes: Int64 = 128 * 1024 * 1024
  private static let minExpandedBytes: Int64 = 1024

  private let lock = NSLock()
  private var generation: UInt64 = 0
  private var activeTasks: [UInt64: URLSessionTask] = [:]

  @objc static func requiresMainQueueSetup() -> Bool { false }

  /// Where the copy lives. Documents, not Caches: Caches is excluded from the backup
  /// and can be evicted under storage pressure.
  private var directoryURL: URL? {
    guard let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first else {
      return nil
    }
    let dir = docs.appendingPathComponent("hq", isDirectory: true)
    try? FileManager.default.createDirectory(
      at: dir,
      withIntermediateDirectories: true,
      attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication]
    )
    return dir
  }

  private var legacyFileURL: URL? {
    FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)
      .first?.appendingPathComponent("gtmux-hq-memory.tar.gz")
  }

  private var fileURL: URL? {
    directoryURL?.appendingPathComponent("gtmux-hq-memory.tar.gz")
  }

  private var existingFileURL: URL? {
    if let primary = fileURL, FileManager.default.fileExists(atPath: primary.path) {
      return primary
    }
    if let legacy = legacyFileURL, FileManager.default.fileExists(atPath: legacy.path) {
      return legacy
    }
    return nil
  }

  private func beginSave() -> UInt64 {
    lock.lock()
    defer { lock.unlock() }
    generation &+= 1
    let id = generation
    let stale = Array(activeTasks.values)
    activeTasks.removeAll()
    stale.forEach { $0.cancel() }
    return id
  }

  private func trackTask(_ task: URLSessionTask, for id: UInt64) -> Bool {
    lock.lock()
    defer { lock.unlock() }
    guard generation == id else { return false }
    activeTasks[id] = task
    return true
  }

  private func untrackTask(for id: UInt64) {
    lock.lock()
    defer { lock.unlock() }
    activeTasks.removeValue(forKey: id)
  }

  private func isCurrent(_ id: UInt64) -> Bool {
    lock.lock()
    defer { lock.unlock() }
    return generation == id
  }

  @objc(save:token:resolver:rejecter:)
  func save(_ urlString: String, token: String,
            resolver resolve: @escaping RCTPromiseResolveBlock,
            rejecter reject: @escaping RCTPromiseRejectBlock) {
    let id = beginSave()
    guard let components = URLComponents(string: urlString),
          let scheme = components.scheme?.lowercased(),
          scheme == "http" || scheme == "https",
          let host = components.host, !host.isEmpty,
          components.user == nil,
          components.password == nil,
          components.query == nil,
          components.fragment == nil,
          components.path.hasSuffix("/api/hq/memory"),
          let url = components.url,
          let dir = directoryURL,
          let dst = fileURL else {
      reject("memory_copy_failed", "Invalid memory archive URL", nil)
      return
    }
    guard !token.isEmpty, token.utf8.count <= 8192, !token.contains("\r"), !token.contains("\n") else {
      reject("memory_copy_failed", "Invalid authorization token", nil)
      return
    }

    let temp = dir.appendingPathComponent("copy-\(id).partial")
    let fm = FileManager.default
    try? fm.removeItem(at: temp)
    guard fm.createFile(
      atPath: temp.path,
      contents: nil,
      attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication]
    ), let handle = try? FileHandle(forWritingTo: temp) else {
      reject("memory_copy_failed", "Could not prepare temporary archive", nil)
      return
    }

    var req = URLRequest(url: url)
    req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
    req.cachePolicy = .reloadIgnoringLocalAndRemoteCacheData
    req.timeoutInterval = 120

    let config = URLSessionConfiguration.ephemeral
    config.urlCache = nil
    config.httpCookieAcceptPolicy = .never
    config.httpShouldSetCookies = false
    config.timeoutIntervalForRequest = 15
    config.timeoutIntervalForResource = 120

    let delegate = MemoryStreamDelegate(
      limit: Self.limitBytes,
      handle: handle,
      isCurrent: { [weak self] in self?.isCurrent(id) ?? false },
      onFinish: { [weak self] statusCode, received, err in
        defer { try? fm.removeItem(at: temp) }
        try? handle.synchronize()
        try? handle.close()
        guard let self else { return }
        self.untrackTask(for: id)

        if let err {
          reject("memory_copy_failed", err.localizedDescription, err)
          return
        }
        guard statusCode == 200, received > 0, received <= Self.limitBytes, self.isCurrent(id) else {
          reject("memory_copy_failed", "Download cancelled or invalid", nil)
          return
        }

        do {
          try Self.validateGzipArchive(at: temp, limit: Self.limitBytes, minExpanded: Self.minExpandedBytes) {
            self.isCurrent(id)
          }
          var committed = false
          self.lock.lock()
          if self.generation == id {
            if fm.fileExists(atPath: dst.path) {
              _ = try fm.replaceItemAt(dst, withItemAt: temp)
            } else {
              try fm.moveItem(at: temp, to: dst)
            }
            if let legacy = self.legacyFileURL, legacy.path != dst.path {
              try? fm.removeItem(at: legacy)
            }
            try? fm.setAttributes(
              [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication],
              ofItemAtPath: dst.path
            )
            committed = true
          }
          self.lock.unlock()
          guard committed else {
            reject("memory_copy_failed", "Download cancelled", nil)
            return
          }
          resolve(self.describe(dst))
        } catch {
          reject("memory_copy_failed", error.localizedDescription, error)
        }
      }
    )

    let session = URLSession(configuration: config, delegate: delegate, delegateQueue: nil)
    let task = session.dataTask(with: req)
    guard trackTask(task, for: id) else {
      session.invalidateAndCancel()
      try? handle.close()
      try? fm.removeItem(at: temp)
      reject("memory_copy_failed", "Download cancelled", nil)
      return
    }
    task.resume()
  }

  /// Verify complete RFC 1952 gzip framing (header + CRC32 + ISIZE) and expanded byte bounds without extracting files.
  private static func validateGzipArchive(at url: URL,
                                          limit: Int64,
                                          minExpanded: Int64,
                                          isCurrent: () -> Bool) throws {
    let reader = try FileHandle(forReadingFrom: url)
    defer { try? reader.close() }

    var stream = z_stream()
    let initCode = inflateInit2_(&stream, MAX_WBITS + 16, ZLIB_VERSION, Int32(MemoryLayout<z_stream>.size))
    guard initCode == Z_OK else {
      throw NSError(domain: "HQMemory", code: Int(initCode),
                    userInfo: [NSLocalizedDescriptionKey: "Could not initialize gzip verifier"])
    }
    defer { inflateEnd(&stream) }

    let chunkSize = 32 * 1024
    var outBuffer = [UInt8](repeating: 0, count: chunkSize)
    var expanded: Int64 = 0
    var endedCleanly = false

    while true {
      guard isCurrent() else {
        throw NSError(domain: "HQMemory", code: 499,
                      userInfo: [NSLocalizedDescriptionKey: "Download cancelled"])
      }
      let input = (try? reader.read(upToCount: chunkSize)) ?? Data()
      if input.isEmpty { break }
      endedCleanly = false

      try input.withUnsafeBytes { rawIn in
        guard let inBase = rawIn.bindMemory(to: Bytef.self).baseAddress else { return }
        stream.next_in = UnsafeMutablePointer<Bytef>(mutating: inBase)
        stream.avail_in = uInt(input.count)

        while stream.avail_in > 0 {
          let code: Int32 = outBuffer.withUnsafeMutableBufferPointer { rawOut in
            stream.next_out = rawOut.baseAddress
            stream.avail_out = uInt(chunkSize)
            return inflate(&stream, Z_NO_FLUSH)
          }
          let produced = chunkSize - Int(stream.avail_out)
          if produced > 0 {
            expanded += Int64(produced)
            if expanded > limit {
              throw NSError(domain: "HQMemory", code: 413,
                            userInfo: [NSLocalizedDescriptionKey: "The memory archive exceeds the import limit"])
            }
          }
          if code == Z_STREAM_END {
            endedCleanly = true
            if stream.avail_in > 0 {
              let resetCode = inflateReset(&stream)
              guard resetCode == Z_OK else {
                throw NSError(domain: "HQMemory", code: Int(resetCode),
                              userInfo: [NSLocalizedDescriptionKey: "Corrupt gzip archive"])
              }
              endedCleanly = false
            }
          } else if code != Z_OK {
            throw NSError(domain: "HQMemory", code: Int(code),
                          userInfo: [NSLocalizedDescriptionKey: "Corrupt gzip archive"])
          }
        }
      }
    }

    guard endedCleanly, expanded >= minExpanded, expanded <= limit else {
      throw NSError(domain: "HQMemory", code: 422,
                    userInfo: [NSLocalizedDescriptionKey: "Incomplete or invalid memory archive"])
    }
  }

  /// What is stored, if anything: size, when it was taken, and the path to share it.
  @objc(state:rejecter:)
  func state(_ resolve: @escaping RCTPromiseResolveBlock, rejecter _: RCTPromiseRejectBlock) {
    guard let url = existingFileURL else {
      resolve(nil)
      return
    }
    resolve(describe(url))
  }

  /// Deletes the stored copy and invalidates any in-flight save.
  @objc(forget:rejecter:)
  func forget(_ resolve: @escaping RCTPromiseResolveBlock, rejecter _: RCTPromiseRejectBlock) {
    _ = beginSave()
    let fm = FileManager.default
    if let url = fileURL { try? fm.removeItem(at: url) }
    if let legacy = legacyFileURL { try? fm.removeItem(at: legacy) }
    if let dir = directoryURL, let items = try? fm.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil) {
      for item in items where item.pathExtension == "partial" {
        try? fm.removeItem(at: item)
      }
    }
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
