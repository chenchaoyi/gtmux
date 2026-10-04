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
    attrs: {}, offsetHeight: 42, parts: new Map(),
    setAttribute(k, v) { this.attrs[k] = v; },
    querySelector(sel) { if (!this.parts.has(sel)) this.parts.set(sel, element()); return this.parts.get(sel); },
    classList: (() => {
      const set = new Set();
      return {add: c => set.add(c), remove: c => set.delete(c), contains: c => set.has(c),
        toggle: c => (set.has(c) ? set.delete(c) : set.add(c))};
    })(),
  });
  const node = id => {
    if (!nodes.has(id)) nodes.set(id, element());
    return nodes.get(id);
  };
  const requests = [];
  const saved = new Map();
  const copied = [];
  const context = {
    document: {readyState: 'loading', getElementById: node, createElement: element, addEventListener() {}},
    navigator: {language, clipboard: {writeText: s => { copied.push(s); return Promise.resolve(); }}},
    ResizeObserver: class { constructor(cb) { context.__resized = cb; } observe(el) { context.__observed = el; } },
    setTimeout: (fn, ms) => setTimeout(fn, ms).unref(),
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
      setAgents: rows => {lastAgents = rows;},
      codexCutRow, splitCodexPinned, charCells, renderPane, WIDE_SYMBOLS, NARROW_EMOJI,
      setPin: (pane, agent, mode, prompts) => {curPane = pane; curAgent = agent; paneMode = mode; pinPrompts = prompts;},
      pin: () => ({prompts: pinPrompts, shown: pinShown, cols: paneCols}),
      written: () => globalThis.__written,
      setupPin, agePin: () => { pinFetchedAt = 0; }};
    globalThis.__written = [];
    writePane = function (t) { globalThis.__written.push(t); };
    fetchTheme = fetchShare = setupSettings = home = function () {};
