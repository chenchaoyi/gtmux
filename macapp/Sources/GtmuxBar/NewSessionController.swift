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
            contentRect: NSRect(x: 0, y: 0, width: 400, height: 170),
            styleMask: [.titled, .closable], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.delegate = self
        let content = NSView()
        window.contentView = content

        let label = NSTextField(labelWithString: l10n.tr("Session name (optional)", "会话名称（可选）"))
        label.font = .systemFont(ofSize: 13, weight: .semibold)
        let field = NSTextField()
        field.font = .systemFont(ofSize: 13)
        field.placeholderString = l10n.tr("For example, project name", "例如：项目名")
        field.setAccessibilityIdentifier("new-session-name")
        field.setAccessibilityLabel(l10n.tr("Session name", "会话名称"))
        let hint = NSTextField(labelWithString: l10n.tr(
            "Leave blank and tmux will name it.", "留空时由 tmux 自动命名。"))
        hint.font = .systemFont(ofSize: 11)
        hint.textColor = .secondaryLabelColor
        let cancel = NSButton(title: l10n.tr("Cancel", "取消"), target: self,
                              action: #selector(cancelClicked))
        cancel.keyEquivalent = "\u{1b}"
        let create = NSButton(title: l10n.tr("Create", "创建"), target: self,
                              action: #selector(createClicked))
        create.keyEquivalent = "\r"

        for view in [label, field, hint, cancel, create] {
            view.translatesAutoresizingMaskIntoConstraints = false
            content.addSubview(view)
        }
        NSLayoutConstraint.activate([
            label.topAnchor.constraint(equalTo: content.topAnchor, constant: 20),
            label.leadingAnchor.constraint(equalTo: content.leadingAnchor, constant: 22),
            field.topAnchor.constraint(equalTo: label.bottomAnchor, constant: 8),
            field.leadingAnchor.constraint(equalTo: label.leadingAnchor),
            field.trailingAnchor.constraint(equalTo: content.trailingAnchor, constant: -22),
            field.heightAnchor.constraint(equalToConstant: 28),
            hint.topAnchor.constraint(equalTo: field.bottomAnchor, constant: 6),
            hint.leadingAnchor.constraint(equalTo: label.leadingAnchor),
            hint.trailingAnchor.constraint(lessThanOrEqualTo: field.trailingAnchor),
            create.trailingAnchor.constraint(equalTo: field.trailingAnchor),
            create.bottomAnchor.constraint(equalTo: content.bottomAnchor, constant: -18),
            create.widthAnchor.constraint(greaterThanOrEqualToConstant: 80),
            cancel.trailingAnchor.constraint(equalTo: create.leadingAnchor, constant: -8),
            cancel.centerYAnchor.constraint(equalTo: create.centerYAnchor),
            cancel.widthAnchor.constraint(greaterThanOrEqualToConstant: 80),
            hint.bottomAnchor.constraint(lessThanOrEqualTo: create.topAnchor, constant: -12),
        ])
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
