import React from 'react';
import {TextInput, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {launchImageLibrary} from 'react-native-image-picker';
import type {UploadResult, UploadStateCallback, UploadProgressCallback} from '../api/client';
import {Composer} from './Composer';
import {AttachSheet} from './AttachSheet';
import {ImageMarkup} from './ImageMarkup';
import {AttachmentUploadStatus} from './AttachmentUploadStatus';
import {paletteFor} from './theme';

jest.mock('react-native-image-picker', () => ({launchImageLibrary: jest.fn(), launchCamera: jest.fn()}));
jest.mock('./ImageMarkup', () => ({ImageMarkup: () => null}));
const photos = [
  {uri: 'file:///first.jpg', fileName: 'first.jpg', type: 'image/jpeg'},
  {uri: 'file:///second.png', fileName: 'second.png', type: 'image/png'},
];
let tree: renderer.ReactTestRenderer;
const sends = jest.fn();
const press = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0].props.onPress();
const flush = async () => {await act(async () => {for (let i = 0; i < 16; i++) await Promise.resolve();});};
const mount = async (extra: Record<string, unknown> = {}) => {
  await act(async () => {tree = renderer.create(<Composer pal={paletteFor('light')} lang="en" onSend={sends} onUpload={async () => ({path: '/saved.jpg'})} {...extra} />);});
};
const pick = async () => {
  if (!tree.root.findAll(n => n.props.testID === 'composer-attach' && typeof n.props.onPress === 'function').length) act(() => {press('composer-kbd');});
  act(() => {press('composer-attach');});
  act(() => {press('attach-0');});
  const modal = tree.root.findByType(AttachSheet).findAll(n => n.props.onDismiss && n.props.animationType != null)[0];
  await act(async () => {await modal.props.onDismiss();});
  await flush();
};
const staged = () => {
  const rows = new Map<string, renderer.ReactTestInstance>();
  for (const n of tree.root.findAll(n => String(n.props.testID ?? '').startsWith('attach-annotate-') && typeof n.props.onPress === 'function')) rows.set(n.props.testID, n);
  return [...rows.values()];
};
const uris = () => staged().map(n => n.props.children.props.source.uri);
const content = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(content).join('');
const text = () => tree.root.findAllByType(Text).map(content).join('|');
beforeEach(() => {sends.mockReset(); (launchImageLibrary as jest.Mock).mockResolvedValue({assets: photos});});
afterEach(async () => {await act(async () => {tree?.unmount();});});

test('batch photos stage in picker order and one edit replaces only its thumbnail', async () => {
  const upload = jest.fn(async (uri: string) => ({path: '/saved/' + uri.split('/').pop()}));
  await mount({onUpload: upload});
  (launchImageLibrary as jest.Mock).mockResolvedValue({assets: [photos[0], {}, photos[1]]});
  await pick();
  expect(launchImageLibrary).toHaveBeenCalledWith(expect.objectContaining({selectionLimit: 0, mediaType: 'photo'}));
  expect(uris()).toEqual(photos.map(p => p.uri));
  expect(tree.root.findByType(ImageMarkup).props.visible).toBe(false);
  expect(upload).not.toHaveBeenCalled();
  expect(sends).not.toHaveBeenCalled();
  act(() => staged()[1].props.onPress());
  expect(tree.root.findByType(ImageMarkup).props.uri).toBe(photos[1].uri);
  act(() => tree.root.findByType(ImageMarkup).props.onCancel());
  expect(uris()).toEqual(photos.map(p => p.uri));
  act(() => staged()[1].props.onPress());
  act(() => tree.root.findByType(ImageMarkup).props.onDone('file:///edited.jpg'));
  expect(uris()).toEqual([photos[0].uri, 'file:///edited.jpg']);
  act(() => {press('composer-send');});
  await flush();
  expect(upload.mock.calls.map(c => c[0])).toEqual([photos[0].uri, 'file:///edited.jpg']);
  expect(sends).toHaveBeenCalledTimes(1);
  expect(sends.mock.calls[0][0].text).toBe('/saved/first.jpg\n/saved/edited.jpg');
});

