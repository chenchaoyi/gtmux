import UIKit

enum DocumentPreviewAction { case retry, copy, share, close }

struct DocumentStatusAction {
  let kind: DocumentPreviewAction
  let title: String
}

struct DocumentFailurePresentation {
  let title: String
  let message: String
  let actions: [DocumentStatusAction]

  static func make(_ reason: DocumentFailure, zh: Bool, copyTitle: String?, hasFile: Bool) -> Self {
    func tr(_ en: String, _ cn: String) -> String { zh ? cn : en }
    let retry = DocumentStatusAction(kind: .retry, title: tr("Retry", "重试"))
    let close = DocumentStatusAction(kind: .close, title: tr("Back to conversation", "返回会话"))
    let copy = copyTitle.map { DocumentStatusAction(kind: .copy, title: $0) }
    switch reason {
    case .permission:
      return Self(title: tr("Access denied", "无权查看文件"),
        message: tr("This connection does not have permission to view this file.", "当前连接没有此文件的查看权限。"), actions: [close])
    case .unavailable:
      return Self(title: tr("File unavailable", "文件暂不可用"),
        message: tr("The file is missing, was removed, or cannot be viewed from this conversation.", "文件不存在、已被移除，或不在当前会话的可查看范围内。"), actions: copy.map { [$0, retry] } ?? [retry])
    case .unsupported:
      return Self(title: tr("Server update required", "服务器需要更新"),
        message: tr("This server does not support file previews yet. Update gtmux on the server, then retry.", "此服务器还不支持文件预览。请更新服务器上的 gtmux 后重试。"), actions: [retry] + (copy.map { [$0] } ?? []))
    case .tooLarge:
      return Self(title: tr("File too large", "文件过大"),
        message: tr("File previews support files up to 30 MiB.", "文件预览支持 30 MiB 以内的文件。"), actions: copy.map { [$0, close] } ?? [close])
    case .invalid:
      return Self(title: tr("Could not verify file", "文件校验未完成"),
        message: tr("The file could not be read or verified. Reload it to try again.", "文件读取或校验未完成，请重新加载。"), actions: [retry] + (copy.map { [$0] } ?? []))
    case .format:
      let share = DocumentStatusAction(kind: .share, title: tr("Share original file", "分享原文件"))
      return Self(title: tr("Preview unavailable", "无法预览此文件"),
        message: tr("You can save the original file from the share sheet and open it in another app.", "可通过分享保存原文件，再用其他应用打开。"), actions: [hasFile ? share : retry] + (copy.map { [$0] } ?? []))
    case .network:
      return Self(title: tr("Could not load file", "无法加载文件"),
        message: tr("Check your connection, then try again.", "请检查连接后重试。"), actions: [retry] + (copy.map { [$0] } ?? []))
    }
  }
}

/// One scrollable reading column for transient progress and recoverable failures.
/// It grows beyond the viewport at accessibility sizes instead of truncating copy.
final class DocumentStatusView: UIView {
  var onAction: ((DocumentPreviewAction, UIButton) -> Void)?
  private let zh: Bool
  private let titleLabel = UILabel()
  private let messageLabel = UILabel()
  private let icon = UIImageView(image: UIImage(systemName: "doc"))
  private let spinner = UIActivityIndicatorView(style: .medium)
  private let progress = UIProgressView(progressViewStyle: .default)
  private let percentage = UILabel()
  private let progressStack = UIStackView()
  private let actionStack = UIStackView()
  private let primary = UIButton(type: .system)
  private let secondary = UIButton(type: .system)
  private var actions: [DocumentStatusAction] = []

