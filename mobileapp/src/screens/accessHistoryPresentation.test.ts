import {accessLogLines, accessMetadata, accessOperation, accessOutcome} from './accessHistoryPresentation';
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
test('retains full identifiers and distinguishes HTTP acceptance from a failed export', () => {
  const e = {...entry, session_id: '12345678-abcd-1234-abcd-123456789abc', device_id: 'entire-device-id', source_ip: '203.0.113.8', platform: 'iOS 26', method: 'GET', duration_ms: 18};
  expect(accessOperation(e, false)).toContain(e.session_id);
  expect(accessMetadata(e, false)).toBe('iOS 26 · 203.0.113.8 · Device entire-device-id · GET · 18 ms');
  expect(accessOutcome({...e, status: 200, outcome: 'failed'}, true)).toBe('失败 · 200');
});
