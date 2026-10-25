import Foundation

enum WidgetUsageFailure: Error { case refused, unavailable, invalid }

/// Parent-only, bounded requests to the selected origin. A redirect never
/// inherits a pairing bearer; widgets themselves have no network client.
final class WidgetUsageRequest: NSObject, URLSessionDataDelegate {
  private let connection: WidgetOwnerConnection
  private let completion: (Result<Data, WidgetUsageFailure>) -> Void
  private var bytes = Data()
  private var status = 0
  private var checkingScope = true
  private var ended = false
  private let queue: OperationQueue = {
    let q = OperationQueue(); q.maxConcurrentOperationCount = 1; return q
  }()
  private lazy var session: URLSession = {
    let config = URLSessionConfiguration.ephemeral
    config.urlCache = nil; config.httpCookieStorage = nil; config.httpShouldSetCookies = false
    config.requestCachePolicy = .reloadIgnoringLocalAndRemoteCacheData
    config.timeoutIntervalForRequest = 15; config.timeoutIntervalForResource = 25
    return URLSession(configuration: config, delegate: self, delegateQueue: queue)
  }()

  init(connection: WidgetOwnerConnection, completion: @escaping (Result<Data, WidgetUsageFailure>) -> Void) {
    self.connection = connection; self.completion = completion; super.init()
  }

  func start() { queue.addOperation { [self] in request("share") } }
  func cancel() { queue.addOperation { [self] in finish(.failure(.unavailable)) } }

  private func request(_ endpoint: String) {
    guard !ended else { return }
    bytes = Data(); status = 0
    let raw = connection.base.absoluteString.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    guard let url = URL(string: raw + "/api/" + endpoint) else { finish(.failure(.invalid)); return }
    var request = URLRequest(url: url)
    request.setValue("Bearer " + connection.token, forHTTPHeaderField: "Authorization")
    request.setValue("ios-widget-refresh", forHTTPHeaderField: "X-Gtmux-Client")
    session.dataTask(with: request).resume()
  }

  private func finish(_ result: Result<Data, WidgetUsageFailure>) {
    guard !ended else { return }
    ended = true; session.invalidateAndCancel()
    DispatchQueue.main.async { self.completion(result) }
  }

  func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                  newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void) {
    completionHandler(nil); finish(.failure(.unavailable))
  }

  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse,
                  completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
    guard !ended, let http = response as? HTTPURLResponse else { completionHandler(.cancel); return }
    status = http.statusCode
    if status == 401 || status == 403 {
      completionHandler(.cancel); finish(.failure(.refused)); return
    }
    let limit: Int64 = checkingScope ? 65_536 : 2 * 1024 * 1024
    guard response.expectedContentLength <= limit else { completionHandler(.cancel); finish(.failure(.invalid)); return }
    completionHandler(.allow)
  }

  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
    guard !ended else { return }
    let limit = checkingScope ? 65_536 : 2 * 1024 * 1024
    guard bytes.count + data.count <= limit else { finish(.failure(.invalid)); return }
    bytes.append(data)
  }

  func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
    guard !ended else { return }
    if status == 401 || status == 403 { finish(.failure(.refused)); return }
    guard error == nil, status == 200 else { finish(.failure(.unavailable)); return }
    if checkingScope {
      guard let scope = try? JSONDecoder().decode(OwnerScope.self, from: bytes), scope.all else {
        finish(.failure(.refused)); return
      }
      checkingScope = false; request("usage")
    } else { finish(.success(bytes)) }
  }
}

private struct OwnerScope: Decodable { let all: Bool }
