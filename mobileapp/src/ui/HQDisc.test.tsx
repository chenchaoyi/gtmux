import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {StyleSheet} from 'react-native';
import {HQDisc} from './HQDisc';
import {Agent} from '../api/types';
import {ERRORED_COLOR, StatusColor, paletteFor} from './theme';

const pal = paletteFor('dark');
const mk = (o: Partial<Agent>): Agent =>
  ({agent: 'Claude Code', source: 'tmux', status: 'idle', ...o} as Agent);

function render(hq: Agent, agents: Agent[]) {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<HQDisc hq={hq} agents={agents} pal={pal} lang="en" onPress={() => {}} />);
  });
  return tree!;
}

// The ring border color encodes the fleet state (铁律 color = status).
const ringColor = (tree: renderer.ReactTestRenderer): string =>
  StyleSheet.flatten(tree.root.findByProps({testID: 'radar-hq-disc-ring'}).props.style).borderColor;

// The needs-you badge text, or null when the disc is quiet.
const badgeText = (tree: renderer.ReactTestRenderer): string | null => {
  const texts = tree.root.findAllByType('Text' as any).map(n => n.props.children);
  const hit = texts.find(c => c === '!' || (typeof c === 'string' && /^\d+$/.test(c)));
  return (hit as string) ?? null;
};

describe('HQDisc ring + badge', () => {
  it('all normal → green ring, no badge', () => {
    const tree = render(mk({role: 'supervisor', status: 'idle'}), [mk({status: 'idle'}), mk({status: 'working'})]);
    expect(ringColor(tree)).toBe(StatusColor.idle);
    expect(badgeText(tree)).toBeNull();
  });

  it('a worker needs you → amber ring + count badge', () => {
    const tree = render(mk({role: 'supervisor', status: 'idle'}), [
      mk({status: 'waiting'}),
      mk({status: 'waiting'}),
      mk({status: 'idle'}),
    ]);
    expect(ringColor(tree)).toBe(ERRORED_COLOR);
    expect(badgeText(tree)).toBe('2');
  });

  it('HQ itself needs you → red ring + "!" badge (its own call outranks a worker)', () => {
    const tree = render(mk({role: 'supervisor', status: 'waiting'}), [mk({status: 'waiting'})]);
    expect(ringColor(tree)).toBe(StatusColor.waiting);
    expect(badgeText(tree)).toBe('!');
  });

  it('speaks the intelligence headline via the accessibility label', () => {
    const tree = render(mk({role: 'supervisor', status: 'idle'}), [mk({status: 'idle'})]);
    const label = tree.root.findByProps({testID: 'radar-hq-disc'}).props.accessibilityLabel;
    expect(label).toContain('gtmux HQ');
    expect(label).toContain('all normal');
  });
});
