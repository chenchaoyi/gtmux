// Exercise the browser's real event handler without a native/browser build. The VM
// stops before boot and replaces only unrelated post-pair view work.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'app.js'), 'utf8');
const formattingContext = vm.createContext({});
formattingContext.window = formattingContext;
for (const file of ['vendor/prettier-standalone.js','vendor/prettier-parser-babel.js','vendor/prettier-parser-html.js','vendor/prettier-parser-postcss.js','vendor/prettier-parser-yaml.js','code-format.js'])
  vm.runInContext(fs.readFileSync(path.join(__dirname,file),'utf8'),formattingContext);

function harness(reply, language = 'en-US', stored = {}, pathname = '/p8765/') {
  const nodes = new Map();
  const element = (tagName) => ({
    tagName,
    dataset: {}, hidden: false, disabled: false, value: '', textContent: '',
    style: {setProperty(k, v) {this[k] = v;}}, scrollHeight: 38, children: [],
    get innerHTML() {
      return this.html ?? String(this.textContent).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    },
    set innerHTML(value) { this.html = value; this.children = []; },
    appendChild(child) { child.parentElement = this; this.children.push(child); return child; },
    replaceChildren(...children) { this.children = []; this.textContent = ''; for(const child of children)this.appendChild(child); },
    closest(selector) { for (let node = this; node; node = node.parentElement) { if (selector === "[hidden]" && node.hidden) return node; } return null; },
    addEventListener(name, fn) { this[name] = fn; },
    select() { this.selected = true; },
    focus() { this.focused = true; },
    click() { this.clicked = true; },
    remove() { this.removed = true; },
    attrs: {}, offsetHeight: 42, parts: new Map(),
    setAttribute(k, v) { this.attrs[k] = v; },
    getBoundingClientRect() { return {x: 0, y: 0, width: 1200, height: 800}; },
    getAttribute(k) { return this.attrs[k]; },
    querySelectorAll() { return []; },
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
  const documentHandlers = new Map();
  const saved = new Map(Object.entries(stored));
  const copied = [];
  const context = {
    document: {documentElement: {dataset:{}}, readyState: 'loading', getElementById: node, createElement: element, createElementNS: (_, name) => element(name), createDocumentFragment: () => element("fragment"),
      createTextNode: text => Object.assign(element("text"), {textContent: text}), addEventListener: (name, fn) => documentHandlers.set(name, fn),
      querySelector: sel => node('query:' + sel), querySelectorAll: () => []},
    window: {addEventListener() {}, innerWidth: 1280, GtmuxActivity: require('./activity.js'), GtmuxPromptIdentity: require('./prompt-identity.js'),GtmuxCodeFormat:formattingContext.GtmuxCodeFormat,GtmuxCodeReading:require('./code-reading.js'),GtmuxTrace:require('./trace-model.js')},
    navigator: {language, clipboard: {writeText: s => { copied.push(s); return Promise.resolve(); }}},
    ResizeObserver: class { constructor(cb) { context.__resized = cb; } observe(el) { context.__observed = el; } },
    setTimeout: (fn, ms) => setTimeout(fn, ms).unref(),
    location: {pathname, hash: ''},
    localStorage: {getItem: k => saved.get(k), setItem: (k, v) => saved.set(k, v), removeItem: k => saved.delete(k), key: i => Array.from(saved.keys())[i], get length() {return saved.size;}},
    fetch: async (url, options) => { requests.push({url, options}); return reply; },
    console, AbortController, clearTimeout, setInterval, clearInterval,
  };
  const marker = "  if (document.readyState === 'loading')";
  assert.ok(source.includes(marker), 'web boot marker changed');
  const script = source.replace(marker, `
    var realFetchShare = fetchShare, realPollChat = pollChat;
    globalThis.__test = {uploadAttachment, attachmentFeedback, attachmentStatusEl, brandLoaderEl, labels: () => LABEL, enableChatPolling: () => {pollChat = realPollChat;}, fetchShare, enableShare: () => {fetchShare = realFetchShare;}, share: () => SHARE, pollWB, paneCanInput, copyText, clipboardAction, toolStepsEl, updateInputBar, setupFocus,
      setupKeyboard, openConversation, pollConversation, conversationControls, readableConversation, nativeSourceLabel, nativeEvidenceLabel, launchHost, setExternalSettings: s => {externalSettings=s;},
      promptObservations, pendingPromptRecorded, setCredential: value => {token = value;}, flushComposer: () => saveCompose(composeCache, true), drawChat, renderTileChat, liveTail, mdRender, labelChrome, setupCodeBox, connStateFor, makeComposer, isHQPane, paneSessionTitle, paneToAgent, renderPanes, renderRadar,
      show, gate, runRead, cancelReads, pollPane, pollChat, pollRadar, renderChat, tilePollChat, renderTree, treeRow, renderCmdk, cyclePane,
      onDraw: fn => {drawChat = fn;},
      setWorkbenchAgents: rows => {WB.agents = rows;},
      setTiles: tiles => {WB.tiles = tiles;},
      workbench: () => WB,
      focusOrigin: () => focusFromWB,
      turns: () => lastTurns,
      setPanes: rows => {panesRows = rows;},
      setPanesFailed: v => {panesFailed = v;},
      setAgents: rows => {lastAgents = rows;},
      addTile, openAgent, setMode, modes: () => ({selected: paneMode, preferred: preferredPaneMode}), codexCutRow, splitCodexPinned, charCells, renderPane, WIDE_SYMBOLS, NARROW_EMOJI,
      setPin: (pane, agent, mode, prompts) => {curPane = pane; curAgent = agent; paneMode = mode; pinPrompts = prompts;},
      pin: () => ({prompts: pinPrompts, shown: pinShown, cols: paneCols}),
      written: () => globalThis.__written,
      setupPin, agePin: () => { pinFetchedAt = 0; }, renderReply, approvalCard, pollOptions,
      setOpts: o => { lastOpts = o; },
      setPaneShown: () => { document.getElementById('pane').hidden = false; document.getElementById('chat').hidden = true; }};
    globalThis.__written = [];
    writePane = function (t) { globalThis.__written.push(t); };
    fetchTheme = fetchShare = setupSettings = home = function () {};
    setTileMode = function (tile, mode) { tile.mode = mode; };
    ensureTerm = pollPane = pollChat = showChatLoader = showPaneLoader = pollOptions = resetPin = updateInputBar = updateEmpty = wbSave = function () {};
    setInterval = function () { return 1; }; clearInterval = function () {};
${marker}`);
  vm.runInNewContext(script, context);
  context.__test.setupCodeBox();
  return {node, requests, saved, copied, context, documentHandlers, api: context.__test};
}

function transport(language = 'en-US') {
  const h = harness({ok: true}, language), requests = [];
  h.context.FormData = class { append(...args) {this.file = args;} };
  h.context.XMLHttpRequest = class {
    constructor() {this.upload = {}; this.headers = {}; requests.push(this);}
    open(method, url) {this.method = method; this.url = url;}
    setRequestHeader(k, v) {this.headers[k] = v;}
    send(body) {this.body = body;}
  };
  const context = {credential: 'synthetic-only', requests: new Set(), revoked: false};
  return {...h, requests, scope: context};
}
test('actual XHR byte stages stay on the thumbnail and wait for persistence acknowledgement', async () => {
  const h = transport(), a = {name: 'photo.png', file: {size: 100}, phase: 'queued'}, seen = [];
  assert.equal(h.api.attachmentFeedback(a).text, 'Queued');
  const promise = h.api.uploadAttachment(a, h.scope, () => seen.push(a.phase), '%10');
  const xhr = h.requests[0];
  assert.equal(xhr.url, '/p8765/api/upload?id=%2510'); assert.equal(xhr.headers.Authorization, 'Bearer synthetic-only');
  assert.deepEqual(seen, ['preparing', 'uploading']);
  xhr.upload.onprogress({lengthComputable: false, loaded: 20, total: 0});
  assert.equal(h.api.attachmentFeedback(a).text, 'Uploading');
  xhr.upload.onprogress({lengthComputable: true, loaded: 37, total: 100});
  assert.equal(h.api.attachmentFeedback(a).text, '37%');
  const overlay = h.api.attachmentStatusEl(a);
  assert.equal(overlay.className, 'cx-upload-status'); assert.equal(overlay.attrs.role, 'progressbar'); assert.equal(overlay.attrs['aria-valuenow'], '37');
  xhr.upload.onprogress({lengthComputable: true, loaded: 999, total: 1000});
  assert.equal(h.api.attachmentFeedback(a).text, '99%');
  xhr.upload.onprogress({lengthComputable: true, loaded: 100, total: 100});
  assert.equal(h.api.attachmentFeedback(a).text, 'Saving');
  assert.equal(a.phase, 'saving'); assert.equal(h.scope.requests.size, 1);
  xhr.status = 200; xhr.response = {path: '/uploads/photo.png'}; xhr.onload();
  assert.equal(await promise, '/uploads/photo.png'); assert.equal(h.api.attachmentFeedback(a).text, 'Uploaded');
  assert.equal(h.scope.requests.size, 0); assert.equal(a.file.size, 100);
});
test('unknown-length body completion still shows saving rather than fake percent', async () => {
  const h = transport('zh-CN'), a = {name: 'photo.png', file: {}};
  const promise = h.api.uploadAttachment(a, h.scope, () => {}, '%10'), xhr = h.requests[0];
  xhr.upload.onload(); assert.equal(h.api.attachmentFeedback(a).text, '保存中');
  xhr.status = 200; xhr.response = {path: '/saved'}; xhr.onload(); await promise;
});
for (const failure of ['permission', 'size', 'server', 'empty-path', 'timeout']) test(`upload ${failure} preserves the attachment and reports failure for explicit retry`, async () => {
  const h = transport(), original = {size: 12}, a = {name: 'photo.png', file: original};
  const promise = h.api.uploadAttachment(a, h.scope, () => {}, '%10'), xhr = h.requests[0];
  if (failure === 'timeout') xhr.ontimeout();
  else {xhr.status = {permission: 403, size: 413, server: 502, 'empty-path': 200}[failure]; xhr.response = {path: ''}; xhr.onload();}
  await assert.rejects(promise); assert.equal(a.phase, 'failed'); assert.equal(a.file, original); assert.equal(a.path, undefined);
  assert.equal(h.api.attachmentFeedback(a).text, 'Upload failed'); assert.equal(h.scope.requests.size, 0);
});
test('retired scope cannot accept a late completed upload or redraw recipient feedback', async () => {
  const h = transport(), a = {name: 'photo.png', file: {}}, seen = [];
  const promise = h.api.uploadAttachment(a, h.scope, () => seen.push(a.phase), '%10'), xhr = h.requests[0];
  h.scope.revoked = true; xhr.upload.onprogress({lengthComputable: true, loaded: 50, total: 100});
  xhr.status = 200; xhr.response = {path: '/saved'}; xhr.onload(); await assert.rejects(promise, /retired/);
  assert.deepEqual(seen, ['preparing', 'uploading']);
});
test('HQ remains a distinct role through every status in the narrow radar', () => {
  const h = harness({ok: true});
  for (const status of ['waiting', 'working', 'idle']) {
    h.api.renderRadar([{pane_id: '%hq', agent: 'Codex', role: 'supervisor', status, source: 'tmux'}, {pane_id: '%10', agent: 'Codex', status: 'idle', source: 'tmux'}]);
    const sections = h.node('radar').children;
    assert.equal(sections.length, 2); assert.equal(sections[0].children[0].dataset.section, 'hq');
    assert.equal(sections[0].children[1].children.length, 1); assert.equal(sections[1].children[0].dataset.section, 'idle');
    assert.equal(sections[1].children[1].children.length, 1);
  }
});
test('own compact loading retains three-pane identity and consistent built-in state words', () => {
  const h = harness({ok: true}), loader = h.api.brandLoaderEl(null);
  assert.equal(loader.children[0].className, 'brandload-mark');
  assert.equal((loader.children[0].innerHTML.match(/<i/g) || []).length, 3);
  assert.equal(loader.attrs['aria-hidden'], 'true'); assert.equal(h.api.labels().working, 'working');
});
test('generated document contract offers new-link recovery without relaxing scope checks', () => {
  const d = require('./documents.js');
  assert.match(d.documentErrorMessage('unpublished', false), /current|this conversation/);
  assert.match(d.documentErrorMessage('unpublished', false), /new attachment link/);
  assert.match(d.documentErrorMessage('failed', false), /retry/);
  assert.equal(d.parseDocument({id: 'a'.repeat(32), sha256: 'b'.repeat(64), name: 'r.md', kind: 'markdown', bytes: 1, media_type: 'text/markdown', source: '/r.md', created_at: 1, session_id: 'own', home_id: 'home'}, 'other', 'home'), null);
});
