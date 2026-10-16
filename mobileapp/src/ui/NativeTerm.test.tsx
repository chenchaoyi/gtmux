import React from 'react';
import {Platform, ScrollView, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NativeTerm} from './NativeTerm';
import {TestIds} from '../constants/testIds';
import {PAD, cellWidthFor} from './term';

// The host (DetailScreen) folds its top chrome while you browse scrollback, and it learns
// whether you are at the live tail ONLY from onLiveEdge. So the contract is not "announce
// a transition" — it is "keep the host's copy true".
//
// Seen once on 2026-09-04, with a screenshot: the jump-to-bottom control was showing (this
// component's other output from the same handler, so it knew the reader had left the tail)
// while the header, the neighbour strip and the segmented control were all still up. The
// finger was already up by then, so no further scroll frame was coming to correct it.
//
// Rendering the real component is the point: a test against a hand-rolled copy of the
// handler would have passed the whole time.
// Advance explicit paint frames, not wall-clock timers: feedback must be committed
// while the old grid is still present, then the requested grid and its scroll restore.
const frames = new Map<number, Parameters<typeof requestAnimationFrame>[0]>();
let frameID = 0;
beforeEach(() => {
  jest.spyOn(globalThis, 'requestAnimationFrame').mockImplementation(callback => { frames.set(++frameID, callback); return frameID; });
  jest.spyOn(globalThis, 'cancelAnimationFrame').mockImplementation(id => { if (typeof id === 'number') frames.delete(id); });
});
const paint = () => act(() => {
  for (const [id, callback] of [...frames]) { frames.delete(id); callback(0); }
});
const settleLayout = () => { for (let i = 0; i < 4; i++) paint(); };
const mounted: renderer.ReactTestRenderer[] = [];
function create(node: React.ReactElement) {
  const tree = renderer.create(node);
  mounted.push(tree);
  return tree;
}
afterEach(() => {
  act(() => { for (const tree of mounted.splice(0)) tree.unmount(); });
  frames.clear();
  jest.restoreAllMocks();
});

const scrollEvent = (over: {content: number; offset: number; view: number}) => ({
  nativeEvent: {
    contentOffset: {x: 0, y: over.offset},
    contentSize: {width: 390, height: over.content},
    layoutMeasurement: {width: 390, height: over.view},
  },
});

function mount(onLiveEdge: (gap: number) => void) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = create(<NativeTerm text={'line\n'.repeat(80)} onLiveEdge={onLiveEdge} />);
  });
  return tree!;
}

const vertical = (tree: renderer.ReactTestRenderer) => tree.root.findAllByType(ScrollView).find(view => !view.props.horizontal)!;

const textContent = (node: renderer.ReactTestInstance | string): string => typeof node === 'string' ? node : node.children.map(textContent).join('');

function rowsText(tree: renderer.ReactTestRenderer): string[] {
  return tree.root.findByProps({testID: TestIds.detail.termRows}).findAllByType(Text).filter(row => row.props.suppressHighlighting).map(textContent);
}

test('toolbar-requested layout preserves rows without adding controls over the terminal', () => {
  const capture = `  ${'中'.repeat(70)} command\n  next physical row`;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={capture} paneCols={189} lang="zh" />); });
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalWrap})).toHaveLength(0);
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalOriginal})).toHaveLength(0);
  expect(rowsText(tree).length).toBeGreaterThan(2);
  const scroller = vertical(tree);
  act(() => tree.update(<NativeTerm text={capture} paneCols={189} layout="original" />));
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalReflow}).length).toBeGreaterThan(0);
  expect(rowsText(tree).length).toBeGreaterThan(2);
  settleLayout();
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalReflow})).toHaveLength(0);
  expect(rowsText(tree)).toEqual(capture.split('\n'));
  expect(vertical(tree)).toBe(scroller);
  expect(tree.root.findAllByType(ScrollView).find(view => view.props.horizontal)?.props.scrollEnabled).toBe(true);
  act(() => tree.update(<NativeTerm text={capture} paneCols={189} layout="fit" />));
  settleLayout();
  expect(rowsText(tree).length).toBeGreaterThan(2);
  expect(vertical(tree)).toBe(scroller);
});

