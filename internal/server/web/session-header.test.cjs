const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, 'app.js'), 'utf8');
function boundary(chinese = false) {
  const node = () => ({dataset: {}, children: [], hidden: false, title: '', attrs: {}, value: '',
    get textContent() {return this.value + this.children.map(x => x.textContent).join('');},
    set textContent(value) {this.value = value; this.children = [];},
    appendChild(child) {this.children.push(child);}, setAttribute(key, value) {this.attrs[key] = value;}});
  const root = node(), caption = node();
  const context = {document: {createElement: node}, T: (en, zh) => chinese ? zh : en,
    curAgent: null, readableConversation: a => !!a?.can_read_conversation,
    $: id => id === 'bar' ? root : caption};
  const start = source.indexOf('  function sessionHeaderState('), end = source.indexOf('  // official icon via', start);
  assert.ok(start > 0 && end > start);
  vm.runInNewContext(source.match(/  var COLORS = [^\n]+/)[0] + '\nvar LABEL;\n' + source.match(/  LABEL = [^\n]+/)[0] + '\n' + source.slice(start, end), context);
  return {root, caption, context};
}
test('WHS-001 canonical states and explicit errors never inspect terminal prose', () => {
  const {context: c} = boundary();
  for (const state of ['waiting', 'working', 'idle', 'running']) assert.equal(c.sessionHeaderState({agent: 'Codex', status: state}, false), state);
  assert.equal(c.sessionHeaderState({agent: 'Codex', status: 'working', error: true}, false), 'errored');
  assert.equal(c.sessionHeaderState({agent: 'Codex', status: 'idle', message: 'asking for input or approval'}, false), 'idle');
  for (const status of ['', 'unverified', '__proto__', 'toString']) assert.equal(c.sessionHeaderState({agent: 'Codex', status}, false), 'unknown');
  assert.equal(c.sessionHeaderState(null, false), 'unknown');
  assert.equal(c.sessionHeaderState({status: 'working', error: true}, false), 'terminal');
  assert.equal(c.sessionHeaderState({status: 'working'}, true), 'working');
  assert.equal(c.sessionHeaderState({status: 'waiting', error: true}, true), 'errored');
  const agent = {agent: 'Codex', pane_id: '%7', status: 'idle', error: true};
  const unlisted = c.withoutRosterState(agent);
  assert.equal(c.sessionHeaderState(unlisted, false), 'unknown');
  assert.equal(unlisted.pane_id, agent.pane_id);
  assert.equal(agent.error, true);
});
test('WHS-001 unchanged polls preserve the label node and changed states retire old styling', () => {
  const {root, caption, context: c} = boundary();
  c.paintSessionHeader(root, caption, {agent: 'Codex', status: 'working'}, false);
  const glyph = caption.children[0];
  assert.equal(caption.textContent, 'working'); assert.equal(glyph.attrs['aria-hidden'], 'true');
  c.paintSessionHeader(root, caption, {agent: 'Codex', status: 'working'}, false);
  assert.equal(caption.children[0], glyph);
  c.paintSessionHeader(root, caption, {agent: 'Codex', status: 'idle', error: true}, false);
  assert.equal(root.dataset.sessionState, 'errored'); assert.equal(caption.textContent, 'errored');
  c.paintSessionHeader(root, caption, {agent: 'Codex', status: 'idle'}, false);
  assert.equal(root.dataset.sessionState, 'idle'); assert.equal(caption.textContent, 'idle');
  root.dataset.other = 'preserve'; c.paintSessionHeader(root, caption, null, false);
  assert.equal(root.dataset.sessionState, undefined); assert.equal(root.dataset.other, 'preserve');
  assert.equal(caption.hidden, true); assert.equal(caption.textContent, '');
});
test('WHS-001 focused state uses localized labels and clears on leaving the conversation', () => {
  const {root, caption, context: c} = boundary(true);
  c.curAgent = {agent: 'Codex', status: 'waiting'}; c.syncFocusSessionHeader(true);
  assert.equal(caption.textContent, '待处理'); assert.equal(caption.title, '会话状态：待处理');
  c.curAgent.status = ''; c.syncFocusSessionHeader(true);
  assert.equal(root.dataset.sessionState, 'unknown'); assert.equal(caption.textContent, '状态未知');
  c.syncFocusSessionHeader(false); assert.equal(root.dataset.sessionState, undefined); assert.equal(caption.hidden, true);
});
test('WHS-001 calibrated header inks and ordinary text meet 4.5 contrast in both themes', () => {
  const css = fs.readFileSync(path.join(__dirname, 'style.css'), 'utf8');
  function tokens(dark) {
    const values = {};
    for (const [pattern, enabled] of [[/:root\s*\{([^}]+)\}/g, true], [/:root\[data-theme="dark"\]\s*\{([^}]+)\}/g, dark]]) {
      if (!enabled) continue;
      for (const block of css.matchAll(pattern)) for (const item of block[1].matchAll(/--([\w-]+):\s*([^;]+);/g)) values[item[1]] = item[2].trim();
    }
    return values;
  }
  function luminance(hex) {return [1,3,5].map(i => parseInt(hex.slice(i, i+2),16)/255).map(v => v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((sum,v,i)=>sum+v*[.2126,.7152,.0722][i],0);}
  for (const dark of [false, true]) {
    const t = tokens(dark);
    for (const state of ['working','waiting','idle','errored','neutral']) {
      const bg = state === 'neutral' ? t.surface : t['session-'+state+'-bg'];
      for (const ink of [state === 'neutral' ? t.fg2 : t['session-'+state+'-ink'], t.fg, t.fg2]) {
        const a = luminance(bg), b = luminance(ink), ratio=(Math.max(a,b)+.05)/(Math.min(a,b)+.05);
        assert.ok(ratio >= 4.5, `${dark?'dark':'light'} ${state} ${ink}: ${ratio}`);
      }
    }
  }
});
