import React from 'react';
import {ScrollView, StyleSheet, Text, View} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import type {AnsiLine} from './ansi';
import {LIVE_CHUNK_MAX_COLS, LIVE_CHUNK_MAX_ROWS, LiveProgress, partitionLiveRows} from './LiveProgress';
import {charCells} from './term';

function renderPreview(lines: AnsiLine[]) {
  return <LiveProgress avatar={null} label="Working" lines={lines} fontSize={13} fontFamily="Menlo"
    lang="en" open onToggle={jest.fn()} heightLimit={720} onResize={jest.fn()} />;
}

test('LVR-006 1,000-line Live scrollback partitions into bounded column chunks below GPU texture limits', () => {
  const initial: AnsiLine[] = Array.from({length: 70}, (_, i) => [{text: `line-${i}`, color: '#D6D6DA'}]);
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(renderPreview(initial)); });
  try {
    const horizontal = () => tree.root.findAllByType(ScrollView).find(n => !!n.props.horizontal)!;
    const column = () => horizontal().findAllByType(View).find(n => n.props.testID === 'chat-live-column')!;
    const chunks = () => column().findAllByType(Text).filter(n => n.props.testID === 'chat-live-chunk');
    expect(StyleSheet.flatten(column().props.style)).toMatchObject({flexDirection: 'column'});
    expect(chunks()).toHaveLength(3);
    const firstChunkText = chunks()[0].findAllByType(Text)[1];

    // Appending fresh lines with freshly parsed row arrays keeps unchanged leading chunks memoized.
    const appended: AnsiLine[] = Array.from({length: 75}, (_, i) => [{text: `line-${i}`, color: '#D6D6DA'}]);
    act(() => { tree.update(renderPreview(appended)); });
    expect(chunks()).toHaveLength(3);
    expect(chunks()[0].findAllByType(Text)[1]).toBe(firstChunkText);

    // At the full 1,000-line scrollback cap, every chunk stays <= 32 rows (576pt = 1,728px @3x << 16,384px).
    const full: AnsiLine[] = Array.from({length: 1000}, (_, i) => [{text: `row-${i}`, color: '#D6D6DA'}]);
    act(() => { tree.update(renderPreview(full)); });
    const renderedChunks = chunks();
    expect(renderedChunks).toHaveLength(Math.ceil(1000 / LIVE_CHUNK_MAX_ROWS));
    for (const chunk of renderedChunks) {
      expect(chunk.props.selectable).toBe(true);
      const plain = chunk.findAllByType(Text).slice(1).map(n => String(n.props.children)).join('');
      const rowCount = plain.split('\n').length;
      expect(rowCount).toBeLessThanOrEqual(LIVE_CHUNK_MAX_ROWS);
      expect(rowCount * 18 * 3).toBeLessThan(16384);
    }

    // Holding scrollback preserves the frozen chunks across incoming updates until resumed at the bottom.
    const vertical = tree.root.findAllByType(ScrollView).find(n => !n.props.horizontal)!;
    act(() => vertical.props.onScrollBeginDrag());
    act(() => vertical.props.onScrollEndDrag({nativeEvent: {
      contentOffset: {y: 120}, layoutMeasurement: {height: 220}, contentSize: {height: 18000},
    }}));
    act(() => { tree.update(renderPreview([[{text: 'fresh-tail', color: '#D6D6DA'}]])); });
    const heldText = column().findAllByType(Text).map(n => n.props.children).flat().join(' ');
    expect(heldText).toContain('row-999');
    expect(heldText).not.toContain('fresh-tail');
    act(() => vertical.props.onMomentumScrollEnd({nativeEvent: {
      contentOffset: {y: 17780}, layoutMeasurement: {height: 220}, contentSize: {height: 18000},
    }}));
    const resumedText = column().findAllByType(Text).map(n => n.props.children).flat().join(' ');
    expect(resumedText).toContain('fresh-tail');
  } finally {
    act(() => tree.unmount());
  }
});

test('LVR-007 normal wide rows stay unwrapped while pathological overwide lines split at 320 cells', () => {
  const wide200: AnsiLine = [{text: 'W'.repeat(200), color: '#27C7E6', bold: true}];
  const cjkOverwide: AnsiLine = [
    {text: '头'.repeat(100), color: '#22C55E', bold: true},
    {text: '🚀'.repeat(80) + 'tail-marker', color: '#F59E0B', bold: false},
  ];
  const partitioned = partitionLiveRows([wide200, cjkOverwide]);
  expect(partitioned).toHaveLength(1);
  // 200 cols stays 1 row; 100*2 + 80*2 + 11 = 371 cells wraps into 2 rows at 320 cells -> 3 rows total.
  expect(partitioned[0]).toHaveLength(3);
  expect(partitioned[0][0]).toEqual(wide200);
  for (const row of partitioned[0]) {
    const cells = row.reduce((sum, s) => sum + [...s.text].reduce((n, ch) => n + charCells(ch), 0), 0);
    expect(cells).toBeLessThanOrEqual(LIVE_CHUNK_MAX_COLS);
  }

  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(renderPreview([wide200, cjkOverwide])); });
  try {
    const chunk = tree.root.findAllByType(Text).find(n => n.props.testID === 'chat-live-chunk')!;
    const spans = chunk.findAllByType(Text).slice(1);
    expect(spans[0].props.style).toEqual({color: '#27C7E6', fontWeight: '700'});
    expect(String(spans[0].props.children)).toContain('W'.repeat(200) + '\n');
    const joined = spans.map(s => String(s.props.children)).join('');
    expect(joined).toContain('tail-marker');
    expect(joined).not.toContain('\uFFFD');
  } finally {
    act(() => tree.unmount());
  }
});