test('original mode survives polling and retains selection text and cursor geometry', () => {
  const capture = `\x1b[42m+${'x'.repeat(188)}\x1b[0m\n› reply\n\n`;
  let tree!: renderer.ReactTestRenderer;
  const props = {paneCols: 189, cursor: {x: 8, up: 2, visible: true}};
  act(() => { tree = create(<NativeTerm text={capture} {...props} />); });
  act(() => tree.update(<NativeTerm text={capture} {...props} layout="original" />));
  settleLayout();
  const overlay = tree.root.findAll(node => node.props.fontName === 'Menlo' && typeof node.props.text === 'string')[0];
  expect(overlay.props.text).toBe(`+${'x'.repeat(188)}\n› reply`);
  expect(rowsText(tree).length).toBe(2);
  act(() => tree.update(<NativeTerm text={capture.replace('reply', 'ready')} {...props} layout="original" />));
  expect(overlay.props.text).toContain('ready');
  act(() => overlay.props.onSelectionActive({nativeEvent: {active: true}}));
  act(() => tree.update(<NativeTerm text={capture.replace('reply', 'ready')} {...props} layout="fit" />));
  expect(rowsText(tree)).toHaveLength(2);
  act(() => overlay.props.onSelectionActive({nativeEvent: {active: false}}));
  settleLayout();
  expect(rowsText(tree).length).toBeGreaterThan(2);
});

test('a toolbar layout request anchors the same captured line while reading history', () => {
  const text = Array.from({length: 80}, (_, i) => `${i} ${'x'.repeat(186)}`).join('\n');
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<NativeTerm text={text} paneCols={189} />);
    mounted.push(tree);
  });
  const scrollTo = vertical(tree).props.ref.current.scrollTo as jest.Mock;
  scrollTo.mockClear();
  const rowHeight = tree.root.findAll(n => n.props.fontName === 'Menlo')[0].props.rowHeight;
  const rowsPerLine = rowsText(tree).length / 80;
  const y = PAD + 5 * rowsPerLine * rowHeight;
  act(() => {
    vertical(tree).props.onScrollBeginDrag({});
    vertical(tree).props.onScroll(scrollEvent({content: 30000, offset: y, view: 700}));
  });
  act(() => tree.update(<NativeTerm text={text} paneCols={189} layout="original" />));
  settleLayout();
  act(() => vertical(tree).props.onContentSizeChange(1400, 10000));
  expect(scrollTo).toHaveBeenCalledWith({y: PAD + 5 * rowHeight, animated: false});
});

test('a poll re-publishes the edge state, so a stale host repairs itself', () => {
  const seen: number[] = [];
  const tree = mount(g => seen.push(g));
  const view = vertical(tree);

  // Drag up into history: the host is told how far away the tail is.
  act(() => {
    view.props.onScroll(scrollEvent({content: 9000, offset: 100, view: 700}));
  });
  expect(seen).toEqual([8200]);

  // The host's copy goes stale (it resets its own collapse on a mode change, and nothing
  // says the reader came back). The next poll grows the content — which happens several
  // times a second on a live pane — and that alone must restate the truth.
  seen.length = 0;
  act(() => {
    view.props.onContentSizeChange(390, 9200);
  });
  expect(seen).toEqual([8200]);
});

test('at the tail it re-publishes the tail, not a fold', () => {
  const seen: number[] = [];
  const tree = mount(g => seen.push(g));
  const view = vertical(tree);
  act(() => {
    view.props.onScroll(scrollEvent({content: 9000, offset: 8300, view: 700})); // gap 0
  });
  seen.length = 0;
  act(() => {
    view.props.onContentSizeChange(390, 9200);
  });
  expect(seen).toEqual([0]);
});

