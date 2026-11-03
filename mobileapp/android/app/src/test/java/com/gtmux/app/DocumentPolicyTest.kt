package com.gtmux.app

import org.junit.Assert.*
import org.junit.Test

class DocumentPolicyTest {
  @Test fun imageBoundsAreSharedByDocumentAndClipboardReaders() {
    assertTrue(DocumentPolicy.imageDimensions(2000, 2500))
    assertFalse(DocumentPolicy.imageDimensions(0, 100))
    assertFalse(DocumentPolicy.imageDimensions(20001, 1))
    assertFalse(DocumentPolicy.imageDimensions(10000, 10000))
  }
  @Test fun resolverKeepsOriginAndRejectsCredentialOrQueryConfusion() {
    assertEquals("localhost", DocumentPolicy.resolver("http://localhost:8765/api/artifacts/resolve").host)
    listOf("file:///api/artifacts/resolve", "https://a:b@example.test/api/artifacts/resolve", "https://example.test/api/artifacts/resolve?token=secret", "https://example.test/api/artifacts/resolve#fragment", "https://example.test/api/artifacts/content").forEach {
      try { DocumentPolicy.resolver(it); fail("accepted $it") } catch (_: IllegalArgumentException) { }
    }
  }
  @Test fun onlyParentLocalImagesUseAuthenticatedTransport() {
    listOf("figure.png", "../images/plot.jpg", "/tmp/report/image.png", "file:///tmp/image.png", "file://localhost/tmp/image.png", "gtmux-file://0123").forEach { assertTrue(it, DocumentPolicy.localReference(it)) }
    listOf("https://example.test/pixel", "//example.test/pixel", "file://remote/tmp/image.png", "javascript:alert(1)", "data:image/png;base64,AA", "x\ny", "").forEach { assertFalse(it, DocumentPolicy.localReference(it)) }
  }
  @Test fun metadataMustMatchTheCompleteContentContract() {
    fun valid(id: String = "a".repeat(32), hash: String = "b".repeat(64), bytes: Long = 1, kind: String = "html", name: String = "report.html", source: String = "/tmp/report.html", created: Long = 1) = DocumentPolicy.descriptorValid(id, hash, bytes, kind, name, source, created)
    assertTrue(valid()); assertTrue(valid(bytes = DocumentPolicy.MAX_BYTES))
    assertFalse(valid(id = "../bad")); assertFalse(valid(hash = "partial")); assertFalse(valid(bytes = 0)); assertFalse(valid(bytes = DocumentPolicy.MAX_BYTES + 1))
    assertFalse(valid(kind = "executable")); assertFalse(valid(name = "bad\nname")); assertFalse(valid(source = "relative")); assertFalse(valid(created = 0))
  }
}
