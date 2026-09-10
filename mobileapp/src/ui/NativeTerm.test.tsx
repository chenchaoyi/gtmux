import React from 'react';
import {ScrollView} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NativeTerm} from './NativeTerm';

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

// The host cannot see a finger; the child has to say so.
//
// Folding resizes this view, and holding the content still through that means writing
// contentOffset — which loses to a live gesture, so the scroll appears to stick at the
// fold point (2026-09-10). The host waits for `moving` to go false, which makes this
// report the load-bearing half: a child that always says "not moving" puts the stall
// straight back with nothing to notice it.
describe('reporting whether a gesture is running', () => {
  const mount = (onLiveEdge: (gap: number, moving: boolean) => void) => {
    let t: renderer.ReactTestRenderer;
    act(() => {
      t = renderer.create(<NativeTerm text={'a\nb\nc'} onLiveEdge={onLiveEdge} />);
    });
    return t!.root.findByType(ScrollView);
  };
  const scroll = (sv: {props: Record<string, (e: unknown) => void>}, y: number) =>
    act(() => {
      sv.props.onScroll({
        nativeEvent: {contentOffset: {y}, contentSize: {height: 4000}, layoutMeasurement: {height: 600}},
      });
    });

  it('says moving while a finger is down', () => {
    const seen: boolean[] = [];
    const sv = mount((_g, m) => seen.push(m));
    act(() => sv.props.onScrollBeginDrag({}));
    scroll(sv as never, 100);
    expect(seen).toContain(true);
  });

  it('says still once the finger lifts with no momentum', () => {
    const seen: boolean[] = [];
    const sv = mount((_g, m) => seen.push(m));
    act(() => sv.props.onScrollBeginDrag({}));
    scroll(sv as never, 100);
    seen.length = 0;
    act(() => sv.props.onScrollEndDrag({nativeEvent: {velocity: {y: 0}}}));
    expect(seen).toEqual([false]); // reported once, at the end — nothing else will carry it
  });

  it('keeps saying moving through a flick, until momentum ends', () => {
    // A fold mid-glide is the same fight one frame later.
    const seen: boolean[] = [];
    const sv = mount((_g, m) => seen.push(m));
    act(() => sv.props.onScrollBeginDrag({}));
    act(() => sv.props.onScrollEndDrag({nativeEvent: {velocity: {y: 2.5}}}));
    seen.length = 0;
    scroll(sv as never, 300);
    expect(seen).toEqual([true]);
    seen.length = 0;
    act(() => sv.props.onMomentumScrollEnd({nativeEvent: {velocity: {y: 0}}}));
    expect(seen).toContain(false);
  });
});