// The jump-to-bottom arrow must not announce an arrival it has not made.
//
// It used to report gap=0 the instant it was tapped. The host believed it, unfolded the
// chrome, and the fold's per-frame offset writes cancelled the very scroll animation the
// tap had started — so the chrome flashed and the view went nowhere (2026-09-10). The
// compensation is gone now, but the lie is the part that must not come back: the host acts
// on this report, and the scroll's own frames are what know where the scroll is.
describe('the jump-to-bottom arrow', () => {
  const mountAway = (onLiveEdge: (gap: number) => void) => {
    let t: renderer.ReactTestRenderer;
    act(() => {
      t = create(<NativeTerm text={'line\n'.repeat(80)} onLiveEdge={onLiveEdge} />);
    });
    const sv = vertical(t!);
    // Scroll away from the tail so the control is on screen.
    act(() => {
      sv.props.onScrollBeginDrag({});
      sv.props.onScroll({
        nativeEvent: {contentOffset: {y: 0}, contentSize: {height: 9000}, layoutMeasurement: {height: 600}},
      });
    });
    return t!;
  };

  it('reports nothing when tapped — the arrival reports itself', () => {
    const seen: number[] = [];
    const tree = mountAway(g => seen.push(g));
    seen.length = 0;
    act(() => tree.root.findByProps({testID: TestIds.detail.jumpBottom}).props.onPress());
    expect(seen).toEqual([]);
  });

  it('reports the tail once the scroll actually gets there', () => {
    const seen: number[] = [];
    const tree = mountAway(g => seen.push(g));
    act(() => tree.root.findByProps({testID: TestIds.detail.jumpBottom}).props.onPress());
    seen.length = 0;
    act(() => {
      vertical(tree).props.onScroll({
        nativeEvent: {contentOffset: {y: 8400}, contentSize: {height: 9000}, layoutMeasurement: {height: 600}},
      });
    });
    expect(seen).toEqual([0]);
  });
});

// The chrome floats above this view and covers its top, so the content is padded by the
// chrome's height — without it the OLDEST line can never be scrolled clear.
describe('room for the floating chrome', () => {
  it('pads the content by what the host asks for', () => {
    let t: renderer.ReactTestRenderer;
    act(() => {
      t = create(<NativeTerm text={'a\nb'} topPad={117} />);
    });
    const style = vertical(t!).props.contentContainerStyle;
    expect(JSON.stringify(style)).toContain('"paddingTop":117');
  });

  it('pads nothing when the host has not measured yet', () => {
    let t: renderer.ReactTestRenderer;
    act(() => {
      t = create(<NativeTerm text={'a\nb'} />);
    });
    const style = vertical(t!).props.contentContainerStyle;
    expect(JSON.stringify(style)).not.toContain('paddingTop');
  });
});

// The terminal renders more often than its rows change: twice per poll (the new text, then
// the snapshot it flushes), and on scroll and parent renders. A render that changes no row
// must hand React the SAME row stack, so the ~1200 rows of a long pane are skipped rather
// than rebuilt and compared one by one.
test('a render that changes no row reuses the row stack', () => {
  let tree!: renderer.ReactTestRenderer;
  const text = 'one\ntwo\nthree';
  act(() => {
    tree = create(<NativeTerm text={text} onLiveEdge={() => {}} />);
  });
  const stack = () => tree.root.findAll(n => n.props.testID === TestIds.detail.termRows)[0];
  const before = stack().props;
  act(() => tree.update(<NativeTerm text={text} onLiveEdge={() => {}} />));
  expect(stack().props).toBe(before);
  act(() => tree.update(<NativeTerm text={'one\ntwo\nfour'} onLiveEdge={() => {}} />));
  expect(stack().props).not.toBe(before);
});

