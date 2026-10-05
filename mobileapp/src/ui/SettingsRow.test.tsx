import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {PickerSheet, SettingsRow} from './SettingsRow';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

const pal = paletteFor('dark');

const OPTIONS: {key: string; label: string; sub?: string}[] = [
  {key: 'system', label: 'System', sub: 'Follow the device'},
  {key: 'en', label: 'English'},
  {key: 'zh', label: '中文'},
];

function render() {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <PickerSheet
        visible
        title="Language"
        options={OPTIONS as any}
        selected="system"
        pal={pal}
        onSelect={() => {}}
        onClose={() => {}}
      />,
    );
  });
  return tree!;
}

// Pins the a11y unmerge (2026-08 exploration finding): the sheet's tap-catcher
// Pressable is accessible by DEFAULT and swallowed every child into ONE merged AX
// element ("Language, System, English, ✓, 中文") — unreadable to VoiceOver and
// untargetable by automation. Now the catcher opts out and every option is its
// own button element.
test('PickerSheet: each option is its OWN accessible button (not one merged element)', () => {
  const tree = render();
  for (const o of OPTIONS) {
    const el = tree.root.find(
      n => n.props.testID === `${TestIds.settings.pickerOption}-${o.key}` && typeof n.props.onPress === 'function',
    );
    expect(el.props.accessible).toBe(true);
    expect(el.props.accessibilityRole).toBe('button');
    expect(el.props.accessibilityLabel).toBe(o.sub ? `${o.label}, ${o.sub}` : o.label);
    expect(el.props.accessibilityState).toEqual({selected: o.key === 'system'});
  }
});

test('PickerSheet: the sheet tap-catcher opts OUT of accessibility (no child merge)', () => {
  const tree = render();
  // the catcher is the pressable that owns onLayout (sheet height measurement)
  const catcher = tree.root.find(n => typeof n.props.onLayout === 'function' && typeof n.props.onPress === 'function');
  expect(catcher.props.accessible).toBe(false);
});

// A child setting keeps the icon column empty, so its text starts where its parent's
// does; without it the text started at the card's edge, left of the parent (F10).
test('an inset row keeps the icon column; a plain icon-less row does not', () => {
  let inset!: renderer.ReactTestRenderer;
  let plain!: renderer.ReactTestRenderer;
  act(() => {
    inset = renderer.create(<SettingsRow inset label="Needs you" pal={pal} toggle onToggle={() => {}} />);
    plain = renderer.create(<SettingsRow label="Plain" pal={pal} />);
  });
  const column = inset.root.findAllByProps({testID: 'settings-row-inset'});
  expect(column.length).toBeGreaterThan(0);
  expect((column[0].props.style as {width: number}).width).toBe(30);
  expect(plain.root.findAllByProps({testID: 'settings-row-inset'})).toHaveLength(0);
});

// The two push kinds are the inset children of Push notifications (F10).
test('Needs you and Finished are inset under Push notifications', () => {
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const src: string = require('fs').readFileSync(require('path').join(__dirname, '../screens/SettingsScreen.tsx'), 'utf8');
  for (const zh of ['等你回应', '已完成']) {
    expect(src).toMatch(new RegExp(`<SettingsRow\\s+inset\\s+label=\\{lang === 'zh' \\? '${zh}'`));
  }
});

