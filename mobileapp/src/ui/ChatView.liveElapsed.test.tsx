import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent} from '../api/types';
import type {AnsiLine} from './ansi';

// The turn in progress says how long it has run whether or not it has printed anything:
// the spec asks for it while the agent works, and the Live card that replaces the
// thinking marker used to drop it (%12, 2026-10-06).
jest.mock('../api/clock', () => ({serverNowSec: jest.fn(() => 2000)}));
const clock = jest.requireMock('../api/clock') as {serverNowSec: jest.Mock};

const agent = {pane_id: '%1', agent: 'Claude Code', status: 'working', loc: 'x:0.0'} as unknown as Agent;
const output: AnsiLine[] = [[{text: 'compiling…'} as AnsiLine[number]]];

function texts(lines: AnsiLine[], lang: 'en' | 'zh', since?: number): string {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <ChatView agent={agent} lines={lines} status="working" fontSize={13} pal={paletteFor('dark')}
        lang={lang} turns={[]} loading={false} workingSince={since} />,
    );
  });
  const out = tree!.root.findAll(n => n.type === Text).map(n => [n.props.children].flat().join('')).join(' | ');
  act(() => tree!.unmount());
  return out;
}

beforeEach(() => clock.serverNowSec.mockReturnValue(2000));

test.each([
  ['en', 'Thinking… 42s', 'Live · 42s'],
  ['zh', '正在思考… 42s', '正在进行 · 42s'],
] as const)('%s: the elapsed time is there with and without output', (lang, thinking, live) => {
  expect(texts([], lang, 1958)).toContain(thinking);
  const withOutput = texts(output, lang, 1958);
  expect(withOutput).toContain(live);
  expect(withOutput).toContain('compiling…');
});

test('an unknown start shows no duration on the Live card', () => {
  const out = texts(output, 'en', undefined);
  expect(out).toContain('Live');
  expect(out).not.toMatch(/Live · /);
});

test('the duration on the Live card moves with the clock', () => {
  jest.useFakeTimers();
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <ChatView agent={agent} lines={output} status="working" fontSize={13} pal={paletteFor('dark')}
        lang="en" turns={[]} loading={false} workingSince={1958} />,
    );
  });
  clock.serverNowSec.mockReturnValue(2003);
  act(() => {
    jest.advanceTimersByTime(1000);
  });
  const out = tree!.root.findAll(n => n.type === Text).map(n => [n.props.children].flat().join('')).join(' | ');
  expect(out).toContain('Live · 45s');
  act(() => tree!.unmount());
  jest.useRealTimers();
});
