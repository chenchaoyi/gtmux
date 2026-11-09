const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const ts = require('typescript');
const { fixture } = require('./permission.fixture.cjs');

function registeredCase(f: any) {
  const filename = path.join(__dirname, 'run.cjs');
  const file = ts.createSourceFile(
    filename,
    fs.readFileSync(filename, 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.JS,
  );
  let callback = '';
  function visit(node: any) {
    if (
      ts.isCallExpression(node) &&
      node.expression.getText(file) === 'check' &&
      node.arguments[0]?.text === '21-camera-permission-denial'
    )
      callback = node.arguments[1].getText(file);
    ts.forEachChild(node, visit);
  }
  visit(file);
  expect(callback).not.toBe('');
  return vm.runInNewContext('(' + callback + ')', {
    launch: async () => {},
    open: async () => {},
    attach: async () => {},
    wait: f.pause,
    shown: async (e: any) => e.isDisplayed().catch(() => false),
    journey: { measure: async (_name: string, action: any) => action() },
    tap: async () => {},
    id: (value: string) => value,
    driver: f.driver,
    denyCameraPermission: (_driver: any, options: any = {}) =>
      require('./permission.cjs').denyCameraPermission(f.driver, {
        ...options,
        now: f.now,
        pause: f.pause,
      }),
    adb: (...args: string[]) => {
      assert.deepEqual(args, [
        'shell',
        'dumpsys',
        'package',
        'com.gtmux.app.qa',
      ]);
      return 'android.permission.CAMERA: granted=' + f.grant + ', flags=[]';
    },
    app: 'com.gtmux.app.qa',
    assert,
    fs: { writeFileSync: (...args: any[]) => f.writes.push(args) },
    path,
    output: '/synthetic',
    console: { log: () => {} },
  });
}

test('ACP-002 registered journey survives one ignored Deny interaction', async () => {
  const f = fixture({ ignored: 1 });
  await registeredCase(f)();
  expect(f.clicks).toBe(2);
  expect(f.declined).toBe(true);
  expect(f.writes).toHaveLength(1);
});
test('ACP-002 registered journey still fails on missing App denial feedback', async () => {
  const f = fixture({ alert: false });
  await expect(registeredCase(f)()).rejects.toThrow(
    'App denial message missing',
  );
});
test('ACP-002 registered journey cannot pass when native CAMERA was granted', async () => {
  const f = fixture({ granted: true });
  await expect(registeredCase(f)()).rejects.toThrow();
});
test('ACP-002 registered journey cannot click a different system permission owner', async () => {
  const f = fixture({ dialogPackage: 'com.unrelated.app' });
  await expect(registeredCase(f)()).rejects.toThrow();
  expect(f.clicks).toBe(0);
});
test('ACP-002 registered journey preserves stuck-dialog failure', async () => {
  const f = fixture({ ignored: Infinity });
  await expect(registeredCase(f)()).rejects.toThrow();
  expect(f.clicks).toBeLessThanOrEqual(2);
});
