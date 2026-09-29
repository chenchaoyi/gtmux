import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {HistoryModal} from './HistoryModal';
import {paletteFor} from './theme';

const pal = paletteFor('light');

function render(history: string[], theme = pal, onPick = (_text: string) => {}) {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <HistoryModal
        visible
        history={history}
        pal={theme}
        lang="en"
        onPick={onPick}
        onDelete={() => {}}
        onClear={() => {}}
        onClose={() => {}}
      />,
    );
  });
  return tree!;
}

const flatStyle = (node: {props: {[k: string]: unknown}}) =>
  Object.assign({}, ...[node.props.style].flat(Infinity).filter(Boolean));

// Pins the fix for "the history Clear button always looks disabled": it was styled
// pal.fg3 — the ~34% tertiary tone every DISABLED control uses — so a live, tappable
// destructive action read as permanently greyed out (user report, 2026-08-08). It must
// render in the danger red (same value as the row Delete action + SettingsRow danger),
// which unambiguously reads "enabled, destructive".
test('Clear renders in danger red, not the disabled-looking tertiary grey', () => {
  const tree = render(['first message', 'second message']);
  const clear = tree.root.findByProps({testID: 'history-clear'});
  const label = clear.findByType(require('react-native').Text);
  const style = flatStyle(label);
  expect(style.color).toBe('#EF4444');
  expect(style.color).not.toBe(pal.fg3);
});

// The button only exists when there is something to clear — empty history renders the
// empty-state line instead (no dead control).
test('Clear is absent when history is empty', () => {
  const tree = render([]);
  expect(tree.root.findAllByProps({testID: 'history-clear'})).toHaveLength(0);
});

test('history rows stay inset on the raised sheet surface and remain selectable', () => {
  const dark = paletteFor('dark');
  const picked: string[] = [];
  const tree = render(['a long command from this session', 'another command'], dark, text => picked.push(text));
  const list = tree.root.findByProps({testID: 'history-list'});
  const row = tree.root.findByProps({testID: 'history-row-0'});
  expect(flatStyle(list).backgroundColor).toBe(dark.raised);
  expect(flatStyle(list).backgroundColor).not.toBe(dark.bg);
  expect(flatStyle(list).borderRadius).toBeGreaterThan(0);
  expect(flatStyle(row).paddingHorizontal).toBeGreaterThanOrEqual(12);
  act(() => row.props.onPress());
  expect(picked).toEqual(['a long command from this session']);
});
