import {AnsiLine} from './ansi';
import {charCells} from './term';

/** Preserve captured rows, including history wider than today's Mac pane. */
export function sourceGridColumns(lines: AnsiLine[], reported: number | undefined, viewport: number, cursorX = 0): number {
  let widest = 0;
  for (const line of lines) {
    let cells = 0;
    for (const span of line) for (const ch of span.text) cells += charCells(ch);
    widest = Math.max(widest, cells);
  }
  const source = typeof reported === 'number' && Number.isFinite(reported) && reported > 0 ? Math.floor(reported) : 0;
  const cursor = Number.isFinite(cursorX) && cursorX >= 0 ? Math.floor(cursorX) + 1 : 0;
  return Math.max(viewport, source, widest, cursor);
}

export interface TermAnchor {line: string; fraction: number}
const lineKey = (key: string): string => key.slice(0, key.lastIndexOf('\u0000'));

/** Keep the same captured line visible when its number of phone rows changes. */
export function termAnchor(rows: {key: string}[], offset: number, rowHeight: number, topPad: number): TermAnchor | null {
  if (offset < topPad || !rows.length) return null;
  const index = Math.min(rows.length - 1, Math.floor((offset - topPad) / rowHeight));
  const line = lineKey(rows[index].key);
  let start = index;
  while (start > 0 && lineKey(rows[start - 1].key) === line) start--;
  let end = index + 1;
  while (end < rows.length && lineKey(rows[end].key) === line) end++;
  return {line, fraction: (index - start) / (end - start)};
}

export function termAnchorOffset(anchor: TermAnchor, rows: {key: string}[], rowHeight: number, topPad: number): number | null {
  const start = rows.findIndex(row => lineKey(row.key) === anchor.line);
  if (start < 0) return null;
  let end = start + 1;
  while (end < rows.length && lineKey(rows[end].key) === anchor.line) end++;
  return topPad + (start + Math.min(end - start - 1, Math.floor(anchor.fraction * (end - start)))) * rowHeight;
}
