import React from 'react';
import {TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {launchImageLibrary} from 'react-native-image-picker';
import {Composer} from './Composer';
import {AttachSheet} from './AttachSheet';
import {paletteFor} from './theme';
jest.mock('react-native-image-picker', () => ({launchImageLibrary: jest.fn(), launchCamera: jest.fn()}));
jest.mock('./ImageMarkup', () => ({ImageMarkup: () => null}));
let tree: renderer.ReactTestRenderer;
const send = jest.fn(), upload = jest.fn();
const props = {pal: paletteFor('light'), lang: 'en' as const, onSend: send, onUpload: upload};
const press = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0].props.onPress();
const flush = async () => {await act(async () => {for (let i = 0; i < 12; i++) await Promise.resolve();});};
beforeEach(() => {jest.clearAllMocks(); (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);});
afterEach(() => {act(() => tree?.unmount());});
test('SFR-007 restore appends once without replacing a newer draft or staged photos', async () => {
  (launchImageLibrary as jest.Mock).mockResolvedValue({assets: [
    {uri: 'file:///one.jpg', fileName: 'one.jpg', type: 'image/jpeg'},
    {uri: 'file:///two.jpg', fileName: 'two.jpg', type: 'image/jpeg'},
  ]});
  await act(async () => {tree = renderer.create(<Composer {...props} />);});
  act(() => press('composer-kbd')); act(() => press('composer-attach')); act(() => press('attach-0'));
  const modal = tree.root.findByType(AttachSheet).findAll(n => n.props.onDismiss && n.props.animationType != null)[0];
  await act(async () => {await modal.props.onDismiss();}); await flush();
  act(() => tree.root.findByType(TextInput).props.onChangeText('newer text'));
  const photos = () => tree.root.findAll(n => String(n.props.testID ?? '').startsWith('attach-annotate-') && typeof n.props.onPress === 'function').map(n => n.props.testID);
  const before = photos(); expect(new Set(before).size).toBe(2);
  const restore = {text: 'held message\n/server/already-uploaded.jpg', at: 1};
  act(() => {tree.update(<Composer {...props} restore={restore} />);}); await flush();
  expect(tree.root.findByType(TextInput).props.value).toBe('newer text\n\n' + restore.text);
  expect(photos()).toEqual(before);
  act(() => {tree.update(<Composer {...props} restore={{...restore}} />);});
  expect(tree.root.findByType(TextInput).props.value).toBe('newer text\n\n' + restore.text);
  expect(send).not.toHaveBeenCalled(); expect(upload).not.toHaveBeenCalled();
  act(() => {tree.update(<Composer {...props} key="new-scope" />);});
  expect(photos()).toHaveLength(0);
  act(() => press('composer-kbd'));
  expect(tree.root.findByType(TextInput).props.value).toBe('');
});
test('SFR-007 empty recovery opens the editor, outranks a delayed stored draft and avoids duplicate restored text', async () => {
  let finish!: (value: string) => void;
  (AsyncStorage.getItem as jest.Mock).mockReturnValue(new Promise(resolve => {finish = resolve;}));
  await act(async () => {tree = renderer.create(<Composer {...props} draftKey="synthetic" />);});
  const restore = {text: 'held', at: 1};
  act(() => {tree.update(<Composer {...props} draftKey="synthetic" restore={restore} />);});
  await act(async () => finish(JSON.stringify({synthetic: {text: 'old disk draft', at: Date.now()}}))); await flush();
  expect(tree.root.findByType(TextInput).props.value).toBe('held');
  act(() => {tree.update(<Composer {...props} draftKey="synthetic" restore={{text: 'held', at: 2}} />);});
  expect(tree.root.findByType(TextInput).props.value).toBe('held');
  expect(send).not.toHaveBeenCalled();
});
