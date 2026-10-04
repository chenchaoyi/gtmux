import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {TimeSeparator, wavePath} from './TimeSeparator';

// The label is drawn as a separator, not as a line of text: centred, with a rule either
// side of it, so a reader scrolling back sees the breaks rather than reading clocks
// 「参考一下这个样式分隔」.
const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => {
  for (const tree of mounted.splice(0)) act(() => tree.unmount());
});

const mount = async (el: React.ReactElement): Promise<renderer.ReactTestRenderer> => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(el);
  });
  mounted.push(tree);
  return tree;
};

describe('TimeSeparator', () => {
  test('shows the label it is given', async () => {
    const tree = await mount(<TimeSeparator label="今天 23:46" />);
    const labels = tree.root.findAllByType(Text).map(n => n.props.children);
    expect(labels).toContain('今天 23:46');
  });

  // Two flexed sides bracket the label, so it is centred by construction rather than by a
  // text alignment a long label would defeat.
  test('puts a rule on BOTH sides of it, with the label between them', async () => {
    const tree = await mount(<TimeSeparator label="Today 09:05" testID="sep" />);
    const row = tree.toJSON() as {children: {type: string}[]};
    expect(row.children).toHaveLength(3);
    expect(row.children[0].type).toBe('View');
    expect(row.children[1].type).toBe('Text');
    expect(row.children[2].type).toBe('View');
  });

  // The colours are fixed light-on-dark: the chat surface is always dark whatever the
  // app's appearance, a trap ChatView has paid for before.
  test('takes no palette', () => {
    const code = require('fs')
      .readFileSync('src/ui/TimeSeparator.tsx', 'utf8')
      .split('\n')
      .filter((l: string) => !l.trim().startsWith('//'))
      .join('\n');
    expect(code).not.toMatch(/pal\./);
  });
});

describe('wavePath', () => {
  test('draws at least one full period and starts at the left edge', () => {
    const d = wavePath(40);
    expect(d.startsWith('M0 ')).toBe(true);
    // Humps come in pairs (up then down), so a 40pt rule at an 11pt period has four.
    expect((d.match(/q /g) || []).length).toBe(8);
  });

  test('a width under one period draws nothing beyond the start', () => {
    expect(wavePath(0)).toBe('M0 2');
  });
});

// In the chat itself: a separator only where the conversation broke.
describe('the chat draws a separator only at a break', () => {
  const ChatView = require('./ChatView').ChatView;
  const {paletteFor} = require('./theme');
  // Local calendar times, never "N minutes ago": a relative time crosses midnight when the
  // suite runs just after it, and two turns minutes apart then sit on two different days.
  const at = (daysAgo: number, hour: number, minute: number) => {
    const d = new Date();
    d.setHours(12, 0, 0, 0);
    d.setDate(d.getDate() - daysAgo);
    d.setHours(hour, minute, 0, 0);
    return d.toISOString();
  };

  const seps = async (times: string[]): Promise<string[]> => {
    const turns = times.map((time, n) => ({prompt: `p${n}`, response: `r${n}`, time}));
    const tree = await mount(
      <ChatView
        agent={{pane_id: '%18', session: 'gtmux dev', agent: 'Claude Code', status: 'idle', source: 'tmux'} as never}
        lines={[]}
        status="idle"
        fontSize={14}
        lang="en"
        pal={paletteFor('dark')}
        turns={turns}
        loading={false}
      />,
    );
    return tree.root.findAllByType(TimeSeparator).map(n => String(n.props.label));
  };

  test('two turns minutes apart share one separator, at the start', async () => {
    expect(await seps([at(1, 23, 50), at(1, 23, 53)])).toHaveLength(1);
  });

  // Use local calendar days: a relative "26 hours ago" can land at 23:57, then its
  // three-minute neighbor lands on the next day and incorrectly earns a separator.
  // fmtTurnTime's wording is pinned deterministically in time.test.ts.
  test('a night and a pause each earn one, the turn between them earns none', async () => {
    const out = await seps([at(2, 10, 0), at(2, 10, 3), at(1, 10, 45), at(1, 11, 56)]);
    expect(out).toHaveLength(3);
    expect(out.every(l => l.length > 0)).toBe(true);
  });

  test('a conversation with no clocks draws none', async () => {
    expect(await seps(['', '', ''])).toHaveLength(0);
  });
});
