import {agentLabel, paneFocusCommand, plainLabel} from './PaneBrowserScreen';
import {Agent, PaneRow} from '../api/types';

const row = (over: Partial<PaneRow>): PaneRow => ({
  pane_id: '%1',
  loc: 'work:1.0',
  session: 'work',
  window: '1',
  pane: '0',
  command: 'claude',
  tier: 'agent',
  ...over,
});

const radar = (over: Partial<Agent>): Agent => ({
  pane_id: '%1',
  session: 'work',
  window: '1',
  pane: '0',
  loc: 'work:1.0',
  agent: 'claude',
  status: 'working',
  task: 't',
  latest: false,
  activity: false,
  source: 'tmux',
  ...over,
});

describe('agentLabel', () => {
  it('prefers the row’s own agent name', () => {
    expect(agentLabel(row({agent: 'claude', command: '2.1.220'}), radar({}))).toBe('claude');
  });

  it('falls back to the radar JOIN before ever showing the raw command (#659: a Claude 2.x pane’s command is its VERSION)', () => {
    expect(agentLabel(row({agent: undefined, command: '2.1.220'}), radar({agent: 'claude'}))).toBe('claude');
    expect(agentLabel(row({agent: '', command: '2.1.220'}), radar({agent: 'claude'}))).toBe('claude');
  });

  it('uses the command only as the last resort (no join available)', () => {
    expect(agentLabel(row({agent: undefined, command: 'codex'}), undefined)).toBe('codex');
  });
});

describe('plainLabel', () => {
  it('keeps a real title', () => {
    expect(plainLabel('dev server', 'npm')).toBe('dev server');
  });

  it('falls back to the command for an empty / path-like / colon-prefixed title', () => {
    expect(plainLabel(undefined, 'bash')).toBe('bash');
    expect(plainLabel('/Users/x/repo', 'vim')).toBe('vim');
    expect(plainLabel('~/repo', 'vim')).toBe('vim');
    expect(plainLabel(':/Users/x/repo', 'zsh')).toBe('zsh');
  });
});

import {browserItems, paneWindows, windowIDsLabel} from './PaneBrowserScreen';

// tmux-id-surface phase 1.1 — the window level on the phone.
const pane = (id: string, win: string, winID: string, winName = 'w'): PaneRow =>
  ({pane_id: id, loc: `S:${win}.0`, session: 'S', window: win, pane: '0',
    command: 'bash', tier: 'plain', win_id: winID, win_name: winName} as PaneRow);

describe('paneWindows', () => {
  it('groups by the STABLE id, not the index', () => {
    // The measured failure: on a real fleet most windows sit at index 0, so an
    // index-keyed grouping merges windows that have nothing to do with each other.
    const ws = paneWindows([pane('%1', '0', '@4'), pane('%2', '0', '@5'), pane('%3', '0', '@4')]);
    expect(ws.map(w => w.id)).toEqual(['@4', '@5']);
    expect(ws[0].rows.map(r => r.pane_id)).toEqual(['%1', '%3']);
  });

  it('keeps first-seen order', () => {
    const ws = paneWindows([pane('%9', '2', '@9'), pane('%1', '0', '@1')]);
    expect(ws.map(w => w.id)).toEqual(['@9', '@1']);
  });

  it('still groups when an older core sends no id', () => {
    const rows = [pane('%1', '0', ''), pane('%2', '1', '')];
    const ws = paneWindows(rows);
    expect(ws).toHaveLength(2); // falls back to the index rather than collapsing to one
    expect(ws[0].id).toBe('');
  });
});

describe('windowIDsLabel', () => {
  it('lists every id — a COLLAPSED session must still say what it holds', () => {
    expect(windowIDsLabel(paneWindows([pane('%1', '0', '@4'), pane('%2', '1', '@5')]))).toBe('@4 @5');
  });

  it('caps so a session with many windows cannot push its own name off the row', () => {
    const ws = Array.from({length: 9}, (_, i) => ({id: `@${i + 1}`, name: 'w', rows: []}));
    expect(windowIDsLabel(ws)).toBe('@1 @2 @3 @4 @5 +4');
  });

  it('says nothing when there are no ids at all', () => {
    expect(windowIDsLabel([{id: '', name: 'w', rows: []}])).toBe('');
  });
});

describe('browserItems', () => {
  it('interleaves a band before each window when there are several', () => {
    const ws = paneWindows([pane('%1', '0', '@4'), pane('%2', '1', '@5')]);
    expect(browserItems(ws, true).map(i => (i.kind === 'win' ? i.win.id : i.row.pane_id)))
      .toEqual(['@4', '%1', '@5', '%2']);
  });

  it('draws NO band for a single window — the levels coincide there', () => {
    const ws = paneWindows([pane('%1', '0', '@4'), pane('%2', '0', '@4')]);
    expect(browserItems(ws, false).map(i => i.kind)).toEqual(['pane', 'pane']);
  });
});

describe('paneFocusCommand', () => {
  // Tapping the id copies this. It must be runnable as-is — and identical to the
  // menu-bar's `PaneCommands.focus` and the web's `paneFocusCommand`, because the point
  // of surfacing `%N` at all is that the token means one thing on every surface.
  it('is a runnable command, sigil included', () => {
    expect(paneFocusCommand('%23')).toBe('gtmux focus %23');
  });
});
