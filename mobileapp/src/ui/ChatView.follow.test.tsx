import React from 'react';
import {ScrollView} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';

// Following the live tail belongs to a reader who is AT the tail.
//
// A new turn used to scroll to the bottom unconditionally AND declare the view at the
// bottom while doing it — during a live conversation, which is precisely when turns
// arrive. It pulled you out of the history you had scrolled up to read, and it moved the
// view's idea of the edge without telling the host, so the folded chrome and the view
// disagreed until some later scroll frame settled it. That disagreement is a large part
// of "对话进行中的过程会更不可控".

const agent = {pane_id: '%1', agent: 'Claude Code', status: 'working', loc: 'x:0.0'} as unknown as Agent;
const turn = (n: number): TranscriptTurn => ({prompt: `p${n}`, response: `r${n}`, time: ''});

function view(turns: TranscriptTurn[], onLiveEdge: (gap: number) => void) {
  return (
    <ChatView
      agent={agent}
      lines={[]}
      status="working"
      fontSize={13}
      pal={paletteFor('dark')}
      lang="en"
      turns={turns}
      loading={false}
      onLiveEdge={onLiveEdge}
    />
  );
}

const scrollEvent = (gap: number) => ({
  nativeEvent: {
    contentOffset: {x: 0, y: 1000},
    contentSize: {width: 390, height: 1000 + 700 + gap},
    layoutMeasurement: {width: 390, height: 700},
  },
});

test('a new turn does not drag a reader out of history, nor tell the host it did', () => {
  const seen: number[] = [];
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(view([turn(1)], g => seen.push(g)));
  });
  const sv = tree!.root.findAllByType(ScrollView)[0];

  // DRAG up into history. The drag matters: leaving the tail only counts under a finger,
  // so that a viewport change (the host revealing its chrome) cannot pass for one.
  act(() => {
    sv.props.onScrollBeginDrag();
    sv.props.onScroll(scrollEvent(4000));
    sv.props.onScrollEndDrag();
  });
  expect(seen[seen.length - 1]).toBe(4000);

  // A turn lands while you are up there.
  seen.length = 0;
  act(() => {
    tree!.update(view([turn(1), turn(2)], g => seen.push(g)));
  });
  expect(seen).toEqual([]); // no follow, and no claim to the host that we are at the tail
  act(() => tree!.unmount());
});

test('a new turn DOES follow for a reader sitting at the tail', () => {
  const seen: number[] = [];
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(view([turn(1)], g => seen.push(g)));
  });
  const sv = tree!.root.findAllByType(ScrollView)[0];
  act(() => {
    sv.props.onScroll(scrollEvent(0));
  });
  seen.length = 0;
  act(() => {
    tree!.update(view([turn(1), turn(2)], g => seen.push(g)));
  });
  expect(seen).toEqual([0]); // still pinned, and the host is told so
  act(() => tree!.unmount());
});

test('the host revealing its chrome does not count as leaving the tail', () => {
  // Revealing costs ~117pt of viewport, which is past the 60pt "at the tail" mark. Read
  // as a departure it stops the pin mid-reveal and raises the jump-to-bottom arrow under
  // a reader who never moved. No finger, no departure.
  const seen: number[] = [];
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(view([turn(1)], g => seen.push(g)));
  });
  const sv = tree!.root.findAllByType(ScrollView)[0];
  act(() => {
    sv.props.onScroll(scrollEvent(0));
    sv.props.onScroll(scrollEvent(117)); // the chrome came back; nobody scrolled
  });
  seen.length = 0;
  act(() => {
    tree!.update(view([turn(1), turn(2)], g => seen.push(g)));
  });
  expect(seen).toEqual([0]); // still following
  act(() => tree!.unmount());
});
