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
const canvasJs = read(resolve(nm, 'addon-canvas/lib/addon-canvas.js'));

// Bundle the Hack font (the user's Ghostty font) as base64 woff2 so the terminal
// renders in it on any phone — iOS has no Hack installed, and a webview can only
// use a system-installed font or one shipped via @font-face. ~106KB each.
const fontB64 = f => readFileSync(resolve(here, '..', 'assets', 'fonts', f)).toString('base64');
const hackRegular = fontB64('Hack-Regular.woff2');
const hackBold = fontB64('Hack-Bold.woff2');
const fontFace =
  "@font-face{font-family:'Hack';font-weight:400;font-style:normal;font-display:block;" +
  "src:url(data:font/woff2;base64," + hackRegular + ") format('woff2')}" +
  "@font-face{font-family:'Hack';font-weight:700;font-style:normal;font-display:block;" +
  "src:url(data:font/woff2;base64," + hackBold + ") format('woff2')}";

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
      cursorInactiveStyle: 'none', // xterm's own cursor sits at the write position (the
      cursorStyle: 'block',        // content end); we draw the REAL pane cursor as a
      disableStdin: true,          // decoration instead (gtmuxCursor) so writes aren't moved
      scrollback: 5000,
      fontFamily: 'Hack, Menlo, Monaco, "Courier New", monospace',  // bundled @font-face
      fontSize: 12,
      allowProposedApi: true,
      // colors taken from the user's Ghostty config (CJK falls back to the system font).
      theme: { background: '#17171a', foreground: '#d4d2cc', cursor: '#bbc1ff', selectionBackground: '#2a2a33' }
    });
    fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    try {
      var u = new Unicode11Addon.Unicode11Addon();
      term.loadAddon(u);
      term.unicode.activeVersion = '11';   // correct CJK / wide-glyph widths
    } catch (e) {}
    term.open(xw());                         // xterm lives in the width-controlled wrapper
    // CANVAS renderer (not WebGL). The DOM renderer can't repaint visible rows fast
    // enough for momentum scroll on a phone (→ jank), but WebGL is unreliable in iOS
    // WKWebView on real devices: its GL context black-frames (a blank terminal on
    // open and after the wrap toggle) in ways that never reproduce on the simulator.
    // The canvas addon is GPU-composited, smooth enough for scroll, and has none of
    // WebGL's context fragility. If it fails to load, xterm falls back to DOM.
    try { term.loadAddon(new CanvasAddon.CanvasAddon()); } catch (e) {}
    relayout();
    window.addEventListener('resize', relayout);
    installScrollControl();
    // SELF-HEAL the initial render. On a real device (and flakily on the sim) the
    // WebView's final width — and the WebGL renderer — aren't ready when the first
    // fit() runs at boot/load. The result is a terminal that paints BLACK, or fits
    // to a too-wide viewport so long lines don't wrap to the visible width. A second
    // relayout() once things settle re-fit()s at the real width and forces a full
    // WebGL redraw (the same recovery the Wrap toggle triggers). Cheap + idempotent
    // once stable, so a few staged passes reliably catch a late-settling layout.
    [120, 350, 800, 1500].forEach(function (ms) { setTimeout(relayout, ms); });
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

  // The no-wrap extent must come from the SOURCE snapshot, NOT the xterm buffer.
  // Once the buffer is wrapped at the narrow fit width every buffer row is <= cols,
  // so measuring the buffer can never widen back out — the wrap/scroll toggle then
  // does nothing (both modes look wrapped). srcMaxCols scans the raw capture text.
  var srcMaxCols = 0;
  // visibleLen: visible column count of one raw line (skips CSI + OSC escape runs)
  // via a charcode scan — no regex/escape literals, which a template literal would
  // mangle. OSC 8 file:// hyperlinks (Claude Code) otherwise inflate length ~3x.
  function visibleLen(line) {
    var n = 0, i = 0, L = line.length, ESC = 27, BEL = 7;
    while (i < L) {
      var c = line.charCodeAt(i);
      if (c === ESC) {
        i++; if (i >= L) break;
        var c2 = line.charCodeAt(i);
        if (c2 === 91) {            // '[' CSI: run ends at a final byte 0x40-0x7e
          i++; while (i < L) { var a = line.charCodeAt(i); i++; if (a >= 64 && a <= 126) break; }
        } else if (c2 === 93) {     // ']' OSC: run ends at BEL or ESC '\'
          i++; while (i < L) { var b2 = line.charCodeAt(i); if (b2 === BEL) { i++; break; } if (b2 === ESC) { i++; if (i < L && line.charCodeAt(i) === 92) i++; break; } i++; }
        } else { i++; }            // other two-char escape
      } else { n++; i++; }
    }
    return n;
  }
  function computeSrcMaxCols(text) {
    var lines = (text || '').split(String.fromCharCode(10)), max = 0;
    for (var i = 0; i < lines.length; i++) { var w = visibleLen(lines[i]); if (w > max) max = w; }
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

  // relayoutCols (no-wrap): set the wrapper width to the SOURCE's longest line
  // (bounded), then refit so xterm fills it — cols then >= every line so nothing
  // wraps and #term scrolls horizontally. Skips when unchanged so streaming appends
  // don't thrash fit().
  function relayoutCols(force) {
    if (!term || wrapOn || !xw()) return;
    var maxLen = Math.max(baseCols, Math.min(srcMaxCols || baseCols, 1000));
    if (!force && maxLen === lastMaxLen) return;
    lastMaxLen = maxLen;
    var inner = el().clientWidth - 12;     // 12 = #term padding
    var contentPx = Math.max(inner, Math.ceil(maxLen * cellPx));
    xw().style.width = contentPx + 'px';
    try { fit.fit(); } catch (e) {}        // xterm now fills the wide wrapper
  }

  // iOS/WebKit renders some symbol codepoints as COLOR EMOJI (ignoring the ANSI
  // color) — Claude Code's U+23FA "BLACK CIRCLE FOR RECORD" tool-call markers show
  // up as the red record-button emoji instead of a clean ANSI-colored dot like in a
  // real terminal. Swap them for U+25CF "BLACK CIRCLE" (no emoji presentation), so
  // the dot renders as a text glyph tinted by the surrounding SGR color (1:1 width).
  var DOT_REC = String.fromCodePoint(0x23fa), DOT_CIRCLE = String.fromCodePoint(0x25cf);
  function normalizeGlyphs(t) { return t.indexOf(DOT_REC) === -1 ? t : t.split(DOT_REC).join(DOT_CIRCLE); }

  window.gtmuxWrite = function (rawText) {
    if (!term) return;
    var text = normalizeGlyphs(rawText);
    if (text === lastText) return;
    if (userScrolling) { pending = text; return; } // hold writes while a gesture is active
    var prev = lastText;
    lastText = text;
    srcMaxCols = computeSrcMaxCols(text);  // no-wrap extent tracks the source, not the wrapped buffer
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

  // gtmuxCursor mirrors the pane's text cursor (capture-pane doesn't carry it, so
  // the Mac sends column x + Up = rows above the last line + visible). It's drawn as
  // a DECORATION (an overlay cell), NOT by moving xterm's cursor — moving the cursor
  // would make the next incremental append write in the wrong place. The decoration
  // is anchored to a marker "up" rows above the write cursor (the content's last
  // line), so it stays correct as you scroll and despite the phone's row count.
  var curDeco = null, curMarker = null;
  // Registering a decoration leaves the WebGL base layer blank until the next
  // repaint. In no-wrap mode the per-poll relayoutCols() refit repaints it, but
  // wrap mode never refits — so the whole terminal showed BLACK on first open
  // until you toggled wrap (which forced a relayout). Always nudge a repaint after
  // the cursor changes, so wrap renders too. requestAnimationFrame lets the
  // decoration lay out first.
  function repaint() {
    try {
      requestAnimationFrame(function () {
        try { term.refresh(0, Math.max(0, term.rows - 1)); } catch (e) {}
      });
    } catch (e) { try { term.refresh(0, Math.max(0, term.rows - 1)); } catch (e2) {} }
  }
  window.gtmuxCursor = function (c) {
    if (!term) return;
    if (curDeco) { try { curDeco.dispose(); } catch (e) {} curDeco = null; }
    if (curMarker) { try { curMarker.dispose(); } catch (e) {} curMarker = null; }
    if (!c || c.visible === false) { repaint(); return; }  // hidden (alt-screen TUI) → no marker
    try {
      curMarker = term.registerMarker(-(c.up | 0));   // up rows above the write cursor
      if (!curMarker) { repaint(); return; }
      curDeco = term.registerDecoration({
        marker: curMarker, x: c.x | 0, width: 1, height: 1, backgroundColor: '#bbc1ff',
      });
      // belt-and-suspenders: also style the element on render (some renderers ignore
      // backgroundColor in the options).
      if (curDeco && curDeco.onRender) {
        curDeco.onRender(function (el) {
          el.style.background = '#bbc1ff';
          el.style.opacity = '0.85';
        });
      }
    } catch (e) {}
    repaint();
  };

  window.gtmuxConfig = function (opts) {
    if (!term || !opts) return;
    if (typeof opts.fontSize === 'number') term.options.fontSize = opts.fontSize;
    if (typeof opts.wrap === 'boolean') wrapOn = opts.wrap;
    if (opts.theme) term.options.theme = opts.theme;
    relayout();
    // force a re-render at the new wrap/size (gtmuxWrite skips an unchanged text).
    var t = lastText; lastText = ''; if (t) window.gtmuxWrite(t);
    // Self-heal after a toggle, same as boot: a single relayout occasionally isn't
    // enough for the renderer to repaint at the new layout, so stage a couple more.
    [60, 250, 600].forEach(function (ms) { setTimeout(relayout, ms); });
  };

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
`;

const html = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
<style>${fontFace}${css}
  html,body{margin:0;padding:0;height:100%;background:#17171a;overflow:hidden}
  /* #term scrolls HORIZONTALLY, bounded by #xwrap's explicit width; the
     .xterm-viewport scrolls VERTICALLY. No touch-action lock — iOS already routes a
     gesture to the nested scroller that can move in that direction, and an explicit
     pan-x lock made horizontal swipes hard to even start. */
  #term{position:absolute;inset:0;padding:6px;overflow-x:hidden;overflow-y:hidden;-webkit-overflow-scrolling:touch}
  #xwrap{position:relative;height:100%;width:100%}
  .xterm-viewport{overflow-x:hidden !important;overflow-y:scroll !important;-webkit-overflow-scrolling:touch}
  /* clip the (absolutely-positioned) render canvas to the logical screen width: on
     retina iOS it renders wider and, as an absolute descendant, would expand
     #term's scrollWidth → unbounded horizontal scroll. Verified via Playwright. */
  .xterm .xterm-screen{overflow:hidden}
</style></head><body><div id="term"><div id="xwrap"></div></div>
<script>${xtermJs}</script><script>${fitJs}</script><script>${uni11Js}</script><script>${canvasJs}</script>
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
