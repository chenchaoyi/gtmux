import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import {PairingScreen} from './PairingScreen';
import {demoEntrySelector, demoEntrySuffix} from '../../e2e/setup/demoEntry';
import {paletteFor} from '../ui/theme';
import {makeT} from '../i18n';
import {useApp} from '../state/AppContext';

// The app tsconfig deliberately omits Node ambient types. Keep these test-only
// filesystem helpers explicit so clean CI and local dependency trees agree.
const {readFileSync} = require('fs') as {readFileSync: (path: string, encoding: string) => string};
const {dirname, resolve} = require('path') as {dirname: (path: string) => string; resolve: (...paths: string[]) => string};
const req = require as typeof require & {resolve: (path: string) => string};
const reviewDocs = resolve(dirname(req.resolve('./PairingScreen.tsx')), '../../../docs/appstore');

jest.mock('./ScanScreen', () => ({ScanScreen: () => null}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));

test.each(['en', 'zh'] as const)('capture selector matches the real %s demo button and visible action', async lang => {
  (useApp as jest.Mock).mockReturnValue({lang, t: makeT(lang), pal: paletteFor('dark'), pair: jest.fn()});
  const onDemo = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<PairingScreen onDemo={onDemo} />);});
  try {
    const button = tree.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel === (lang === 'zh' ? '查看演示' : 'View demo'))!;
    expect(button.props.accessibilityRole).toBe('button');
    expect(button.props.accessibilityLabel.endsWith(demoEntrySuffix(lang))).toBe(true);
    const visible = button.findAllByType(Text).find(n => typeof n.props.children === 'string' && n.props.children.endsWith(demoEntrySuffix(lang)))!.props.children;
    expect(typeof visible).toBe('string');
    for (const doc of ['submit.md', 'assets-ci.md', 'assets-ci.zh.md']) {
      expect(readFileSync(resolve(reviewDocs, doc), 'utf8')).toContain(visible);
    }
    expect(demoEntrySelector(lang)).toContain(`label ENDSWITH "${demoEntrySuffix(lang)}"`);
    act(() => button.props.onPress());
    expect(onDemo).toHaveBeenCalledTimes(1);
  } finally {act(() => tree.unmount());}
});

test('capture refuses an unsupported language instead of taking the wrong localized image', () => {
  expect(() => demoEntrySelector('fr')).toThrow('Unsupported demo language');
});
