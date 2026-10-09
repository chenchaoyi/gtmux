import React from 'react';
import {Modal, StyleSheet, Switch, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {SessionFollowSheet} from './SessionFollowSheet';
import {SettingsRow} from './SettingsRow';
import {ApiError} from '../api/client';
import {Agent, SessionFollowSettings} from '../api/types';
import {paletteFor, counts, sections} from './theme';
import {buildRowSheet} from './rowSheetModel';

const initial: SessionFollowSettings = {hq: false, notify: false, knowledge: false, revision: 0};
const agent: Agent = {pane_id: '', session: '', window: '', pane: '', loc: '', agent: 'Codex', status: 'working', task: 'Investigate crash', latest: false, activity: false, source: 'native', client: 'chatgpt_desktop', session_id: 'desktop-a', follow: initial};
const mounted: renderer.ReactTestRenderer[] = [];
async function mount(value = initial, lang: 'en' | 'zh' = 'en', read = async () => value, theme = 'dark') {
  const client = {sessionFollow: jest.fn().mockImplementation(read), saveSessionFollow: jest.fn().mockImplementation(async (_id, next) => ({...next, revision: next.revision + 1}))};
  const onSaved = jest.fn(); const onClose = jest.fn(); let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<SessionFollowSheet agent={agent} client={client} lang={lang} pal={paletteFor(theme)} onClose={onClose} onSaved={onSaved} />); });
  mounted.push(tree);
  const node = (id: string) => tree.root.findByProps({testID: id});
  const text = () => tree.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');
  const toggle = async (key: string, enabled: boolean) => { await act(async () => { await node(`follow-${key}`).props.onValueChange(enabled); }); };
  return {tree, client, onSaved, onClose, node, text, toggle};
}
afterEach(() => { for (const tree of mounted.splice(0)) act(() => tree.unmount()); });

test('default status-only is separate from totals and offers settings, not input/adopt', () => {
  expect(counts([agent])).toMatchObject({total: 0, working: 0});
  expect(sections([agent]).map(s => s.status)).toContain('desktop');
  const actions = buildRowSheet(agent, 'en', 0).actions.map(a => a.key);
  expect(actions).toContain('follow'); expect(actions).not.toContain('adopt');
  expect(counts([{...agent, follow: {...initial, hq: true}}])).toMatchObject({total: 1, working: 1});
});

test.each(['dark', 'light'])('uses the same native switch appearance as App settings in %s theme', async theme => {
  const m = await mount({...initial, hq: true}, 'en', undefined, theme);
  let row!: renderer.ReactTestRenderer;
  act(() => { row = renderer.create(<SettingsRow label="Notifications" toggle pal={paletteFor(theme)} />); });
  mounted.push(row);
  const reference = row.root.findByType(Switch).props;
  for (const sw of m.tree.root.findAllByType(Switch)) {
    expect(sw.props.trackColor).toBe(reference.trackColor);
    expect(sw.props.trackColor).toBeUndefined(); // UIKit's enabled colour, never overridden with grey
    expect(sw.props.thumbColor).toBeUndefined();
  }
});

test('toggles save immediately, HQ grants no child capability and later writes use acknowledged revisions', async () => {
  const m = await mount(initial, 'zh');
  expect(m.onSaved).not.toHaveBeenCalled();
  expect(m.tree.root.findAllByProps({testID: 'follow-save'})).toHaveLength(0);
  expect(m.tree.root.findAllByProps({testID: 'follow-cancel'})).toHaveLength(0);
  expect(m.node('follow-notify').props.disabled).toBe(true);
  await m.toggle('hq', true);
  expect(m.client.saveSessionFollow).toHaveBeenNthCalledWith(1, 'desktop-a', {...initial, hq: true});
  expect(m.node('follow-notify').props.value).toBe(false);
  expect(m.node('follow-knowledge').props.value).toBe(false);
  expect(m.onSaved).toHaveBeenCalledTimes(1);
  expect(m.text()).toContain('已更新');
  await m.toggle('notify', true);
  expect(m.client.saveSessionFollow).toHaveBeenNthCalledWith(2, 'desktop-a', {...initial, hq: true, notify: true, revision: 1});
  expect(m.node('follow-knowledge').props.value).toBe(false);
});

test('all rows keep their places and status height through off, on, saving, error and stopping', async () => {
  const m = await mount();
  const rows = ['hq', 'notify', 'knowledge'].map(k => m.node(`follow-row-${k}`));
  const statusStyle = StyleSheet.flatten(m.node('follow-status').props.style);
  expect(m.tree.root.findAllByType(Switch)).toHaveLength(3);
  await m.toggle('hq', true);
  await m.toggle('notify', true);
  await m.toggle('knowledge', true);
  await m.toggle('hq', false);
  expect(m.client.saveSessionFollow).toHaveBeenLastCalledWith('desktop-a', {...initial, revision: 3});
  expect(m.tree.root.findAllByType(Switch)).toHaveLength(3);
  expect(m.node('follow-notify').props.disabled).toBe(true);
  expect(m.node('follow-notify').props.value).toBe(false);
  expect(m.node('follow-knowledge').props.value).toBe(false);
  await m.toggle('hq', true);
  expect(m.node('follow-notify').props.value).toBe(false); // previous grants never restored
  expect(m.node('follow-knowledge').props.value).toBe(false);
  m.client.saveSessionFollow.mockRejectedValueOnce(new ApiError(409, 'conflict'));
  await m.toggle('notify', true);
  for (let i = 0; i < rows.length; i++) expect(m.node(`follow-row-${['hq','notify','knowledge'][i]}`)).toBe(rows[i]);
  expect(StyleSheet.flatten(m.node('follow-status').props.style)).toEqual(statusStyle);
  expect(statusStyle.height).toBe(56);
});

