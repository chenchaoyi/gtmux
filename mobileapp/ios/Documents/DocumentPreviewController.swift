import UIKit
import QuickLook
import WebKit
import ImageIO
import SafariServices

/// Preview UI has no pairing credentials, application bridge or shared web login.
final class DocumentPreviewController: UIViewController, QLPreviewControllerDataSource, QLPreviewControllerDelegate, WKNavigationDelegate, WKUIDelegate {
  private let request: [String: Any]
  private var transport: DocumentTransport?
  private var file: ConversationDocument?
  private var originalURL: URL?
  private var htmlURL: URL?
  private var webView: WKWebView?
  private var markdownResources: DocumentResources?
  private var markdownLinks: [String: String] = [:]
  private var markdownSource = false
  private let content = UIView()
  private let status = UILabel()
  private lazy var state = DocumentStatusView(zh: zh)
  private let readyHeader = UIView()
  private let controls = UIStackView()
  private var shareButton: UIBarButtonItem!
  private var infoButton: UIBarButtonItem!
  private let filename = UILabel()
  private let interaction = UIButton(type: .system)
  private var authorizedScriptHash: String?
  private var renderID = UUID()
  private var closing = false
  private var downloadFinished = false
  private var zh: Bool { request["lang"] as? String == "zh" }
  private func tr(_ en: String, _ cn: String) -> String { zh ? cn : en }

  init(request: [String: Any]) { self.request = request; super.init(nibName: nil, bundle: nil) }
  required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

