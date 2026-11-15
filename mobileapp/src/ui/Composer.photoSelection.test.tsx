import React from 'react';
import {Text, TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {launchImageLibrary} from 'react-native-image-picker';
import {pick} from '@react-native-documents/picker';
import {Composer} from './Composer';
import {AttachSheet} from './AttachSheet';
import {paletteFor} from './theme';
import type {UploadResult} from '../api/client';

jest.mock('react-native-image-picker', () => ({launchImageLibrary: jest.fn(), launchCamera: jest.fn()}));
jest.mock('@react-native-documents/picker', () => ({pick: jest.fn()}));
jest.mock('./ImageMarkup', () => ({ImageMarkup: () => null}));
const initial = [{uri: 'file:///initial1.jpg'}, {uri: 'file:///initial2.jpg'}];
const later = [{uri: 'file:///later1.jpg'}, {uri: 'file:///later2.jpg'}];
let tree: renderer.ReactTestRenderer;
let releasePicker: ((value: any) => void) | undefined;
const button = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0];
const flush = async () => {await act(async () => {for (let i = 0; i < 16; i++) await Promise.resolve();});};
const choose = async (id = 'attach-0') => {
  act(() => {button('composer-attach').props.onPress();});
  act(() => {button(id).props.onPress();});
  const modal = tree.root.findByType(AttachSheet).findAll(n => n.props.onDismiss && n.props.animationType != null)[0];
  await act(async () => {modal.props.onDismiss();}); await flush();
};
const uris = () => [...new Map(tree.root.findAll(n => String(n.props.testID ?? '').startsWith('attach-annotate-') && typeof n.props.onPress === 'function').map(n => [n.props.testID, n])).values()].map(n => n.props.children.props.source.uri);
const content = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(content).join('');
const words = () => tree.root.findAllByType(Text).map(content).join('|');
const mount = async (onUpload: (uri: string) => Promise<UploadResult>, onSend: jest.Mock) => {
  (launchImageLibrary as jest.Mock).mockResolvedValueOnce({assets: initial});
  await act(async () => {tree = renderer.create(<Composer pal={paletteFor('light')} lang="en" returnSends onUpload={onUpload} onSend={onSend} />);});
  act(() => {button('composer-kbd').props.onPress();}); await choose();
  act(() => {tree.root.findByType(TextInput).props.onChangeText('Original caption');});
};
beforeEach(() => {jest.clearAllMocks(); releasePicker = undefined;});
afterEach(async () => {await act(async () => {tree?.unmount(); releasePicker?.({didCancel: true});}); await flush();});

test('pending photo reading blocks button Return and captured callbacks until all photos stage', async () => {
  const send = jest.fn(); const upload = jest.fn(async (uri: string) => ({path: '/saved/' + uri.split('/').pop()}));
  await mount(upload, send);
  const capturedSend = button('composer-send').props.onPress;
  const capturedReturn = tree.root.findByType(TextInput).props.onSubmitEditing;
  const capturedPick = tree.root.findByType(AttachSheet).props.onPhoto;
  (launchImageLibrary as jest.Mock).mockImplementationOnce(() => new Promise(resolve => {releasePicker = resolve;}));
  await choose();
  expect(button('composer-send').props.disabled).toBe(true);
  expect(words()).toContain('Reading photos');
  await act(async () => {button('composer-send').props.onPress(); capturedSend(); capturedReturn(); void capturedPick();});
  expect(launchImageLibrary).toHaveBeenCalledTimes(2);
  expect(upload).not.toHaveBeenCalled(); expect(send).not.toHaveBeenCalled();
  expect(uris()).toEqual(initial.map(a => a.uri));
  expect(tree.root.findByType(TextInput).props.value).toBe('Original caption');
  await act(async () => {releasePicker!({assets: later});}); await flush();
  expect(uris()).toEqual([...initial, ...later].map(a => a.uri));
  expect(button('composer-send').props.disabled).toBe(false);
  expect(words()).not.toContain('Reading photos');
  expect(upload).not.toHaveBeenCalled(); expect(send).not.toHaveBeenCalled();
  act(() => {button('composer-send').props.onPress();}); await flush();
  expect(upload.mock.calls.map(c => c[0])).toEqual([...initial, ...later].map(a => a.uri));
  expect(send).toHaveBeenCalledTimes(1);
  expect(send.mock.calls[0][0].text).toBe('Original caption\n/saved/initial1.jpg\n/saved/initial2.jpg\n/saved/later1.jpg\n/saved/later2.jpg');
});

test('cancel error and rejection release photo reading while preserving the existing draft', async () => {
  const send = jest.fn(); const upload = jest.fn(async (uri: string) => ({path: '/saved/' + uri.split('/').pop()}));
  await mount(upload, send);
  for (const kind of ['cancel', 'error', 'reject']) {
    let reject!: (error: Error) => void;
    (launchImageLibrary as jest.Mock).mockImplementationOnce(() => new Promise((resolve, fail) => {releasePicker = resolve; reject = fail;}));
    await choose(); expect(button('composer-send').props.disabled).toBe(true);
    await act(async () => {if (kind === 'reject') reject(new Error('synthetic read failure')); else releasePicker!((kind === 'cancel') ? {didCancel: true} : {errorCode: 'others'});}); await flush();
    expect(button('composer-send').props.disabled).toBe(false);
    expect(uris()).toEqual(initial.map(a => a.uri));
    expect(tree.root.findByType(TextInput).props.value).toBe('Original caption');
    expect(upload).not.toHaveBeenCalled(); expect(send).not.toHaveBeenCalled();
  }
  expect(launchImageLibrary).toHaveBeenCalledTimes(4);
  act(() => {button('composer-send').props.onPress();}); await flush(); expect(send).toHaveBeenCalledTimes(1);
});

test('a successful send removes only its snapshot and preserves a file returned during upload', async () => {
  const finish: ((result: UploadResult) => void)[] = [];
  const send = jest.fn(); const upload = jest.fn((_uri: string) => new Promise<UploadResult>(resolve => {finish.push(resolve);}));
  await mount(upload, send);
  let finishFile!: (value: unknown) => void;
  (pick as jest.Mock).mockImplementationOnce(() => new Promise(resolve => {finishFile = resolve;}));
  await choose('attach-2');
  act(() => {button('composer-send').props.onPress();}); expect(upload).toHaveBeenCalledTimes(1);
  await act(async () => {finishFile([{uri: 'file:///later.txt', name: 'later.txt', type: 'text/plain'}]);}); await flush();
  expect(words()).toContain('later.txt');
  await act(async () => {finish[0]({path: '/saved/initial1.jpg'});});
  await act(async () => {finish[1]({path: '/saved/initial2.jpg'});}); await flush();
  expect(upload.mock.calls.map(c => c[0])).toEqual(initial.map(a => a.uri));
  expect(send).toHaveBeenCalledTimes(1);
  expect(send.mock.calls[0][0].text).toBe('Original caption\n/saved/initial1.jpg\n/saved/initial2.jpg');
  expect(words()).toContain('later.txt');
  const remaining = new Map(tree.root.findAll(n => String(n.props.testID ?? '').startsWith('attach-remove-') && typeof n.props.onPress === 'function').map(n => [n.props.testID, n]));
  expect(remaining.size).toBe(1);
});
