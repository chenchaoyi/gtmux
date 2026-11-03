package com.gtmux.app

import android.content.Context
import org.json.JSONObject
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URI
import java.net.URLEncoder
import java.security.MessageDigest
import java.util.UUID

object DocumentPolicy {
  const val MAX_BYTES = 30L * 1024 * 1024
  const val IMAGE_BYTES = 8L * 1024 * 1024
  fun imageDimensions(width: Int, height: Int) = width in 1..20000 && height in 1..20000 && width.toLong() * height <= 50_000_000
  fun resolver(raw: String): URI {
    val uri = URI(raw)
    require(uri.scheme in setOf("http", "https") && !uri.host.isNullOrEmpty() && uri.rawUserInfo == null && uri.rawQuery == null && uri.rawFragment == null && uri.path.endsWith("/api/artifacts/resolve")) { "invalid" }
    return uri
  }
  fun localReference(value: String): Boolean {
    if (value.isBlank() || value.length > 4096 || value.any { it.code < 32 } || value.startsWith("//")) return false
    val uri = runCatching { URI(value.replace(" ", "%20")) }.getOrNull() ?: return false
    return uri.scheme == null || uri.scheme == "gtmux-file" || (uri.scheme == "file" && (uri.host.isNullOrEmpty() || uri.host == "localhost"))
  }
  fun descriptorValid(id: String, hash: String, bytes: Long, kind: String, name: String, source: String, created: Long): Boolean =
    id.matches(Regex("[a-f0-9]{32}")) && hash.matches(Regex("[a-f0-9]{64}")) && bytes in 1..MAX_BYTES &&
      kind in setOf("html", "markdown", "text", "pdf", "image") && name.isNotBlank() && name.length <= 1024 && name.none { it.code < 32 } &&
      source.startsWith("/") && source.length <= 4096 && created > 0
}

data class DocumentRequest(val resolveURL: String, val headers: Map<String, String>, val reference: String, val session: String, val home: String, val lang: String, val parent: String? = null, val appearance: String = "") {
  init {
    DocumentPolicy.resolver(resolveURL)
    require(appearance in setOf("", "light", "dark")) { "invalid" }
    require(reference.isNotEmpty() && reference.toByteArray().size <= 4096 && session.isNotEmpty() && session.length <= 256 && home.length <= 256) { "invalid" }
    require(headers.all { (key, value) -> key.lowercase() in setOf("authorization", "x-gtmux-client") && value.length <= 8192 && value.none { it == '\r' || it == '\n' } }) { "invalid" }
  }
  companion object {
    fun parse(json: JSONObject): DocumentRequest {
      val h = json.optJSONObject("headers") ?: JSONObject()
      return DocumentRequest(json.getString("resolveURL"), h.keys().asSequence().associateWith { h.getString(it) }, json.getString("reference"), json.getString("session_id"), json.optString("home_id"), json.optString("lang"), json.optString("parent_id").ifBlank { null }, json.optString("appearance"))
    }
  }
}
data class LocalDocument(val id: String, val name: String, val kind: String, val media: String, val file: File, val hash: String)

