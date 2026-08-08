import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {PickerSheet} from './SettingsRow';
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
