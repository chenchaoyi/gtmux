import AppKit
import Carbon.HIToolbox

/// GlobalHotkey registers a system-wide hotkey via Carbon (no Accessibility
/// permission needed, unlike NSEvent global monitors) and fires `action` on
/// press. Default binding: ⌘⌥G. Hold a reference for the app's lifetime.
final class GlobalHotkey {
    private var ref: EventHotKeyRef?
    private var handlerRef: EventHandlerRef?
    private let action: () -> Void

    /// keyCode is a Carbon virtual key (e.g. kVK_ANSI_G); modifiers are Carbon
    /// flags (cmdKey | optionKey | controlKey | shiftKey).
    init?(keyCode: UInt32, modifiers: UInt32, action: @escaping () -> Void) {
        self.action = action

        var eventType = EventTypeSpec(
            eventClass: OSType(kEventClassKeyboard),
            eventKind: OSType(kEventHotKeyPressed))
        let selfPtr = Unmanaged.passUnretained(self).toOpaque()

        let installed = InstallEventHandler(
            GetApplicationEventTarget(),
            { _, _, userData -> OSStatus in
                guard let userData = userData else { return OSStatus(eventNotHandledErr) }
                let me = Unmanaged<GlobalHotkey>.fromOpaque(userData).takeUnretainedValue()
                me.action()
                return noErr
            },
            1, &eventType, selfPtr, &handlerRef)
        guard installed == noErr else {
            dbg("hotkey: InstallEventHandler failed (\(installed))")
            return nil
        }

        let hotKeyID = EventHotKeyID(signature: OSType(0x47544D58), id: 1) // 'GTMX'
        let registered = RegisterEventHotKey(
            keyCode, modifiers, hotKeyID, GetApplicationEventTarget(), 0, &ref)
        guard registered == noErr else {
            dbg("hotkey: RegisterEventHotKey failed (\(registered))")
            return nil
        }
        dbg("hotkey: registered keyCode=\(keyCode) modifiers=\(modifiers)")
    }

    deinit {
        if let ref = ref { UnregisterEventHotKey(ref) }
        if let handlerRef = handlerRef { RemoveEventHandler(handlerRef) }
    }
}
