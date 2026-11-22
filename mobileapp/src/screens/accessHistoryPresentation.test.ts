import {accessDetailFields, accessLogLines, accessOperation, accessOutcome} from './accessHistoryPresentation';
import type {AccessEntry} from '../api/accessHistory';
const entry: AccessEntry = {id: 'a', at: '2026-10-16T16:00:01.123Z', actor: 'phone:a', name: 'Phone', scope: 'device', operation: 'conversation', label_en: 'Read conversation', label_zh: '读取对话', outcome: 'ok'};
test('groups by local date across UTC midnight, formats Chinese independently of system locale', () => {
  const lines = accessLogLines([entry, {...entry, id: 'b'}, {...entry, id: 'c', at: '2026-10-16T15:59:59Z'}], true, 'Asia/Shanghai');
  expect(lines.map(l => l.clock)).toEqual(['00:00:01', '00:00:01', '23:59:59']);
  expect(lines.map(l => l.day)).toEqual(['2026年10月17日', undefined, '2026年10月16日']);
  expect(lines.map(l => l.entry.id)).toEqual(['a', 'b', 'c']);
  expect(accessLogLines([entry], false, 'UTC')[0]).toMatchObject({day: 'Oct 16, 2026', clock: '16:00:01'});
});
test('invalid times remain visible and cannot crash the page', () => {
  expect(accessLogLines([{...entry, at: 'bad-date'}], true, 'UTC')[0]).toMatchObject({day: '时间未知', clock: 'bad-date'});
});
test('keeps full labelled request details and a failed outcome with HTTP 200', () => {
  const e = {...entry, session_id: '12345678-abcd-1234-abcd-123456789abc', device_id: 'entire-device-id', source_ip: '2001:db8::1234', platform: 'iOS 26', method: 'GET', duration_ms: 18, status: 200, outcome: 'failed'};
  expect(accessOperation(e, false)).toContain(e.session_id);
  expect(accessDetailFields(e, false).map(({value}) => value)).toEqual([e.at, 'iOS 26', e.source_ip, e.device_id, 'GET', '200', '18 ms']);
  expect(accessDetailFields(e, true).find(({key}) => key === 'device')).toEqual({key: 'device', label: '设备 ID', value: e.device_id});
  expect(accessOutcome(e, true)).toBe('失败 · 200');
  expect(accessOutcome(e, false, false)).toBe('Failed');
});
