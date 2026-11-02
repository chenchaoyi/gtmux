package com.gtmux.app

import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.widget.RemoteViews
import com.facebook.react.bridge.*
import org.json.JSONArray
import org.json.JSONObject

/** Credential-free, last-seen snapshot. The foreground parent owns all authenticated requests. */
class UsageWidgetsModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  override fun getName() = "UsageWidgetsModule"
  @ReactMethod fun configure(owner: String?, promise: Promise) {
    val prefs = reactApplicationContext.getSharedPreferences("usage-widget", Context.MODE_PRIVATE)
    if (owner == null || owner != prefs.getString("owner", null)) {
      prefs.edit().clear().apply()
      if (owner != null && owner.length <= 4096) prefs.edit().putString("owner", owner).apply()
      UsageWidget.refresh(reactApplicationContext)
    }
    promise.resolve(null)
  }
  @ReactMethod fun updateSnapshot(value: ReadableMap, promise: Promise) {
    try {
      val prefs = reactApplicationContext.getSharedPreferences("usage-widget", Context.MODE_PRIVATE)
      require(value.getString("owner") == prefs.getString("owner", null) && value.getString("owner") != null)
      val name = value.getString("name") ?: "gtmux"
      require(name.length <= 120 && name.none { it.code < 32 })
      val input = value.getArray("windows") ?: Arguments.createArray()
      val windows = JSONArray()
      for (i in 0 until minOf(input.size(), 6)) {
        val row = input.getMap(i) ?: continue; val label = row.getString("label") ?: continue; val used = row.getDouble("used")
        require(label.length <= 120 && label.none { it.code < 32 } && used.isFinite() && used in 0.0..100.0)
        windows.put(JSONObject().put("label", label).put("used", used))
      }
      prefs.edit().putString("snapshot", JSONObject().put("name", name).put("at", System.currentTimeMillis()).put("windows", windows).toString()).apply()
      UsageWidget.refresh(reactApplicationContext); promise.resolve(null)
    } catch (_: Exception) { promise.reject("widget_snapshot_invalid", "Invalid widget snapshot") }
  }
}

class UsageWidget : AppWidgetProvider() {
  override fun onUpdate(context: Context, manager: AppWidgetManager, ids: IntArray) { ids.forEach { update(context, manager, it) } }
  companion object {
    fun refresh(context: Context) {
      val manager = AppWidgetManager.getInstance(context)
      manager.getAppWidgetIds(ComponentName(context, UsageWidget::class.java)).forEach { update(context, manager, it) }
    }
    private fun update(context: Context, manager: AppWidgetManager, id: Int) {
      val prefs = context.getSharedPreferences("usage-widget", Context.MODE_PRIVATE)
      val snapshot = runCatching { JSONObject(prefs.getString("snapshot", "") ?: "") }.getOrNull()
      val views = RemoteViews(context.packageName, R.layout.usage_widget)
      val zh = context.resources.configuration.locales[0].language == "zh"
      val owner = prefs.getString("owner", null)
      views.setTextViewText(R.id.widget_server, snapshot?.optString("name") ?: "gtmux")
      val at = snapshot?.optLong("at") ?: 0
      val stamp = if (at > 0) android.text.format.DateFormat.getTimeFormat(context).format(java.util.Date(at)) else ""
      views.setTextViewText(R.id.widget_updated, if (at > 0) (if (zh) "上次打开 · $stamp" else "Last seen · $stamp") else if (zh) "打开 app 查看用量" else "Open app for usage")
      val rows = snapshot?.optJSONArray("windows")
      val text = if (owner == null) { if (zh) "请先连接自己的服务器" else "Connect to your server" }
        else if (rows == null || rows.length() == 0) { if (zh) "暂无套餐用量" else "No quota data" }
        else (0 until minOf(3, rows.length())).joinToString("\n") { index -> val row = rows.getJSONObject(index); "${row.getString("label")}  ${row.getDouble("used").toInt()}%" }
      views.setTextViewText(R.id.widget_quotas, text)
      val intent = Intent(context, MainActivity::class.java).setAction(Intent.ACTION_VIEW).setData(Uri.parse("gtmux://usage")).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
      views.setOnClickPendingIntent(R.id.widget_root, PendingIntent.getActivity(context, 8601, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE))
      manager.updateAppWidget(id, views)
    }
  }
}
