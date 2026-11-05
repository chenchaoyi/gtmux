const { test } = require('node:test');
const assert = require('node:assert/strict');
const { contract, verify } = require('../contracts/http-contract.cjs');
test('wire contract rejects type, permission, ordinal, omission and security-header drift', () => {
  const row = contract.cases.find(c => c.name === 'trace-owner-context');
  for (const mutate of [
    a => (a.status = 403),
    a => a.body.pop(),
    a => (a.body[0].turn_ordinal = '151'),
    a => delete a.body[0].record_id,
    a => (a.headers.etag = null),
  ]) {
    const actual = structuredClone(row.expected);
    mutate(actual);
    assert.throws(() => verify(actual, row));
  }
  const denied = contract.cases.find(c => c.name === 'document-forbidden');
  assert.throws(() => verify({ ...denied.expected, status: 200 }, denied));
  const download = contract.cases.find(c => c.name === 'document-download');
  const insecure = structuredClone(download.expected);
  delete insecure.headers['content-security-policy'];
  assert.throws(() => verify(insecure, download));
});
