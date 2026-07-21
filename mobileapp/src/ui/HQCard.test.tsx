import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {HQCard, fleetHeadline} from './HQCard';
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

// The intelligence headline REPLACES the fleet pips (hq-meta-layer): a synthesized
// chief-of-staff conclusion, not anonymous dots.
test('fleetHeadline synthesizes a chief-of-staff conclusion', () => {
  const workers = [
    mk({session: 'api', status: 'waiting'}),
    mk({session: 'web', status: 'working'}),
    mk({session: 'docs', status: 'idle'}),
  ];
  // one waiter → names it + how many others are normal
  expect(fleetHeadline(mk({status: 'working'}), workers, false)).toBe('api needs you · 2 others normal');
  expect(fleetHeadline(mk({status: 'working'}), workers, true)).toBe('api 在等你拍板 · 其余 2 个正常');
  // quiet fleet
  expect(fleetHeadline(mk({status: 'working'}), [mk({status: 'idle'})], false)).toMatch(/all normal/);
  // HQ itself waiting supersedes the fleet line
  expect(fleetHeadline(mk({status: 'waiting'}), workers, false)).toBe('needs your call');
});

test('no fleet pips are rendered anymore', () => {
  const hq = mk({role: 'supervisor', status: 'working'});
  const tree = render(hq, [hq, mk({session: 'api', status: 'waiting'}), mk({status: 'idle'})]);
  expect(tree.root.findAllByProps({testID: 'hq-pip-square'})).toHaveLength(0);
  expect(tree.root.findAllByProps({testID: 'hq-pip-dot'})).toHaveLength(0);
});

test('HQ itself waiting → amber card border + red subtitle (整卡琥珀)', () => {
  const hq = mk({role: 'supervisor', status: 'waiting'});
  const tree = render(hq, [hq, mk({status: 'idle'})]);
  const card = tree.root.findByProps({testID: 'radar-hq-card'});
  expect(flatStyle(card).borderColor).toBe(ERRORED_COLOR);
  const sub = tree.root.findAllByType(require('react-native').Text as any)
    .find(t => t.props.children === 'needs your call');
  expect(flatStyle(sub)).toMatchObject({color: StatusColor.waiting});
});

test('quiet fleet → neutral border, "all normal" subtitle, dim', () => {
  const hq = mk({role: 'supervisor', status: 'working'});
  const tree = render(hq, [hq, mk({status: 'idle'})]);
  const card = tree.root.findByProps({testID: 'radar-hq-card'});
  expect(flatStyle(card).borderColor).toBe(pal.divider);
  const sub = tree.root.findAllByType(require('react-native').Text as any)
    .find(t => typeof t.props.children === 'string' && t.props.children.includes('all normal'));
  expect(sub).toBeTruthy();
  expect(flatStyle(sub)).toMatchObject({color: pal.fg2});
});

test('a waiting worker turns the subtitle amber (attention-worthy)', () => {
  const hq = mk({role: 'supervisor', status: 'working'});
  const tree = render(hq, [hq, mk({session: 'api', status: 'waiting'})]);
  const sub = tree.root.findAllByType(require('react-native').Text as any)
    .find(t => typeof t.props.children === 'string' && t.props.children.includes('needs you'));
  expect(flatStyle(sub)).toMatchObject({color: ERRORED_COLOR});
});
