import AppKit

/// A compact, keyboard-first prompt for `gtmux new`. NSAlert's accessory-view
/// layout grows into a large centred dialog on current macOS and does not reliably
/// give its text field the insertion point. A normal small window keeps the label,
/// field and actions in one reading order and makes the field first responder only
/// after the window becomes key.
final class NewSessionController: NSObject, NSWindowDelegate {
    static let shared = NewSessionController()

    private var window: NSWindow?
    private var nameField: NSTextField?
    private var onCreate: ((String) -> Void)?

    static func arguments(for name: String) -> [String] {
        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? ["new"] : ["new", trimmed]
    }

    func show(l10n: L10n, onCreate: @escaping (String) -> Void) {
        // Rebuild the small window so labels follow the current app language.
        window?.close()
        self.onCreate = onCreate
        build(l10n: l10n)
        guard let window, let nameField else { return }
        window.title = l10n.tr("New session", "新建会话")
        nameField.stringValue = ""
        window.center()
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
        window.makeFirstResponder(nameField)
        nameField.selectText(nil)
    }

    private func build(l10n: L10n) {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 380, height: 112),
            styleMask: [.titled, .closable], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.delegate = self
        let content = NSView()
        window.contentView = content

        let label = NSTextField(labelWithString: l10n.tr("Name", "名称"))
        label.font = .systemFont(ofSize: 13, weight: .medium)
        let field = NSTextField()
        field.font = .systemFont(ofSize: 13)
        field.placeholderString = l10n.tr("Automatic", "自动命名")
        field.bezelStyle = .roundedBezel
        field.toolTip = l10n.tr("Enter a name, or leave blank to name the session automatically.", "输入会话名称，留空则自动命名。")
        field.setAccessibilityIdentifier("new-session-name")
        field.setAccessibilityLabel(l10n.tr("Session name", "会话名称"))
        let cancel = NSButton(title: l10n.tr("Cancel", "取消"), target: self,
                              action: #selector(cancelClicked))
        cancel.keyEquivalent = "\u{1b}"
        let create = NSButton(title: l10n.tr("Create", "创建"), target: self,
                              action: #selector(createClicked))
        create.keyEquivalent = "\r"
        for button in [cancel, create] { button.bezelStyle = .rounded }
        cancel.setAccessibilityIdentifier("new-session-cancel")
        create.setAccessibilityIdentifier("new-session-create")
        field.nextKeyView = cancel
        cancel.nextKeyView = create
        create.nextKeyView = field

        for view in [label, field, cancel, create] {
            view.translatesAutoresizingMaskIntoConstraints = false
            content.addSubview(view)
        }
        NSLayoutConstraint.activate([
            field.topAnchor.constraint(equalTo: content.topAnchor, constant: 22),
            label.centerYAnchor.constraint(equalTo: field.centerYAnchor),
            label.leadingAnchor.constraint(equalTo: content.leadingAnchor, constant: 22),
            field.leadingAnchor.constraint(equalTo: label.trailingAnchor, constant: 12),
            field.trailingAnchor.constraint(equalTo: content.trailingAnchor, constant: -22),
            field.heightAnchor.constraint(equalToConstant: 26),
            create.trailingAnchor.constraint(equalTo: field.trailingAnchor),
            create.topAnchor.constraint(equalTo: field.bottomAnchor, constant: 20),
            create.bottomAnchor.constraint(equalTo: content.bottomAnchor, constant: -18),
            create.widthAnchor.constraint(greaterThanOrEqualToConstant: 72),
            cancel.trailingAnchor.constraint(equalTo: create.leadingAnchor, constant: -8),
            cancel.centerYAnchor.constraint(equalTo: create.centerYAnchor),
            cancel.widthAnchor.constraint(greaterThanOrEqualToConstant: 72),
        ])
        label.setContentHuggingPriority(.required, for: .horizontal)
        label.setContentCompressionResistancePriority(.required, for: .horizontal)
        // Derive height from the native control metrics instead of reserving empty space.
        window.setContentSize(NSSize(width: 380, height: content.fittingSize.height))
        window.initialFirstResponder = field
        self.window = window
        nameField = field
    }

    @objc private func cancelClicked() {
        window?.close()
    }

    @objc private func createClicked() {
        guard let nameField else { return }
        let name = nameField.stringValue
        let callback = onCreate
        window?.close()
        callback?(name)
    }

    func windowWillClose(_ notification: Notification) {
        window = nil
        nameField = nil
        onCreate = nil
    }
}
