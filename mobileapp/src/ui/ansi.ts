// Lightweight ANSI/SGR parser (MOBILE §4): tmux `capture-pane -e` output → styled
// spans for native colored <Text>. Offline-friendly (no webview / xterm.js). Maps
// SGR fg colors to a macOS-Terminal-Pro-like palette aligned to theme.ts; ignores
// background + cursor/other escapes. Color encodes terminal output only.

export interface Span {
  text: string;
  color: string;
  bold?: boolean;
  bg?: string; // background color — only emitted when opts.bg is set (terminal grid)
  href?: string; // OSC 8 hyperlink target for this span's text (a terminal link)
}
export type AnsiLine = Span[];

// parseAnsi options. Defaults preserve the original Chat-view behavior exactly
// (fixed PALETTE, no background) so existing callers/tests are unaffected; the
// native terminal grid passes a theme palette + opts.bg for full fidelity.
export interface AnsiOpts {
  palette?: string[]; // 16 hex colors (theme); falls back to PALETTE per-index
  base?: string; // default foreground
  dim?: string; // dim default foreground
  bg?: boolean; // parse background SGR (40–49 / 48 / 100–107) into Span.bg
}

const BASE = '#D6D6DA'; // default foreground (MOBILE §4)
const DIM = '#9AA0A8'; // raised a notch so dim/comment lines stay readable on the dark bg (REVIEW #2)

// 16-color palette: 0–7 normal, 8–15 bright. Status hues reused (green #22C55E,
// red #EF4444, cyan #06B6D4) so the terminal reads consistently with the badges.
const PALETTE = [
  '#5A5A60', '#EF4444', '#22C55E', '#EAB308', '#61AFEF', '#D946EF', '#06B6D4', '#D6D6DA',
  '#8E8E93', '#F87171', '#4ADE80', '#FDE047', '#93C5FD', '#E879F9', '#67E8F9', '#FFFFFF',
];

function rgb(r: number, g: number, b: number): string {
  const h = (x: number) => Math.max(0, Math.min(255, x)).toString(16).padStart(2, '0');
  return `#${h(r)}${h(g)}${h(b)}`;
}

function color256(n: number, pal: string[]): string {
  if (n < 16) return pal[n];
  if (n >= 232) {
    const v = 8 + (n - 232) * 10;
    return rgb(v, v, v);
  }
  const c = n - 16;
  const ch = (x: number) => (x === 0 ? 0 : 55 + x * 40);
  return rgb(ch(Math.floor(c / 36)), ch(Math.floor((c % 36) / 6)), ch(c % 6));
}

interface Style {
  color?: string;
  bold?: boolean;
  dim?: boolean;
  bg?: string;
}

// applySGR folds a code list into the running style. `pal` is the active 16-color
// palette; `wantBg` enables background parsing (40–49 / 48 / 100–107).
function applySGR(style: Style, codes: number[], pal: string[], wantBg: boolean): Style {
  const s = {...style};
  for (let i = 0; i < codes.length; i++) {
    const c = codes[i];
    if (c === 0) {
      s.color = undefined;
      s.bold = false;
      s.dim = false;
      s.bg = undefined;
    } else if (c === 1) {
      s.bold = true;
    } else if (c === 2) {
      s.dim = true;
    } else if (c === 22) {
      s.bold = false;
      s.dim = false;
    } else if (c === 39) {
      s.color = undefined;
    } else if (c >= 30 && c <= 37) {
      s.color = pal[c - 30];
    } else if (c >= 90 && c <= 97) {
      s.color = pal[c - 90 + 8];
    } else if (c === 38) {
      if (codes[i + 1] === 5) {
        s.color = color256(codes[i + 2] || 0, pal);
        i += 2;
      } else if (codes[i + 1] === 2) {
        s.color = rgb(codes[i + 2] || 0, codes[i + 3] || 0, codes[i + 4] || 0);
        i += 4;
      }
    } else if (wantBg && c === 49) {
      s.bg = undefined;
    } else if (wantBg && c >= 40 && c <= 47) {
      s.bg = pal[c - 40];
    } else if (wantBg && c >= 100 && c <= 107) {
      s.bg = pal[c - 100 + 8];
    } else if (wantBg && c === 48) {
      if (codes[i + 1] === 5) {
        s.bg = color256(codes[i + 2] || 0, pal);
        i += 2;
      } else if (codes[i + 1] === 2) {
        s.bg = rgb(codes[i + 2] || 0, codes[i + 3] || 0, codes[i + 4] || 0);
        i += 4;
      }
    } else if (!wantBg && c === 48) {
      // skip the 256/truecolor bg payload so it isn't mis-read as fg codes.
      if (codes[i + 1] === 5) i += 2;
      else if (codes[i + 1] === 2) i += 4;
    }
    // when !wantBg: 40–47 / 49 / 100–107 ignored (Chat-view default).
  }
  return s;
}

// One tokenizer for the escape sequences that carry rendering meaning: an SGR
// (color) sequence, or an OSC 8 hyperlink OPEN (ESC ] 8 ; params ; URI ST) /
// CLOSE (ESC ] 8 ; ; ST). ST = BEL (\u0007) or ESC-backslash (\u001b\\). Group
// 1 = SGR codes; group 2 = the OSC 8 URI (only that alternative sets it).
// eslint-disable-next-line no-control-regex
const TOKEN =
  /\u001b\[([0-9;]*)m|\u001b\]8;[^;]*;([^\u0007\u001b]*)(?:\u0007|\u001b\\)|\u001b\]8;;(?:\u0007|\u001b\\)/g;
// Sequences with NO rendering effect that must be STRIPPED from displayed text
// (else they leak as garbage): CSI cursor/erase moves, charset selects, any OTHER
// OSC (title-set etc.), and a stray lone ESC/BEL left by a partial capture.
// eslint-disable-next-line no-control-regex
const STRIP =
  /\u001b\[[0-9;?]*[A-Za-z]|\u001b[()][AB0]|\u001b\][^\u0007\u001b]*(?:\u0007|\u001b\\)?|[\u0007\u001b]/g;

export function parseAnsi(input: string, opts?: AnsiOpts): AnsiLine[] {
  const pal = opts?.palette && opts.palette.length >= 16 ? opts.palette : PALETTE;
  const base = opts?.base ?? BASE;
  const dim = opts?.dim ?? DIM;
  const wantBg = !!opts?.bg;
  return input.split('\n').map(raw => {
    const spans: AnsiLine = [];
    let style: Style = {};
    let href: string | undefined;
    let last = 0;
    TOKEN.lastIndex = 0;
    const push = (text: string) => {
      text = text.replace(STRIP, '');
      if (!text) return;
      const color = style.color ?? (style.dim ? dim : base);
      spans.push({text, color, bold: style.bold, bg: wantBg ? style.bg : undefined, href});
    };
    let m: RegExpExecArray | null;
    while ((m = TOKEN.exec(raw))) {
      push(raw.slice(last, m.index));
      if (m[1] !== undefined) {
        // SGR (color).
        const codes = m[1] === '' ? [0] : m[1].split(';').map(x => parseInt(x, 10) || 0);
        style = applySGR(style, codes, pal, wantBg);
      } else if (m[2] !== undefined) {
        href = m[2] || undefined; // OSC 8 OPEN - text until the close is a link
      } else {
        href = undefined; // OSC 8 CLOSE
      }
      last = m.index + m[0].length;
    }
    push(raw.slice(last));
    return spans;
  });
}