  init(zh: Bool) {
    self.zh = zh
    super.init(frame: .zero)
    backgroundColor = .systemBackground
    accessibilityIdentifier = "document-preview-state"
    let scroll = UIScrollView()
    scroll.alwaysBounceVertical = false
    let canvas = UIView()
    let symbol = UIView()
    icon.tintColor = .secondaryLabel
    icon.contentMode = .scaleAspectFit
    icon.isAccessibilityElement = false
    spinner.hidesWhenStopped = true
    spinner.isAccessibilityElement = false
    symbol.addSubview(icon); symbol.addSubview(spinner)
    for label in [titleLabel, messageLabel, percentage] {
      label.numberOfLines = 0; label.textAlignment = .center
      label.adjustsFontForContentSizeCategory = true
      label.setContentCompressionResistancePriority(.required, for: .vertical)
    }
    titleLabel.font = UIFontMetrics(forTextStyle: .title3).scaledFont(for: .systemFont(ofSize: 20, weight: .semibold))
    titleLabel.textColor = .label
    titleLabel.accessibilityTraits.insert(.header)
    titleLabel.accessibilityIdentifier = "document-preview-state-title"
    messageLabel.font = .preferredFont(forTextStyle: .body)
    messageLabel.textColor = .secondaryLabel
    messageLabel.accessibilityIdentifier = "document-preview-state-message"
    percentage.font = .preferredFont(forTextStyle: .caption1)
    percentage.textColor = .secondaryLabel
    percentage.isAccessibilityElement = false
    progress.accessibilityLabel = tr("Download progress", "下载进度")
    progress.accessibilityTraits.insert(.updatesFrequently)
    progressStack.axis = .vertical; progressStack.spacing = 8
    progressStack.addArrangedSubview(progress); progressStack.addArrangedSubview(percentage)
    actionStack.axis = .vertical; actionStack.spacing = 4
    actionStack.addArrangedSubview(primary); actionStack.addArrangedSubview(secondary)
    for (index, button) in [primary, secondary].enumerated() {
      button.accessibilityIdentifier = index == 0 ? "document-preview-primary" : "document-preview-secondary"
      button.titleLabel?.numberOfLines = 0
      button.titleLabel?.adjustsFontForContentSizeCategory = true
      button.titleLabel?.textAlignment = .center
      button.tag = index
      button.addTarget(self, action: #selector(performAction(_:)), for: .touchUpInside)
      let height = button.heightAnchor.constraint(greaterThanOrEqualToConstant: 44)
      height.priority = UILayoutPriority(999); height.isActive = true
    }
    let stack = UIStackView(arrangedSubviews: [symbol, titleLabel, messageLabel, progressStack, actionStack])
    stack.axis = .vertical; stack.alignment = .center; stack.spacing = 12
    stack.setCustomSpacing(20, after: symbol)
    stack.setCustomSpacing(24, after: messageLabel)
    addSubview(scroll); scroll.addSubview(canvas); canvas.addSubview(stack)
    for view in [scroll, canvas, stack, icon, spinner, symbol, progressStack, actionStack] { view.translatesAutoresizingMaskIntoConstraints = false }
    let preferredWidth = stack.widthAnchor.constraint(equalTo: canvas.widthAnchor, constant: -48)
    preferredWidth.priority = .defaultHigh
    let actionWidth = actionStack.widthAnchor.constraint(equalTo: stack.widthAnchor)
    actionWidth.priority = .defaultLow
    NSLayoutConstraint.activate([
      scroll.leadingAnchor.constraint(equalTo: leadingAnchor), scroll.trailingAnchor.constraint(equalTo: trailingAnchor),
      scroll.topAnchor.constraint(equalTo: topAnchor), scroll.bottomAnchor.constraint(equalTo: bottomAnchor),
      canvas.leadingAnchor.constraint(equalTo: scroll.contentLayoutGuide.leadingAnchor), canvas.trailingAnchor.constraint(equalTo: scroll.contentLayoutGuide.trailingAnchor),
      canvas.topAnchor.constraint(equalTo: scroll.contentLayoutGuide.topAnchor), canvas.bottomAnchor.constraint(equalTo: scroll.contentLayoutGuide.bottomAnchor),
      canvas.widthAnchor.constraint(equalTo: scroll.frameLayoutGuide.widthAnchor), canvas.heightAnchor.constraint(greaterThanOrEqualTo: scroll.frameLayoutGuide.heightAnchor),
      stack.centerXAnchor.constraint(equalTo: canvas.centerXAnchor), stack.centerYAnchor.constraint(equalTo: canvas.centerYAnchor),
      stack.topAnchor.constraint(greaterThanOrEqualTo: canvas.topAnchor, constant: 24), stack.bottomAnchor.constraint(lessThanOrEqualTo: canvas.bottomAnchor, constant: -24),
      stack.widthAnchor.constraint(lessThanOrEqualToConstant: 420), stack.widthAnchor.constraint(lessThanOrEqualTo: canvas.widthAnchor, constant: -48), preferredWidth,
      symbol.heightAnchor.constraint(equalToConstant: 52), symbol.widthAnchor.constraint(equalToConstant: 52),
      icon.centerXAnchor.constraint(equalTo: symbol.centerXAnchor), icon.centerYAnchor.constraint(equalTo: symbol.centerYAnchor),
      icon.widthAnchor.constraint(equalToConstant: 40), icon.heightAnchor.constraint(equalToConstant: 48),
      spinner.centerXAnchor.constraint(equalTo: symbol.centerXAnchor), spinner.centerYAnchor.constraint(equalTo: symbol.centerYAnchor),
      titleLabel.widthAnchor.constraint(equalTo: stack.widthAnchor), messageLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
      progressStack.widthAnchor.constraint(equalTo: actionStack.widthAnchor),
      actionStack.widthAnchor.constraint(lessThanOrEqualToConstant: 280), actionStack.widthAnchor.constraint(greaterThanOrEqualToConstant: 160), actionWidth,
    ])
  }
  required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }
  private func tr(_ en: String, _ cn: String) -> String { zh ? cn : en }

