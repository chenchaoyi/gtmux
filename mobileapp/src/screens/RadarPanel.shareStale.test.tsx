import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {RadarPanel} from './RadarPanel';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';
import {makeT} from '../i18n';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/SectionList', () => ({SectionList: ({ListEmptyComponent}: any) => ListEmptyComponent}));
jest.mock('../ui/RowSheet', () => ({RowSheet: () => null}));
jest.mock('../ui/HQDisc', () => ({HQDisc: () => null, DISC_CLEARANCE: 0}));
jest.mock('../ui/NewSessionAction', () => ({NewSessionAction: () => null}));

let tree: renderer.ReactTestRenderer;
afterEach(() => { act(() => tree?.unmount()); });

test.each(['screen', 'sidebar'] as const)('%s shows localized expired access only to stale guests', async variant => {
  for (const lang of ['en', 'zh'] as const) {
    const state = {agents: [], conn: 'live', isGuest: true, shareStale: true, refresh: jest.fn(),
      client: {host: jest.fn().mockResolvedValue({ok: false}), usage: jest.fn().mockResolvedValue(null), serverMode: jest.fn().mockResolvedValue(null)}};
    (useAgents as jest.Mock).mockImplementation(() => state);
    (useApp as jest.Mock).mockReturnValue({lang, pal: paletteFor('dark'), t: makeT(lang)});
    await act(async () => { tree = renderer.create(<RadarPanel variant={variant} />); });
    const texts = () => tree.root.findAllByType(Text).map(n => n.props.children).filter(x => typeof x === 'string');
    const message = lang === 'zh' ? '分享授权已失效，请联系所有者重新授权。' : 'Share access has expired. Ask the owner to grant access again.';
    const banner = tree.root.findAllByProps({testID: 'radar-guest-banner'})[0];
    expect(banner.findAllByType(Text).map(n => n.props.children)).toContain(message);
    expect(texts().filter(x => x === message)).toHaveLength(2); // Banner and empty state.
    expect(texts().join(' ')).not.toMatch(/start your agent|启动你的 agent|重新配对|pair again/);
    state.shareStale = false;
    await act(async () => { tree.update(<RadarPanel variant={variant} />); });
    expect(texts()).not.toContain(message);
    expect(texts()).toContain(lang === 'zh' ? '还没有可查看的共享 agent。' : 'No shared agents to view yet.');
    state.isGuest = false;
    state.shareStale = true;
    await act(async () => { tree.update(<RadarPanel variant={variant} />); });
    expect(texts()).not.toContain(message);
    expect(tree.root.findAllByProps({testID: 'radar-guest-banner'})).toHaveLength(0);
    act(() => tree.unmount());
  }
});
