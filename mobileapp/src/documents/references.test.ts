import {documentReference, parseDocument} from './references';

test('local report destinations remain data instead of launchable OS URLs', () => {
  expect(documentReference('/private/tmp/report.html')).toEqual({reference: '/private/tmp/report.html', name: 'report.html', path: '/private/tmp/report.html'});
  expect(documentReference('file:///tmp/my%20report.html')?.reference).toBe('/tmp/my report.html');
  expect(documentReference('./report.md')?.name).toBe('report.md');
  expect(documentReference('reports/review.md')?.reference).toBe('reports/review.md');
  expect(documentReference('report.md')?.name).toBe('report.md');
  expect(documentReference('gtmux-file:' + 'a'.repeat(32))?.reference).toBe('gtmux-file:' + 'a'.repeat(32));
  for (const value of ['javascript:alert(1)', 'data:text/html,hello', '//host/file', 'file://other/tmp/a', 'file:///tmp/a%00.txt', '/tmp/a\n.txt']) expect(documentReference(value)).toBeNull();
});

test('download metadata must match conversation identity and a bounded content hash', () => {
  const value = {id: 'a'.repeat(32), sha256: 'b'.repeat(64), name: 'report.html', kind: 'html', media_type: 'text/html', bytes: 100, created_at: 100, session_id: 'session', source: '/tmp/report.html'};
  expect(parseDocument(value, 'session')).toEqual(value);
  expect(parseDocument(value, 'another')).toBeNull();
  expect(parseDocument({...value, bytes: 31 * 1024 * 1024}, 'session')).toBeNull();
  expect(parseDocument({...value, sha256: 'invalid'}, 'session')).toBeNull();
  expect(parseDocument({...value, home_id: 'other'}, 'session')).toBeNull();
});
