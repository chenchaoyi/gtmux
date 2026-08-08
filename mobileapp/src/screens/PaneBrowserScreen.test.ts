import {agentLabel, plainLabel} from './PaneBrowserScreen';
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
