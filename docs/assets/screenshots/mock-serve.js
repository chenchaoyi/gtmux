// Generic mock of `gtmux serve` — just enough endpoints for the iOS app to render
// the radar, an agent's live pane (Detail → Terminal), and the connection page,
// with fully GENERIC data (no real session names, paths, cost, or server name).
// Used ONLY to re-capture clean README/docs screenshots from the simulator (see
// regenerate.sh). Edit the fixtures below to change what the screenshots show.
const http = require('http');

const PORT = Number(process.env.MOCK_PORT || 8799);
const now = Math.floor(Date.now() / 1000);

// Radar rows — neutral project names + tasks, a mix of states and agents.
const AGENTS = [
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

const server = http.createServer((req, res) => {
  const u = new URL(req.url, 'http://x');
  const p = u.pathname;
  if (p === '/api/health') return json(res, 200, {service: 'gtmux', status: 'ok'});
  if (p === '/api/agents') return json(res, 200, AGENTS);
  if (p === '/api/pane') return json(res, 200, {id: u.searchParams.get('id') || '%7', text: PANE_TEXT,
    cursor: {x: 0, up: 0, visible: false}});
  if (p === '/api/theme') return json(res, 200, THEME);
  if (p === '/api/options') return json(res, 200, OPTIONS);
  if (p === '/api/transcript') return json(res, 200, []);
  if (p === '/api/diff') return json(res, 200, {diff: ''});
  if (p === '/api/icon') { res.writeHead(404); return res.end(); } // → neutral single-letter mark
  return json(res, 200, {status: 'ok'});
});

server.listen(PORT, '127.0.0.1', () => console.log(`mock gtmux serve on http://127.0.0.1:${PORT}`));
