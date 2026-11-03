package com.gtmux.app

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.pdf.PdfRenderer
import android.net.Uri
import android.os.Bundle
import android.os.ParcelFileDescriptor
import android.view.View
import android.view.ViewGroup
import android.webkit.*
import android.widget.*
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowCompat
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import com.facebook.react.bridge.*
import org.commonmark.ext.gfm.tables.TablesExtension
import org.commonmark.parser.Parser
import org.commonmark.renderer.html.HtmlRenderer
import org.jsoup.Jsoup
import java.io.ByteArrayInputStream
import java.util.concurrent.Executors
import java.util.concurrent.Semaphore
import java.util.concurrent.atomic.AtomicLong
import java.util.UUID

/** Requests stay in process memory, never an exported Intent, saved state or a WebView. */
object DocumentLaunches {
  private val requests = java.util.concurrent.ConcurrentHashMap<String, DocumentRequest>()
  fun add(request: DocumentRequest): String { val id = UUID.randomUUID().toString(); requests[id] = request; return id }
  fun take(id: String?) = if (id == null) null else requests.remove(id)
}

class DocumentsModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  override fun getName() = "GtmuxDocuments"
  @ReactMethod fun open(value: ReadableMap, promise: Promise) {
    try {
      val request = DocumentRequest.parse(org.json.JSONObject(value.toHashMap()))
      reactApplicationContext.runOnUiQueueThread {
        val activity = reactApplicationContext.currentActivity
        if (activity == null) { promise.reject("unavailable", "No active window"); return@runOnUiQueueThread }
        val id = DocumentLaunches.add(request)
        try { activity.startActivity(Intent(activity, DocumentActivity::class.java).putExtra("request", id)); promise.resolve(null) }
        catch (error: Exception) { DocumentLaunches.take(id); promise.reject("unavailable", "Could not open this document", error) }
      }
    } catch (error: Exception) { promise.reject("invalid", "Invalid document request", error) }
  }
}

