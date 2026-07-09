// Sample radar data for the DEMO mode — lets a reviewer (and a curious first-run
// user) see what gtmux does WITHOUT a Mac running `gtmux serve`. It's only ever shown
// behind an explicit "See a demo" tap on the pairing screen, clearly labelled as
// sample data, and never reaches a paired user. Covers every section so the status
// language + ordering (needs-you → working → idle → running → Elsewhere) is on display.

import {Agent} from '../api/types';

// Built relative to "now" so the "5m / 1h ago" labels stay realistic over time.
export function sampleAgents(): Agent[] {
  const now = Math.floor(Date.now() / 1000);
  const base = {window: '0', pane: '0', loc: '', activity: false, latest: false};
  const rows: Agent[] = [
    {...base, pane_id: '%7', session: 'api', loc: 'api:0.0', agent: 'Claude Code',
      status: 'waiting', task: 'permission to run tests', source: 'tmux', since: now - 240},
    {...base, pane_id: '%11', session: 'web', loc: 'web:0.0', agent: 'Claude Code',
      status: 'working', task: 'refactor auth middleware', source: 'tmux', since: now - 95},
    {...base, pane_id: '%8', session: 'worker', loc: 'worker:0.0', agent: 'Codex',
      status: 'idle', task: 'add retry backoff', source: 'tmux', latest: true, since: now - 720},
    {...base, pane_id: '%3', session: 'docs', loc: 'docs:0.0', agent: 'Gemini',
      status: 'idle', task: 'draft the API reference', source: 'tmux', since: now - 5400},
    {...base, pane_id: '%9', session: 'app', loc: 'app:0.0', agent: 'Claude Code',
      status: 'idle', task: 'wire up the dashboard', source: 'tmux', since: now - 130,
      bg: true, bg_count: 1, bg_text: 'npm run dev'},
    {...base, pane_id: '%5', session: 'infra', loc: 'infra:0.0', agent: 'Claude Code',
      status: 'running', task: '', source: 'tmux', since: now - 40},
    {...base, pane_id: '', session: '', agent: 'Codex', status: 'idle', task: '',
      source: 'native', project: 'scratch', terminal: 'Ghostty'},
  ];
  return rows;
}
