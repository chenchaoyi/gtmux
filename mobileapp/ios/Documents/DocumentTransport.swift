import Foundation
import CryptoKit

struct ConversationDocument: Decodable {
  let id: String
  let name: String
  let kind: String
  let media_type: String
  let bytes: Int64
  let sha256: String
  let created_at: Int64
  let session_id: String
  let home_id: String?
  let source: String

  func valid(session: String, home: String) -> Bool {
    id.range(of: "^[a-f0-9]{32}$", options: .regularExpression) != nil &&
    sha256.range(of: "^[a-f0-9]{64}$", options: .regularExpression) != nil &&
    bytes > 0 && bytes <= DocumentTransport.maximumBytes && session_id == session && (home_id ?? "") == home &&
    !name.isEmpty && name.count <= 1024 && name.rangeOfCharacter(from: .controlCharacters) == nil &&
    ["html", "markdown", "text", "pdf", "image"].contains(kind) && source.hasPrefix("/") && source.count <= 4096 && created_at > 0
  }
}

enum DocumentFailure: Error { case permission, unavailable, unsupported, network, invalid, tooLarge, format }

/// Owns an ephemeral HTTP session and one private cache directory. No redirect
/// inherits the pairing header, and downloaded bytes never cross the JS bridge.
final class DocumentTransport: NSObject, URLSessionDataDelegate, URLSessionDownloadDelegate {
  static let maximumBytes: Int64 = 30 * 1024 * 1024
  private let resolveURL: URL
  private let headers: [String: String]
  private let reference: String
  private let sessionID: String
  private let homeID: String
  private let parentID: String?
  private let byteLimit: Int64
  private var metadataBytes = Data()
  private var metadataStatus = 0
  private var metadata: ConversationDocument?
  private var failed = false
  private var cancelled = false
  private(set) var directory: URL?
  var onProgress: ((Double) -> Void)?
  var onReady: ((ConversationDocument, URL) -> Void)?
  var onFailure: ((DocumentFailure) -> Void)?

  private let delegateQueue: OperationQueue = {
    let queue = OperationQueue(); queue.maxConcurrentOperationCount = 1; queue.name = "gtmux.document.transport"; return queue
  }()
  private lazy var session: URLSession = {
    let config = URLSessionConfiguration.ephemeral
    config.urlCache = nil
    config.httpCookieStorage = nil
    config.httpShouldSetCookies = false
    config.requestCachePolicy = .reloadIgnoringLocalAndRemoteCacheData
    config.timeoutIntervalForRequest = 30
    config.timeoutIntervalForResource = 300
    return URLSession(configuration: config, delegate: self, delegateQueue: delegateQueue)
  }()

  init?(request: [String: Any]) {
    guard let raw = request["resolveURL"] as? String, let url = URL(string: raw),
          ["http", "https"].contains(url.scheme?.lowercased() ?? ""), url.host != nil,
          url.user == nil, url.password == nil, url.query == nil, url.fragment == nil,
          url.path.hasSuffix("/api/artifacts/resolve"),
          let reference = request["reference"] as? String, !reference.isEmpty, reference.utf8.count <= 4096,
          let sid = request["session_id"] as? String, !sid.isEmpty, sid.count <= 256 else { return nil }
    self.resolveURL = url
    self.reference = reference
    self.sessionID = sid
    self.homeID = request["home_id"] as? String ?? ""
    self.parentID = request["parent_id"] as? String
    self.byteLimit = min(Self.maximumBytes, max(1, request["maximumBytes"] as? Int64 ?? Self.maximumBytes))
    let supplied = request["headers"] as? [String: String] ?? [:]
    self.headers = supplied.filter { ["authorization", "x-gtmux-client"].contains($0.key.lowercased()) && $0.value.rangeOfCharacter(from: .newlines) == nil }
    super.init()
  }

  func start() {
    var req = authorized(resolveURL)
    req.httpMethod = "POST"
    req.setValue("application/json", forHTTPHeaderField: "Content-Type")
    var body = ["reference": reference, "session_id": sessionID, "home_id": homeID]
    if let parentID { body["parent_id"] = parentID }
    req.httpBody = try? JSONSerialization.data(withJSONObject: body)
    delegateQueue.addOperation { [self] in
      guard !cancelled else { return }; session.dataTask(with: req).resume()
    }
  }

  private func authorized(_ url: URL) -> URLRequest {
    var request = URLRequest(url: url, cachePolicy: .reloadIgnoringLocalAndRemoteCacheData)
    for (key, value) in headers { request.setValue(value, forHTTPHeaderField: key) }
    return request
  }

  func cancel() {
    delegateQueue.addOperation { [self] in
      cancelled = true; session.invalidateAndCancel()
    }
  }

