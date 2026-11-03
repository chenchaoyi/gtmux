package com.gtmux.app

import android.Manifest
import android.app.*
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import com.facebook.react.bridge.*
import com.facebook.react.modules.core.DeviceEventManagerModule
import com.facebook.react.modules.core.PermissionAwareActivity
import com.facebook.react.modules.core.PermissionListener
import com.google.android.gms.common.GoogleApiAvailabilityLight
import com.google.firebase.FirebaseApp
import com.google.firebase.messaging.FirebaseMessaging
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import org.json.JSONObject
import java.util.UUID
import java.lang.ref.WeakReference

class PushNotificationsModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context), ActivityEventListener {
  init { instance = WeakReference(this); context.addActivityEventListener(this) }
  override fun getName() = "PushNotifications"
  private fun ready() = BuildConfig.FIREBASE_CONFIGURED && GoogleApiAvailabilityLight.getInstance().isGooglePlayServicesAvailable(reactApplicationContext) == 0
  @ReactMethod fun configure(names: ReadableArray) {
    val owners = (0 until minOf(names.size(), 64)).mapNotNull { names.getString(it) }.filter { it.isNotBlank() && it.length <= 256 }.toSet()
    val preferences = reactApplicationContext.getSharedPreferences("push-owners", Context.MODE_PRIVATE)
    val previous = preferences.getStringSet("names", emptySet()) ?: emptySet()
    preferences.edit().putStringSet("names", owners).apply()
    val removed = previous - owners
    contextNotifications().activeNotifications.filter { notification -> removed.any { notification.tag?.startsWith("$it/") == true } }.forEach { contextNotifications().cancel(it.tag, it.id) }
    if (owners.isEmpty()) contextNotifications().cancelAll()
  }
  private fun contextNotifications() = reactApplicationContext.getSystemService(NotificationManager::class.java)
  @ReactMethod fun status(promise: Promise) {
    val granted = NotificationManagerCompat.from(reactApplicationContext).areNotificationsEnabled()
    promise.resolve(Arguments.createMap().apply {
      putBoolean("configured", BuildConfig.FIREBASE_CONFIGURED); putBoolean("available", ready()); putBoolean("granted", granted)
      putString("reason", if (!BuildConfig.FIREBASE_CONFIGURED) "not-configured" else if (!ready()) "google-services-unavailable" else if (!granted) "permission-denied" else "ready")
    })
  }
  @ReactMethod fun token(askPermission: Boolean, promise: Promise) {
    if (!ready()) { promise.resolve(null); return }
    reactApplicationContext.runOnUiQueueThread {
      val activity = reactApplicationContext.currentActivity
      val getToken = {
        if (!NotificationManagerCompat.from(reactApplicationContext).areNotificationsEnabled()) promise.resolve(null)
        else try {
          if (FirebaseApp.getApps(reactApplicationContext).isEmpty()) FirebaseApp.initializeApp(reactApplicationContext)
          FirebaseMessaging.getInstance().isAutoInitEnabled = true
          FirebaseMessaging.getInstance().token.addOnSuccessListener { promise.resolve(it) }
            .addOnFailureListener { promise.reject("push_unavailable", "Could not register Android notifications") }
        } catch (_: Exception) { promise.resolve(null) }
      }
      if (Build.VERSION.SDK_INT >= 33 && reactApplicationContext.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
        if (!askPermission || activity !is PermissionAwareActivity) { promise.resolve(null); return@runOnUiQueueThread }
        activity.requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 8416, PermissionListener { code, _, result ->
          if (code != 8416) false else { if (result.firstOrNull() == PackageManager.PERMISSION_GRANTED) getToken() else promise.resolve(null); true }
        })
      } else getToken()
    }
  }
  @ReactMethod fun initial(promise: Promise) { promise.resolve(consume(reactApplicationContext.currentActivity?.intent)) }
  @ReactMethod fun badge(n: Double) = Unit // Android launchers derive badge dots from active notifications.
  @ReactMethod fun addListener(name: String) = Unit
  @ReactMethod fun removeListeners(count: Double) = Unit
  override fun onActivityResult(activity: Activity, requestCode: Int, resultCode: Int, data: Intent?) = Unit
  override fun onNewIntent(intent: Intent) { consume(intent)?.let { emit("onPushTap", it) } }
  private fun consume(intent: Intent?): WritableMap? {
    val nonce = intent?.getStringExtra("gtmux_notification_tap") ?: return null
    intent.removeExtra("gtmux_notification_tap")
    val prefs = reactApplicationContext.getSharedPreferences("notification-taps", Context.MODE_PRIVATE)
    val raw = prefs.getString(nonce, null) ?: return null
    prefs.edit().remove(nonce).apply()
    return runCatching {
      val json = JSONObject(raw)
      require(System.currentTimeMillis() - json.getLong("at") in 0..15 * 60 * 1000L)
      require(json.getString("server") in (reactApplicationContext.getSharedPreferences("push-owners", Context.MODE_PRIVATE).getStringSet("names", emptySet()) ?: emptySet()))
      Arguments.createMap().apply { putString("pane", json.getString("pane")); putString("server", json.getString("server")) }
    }.getOrNull()
  }
  private fun emit(name: String, value: WritableMap) {
    if (reactApplicationContext.hasActiveReactInstance()) reactApplicationContext.getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter::class.java).emit(name, value)
  }
  override fun invalidate() { reactApplicationContext.removeActivityEventListener(this); super.invalidate() }
  companion object {
    private var instance = WeakReference<PushNotificationsModule>(null)
    fun newToken(token: String) { instance.get()?.emit("onPushToken", Arguments.createMap().apply { putString("token", token) }) }
  }
}

