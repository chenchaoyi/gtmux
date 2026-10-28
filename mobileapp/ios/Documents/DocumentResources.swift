import Foundation
import WebKit
import ImageIO

/// A fixed set of parent-document image references. The page cannot choose an API
/// URL, pairing header, filesystem path outside the parent or an unbounded download.
final class DocumentResources: NSObject, WKURLSchemeHandler {
  private struct Pending { let task: WKURLSchemeTask; let reference: String; var transport: DocumentTransport? }
  private var tasks: [ObjectIdentifier: Pending] = [:]
  private var queue: [ObjectIdentifier] = []
  private let references: [String: String]
  private let request: [String: Any]
  private var active = 0
  private var served = 0
  private var closed = false
  init(references: [String: String], request: [String: Any], parentID: String) {
    self.references = references
    var related = request; related["parent_id"] = parentID; related["maximumBytes"] = Int64(8 * 1024 * 1024)
    self.request = related
  }
  func webView(_ webView: WKWebView, start urlSchemeTask: WKURLSchemeTask) {
    guard !closed, let url = urlSchemeTask.request.url, let ref = references[url.absoluteString] else {
      urlSchemeTask.didFailWithError(URLError(.resourceUnavailable)); return
    }
    let id = ObjectIdentifier(urlSchemeTask)
    tasks[id] = Pending(task: urlSchemeTask, reference: ref); queue.append(id); pump()
  }
  func webView(_ webView: WKWebView, stop urlSchemeTask: WKURLSchemeTask) {
    let id = ObjectIdentifier(urlSchemeTask)
    guard let value = tasks.removeValue(forKey: id) else { return }
    if let transport = value.transport { active -= 1; transport.cleanup() }
    queue.removeAll { $0 == id }; pump()
  }
  func cleanup() {
    closed = true
    for value in tasks.values { value.transport?.cleanup() }
    tasks.removeAll(); queue.removeAll(); active = 0
  }
  private func pump() {
    while !closed && active < 2 && !queue.isEmpty {
      let id = queue.removeFirst()
      guard var pending = tasks[id] else { continue }
      guard served < 20 * 1024 * 1024 else { finish(id, error: URLError(.dataLengthExceedsMaximum)); continue }
      var input = request; input["reference"] = pending.reference
      // Published IDs already carry exact session/home scope; parent lookup is for paths.
      if pending.reference.hasPrefix("gtmux-file:") { input["parent_id"] = nil }
      guard let transport = DocumentTransport(request: input) else { finish(id, error: URLError(.badURL)); continue }
      pending.transport = transport; tasks[id] = pending; active += 1
      transport.onFailure = { [weak self] _ in self?.finish(id, error: URLError(.resourceUnavailable)) }
      transport.onReady = { [weak self] file, local in
        guard let self, let task = self.tasks[id]?.task, !self.closed else { return }
        guard file.kind == "image", file.bytes <= 8 * 1024 * 1024, self.served + Int(file.bytes) <= 20 * 1024 * 1024,
              let image = CGImageSourceCreateWithURL(local as CFURL, nil),
              let props = CGImageSourceCopyPropertiesAtIndex(image, 0, nil) as? [CFString: Any],
              let width = props[kCGImagePropertyPixelWidth] as? Int, let height = props[kCGImagePropertyPixelHeight] as? Int,
              width > 0, height > 0, width <= 20000, height <= 20000, width * height <= 50_000_000,
              let data = try? Data(contentsOf: local), let url = task.request.url else {
          self.finish(id, error: URLError(.cannotDecodeContentData)); return
        }
        self.served += data.count
        task.didReceive(URLResponse(url: url, mimeType: file.media_type, expectedContentLength: data.count, textEncodingName: nil))
        task.didReceive(data); task.didFinish(); self.finish(id)
      }
      transport.start()
    }
  }
  private func finish(_ id: ObjectIdentifier, error: Error? = nil) {
    guard let value = tasks.removeValue(forKey: id) else { return }
    if let error, !closed { value.task.didFailWithError(error) }
    if let transport = value.transport { active -= 1; transport.cleanup() }
    pump()
  }
}