  func cleanup() {
    cancel()
    // This UUID directory was created exclusively by this transport, not a
    // caller-selected URL, source document or shared/global cache.
    delegateQueue.addOperation { [self] in
      if let directory { try? FileManager.default.removeItem(at: directory) }
      directory = nil; onProgress = nil; onReady = nil; onFailure = nil
    }
  }

  private func fail(_ reason: DocumentFailure) {
    guard !failed && !cancelled else { return }
    failed = true
    session.invalidateAndCancel()
    let callback = onFailure
    DispatchQueue.main.async { callback?(reason) }
  }

  func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                  newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void) {
    completionHandler(nil)
    fail(.network)
  }

  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse,
                  completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
    guard let http = response as? HTTPURLResponse, response.expectedContentLength <= 24 * 1024 else {
      completionHandler(.cancel); fail(.invalid); return
    }
    metadataStatus = http.statusCode
    completionHandler(.allow)
  }

  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
    guard metadataBytes.count + data.count <= 24 * 1024 else { fail(.invalid); return }
    metadataBytes.append(data)
  }

  func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
    guard !failed && !cancelled else { return }
    if error != nil { fail(.network); return }
    guard task is URLSessionDataTask else { return }
    if metadataStatus == 401 || metadataStatus == 403 { fail(.permission); return }
    if metadataStatus == 503 { fail(.unsupported); return }
    if metadataStatus == 413 { fail(.tooLarge); return }
    if metadataStatus >= 500 { fail(.network); return }
    if metadataStatus == 404 {
      let json = (try? JSONSerialization.jsonObject(with: metadataBytes)) as? [String: Any]
      fail(json?["code"] as? String == "document_unpublished" ? .unavailable : .unsupported)
      return
    }
    guard metadataStatus == 200,
          let file = try? JSONDecoder().decode(ConversationDocument.self, from: metadataBytes),
          file.valid(session: sessionID, home: homeID),
          var parts = URLComponents(url: resolveURL, resolvingAgainstBaseURL: false) else { fail(.invalid); return }
    guard file.bytes <= byteLimit else { fail(.tooLarge); return }
    metadata = file
    parts.path = String(parts.path.dropLast("resolve".count)) + "content"
    parts.queryItems = [URLQueryItem(name: "id", value: file.id), URLQueryItem(name: "session_id", value: sessionID), URLQueryItem(name: "home_id", value: homeID)]
    guard let url = parts.url else { fail(.invalid); return }
    session.downloadTask(with: authorized(url)).resume()
  }

  func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didWriteData bytesWritten: Int64,
                  totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64) {
    guard !failed && !cancelled else { return }
    guard let file = metadata, totalBytesWritten <= file.bytes, totalBytesWritten <= Self.maximumBytes else { fail(.tooLarge); return }
    let progress = Double(totalBytesWritten) / Double(file.bytes)
    let callback = onProgress
    DispatchQueue.main.async { callback?(progress) }
  }

  func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
    guard !failed && !cancelled, let file = metadata,
          let response = downloadTask.response as? HTTPURLResponse else { return }
    if response.statusCode == 401 || response.statusCode == 403 { fail(.permission); return }
    if response.statusCode == 404 { fail(.unavailable); return }
    guard response.statusCode == 200 else { fail(.network); return }
    do {
      let attributes = try FileManager.default.attributesOfItem(atPath: location.path)
      guard (attributes[.size] as? NSNumber)?.int64Value == file.bytes else { fail(.invalid); return }
      let digest = try Self.digest(location)
      guard digest == file.sha256 else { fail(.invalid); return }
      let parent = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0].appendingPathComponent("ConversationDocuments", isDirectory: true)
      let folder = parent.appendingPathComponent(UUID().uuidString, isDirectory: true)
      try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
      directory = folder
      let suffix = file.kind == "html" ? "html" : file.kind == "pdf" ? "pdf" : file.kind == "image" ? (file.media_type == "image/png" ? "png" : "jpg") : "txt"
      let name = file.name
      let safeName = name.utf8.count <= 240 && !name.hasPrefix(".") && !name.contains("/") && !name.contains("\\") ? name : "document.\(suffix)"
      let target = folder.appendingPathComponent(safeName)
      try FileManager.default.moveItem(at: location, to: target)
      try FileManager.default.setAttributes([.posixPermissions: 0o600, .protectionKey: FileProtectionType.complete], ofItemAtPath: target.path)
      let callback = onReady
      DispatchQueue.main.async { callback?(file, target) }
      session.finishTasksAndInvalidate()
    } catch { fail(.invalid) }
  }

  static func digest(_ url: URL) throws -> String {
    let input = try FileHandle(forReadingFrom: url)
    defer { try? input.close() }
    var hash = SHA256()
    while let data = try input.read(upToCount: 1024 * 1024), !data.isEmpty { hash.update(data: data) }
    return hash.finalize().map { String(format: "%02x", $0) }.joined()
  }
}
