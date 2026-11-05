import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {FlatList, Text, TouchableOpacity, Linking} from 'react-native';
import contract from '../../../tests/contracts/client-behavior.json';
import {TraceReader} from '../trace/TraceModal';
import {ChatView} from './ChatView';
import {MarkdownView} from './MarkdownView';
import {SenderAvatar} from './SenderAvatar';
import {ParticipantProfileModal} from './ParticipantProfileModal';
import {nativeDocumentPreview} from '../documents/nativePreview';
import {GtmuxClient, TranscriptTurn} from '../api/client';
import {toAgent} from '../api/types';
import {paletteFor} from './theme';

jest.mock('./ParticipantProfileModal', () => ({ParticipantProfileModal: jest.fn(() => null)}));
jest.mock('../documents/nativePreview', () => ({nativeDocumentPreview: {open: jest.fn().mockResolvedValue(undefined)}}));
let tree: renderer.ReactTestRenderer;
afterEach(() => { if (tree) act(() => tree.unmount()); jest.clearAllMocks(); });
const pal = paletteFor('light');
const agent = (name = 'Codex') => toAgent({pane_id:'%901',session_id:'contract-session',home_id:'fixture-home',agent:name,task:'Contract review',status:'idle',source:'tmux'});
const text = (node: renderer.ReactTestInstance) => node.findAllByType(Text).map(n => [n.props.children].flat(8).filter(v => typeof v === 'string' || typeof v === 'number').join('')).join('\n');

test.each(contract.agents)('CP-001 %s renders the shared Trace contract, not just a helper result', name => {
  act(() => {tree = renderer.create(<TraceReader visible agent={agent(name)} turns={[{...contract.trace.turn,agent:name} as TranscriptTurn]} loading={false} pal={pal} lang="en" onClose={() => {}} />);});
  expect(text(tree.root.findAllByProps({testID:'trace-count'})[0])).toContain(contract.trace.count);
  let list = tree.root.findByType(FlatList);
  expect(list.props.data.filter((r:any) => r.type==='event')).toHaveLength(0);
  const row = list.props.data.find((r:any) => r.type==='turn');
  expect(text(tree.root)).toContain('Turn 151');
  expect(text(tree.root)).toContain(contract.trace.summary);
  act(() => list.props.renderItem({item:row}).props.onToggle(row));
  list = tree.root.findByType(FlatList);
  const tool = list.props.data.find((r:any) => r.type==='event' && r.event.kind==='tool');
  expect(tool).toBeDefined();
  act(() => list.props.renderItem({item:tool}).props.onSelect(tool.event));
  expect(text(tree.root)).toContain(contract.trace.toolLabel);
  expect(tree.root.findAllByProps({testID:'trace-source'})[0].props.source).toBe(contract.trace.input);
  const reply = list.props.data.find((r:any) => r.type==='event' && r.event.kind==='assistant');
  act(() => list.props.renderItem({item:reply}).props.onSelect(reply.event));
  expect(text(tree.root)).toContain(contract.trace.replyLabel);
});

test('CP-002 HQ request retains its complete content and sender profile target', () => {
  act(() => {tree = renderer.create(<ChatView agent={agent()} lines={[]} status="idle" fontSize={15} pal={pal} lang="en" turns={[{...contract.hq,session_id:'contract-session'} as TranscriptTurn]} loading={false} />);});
  expect(tree.root.findAllByType(MarkdownView).some(n => n.props.source===contract.hq.prompt)).toBe(true);
  const buttons = tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID==='participant-avatar');
  const hq = buttons.find(n => n.findAllByType(SenderAvatar).some(a => a.props.from.kind === 'hq'))!;
  expect(hq).toBeDefined();
  act(() => hq.props.onPress());
  expect(tree.root.findByType(ParticipantProfileModal).props.target.sender.kind).toBe('hq');
  expect(tree.root.findByType(ParticipantProfileModal).props.target.turn.prompt).toBe(contract.hq.prompt);
});

test('CP-003 document actions use the scoped preview adapter and Markdown retains code', async () => {
  const client = new GtmuxClient('http://127.0.0.1:19234','synthetic');
  const open = jest.spyOn(Linking,'openURL');
  const prompt = `[Contract report](${contract.document.reference})`;
  act(() => {tree = renderer.create(<ChatView client={client} agent={agent()} lines={[]} status="idle" fontSize={15} pal={pal} lang="en" turns={[{prompt,response:contract.document.source,session_id:'contract-session'}]} loading={false} />);});
  const link=tree.root.findAllByType(Text).find(n=>n.props.onPress && n.props.accessibilityHint?.includes(contract.document.reference))!;
  expect(link).toBeDefined();
  await act(async () => {link.props.onPress();});
  expect(nativeDocumentPreview.open).toHaveBeenCalledWith(expect.objectContaining({reference:contract.document.reference,session_id:'contract-session',home_id:'fixture-home'}));
  expect(open).not.toHaveBeenCalled();
  expect(tree.root.findAllByType(MarkdownView).some(n=>n.props.source===contract.document.source)).toBe(true);
  expect(text(tree.root)).toContain(contract.document.emphasis);
  expect(JSON.stringify(tree.toJSON())).toContain('retained value');
  open.mockRestore();
});
