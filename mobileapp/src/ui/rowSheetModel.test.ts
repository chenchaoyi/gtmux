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

// The alert this replaces showed the agent name and the task — both already on the row
// that was pressed. What the row genuinely cannot show is the whole of a clamped task
// and error, and WHICH pane this is.
describe('what the row cannot say', () => {
  // The header is the ANCHOR, not the agent's name: the avatar identifies the tool, and
  // titling the sheet "Claude Code" spends the largest type on the page on something the
  // reader already knew before they pressed.
  it('is headed by where this session is, not by what agent it runs', () => {
    const m = buildRowSheet(agent({}), 'en', NOW);
    expect(m.anchor).toContain('%60');
    expect(m.anchor).not.toContain('Claude Code');
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
    expect(m.where).toContain('not in tmux');
    const jump = m.actions.find(a => a.key === 'jump')!;
    expect(jump.disabled).toBe(true);
    // The reason must be present: an action that is simply missing leaves the reader
    // wondering whether they missed it.
    expect(jump.sub.toLowerCase()).toContain('not in tmux');
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

// A long-press is a browsing gesture, so a misfire must not cost a session. The sheet now
// carries actions that DO something (answer, carry on, stop, hand to HQ), which is the
// point of it — but the gesture only OPENS the sheet. Acting takes a second, deliberate
// tap on a card inside it, and none of these ends a session: the worst of them interrupts
// a turn, which the next "carry on" undoes. Nothing here kills, reaps or clears.
it('offers nothing that cannot be undone', () => {
  const all = [
    agent({}), agent({source: 'native'}), agent({watched: true}),
    agent({status: 'waiting'}), agent({error: true, error_text: 'boom'}),
  ].flatMap(a => buildRowSheet(a, 'en', NOW).actions.map(x => x.key));
  for (const k of all) {
    expect(['reply', 'continue', 'stop', 'ask-hq', 'diff', 'jump']).toContain(k);
  }
});

// The same token means the same thing on every surface — that is the point of showing %N.
it('names the pane the way every other surface does', () => {
  expect(focusCommand('%60')).toBe('gtmux focus %60');
});

// The first action set was ranked by what was easy to build, not by what a phone is for:
// `open` (which is what tapping the row already does), `copy the jump command` and
// `jump on the Mac` (both useless to someone holding a phone away from their desk), and
// one genuinely useful entry. The commander's word for it was 鸡肋.
describe('what the sheet offers', () => {
  const at = (a: Partial<Agent>) => buildRowSheet({pane_id: '%1', agent: 'Claude Code', status: 'idle', ...a} as Agent, 'en', 0);
  const keys = (a: Partial<Agent>) => at(a).actions.map(x => x.key);

  test('tapping the row is not an action', () => {
    // `open` was in this list. The gesture that opens the sheet is a long press ON the
    // row a plain tap already opens.
    expect(keys({})).not.toContain('open');
    expect(keys({})).not.toContain('copy');
  });

  test('a blocked session leads with answering it', () => {
    const m = at({status: 'waiting'});
    expect(m.actions[0].key).toBe('reply');
    expect(m.actions[0].group).toBe('answer');
    expect(m.blocked).toBe(true);
  });

  test('a session that is not blocked has nothing to answer', () => {
    expect(keys({status: 'working'})).not.toContain('reply');
    expect(at({status: 'working'}).blocked).toBeFalsy();
  });

  test('every session can be driven or handed to the supervisor', () => {
    for (const s of ['working', 'idle', 'waiting'] as const) {
      expect(keys({status: s})).toEqual(expect.arrayContaining(['continue', 'stop', 'ask-hq']));
    }
  });

  test('a sensed session offers none of the talking actions, and says why it cannot jump', () => {
    // Not in tmux: gtmux can see it and nothing else. Offering "carry on" would be a
    // button that cannot work.
    const m = at({source: 'native', status: 'waiting'});
    expect(m.actions.map(x => x.key)).toEqual(['jump']);
    expect(m.actions[0].disabled).toBe(true);
    expect(m.blocked).toBeFalsy();
  });

  test('the read-only actions come last', () => {
    const m = at({status: 'waiting', branch: 'feat/x'});
    const order = ['answer', 'go', 'drive', 'look'];
    const groups = m.actions.map(x => x.group);
    expect(groups).toEqual([...groups].sort((a, b) => order.indexOf(a) - order.indexOf(b)));
    expect(m.actions[m.actions.length - 1].key).toBe('diff');
  });

  // Jumping the Mac's terminal to a session is what the phone is a remote control FOR,
  // and it is the only action that means the same thing in every state. Ranked last it
  // sat under five rows on a sheet that scrolls, and read as removed.
  test('jumping to it on the Mac is never buried', () => {
    for (const s of ['waiting', 'working', 'idle'] as const) {
      const keys = at({status: s, branch: 'feat/x'}).actions.map(x => x.key);
      const at2 = keys.indexOf('jump');
      expect(at2).toBeGreaterThanOrEqual(0);
      expect(at2).toBeLessThanOrEqual(1); // first, or straight after answering a blocked one
    }
  });
});
