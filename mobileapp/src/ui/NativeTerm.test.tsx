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

function mount(onLiveEdge: (b: boolean) => void) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(<NativeTerm text={'line\n'.repeat(80)} onLiveEdge={onLiveEdge} />);
  });
  return tree!;
}

test('a poll re-publishes the edge state, so a stale host repairs itself', () => {
  const seen: boolean[] = [];
  const tree = mount(b => seen.push(b));
  const view = tree.root.findAllByType(ScrollView)[0];

  // Drag up into history: the host is told to fold.
  act(() => {
    view.props.onScroll(scrollEvent({content: 9000, offset: 100, view: 700}));
  });
  expect(seen).toContain(false);

  // The host's copy goes stale (it resets its own collapse on a mode change, and nothing
  // says the reader came back). The next poll grows the content — which happens several
  // times a second on a live pane — and that alone must restate the truth.
  seen.length = 0;
  act(() => {
    view.props.onContentSizeChange(390, 9200);
  });
  expect(seen).toEqual([false]);
});

test('at the tail it re-publishes the tail, not a fold', () => {
  const seen: boolean[] = [];
  const tree = mount(b => seen.push(b));
  const view = tree.root.findAllByType(ScrollView)[0];
  act(() => {
    view.props.onScroll(scrollEvent({content: 9000, offset: 8300, view: 700})); // gap 0
  });
  seen.length = 0;
  act(() => {
    view.props.onContentSizeChange(390, 9200);
  });
  expect(seen).toEqual([true]);
});