/** Native, bounded, authenticated transport. No cookies, redirects, WebView headers or JS bytes. */
class DocumentTransport(private val context: Context) : AutoCloseable {
  private val directory = createDirectory(context)
  @Volatile private var closed = false
  private val connections = java.util.Collections.synchronizedSet(mutableSetOf<HttpURLConnection>())
  private fun connection(uri: URI, headers: Map<String, String>): HttpURLConnection {
    check(!closed)
    return (uri.toURL().openConnection() as HttpURLConnection).apply {
      instanceFollowRedirects = false; useCaches = false; connectTimeout = 15000; readTimeout = 30000
      headers.forEach { (key, value) -> setRequestProperty(key, value) }
      connections.add(this)
    }
  }
  fun fetch(request: DocumentRequest, limit: Long = DocumentPolicy.MAX_BYTES): LocalDocument {
    val endpoint = DocumentPolicy.resolver(request.resolveURL)
    val resolver = connection(endpoint, request.headers)
    val metadata: JSONObject
    try {
      resolver.requestMethod = "POST"; resolver.doOutput = true; resolver.setRequestProperty("Content-Type", "application/json")
      val body = JSONObject().put("reference", request.reference).put("session_id", request.session).put("home_id", request.home).apply { request.parent?.let { put("parent_id", it) } }.toString().toByteArray()
      resolver.setFixedLengthStreamingMode(body.size); resolver.outputStream.use { it.write(body) }
      status(resolver.responseCode)
      require(resolver.contentLengthLong <= 24 * 1024) { "invalid" }
      metadata = JSONObject(resolver.inputStream.use { input ->
        val bytes = input.readBytesBounded(24 * 1024L); String(bytes, Charsets.UTF_8)
      })
    } finally { resolver.disconnect(); connections.remove(resolver) }
    val id = metadata.getString("id"); val hash = metadata.getString("sha256"); val bytes = metadata.getLong("bytes")
    val name = metadata.getString("name"); val kind = metadata.getString("kind"); val media = metadata.getString("media_type")
    require(DocumentPolicy.descriptorValid(id, hash, bytes, kind, name, metadata.getString("source"), metadata.getLong("created_at")) &&
      metadata.getString("session_id") == request.session && metadata.optString("home_id") == request.home) { "invalid" }
    require(bytes <= limit) { "tooLarge" }
    fun enc(s: String) = URLEncoder.encode(s, "UTF-8")
    val content = URI("${endpoint.scheme}://${endpoint.rawAuthority}${endpoint.rawPath.removeSuffix("resolve")}content?id=${enc(id)}&session_id=${enc(request.session)}&home_id=${enc(request.home)}")
    val connection = connection(content, request.headers)
    val suffix = when (kind) { "pdf" -> "pdf"; "html" -> "html"; "image" -> if (media == "image/png") "png" else "jpg"; else -> "txt" }
    val file = File(directory, "$id-${UUID.randomUUID()}.$suffix")
    try {
      status(connection.responseCode)
      require(connection.contentLengthLong < 0 || connection.contentLengthLong == bytes) { "invalid" }
      val digest = MessageDigest.getInstance("SHA-256"); var count = 0L
      connection.inputStream.use { input -> FileOutputStream(file).use { output ->
        val buffer = ByteArray(32768)
        while (true) {
          check(!closed); val n = input.read(buffer); if (n < 0) break
          count += n; require(count <= bytes && count <= limit) { "tooLarge" }
          digest.update(buffer, 0, n); output.write(buffer, 0, n)
        }
      } }
      require(count == bytes && digest.digest().joinToString("") { "%02x".format(it) } == hash) { "invalid" }
      check(!closed)
      return LocalDocument(id, name, kind, media, file, hash)
    } catch (e: Exception) { file.delete(); throw e }
    finally { connection.disconnect(); connections.remove(connection) }
  }
  companion object {
    private val active = mutableSetOf<String>()
    @Synchronized private fun createDirectory(context: Context): File {
      val parent = File(context.cacheDir, "documents").apply { mkdirs() }
      val cutoff = System.currentTimeMillis() - 24 * 60 * 60 * 1000L
      parent.listFiles()?.filter { it.name.matches(Regex("[a-f0-9-]{36}")) && it.path !in active && it.lastModified() < cutoff }?.forEach { it.deleteRecursively() }
      return File(parent, UUID.randomUUID().toString()).apply { mkdirs(); active.add(path) }
    }
    @Synchronized private fun forgetDirectory(directory: File) { active.remove(directory.path); directory.deleteRecursively() }
  }
  private fun status(code: Int) {
    if (code != 200) error(when (code) { 401, 403 -> "permission"; 404 -> "unavailable"; 413 -> "tooLarge"; 503 -> "unsupported"; else -> "network" })
  }
  override fun close() {
    closed = true
    synchronized(connections) { connections.forEach { it.disconnect() }; connections.clear() }
    // Exclusively created private UUID subtree; never a caller-selected path.
    forgetDirectory(directory)
  }
}

private fun java.io.InputStream.readBytesBounded(limit: Long): ByteArray {
  val output = java.io.ByteArrayOutputStream(); val buffer = ByteArray(8192); var total = 0L
  while (true) { val n = read(buffer); if (n < 0) break; total += n; require(total <= limit) { "tooLarge" }; output.write(buffer, 0, n) }
  return output.toByteArray()
}
