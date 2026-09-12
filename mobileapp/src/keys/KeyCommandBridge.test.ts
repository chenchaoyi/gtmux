import {Agent} from '../api/types';
import {orderedRows} from './KeyCommandBridge';

// ↑/↓ and ⌘N walk the rows in the order the reader sees them: the radar's sections
// (waiting, errored, working, idle, …), never the roster's arrival order — and never a
// native session, which the list does not open either.
const mk = (o: Partial<Agent>): Agent =>
  ({pane_id: '%1', session: 's', window: '0', pane: '0', loc: 's:0.0', agent: 'Claude Code',
    status: 'idle', task: '', latest: false, activity: false, source: 'tmux', ...o} as Agent);

test('orderedRows follows the sections', () => {
  const rows = orderedRows([
    mk({pane_id: '%3', status: 'idle', since: 10}),
    mk({pane_id: '%2', status: 'working'}),
    mk({pane_id: '%1', status: 'waiting'}),
    mk({pane_id: '%9', status: 'idle', source: 'native'}),
    mk({pane_id: '%6', status: 'working', role: 'supervisor'}),
  ]);
  expect(rows.map(a => a.pane_id)).toEqual(['%1', '%2', '%3']);
});
