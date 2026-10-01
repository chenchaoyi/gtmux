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
    get innerHTML() {
      return this.html ?? String(this.textContent).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    },
    set innerHTML(value) { this.html = value; this.children = []; },
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
    globalThis.__test = {setupCodeBox, connStateFor, makeComposer, isHQPane, paneSessionTitle, paneToAgent, renderPanes,
      setPanes: rows => {panesRows = rows;},
      setAgents: rows => {lastAgents = rows;}};
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

test('pane browser HQ identity uses role, preserves custom names and raw targets', () => {
  const h = harness({ok: true});
  const p = {pane_id: '%1', session: 'hq', loc: 'hq:0.0', tier: 'agent', command: 'codex'};
  assert.equal(h.api.isHQPane(p), false);
  h.api.setAgents([{pane_id: '%1', role: 'supervisor'}]);
  assert.equal(h.api.isHQPane(p), true);
  assert.equal(h.api.isHQPane({...p, role: 'worker'}), false);
  assert.equal(h.api.isHQPane({...p, tier: 'plain', role: 'supervisor'}), false);
  assert.equal(h.api.paneSessionTitle('HQ', true), 'Gtmux HQ');
  assert.equal(h.api.paneSessionTitle('hq', false), 'hq');
  assert.equal(h.api.paneSessionTitle('My HQ', true), 'My HQ');
  const a = h.api.paneToAgent(p);
  assert.equal(a.role, 'supervisor');
  assert.equal(a.task, 'Gtmux HQ');
  assert.equal(a.loc, 'hq:0.0');
});

test('real pane renderer marks HQ groups and rows, including a filtered sibling', () => {
  const h = harness({ok: true});
  const hq = {pane_id: '%1', session: 'hq', loc: 'hq:0.0', window: '0', tier: 'agent', agent: 'Codex', command: 'codex', role: 'supervisor'};
  const plain = {pane_id: '%2', session: 'hq', loc: 'hq:0.1', window: '0', tier: 'plain', command: 'bash'};
  h.api.setPanes([hq, plain]);
  h.api.renderPanes();
  const root = h.node('panes-list');
  const header = root.children.find(n => n.className === 'pb-session');
  assert.match(header.innerHTML, /Gtmux HQ/);
  assert.match(header.innerHTML, /pb-hq/);
  const collect = n => [n, ...(n.children || []).flatMap(collect)];
  assert.equal(collect(root).filter(n => n.className === 'pb-hq' && n.textContent === 'HQ').length, 1);
  h.node('panes-search').value = 'bash';
  root.children = [];
  h.api.renderPanes();
  assert.match(root.children.find(n => n.className === 'pb-session').innerHTML, /Gtmux HQ/);
  assert.equal(collect(root).filter(n => n.className === 'pb-hq').length, 0);
  // A role-only change must invalidate the renderer's signature.
  h.node('panes-search').value = '';
  h.api.setPanes([{...hq, role: 'worker'}, plain]);
  root.children = [];
  h.api.renderPanes();
  assert.doesNotMatch(root.children.find(n => n.className === 'pb-session').innerHTML, /pb-hq/);
});
