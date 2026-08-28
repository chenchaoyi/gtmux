import {buildRowSheet, focusCommand} from './rowSheetModel';
import {Agent} from '../api/types';

const NOW = 1_800_000_000;
const agent = (over: Partial<Agent>): Agent =>
  ({
    pane_id: '%60', session: 'MP', window: '0', pane: '1', loc: 'MP:0.1',
    agent: 'Codex', status: 'working', task: 'skill review', latest: false,
    activity: false, source: 'tmux', since: NOW - 600,
    ...over,
  } as Agent);

const keys = (a: Agent) => buildRowSheet(a, 'en', NOW).actions.map(x => x.key);
const action = (a: Agent, k: string) => buildRowSheet(a, 'en', NOW).actions.find(x => x.key === k)!;

// The alert this replaces showed the agent name and the task — both already on the row
// that was pressed. What the row genuinely cannot show is the whole of a clamped task
// and error, and WHICH pane this is.
describe('what the row cannot say', () => {
  it('leads with the pane id, because a truncated name cannot tell two panes apart', () => {
    const m = buildRowSheet(agent({}), 'en', NOW);
    expect(m.identity[0]).toBe('%60 · MP:0.1');
  });

  it('carries the full error text, which is exactly what gets truncated on the row', () => {
    const long = "You've hit your weekly limit · resets Aug 28 at 11:00 (America/Los_Angeles)";
    const m = buildRowSheet(agent({status: 'idle', error: true, error_text: long}), 'en', NOW);
    expect(m.error).toBe(long);
    expect(m.status).toContain('errored');
  });

  it('says how long it has been in this state', () => {
    expect(buildRowSheet(agent({}), 'en', NOW).status).toBe('working · 10m');
    expect(buildRowSheet(agent({}), 'zh', NOW).status).toContain('10 分钟');
  });
});

// The radar holds four different kinds of thing and the alert gave all four the same two
// lines. For two of them that is not terse — it is wrong.
describe('each kind tells its own truth', () => {
  it('a session outside tmux cannot be jumped to, and says why', () => {
    const m = buildRowSheet(agent({source: 'native'}), 'en', NOW);
    expect(m.kind).toBe('native');
    expect(m.identity[0]).toContain('Not in tmux');
    const jump = m.actions.find(a => a.key === 'jump')!;
    expect(jump.disabled).toBe(true);
    // The reason must be present: an action that is simply missing leaves the reader
    // wondering whether they missed it.
    expect(jump.sub).toContain('not in tmux');
  });

  it('a watched plain pane is given no agent status', () => {
    const m = buildRowSheet(agent({watched: true, agent: '', task: 'bash'}), 'en', NOW);
    expect(m.kind).toBe('watched');
    expect(m.status).toBeUndefined();
    expect(keys(agent({watched: true}))).toContain('jump'); // a plain pane is still a pane
  });

  it('offers the diff only where there is a repo', () => {
    expect(keys(agent({branch: 'main'}))).toContain('diff');
    expect(keys(agent({branch: undefined}))).not.toContain('diff');
  });
});

// A long-press is a browsing gesture. A misfire must not cost a session.
it('offers nothing destructive', () => {
  const all = [
    agent({}), agent({source: 'native'}), agent({watched: true}),
    agent({error: true, error_text: 'boom'}),
  ].flatMap(a => buildRowSheet(a, 'en', NOW).actions.map(x => x.key));
  for (const k of all) {
    expect(['jump', 'copy', 'diff', 'open']).toContain(k);
  }
});

// The same token means the same thing on every surface — that is the point of showing %N.
it('copies the command the other surfaces show', () => {
  expect(focusCommand('%60')).toBe('gtmux focus %60');
  expect(action(agent({}), 'copy').sub).toBe('gtmux focus %60');
});