test('disabled children, unchanged values and rapid duplicate inputs cannot grant extra permission', async () => {
  const m = await mount();
  await m.toggle('notify', true); await m.toggle('hq', false);
  expect(m.client.saveSessionFollow).not.toHaveBeenCalled();
  let resolve!: (value: SessionFollowSettings) => void;
  m.client.saveSessionFollow.mockReturnValueOnce(new Promise(r => {resolve = r;}));
  let pending!: Promise<void>;
  act(() => { pending = m.node('follow-hq').props.onValueChange(true); m.node('follow-hq').props.onValueChange(true); m.node('follow-notify').props.onValueChange(true); });
  expect(m.client.saveSessionFollow).toHaveBeenCalledTimes(1);
  expect(m.onSaved).not.toHaveBeenCalled();
  expect(m.node('follow-done').props.disabled).toBe(true);
  act(() => m.tree.root.findByType(Modal).props.onRequestClose());
  expect(m.onClose).not.toHaveBeenCalled();
  await act(async () => {resolve({...initial, hq: true, revision: 1}); await pending;});
  expect(m.onSaved).toHaveBeenCalledTimes(1);
  act(() => m.node('follow-done').props.onPress());
  expect(m.onClose).toHaveBeenCalledTimes(1);
});

test('conflicts read current settings and never overwrite another device using the stale revision', async () => {
  const m = await mount({...initial, hq: true, revision: 4});
  m.client.saveSessionFollow.mockRejectedValueOnce(new ApiError(409, 'conflict'));
  m.client.sessionFollow.mockResolvedValueOnce({...initial, hq: true, knowledge: true, revision: 9});
  await m.toggle('notify', true);
  expect(m.text()).toContain('Changed on another device');
  expect(m.text()).not.toContain('Updated');
  expect(m.node('follow-notify').props.value).toBe(false);
  expect(m.node('follow-knowledge').props.value).toBe(true);
  await m.toggle('notify', true);
  expect(m.client.saveSessionFollow).toHaveBeenLastCalledWith('desktop-a', {...initial, hq: true, notify: true, knowledge: true, revision: 9});
});

test('a lost write receipt may have committed: show read-back state rather than falsely rolling it back', async () => {
  const m = await mount();
  m.client.saveSessionFollow.mockRejectedValueOnce(new Error('connection lost'));
  m.client.sessionFollow.mockResolvedValueOnce({...initial, hq: true, revision: 1});
  await m.toggle('hq', true);
  expect(m.node('follow-hq').props.value).toBe(true);
  expect(m.text()).toContain('Update not confirmed');
  expect(m.client.saveSessionFollow).toHaveBeenCalledTimes(1); // never auto-retry a grant
  expect(m.onSaved).toHaveBeenCalledTimes(1); // refresh radar from observed canonical state
});

test('if write and read-back fail, disable edits until explicit reload recovers canonical state', async () => {
  const m = await mount();
  m.client.saveSessionFollow.mockRejectedValueOnce(new Error('offline'));
  m.client.sessionFollow.mockRejectedValueOnce(new Error('offline'));
  await m.toggle('hq', true);
  expect(m.text()).toContain('Could not confirm settings');
  expect(m.tree.root.findAllByType(Switch).every(sw => sw.props.disabled)).toBe(true);
  await m.toggle('hq', true);
  expect(m.client.saveSessionFollow).toHaveBeenCalledTimes(1);
  m.client.sessionFollow.mockResolvedValueOnce({...initial, hq: true, revision: 7});
  await act(async () => { await m.node('follow-reload').props.onPress(); });
  expect(m.node('follow-notify').props.disabled).toBe(false);
  await m.toggle('notify', true);
  expect(m.client.saveSessionFollow).toHaveBeenLastCalledWith('desktop-a', {...initial, hq: true, notify: true, revision: 7});
});

test('loading and failed reads keep row placeholders, never show editable defaults, and allow Done', async () => {
  let reject!: (e: Error) => void;
  const m = await mount(initial, 'en', () => new Promise((_resolve, r) => {reject = r;}));
  expect(m.text()).toContain('Loading settings');
  expect(m.tree.root.findAllByType(Switch)).toHaveLength(0);
  expect(['hq','notify','knowledge'].map(k => m.node(`follow-row-${k}`))).toHaveLength(3);
  expect(m.node('follow-done').props.disabled).toBe(false);
  await act(async () => {reject(new ApiError(503, 'unsupported'));});
  expect(m.text()).toContain('Update gtmux');
  expect(m.tree.root.findAllByType(Switch)).toHaveLength(0);
  act(() => m.node('follow-done').props.onPress());
  expect(m.onClose).toHaveBeenCalledTimes(1);
  expect(m.client.saveSessionFollow).not.toHaveBeenCalled();
});

test('late writes after switching conversation cannot refresh or overwrite another session', async () => {
  const m = await mount();
  let resolve!: (value: SessionFollowSettings) => void;
  m.client.saveSessionFollow.mockReturnValueOnce(new Promise(r => {resolve = r;}));
  let pending!: Promise<void>;
  act(() => { pending = m.node('follow-hq').props.onValueChange(true); });
  act(() => m.tree.unmount());
  await act(async () => {resolve({...initial, hq: true, revision: 1}); await pending;});
  expect(m.onSaved).not.toHaveBeenCalled();
});