class DocumentActivity : Activity() {
  private val worker = Executors.newSingleThreadExecutor()
  private lateinit var request: DocumentRequest
  private lateinit var transport: DocumentTransport
  private lateinit var root: LinearLayout
  private lateinit var content: FrameLayout
  private lateinit var title: TextView
  private lateinit var mode: Button
  private val iconButtons = mutableListOf<ImageButton>()
  private val dark get() = when (request.appearance) {"dark" -> true; "light" -> false; else -> resources.configuration.uiMode and 0x30 == 0x20}
  private var current: LocalDocument? = null
  private var web: WebView? = null
  private var pdf: PdfRenderer? = null
  private var scripts = false
  private var source = false
  private val images = java.util.concurrent.ConcurrentHashMap<String, String>()
  private val links = java.util.concurrent.ConcurrentHashMap<String, String>()
  private val imageCache = java.util.concurrent.ConcurrentHashMap<String, Pair<String, ByteArray>>()
  private val imageBudget = AtomicLong()
  private val imageSlots = Semaphore(2)
  private fun tr(en: String, zh: String) = if (request.lang == "zh") zh else en
  private fun dp(value: Int) = (resources.displayMetrics.density * value).toInt()
  override fun onCreate(state: Bundle?) {
    super.onCreate(state)
    val input = DocumentLaunches.take(intent.getStringExtra("request"))
    if (input == null) { finish(); return } // Recreated process has no credential-bearing saved state.
    request = input; transport = DocumentTransport(this)
    val background = if (dark) Color.rgb(17, 17, 20) else Color.WHITE
    val foreground = if (dark) Color.rgb(232, 232, 237) else Color.rgb(36, 38, 43)
    window.setBackgroundDrawable(android.graphics.drawable.ColorDrawable(background))
    root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setBackgroundColor(background) }
    // The toolbar takes its own layout space; it never covers the document's first pixels.
    ViewCompat.setOnApplyWindowInsetsListener(root) { view, insets ->
      val bars = insets.getInsets(WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout())
      view.setPadding(bars.left, bars.top, bars.right, bars.bottom); insets
    }
    val toolbar = LinearLayout(this).apply { gravity = android.view.Gravity.CENTER_VERTICAL; setPadding(dp(4), 0, dp(4), 0) }
    fun button(label: String, action: () -> Unit) = Button(this).apply { text = label; minWidth = dp(48); minHeight = dp(48); setTextColor(foreground); setOnClickListener { action() } }
    fun icon(drawable: Int, label: String, action: () -> Unit) = ImageButton(this).apply {
      layoutParams = LinearLayout.LayoutParams(dp(48), dp(48)); setImageResource(drawable); imageTintList = android.content.res.ColorStateList.valueOf(foreground)
      val value = android.util.TypedValue(); theme.resolveAttribute(android.R.attr.selectableItemBackgroundBorderless, value, true); setBackgroundResource(value.resourceId)
      contentDescription = label; setPadding(dp(12), dp(12), dp(12), dp(12)); setOnClickListener { action() }; iconButtons.add(this)
    }
    toolbar.addView(icon(R.drawable.document_close, tr("Close document", "关闭文件")) { finish() })
    title = TextView(this).apply { text = tr("Loading…", "正在加载…"); setTextColor(foreground); textSize = 16f; maxLines = 2; ellipsize = android.text.TextUtils.TruncateAt.MIDDLE }
    toolbar.addView(title, LinearLayout.LayoutParams(0, dp(56), 1f))
    mode = button(tr("Preview", "预览")) { toggleMode() }.apply { visibility = View.GONE }
    toolbar.addView(mode)
    toolbar.addView(icon(R.drawable.document_share, tr("Share document", "分享文件")) { share() })
    root.addView(toolbar)
    content = FrameLayout(this); root.addView(content, LinearLayout.LayoutParams(-1, 0, 1f)); setContentView(root)
    // API 35+ obtains the controller from DecorView. Attach content before asking for it.
    WindowCompat.getInsetsController(window, root).apply { isAppearanceLightStatusBars = !dark; isAppearanceLightNavigationBars = !dark }
    showStatus(tr("Loading…", "正在加载…"))
    worker.execute {
      try { val doc = transport.fetch(request); runOnUiThread { if (!isFinishing && !isDestroyed) { current = doc; title.text = doc.name; render() } } }
      catch (error: Exception) { runOnUiThread { if (!isFinishing) showStatus(failure(error.message)) } }
    }
  }
  private fun failure(code: String?) = when (code) {
    "permission" -> tr("This connection cannot open this file.", "当前连接无权打开此文件。")
    "tooLarge" -> tr("This file exceeds the preview limit.", "此文件超过预览大小限制。")
    "unavailable" -> tr("The file is no longer available on the server.", "服务器上的文件已不可用。")
    "unsupported" -> tr("Update the server to preview this file.", "请更新服务器后预览此文件。")
    else -> tr("Could not load this file. Check the connection and try again.", "文件未能加载，请检查连接后重试。")
  }
  private fun showStatus(message: String) {
    content.removeAllViews()
    content.addView(TextView(this).apply { text = message; textSize = 16f; gravity = android.view.Gravity.CENTER; setPadding(dp(24), dp(24), dp(24), dp(24)); setTextColor(if (dark) Color.LTGRAY else Color.DKGRAY) }, FrameLayout.LayoutParams(-1, -1))
  }
  private fun render() {
    val doc = current ?: return
    web?.destroy(); web = null; content.removeAllViews(); images.clear(); links.clear(); imageCache.clear(); imageBudget.set(0)
    mode.visibility = if (doc.kind == "html" || doc.kind == "markdown") View.VISIBLE else View.GONE
    mode.text = if (doc.kind == "markdown") { if (source) tr("Preview", "预览") else tr("Source", "原文") }
      else if (scripts) tr("Static", "静态") else tr("Interact", "交互")
    when (doc.kind) {
      "pdf" -> renderPDF(doc)
      "image" -> {
        val opts = BitmapFactory.Options().apply { inJustDecodeBounds = true }; BitmapFactory.decodeFile(doc.file.path, opts)
        if (!safeImage(opts)) { showStatus(failure("invalid")); return }
        val sample = maxOf(1, opts.outWidth / 2048, opts.outHeight / 4096)
        val image = BitmapFactory.decodeFile(doc.file.path, BitmapFactory.Options().apply { inSampleSize = sample })
        content.addView(ZoomImageView(this).apply { setImageBitmap(image); contentDescription = doc.name }, FrameLayout.LayoutParams(-1, -1))
      }
      else -> renderWeb(doc)
    }
  }
  private fun renderWeb(doc: LocalDocument) {
    val reader = WebView(this); web = reader
    val rendered = java.util.concurrent.atomic.AtomicReference<ByteArray?>()
    val documentURL = "$ORIGIN/document"
    reader.settings.apply {
      textZoom = (resources.configuration.fontScale * 100).toInt().coerceIn(80, 220)
      javaScriptEnabled = doc.kind == "html" && scripts
      allowFileAccess = false; allowContentAccess = false; domStorageEnabled = false
      javaScriptCanOpenWindowsAutomatically = false; setSupportMultipleWindows(false)
      mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW; setGeolocationEnabled(false)
      builtInZoomControls = true; displayZoomControls = false; loadsImagesAutomatically = true
      cacheMode = WebSettings.LOAD_NO_CACHE
    }
    CookieManager.getInstance().setAcceptThirdPartyCookies(reader, false)
    reader.setDownloadListener { _, _, _, _, _ -> Unit }
    reader.webViewClient = object : WebViewClient() {
      override fun onRenderProcessGone(view: WebView, detail: android.webkit.RenderProcessGoneDetail): Boolean {
        content.removeView(view)
        if (web === view) web = null
        view.destroy()
        if (!isFinishing && !isDestroyed) showStatus(tr("Preview stopped. Close and reopen the document.", "预览已停止，请关闭后重新打开文件。"))
        return true
      }
      override fun shouldOverrideUrlLoading(view: WebView, navigation: WebResourceRequest): Boolean {
        val url = navigation.url.toString()
        if (url == documentURL || url.startsWith("$documentURL#")) return false
        if (navigation.isForMainFrame && navigation.hasGesture()) runOnUiThread { openLink(url) }
        return true
      }
      override fun shouldInterceptRequest(view: WebView, resource: WebResourceRequest): WebResourceResponse {
        val key = resource.url.toString().substringBefore('#')
        if (resource.method == "GET" && resource.isForMainFrame && key == documentURL) {
          val bytes = rendered.get() ?: return blocked()
          return WebResourceResponse("text/html", "utf-8", ByteArrayInputStream(bytes))
        }
        return if (resource.method == "GET" && !resource.isForMainFrame && images.containsKey(key)) image(key, doc) else blocked()
      }
    }
    content.addView(reader, FrameLayout.LayoutParams(-1, -1))
    worker.execute {
      try {
        require(doc.file.length() <= 8L * 1024 * 1024) { "tooLarge" }
        val text = doc.file.readText(Charsets.UTF_8)
        val raw = when {
          doc.kind == "text" || source -> "<pre><code>${org.jsoup.nodes.Entities.escape(text)}</code></pre>"
          doc.kind == "markdown" -> {
            val extensions = listOf(TablesExtension.create())
            HtmlRenderer.builder().extensions(extensions).escapeHtml(true).sanitizeUrls(true).build()
              .render(Parser.builder().extensions(extensions).build().parse(text))
          }
          else -> text
        }
        val html = prepareHTML(raw, doc.kind == "html" && scripts)
        // Serve the one in-memory main document explicitly. loadDataWithBaseURL can
        // otherwise send its data: navigation through the deny-by-default interceptor.
        rendered.set(html.toByteArray(Charsets.UTF_8))
        runOnUiThread { if (!isFinishing && web === reader) reader.loadUrl(documentURL) }
      } catch (error: Exception) { runOnUiThread { if (!isFinishing) showStatus(failure(error.message)) } }
    }
  }
  private fun prepareHTML(raw: String, interactive: Boolean): String {
    val epoch = UUID.randomUUID().toString()
    val dom = Jsoup.parse(raw)
    if (request.appearance.isNotEmpty()) dom.selectFirst("html")?.attr("data-appearance", request.appearance)
    dom.select("base,meta[http-equiv],iframe,frame,frameset,object,embed,link,video,audio,source,script[src]").remove()
    if (!interactive) dom.select("script").remove()
    dom.select("img").forEach { img ->
      val ref = img.attr("src").ifBlank { img.attr("srcset").split(',').firstOrNull()?.trim()?.substringBefore(' ') ?: "" }
      img.removeAttr("srcset"); img.removeAttr("onerror"); img.removeAttr("onload")
      if (DocumentPolicy.localReference(ref) && images.size < 32) {
        val key = "$ORIGIN/image/$epoch/${images.size}"; images[key] = ref; img.attr("src", key)
      } else if (!(ref.startsWith("data:image/") && ref.length < 2 * 1024 * 1024)) {
        img.replaceWith(org.jsoup.nodes.TextNode(img.attr("alt").ifBlank { tr("Image unavailable", "图片不可用") }))
      }
    }
    dom.select("table").forEach { it.wrap("<div class=\"table-scroll\"></div>") }
    dom.select("a[href]").forEach { a ->
      val ref = a.attr("href")
      if (DocumentPolicy.localReference(ref) && !ref.startsWith('#')) {
        if (links.size < 256) { val key = "$ORIGIN/link/$epoch/${links.size}"; links[key] = ref; a.attr("href", key) } else a.removeAttr("href")
      } else if (!(ref.startsWith('#') || ref.startsWith("https://") || ref.startsWith("http://"))) a.removeAttr("href")
      a.removeAttr("target")
    }
    val css = resources.openRawResource(R.raw.document_reader).bufferedReader().use { it.readText() }.replace("__FONT_SIZE__", "17")
    val policy = "default-src 'none'; script-src ${if (interactive) "'unsafe-inline'" else "'none'"}; style-src 'unsafe-inline'; img-src data: blob: $ORIGIN; font-src data:; connect-src 'none'; frame-src 'none'; worker-src 'none'; object-src 'none'; media-src 'none'; base-uri 'none'; form-action 'none'"
    dom.head().prependElement("style").appendText(css)
    dom.head().prependElement("meta").attr("name", "viewport").attr("content", "width=device-width, initial-scale=1")
    dom.head().prependElement("meta").attr("http-equiv", "Content-Security-Policy").attr("content", policy)
    return dom.outerHtml()
  }
  private fun image(key: String, doc: LocalDocument): WebResourceResponse {
    imageCache[key]?.let { return WebResourceResponse(it.first, null, ByteArrayInputStream(it.second)) }
    if (!imageSlots.tryAcquire(30, java.util.concurrent.TimeUnit.SECONDS)) return blocked()
    try {
      imageCache[key]?.let { return WebResourceResponse(it.first, null, ByteArrayInputStream(it.second)) }
      val ref = images[key] ?: return blocked()
      if (imageBudget.get() >= 20L * 1024 * 1024) return blocked()
      val related = request.copy(reference = ref, parent = if (ref.startsWith("gtmux-file:")) null else doc.id)
      val value = transport.fetch(related, DocumentPolicy.IMAGE_BYTES)
      if (value.kind != "image") return blocked()
      val opts = BitmapFactory.Options().apply { inJustDecodeBounds = true }; BitmapFactory.decodeFile(value.file.path, opts)
      if (!safeImage(opts)) return blocked()
      val bytes = value.file.readBytes()
      if (imageBudget.addAndGet(bytes.size.toLong()) > 20L * 1024 * 1024) return blocked()
      imageCache[key] = Pair(value.media, bytes)
      return WebResourceResponse(value.media, null, ByteArrayInputStream(bytes))
    } catch (_: Exception) { return blocked() } finally { imageSlots.release() }
  }
  private fun safeImage(opts: BitmapFactory.Options) = DocumentPolicy.imageDimensions(opts.outWidth, opts.outHeight)
  private fun blocked() = WebResourceResponse("text/plain", "utf-8", 404, "Not available", emptyMap(), ByteArrayInputStream(ByteArray(0)))
  private fun openLink(url: String) {
    val ref = links[url]
    if (ref != null) {
      val next = request.copy(reference = ref, parent = if (ref.startsWith("gtmux-file:")) null else current?.id)
      startActivity(Intent(this, DocumentActivity::class.java).putExtra("request", DocumentLaunches.add(next))); return
    }
    val uri = Uri.parse(url)
    if (uri.scheme !in setOf("http", "https") || uri.host == "gtmux-document.invalid") return
    AlertDialog.Builder(this).setTitle(tr("Open in browser?", "在浏览器中打开？")).setMessage(uri.host)
      .setNegativeButton(tr("Cancel", "取消"), null).setPositiveButton(tr("Open", "打开")) { _, _ -> runCatching { startActivity(Intent(Intent.ACTION_VIEW, uri)) } }.show()
  }
  private fun toggleMode() {
    if (current?.kind == "markdown") { source = !source; render(); return }
    if (scripts) { scripts = false; render(); return }
    AlertDialog.Builder(this).setTitle(tr("Enable interaction?", "开启交互？"))
      .setMessage(tr("This runs scripts from this file. Enable it only for a file you trust. Network access remains blocked.", "这会运行文件中的脚本，请只对信任的文件开启。网络访问仍被阻止。"))
      .setNegativeButton(tr("Cancel", "取消"), null).setPositiveButton(tr("Enable", "开启")) { _, _ -> scripts = true; render() }.show()
  }
  private fun share() {
    val doc = current ?: return
    worker.execute {
      try {
        val uri = SharedFiles.export(this, doc.file, doc.name)
        runOnUiThread {
          if (isFinishing || isDestroyed) return@runOnUiThread
          runCatching { startActivity(Intent.createChooser(Intent(Intent.ACTION_SEND).setType(doc.media).putExtra(Intent.EXTRA_STREAM, uri).putExtra(Intent.EXTRA_TITLE, doc.name).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION), tr("Share document", "分享文件"))) }
            .onFailure { Toast.makeText(this, tr("Could not share this file.", "暂时无法分享此文件。"), Toast.LENGTH_LONG).show() }
        }
      } catch (_: Exception) { runOnUiThread { if (!isFinishing) Toast.makeText(this, tr("Could not share this file.", "暂时无法分享此文件。"), Toast.LENGTH_LONG).show() } }
    }
  }
  private fun renderPDF(doc: LocalDocument) {
    try {
      pdf?.close(); val renderer = PdfRenderer(ParcelFileDescriptor.open(doc.file, ParcelFileDescriptor.MODE_READ_ONLY)); pdf = renderer
      val list = RecyclerView(this); list.layoutManager = LinearLayoutManager(this)
      list.isVerticalScrollBarEnabled = true; list.scrollBarStyle = View.SCROLLBARS_INSIDE_OVERLAY
      list.adapter = object : RecyclerView.Adapter<PageHolder>() {
        override fun getItemCount() = renderer.pageCount
        override fun onCreateViewHolder(parent: ViewGroup, type: Int) = PageHolder(ImageView(this@DocumentActivity).apply { adjustViewBounds = true; scaleType = ImageView.ScaleType.FIT_CENTER; layoutParams = ViewGroup.LayoutParams(-1, -2) })
        override fun onBindViewHolder(holder: PageHolder, position: Int) {
          renderer.openPage(position).use { page ->
            val width = minOf(1600, resources.displayMetrics.widthPixels); val height = (page.height.toLong() * width / page.width).toInt().coerceAtMost(4096)
            val bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888); bitmap.eraseColor(Color.WHITE)
            page.render(bitmap, null, null, PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY)
            holder.bitmap?.recycle(); holder.bitmap = bitmap; holder.image.setImageBitmap(bitmap)
            holder.image.contentDescription = tr("Page ${position + 1} of ${renderer.pageCount}", "第 ${position + 1} 页，共 ${renderer.pageCount} 页")
          }
        }
        override fun onViewRecycled(holder: PageHolder) { holder.image.setImageDrawable(null); holder.bitmap?.recycle(); holder.bitmap = null }
      }
      content.addView(list)
    } catch (_: Exception) { showStatus(failure("invalid")) }
  }
  class PageHolder(val image: ImageView) : RecyclerView.ViewHolder(image) { var bitmap: Bitmap? = null }
  override fun onDestroy() {
    web?.stopLoading(); web?.destroy(); pdf?.close(); worker.shutdownNow()
    if (::transport.isInitialized) transport.close()
    imageCache.clear(); super.onDestroy()
  }
  companion object { private const val ORIGIN = "https://gtmux-document.invalid" }
}
