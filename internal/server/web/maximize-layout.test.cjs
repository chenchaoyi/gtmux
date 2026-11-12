const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const source=fs.readFileSync(process.env.GTMUX_MAXIMIZE_SOURCE||path.join(__dirname,'app.js'),'utf8');
function fixture(){
 class Element{
  constructor(name){this.name=name;this.children=[];this.style={};this.attrs={};const values=new Set();this.classList={add:x=>values.add(x),remove:x=>values.delete(x),contains:x=>values.has(x)};}
  get firstChild(){return this.children[0]||null;}
  appendChild(e){e.parent=this;this.children.push(e);return e;}
  insertBefore(e,before){e.parent=this;const i=this.children.indexOf(before);if(i<0)this.children.push(e);else this.children.splice(i,0,e);return e;}
  remove(){if(this.parent)this.parent.children.splice(this.parent.children.indexOf(this),1);this.parent=null;}
  setAttribute(k,v){this.attrs[k]=v;}
  getBoundingClientRect(){return {width:1000,height:700};}
  querySelector(selector){return this.children.find(e=>e.className===selector.slice(1))||null;}
 }
 const board=new Element('board');const fits=[];
 const find=(n,id)=>n.id===id?n:n.children.map(c=>find(c,id)).find(Boolean);
 const context={document:{createElement:tag=>new Element(tag),getElementById:id=>find(board,id)},$:id=>id==='board'?board:null,uiIcon:()=>'<svg></svg>',T:(en)=>en,refitTile:t=>fits.push(t),maxedTile:null,zTop:0};
 const begin=source.indexOf('  function maximizeTile('),end=source.indexOf('  function refitTile(',begin);assert.ok(begin>0&&end>begin);
 vm.runInNewContext(source.slice(begin,end)+';globalThis.api={maximizeTile,restoreBoard};',context);
 const tile=()=>{const el=new Element('tile'),head=new Element('header'),title=new Element('title'),identity=new Element('div'),avatar=new Element('button');head.className='tile-head';avatar.className='speaker-avatar';identity.className='tile-identity';title.className='tile-name';head.appendChild(avatar);identity.appendChild(title);head.appendChild(identity);el.appendChild(head);board.appendChild(el);return {el,head,title,identity,w:420,h:300,mode:'chat'};};
 return {...context.api,board,tile,fits,find};
}

test('AOP004 Restore layout occupies the actual header before the title',()=>{
 const f=fixture(),t=f.tile();f.maximizeTile(t);const button=f.find(f.board,'max-exit');
 assert.equal(button.parent,t.head,'restore button must be a header child, not a board overlay');
 assert.equal(t.head.firstChild,button);assert.equal(button.attrs['aria-label'],'Restore layout');assert.equal(button.onclick,f.restoreBoard);
 assert.equal(t.el.style.width,'984px');assert.equal(t.el.style.height,'684px');
});
test('AOP004 switching and restoring keep one control and restore prior dimensions',()=>{
 const f=fixture(),a=f.tile(),b=f.tile();f.maximizeTile(a);f.maximizeTile(b);
 assert.equal(a.el.style.width,'420px');assert.equal(a.el.style.height,'300px');
 assert.equal(f.find(f.board,'max-exit').parent,b.head);
 f.restoreBoard();assert.equal(f.find(f.board,'max-exit'),undefined);assert.equal(b.el.style.width,'420px');assert.equal(b.el.style.height,'300px');
 f.restoreBoard();assert.equal(f.fits.length,4);
});
test('AOP004 maximized header reserves grid space and names its narrow icon',()=>{
 const css=fs.readFileSync(process.env.GTMUX_MAXIMIZE_STYLE||path.join(__dirname,'style.css'),'utf8');
 assert.ok(/\.tile\.max \.max-exit\s*\{[^}]*position:\s*static/.test(css),'restore control still uses an absolute overlay');
 assert.ok(/\.tile\.max \.tile-head\s*\{[^}]*grid-template-columns:/.test(css),'maximized header has no reserved grid');
 assert.ok(css.includes('.max-exit-label'),'narrow icon label rule missing');
 assert.ok(/\.tile\.max\s*\{[^}]*width:\s*calc\(100% - 16px\)/.test(css),'maximized width does not follow board resize');
});

test('AOP006 real identity wrapper occupies the flexible maximized column',()=>{
 const f=fixture(),t=f.tile();f.maximizeTile(t);assert.equal(t.title.parent,t.identity);
 assert.ok(source.includes("identity.className = 'tile-identity'"),'fixture must model the production buildTile wrapper');
 const css=fs.readFileSync(path.join(__dirname,'style.css'),'utf8');
 assert.match(css,/\.tile\.max \.tile-head > \.tile-identity\s*\{[^}]*grid-column:\s*3/);
});