// The colour layer's fallback linkified a span's text even when the span had declared a
// non-web OSC 8 link, so a file:// link labelled with a web address became tappable
// (%12, 2026-10-06). Nothing is opened here: Linking is a mock.
describe('links in the colour layer', () => {
  const osc8 = (href: string, label: string) => `\x1b]8;;${href}\x1b\\${label}\x1b]8;;\x1b\\`;
  const tappable = (text: string) => {
    let tree!: renderer.ReactTestRenderer;
    act(() => {
      tree = create(<NativeTerm text={text} />);
    });
    const labels = tree.root
      .findAll(n => typeof n.props.onPress === 'function' && n.props.children !== undefined)
      .map(n => [n.props.children].flat().join(''));
    return labels;
  };

  test('a non-web OSC 8 link stays plain, however its label reads', () => {
    expect(tappable(`see ${osc8('file:///audit-only', 'https://example.invalid/audit')} end\n`)).not.toContain(
      'https://example.invalid/audit',
    );
  });

  test('a web OSC 8 link and a bare URL are still tappable', () => {
    expect(tappable(`see ${osc8('https://ok.example/a', 'the docs')} end\n`)).toContain('the docs');
    expect(tappable('open https://bare.example/x now\n')).toContain('https://bare.example/x');
  });
});

test('Android original mode preserves source rows and enables horizontal reading', () => {
  const previous = Platform.OS;
  Object.defineProperty(Platform, 'OS', {value: 'android', configurable: true});
  try {
    let tree!: renderer.ReactTestRenderer;
    const capture = `  ${'中'.repeat(70)} command\n  next row`;
    act(() => { tree = create(<NativeTerm text={capture} paneCols={189} layout="original" />); });
    const horizontal = tree.root.findAllByType(ScrollView).find(view => view.props.horizontal)!;
    expect(horizontal.props.scrollEnabled).toBe(true);
    const overlay = tree.root.findAllByType(Text).find(node => node.props.selectable)!;
    expect(textContent(overlay)).toBe(capture);
  } finally {
    Object.defineProperty(Platform, 'OS', {value: previous, configurable: true});
  }
});


test('same-height reflow finishes without a size event and keeps live-tail intent', () => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={'short\n'.repeat(40)} />); });
  const scroll = vertical(tree);
  const end = scroll.props.ref.current.scrollToEnd as jest.Mock;
  end.mockClear();
  act(() => tree.update(<NativeTerm text={'short\n'.repeat(40)} layout="original" />));
  paint();
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalReflow}).length).toBeGreaterThan(0);
  expect(end).not.toHaveBeenCalled();
  settleLayout();
  expect(end).toHaveBeenCalledWith({animated: false});
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalReflow})).toHaveLength(0);
  expect(vertical(tree)).toBe(scroll);
});

test('intermediate clamping cannot report history as the live tail during reflow', () => {
  const seen: number[] = [];
  const text = Array.from({length: 50}, (_, i) => `${i} ${'中'.repeat(80)}`).join('\n');
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={text} onLiveEdge={g => seen.push(g)} />); });
  const end = vertical(tree).props.ref.current.scrollToEnd as jest.Mock;
  act(() => {
    vertical(tree).props.onScrollBeginDrag({});
    vertical(tree).props.onScroll(scrollEvent({content: 20000, offset: 500, view: 700}));
    vertical(tree).props.onScrollEndDrag({});
  });
  seen.length = 0;
  end.mockClear();
  act(() => tree.update(<NativeTerm text={text} layout="original" onLiveEdge={g => seen.push(g)} />));
  act(() => {
    vertical(tree).props.onContentSizeChange(1500, 700);
    vertical(tree).props.onScroll(scrollEvent({content: 700, offset: 0, view: 700}));
  });
  expect(seen).toEqual([]);
  settleLayout();
  expect(end).not.toHaveBeenCalled();
  expect(tree.root.findByProps({testID: TestIds.detail.jumpBottom})).toBeTruthy();
});