  override func viewDidLoad() {
    super.viewDidLoad()
    view.backgroundColor = .systemBackground
    filename.font = .preferredFont(forTextStyle: .headline)
    filename.adjustsFontForContentSizeCategory = true
    filename.lineBreakMode = .byTruncatingMiddle
    filename.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
    filename.accessibilityIdentifier = "document-preview-filename"
    navigationItem.titleView = filename
    setFilename((request["name"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? tr("File preview", "文件预览"))
    navigationItem.leftBarButtonItem = UIBarButtonItem(barButtonSystemItem: .close, target: self, action: #selector(closePreview))
    shareButton = UIBarButtonItem(barButtonSystemItem: .action, target: self, action: #selector(shareFile))
    infoButton = UIBarButtonItem(image: UIImage(systemName: "info.circle"), style: .plain, target: self, action: #selector(showInfo))
    navigationItem.rightBarButtonItems = [infoButton]
    navigationItem.leftBarButtonItem?.accessibilityIdentifier = "document-preview-close"
    navigationItem.leftBarButtonItem?.accessibilityLabel = tr("Close preview", "关闭预览")
    shareButton.accessibilityIdentifier = "document-preview-share"
    shareButton.accessibilityLabel = tr("Share file", "分享文件")
    infoButton.accessibilityIdentifier = "document-preview-info"
    infoButton.accessibilityLabel = tr("File information", "文件信息")
    status.numberOfLines = 0
    status.font = .preferredFont(forTextStyle: .caption1)
    status.textColor = .secondaryLabel
    status.adjustsFontForContentSizeCategory = true
    interaction.setTitle(tr("Enable interaction", "开启交互"), for: .normal)
    interaction.accessibilityIdentifier = "document-preview-interaction"
    interaction.titleLabel?.font = .preferredFont(forTextStyle: .subheadline)
    interaction.titleLabel?.adjustsFontForContentSizeCategory = true
    interaction.titleLabel?.numberOfLines = 0
    interaction.titleLabel?.textAlignment = .center
    interaction.addTarget(self, action: #selector(toggleInteraction), for: .touchUpInside)
    controls.addArrangedSubview(status); controls.addArrangedSubview(interaction)
    controls.alignment = .center; controls.spacing = 12
    let line = UIView(); line.backgroundColor = .separator
    readyHeader.addSubview(controls); readyHeader.addSubview(line)
    for child in [controls, line] { child.translatesAutoresizingMaskIntoConstraints = false }
    let interactionHeight = interaction.heightAnchor.constraint(greaterThanOrEqualToConstant: 44)
    interactionHeight.priority = UILayoutPriority(999)
    let headerTop = controls.topAnchor.constraint(equalTo: readyHeader.topAnchor, constant: 4)
    let headerBottom = controls.bottomAnchor.constraint(equalTo: readyHeader.bottomAnchor, constant: -8)
    headerTop.priority = UILayoutPriority(999); headerBottom.priority = UILayoutPriority(999)
    NSLayoutConstraint.activate([
      controls.leadingAnchor.constraint(equalTo: readyHeader.leadingAnchor, constant: 16),
      controls.trailingAnchor.constraint(equalTo: readyHeader.trailingAnchor, constant: -16),
      headerTop, headerBottom,
      line.leadingAnchor.constraint(equalTo: readyHeader.leadingAnchor), line.trailingAnchor.constraint(equalTo: readyHeader.trailingAnchor),
      line.bottomAnchor.constraint(equalTo: readyHeader.bottomAnchor), line.heightAnchor.constraint(equalToConstant: 0.5), interactionHeight,
    ])
    content.setContentHuggingPriority(UILayoutPriority(1), for: .vertical)
    let stack = UIStackView(arrangedSubviews: [readyHeader, content])
    stack.axis = .vertical; stack.translatesAutoresizingMaskIntoConstraints = false
    view.addSubview(stack)
    NSLayoutConstraint.activate([
      stack.leadingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.leadingAnchor),
      stack.trailingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.trailingAnchor),
      stack.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
      stack.bottomAnchor.constraint(equalTo: view.safeAreaLayoutGuide.bottomAnchor),
      content.heightAnchor.constraint(greaterThanOrEqualToConstant: 1),
    ])
    content.addSubview(state); state.translatesAutoresizingMaskIntoConstraints = false
    NSLayoutConstraint.activate([state.leadingAnchor.constraint(equalTo: content.leadingAnchor), state.trailingAnchor.constraint(equalTo: content.trailingAnchor), state.topAnchor.constraint(equalTo: content.topAnchor), state.bottomAnchor.constraint(equalTo: content.bottomAnchor)])
    state.onAction = { [weak self] action, button in
      guard let self, !self.closing else { return }
      switch action {
      case .retry: self.start()
      case .copy:
        guard let reference = self.copyReference else { return }
        UIPasteboard.general.string = reference
        self.state.copied(button)
      case .share: self.presentShare(from: button)
      case .close: self.closePreview()
      }
    }
    updateHeaderLayout()
    start()
  }

  @objc private func start() {
    guard !closing else { return }
    transport?.cleanup(); downloadFinished = false
    authorizedScriptHash = nil; renderID = UUID(); interaction.isHidden = true; markdownSource = false
    navigationItem.rightBarButtonItems = [infoButton]
    clearContent()
    originalURL = nil; file = nil; htmlURL = nil
    readyHeader.isHidden = true; state.loading()
    guard let next = DocumentTransport(request: request) else { showFailure(.invalid); return }
    transport = next
    next.onProgress = { [weak self, weak next] value in
      guard let self, !self.closing, !self.downloadFinished, self.transport === next else { return }
      self.state.loading(progress: value)
    }
    next.onFailure = { [weak self, weak next] reason in
      guard let self, !self.closing, self.transport === next else { return }; self.showFailure(reason)
    }
    next.onReady = { [weak self, weak next] meta, url in
      guard let self, !self.closing, self.transport === next else { return }; self.show(meta, url: url)
    }
    next.start()
  }

  private var copyReference: String? {
    let reference = file?.source ?? request["reference"] as? String ?? ""
    return reference.isEmpty ? nil : reference
  }
  private var copyTitle: String? {
    copyReference.map { $0.hasPrefix("gtmux-file:") ? tr("Copy file link", "复制文件链接") : tr("Copy path", "复制路径") }
  }
  private func setFilename(_ value: String) {
    title = value; filename.text = value; filename.accessibilityLabel = value
  }
  private func clearContent() {
    renderID = UUID()
    markdownResources?.cleanup(); markdownResources = nil; markdownLinks = [:]
    let old = webView; webView = nil
    old?.stopLoading(); old?.navigationDelegate = nil; old?.uiDelegate = nil
    for child in children { child.willMove(toParent: nil); child.view.removeFromSuperview(); child.removeFromParent() }
    content.subviews.filter { $0 !== state }.forEach { $0.removeFromSuperview() }
  }
  private func showFailure(_ reason: DocumentFailure) {
    guard !closing else { return }
    downloadFinished = true; clearContent(); authorizedScriptHash = nil
    readyHeader.isHidden = true; interaction.isHidden = true
    state.failure(.make(reason, zh: zh, copyTitle: copyTitle, hasFile: originalURL != nil))
  }
  private func updateHeaderLayout() {
    controls.axis = traitCollection.preferredContentSizeCategory.isAccessibilityCategory ? .vertical : .horizontal
    controls.alignment = controls.axis == .vertical ? .leading : .center
  }
  override func traitCollectionDidChange(_ previousTraitCollection: UITraitCollection?) {
    super.traitCollectionDidChange(previousTraitCollection)
    if isViewLoaded { updateHeaderLayout() }
  }

  private func show(_ file: ConversationDocument, url: URL) {
    guard !closing else { return }
    downloadFinished = true
    self.file = file; originalURL = url; setFilename(file.name)
    navigationItem.rightBarButtonItems = [shareButton, infoButton]
    readyHeader.isHidden = false
    updateFileStatus()
    switch file.kind {
    case "html": interaction.isHidden = false; showHTML(url)
    case "markdown": interaction.isHidden = false; showMarkdown(url)
    case "text": showText(url)
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

  private func previewText(_ source: URL) throws -> String {
    let handle = try FileHandle(forReadingFrom: source); defer { try? handle.close() }
    let bytes = try handle.read(upToCount: 512 * 1024) ?? Data()
    if let file, file.bytes > bytes.count { status.text = tr("Previewing the first 512 KiB · Share saves the complete file", "仅预览前 512 KiB · 分享可保存完整文件") }
    return String(decoding: bytes, as: UTF8.self)
  }
  private func showText(_ source: URL) {
    do {
      let text = UITextView(); text.isEditable = false; text.isSelectable = true
      text.font = UIFontMetrics(forTextStyle: .body).scaledFont(for: .monospacedSystemFont(ofSize: 15, weight: .regular))
      text.adjustsFontForContentSizeCategory = true; text.backgroundColor = .clear
      text.textContainerInset = UIEdgeInsets(top: 16, left: 16, bottom: 24, right: 16)
      text.accessibilityIdentifier = "document-preview-source-text"
      text.text = try previewText(source); mount(text)
    } catch { showFailure(.format) }
  }
  private func showMarkdown(_ source: URL) {
    guard let file else { return }
    do {
      let result = try DocumentMarkdown.render(previewText(source), fontSize: Double(UIFont.preferredFont(forTextStyle: .body, compatibleWith: traitCollection).pointSize))
      markdownLinks = result.links
      let resources = DocumentResources(references: result.images, request: request, parentID: file.id)
      markdownResources = resources
      showHTML(source, renderedHTML: result.html, resources: resources)
    } catch { showFailure(.format) }
  }

  private func mount(_ child: UIView, ready: Bool = true) {
    child.translatesAutoresizingMaskIntoConstraints = false; content.insertSubview(child, belowSubview: state)
    NSLayoutConstraint.activate([child.leadingAnchor.constraint(equalTo: content.leadingAnchor), child.trailingAnchor.constraint(equalTo: content.trailingAnchor), child.topAnchor.constraint(equalTo: content.topAnchor), child.bottomAnchor.constraint(equalTo: content.bottomAnchor)])
    if ready { state.isHidden = true }
  }

  private var scriptsAllowed: Bool { file != nil && authorizedScriptHash == file?.sha256 }

  private func updateFileStatus() {
    guard let file else { return }
    let kind = ["html": "HTML", "pdf": "PDF", "markdown": "Markdown", "text": tr("Text", "文本"), "image": tr("Image", "图片")][file.kind] ?? file.kind
    let mode = file.kind == "markdown" ? (markdownSource ? tr(" · Source", " · 原文") : tr(" · Preview", " · 预览")) : file.kind == "html" ? (scriptsAllowed ? tr(" · Interactive", " · 已开启交互") : tr(" · Static preview", " · 静态预览")) : ""
    status.text = "\(kind) · \(ByteCountFormatter.string(fromByteCount: file.bytes, countStyle: .file))\(mode)"
    interaction.setTitle(file.kind == "markdown" ? (markdownSource ? tr("Show preview", "查看预览") : tr("Show source", "查看原文")) : scriptsAllowed ? tr("Use static preview", "关闭交互") : tr("Enable interaction", "开启交互"), for: .normal)
    interaction.accessibilityIdentifier = file.kind == "markdown" ? "document-preview-source" : "document-preview-interaction"
  }

  @objc private func toggleInteraction() {
    if file?.kind == "markdown", let source = originalURL {
      markdownSource.toggle(); clearContent(); updateFileStatus()
      if markdownSource { interaction.isEnabled = true; showText(source) } else { showMarkdown(source) }
      return
    }
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

  private func showHTML(_ source: URL, renderedHTML: String? = nil, resources: DocumentResources? = nil) {
    interaction.isEnabled = false; state.loading(opening: true)
    do {
      let generation = UUID(); renderID = generation
      let allowScripts = file?.kind == "html" && scriptsAllowed
      let html = try renderedHTML ?? String(contentsOf: source, encoding: .utf8)
      let policy = "default-src 'none'; script-src \(allowScripts ? "'unsafe-inline'" : "'none'"); style-src 'unsafe-inline'; img-src data: blob: gtmux-document:; font-src data:; connect-src 'none'; frame-src 'none'; worker-src 'none'; object-src 'none'; media-src 'none'; base-uri 'none'; form-action 'none'"
      let secured = "<!doctype html><meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"\(policy)\">" + html
      let rendered = source.deletingLastPathComponent().appendingPathComponent("Rendered", isDirectory: true)
      try FileManager.default.createDirectory(at: rendered, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
      let target = rendered.appendingPathComponent("preview.html")
      try secured.write(to: target, atomically: true, encoding: .utf8)
      try FileManager.default.setAttributes([.posixPermissions: 0o600, .protectionKey: FileProtectionType.complete], ofItemAtPath: target.path)
      htmlURL = target
      let configuration = WKWebViewConfiguration()
      configuration.websiteDataStore = .nonPersistent()
      if let resources { configuration.setURLSchemeHandler(resources, forURLScheme: "gtmux-document") }
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
          web.accessibilityIdentifier = self.file?.kind == "markdown" ? "document-preview-markdown" : "document-preview-html"
          self.webView = web; self.mount(web, ready: false)
          web.loadFileURL(target, allowingReadAccessTo: target.deletingLastPathComponent())
        }
      }
    } catch { showFailure(.format) }
  }

  func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
    guard !closing, self.webView === webView else { return }
    state.isHidden = true; interaction.isEnabled = true
  }
  func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction,
               decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
    guard !closing, self.webView === webView else { decisionHandler(.cancel); return }
    let url = navigationAction.request.url
    if file?.kind == "markdown", navigationAction.navigationType == .linkActivated,
       let url, let target = markdownLinks[url.absoluteString] {
      decisionHandler(.cancel); openMarkdownLink(target); return
    }
    let ownDocument = url?.isFileURL == true && url?.standardizedFileURL.path == htmlURL?.standardizedFileURL.path
    decisionHandler(ownDocument && navigationAction.targetFrame?.isMainFrame == true && !navigationAction.shouldPerformDownload ? .allow : .cancel)
  }
  private func openMarkdownLink(_ target: String) {
    guard let file, let url = URL(string: target) else { return }
    if ["http", "https"].contains(url.scheme?.lowercased() ?? "") {
      present(SFSafariViewController(url: url), animated: true); return
    }
    if url.scheme?.lowercased() == "mailto" { UIApplication.shared.open(url); return }
    var related = request; related["reference"] = target; related["name"] = url.lastPathComponent
    related["parent_id"] = target.hasPrefix("gtmux-file:") ? nil : file.id
    navigationController?.pushViewController(DocumentPreviewController(request: related), animated: true)
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
      if let copyTitle { alert.addAction(UIAlertAction(title: copyTitle, style: .default) { _ in UIPasteboard.general.string = path }) }
      alert.addAction(UIAlertAction(title: tr("Close", "关闭"), style: .cancel)); present(alert, animated: true); return
    }
    let time = DateFormatter.localizedString(from: Date(timeIntervalSince1970: Double(file.created_at)), dateStyle: .medium, timeStyle: .short)
    let alert = UIAlertController(title: file.name, message: "\(file.source)\n\n\(time)\n\(tr("Preview copy", "本次预览的文件副本"))", preferredStyle: .alert)
    alert.addAction(UIAlertAction(title: tr("Copy path", "复制路径"), style: .default) { _ in UIPasteboard.general.string = file.source })
    alert.addAction(UIAlertAction(title: tr("Close", "关闭"), style: .cancel))
    present(alert, animated: true)
  }
  @objc private func shareFile() { presentShare() }
  private func presentShare(from source: UIView? = nil) {
    guard !closing, let originalURL else { return }
    let share = UIActivityViewController(activityItems: [originalURL], applicationActivities: nil)
    if let source {
      share.popoverPresentationController?.sourceView = source
      share.popoverPresentationController?.sourceRect = source.bounds
    } else { share.popoverPresentationController?.barButtonItem = shareButton }
    present(share, animated: true)
  }
  @objc private func closePreview() { closing = true; markdownResources?.cleanup(); webView?.stopLoading(); transport?.cancel(); if let navigationController, navigationController.viewControllers.first !== self { navigationController.popViewController(animated: true) } else { dismiss(animated: true) } }
  override func viewDidDisappear(_ animated: Bool) {
    super.viewDidDisappear(animated)
    if isBeingDismissed || isMovingFromParent || navigationController?.isBeingDismissed == true { closing = true; markdownResources?.cleanup(); transport?.cleanup() }
  }
  deinit { markdownResources?.cleanup(); transport?.cleanup() }
}
