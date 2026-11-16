import React from 'react';
import {AccessibilityInfo, Animated, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {AttachmentUploadStatus} from './AttachmentUploadStatus';
import type {UploadState} from '../api/client';

let tree: renderer.ReactTestRenderer;
const attachment = {id: 'second', name: 'second.png'};
const render = async (state?: UploadState, lang: 'en' | 'zh' = 'en', extra: {failed?: boolean; onRetry?: () => void} = {}) => {
  await act(async () => {
    const node = <AttachmentUploadStatus attachment={attachment} state={state} lang={lang} {...extra} />;
    if (tree) tree.update(node); else tree = renderer.create(node);
  });
};
const content = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(content).join('');
const words = () => tree.root.findAllByType(Text).map(content).join('|');
const progress = () => tree.root.findAll(n => n.props.accessibilityRole === 'progressbar')[0];
beforeEach(() => {jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);});
afterEach(async () => {await act(async () => tree?.unmount()); tree = undefined as any; jest.restoreAllMocks();});

test('thumbnail status names its file and keeps queued preparation and saving indeterminate', async () => {
  await render();
  expect(progress().props.accessibilityLabel).toBe('second.png, Queued');
  expect(progress().props.accessibilityValue.now).toBeUndefined();
  expect(words()).not.toContain('0%');
  await render({phase: 'preparing'});
  expect(words()).toContain('Preparing'); expect(words()).not.toContain('0%');
  await render({phase: 'uploading', fraction: 0.37});
  expect(progress().props.accessibilityLabel).toBe('second.png, Uploading');
  expect(words()).toContain('37%');
  expect(progress().props.accessibilityValue.now).toBe(37);
  await render({phase: 'uploading', fraction: 1}, 'zh');
  expect(words()).toContain('保存中'); expect(words()).not.toContain('100%');
  expect(progress().props.accessibilityValue.text).toBe('保存中');
  expect(progress().props.accessibilityValue.now).toBeUndefined();
  await render({phase: 'complete', fraction: 1});
  expect(progress().props.accessibilityLabel).toBe('second.png, Uploaded');
  expect(progress().props.accessibilityValue.now).toBe(100);
});

test('Reduce Motion presents measured values without starting progress animation', async () => {
  const timing = jest.spyOn(Animated, 'timing');
  await render({phase: 'uploading', fraction: 0.25});
  await render({phase: 'uploading', fraction: 0.62});
  expect(timing).not.toHaveBeenCalled();
  expect(progress().props.accessibilityValue.now).toBe(62);
  expect(words()).toContain('62%');
});

test('progress animation only targets the latest measured byte fraction', async () => {
  (AccessibilityInfo.isReduceMotionEnabled as jest.Mock).mockResolvedValue(false);
  const timing = jest.spyOn(Animated, 'timing');
  await render({phase: 'uploading', fraction: 0.37});
  expect(timing.mock.calls.at(-1)?.[1].toValue).toBe(0.37);
  await render({phase: 'uploading', fraction: 0.62});
  expect(timing.mock.calls.at(-1)?.[1].toValue).toBe(0.62);
  expect(progress().props.accessibilityValue.now).toBe(62);
});

test('a failed thumbnail retries only on explicit action and respects send availability', async () => {
  const retry = jest.fn();
  await render(undefined, 'en', {failed: true, onRetry: retry});
  const button = () => tree.root.findAll(n => n.props.accessibilityRole === 'button' && typeof n.props.onPress === 'function')[0];
  expect(button().props.accessibilityLabel).toBe('second.png, Upload failed. Retry sending message');
  expect(retry).not.toHaveBeenCalled();
  act(() => button().props.onPress());
  expect(retry).toHaveBeenCalledTimes(1);
  await render(undefined, 'en', {failed: true});
  expect(tree.root.findAll(n => n.props.accessibilityRole === 'button')[0].props.disabled).toBe(true);
  expect(retry).toHaveBeenCalledTimes(1);
});
