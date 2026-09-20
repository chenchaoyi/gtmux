import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {SenderAvatar} from './SenderAvatar';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import type {TranscriptTurn} from '../api/client';

// A conversation has to say whose words these are. On 2026-09-20 a message HQ relayed
// into the commander's pane was drawn under his own avatar, and his history stopped
// telling him which instructions were his 「对话里应该增加hq的角色」.

const texts = (tree: renderer.ReactTestRenderer): string[] =>
  tree.root.findAllByType(Text).flatMap(n => (Array.isArray(n.props.children) ? n.props.children : [n.props.children]))
    .filter((c): c is string => typeof c === 'string');

// Mount inside act: these views run effects, and reading .root on a renderer that has
// not settled reports it as unmounted.
const mount = async (el: React.ReactElement): Promise<renderer.ReactTestRenderer> => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(el);
  });
  return tree;
};

describe('SenderAvatar', () => {
  test('HQ wears a word, because it is a role and not a tool', async () => {
    expect(texts(await mount(<SenderAvatar from={{kind: 'hq', label: 'HQ'}} />))).toContain('HQ');
  });

  test('another session wears its agent, not the letters HQ', async () => {
    const tree = await mount(<SenderAvatar from={{kind: 'agent', label: 'gtmux dev', agent: 'Claude Code', pane: '%31'}} />);
    expect(texts(tree)).not.toContain('HQ');
  });

  // The chat surface is always dark whatever the app's appearance, so a theme colour here
  // goes invisible in light mode — the trap ChatView already paid for once. The mark takes
  // no palette at all, so light and dark must render identically.
  test('its colours are fixed light-on-dark, never the theme', async () => {
    const light = await mount(<SenderAvatar from={{kind: 'hq', label: 'HQ'}} />);
    const dark = await mount(<SenderAvatar from={{kind: 'hq', label: 'HQ'}} />);
    expect(JSON.stringify(light.toJSON())).toBe(JSON.stringify(dark.toJSON()));
    const flat = JSON.stringify(light.toJSON());
    // Light-on-dark: a near-white mark, never the near-black the theme would hand it.
    expect(flat).toContain('rgba(255,255,255,0.92)');
    expect(flat).not.toContain('#1D1D1F');
  });

});

describe('the chat says who sent a turn', () => {
  const turn = (o: Partial<TranscriptTurn>): TranscriptTurn => ({prompt: 'p', response: 'r', ...o});
  const render = (turns: TranscriptTurn[], lang: 'en' | 'zh' = 'en') =>
    mount(
      <ChatView
        agent={{pane_id: '%18', session: 'gtmux dev', agent: 'Claude Code', status: 'idle', source: 'tmux'} as never}
        lines={[]}
        status="idle"
        fontSize={14}
        lang={lang}
        pal={paletteFor('dark')}
        turns={turns}
        loading={false}
      />,
    );

  test("a turn HQ delivered names HQ, and the reader's own turn names nobody", async () => {
    const tree = await render([
      turn({prompt: '先停一下，版本对不上', from: {kind: 'hq', label: 'HQ'}}),
      turn({prompt: '对话里应该增加hq的角色'}),
    ]);
    const all = texts(tree);
    expect(all).toContain('from HQ');
    // Exactly one turn is attributed: the unmarked one must gain no chrome at all.
    expect(all.filter(s => s.startsWith('from ')).length).toBe(1);
  });

  test('a dispatch from another session names that session', async () => {
    const tree = await render([turn({prompt: '把这个做完', from: {kind: 'agent', label: 'MP analysis', agent: 'Codex', pane: '%11'}})]);
    expect(texts(tree)).toContain('from MP analysis');
  });

  test('it speaks the reader’s language', async () => {
    expect(texts(await render([turn({prompt: 'x', from: {kind: 'hq', label: 'HQ'}})], 'zh'))).toContain('HQ 发来的');
  });

  test('an ordinary conversation is unchanged', async () => {
    expect(texts(await render([turn({prompt: 'hello'})])).some(s => s.startsWith('from '))).toBe(false);
  });
});
