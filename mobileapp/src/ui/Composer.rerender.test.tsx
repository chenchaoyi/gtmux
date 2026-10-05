import React, {useState} from 'react';
import {TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {Composer} from './Composer';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

// Typing stuttered while the terminal refreshed (2026-10-05). Two things the composer did
// on the JS thread every keystroke of its controlled input waits for:
// - its key row was a component declared inside Composer's body, a new type on every
//   render, so every keystroke and every re-render above unmounted and remounted the
//   whole row of touchables;
// - it re-rendered on every terminal refresh of the screen above, its props unchanged.

jest.mock('react-native-image-picker', () => ({launchCamera: jest.fn(), launchImageLibrary: jest.fn()}));
jest.mock('@react-native-documents/picker', () => ({pick: jest.fn()}));
jest.mock('@react-native-clipboard/clipboard', () => ({hasImage: jest.fn(), getImagePNG: jest.fn(), getString: jest.fn()}));
// Composer calls this once per render: counting the calls counts its renders.
jest.mock('react-native-safe-area-context', () => {
  const actual = jest.requireActual('react-native-safe-area-context');
  return {...actual, useSafeAreaInsets: jest.fn(() => ({top: 0, bottom: 0, left: 0, right: 0}))};
});

const pal = paletteFor('dark');
const tab = `${TestIds.composer.controlKey}-Tab`;

test('the key row is not torn down and rebuilt by a keystroke', async () => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<Composer pal={pal} lang="en" demo onSend={() => {}} />);
  });
  // An update keeps a component's fiber (it alternates between two); a remount makes a
  // new one, so the pill's fiber after typing says which happened.
  const fiber = () => (tree.root.findAll(n => n.props.testID === tab)[0] as unknown as {_fiber: {alternate: unknown}})._fiber;
  const before = fiber();
  // Open the keyboard, then type.
  await act(async () => { tree.root.findByProps({testID: TestIds.composer.keyboard}).props.onPress(); });
  const input = tree.root.findAllByType(TextInput)[0];
  await act(async () => { input.props.onChangeText('h'); });
  await act(async () => { input.props.onChangeText('hi'); });
  const after = fiber();
  expect(after === before || after === before.alternate).toBe(true);
  act(() => tree.unmount());
});

test('a re-render above with the same props does not re-render the composer', async () => {
  const onSend = () => {};
  let bump!: () => void;
  function Screen() {
    const [, setN] = useState(0);
    bump = () => setN(n => n + 1); // a terminal refresh, as far as the composer is concerned
    return <Composer pal={pal} lang="en" demo onSend={onSend} />;
  }
  let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<Screen />); });
  const renders = (useSafeAreaInsets as jest.Mock).mock.calls.length;
  await act(async () => { bump(); });
  await act(async () => { bump(); });
  expect((useSafeAreaInsets as jest.Mock).mock.calls.length).toBe(renders);
  act(() => tree.unmount());
});
