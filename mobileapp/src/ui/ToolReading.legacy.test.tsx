import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {TouchableOpacity} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {ToolCallDetails} from './ToolCallDetails';
import {CodeText} from './CodeText';
import {callRaw} from './toolPresentation';
import type {TranscriptStep} from '../api/client';

test.each(['en', 'zh'] as const)('RFD-001 legacy exec %s reads as one command and clean output while copy remains exact', lang => {
  const command = "printf 'hello'", source = JSON.stringify(['/bin/bash', '-c', command]);
  const step = {kind: 'tool', title: 'exec', input_kind: 'arguments', input: source, call: {name: 'exec', loaded: true, has_arguments: true, arguments: source, has_result: true, result: JSON.stringify({content: [{type: 'text', text: '[32mOK[0m'}, {type: 'text', text: 'id=18446744073709551615'}]})}} as TranscriptStep;
  const before = JSON.stringify(step); let tree!: renderer.ReactTestRenderer;
  act(() => {tree = renderer.create(<ToolCallDetails step={step} lang={lang} />);});
  try {
    expect(tree.root.findAllByProps({testID: 'call-section-input'}).length).toBeGreaterThan(0);
    const input = tree.root.findByProps({testID: 'call-section-input'});
    expect(input.findByType(CodeText).props.source).toBe(command);
    expect(input.findAllByType(TouchableOpacity).filter(n => n.props.testID === 'call-code-outline-toggle')).toHaveLength(0);
    expect(tree.root.findAllByType(CodeText).map(n => n.props.source)).toEqual([command, 'OK', 'id=18446744073709551615']);
    const copy = input.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel === (lang === 'zh' ? '复制完整命令参数' : 'Copy full Command arguments'))!;
    act(() => copy.props.onPress()); expect(Clipboard.setString).toHaveBeenLastCalledWith(source);
    const raw = input.findAllByType(TouchableOpacity).find(n => n.props.testID === 'call-code-raw-toggle')!;
    act(() => raw.props.onPress()); expect(input.findByType(CodeText).props.source).toBe(source);
    act(() => tree.update(<ToolCallDetails step={step} lang={lang} raw />));
    expect(tree.root.findByType(CodeText).props.source).toBe(callRaw(step)); expect(JSON.stringify(step)).toBe(before);
  } finally {act(() => tree.unmount());}
});

test('RFD-001 an explicit generic tool operation keeps its original data instead of guessing a shell command', () => {
  const source = '["/bin/bash","-c","literal"]';
  const step = {kind: 'tool', title: 'exec', operation: 'tool', input_kind: 'arguments', input: source, call: {name: 'exec', loaded: true, has_arguments: true, arguments: source}} as TranscriptStep;
  let tree!: renderer.ReactTestRenderer; act(() => {tree = renderer.create(<ToolCallDetails step={step} lang="en" />);});
  try {expect(tree.root.findAllByProps({testID: 'call-section-input'})).toHaveLength(0); expect(tree.root.findByType(CodeText).props.source).toBe(source);}
  finally {act(() => tree.unmount());}
});
