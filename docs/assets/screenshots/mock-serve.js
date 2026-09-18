// Generic mock of `gtmux serve` — just enough endpoints for the iOS app to render
// the radar, an agent's live pane (Detail → Terminal), and the connection page,
// with fully GENERIC data (no real session names, paths, cost, or server name).
// Used ONLY to re-capture clean README/docs screenshots (see regenerate.sh): the
// simulator shots talk to the API here, and the README's browser shot loads the REAL
// web page (internal/server/web) from here too. Edit the fixtures below to change
// what the screenshots show.
const http = require('http');
const fs = require('fs');
const path = require('path');

const WEB = path.resolve(__dirname, '../../../internal/server/web');

const PORT = Number(process.env.MOCK_PORT || 8799);
const now = Math.floor(Date.now() / 1000);

// Radar rows — neutral project names + tasks, a mix of states and agents.
const AGENTS = [
  {pane_id: '%2', session: 'hq', window: '0', pane: '0', loc: 'hq:0.0', agent: 'Claude Code',
   role: 'supervisor', status: 'working', task: 'api is waiting on you · rest normal',
   latest: false, activity: true, source: 'tmux', project: 'hq', branch: 'main', since: now - 30},
  {pane_id: '%7', session: 'api', window: '0', pane: '0', loc: 'api:0.0', agent: 'Claude Code',
   status: 'waiting', task: 'permission to run tests', latest: false, activity: false,
   source: 'tmux', project: 'api', branch: 'main', since: now - 40},
  {pane_id: '%11', session: 'web', window: '0', pane: '0', loc: 'web:0.0', agent: 'Claude Code',
   status: 'working', task: 'refactor auth middleware', latest: false, activity: true,
   source: 'tmux', project: 'web', branch: 'main', since: now - 90},
  {pane_id: '%3', session: 'worker', window: '0', pane: '0', loc: 'worker:0.0', agent: 'Codex',
   status: 'working', task: 'add retry backoff', latest: false, activity: true,
   source: 'tmux', project: 'worker', branch: 'jobs', since: now - 130},
  {pane_id: '%8', session: 'docs', window: '0', pane: '0', loc: 'docs:0.0', agent: 'Claude Code',
   status: 'idle', task: 'update API reference', latest: true, activity: false,
   source: 'tmux', project: 'docs', branch: 'main', since: now - 6 * 60},
  {pane_id: '%1', session: 'cli', window: '0', pane: '0', loc: 'cli:0.0', agent: 'Gemini',
   status: 'idle', task: 'fix flaky test', latest: false, activity: false,
   source: 'tmux', project: 'cli', branch: 'main', since: now - 22 * 60},
];

// A generic Claude Code "permission" screen for the waiting pane (%7) — what you
// jump to. No real paths/content. ANSI colors so the mirror renders in color.
const PANE_TEXT = [
  '\x1b[2m› web-api  ·  main\x1b[0m',
  '',
  '\x1b[38;5;114m●\x1b[0m Bash(npm test)',
  '  \x1b[2m⎿ running the test suite…\x1b[0m',
  '',
  '\x1b[1mDo you want to run this command?\x1b[0m',
  '',
  '\x1b[38;5;114m❯ 1. Yes\x1b[0m',
  '  2. Yes, and don’t ask again this session',
  '  3. No, tell the agent what to change  \x1b[2m(esc)\x1b[0m',
  '',
].join('\r\n');

// One screen per pane, so the browser's concurrent tiles don't all show the same thing.
const PANE_TEXTS = {
  '%11': [
    '\x1b[2m› web  ·  main\x1b[0m',
    '',
    '\x1b[38;5;114m●\x1b[0m Read(src/auth/middleware.ts)',
    '  \x1b[2m⎿ 214 lines\x1b[0m',
    '',
    '\x1b[38;5;114m●\x1b[0m Edit(src/auth/middleware.ts)',
    '  \x1b[2m⎿ split verifyToken() out of the request handler\x1b[0m',
    '',
    '\x1b[38;5;75m✻\x1b[0m Writing the table-driven test…',
  ].join('\r\n'),
  '%3': [
    '\x1b[2m› worker  ·  jobs\x1b[0m',
    '',
    '\x1b[38;5;114m●\x1b[0m retry backoff: 1s → 2s → 4s, capped at 30s',
    '  \x1b[2m⎿ queue/retry.go +48 −6\x1b[0m',
    '',
    '\x1b[38;5;75m✻\x1b[0m Running go test ./queue/…',
  ].join('\r\n'),
  '%2': [
    '\x1b[2m› hq  ·  supervisor\x1b[0m',
    '',
    '\x1b[38;5;180m⟣ ◈ brief 09:41 │ 1 needs you · 2 working\x1b[0m',
    '  \x1b[2m· api is waiting on a test-run approval\x1b[0m',
    '  \x1b[2m· web and worker are moving, nothing stuck\x1b[0m',
  ].join('\r\n'),
};