${marker}`);
  vm.runInNewContext(script, context);
  context.__test.setupCodeBox();
  return {node, requests, saved, copied, context, api: context.__test};
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

// ---- Codex's pinned prompt: the browser's copy of the phone's matcher ----------------------
const repo = path.join(__dirname, '..', '..', '..');
const {cases} = JSON.parse(fs.readFileSync(path.join(repo, 'mobileapp/src/ui/codexPinnedCases.json'), 'utf8'));

test('the browser matcher gives the phone\'s answer for every shared case', () => {
  const h = harness({ok: true});
  for (const c of cases) {
    const r = h.api.splitCodexPinned(c.text, c.agent, c.prompts, c.cols || undefined);
    assert.equal(r ? r.prompt : null, c.want, c.name);
    if (c.want !== null) assert.equal(r.text, c.rest, c.name);
  }
});

test('the browser costs characters with the phone\'s tmux-measured widths', () => {
  const h = harness({ok: true});
  const ts = fs.readFileSync(path.join(repo, 'mobileapp/src/ui/term.ts'), 'utf8');
  const table = name => {
    const body = ts.match(new RegExp('export const ' + name + '[^=]*= \\[\\n([\\s\\S]*?)\\n\\];'))[1];
    return [...body.matchAll(/\[(0x[0-9a-f]+), (0x[0-9a-f]+)\]/g)].map(m => [Number(m[1]), Number(m[2])]);
  };
  assert.deepEqual(JSON.parse(JSON.stringify(h.api.WIDE_SYMBOLS)), table('WIDE_SYMBOLS'));
  assert.deepEqual(JSON.parse(JSON.stringify(h.api.NARROW_EMOJI)), table('NARROW_EMOJI'));
  for (const [ch, w] of [['a', 1], ['中', 2], ['✅', 2], ['⭐', 2], ['⚠', 1], ['🌡', 1], ['🚀', 2], ['\uFE0F', 0]]) {
    assert.equal(h.api.charCells(ch.codePointAt(0)), w, ch);
  }
});

test('the pane view moves a recognised row into the bar and leaves everything else alone', () => {
  const h = harness({ok: true, json: async () => []});
  const c = cases[0]; // the real idle capture
  h.api.setPin('%5', {agent: 'Codex'}, 'term', c.prompts);
  h.api.renderPane(c.text, c.cols);
  assert.equal(h.api.written().pop(), c.rest);
  assert.equal(h.node('pinned').hidden, false);
  assert.equal(h.node('pinned').querySelector('.pin-body').textContent, c.want);
  assert.match(h.node('pinned').querySelector('.pin-toggle').attrs['aria-label'], /^This turn's prompt: This is a throwaway/);
  assert.equal(h.node('term').style.top, (49 + 42) + 'px');

  // Claude Code with the same bytes: written as captured, no bar.
  h.api.setPin('%6', {agent: 'Claude Code'}, 'term', c.prompts);
  h.api.renderPane(c.text, c.cols);
  assert.equal(h.api.written().pop(), c.text);
  assert.equal(h.node('pinned').hidden, true);
  assert.equal(h.node('term').style.top, '49px');
});

test('an unexplained cut row asks the log, at most once per 4 s, then shows the bar', async () => {
  const c = cases[0];
  const h = harness({ok: true, json: async () => c.prompts.map(p => ({prompt: p}))});
  h.api.setPin('%5', {agent: 'Codex'}, 'term', []);
  h.api.renderPane(c.text, c.cols);
  assert.equal(h.api.written().pop(), c.text, 'unexplained: shown as captured');
  h.api.renderPane(c.text, c.cols); // a second poll before the reply
  assert.equal(h.requests.filter(r => r.url.includes('/api/transcript?id=%255')).length, 1);
  await new Promise(resolve => setImmediate(resolve));
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(h.api.pin().prompts, c.prompts);
  assert.equal(h.node('pinned').hidden, false, 'the reply explains the row');
  assert.equal(h.api.written().pop(), c.rest);
  h.api.renderPane(c.text, c.cols);
  assert.equal(h.requests.filter(r => r.url.includes('/api/transcript')).length, 1, 'explained: no more fetches');
});

test('chat mode never takes the row', () => {
  const h = harness({ok: true});
  const c = cases[0];
  h.api.setPin('%5', {agent: 'Codex'}, 'chat', c.prompts);
  h.api.renderPane(c.text, c.cols);
  assert.equal(h.api.written().pop(), c.text);
  assert.equal(h.requests.length, 0);
});

// #1290 W1: Copy sat inside a role=button bar whose keydown took Enter and Space for the
// bar, so a keyboard Copy toggled instead. Now two sibling buttons, no key handler on the bar.
test('the bar is two real buttons, and each does its own thing', () => {
  const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const open = html.match(/<div id="pinned"[^>]*>/)[0];
  assert.doesNotMatch(open, /role=|tabindex=/, 'the bar itself is not a button');
  const bar = html.slice(html.indexOf(open), html.indexOf('</div>', html.indexOf('pin-copy')));
  assert.match(bar, /<button class="pin-toggle"[^>]*aria-expanded="false"/);
  assert.match(bar, /<button class="pin-copy"/);

  const h = harness({ok: true});
  const c = cases[0];
  h.api.setupPin();
  h.api.setPin('%5', {agent: 'Codex'}, 'term', c.prompts);
  h.api.renderPane(c.text, c.cols);
  const pin = h.node('pinned');
  assert.equal(pin.keydown, undefined, 'no key handler on the bar to swallow the buttons\' keys');
  const toggle = pin.querySelector('.pin-toggle');
  const copy = pin.querySelector('.pin-copy');
  let stopped = 0;
  const ev = {stopPropagation() { stopped++; }};
  copy.click(ev);
  assert.deepEqual(h.copied, [c.want], 'Copy copies the whole prompt');
  assert.equal(pin.classList.contains('open'), false, 'and does not open the bar');
  assert.equal(stopped, 1);
  toggle.click(ev);
  assert.equal(pin.classList.contains('open'), true);
  assert.equal(toggle.attrs['aria-expanded'], 'true');
  pin.click({});
  assert.equal(pin.classList.contains('open'), false, 'a click on the text toggles too');
  assert.equal(toggle.attrs['aria-expanded'], 'false');
});

// #1290 W2: the bar rewraps when the window narrows; the terminal must move with it.
test('a taller bar moves the terminal down', () => {
  const h = harness({ok: true});
  const c = cases[0];
  h.api.setupPin();
  h.api.setPin('%5', {agent: 'Codex'}, 'term', c.prompts);
  h.api.renderPane(c.text, c.cols);
  assert.equal(h.node('term').style.top, (49 + 42) + 'px');
  assert.equal(h.context.__observed, h.node('pinned'), 'the bar is watched for size changes');
  h.node('pinned').offsetHeight = 58; // two lines after the window narrowed
  h.context.__resized();
  assert.equal(h.node('term').style.top, (49 + 58) + 'px');
});

test('the log is asked again only with the ETag it last gave', async () => {
  const c = cases[0];
  const h = harness({ok: true, headers: {get: k => (k === 'ETag' ? 'W/"t1"' : null)}, json: async () => []});
  h.api.setPin('%5', {agent: 'Codex'}, 'term', []);
  h.api.renderPane(c.text, c.cols);
  await new Promise(resolve => setImmediate(resolve));
  await new Promise(resolve => setImmediate(resolve));
  h.api.agePin();
  h.api.renderPane(c.text, c.cols);
  const asks = h.requests.filter(r => r.url.includes('/api/transcript'));
  assert.equal(asks.length, 2);
  assert.equal(asks[1].options.headers['If-None-Match'], 'W/"t1"');
});
