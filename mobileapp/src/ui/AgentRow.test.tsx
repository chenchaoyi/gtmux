import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {AgentRow} from './AgentRow';
import {paletteFor} from './theme';
import {Agent} from '../api/types';
import {TestIds} from '../constants/testIds';
import {Haptics} from '../native/haptics';
import {LONG_PRESS_MS} from './pressFeel';

// A chip squeezed to zero width is not gone — it is its own padding and border, which is
// a small empty pill. One shipped on the rate-limited row (2026-09-05): the error message
// had `flexShrink: 0` and took the whole line, so the branch chip beside it rendered as a
// blank white lozenge with nothing in it.

const row = (o: Partial<Agent> = {}): Agent =>
  ({
    pane_id: '%3',
    agent: 'Claude Code',
    source: 'tmux',
    status: 'idle',
    loc: 'mp:0.0',
    task: 'MP analysis dev',
    project: 'mp',
    ...o,
  }) as Agent;

function render(agent: Agent) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <AgentRow agent={agent} pal={paletteFor('light')} lang="en" onPress={() => {}} />,
    );
  });
  return tree!;
}

const texts = (tree: renderer.ReactTestRenderer): string[] =>
  tree.root
    .findAllByType(Text)
    .flatMap(n => ([] as unknown[]).concat(n.props.children as unknown[]))
    .filter(c => typeof c === 'string') as string[];

test('a pane in trouble gives its whole second line to saying why', () => {
  const t = render(
    row({
      status: 'idle',
      error: true,
      error_text: "You've hit your weekly limit · resets Aug 28 at 11pm",
      branch: 'main',
    }),
  );
  const said = texts(t);
  expect(said).toContain("You've hit your weekly limit · resets Aug 28 at 11pm");
  expect(said).not.toContain('main'); // no chip to crush, so no empty pill
});

test('an ordinary row still wears its branch', () => {
  expect(texts(render(row({branch: 'main'})))).toContain('main');
});

test('the second line yields before the chip does', () => {
  // The direction of the shrink IS the bug: whichever of the two is too long has to
  // ellipsize inside its own budget, and a chip that can be shrunk to nothing will be.
  const t = render(row({branch: 'feat/issues-sort-by'}));
  const chip = t.root
    .findAllByType(Text)
    .find(n => n.props.children === 'feat/issues-sort-by')!;
  const flat = (style: unknown): Array<Record<string, unknown>> =>
    ([] as unknown[]).concat(style as unknown[]).filter(Boolean) as Array<Record<string, unknown>>;
  const secondary = t.root
    .findAllByType(Text)
    .find(n => flat(n.props.style).some(s => s?.fontSize === 12.5))!;
  expect(flat(secondary.props.style).some(s => s?.flexShrink === 1)).toBe(true);
  // The chip's own wrapper must not be shrinkable.
  const wrapper = chip.parent!;
  expect(flat(wrapper.props.style).some(s => s?.flexShrink === 0)).toBe(true);
});

// The long press had no feedback at all: the row dimmed to 0.6 and 350ms later a sheet
// appeared from off-screen (user report, 2026-09-10). Three things were missing; these
// pin all three, because any one of them missing puts the complaint straight back.
describe('what a long press feels like', () => {
  const A = row();
  const mount = (onLongPress?: () => void) => {
    let tree: renderer.ReactTestRenderer | undefined;
    act(() => {
      tree = renderer.create(
        <AgentRow agent={A} pal={paletteFor('dark')} lang="en" onPress={() => {}} onLongPress={onLongPress} />,
      );
    });
    return tree!.root.findByProps({testID: `${TestIds.agent.row}-${A.pane_id}`});
  };

  it('warms the engine when the finger goes down, so the impact lands with the gesture', () => {
    const spy = jest.spyOn(Haptics, 'arm');
    const el = mount(() => {});
    act(() => el.props.onPressIn());
    expect(spy).toHaveBeenCalled();
    spy.mockRestore();
  });

  it('fires ONE impact at recognition — the system s own language for a long press', () => {
    const hit = jest.spyOn(Haptics, 'hit');
    const opened = jest.fn();
    const el = mount(opened);
    act(() => el.props.onLongPress());
    expect(hit).toHaveBeenCalledTimes(1);
    expect(opened).toHaveBeenCalledTimes(1);
    hit.mockRestore();
  });

  it('does none of it on a row that has no long press to give', () => {
    const arm = jest.spyOn(Haptics, 'arm');
    const el = mount(undefined);
    act(() => el.props.onPressIn());
    expect(arm).not.toHaveBeenCalled();
    // And the gesture itself must be absent, not a handler that does nothing.
    expect(el.props.onLongPress).toBeUndefined();
    arm.mockRestore();
  });

  it('holds for the shared delay rather than a number of its own', () => {
    expect(mount(() => {}).props.delayLongPress).toBe(LONG_PRESS_MS);
  });
});
