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
const webglJs = read(resolve(nm, 'addon-webgl/lib/addon-webgl.js'));

// The bridge: RN calls window.gtmuxWrite / gtmuxConfig via injectJavaScript. The
// terminal is read-only here (no key input wired yet — that stays on the existing
// FloatingKeys/Composer path); we just render the colored capture-pane snapshot.
const bootstrap = `
  var term, fit, wrapOn = true, lastText = '', baseCols = 80, cellPx = 8;
  var userScrolling = false, pending = null, lastMaxLen = -1;
  function el() { return document.getElementById('term'); }      // outer: horizontal scroller
  function xw() { return document.getElementById('xwrap'); }     // inner: width = content extent

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
    term.open(xw());                         // xterm lives in the width-controlled wrapper
    // GPU renderer — the DOM renderer can't repaint visible rows fast enough to keep
    // up with momentum scroll on a phone (→ jank). WebGL2 keeps up; on context loss
    // (e.g. backgrounding) dispose it so xterm falls back to the DOM renderer.
    try {
      var webgl = new WebglAddon.WebglAddon();
      webgl.onContextLoss(function () { try { webgl.dispose(); } catch (e) {} });
      term.loadAddon(webgl);
    } catch (e) {}
    relayout();
    window.addEventListener('resize', relayout);
    // While the finger is down, pause snapshot writes: a working pane updates every
    // poll, and a reset+rewrite mid-gesture would yank you back to the bottom.
    el().addEventListener('touchstart', function () { userScrolling = true; }, { passive: true });
    function release() {
      setTimeout(function () {
        userScrolling = false;
        if (pending !== null) { var t = pending; pending = null; window.gtmuxWrite(t); }
      }, 400);
    }
    el().addEventListener('touchend', release, { passive: true });
    el().addEventListener('touchcancel', release, { passive: true });
  }

  // visibleLen: a line's on-screen width, ignoring ANSI escapes (capture-pane -e
  // embeds SGR + sometimes OSC, which must NOT count toward the no-wrap width).
  function visibleLen(l) {
    return l
      .replace(/\\x1b\\][^\\x07]*\\x07/g, '') // OSC … BEL
      .replace(/\\x1b\\[[0-9;?]*[A-Za-z]/g, '') // CSI
      .length;
  }

  // relayout: measure the cell size at fill width, then (no-wrap) widen the wrapper
  // to the content. Horizontal scroll = #term scrolling the wrapper, whose explicit
  // width is the exact extent — so it can't scroll into empty space, on any renderer.
  function relayout() {
    if (!term || !el() || !xw()) return;
    xw().style.width = '100%';
    try { fit.fit(); } catch (e) {}
    baseCols = term.cols;
    if (term.element) cellPx = term.element.getBoundingClientRect().width / Math.max(1, baseCols);
    lastMaxLen = -1;
    if (wrapOn) {
      el().style.overflowX = 'hidden';
    } else {
      el().style.overflowX = 'auto';
      relayoutCols(true);
    }
  }

  // relayoutCols (no-wrap): set the wrapper width to the content's longest VISIBLE
  // line (bounded), then refit so xterm fills it. Skips when the max width is
  // unchanged, so streaming appends don't thrash fit().
  function relayoutCols(force) {
    if (!term || wrapOn || !xw()) return;
    var maxLen = 0;
    lastText.split('\\n').forEach(function (l) { var n = visibleLen(l); if (n > maxLen) maxLen = n; });
    maxLen = Math.min(maxLen || baseCols, 1000);
    if (!force && maxLen === lastMaxLen) return;
    lastMaxLen = maxLen;
    var inner = el().clientWidth - 12;     // 12 = #term padding
    var contentPx = Math.max(inner, Math.ceil(maxLen * cellPx));
    xw().style.width = contentPx + 'px';
    try { fit.fit(); } catch (e) {}        // xterm now fills the wide wrapper
  }

  window.gtmuxWrite = function (text) {
    if (!term || text === lastText) return;
    if (userScrolling) { pending = text; return; } // hold writes while a gesture is active
    var prev = lastText;
    lastText = text;
    // Append-only (the common case: more output at the bottom): write just the new
    // tail — no reset, so no flash, cheap, and xterm natively keeps your scroll
    // position (only follows the bottom if you were already there).
    if (prev && text.length > prev.length && text.lastIndexOf(prev, 0) === 0) {
      relayoutCols();
      term.write(text.slice(prev.length));
      return;
    }
    // Full change (scrolled-off / TUI redraw) → repaint, preserving the reader's
    // distance from the bottom so a manual scroll-up isn't yanked back.
    var b = term.buffer.active;
    var wasBottom = b.viewportY >= b.baseY;
    var fromBottom = b.baseY - b.viewportY;
    relayoutCols();
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
    // force a re-render at the new wrap/size (gtmuxWrite skips an unchanged text).
    var t = lastText; lastText = ''; if (t) window.gtmuxWrite(t);
  };

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
`;

const html = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
<style>${css}
  html,body{margin:0;padding:0;height:100%;background:#0B0B0F;overflow:hidden}
  /* #term scrolls HORIZONTALLY, bounded by #xwrap's explicit width. touch-action
     locks axes: #term takes horizontal pans, the .xterm-viewport takes vertical —
     so a horizontal swipe never bleeds into a stray vertical scroll, and vice versa. */
  #term{position:absolute;inset:0;padding:6px;overflow-x:hidden;overflow-y:hidden;-webkit-overflow-scrolling:touch;touch-action:pan-x}
  #xwrap{position:relative;height:100%;width:100%}
  .xterm-viewport{overflow-x:hidden !important;overflow-y:scroll !important;-webkit-overflow-scrolling:touch;touch-action:pan-y}
  /* clip the (absolutely-positioned) WebGL canvas to the logical screen width: on
     retina iOS it renders wider and, as an absolute descendant, would expand
     #term's scrollWidth → unbounded horizontal scroll. Verified via Playwright. */
  .xterm .xterm-screen{overflow:hidden}
</style></head><body><div id="term"><div id="xwrap"></div></div>
<script>${xtermJs}</script><script>${fitJs}</script><script>${uni11Js}</script><script>${webglJs}</script>
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
