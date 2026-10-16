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

export interface TermAnchor {line: string; fraction: number; cell?: number; rowOffset?: number}
type AnchorRow = {key: string; spans?: AnsiLine};
const rowCells = (row: AnchorRow): number => {
  let cells = 0;
  for (const span of row.spans ?? []) for (const char of span.text) cells += charCells(char);
  return cells;
};
const lineKey = (key: string): string => key.slice(0, key.lastIndexOf('\u0000'));

/** Keep the same captured line visible when its number of phone rows changes. */
export function termAnchor(rows: AnchorRow[], offset: number, rowHeight: number, topPad: number, horizontalCells = 0): TermAnchor | null {
  if (offset < topPad || !rows.length) return null;
  const index = Math.min(rows.length - 1, Math.floor((offset - topPad) / rowHeight));
  const line = lineKey(rows[index].key);
  let start = index;
  while (start > 0 && lineKey(rows[start - 1].key) === line) start--;
  let end = index + 1;
  while (end < rows.length && lineKey(rows[end].key) === line) end++;
  const before = rows.slice(start, index).reduce((sum, row) => sum + rowCells(row), 0);
  return {line, fraction: (index - start) / (end - start),
    cell: rows[index].spans ? before + Math.max(0, Math.floor(horizontalCells)) : undefined,
    rowOffset: Math.max(0, Math.min(0.999, ((offset - topPad) / rowHeight) - index))};
}

export function termAnchorOffset(anchor: TermAnchor, rows: AnchorRow[], rowHeight: number, topPad: number): number | null {
  const start = rows.findIndex(row => lineKey(row.key) === anchor.line);
  if (start < 0) return null;
  let end = start + 1;
  while (end < rows.length && lineKey(rows[end].key) === anchor.line) end++;
  let index = start + Math.min(end - start - 1, Math.floor(anchor.fraction * (end - start)));
  if (anchor.cell !== undefined && rows[start].spans) {
    index = start;
    let cells = 0;
    while (index + 1 < end && cells + rowCells(rows[index]) <= anchor.cell) {
      cells += rowCells(rows[index]);
      index++;
    }
  }
  return topPad + (index + (anchor.rowOffset ?? 0)) * rowHeight;
}
