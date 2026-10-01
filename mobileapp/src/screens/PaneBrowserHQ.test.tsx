import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TextInput} from 'react-native';
import {PaneBrowserView} from './PaneBrowserScreen';
import {useApp} from '../state/AppContext';
import {useAgents, useAgentsOptional} from '../state/AgentsContext';
import {useWorkspace} from '../state/WorkspaceContext';
import {paletteFor} from '../ui/theme';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {TestIds} from '../constants/testIds';
import {PaneRow} from '../api/types';
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: jest.fn()}));
let tree: renderer.ReactTestRenderer;
const select = jest.fn();
const rows: PaneRow[] = [
 {pane_id: '%1', session: 'hq', loc: 'hq:0.0', window: '0', pane: '0', command: 'codex', tier: 'agent', role: 'supervisor'},
 {pane_id: '%2', session: 'hq', loc: 'hq:0.1', window: '0', pane: '1', command: 'bash', tier: 'plain'},
 {pane_id: '%3', session: 'HQ', loc: 'HQ:0.0', window: '0', pane: '0', command: 'bash', tier: 'plain'},
];
function text() { return tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' '); }
beforeEach(() => {
 (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
 const context = {isGuest: false, agents: [], client: {panes: jest.fn().mockResolvedValue(rows)}};
 (useAgents as jest.Mock).mockReturnValue(context);
 (useAgentsOptional as jest.Mock).mockReturnValue(context);
 (useApp as jest.Mock).mockReturnValue({pal: paletteFor('dark'), lang: 'en', mac: {name: 'Mac'}});
 (useWorkspace as jest.Mock).mockReturnValue({select});
});
afterEach(() => {act(() => tree?.unmount()); jest.clearAllMocks();});
test.each(['compact', 'regular'] as const)('HQ header keeps collapse keys and focus in %s browser', async layout => {
 await act(async () => {tree = renderer.create(<PaneBrowserView layout={layout} />);});
 expect(text()).toContain('Gtmux HQ');
 // Header badge + pane badge + the ordinary session's raw HQ name.
 expect(tree.root.findAllByType(Text).filter(n => n.props.children === 'HQ')).toHaveLength(3);
 const header = tree.root.findByProps({testID: `${TestIds.panes.section}-hq`});
 expect(header.props.accessibilityLabel).toBe('Gtmux HQ, HQ');
 expect(tree.root.findByProps({testID: `${TestIds.panes.section}-HQ`}).props.accessibilityLabel).toBe('HQ');
 const pane = tree.root.findAll(n => n.props.testID === `${TestIds.panes.row}-%1` && typeof n.props.onPress === 'function')[0];
 expect(pane).toBeDefined();
 act(() => pane!.props.onPress());
 expect(select).toHaveBeenCalledWith(expect.objectContaining({kind: 'pane', agent: expect.objectContaining({pane_id: '%1', role: 'supervisor', session: 'hq', loc: 'hq:0.0'})}));
 act(() => tree.root.findByProps({testID: `${TestIds.panes.section}-hq`}).props.onPress());
 expect(tree.root.findAllByProps({testID: `${TestIds.panes.row}-%1`})).toHaveLength(0);
 act(() => tree.root.findByProps({testID: `${TestIds.panes.section}-hq`}).props.onPress());
 expect(tree.root.findAllByProps({testID: `${TestIds.panes.row}-%1`}).length).toBeGreaterThan(0);
 act(() => tree.root.findByType(TextInput).props.onChangeText('Gtmux HQ'));
 expect(tree.root.findAllByProps({testID: `${TestIds.panes.section}-HQ`})).toHaveLength(0);
 expect(text()).toContain('Gtmux HQ');
 act(() => tree.root.findByType(TextInput).props.onChangeText('bash'));
 expect(tree.root.findByProps({testID: `${TestIds.panes.section}-hq`}).props.accessibilityLabel).toBe('Gtmux HQ, HQ');
});
