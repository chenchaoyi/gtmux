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

// SYNCHRONOUS render + unmount only. HQDisc renders immediately (no async gate), and
// its AsyncStorage position load is fire-and-forget (setValue on the Animated value,
// never a React state update), so there is nothing to await — using async act() here
// hung under CI timing. Unmount each tree after the test so the pending getItem promise
// (guarded by the effect's `alive` flag) can't touch a torn-down module.
const trees: renderer.ReactTestRenderer[] = [];
afterEach(() => {
  act(() => {
    trees.forEach(t => t.unmount());
  });
  trees.length = 0;
});

function render(opts: {hq?: Agent; agents?: Agent[]; resourceCritical?: boolean; onOpen?: () => void}) {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <HQDisc
        hq={opts.hq}
        agents={opts.agents ?? []}
        pal={pal}
        lang="en"
        resourceCritical={opts.resourceCritical}
        onOpen={opts.onOpen ?? (() => {})}
      />,
    );
  });
  trees.push(tree!);
  return tree!;
}

const ringColor = (tree: renderer.ReactTestRenderer): string =>
  StyleSheet.flatten(tree.root.findByProps({testID: 'radar-hq-disc-ring'}).props.style).borderColor;
const badgeText = (tree: renderer.ReactTestRenderer): string | null => {
  const texts = tree.root.findAllByType('Text' as any).map(n => n.props.children);
  // The resource badge carries a trailing U+FE0E (text-presentation selector), so match
  // the ⚠ base rather than an exact string.
  const hit = texts.find(
    c => c === '!' || c === '?' || (typeof c === 'string' && (c.startsWith('⚠') || /^\d+$/.test(c))),
  );
  return (hit as string) ?? null;
};

describe('discState (priority-ordered)', () => {
  it('resolves each state by priority', () => {
    expect(discState(undefined, 0, false)).toBe('absent'); // no HQ wins over everything
    expect(discState(sup({status: 'waiting'}), 3, true)).toBe('hqCall'); // HQ's own call is top
    expect(discState(sup({status: 'idle'}), 2, true)).toBe('needsYou'); // workers outrank resource
    expect(discState(sup({status: 'idle'}), 0, true)).toBe('resource'); // 3rd arg = a RED-tier bottleneck only
    expect(discState(sup({status: 'working'}), 0, false)).toBe('working');
    expect(discState(sup({status: 'idle'}), 0, false)).toBe('normal');
  });

  it('a soft (non-critical) resource condition does NOT redden the disc', () => {
    // resourceCritical is false for a mere amber — the disc stays on HQ's own state
    // (working/normal), never the red "resource" bottleneck. This is the 37GB-free case.
    expect(discState(sup({status: 'idle'}), 0, false)).toBe('normal');
    expect(discState(sup({status: 'working'}), 0, false)).toBe('working');
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

  it('resource bottleneck (red tier) → red ring + ⚠ (when nothing needs a decision)', async () => {
    const tree = await render({hq: sup(), agents: [mk({status: 'idle'})], resourceCritical: true});
    expect(ringColor(tree)).toBe(StatusColor.waiting);
    expect(badgeText(tree)?.startsWith('⚠')).toBe(true);
  });

  it('a soft resource warn does NOT redden the disc → green, no badge', async () => {
    const tree = await render({hq: sup(), agents: [mk({status: 'idle'})], resourceCritical: false});
    expect(ringColor(tree)).toBe(StatusColor.idle);
    expect(badgeText(tree)).toBeNull();
  });

  it('HQ itself needs you → red ring + "!" (outranks a worker + resource)', async () => {
    const tree = await render({hq: sup({status: 'waiting'}), agents: [mk({status: 'waiting'})], resourceCritical: true});
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
