import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {SettingsScreen} from './SettingsScreen';
import {PickerSheet, SettingsGroup, SettingsRow} from '../ui/SettingsRow';
import {paletteFor} from '../ui/theme';
import {TestIds} from '../constants/testIds';
import {TouchableOpacity} from 'react-native';

const mockTheme = jest.fn();
jest.mock('../state/AppContext', () => ({useApp: () => ({
  t: (key: string) => key, lang: 'en', pal: require('../ui/theme').paletteFor('light'), langPref: 'system', setLangPref: jest.fn(),
  mac: null, servers: [], pushEnabled: false, pushKinds: {}, themePref: 'system', setThemePref: mockTheme,
})}));
jest.mock('../state/AgentsContext', () => ({useAgents: () => ({isGuest: true, conn: 'offline', client: null})}));
jest.mock('./useRouteChoices', () => ({useRouteChoices: () => ({routes: [], loading: false, error: false, load: jest.fn()})}));
jest.mock('../state/hqMemory', () => ({readCopy: async () => null}));

test('global color scheme lives in Appearance and uses the existing shared preference', async () => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<SettingsScreen navigation={{addListener: () => () => {}, goBack: jest.fn()}} />);});
  try {
    const group = tree.root.findAllByType(SettingsGroup).find(n => n.props.title === 'Appearance')!;
    expect(group).toBeDefined();
    const row = group.findAllByType(SettingsRow).find(n => n.props.label === 'Color scheme')!;
    expect(row.props.testID).toBe(TestIds.settings.appearance);
    const control = row.findByType(TouchableOpacity);
    expect(control.props.testID).toBe(TestIds.settings.appearance);
    expect(control.props.accessibilityLabel).toBe('Color scheme, system');
    expect(row.props.value).toBe('system'); expect(row.props.sub).toContain('throughout the app');
    expect(group.props.pal).toEqual(paletteFor('light'));
    act(() => row.props.onPress());
    const picker = tree.root.findAllByType(PickerSheet).find(n => n.props.visible && n.props.title === 'Appearance')!;
    expect(picker.props.options.map((o: {key: string}) => o.key)).toEqual(['system', 'light', 'dark']);
    for (const key of ['light', 'dark', 'system']) act(() => picker.props.onSelect(key));
    expect(mockTheme.mock.calls.map(c => c[0])).toEqual(['light', 'dark', 'system']);
    for (const other of tree.root.findAllByType(SettingsGroup).filter(n => n !== group)) {
      expect(other.findAllByType(SettingsRow).some(n => n.props.label === 'Color scheme')).toBe(false);
    }
  } finally {act(() => tree.unmount());}
});
