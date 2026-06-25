// Generates src/ui/xtermAsset.ts — a single self-contained HTML document with
// xterm.js + its CSS + the fit/unicode11 addons + a small bridge, inlined so it
// loads in a react-native-webview via source={{html}} (no CDN, offline-safe).
//
// Re-run after bumping @xterm/* (devDeps):  node scripts/gen-xterm-asset.mjs
import {readFileSync, writeFileSync, mkdirSync} from 'node:fs';
import {dirname, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const nm = resolve(here, '..', 'node_modules', '@xterm');
const read = p => readFileSync(p, 'utf8');

const css = read(resolve(nm, 'xterm/css/xterm.css'));
const xtermJs = read(resolve(nm, 'xterm/lib/xterm.js'));
const fitJs = read(resolve(nm, 'addon-fit/lib/addon-fit.js'));
const uni11Js = read(resolve(nm, 'addon-unicode11/lib/addon-unicode11.js'));

// The bridge: RN calls window.gtmuxWrite / gtmuxConfig via injectJavaScript. The
// terminal is read-only here (no key input wired yet — that stays on the existing
// FloatingKeys/Composer path); we just render the colored capture-pane snapshot.
const bootstrap = `
  var term, fit, wrapOn = true, lastText = '';
  function el() { return document.getElementById('term'); }

  function boot() {
    term = new Terminal({
      convertEol: true,            // capture-pane lines are LF-only → treat as CRLF
      cursorBlink: false,
      disableStdin: true,
      scrollback: 5000,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: 12,
      allowProposedApi: true,
      theme: { background: '#0B0B0F', foreground: '#D6D6DA' }
    });
    fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    try {
      var u = new Unicode11Addon.Unicode11Addon();
      term.loadAddon(u);
      term.unicode.activeVersion = '11';   // correct CJK / wide-glyph widths
    } catch (e) {}
    term.open(el());
    relayout();
    window.addEventListener('resize', relayout);
    installTouchScroll();
  }

  // relayout: wrap on → fit cols to width (long lines wrap); wrap off → widen cols
  // to the content's longest line so nothing wraps, with horizontal scroll.
  function relayout() {
    if (!term || !el()) return;
    try { fit.fit(); } catch (e) {}       // sets rows for the current height
    if (wrapOn) {
      el().style.overflowX = 'hidden';
    } else {
      el().style.overflowX = 'auto';
      var maxLen = 0;
      lastText.split('\\n').forEach(function (l) { if (l.length > maxLen) maxLen = l.length; });
      var cols = Math.max(term.cols, Math.min(maxLen || term.cols, 500));
      if (cols !== term.cols) { try { term.resize(cols, term.rows); } catch (e) {} }
    }
  }

  function rowPx() {
    var rows = (term && term.rows) || 24;
    var h = el() ? el().clientHeight : 200;
    return Math.max(8, h / rows);
  }

  // xterm only scrolls its scrollback on wheel events; iOS touch produces none, so
  // wire touch-drag → term.scrollLines (vertical), letting horizontal drags fall
  // through to the container's native overflow-x scroll (no-wrap mode).
  function installTouchScroll() {
    var e0 = el(), lastY = null, sx = 0, sy = 0, axis = '';
    e0.addEventListener('touchstart', function (e) {
      var t = e.touches[0]; lastY = t.clientY; sx = t.clientX; sy = t.clientY; axis = '';
    }, { passive: true });
    e0.addEventListener('touchmove', function (e) {
      if (lastY === null) return;
      var t = e.touches[0];
      if (!axis) {
        var dx = Math.abs(t.clientX - sx), dy = Math.abs(t.clientY - sy);
        if (dx < 6 && dy < 6) return;
        axis = dx > dy ? 'x' : 'y';
        if (axis === 'x') { lastY = null; return; } // native horizontal scroll
      }
      var delta = lastY - t.clientY;
      var lines = delta > 0 ? Math.floor(delta / rowPx()) : Math.ceil(delta / rowPx());
      if (lines !== 0) { term.scrollLines(lines); lastY = t.clientY; }
    }, { passive: true });
    function end() { lastY = null; axis = ''; }
    e0.addEventListener('touchend', end, { passive: true });
    e0.addEventListener('touchcancel', end, { passive: true });
  }

  window.gtmuxWrite = function (text) {
    if (!term) return;
    lastText = text;
    // preserve the reader's scroll position across the full reset+rewrite: keep the
    // same distance from the bottom (so scrolling up isn't yanked back every poll).
    var b = term.buffer.active;
    var wasBottom = b.viewportY >= b.baseY;
    var fromBottom = b.baseY - b.viewportY;
    if (!wrapOn) relayout();             // widen before writing so no-wrap is correct
    term.reset();
    term.write(text, function () {
      var nb = term.buffer.active;
      if (wasBottom) term.scrollToBottom();
      else { try { term.scrollToLine(Math.max(0, nb.baseY - fromBottom)); } catch (e) {} }
    });
  };

  window.gtmuxConfig = function (opts) {
    if (!term || !opts) return;
    if (typeof opts.fontSize === 'number') term.options.fontSize = opts.fontSize;
    if (typeof opts.wrap === 'boolean') wrapOn = opts.wrap;
    if (opts.theme) term.options.theme = opts.theme;
    relayout();
    if (lastText) window.gtmuxWrite(lastText); // re-render at the new wrap/size
  };

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
`;

const html = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
<style>${css}
  html,body{margin:0;padding:0;height:100%;background:#0B0B0F;overflow:hidden}
  #term{position:absolute;inset:0;padding:6px;overflow-y:hidden;-webkit-overflow-scrolling:touch}
</style></head><body><div id="term"></div>
<script>${xtermJs}</script><script>${fitJs}</script><script>${uni11Js}</script>
<script>${bootstrap}</script></body></html>`;

const out = resolve(here, '..', 'src', 'ui', 'xtermAsset.ts');
mkdirSync(dirname(out), {recursive: true});
writeFileSync(
  out,
  '// AUTO-GENERATED by scripts/gen-xterm-asset.mjs — do not edit by hand.\n' +
    '// Self-contained xterm.js terminal document for the react-native-webview renderer.\n' +
    '// (eslint-ignored in .eslintrc.js — a ~300KB inline bundle.)\n' +
    `export const XTERM_HTML: string = ${JSON.stringify(html)};\n`,
);
console.log(`wrote ${out} (${(html.length / 1024).toFixed(0)} KB)`);