const THEME = {
  source: 'mock', background: '#0d1117', foreground: '#c9d1d9', cursor: '#06B6D4',
  palette: ['#0d1117', '#ff7b72', '#3fb950', '#d29922', '#58a6ff', '#bc8cff', '#39c5cf', '#b1bac4',
    '#6e7681', '#ffa198', '#56d364', '#e3b341', '#79c0ff', '#d2a8ff', '#56d4dd', '#f0f6fc'],
  fontFamily: 'Menlo', fontSize: 13,
};

const OPTIONS = {options: [
  {n: 1, label: 'Yes'},
  {n: 2, label: 'Yes, and don’t ask again this session'},
  {n: 3, label: 'No, tell the agent what to change'},
]};

function json(res, code, body) {
  const b = JSON.stringify(body);
  res.writeHead(code, {'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(b)});
  res.end(b);
}

// The REAL browser surface, served from the repo. The page keeps its token and its
// board layout in localStorage, so `/` gets one extra script that seeds both — that is
// the whole difference between this and what a paired browser loads.
const BOARD = {railW: 232, snap: false, surface: false, tiles: [
  {id: '%7',  x: 18,  y: 16,  w: 500, h: 402, mode: 'term'},
  {id: '%11', x: 534, y: 16,  w: 458, h: 300, mode: 'term'},
  {id: '%3',  x: 534, y: 332, w: 458, h: 300, mode: 'term'},
  {id: '%2',  x: 18,  y: 434, w: 500, h: 198, mode: 'term'},
]};
const TYPES = {'.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css',
  '.woff2': 'font/woff2', '.woff': 'font/woff', '.png': 'image/png', '.svg': 'image/svg+xml'};

function web(res, p) {
  const rel = p === '/' ? 'index.html' : p.replace(/^\/+/, '');
  const file = path.resolve(WEB, rel);
  if (!file.startsWith(WEB + path.sep) || !fs.existsSync(file)) { res.writeHead(404); return res.end(); }
  let body = fs.readFileSync(file);
  if (rel === 'index.html') {
    const boot = '<script>try{localStorage.setItem(\'gtmux.token\',\'ab12cd34ef567890\');' +
      'localStorage.setItem(\'gtmux.board\',' + JSON.stringify(JSON.stringify(BOARD)) + ');}catch(e){}</script>';
    body = Buffer.from(body.toString().replace('</head>', boot + '</head>'));
  }
  res.writeHead(200, {'Content-Type': TYPES[path.extname(file)] || 'application/octet-stream',
    'Content-Length': body.length});
  res.end(body);
}

const server = http.createServer((req, res) => {
  const u = new URL(req.url, 'http://x');
  const p = u.pathname;
  if (p === '/api/health') return json(res, 200, {service: 'gtmux', status: 'ok'});
  if (p === '/api/agents') return json(res, 200, AGENTS);
  if (p === '/api/pane') {
    const id = u.searchParams.get('id') || '%7';
    return json(res, 200, {id, text: PANE_TEXTS[id] || PANE_TEXT, cursor: {x: 0, up: 0, visible: false}});
  }
  // The caller's own capability. This mock has one caller and it is the owner.
  if (p === '/api/share') return json(res, 200, {input: true, all: true, panes: [], view_panes: []});
  if (p === '/api/theme') return json(res, 200, THEME);
  if (p === '/api/options') return json(res, 200, OPTIONS);
  if (p === '/api/transcript') return json(res, 200, []);
  if (p === '/api/diff') return json(res, 200, {diff: ''});
  if (p === '/api/icon') { res.writeHead(404); return res.end(); } // → neutral single-letter mark
  if (p.startsWith('/api/')) return json(res, 200, {status: 'ok'});
  return web(res, p);
});

server.listen(PORT, '127.0.0.1', () => console.log(`mock gtmux serve on http://127.0.0.1:${PORT}`));
