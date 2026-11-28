import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import type {Agent} from '../api/types';

let mockMarkdownBodyRenders = 0;
let mockToolStepsBodyRenders = 0;

jest.mock('./markdown', () => {
  const real = jest.requireActual('./markdown');
  return {
    ...real,
    parseBlocks: jest.fn((src: string) => {
      mockMarkdownBodyRenders++;
      return real.parseBlocks(src);
    }),
  };
});

jest.mock('./activity', () => {
  const real = jest.requireActual('./activity');
  return {
    ...real,
    activitySummary: jest.fn((...args: any[]) => {
      mockToolStepsBodyRenders++;
      return real.activitySummary(...args);
    }),
  };
});

beforeEach(() => {
  jest.useFakeTimers();
  mockMarkdownBodyRenders = 0;
  mockToolStepsBodyRenders = 0;
});

afterEach(() => {
  jest.useRealTimers();
});

test('1-second working elapsed timer updates LiveProgress without re-rendering historical MarkdownView or ToolSteps', () => {
  const startNow = Math.floor(Date.now() / 1000);
  const agent: Agent = {
    pane_id: '%5',
    session: 'dev',
    session_id: 's5',
    agent: 'Claude Code',
    status: 'working',
    since: startNow - 10,
    source: 'tmux',
  } as Agent;

  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <ChatView
        agent={agent}
        lines={[[{text: 'compiling...', color: ''}]]}
        hasLiveOutput
        status="working"
        workingSince={startNow - 10}
        fontSize={14}
        pal={paletteFor('dark')}
        lang="en"
        loading={false}
        turns={[
          {
            prompt: 'Inspect the build',
            response: 'Checked the build output and prepared the patch.',
            segments: [
              {
                text: 'Checked the build output and prepared the patch.',
                steps: [{kind: 'tool', title: 'Bash', detail: 'npm test', operation: 'command'}],
              },
            ],
          },
          {
            prompt: 'Ship it',
            response: 'Running verification suite now.',
            segments: [
              {
                text: 'Running verification suite now.',
                steps: [{kind: 'tool', title: 'Read', detail: 'package.json', operation: 'read'}],
              },
            ],
          },
        ]}
      />,
    );
  });

  const initialMdRenders = mockMarkdownBodyRenders;
  const initialToolRenders = mockToolStepsBodyRenders;
  expect(initialMdRenders).toBeGreaterThan(0);
  expect(initialToolRenders).toBeGreaterThan(0);

  const allText = () =>
    tree.root
      .findAllByType(Text)
      .map(n => n.props.children)
      .flat()
      .join(' ');
  expect(allText()).toContain('Working · 10s');

  act(() => {
    jest.advanceTimersByTime(5_000);
  });

  expect(allText()).toContain('Working · 15s');
  expect(mockMarkdownBodyRenders).toBe(initialMdRenders);
  expect(mockToolStepsBodyRenders).toBe(initialToolRenders);

  act(() => tree.unmount());
});
