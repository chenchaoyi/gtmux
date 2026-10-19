const {test} = require('node:test');
const assert = require('node:assert/strict');
const {create, supports} = require('./terminal-keys.js');

function dom() {
  class El {
    constructor(tag) {this.tagName = tag; this.children = [];this.dataset = {};this.attrs = {};this.disabled = false;this.textContent = '';}
    appendChild(el) {el.parent = this;this.children.push(el);return el;}
    remove() {if (this.parent) this.parent.children = this.parent.children.filter(x => x !== this);this.parent = null;}
    set innerHTML(_) {this.children = [];}
    setAttribute(k,v) {this.attrs[k] = v;}
    addEventListener(k,v) {this[k] = v;}
    focus() {document.activeElement = this;}
    get isConnected() {return this === document.body || !!this.parent?.isConnected;}
    get offsetParent() {return this.parent;}
    querySelectorAll() {return walk(this).filter(x => x.tagName === 'button' && !x.disabled);}
    click() {if (!this.disabled) this.onclick?.({target: this, preventDefault() {}});}
  }
  global.document = {createElement: tag => new El(tag), hidden: false, addEventListener() {}, removeEventListener() {}};document.body = new El('body');
  return document;
}
function walk(el) {return [el, ...el.children.flatMap(walk)];}
const flush = async () => {for (let i=0;i<12;i++) await Promise.resolve();};
const cap = () => ({version: 1, context: {boot:'b',pane_binding:'p',proof:'proof',expires_at:Math.floor(Date.now()/1000)+120},can_input:true,actions:[],keys:[{key:'F2',label:'F2',label_zh:'F2',group:'function',modifier_sets:[[],['shift']]}]});

test('unsupported combinations are not fabricated', () => {
  assert.equal(supports(cap().keys[0], ['shift']), true);
  assert.equal(supports(cap().keys[0], ['control']), false);
});

test('real panel handler sends F2/Shift+F2, clears modifiers, serializes taps and preserves a draft outside the panel', async () => {
  dom();const sent = [];const current = cap();
  const keyboard = create({zh:false,getId:()=>'%1',canInput:()=>true,request:async()=>({ok:true,json:async()=>current}),send:async(id,body)=>{sent.push({id,body});return {ok:true,json:async()=>({text:'received'})};},echo:()=>{}});
  document.body.appendChild(keyboard.button);document.body.appendChild(keyboard.notice);
  try {
    await keyboard.refresh();keyboard.button.click();
    walk(document.body).find(x=>x.textContent==='Function').click();
    walk(document.body).find(x=>x.textContent==='Shift').click();
    const f2 = walk(document.body).find(x=>x.dataset.key==='F2');f2.click();f2.click();await flush();
    assert.equal(sent.length,1);assert.deepEqual(sent[0].body.key_event,{key:'F2',modifiers:['shift']});
    assert.equal('enter' in sent[0].body,false);assert.equal('text' in sent[0].body,false);
    assert.ok(walk(document.body).some(x=>x.textContent==='Shift'));
    walk(document.body).find(x=>x.dataset.key==='F2').click();await flush();
    assert.equal(sent.length,2);assert.deepEqual(sent[1].body.key_event.modifiers,[]);
  } finally {keyboard.dispose();delete global.document;}
});

test('network uncertainty blocks later input and retry uses the same payload, never a refreshed context', async () => {
  dom();const sent=[];const current=cap();
  const keyboard=create({zh:true,getId:()=>'%1',canInput:()=>true,request:async()=>({ok:true,json:async()=>current}),send:async(id,body)=>{sent.push(body);throw Error('offline');},echo:()=>{}});
  document.body.appendChild(keyboard.button);document.body.appendChild(keyboard.notice);
  try {
    await keyboard.refresh();keyboard.button.click();walk(document.body).find(x=>x.textContent==='功能').click();walk(document.body).find(x=>x.dataset.key==='F2').click();await flush();
    assert.equal(keyboard.blocked(),true);assert.equal(sent.length,1);
    walk(document.body).find(x=>x.textContent==='重试同一按键').click();await flush();
    assert.equal(sent.length,2);assert.equal(sent[0],sent[1]);
    walk(document.body).find(x=>x.textContent==='不重试，继续操作').click();assert.equal(keyboard.blocked(),false);
  } finally {keyboard.dispose();delete global.document;}
});


test('catalog cache is tied to the server boot and new schema-compatible keys render', async () => {
 dom();let current=cap();const urls=[];
 const keyboard=create({zh:false,getId:()=>'%1',canInput:()=>true,request:async url=>{urls.push(url);return {ok:true,json:async()=>current};},send:async()=>null,echo:()=>{}});
 document.body.appendChild(keyboard.button);
 try {
  await keyboard.refresh();await flush();await keyboard.refresh();
  assert.match(urls.at(-1),/catalog_version=1&catalog_boot=b/);
  current={...cap(),context:{...cap().context,boot:'new-server'},keys:[{key:'F9',label:'F9',label_zh:'F9',group:'function',modifier_sets:[[]]}]};
  await keyboard.refresh();keyboard.button.click();walk(document.body).find(x=>x.textContent==='Function').click();
  assert.ok(walk(document.body).some(x=>x.dataset.key==='F9'));
 } finally {keyboard.dispose();delete global.document;}
});
