import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Modal, PanResponder, Platform, StyleSheet} from 'react-native';
import {HQDisc, HQShortcut, discState} from './HQDisc';
import {AnchoredMenu} from './AnchoredMenu';
import {Haptics} from '../native/haptics';
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

function render(opts: {hq?: Agent; agents?: Agent[]; resourceCritical?: boolean; onOpen?: () => void; onShortcut?: (page: HQShortcut) => void; lang?: string}) {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <HQDisc
        hq={opts.hq}
        agents={opts.agents ?? []}
        pal={pal}
        lang={opts.lang ?? "en"}
        resourceCritical={opts.resourceCritical}
        onShortcut={opts.onShortcut}
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


describe('HQ shortcut gestures', () => {
  let spy: jest.SpyInstance;
  let hit: jest.SpyInstance;
  beforeEach(() => { jest.useFakeTimers(); spy = jest.spyOn(PanResponder, 'create'); hit = jest.spyOn(Haptics, 'hit'); });
  afterEach(() => { spy.mockRestore(); hit.mockRestore(); jest.useRealTimers(); });
  const gesture = () => spy.mock.calls[spy.mock.calls.length - 1][0];
  const g = (dx = 0, dy = 0) => ({dx, dy});
  const disc = (tree: renderer.ReactTestRenderer) => tree.root.findByProps({testID: 'radar-hq-disc'});
  const menu = (tree: renderer.ReactTestRenderer) => tree.root.findByType(AnchoredMenu);
  const dismiss = (tree: renderer.ReactTestRenderer) => menu(tree).findByType(Modal).props.onDismiss();

  test('a short tap opens HQ only; releasing cancels the hold timer', () => {
    const onOpen = jest.fn(), onShortcut = jest.fn();
    const tree = render({hq: sup(), onOpen, onShortcut});
    const r = gesture();
    act(() => { r.onPanResponderGrant(); jest.advanceTimersByTime(200); r.onPanResponderRelease(null, g()); jest.advanceTimersByTime(1000); });
    expect(onOpen).toHaveBeenCalledTimes(1);
    expect(onShortcut).not.toHaveBeenCalled();
    expect(menu(tree).props.visible).toBe(false);
  });

  test.each(['usage', 'knowledge'] as const)('hold opens the menu; %s waits for native dismissal and suppresses release tap', page => {
    const onOpen = jest.fn(), onShortcut = jest.fn();
    const tree = render({hq: sup(), onOpen, onShortcut, lang: 'zh'});
    const r = gesture();
    act(() => { r.onPanResponderGrant(); jest.advanceTimersByTime(500); r.onPanResponderRelease(null, g()); });
    expect(menu(tree).props.visible).toBe(true);
    expect(hit).toHaveBeenCalledTimes(1);
    expect(onOpen).not.toHaveBeenCalled();
    const item = tree.root.findByProps({testID: `hq-shortcut-menu-${page}`});
    act(() => item.props.onPress());
    expect(menu(tree).props.visible).toBe(false);
    if (Platform.OS === 'ios') expect(onShortcut).not.toHaveBeenCalled();
    act(() => { dismiss(tree); dismiss(tree); });
    expect(onShortcut).toHaveBeenCalledTimes(1);
    expect(onShortcut).toHaveBeenCalledWith(page);
  });

  test('dragging out and back never turns into a hold or tap', () => {
    const onOpen = jest.fn(), onShortcut = jest.fn();
    const tree = render({hq: sup(), onOpen, onShortcut}); const r = gesture();
    act(() => { r.onPanResponderGrant(); r.onPanResponderMove(null, g(12)); r.onPanResponderMove(null, g()); jest.advanceTimersByTime(1000); r.onPanResponderRelease(null, g()); });
    expect(menu(tree).props.visible).toBe(false);
    expect(onOpen).not.toHaveBeenCalled(); expect(onShortcut).not.toHaveBeenCalled();
  });

  test('interruption, cancellation and unmount discard pending shortcuts', () => {
    const onShortcut = jest.fn(); const tree = render({hq: sup(), onShortcut}); const r = gesture();
    act(() => { r.onPanResponderGrant(); r.onPanResponderTerminate(); jest.advanceTimersByTime(1000); });
    expect(menu(tree).props.visible).toBe(false);
    act(() => { r.onPanResponderGrant(); jest.advanceTimersByTime(500); });
    act(() => { menu(tree).props.onClose(); dismiss(tree); });
    expect(onShortcut).not.toHaveBeenCalled();
    const hits = hit.mock.calls.length;
    act(() => r.onPanResponderGrant());
    act(() => tree.unmount());
    act(() => jest.advanceTimersByTime(1000));
    expect(onShortcut).not.toHaveBeenCalled();
    expect(hit).toHaveBeenCalledTimes(hits);
  });

  test('VoiceOver has named resource actions; absent HQ exposes none', () => {
    const onShortcut = jest.fn(); const tree = render({hq: sup(), onShortcut});
    expect(disc(tree).props.accessibilityActions.map((a: {name: string}) => a.name)).toEqual(['usage', 'knowledge']);
    act(() => disc(tree).props.onAccessibilityAction({nativeEvent: {actionName: 'usage'}}));
    expect(onShortcut).toHaveBeenCalledWith('usage');
    const absent = render({onShortcut});
    expect(disc(absent).props.accessibilityActions).toBeUndefined();
    act(() => { gesture().onPanResponderGrant(); jest.advanceTimersByTime(500); });
    expect(menu(absent).props.visible).toBe(false);
    expect(onShortcut).toHaveBeenCalledTimes(1);
  });

  test.each([undefined, sup({pane_id: '%new'})])('changing HQ identity discards a pending native dismissal handoff', nextHQ => {
    const onShortcut = jest.fn(); const tree = render({hq: sup({pane_id: '%old'}), onShortcut});
    act(() => { gesture().onPanResponderGrant(); jest.advanceTimersByTime(500); });
    act(() => tree.root.findByProps({testID: 'hq-shortcut-menu-usage'}).props.onPress());
    act(() => tree.update(<HQDisc hq={nextHQ} agents={[]} pal={pal} lang="en" onOpen={() => {}} onShortcut={onShortcut} />));
    act(() => dismiss(tree));
    expect(onShortcut).not.toHaveBeenCalled();
    expect(menu(tree).props.visible).toBe(false);
  });
});