test('cancelling or failing a later selection preserves caption and staged batch', async () => {
  await mount(); await pick();
  act(() => tree.root.findByType(TextInput).props.onChangeText('Keep my caption'));
  for (const response of [{didCancel: true}, {errorCode: 'others'}, {assets: [{}]}]) {
    (launchImageLibrary as jest.Mock).mockResolvedValue(response);
    await pick();
    expect(uris()).toEqual(photos.map(p => p.uri));
    expect(tree.root.findByType(TextInput).props.value).toBe('Keep my caption');
    expect(sends).not.toHaveBeenCalled();
  }
});

test('a late picker result after unmount cannot stage or submit images', async () => {
  let finish!: (value: unknown) => void;
  (launchImageLibrary as jest.Mock).mockImplementation(() => new Promise(resolve => {finish = resolve;}));
  await mount();
  act(() => {press('composer-kbd');});
  act(() => {press('composer-attach');});
  act(() => {press('attach-0');});
  const modal = tree.root.findByType(AttachSheet).findAll(n => n.props.onDismiss && n.props.animationType != null)[0];
  let pending: Promise<void>;
  act(() => {pending = modal.props.onDismiss();});
  act(() => tree.unmount());
  await act(async () => {finish({assets: photos}); await pending;});
  expect(sends).not.toHaveBeenCalled();
});

test('upload phases keep the batch and caption on failure until an explicit successful retry', async () => {
  const jobs: {state?: UploadStateCallback; finish: (r: UploadResult) => void}[] = [];
  const upload = jest.fn((_uri: string, _name: string, _type: string, progress?: UploadProgressCallback) => new Promise<UploadResult>(finish => {jobs.push({state: progress?.onState, finish});}));
  await mount({onUpload: upload}); await pick();
  act(() => tree.root.findByType(TextInput).props.onChangeText('Two photos'));
  act(() => {press('composer-send');});
  const statuses = () => tree.root.findAllByType(AttachmentUploadStatus);
  const status = (name: string) => statuses().find(n => n.props.attachment.name === name)!;
  expect(statuses()).toHaveLength(2);
  for (const item of statuses()) {
    const thumbnail = tree.root.findAllByProps({testID: `attach-thumbnail-${item.props.attachment.id}`})[0];
    expect(thumbnail.findByType(AttachmentUploadStatus)).toBe(item);
  }
  expect(status('second.png').props.state).toBeUndefined();
  expect(tree.root.findAllByProps({testID: 'attachment-upload-status'})).toHaveLength(0);
  expect(text()).toContain('Preparing');
  expect(text()).not.toContain('0%');
  act(() => jobs[0].state?.({phase: 'uploading', fraction: 0.37, loaded: 37, total: 100}));
  expect(text()).toContain('37%');
  act(() => jobs[0].state?.({phase: 'processing', fraction: 1}));
  expect(text()).toContain('Saving');
  expect(text()).not.toContain('100%');
  expect(sends).not.toHaveBeenCalled();
  await act(async () => {jobs[0].finish({path: '/saved/first.jpg'});});
  expect(status('first.jpg').props.state.phase).toBe('complete');
  expect(status('second.png').props.state.phase).toBe('preparing');
  await act(async () => {jobs[1].finish({error: 'failed'});});
  expect(uris()).toEqual(photos.map(p => p.uri));
  expect(tree.root.findByType(TextInput).props.value).toBe('Two photos');
  expect(sends).not.toHaveBeenCalled();
  expect(status('second.png').props.failed).toBe(true);
  expect(status('first.jpg').props.failed).toBe(false);
  expect(upload).toHaveBeenCalledTimes(2);
  const retry = status('second.png').findAll(n => n.props.accessibilityRole === 'button' && typeof n.props.onPress === 'function')[0];
  act(() => {retry.props.onPress(); retry.props.onPress();});
  expect(upload).toHaveBeenCalledTimes(3);
  await act(async () => {jobs[2].finish({path: '/saved/first.jpg'});});
  expect(sends).not.toHaveBeenCalled();
  await act(async () => {jobs[3].finish({path: '/saved/second.png'});});
  await flush();
  expect(sends).toHaveBeenCalledTimes(1);
  expect(sends.mock.calls[0][0].text).toBe('Two photos\n/saved/first.jpg\n/saved/second.png');
  expect(statuses()).toHaveLength(0);
});
