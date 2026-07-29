import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {StyleSheet} from 'react-native';
import {HQDisc, discState} from './HQDisc';
import {Agent} from '../api/types';
import {StatusColor, paletteFor} from './theme';

const pal = paletteFor('dark');
const mk = (o: Partial<Agent>): Agent =>
  ({agent: 'Claude Code', source: 'tmux', status: 'idle', ...o} as Agent);
const sup = (o: Partial<Agent> = {}): Agent => mk({role: 'supervisor', status: 'idle', ...o});

// HQDisc loads its persisted position from AsyncStorage (a microtask) on mount and
// gates its first render on it. Track every tree and UNMOUNT it after each test — an
// un-unmounted component's pending promise resolves after the Jest environment tears
// down, which throws "import after teardown" (flaky, only bit in CI).
const trees: renderer.ReactTestRenderer[] = [];
afterEach(async () => {
  await act(async () => {
    trees.forEach(t => t.unmount());
  });
  trees.length = 0;
});

async function render(opts: {hq?: Agent; agents?: Agent[]; resourceWarn?: string; onOpen?: () => void}) {
  let tree: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(
      <HQDisc
        hq={opts.hq}
        agents={opts.agents ?? []}
        pal={pal}
        lang="en"
        resourceWarn={opts.resourceWarn}
        onOpen={opts.onOpen ?? (() => {})}
      />,
    );
  });
  // The disc renders null until its persisted position loads (AsyncStorage). Drain that
  // load with a macrotask flush so `ready` is true before we assert — a single create
  // act doesn't reliably settle the promise → re-render chain (green locally, flaky in CI).
  await act(async () => {
    await new Promise<void>(r => setTimeout(() => r(), 0));
  });
  trees.push(tree!);
  return tree!;
}

const ringColor = (tree: renderer.ReactTestRenderer): string =>
  StyleSheet.flatten(tree.root.findByProps({testID: 'radar-hq-disc-ring'}).props.style).borderColor;
const badgeText = (tree: renderer.ReactTestRenderer): string | null => {
  const texts = tree.root.findAllByType('Text' as any).map(n => n.props.children);
  const hit = texts.find(c => c === '!' || c === '⚠' || c === '?' || (typeof c === 'string' && /^\d+$/.test(c)));
  return (hit as string) ?? null;
};

describe('discState (priority-ordered)', () => {
  it('resolves each state by priority', () => {
    expect(discState(undefined, 0, false)).toBe('absent'); // no HQ wins over everything
    expect(discState(sup({status: 'waiting'}), 3, true)).toBe('hqCall'); // HQ's own call is top
    expect(discState(sup({status: 'idle'}), 2, true)).toBe('needsYou'); // workers outrank resource
    expect(discState(sup({status: 'idle'}), 0, true)).toBe('resource');
    expect(discState(sup({status: 'working'}), 0, false)).toBe('working');
    expect(discState(sup({status: 'idle'}), 0, false)).toBe('normal');
  });
});

describe('HQDisc ring + badge per state', () => {
  it('not started (no HQ) → grey ring + "?" + explainer affordance', async () => {
    const tree = await render({hq: undefined, agents: [mk({status: 'idle'})]});
    expect(ringColor(tree)).toBe(pal.fg3);
    expect(badgeText(tree)).toBe('?');
  });

  it('all normal → green ring, no badge', async () => {
    const tree = await render({hq: sup(), agents: [mk({status: 'idle'}), mk({status: 'working'})]});
    expect(ringColor(tree)).toBe(StatusColor.idle);
    expect(badgeText(tree)).toBeNull();
  });

  it('HQ working → cyan ring', async () => {
    const tree = await render({hq: sup({status: 'working'}), agents: [mk({status: 'idle'})]});
    expect(ringColor(tree)).toBe(StatusColor.working);
    expect(badgeText(tree)).toBeNull();
  });

  it('a worker needs you → red ring + count badge', async () => {
    const tree = await render({hq: sup(), agents: [mk({status: 'waiting'}), mk({status: 'waiting'}), mk({status: 'idle'})]});
    expect(ringColor(tree)).toBe(StatusColor.waiting);
    expect(badgeText(tree)).toBe('2');
  });

  it('resource bottleneck → red ring + ⚠ (when nothing needs a decision)', async () => {
    const tree = await render({hq: sup(), agents: [mk({status: 'idle'})], resourceWarn: 'disk 3% free'});
    expect(ringColor(tree)).toBe(StatusColor.waiting);
    expect(badgeText(tree)).toBe('⚠');
  });

  it('HQ itself needs you → red ring + "!" (outranks a worker + resource)', async () => {
    const tree = await render({hq: sup({status: 'waiting'}), agents: [mk({status: 'waiting'})], resourceWarn: 'x'});
    expect(ringColor(tree)).toBe(StatusColor.waiting);
    expect(badgeText(tree)).toBe('!');
  });

  it('shows the HQ wordmark + speaks the state via the accessibility label', async () => {
    const tree = await render({hq: sup(), agents: [mk({status: 'idle'})]});
    const texts = tree.root.findAllByType('Text' as any).map(n => n.props.children);
    expect(texts).toContain('HQ');
    const label = tree.root.findByProps({testID: 'radar-hq-disc'}).props.accessibilityLabel;
    expect(label).toContain('gtmux HQ');
    expect(label).toContain('all normal');
  });
});
