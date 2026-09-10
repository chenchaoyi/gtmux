import React from 'react';
import {ScrollView} from 'react-native';
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
    tree = renderer.create(<NativeTerm text={'line\n'.repeat(80)} onLiveEdge={onLiveEdge} />);
  });
  return tree!;
}

test('a poll re-publishes the edge state, so a stale host repairs itself', () => {
  const seen: number[] = [];
  const tree = mount(g => seen.push(g));
  const view = tree.root.findAllByType(ScrollView)[0];

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
  const view = tree.root.findAllByType(ScrollView)[0];
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
      t = renderer.create(<NativeTerm text={'line\n'.repeat(80)} onLiveEdge={onLiveEdge} />);
    });
    const sv = t!.root.findByType(ScrollView);
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
      tree.root.findByType(ScrollView).props.onScroll({
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
      t = renderer.create(<NativeTerm text={'a\nb'} topPad={117} />);
    });
    const style = t!.root.findByType(ScrollView).props.contentContainerStyle;
    expect(JSON.stringify(style)).toContain('"paddingTop":117');
  });

  it('pads nothing when the host has not measured yet', () => {
    let t: renderer.ReactTestRenderer;
    act(() => {
      t = renderer.create(<NativeTerm text={'a\nb'} />);
    });
    const style = t!.root.findByType(ScrollView).props.contentContainerStyle;
    expect(JSON.stringify(style)).not.toContain('paddingTop');
  });
});
