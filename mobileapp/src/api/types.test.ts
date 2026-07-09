import {toAgent, agentId, primary, secondary} from './types';

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
