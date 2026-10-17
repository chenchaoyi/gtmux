import {accessGroupFilter, mergeAccessGroups} from './accessHistoryExploration';
import type {AccessGroup} from '../api/accessHistory';
const group: AccessGroup = {key: 'device:full-a', device_id: 'full-a', actor: 'phone:a', name: 'Phone', scope: 'device', count: 3, accepted: 2, refused: 0, failed: 1, first_at: '2026-10-16T10:00:00Z', last_at: '2026-10-17T10:00:00Z'};
test('merges disjoint pages by full identity, sums counters and keeps the latest name', () => {
  const older = {...group, name: 'Old name', count: 2, accepted: 1, last_at: '2026-10-16T11:00:00Z'};
  const result = mergeAccessGroups([group], [older, {...group, key: 'device:full-b', device_id: 'full-b'}], true);
  expect(result).toHaveLength(2);expect(result[0]).toMatchObject({name: 'Phone', count: 5, accepted: 3, failed: 2});
  expect(mergeAccessGroups([group], [older], false)[0].count).toBe(3);
});
test('drills into the exact ID, preserves operation/outcome and leaves summary mode', () => {
  expect(accessGroupFilter({operation: 'send', outcome: 'failed', actor: 'old', group_by: 'device'}, group)).toEqual({operation: 'send', outcome: 'failed', actor: undefined, device_id: 'full-a', group_by: undefined});
});
