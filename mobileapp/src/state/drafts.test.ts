import {DRAFT_CAP, DRAFT_TTL_MS, DraftMap, getDraft, prune, putDraft} from './drafts';

const NOW = 1_788_700_000_000;

describe('putDraft', () => {
  it('keeps a half-typed message under its own pane', () => {
    let m: DraftMap = {};
    m = putDraft(m, '%7', 'the migration should target', NOW);
    m = putDraft(m, '%11', 'ship it after the tests', NOW);
    expect(getDraft(m, '%7', NOW)).toBe('the migration should target');
    expect(getDraft(m, '%11', NOW)).toBe('ship it after the tests');
    // A pane with no draft has nothing, never someone else's. The composer types
    // into a live terminal, so a stray draft is one Enter from being sent
    // somewhere it makes no sense.
    expect(getDraft(m, '%99', NOW)).toBe('');
  });

  it('treats an emptied box as no draft, not a blank one', () => {
    let m = putDraft({}, '%7', 'half a thought', NOW);
    m = putDraft(m, '%7', '   ', NOW);
    expect(m['%7']).toBeUndefined();
  });

  it('drops the oldest once more panes hold drafts than the cap', () => {
    let m: DraftMap = {};
    for (let i = 0; i < DRAFT_CAP + 5; i++) m = putDraft(m, `%${i}`, `draft ${i}`, NOW + i);
    expect(Object.keys(m)).toHaveLength(DRAFT_CAP);
    expect(getDraft(m, '%0', NOW + 100)).toBe(''); // the oldest went
    expect(getDraft(m, `%${DRAFT_CAP + 4}`, NOW + 100)).toBe(`draft ${DRAFT_CAP + 4}`);
  });
});

describe('expiry', () => {
  // A tmux pane id is a per-server sequence number, so %7 after a restart is a
  // different pane. An old draft must not wait around to surface in it.
  it('does not offer back a draft older than the TTL', () => {
    const m = putDraft({}, '%7', 'last week', NOW);
    expect(getDraft(m, '%7', NOW + DRAFT_TTL_MS - 1)).toBe('last week');
    expect(getDraft(m, '%7', NOW + DRAFT_TTL_MS)).toBe('');
  });

  it('prunes expired entries on the next write rather than keeping them forever', () => {
    let m = putDraft({}, '%7', 'last week', NOW);
    m = putDraft(m, '%8', 'today', NOW + DRAFT_TTL_MS);
    expect(m['%7']).toBeUndefined();
    expect(m['%8']).toBeDefined();
  });

  it('survives a malformed stored entry instead of throwing', () => {
    const bad = {'%7': undefined as unknown as {text: string; at: number}, '%8': {text: 'ok', at: NOW}};
    expect(getDraft(prune(bad, NOW), '%8', NOW)).toBe('ok');
  });
});
