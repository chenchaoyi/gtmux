package com.gtmux.app

import android.content.Context
import android.net.Uri
import androidx.core.content.FileProvider
import java.io.File
import java.util.UUID

/** A granted share URI survives closing its reader. Never exposes the whole cache. */
object SharedFiles {
  private const val DAY = 24 * 60 * 60 * 1000L
  private const val BUDGET = 256L * 1024 * 1024
  @Synchronized fun export(context: Context, source: File, name: String): Uri {
    val directory = File(context.cacheDir, "share-exports").apply { mkdirs() }
    val now = System.currentTimeMillis()
    directory.listFiles()?.filter { now - it.lastModified() > DAY }?.forEach { it.deleteRecursively() }
    val retained = directory.walkTopDown().filter { it.isFile }.sumOf { it.length() }
    require(source.isFile && retained + source.length() <= BUDGET) { "Share cache is full; try again after older exports expire" }
    val leaf = File(directory, UUID.randomUUID().toString()).apply { mkdirs() }
    val safe = name.substringAfterLast('/').substringAfterLast('\\').replace(Regex("[\\p{Cntrl}]"), "_").take(200).ifBlank { "gtmux-export" }
    val copy = File(leaf, safe)
    try { source.inputStream().use { input -> copy.outputStream().use { input.copyTo(it) } } }
    catch (error: Exception) { leaf.deleteRecursively(); throw error }
    return FileProvider.getUriForFile(context, BuildConfig.APPLICATION_ID + ".files", copy)
  }
}
