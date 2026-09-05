import {toAgent, agentId, primary, secondary, serverModeNeedsAttention, paneRowToAgent, paneLabel, PaneRow} from './types';

describe('toAgent', () => {
  it('decodes a fully populated agent', () => {
    const raw = {
      pane_id: '%1',
      session: 'work',
      window: 'win',
      pane: '0',
      loc: 'work:1.0',
      agent: 'claude',
      status: 'waiting',
      task: 'fixing tests',
      latest: true,
      activity: true,
      source: 'tmux',
      project: 'gtmux',
      terminal: 'ghostty',
      tab: '2',
      activity_at: 1700000000,
      since: 1699999999,
      icon: '/Applications/Claude.app',
    };
    expect(toAgent(raw)).toEqual(raw);
  });

  it('decodes the errored-idle modifier', () => {
    const a = toAgent({pane_id: '%1', status: 'idle', error: true, error_text: 'Internal server error'});
    expect(a.error).toBe(true);
    expect(a.error_text).toBe('Internal server error');
    // absent → undefined (not surfaced), status unchanged
    const b = toAgent({pane_id: '%2', status: 'idle'});
    expect(b.error).toBeUndefined();
    expect(b.error_text).toBeUndefined();
  });

  it('decodes the background-running modifier', () => {
    const a = toAgent({pane_id: '%1', status: 'idle', bg: true, bg_count: 2, bg_text: 'npm run dev'});
    expect(a.bg).toBe(true);
    expect(a.bg_count).toBe(2);
    expect(a.bg_text).toBe('npm run dev');
    // absent → undefined (a bg-unaware row is unaffected)
    const b = toAgent({pane_id: '%2', status: 'idle'});
    expect(b.bg).toBeUndefined();
    expect(b.bg_count).toBeUndefined();
    expect(b.bg_text).toBeUndefined();
  });

  it('decodes the watched flag (a pinned plain pane → its own section, not RUNNING)', () => {
    expect(toAgent({pane_id: '%1', watched: true}).watched).toBe(true);
    // absent/false → undefined, so the section/count filters treat it as a normal agent
    expect(toAgent({pane_id: '%2'}).watched).toBeUndefined();
    expect(toAgent({pane_id: '%3', watched: false}).watched).toBeUndefined();
  });

  it('applies defaults for an empty object', () => {
    expect(toAgent({})).toEqual({
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
      project: undefined,
      terminal: undefined,
      tab: undefined,
      activity_at: undefined,
      since: undefined,
      icon: undefined,
    });
  });

  it('defaults status to "running" when absent', () => {
    expect(toAgent({}).status).toBe('running');
  });

  it('defaults source to "tmux" when absent or empty', () => {
    expect(toAgent({}).source).toBe('tmux');
    expect(toAgent({source: ''}).source).toBe('tmux');
    expect(toAgent({source: 'native'}).source).toBe('native');
  });

  it('coerces non-string string fields to empty string', () => {
    const a = toAgent({pane_id: 42, session: null, loc: {}, agent: undefined});
    expect(a.pane_id).toBe('');
    expect(a.session).toBe('');
    expect(a.loc).toBe('');
    expect(a.agent).toBe('');
  });

  it('treats only the literal `true` as a boolean flag', () => {
    expect(toAgent({latest: true, activity: true}).latest).toBe(true);
    expect(toAgent({latest: 'true', activity: 1}).latest).toBe(false);
    expect(toAgent({latest: 1}).latest).toBe(false);
    expect(toAgent({activity: 'yes'}).activity).toBe(false);
  });

  it('keeps numeric fields only when numbers, else undefined', () => {
    expect(toAgent({activity_at: 123, since: 456}).activity_at).toBe(123);
    expect(toAgent({activity_at: 123}).since).toBeUndefined();
    expect(toAgent({activity_at: '123'}).activity_at).toBeUndefined();
    expect(toAgent({activity_at: 0}).activity_at).toBe(0);
  });

  it('maps empty optional strings to undefined, not ""', () => {
    const a = toAgent({project: '', terminal: '', tab: '', icon: ''});
    expect(a.project).toBeUndefined();
    expect(a.terminal).toBeUndefined();
    expect(a.tab).toBeUndefined();
    expect(a.icon).toBeUndefined();
  });

  it('preserves an unknown status string verbatim (cast)', () => {
    // The decoder casts whatever string is present; only falsy -> "running".
    expect(toAgent({status: 'bogus'}).status).toBe('bogus' as any);
  });

  it('tolerates null/undefined raw input without throwing', () => {
    expect(() => toAgent(null)).not.toThrow();
    expect(() => toAgent(undefined)).not.toThrow();
    expect(toAgent(null).source).toBe('tmux');
    expect(toAgent(undefined).status).toBe('running');
  });

  it('ignores unknown extra fields', () => {
    const a = toAgent({pane_id: '%9', bogus: 'x', extra: 1});
    expect(a.pane_id).toBe('%9');
    expect((a as any).bogus).toBeUndefined();
  });
});

