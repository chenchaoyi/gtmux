import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ServersScreen} from './ServersScreen';
import {useApp} from '../state/AppContext';
import {useAgentsOptional} from '../state/AgentsContext';
import {paletteFor} from '../ui/theme';
import {makeT} from '../i18n';
import {AnchoredMenu} from '../ui/AnchoredMenu';
import {RestoreSessionsSheet} from './RestoreSessionsSheet';
import {Modal, TouchableOpacity} from 'react-native';
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgentsOptional: jest.fn()}));
jest.mock('./PairingScreen', () => ({PairingScreen: () => null}));
jest.mock('./DemoScreen', () => ({DemoScreen: () => null}));
jest.mock('./RestoreSessionsSheet', () => ({RestoreSessionsSheet: () => null}));
jest.setTimeout(20000);
test('owner server More offers restore and targets that server; guest does not', async () => {
 const servers = [{name: 'Owner', url: 'https://owner.example', token: 'a'}, {name: 'Guest', url: 'https://guest.example', token: 'b', scope: 'guest'}];
 (useApp as jest.Mock).mockReturnValue({t: makeT('en'), lang: 'en', pal: paletteFor('dark'), servers, activeUrl: servers[0].url, pushEnabled: true, pushKinds: {waiting: true, done: true}, pushSync: {}, setServerPushEnabled: jest.fn(), retryPushSync: jest.fn()});
 (useAgentsOptional as jest.Mock).mockReturnValue(null);
 const old = globalThis.fetch; globalThis.fetch = jest.fn().mockResolvedValue({ok: false}); let tree!: renderer.ReactTestRenderer;
 try {await act(async () => {tree = renderer.create(<ServersScreen />);});
 const more = (name: string) => tree.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel === `${name} · More options`)!;
 act(() => more('Owner').props.onPress()); const action = tree.root.findByProps({testID: 'server-menu-restore'}); act(() => action.props.onPress()); act(() => tree.root.findByType(AnchoredMenu).findByType(Modal).props.onDismiss());
 expect(tree.root.findByType(RestoreSessionsSheet).props.mac).toEqual(servers[0]);
 act(() => tree.root.findByType(RestoreSessionsSheet).props.onClose()); act(() => more('Guest').props.onPress()); expect(tree.root.findAllByProps({testID: 'server-menu-restore'})).toHaveLength(0);
 } finally {act(() => tree?.unmount()); globalThis.fetch = old;}
});
