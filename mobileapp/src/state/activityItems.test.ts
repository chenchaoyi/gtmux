import {buildActivityItems} from './activityItems';
import {Agent} from '../api/types';

const agent = (over: Partial<Agent>): Agent =>
  ({
    pane_id: '%1',
    session: 'sess',
    window: '',
    pane: '',
    loc: 'sess:1.0',
    agent: 'Claude Code',
    status: 'working',
    task: '',
    latest: false,
    activity: false,
    source: 'tmux',
    ...over,
  }) as Agent;

describe('buildActivityItems', () => {
  it('lists waiting first, then working, most-recent first, capped at 3 + more', () => {
    const agents = [
      agent({pane_id: '%1', status: 'waiting', task: 'fix bug', since: 940}),
      agent({pane_id: '%2', status: 'waiting', task: 'review', since: 880}),
      agent({pane_id: '%3', status: 'working', task: 'tests', since: 760}),
      agent({pane_id: '%4', status: 'working', task: 'build', since: 700}),
      agent({pane_id: '%5', status: 'idle', task: 'done', since: 500}),
    ];
    const {items, more} = buildActivityItems(agents);
    expect(items.map(i => i.title)).toEqual(['fix bug', 'review', 'tests']);
    // since is passed verbatim — the widget renders the relative time locally.
    expect(items[0]).toEqual({title: 'fix bug', status: 'waiting', since: 940});
    expect(items[2]).toEqual({title: 'tests', status: 'working', since: 760});
    expect(more).toBe(1); // 4 active − 3 shown (idle excluded)
  });

  it('falls back to session when task is empty', () => {
    const {items} = buildActivityItems([agent({status: 'waiting', task: '', session: 'mysess', since: 900})]);
    expect(items[0].title).toBe('mysess');
  });

  it('returns empty when nothing is in flight', () => {
    const {items, more} = buildActivityItems([agent({status: 'idle'})]);
    expect(items).toEqual([]);
    expect(more).toBe(0);
  });
});
