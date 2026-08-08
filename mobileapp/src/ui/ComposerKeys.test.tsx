import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Composer} from './Composer';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

// Native pickers aren't loadable under jest — the key row doesn't touch them.
jest.mock('react-native-image-picker', () => ({launchCamera: jest.fn(), launchImageLibrary: jest.fn()}));
jest.mock('@react-native-documents/picker', () => ({pick: jest.fn()}));
jest.mock('@react-native-clipboard/clipboard', () => ({hasImage: jest.fn(), getImagePNG: jest.fn(), getString: jest.fn()}));

const pal = paletteFor('dark');

function render(onSend: (p: unknown) => void) {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<Composer pal={pal} lang="en" demo onSend={onSend} />);
  });
  return tree!;
}

// Pins the 2026-08-08 key-row rework (user calls): the ␣ Space pill is GONE (it
// did nothing useful; a literal space is typable in the field), ⏎ moved to the
// RIGHT of ↑/↓ (navigate → commit → erase → interrupt reading order), and a new
// ⌫ pill sends tmux `BSpace` (allowlisted server-side since v0.10.0).
test('resting key row: no Space, order Tab ↑ ↓ ⏎ ⌫ Ctrl-C Esc', () => {
  const tree = render(() => {});
  const ids = tree.root
    .findAll(n => typeof n.props.testID === 'string' && n.props.testID.startsWith(`${TestIds.composer.controlKey}-`))
    .map(n => n.props.testID as string);
  // findAll returns render order = visual left→right order in the row.
  const uniq = [...new Set(ids)];
  expect(uniq).toEqual([
    'composer-key-Tab',
    'composer-key-Up',
    'composer-key-Down',
    'composer-key-Enter',
    'composer-key-BSpace',
    'composer-key-C-c',
    'composer-key-Escape',
  ]);
});

// The wiring, not just the label: tapping ⌫ must emit {key: 'BSpace'} — the tmux
// key name POST /api/send expects. (The e2e proves the same end-to-end against a
// live pane; this pins it deterministically against XCUITest tap flakiness.)
test('⌫ onPress sends {key: BSpace}; ⏎ sends {key: Enter}', () => {
  const sent: unknown[] = [];
  const tree = render(p => sent.push(p));
  const press = (id: string) => {
    const el = tree.root.find(n => n.props.testID === id && typeof n.props.onPress === 'function');
    act(() => el.props.onPress());
  };
  press('composer-key-BSpace');
  press('composer-key-Enter');
  expect(sent).toEqual([{key: 'BSpace'}, {key: 'Enter'}]);
});
