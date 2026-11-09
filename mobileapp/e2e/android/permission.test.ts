const { denyCameraPermission } = require('./permission.cjs');
const { fixture } = require('./permission.fixture.cjs');
const options = (f: any) => ({
  now: f.now,
  pause: f.pause,
  onAttempt: (n: number) => f.attempts.push(n),
});

test('ACP-001 enabled camera dialog uses one native element Deny click', async () => {
  const f = fixture();
  expect(await denyCameraPermission(f.driver, options(f))).toEqual({
    attempts: 1,
    elapsed_ms: 100,
  });
  expect(f.clicks).toBe(1);
  expect(f.attempts).toEqual([1]);
});
test('ACP-001 one ignored interaction uses one fresh bounded fallback', async () => {
  const f = fixture({ ignored: 1 });
  const r = await denyCameraPermission(f.driver, options(f));
  expect(r.attempts).toBe(2);
  expect(f.attempts).toEqual([1, 2]);
  expect(f.now()).toBeLessThan(10000);
});
test('ACP-001 unchanged dialog fails at the original deadline without a third click', async () => {
  const f = fixture({ ignored: Infinity });
  await expect(denyCameraPermission(f.driver, options(f))).rejects.toThrow(
    'within 10000ms',
  );
  expect(f.clicks).toBe(2);
  expect(f.now()).toBe(10000);
});
test.each([
  { dialogPackage: 'com.unrelated.app' },
  { message: 'Allow gtmux to access your location?' },
])(
  'ACP-001 another permission owner or purpose is never clicked %j',
  async state => {
    const f = fixture(state);
    await expect(denyCameraPermission(f.driver, options(f))).rejects.toThrow(
      'unexpected',
    );
    expect(f.clicks).toBe(0);
  },
);
test('ACP-001 original driver failure is propagated without a fresh action', async () => {
  const original = new Error('socket hang up');
  const f = fixture({ error: original });
  await expect(denyCameraPermission(f.driver, options(f))).rejects.toBe(
    original,
  );
  expect(f.attempts).toEqual([1]);
});
test('ACP-001 late readiness waits rather than hitting a disabled button', async () => {
  const f = fixture();
  f.button.isEnabled = async () => f.now() >= 500;
  const r = await denyCameraPermission(f.driver, options(f));
  expect(r.attempts).toBe(1);
  expect(f.now()).toBe(600);
});
test('ACP-001 a slow read cannot trigger a click after the deadline', async () => {
  const f = fixture();
  f.driver.getCurrentPackage = async () => {
    await f.pause(10001);
    return 'com.google.android.permissioncontroller';
  };
  await expect(denyCameraPermission(f.driver, options(f))).rejects.toThrow(
    'within 10000ms',
  );
  expect(f.clicks).toBe(0);
});
test('ACP-001 absent dialog is not confused with a successful denial', async () => {
  const f = fixture();
  f.button.isDisplayed = async () => false;
  await expect(denyCameraPermission(f.driver, options(f))).rejects.toThrow(
    'within 10000ms',
  );
  expect(f.clicks).toBe(0);
});

test('ACP-001 closing before any Deny interaction cannot pass', async () => {
  const f = fixture();
  let reads = 0;
  f.button.isDisplayed = async () => ++reads === 1;
  f.button.isEnabled = async () => false;
  await expect(denyCameraPermission(f.driver, options(f))).rejects.toThrow(
    'before any Deny',
  );
  expect(f.clicks).toBe(0);
});

test('ACP-001 a newly repeated camera prompt is not mistaken for a lost click', async () => {
  const f = fixture({ ignored: Infinity });
  const click = f.button.click;
  f.button.click = async () => {
    await click();
    f.button.elementId = 'new-camera-dialog';
  };
  await expect(denyCameraPermission(f.driver, options(f))).rejects.toThrow(
    'dialog changed',
  );
  expect(f.clicks).toBe(1);
});
