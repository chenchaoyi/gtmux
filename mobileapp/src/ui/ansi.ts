// Lightweight ANSI/SGR parser (MOBILE §4): tmux `capture-pane -e` output → styled
// spans for native colored <Text>. Offline-friendly (no webview / xterm.js). Maps
// SGR fg colors to a macOS-Terminal-Pro-like palette aligned to theme.ts; ignores
// background + cursor/other escapes. Color encodes terminal output only.

export interface Span {
  text: string;
  color: string;
  bold?: boolean;
}
export type AnsiLine = Span[];

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

function color256(n: number): string {
  if (n < 16) return PALETTE[n];
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
}

function applySGR(style: Style, codes: number[]): Style {
  const s = {...style};
  for (let i = 0; i < codes.length; i++) {
    const c = codes[i];
    if (c === 0) {
      s.color = undefined;
      s.bold = false;
      s.dim = false;
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
      s.color = PALETTE[c - 30];
    } else if (c >= 90 && c <= 97) {
      s.color = PALETTE[c - 90 + 8];
    } else if (c === 38) {
      if (codes[i + 1] === 5) {
        s.color = color256(codes[i + 2] || 0);
        i += 2;
      } else if (codes[i + 1] === 2) {
        s.color = rgb(codes[i + 2] || 0, codes[i + 3] || 0, codes[i + 4] || 0);
        i += 4;
      }
    }
    // 40–49 / 48 (background) and others ignored.
  }
  return s;
}

// eslint-disable-next-line no-control-regex
const SGR_RE = /\u001b\[([0-9;]*)m/g;
// eslint-disable-next-line no-control-regex
const OTHER_ESC = /\u001b\[[0-9;?]*[A-Za-z]|\u001b[()][AB0]/g;

export function parseAnsi(input: string): AnsiLine[] {
  return input.split('\n').map(raw => {
    const spans: AnsiLine = [];
    let style: Style = {};
    let last = 0;
    SGR_RE.lastIndex = 0;
    const push = (text: string) => {
      text = text.replace(OTHER_ESC, '');
      if (!text) return;
      const color = style.color ?? (style.dim ? DIM : BASE);
      spans.push({text, color, bold: style.bold});
    };
    let m: RegExpExecArray | null;
    while ((m = SGR_RE.exec(raw))) {
      push(raw.slice(last, m.index));
      const codes = m[1] === '' ? [0] : m[1].split(';').map(x => parseInt(x, 10) || 0);
      style = applySGR(style, codes);
      last = m.index + m[0].length;
    }
    push(raw.slice(last));
    return spans;
  });
}
