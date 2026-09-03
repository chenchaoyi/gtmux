import React from 'react';
import {Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';
import {paletteFor} from './theme';

// The reader's "show me this turn's steps" choice is keyed by the turn's index in the
// transcript. Indexes are stable while turns are only APPENDED — but the server drops the
// oldest turns to bound the payload, and every drop shifts the rest down by one.
//
// If the choice is positional, it then belongs to a different turn than the one that was
// tapped: steps stand open on a turn nobody touched and closed on the one that was.
const agent = {pane_id: '%1', agent: 'Claude Code', status: 'working'} as unknown as Agent;

const turn = (n: number): TranscriptTurn => ({
  prompt: `question ${n}`,
  response: `answer ${n}`,
  segments: [{text: `answer ${n}`, steps: [{title: `step-of-${n}`, detail: ''}]} as never],
  time: `2026-09-04T0${n}:00:00Z`,
});

const render = (turns: TranscriptTurn[]) => {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <ChatView agent={agent} lines={[]} status="idle" fontSize={13} lang="en" turns={turns} pal={paletteFor('dark')} loading={false} />,
    );
  });
  return tree!;
};

/** The step toggles on screen, in order, as their labels read. */
const toggles = (t: renderer.ReactTestRenderer) =>
  t.root
    .findAllByType(TouchableOpacity)
    .filter(n => {
      const texts = n.findAllByType(Text).flatMap(x => ([] as unknown[]).concat(x.props.children as unknown[]));
      return texts.some(c => typeof c === 'string' && /steps?$/.test(c.trim()));
    });

test('an expansion follows its turn when the server drops an older one', () => {
  const four = [turn(1), turn(2), turn(3), turn(4)];
  const t = render(four);
  const before = toggles(t);
  expect(before.length).toBe(4);

  // Open the steps on the LAST turn (question 4).
  act(() => {
    before[3].props.onPress();
  });
  const openCountBefore = t.root
    .findAllByType(Text)
    .filter(n => ([] as unknown[]).concat(n.props.children as unknown[]).some(c => c === 'step-of-4')).length;
  expect(openCountBefore).toBeGreaterThan(0);

  // The oldest turn falls out of the payload and a new one arrives: same four turns on
  // screen, every index shifted by one.
  act(() => {
    t.update(
      <ChatView
        agent={agent}
        lines={[]}
        status="idle"
        fontSize={13}
        lang="en"
        turns={[turn(2), turn(3), turn(4), turn(5)]}
        droppedTurns={1}
        pal={paletteFor('dark')}
        loading={false}
      />,
    );
  });

  const openNow = (name: string) =>
    t.root
      .findAllByType(Text)
      .filter(n => ([] as unknown[]).concat(n.props.children as unknown[]).some(c => c === name)).length > 0;

  // The choice must still belong to question 4 — and must NOT have migrated to another.
  expect({turn4: openNow('step-of-4'), turn5: openNow('step-of-5'), turn3: openNow('step-of-3')}).toEqual({
    turn4: true,
    turn5: false,
    turn3: false,
  });
});
