import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {PairingScreen} from './PairingScreen';
import {TestIds} from '../constants/testIds';
import {checkServer} from '../pairing/deadline';

// F13 (%6, 2026-10-06): Connect with the address and token left empty said "Can't reach
// this Mac … if a VPN or proxy is on, turn it off", although nothing was tried. An empty
// field now names the field, and no connection is attempted.
jest.mock('../pairing/deadline', () => ({checkServer: jest.fn()}));
jest.mock('./ScanScreen', () => ({ScanScreen: () => null}));
const strings: Record<string, string> = {
  pairNeedAddress: "Enter the Mac's address.",
  pairNeedToken: 'Enter the token too (gtmux pair on the Mac shows it).',
  cantReach: "Can't reach this Mac.",
};
jest.mock('../state/AppContext', () => ({
  useApp: () => ({t: (k: string) => strings[k] ?? k, pal: require('../ui/theme').paletteFor('dark'), pair: jest.fn(), lang: 'en'}),
}));

async function render() {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<PairingScreen />);
  });
  return tree;
}
const field = (tree: renderer.ReactTestRenderer, id: string) => tree.root.findByProps({testID: id});
const error = (tree: renderer.ReactTestRenderer) =>
  tree.root.findAll(n => n.type === Text && n.props.testID === TestIds.pairing.error).map(n => String(n.props.children)).join('');

test.each([
  ['', '', "Enter the Mac's address."],
  ['', 'tok', "Enter the Mac's address."],
  ['studio.local', '', 'Enter the token too (gtmux pair on the Mac shows it).'],
  ['studio.local', '   ', 'Enter the token too (gtmux pair on the Mac shows it).'],
])('address %j, token %j: names the field and tries nothing', async (host, token, want) => {
  const tree = await render();
  await act(async () => {
    field(tree, TestIds.pairing.host).props.onChangeText(host);
    field(tree, TestIds.pairing.token).props.onChangeText(token);
  });
  await act(async () => {
    field(tree, TestIds.pairing.connect).props.onPress();
  });
  expect(error(tree)).toBe(want);
  expect(checkServer).not.toHaveBeenCalled();
});
