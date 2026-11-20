import {parseRestoreJob, RestoreError} from './restore';
const fixture = () => ({id: '1800000000-visible-1234567890', revision: 'a'.repeat(64), created_at: 1800000000, updated_at: 1800000000, phase: 'partial', sessions: [], agents: [{loc: 'saved:0.0', pane: '%7', state: 'startup-timeout'}]});
test('RRV003 preserves optional first issue state and accepts old receipts', () => {
  const old = fixture(); expect(parseRestoreJob(old)).toEqual(old);
  const current = {...old, agents: [{...old.agents[0], issue_state: 'unknown'}]};
  expect(parseRestoreJob(current).agents?.[0].issue_state).toBe('unknown');
});
test.each([null, 3, {}, 'x'.repeat(41)])('RRV003 rejects malformed first issue state %p', issue_state => {
  const old = fixture(); expect(() => parseRestoreJob({...old, agents: [{...old.agents[0], issue_state}]})).toThrow(RestoreError);
});
