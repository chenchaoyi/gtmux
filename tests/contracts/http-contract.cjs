// Request/expectation data is shared; each server must compute its own answer.
const assert = require('node:assert/strict');
const contract = require('./http-contract.json');

async function observe(base, token, row) {
  const response = await fetch(base + row.path, {
    method: row.method,
    headers: {
      ...row.headers,
      ...(row.role === 'unauthenticated'
        ? {}
        : {
            Authorization:
              'Bearer ' +
              (row.role === 'invalid' ? 'invalid-synthetic-token' : token),
          }),
    },
    ...(row.body === undefined ? {} : { body: JSON.stringify(row.body) }),
  });
  const text = await response.text();
  return {
    status: response.status,
    headers: Object.fromEntries(
      Object.keys(row.expected.headers).map(key => [
        key,
        response.headers.get(key),
      ]),
    ),
    body: !text
      ? null
      : response.headers.get('content-type')?.includes('application/json')
      ? JSON.parse(text)
      : text,
  };
}
function verify(actual, row) {
  assert.deepEqual(
    actual,
    row.expected,
    row.name + ': differs from real Go HTTP handler',
  );
}
module.exports = { contract, observe, verify };
