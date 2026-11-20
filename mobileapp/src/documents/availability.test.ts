import {documentErrorMessage, parseDocument} from './references';

test('unavailable attachment explains scope and new-link recovery without asserting deletion', () => {
  expect(documentErrorMessage('unpublished', false)).toBe('This attachment is not available to this conversation. Ask the sender for a new attachment link.');
  expect(documentErrorMessage('unpublished', true)).toContain('重新提供附件链接');
  expect(documentErrorMessage('failed', false)).toContain('retry');
  expect(documentErrorMessage('forbidden', false)).toContain('cannot view');
  expect(documentErrorMessage('unsupported', false)).toContain('app version');
});
test('clearer recovery text preserves strict session and home binding', () => {
  const doc = {id: 'a'.repeat(32), sha256: 'b'.repeat(64), name: 'report.md', bytes: 100, kind: 'markdown', media_type: 'text/markdown', session_id: 'owning-turn', home_id: '', source: '/project/report.md', created_at: 100};
  expect(parseDocument(doc, 'owning-turn', '')).toEqual(doc);
  expect(parseDocument(doc, 'other-turn', '')).toBeNull();
  expect(parseDocument(doc, 'owning-turn', 'another-home')).toBeNull();
  expect(parseDocument({...doc, home_id: 'native-home'}, 'owning-turn', '')).toBeNull();
});
