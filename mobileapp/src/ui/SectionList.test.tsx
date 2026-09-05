import React from 'react';
import {SectionList as RNSectionList} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {SectionList, listEndLabel} from './SectionList';
import {Agent} from '../api/types';
import {paletteFor} from './theme';

// Regression (reported 2026-09-03: "都折叠起来后，没法下拉刷新了"). Collapsing the
// sections leaves a list far shorter than the screen. If the list is sized to its
// content, the blank lower half belongs to the screen BEHIND it, so a pull that starts
// there never reaches a scroll view and pull-to-refresh looks broken — while the same
// pull higher up, over the remaining rows, still works. Reproduced on the simulator
// (e2e/__tests__/radar-refresh-collapsed.test.ts): content ended at 32% of the screen,
// a pull from 40% fired onRefresh zero times.
//
// So the list must FILL the screen, content or not: `flex: 1` on the list itself (not
// flexGrow alone — RN's flexShrink defaults to 0, so a long list would overflow rather
// than scroll inside its parent) and flexGrow on the content container.
function render(collapsed: Set<string>) {
  const agents = [
    {pane_id: '%1', agent: 'Claude Code', status: 'idle', session: 'a'},
    {pane_id: '%2', agent: 'Claude Code', status: 'working', session: 'b'},
  ] as unknown as Agent[];
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <SectionList
        agents={agents}
        pal={paletteFor('dark')}
        lang="en"
        onPressAgent={() => {}}
        onLongPressAgent={() => {}}
        refreshing={false}
        onRefresh={() => {}}
        collapsed={collapsed as Set<never>}
        onToggle={() => {}}
      />,
    );
  });
  return tree!;
}

const grows = (style: unknown): boolean => {
  const flat = ([] as unknown[]).concat(style as unknown[]).filter(Boolean) as Array<Record<string, unknown>>;
  return flat.some(s => s.flexGrow === 1 || s.flex === 1);
};

test.each([
  ['expanded', new Set<string>()],
  ['all collapsed', new Set(['idle', 'working'])],
])('the list fills the screen when %s, so a pull anywhere reaches it', (_name, collapsed) => {
  const list = render(collapsed).root.findByType(RNSectionList);
  expect(grows(list.props.style)).toBe(true);
  expect(grows(list.props.contentContainerStyle)).toBe(true);
});

// A list that just stops leaves the reader guessing whether that was everything or
// whether more is still coming — the radar ends in dark space, and it was read as the
// latter (2026-09-05). The close must therefore be honest about folded sections too.
describe('listEndLabel', () => {
  test('claims no total when it showed everything it has', () => {
    // The list's total is not the fleet's — the supervisor sits on the floating disc —
    // so a footer counting "16" under a header counting "17" would send the reader
    // looking for the missing one.
    expect(listEndLabel(16, 16, 'en')).toBe('end of list');
    expect(listEndLabel(16, 16, 'zh')).toBe('到底了');
  });

  test('does not claim you saw everything when a section is folded', () => {
    expect(listEndLabel(16, 5, 'en')).toBe('end of list · 5 of 16 shown');
    expect(listEndLabel(16, 0, 'zh')).toBe('到底了 · 显示 0 / 16');
  });
});
