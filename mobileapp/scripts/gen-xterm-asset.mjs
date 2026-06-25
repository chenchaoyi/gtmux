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
    installScrollControl();
    // As you scroll vertically (no-wrap), re-fit the horizontal extent to the lines
    // now in view, so it never lets you scroll past the visible content.
    var scrollDebounce = null;
    term.onScroll(function () {
      if (wrapOn) return;
      clearTimeout(scrollDebounce);
      scrollDebounce = setTimeout(function () { relayoutCols(true); }, 120);
    });
  }

  // installScrollControl: per-gesture AXIS LOCK. Nested scrollers (outer #term =
  // horizontal, inner .xterm-viewport = vertical) otherwise fight — when not at the
  // vertical bottom, the inner viewport greedily grabs a near-horizontal swipe, so
  // horizontal won't trigger and you feel a stray vertical nudge. Decide the axis
  // from the first real movement: horizontal → preventDefault (block the native
  // vertical scroll) and drive #term.scrollLeft 1:1 with the finger; vertical → let
  // native momentum scrolling do its thing. Also pauses snapshot writes mid-gesture.
  function installScrollControl() {
    var t = el(), x0 = 0, y0 = 0, lastX = 0, axis = '';
    t.addEventListener('touchstart', function (e) {
      userScrolling = true;
      var p = e.touches[0];
      x0 = lastX = p.clientX; y0 = p.clientY; axis = '';
    }, { passive: true });
    t.addEventListener('touchmove', function (e) {
      if (e.touches.length !== 1) return;
      var p = e.touches[0];
      if (!axis) {
        var dx = Math.abs(p.clientX - x0), dy = Math.abs(p.clientY - y0);
        if (dx < 8 && dy < 8) return;
        axis = dx > dy ? 'x' : 'y';
      }
      if (axis === 'x') {
        e.preventDefault();                  // stop the inner vertical scroller grabbing it
        t.scrollLeft += (lastX - p.clientX); // drive horizontal 1:1 with the finger
        lastX = p.clientX;
      }
    }, { passive: false });
    function release() {
      setTimeout(function () {
        userScrolling = false;
        if (pending !== null) { var x = pending; pending = null; window.gtmuxWrite(x); }
      }, 400);
    }
    t.addEventListener('touchend', release, { passive: true });
    t.addEventListener('touchcancel', release, { passive: true });
  }

  // visibleMaxCols: the widest line currently in the VIEWPORT, in columns (trailing
  // blanks trimmed). The no-wrap extent tracks this — not the longest line anywhere
  // in scrollback — so you can never scroll past what's on screen into empty space
  // (e.g. one very long line far up in history used to make the whole pane scroll
  // into a black void). Recomputed as you scroll vertically.
  function visibleMaxCols() {
    var b = term.buffer.active, max = 0;
    var start = b.viewportY, end = Math.min(b.length, start + term.rows);
    for (var i = start; i < end; i++) {
      var ln = b.getLine(i);
      if (ln) { var s = ln.translateToString(true); if (s.length > max) max = s.length; }
    }
    return max;
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
    var maxLen = Math.max(baseCols, Math.min(visibleMaxCols() || baseCols, 1000));
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
  /* #term scrolls HORIZONTALLY, bounded by #xwrap's explicit width; the
     .xterm-viewport scrolls VERTICALLY. No touch-action lock — iOS already routes a
     gesture to the nested scroller that can move in that direction, and an explicit
     pan-x lock made horizontal swipes hard to even start. */
  #term{position:absolute;inset:0;padding:6px;overflow-x:hidden;overflow-y:hidden;-webkit-overflow-scrolling:touch}
  #xwrap{position:relative;height:100%;width:100%}
  .xterm-viewport{overflow-x:hidden !important;overflow-y:scroll !important;-webkit-overflow-scrolling:touch}
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
