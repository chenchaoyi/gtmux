import AppKit

/// SwiftUI's AX nodes implement informal Objective-C accessors rather than the full
/// AppKit accessibility protocol. Read only selectors the node actually exposes.
struct NativeUIElement {
    let object: NSObject
    private func value(_ key: String) -> Any? {
        guard object.responds(to: NSSelectorFromString(key)) else { return nil }
        return object.value(forKey: key)
    }
    func accessibilityIdentifier() -> String? { value("accessibilityIdentifier") as? String }
    func accessibilityLabel() -> String? { value("accessibilityLabel") as? String }
    func accessibilityValue() -> Any? { value("accessibilityValue") }
    func accessibilityChildren() -> [Any] { value("accessibilityChildren") as? [Any] ?? [] }
    func accessibilityFrame() -> NSRect { (value("accessibilityFrame") as? NSValue)?.rectValue ?? .zero }
    func accessibilityPerformPress() -> Bool {
        let selector = NSSelectorFromString("accessibilityPerformPress")
        guard object.responds(to: selector) else { return false }
        typealias Press = @convention(c) (AnyObject, Selector) -> Bool
        return unsafeBitCast(object.method(for: selector), to: Press.self)(object, selector)
    }
}

/// In-process AX and geometry only: no installed app, device or VoiceOver speech.
enum NativeUIProbe {
    static func elements(_ root: Any) -> [NativeUIElement] {
        var result: [NativeUIElement] = []
        var seen: Set<ObjectIdentifier> = []
        func visit(_ value: Any) {
            guard let object = value as? NSObject, seen.insert(ObjectIdentifier(object)).inserted else { return }
            let node = NativeUIElement(object: object)
            result.append(node)
            node.accessibilityChildren().forEach(visit)
            if let view = object as? NSView { view.subviews.forEach(visit) }
            if let window = object as? NSWindow, let content = window.contentView?.superview { visit(content) }
        }
        visit(root)
        return result
    }
    static func find(_ id: String, in root: Any) -> NativeUIElement? {
        // Prefer the leaf control over NSToolbar's reparenting proxy, whose press
        // accessor can return false after forwarding the action to its child.
        elements(root).last { $0.accessibilityIdentifier() == id }
    }

    static func prepareAccessibility() {
        NSApplication.shared.setValue(true, forKey: "accessibilityEnhancedUserInterface")
    }

    static func views<T: NSView>(_ type: T.Type, in root: NSView) -> [T] {
        (root as? T).map { [$0] } ?? root.subviews.flatMap { views(type, in: $0) }
    }
}
