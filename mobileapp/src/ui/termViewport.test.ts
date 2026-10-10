import {parseAnsi} from './ansi';
import {flattenGrid} from './term';
import {sourceGridColumns, termAnchor, termAnchorOffset} from './termViewport';

const grid = (text: string, cols: number) => {
  const lines = parseAnsi(text);
  return flattenGrid(lines, lines, null, cols);
};

test('original columns count display cells, not ANSI bytes or string length', () => {
  expect(sourceGridColumns(parseAnsi('\x1b[42m中文abc\x1b[0m'), undefined, 4)).toBe(7);
  expect(sourceGridColumns(parseAnsi('✅中文'), undefined, 4)).toBe(6);
  expect(sourceGridColumns(parseAnsi('abc'), 189, 50)).toBe(189);
});

test('old wider history and the cursor are preserved when the Mac narrows', () => {
  expect(sourceGridColumns(parseAnsi('x'.repeat(189)), 80, 50)).toBe(189);
  expect(sourceGridColumns(parseAnsi('abc'), 80, 50, 100)).toBe(101);
  for (const width of [undefined, 0, -1, NaN, Infinity]) {
    expect(sourceGridColumns(parseAnsi('x'.repeat(80)), width, 50)).toBe(80);
  }
  expect(sourceGridColumns(parseAnsi('abc'), 20, 50)).toBe(50);
});

test('switching widths preserves the same history line rather than its old pixel offset', () => {
  const text = 'first'.repeat(20) + '\n' + 'second'.repeat(20) + '\nlast';
  const wrapped = grid(text, 30);
  const original = grid(text, 189);
  const anchor = termAnchor(wrapped.rows, 80 + 5 * 20, 20, 80)!;
  expect(termAnchorOffset(anchor, original.rows, 20, 80)).toBe(100);
  const back = termAnchor(original.rows, 100, 20, 80)!;
  expect(termAnchorOffset(back, wrapped.rows, 20, 80)).toBe(160);
});

test('padding, empty buffers and missing anchors do not invent a history position', () => {
  const rows = grid('abc', 30).rows;
  expect(termAnchor(rows, 10, 20, 80)).toBeNull();
  expect(termAnchor([], 100, 20, 80)).toBeNull();
  expect(termAnchorOffset({line: 'missing', fraction: 0}, rows, 20, 80)).toBeNull();
});
