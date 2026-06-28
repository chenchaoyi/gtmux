// markdown.ts — a tiny, dependency-free Markdown subset parser for the chat view
// (agent responses). Pure + tested; rendered by Markdown.tsx. We hand-roll a
// subset (no markdown-it dependency) to stay light on Hermes/RN 0.86 and to fully
// control styling on the always-dark chat surface.
//
// Blocks: heading / paragraph / fenced code / bullet+ordered list / blockquote /
// hr / table (GitHub pipe tables). Inline: **bold**, *italic*, `code`,
// [text](url). Underscore emphasis is intentionally NOT supported — snake_case
// identifiers are common in agent prose and would be mangled into italics.

export type Inline =
  | {t: 'text'; s: string}
  | {t: 'b'; s: string}
  | {t: 'i'; s: string}
  | {t: 'code'; s: string}
  | {t: 'link'; s: string; href: string};

export type Align = 'left' | 'center' | 'right';

export type Block =
  | {t: 'h'; level: number; spans: Inline[]}
  | {t: 'p'; spans: Inline[]}
  | {t: 'code'; lang: string; text: string}
  | {t: 'ul'; items: Inline[][]}
  | {t: 'ol'; items: Inline[][]}
  | {t: 'quote'; spans: Inline[]}
  | {t: 'table'; align: Align[]; header: Inline[][]; rows: Inline[][][]}
  | {t: 'hr'};

// parseInline splits a line of text into styled spans. It repeatedly finds the
// LEFTMOST match among the patterns, emitting the plain text before it.
export function parseInline(s: string): Inline[] {
  const out: Inline[] = [];
  let rest = s;
  while (rest.length > 0) {
    let best: {idx: number; len: number; node: Inline} | null = null;
    const consider = (idx: number, len: number, node: Inline) => {
      if (idx >= 0 && (!best || idx < best.idx)) best = {idx, len, node};
    };
    let m: RegExpExecArray | null;
    if ((m = /`([^`]+)`/.exec(rest))) consider(m.index, m[0].length, {t: 'code', s: m[1]});
    if ((m = /\[([^\]]+)\]\(([^)\s]+)\)/.exec(rest))) consider(m.index, m[0].length, {t: 'link', s: m[1], href: m[2]});
    if ((m = /\*\*([^*]+)\*\*/.exec(rest))) consider(m.index, m[0].length, {t: 'b', s: m[1]});
    if ((m = /\*([^*\n]+)\*/.exec(rest))) consider(m.index, m[0].length, {t: 'i', s: m[1]});
    if (!best) {
      out.push({t: 'text', s: rest});
      break;
    }
    const b: {idx: number; len: number; node: Inline} = best;
    if (b.idx > 0) out.push({t: 'text', s: rest.slice(0, b.idx)});
    out.push(b.node);
    rest = rest.slice(b.idx + b.len);
  }
  return out;
}

const FENCE = /^\s*```(.*)$/;
const HR = /^\s*([-*_])(\s*\1){2,}\s*$/;
const HEADING = /^\s*(#{1,6})\s+(.*)$/;
const QUOTE = /^\s*>\s?/;
const BULLET = /^\s*[-*+]\s+/;
const ORDERED = /^\s*\d+\.\s+/;
// A GitHub pipe-table separator row, e.g. `|---|:--:|--:|` (cells of dashes with
// optional alignment colons). Must contain a pipe, so a bare `---` stays an <hr>.
const TABLE_SEP = /^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$/;

// splitRow splits a `| a | b |` row into trimmed cells (leading/trailing pipes
// dropped), each parsed as inline spans.
function splitRow(line: string): Inline[][] {
  let s = line.trim();
  if (s.startsWith('|')) s = s.slice(1);
  if (s.endsWith('|')) s = s.slice(0, -1);
  return s.split('|').map(c => parseInline(c.trim()));
}

// parseAlign reads a separator row into per-column alignments (`:--`/`:-:`/`--:`).
function parseAlign(line: string): Align[] {
  let s = line.trim();
  if (s.startsWith('|')) s = s.slice(1);
  if (s.endsWith('|')) s = s.slice(0, -1);
  return s.split('|').map(c => {
    const t = c.trim();
    const l = t.startsWith(':');
    const r = t.endsWith(':');
    return l && r ? 'center' : r ? 'right' : 'left';
  });
}

// isTableStart: a header row containing a pipe, immediately followed by a
// separator row — the GitHub pipe-table shape.
function isTableStart(lines: string[], i: number): boolean {
  return (
    lines[i].includes('|') &&
    i + 1 < lines.length &&
    lines[i + 1].includes('|') &&
    TABLE_SEP.test(lines[i + 1])
  );
}

// parseBlocks turns Markdown source into a flat list of blocks (line-based).
export function parseBlocks(src: string): Block[] {
  const lines = src.replace(/\r\n/g, '\n').split('\n');
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];

    const fence = FENCE.exec(line);
    if (fence) {
      const body: string[] = [];
      i++;
      while (i < lines.length && !/^\s*```/.test(lines[i])) {
        body.push(lines[i]);
        i++;
      }
      i++; // skip the closing fence (no-op if EOF)
      blocks.push({t: 'code', lang: fence[1].trim(), text: body.join('\n')});
      continue;
    }
    if (line.trim() === '') {
      i++;
      continue;
    }
    if (HR.test(line)) {
      blocks.push({t: 'hr'});
      i++;
      continue;
    }
    const h = HEADING.exec(line);
    if (h) {
      blocks.push({t: 'h', level: h[1].length, spans: parseInline(h[2].trim())});
      i++;
      continue;
    }
    if (QUOTE.test(line)) {
      const q: string[] = [];
      while (i < lines.length && QUOTE.test(lines[i])) {
        q.push(lines[i].replace(QUOTE, ''));
        i++;
      }
      blocks.push({t: 'quote', spans: parseInline(q.join(' '))});
      continue;
    }
    if (BULLET.test(line)) {
      const items: Inline[][] = [];
      while (i < lines.length && BULLET.test(lines[i])) {
        items.push(parseInline(lines[i].replace(BULLET, '')));
        i++;
      }
      blocks.push({t: 'ul', items});
      continue;
    }
    if (ORDERED.test(line)) {
      const items: Inline[][] = [];
      while (i < lines.length && ORDERED.test(lines[i])) {
        items.push(parseInline(lines[i].replace(ORDERED, '')));
        i++;
      }
      blocks.push({t: 'ol', items});
      continue;
    }
    if (isTableStart(lines, i)) {
      const header = splitRow(lines[i]);
      const align = parseAlign(lines[i + 1]);
      i += 2;
      const rows: Inline[][][] = [];
      while (i < lines.length && lines[i].trim() !== '' && lines[i].includes('|')) {
        rows.push(splitRow(lines[i]));
        i++;
      }
      blocks.push({t: 'table', align, header, rows});
      continue;
    }
    // paragraph: gather consecutive lines until a blank or a block starter.
    const para: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== '' &&
      !/^\s*```/.test(lines[i]) &&
      !HR.test(lines[i]) &&
      !HEADING.test(lines[i]) &&
      !QUOTE.test(lines[i]) &&
      !BULLET.test(lines[i]) &&
      !ORDERED.test(lines[i]) &&
      !isTableStart(lines, i)
    ) {
      para.push(lines[i]);
      i++;
    }
    blocks.push({t: 'p', spans: parseInline(para.join(' '))});
  }
  return blocks;
}
