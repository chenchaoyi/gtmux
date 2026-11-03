package com.gtmux.app

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.graphics.BitmapFactory
import android.net.Uri
import com.facebook.react.bridge.*
import java.io.File
import java.util.UUID
import java.util.concurrent.Executors

/** Reads only an explicit image Paste. No listener, network URI or shared storage permission. */
class ClipboardImageModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  private val worker = Executors.newSingleThreadExecutor()
  private val owned = java.util.concurrent.ConcurrentHashMap.newKeySet<String>()
  @Volatile private var closed = false
  override fun getName() = "GtmuxClipboardImage"
  @ReactMethod fun read(promise: Promise) {
    reactApplicationContext.runOnUiQueueThread {
      try {
      check(!closed)
      val clipboard = reactApplicationContext.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
      val clip = clipboard.primaryClip
      val uri = if (clip?.description?.hasMimeType("image/*") == true && clip.itemCount > 0) clip.getItemAt(0).uri else null
      if (uri == null) { promise.resolve(null); return@runOnUiQueueThread }
      if (uri.scheme != "content") { promise.reject("clipboard_image_invalid", "Unsupported image source"); return@runOnUiQueueThread }
      worker.execute {
        var file: File? = null
        try {
          val directory = File(reactApplicationContext.cacheDir, "clipboard-images").apply { mkdirs() }
          val cutoff = System.currentTimeMillis() - 24 * 60 * 60 * 1000L
          directory.listFiles()?.filter { it.name.matches(Regex("clipboard-[a-f0-9-]{36}\\.(png|jpg|webp|heic)")) && it.path !in owned && it.lastModified() < cutoff }?.forEach { it.delete() }
          val bytes = reactApplicationContext.contentResolver.openInputStream(uri)?.use { input ->
            val output = java.io.ByteArrayOutputStream(); val buffer = ByteArray(32768); var count = 0L
            while (true) { check(!closed);val size = input.read(buffer); if (size < 0) break; count += size; require(count <= DocumentPolicy.IMAGE_BYTES); output.write(buffer, 0, size) }
            output.toByteArray()
          } ?: error("unavailable")
          val options = BitmapFactory.Options().apply { inJustDecodeBounds = true }; BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)
          require(DocumentPolicy.imageDimensions(options.outWidth, options.outHeight))
          val extension = when (options.outMimeType) { "image/png" -> "png"; "image/jpeg" -> "jpg"; "image/webp" -> "webp"; "image/heif", "image/heic" -> "heic"; else -> error("unsupported") }
          require((directory.listFiles()?.sumOf { it.length() } ?: 0) + bytes.size <= 128L * 1024 * 1024)
          file = File(directory, "clipboard-${UUID.randomUUID()}.$extension"); file.writeBytes(bytes); owned.add(file.path)
          if (closed) {owned.remove(file.path);error("closed")}
          promise.resolve(Uri.fromFile(file).toString())
        } catch (error: Exception) { file?.let { owned.remove(it.path); it.delete() }; promise.reject("clipboard_image_unavailable", "Could not read this clipboard image", error) }
      }
      } catch (error: Exception) {promise.reject("clipboard_image_unavailable", "Could not access the clipboard image", error)}
    }
  }
  @ReactMethod fun release(value: String) {
    val path = runCatching { Uri.parse(value).takeIf { it.scheme == "file" }?.path }.getOrNull() ?: return
    if (owned.remove(path)) File(path).delete()
  }
  override fun invalidate() { closed = true;worker.shutdownNow(); owned.forEach { File(it).delete() }; owned.clear(); super.invalidate() }
  companion object {
    fun seedFixture(context: Context) {
      check(BuildConfig.QA_TESTING)
      val source = File(context.cacheDir, "synthetic-clipboard.png")
      val image = BitmapFactory.decodeResource(context.resources, R.drawable.brand) ?: error("fixture image")
      try { source.outputStream().use { require(image.compress(android.graphics.Bitmap.CompressFormat.PNG, 100, it)) } }
      finally { image.recycle() }
      val uri = SharedFiles.export(context, source, "synthetic-clipboard.png"); source.delete()
      (context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager).setPrimaryClip(ClipData.newUri(context.contentResolver, "Synthetic QA image", uri))
    }
  }
}
