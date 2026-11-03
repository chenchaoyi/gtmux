package com.gtmux.app

import android.view.HapticFeedbackConstants
import android.view.KeyEvent
import android.widget.EditText
import com.facebook.react.ReactPackage
import com.facebook.react.bridge.*
import com.facebook.react.modules.core.DeviceEventManagerModule
import com.facebook.react.uimanager.ViewManager
import java.io.File
import java.lang.ref.WeakReference
import org.json.JSONObject

class GtmuxPackage : ReactPackage {
  override fun createNativeModules(context: ReactApplicationContext): List<NativeModule> = buildList {
    add(HapticsModule(context)); add(KeyCommandsModule(context)); add(ClipboardImageModule(context))
    add(HQMemoryModule(context)); add(DocumentsModule(context)); add(PushNotificationsModule(context)); add(UsageWidgetsModule(context))
    if (BuildConfig.QA_TESTING) add(DebugSettingsModule(context))
  }
  override fun createViewManagers(context: ReactApplicationContext): List<ViewManager<*, *>> = emptyList()
}

class HapticsModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  override fun getName() = "Haptics"
  @ReactMethod fun prepare() = Unit
  @ReactMethod fun impact(kind: String) {
    reactApplicationContext.runOnUiQueueThread {
      reactApplicationContext.currentActivity?.window?.decorView?.performHapticFeedback(
        if (kind == "selection") HapticFeedbackConstants.CLOCK_TICK else HapticFeedbackConstants.LONG_PRESS
      ) // Respects the system haptic setting; no vibration permission or override.
    }
  }
}

class KeyCommandsModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  data class Binding(val id: String, val input: String, val modifiers: Set<String>)
  private var bindings = emptyList<Binding>()
  init { instance = WeakReference(this) }
  override fun getName() = "KeyCommands"
  @ReactMethod fun register(list: ReadableArray) {
    bindings = (0 until minOf(list.size(), 64)).mapNotNull { index ->
      val row = list.getMap(index) ?: return@mapNotNull null
      val id = row.getString("id") ?: return@mapNotNull null
      val input = row.getString("input") ?: return@mapNotNull null
      val mods = row.getArray("modifiers")?.toArrayList()?.filterIsInstance<String>()?.toSet() ?: emptySet()
      if (id.length > 64 || input.length > 16 || mods.any { it !in setOf("cmd", "ctrl", "alt", "shift") }) null
      else Binding(id, input, mods)
    }
  }
  @ReactMethod fun addListener(name: String) = Unit
  @ReactMethod fun removeListeners(count: Double) = Unit
  override fun invalidate() { bindings = emptyList(); super.invalidate() }
  private fun dispatch(activity: MainActivity, event: KeyEvent): Boolean {
    if (event.action != KeyEvent.ACTION_DOWN || event.repeatCount != 0) return false
    val mods = buildSet {
      if (event.isCtrlPressed) add("ctrl"); if (event.isMetaPressed) add("cmd")
      if (event.isAltPressed) add("alt"); if (event.isShiftPressed) add("shift")
    }
    // Never steal cursor keys, Return, Escape or composition from a text editor.
    if (activity.currentFocus is EditText && mods.none { it in setOf("ctrl", "cmd", "alt") }) return false
    val input = when (event.keyCode) {
      KeyEvent.KEYCODE_DPAD_UP -> "up"; KeyEvent.KEYCODE_DPAD_DOWN -> "down"
      KeyEvent.KEYCODE_DPAD_LEFT -> "left"; KeyEvent.KEYCODE_DPAD_RIGHT -> "right"
      KeyEvent.KEYCODE_ESCAPE -> "escape"; KeyEvent.KEYCODE_ENTER -> "enter"
      else -> event.getUnicodeChar(event.metaState and (KeyEvent.META_SHIFT_MASK or KeyEvent.META_ALT_MASK)).toChar().lowercase()
    }
    val binding = bindings.firstOrNull { it.input == input && it.modifiers == mods } ?: return false
    if (!reactApplicationContext.hasActiveReactInstance()) return false
    reactApplicationContext.getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter::class.java)
      .emit("onKeyCommand", Arguments.createMap().apply { putString("id", binding.id) })
    return true
  }
  companion object {
    private var instance = WeakReference<KeyCommandsModule>(null)
    fun dispatch(activity: MainActivity, event: KeyEvent) = instance.get()?.dispatch(activity, event) ?: false
  }
}

/** Included only in QA/debug. Flags are app-private and installed before JS starts. */
class DebugSettingsModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  override fun getName() = "DebugSettings"
  override fun getConstants(): Map<String, Any> {
    if (!BuildConfig.QA_TESTING) return emptyMap()
    val file = File(reactApplicationContext.filesDir, "gtmux-debug-flags.json")
    val flags = runCatching {
      require(file.length() <= 64 * 1024)
      val json = JSONObject(file.readText())
      json.keys().asSequence().filter { it.matches(Regex("GTMUX_DEBUG_[A-Z_]+")) }
        .associateWith { json.getString(it) }
    }.getOrDefault(emptyMap())
    if (flags["GTMUX_DEBUG_CLIPBOARD_IMAGE"] == "1") ClipboardImageModule.seedFixture(reactApplicationContext)
    return mapOf("flags" to flags)
  }
  @ReactMethod fun record(line: String) {
    if (!BuildConfig.QA_TESTING || line.length > 32768) return
    val file = File(reactApplicationContext.filesDir, "gtmux-debug.jsonl")
    if (file.length() < 1024 * 1024) file.appendText(line + "\n")
  }
  @ReactMethod fun reset() { if (BuildConfig.QA_TESTING) File(reactApplicationContext.filesDir, "gtmux-debug.jsonl").delete() }
}
