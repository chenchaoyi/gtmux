import React from 'react';
import {StyleSheet, Switch} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {launchImageLibrary} from 'react-native-image-picker';
import {paletteFor} from './theme';
import {SettingsRow} from './SettingsRow';
import {AgentRow} from './AgentRow';
import type {Agent} from '../api/types';
import {Composer} from './Composer';
import {AttachSheet} from './AttachSheet';
import {ImageMarkup} from './ImageMarkup';

jest.mock('./ImageMarkup', () => ({ImageMarkup: () => null}));
const rgb = (value: string): number[] => value.startsWith('#')
  ? [1, 3, 5].map(i => parseInt(value.slice(i, i + 2), 16)).concat(1)
  : value.match(/[\d.]+/g)!.map(Number);
const over = (foreground: string, background: number[]): number[] => {
  const [r, g, b, a = 1] = rgb(foreground);
  return [r, g, b].map((c, i) => c * a + background[i] * (1 - a));
};
const luminance = (color: number[]) => color.slice(0, 3).map(v => v / 255)
  .map(v => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
  .reduce((out, v, i) => out + v * [0.2126, 0.7152, 0.0722][i], 0);
const contrast = (a: number[], b: number[]) => {
  const [lo, hi] = [luminance(a), luminance(b)].sort((x, y) => x - y);
  return (hi + 0.05) / (lo + 0.05);
};
const trees: renderer.ReactTestRenderer[] = [];
function mount(element: React.ReactElement) {
  let tree!: renderer.ReactTestRenderer;
  act(() => {tree = renderer.create(element);}); trees.push(tree); return tree;
}
afterEach(() => {act(() => trees.splice(0).forEach(tree => tree.unmount())); jest.restoreAllMocks();});

for (const theme of ['light', 'dark']) test(`${theme}: secondary reading text meets contrast on normal, raised, waiting and selected surfaces`, () => {
  const pal = paletteFor(theme);
  for (const base of [pal.bg, pal.surface, pal.raised]) {
    const backgrounds = [rgb(base), over(pal.waitingTint, rgb(base)), over(pal.rowSelected, rgb(base))];
    for (const background of backgrounds) for (const fg of [pal.fg2, pal.fg3]) {
      expect(contrast(over(fg, background), background)).toBeGreaterThanOrEqual(4.5);
    }
  }
  expect(pal.fgDisabled).not.toBe(pal.fg3);
});

test('a settings switch has its own identity, current state and disabled recovery context', () => {
  const change = jest.fn();
  const tree = mount(<SettingsRow label="Needs you" sub="Notify when a reply is needed" toggle={false}
    toggleDisabled disabledReason="Enable push notifications first" onToggle={change} pal={paletteFor('light')} />);
  const toggle = tree.root.findByType(Switch);
  expect(toggle.props.accessibilityLabel).toBe('Needs you');
  expect(tree.root.findAll(n => n.props.accessibilityElementsHidden === true && n.props.importantForAccessibility === 'no-hide-descendants').length).toBeGreaterThan(0);
  expect(toggle.props.accessibilityState).toEqual({checked: false, disabled: true});
  expect(toggle.props.accessibilityHint).toContain('Enable push notifications first');
  act(() => tree.update(<SettingsRow label="Needs you" toggle onToggle={change} pal={paletteFor('dark')} />));
  expect(toggle.props.accessibilityState).toEqual({checked: true, disabled: false});
  act(() => toggle.props.onValueChange(false)); expect(change).toHaveBeenCalledWith(false);
});

test('same-named sessions expose distinct location, state, time and destination without merging controls', () => {
  jest.spyOn(Date, 'now').mockReturnValue(2_000_000_000_000);
  const agent = {pane_id: '%10', agent: 'Codex', task: 'Review', status: 'working', source: 'tmux', session: 'gtmux dev', loc: 'dev:1.0', since: 1_999_999_880} as Agent;
  const open = jest.fn();
  const tree = mount(<AgentRow agent={agent} lang="en" pal={paletteFor('light')} onPress={open} selected />);
  const row = tree.root.findAll(n => n.props.accessibilityRole === 'button' && typeof n.props.onPress === 'function')[0];
  for (const text of ['Review', 'Codex', 'working', 'gtmux dev', 'dev:1.0', '%10', '2 minutes']) expect(row.props.accessibilityLabel).toContain(text);
  expect(row.props.accessibilityHint).toBe('Open session details');
  expect(row.props.accessibilityState.selected).toBe(true);
  act(() => row.props.onPress()); expect(open).toHaveBeenCalledTimes(1);
  const prior = row.props.accessibilityLabel;
  act(() => tree.update(<AgentRow agent={{...agent, pane_id: '%11', loc: 'dev:2.0', status: 'idle'}} lang="en" pal={paletteFor('dark')} onPress={open} />));
  expect(row.props.accessibilityLabel).not.toBe(prior); expect(row.props.accessibilityLabel).toContain('dev:2.0');
});

test('composer actions and independent thumbnail remove targets are at least44 points', async () => {
  const tree = mount(<Composer lang="en" pal={paletteFor('light')} onSend={jest.fn()} onUpload={async () => ({path: '/saved/photo.jpg'})} />);
  const control = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0];
  act(() => control('composer-kbd').props.onPress());
  for (const id of ['composer-attach', 'composer-expand', 'composer-send']) {
    const style = StyleSheet.flatten(control(id).props.style);
    expect(style.width).toBeGreaterThanOrEqual(44); expect(style.height).toBeGreaterThanOrEqual(44);
  }
  (launchImageLibrary as jest.Mock).mockResolvedValue({assets: [
    {uri: 'file:///one.jpg', fileName: 'one.jpg', type: 'image/jpeg'},
    {uri: 'file:///two.jpg', fileName: 'two.jpg', type: 'image/jpeg'},
  ]});
  act(() => control('composer-attach').props.onPress()); act(() => control('attach-0').props.onPress());
  const modal = tree.root.findByType(AttachSheet).findAll(n => n.props.onDismiss && n.props.animationType != null)[0];
  await act(async () => {await modal.props.onDismiss();});
  const image = () => [...new Map(tree.root.findAll(n => String(n.props.testID ?? '').startsWith('attach-annotate-') && typeof n.props.onPress === 'function').map(n => [n.props.testID, n])).values()];
  expect(image()).toHaveLength(2); expect(tree.root.findByType(ImageMarkup).props.visible).toBe(false);
  const remove = tree.root.findAll(n => String(n.props.testID ?? '').startsWith('attach-remove-') && typeof n.props.onPress === 'function')[0];
  const removeStyle = StyleSheet.flatten(remove.props.style);
  expect(removeStyle.width).toBeGreaterThanOrEqual(44); expect(removeStyle.height).toBeGreaterThanOrEqual(44);
  const frame = remove.parent!; const frameStyle = StyleSheet.flatten(frame.props.style);
  const preview = frame.children.find(n => typeof n !== 'string' && StyleSheet.flatten(n.props.style)?.width === 72) as renderer.ReactTestInstance;
  const previewWidth = StyleSheet.flatten(preview.props.style).width;
  expect(frameStyle.width - 2 * (frameStyle.borderWidth ?? 0) - removeStyle.right - removeStyle.width).toBeGreaterThanOrEqual(previewWidth);
  act(() => remove.props.onPress()); expect(image()).toHaveLength(1); expect(tree.root.findByType(ImageMarkup).props.visible).toBe(false);
  act(() => image()[0].props.onPress()); expect(tree.root.findByType(ImageMarkup).props.visible).toBe(true);
});
