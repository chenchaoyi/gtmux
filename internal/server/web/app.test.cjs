// Exercise the browser's real event handler without a native/browser build. The VM
// stops before boot and replaces only unrelated post-pair view work.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'app.js'), 'utf8');

function harness(reply, language = 'en-US') {
  const nodes = new Map();
  const element = () => ({
    dataset: {}, hidden: false, disabled: false, value: '', textContent: '',
    style: {}, scrollHeight: 38, children: [],
    appendChild(child) { this.children.push(child); return child; },
    addEventListener(name, fn) { this[name] = fn; },
    select() { this.selected = true; },
    focus() { this.focused = true; },
    click() { this.clicked = true; },
  });
  const node = id => {
    if (!nodes.has(id)) nodes.set(id, element());
    return nodes.get(id);
  };
  const requests = [];
  const saved = new Map();
  const context = {
    document: {readyState: 'loading', getElementById: node, createElement: element, addEventListener() {}},
    navigator: {language},
    location: {pathname: '/p8765/', hash: ''},
    localStorage: {getItem: k => saved.get(k), setItem: (k, v) => saved.set(k, v)},
    fetch: async (url, options) => { requests.push({url, options}); return reply; },
    console,
  };
  const marker = "  if (document.readyState === 'loading')";
  assert.ok(source.includes(marker), 'web boot marker changed');
  const script = source.replace(marker, `
    globalThis.__test = {setupCodeBox, connStateFor, makeComposer};
    fetchTheme = fetchShare = setupSettings = home = function () {};
${marker}`);
  vm.runInNewContext(script, context);
  context.__test.setupCodeBox();
  return {node, requests, saved, api: context.__test};
}

async function submit(h, value) {
  h.node('gate-code-input').value = value;
  let prevented = false;
  h.node('gate-code').submit({preventDefault() { prevented = true; }});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(prevented, true);
}

test('manual code uses the Mac path prefix, enrolls, and clears the code', async () => {
  const h = harness({ok: true, json: async () => ({token: 'device-token'})});
  await submit(h, '  ABCD-1234  ');
  assert.equal(h.requests.length, 1);
  assert.equal(h.requests[0].url, '/p8765/api/enroll');
  assert.equal(h.requests[0].options.method, 'POST');
  assert.equal(JSON.parse(h.requests[0].options.body).enrollCode, 'ABCD-1234');
  assert.equal(h.saved.get('gtmux.token'), 'device-token');
  assert.equal(h.node('gate-code-input').value, '');
});

test('rejected code stays editable and shows a clear error', async () => {
  const h = harness({ok: false});
  await submit(h, 'WRONG-1234');
  assert.equal(h.node('gate-code-go').disabled, false);
  assert.equal(h.node('gate-code-why').hidden, false);
  assert.match(h.node('gate-code-why').textContent, /expired or was already used/);
  assert.equal(h.node('gate-code-input').selected, true);
  assert.equal(h.saved.has('gtmux.token'), false);
});

test('the rejected-code message also applies to the owner pairing a browser', async () => {
  const h = harness({ok: false}, 'zh-CN');
  await submit(h, 'WRONG-1234');
  assert.match(h.node('gate-code-why').textContent, /已过期或已用过/);
  assert.doesNotMatch(h.node('gate-code-why').textContent, /让对方/);
});

test('connection badge distinguishes one retry from offline', () => {
  const h = harness({ok: false});
  assert.equal(h.api.connStateFor(true), 'live');
  assert.equal(h.api.connStateFor(false), 'retry');
  assert.equal(h.api.connStateFor(false), 'off');
  assert.equal(h.api.connStateFor(true), 'live');
});

test('composer restores a refused message and shows the server reason', async () => {
  const h = harness({ok: false, status: 409, json: async () => ({error: 'pane has a draft'})});
  const c = h.api.makeComposer(() => '%21', () => {}, false);
  c.input.value = 'Please check the build';
  const send = c.el.children[0].children[3];
  send.onclick({stopPropagation() {}});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(h.requests[0].url, '/p8765/api/send');
  assert.deepEqual(JSON.parse(h.requests[0].options.body), {text: 'Please check the build', enter: true, id: '%21'});
  assert.equal(c.input.value, 'Please check the build');
  assert.equal(c.el.children[2].textContent, 'pane has a draft');
  assert.equal(c.el.children[2].hidden, false);
});
