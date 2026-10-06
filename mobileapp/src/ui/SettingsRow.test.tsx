import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {PickerSheet, SettingsRow, SheetShell} from './SettingsRow';
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
  // Read from jest's cwd (mobileapp/), as other source-reading tests do: the app's
  // tsconfig has no Node types, so `__dirname` failed CI's typecheck.
  const fs = require('fs') as {readFileSync: (p: string, e: string) => string};
  const src = fs.readFileSync('src/screens/SettingsScreen.tsx', 'utf8');
  for (const zh of ['等你回应', '已完成']) {
    expect(src).toMatch(new RegExp(`<SettingsRow\\s+inset\\s+label=\\{lang === 'zh' \\? '${zh}'`));
  }
});


// A sheet is never taller than the window less the top safe area and a gap, and follows
// the window when it changes (rotation). The server details sheet rose past the top of a
// landscape phone, header and first group out of reach (%6, F14, 2026-10-06).
describe('SheetShell height', () => {
  const RN = require('react-native');
  const insets = require('react-native-safe-area-context');
  afterEach(() => jest.restoreAllMocks());
  test('bounded by the window, and updated when it changes', () => {
    const dims = jest.spyOn(RN, 'useWindowDimensions').mockReturnValue({width: 874, height: 402, scale: 3, fontScale: 1});
    jest.spyOn(insets, 'useSafeAreaInsets').mockReturnValue({top: 20, bottom: 21, left: 59, right: 59});
    let tree!: renderer.ReactTestRenderer;
    const sheet = () => <SheetShell visible pal={pal} onClose={() => {}}><RN.Text>body</RN.Text></SheetShell>;
    renderer.act(() => { tree = renderer.create(sheet()); });
    const maxH = () => RN.StyleSheet.flatten(tree.root.findByProps({testID: 'sheet-shell'}).props.style).maxHeight;
    expect(maxH()).toBe(402 - 20 - 24); // landscape phone
    dims.mockReturnValue({width: 402, height: 874, scale: 3, fontScale: 1});
    renderer.act(() => tree.update(sheet()));
    expect(maxH()).toBe(874 - 20 - 24); // rotated back to portrait
    renderer.act(() => tree.unmount());
  });
});
