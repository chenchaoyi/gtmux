import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';

const agent = {agent: 'Claude Code', pane_id: '%1'} as Agent;

const turn = (prompt: string, steps: string[]): TranscriptTurn => ({
  prompt,
  response: 'ok',
  segments: [{text: 'ok', steps: steps.map(title => ({title, detail: ''}))}],
} as TranscriptTurn);

// Walk the rendered JSON: RN's <Text> is a HOST node whose type is the string 'Text',
// so querying by component type finds nothing (see HQActs.test).
type Json = {children?: (Json | string)[]} | string | null;
function texts(tree: renderer.ReactTestRenderer): string {
  const out: string[] = [];
  const walk = (n: Json) => {
    if (n == null) return;
    if (typeof n === 'string') return void out.push(n);
    (n.children ?? []).forEach(walk);
  };
  walk(tree.toJSON() as Json);
  return out.join(' | ');
}

function render(props: Partial<React.ComponentProps<typeof ChatView>>) {
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <ChatView
        agent={agent}
        lines={[]}
        status="idle"
        fontSize={13}
        pal={{fg: '#fff', fg2: '#ccc', fg3: '#888', divider: '#333', surface: '#222'}}
        lang="en"
        turns={[]}
        loading={false}
        {...props}
      />,
    );
  });
  return tree;
}

const history = turn('older', ['read events', 'ran digest']);
const live = turn('newest', ['read the board', 'checked %11']);

// The supervisor can take minutes over one turn. What the phone showed while that
// happened was an elapsed timer; what it showed afterwards was an 11.5pt toggle reading
// "2 steps". The process was presented exactly when it could not be watched.
test('the turn in flight shows its steps, not just a toggle', () => {
  const out = texts(render({turns: [history, live], status: 'working'}));
  expect(out).toContain('read the board');
  expect(out).toContain('checked %11');
});

test('history stays collapsed while the newest turn runs', () => {
  const out = texts(render({turns: [history, live], status: 'working'}));
  expect(out).not.toContain('ran digest');
  expect(out).toContain('2 steps'); // the toggle is still there to open it
});

test('a finished conversation reads as a conversation, not a log of tool calls', () => {
  const out = texts(render({turns: [history, live], status: 'idle'}));
  expect(out).not.toContain('read the board');
  expect(out).not.toContain('ran digest');
});

// Nothing to show yet is the case that reads as "the app froze", and it keeps the
// elapsed-time line — which never fabricates a duration it does not have.
test('working with no steps yet still says how long', () => {
  const bare = {prompt: 'go', response: '', segments: [{text: '', steps: []}]} as TranscriptTurn;
  const out = texts(render({turns: [bare], status: 'working', workingSince: 1000}));
  expect(out).toMatch(/Thinking…/);
});