  func loading(progress value: Double? = nil, opening: Bool = false) {
    actions = []; isHidden = false; icon.isHidden = true; spinner.startAnimating()
    titleLabel.text = opening ? tr("Opening file", "正在打开文件") : value == nil ? tr("Reading file", "正在读取文件") : tr("Downloading file", "正在下载文件")
    messageLabel.isHidden = true; primary.isHidden = true; secondary.isHidden = true; actionStack.isHidden = true
    progressStack.isHidden = value == nil; progress.isHidden = value == nil; percentage.isHidden = value == nil
    if let value {
      let bounded = min(1, max(0, value))
      progress.progress = Float(bounded); percentage.text = "\(Int(bounded * 100))%"
      progress.accessibilityValue = percentage.text
    }
  }

  func failure(_ presentation: DocumentFailurePresentation) {
    isHidden = false; spinner.stopAnimating(); icon.isHidden = false; progressStack.isHidden = true; progress.isHidden = true; percentage.isHidden = true
    titleLabel.text = presentation.title; messageLabel.text = presentation.message; messageLabel.isHidden = false
    actions = presentation.actions; actionStack.isHidden = false
    for (index, button) in [primary, secondary].enumerated() {
      button.isHidden = !actions.indices.contains(index)
      guard !button.isHidden else { continue }
      configure(button, title: actions[index].title)
    }
  }

  func copied(_ button: UIButton) {
    configure(button, title: tr("Copied", "已复制"))
    UIAccessibility.post(notification: .announcement, argument: tr("Copied", "已复制"))
  }

  private func configure(_ button: UIButton, title: String) {
    var configuration: UIButton.Configuration = button === primary ? .filled() : .plain()
    configuration.title = title; configuration.cornerStyle = .medium
    configuration.titleLineBreakMode = .byWordWrapping
    configuration.contentInsets = NSDirectionalEdgeInsets(top: 12, leading: 20, bottom: 12, trailing: 20)
    configuration.titleTextAttributesTransformer = UIConfigurationTextAttributesTransformer { incoming in
      var attributes = incoming; attributes.font = UIFont.preferredFont(forTextStyle: .body); return attributes
    }
    button.configuration = configuration
    button.accessibilityLabel = title
  }

  @objc private func performAction(_ sender: UIButton) {
    guard actions.indices.contains(sender.tag) else { return }
    onAction?(actions[sender.tag].kind, sender)
  }
}
