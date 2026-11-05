const { test } = require('node:test');
const assert = require('node:assert/strict');
const { browserPool } = require('./browser-pool.cjs');
test('concurrent borrowers share a process, disconnect gets a replacement and close releases it', async () => {
  const instances = [];
  const pool = browserPool(async () => {
    const b = {
      connected: true,
      isConnected() {
        return this.connected;
      },
      async close() {
        this.closed = true;
        this.connected = false;
      },
    };
    instances.push(b);
    return b;
  });
  const [a, b] = await Promise.all([pool.get(), pool.get()]);
  assert.equal(a, b);
  assert.equal(pool.launches, 1);
  a.connected = false;
  const c = await pool.get();
  assert.notEqual(c, a);
  assert.equal(pool.launches, 2);
  await pool.close();
  assert.equal(c.closed, true);
});
test('failed launch propagates without hidden retry and does not poison the next case', async () => {
  let attempts = 0;
  const b = { isConnected: () => true, close: async () => {} };
  const pool = browserPool(async () => {
    if (++attempts === 1) throw Error('synthetic launch failure');
    return b;
  });
  await assert.rejects(pool.get(), /synthetic launch failure/);
  assert.equal(attempts, 1);
  assert.equal(await pool.get(), b);
  assert.equal(attempts, 2);
  await pool.close();
});
