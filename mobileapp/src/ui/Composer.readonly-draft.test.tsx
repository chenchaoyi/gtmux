import React from 'react';
import {Modal, TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {Composer} from './Composer';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

jest.mock('react-native-image-picker', () => ({launchCamera: jest.fn(), launchImageLibrary: jest.fn()}));
jest.mock('@react-native-documents/picker', () => ({pick: jest.fn()}));
jest.mock('@react-native-clipboard/clipboard', () => ({hasImage: jest.fn(), getImagePNG: jest.fn(), getString: jest.fn()}));

const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => {for (const tree of mounted.splice(0)) act(() => tree.unmount());});

test.each(['en', 'zh'] as const)('CKD001 local-only draft opens and survives blocked submits in %s', async lang => {
  const onSend = jest.fn();
  const onUpload = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<Composer pal={paletteFor('dark')} lang={lang} enabled={false} draftOnly returnSends demo onSend={onSend} onUpload={onUpload} />);});
  mounted.push(tree);
  act(() => tree.root.findByProps({testID: TestIds.composer.keyboard}).props.onPress());
  const input = () => tree.root.findByProps({testID: TestIds.composer.input});
  expect(input().props.editable).toBe(true);
  expect(input().props.autoFocus).toBe(true);
  expect(tree.root.findByProps({testID: 'composer-draft-only'}).props.children).toContain(lang === 'zh' ? '草稿' : 'draft');
  act(() => input().props.onChangeText('Keep this local draft'));
  await act(async () => input().props.onSubmitEditing());
  await act(async () => tree.root.findByProps({testID: TestIds.composer.send}).props.onPress());
  expect(input().props.value).toBe('Keep this local draft');
  expect(tree.root.findByProps({testID: TestIds.composer.send}).props.disabled).toBe(true);
  expect(tree.root.findByProps({testID: TestIds.composer.attach}).props.disabled).toBe(true);
  act(() => tree.root.findByProps({testID: `${TestIds.composer.controlKey}-Enter`}).props.onPress());
  act(() => tree.root.findByProps({testID: TestIds.composer.expand}).props.onPress());
  const modal = tree.root.findAllByType(Modal).find(n => n.props.visible)!;
  const editor = modal.findByType(TextInput);
  expect(editor.props.editable).toBe(true);
  expect(editor.props.value).toBe('Keep this local draft');
  expect(onSend).not.toHaveBeenCalled();
  expect(onUpload).not.toHaveBeenCalled();
});

test('CKD001 verified sends remain enabled and ordinary disabled input stays disabled', async () => {
  const onSend = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  const render = (enabled: boolean) => <Composer pal={paletteFor('dark')} lang="en" enabled={enabled} demo onSend={onSend} />;
  await act(async () => {tree = renderer.create(render(false));});
  mounted.push(tree);
  act(() => tree.root.findByProps({testID: TestIds.composer.keyboard}).props.onPress());
  const input = () => tree.root.findByProps({testID: TestIds.composer.input});
  expect(input().props.editable).toBe(false);
  expect(tree.root.findAllByProps({testID: 'composer-draft-only'})).toHaveLength(0);
  await act(async () => tree.update(render(true)));
  act(() => input().props.onChangeText('A verified message'));
  await act(async () => tree.root.findByProps({testID: TestIds.composer.send}).props.onPress());
  expect(onSend).toHaveBeenCalledTimes(1);
  expect(onSend.mock.calls[0][0]).toEqual({text: 'A verified message', enter: true});
});
