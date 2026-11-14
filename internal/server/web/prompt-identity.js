// Shared by the native clients and the generated browser asset. This is only a
// comparison key; the original prompt remains the source of displayed text.
(function (root, factory) {
  const api = factory();
  if (typeof module !== 'undefined' && module.exports) { module.exports = api; }
  if (root) { root.GtmuxPromptIdentity = api; }
})(typeof window === 'undefined' ? null : window, function () {
  function promptIdentity(value) {
    const text = String(value || '').replace(/\r\n/g, '\n').trim();
    const lines = text.split('\n'), prose = [], images = [], bareImages = [], renderedImages = [], labels = new Set();
    let fence = null;
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i], trimmed = line.trim();
      const marker = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(line);
      if (marker) {
        if (!fence) { fence = marker[1]; }
        else if (marker[1][0] === fence[0] && marker[1].length >= fence.length && !marker[2].trim()) { fence = null; }
        prose.push(line); continue;
      }
      if (!fence) {
        let match = /^!\[(Image #[1-9]\d*)\]\((\/[^\n]+)\)$/.exec(line);
        let path, closesOnNextLine = false;
        if (match) {
          try { path = decodeURIComponent(match[2]); } catch (_) { /* retain malformed input */ }
        } else {
          match = /^<image name=\[(Image #[1-9]\d*)\] path="(\/[^"\n<>]+)">[ \t]*(?:<\/image>)?$/.exec(line);
          if (match && (line.endsWith('</image>') || lines[i + 1]?.trim() === '</image>')) {
            path = match[2];
            closesOnNextLine = !line.endsWith('</image>');
          }
        }
        if (path && /^\/(?!\/)[^\x00\r\n]+\.(?:png|jpe?g|webp|gif|heic|heif|bmp|tiff?)$/i.test(path)) {
          if (closesOnNextLine) { i++; }
          images.push(path); renderedImages.push(path); labels.add('[' + match[1] + ']'); continue;
        }
        if (/^\/(?!\/)[^\x00\r\n]+\.(?:png|jpe?g|webp|gif|heic|heif|bmp|tiff?)$/i.test(line)) {
          images.push(line); bareImages.push(line); continue;
        }
        if (line.startsWith('[') && labels.has(trimmed)) { continue; }
      }
      prose.push(line);
    }
    // A transcript can retain the Composer's path list AND add image links.
    // Collapse only complete, equal ordered inventories: these are two views of
    // the same attachments. Keep multiplicity, order and unmatched references.
    const dualInventory = bareImages.length > 0 && bareImages.length === renderedImages.length &&
      bareImages.every((path, i) => path === renderedImages[i]);
    return JSON.stringify([prose.join('\n').trim(), dualInventory ? bareImages : images]);
  }
  return {promptIdentity};
});
