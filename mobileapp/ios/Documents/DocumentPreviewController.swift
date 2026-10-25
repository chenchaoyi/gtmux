import UIKit
import QuickLook
import WebKit
import ImageIO

/// Preview UI has no pairing credentials, application bridge or shared web login.
final class DocumentPreviewController: UIViewController, QLPreviewControllerDataSource, QLPreviewControllerDelegate, WKNavigationDelegate, WKUIDelegate {
  private let request: [String: Any]
  private var transport: DocumentTransport?
  private var file: ConversationDocument?
  private var originalURL: URL?
  private var htmlURL: URL?
  private var webView: WKWebView?
  private let content = UIView()
  private let status = UILabel()
  private let progress = UIProgressView(progressViewStyle: .default)
  private let retry = UIButton(type: .system)
  private let interaction = UIButton(type: .system)
  private var authorizedScriptHash: String?
  private var renderID = UUID()
  private let spinner = UIActivityIndicatorView(style: .medium)
  private var closing = false
  private var zh: Bool { request["lang"] as? String == "zh" }
  private func tr(_ en: String, _ cn: String) -> String { zh ? cn : en }

  init(request: [String: Any]) { self.request = request; super.init(nibName: nil, bundle: nil) }
  required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

  override func viewDidLoad() {
    super.viewDidLoad()
    view.backgroundColor = .systemBackground
    title = (request["name"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? tr("File preview", "文件预览")
    navigationItem.leftBarButtonItem = UIBarButtonItem(barButtonSystemItem: .close, target: self, action: #selector(closePreview))
    navigationItem.rightBarButtonItems = [UIBarButtonItem(barButtonSystemItem: .action, target: self, action: #selector(shareFile)), UIBarButtonItem(image: UIImage(systemName: "info.circle"), style: .plain, target: self, action: #selector(showInfo))]
    navigationItem.leftBarButtonItem?.accessibilityIdentifier = "document-preview-close"
    navigationItem.leftBarButtonItem?.accessibilityLabel = tr("Close preview", "关闭预览")
    navigationItem.rightBarButtonItems?.first?.accessibilityIdentifier = "document-preview-share"
    navigationItem.rightBarButtonItems?.first?.accessibilityLabel = tr("Share file", "分享文件")
    navigationItem.rightBarButtonItems?.last?.accessibilityIdentifier = "document-preview-info"
    navigationItem.rightBarButtonItems?.last?.accessibilityLabel = tr("File information", "文件信息")
    navigationItem.rightBarButtonItems?.first?.isEnabled = false
    status.numberOfLines = 0
    status.font = .preferredFont(forTextStyle: .subheadline)
    status.textColor = .secondaryLabel
    status.adjustsFontForContentSizeCategory = true
    retry.setTitle(tr("Retry", "重试"), for: .normal)
    retry.addTarget(self, action: #selector(start), for: .touchUpInside)
    interaction.setTitle(tr("Enable interaction", "开启交互"), for: .normal)
    interaction.accessibilityIdentifier = "document-preview-interaction"
    interaction.titleLabel?.font = .preferredFont(forTextStyle: .subheadline)
    interaction.titleLabel?.adjustsFontForContentSizeCategory = true
    interaction.titleLabel?.numberOfLines = 2
    interaction.titleLabel?.textAlignment = .center
    interaction.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
    interaction.addTarget(self, action: #selector(toggleInteraction), for: .touchUpInside)
    interaction.isHidden = true
    let controls = UIStackView(arrangedSubviews: [spinner, status, interaction, retry])
    controls.axis = .horizontal; controls.alignment = .fill; controls.spacing = 12
    controls.setContentHuggingPriority(.required, for: .vertical)
    controls.setContentCompressionResistancePriority(.required, for: .vertical)
    content.setContentHuggingPriority(UILayoutPriority(1), for: .vertical)
    let stack = UIStackView(arrangedSubviews: [controls, progress, content])
    stack.axis = .vertical; stack.spacing = 12; stack.translatesAutoresizingMaskIntoConstraints = false
    view.addSubview(stack)
    let retryWidth = retry.widthAnchor.constraint(greaterThanOrEqualToConstant: 44)
    retryWidth.priority = UILayoutPriority(999)
    let interactionWidth = interaction.widthAnchor.constraint(greaterThanOrEqualToConstant: 88)
    interactionWidth.priority = UILayoutPriority(999)
    NSLayoutConstraint.activate([
      stack.leadingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.leadingAnchor, constant: 16),
      stack.trailingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.trailingAnchor, constant: -16),
      stack.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 12),
      stack.bottomAnchor.constraint(equalTo: view.safeAreaLayoutGuide.bottomAnchor, constant: -12),
      content.heightAnchor.constraint(greaterThanOrEqualToConstant: 1),
      interactionWidth, interaction.heightAnchor.constraint(greaterThanOrEqualToConstant: 44),
      retryWidth, retry.heightAnchor.constraint(greaterThanOrEqualToConstant: 44),
    ])
    start()
  }

  @objc private func start() {
    transport?.cleanup()
    authorizedScriptHash = nil; renderID = UUID(); interaction.isHidden = true
    navigationItem.rightBarButtonItems?.first?.isEnabled = false
    content.subviews.forEach { $0.removeFromSuperview() }
    for child in children { child.willMove(toParent: nil); child.removeFromParent() }
    webView = nil; originalURL = nil; file = nil; htmlURL = nil
    retry.isHidden = true; progress.isHidden = false; progress.progress = 0
    status.text = tr("Reading file…", "正在读取文件…")
    spinner.startAnimating()
    guard let next = DocumentTransport(request: request) else { showFailure(.invalid); return }
    transport = next
    next.onProgress = { [weak self, weak next] value in
      guard let self, !self.closing, self.transport === next else { return }; self.progress.progress = Float(value)
      self.status.text = self.tr("Downloading · \(Int(value * 100))%", "正在下载 · \(Int(value * 100))%")
    }
    next.onFailure = { [weak self, weak next] reason in
      guard let self, !self.closing, self.transport === next else { return }; self.showFailure(reason)
    }
    next.onReady = { [weak self, weak next] meta, url in
      guard let self, !self.closing, self.transport === next else { return }; self.show(meta, url: url)
    }
    next.start()
  }

  private func showFailure(_ reason: DocumentFailure) {
    spinner.stopAnimating(); progress.isHidden = true; retry.isHidden = false
    switch reason {
    case .permission: status.text = tr("This connection cannot view this file.", "此连接没有文件查看权限。")
    case .unavailable: status.text = tr("This file has not been made available, or was removed. Ask the agent to provide it, or copy its path from File information.", "此文件尚未提供预览，或已被移除。可让 Agent 重新提供，也可在文件信息中复制路径。")
    case .unsupported: status.text = tr("Update the server to preview conversation files.", "请更新服务器以预览会话文件。")
    case .tooLarge: status.text = tr("This file exceeds the 30 MiB preview limit.", "文件超过 30 MiB 预览上限。")
    case .invalid: status.text = tr("File verification failed. Nothing was opened.", "文件校验失败，未打开内容。")
    case .format: status.text = tr("Preview is unavailable for this format. You can save the file.", "此格式暂时无法预览，可以保存原文件。")
    case .network: status.text = tr("Could not read the file. Check the connection and retry.", "暂时无法读取文件，请检查连接后重试。")
    }
  }

  private func show(_ file: ConversationDocument, url: URL) {
    guard !closing else { return }
    self.file = file; originalURL = url; title = file.name
    navigationItem.rightBarButtonItems?.forEach { $0.isEnabled = true }
    spinner.stopAnimating(); progress.isHidden = true
    updateFileStatus()
    switch file.kind {
    case "html": interaction.isHidden = false; showHTML(url)
    case "text", "markdown":
      do {
        let handle = try FileHandle(forReadingFrom: url); defer { try? handle.close() }
        let bytes = try handle.read(upToCount: 512 * 1024) ?? Data()
        let text = UITextView(); text.isEditable = false; text.isSelectable = true
        text.font = UIFontMetrics(forTextStyle: .body).scaledFont(for: .monospacedSystemFont(ofSize: 15, weight: .regular))
        text.adjustsFontForContentSizeCategory = true; text.backgroundColor = .clear
        text.text = String(decoding: bytes, as: UTF8.self)
        if file.bytes > bytes.count { status.text = tr("Previewing the first 512 KiB · Share saves the complete file", "仅预览前 512 KiB · 分享可保存完整文件") }
        mount(text)
      } catch { showFailure(.format) }
    default:
      if file.kind == "image" {
        guard let image = CGImageSourceCreateWithURL(url as CFURL, nil),
              let props = CGImageSourceCopyPropertiesAtIndex(image, 0, nil) as? [CFString: Any],
              let width = props[kCGImagePropertyPixelWidth] as? Int, let height = props[kCGImagePropertyPixelHeight] as? Int,
              width > 0, height > 0, width <= 20000, height <= 20000, width * height <= 50_000_000 else { showFailure(.format); return }
      }
      guard QLPreviewController.canPreview(url as NSURL) else { showFailure(.format); return }
      let preview = QLPreviewController(); preview.dataSource = self; preview.delegate = self
      addChild(preview); mount(preview.view); preview.didMove(toParent: self)
    }
  }

  private func mount(_ child: UIView) {
    child.translatesAutoresizingMaskIntoConstraints = false; content.addSubview(child)
    NSLayoutConstraint.activate([child.leadingAnchor.constraint(equalTo: content.leadingAnchor), child.trailingAnchor.constraint(equalTo: content.trailingAnchor), child.topAnchor.constraint(equalTo: content.topAnchor), child.bottomAnchor.constraint(equalTo: content.bottomAnchor)])
  }

  private var scriptsAllowed: Bool { file != nil && authorizedScriptHash == file?.sha256 }

  private func updateFileStatus() {
    guard let file else { return }
    let kind = ["html": "HTML", "pdf": "PDF", "markdown": "Markdown", "text": tr("Text", "文本"), "image": tr("Image", "图片")][file.kind] ?? file.kind
    let mode = file.kind == "html" ? (scriptsAllowed ? tr(" · Interactive", " · 已开启交互") : tr(" · Static preview", " · 静态预览")) : ""
    status.text = "\(kind) · \(ByteCountFormatter.string(fromByteCount: file.bytes, countStyle: .file))\(mode)"
    interaction.setTitle(scriptsAllowed ? tr("Use static preview", "关闭交互") : tr("Enable interaction", "开启交互"), for: .normal)
  }

  @objc private func toggleInteraction() {
    guard let file, file.kind == "html", let source = originalURL else { return }
    if scriptsAllowed { reloadHTML(allowing: nil); return }
    let alert = UIAlertController(title: tr("Trust this file?", "信任此文件？"),
      message: tr("Interactive mode runs scripts from this file. They may attempt network connections. Enable it only for a file you trust.", "交互模式会运行文件中的脚本，脚本可能尝试联网。仅对信任的文件开启。"), preferredStyle: .alert)
    alert.addAction(UIAlertAction(title: tr("Cancel", "取消"), style: .cancel))
    let enable = UIAlertAction(title: tr("Enable", "开启"), style: .default) { [weak self] _ in
      guard let self, !self.closing, self.file?.sha256 == file.sha256, self.originalURL == source else { return }
      self.reloadHTML(allowing: file.sha256)
    }
    enable.accessibilityIdentifier = "document-preview-enable-scripts"
    alert.addAction(enable); present(alert, animated: true)
  }

  private func reloadHTML(allowing hash: String?) {
    guard let source = originalURL, file?.kind == "html" else { return }
    authorizedScriptHash = hash; renderID = UUID()
    webView?.stopLoading(); webView?.removeFromSuperview(); webView = nil
    updateFileStatus(); showHTML(source)
  }

  private func showHTML(_ source: URL) {
    do {
      let generation = UUID(); renderID = generation
      let allowScripts = scriptsAllowed
      let html = try String(contentsOf: source, encoding: .utf8)
      let policy = "default-src 'none'; script-src \(allowScripts ? "'unsafe-inline'" : "'none'"); style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; frame-src 'none'; worker-src 'none'; object-src 'none'; media-src 'none'; base-uri 'none'; form-action 'none'"
      let secured = "<!doctype html><meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"\(policy)\">" + html
      let rendered = source.deletingLastPathComponent().appendingPathComponent("Rendered", isDirectory: true)
      try FileManager.default.createDirectory(at: rendered, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
      let target = rendered.appendingPathComponent("preview.html")
      try secured.write(to: target, atomically: true, encoding: .utf8)
      try FileManager.default.setAttributes([.posixPermissions: 0o600, .protectionKey: FileProtectionType.complete], ofItemAtPath: target.path)
      htmlURL = target
      let configuration = WKWebViewConfiguration()
      configuration.websiteDataStore = .nonPersistent()
      configuration.defaultWebpagePreferences.allowsContentJavaScript = allowScripts
      configuration.preferences.javaScriptCanOpenWindowsAutomatically = false
      if #available(iOS 15.4, *) { configuration.preferences.isElementFullscreenEnabled = false }
      let rules = "[" + ["http", "https", "ws", "wss", "ftp"].map {
        "{\"trigger\":{\"url-filter\":\"^\($0):\"},\"action\":{\"type\":\"block\"}}"
      }.joined(separator: ",") + "]"
      WKContentRuleListStore.default().compileContentRuleList(forIdentifier: "gtmux-document-offline-v1", encodedContentRuleList: rules) { [weak self] list, error in
        DispatchQueue.main.async {
          guard let self, !self.closing, self.htmlURL == target, self.originalURL == source, self.renderID == generation else { return }
          guard let list, error == nil else { self.showFailure(.format); return }
          configuration.userContentController.add(list)
          // No WKScriptMessageHandler, user credentials or business API bridge.
          let web = WKWebView(frame: .zero, configuration: configuration)
          web.navigationDelegate = self; web.uiDelegate = self; web.allowsLinkPreview = false
          self.webView = web; self.mount(web)
          web.loadFileURL(target, allowingReadAccessTo: target.deletingLastPathComponent())
        }
      }
    } catch { showFailure(.format) }
  }

  func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction,
               decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
    guard !closing, self.webView === webView else { decisionHandler(.cancel); return }
    let url = navigationAction.request.url
    let ownDocument = url?.isFileURL == true && url?.standardizedFileURL.path == htmlURL?.standardizedFileURL.path
    decisionHandler(ownDocument && navigationAction.targetFrame?.isMainFrame == true && !navigationAction.shouldPerformDownload ? .allow : .cancel)
  }
  func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
               for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? { nil }
  // Reports never need the camera, microphone, motion sensors or device location.
  func webView(_ webView: WKWebView, requestMediaCapturePermissionFor origin: WKSecurityOrigin,
               initiatedByFrame frame: WKFrameInfo, type: WKMediaCaptureType,
               decisionHandler: @escaping (WKPermissionDecision) -> Void) { decisionHandler(.deny) }
  func webView(_ webView: WKWebView, requestDeviceOrientationAndMotionPermissionFor origin: WKSecurityOrigin,
               initiatedByFrame frame: WKFrameInfo,
               decisionHandler: @escaping (WKPermissionDecision) -> Void) { decisionHandler(.deny) }
  @available(iOS 27.0, *)
  @objc(webView:requestGeolocationPermissionForOrigin:initiatedByFrame:decisionHandler:)
  func webView(_ webView: WKWebView, requestGeolocationPermissionFor origin: WKSecurityOrigin,
               initiatedByFrame frame: WKFrameInfo,
               decisionHandler: @escaping (WKPermissionDecision) -> Void) { decisionHandler(.deny) }

  func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
    if !closing && self.webView === webView { showFailure(.format) }
  }
  func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
    if !closing && self.webView === webView && (error as NSError).code != NSURLErrorCancelled { showFailure(.format) }
  }
  func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
    if !closing && self.webView === webView && (error as NSError).code != NSURLErrorCancelled { showFailure(.format) }
  }

  func numberOfPreviewItems(in controller: QLPreviewController) -> Int { originalURL == nil ? 0 : 1 }
  func previewController(_ controller: QLPreviewController, previewItemAt index: Int) -> QLPreviewItem { originalURL! as NSURL }

  func previewController(_ controller: QLPreviewController, shouldOpen url: URL, for item: QLPreviewItem) -> Bool { false }
  func previewController(_ controller: QLPreviewController, editingModeFor previewItem: QLPreviewItem) -> QLPreviewItemEditingMode { .disabled }

  @objc private func showInfo() {
    guard let file else {
      let path = request["reference"] as? String ?? ""
      let alert = UIAlertController(title: tr("File reference", "文件位置"), message: path, preferredStyle: .alert)
      alert.addAction(UIAlertAction(title: tr("Copy path", "复制路径"), style: .default) { _ in UIPasteboard.general.string = path })
      alert.addAction(UIAlertAction(title: tr("Close", "关闭"), style: .cancel)); present(alert, animated: true); return
    }
    let time = DateFormatter.localizedString(from: Date(timeIntervalSince1970: Double(file.created_at)), dateStyle: .medium, timeStyle: .short)
    let alert = UIAlertController(title: file.name, message: "\(file.source)\n\n\(time)\n\(tr("Registered snapshot", "已提供的文件快照"))", preferredStyle: .alert)
    alert.addAction(UIAlertAction(title: tr("Copy path", "复制路径"), style: .default) { _ in UIPasteboard.general.string = file.source })
    alert.addAction(UIAlertAction(title: tr("Close", "关闭"), style: .cancel))
    present(alert, animated: true)
  }
  @objc private func shareFile() {
    guard let originalURL else { return }
    let share = UIActivityViewController(activityItems: [originalURL], applicationActivities: nil)
    share.popoverPresentationController?.barButtonItem = navigationItem.rightBarButtonItems?.first
    present(share, animated: true)
  }
  @objc private func closePreview() { closing = true; webView?.stopLoading(); transport?.cancel(); dismiss(animated: true) }
  override func viewDidDisappear(_ animated: Bool) {
    super.viewDidDisappear(animated)
    if isBeingDismissed || navigationController?.isBeingDismissed == true { closing = true; transport?.cleanup() }
  }
  deinit { transport?.cleanup() }
}