test('round-trip layout keeps the character area and partial row in a long CJK line', () => {
  const text = ['header', '中'.repeat(100), ...Array.from({length: 50}, (_, i) => `tail ${i}`)].join('\n');
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={text} paneCols={220} />); });
  const rows = rowsText(tree);
  const rowHeight = tree.root.findAll(n => n.props.fontName === 'Menlo')[0].props.rowHeight;
  const firstWrap = rows.findIndex(line => line.startsWith('中'));
  const y = PAD + (firstWrap + 2.25) * rowHeight;
  act(() => {
    vertical(tree).props.onScrollBeginDrag({});
    vertical(tree).props.onScroll(scrollEvent({content: 10000, offset: y, view: 700}));
    vertical(tree).props.onScrollEndDrag({});
  });
  const scrollTo = vertical(tree).props.ref.current.scrollTo as jest.Mock;
  act(() => tree.update(<NativeTerm text={text} paneCols={220} layout="original" />));
  settleLayout();
  expect(scrollTo).toHaveBeenLastCalledWith({y: PAD + (firstWrap + 0.25) * rowHeight, animated: false});
  const horizontal = tree.root.findAllByType(ScrollView).find(view => view.props.horizontal)!;
  const toX = horizontal.props.ref.current.scrollTo as jest.Mock;
  const expectedCells = rows[firstWrap].length * 2 * 2;
  const effectiveSize = tree.root.findAll(n => n.props.fontName === 'Menlo')[0].props.fontSize;
  expect(toX.mock.calls.filter(([arg]) => arg.x !== undefined).slice(-1)[0][0]).toEqual({x: expectedCells * cellWidthFor(effectiveSize), animated: false});
  act(() => tree.update(<NativeTerm text={text} paneCols={220} layout="fit" />));
  settleLayout();
  expect(scrollTo).toHaveBeenLastCalledWith({y, animated: false});
});

test('superseded layout requests and unmount cancel obsolete frames', () => {
  const text = 'x'.repeat(190);
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={text} />); });
  const initial = rowsText(tree);
  act(() => tree.update(<NativeTerm text={text} layout="original" />));
  paint();
  act(() => tree.update(<NativeTerm text={text} layout="fit" />));
  settleLayout();
  expect(rowsText(tree)).toEqual(initial);
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalReflow})).toHaveLength(0);
  act(() => tree.update(<NativeTerm text={text} layout="original" />));
  expect(frames.size).toBeGreaterThan(0);
  act(() => tree.unmount());
  mounted.splice(mounted.indexOf(tree), 1);
  expect(frames.size).toBe(0);
});


test('a request reversed after grid commit retains the original history anchor', () => {
  const text = Array.from({length: 80}, (_, i) => `${i} ${'x'.repeat(186)}`).join('\n');
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={text} paneCols={189} />); });
  const oldRows = rowsText(tree);
  const rowHeight = tree.root.findAll(n => n.props.fontName === 'Menlo')[0].props.rowHeight;
  const y = PAD + 7.25 * rowHeight;
  act(() => {
    vertical(tree).props.onScrollBeginDrag({});
    vertical(tree).props.onScroll(scrollEvent({content: 30000, offset: y, view: 700}));
    vertical(tree).props.onScrollEndDrag({});
    tree.update(<NativeTerm text={text} paneCols={189} layout="original" />);
  });
  paint(); paint(); // New grid committed, native restoration still pending.
  expect(rowsText(tree).length).toBeLessThan(oldRows.length);
  act(() => tree.update(<NativeTerm text={text} paneCols={189} layout="fit" />));
  settleLayout();
  expect(rowsText(tree)).toEqual(oldRows);
  expect(vertical(tree).props.ref.current.scrollTo).toHaveBeenLastCalledWith({y, animated: false});
  expect(frames.size).toBe(0);
});

test('polls are held until layout restoration finishes, then following resumes', () => {
  const first = 'x'.repeat(190);
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={first} />); });
  act(() => tree.update(<NativeTerm text={first} layout="original" />));
  act(() => tree.update(<NativeTerm text={`${first}\nnew output`} layout="original" />));
  expect(rowsText(tree).join('')).not.toContain('new output');
  settleLayout();
  expect(rowsText(tree)).toEqual([first, 'new output']);
  expect(tree.root.findAllByProps({testID: TestIds.detail.terminalReflow})).toHaveLength(0);
});
