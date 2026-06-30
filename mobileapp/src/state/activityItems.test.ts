import {buildActivityItems, relTime} from './activityItems';
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

describe('relTime', () => {
  it('formats compactly', () => {
    expect(relTime(1000, 0)).toBe('');
    expect(relTime(1000, undefined)).toBe('');
    expect(relTime(1000, 1000)).toBe('now');
    expect(relTime(1000, 970)).toBe('now'); // 30s
    expect(relTime(1000, 880)).toBe('2m'); // 120s
    expect(relTime(10000, 6400)).toBe('1h');
    expect(relTime(200000, 27200)).toBe('2d');
  });
});

describe('buildActivityItems', () => {
  it('lists waiting first, then working, most-recent first, capped at 3 + more', () => {
    const agents = [
      agent({pane_id: '%1', status: 'waiting', task: 'fix bug', since: 940}), // 1m
      agent({pane_id: '%2', status: 'waiting', task: 'review', since: 880}), // 2m
      agent({pane_id: '%3', status: 'working', task: 'tests', since: 760}), // 4m
      agent({pane_id: '%4', status: 'working', task: 'build', since: 700}), // 5m
      agent({pane_id: '%5', status: 'idle', task: 'done', since: 500}),
    ];
    const {items, more} = buildActivityItems(agents, 1000);
    expect(items.map(i => i.title)).toEqual(['fix bug', 'review', 'tests']);
    expect(items[0]).toEqual({title: 'fix bug', status: 'waiting', time: '1m'});
    expect(items[2]).toEqual({title: 'tests', status: 'working', time: '4m'});
    expect(more).toBe(1); // 4 active − 3 shown (idle excluded)
  });

  it('falls back to session when task is empty', () => {
    const {items} = buildActivityItems([agent({status: 'waiting', task: '', session: 'mysess', since: 900})], 1000);
    expect(items[0].title).toBe('mysess');
  });

  it('returns empty when nothing is in flight', () => {
    const {items, more} = buildActivityItems([agent({status: 'idle'})], 1000);
    expect(items).toEqual([]);
    expect(more).toBe(0);
  });
});
