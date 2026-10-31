// Real embedded assets, synthetic HTTP responses, random loopback port. Never
// contacts a running gtmux, writes a user record or sends to a real pane.
const http = require('node:http');
const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const path = require('node:path');
const root = path.resolve(__dirname, '../../internal/server/web');
const now = Math.floor(Date.now() / 1000);
const agents = [
  {pane_id: '%901', session: 'Web QA', agent: 'Codex', task: 'Investigate retries', status: 'working', since: now, source: 'tmux'},
  {pane_id: '%902', session: 'Web QA', agent: 'Claude Code', task: 'Review notes', status: 'waiting', since: now, source: 'tmux'},
  {pane_id: '%903', session: 'hq', agent: 'Codex', task: 'Gtmux HQ', role: 'supervisor', status: 'idle', since: now, source: 'tmux'},
  {pane_id: '', session_id: 'synthetic-native', agent: 'Codex', task: 'Desktop research', status: 'idle', since: now, source: 'native', client: 'chatgpt_desktop'},
];
const turns = () => [{prompt: 'Check **retries**', response: 'Initial finding',
  segments: [{text: 'Initial finding', steps: [{title: 'Read logs', detail: 'Synthetic detail'}]}]}];

async function start() {
  const state = {follow:{hq:false,notify:false,knowledge:false,revision:0}, deny: false, agents: agents.map(a => ({...a})), screens: {}, uploads: [], uploadStatus: 200, expectedHttpErrors: [], share: {input: true, all: true, panes: [], view_panes: []}, turns: {}, sends: []};
  const json = (res, status, body) => {
    res.writeHead(status, {'Content-Type': 'application/json', 'Cache-Control': 'no-store'});
    res.end(JSON.stringify(body));
  };
  const body = async req => {
    let data = '';
    for await (const chunk of req) data += chunk;
    return data ? JSON.parse(data) : {};
  };
  const server = http.createServer(async (req, res) => {
    try {
      const url = new URL(req.url, 'http://localhost');
      const p = url.pathname.replace(/^\/p8765/, '');
      if (p.startsWith('/api/')) {
        if (p === '/api/enroll') {
          const input = await body(req);
          return json(res, input.enrollCode === 'WRONG-1234' ? 401 : 200,
            input.enrollCode === 'WRONG-1234' ? {error: 'invalid'} : {token: 'synthetic-token'});
        }
        if (state.deny) return json(res, 401, {error: 'unauthorized'});
        if (p === '/api/upload') {
          const chunks = []; for await (const chunk of req) chunks.push(chunk);
          const input = Buffer.concat(chunks), name = /filename="([^"]+)"/.exec(input.toString('utf8'))?.[1] || 'file';
          state.uploads.push({name, bytes: input.length});
          return json(res, state.uploadStatus, state.uploadStatus === 200 ? {path: '/tmp/gtmux-test-uploads/' + name} : {error: 'synthetic upload refusal'});
        }
        const id = url.searchParams.get('id');
        if (['/api/pane', '/api/transcript', '/api/diff'].includes(p) && !id) return json(res, 400, {error: 'id required'});
        if (p === '/api/artifacts/resolve' || p === '/api/artifacts/content') {
          state.documentReads = (state.documentReads || 0) + 1;
          if (!state.share.all) return json(res,403,{error:'forbidden'});
          const image = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=','base64');
          if (p.endsWith('/resolve')) {
            const input=await body(req);
            if(input.session_id!=='image-session'||input.home_id!=='image-home'||input.reference!=='/tmp/synthetic.png') return json(res,404,{error:'unpublished'});
            return json(res,200,{id:'a'.repeat(32),name:'synthetic.png',kind:'image',media_type:'image/png',bytes:image.length,sha256:crypto.createHash('sha256').update(image).digest('hex'),created_at:1793180000,session_id:'image-session',home_id:'image-home',source:'/tmp/synthetic.png'});
          }
          if(url.searchParams.get('session_id')!=='image-session'||url.searchParams.get('home_id')!=='image-home')return json(res,404,{error:'unpublished'});
          res.writeHead(200,{'Content-Type':'application/octet-stream'}); return res.end(state.corruptDocument?Buffer.from('wrong'):image);
        }
        if (p === '/api/agents') return json(res, 200, state.agents);
        if (p === '/api/share') return json(res, 200, state.share);
        if (p === '/api/panes') return json(res, 200, [...state.agents.filter(a => a.pane_id).map(a => ({...a, tier: 'agent', window: '0', command: a.agent})),
          {pane_id: '%904', session: 'Shell', tier: 'plain', command: 'bash', window: '0', cwd: '/tmp/synthetic'}]);
        if (p === '/api/pane') return json(res, 200, {id, text: state.screens[id] ?? 'Synthetic terminal\n› Ready', cols: 90});
        if (p === '/api/transcript') return json(res, 200, state.turns[id] || turns());
        if (p === '/api/session/transcript') {
          if (state.externalSessions) {
            const record=state.externalSessions[url.searchParams.get('session_id')+'\u0000'+url.searchParams.get('home_id')];
            return json(res,record?200:404,record||{error:'not found'});
          }
          if (url.searchParams.get('home_id') !== 'synthetic-dsh-home') return json(res,422,{error:'home'});
          res.setHeader('ETag','W/"dsh-test-1"');res.setHeader('X-Gtmux-Turns-Dropped','2');
          if(req.headers['if-none-match']==='W/"dsh-test-1"'){res.writeHead(304);return res.end();}
          return json(res,200,turns());
        }
        if (p === '/api/tool-input') { state.toolReads = (state.toolReads || 0) + 1; const record = state.toolInputs && state.toolInputs[url.searchParams.get('ref')]; return json(res, record ? 200 : 404, record || {error:'not found'}); }
        if (p === '/api/session-follow') {
          if(req.method==='POST') {const input=await body(req);if(input.home_id!=='synthetic-dsh-home')return json(res,422,{error:'home'});
            if(input.settings.revision!==state.follow.revision)return json(res,409,{error:'conflict'});
            state.follow={...input.settings,revision:state.follow.revision+1};}
          return json(res,200,state.follow);
        }
        const icon = {'Codex':'codex', 'Claude Code':'claude', 'Kimi':'kimi', 'OpenCode':'opencode', 'DeepSeek Harness':'dsh'}[url.searchParams.get('agent')];
        if (p === '/api/icon' && icon) {
          const image=await fs.readFile(path.resolve(__dirname,'../../assets/agent-icons/'+icon+'.png'));res.writeHead(200,{'Content-Type':'image/png'});return res.end(image);
        }
        if (p === '/api/input-capabilities') return json(res, 200, {version:1, can_input:state.share.input, context:{boot:'fixture',pane_binding:id,proof:'synthetic',expires_at:Math.floor(Date.now()/1000)+120}, actions:[], retry_seconds:120, ...require('../../mobileapp/src/api/terminalKeysCatalog.json')});
        if (p === '/api/interaction') return json(res, 200, state.interaction || {text: 'Choose projects\n❯ [ ] Alpha\n  [x] Beta\nSpace to toggle, Enter to submit', options: [], awaiting_reply: id === '%902', truncated: false});
        if (p === '/api/options') return json(res, 200, {options: id === '%902' ? [{n: 1, label: 'Proceed'}, {n: 2, label: 'Cancel'}] : []});
        if (p === '/api/icon') { res.writeHead(200, {'Content-Type': 'image/svg+xml'}); return res.end('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="8" fill="#ececef"/><path d="m10 10-5 6 5 6m12-12 5 6-5 6" fill="none" stroke="#333" stroke-width="3"/></svg>'); }
        if (p === '/api/theme') return json(res, 200, {});
        if (p === '/api/participant-profile') {
          const sid=url.searchParams.get('session_id'),at=url.searchParams.get('at');
          const selected=state.profiles?.[sid+'@'+at];
          const agent=state.agents.find(a=>a.pane_id===id||a.session_id===sid)?.agent||'Codex';
          return json(res,200,selected||{agent,name:agent,session_id:sid||'fixture-session',historical:!!at,facts:[]});
        }
        if (p === '/api/diff') return json(res, 200, {diff: state.diff === undefined ? 'diff --git a/a b/a\n--- a/a\n+++ b/a\n@@ -1 +1 @@\n-old\n+new' : state.diff});
        if (p === '/api/send') {state.sends.push(await body(req)); return json(res, 200, {status: 'ok'});}
        return json(res, 404, {error: 'not found'});
      }
      if (p === '/favicon.ico') {res.writeHead(204); return res.end();}
      const file = path.resolve(root, '.' + (p === '' || p === '/' ? '/index.html' : p));
      if (!file.startsWith(root + path.sep)) {res.writeHead(404); return res.end();}
      const content = await fs.readFile(file);
      res.writeHead(200, {'Content-Type': {'.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.svg': 'image/svg+xml'}[path.extname(file)] || 'application/octet-stream'});
      res.end(content);
    } catch {res.writeHead(404); res.end();}
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return {state, turns, url: `http://127.0.0.1:${server.address().port}/p8765`,
    close: () => new Promise(resolve => {server.close(resolve); server.closeAllConnections();})};
}
module.exports = {start};
