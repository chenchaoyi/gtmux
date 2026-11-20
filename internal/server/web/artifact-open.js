(function (root) {
  'use strict';
  const maxImageBytes = 8 * 1024 * 1024;
  const maxImages = 32;
  const maxTotalImageBytes = 16 * 1024 * 1024;

  function startupDocument(hash) {
    const fields = new URLSearchParams(hash.replace(/^#/, ''));
    for (const key of fields.keys()) {
      if (!['document', 'session', 'home'].includes(key) || fields.getAll(key).length !== 1) return null;
    }
    const id = fields.get('document') || '', session = fields.get('session') || '', home = fields.get('home') || '';
    if (!/^[a-f0-9]{32}$/.test(id) || !session || session.length > 256 || home.length > 128 || /[\x00-\x1f\x7f/\\]/.test(session + home)) return null;
    return {reference: 'gtmux-file:' + id, session_id: session, home_id: home};
  }

  const tags = new Set('a article aside b blockquote br caption code col colgroup dd del details div dl dt em figcaption figure footer h1 h2 h3 h4 h5 h6 header hr i img li main nav ol p picture pre s section small span strong style sub summary sup table tbody td th thead time tr u ul'.split(' '));
  const attributes = new Set('class id title colspan rowspan scope open alt width height datetime'.split(' '));

  // Parse with the browser's CSS parser. Rebuild only layout properties with
  // literal values. Functions, substitutions, resource properties, nested rules
  // and at-rules have no representation in the output.
  const layout = new Set(('display flex flex-direction flex-wrap flex-grow flex-shrink justify-content align-items gap grid-template-columns ' +
    'color background-color font-family font-size font-style font-weight line-height text-align text-decoration white-space ' +
    'margin margin-top margin-right margin-bottom margin-left padding padding-top padding-right padding-bottom padding-left ' +
    'width height max-width max-height min-width min-height border border-width border-style border-color border-radius border-collapse vertical-align').split(' '));

  function declarations(style) {
    const safe = [];
    for (const property of style) {
      const value = style.getPropertyValue(property);
      if (!layout.has(property)) continue;
      const literal = /^[#\w.% ,\-]+$/.test(value);
      // CSSOM serializes ordinary hexadecimal colors as rgb()/rgba().
      const color = /^(?:color|background-color|border-color)$/.test(property) && /^rgba?\([0-9.,% /]+\)$/.test(value);
      if (literal || color) safe.push(property + ':' + value);
    }
    return safe.join(';');
  }

  function stylesheet(source) {
    const sheet = new CSSStyleSheet();
    sheet.replaceSync(source);
    const rules = [];
    for (const rule of sheet.cssRules) {
      if (!(rule instanceof CSSStyleRule) || !/^[a-zA-Z0-9_.#,: >+~*\-]+$/.test(rule.selectorText)) continue;
      const style = declarations(rule.style);
      if (style) rules.push(rule.selectorText + '{' + style + '}');
    }
    return rules.join('\n');
  }

  function inlineRaster(src, limit) {
    if (src.length > 32 + 4 * Math.ceil(limit / 3)) return null;
    const parsed = /^data:(image\/(?:png|jpeg));base64,([a-z0-9+/]+=*)$/i.exec(src);
    if (!parsed) return null;
    try {
      const data = atob(parsed[2]);
      if (data.length > limit) return null;
      return {type: parsed[1].toLowerCase(), bytes: Uint8Array.from(data, ch => ch.charCodeAt(0))};
    } catch (_) { return null; }
  }

  function dataURL(bytes, type) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = () => reject(new Error('failed'));
      reader.readAsDataURL(new Blob([bytes], {type}));
    });
  }

  async function htmlDocument(source, rasterType, readRelated, signal) {
    const template = document.createElement('template');
    template.innerHTML = source;
    let attempts = 0, remaining = maxTotalImageBytes;
    for (const element of template.content.querySelectorAll('*')) {
      if (signal.aborted) throw new Error('retired');
      if (!template.content.contains(element)) continue;
      if (element.namespaceURI !== 'http://www.w3.org/1999/xhtml' || !tags.has(element.localName)) {
        element.remove();
        continue;
      }
      const src = element.getAttribute('src') || '';
      const style = element.hasAttribute('style') ? declarations(element.style) : '';
      for (const attr of Array.from(element.attributes)) {
        if (!attributes.has(attr.name)) element.removeAttributeNode(attr);
      }
      if (style) element.setAttribute('style', style);
      if (element.localName === 'style') element.textContent = stylesheet(element.textContent);
      if (element.localName !== 'img') continue;

      const limit = Math.min(remaining, maxImageBytes);
      let image = null;
      if (attempts < maxImages && limit > 0 && src) {
        attempts++;
        if (/^data:/i.test(src)) image = inlineRaster(src, limit);
        else if (src.length <= 4096 && !/^(?:[a-z][\w+.\-]*:|\/\/)/i.test(src) && !/[\x00-\x1f\x7f\\]/.test(src)) {
          image = await readRelated(src, limit);
        }
      }
      if (!image || image.bytes.length > limit || rasterType(image.bytes) !== image.type) {
        element.remove();
        continue;
      }
      remaining -= image.bytes.length;
      element.setAttribute('src', await dataURL(image.bytes, image.type));
    }
    const csp = "default-src 'none'; img-src data:; style-src 'unsafe-inline'; form-action 'none'; base-uri 'none'; frame-src 'none'; object-src 'none'";
    return '<!doctype html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="' + csp + '"><meta name="viewport" content="width=device-width,initial-scale=1"><style>html{color-scheme:light dark}body{margin:16px;overflow-wrap:anywhere}img{max-width:100%;height:auto}</style>' + template.innerHTML;
  }
  const api = {startupDocument, htmlDocument, maxImageBytes};
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.GtmuxArtifactOpen = api;
})(typeof window !== 'undefined' ? window : globalThis);
