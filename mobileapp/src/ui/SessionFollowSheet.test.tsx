import React from 'react';
import {Modal, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {SessionFollowSheet} from './SessionFollowSheet';
import {ApiError} from '../api/client';
import {Agent, SessionFollowSettings} from '../api/types';
import {paletteFor, counts, sections} from './theme';
import {buildRowSheet} from './rowSheetModel';

const initial: SessionFollowSettings = {hq: false, notify: false, knowledge: false, revision: 0};
const agent: Agent = {pane_id: '', session: '', window: '', pane: '', loc: '', agent: 'Codex', status: 'working', task: 'Investigate crash', latest: false, activity: false, source: 'native', client: 'chatgpt_desktop', session_id: 'desktop-a', follow: initial};
const mounted: renderer.ReactTestRenderer[] = [];
async function mount(value = initial, lang: 'en' | 'zh' = 'en') {
  const client = {sessionFollow: jest.fn().mockResolvedValue(value), saveSessionFollow: jest.fn().mockImplementation(async (_id, draft) => ({...draft, revision: draft.revision + 1}))};
  const onSaved = jest.fn(); const onClose = jest.fn(); let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<SessionFollowSheet agent={agent} client={client} lang={lang} pal={paletteFor('dark')} onClose={onClose} onSaved={onSaved} />); });
  mounted.push(tree);
  const node = (id: string) => tree.root.findByProps({testID: id});
  const text = () => tree.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');
  return {tree, client, onSaved, onClose, node, text};
}
afterEach(() => { for (const tree of mounted.splice(0)) act(() => tree.unmount()); });

test('default status-only is separate from totals and offers session settings, not desktop input/adopt', () => {
  expect(counts([agent])).toMatchObject({total: 0, working: 0});
  expect(sections([agent]).map(s => s.status)).toContain('desktop');
  const actions = buildRowSheet(agent, 'en', 0).actions.map(a => a.key);
  expect(actions).toContain('follow'); expect(actions).not.toContain('adopt');
  expect(counts([{...agent, follow: {...initial, hq: true}}])).toMatchObject({total: 1, working: 1});
  expect(counts([{...agent, client: 'terminal'}])).toMatchObject({total: 1, working: 1});
});

test('HQ does not grant other permissions and badges wait for a successful receipt', async () => {
  const m = await mount(initial, 'zh');
  expect(m.client.sessionFollow).toHaveBeenCalledWith('desktop-a');
  act(() => m.node('follow-hq').props.onPress());
  expect(m.node('follow-notify').props.value).toBe(false);
  expect(m.node('follow-knowledge').props.value).toBe(false);
  expect(m.text()).toContain('仅显示状态'); expect(m.onSaved).not.toHaveBeenCalled();
  await act(async () => { await m.node('follow-save').props.onPress(); });
  expect(m.client.saveSessionFollow).toHaveBeenCalledWith('desktop-a', {...initial, hq: true});
  expect(m.text()).toContain('HQ 跟进中'); expect(m.text()).toContain('设置已保存'); expect(m.onSaved).toHaveBeenCalledTimes(1);
});

test('stop clears child permissions; conflict retains the draft and offers reload', async () => {
  const current = {hq: true, notify: true, knowledge: true, revision: 4};
  const m = await mount(current);
  act(() => m.node('follow-status-only').props.onPress());
  expect(m.text()).toContain('Keeps existing records');
  m.client.saveSessionFollow.mockRejectedValueOnce(new ApiError(409, 'conflict'));
  await act(async () => { await m.node('follow-save').props.onPress(); });
  expect(m.client.saveSessionFollow).toHaveBeenCalledWith('desktop-a', {...initial, revision: 4});
  expect(m.onSaved).not.toHaveBeenCalled(); expect(m.text()).toContain('another device');
  expect(m.text()).not.toContain('Settings saved'); expect(m.text()).toContain('Reload settings');
});

test('repeated taps cannot duplicate writes, dismissal waits for the receipt, late responses cannot update another session', async () => {
  const m = await mount(); let resolve!: (value: SessionFollowSettings) => void;
  m.client.saveSessionFollow.mockReturnValue(new Promise(r => {resolve = r;}));
  act(() => m.node('follow-hq').props.onPress());
  let pending!: Promise<void>;
  act(() => { pending = m.node('follow-save').props.onPress(); m.node('follow-save').props.onPress(); });
  expect(m.client.saveSessionFollow).toHaveBeenCalledTimes(1);
  act(() => m.tree.root.findByType(Modal).props.onRequestClose()); expect(m.onClose).not.toHaveBeenCalled();
  act(() => m.tree.unmount());
  await act(async () => {resolve({...initial, hq: true, revision: 1}); await pending;});
  expect(m.onSaved).not.toHaveBeenCalled();
});
