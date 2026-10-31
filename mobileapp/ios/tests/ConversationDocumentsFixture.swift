// Standalone, synthetic UIKit acceptance fixture; never part of the shipped app.
import UIKit
import WebKit

@main final class FixtureApp: UIResponder, UIApplicationDelegate {
  var window: UIWindow?
  func application(_ application: UIApplication, didFinishLaunchingWithOptions options: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
    window = UIWindow(frame: UIScreen.main.bounds)
    let arguments = ProcessInfo.processInfo.arguments
    if arguments.contains("--light") { window?.overrideUserInterfaceStyle = .light }
    if arguments.contains("--dark") { window?.overrideUserInterfaceStyle = .dark }
    if #available(iOS 17.0, *), arguments.contains("--large-text") { window?.traitOverrides.preferredContentSizeCategory = .accessibilityExtraExtraExtraLarge }
    window?.rootViewController = UINavigationController(rootViewController: FixtureController())
    window?.makeKeyAndVisible(); return true
  }
}

final class FixtureController: UIViewController {
  let selection = ConversationTextView()
  let result = UILabel()
  var findings: [String: String] = [:]
  var transports: [DocumentTransport] = []
  let args = ProcessInfo.processInfo.arguments
  var base: String { args.firstIndex(of: "--server").map { args[$0 + 1] } ?? "http://127.0.0.1:18739" }
  var lang: String { args.firstIndex(of: "--lang").map { args[$0 + 1] } ?? "zh" }
  var filename: String { args.firstIndex(of: "--name").map { args[$0 + 1] } ?? "Report.html" }
  var reference: String { args.firstIndex(of: "--reference").map { args[$0 + 1] } ?? "/fixture/report.html" }
  override func viewDidLoad() {
    super.viewDidLoad(); title = "Conversation document fixture"; view.backgroundColor = .systemBackground
    result.numberOfLines = 0; result.font = .preferredFont(forTextStyle: .body)
    let open = UIButton(type: .system); open.setTitle("Open report / 打开报告", for: .normal); open.addTarget(self, action: #selector(openReport), for: .touchUpInside)
    let stack = UIStackView(arrangedSubviews: [result, open, selection]); stack.axis = .vertical; stack.spacing = 20; stack.translatesAutoresizingMaskIntoConstraints = false
    view.addSubview(stack)
    NSLayoutConstraint.activate([stack.leadingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.leadingAnchor, constant: 20), stack.trailingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.trailingAnchor, constant: -20), stack.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 20), selection.heightAnchor.constraint(equalToConstant: 340)])
    selection.document = doc("1", "你好 😀，这是可拖动选择的文字。\n\nSecond paragraph with a link and **rich text**.")
    selection.onLink = { [weak self] event in self?.record("link", event["revision"] as? String ?? "missing") }
    if !args.contains("--skip-selection-checks") {
      DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { self.checkSelection() }
    }
    if args.contains("--text-layout-checks") {
      DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { self.checkTextLayout() }
    }
    if #available(iOS 16.0, *) {
      DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
        self.view.window?.windowScene?.requestGeometryUpdate(.iOS(interfaceOrientations: self.args.contains("--landscape") ? .landscapeRight : .portrait)) { error in
          self.record("orientation-error", error.localizedDescription)
        }
      }
    }
    if args.contains("--transport-checks") { runTransportChecks() }
    if args.contains("--preview") { DispatchQueue.main.asyncAfter(deadline: .now() + 1) { self.openReport() } }
  }
  func doc(_ revision: String, _ text: String) -> NSDictionary {
    ["revision": revision, "runs": [["text": text, "style": ["color": "#777777"]]], "style": ["fontSize": 18, "lineHeight": 28], "scale": 1]
  }
  func record(_ name: String, _ value: String) {
    findings[name] = value; result.text = findings.keys.sorted().map { "\($0): \(findings[$0]!)" }.joined(separator: "\n")
    let url = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("fixture-results.json")
    if let data = try? JSONSerialization.data(withJSONObject: findings, options: [.sortedKeys]) { try? data.write(to: url) }
  }
  func checkSelection() {
    let original = selection.textView.text! as NSString
    let range = original.range(of: "😀，这是")
    selection.textView.selectedRange = range; selection.textViewDidChangeSelection(selection.textView)
    selection.document = doc("2", "A new streaming response should wait for selection to finish.")
    record("selection-frozen", selection.textView.text == original as String ? "pass" : "FAIL")
    selection.textView.copy(nil)
    record("copy-range", UIPasteboard.general.string == "😀，这是" ? "pass" : "FAIL")
    selection.clearSelection()
    record("selection-resumes", selection.textView.text.hasPrefix("A new streaming") ? "pass" : "FAIL")
    selection.document = doc("3", "长按这里可以选择部分文字，然后拖动手柄调整。\n\nYou can select across these paragraphs, including 中文 😀 and links.")
  }
  func checkTextLayout() {
    let text = String(repeating: "这是一段完整的会话。刷新后应该保持完整排版，不能只显示首字。中文 😀 and English text. ", count: 8)
    let probe = ConversationTextView(frame: CGRect(x: 0, y: 0, width: 1, height: 28))
    var events: [[String: Any]] = []
    probe.onSize = { events.append($0) }
    probe.document = doc("layout-1", text)
    probe.layoutIfNeeded()
    record("text-unresolved-width", events.isEmpty ? "pass" : "FAIL published before usable width")
    probe.frame.size.width = 320; probe.setNeedsLayout(); probe.layoutIfNeeded()
    let wide = events.last?["height"] as? CGFloat ?? 0
    record("text-constrained-height", wide > 56 ? "pass" : "FAIL \(wide)")
    let count = events.count
    for _ in 0..<20 { probe.document = doc("layout-1", text); probe.layoutIfNeeded() }
    record("text-identical-refresh", events.count == count ? "pass" : "FAIL \(events.count - count) redundant measurements")
    probe.frame.size.width = 160; probe.setNeedsLayout(); probe.layoutIfNeeded()
    let narrow = events.last?["height"] as? CGFloat ?? 0
    record("text-resize", narrow > wide ? "pass" : "FAIL \(wide) -> \(narrow)")
    probe.document = doc("layout-2", text + " 新增进度。")
    probe.layoutIfNeeded()
    record("text-stream", probe.textView.text.hasSuffix("新增进度。") && (events.last?["height"] as? CGFloat ?? 0) >= narrow ? "pass" : "FAIL")
    let original = probe.textView.text! as NSString
    probe.textView.selectedRange = original.range(of: "中文 😀")
    probe.textViewDidChangeSelection(probe.textView)
    probe.document = doc("layout-3", "Selection finished; latest content is intact.")
    record("text-selection-pinned", probe.textView.text == original as String ? "pass" : "FAIL")
    probe.clearSelection()
    record("text-selection-resumes", probe.textView.text.hasPrefix("Selection finished") ? "pass" : "FAIL")
    let code: NSDictionary = ["revision": "code", "runs": [["text": String(repeating: "code", count: 40)]], "style": ["fontSize": 18, "lineHeight": 28], "scale": 1, "unbounded": true]
    probe.document = code; probe.frame.size.width = 1; probe.setNeedsLayout(); probe.layoutIfNeeded()
    record("text-unbounded-width", (events.last?["width"] as? CGFloat ?? 0) > 320 ? "pass" : "FAIL")
    selection.document = doc("visible", "这是一段完整的对话内容。\n\n刷新时正文应保持稳定，不会先缩成一个字，再展开。\n\nRich text and 中文 😀 remain selectable across paragraphs.")
  }
  func runTransportChecks() {
    for (path, expected) in [("redirect", "network"), ("denied", "permission"), ("unpublished", "unavailable"), ("old", "unsupported"), ("checksum", "invalid"), ("network", "network"), ("large", "tooLarge")] {
      let request: [String: Any] = ["resolveURL": base + "/api/artifacts/resolve", "headers": ["Authorization": "Bearer fixture-only"], "reference": "/fixture/" + path, "session_id": "fixture"]
      guard let transport = DocumentTransport(request: request) else { record(path, "FAIL init"); continue }
      transports.append(transport)
      transport.onFailure = { [weak self, weak transport] reason in
        self?.record(path, String(describing: reason) == expected ? "pass" : "FAIL \(reason)"); transport?.cleanup()
      }
      transport.onReady = { [weak self, weak transport] _, _ in self?.record(path, "FAIL opened"); transport?.cleanup() }
      transport.start()
    }
  }
  @objc func openReport() {
    let request: [String: Any] = ["resolveURL": base + "/api/artifacts/resolve", "headers": ["Authorization": "Bearer fixture-only"], "reference": reference, "session_id": "fixture", "home_id": "", "lang": lang, "name": filename, "allowScripts": true]
    let preview = DocumentPreviewController(request: request)
    let navigation = UINavigationController(rootViewController: preview)
    navigation.modalPresentationStyle = traitCollection.userInterfaceIdiom == .pad ? .formSheet : .fullScreen
    navigation.preferredContentSize = CGSize(width: 900, height: 800)
    present(navigation, animated: true)
    preview.loadViewIfNeeded()
    if args.contains("--state-checks") {
      func descendants(_ view: UIView) -> [UIView] { [view] + view.subviews.flatMap(descendants) }
      if let state = descendants(preview.view).compactMap({ $0 as? DocumentStatusView }).first {
        let action = state.onAction
        state.onAction = { [weak self, weak preview] kind, button in
          if case .copy = kind {
            action?(kind, button)
            self?.record("copy-reference", UIPasteboard.general.string == self?.reference ? "pass" : "FAIL")
          } else {
            action?(kind, button)
            if case .share = kind {
              DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
                guard let share = preview?.presentedViewController as? UIActivityViewController else {
                  self?.record("share-anchor", "FAIL no share controller"); return
                }
                self?.record("share-anchor", share.popoverPresentationController?.sourceView === button ? "pass" : "FAIL")
              }
            }
          }
        }
      }
    }
    if args.contains("--state-checks") {
      DispatchQueue.main.asyncAfter(deadline: .now() + 2) { self.auditStateLayout(preview) }
      if args.contains("--share-format-check") {
        DispatchQueue.main.asyncAfter(deadline: .now() + 3) {
          func descendants(_ view: UIView) -> [UIView] { [view] + view.subviews.flatMap(descendants) }
          if let button = descendants(preview.view).first(where: { $0.accessibilityIdentifier == "document-preview-primary" }) as? UIButton {
            button.sendActions(for: .touchUpInside)
          } else { self.record("share-anchor", "FAIL missing action") }
        }
      }
    }
    poll(preview, attempts: 30)
  }
  func checkMarkdownShare(_ preview: UIViewController, attempts: Int = 10) {
    DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
      if let share = preview.presentedViewController as? UIActivityViewController {
        self.record("markdown-share", share.popoverPresentationController?.barButtonItem === preview.navigationItem.rightBarButtonItems?.first ? "pass" : "FAIL anchor")
      } else if attempts > 0 { self.checkMarkdownShare(preview, attempts: attempts - 1) }
      else { self.record("markdown-share", "FAIL no controller") }
    }
  }
  func auditStateLayout(_ preview: UIViewController) {
    func descendants(_ view: UIView) -> [UIView] { [view] + view.subviews.flatMap(descendants) }
    let elements = descendants(preview.view)
    guard let state = elements.compactMap({ $0 as? DocumentStatusView }).first, !state.isHidden else { return }
    state.layoutIfNeeded()
    var failures: [String] = []
    for label in descendants(state).compactMap({ $0 as? UILabel }) where ["document-preview-state-title", "document-preview-state-message"].contains(label.accessibilityIdentifier ?? "") && !label.isHidden {
      let needed = ceil((label.text! as NSString).boundingRect(with: CGSize(width: label.bounds.width, height: .greatestFiniteMagnitude), options: [.usesLineFragmentOrigin], attributes: [.font: label.font!], context: nil).height)
      let frame = label.convert(label.bounds, to: state)
      if label.bounds.width <= 0 || needed > label.bounds.height + 1 || frame.minX < 0 || frame.maxX > state.bounds.width + 1 {
        failures.append("\(label.accessibilityIdentifier!): needed=\(needed), actual=\(label.bounds.height)")
      }
    }
    record("state-layout", failures.isEmpty ? "pass" : "FAIL " + failures.joined(separator: "; "))
  }
  func poll(_ preview: UIViewController, attempts: Int) {
    DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
      func descendants(_ view: UIView) -> [UIView] { [view] + view.subviews.flatMap(descendants) }
      if let web = descendants(preview.view).compactMap({ $0 as? WKWebView }).first {
        if self.args.contains("--html-image-checks") {
          web.evaluateJavaScript("JSON.stringify({images:[...document.images].filter(i=>i.complete&&i.naturalWidth>0).length,external:[...document.images].filter(i=>i.src.startsWith('https:')&&i.naturalWidth>0).length,inline:!!window.__imageFixtureScript,bridge:!!window.webkit?.messageHandlers,overflow:document.documentElement.scrollWidth>innerWidth})") { value, error in
            if let value = value as? String, let data = value.data(using:.utf8), let state = try? JSONSerialization.jsonObject(with:data) as? [String:Any], (state["images"] as? Int) == 2 {
              self.record("html-images", value)
              self.record("html-images-static", !web.configuration.defaultWebpagePreferences.allowsContentJavaScript ? "pass" : "FAIL")
              precondition(state["external"] as? Int == 0 && state["inline"] as? Bool == false && state["bridge"] as? Bool == false)
            } else if attempts > 0 { self.poll(preview, attempts: attempts-1) }
            else { self.record("html-images", "FAIL \(error?.localizedDescription ?? value as? String ?? "not loaded")") }
          }
          return
        }
        if self.args.contains("--markdown-checks") {
          web.evaluateJavaScript("JSON.stringify({headings:document.querySelectorAll('h1,h2').length, tables:document.querySelectorAll('table').length, strong:document.querySelectorAll('strong').length, code:document.querySelectorAll('pre code').length, images:[...document.images].filter(i=>i.complete&&i.naturalWidth>0).length, bridge:!!window.webkit?.messageHandlers, overflow:document.documentElement.scrollWidth>innerWidth})") { value, error in
            if let value = value as? String, let data = value.data(using: .utf8), let result = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
               (result["headings"] as? Int ?? 0) >= 3, (result["tables"] as? Int ?? 0) == 1, (result["images"] as? Int ?? 0) == 2 {
              self.record("markdown", value); self.record("markdown-scripts-disabled", web.configuration.defaultWebpagePreferences.allowsContentJavaScript ? "FAIL" : "pass")
              web.scrollView.setContentOffset(.zero, animated: false)
              if self.args.contains("--markdown-share-check") {
                preview.perform(NSSelectorFromString("shareFile")); self.checkMarkdownShare(preview)
              }
            } else if attempts > 0 {
              // Bring lazy images into view at accessibility text sizes before asserting.
              if web.scrollView.contentSize.height > web.scrollView.bounds.height {
                web.scrollView.setContentOffset(CGPoint(x: 0, y: web.scrollView.contentSize.height - web.scrollView.bounds.height), animated: false)
              }
              self.poll(preview, attempts: attempts - 1)
            }
            else { self.record("markdown", "FAIL \(error?.localizedDescription ?? value as? String ?? "not loaded")") }
          }
          return
        }
        web.evaluateJavaScript("JSON.stringify({inline:document.getElementById('inline')?.textContent, bridge:!!window.webkit?.messageHandlers, rows:document.querySelectorAll('tr').length})") { value, error in
          if let value = value as? String, value.contains("INLINE_OK") {
            self.record("html-interactive", value)
          } else if let value = value as? String, !web.configuration.defaultWebpagePreferences.allowsContentJavaScript, value.contains("LOADING") {
            self.record("html-static", value)
            if attempts > 0 { self.poll(preview, attempts: attempts - 1) }
          }
          else if attempts > 0 { self.poll(preview, attempts: attempts - 1) }
          else { self.record("html", "FAIL \(error?.localizedDescription ?? "not loaded")") }
        }
      } else if attempts > 0 { self.poll(preview, attempts: attempts - 1) }
      else {
        self.record("preview-status", descendants(preview.view).compactMap { ($0 as? UILabel)?.text }.joined(separator: " | "))
        self.record("preview-child-height", String(describing: preview.children.first?.view.bounds.height ?? -1))
      }
    }
  }
}
