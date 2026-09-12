//
//  KeyCommands — hardware-keyboard commands for the iPad (change ipad-universal-app, D7).
//
//  React Native has no hardware-key API on iOS, so this is the one bridge: JS registers
//  the keymap (id · key · modifiers · title) once, this module installs it into the app's
//  main menu through UIMenuBuilder, and iPadOS does the rest — dispatch through the
//  responder chain, and the ⌘-hold overlay drawn from the titles. A press comes back to
//  JS as an event carrying the command id; what the id DOES lives in JS (keys/keymap.ts),
//  never here. Text fields keep their own keys: a bare arrow inside the composer is text
//  navigation, because nothing here asks for priority over system behaviour.
//

import Foundation
import React
import UIKit

struct KeyBinding {
  let id: String
  let input: String
  let modifiers: UIKeyModifierFlags
  let title: String
}

@objc(KeyCommands)
class KeyCommands: RCTEventEmitter {
  static var shared: KeyCommands?
  static var bindings: [KeyBinding] = []

  override init() {
    super.init()
    KeyCommands.shared = self
  }

  @objc override static func requiresMainQueueSetup() -> Bool { true }
  override func supportedEvents() -> [String]! { ["onKeyCommand"] }
  private var listening = false
  override func startObserving() { listening = true }
  override func stopObserving() { listening = false }

  /// register replaces the keymap. Each entry: {id, input, modifiers: [cmd|shift|ctrl|alt], title}.
  @objc(register:)
  func register(_ list: NSArray) {
    var out: [KeyBinding] = []
    for case let e as NSDictionary in list {
      guard let id = e["id"] as? String, let input = e["input"] as? String, let title = e["title"] as? String else { continue }
      var flags: UIKeyModifierFlags = []
      for case let m as String in (e["modifiers"] as? NSArray) ?? [] {
        switch m {
        case "cmd": flags.insert(.command)
        case "shift": flags.insert(.shift)
        case "ctrl": flags.insert(.control)
        case "alt": flags.insert(.alternate)
        default: break
        }
      }
      out.append(KeyBinding(id: id, input: KeyCommands.inputFor(input), modifiers: flags, title: title))
    }
    DispatchQueue.main.async {
      KeyCommands.bindings = out
      UIMenuSystem.main.setNeedsRebuild()
    }
  }

  /// The named keys JS spells by word; anything else is the literal character.
  static func inputFor(_ name: String) -> String {
    switch name {
    case "up": return UIKeyCommand.inputUpArrow
    case "down": return UIKeyCommand.inputDownArrow
    case "left": return UIKeyCommand.inputLeftArrow
    case "right": return UIKeyCommand.inputRightArrow
    case "escape": return UIKeyCommand.inputEscape
    case "enter", "return": return "\r"
    default: return name
    }
  }

  func emit(_ id: String) {
    guard listening else { return }
    sendEvent(withName: "onKeyCommand", body: ["id": id])
  }

  /// The menu the app delegate builds: every binding under one "gtmux" menu, so the
  /// ⌘-hold overlay groups them and the responder chain can reach the delegate's action.
  static func install(into builder: UIMenuBuilder) {
    // A probe the e2e reads back: did the menu get built, and with how many commands?
    shared?.emit("__menu.built." + String(bindings.count))
    guard !bindings.isEmpty else { return }
    let commands: [UIKeyCommand] = bindings.map { b in
      let c = UIKeyCommand(title: b.title, action: #selector(AppDelegate.gtmuxKeyCommand(_:)),
                           input: b.input, modifierFlags: b.modifiers, propertyList: b.id)
      return c
    }
    let menu = UIMenu(title: "gtmux", identifier: UIMenu.Identifier("dev.gtmux.keys"), options: [], children: commands)
    builder.insertSibling(menu, afterMenu: .edit)
  }

  /// The same commands as responder key commands (the pre-menu mechanism). UIKit matches
  /// a hardware press against the menu AND the responder chain; keeping both means a
  /// press is dispatched even where the menu system is not consulted (some simulator /
  /// automation paths), while the menu still feeds the ⌘-hold overlay.
  static func responderCommands() -> [UIKeyCommand] {
    bindings.map { b in
      let c = UIKeyCommand(title: b.title, action: #selector(AppDelegate.gtmuxKeyCommand(_:)),
                           input: b.input, modifierFlags: b.modifiers, propertyList: b.id)
      return c
    }
  }
}
