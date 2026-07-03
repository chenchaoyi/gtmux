import {
  StatusColor,
  statusRank,
  SECTION_ORDER,
  Size,
  paletteFor,
  sections,
  counts,
} from './theme';
import {Agent, StatusName} from '../api/types';

// Helper: build a minimal Agent with the fields the theme helpers read.
const mk = (over: Partial<Agent>): Agent => ({
  pane_id: '',
  session: '',
  window: '',
  pane: '',
  loc: '',
  agent: '',
  status: 'running',
  task: '',
  latest: false,
  activity: false,
  source: 'tmux',
  ...over,
});

describe('StatusColor (authoritative design hex — regression guard)', () => {
  it('uses the exact DESIGN §1/§9 status colors', () => {
    expect(StatusColor.waiting).toBe('#EF4444'); // red
    expect(StatusColor.working).toBe('#06B6D4'); // cyan
    expect(StatusColor.idle).toBe('#22C55E'); // green
    expect(StatusColor.running).toBe('#8E8E93'); // gray
  });

  it('defines a color for every status name and only those', () => {
    expect(Object.keys(StatusColor).sort()).toEqual(
      ['idle', 'running', 'waiting', 'working'].sort(),
    );
  });
});

describe('statusRank + SECTION_ORDER (needs-you → working → idle → running)', () => {
  it('ranks waiting first, running last', () => {
    expect(statusRank.waiting).toBe(0);
    expect(statusRank.working).toBe(1);
    expect(statusRank.idle).toBe(2);
    expect(statusRank.running).toBe(3);
  });

  it('SECTION_ORDER matches the rank order', () => {
    expect(SECTION_ORDER).toEqual(['waiting', 'working', 'idle', 'running']);
    const byRank = ([...SECTION_ORDER] as StatusName[]).sort(
      (a, b) => statusRank[a] - statusRank[b],
    );
    expect(SECTION_ORDER).toEqual(byRank);
  });
});

describe('Size tokens', () => {
  it('exposes the expected layout sizes', () => {
    expect(Size.avatar).toBe(34);
    expect(Size.badge).toBe(16);
    expect(Size.radiusAvatar).toBe(9);
    expect(Size.radiusRow).toBe(12);
    expect(Size.radiusBadgeSquare).toBe(4);
    expect(Size.pad).toBe(14);
    expect(Size.gap).toBe(12);
  });
});

describe('paletteFor', () => {
  it('returns the light palette only for the exact string "light"', () => {
    const p = paletteFor('light');
    expect(p.bg).toBe('#F2F2F7');
    expect(p.surface).toBe('#FFFFFF');
    expect(p.fg).toBe('#1D1D1F');
  });

  it('returns the dark palette for "dark"', () => {
    const p = paletteFor('dark');
    expect(p.bg).toBe('#0D0D0F');
    expect(p.surface).toBe('#1C1C1F');
  });

  it('defaults to dark for null / undefined / unspecified / unknown', () => {
    const darkBg = '#0D0D0F';
    expect(paletteFor(null).bg).toBe(darkBg);
    expect(paletteFor(undefined).bg).toBe(darkBg);
    expect(paletteFor().bg).toBe(darkBg);
    expect(paletteFor('unspecified').bg).toBe(darkBg);
    expect(paletteFor('Light').bg).toBe(darkBg); // case-sensitive: not "light"
    expect(paletteFor('').bg).toBe(darkBg);
  });

  it('the two palettes are distinct (light != dark)', () => {
    expect(paletteFor('light')).not.toEqual(paletteFor('dark'));
  });

  it('every palette key is populated for both schemes', () => {
    const keys = [
      'bg',
      'surface',
      'fg',
      'fg2',
      'fg3',
      'divider',
      'divLoud',
      'rowSelected',
      'waitingTint',
    ];
    for (const scheme of ['light', 'dark'] as const) {
      const p = paletteFor(scheme) as unknown as Record<string, string>;
      for (const k of keys) {
        expect(typeof p[k]).toBe('string');
        expect(p[k].length).toBeGreaterThan(0);
      }
    }
  });
});

describe('sections', () => {
  it('groups into fixed rank order, drops empty sections', () => {
    const agents = [
      mk({status: 'idle', session: 'b'}),
      mk({status: 'waiting', session: 'a'}),
      mk({status: 'idle', session: 'a'}),
    ];
    const out = sections(agents, false);
    expect(out.map(s => s.status)).toEqual(['waiting', 'idle']); // no working/running
  });

  it('sorts a non-idle section by primary() case-insensitively', () => {
    const agents = [
      mk({status: 'working', session: 'Zebra'}),
      mk({status: 'working', session: 'apple'}),
      mk({status: 'working', session: 'Mango'}),
    ];
    const out = sections(agents, false);
    expect(out[0].agents.map(a => a.session)).toEqual(['apple', 'Mango', 'Zebra']);
  });

  it('sorts the idle (finished) section most-recently-finished first (since desc)', () => {
    const agents = [
      mk({status: 'idle', session: 'old', since: 100}),
      mk({status: 'idle', session: 'newest', since: 300}),
      mk({status: 'idle', session: 'mid', since: 200}),
    ];
    const out = sections(agents, false);
    expect(out[0].agents.map(a => a.session)).toEqual(['newest', 'mid', 'old']);
  });

  it('waitingOnly keeps only the waiting section', () => {
    const agents = [
      mk({status: 'waiting', session: 'a'}),
      mk({status: 'working', session: 'b'}),
      mk({status: 'idle', session: 'c'}),
    ];
    const out = sections(agents, true);
    expect(out.map(s => s.status)).toEqual(['waiting']);
    expect(out[0].agents).toHaveLength(1);
  });

  it('returns an empty array when there are no agents', () => {
    expect(sections([], false)).toEqual([]);
  });
});

describe('counts', () => {
  it('counts waiting/working and derives idle as the remainder', () => {
    const agents = [
      mk({status: 'waiting'}),
      mk({status: 'waiting'}),
      mk({status: 'working'}),
      mk({status: 'idle'}),
      mk({status: 'running'}), // not waiting/working → falls into idle remainder
    ];
    expect(counts(agents)).toEqual({total: 5, waiting: 2, working: 1, idle: 2});
  });

  it('is all-zero for an empty list', () => {
    expect(counts([])).toEqual({total: 0, waiting: 0, working: 0, idle: 0});
  });
});
