import AppKit
import Carbon.HIToolbox

/// GlobalHotkey registers a system-wide hotkey via Carbon (no Accessibility
/// permission needed, unlike NSEvent global monitors) and fires `action` on
/// press. Bindings: ⌘⌥G (palette), ⌥⌘4 (screenshot). Hold a reference for the
/// app's lifetime.
///
/// Every key shares ONE application event handler, which dispatches by the hotkey's
/// id. It used to be one handler per instance that ran its action for ANY hotkey
/// press, with every id hard-coded to 1 — harmless with one key, and with a second
/// key both actions would have fired on either press.
final class GlobalHotkey {
    private static let signature = OSType(0x47544D58) // 'GTMX'
    private static var handlerRef: EventHandlerRef?
    private static var actions: [UInt32: () -> Void] = [:]
    private static var nextID: UInt32 = 1

    private var ref: EventHotKeyRef?
    private let id: UInt32

    /// keyCode is a Carbon virtual key (e.g. kVK_ANSI_G); modifiers are Carbon
    /// flags (cmdKey | optionKey | controlKey | shiftKey).
    init?(keyCode: UInt32, modifiers: UInt32, action: @escaping () -> Void) {
        guard Self.installHandler() else { return nil }
        id = Self.register(action)
        let hotKeyID = EventHotKeyID(signature: Self.signature, id: id)
        let registered = RegisterEventHotKey(
            keyCode, modifiers, hotKeyID, GetApplicationEventTarget(), 0, &ref)
        guard registered == noErr else {
            dbg("hotkey: RegisterEventHotKey failed (\(registered))")
            Self.actions[id] = nil
            return nil
        }
        dbg("hotkey: registered id=\(id) keyCode=\(keyCode) modifiers=\(modifiers)")
    }

    deinit {
        if let ref = ref { UnregisterEventHotKey(ref) }
        Self.actions[id] = nil
    }

    /// Adds an action to the dispatch table and returns its id. Internal so a test can
    /// exercise dispatch without registering a real system-wide key.
    static func register(_ action: @escaping () -> Void) -> UInt32 {
        let id = nextID
        nextID += 1
        actions[id] = action
        return id
    }

    /// Runs the action registered under `id`; false when there is none.
    @discardableResult
    static func dispatch(_ id: UInt32) -> Bool {
        guard let action = actions[id] else { return false }
        action()
        return true
    }

    private static func installHandler() -> Bool {
        if handlerRef != nil { return true }
        var eventType = EventTypeSpec(
            eventClass: OSType(kEventClassKeyboard),
            eventKind: OSType(kEventHotKeyPressed))
        let installed = InstallEventHandler(
            GetApplicationEventTarget(),
            { _, event, _ -> OSStatus in
                guard let event else { return OSStatus(eventNotHandledErr) }
                var hk = EventHotKeyID()
                let got = GetEventParameter(
                    event, EventParamName(kEventParamDirectObject), EventParamType(typeEventHotKeyID),
                    nil, MemoryLayout<EventHotKeyID>.size, nil, &hk)
                guard got == noErr, hk.signature == GlobalHotkey.signature else {
                    return OSStatus(eventNotHandledErr)
                }
                return GlobalHotkey.dispatch(hk.id) ? noErr : OSStatus(eventNotHandledErr)
            },
            1, &eventType, nil, &handlerRef)
        guard installed == noErr else {
            dbg("hotkey: InstallEventHandler failed (\(installed))")
            return false
        }
        return true
    }
}
