import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {RowSheet} from './RowSheet';
import {Agent, ReplyOption} from '../api/types';

const pal = {bg: '#000', fg: '#fff', fg2: '#ccc', fg3: '#888', divider: '#333', surface: '#111'} as never;
const agent = (o: Partial<Agent> = {}): Agent =>
  ({pane_id: '%60', session: 'MP', loc: 'MP:0.1', agent: 'Claude Code', status: 'waiting', source: 'tmux', ...o} as Agent);

// Reported: "长按弹出会有多次弹出". The radar re-renders every ~1.5s, and the natural way
// to pass the fetcher — `loadOptions={a => client.options(a.pane_id)}` — is a new function
// identity each time. With that in the effect's dependencies the open sheet restarted its
// entrance spring and re-fetched, over and over, while it sat there.
test('a re-render of the screen above does not re-open the sheet', async () => {
  let fetches = 0;
  const load = async (): Promise<ReplyOption[]> => {
    fetches++;
    return [{n: 1, label: 'Yes'}];
  };
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(
      <RowSheet agent={agent()} pal={pal} lang="en" onClose={() => {}} onJump={() => {}} onDiff={() => {}} onAct={() => {}} loadOptions={load} />,
    );
  });
  expect(fetches).toBe(1);

  // Five polls: a NEW agent object and a NEW inline callback each time, same pane.
  for (let i = 0; i < 5; i++) {
    await act(async () => {
      tree.update(
        <RowSheet
          agent={agent({activity_at: 1000 + i} as Partial<Agent>)}
          pal={pal}
          lang="en"
          onClose={() => {}}
          onJump={() => {}}
          onDiff={() => {}}
          onAct={() => {}}
          loadOptions={async () => {
            fetches++;
            return [{n: 1, label: 'Yes'}];
          }}
        />,
      );
    });
  }
  expect(fetches).toBe(1);
});

// Opening a DIFFERENT row is a different sheet and must start over.
test('a different row re-opens', async () => {
  let fetches = 0;
  const load = async (): Promise<ReplyOption[]> => {
    fetches++;
    return [];
  };
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(
      <RowSheet agent={agent()} pal={pal} lang="en" onClose={() => {}} onJump={() => {}} onDiff={() => {}} onAct={() => {}} loadOptions={load} />,
    );
  });
  await act(async () => {
    tree.update(
      <RowSheet agent={agent({pane_id: '%61'})} pal={pal} lang="en" onClose={() => {}} onJump={() => {}} onDiff={() => {}} onAct={() => {}} loadOptions={load} />,
    );
  });
  expect(fetches).toBe(2);
});
