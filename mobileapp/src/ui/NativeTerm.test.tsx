import React from 'react';
import {Platform, ScrollView, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NativeTerm} from './NativeTerm';
import {TestIds} from '../constants/testIds';

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
const mounted: renderer.ReactTestRenderer[] = [];
function create(node: React.ReactElement) {
  const tree = renderer.create(node);
  mounted.push(tree);
  return tree;
}
afterEach(() => act(() => { for (const tree of mounted.splice(0)) tree.unmount(); }));

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

test('original mode preserves captured rows and exposes the current choice', () => {
  const capture = `  ${'中'.repeat(70)} command\n  next physical row`;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = create(<NativeTerm text={capture} paneCols={189} lang="zh" />); });
  const wrap = tree.root.findByProps({testID: TestIds.detail.terminalWrap});
  const original = tree.root.findByProps({testID: TestIds.detail.terminalOriginal});
  expect(wrap.props.accessibilityState.selected).toBe(true);
  expect(rowsText(tree).length).toBeGreaterThan(2);
  const scroller = vertical(tree);
  act(() => original.props.onPress());
  expect(original.props.accessibilityState.selected).toBe(true);
  expect(wrap.props.accessibilityState.selected).toBe(false);
  expect(rowsText(tree)).toEqual(capture.split('\n'));
  expect(vertical(tree)).toBe(scroller);
  expect(tree.root.findAllByType(ScrollView).find(view => view.props.horizontal)?.props.scrollEnabled).toBe(true);
  act(() => wrap.props.onPress());
  expect(rowsText(tree).length).toBeGreaterThan(2);
  expect(vertical(tree)).toBe(scroller);
});

test('original mode survives polling and retains selection text and cursor geometry', () => {
  const capture = `\x1b[42m+${'x'.repeat(188)}\x1b[0m\n› reply\n\n`;
  let tree!: renderer.ReactTestRenderer;
  const props = {paneCols: 189, cursor: {x: 8, up: 2, visible: true}};
  act(() => { tree = create(<NativeTerm text={capture} {...props} />); });
  act(() => tree.root.findByProps({testID: TestIds.detail.terminalOriginal}).props.onPress());
  const overlay = tree.root.findAll(node => node.props.fontName === 'Menlo' && typeof node.props.text === 'string')[0];
  expect(overlay.props.text).toBe(`+${'x'.repeat(188)}\n› reply`);
  expect(rowsText(tree).length).toBe(2);
  act(() => tree.update(<NativeTerm text={capture.replace('reply', 'ready')} {...props} />));
  expect(tree.root.findByProps({testID: TestIds.detail.terminalOriginal}).props.accessibilityState.selected).toBe(true);
  expect(overlay.props.text).toContain('ready');
  act(() => overlay.props.onSelectionActive({nativeEvent: {active: true}}));
  expect(tree.root.findByProps({testID: TestIds.detail.terminalWrap}).props.disabled).toBe(true);
  act(() => tree.root.findByProps({testID: TestIds.detail.terminalWrap}).props.onPress());
  expect(tree.root.findByProps({testID: TestIds.detail.terminalOriginal}).props.accessibilityState.selected).toBe(true);
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
    act(() => { tree = create(<NativeTerm text={capture} paneCols={189} />); });
    const original = tree.root.findByProps({testID: TestIds.detail.terminalOriginal});
    act(() => original.props.onPress());
    const horizontal = tree.root.findAllByType(ScrollView).find(view => view.props.horizontal)!;
    expect(horizontal.props.scrollEnabled).toBe(true);
    const overlay = tree.root.findAllByType(Text).find(node => node.props.selectable)!;
    expect(textContent(overlay)).toBe(capture);
    expect(original.props.accessibilityState.selected).toBe(true);
  } finally {
    Object.defineProperty(Platform, 'OS', {value: previous, configurable: true});
  }
});
