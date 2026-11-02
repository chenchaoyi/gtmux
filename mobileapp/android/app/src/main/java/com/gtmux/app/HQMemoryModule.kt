package com.gtmux.app

import android.content.Intent
import android.system.Os
import com.facebook.react.bridge.*
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URI
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicLong
import java.util.zip.GZIPInputStream

class HQMemoryModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  private val worker = Executors.newSingleThreadExecutor()
  private val generation = AtomicLong()
  private val directory get() = File(reactApplicationContext.filesDir, "hq").apply { mkdirs() }
  private val archive get() = File(directory, "gtmux-hq-memory.tar.gz")
  override fun getName() = "HQMemory"
  @ReactMethod fun save(raw: String, token: String, promise: Promise) {
    val id = generation.incrementAndGet()
    worker.execute {
      val temp = File(directory, "copy-$id.partial")
      var connection: HttpURLConnection? = null
      try {
        val uri = URI(raw)
        require(uri.scheme in listOf("http", "https") && uri.host != null && uri.rawUserInfo == null && uri.rawQuery == null && uri.rawFragment == null && uri.path.endsWith("/api/hq/memory"))
        require(token.isNotEmpty() && token.length <= 8192 && token.none { it == '\r' || it == '\n' })
        connection = (uri.toURL().openConnection() as HttpURLConnection).apply {
          instanceFollowRedirects = false; connectTimeout = 15000; readTimeout = 120000
          useCaches = false; setRequestProperty("Authorization", "Bearer $token")
        }
        require(connection.responseCode == 200) { "The server could not provide a memory archive (${connection.responseCode})" }
        require(connection.contentLengthLong <= LIMIT) { "The memory archive exceeds the import limit" }
        var count = 0L
        connection.inputStream.use { input -> FileOutputStream(temp).use { output ->
          val buffer = ByteArray(32768)
          while (true) {
            val n = input.read(buffer); if (n < 0) break
            count += n; require(count <= LIMIT && id == generation.get()) { "Download cancelled or too large" }
            output.write(buffer, 0, n)
          }
          output.fd.sync()
        } }
        require(count > 0)
        // Validate the complete gzip footer and uncompressed bound. Nothing is extracted.
        var expanded = 0L
        GZIPInputStream(temp.inputStream()).use { input ->
          val buffer = ByteArray(32768)
          while (true) { val n = input.read(buffer); if (n < 0) break; expanded += n; require(expanded <= LIMIT) }
        }
        require(expanded >= 1024 && id == generation.get()) { "Download cancelled or invalid" }
        Os.rename(temp.path, archive.path) // Same-directory atomic replacement preserves the previous copy on failure.
        promise.resolve(describe())
      } catch (error: Exception) {
        promise.reject("memory_copy_failed", error.message ?: "Could not save this copy")
      } finally { connection?.disconnect(); temp.delete() }
    }
  }
  @ReactMethod fun state(promise: Promise) { worker.execute { promise.resolve(describe()) } }
  @ReactMethod fun forget(promise: Promise) {
    generation.incrementAndGet()
    worker.execute { archive.delete(); promise.resolve(null) }
  }
  @ReactMethod fun share(promise: Promise) {
    worker.execute {
      try {
        require(archive.isFile)
        val uri = SharedFiles.export(reactApplicationContext, archive, "gtmux-hq-memory.tar.gz")
        val intent = Intent(Intent.ACTION_SEND).setType("application/gzip")
          .putExtra(Intent.EXTRA_STREAM, uri).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        reactApplicationContext.runOnUiQueueThread {
          try {
            val activity = reactApplicationContext.currentActivity ?: error("No active window")
            activity.startActivity(Intent.createChooser(intent, "Export HQ copy")); promise.resolve(null)
          } catch (error: Exception) { promise.reject("share_failed", "Could not export this copy", error) }
        }
      } catch (error: Exception) { promise.reject("share_failed", "Could not export this copy", error) }
    }
  }
  private fun describe(): WritableMap? {
    if (!archive.isFile) return null
    return Arguments.createMap().apply {
      putString("path", archive.path)
      putString("url", "")
      putDouble("bytes", archive.length().toDouble()); putDouble("at", archive.lastModified() / 1000.0)
    }
  }
  override fun invalidate() { generation.incrementAndGet(); worker.shutdownNow(); super.invalidate() }
  companion object { private const val LIMIT = 128L * 1024 * 1024 }
}