describe('agentId', () => {
  it('uses pane_id when present', () => {
    const a = toAgent({pane_id: '%7', source: 'native', agent: 'x'});
    expect(agentId(a)).toBe('%7');
  });

  it('falls back to a composite identity when pane_id is empty', () => {
    const a = toAgent({
      source: 'native',
      terminal: 'ghostty',
      tab: '3',
      project: 'proj',
      agent: 'codex',
    });
    expect(agentId(a)).toBe('native:ghostty:3:proj:codex');
  });

  it('uses literal undefined in the composite when optionals are absent', () => {
    const a = toAgent({source: 'native', agent: 'codex'});
    // terminal/tab/project default to undefined -> appear as "undefined".
    expect(agentId(a)).toBe('native:undefined:undefined:undefined:codex');
  });
});

describe('primary', () => {
  it('prefers the task when present', () => {
    const a = toAgent({task: 'do the thing', session: 's', loc: 'l'});
    expect(primary(a)).toBe('do the thing');
  });

  it('for a tmux agent (no task) uses session, then loc', () => {
    expect(primary(toAgent({session: 'sess', loc: 'work:1'}))).toBe('sess');
    expect(primary(toAgent({loc: 'work:1'}))).toBe('work:1');
  });

  it('returns "" for a bare tmux agent with no task/session/loc', () => {
    expect(primary(toAgent({}))).toBe('');
  });

  it('for a native agent (no task) uses project, then terminal', () => {
    expect(primary(toAgent({source: 'native', project: 'p', terminal: 't'}))).toBe('p');
    expect(primary(toAgent({source: 'native', terminal: 't'}))).toBe('t');
    expect(primary(toAgent({source: 'native'}))).toBe('');
  });

  it('task wins even for a native agent', () => {
    const a = toAgent({source: 'native', task: 'job', project: 'p'});
    expect(primary(a)).toBe('job');
  });
});

describe('secondary', () => {
  it('for a native agent returns the terminal (or "")', () => {
    expect(secondary(toAgent({source: 'native', terminal: 'iterm'}))).toBe('iterm');
    expect(secondary(toAgent({source: 'native'}))).toBe('');
  });

  it('for a tmux agent joins session and pane_id with " · "', () => {
    const a = toAgent({session: 'sess', pane_id: '%4'});
    expect(secondary(a)).toBe('sess · %4');
  });

  it('falls back to loc when session is empty', () => {
    const a = toAgent({loc: 'work:1.0', pane_id: '%2'});
    expect(secondary(a)).toBe('work:1.0 · %2');
  });

  it('omits the pane suffix when pane_id is empty', () => {
    expect(secondary(toAgent({session: 'sess'}))).toBe('sess');
    expect(secondary(toAgent({loc: 'work:1'}))).toBe('work:1');
  });

  it('returns "" for an empty tmux agent', () => {
    expect(secondary(toAgent({}))).toBe('');
  });
});