class GtmuxMessagingService : FirebaseMessagingService() {
  override fun onNewToken(token: String) { if (BuildConfig.FIREBASE_CONFIGURED) PushNotificationsModule.newToken(token) }
  override fun onMessageReceived(message: RemoteMessage) {
    if (BuildConfig.FIREBASE_CONFIGURED) show(this, message.data)
  }
  companion object {
    fun show(context: Context, data: Map<String, String>) {
      val pane = data["pane"] ?: return; val server = data["server"] ?: return
      if (server.isBlank() || server.length > 256) return
      if (server !in (context.getSharedPreferences("push-owners", Context.MODE_PRIVATE).getStringSet("names", emptySet()) ?: emptySet())) return
      val manager = context.getSystemService(NotificationManager::class.java)
      val key = "$server/$pane"
      if (data["silent"] == "true") {
        if (pane.matches(Regex("%[0-9]+"))) manager.cancel(key, 1)
        else if (data["badge"] == "0") manager.activeNotifications.filter { it.tag?.startsWith("$server/") == true }.forEach { manager.cancel(it.tag, it.id) }
        return
      }
      if (!pane.matches(Regex("%[0-9]+")) && pane != "gtmux-test") return
      if (data["kind"] !in setOf("waiting", "done")) return
      if (!NotificationManagerCompat.from(context).areNotificationsEnabled()) return
      val channel = "gtmux-agent-status"
      if (Build.VERSION.SDK_INT >= 26) manager.createNotificationChannel(NotificationChannel(channel, "Agent updates", NotificationManager.IMPORTANCE_DEFAULT))
      val prefs = context.getSharedPreferences("notification-taps", Context.MODE_PRIVATE)
      val now = System.currentTimeMillis(); val editor = prefs.edit()
      // One-use private tap records, at most 64 retained, expiring after 15 minutes.
      prefs.all.entries.sortedBy { it.key }.dropLast(63).forEach { editor.remove(it.key) }
      val nonce = UUID.randomUUID().toString()
      editor.putString(nonce, JSONObject().put("pane", pane).put("server", server).put("at", now).toString()).apply()
      val intent = Intent(context, MainActivity::class.java).setAction("gtmux.notification.$nonce").putExtra("gtmux_notification_tap", nonce).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
      val tap = PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
      val notification = NotificationCompat.Builder(context, channel).setSmallIcon(R.drawable.ic_notification)
        .setContentTitle((data["title"] ?: "gtmux").take(256)).setContentText((data["body"] ?: "").take(2048))
        .setSubText(server).setStyle(NotificationCompat.BigTextStyle().bigText((data["body"] ?: "").take(2048)))
        .setContentIntent(tap).setAutoCancel(true).setOnlyAlertOnce(true).setNumber(data["badge"]?.toIntOrNull()?.coerceIn(0, 9999) ?: 0)
        // Opening the current prompt is safe across pane reuse. Background numeric actions need an exact prompt identity contract.
        .addAction(0, if (context.resources.configuration.locales[0].language == "zh") "查看" else "Open", tap).build()
      try { manager.notify(key, 1, notification) } catch (_: SecurityException) { }
    }
  }
}
