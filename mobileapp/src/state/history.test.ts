import {
  HISTORY_CAP,
  RECENT_CAP,
  SCOPE_CAP,
  emptyStore,
  historyFor,
  historyScope,
  migrate,
  pushHistory,
  removeHistory,
} from './history';

const NOW = 1_788_700_000_000;
const push = (s: ReturnType<typeof emptyStore>, scope: string, t: string, at = NOW) =>
  pushHistory(s, scope, t, at);

// The scope key is what makes the list relevant, so its fallback chain is the part
// worth pinning: a pane id would decay (tmux recycles %7 after a restart), and on a
// real fleet five of eighteen panes — including HQ— sit outside any repo.
describe('historyScope', () => {
  it('uses the repo when the pane is in one', () => {
    expect(historyScope({project: 'gtmux', loc: 'gtmux dev:0.0'})).toBe('gtmux');
  });

  it('falls back to the tmux session name outside a repo', () => {
    expect(historyScope({loc: 'HQ:0.0'})).toBe('HQ');
    expect(historyScope({project: '', loc: 'vps-audit:0.0'})).toBe('vps-audit');
    expect(historyScope({loc: '日常更新:2.0'})).toBe('日常更新');
  });

  it('gives two panes in one repo the SAME scope', () => {
    // They are the same work; what you typed in one is what you want in the other.
    const a = historyScope({project: 'hss-skills', loc: 'HSS AI Workspace:0.0'});
    const b = historyScope({project: 'hss-skills', loc: 'skill review - opencrab:0.0'});
    expect(a).toBe(b);
  });

  it('never returns empty', () => {
    expect(historyScope({})).toBe('default');
  });
});

describe('pushHistory', () => {
  it('files a send under its scope and floats a repeat to the top', () => {
    let s = push(emptyStore(), 'gtmux', 'cut release');
    s = push(s, 'gtmux', '发布');
    s = push(s, 'gtmux', 'cut release');
    expect(s.scopes.gtmux.list).toEqual(['cut release', '发布']);
  });

  it('keeps one project out of another project’s list', () => {
    let s = push(emptyStore(), 'gtmux', 'cut release');
    s = push(s, 'diting-mobile', '跑测试');
    expect(s.scopes.gtmux.list).toEqual(['cut release']);
    expect(s.scopes['diting-mobile'].list).toEqual(['跑测试']);
  });

  it('ignores empty input', () => {
    expect(push(emptyStore(), 'gtmux', '   ').scopes.gtmux).toBeUndefined();
  });

  it('caps a scope, the tail, and how many scopes are remembered', () => {
    let s = emptyStore();
    for (let i = 0; i < HISTORY_CAP + 5; i++) s = push(s, 'gtmux', `m${i}`, NOW + i);
    expect(s.scopes.gtmux.list).toHaveLength(HISTORY_CAP);
    expect(s.recent.length).toBeLessThanOrEqual(RECENT_CAP);

    let many = emptyStore();
    for (let i = 0; i < SCOPE_CAP + 4; i++) many = push(many, `p${i}`, 'x', NOW + i);
    expect(Object.keys(many.scopes)).toHaveLength(SCOPE_CAP);
    expect(many.scopes.p0).toBeUndefined(); // least recently written went
  });
});

describe('historyFor', () => {
  it('puts this scope first and tops up from the cross-scope tail', () => {
    // The complaint was never "other projects are present" — it was that the entry
    // you wanted had been pushed out by them. Yours first fixes exactly that.
    let s = push(emptyStore(), 'other', 'from elsewhere');
    s = push(s, 'gtmux', 'cut release');
    expect(historyFor(s, 'gtmux')).toEqual(['cut release', 'from elsewhere']);
  });

  it('a brand-new scope is never emptier than the old global list was', () => {
    let s = push(emptyStore(), 'other', 'a');
    s = push(s, 'other', 'b');
    expect(historyFor(s, 'a-project-never-used-before')).toEqual(['b', 'a']);
  });

  it('never shows the same text twice', () => {
    const s = push(emptyStore(), 'gtmux', 'cut release');
    expect(historyFor(s, 'gtmux')).toEqual(['cut release']);
  });
});

describe('migrate', () => {
  it('turns the old flat array into the tail, so an upgrade loses nothing', () => {
    const old = ['cut release', '继续', 'ok'];
    const s = migrate(old);
    expect(s.recent).toEqual(old);
    // And it shows up immediately, in any scope, exactly as it did before.
    expect(historyFor(s, 'gtmux')).toEqual(old);
  });

  it('reads the new shape back, and survives junk', () => {
    const s = migrate({scopes: {gtmux: {list: ['x'], at: NOW}}, recent: ['y']});
    expect(historyFor(s, 'gtmux')).toEqual(['x', 'y']);
    expect(migrate(null)).toEqual(emptyStore());
    expect(migrate('nonsense')).toEqual(emptyStore());
  });
});

describe('removeHistory', () => {
  it('removes the entry everywhere, so it cannot reappear from the tail', () => {
    // Deleting it from the scope alone would let the top-up put it straight back a
    // line lower, which reads as a delete that did not work.
    let s = push(emptyStore(), 'gtmux', 'cut release');
    s = push(s, 'other', 'cut release');
    const gone = removeHistory(s, 'gtmux', 'cut release');
    expect(historyFor(gone, 'gtmux')).not.toContain('cut release');
    expect(gone.recent).not.toContain('cut release');
  });
});