describe('server mode', () => {
  const base = {
    state: 'on' as const,
    power: 'ac' as const,
    guard: {installed: true, healthy: true},
    system_disablesleep: true,
    owned_by_gtmux: true,
    platform: {ok: true, verified: true},
  };

  // Red is reserved for "a human should look at this". A healthy session running on
  // mains power must stay quiet, or the indicator becomes noise and gets ignored —
  // which defeats the one job it has.
  it('stays quiet while healthy', () => {
    expect(serverModeNeedsAttention(base)).toBe(false);
  });

  it('reddens when the safety guard is missing', () => {
    // Nothing would restore sleep if gtmux went away — the user must know.
    expect(
      serverModeNeedsAttention({...base, guard: {installed: false, healthy: false}}),
    ).toBe(true);
  });

  it('reddens on a low battery, not merely on battery', () => {
    expect(serverModeNeedsAttention({...base, power: 'battery', battery_pct: 80})).toBe(false);
    expect(serverModeNeedsAttention({...base, power: 'battery', battery_pct: 25})).toBe(true);
  });

  it('reddens when the setting lapsed', () => {
    expect(
      serverModeNeedsAttention({...base, state: 'lapsed', system_disablesleep: false}),
    ).toBe(true);
  });

  // Go omits zero-valued fields, so the optional ones must decode absent.
  it('tolerates the fields Go omits', () => {
    const minimal = {
      state: 'off' as const,
      power: 'ac' as const,
      guard: {installed: false, healthy: false},
      system_disablesleep: false,
      owned_by_gtmux: false,
      platform: {ok: true, verified: true},
    };
    expect(serverModeNeedsAttention(minimal)).toBe(false);
  });
});

// A plain pane's git identity reaches the app, which is what lets a surface gate on it:
// the Diff control is offered only when the pane's cwd IS a repo, since /api/diff returns
// nothing outside one.
describe('paneRowToAgent git identity', () => {
  const row = (extra: object) =>
    ({pane_id: '%1', loc: 's:0.0', session: 's', window: '0', pane: '0', command: 'bash', tier: 'plain' as const, ...extra});

  it('carries branch from a plain pane in a repo', () => {
    expect(paneRowToAgent(row({cwd: '/w/repo', branch: 'main', project: 'repo'})).branch).toBe('main');
  });

  it('leaves branch undefined outside a repo', () => {
    expect(paneRowToAgent(row({cwd: '/tmp'})).branch).toBeUndefined();
  });

  // `project` on a browser row stays the CWD (what the row displays), NOT the repo name
  // the radar puts in the same field — the two surfaces mean different things by it.
  it('keeps project as the cwd for display', () => {
    expect(paneRowToAgent(row({cwd: '/w/repo', project: 'repo'})).project).toBe('/w/repo');
  });
});

// Three sibling shells all read "bash" in the neighbour strip. That is true and it
// identifies nothing — the reader is choosing between panes, and telling them apart is
// the label's whole job.
describe('paneLabel', () => {
  const row = (o: Partial<PaneRow>): PaneRow =>
    ({pane_id: '%1', loc: 's:0.0', session: 's', window: '0', pane: '0', command: 'bash', tier: 'plain', active: false, ...o} as PaneRow);

  test('a title someone chose wins', () => {
    expect(paneLabel(row({title: 'log tail'}))).toBe('log tail');
  });

  test('a title that is a whole path is not a name either', () => {
    // Many shells write the cwd into the title, sometimes colon-prefixed. The folder's
    // own name says more, and the later rules produce it.
    expect(paneLabel(row({title: ':/Users/x/src', cwd: '/Users/x/src'}))).toBe('src');
    expect(paneLabel(row({title: '~/src', project: 'gtmux'}))).toBe('gtmux');
  });

  test('a title that is just the command is not a name', () => {
    // tmux and shells both write the command into the title.
    expect(paneLabel(row({title: 'bash', cwd: '/Users/x/gtmux'}))).toBe('gtmux');
  });

  test('a window name counts, unless tmux auto-renamed it to the command', () => {
    expect(paneLabel(row({win_name: 'deploy'}))).toBe('deploy');
    expect(paneLabel(row({win_name: 'bash', project: 'gtmux'}))).toBe('gtmux');
  });

  test('a shell in a repo is named by the repo, whatever subdirectory it sits in', () => {
    expect(paneLabel(row({project: 'gtmux', cwd: '/Users/x/gtmux/mobileapp'}))).toBe('gtmux');
  });

  test('outside a repo it is the folder', () => {
    expect(paneLabel(row({cwd: '/private/tmp'}))).toBe('tmp');
    expect(paneLabel(row({cwd: '/Users/x/notes/'}))).toBe('notes');
  });

  test('with nothing else it is still the command, which is at least true', () => {
    expect(paneLabel(row({}))).toBe('bash');
    expect(paneLabel(row({command: ''}))).toBe('%1');
  });
});
