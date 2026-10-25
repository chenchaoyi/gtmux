import React from 'react';
import {StyleSheet, Text, TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {Composer} from './Composer';
import {TerminalKeyPanel} from './TerminalKeyPanel';
import type {InputCapabilities, KeyResult} from './terminalKeys';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';
import {Haptics} from '../native/haptics';

// Native pickers aren't loadable under jest — the key row doesn't touch them.
jest.mock('react-native-image-picker', () => ({launchCamera: jest.fn(), launchImageLibrary: jest.fn()}));
jest.mock('@react-native-documents/picker', () => ({pick: jest.fn()}));
jest.mock('@react-native-clipboard/clipboard', () => ({hasImage: jest.fn(), getImagePNG: jest.fn(), getString: jest.fn()}));

const pal = paletteFor('dark');
const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => {
  for (const tree of mounted.splice(0)) act(() => tree.unmount());
});

async function render(onSend: (p: unknown) => void) {
  let tree: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<Composer pal={pal} lang="en" demo onSend={onSend} />);
  });
  mounted.push(tree!);
  return tree!;
}

// Pins the 2026-08-08 key-row rework (user calls): the ␣ Space pill is GONE (it
// did nothing useful; a literal space is typable in the field), ⏎ moved to the
// RIGHT of ↑/↓ (navigate → commit → erase → interrupt reading order), and a new
// ⌫ pill sends tmux `BSpace` (allowlisted server-side since v0.10.0).
test('resting key row: no Space, order Tab ↑ ↓ ⏎ ⌫ Ctrl-C Esc', async () => {
  const tree = await render(() => {});
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
test('⌫ onPress sends {key: BSpace}; ⏎ sends {key: Enter}', async () => {
  const sent: unknown[] = [];
  const tree = await render(p => sent.push(p));
  const press = (id: string) => {
    const el = tree.root.find(n => n.props.testID === id && typeof n.props.onPress === 'function');
    act(() => el.props.onPress());
  };
  press('composer-key-BSpace');
  press('composer-key-Enter');
  expect(sent).toEqual([{key: 'BSpace'}, {key: 'Enter'}]);
});

test('control key confirms the local tap briefly without shifting the row', async () => {
  jest.useFakeTimers();
  const hit = jest.spyOn(Haptics, 'select').mockImplementation(() => {});
  try {
    const sent: unknown[] = [];
    const tree = await render(p => sent.push(p));
    const press = (key: string) => {
      const el = tree.root.find(n => n.props.testID === `composer-key-${key}` && typeof n.props.onPress === 'function');
      act(() => el.props.onPress());
    };
    const ack = (key: string) => tree.root.findAll(n => n.type === Text && n.props.testID === `composer-key-${key}-ack`);
    press('Up');
    expect(sent).toEqual([{key: 'Up'}]);
    expect(hit).toHaveBeenCalledTimes(1);
    expect(ack('Up')).toHaveLength(1);
    act(() => jest.advanceTimersByTime(500));
    press('Down');
    expect(ack('Up')).toHaveLength(0);
    expect(ack('Down')).toHaveLength(1);
    act(() => jest.advanceTimersByTime(750));
    expect(ack('Down')).toHaveLength(0);
  } finally {
    hit.mockRestore();
    jest.useRealTimers();
  }
});

test('disabled input does not acknowledge or send a control key', async () => {
  const sent: unknown[] = [];
  let tree: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<Composer pal={pal} lang="en" demo enabled={false} onSend={p => sent.push(p)} />);
  });
  mounted.push(tree!);
  const key = tree!.root.find(n => n.props.testID === 'composer-key-Tab' && typeof n.props.onPress === 'function');
  expect(key.props.disabled).toBe(true);
  act(() => key.props.onPress());
  expect(sent).toHaveLength(0);
  expect(tree!.root.findAll(n => n.type === Text && n.props.testID === 'composer-key-Tab-ack')).toHaveLength(0);
});

// The visible symbol and the tappable target are separate usability requirements.
test('symbol keys have readable glyphs, 44pt targets and spoken labels', async () => {
  const tree = await render(() => {});
  for (const [key, spoken] of [['Up', 'Up arrow'], ['Down', 'Down arrow'], ['Enter', 'Return'], ['BSpace', 'Backspace']]) {
    const button = tree.root.find(n => n.props.testID === `composer-key-${key}` && !!n.props.style);
    const box = StyleSheet.flatten(button.props.style);
    expect(box.height).toBeGreaterThanOrEqual(44);
    expect(box.minWidth).toBeGreaterThanOrEqual(44);
    expect(button.props.accessibilityLabel).toBe(spoken);
    const label = button.findAllByType(Text).find(n => !n.props.testID);
    expect(StyleSheet.flatten(label!.props.style).fontSize).toBeGreaterThanOrEqual(24);
    expect(label!.props.allowFontScaling).toBe(false); // printed key symbols behave like the adjacent vector icons
  }
});


test('a terminal key blocks even a text-send callback captured before the next render', async () => {
 const onSend = jest.fn();
 let finish!: (r: KeyResult) => void;
 const sendKey = () => new Promise<KeyResult>(resolve => {finish = resolve;});
 const load = async (): Promise<InputCapabilities> => ({version:1, can_input:true, context:{boot:'b',pane_binding:'p',proof:'s',expires_at:Math.floor(Date.now()/1000)+120},retry_seconds:120,keys:[],actions:[]});
 let tree!: renderer.ReactTestRenderer;
 await act(async () => {tree = renderer.create(<Composer pal={pal} lang="en" demo onSend={onSend} keyTarget="fixture|%1" onLoadKeys={load} onSendKey={sendKey} />);});
 mounted.push(tree);
 act(() => tree.root.findAllByProps({testID:'composer-kbd'})[0].props.onPress());
 act(() => tree.root.findAllByType(TextInput)[0].props.onChangeText('Keep this draft'));
 const submit = tree.root.find(n => n.props.testID === 'composer-send' && typeof n.props.onPress === 'function').props.onPress;
 const control = tree.root.findByType(TerminalKeyPanel).props.control;
 act(() => {control.fire({key:'F2',modifiers:[]});submit();});
 expect(onSend).not.toHaveBeenCalled();
 expect(tree.root.findAllByType(TextInput)[0].props.value).toBe('Keep this draft');
 await act(async () => finish({ok:true}));
});


test('draft send and expand controls expose actionable native semantics', async () => {
  const tree = await render(jest.fn());
  await act(async () => tree.root.findAllByProps({testID: TestIds.composer.keyboard})[0].props.onPress());
  const control = () => tree.root.findByProps({testID: TestIds.composer.send});
  expect(control().props.accessibilityRole).toBe('button');
  expect(control().props.accessibilityState).toEqual({disabled: true});
  expect(tree.root.findByProps({testID: TestIds.composer.expand}).props.accessibilityRole).toBe('button');
  await act(async () => tree.root.findByProps({testID: TestIds.composer.input}).props.onChangeText('A synthetic draft'));
  expect(control().props.accessibilityState).toEqual({disabled: false});
});
