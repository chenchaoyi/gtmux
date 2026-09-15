import {foldRuns, placeActs} from './consoleActs';
import {Act} from '../screens/hqActsModel';

const act = (ts: number, verb: string, extra: Partial<Act> = {}): Act => ({ts, kind: 'k', verb, target: '', detail: `d${ts}`, ...extra});
const T0 = 1_789_400_000;
const turn = (secs: number) => ({prompt: 'p', response: 'r', time: new Date(secs * 1000).toISOString()});

// The acts sit in the conversation at the moment they happened (hq-work A).
describe('placeActs', () => {
  it('puts each act above the first turn that came after it, the rest after the last', () => {
    const turns = [turn(T0), turn(T0 + 600), turn(T0 + 1200)];
    const acts = [act(T0 + 100, '派活'), act(T0 + 700, '记账'), act(T0 + 2000, '回收'), act(T0 - 50, '旧的')];
    const {before, after} = placeActs(turns, acts);
    expect(before[0]).toEqual([]);
    expect(before[1].map(r => (r.kind === 'act' ? r.act.verb : r.verb))).toEqual(['派活']);
    expect(before[2].map(r => (r.kind === 'act' ? r.act.verb : r.verb))).toEqual(['记账']);
    expect(after.map(r => (r.kind === 'act' ? r.act.verb : r.verb))).toEqual(['回收']);
  });
  it('gives a clockless turn no acts, and keeps them for the tail', () => {
    const {before, after} = placeActs([{prompt: 'p', response: 'r'}], [act(T0, '派活')]);
    expect(before[0]).toEqual([]);
    expect(after).toHaveLength(1);
  });
  it('floors the acts at the session start, and at a dozen when nothing is known', () => {
    const acts = Array.from({length: 30}, (_, i) => act(T0 + i * 100, '记账'));
    // A session that began at T0+2000 shows only what happened since.
    const {after} = placeActs([{prompt: 'p', response: 'r'}], acts, T0 + 2000);
    expect(after.flatMap(r => (r.kind === 'fold' ? r.acts : [r.act]))).toHaveLength(10);
    // No clock anywhere: the newest twelve, not the week.
    const none = placeActs([{prompt: 'p', response: 'r'}], acts);
    expect(none.after.flatMap(r => (r.kind === 'fold' ? r.acts : [r.act]))).toHaveLength(12);
  });
});

describe('foldRuns', () => {
  it('folds three or more same-verb acts within an hour, leaves two alone', () => {
    const rows = foldRuns([act(T0, '记账'), act(T0 + 60, '记账'), act(T0 + 120, '记账'), act(T0 + 200, '派活'), act(T0 + 260, '记账'), act(T0 + 300, '记账')]);
    expect(rows.map(r => (r.kind === 'fold' ? `fold ${r.verb}×${r.n}` : r.act.verb))).toEqual(['fold 记账×3', '派活', '记账', '记账']);
  });
  it('never folds across an hour or over an alarm', () => {
    const rows = foldRuns([act(T0, '记账'), act(T0 + 3700, '记账'), act(T0 + 3800, '记账'), act(T0 + 3900, '记账', {alarm: true})]);
    expect(rows.every(r => r.kind === 'act')).toBe(true);
  });
});
