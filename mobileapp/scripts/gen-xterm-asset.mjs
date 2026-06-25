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
  var term, fit, wrapOn = true, lastText = '', baseCols = 80;
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
    // Vertical scroll is the .xterm-viewport's native momentum scroll (smooth); a
    // custom scrollLines handler just fought it and felt janky, so it's gone.
  }

  // visibleLen: a line's on-screen width, ignoring ANSI escapes (capture-pane -e
  // embeds SGR codes, which must NOT count toward the no-wrap width).
  function visibleLen(l) {
    return l.replace(/\\x1b\\[[0-9;?]*[A-Za-z]/g, '').length;
  }

  // relayout: wrap on → fit cols to width (long lines wrap); wrap off → widen cols
  // to the content's longest line so nothing wraps, with horizontal scroll.
  function relayout() {
    if (!term || !el()) return;
    try { fit.fit(); } catch (e) {}       // sets rows + the fill-the-width col count
    baseCols = term.cols;                 // the floor: cols that exactly fill the screen
    if (wrapOn) {
      el().style.overflowX = 'hidden';
    } else {
      el().style.overflowX = 'auto';
      relayoutCols();
    }
  }

  // relayoutCols (no-wrap only): set cols to the content's longest visible line (but
  // never below fill-width), so narrow content doesn't scroll and wide content
  // scrolls to exactly the last column — not into empty space.
  function relayoutCols() {
    if (!term || wrapOn) return;
    var maxLen = 0;
    lastText.split('\\n').forEach(function (l) { var n = visibleLen(l); if (n > maxLen) maxLen = n; });
    var cols = Math.max(baseCols, Math.min(maxLen || baseCols, 500));
    if (cols !== term.cols) { try { term.resize(cols, term.rows); } catch (e) {} }
  }

  window.gtmuxWrite = function (text) {
    if (!term || text === lastText) return;
    var prev = lastText;
    lastText = text;
    // Append-only (the common case: more output at the bottom): write just the new
    // tail — no reset, so no flash, cheap, and xterm natively keeps your scroll
    // position (only follows the bottom if you were already there). Holds until the
    // captured scrollback window starts dropping its top lines.
    if (prev && text.length > prev.length && text.lastIndexOf(prev, 0) === 0) {
      if (!wrapOn) relayoutCols();
      term.write(text.slice(prev.length));
      return;
    }
    // Full change (scrolled-off / TUI redraw) → repaint, preserving the reader's
    // distance from the bottom so a manual scroll-up isn't yanked back.
    var b = term.buffer.active;
    var wasBottom = b.viewportY >= b.baseY;
    var fromBottom = b.baseY - b.viewportY;
    if (!wrapOn) relayoutCols();
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
  #term{position:absolute;inset:0;padding:6px;overflow:hidden}
  /* native momentum scroll both ways (xterm clips horizontal by default) */
  .xterm-viewport{overflow-x:auto !important;overflow-y:scroll !important;-webkit-overflow-scrolling:touch}
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
