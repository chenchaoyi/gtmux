import {DigestRow} from '../api/client';

// The fleet-board grouping logic mirrored from HQScreen (kept in lockstep): rows
// grouped by status, needs-you→working→idle→running, supervisor excluded.
const RANK: Record<string, number> = {waiting: 0, working: 1, idle: 2, running: 3};
function fleetSections(digest: DigestRow[]) {
  const rows = digest.filter(r => r.role !== 'supervisor');
  const by: Record<string, DigestRow[]> = {};
  for (const r of rows) (by[r.status] ??= []).push(r);
  return Object.keys(by)
    .sort((a, b) => (RANK[a] ?? 9) - (RANK[b] ?? 9))
    .map(s => ({status: s, rows: by[s]}));
}

const mk = (o: Partial<DigestRow>): DigestRow => ({agent: 'Claude Code', source: 'tmux', status: 'idle', ...o});

test('fleet board: supervisor excluded, needs-you first', () => {
  const secs = fleetSections([
    mk({loc: 'a', status: 'idle'}),
    mk({loc: 'HQ', status: 'working', role: 'supervisor'}),
    mk({loc: 'b', status: 'waiting'}),
    mk({loc: 'c', status: 'working'}),
  ]);
  expect(secs.map(s => s.status)).toEqual(['waiting', 'working', 'idle']);
  expect(secs.flatMap(s => s.rows).some(r => r.role === 'supervisor')).toBe(false);
});
