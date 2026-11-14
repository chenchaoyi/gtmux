import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {FlatList, TouchableOpacity} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {TraceReader} from '../trace/TraceModal';
import {ToolSteps} from './ToolSteps';
import {ToolCallDetails} from './ToolCallDetails';
import {CodeBlock} from './CodeBlock';
import {CodeText} from './CodeText';
import {callRaw} from './toolPresentation';
import {paletteFor} from './theme';
import type {TranscriptStep} from '../api/client';

const agent: any = {pane_id: '%fixture', session_id: 'fixture', home_id: 'home', agent: 'Codex', source: 'tmux', status: 'idle'};
let tree: renderer.ReactTestRenderer;
function press(id: string, within = tree.root) {const button = within.findAllByType(TouchableOpacity).find(n => n.props.testID === id); expect(button).toBeDefined(); act(() => button!.props.onPress());}
function stepFor(original: string): TranscriptStep {return {kind: 'tool', title: 'exec', operation: 'command', input_kind: 'arguments', input: '["/bin/bash","-c","printf fixture"]', call: {name: 'exec', loaded: true, has_arguments: true, arguments: '["/bin/bash","-c","printf fixture"]', has_result: true, result: JSON.stringify({content: [{type: 'text', text: original}]})}};}
async function mount(surface: string, step: TranscriptStep, lang: 'en' | 'zh') {
  if (surface === 'Chat') {
    await act(async () => {tree = renderer.create(<ToolSteps steps={[step]} open onToggle={() => {}} lang={lang} fontSize={14}/>);});
    press('tool-step-toggle-0');
  } else {
    await act(async () => {tree = renderer.create(<TraceReader visible agent={agent} turns={[{session_id: 'fixture', prompt: 'fixture', response: '', segments: [{steps: [step]}]}]} loading={false} pal={paletteFor('dark')} lang={lang} onClose={() => {}}/>);});
    press('trace-fold-turns');
    let rows = tree.root.findByType(FlatList).props.data;
    const batch = rows.find((r: any) => r.type === 'calls'); if (batch) press('trace-calls-' + batch.key);
    rows = tree.root.findByType(FlatList).props.data;
    const row = rows.find((r: any) => r.type === 'event' && r.event.kind === 'tool'); expect(row).toBeDefined();
    press('trace-record-' + row.event.key);
  }
}
afterEach(() => {act(() => tree?.unmount()); jest.clearAllMocks();});
const inputs = ['\x1b[32mOK\x1b[0m', '\x1b[32m' + 'recorded line\n'.repeat(1600) + '\x1b[0mFINAL-ORIGINAL-MARKER'];
test.each(['Chat', 'Trace'].flatMap(surface => (['en', 'zh'] as const).flatMap(lang => inputs.map((original, i) => ({surface, lang, original, paged: i === 1})))))('CRR-002 $surface $lang paged=$paged expanded copy retains complete original bytes', async ({surface, lang, original, paged}) => {
  const step = stepFor(original), before = JSON.stringify(step); await mount(surface, step, lang);
  const section = tree.root.findAllByProps({testID: 'call-section-result-0'})[0]; expect(section).toBeDefined();
  press('call-code-copy', section); expect(Clipboard.setString).toHaveBeenLastCalledWith(original);
  press('call-code-open', section); press('code-reader-copy');
  expect(Clipboard.setString).toHaveBeenLastCalledWith(original);
  const copy = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === 'code-reader-copy')!;
  expect(copy.props.accessibilityLabel).toBe(lang === 'zh' ? '复制完整结果' : 'Copy full Result');
  if (paged) {
    expect(tree.root.findAllByProps({testID: 'code-reader-range'})[0].props.children).toBe(lang === 'zh' ? '第 1 / 2 页' : 'Page 1 / 2');
    press('code-reader-close');
    const next = section.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel === (lang === 'zh' ? '下一页' : 'Next page'))!;
    act(() => next.props.onPress()); press('call-code-open', section); press('code-reader-copy');
    expect(Clipboard.setString).toHaveBeenLastCalledWith(original);
    expect(tree.root.findAllByProps({testID: 'code-reader-range'})[0].props.children).toBe(lang === 'zh' ? '第 2 / 2 页' : 'Page 2 / 2');
  }
  expect(JSON.stringify(step)).toBe(before);
});

test('CRR-002 Raw and ordinary readers keep their original complete copy source', () => {
  const step = stepFor(inputs[1]); act(() => {tree = renderer.create(<ToolCallDetails step={step} lang="en" raw/>);});
  const raw = callRaw(step); press('call-code-open'); press('code-reader-copy'); expect(Clipboard.setString).toHaveBeenLastCalledWith(raw);
  act(() => tree.unmount()); const plain = 'const original = {value: 1};'; act(() => {tree = renderer.create(<CodeBlock source={plain} lang="en" hint="javascript"/>);});
  press('code-open'); press('code-reader-copy'); expect(Clipboard.setString).toHaveBeenLastCalledWith(plain);
  expect(tree.root.findAllByType(CodeText).some(n => n.props.source === plain)).toBe(true);
});

test('CRR-002 an open reader freezes exact copy bytes and label with the displayed snapshot', () => {
  const first = {source: 'readable first', copySource: '\x1b[32mrecorded first\x1b[0m', copyLabel: 'Copy first result'};
  act(() => {tree = renderer.create(<CodeBlock {...first} lang="en"/>);}); press('code-open');
  act(() => tree.update(<CodeBlock source="readable second" copySource="recorded second" copyLabel="Copy second result" lang="en"/>));
  const reader = tree.root.findAllByType(CodeText).find(n => n.props.testID === 'code-reader-text')!;
  expect(reader.props.source).toBe(first.source);
  press('code-reader-copy'); expect(Clipboard.setString).toHaveBeenLastCalledWith(first.copySource);
  expect(tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === 'code-reader-copy')!.props.accessibilityLabel).toBe(first.copyLabel);
});
