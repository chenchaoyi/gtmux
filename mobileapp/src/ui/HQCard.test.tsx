import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {HQCard} from './HQCard';
import {Agent} from '../api/types';
import {ERRORED_COLOR, StatusColor, paletteFor} from './theme';

const pal = paletteFor('dark');
const mk = (o: Partial<Agent>): Agent =>
  ({agent: 'Claude Code', source: 'tmux', status: 'idle', ...o} as Agent);

function render(hq: Agent, agents: Agent[]) {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <HQCard hq={hq} agents={agents} pal={pal} lang="en" onPress={() => {}} />,
    );
  });
  return tree!;
}

const flatStyle = (node: any) =>
  Object.assign({}, ...[node.props.style].flat(Infinity).filter(Boolean));

test('fleet pips: one per worker, supervisor excluded, square = waiting', () => {
  const hq = mk({role: 'supervisor', status: 'working'});
  const tree = render(hq, [
    hq,
    mk({status: 'waiting'}),
    mk({status: 'working'}),
    mk({status: 'idle'}),
  ]);
  const squares = tree.root.findAllByProps({testID: 'hq-pip-square'});
  const dots = tree.root.findAllByProps({testID: 'hq-pip-dot'});
  // findAllByProps matches composite+host pairs — count unique host Views by type
  const hostSquares = squares.filter(n => typeof n.type === 'string');
  const hostDots = dots.filter(n => typeof n.type === 'string');
  expect(hostSquares).toHaveLength(1); // the waiting worker
  expect(hostDots).toHaveLength(2); // working + idle; supervisor never pips itself
  // waiting pip is the SQUARE (radius 2) in the status color; the rest are circles
  expect(flatStyle(hostSquares[0]).borderRadius).toBe(2);
  expect(flatStyle(hostSquares[0]).backgroundColor).toBe(StatusColor.waiting);
  expect(flatStyle(hostDots[0]).borderRadius).toBeGreaterThan(3);
});

test('HQ itself waiting → amber card border + red subtitle (整卡琥珀)', () => {
  const hq = mk({role: 'supervisor', status: 'waiting', task: 'approve the plan'});
  const tree = render(hq, [hq, mk({status: 'idle'})]);
  const card = tree.root.findByProps({testID: 'radar-hq-card'});
  expect(flatStyle(card).borderColor).toBe(ERRORED_COLOR);
  const sub = tree.root.findAllByType(require('react-native').Text as any)
    .find(t => t.props.children === 'approve the plan');
  expect(flatStyle(sub)).toMatchObject({color: StatusColor.waiting});
});

test('quiet fleet → neutral border, default subtitle, no status badge dot', () => {
  const hq = mk({role: 'supervisor', status: 'working', task: ''});
  const tree = render(hq, [hq, mk({status: 'idle'})]);
  const card = tree.root.findByProps({testID: 'radar-hq-card'});
  expect(flatStyle(card).borderColor).toBe(pal.divider);
  const sub = tree.root.findAllByType(require('react-native').Text as any)
    .find(t => t.props.children === 'all sessions normal');
  expect(sub).toBeTruthy();
  expect(flatStyle(sub)).toMatchObject({color: pal.fg2});
});

test('a waiting worker turns the subtitle amber (attention-worthy)', () => {
  const hq = mk({role: 'supervisor', status: 'working', task: ''});
  const tree = render(hq, [hq, mk({status: 'waiting'})]);
  const sub = tree.root.findAllByType(require('react-native').Text as any)
    .find(t => t.props.children === 'all sessions normal');
  expect(flatStyle(sub)).toMatchObject({color: ERRORED_COLOR});
});
