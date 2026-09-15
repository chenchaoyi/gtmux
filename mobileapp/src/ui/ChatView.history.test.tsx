import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';
import {TestIds} from '../constants/testIds';

// hq-console-history: a `/clear` used to be the end of the visible past (2026-09-15:
// 「每次只能展示上一次 clear 后的一点内容」). The serve now stitches the session before it
// on request, marking the seam on the first turn of the later session; the view draws
// the seam and offers the next hop while the serve says one exists.

const agent = {pane_id: '%1', agent: 'Claude Code', status: 'idle', loc: 'hq:0.0'} as unknown as Agent;
const turn = (n: number, extra: Partial<TranscriptTurn> = {}): TranscriptTurn => ({prompt: `p${n}`, response: `r${n}`, time: '', ...extra});

function mount(turns: TranscriptTurn[], earlierAvailable: boolean, onLoadEarlier?: () => void) {
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <ChatView agent={agent} lines={[]} status="idle" fontSize={13} pal={paletteFor('dark')} lang="zh" turns={turns} loading={false} earlierAvailable={earlierAvailable} onLoadEarlier={onLoadEarlier} />,
    );
  });
  return tree;
}
const texts = (t: renderer.ReactTestRenderer) => t.root.findAllByType(require('react-native').Text).map(n => String(n.props.children));

test('draws the seam where a later session began, with its clock and command', () => {
  const t = mount([turn(1), turn(2, {session_break: {kind: 'clear', at: Math.floor(new Date(2026, 8, 14, 23, 3).getTime() / 1000)}}), turn(3)], false);
  const seams = t.root.findAll(n => n.props?.testID === 'chat-session-seam' && typeof n.type === 'string');
  expect(seams).toHaveLength(1);
  expect(texts(t).join(' ')).toContain('— 新一段对话 · 23:03 /clear —');
});

test('offers the earlier session while the serve says one exists, and asks for it on tap', () => {
  let asked = 0;
  const t = mount([turn(1)], true, () => asked++);
  const row = t.root.findByProps({testID: TestIds.detail.chatEarlierSession});
  act(() => row.props.onPress());
  expect(asked).toBe(1);
  expect(mount([turn(1)], false, () => asked++).root.findAll(n => n.props?.testID === TestIds.detail.chatEarlierSession)).toHaveLength(0);
});

// hq-work direction A: the recorded acts sit between the bubbles at the moment they
// happened, and a run of the same verb folds to one row that opens on tap.
test('draws the recorded acts between the turns, and folds a run', () => {
  const at = (s: number) => new Date(s * 1000).toISOString();
  const T = 1_789_400_000;
  const turns: TranscriptTurn[] = [{...turn(1), time: at(T)}, {...turn(2), time: at(T + 600)}];
  const acts = [
    {ts: T + 100, kind: 'gtmux:audit:send', verb: '派活', target: '%9', detail: '司令答了…', outcome: '已送达', link: {kind: 'pane' as const, id: '%9'}},
    {ts: T + 700, kind: 'k', verb: '记账', target: '', detail: 'a'},
    {ts: T + 760, kind: 'k', verb: '记账', target: '', detail: 'b'},
    {ts: T + 820, kind: 'k', verb: '记账', target: '', detail: 'c'},
  ];
  let opened = '';
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <ChatView agent={agent} lines={[]} status="idle" fontSize={13} pal={paletteFor('dark')} lang="zh" turns={turns} loading={false} acts={acts} onOpenAct={l => (opened = l.id)} />,
    );
  });
  const hosts = (id: string) => tree.root.findAll(n => n.props?.testID === id && typeof n.type === 'string');
  const press = (id: string) => tree.root.findAll(n => n.props?.testID === id && typeof n.props.onPress === 'function')[0];
  expect(hosts('chat-act')).toHaveLength(1);
  expect(texts(tree).join(' ')).toContain('派活 → %9');
  act(() => press('chat-act').props.onPress());
  expect(opened).toBe('%9');
  expect(hosts('chat-act-fold')).toHaveLength(1);
  expect(texts(tree).join(' ').replace(/,/g, '')).toContain('×3');
  act(() => press('chat-act-fold').props.onPress());
  expect(hosts('chat-act')).toHaveLength(4);
});
