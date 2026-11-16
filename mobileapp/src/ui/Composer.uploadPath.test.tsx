import React from 'react';
import {AccessibilityInfo, TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {launchImageLibrary} from 'react-native-image-picker';
import {Composer} from './Composer';
import {AttachSheet} from './AttachSheet';
import {AttachmentUploadStatus} from './AttachmentUploadStatus';
import {paletteFor} from './theme';

jest.mock('react-native-image-picker', () => ({launchImageLibrary: jest.fn(), launchCamera: jest.fn()}));
jest.mock('./ImageMarkup', () => ({ImageMarkup: () => null}));
let tree: renderer.ReactTestRenderer;
const sends = jest.fn();
const photos = [
  {uri: 'file:///first.jpg', fileName: 'first.jpg', type: 'image/jpeg'},
  {uri: 'file:///second.jpg', fileName: 'second.jpg', type: 'image/jpeg'},
];
const press = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0].props.onPress();
const flush = async () => {await act(async () => {for (let i = 0; i < 16; i++) await Promise.resolve();});};
beforeEach(() => {
  sends.mockReset(); (launchImageLibrary as jest.Mock).mockResolvedValue({assets: photos});
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);
});
afterEach(async () => {await act(async () => tree?.unmount()); jest.restoreAllMocks();});

test.each(['', ' \t\n'])('a blank later upload path retains the batch and caption until explicit retry: %j', async invalidPath => {
  const upload = jest.fn()
    .mockResolvedValueOnce({path: '/saved/first.jpg'})
    .mockResolvedValueOnce({path: invalidPath})
    .mockResolvedValueOnce({path: '/saved/first.jpg'})
    .mockResolvedValueOnce({path: '/saved/second.jpg'});
  await act(async () => {tree = renderer.create(<Composer pal={paletteFor('light')} lang="en" onSend={sends} onUpload={upload} />);});
  if (!tree.root.findAll(n => n.props.testID === 'composer-attach' && typeof n.props.onPress === 'function').length) act(() => {press('composer-kbd');});
  act(() => {press('composer-attach');});
  act(() => {press('attach-0');});
  const modal = tree.root.findByType(AttachSheet).findAll(n => n.props.onDismiss && n.props.animationType != null)[0];
  await act(async () => {await modal.props.onDismiss();}); await flush();
  act(() => tree.root.findByType(TextInput).props.onChangeText('Keep both photos'));
  act(() => {press('composer-send');}); await flush();
  expect(upload).toHaveBeenCalledTimes(2);
  expect(sends).not.toHaveBeenCalled();
  expect(tree.root.findByType(TextInput).props.value).toBe('Keep both photos');
  const statuses = tree.root.findAllByType(AttachmentUploadStatus);
  expect(statuses).toHaveLength(2);
  const failed = statuses.find(n => n.props.attachment.name === 'second.jpg')!;
  expect(failed.props.failed).toBe(true);
  expect(failed.props.state.phase).not.toBe('complete');
  const retry = failed.findAll(n => n.props.accessibilityRole === 'button' && typeof n.props.onPress === 'function')[0];
  act(() => {retry.props.onPress(); retry.props.onPress();}); await flush();
  expect(upload).toHaveBeenCalledTimes(4);
  expect(sends).toHaveBeenCalledTimes(1);
  expect(sends.mock.calls[0][0].text).toBe('Keep both photos\n/saved/first.jpg\n/saved/second.jpg');
  expect(tree.root.findAllByType(AttachmentUploadStatus)).toHaveLength(0);
});
