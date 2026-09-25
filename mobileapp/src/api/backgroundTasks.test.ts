import {
  tally, showRow, rowText, rowState, ordered, canOpen, isRunning, statusWord, rowDetail,
  elapsed, type BackgroundTask,
} from './backgroundTasks';

const t = (id: string, status: BackgroundTask['status'], since = 0, pane = '%1'): BackgroundTask => ({
  id, goal: 'do a thing', agent: 'claude', pane, status, since,
});

describe('what counts as running', () => {
  // A task waiting on you has not finished: it has stopped and is asking. Rolling it into
  // "finished" hides the one state that needs a person, which is why the row exists.
  it('counts waiting as running, not as finished', () => {
    expect(isRunning(t('a', 'waiting'))).toBe(true);
    expect(isRunning(t('b', 'working'))).toBe(true);
    expect(isRunning(t('c', 'idle'))).toBe(false);
    expect(isRunning(t('d', 'gone'))).toBe(false);
  });

  it('tallies the three buckets', () => {
    const got = tally([t('a', 'waiting'), t('b', 'working'), t('c', 'working'), t('d', 'idle'), t('e', 'gone')]);
    expect(got).toEqual({running: 3, waiting: 1, finished: 2});
  });
});

describe('the row above the composer', () => {
  it('is not rendered when nothing is running', () => {
    expect(showRow(tally([t('a', 'idle'), t('b', 'gone')]))).toBe(false);
    expect(showRow(tally([]))).toBe(false);
    expect(showRow(tally([t('a', 'working')]))).toBe(true);
  });

  it('says what needs you first, and the rest as a remainder', () => {
    const three = tally([t('a', 'waiting'), t('b', 'working'), t('c', 'working')]);
    expect(rowText(three, true)).toEqual({lead: '1 个在等你输入', rest: ' · 另外 2 个在跑'});
    expect(rowText(three, false)).toEqual({lead: '1 needs you', rest: ' · 2 more running'});
  });

  it('says one clause when nothing is waiting, and when everything is', () => {
    expect(rowText(tally([t('a', 'working')]), true)).toEqual({lead: '1 个在跑', rest: ''});
    expect(rowText(tally([t('a', 'waiting')]), true)).toEqual({lead: '1 个在等你输入', rest: ''});
  });

  it('wears the most urgent state present, and never a colour for tappability', () => {
    expect(rowState(tally([t('a', 'waiting'), t('b', 'working')]))).toBe('waiting');
    expect(rowState(tally([t('a', 'working'), t('b', 'working')]))).toBe('working');
  });

  it('says something in both languages', () => {
    const one = tally([t('a', 'waiting')]);
    expect(rowText(one, true).lead).not.toEqual(rowText(one, false).lead);
    for (const s of ['waiting', 'working', 'idle', 'gone'] as const) {
      expect(statusWord(s, true)).not.toEqual(statusWord(s, false));
      expect(statusWord(s, true).length).toBeGreaterThan(0);
    }
  });
});

describe('the sheet', () => {
  it('puts what needs you first, then working, then done, each newest first', () => {
    const got = ordered([
      t('old-done', 'idle', 100), t('new-work', 'working', 900), t('gone', 'gone', 950),
      t('needs', 'waiting', 500), t('old-work', 'working', 200),
    ]);
    expect(got.map(x => x.id)).toEqual(['needs', 'new-work', 'old-work', 'old-done', 'gone']);
  });

  it('offers nowhere to go when the pane is gone', () => {
    expect(canOpen(t('a', 'working'))).toBe(true);
    expect(canOpen(t('b', 'gone'))).toBe(false);
    expect(canOpen({...t('c', 'working'), pane: ''})).toBe(false);
  });

  it('names the agent, the pane, the state and how long', () => {
    const age = (s: number) => `${Math.round(s / 60)}m`;
    expect(rowDetail(t('a', 'waiting', 1000, '%21'), 1720, true, age)).toBe('claude · %21 · 等你输入 12m');
    expect(rowDetail(t('a', 'working', 1000, '%21'), 1240, false, age)).toBe('claude · %21 · working 4m');
  });

  it('leaves out an age it does not have', () => {
    const age = (s: number) => `${s}s`;
    expect(rowDetail(t('a', 'idle', 0, '%9'), 5000, true, age)).toBe('claude · %9 · 干完了');
  });
});

describe('elapsed', () => {
  it('says how long, not when', () => {
    expect(elapsed(30)).toBe('<1m');
    expect(elapsed(720)).toBe('12m');
    expect(elapsed(4 * 3600)).toBe('4h');
    expect(elapsed(3 * 86400)).toBe('3d');
  });
});

// The row belongs to HQ's chat only: HQ is the one that dispatches, and a worker pane IS
// one of the running things — listing it inside its own conversation says nothing. The
// screen gates on role; this pins the rule next to the model it gates.
describe('where the row belongs', () => {
  const showsFor = (role: string | undefined, running: number) =>
    role === 'supervisor' && showRow({running, waiting: 0, finished: 0});

  it('shows in HQ chat when something runs, and nowhere else', () => {
    expect(showsFor('supervisor', 2)).toBe(true);
    expect(showsFor('supervisor', 0)).toBe(false);
    expect(showsFor(undefined, 2)).toBe(false);
    expect(showsFor('', 2)).toBe(false);
  });
});
