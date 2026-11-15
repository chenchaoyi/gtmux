import React from 'react';
import {AccessibilityInfo, Animated, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {AttachmentUploadStatus} from './AttachmentUploadStatus';
import {paletteFor} from './theme';
import type {UploadState} from '../api/client';

let tree: renderer.ReactTestRenderer;
const attachments = [{id: 'first', name: 'first.jpg'}, {id: 'second', name: 'second.png'}];
const render = async (states: Record<string, UploadState>, lang: 'en' | 'zh' = 'en') => {
  await act(async () => {
    const node = <AttachmentUploadStatus attachments={attachments} states={states} pal={paletteFor('light')} lang={lang} />;
    if (tree) tree.update(node); else tree = renderer.create(node);
  });
};
const content = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(content).join('');
const words = () => tree.root.findAllByType(Text).map(content).join('|');
const progress = () => tree.root.findAll(n => n.props.accessibilityRole === 'progressbar')[0];
beforeEach(() => {jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);});
afterEach(async () => {await act(async () => tree?.unmount()); tree = undefined as any; jest.restoreAllMocks();});

test('upload status names the active file and hides percent during preparation and saving', async () => {
  await render({});
  expect(words()).toContain('Preparing'); expect(words()).not.toContain('0%');
  await render({first: {phase: 'complete', fraction: 1}, second: {phase: 'uploading', fraction: 0.37}});
  expect(words()).toContain('second.png'); expect(words()).toContain('2/2'); expect(words()).toContain('37%');
  expect(progress().props.accessibilityValue.now).toBe(37);
  await render({first: {phase: 'complete'}, second: {phase: 'processing', fraction: 1}}, 'zh');
  expect(words()).toContain('保存中'); expect(words()).not.toContain('100%');
  expect(progress().props.accessibilityValue.text).toBe('保存中');
});

test('Reduce Motion presents measured values without starting progress animation', async () => {
  const timing = jest.spyOn(Animated, 'timing');
  await render({first: {phase: 'uploading', fraction: 0.25}});
  await render({first: {phase: 'uploading', fraction: 0.62}});
  expect(timing).not.toHaveBeenCalled();
  expect(progress().props.accessibilityValue.now).toBe(62);
  expect(words()).toContain('62%');
});

test('progress animation only targets the latest measured byte fraction', async () => {
  (AccessibilityInfo.isReduceMotionEnabled as jest.Mock).mockResolvedValue(false);
  const timing = jest.spyOn(Animated, 'timing');
  await render({first: {phase: 'uploading', fraction: 0.37}});
  expect(timing.mock.calls.at(-1)?.[1].toValue).toBe(0.37);
  await render({first: {phase: 'uploading', fraction: 0.62}});
  expect(timing.mock.calls.at(-1)?.[1].toValue).toBe(0.62);
  expect(progress().props.accessibilityValue.now).toBe(62);
});
