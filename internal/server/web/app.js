/* gtmux browser mirror. Pairs via a one-time #c=<code> link, then polls /api/agents
 * (radar, app-styled) and /api/pane (live terminal, xterm.js, Ghostty colors).
 * Incremental writes (append the new tail, not a full reset) so the terminal doesn't
 * flash. Input goes through the shared composer where this page may type: the owner
 * everywhere, a guest only on panes the host allowed (GET /api/share). */
(function () {
  'use strict';
  var TOKEN_KEY = 'gtmux.token';
  var COLORS = {waiting: '#EF4444', working: '#06B6D4', idle: '#22C55E', running: '#8E8E93'};
  // 'errored' is a SECTION, not a status: the state language has four states, and errored
  // says how an idle turn ENDED. It sits right after "needs you" because a session stopped
  // on a failure needs a person — but NOT inside it, because that section means an agent is
  // asking something you can answer, and an error is not a question.
  var ORDER = ['waiting', 'errored', 'working', 'idle'];
  // A quiet presentation group does not rewrite lifecycle evidence on the row.
  function sectionStatus(status) { return status === 'waiting' || status === 'working' ? status : 'idle'; }
  var LABEL; // the section and status words, in the reader's language: set beside T below
  // terminal defaults taken from the user's Ghostty config (Hack 15, #17171a/#d4d2cc).
  var GHOSTTY = {bg: '#17171a', fg: '#d4d2cc', cursor: '#bbc1ff', sel: '#2a2a33', font: 'Hack, Menlo, Monaco, "Courier New", monospace', size: 15};
  var MARKS = {'deepseek harness':'DS', dsh:'DS', 'claude code': 'CC', claude: 'CC', codex: 'Cx', gemini: 'G', aider: 'Ai', opencode: 'oc', cursor: 'Cu', crush: 'Cr', amp: 'Am', cline: 'Cl'};

  var $ = function (id) { return document.getElementById(id); };
  var token = null, radarTimer = null, paneTimer = null, selIdx = -1;
  var viewEpoch = 0, readSeq = {}, appliedSeq = {};
  // A response belongs to the view and credential that requested it. A newer read
  // of the same resource wins, even if the network delivers replies out of order.
  function currentRead(kind, signal) {
    var epoch = viewEpoch, credential = token, seq = (readSeq[kind] || 0) + 1;
    readSeq[kind] = seq;
    return function (apply) {
      if ((signal && signal.aborted) || epoch !== viewEpoch || credential !== token || seq < (appliedSeq[kind] || 0)) return false;
      if (apply) appliedSeq[kind] = seq;
      return true;
    };
  }
  // Read lanes belong to one view and credential. Mutations never enter a lane.
  var readLanes = new Map();
  function cancelRead(key) {
    var lane = readLanes.get(key);
    if (!lane) return;
    readLanes.delete(key); lane.cancel();
  }
  function cancelReads() { Array.from(readLanes.keys()).forEach(cancelRead); }
  function runRead(key, work, timeoutMs, onTimeout) {
    var held = readLanes.get(key);
    if (held && held.epoch === viewEpoch && held.credential === token) return;
    cancelRead(key);
    var controller = new AbortController(), timer, rejectRead;
    var lane = {epoch: viewEpoch, credential: token, cancel: function () {
      clearTimeout(timer); controller.abort(); rejectRead(new Error('read retired'));
    }};
    var deadline = new Promise(function (_, reject) {
      rejectRead = reject;
      timer = setTimeout(function () {
        controller.abort(); reject(new Error('read deadline exceeded'));
        if (readLanes.get(key) === lane && lane.epoch === viewEpoch && lane.credential === token && onTimeout) onTimeout();
      }, timeoutMs || 15000);
    });
    readLanes.set(key, lane);
    var request;
    try { request = work(controller.signal); } catch (error) { request = Promise.reject(error); }
    Promise.race([request, deadline]).catch(function () {}).finally(function () {
      clearTimeout(timer);
      if (readLanes.get(key) === lane) readLanes.delete(key);
    });
  }
  var term = null, fit = null, lastText = '', curPane = null, lastSig = '', theme = null;
  var curAgent = null, chatTimer = null, chatSig = '', lastTurns = [], chatExpanded = {};
  var chatLive = {}, serverClock = 0, serverClockAt = 0;
  var pendingPrompts = {};
  var lastAgents = [], optTimer = null; // for focus prev/next + the waiting reply bar
  var interactionAnsweredAt = 0;
  var lastInteraction = null, interactionQuestion = '', interactionFailed = false, interactionPeek = false, interactionCollapsed = false, interactionRenderSig = '';
  var lastOpts = [], lastOptsSig = ''; // waiting options, shared by reply bar (term) + approval card (chat)
  var userScrolling = false, scrollIdle = null, pendingText = null, resizeIdle = null;
  var iconCache = {}; // agentName -> objectURL | 'none' | Promise
  var BUNDLED = ['Hack', 'JetBrains Mono', 'Fira Code', 'IBM Plex Mono'];
  function lsGet(k, d) { try { return localStorage.getItem(k) || d; } catch (e) { return d; } }
  var fontPref = lsGet('gtmux.fontPref', 'auto');          // 'auto' | 'system' | a bundled family
  var appearancePref = lsGet('gtmux.appearance', 'system');
  if (!['system', 'light', 'dark'].includes(appearancePref)) appearancePref = 'system';
  var systemAppearance = null;
  var sizePref = parseInt(lsGet('gtmux.fontSize', '0'), 10) || 0; // 0 follows the host terminal/default.
  if (sizePref) sizePref = Math.max(10, Math.min(22, sizePref));
  var chatSizePref = Math.max(12, Math.min(24, parseInt(lsGet('gtmux.chatFontSize', '14'), 10) || 14));
  var preferredPaneMode = lsGet('gtmux.paneMode', 'term');
  if (preferredPaneMode !== 'chat' && preferredPaneMode !== 'term') preferredPaneMode = 'chat';
  var activeSessionTool = null;
  var paneMode = preferredPaneMode;          // pane view tab: 'term' | 'chat'

  // ---- auth -------------------------------------------------------------
  // BASE is the path prefix this mirror is served under. The multi-tenant Direct
  // tunnel routes each Mac at https://host/p<port>/…, so the page loads at /p<port>/
  // and every /api/… call must carry that prefix — an absolute /api/… would hit the
  // VPS's legacy fallback (the WRONG Mac) and pairing would fail. Empty for a plain
  // serve/Cloudflare tunnel served at the root.
  var BASE = typeof window !== 'undefined' && typeof window.GTMUX_WEB_BASE === 'string'
    ? window.GTMUX_WEB_BASE
    : (location.pathname || '/').replace(/\/index\.html$/, '').replace(/\/+$/, '');
  function authHeaders() { return token ? {Authorization: 'Bearer ' + token} : {}; }
  function api(path, opts) {
    if (/^\/api\/(?:session\/)?transcript\?/.test(path)) path += '&context=1';
    opts = opts || {};
    opts.headers = Object.assign({}, opts.headers || {}, authHeaders());
    return fetch(BASE + path, opts);
  }
  function pair(code) {
    return fetch(BASE + '/api/enroll', {
      method: 'POST', headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({enrollCode: code, name: 'Browser'}),
    }).then(function (r) { if (!r.ok) throw new Error('pair'); return r.json(); })
      .then(function (j) { token = j.token; try { localStorage.setItem(TOKEN_KEY, token); } catch (e) {} });
  }

  // ---- the code box ------------------------------------------------------
  //
  // A share link IS a short code: the address, then eight characters. Pasted whole it
  // arrives as `#code=`, and where nothing can be pasted (a TV browser, a locked-down
  // machine, someone else's laptop you are reading your own screen into) the same two
  // lines are read out and typed here. Either way it redeems through the endpoint a
  // pairing code uses, so there is one door and one kind of credential behind it.
  function setupCodeBox() {
    var form = $('gate-code'), input = $('gate-code-input'), go = $('gate-code-go'), why = $('gate-code-why');
    if (!form || form.dataset.ready) return;
    form.dataset.ready = '1';
    $('gate-code-label').textContent = T('Enter a pairing or share code', '输入配对码或分享码');
    input.placeholder = 'XXXX-XXXX';
    go.textContent = T('Open', '打开');
    form.addEventListener('submit', function (ev) {
      ev.preventDefault();
      var code = (input.value || '').trim();
      if (!code) return;
      why.hidden = true;
      go.disabled = true;
      go.textContent = T('Opening…', '正在打开…');
      pair(code).then(function () {
        // It opened. The browser keeps the token from here on, so the box empties and the
        // code is not left sitting on screen.
        input.value = '';
        go.disabled = false; go.textContent = T('Open', '打开');
        fetchTheme(); fetchShare(); setupSettings(); home();
      }).catch(function () {
        go.disabled = false;
        go.textContent = T('Open', '打开');
        why.textContent = T('Could not use this code. Check it and try again. If it has expired or was already used, get a new code.',
                            '无法使用此码。请核对后重试；如果已过期或已用过，请获取新码。');
        why.hidden = false;
        input.select();
      });
    });
  }

  // ---- language ---------------------------------------------------------
  //
  // The browser mirror is the ONE surface you hand to someone else: a share link goes to
  // a guest, who is the person least likely to read the host's language. It had drifted
  // into Chinese-only chrome anyway — the gate screen and the connection tooltip were
  // bilingual, everything else was not — so an English reader opening a guest link met a
  // page they could not operate.
  //
  // It follows the BROWSER's language, the way the phone follows the device's. There is
  // no gtmux setting to read here: the page is opened by someone who may have no gtmux
  // install at all. The gate screen keeps showing BOTH languages on purpose — it is the
  // screen you photograph and send to whoever can fix it.
  var ZH = /^zh\b/i.test((navigator.languages && navigator.languages[0]) || navigator.language || '');
  function T(en, zh) { return ZH ? zh : en; }
  // The radar's section headers, the rail's groups and the ⌘K rows. These were English on
  // a Chinese page (%12, 2026-10-06, after #1441's first round); the words are the phone's.
  // Set here, not where the other constants are, because T reads ZH, which is set above.
  LABEL = {waiting: T('needs attention', '待处理'), errored: T('errored', '出错'), working: T('working', '运行中'), idle: T('idle', '空闲'), running: T('idle', '空闲')};

  // Chrome that lives in index.html, labelled once at boot. Keeping the markup's text as
  // the CHINESE half and translating in one place beats sprinkling data-en attributes
  // through the HTML, and it puts every string a reader sees in one list.
  var CHROME = [
    ['back', {html: uiIcon('back'), title: T('Back to workspace', '返回工作区'), label: T('Back to workspace', '返回工作区')}],
    ['panes-btn', {html: uiIcon('layout'), title: T('All panes', '所有窗格'), label: T('All panes', '所有窗格')}],
    ['panes-title', {text: T('All panes', '所有窗格')}],
    ['panes-search', {placeholder: T('Session, command or directory', '搜索会话、命令或目录')}],
    ['cmdk-input', {placeholder: T('Search by name, agent or %id', '搜索名称、Agent 或 %id'), label: T('Find a session', '查找会话')}],
    ['jump', {html: uiIcon('down') + T('Latest messages', '最新消息') + '<span class="jdot" hidden></span>'}],
    ['chat-ro', {text: T('Read-only · ask the owner to enable input for this pane.', '只读 · 可请分享者开启此窗格的输入。')}],
    ['pane-ro', {text: T('🔒 Input is not allowed in this pane', '🔒 此窗格不允许输入')}],
    ['wb-snap', {html: '<span class="wb-sw"></span>' + T('Snap to grid', '对齐网格'), title: T('Snap to grid', '对齐网格')}],
    ['wb-preset', {html: uiIcon('layout') + T('Layout', '布局') + '<span id="wb-preset-cur"></span>' + uiIcon('chevron'), title: T('Layout presets', '布局预设')}],
    // The focus bar, the appearance panel and the workbench rail (%12, 2026-10-06: these
    // stayed English on a Chinese page; the test above only caught the other direction).
    ['pane-prev', {html: uiIcon('left'), title: T('Previous pane', '上一个窗格'), label: T('Previous pane', '上一个窗格')}],
    ['pane-next', {html: uiIcon('right'), title: T('Next pane', '下一个窗格'), label: T('Next pane', '下一个窗格')}],
    ['copy-screen', {html: uiIcon('copy'), title: T('Copy visible screen', '复制屏幕内容'), label: T('Copy visible screen', '复制屏幕内容')}],
    ['gear', {html: uiIcon('settings'), title: T('Settings', '设置'), label: T('Settings', '设置')}],
    ['wb-gear', {html: uiIcon('settings'), title: T('Settings', '设置'), label: T('Settings', '设置')}],
    ['wb-conn', {title: T('Connection', '连接')}],
    ['font-lbl', {text: T('Font', '字体')}],
    ['font-auto', {text: T('Match terminal', '跟随终端')}],
    ['font-system', {text: T('System', '系统')}],
    ['appearance-lbl', {text: T('Appearance', '外观')}],
    ['appearance-system', {text: T('System', '跟随系统')}],
    ['appearance-light', {text: T('Light', '浅色')}],
    ['appearance-dark', {text: T('Dark', '深色')}],
    ['return-send-lbl', {text: T('Enter to send', '回车发送')}],
    ['settings-title', {text: T('Settings', '设置')}],
    ['settings-chat', {text: T('Chat', '对话')}],
    ['settings-terminal', {text: T('Terminal', '终端')}],
    ['settings-scope', {text: T('Saved in this browser. Changes apply immediately.', '仅影响当前浏览器，即时生效。')}],
    ['settings-navigation', {label: T('Settings categories', '设置分类')}],
    ['settings-nav-appearance', {html: uiIcon('sun') + T('Appearance', '外观')}],
    ['settings-nav-chat', {html: uiIcon('snippets') + T('Chat', '对话')}],
    ['settings-nav-terminal', {html: uiIcon('terminal') + T('Terminal', '终端')}],
    ['settings-chat-hint', {text: T('Reading size and keyboard sending behavior.', '调整阅读字号和键盘发送方式。')}],
    ['settings-terminal-hint', {text: T('Font and size for the terminal display.', '字体和字号仅用于终端画面。')}],
    ['settings-close', {html: uiIcon('close'), label: T('Close settings', '关闭设置'), title: T('Close settings', '关闭设置')}],
    ['settings-reset', {text: T('Reset', '恢复默认')}],
    ['chat-size-lbl', {text: T('Text size', '正文字号')}],
    ['session-trace', {title: T('Trace', '轨迹'), html: uiIcon('trace') + T('Trace', '轨迹')}],
    ['session-diff', {title: 'Diff', html: uiIcon('diff') + 'Diff'}],

    ['size-lbl', {text: T('Text size', '字号')}],
    ['rail-title', {text: T('Sessions', '会话')}],
    ['rail-search', {placeholder: T('Search sessions', '搜索会话')}],
    ['rail-collapse', {html: uiIcon('sidebarClose'), title: T('Collapse sidebar', '收起侧栏'), label: T('Collapse sidebar', '收起侧栏')}],
    ['rail-resize', {title: T('Drag to resize', '拖动调整宽度')}],
    ['rail-tab', {html: uiIcon('sidebarOpen') + '<span id="rail-tab-n"></span>', title: T('Show sidebar', '显示侧栏'), label: T('Show sidebar', '显示侧栏')}],
    ['panes-back', {html: uiIcon('back'), title: T('Back', '返回'), label: T('Back', '返回')}],
  ];
  function labelChrome() {
    // The page's language is the reader's, so screen readers, hyphenation and fonts pick
    // the right one (it was fixed at "en").
    document.documentElement.lang = ZH ? 'zh-CN' : 'en';
    CHROME.forEach(function (row) {
      var e = $(row[0]); if (!e) return;
      var v = row[1];
      if (v.html != null) e.innerHTML = v.html;
      else if (v.text != null) e.textContent = v.text;
      if (v.placeholder != null) e.placeholder = v.placeholder;
      if (v.title != null) e.title = v.title;
      if (v.label != null) e.setAttribute('aria-label', v.label);
    });
    document.querySelectorAll('#mode button').forEach(function (b) {
      b.textContent = b.dataset.mode === 'term' ? T('Terminal', '终端') : b.dataset.mode === 'trace' ? T('Trace', '轨迹') : T('Chat', '对话');
    });
    var hint = document.querySelector('.rail-hint');
    // A double-click on a row ADDS it to the board (treeRow.ondblclick → addTile); it never
    // opened full screen, which is the tile's ⤢ (%12, 2026-10-06).
    if (hint) hint.textContent = T('Double-click a session to open it, or drag it into the workspace', '双击打开会话，或拖入工作区');
    $('rail-search').setAttribute('aria-label', T('Search sessions', '搜索会话'));
    $('panes-search').setAttribute('aria-label', T('Search panes', '搜索窗格'));
    var lead = document.querySelector('.rb-lead');
    if (lead) lead.textContent = T('Read-only · awaiting reply:', '只读 · 等待回复：');
    var rhint = document.querySelector('.rb-hint');
    // Only shown where this page may not type. It used to say "or scan to take over":
    // there is no scan here, and the same link on a phone is still view-only (%12,
    // 2026-10-06). What does work is the sharer allowing input, or another device.
    if (rhint) rhint.textContent = T('Read-only. Ask the owner to enable input, or reply from a device with access.', '当前为只读。请联系分享者开启输入，或在已授权的设备上回复。');
    $('cmdk').setAttribute('aria-label', T('Find a session', '查找会话'));
    $('cmdk-list').setAttribute('aria-label', T('Sessions', '会话'));
  }

  // ---- helpers ----------------------------------------------------------
  function show(which) { if (activeSessionTool) activeSessionTool.sheet.el.retire(); $('session-toolbar').hidden = which !== 'chat' && which !== 'pane'; $('session-trace').hidden = !(curAgent && (curAgent.agent || readableConversation(curAgent))); $('session-diff').hidden = !hasPane(curAgent); if (which !== 'chat') $('chat-interaction').hidden = true; cancelReads(); viewEpoch++; ['gate', 'radar', 'panes', 'pane', 'chat', 'workbench'].forEach(function (id) { $(id).hidden = id !== which; }); if ($('chat-input')) $('chat-input').hidden = true; if (which !== 'workbench') WB.on = false; updatePageTitle(which); }
  function updatePageTitle(view) {
    view = view || ['gate', 'radar', 'panes', 'pane', 'chat', 'workbench'].find(function (id) { return !$(id).hidden; });
    var title = T('Agent workspace', 'Agent 工作台');
    if (activeSessionTool && !activeSessionTool.closed) title = primary(activeSessionTool.agent) + ' · ' + (activeSessionTool.kind === 'trace' ? T('Trace', '轨迹') : 'Diff');
    else if (view === 'gate') title = T('Connect', '连接');
    else if (view === 'panes') title = T('All panes', '所有窗格');
    else if ((view === 'chat' || view === 'pane') && curAgent) {
      var mode = paneMode === 'chat' ? T('Chat', '对话') : T('Terminal', '终端');
      title = primary(curAgent) + ' · ' + mode;
    }
    document.title = title + ' · gtmux';
  }
  // GATE states. Each says what the situation IS and the one thing to do about it, in
  // both languages — the old screen was a single English sentence with no subject, and it
  // pointed at a phone-app affordance ("open on computer") that does not exist. The
  // instruction below is the one `gtmux pair` actually prints.
  // Who actually lands here decides the order of these words.
  //
  // This page is reached by two people. The owner, who typed their own tunnel address and
  // needs `gtmux pair`. And a GUEST, whose share link stopped working — the token their
  // browser kept was revoked, or the browser dropped it (Safari clears script storage for
  // a site nobody has visited in a week). The copy used to lead with "this browser isn't
  // paired with YOUR Mac" over an instruction to run a command on a Mac a guest does not
  // own, with their own way in as a footnote. On a URL whose whole purpose is being handed
  // to someone else, that is the wrong reader first.
  //
  // So both paths are on the page, the shared one first, and neither pretends to know
  // which person is reading.
  var SHARED_STEP = {
    zh: '收到分享链接：打开完整链接，或在下方输入链接末尾的分享码。',
    en: 'Have a share link? Open the full link or enter its code below.'
  };
  var OWN_STEP = {
    zh: '连接自己的服务器：在服务器上运行 gtmux pair，然后打开输出中的浏览器链接。',
    en: 'Connecting to your own server? Run gtmux pair on the server, then open the browser link it prints.',
    code: 'gtmux pair'
  };
  var GATE = {
    unpaired: {
      zh: '这个浏览器尚未获得访问权限。',
      en: 'This browser does not have access yet.',
      steps: [SHARED_STEP, OWN_STEP]
    },
    expired: {
      zh: '这个浏览器的访问权限已失效。',
      en: 'This browser’s access is no longer valid.',
      steps: [SHARED_STEP, OWN_STEP],
      note: {zh: '可能是访问已被撤销、分享链接已过期，或浏览器清除了保存的凭证。',
             en: 'Access may have been revoked, the share link may have expired, or the browser may have cleared its saved credentials.'}
    }
  };

  // gate renders one of those states. Built as DOM (not a string) so a command renders as
  // a command — the old copy carried markdown backticks into a rendered page, where they
  // showed up as literal ` characters.
  function gate(which) {
    clearComposeData();
    document.querySelectorAll('.browser-dialog').forEach(function (dialog) { if (dialog.retire) dialog.retire(); else { dialog.close(); dialog.remove(); } });
    var g = GATE[which] || GATE.unpaired;
    var state = $('gate-state'), steps = $('gate-steps');
    state.textContent = g.zh;
    state.appendChild(el('span', 'en', g.en));
    steps.textContent = '';
    g.steps.forEach(function (st) {
      steps.appendChild(el('p', '', st.zh));
      steps.appendChild(el('p', 'en', st.en));
      if (st.code) steps.appendChild(el('code', '', st.code));
    });
    if (g.note) {
      var n = el('p', 'note', g.note.zh);
      n.appendChild(el('span', 'en', g.note.en));
      steps.appendChild(n);
    }
    setupCodeBox();
    $('gate-code').hidden = false;
    show('gate');
    $('bar').hidden = false; $('title').textContent = 'gtmux'; $('sub').textContent = '';
    $('conn').hidden = true;
    $('mode').hidden = true; $('back').hidden = true; $('gear').hidden = false; $('panes-btn').hidden = true;
    $('settings').hidden = true; hidePresetMenu(); closeCmdk(); $('cmdk-list').textContent = ''; hideFocusChrome();
    ['identity', 'wb-identity', 'guest-scope'].forEach(function (id) { var n = $(id); if (n) n.hidden = true; });
    SHARE = {known: false, input: false, all: false, panes: {}};
    curPane = null; curAgent = null; lastAgents = [];
    lastTurns = []; chatSig = ''; chatExpanded = {}; chatLive = {}; pendingPrompts = {}; $('chat').innerHTML = '';
    if (paneComposer) paneComposer.reset();
    WB.agents = [];
    clearInterval(radarTimer); clearInterval(paneTimer); clearInterval(chatTimer); clearInterval(panesTimer);
    radarTimer = paneTimer = chatTimer = panesTimer = null;
    // Preserve the layout, but discard cached contents and stop all tile requests.
    // The next successful pairing restores only panes in its authorized roster.
    if (WB.tiles.length) {
      WB.saved = WB.tiles.map(function (t) { return {id: t.id, x: t.x, y: t.y, w: t.w, h: t.h, mode: t.mode}; });
      WB.tiles.forEach(function (t) { clearInterval(t.timer); t.removed = true; if (t.inputDispose) t.inputDispose(); if (t.term) t.term.dispose(); t.el.remove(); });
      WB.tiles = []; maxedTile = null; $('board').classList.remove('maximized');
      var exit = $('board').querySelector('.max-exit'); if (exit) exit.remove();
    }
  }

  // el builds a text node element (no innerHTML anywhere in the gate).
  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    e.textContent = text;
    return e;
  }
  // Connection indicator (the rule: server name + a status dot, never the word "live"). 3 states:
  // connected green / reconnecting amber (first failure) / offline red (persistent). Both the radar/pane bar
  // (#conn) and the workbench bar (#wb-conn) render the same shared state.
  function serverLabel() {
    var h = location.hostname || 'server';
    if (h === 'localhost' || /^[0-9.]+$/.test(h) || h.indexOf(':') !== -1) return h; // IP / localhost as-is
    return h.split('.')[0]; // gtmux-qclyu2s2.ccy.dev → gtmux-qclyu2s2
  }
  var connFails = 0;
  function connStateFor(ok) { if (ok) { connFails = 0; return 'live'; } connFails++; return connFails === 1 ? 'retry' : 'off'; }
  var CONN_TITLE = {live: T('connected', '已连接'), retry: T('reconnecting…', '重连中'), off: T('offline', '离线')};
  function renderConn(el, st) { el.hidden = false; el.className = 'conn ' + st; el.textContent = serverLabel(); el.title = CONN_TITLE[st] || ''; }
  function setConn(ok) { renderConn($('conn'), connStateFor(ok)); }
  function isNative(a) { return a.source === 'native'; }
  function readableConversation(a) { return !!a && isNative(a) && a.can_read_conversation === true && !!a.session_id; }
  function hasPane(a) { return !!a && !isNative(a) && !!a.pane_id; }
  function primary(a) {
    if (a.task) return a.task;
    if (isNative(a)) return a.project || a.terminal || '';
    return a.session || a.loc || a.pane_id || '';
  }
  function launchHost(a) {
    var h = a && a.launch_host;
    return h && typeof h.name === 'string' && h.name && Array.from(h.name).length <= 80 && (h.kind === 'app' || h.kind === 'process') && Number.isSafeInteger(h.observed_at) && h.observed_at > 0 ? h : null;
  }
  function nativeSourceLabel(a) {
    var h = launchHost(a);
    if (h) return a.agent + ' · ' + h.name;
    if (a.current_client === 'unconfirmed') return a.agent + ' · ' + T('External session', '外部会话');
    if (a.client === 'chatgpt_desktop') return a.agent + ' · ' + T('ChatGPT desktop', 'ChatGPT 桌面版');
    if (a.client === 'terminal') return a.agent + ' · ' + (a.terminal || T('terminal', '终端'));
    return a.terminal || a.agent;
  }
  function nativeEvidenceLabel(a) {
    var h = launchHost(a), parts = [];
    if (h) parts.push(T('Launch host: ', '启动宿主：') + h.name + T(' · observed ', ' · 记录于 ') + new Date(h.observed_at * 1000).toLocaleString(ZH ? 'zh' : 'en'));
    if (a && a.current_client === 'unconfirmed') parts.push(T('Current input client unconfirmed', '当前输入客户端未确认'));
    return parts.join(' · ');
  }
  function secondary(a) {
    if (isNative(a)) return nativeSourceLabel(a); // sensed agent, no pane locator
    var b = a.session || a.loc || '';
    return a.pane_id ? b + ' · ' + a.pane_id : b;
  }

  function agentMark(name) {
    var k = (name || '').trim().toLowerCase();
    if (MARKS[k]) return MARKS[k];
    for (var key in MARKS) { if (k.indexOf(key) !== -1) return MARKS[key]; }
    var c = (name || '').trim();
    // A pane with NO coding agent (a plain shell, opened from the all-panes browser)
    // shows a terminal glyph, not '?' — a plain pane is first-class, not an unknown
    // agent (tiered-pane-control; matches the mobile agentMark).
    return c ? c.slice(0, 2) : '$_';
  }
  function relTime(since) {
    if (!since) return '';
    var s = Math.max(0, Math.floor(Date.now() / 1000) - since);
    if (s < 60) return s + 's';
    if (s < 3600) return Math.floor(s / 60) + 'm';
    if (s < 86400) return Math.floor(s / 3600) + 'h';
    return Math.floor(s / 86400) + 'd';
  }
  // status badge — the triple-encoded mark (color+shape+glyph), same as the app.
  function badgeSVG(st, errored) {
    if (errored) return '<svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="7" fill="#F59E0B"/><path d="M8 4v5" stroke="#fff" stroke-width="1.8" stroke-linecap="round"/><circle cx="8" cy="11.5" r="1" fill="#fff"/></svg>';
    var c = COLORS[st] || COLORS.running, W = '#fff', shape, glyph = '';
    if (st === 'waiting') {
      shape = '<rect x="1" y="1" width="14" height="14" rx="4" fill="' + c + '"/>';
      glyph = '<rect x="5.1" y="4.6" width="1.7" height="6.8" rx="0.85" fill="' + W + '"/><rect x="9.2" y="4.6" width="1.7" height="6.8" rx="0.85" fill="' + W + '"/>';
    } else {
      shape = '<circle cx="8" cy="8" r="7" fill="' + c + '"/>';
      // The working ring TURNS (DESIGN §10, 2026-08-13). The class carries the
      // animation so CSS can stop it under prefers-reduced-motion.
      if (st === 'working') glyph = '<circle class="spin" cx="8" cy="8" r="3.4" stroke="' + W + '" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-dasharray="13 6"/>';
      else if (st === 'idle') glyph = '<path d="M4.8 8.3 L7 10.5 L11.2 5.6" stroke="' + W + '" stroke-width="1.7" fill="none" stroke-linecap="round" stroke-linejoin="round"/>';
      else glyph = '<circle cx="8" cy="8" r="1.9" fill="' + W + '"/>';
    }
    return '<svg width="14" height="14" viewBox="0 0 16 16">' + shape + glyph + '</svg>';
  }
  // official icon via /api/icon (authed → blob), cached; falls back to a monogram.
  function loadIcon(name, img, mono) {
    var key = name || '', v = iconCache[key];
    if (v === 'none') return;
    if (typeof v === 'string') { img.src = v; img.style.display = ''; mono.style.display = 'none'; return; }
    if (v && v.then) { v.then(function () { loadIcon(key, img, mono); }); return; }
    var p = api('/api/icon?agent=' + encodeURIComponent(key)).then(function (r) {
      if (!r.ok) throw new Error('noicon'); return r.blob();
    }).then(function (b) { iconCache[key] = URL.createObjectURL(b); }).catch(function () { iconCache[key] = 'none'; });
    iconCache[key] = p;
    p.then(function () { loadIcon(key, img, mono); });
  }

  // avatarEl builds the rounded-square avatar (official icon → monogram fallback),
  // optionally with the status badge. Shared by the radar rows + the chat view.
  function avatarEl(a, size, withBadge) {
    var av = document.createElement('div'); av.className = 'avatar';
    if (size) { av.style.width = size + 'px'; av.style.height = size + 'px'; }
    var img = document.createElement('img'); img.alt = ''; img.style.display = 'none';
    var mono = document.createElement('span'); mono.className = 'mono'; mono.textContent = agentMark(a.agent);
    av.appendChild(img); av.appendChild(mono);
    if (withBadge) { var bdg = document.createElement('span'); bdg.className = 'badge'; bdg.innerHTML = badgeSVG(COLORS[a.status] ? a.status : 'running', a.error); bdg.setAttribute('role', 'img'); bdg.setAttribute('aria-label', a.error ? LABEL.errored : LABEL[a.status] || LABEL.running); av.appendChild(bdg); }
    if (a.role === 'supervisor') {
      av.classList.add('avatar-hq'); av.title = 'Gtmux HQ'; mono.hidden = true;
      var mark = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      mark.setAttribute('viewBox', '0 0 100 100'); mark.setAttribute('aria-hidden', 'true'); mark.classList.add('hq-brand-mark');
      mark.innerHTML = '<rect x="8" y="8" width="40" height="40" rx="9" fill="currentColor"/><rect x="52" y="8" width="40" height="40" rx="9" fill="#06B6D4"/><rect x="8" y="52" width="84" height="40" rx="9" fill="currentColor"/>';
      av.appendChild(mark);
    }
    else if (a.agent) loadIcon(a.agent, img, mono);
    return av;
  }

  // userAvatarEl — the human's "人形电池 / person-battery" avatar (WEB.md §6): a
  // person inside a battery on a cyan gradient disc. Same mark as the mobile
  // UserAvatar. Static SVG (no user data) so innerHTML is safe; unique gradient id.
  var uaSeq = 0;
  function userAvatarEl(size) {
    var id = 'ua' + (++uaSeq), el = document.createElement('div'); el.className = 'user-avatar';
    if (size) { el.style.width = size + 'px'; el.style.height = size + 'px'; }
    el.innerHTML = '<svg viewBox="0 0 40 40" width="100%" height="100%">' +
      '<defs><linearGradient id="' + id + '" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#22D3EE"/><stop offset="1" stop-color="#0E7490"/></linearGradient></defs>' +
      '<circle cx="20" cy="20" r="20" fill="url(#' + id + ')"/>' +
      '<rect x="9" y="7" width="22" height="28" rx="4" stroke="#fff" stroke-width="2.4" fill="none"/>' +
      '<rect x="15.5" y="4" width="9" height="4" rx="1.5" fill="#fff"/>' +
      '<circle cx="20" cy="17" r="3.4" fill="#fff"/>' +
      '<path d="M13.5 30 Q13.5 23 20 23 Q26.5 23 26.5 30 Z" fill="#fff"/></svg>';
    return el;
  }

  // ---- chat time separators (chat-time-separator) -----------------------
  //
  // The mark where the conversation STOPPED and started again: the first turn with a
  // clock, a calendar day change, or a gap of SEP_GAP_MIN or more. Never once a minute —
  // a timestamp above nearly every turn marks nothing to a reader scrolling back.
  var SEP_GAP_MIN = 15;

  function sepTime(d) {
    var hm = String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0');
    var now = new Date();
    var sameDay = function (a, b) {
      return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
    };
    if (sameDay(d, now)) return T('Today ', '今天 ') + hm;
    var y = new Date(now); y.setDate(now.getDate() - 1);
    if (sameDay(d, y)) return T('Yesterday ', '昨天 ') + hm;
    if (ZH) {
      var md = (d.getMonth() + 1) + '月' + d.getDate() + '日';
      return (d.getFullYear() === now.getFullYear() ? md : d.getFullYear() + '年' + md) + ' ' + hm;
    }
    var mon = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'][d.getMonth()];
    return (d.getFullYear() === now.getFullYear()
      ? mon + ' ' + d.getDate() + ', ' + hm
      : mon + ' ' + d.getDate() + ' ' + d.getFullYear() + ', ' + hm);
  }

  // sepLabels mirrors the phone's time.separatorLabels: one entry per turn, '' for most.
  // A turn with no usable clock is INVISIBLE to it — it neither marks nor breaks the
  // comparison for the turns around it.
  function sepLabels(turns) {
    var out = turns.map(function () { return ''; });
    var prev = null;
    turns.forEach(function (t, i) {
      if (!t.time) return;
      var d = new Date(t.time);
      if (isNaN(d.getTime())) return;
      var sameDay = prev && d.getFullYear() === prev.getFullYear() &&
        d.getMonth() === prev.getMonth() && d.getDate() === prev.getDate();
      var gapped = prev && d.getTime() - prev.getTime() >= SEP_GAP_MIN * 60000;
      if (!prev || !sameDay || gapped) out[i] = sepTime(d);
      prev = d;
    });
    return out;
  }

  // The separator itself: the label centred between two wavy rules. The wave is a
  // repeating background rather than a measured SVG — the browser can tile it.
  function sepEl(label) {
    var row = document.createElement('div'); row.className = 'csep';
    var l = document.createElement('span'); l.className = 'csep-rule';
    var t = document.createElement('span'); t.className = 'csep-label'; t.textContent = label;
    var r = document.createElement('span'); r.className = 'csep-rule';
    row.appendChild(l); row.appendChild(t); row.appendChild(r);
    return row;
  }

  // senderAvatarEl — whose words a prompt is, when they are not the reader's.
  //
  // Every prompt wore the person-battery, because a session log records what ARRIVED in a
  // pane and never who caused it to arrive. gtmux performed the delivery and now says so
  // (`from` on a turn), so HQ's relay stops appearing under the reader's own face.
  // HQ is a role and wears a word; another session wears its agent's icon.
  function senderAvatarEl(from, size) {
    if (from.kind === 'agent') {
      var av = avatarEl({agent: from.agent || '', icon: !!from.agent}, size, false);
      av.title = from.label || '';
      return av;
    }
    var el = document.createElement('div');
    el.className = 'sender-hq';
    if (size) { el.style.width = size + 'px'; el.style.height = size + 'px'; }
    el.textContent = 'HQ';
    el.title = from.label || 'HQ';
    return el;
  }

  // senderNoteEl names the sender under the bubble, so the mark is not the only cue.
  function senderNoteEl(from) {
    var p = document.createElement('div');
    p.className = 'sender-note';
    p.textContent = from.kind === 'hq'
      ? T('from HQ', '来自 HQ')
      : T('from ' + (from.label || 'another session'), '来自 ' + (from.label || '其他会话'));
    return p;
  }

  var collapsedSections = new Set();
  try { JSON.parse(lsGet('gtmux.collapsedSections', '[]')).forEach(function (key) { if (['hq', 'waiting', 'errored', 'working', 'idle', 'native', 'external'].includes(key)) collapsedSections.add(key); }); } catch (e) {}
  function rowSummary(a, node) {
    var message = a.error && a.error_text || a.bg && a.bg_text || '';
    node.textContent = message || secondary(a); node.title = node.textContent;
    node.classList.toggle('row-alert', !!message);
  }
  function rosterSection(root, key, label, count, tree, className) {
    var section = el('section', className || 'roster-section');
    var button = el('button', (tree ? 'tree-group' : 'group-label') + ' section-toggle ' + key);
    button.type = 'button'; button.dataset.section = key;
    var id = (tree ? 'tree-' : 'radar-') + key + '-rows', body = el('div', 'section-rows'); body.id = id;
    button.setAttribute('aria-controls', id);
    // Match the App's CollapseBar headings without uppercasing inline status labels.
    button.appendChild(el('span', 'section-name', label.toUpperCase())); button.appendChild(el('span', 'section-count', String(count)));
    var action = el('span', 'section-action'), chevron = el('span', 'section-chevron'); chevron.innerHTML = uiIcon('chevron'); button.appendChild(action); button.appendChild(chevron);
    function sync() { var open = !collapsedSections.has(key); body.hidden = !open; button.setAttribute('aria-expanded', String(open)); action.textContent = open ? T('Hide', '收起') : T('Show', '展开'); }
    button.onclick = function () {
      if (collapsedSections.has(key)) collapsedSections.delete(key); else collapsedSections.add(key);
      try { localStorage.setItem('gtmux.collapsedSections', JSON.stringify(Array.from(collapsedSections))); } catch (e) {}
      sync(); lastSig = ''; // Other entry point must pick up the same preference on its next poll.
    };
    sync(); section.appendChild(button); section.appendChild(body); root.appendChild(section); return body;
  }

  // ---- radar ------------------------------------------------------------
  function rowEl(a) {
    var st = COLORS[a.status] ? a.status : 'running';
    var row = document.createElement('div');
    row.className = 'row' + (st === 'waiting' ? ' waiting' : '');

    var av = avatarEl(a, 0, true);

    var text = document.createElement('div'); text.className = 'text';
    var p = document.createElement('div'); p.className = 'primary'; p.textContent = primary(a);
    var s = document.createElement('div'); s.className = 'secondary'; s.textContent = secondary(a);
    rowSummary(a, s);
    text.appendChild(p); text.appendChild(s);

    var right = document.createElement('div'); right.className = 'right';
    // An unknown completion must not be dated by a window repaint.
    var t = relTime(a.since || (a.status !== 'idle' ? a.activity_at : 0));
    if (t) { var tm = document.createElement('span'); tm.className = 'time'; tm.textContent = t; right.appendChild(tm); }
    var ag = document.createElement('span'); ag.className = 'agent'; ag.textContent = a.agent || ''; ag.hidden = isNative(a); right.appendChild(ag);
    // Native (non-tmux) agents are SENSED read-only: no pane to open → a "native" tag
    // instead of the chevron, and no click target (mirrors the app/menu-bar).
    if (isNative(a)) {
      var nt = document.createElement('span'); nt.className = 'native-tag'; nt.textContent = readableConversation(a) ? T('Read only', '只读') : (a.current_client === 'unconfirmed' || a.client === 'chatgpt_desktop') ? (a.follow && a.follow.hq ? T('HQ following', 'HQ 跟进中') : T('Status only', '仅显示状态')) : 'native'; right.appendChild(nt);
      row.classList.add('native');
      if (readableConversation(a)) { row.onclick = function () { openConversation(a); }; row.tabIndex=0; row.setAttribute('role','button'); row.onkeydown=function(e){if(e.key==='Enter'||e.key===' '){e.preventDefault();openConversation(a);}}; }
    } else {
      var ch = document.createElement('span'); ch.className = 'chev'; ch.innerHTML = uiIcon('right'); right.appendChild(ch);
      row.onclick = function () { openAgent(a); };
      keyboardLink(row, row.onclick);
    }

    row.appendChild(av); row.appendChild(text); row.appendChild(right);
    return row;
  }

  function renderRadar(agents) {
    // only repaint when something actually changed (avoids list flicker every poll)
    var sig = JSON.stringify(agents.map(function (a) { return [a.pane_id, a.source, a.session_id, a.home_id, a.current_client, a.launch_host, a.client, a.follow, a.project, a.terminal, a.session, a.loc, a.agent, a.role, a.activity_at, a.status, a.task, a.since, a.icon, a.error, a.error_text, a.bg, a.bg_count, a.bg_text]; }));
    if (sig === lastSig) return;
    lastSig = sig;
    // tmux agents bucket by status; native (non-tmux) agents are SENSED read-only, so
    // they get their own "Elsewhere" section at the end (mirrors the app / menu-bar).
    var by = {waiting: [], errored: [], working: [], idle: []};
    var natives = [], desktops = [];
    agents.forEach(function (a) {
      if (isNative(a)) { ((a.current_client === 'unconfirmed' || a.client === 'chatgpt_desktop') ? desktops : natives).push(a); return; }
      // An idle turn that ended on a failure is not "finished" — it was sitting under a
      // green tick among the completed ones.
      if (a.status === 'idle' && a.error) { by.errored.push(a); return; }
      by[sectionStatus(a.status)].push(a);
    });
    var root = $('radar'); root.innerHTML = '';
    function section(key, label, list) {
      if (!list.length) return;
      var body = rosterSection(root, key, label, list.length, false);
      list.forEach(function (a) { body.appendChild(rowEl(a)); });
    }
    ORDER.forEach(function (st) { section(st, LABEL[st], by[st]); });
    section('native', T('Elsewhere', '不在 tmux'), natives);
    section('external', T('External conversations', '外部会话'), desktops);
    if (!root.children.length) { var e = document.createElement('div'); e.className = 'group-label'; e.textContent = emptyAgentsText(); root.appendChild(e); }
    if (selIdx >= 0) { selIdx = Math.min(selIdx, radarRows().length - 1); highlightSel(); }
  }

  // ---- desktop keyboard nav (j/k or ↑↓ select · Enter open · Esc back) -----
  function radarRows() { return Array.prototype.slice.call($('radar').querySelectorAll('.row')).filter(function (row) { return !row.closest('[hidden]'); }); }
  function highlightSel() {
    var rows = radarRows();
    rows.forEach(function (r, i) { r.classList.toggle('kbd-sel', i === selIdx); });
    if (selIdx >= 0 && rows[selIdx]) rows[selIdx].scrollIntoView({block: 'nearest'});
  }
  function moveSel(d) {
    var rows = radarRows(); if (!rows.length) return;
    selIdx = Math.max(0, Math.min(rows.length - 1, (selIdx < 0 ? 0 : selIdx) + d));
    highlightSel();
  }
  function setupKeyboard() {
    document.addEventListener('keydown', function (e) {
      if (e.defaultPrevented || e.isComposing || e.altKey) return;
      var tag = (e.target && e.target.tagName) || '';
      if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA') return;
      if (!$('workbench').hidden && (e.metaKey || e.ctrlKey) && !e.shiftKey && (e.key === 'k' || e.key === 'K') && !(e.target && e.target.closest && e.target.closest('[contenteditable]:not([contenteditable="false"]), [role="textbox"]'))) { e.preventDefault(); openCmdk(); return; }
      // Native controls own their activation keys; editable regions own typing.
      // Inspect ancestors too: focus can sit on a child inside a custom control.
      if (e.target && e.target.closest && e.target.closest('button, summary, .step-input, a[href], [role="button"], [role="textbox"], [contenteditable]:not([contenteditable="false"])')) return;
      if (!$('cmdk').hidden) { return; } // cmdk has its own input handler
      // Keep browser/OS combinations (especially Copy) out of plain-key navigation.
      // The workbench palette is the one explicit modified page shortcut.
      if (e.metaKey || e.ctrlKey) {
        if (!$('workbench').hidden && !e.shiftKey && (e.key === 'k' || e.key === 'K')) { e.preventDefault(); openCmdk(); }
        return;
      }
      if (!$('workbench').hidden) {
        if (e.key === 'Escape' && maxedTile) { e.preventDefault(); restoreBoard(); }
        else if (e.key === '/') { e.preventDefault(); $('rail-search').focus(); }
        else if (e.key === 'g') { e.preventDefault(); $('wb-snap').click(); }
        else if (e.key === 'f') { e.preventDefault(); focusTopTile(); }
        else if (e.key === '[') { e.preventDefault(); cyclePreset(-1); }
        else if (e.key === ']') { e.preventDefault(); cyclePreset(1); }
        else if (e.key >= '1' && e.key <= '9') { e.preventDefault(); var ti = WB.tiles[parseInt(e.key, 10) - 1]; if (ti) maximizeTile(ti); }
      } else if (!$('radar').hidden) {
        if (e.key === 'j' || e.key === 'ArrowDown') { e.preventDefault(); moveSel(1); }
        else if (e.key === 'k' || e.key === 'ArrowUp') { e.preventDefault(); moveSel(-1); }
        else if (e.key === 'Enter') { var rows = radarRows(); if (rows[selIdx]) { e.preventDefault(); rows[selIdx].click(); } }
        else if (e.key === 'p') { e.preventDefault(); openPanes(); }
      } else if (!$('panes').hidden) {
        if (e.key === 'Escape') { e.preventDefault(); startRadar(); }
        else if (e.key === '/') { e.preventDefault(); $('panes-search').focus(); }
      } else if (!$('pane').hidden || !$('chat').hidden) {
        var inChat = !$('chat').hidden;
        if (e.key === 'Escape') { e.preventDefault(); $('back').click(); }
        else if (e.key === 't') { e.preventDefault(); setMode('term'); }
        // in chat: c = collapse all step groups (mockup §03); in term: c = switch to chat.
        else if (e.key === 'c') { e.preventDefault(); if (inChat) collapseAllSteps(); else setMode('chat'); }
        // in chat: j/k walks the turn outline; in term: j/k cycles panes.
        else if (e.key === 'j' || e.key === 'ArrowDown') { e.preventDefault(); inChat ? selectTurn(curTurnIdx + 1, true) : cyclePane(1); }
        else if (e.key === 'k' || e.key === 'ArrowUp') { e.preventDefault(); inChat ? selectTurn(curTurnIdx - 1, true) : cyclePane(-1); }
      }
    });
  }

  function pollRadar() {
    runRead('radar', function (signal) {
      var current = currentRead('radar', signal);
      return api('/api/agents', {signal: signal}).then(function (r) {
        if (!current()) return null;
        if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
        if (!r.ok) throw new Error('agents'); rememberServerClock(r); return r.json();
      }).then(function (agents) { if (!agents || !current(true)) return; setConn(true); lastAgents = agents; renderRadar(agents); fetchShare(); })
        .catch(function () { if (current()) setConn(false); });
    }, 15000, function () { setConn(false); });
  }

  function startRadar() {
    show('radar'); $('bar').hidden = false; $('back').hidden = true; $('mode').hidden = true; hideFocusChrome(); $('title').textContent = 'gtmux'; $('sub').textContent = '';
    $('gear').hidden = false;
    $('panes-btn').hidden = false; // the Browse-all-panes entry lives on the radar only
    curPane = null; curAgent = null; lastSig = '';
    clearInterval(paneTimer); paneTimer = null; clearInterval(chatTimer); chatTimer = null;
    clearInterval(panesTimer); panesTimer = null;
    pollRadar(); clearInterval(radarTimer); radarTimer = setInterval(pollRadar, 2000);
  }

  // ---- all-panes browser (tiered-pane-control) --------------------------
  // The radar is agent-first; THIS lists EVERY tmux pane (agent + plain) grouped
  // session → window, so a pane in a session with no coding agent is one tap away.
  // Tapping any row opens it in the same pane mirror as an agent (view + type where
  // allowed). Fed by GET /api/panes (guest-scoped server-side, so no leak).
  // panesRows is null until a read of the list lands: "no panes" is a statement about the
  // Mac, and before the first read nothing is known. panesFailed: the last read failed.
  var panesTimer = null, panesRows = null, panesSig = '', panesFailed = false;
  // Folded sessions, by name, kept across openings in this browser.
  var PANES_FOLD_KEY = 'gtmux.panes.folded';
  var panesFolded = (function () {
    try { var v = JSON.parse(localStorage.getItem(PANES_FOLD_KEY) || '[]'); return Array.isArray(v) ? v : []; } catch (e) { return []; }
  })();
  function setFolded(names) {
    panesFolded = names;
    try { localStorage.setItem(PANES_FOLD_KEY, JSON.stringify(names)); } catch (e) {}
    panesSig = ''; renderPanes();
  }
  // Adapt a PaneRow → the shape openAgent/pollPane consume. A plain pane invents no
  // agent status (status 'running' = the neutral bucket); its label follows the same plainLabel chain as the roster.
  function isHQPane(p) {
    if (!p || p.tier && p.tier !== 'agent') return false;
    var live = p.pane_id && byId(lastAgents, p.pane_id);
    return (p.role == null ? (live && live.role) : p.role) === 'supervisor';
  }
  function paneSessionTitle(session, hq) {
    return hq && session.trim().toLowerCase() === 'hq' ? 'Gtmux HQ' : session;
  }
  function paneToAgent(p) {
    return {
      pane_id: p.pane_id, session: p.session, window: p.window, pane: p.pane, loc: p.loc,
      win_id: p.win_id || '', win_name: p.win_name || '',
      agent: p.tier === 'agent' ? (p.agent || '') : '', status: 'running',
      task: isHQPane(p) ? 'Gtmux HQ' : p.tier === 'agent' ? (p.title || p.command) : plainLabel(p), source: 'tmux', project: p.cwd,
      role: isHQPane(p) ? 'supervisor' : undefined,
      // an agent pane shows its OFFICIAL icon (avatarEl → loadIcon via /api/icon); a
      // plain pane has none → the $_ monogram.
      icon: p.tier === 'agent' ? p.icon : undefined,
    };
  }
  function openPanes() {
    show('panes'); $('bar').hidden = true;
    clearInterval(radarTimer); radarTimer = null;
    clearInterval(paneTimer); paneTimer = null; clearInterval(chatTimer); chatTimer = null;
    $('panes-search').value = '';
    panesSig = ''; panesRows = null; panesFailed = false;
    renderPanes(); // the reading state, at once
    pollPanes(); clearInterval(panesTimer); panesTimer = setInterval(pollPanes, 3000);
    $('panes-search').focus();
  }
  function pollPanes() {
    runRead('panes', function (signal) {
      var current = currentRead('panes', signal);
      return Promise.all([
      // The radar stops polling while this view is up, and an agent row's status and task
      // come from it, so they are read here too: a pane that started waiting after the
      // browser opened used to keep the status it had when the radar stopped.
      api('/api/agents', {signal: signal}).then(function (r) { return r.ok ? r.json() : null; })
        .then(function (agents) { if (current() && Array.isArray(agents)) { lastAgents = agents; renderPanes(); } })
        .catch(function () {}),
      api('/api/panes', {signal: signal}).then(function (r) {
        if (!current()) return null;
        if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
        if (!r.ok) throw new Error('panes'); return r.json();
      }).then(function (rows) {
        if (!rows || !current(true)) return;
        if (!Array.isArray(rows)) throw new Error('panes');
        setConn(true); panesRows = rows; panesFailed = false; renderPanes(); fetchShare();
      }).catch(function () { if (current()) { setConn(false); panesFailed = true; renderPanes(); } })
      ]);
    }, 15000, function () { setConn(false); panesFailed = true; renderPanes(); });
  }
  // The live radar status of an agent-tier row ('' when the radar has not listed it).

  function paneStatus(p) {
    if (p.tier !== 'agent') return '';
    var live = byId(lastAgents, p.pane_id);
    return live && COLORS[live.status] ? live.status : '';
  }
  function renderPanes() {
    var q = ($('panes-search').value || '').trim().toLowerCase();
    var root = $('panes-list');
    if (panesRows === null) {
      // Nothing has landed yet: say so, never "no panes" or a zero. A failed first read
      // is not an empty Mac either (the same rule as the phone's browser).
      var pending = 'pending|' + panesFailed;
      if (panesSig === pending) return;
      panesSig = pending;
      $('panes-count').textContent = panesFailed ? T('Unable to load', '加载失败') : T('reading…', '正在读取…');
      $('panes-fold-all').hidden = true;
      root.innerHTML = '';
      if (panesFailed) {
        var f = document.createElement('div'); f.className = 'pb-empty pb-failed';
        var f1 = document.createElement('div'); f1.textContent = T('Unable to load panes', '无法加载窗格列表');
        var f2 = document.createElement('div'); f2.className = 'pb-hint'; f2.textContent = T('Reconnecting automatically…', '正在自动重试…');
        f.appendChild(f1); f.appendChild(f2); root.appendChild(f);
      } else {
        root.appendChild(brandLoaderEl(T('Reading the panes on this server…', '正在读取这台服务器上的窗格…')));
      }
      return;
    }
    var match = function (p) {
      // The IDs are searchable: `%23` is what the tab title, `gtmux focus %23` and HQ
      // all use, so it is what someone types — and it was not in the haystack at all.
      return !q || [p.session, p.window, p.command, p.title, p.cwd, p.agent, p.loc,
                    p.pane_id, p.win_id, p.win_name, isHQPane(p) ? 'Gtmux HQ' : '']
        .some(function (v) { return String(v || '').toLowerCase().indexOf(q) !== -1; });
    };
    // group session → (first-seen order), preserving pane order within
    var order = [], byS = {}, shown = 0;
    panesRows.forEach(function (p) {
      if (!match(p)) return;
      shown++;
      if (!byS[p.session]) { byS[p.session] = []; order.push(p.session); }
      byS[p.session].push(p);
    });
    var sig = q + '|' + panesFailed + '|' + panesFolded.join('\u0000') + '|' + JSON.stringify(panesRows.map(function (p) {
      var live = byId(lastAgents, p.pane_id);
      return [p.pane_id, p.tier, p.agent, p.title, p.command, p.active, p.cwd, p.session, p.win_id, p.win_name, isHQPane(p),
        paneStatus(p), live ? live.task : ''];
    }));
    if (sig === panesSig) return; // avoid repaint (+ losing focus) every poll
    panesSig = sig;
    var needYou = panesRows.filter(function (p) { return paneStatus(p) === 'waiting'; }).length;
    var count = $('panes-count'); count.innerHTML = '';
    var ct = document.createElement('span');
    ct.textContent = (q ? shown + '/' + panesRows.length : String(panesRows.length)) +
      T(' panes · ', ' 个窗格 · ') + order.length + T(' sessions', ' 个会话') +
      (panesFailed ? T(' · not refreshed', ' · 刷新失败') : '');
    count.appendChild(ct);
    if (needYou) {
      var ny = document.createElement('span'); ny.className = 'pb-needyou';
      ny.textContent = T(' · ' + needYou + ' need attention', ' · ' + needYou + ' 个待处理');
      count.appendChild(ny);
    }
    var allFolded = order.length > 0 && order.every(function (s) { return panesFolded.indexOf(s) >= 0; });
    var fa = $('panes-fold-all');
    fa.hidden = order.length === 0;
    fa.textContent = allFolded ? T('Expand all', '展开全部') : T('Collapse all', '折叠全部');
    fa.onclick = function () {
      setFolded(allFolded ? panesFolded.filter(function (s) { return order.indexOf(s) < 0; })
        : panesFolded.concat(order.filter(function (s) { return panesFolded.indexOf(s) < 0; })));
    };
    root.innerHTML = '';
    if (!order.length) {
      var e = document.createElement('div'); e.className = 'pb-empty';
      e.textContent = q ? T('No matching panes', '未找到匹配的窗格')
        : T('No panes. Open a tmux pane on the server to begin.', '暂无窗格。在服务器上新建 tmux 窗格后即可查看。');
      root.appendChild(e); return;
    }
    order.forEach(function (sess) {
      var list = byS[sess];
      var nAgent = list.filter(function (p) { return p.tier === 'agent'; }).length;
      // A search shows what matches inside a folded session: the reader asked for it.
      var folded = !q && panesFolded.indexOf(sess) >= 0;
      var roll = {waiting: 0, working: 0, idle: 0, running: 0};
      list.forEach(function (p) { var st = paneStatus(p); if (roll[st] != null) roll[st]++; });
      var hd = document.createElement('div'); hd.className = 'pb-session' + (folded ? ' folded' : '');
      hd.setAttribute('role', 'button');
      hd.setAttribute('aria-expanded', String(!folded));
      hd.onclick = function () {
        setFolded(panesFolded.indexOf(sess) >= 0 ? panesFolded.filter(function (s) { return s !== sess; }) : panesFolded.concat([sess]));
      };
      // Every window id, so the session says what it holds at a glance. Capped, so a
      // session with many windows cannot push its own name off the row.
      var wids = [];
      list.forEach(function (p) { if (p.win_id && wids.indexOf(p.win_id) < 0) wids.push(p.win_id); });
      var widLabel = wids.length === 0 ? '' :
        wids.length <= 6 ? wids.join(' ') : wids.slice(0, 5).join(' ') + ' +' + (wids.length - 5);
      var hq = panesRows.some(function (p) { return p.session === sess && isHQPane(p); });
      // The rollup stays on the header when the group is folded: folding must not hide
      // that something inside is waiting on the reader.
      // Every non-zero agent state, running included: its rows carry the grey badge, and
      // a header that left it out undercounted the session (%12's review of b8503f6b).
      var pips = ['waiting', 'working', 'idle', 'running'].filter(function (st) { return roll[st]; }).map(function (st) {
        return '<span class="pb-pip" style="color:' + COLORS[st] + '">' + badgeSVG(st) + roll[st] + '</span>';
      }).join('');
      hd.innerHTML = '<span class="pb-chev-fold" aria-hidden="true">' + (folded ? '▸' : '▾') + '</span>' +
        '<span class="pb-sname">' + esc(paneSessionTitle(sess, hq)) + '</span>' +
        (hq ? '<span class="pb-hq">HQ</span>' : '') +
        (widLabel ? '<span class="pb-swins">' + esc(widLabel) + '</span>' : '') +
        '<span class="pb-smeta">' + list.length +
        (nAgent ? ' · ' + nAgent + T(nAgent === 1 ? ' agent' : ' agents', ' 个 agent') : '') + '</span>' + pips;
      root.appendChild(hd);
      if (folded) return;
      // Divide on the STABLE window id, not the index: on a real fleet most of a
      // server's windows sit at index 0, so an index-keyed divider merges windows that
      // have nothing to do with each other. The label leads with the id because the id
      // is the anchor — the name drifts with `automatic-rename` and can be shared.
      //
      // A single-window session gets no divider: the levels coincide there, and the id
      // rides on the session header instead.
      var wins = [];
      list.forEach(function (p) {
        var k = p.win_id || 'idx:' + p.window;
        var w = wins.filter(function (x) { return x.k === k; })[0];
        if (!w) { w = {k: k, id: p.win_id || '', name: p.win_name || '', rows: []}; wins.push(w); }
        w.rows.push(p);
      });
      // EVERY session draws its window bands, including one holding a single window: the
      // band used to be conditional, so a single-window session put its panes at the same
      // indent that elsewhere means "window" and the same shape meant two things.
      wins.forEach(function (w) {
        {
          var wl = document.createElement('div'); wl.className = 'pb-win';
          wl.innerHTML = (w.id ? '<span class="pb-win-id">' + esc(w.id) + '</span>' : '') +
            (w.name ? '<span class="pb-win-name">' + esc(w.name) + '</span>' : '') +
            '<span class="pb-win-n">' + w.rows.length + '</span>';
          root.appendChild(wl);
        }
        w.rows.forEach(function (p) { root.appendChild(paneRowEl(p)); });
      });
    });
  }
  // Label for a PLAIN pane — a shell, an editor, a log tail — for a reader choosing
  // between rows.
  //
  // Ending at the command made every shell on a machine read "bash": true, and it
  // identifies nothing. The order is "what does this say that the previous did not", and
  // it is the SAME chain the other two surfaces use (macapp PaneLabels.plain, mobile
  // api/types paneLabel). Three copies of one rule is exactly how these drifted before,
  // so DESIGN §16 carries the chain and says all three move together.
  //
  // A title that is a whole path is not a name: many shells set the title to the cwd,
  // sometimes colon-prefixed, and the later rules produce its meaningful part anyway.
  function plainLabel(p) {
    var cmd = String((p && p.command) || '').trim();
    var t = String((p && p.title) || '').replace(/^:+\s*/, '').trim();
    if (t && t !== cmd && t.charAt(0) !== '/' && t.charAt(0) !== '~') return t;
    // tmux's automatic-rename writes the command into the window name.
    var win = String((p && p.win_name) || '').trim();
    if (win && win !== cmd) return win;
    // The repo it sits in: stable across its subdirectories, and how people refer to a
    // shell out loud.
    var project = String((p && p.project) || '').trim();
    if (project) return project;
    var cwd = String((p && p.cwd) || '').replace(/\/+$/, '');
    var leaf = cwd.slice(cwd.lastIndexOf('/') + 1);
    if (leaf) return leaf;
    return cmd;
  }
  // The command a pane id turns into — same shape as the menu-bar's `PaneCommands.focus`
  // and the mobile `paneFocusCommand`. Surfacing `%N` is only worth it if the token means
  // the same thing on every surface.
  function paneFocusCommand(id) { return 'gtmux focus ' + id; }

  // navigator.clipboard needs a SECURE context. The web surface is reached over the
  // https tunnel (fine) but also over a plain `http://<lan-ip>:8765`, where the API is
  // simply absent — so the old execCommand path is the fallback, not a legacy leftover.
  function copyText(s) {
    try {
      if (navigator.clipboard && navigator.clipboard.writeText)
        return Promise.resolve(navigator.clipboard.writeText(s)).then(function () { return true; }, function () { return legacyCopy(s); });
    } catch (e) { /* Some browsers throw before returning a promise. */ }
    return Promise.resolve(legacyCopy(s));
  }
  function legacyCopy(s) {
    var active = document.activeElement, selection = window.getSelection && window.getSelection();
    var ranges = [], start = active && active.selectionStart, end = active && active.selectionEnd;
    if (selection) for (var i = 0; i < selection.rangeCount; i++) ranges.push(selection.getRangeAt(i).cloneRange());
    var ta = document.createElement('textarea');
    ta.value = s; ta.setAttribute('readonly', ''); ta.style.position = 'fixed'; ta.style.opacity = '0';
    document.body.appendChild(ta); ta.select();
    var ok = false;
    try { ok = document.execCommand('copy'); } catch (e) { /* Nothing else to try. */ }
    document.body.removeChild(ta);
    if (active && active.isConnected) {
      active.focus({preventScroll: true});
      if (typeof start === 'number' && active.setSelectionRange) active.setSelectionRange(start, end);
    }
    if (selection && ranges.length) { selection.removeAllRanges(); ranges.forEach(function (r) { selection.addRange(r); }); }
    return !!ok;
  }

  function paneRowEl(p) {
    var a = paneToAgent(p), isAgent = p.tier === 'agent';
    // An agent row leads with what it is DOING (from the live radar join), not its
    // NAME, which repeats down the whole list while the distinguishing text hides in the
    // sub-line. The avatar already carries identity.
    var live = byId(lastAgents, p.pane_id);
    var task = (live && live.task ? String(live.task) : '').trim();
    var hq = isHQPane(p);
    var label = hq ? 'Gtmux HQ' : isAgent ? (task || p.agent || p.command) : plainLabel(p);
    var row = document.createElement('div'); row.className = 'pb-row';
    row.appendChild(avatarEl(a, 30, false));
    var tx = document.createElement('div'); tx.className = 'pb-text';
    var nm = document.createElement('div'); nm.className = 'pb-name';
    var nt = document.createElement('span'); nt.className = 'pb-nm-text'; nt.textContent = label; nm.appendChild(nt);
    if (hq) { var badge = document.createElement('span'); badge.className = 'pb-hq'; badge.textContent = 'HQ'; nm.appendChild(badge); }
    // An agent row shows its REAL status from the radar join (the triple-encoded badge);
    // it used to carry an "on radar" tag, which said nothing about a pane waiting on you.
    var st = paneStatus(p);
    if (st) {
      var sb = document.createElement('span'); sb.className = 'pb-status'; sb.innerHTML = badgeSVG(st);
      sb.setAttribute('aria-label', LABEL[st] || LABEL.running);
      nm.appendChild(sb);
    }
    if (p.active) { var ad = document.createElement('span'); ad.className = 'pb-active'; nm.appendChild(ad); }
    var sub = document.createElement('div'); sub.className = 'pb-sub';
    // The PANE ID, not the `w.p` coordinate: stable, unique, and the token every other
    // surface names. The coordinate changes whenever panes are reordered.
    var dir = (p.cwd || '').replace(/\/+$/, '').split('/').pop() || '';
    // The id is its own element, not part of the joined text, because it is CLICKABLE:
    // it copies `gtmux focus %N`, turning a token you can read into one you can run.
    // The click stops there — the row itself still opens the pane.
    var pid = document.createElement('button'); pid.type = 'button'; pid.className = 'pb-pid'; pid.textContent = p.pane_id;
    pid.title = T('Copy ', '复制 ') + paneFocusCommand(p.pane_id);
    pid.setAttribute('aria-label', pid.title);
    pid.onclick = function (e) {
      e.stopPropagation();
      copyText(paneFocusCommand(p.pane_id)).then(function (ok) {
        pid.textContent = ok ? T('Copied', '已复制') : T('Copy failed', '复制失败'); pid.classList.toggle('ok', ok);
        setTimeout(function () { pid.textContent = p.pane_id; pid.classList.remove('ok'); }, 1600);
      });
    };
    sub.appendChild(pid);
    // No agent name here — the icon at the head of the row already says it.
    var bits = [];
    if (dir) bits.push(dir);
    if (!isAgent && p.command && p.command !== label) bits.push(p.command);
    if (bits.length) sub.appendChild(document.createTextNode('  ·  ' + bits.join('  ·  ')));
    tx.appendChild(nm); tx.appendChild(sub); row.appendChild(tx);
    var ch = document.createElement('span'); ch.className = 'pb-chev'; ch.innerHTML = uiIcon('right'); row.appendChild(ch);
    row.onclick = function () { var live = byId(lastAgents, p.pane_id); openAgent(live || a); };
    keyboardLink(row, row.onclick);
    return row;
  }

  // ---- pane mirror ------------------------------------------------------
  // Build an xterm theme from /api/theme (the user's real terminal), falling back
  // to the GHOSTTY defaults. palette[0..15] → the 16 named xterm color keys.
  var PKEYS = ['black','red','green','yellow','blue','magenta','cyan','white','brightBlack','brightRed','brightGreen','brightYellow','brightBlue','brightMagenta','brightCyan','brightWhite'];
  function xtermTheme() {
    var t = theme || {};
    var o = {
      background: t.background || GHOSTTY.bg,
      foreground: t.foreground || GHOSTTY.fg,
      cursor: t.cursor || GHOSTTY.cursor,
      selectionBackground: GHOSTTY.sel,
    };
    if (t.palette) { for (var i = 0; i < 16 && i < t.palette.length; i++) { if (t.palette[i]) o[PKEYS[i]] = t.palette[i]; } }
    return o;
  }
  function normFont(s) { return s.toLowerCase().replace(/[\s_-]/g, ''); }
  // resolveFont: an explicit picker choice wins; 'system' = SF Mono; 'auto' = the
  // terminal's font mapped to a bundled family (else default chain).
  function resolveFont() {
    if (fontPref === 'system') return 'ui-monospace, Menlo, Monaco, monospace';
    if (fontPref && fontPref !== 'auto') return '"' + fontPref + '", ' + GHOSTTY.font;
    if (theme && theme.fontFamily) {
      var n = normFont(theme.fontFamily), m = null;
      for (var i = 0; i < BUNDLED.length; i++) { if (normFont(BUNDLED[i]) === n) { m = BUNDLED[i]; break; } }
      return '"' + (m || theme.fontFamily) + '", ' + GHOSTTY.font;
    }
    return GHOSTTY.font;
  }
  function termSize() { return sizePref || (theme && theme.fontSize) || 14; }
  function applyAppearance() {
    document.documentElement.style.setProperty('--chat-text-size', chatSizePref + 'px');
    var preview = document.querySelector('.terminal-font-preview'); if (preview) { preview.style.fontFamily = resolveFont(); preview.style.fontSize = termSize() + 'px'; }
    document.documentElement.style.setProperty('--terminal-bg', (theme && theme.background) || GHOSTTY.bg);
    if (term) {
      term.options.theme = xtermTheme();
      term.options.fontFamily = resolveFont();
      term.options.fontSize = termSize();
      try { fit.fit(); } catch (e) {}
    }
    // tiles each own a smaller xterm — keep them in step with the picker.
    if (WB && WB.tiles) WB.tiles.forEach(function (t) {
      if (!t.term) return;
      t.term.options.theme = xtermTheme();
      t.term.options.fontFamily = resolveFont();
      t.term.options.fontSize = termSize();
      try { t.fit.fit(); } catch (e) {}
    });
  }
  // Browser chrome follows its own appearance; terminal colors remain authentic.
  function applyWebAppearance() {
    var dark = appearancePref === 'dark' || (appearancePref === 'system' && systemAppearance && systemAppearance.matches);
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
    document.querySelectorAll('#appearance-options input').forEach(function (radio) { radio.checked = radio.value === appearancePref; });
  }
  function setupWebAppearance() {
    systemAppearance = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
    applyWebAppearance();
    if (systemAppearance) {
      var changed = function () { if (appearancePref === 'system') applyWebAppearance(); };
      if (systemAppearance.addEventListener) systemAppearance.addEventListener('change', changed);
      else if (systemAppearance.addListener) systemAppearance.addListener(changed);
    }
  }
  function fetchTheme() {
    return api('/api/theme').then(function (r) { return r.ok ? r.json() : null; }).then(function (j) {
      if (!j) return;
      theme = j; applyAppearance(); syncSettings();
    }).catch(function () {});
  }

  function ensureTerm() {
    if (term) return;
    term = new Terminal({
      convertEol: true, cursorBlink: false, disableStdin: true, cursorInactiveStyle: 'none',
      scrollback: 5000, allowProposedApi: true,
      fontFamily: resolveFont(), fontSize: termSize(),
      theme: xtermTheme(),
    });
    fit = new FitAddon.FitAddon(); term.loadAddon(fit);
    try { var u = new Unicode11Addon.Unicode11Addon(); term.loadAddon(u); term.unicode.activeVersion = '11'; } catch (e) {}
    term.open($('term'));
    // Hold repaints while the reader is actively scrolling (wheel/drag), so a poll
    // mid-gesture doesn't yank the view; flush the held frame once they settle.
    var el = $('term');
    var mark = function () {
      userScrolling = true; clearTimeout(scrollIdle);
      scrollIdle = setTimeout(function () {
        userScrolling = false;
        if (pendingText !== null) { var t = pendingText; pendingText = null; writePane(t); }
      }, 900);
    };
    el.addEventListener('wheel', mark, {passive: true});
    el.addEventListener('touchmove', mark, {passive: true});
    term.onScroll(function () { updateJump(false); }); // §02 jump-to-latest pill
    // On window resize: refit cols/rows, then re-render the current frame so long
    // lines re-wrap to the new browser width (dynamic 折行), debounced.
    window.addEventListener('resize', function () {
      clearTimeout(resizeIdle);
      resizeIdle = setTimeout(function () {
        try { refitTerm(); } catch (e) {} // the bar may have rewrapped: place the terminal under it, then fit
        try { fit.fit(); } catch (e) {}
        if (lastText) { var t = lastText; lastText = ''; writePane(t); }
      }, 120);
    });
  }
  // ---- Codex's pinned prompt --------------------------------------------------------
  // A JavaScript copy of mobileapp/src/ui/codexPinned.ts, rules unchanged. Codex pins the
  // prompt of the turn on screen to row 0 as ONE row: "› ", the prompt with its newlines
  // joined, cut at the pane's width and ended with "…". Codex runs in the alternate screen,
  // so the rest of the prompt is not in the capture; the conversation log has it. Both copies
  // run mobileapp/src/ui/codexPinnedCases.json, and app.test.cjs checks that the width tables
  // below are the phone's, so neither copy can change alone.
  var PIN_MARK = '\u203A ', PIN_CUT = '\u2026', PIN_MIN_HEAD = 6, PIN_RECENT = 10, PIN_SLACK = 2;
  var PIN_REFRESH_MS = 4000; // the terminal's log fetch while a cut row is unexplained

  // Cell widths as tmux draws them (term.ts WIDE_SYMBOLS / NARROW_EMOJI, measured on tmux 3.7b).
  var WIDE_SYMBOLS = [
  [0x231a, 0x231b], [0x2329, 0x232a], [0x23e9, 0x23ec], [0x23f0, 0x23f0], [0x23f3, 0x23f3],
  [0x25fd, 0x25fe], [0x2614, 0x2615], [0x261d, 0x261d], [0x2630, 0x2637], [0x2648, 0x2653],
  [0x267f, 0x267f], [0x268a, 0x268f], [0x2693, 0x2693], [0x26a1, 0x26a1], [0x26aa, 0x26ab],
  [0x26bd, 0x26be], [0x26c4, 0x26c5], [0x26ce, 0x26ce], [0x26d4, 0x26d4], [0x26ea, 0x26ea],
  [0x26f2, 0x26f3], [0x26f5, 0x26f5], [0x26f9, 0x26fa], [0x26fd, 0x26fd], [0x2705, 0x2705],
  [0x270a, 0x270d], [0x2728, 0x2728], [0x274c, 0x274c], [0x274e, 0x274e], [0x2753, 0x2755],
  [0x2757, 0x2757], [0x2795, 0x2797], [0x27b0, 0x27b0], [0x27bf, 0x27bf], [0x2b1b, 0x2b1c],
  [0x2b50, 0x2b50], [0x2b55, 0x2b55], [0x1f004, 0x1f004], [0x1f0cf, 0x1f0cf], [0x1f18e, 0x1f18e],
  [0x1f191, 0x1f19a], [0x1f200, 0x1f202], [0x1f210, 0x1f23b], [0x1f240, 0x1f248],
  [0x1f250, 0x1f251], [0x1f260, 0x1f265],
  ];
  var NARROW_EMOJI = [
  [0x1f321, 0x1f32c], [0x1f336, 0x1f336], [0x1f37d, 0x1f37d], [0x1f394, 0x1f39f],
  [0x1f3cd, 0x1f3ce], [0x1f3d4, 0x1f3df], [0x1f3f1, 0x1f3f3], [0x1f3f5, 0x1f3f7],
  [0x1f43f, 0x1f43f], [0x1f441, 0x1f441], [0x1f4fd, 0x1f4fe], [0x1f53e, 0x1f54a],
  [0x1f54f, 0x1f54f], [0x1f568, 0x1f573], [0x1f576, 0x1f579], [0x1f57b, 0x1f58f],
  [0x1f591, 0x1f594], [0x1f597, 0x1f5a3], [0x1f5a5, 0x1f5fa], [0x1f650, 0x1f67f],
  [0x1f6c6, 0x1f6cb], [0x1f6cd, 0x1f6cf], [0x1f6d3, 0x1f6d4], [0x1f6e0, 0x1f6ea],
  [0x1f6f0, 0x1f6f3], [0x1f700, 0x1f7d9], [0x1f800, 0x1f80b],
  [0x1f810, 0x1f847], [0x1f850, 0x1f859], [0x1f860, 0x1f887], [0x1f890, 0x1f8ad],
  [0x1f8b0, 0x1f8bb], [0x1f8c0, 0x1f8c1], [0x1f8d0, 0x1f8d8], [0x1f900, 0x1f90b], [0x1f93b, 0x1f93b], [0x1f946, 0x1f946],
  [0x1fa00, 0x1fa57], [0x1fa60, 0x1fa6d],
  ];
  function inRanges(cp, t) {
    var lo = 0, hi = t.length - 1;
    while (lo <= hi) {
      var mid = Math.floor((lo + hi) / 2);
      if (cp < t[mid][0]) hi = mid - 1;
      else if (cp > t[mid][1]) lo = mid + 1;
      else return true;
    }
    return false;
  }
  // charCells is term.ts charCells: 2 for CJK and the emoji tmux draws wide, 0 for variation
  // selectors and ZWJ, else 1.
  function charCells(cp) {
    if (cp === 0xfe0e || cp === 0xfe0f || cp === 0x200d) return 0;
    if ((cp >= 0x1100 && cp <= 0x115f) || (cp >= 0x2e80 && cp <= 0x303e) || (cp >= 0x3041 && cp <= 0x33ff) ||
        (cp >= 0x3400 && cp <= 0x4dbf) || (cp >= 0x4e00 && cp <= 0x9fff) || (cp >= 0xa000 && cp <= 0xa4cf) ||
        (cp >= 0xac00 && cp <= 0xd7a3) || (cp >= 0xf900 && cp <= 0xfaff) || (cp >= 0xfe30 && cp <= 0xfe4f) ||
        (cp >= 0xff00 && cp <= 0xff60) || (cp >= 0xffe0 && cp <= 0xffe6) ||
        (cp >= 0x1f300 && cp <= 0x1faff && !inRanges(cp, NARROW_EMOJI)) ||
        (cp >= 0x20000 && cp <= 0x3fffd) || inRanges(cp, WIDE_SYMBOLS)) return 2;
    return 1;
  }
  function cellsOf(s) {
    var n = 0;
    for (var i = 0; i < s.length; i++) {
      var cp = s.codePointAt(i);
      if (cp > 0xffff) i++;
      n += charCells(cp);
    }
    return n;
  }
  // The text of one captured row, escape sequences removed (the capture keeps SGR colours).
  function plainRow(line) {
    return String(line || '').replace(/\x1b\[[0-?]*[ -\/]*[@-~]/g, '').replace(/\x1b\][^\x07\x1b]*(\x07|\x1b\\)/g, '');
  }
  // Whitespace-free and without variation selectors, so a prompt matches however Codex joined it.
  function pinSkeleton(s) { return String(s).replace(/[\s\uFE0E\uFE0F]+/g, ''); }

  // codexCutRow is the row's shape before any prompt is consulted: Codex's pane, row 0 a
  // "› " row ending in "…" at the pane's right edge, the composer's "› " further down.
  function codexCutRow(text, agent, cols) {
    if (String(agent || '').trim().toLowerCase() !== 'codex' || !text || !cols) return null;
    var lines = text.split('\n');
    var first = plainRow(lines[0]).replace(/\s+$/, '');
    if (first.indexOf(PIN_MARK) !== 0 || first.slice(-PIN_CUT.length) !== PIN_CUT) return null;
    var w = cellsOf(first);
    if (w > cols || w < cols - PIN_SLACK) return null;
    var head = pinSkeleton(first.slice(PIN_MARK.length, -PIN_CUT.length));
    if (head.length < PIN_MIN_HEAD) return null;
    var rest = lines.slice(1);
    if (!rest.some(function (l) { return plainRow(l).indexOf(PIN_MARK) === 0; })) return null;
    return {head: head, next: pinSkeleton(plainRow(lines[1] || '')), rest: rest};
  }
  // splitCodexPinned returns {text, prompt} — the capture without the row and the full prompt
  // — when exactly one of the recent logged prompts explains the row; else null, and the
  // capture is shown as it is.
  function splitCodexPinned(text, agent, prompts, cols) {
    var row = codexCutRow(text, agent, cols);
    if (!row) return null;
    var recent = (prompts || []).filter(function (q) { return q && String(q).trim(); }).slice(-PIN_RECENT).reverse();
    var found = null, seen = {}, n = 0;
    for (var i = 0; i < recent.length; i++) {
      var full = pinSkeleton(recent[i]);
      if (full.indexOf(row.head) !== 0 || full === row.head + PIN_CUT || full.length <= row.head.length) continue;
      if (row.next && full.indexOf(row.head + PIN_CUT + row.next) === 0) return null;
      if (!seen[full]) { seen[full] = true; n++; }
      if (n > 1) return null;
      if (found === null) found = String(recent[i]).trim();
    }
    return found === null ? null : {text: row.rest.join('\n'), prompt: found};
  }

  // The bar above the single-pane terminal. Fed by renderPane; the prompts come from the
  // conversation log, fetched while a cut row is on screen and unexplained.
  var pinPrompts = [], pinFetchedAt = 0, pinFetching = false, pinShown = '', lastRaw = '', paneCols = 0, pinEtag = '';
  function resetPin() {
    pinPrompts = []; pinFetchedAt = 0; pinShown = ''; lastRaw = ''; paneCols = 0; pinEtag = '';
    showPin(null);
  }
  function fetchPinPrompts() {
    if (pinFetching || !curPane || Date.now() - pinFetchedAt < PIN_REFRESH_MS) return;
    pinFetching = true; pinFetchedAt = Date.now();
    var pane = curPane;
    // Conditional on the last ETag: an unchanged log answers 304 with no body, as on the phone.
    api('/api/transcript?id=' + encodeURIComponent(pane), pinEtag ? {headers: {'If-None-Match': pinEtag}} : {})
      .then(function (r) {
        if (!r.ok) return null; // a 304 is not ok: nothing new
        var tag = r.headers && r.headers.get && r.headers.get('ETag');
        if (tag && pane === curPane) pinEtag = tag;
        return r.json();
      })
      .then(function (turns) {
        if (pane !== curPane || !turns) return;
        pinPrompts = turns.map(function (t) { return t.prompt || ''; });
        if (lastRaw) renderPane(lastRaw, paneCols);
      })
      .catch(function () {})
      .then(function () { pinFetching = false; });
  }
  function showPin(prompt) {
    var bar = $('pinned');
    if (!bar) return;
    if (!prompt) {
      if (!bar.hidden) { bar.hidden = true; $('pane').classList.remove('has-pin'); pinShown = ''; refitTerm(); }
      return;
    }
    if (prompt === pinShown && !bar.hidden) return;
    pinShown = prompt;
    bar.classList.remove('open');
    bar.querySelector('.pin-body').textContent = prompt;
    var flat = prompt.replace(/\s+/g, ' ');
    var chars = Array.from(flat);
    var toggle = bar.querySelector('.pin-toggle');
    toggle.setAttribute('aria-expanded', 'false');
    toggle.setAttribute('aria-label', T("This turn's prompt: ", '本轮提示：') +
      (chars.length > 160 ? chars.slice(0, 160).join('') + '\u2026' : flat));
    bar.hidden = false; $('pane').classList.add('has-pin');
    refitTerm();
  }
  // The bar takes room from the terminal: its height moves the terminal down, then xterm refits.
  function refitTerm() {
    var bar = $('pinned');
    var h = bar && !bar.hidden ? bar.offsetHeight : 0;
    $('term').style.top = (49 + h) + 'px';
    try { if (fit) fit.fit(); } catch (e) {}
  }
  function setupPin() {
    var bar = $('pinned');
    if (!bar) return;
    var copy = bar.querySelector('.pin-copy');
    var toggleBtn = bar.querySelector('.pin-toggle');
    copy.innerHTML = uiIcon('copy'); copy.title = T('Copy prompt', '复制本轮消息'); copy.setAttribute('aria-label', copy.title);
    toggleBtn.innerHTML = uiIcon('chevron');
    // Keyboard and screen readers use the two buttons, each with its own native Enter and
    // Space; no key handler sits on the bar to swallow theirs (#1290 W1).
    var toggle = function () {
      bar.classList.toggle('open');
      toggleBtn.setAttribute('aria-expanded', bar.classList.contains('open') ? 'true' : 'false');
      refitTerm();
    };
    bar.addEventListener('click', toggle);
    toggleBtn.addEventListener('click', function (e) { e.stopPropagation(); toggle(); });
    // The bar's height follows its text, which rewraps when the window narrows: the terminal
    // must move with it, or the bar covers its top rows (#1290 W2).
    if (typeof ResizeObserver !== 'undefined') {
      new ResizeObserver(function () { if (!bar.hidden) refitTerm(); }).observe(bar);
    }
    copy.addEventListener('click', function (e) {
      e.stopPropagation();
      copyWithFeedback(copy, pinShown, T('Copy prompt', '复制本轮消息'), 'copy', bar.querySelector('.pin-status'));
    });
  }
  // renderPane takes a fresh capture: the cut row, when recognised, goes to the bar and the
  // rest to the terminal; anything else reaches the terminal as captured.
  function renderPane(text, cols) {
    lastRaw = text || ''; if (cols) paneCols = cols;
    var agent = curAgent && curAgent.agent;
    var pin = paneMode === 'chat' ? null : splitCodexPinned(lastRaw, agent, pinPrompts, paneCols);
    showPin(pin && pin.prompt);
    if (!pin && paneMode !== 'chat' && codexCutRow(lastRaw, agent, paneCols)) fetchPinPrompts();
    writePane(pin ? pin.text : lastRaw);
  }

  function normalize(t) { return t.indexOf('⏺') === -1 ? t : t.split('⏺').join('●'); }

  // Incremental write that preserves the reader's position (matches the mobile
  // xterm bridge). Append-only growth writes just the new tail (no reset → no
  // flash, scroll untouched). A full change (TUI redraw / scrolled-off) repaints
  // but keeps the reader's DISTANCE FROM THE BOTTOM, so a manual scroll-up isn't
  // snapped back to the bottom every poll.
  function writePane(text) {
    text = normalize(text || '');
    if (text === lastText) return;
    if (userScrolling) { pendingText = text; return; }
    var prev = lastText; lastText = text;
    if (prev && text.length > prev.length && text.lastIndexOf(prev, 0) === 0) {
      term.write(text.slice(prev.length), function () { updateJump(true); });
      return;
    }
    var b = term.buffer.active;
    var wasBottom = b.viewportY >= b.baseY;
    var fromBottom = b.baseY - b.viewportY;
    term.reset();
    term.write(text, function () {
      var nb = term.buffer.active;
      if (wasBottom) term.scrollToBottom();
      else { try { term.scrollToLine(Math.max(0, nb.baseY - fromBottom)); } catch (e) {} }
      updateJump(true);
    });
  }

  // Branded loading placeholder (the gtmux 2×2 mark + a pulse) shown over a pane/
  // chat/tile before its first frame arrives — so opening a session reads as
  // "loading…", not a long black screen (mirrors the phone's BrandLoader).
  function brandLoaderEl(label) {
    var w = document.createElement('div'); w.className = 'brandload';
    var m = document.createElement('div'); m.className = 'brandload-mark';
    m.innerHTML = '<i></i><i class="lit"></i><i class="wide"></i>';
    w.appendChild(m);
    if (label) { var l = document.createElement('div'); l.className = 'brandload-label'; l.textContent = label; w.appendChild(l); }
    return w;
  }
  function showPaneLoader() {
    hidePaneLoader();
    var el = brandLoaderEl(T('Loading the screen…', '正在加载屏幕…')); el.id = 'pane-load';
    $('pane').appendChild(el);
  }
  function hidePaneLoader() { var e = $('pane-load'); if (e) e.remove(); }
  function showChatLoader() {
    hideChatLoader();
    var el = brandLoaderEl(T('Loading the conversation…', '正在加载对话…')); el.id = 'chat-load';
    $('chat').appendChild(el);
  }
  function hideChatLoader() { var e = $('chat-load'); if (e) e.remove(); }

  // --- shared input (web-shared-input) -----------------------------------
  // The server gate is authoritative; this only mirrors it. GET /api/share tells us,
  // for THIS caller, whether input is on and which panes are allowed (or all, for the
  // owner). We show the input bar ONLY for an allowed pane; a blocked send still 403s.
  var SHARE = {input: false, all: false, stale: false, panes: {}, viewCount: 0, typeCount: 0};
  function shareStaleText() {
    return T('Share access has expired. Ask the owner to grant access again.', '分享授权已失效，请联系所有者重新授权。');
  }
  function emptyAgentsText() {
    if (SHARE.known && !SHARE.all) return SHARE.stale ? shareStaleText() : T('No shared agents to view yet.', '还没有可查看的共享 agent。');
    return T('no agents yet · start one in a tmux pane', '还没有 agent · 在 tmux 的 pane 里启动一个');
  }
  function fetchShare() {
    runRead('share', function (signal) {
      var current = currentRead('share', signal);
      return api('/api/share', {signal: signal}).then(function (r) { return r && r.ok ? r.json() : null; }).then(function (j) {
        if (!j || !current(true)) return;
        var previous = JSON.stringify(SHARE);
        SHARE = {input: !!j.input, all: !!j.all, stale: !!j.stale, panes: {},
                 viewCount: (j.view_panes || []).length, typeCount: (j.panes || []).length};
        (j.panes || []).forEach(function (p) { SHARE.panes[p] = true; });
        SHARE.known = true;
        updateInputBar();
        updateGuestScopeStrip();
        updateIdentity();
        if (previous !== JSON.stringify(SHARE)) {
          lastSig = ''; renderRadar(lastAgents); renderTree(WB.agents);
          if (!$('cmdk').hidden) renderCmdk($('cmdk-input').value);
        }
      }).catch(function () {});
    });
  }

  // Identity chips (WEB §11): owner and guest top bars must read differently —
  // owner = 全权 (full, every pane typable), guest = 协作视图 (this link's scope).
  function updateIdentity() {
    ['identity', 'wb-identity'].forEach(function (id) {
      var el = document.getElementById(id); if (!el) return;
      el.hidden = !SHARE.known;
      if (!SHARE.known) return;
      el.classList.toggle('id-owner', !!SHARE.all);
      el.textContent = SHARE.all ? ((location.hostname === '127.0.0.1' || location.hostname === 'localhost') ? T('local workspace', '本机工作台') : T('full access · ', '全权 · ') + (location.hostname || 'local')) : T('shared view · guest', '协作视图 · 访客');
    });
  }

  // "Your access" strip (pair-share-model S4): a GUEST sees what this link grants —
  // N sessions visible, M typable — so the page never feels arbitrarily empty. The
  // owner (all:true) shows nothing.
  function updateGuestScopeStrip() {
    var el = document.getElementById('guest-scope');
    if (!el) {
      el = document.createElement('div');
      el.id = 'guest-scope';
      document.body.insertBefore(el, document.body.firstChild);
    }
    if (SHARE.all) { el.hidden = true; return; }
    el.hidden = false;
    if (SHARE.stale) { el.textContent = shareStaleText(); return; }
    // Both halves spelled out rather than assembled from T() fragments: the counts sit
    // INSIDE the sentence, and the two languages put them in different places.
    el.textContent = ZH
      ? '协作视图 · 访客 · 可查看 ' + SHARE.viewCount + ' 个会话 · 可输入 ' + SHARE.typeCount + ' 个。分享者可随时撤销访问。'
      : 'shared view · guest · ' + SHARE.viewCount + ' visible · ' + SHARE.typeCount + ' open for input. The person who shared the link can revoke access at any time.';
  }
  function paneCanInput(id) { return !!id && SHARE.input && (SHARE.all || (!SHARE.stale && !!SHARE.panes[id])); }
  // The composer makes input capability visible. Only read-only needs a label.
  function setCapChip(el, can) {
    if (!el) return;
    el.classList.toggle('cap-in', can);
    el.classList.toggle('cap-ro', !can);
    el.innerHTML = can ? '' : uiIcon('eye') + T('Read-only', '只读');
  }
  var paneComposer = null;
  function updateInputBar() {
    var bar = $('pane-input'); if (!bar) return;
    var inPaneTerm = !$('pane').hidden && paneMode === 'term';
    var inChat = !$('chat').hidden;
    var can = paneCanInput(curPane);
    var inPaneChat = !!curPane && inChat && paneMode === 'chat';
    var chatInput = $('chat-input');
    if (chatInput) {
      chatInput.hidden = !(inChat && paneMode === 'chat' && SHARE.known);
      var note = $('chat-ro');
      if (note) note.hidden = can || !SHARE.known;
    }
    bar.hidden = !(inPaneTerm && can);
    if (paneComposer) {
      var mount = inPaneChat ? chatInput : bar;
      if (paneComposer.el.parentNode !== mount) mount.appendChild(paneComposer.el);
      if (paneComposer.sync) paneComposer.sync((inPaneTerm || inPaneChat) && can);
      paneComposer.el.hidden = !can;
    }
    renderChatInteraction();
    // read-only panes state WHY there is no input row (one line, no empty textbox)
    var ro = $('pane-ro'); if (ro) ro.hidden = !(inPaneTerm && SHARE.known && !can);
    $('pane').classList.toggle('has-input', inPaneTerm && SHARE.known);
    // Read-only is explicit; the composer itself identifies writable panes.
    var cap = $('cap');
    if (cap) {
      var focused = !!curPane && (!$('pane').hidden || !$('chat').hidden);
      cap.hidden = !(focused && SHARE.known && !can);
      setCapChip(cap, can);
    }
    if (WB && WB.tiles) WB.tiles.forEach(tileInputSync); // the workbench tiles too
  }
  // postSend is the shared /api/send call used by BOTH the single-pane bar and the
  // workbench tiles. The server gate (guest consent + allowlist) is authoritative.
  function postSend(id, body) {
    body.id = id;
    if (!body.send_id) body.send_id = newSendId();
    return api('/api/send', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)});
  }
  function newSendId() { return window.crypto && window.crypto.randomUUID ? window.crypto.randomUUID() : Date.now().toString(36) + '-' + Math.random().toString(36).slice(2); }
  function sendPane(body) {
    var pin = $('pin');
    return postSend(curPane, body).then(function (r) {
      if (!r) return;
      if (r.status === 403) { if (pin) { pin.value = ''; pin.placeholder = T('Input is not allowed in this pane', '此窗格不允许输入'); } return; }
      if (r.status === 401) { token = null; try { localStorage.removeItem(TOKEN_KEY); } catch (e) {} gate('expired'); return; }
      if (!r.ok) return;
      return r.json();
    }).then(function (j) { if (j && typeof j.text === 'string') { renderPane(j.text, j.cols); hidePaneLoader(); } });
  }
  // tileInputSync shows a tile's input row iff it is in term mode and the host allowed
  // this pane — and keeps the head's ⌨/👁 capability chip + the read-only note in
  // sync; tileSendThen renders the post-send echo into the tile's own terminal.
  function tileInputSync(t) {
    if (!t) return;
    var can = paneCanInput(t.id);
    if (t.inputEl) t.inputEl.hidden = !((t.mode === 'chat' || t.mode === 'term') && can);
    if (t.composer && !t.inputEl.hidden) t.composer.sync();
    if (t.roEl) t.roEl.hidden = !((t.mode === 'chat' || t.mode === 'term') && SHARE.known && !can);
    if (t.capEl) { t.capEl.hidden = !SHARE.known || can; setCapChip(t.capEl, can); }
  }
  function tileSendThen(t, pin) {
    return function (r) {
      if (!r) return;
      if (r.status === 403) { if (pin) { pin.value = ''; pin.placeholder = T('Input not allowed', '不允许输入'); } return; }
      if (!r.ok) return;
      r.json().then(function (j) { if (j && typeof j.text === 'string' && t.term) tileWrite(t, j.text); });
    };
  }

  function pollPane() {
    runRead('pane', function (signal) {
      if (!curPane) return;
      var current = currentRead('pane', signal);
      return api('/api/pane?id=' + encodeURIComponent(curPane), {signal: signal}).then(function (r) {
        if (!current()) return null;
        if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
        if (!r.ok) throw new Error('pane'); return r.json();
      }).then(function (j) { if (!j || !current(true)) return; setConn(true); renderPane(j.text, j.cols); hidePaneLoader(); })
        .catch(function () { if (current()) setConn(false); });
    }, 15000, function () { setConn(false); });
  }

  // makeComposer builds the shared web composer (WEB §11, aligned with the mobile app's
  // Composer): a multiline auto-grow textarea (⏎ send · ⇧⏎/⌥⏎ newline), an attach button
  // that uploads an image/file via /api/upload and inserts its saved path, a send button,
  // and a subtle key strip (⇥/esc/^C/↑/↓) on its OWN row below — so the keys no longer
  // crowd the text field. `getId()` yields the target pane; `echo(text)` renders the
  // post-send screen. `small` = the compact workbench-tile variant.

  function annotateImage(attachment, context, onChange) {
    if (context.revoked) return;
    var sheet = browserDialog(T('Annotate image', '标注图片')); sheet.el.classList.add('image-editor');
    var toolbar = document.createElement('div'); toolbar.className = 'markup-tools'; sheet.body.appendChild(toolbar);
    var canvas = document.createElement('canvas'); canvas.className = 'markup-canvas'; canvas.setAttribute('aria-label', T('Image annotation canvas', '图片标注画布')); sheet.body.appendChild(canvas);
    var message = document.createElement('p'); message.className = 'markup-note'; message.setAttribute('role', 'status'); message.textContent = T('Draw on the image, then choose Done.', '在图片上拖动画出标记，完成后保存。'); sheet.body.appendChild(message);
    var image = new Image(), strokes = [], current = null, tool = 'pen', color = '#ef4444', loaded = false;
    function button(icon, label, action) { var b = document.createElement('button'); b.type = 'button'; b.className = 'markup-tool'; b.innerHTML = uiIcon(icon); b.title = label; b.setAttribute('aria-label', label); b.onclick = action; toolbar.appendChild(b); return b; }
    var tools = [];
    [['pen', T('Draw', '画笔')], ['arrow', T('Arrow', '箭头')], ['square', T('Box', '方框')], ['redact', T('Redact', '打码')]].forEach(function (item) { var b = button(item[0], item[1], function () { tool = item[0]; tools.forEach(function (entry) { entry.button.setAttribute('aria-pressed', String(entry.tool === tool)); }); }); tools.push({button: b, tool: item[0]}); b.setAttribute('aria-pressed', String(item[0] === tool)); });
    var colors = document.createElement('input'); colors.type = 'color'; colors.value = color; colors.setAttribute('aria-label', T('Annotation color', '标注颜色')); colors.oninput = function () { color = colors.value; }; toolbar.appendChild(colors);
    button('undo', T('Undo', '撤销'), function () { strokes.pop(); draw(); });
    button('clear', T('Clear marks', '清除标注'), function () { strokes = []; current = null; draw(); });
    function line(ctx, stroke) {
      var from = stroke.points[0], to = stroke.points[stroke.points.length - 1]; ctx.strokeStyle = stroke.color; ctx.lineWidth = Math.max(3, Math.max(canvas.width, canvas.height) / 250); ctx.lineCap = 'round'; ctx.lineJoin = 'round'; ctx.beginPath();
      if (stroke.tool === 'redact') { ctx.fillStyle = '#000000'; ctx.fillRect(Math.min(from.x, to.x), Math.min(from.y, to.y), Math.abs(to.x - from.x), Math.abs(to.y - from.y)); return; }
      if (stroke.tool === 'square') ctx.rect(from.x, from.y, to.x - from.x, to.y - from.y);
      else { ctx.moveTo(from.x, from.y); stroke.points.slice(1).forEach(function (p) { ctx.lineTo(p.x, p.y); }); }
      ctx.stroke();
      if (stroke.tool === 'arrow') { var angle = Math.atan2(to.y - from.y, to.x - from.x), size = ctx.lineWidth * 5; ctx.beginPath(); ctx.moveTo(to.x - size * Math.cos(angle - Math.PI / 6), to.y - size * Math.sin(angle - Math.PI / 6)); ctx.lineTo(to.x, to.y); ctx.lineTo(to.x - size * Math.cos(angle + Math.PI / 6), to.y - size * Math.sin(angle + Math.PI / 6)); ctx.stroke(); }
    }
    function draw() { if (!loaded) return; var ctx = canvas.getContext('2d'); ctx.fillStyle = '#ffffff'; ctx.fillRect(0, 0, canvas.width, canvas.height); ctx.drawImage(image, 0, 0, canvas.width, canvas.height); strokes.forEach(function (s) { line(ctx, s); }); if (current) line(ctx, current); }
    function point(e) { var r = canvas.getBoundingClientRect(); return {x: (e.clientX - r.left) * canvas.width / r.width, y: (e.clientY - r.top) * canvas.height / r.height}; }
    canvas.onpointerdown = function (e) { if (!loaded) return; e.preventDefault(); canvas.setPointerCapture(e.pointerId); current = {tool: tool, color: color, points: [point(e)]}; };
    canvas.onpointermove = function (e) { if (!current) return; var p = point(e); if (current.tool === 'pen') current.points.push(p); else current.points = [current.points[0], p]; draw(); };
    canvas.onpointerup = function () { if (current) { strokes.push(current); current = null; draw(); } }; canvas.onpointercancel = function () { current = null; draw(); };
    sheet.foot.hidden = false; var original = document.createElement('button'); original.type = 'button'; original.className = 'dialog-secondary'; original.textContent = T('Use original', '使用原图'); original.onclick = sheet.close; sheet.foot.appendChild(original);
    var scale = document.createElement('button'); scale.type = 'button'; scale.className = 'dialog-secondary'; scale.textContent = T('Scale to 4096 px', '缩小至 4096 像素'); scale.hidden = true; sheet.foot.appendChild(scale);
    scale.onclick = function () { var ratio = 4096 / Math.max(canvas.width, canvas.height); if (ratio >= 1) return; strokes.forEach(function (stroke) { stroke.points.forEach(function (point) { point.x *= ratio; point.y *= ratio; }); }); canvas.width = Math.round(canvas.width * ratio); canvas.height = Math.round(canvas.height * ratio); loaded = true; done.disabled = false; scale.hidden = true; message.textContent = T('Export size: ', '导出尺寸：') + canvas.width + ' × ' + canvas.height; draw(); };
    function exportFailure() { message.textContent = T('Export failed. Your marks are preserved. Retry, use the original, or choose a smaller export.', '导出失败，标注已保留。可重试、使用原图，或选择缩小尺寸。'); scale.hidden = Math.max(canvas.width, canvas.height) <= 4096; done.disabled = false; }
    var done = document.createElement('button'); done.type = 'button'; done.className = 'dialog-primary'; done.textContent = T('Done', '完成'); done.disabled = true; sheet.foot.appendChild(done);
    done.onclick = function () {
      if (!strokes.length) { sheet.close(); return; } done.disabled = true;
      try { canvas.toBlob(function (blob) {
        if (context.revoked) return;
        if (!blob || blob.size > 30 * 1024 * 1024) { exportFailure(); return; }
        var name = attachment.name.replace(/(-marked)?\.[^.]+$/, '') + '-marked.jpg';
        if (attachment.url) { URL.revokeObjectURL(attachment.url); context.urls.delete(attachment.url); }
        attachment.file = new File([blob], name, {type: 'image/jpeg'}); attachment.name = name; attachment.url = URL.createObjectURL(attachment.file); context.urls.add(attachment.url); delete attachment.path; attachment.progress = null; onChange(); sheet.close();
      }, 'image/jpeg', .92); } catch (e) { exportFailure(); }
    };
    image.onload = function () { if (context.revoked) return; canvas.width = image.naturalWidth; canvas.height = image.naturalHeight; loaded = true; done.disabled = false; try { draw(); message.textContent = T('Original size: ', '原始尺寸：') + canvas.width + ' × ' + canvas.height + T(' · JPEG export', ' · JPEG 导出'); } catch (e) { exportFailure(); } };
    image.onerror = function () { message.textContent = T('This image cannot be previewed here. You can still send the original file.', '此图片无法预览，仍可发送原文件。'); }; image.src = attachment.url;
  }

  var composeCache = null, composeAttachmentSeq = 0;
  function composeNamespace() {
    var one = 2166136261, two = 5381, value = token || 'anonymous';
    for (var i = 0; i < value.length; i++) { one = Math.imul(one ^ value.charCodeAt(i), 16777619); two = Math.imul(two, 33) ^ value.charCodeAt(i); }
    return 'gtmux.compose:' + BASE + ':' + (one >>> 0).toString(36) + (two >>> 0).toString(36);
  }
  function composeData() {
    var key = composeNamespace();
    if (composeCache && composeCache.key === key && !composeCache.revoked) return composeCache;
    var saved = {}; try { saved = JSON.parse(localStorage.getItem(key) || '{}'); } catch (e) {}
    if (!saved || typeof saved !== 'object' || Array.isArray(saved)) saved = {};
    var data = {key: key, credential: token, drafts: Object.create(null), history: Object.create(null), recent: [], snippets: [], urls: new Set(), requests: new Set(), timer: null, revoked: false};
    Object.keys(saved.drafts || {}).forEach(function (id) { var d = saved.drafts[id]; if (d && typeof d.text === 'string') data.drafts[id] = {text: d.text, files: [], version: 0, lastAttempt: d.attempt && typeof d.attempt.id === 'string' && typeof d.attempt.payload === 'string' ? d.attempt : null, error: [typeof d.error === 'string' ? d.error : '', d.fileCount ? T('Attachments need to be selected again after reloading.', '页面已重新加载，请重新选择附件。') : ''].filter(Boolean).join(' ')}; });
    Object.keys(saved.history || {}).forEach(function (scope) { if (Array.isArray(saved.history[scope])) data.history[scope] = saved.history[scope].filter(function (s) { return typeof s === 'string'; }).slice(0, 50); });
    data.recent = Array.isArray(saved.recent) ? saved.recent.filter(function (s) { return typeof s === 'string'; }).slice(0, 30) : [];
    data.snippets = Array.isArray(saved.snippets) ? saved.snippets.filter(function (s) { return typeof s === 'string'; }).slice(0, 30) : [];
    composeCache = data; return data;
  }
  function saveCompose(data, immediate) {
    if (!data || data.revoked) return; clearTimeout(data.timer);
    function write() {
      if (data.revoked) return;
      var drafts = Object.create(null); Object.keys(data.drafts).slice(-50).forEach(function (id) { var d = data.drafts[id]; if (d.text || d.files.length) drafts[id] = {text: d.text, fileCount: d.files.length, attempt: d.lastAttempt, error: d.error}; });
      try { localStorage.setItem(data.key, JSON.stringify({drafts: drafts, history: data.history, recent: data.recent, snippets: data.snippets})); } catch (e) {}
    }
    if (immediate) write(); else data.timer = setTimeout(write, 400);
  }
  function clearComposeData() {
    var data = composeCache;
    if (data) {
      data.revoked = true; clearTimeout(data.timer); data.requests.forEach(function (r) { r.abort(); }); data.urls.forEach(function (url) { URL.revokeObjectURL(url); }); data.urls.clear();
      Object.keys(data.drafts).forEach(function (id) { var draft = data.drafts[id]; draft.text = ''; draft.files = []; draft.recovery = null; });
    }
    try { var prefix = 'gtmux.compose:' + BASE + ':'; for (var i = localStorage.length - 1; i >= 0; i--) { var key = localStorage.key(i); if (key && key.indexOf(prefix) === 0) localStorage.removeItem(key); } } catch (e) {}
    composeCache = null;
  }

  function composeScope(id) { var a = byId(lastAgents, id) || (curAgent && curAgent.pane_id === id ? curAgent : {}); return a.project || a.session || (a.loc || '').split(':')[0] || id || 'default'; }
  function composeDraft(data, id) { return data.drafts[id] || (data.drafts[id] = {text: '', files: [], version: 0, error: ''}); }

  function makeComposer(getId, echo, small, onAction) {
    echo = echo || function () {};
    var generation = 0, target = null, data = null, draft = null, localVersion = -1, appliedText = '', noteSignature = '', editor = null;
    var inputMedia = window.matchMedia && window.matchMedia('(pointer: coarse)');
    var compact = !!(inputMedia && inputMedia.matches), expanded = !compact;
    var root = document.createElement('div'); root.className = 'composer' + (small ? ' composer-sm' : '');
    var keys = document.createElement('div'); keys.className = 'cx-keys'; keys.setAttribute('aria-label', T('Terminal keys and input tools', '终端按键与输入工具'));
    var terminalKeyboard = window.GtmuxTerminalKeys ? window.GtmuxTerminalKeys.create({zh: ZH, getId: getId, canInput: function () { return paneCanInput(getId()); }, request: api, send: postSend, echo: echo, onAction: onAction}) : null;
    if (terminalKeyboard) { keys.appendChild(terminalKeyboard.button); root.appendChild(terminalKeyboard.notice); }
    var attachments = document.createElement('div'); attachments.className = 'cx-attachments'; attachments.hidden = true;
    var row = document.createElement('div'); row.className = 'cx-row';
    var file = document.createElement('input'), pickerContext = null; file.type = 'file'; file.multiple = true; file.hidden = true;
    var attach = iconButton('plus', T('Add attachment', '添加附件'), 'cx-attach');
    var ta = document.createElement('textarea'); ta.className = 'cx-input'; ta.rows = 1; ta.placeholder = T('Message or command…', '输入消息或命令…'); ta.setAttribute('aria-label', T('Message or command', '消息或命令')); ta.autocomplete = 'off'; ta.spellcheck = false;
    var expand = iconButton('expand', T('Expand editor', '展开编辑'), 'cx-expand');
    var send = iconButton('send', T('Send', '发送'), 'cx-send'); send.title = T('Send · ⌘/Ctrl+Enter', '发送 · ⌘/Ctrl+Enter');
    var keyboard = iconButton('keyboard', T('Show input', '展开输入框'), 'cx-keyboard'); keys.appendChild(keyboard);
    var remoteKeys = [];
    [['Tab', 'Tab', 'Tab'], ['Up', null, T('Up arrow', '方向键上'), 'up'], ['Down', null, T('Down arrow', '方向键下'), 'down'], ['Enter', null, T('Return key', '回车键'), 'return'], ['BSpace', null, T('Backspace', '退格键'), 'backspace'], ['C-c', 'Ctrl C', 'Ctrl-C'], ['Escape', 'Esc', 'Esc']].forEach(function (k) {
      var b = document.createElement('button'); b.type = 'button'; b.className = 'cx-control' + (k[3] ? ' cx-symbol' : ''); b.dataset.key = k[0];
      if (k[3]) b.innerHTML = uiIcon(k[3]); else b.textContent = k[1]; b.title = k[2]; b.setAttribute('aria-label', k[2]);
      b.onclick = function (e) {
        e.stopPropagation(); sync(); if (!draft || draft.busy || terminalKeyboard && terminalKeyboard.blocked()) return; if (terminalKeyboard && ['Tab', 'Escape', 'C-c', 'Up', 'Down'].includes(k[0]) && terminalKeyboard.legacy(k[0])) return;
        var recipient = target, currentData = data, currentGeneration = generation;
        postSend(recipient, {key: k[0]}).then(function (r) {
          if (!valid(currentData, currentGeneration)) return;
          if (r.status === 401) { token = null; gate('expired'); return; }
          if (!r.ok) { failure(recipient, currentData, T('Key could not be sent.', '按键未能发送。')); return; }
          b.dataset.state = 'done'; clearTimeout(b.ackTimer); b.ackTimer = setTimeout(function () { delete b.dataset.state; }, 750);
        }, function () { if (valid(currentData, currentGeneration)) failure(recipient, currentData, T('Key could not be sent.', '按键未能发送。')); });
      }; keys.appendChild(b); remoteKeys.push(b);
    });
    var snippets = iconButton('snippets', T('Quick replies', '常用语'), 'cx-snippets'); var snippetsLabel = document.createElement('span'); snippetsLabel.textContent = T('Quick replies', '常用语'); snippets.appendChild(snippetsLabel); keys.appendChild(snippets);
    var history = iconButton('history', T('Input history', '输入历史'), 'cx-history'); keys.appendChild(history);
    var note = document.createElement('div'); note.className = 'cx-note'; note.hidden = true; note.setAttribute('role', 'status'); note.setAttribute('aria-live', 'polite');
    row.appendChild(attach); row.appendChild(file); row.appendChild(ta); row.appendChild(expand); row.appendChild(send);
    root.appendChild(keys); root.appendChild(attachments); root.appendChild(row); root.appendChild(note);
    function iconButton(icon, label, cls) { var b = document.createElement('button'); b.type = 'button'; b.className = cls; b.innerHTML = uiIcon(icon); b.title = label; b.setAttribute('aria-label', label); return b; }
    function valid(context, version) { return !context.revoked && context.credential === token && generation === version; }
    function changed(d, context) { d.version++; saveCompose(context || data); }
    function remember() { if (!draft || data.revoked) return; if (draft.text !== ta.value) draft.lastAttempt = null; draft.text = ta.value; appliedText = ta.value; changed(draft); localVersion = draft.version; }
    function sync(visible) {
      if (terminalKeyboard) terminalKeyboard.sync(!!(visible !== false && root.isConnected && paneCanInput(getId())));
      var next = getId(), context = composeData();
      if (data && draft && !data.revoked && ta.value !== appliedText) remember();
      if (next !== target || context !== data) { data = context; target = next; draft = composeDraft(data, next || 'default'); localVersion = -1; noteSignature = ''; }
      if (draft.version !== localVersion) { ta.value = draft.text; appliedText = ta.value; localVersion = draft.version; renderAttachments(); }
      update(); grow();
    }
    function grow() { if (editor) return; ta.style.height = 'auto'; ta.style.height = Math.min(132, Math.max(40, ta.scrollHeight)) + 'px'; }
    function update() {
      var busy = !!(draft && draft.busy), canSend = !!(draft && (ta.value.trim() || draft.files.length));
      row.hidden = !expanded && !editor; keyboard.setAttribute('aria-expanded', String(expanded)); keyboard.title = expanded ? T('Hide input', '收起输入框') : T('Show input', '展开输入框'); keyboard.setAttribute('aria-label', keyboard.title);
      attach.disabled = expand.disabled = busy; ta.readOnly = busy; send.disabled = busy || !canSend; snippets.disabled = history.disabled = busy; remoteKeys.forEach(function (b) { b.disabled = busy; });
      send.innerHTML = uiIcon(busy ? 'loading' : 'send'); send.classList.toggle('sending', busy); send.classList.toggle('ready', canSend && !busy);
      ta.setAttribute('enterkeyhint', lsGet('gtmux.returnSends', '0') === '1' ? 'send' : 'enter');
      var message = draft && (draft.error || draft.info) || '', signature = message + (draft && draft.recovery ? ':recover' : '');
      if (signature !== noteSignature) {
        noteSignature = signature; note.textContent = message; note.hidden = !message; note.className = 'cx-note' + (draft && !draft.error ? ' info' : '');
        if (draft && draft.recovery) { var restore = document.createElement('button'); restore.type = 'button'; restore.className = 'cx-recover'; restore.textContent = T('Recover previous input', '恢复上次输入'); restore.onclick = function () { var recovery = draft.recovery; draft.text = [ta.value, recovery.text].filter(Boolean).join('\n'); draft.files = draft.files.concat(recovery.files); draft.recovery = null; changed(draft); sync(); }; note.appendChild(restore); }
      }
      if (editor && editor.sendButton) editor.sendButton.disabled = send.disabled;
    }
    function focusInput() { expanded = true; update(); ta.focus(); }
    keyboard.onclick = function () { expanded = !expanded; update(); if (expanded) ta.focus(); else ta.blur(); };
    ta.addEventListener('input', function () { remember(); if (draft) { draft.error = ''; draft.info = ''; } grow(); update(); });
    ta.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && !e.isComposing && e.keyCode !== 229 && !e.repeat && ((e.metaKey || e.ctrlKey) || (!editor && lsGet('gtmux.returnSends', '0') === '1' && !e.shiftKey && !e.altKey))) { e.preventDefault(); submit(); }
      e.stopPropagation();
    });
    ta.addEventListener('paste', function (e) { var files = e.clipboardData && Array.from(e.clipboardData.files || []); if (files && files.length) { e.preventDefault(); stage(files, true); } });
    root.addEventListener('dragover', function (e) { if (e.dataTransfer && Array.from(e.dataTransfer.types).indexOf('Files') !== -1) { e.preventDefault(); root.classList.add('dragging-file'); } });
    root.addEventListener('dragleave', function () { root.classList.remove('dragging-file'); });
    root.addEventListener('drop', function (e) { root.classList.remove('dragging-file'); if (e.dataTransfer && e.dataTransfer.files.length) { e.preventDefault(); stage(Array.from(e.dataTransfer.files), true); } });
    send.onclick = function (e) { e.stopPropagation(); submit(); };
    function failure(id, context, message, snapshot) {
      if (context.revoked) return; var d = composeDraft(context, id); d.busy = false; d.error = message; d.info = '';
      clearTimeout(d.infoTimer);
      if (id === target && ta.value !== appliedText) { d.text = ta.value; appliedText = ta.value; }
      if (snapshot) {
        if (!d.text && !d.files.length) { d.text = snapshot.text; d.files = snapshot.files; }
        else d.recovery = snapshot;
      }
      changed(d, context); if (context === data && id === target) sync();
    }
    async function submit() {
      if (terminalKeyboard && terminalKeyboard.blocked()) return;
      sync(); if (!draft || draft.busy || (!ta.value.trim() && !draft.files.length)) return;
      var recipient = target, context = data, currentGeneration = generation, d = draft, body = ta.value, staged = draft.files.slice(), scope = composeScope(recipient);
      clearTimeout(d.infoTimer); d.text = body; d.busy = true; d.error = ''; d.info = ''; update();
      try {
        for (var i = 0; i < staged.length; i++) {
          if (!valid(context, currentGeneration)) return;
          if (!staged[i].path) { staged[i].path = await upload(staged[i], context, function () { if (recipient === target && context === data) renderAttachments(); }, recipient); }
        }
      } catch (error) { if (valid(context, currentGeneration)) failure(recipient, context, error.message || T('Upload failed. Send again to retry.', '上传失败，可再次发送重试。')); return; }
      if (!valid(context, currentGeneration)) return;
      var payload = [body.trim() ? body : '', staged.map(function (a) { return a.path; }).join('\n')].filter(Boolean).join('\n');
      var attempt = d.lastAttempt && d.lastAttempt.payload === payload ? d.lastAttempt : {payload: payload, id: newSendId()}; d.lastAttempt = attempt;
      var pending = beginPendingPrompt(recipient, payload, attempt.id);
      d.text = ''; d.files = []; changed(d, context); if (recipient === target && context === data) { ta.value = ''; appliedText = ''; localVersion = -1; sync(); }
      try {
        var r = await postSend(recipient, {text: payload, enter: true, send_id: attempt.id});
        if (!valid(context, currentGeneration)) return;
        if (r.status === 401) { token = null; gate('expired'); return; }
        if (!r.ok) {
          finishPendingPrompt(recipient, pending, false);
          var message = r.status === 403 ? T('Not sent: input is not allowed in this pane.', '未发送：当前窗格未开放输入。') : T('Send failed.', '发送失败。');
          try { var refusal = await r.json(); if (refusal.error && r.status !== 403) message = refusal.error; } catch (e) {}
          failure(recipient, context, message, {text: body, files: staged}); if (r.status === 403) fetchShare(); return;
        }
        d.busy = false;
        d.lastAttempt = null; finishPendingPrompt(recipient, pending, true);
        if (body.trim()) { context.history[scope] = [body].concat((context.history[scope] || []).filter(function (s) { return s !== body; })).slice(0, 50); context.recent = [body].concat(context.recent.filter(function (s) { return s !== body; })).slice(0, 30); }
        staged.forEach(function (a) { release(a, context); });
        var live = byId(lastAgents, recipient);
        d.info = live && live.status === 'working' ? T('Sent to terminal. The agent is working.', '已发送到终端，当前会话正在工作。') : T('Sent', '已发送'); changed(d, context);
        d.infoTimer = setTimeout(function () { if (context.revoked) return; d.info = ''; changed(d, context); if (recipient === target && context === data) sync(); }, 6000);
        if (recipient === target && context === data) { if (editor) editor.close(); if (compact) { expanded = false; ta.blur(); } sync(); }
        try { var response = await r.json(); if (valid(context, currentGeneration) && recipient === getId() && response && typeof response.text === 'string') echo(response.text); } catch (e) {}
      } catch (e) { if (valid(context, currentGeneration)) { finishPendingPrompt(recipient, pending, false); failure(recipient, context, T('Delivery unconfirmed. Check the conversation before sending again.', '服务器未确认送达。请先检查会话，再决定是否重发。'), {text: body, files: staged}); } }
    }
    function release(a, context) { if (a.url) { URL.revokeObjectURL(a.url); context.urls.delete(a.url); } }
    function renderAttachments() {
      attachments.textContent = ''; attachments.hidden = !draft || !draft.files.length;
      if (!draft) return;
      draft.files.forEach(function (a) {
        var item = document.createElement('div'); item.className = 'cx-attachment';
        var preview = document.createElement('button'); preview.type = 'button'; preview.className = 'cx-preview'; preview.disabled = !!draft.busy;
        if (a.image) { var img = document.createElement('img'); img.src = a.url; img.alt = a.name; img.loading = 'lazy'; preview.appendChild(img); preview.title = T('Annotate image', '标注图片'); preview.onclick = function () { annotateImage(a, data, function () { changed(draft); renderAttachments(); }); }; }
        else { preview.innerHTML = uiIcon('file'); preview.disabled = true; }
        item.appendChild(preview);
        var name = document.createElement('span'); name.className = 'cx-file-name'; name.textContent = a.name; name.title = a.name; item.appendChild(name);
        var progress = document.createElement('span'); progress.className = 'cx-file-progress'; progress.textContent = draft.busy ? a.path ? T('Uploaded', '已上传') : a.progress == null ? T('Uploading…', '上传中…') : Math.round(a.progress * 100) + '%' : a.path ? T('Ready', '已就绪') : Math.ceil(a.file.size / 1024) + ' KB'; item.appendChild(progress);
        var remove = iconButton('close', T('Remove attachment: ', '移除附件：') + a.name, 'cx-remove-file'); remove.disabled = !!draft.busy;
        remove.onclick = function () { draft.files = draft.files.filter(function (x) { return x !== a; }); release(a, data); changed(draft); renderAttachments(); update(); }; item.appendChild(remove); attachments.appendChild(item);
      });
    }
    function stage(files, edit) {
      sync(); if (draft.busy) return; draft.error = ''; draft.info = ''; var firstImage = null;
      files.forEach(function (f) {
        if (draft.files.length >= 20) { draft.error = T('Attach up to 20 files per message.', '每条消息最多可添加 20 个附件。'); return; }
        if (f.size > 30 * 1024 * 1024) { draft.error = T('File exceeds the 30 MB limit: ', '文件超过 30 MB：') + f.name; return; }
        var a = {id: ++composeAttachmentSeq, file: f, name: f.name || 'attachment', image: /^image\//.test(f.type), url: null, progress: null};
        if (a.image) { a.url = URL.createObjectURL(f); data.urls.add(a.url); if (!firstImage) firstImage = a; }
        draft.files.push(a);
      });
      expanded = true; changed(draft); renderAttachments(); update(); if (edit && firstImage) annotateImage(firstImage, data, function () { changed(draft); renderAttachments(); });
    }
    file.onchange = function () { if (!pickerContext || (valid(pickerContext.data, pickerContext.generation) && getId() === pickerContext.id)) stage(Array.from(file.files || []), file.accept === 'image/*'); file.value = ''; };
    attach.onclick = function () {
      sync(); if (draft.busy) return;
      var sheet = browserDialog(T('Add attachment', '添加附件')); sheet.body.classList.add('attachment-actions');
      [[T('Photos', '图片'), 'image', 'image/*'], [T('Files', '文件'), 'file', ''], [T('Camera', '拍照'), 'camera', 'image/*', true]].forEach(function (entry) {
        if (entry[3] && !compact) return;
        var b = iconButton(entry[1], entry[0], 'attachment-action'); var label = document.createElement('span'); label.textContent = entry[0]; b.appendChild(label);
        b.onclick = function () { pickerContext = {data: data, id: target, generation: generation}; sheet.close(); file.accept = entry[2]; if (entry[3]) file.setAttribute('capture', 'environment'); else file.removeAttribute('capture'); file.click(); }; sheet.body.appendChild(b);
      });
      var paste = iconButton('paste', T('Paste from clipboard', '从剪贴板粘贴'), 'attachment-action'); var label = document.createElement('span'); label.textContent = T('Paste', '粘贴'); paste.appendChild(label);
      paste.onclick = async function () {
        var pasteContext = data, pasteRecipient = target, pasteGeneration = generation;
        sheet.close();
        try {
          if (navigator.clipboard && navigator.clipboard.read) {
            var items = await navigator.clipboard.read(), picked = [];
            for (var item of items) { var type = item.types.find(function (t) { return /^image\//.test(t); }); if (type) picked.push(new File([await item.getType(type)], 'clipboard-' + Date.now() + '.png', {type: type})); }
            if (!valid(pasteContext, pasteGeneration) || getId() !== pasteRecipient) return;
            if (picked.length) { stage(picked, true); return; }
          }
          var text = navigator.clipboard && navigator.clipboard.readText ? await navigator.clipboard.readText() : '';
          if (!valid(pasteContext, pasteGeneration) || getId() !== pasteRecipient) return;
          if (!text) throw new Error('empty clipboard'); ta.value += text; remember(); grow(); focusInput();
        } catch (e) { if (valid(pasteContext, pasteGeneration) && getId() === pasteRecipient) { draft.error = T('Clipboard unavailable. Paste into the input or choose a file.', '无法读取剪贴板。请直接粘贴到输入框，或选择文件。'); update(); } }
      }; sheet.body.appendChild(paste);
    };
    expand.onclick = function () {
      if (editor || draft.busy) return; expanded = true; editor = browserDialog(T('Compose', '撰写')); editor.el.classList.add('compose-editor'); editor.body.appendChild(ta); editor.foot.hidden = false;
      var done = document.createElement('button'); done.type = 'button'; done.className = 'dialog-secondary'; done.textContent = T('Done', '收起'); done.onclick = function () { editor.close(); }; editor.foot.appendChild(done);
      var submitButton = document.createElement('button'); submitButton.type = 'button'; submitButton.className = 'dialog-primary'; submitButton.innerHTML = uiIcon('send') + T('Send', '发送'); submitButton.onclick = submit; editor.sendButton = submitButton; editor.foot.appendChild(submitButton);
      editor.onClose = function () { row.insertBefore(ta, expand); editor = null; update(); grow(); }; update(); ta.focus();
    };
    history.onclick = function () { recall(false); }; snippets.onclick = function () { recall(true); };
    function recall(savedPhrases) {
      sync(); var context = data, id = target, sheet = browserDialog(savedPhrases ? T('Quick replies', '常用语') : T('Input history', '输入历史'));
      function insert(value) { if (context.revoked || getId() !== id) { sheet.close(); return; } ta.value = ta.value ? ta.value + '\n' + value : value; remember(); grow(); sheet.restoreFocus = false; sheet.close(); focusInput(); }
      function draw() {
        sheet.body.textContent = '';
        var scope = composeScope(id), values = savedPhrases ? context.snippets : Array.from(new Set((context.history[scope] || []).concat(context.recent))).slice(0, 50);
        if (!values.length) { var empty = document.createElement('p'); empty.className = 'dialog-empty'; empty.textContent = savedPhrases ? T('No saved replies', '暂无常用语') : T('No input history', '暂无输入历史'); sheet.body.appendChild(empty); }
        values.forEach(function (value, index) {
          var row = document.createElement('div'); row.className = 'recall-row'; var pick = document.createElement('button'); pick.type = 'button'; pick.className = 'recall-text'; pick.textContent = value; pick.title = T('Insert into input', '插入输入框'); pick.onclick = function () { insert(value); }; row.appendChild(pick);
          if (savedPhrases) { var edit = iconButton('pen', T('Edit reply', '编辑常用语'), 'recall-action'); edit.onclick = function () { editPhrase(index); }; row.appendChild(edit); }
          var remove = iconButton('close', savedPhrases ? T('Delete reply', '删除常用语') : T('Delete history entry', '删除历史记录'), 'recall-action');
          remove.onclick = function () { if (savedPhrases) context.snippets.splice(index, 1); else { Object.keys(context.history).forEach(function (key) { context.history[key] = context.history[key].filter(function (s) { return s !== value; }); }); context.recent = context.recent.filter(function (s) { return s !== value; }); } saveCompose(context, true); draw(); }; row.appendChild(remove); sheet.body.appendChild(row);
        });
      }
      function editPhrase(index) {
        sheet.body.textContent = ''; var input = document.createElement('textarea'); input.className = 'phrase-input'; input.setAttribute('aria-label', T('Reply text', '常用语内容')); input.value = index == null ? '' : context.snippets[index]; sheet.body.appendChild(input);
        var save = document.createElement('button'); save.type = 'button'; save.className = 'dialog-primary'; save.textContent = T('Save reply', '保存常用语'); save.onclick = function () { if (!input.value.trim()) return; if (index == null) context.snippets.push(input.value.trim()); else context.snippets[index] = input.value.trim(); context.snippets = context.snippets.slice(-30); saveCompose(context, true); draw(); }; sheet.body.appendChild(save); input.focus();
      }
      sheet.foot.hidden = false; var action = document.createElement('button'); action.type = 'button'; action.className = 'dialog-secondary'; action.textContent = savedPhrases ? T('New reply', '新增常用语') : T('Clear history', '清空历史');
      action.onclick = function () { if (savedPhrases) editPhrase(null); else { var oldHistory = context.history, oldRecent = context.recent; context.history = Object.create(null); context.recent = []; saveCompose(context, true); draw(); action.textContent = T('Undo clear', '撤销清空'); action.onclick = function () { context.history = oldHistory; context.recent = oldRecent; saveCompose(context, true); sheet.close(); }; } }; sheet.foot.appendChild(action); draw();
    }
    function upload(a, context, progress, recipient) {
      return new Promise(function (resolve, reject) {
        var request = new XMLHttpRequest(), form = new FormData(); form.append('file', a.file, a.name); request.open('POST', BASE + '/api/upload?id=' + encodeURIComponent(recipient)); request.timeout = 60000;
        if (context.credential) request.setRequestHeader('Authorization', 'Bearer ' + context.credential); request.responseType = 'json'; context.requests.add(request);
        request.upload.onprogress = function (e) { if (context.revoked) return; a.progress = e.lengthComputable ? e.loaded / e.total : null; progress(); };
        function fail(message) { context.requests.delete(request); reject(new Error(message)); }
        request.onload = function () {
          context.requests.delete(request); if (context.revoked) { reject(new Error('retired')); return; }
          if (request.status === 401) { token = null; gate('expired'); reject(new Error('retired')); return; }
          if (request.status === 413) { fail(T('File exceeds the server size limit. Choose a smaller file.', '文件超过服务端大小限制，请选择更小的文件。')); return; }
          if (request.status === 403) { fail(T('Upload is not allowed in this session.', '当前会话未开放上传权限。')); return; }
          if (request.status < 200 || request.status >= 300 || !request.response || typeof request.response.path !== 'string') { fail(T('Upload failed. Send again to retry.', '上传失败，可再次发送重试。')); return; }
          a.progress = 1; resolve(request.response.path);
        };
        request.onerror = request.ontimeout = function () { fail(T('Upload failed. Check the connection and send again.', '上传失败，请检查连接后重试。')); }; request.onabort = function () { fail(T('Upload cancelled.', '上传已取消。')); }; request.send(form);
      });
    }
    sync();
    function inputDeviceChanged() { compact = !!inputMedia.matches; if (!compact) expanded = true; update(); grow(); }
    if (inputMedia) { if (inputMedia.addEventListener) inputMedia.addEventListener('change', inputDeviceChanged); else inputMedia.addListener(inputDeviceChanged); }
    return {keys: terminalKeyboard, dispose: function () { if (inputMedia) { if (inputMedia.removeEventListener) inputMedia.removeEventListener('change', inputDeviceChanged); else inputMedia.removeListener(inputDeviceChanged); } if (terminalKeyboard) terminalKeyboard.dispose(); }, el: root, sync: sync, input: ta, focus: focusInput, reset: function () { generation++; if (data && !data.revoked) { Object.keys(data.drafts).forEach(function (id) { var d = data.drafts[id]; d.files.forEach(function (a) { release(a, data); }); d.text = ''; d.files = []; d.version++; d.busy = false; d.recovery = null; d.error = ''; d.info = ''; }); saveCompose(data, true); } target = null; draft = null; data = null; localVersion = -1; ta.value = ''; appliedText = ''; note.textContent = ''; note.hidden = true; attachments.textContent = ''; attachments.hidden = true; }};
  }

  // openAgent enters the pane view for an agent; the selected tab (terminal mirror
  // vs chat history) is remembered across agents (gtmux.paneMode).
  var focusFromWB = false;
  var externalEtag = '',
    externalDropped = 0,
    externalSettings = null,
    externalBusy = false,
    externalError = '';
  function conversationQuery(a) {
    return (
      '?session_id=' + encodeURIComponent(a.session_id) + '&home_id=' + encodeURIComponent(a.home_id || '')
    );
  }
  function openConversation(a) {
    if (!readableConversation(a)) return;
    focusFromWB = !$('workbench').hidden;
    curPane = null;
    curAgent = a;
    paneMode = 'chat';
    clearInterval(radarTimer);
    radarTimer = null;
    clearInterval(paneTimer);
    paneTimer = null;
    clearInterval(chatTimer);
    show('chat');
    hideFocusChrome();
    $('bar').hidden = false;
    $('back').hidden = false;
    $('mode').hidden = false;
    syncModeButtons();
    $('panes-btn').hidden = true;
    $('title').textContent = primary(a) || a.agent;
    $('sub').textContent = nativeSourceLabel(a);
    externalEtag = '';
    externalDropped = 0;
    externalSettings = null;
    externalBusy = false;
    externalError = '';
    chatSig = '';
    lastTurns = [];
    chatExpanded = {};
    showChatLoader();
    updateInputBar();
    pollConversation();
    chatTimer = setInterval(pollConversation, 2500);
  }
  function pollConversation() {
    if (!readableConversation(curAgent) || $('chat').hidden) return;
    pollConversationStatus();
    runRead(
      'conversation',
      function (signal) {
        var a = curAgent,
          current = currentRead('conversation', signal);
        var headers = externalEtag ? { 'If-None-Match': externalEtag } : {};
        return api('/api/session/transcript' + conversationQuery(a), { signal: signal, headers: headers })
          .then(function (r) {
            if (!current()) return null;
            if (r.status === 401) {
              token = null;
              localStorage.removeItem(TOKEN_KEY);
              gate('expired');
              return null;
            }
            if (r.status === 403 || r.status === 422) {
              lastTurns = [];
              externalEtag = '';
              drawChat([]);
              throw new Error('unavailable');
            }
            if (r.status === 304) return null;
            if (!r.ok) throw new Error('conversation');
            var etag = r.headers.get('ETag') || '',
              dropped = Number(r.headers.get('X-Gtmux-Turns-Dropped') || 0);
            return r.json().then(function (turns) {
              if (!current()) return null;
              externalEtag = etag;
              externalDropped = dropped;
              return turns;
            });
          })
          .then(function (turns) {
            if (!current(true)) return;
            setConn(true);
            externalError = '';
            hideChatLoader();
            if (turns !== null) renderChat(turns || []);
          })
          .catch(function () {
            if (current()) {
              setConn(false);
              hideChatLoader();
              externalError = T('Could not update this conversation. Retry shortly.', '暂时无法更新此对话，将稍后重试。');
              drawChat(lastTurns);
            }
          });
      },
      15000,
      function () {
        setConn(false);
      }
    );
    if (!externalBusy)
      runRead('follow', function (signal) {
        var current = currentRead('follow', signal);
        return api('/api/session-follow' + conversationQuery(curAgent), { signal: signal })
          .then(function (r) {
            if (!r.ok) throw new Error('follow');
            return r.json();
          })
          .then(function (v) {
            if (current(true) && !externalBusy && JSON.stringify(externalSettings) !== JSON.stringify(v)) {
              externalSettings = v;
              drawChat(lastTurns);
            }
          })
          .catch(function () {
            if (current() && !externalBusy) {
              externalSettings = null;
              drawChat(lastTurns);
            }
          });
      });
  }
  function pollConversationStatus() {
    runRead('conversation-status', function (signal) {
      var current = currentRead('conversation-status', signal), a = curAgent;
      return api('/api/agents', {signal: signal}).then(function (r) {
        if (!r.ok) throw new Error('status');
        return r.json();
      }).then(function (rows) {
        if (!current(true) || !Array.isArray(rows)) return;
        var next = rows.find(function (row) {return isNative(a) ? row.source === 'native' && row.session_id === a.session_id && row.home_id === a.home_id : row.pane_id === a.pane_id;});
        if (!next) return;
        var changed = next.status !== a.status || next.task !== a.task || next.can_read_conversation !== a.can_read_conversation || next.current_client !== a.current_client || JSON.stringify(next.launch_host) !== JSON.stringify(a.launch_host);
        curAgent = next;
        if (isNative(next) && !readableConversation(next)) {lastTurns = []; externalSettings = null;}
        if (changed) {$('title').textContent = primary(next) || next.agent; $('sub').textContent = isNative(next) ? nativeSourceLabel(next) : (next.agent ? next.agent + ' · ' : '') + secondary(next); drawChat(lastTurns);updateInputBar();}
        var main = $('chat').querySelector('.conversation-main');
        if (curPane && paneMode === 'chat' && main) syncChatLive(main, next, function () { return !$('chat').hidden && paneMode === 'chat' && curPane === next.pane_id && curAgent && curAgent.status === 'working'; });
      }).catch(function () {});
    });
  }
  function conversationControls() {
    var box = el('div', 'external-controls'),
      msg = el('p', '', T('Read only · Continue in the original client', '只读 · 请在原客户端继续对话'));
    box.appendChild(msg);
    if (curAgent) box.appendChild(el('p', '', nativeSourceLabel(curAgent)));
    var evidence = nativeEvidenceLabel(curAgent);
    if (evidence) box.appendChild(el('p', '', evidence));
    if (externalDropped > 0) box.appendChild(el('p', '', T('Earlier messages omitted', '已省略部分较早对话')));
    if (externalError) box.appendChild(el('p', '', externalError));
    var titles = {
      hq: T('HQ follow', 'HQ 跟进'),
      notify: T('Conversation notifications', '会话通知'),
      knowledge: T('Save future lessons to knowledge', '留存开启后的知识'),
    };
    Object.keys(titles).forEach(function (key) {
      var label = el('label', 'external-toggle'),
        input = document.createElement('input');
      input.type = 'checkbox';
      input.checked = !!(externalSettings && externalSettings[key]);
      input.disabled = externalBusy || !externalSettings || (key !== 'hq' && !externalSettings.hq);
      input.onchange = function () {
        var epoch = viewEpoch,
          credential = token,
          a = curAgent,
          next = Object.assign({}, externalSettings);
        next[key] = input.checked;
        if (key === 'hq' && !next.hq) {
          next.notify = false;
          next.knowledge = false;
        }
        externalBusy = true;
        externalError = '';
        cancelRead('follow');
        drawChat(lastTurns);
        api('/api/session-follow', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ session_id: a.session_id, home_id: a.home_id, settings: next }),
        })
          .then(function (r) {
            if (!r.ok) throw new Error('save');
            return r.json();
          })
          .then(function (v) {
            if (epoch === viewEpoch && credential === token) {
              externalSettings = v;
            }
          })
          .catch(function () {
            if (epoch === viewEpoch && credential === token) {
              externalSettings = null;
              externalError = T('Update not confirmed. Reloading current settings.', '更新未确认，正在重新读取设置。');
            }
          })
          .finally(function () {
            if (epoch === viewEpoch && credential === token) {
              externalBusy = false;
              drawChat(lastTurns);
              pollConversation();
            }
          });
      };
      label.appendChild(input);
      label.appendChild(document.createTextNode(titles[key]));
      box.appendChild(label);
    });
    if (!externalSettings)
      box.appendChild(el('p', '', T('Follow settings are unavailable until verified.', '跟进设置尚未确认。')));
    return box;
  }

  function openAgent(a, initialMode) {
    if (readableConversation(a)) {openConversation(a);return;}
    if (!hasPane(a)) return;
    // Previous/next changes the pane, not the top-level view it was opened from.
    if ($('pane').hidden && $('chat').hidden) focusFromWB = !$('workbench').hidden;
    curPane = a.pane_id; curAgent = a; paneMode = a.agent ? (initialMode || preferredPaneMode) : 'term'; $('bar').hidden = false; $('back').hidden = false; $('mode').hidden = !a.agent; $('gear').hidden = false; $('panes-btn').hidden = true;
    $('title').textContent = primary(a); $('sub').textContent = (a.agent ? a.agent + ' · ' : '') + secondary(a);
    clearInterval(radarTimer); radarTimer = null;
    applyMode();
  }
  // applyMode shows the active pane tab and (re)starts ONLY that tab's poll loop.
  function applyMode() {
    syncModeButtons();
    if (paneMode !== 'term') {
      clearInterval(paneTimer); paneTimer = null;
      show('chat'); chatSig = ''; lastTurns = []; chatExpanded = {}; curTurnIdx = 1e9;
      showChatLoader();
      showFocusChrome(false);
      pollChat(); clearInterval(chatTimer); chatTimer = setInterval(pollChat, 2500);
      updateInputBar(); // keep the top-bar ⌨/👁 capability chip in sync in chat mode
    } else {
      clearInterval(chatTimer); chatTimer = null;
      show('pane'); ensureTerm(); lastText = ''; pendingText = null; userScrolling = false; resetPin();
      showPaneLoader();
      try { fit.fit(); } catch (e) {}
      showFocusChrome(true);
      pollPane(); clearInterval(paneTimer); paneTimer = setInterval(pollPane, 1200);
      updateInputBar(); // show the input row iff this caller may type into this pane
    }
  }
  function setMode(m) {
    if (!curPane && readableConversation(curAgent)) {
      if (m !== 'chat' || m === paneMode) return;
      paneMode = m; syncModeButtons(); updatePageTitle('chat'); drawChat(lastTurns); updateInputBar(); return;
    }
    if (!['chat', 'term'].includes(m) || m === paneMode || !curPane || (m !== 'term' && !(curAgent && curAgent.agent))) return;
    paneMode = m; preferredPaneMode = m;
    try { localStorage.setItem('gtmux.paneMode', m); } catch (e) {}
    applyMode();
  }
  function syncModeButtons() {
    var bs = $('mode').querySelectorAll('button');
    Array.prototype.forEach.call(bs, function (b) { var mode = b.getAttribute('data-mode'), selected = mode === paneMode; b.hidden = !curPane && readableConversation(curAgent) && mode === 'term'; b.classList.toggle('on', selected); b.setAttribute('aria-pressed', String(selected)); });
  }

  // ---- chat (对话 mode) -------------------------------------------------
  // Renders the parsed transcript (GET /api/transcript) as prompt → collapsed
  // steps → final response, mirroring the phone's ChatView. Input uses the shared composer.
  function pollChat() {
    pollConversationStatus();
    runRead('chat', function (signal) {
      if (!curPane) return;
      var current = currentRead('chat', signal);
      return api('/api/transcript?id=' + encodeURIComponent(curPane), {signal: signal}).then(function (r) {
        if (!current()) return null;
        if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
        if (r.status === 404) { setConn(true); return []; } // no resume record / not available
        if (!r.ok) throw new Error('transcript'); return r.json();
      }).then(function (turns) { if (turns === null || !current(true)) return; setConn(true); hideChatLoader(); renderChat(turns || []); })
        .catch(function () { if (current()) setConn(false); });
    }, 15000, function () { setConn(false); });
  }

  function renderChat(turns) {
    reconcilePendingPrompts(curPane, turns);
    lastTurns = turns;
    // only repaint when content changed (keeps scroll position + expanded steps)
    var sig = JSON.stringify(turns);
    if (sig === chatSig) return;
    chatSig = sig;
    drawChat(turns);
  }
  // §03 wide chat: a turn-outline rail (left) + a centered conversation column with
  // copy actions beside agent messages + a waiting approval card. Narrow: no rail.
  function drawChat(turns) {
    var root = $('chat');
    var preserved = snapshotToolState(root), focused = preserved.focused, inputScroll = preserved.scroll;
    var oldScroll = root.querySelector('.conv-scroll');
    var atBottom = !oldScroll || (oldScroll.scrollHeight - oldScroll.scrollTop - oldScroll.clientHeight < 48);
    var prevTop = oldScroll ? oldScroll.scrollTop : 0;
    root.innerHTML = ''; root.classList.remove('wide');
    var a = curAgent || {};

    var scroll = document.createElement('div'); scroll.className = 'conv-scroll';
    var col = document.createElement('div'); col.className = 'chat-col';
    if(readableConversation(a)) col.appendChild(conversationControls());

    var sr = document.createElement('div'); sr.className = 'chat-state';
    sr.appendChild(speakerAvatar('agent', a, avatarEl(a, 30, false)));
    var stx = document.createElement('div'); stx.className = 'cs-text';
    var nm = document.createElement('div'); nm.className = 'cs-name'; nm.textContent = a.agent || ''; stx.appendChild(nm);
    var stl = document.createElement('div'); stl.className = 'cs-status';
    var dot = document.createElement('span'); dot.className = 'cs-dot'; dot.style.background = COLORS[a.status] || COLORS.running; stl.appendChild(dot);
    var lb = document.createElement('span'); lb.textContent = LABEL[a.status] || a.status || ''; stl.appendChild(lb);
    stx.appendChild(stl); sr.appendChild(stx); col.appendChild(sr);
    if (turns.some(function (t) { return (t.segments || []).some(function (s) { return s.steps && s.steps.length; }); })) {
      var collapse = el('button', 'chat-collapse', T('Collapse all', '折叠全部'));
      collapse.type = 'button'; collapse.dataset.chatFocus = 'collapse'; collapse.onclick = collapseAllSteps; sr.appendChild(collapse);
    }

    if (!turns.length && !(pendingPrompts[curPane] || []).length) {
      var e = document.createElement('div'); e.className = 'chat-empty';
      e.textContent = readableConversation(a) ? T('No recorded conversation yet.','尚无已记录的对话。') : T('No conversation yet. Switch to Terminal to see the current screen.',
        '暂无对话记录。可切换到「终端」查看当前屏幕。');
      col.appendChild(e);
    }
    var seps = sepLabels(turns);
    turns.forEach(function (t, idx) { col.appendChild(conversationTurn(t, a, idx, chatExpanded, seps[idx], true)); });
    appendPendingPrompts(col, curPane, a);
    // waiting → an inline approval card (read-only 1/2/3 from /api/options).
    var main = document.createElement('div'); main.className = 'conversation-main';
    scroll.appendChild(col); main.appendChild(scroll); root.appendChild(main);
    renderChatInteraction();
    syncChatLive(main, a, function () { return !$('chat').hidden && paneMode === 'chat' && curPane === a.pane_id && curAgent && curAgent.status === 'working'; });
    scroll.scrollTop = atBottom ? scroll.scrollHeight : prevTop;
    if (curTurnIdx >= turns.length) curTurnIdx = Math.max(0, turns.length - 1);
    root.querySelectorAll('[data-chat-focus]').forEach(function (node) {
      var key = node.dataset.chatFocus, pos = inputScroll[key];
      if (pos) { node.scrollLeft = pos[0]; node.scrollTop = pos[1]; }
      if (key === focused) node.focus({preventScroll: true});
    });
  }

  function snapshotToolState(root) {
    var active = document.activeElement, scroll = {};
    root.querySelectorAll('.step-inspect').forEach(function (node) { if (node.saveOpenState) node.saveOpenState(); });
    root.querySelectorAll('.step-input').forEach(function (node) { scroll[node.dataset.chatFocus] = [node.scrollLeft, node.scrollTop]; });
    return {focused: active && root.contains && root.contains(active) ? active.getAttribute('data-chat-focus') : null, scroll: scroll};
  }
  function restoreToolState(root, preserved) {
    root.querySelectorAll('[data-chat-focus]').forEach(function (node) {
      var key = node.dataset.chatFocus, scroll = preserved.scroll[key];
      if (scroll) { node.scrollLeft = scroll[0]; node.scrollTop = scroll[1]; }
      if (key === preserved.focused) node.focus({preventScroll: true});
    });
  }
  // Plain text only: tool arguments are untrusted transcript content, never markup.
  function toolStepsEl(steps, open, onToggle, stateKey, expansion) {
    var detailState = expansion || chatExpanded;
    var activity = window.GtmuxActivity, lang = ZH ? 'zh' : 'en';
    var card = el('div', 'tool-steps');
    var toggle = el('button', 'steps-toggle');
    var count = T(steps.length + ' call' + (steps.length === 1 ? '' : 's'), steps.length + ' 次调用');
    var title = activity.summary(steps, lang);
    toggle.type = 'button';
    toggle.setAttribute('aria-expanded', String(open));
    toggle.setAttribute('aria-label', title + ', ' + count);
    toggle.dataset.chatFocus = 'group-' + stateKey;
    toggle.onclick = onToggle;
    var head = el('span', 'steps-heading');
    head.appendChild(activityIcon('tool'));
    head.appendChild(el('span', 'steps-title', title));
    head.appendChild(el('span', 'steps-count', count));
    var chevron = el('span', 'steps-chevron'); chevron.innerHTML = uiIcon(open ? 'up' : 'down'); head.appendChild(chevron);
    toggle.appendChild(head);
    card.appendChild(toggle);
    if (open) {
      var list = el('ol', 'steps-list');
      steps.forEach(function (s, i) {
        var row = el('li', 'step-row');
        var op = activity.operation(s), key = 'step-' + stateKey + '-' + i;
        var inspectable = !!(s.input || s.workdir || s.details_truncated);
        var details = el(inspectable ? 'details' : 'div', 'step-inspect'); if (inspectable) details.open = !!detailState[key];
        details.saveOpenState = function () { detailState[key] = details.open; };
        details.ontoggle = function () { if (details.isConnected) detailState[key] = details.open; };
        var summary = el(inspectable ? 'summary' : 'div', 'step-toggle' + (inspectable ? '' : ' step-static')); summary.dataset.chatFocus = key;
        summary.appendChild(activityIcon(op));
        var body = el('span', 'step-body');
        body.appendChild(el('span', 'step-label', activity.label(op, lang)));
        body.appendChild(el('span', 'step-detail' + (op === 'agent' || op === 'wait' ? ' step-prose' : ''), activity.preview(s, lang)));
        summary.appendChild(body); details.appendChild(summary);
        if (!inspectable) { row.appendChild(details); list.appendChild(row); return; }
        var inspector = el('div', 'step-inspector');
        var meta = el('dl', 'step-meta');
        meta.appendChild(el('dt', '', T('Tool', '工具'))); meta.appendChild(el('dd', 'step-name', s.title || 'tool'));
        if (s.workdir) { meta.appendChild(el('dt', '', T('Working directory', '工作目录'))); meta.appendChild(el('dd', '', s.workdir)); }
        inspector.appendChild(meta);
        var input = s.input || s.detail;
        if (input) {
          var inputLabel = !s.input ? T('Recorded detail', '已记录详情')
            : s.input_kind === 'command' ? T('Recorded command', '已记录命令')
            : s.input_kind === 'code' ? T('Recorded code', '已记录代码')
            : s.input_kind === 'patch' ? T('Recorded patch', '已记录补丁')
            : s.input_kind === 'parameters' || s.input_kind === 'arguments' ? T('Recorded parameters', '已记录参数') : T('Recorded input', '已记录输入');
          var toolbar = el('div', 'step-toolbar'); toolbar.appendChild(el('span', '', inputLabel));
          toolbar.appendChild(clipboardAction(T('Copy', '复制'), '⧉', input, key + '-copy')); inspector.appendChild(toolbar);
          var pre = el('pre', 'step-input', input); pre.tabIndex = 0; pre.dataset.chatFocus = key + '-input';
          pre.setAttribute('role', 'region'); pre.setAttribute('aria-label', inputLabel); inspector.appendChild(pre);
        }
        if (s.details_truncated) inspector.appendChild(el('p', 'step-truncated', T('This record was truncated at the source.', '此记录已在来源处截断。')));
        details.appendChild(inspector); row.appendChild(details); list.appendChild(row);
      });
      card.appendChild(list);
    }
    return card;
  }

  function activityIcon(operation) {
    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 24 24'); svg.setAttribute('class', 'activity-icon'); svg.setAttribute('aria-hidden', 'true');
    var path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', window.GtmuxActivity.paths[operation]); svg.appendChild(path); return svg;
  }

  // Keep glyph-only actions discoverable on touch and keyboard, with honest feedback.
  function clipboardAction(label, glyph, text, focusKey) {
    var wrap = el('span', 'clipboard-action'), b = el('button', '', ''), status = el('span', 'copy-status');
    b.type = 'button'; b.innerHTML = uiIcon('copy'); b.title = label; b.setAttribute('aria-label', label);
    if (focusKey) b.dataset.chatFocus = focusKey;
    status.setAttribute('role', 'status');
    var pending = false;
    b.onclick = function (e) {
      e.stopPropagation(); if (pending) return; pending = true; b.setAttribute('aria-busy', 'true'); status.textContent = '';
      copyWithFeedback(b, text, label, 'copy', status).then(function () { pending = false; b.setAttribute('aria-busy', 'false'); });
    };
    wrap.appendChild(b); wrap.appendChild(status); return wrap;
  }

  function promptText(value) { return window.GtmuxPromptIdentity.promptIdentity(value); }
  function promptObservations(text, turns) {
    var counts = new Map(); turns.forEach(function (turn) { if (promptText(turn.prompt) === promptText(text)) { var key = turn.time || ''; counts.set(key, (counts.get(key) || 0) + 1); } }); return counts;
  }
  function pendingPromptRecorded(echo, turns) {
    var additions = 0; promptObservations(echo.text, turns).forEach(function (count, time) { additions += Math.max(0, count - (echo.observed.get(time) || 0)); }); return additions > echo.priorPending;
  }
  function reconcilePendingPrompts(id, turns) { if (id && pendingPrompts[id]) pendingPrompts[id] = pendingPrompts[id].filter(function (echo) { return !pendingPromptRecorded(echo, turns); }); }
  function refreshPendingConversation(id, follow) {
    if (curPane === id && !$('chat').hidden && paneMode === 'chat') { drawChat(lastTurns); var scroll = $('chat').querySelector('.conv-scroll'); if (follow && scroll) scroll.scrollTop = scroll.scrollHeight; }
    else if (WB.on) WB.tiles.forEach(function (tile) { if (tile.id === id && tile.mode === 'chat') { renderTileChat(tile, tile.lastTurns || []); var scroll = tile.body.querySelector('.tile-chat'); if (follow && scroll) scroll.scrollTop = scroll.scrollHeight; } });
  }
  function beginPendingPrompt(id, text, sendId) {
    var tile = WB.tiles.find(function (t) { return t.id === id; }), turns = curPane === id ? lastTurns : tile && tile.lastTurns || [];
    reconcilePendingPrompts(id, turns); var list = pendingPrompts[id] || (pendingPrompts[id] = []);
    var echo = {id: sendId, text: text, delivered: false, observed: promptObservations(text, turns), priorPending: list.filter(function (e) { return promptText(e.text) === promptText(text); }).length};
    list.push(echo); if (list.length > 20) list.shift(); refreshPendingConversation(id, true); return echo;
  }
  function finishPendingPrompt(id, echo, delivered) {
    if (!delivered) pendingPrompts[id] = (pendingPrompts[id] || []).filter(function (e) { return e !== echo; });
    else echo.delivered = true;
    refreshPendingConversation(id);
  }
  function appendPendingPrompts(host, id, agent) {
    (pendingPrompts[id] || []).forEach(function (echo) {
      var turn = conversationTurn({prompt: echo.text}, agent, -1, {}, '', false); turn.classList.add('pending-prompt');
      var status = document.createElement('span'); status.className = 'pending-prompt-note'; status.textContent = echo.delivered ? T('Sent', '已发送') : T('Sending…', '正在发送…');
      var footer = turn.querySelector('.message-meta'); footer.insertBefore(status, footer.firstChild); host.appendChild(turn);
    });
  }

  // Like the app, Live reads the current screen, never a previous transcript reply.
  function rememberServerClock(response) {
    var value = response.headers && response.headers.get && Date.parse(response.headers.get('Date'));
    if (value) { serverClock = value; serverClockAt = Date.now(); }
  }
  function liveDuration(agent) {
    if (!serverClock || !(agent.since > 0)) return '';
    var seconds = Math.floor((serverClock + Date.now() - serverClockAt) / 1000 - agent.since);
    if (seconds < 0) return '';
    if (seconds < 60) return seconds + T('s', '秒');
    var minutes = Math.floor(seconds / 60);
    return minutes < 60 ? minutes + T('m ', '分') + seconds % 60 + T('s', '秒') : Math.floor(minutes / 60) + T('h ', '小时') + minutes % 60 + T('m', '分');
  }
  function liveTail(value) {
    var lines = String(value || '').split('\n').map(function (line) { return plainRow(line).replace(/[\x00-\x08\x0b-\x1f\x7f]/g, ''); });
    while (lines.length && !lines[lines.length - 1].trim()) lines.pop();
    return lines.slice(-14).join('\n');
  }
  function syncChatLive(host, agent, valid) {
    var id = agent && agent.pane_id; if (!id) return;
    var state = chatLive[id] || (chatLive[id] = {open: false, revision: 0});
    var panel = host.querySelector('.chat-live'), working = agent.status === 'working';
    if (state.working !== working || state.since !== agent.since) {
      state.revision++; state.text = ''; state.error = false; state.loading = false;
      cancelRead('live:' + id);
    }
    state.working = working; state.since = agent.since; state.agent = agent; state.valid = valid;
    if (!working) { if (panel) panel.remove(); return; }
    if (!panel) {
      panel = document.createElement('details'); panel.className = 'chat-live'; panel.dataset.pane = id;
      var summary = document.createElement('summary'); summary.className = 'live-summary';
      var dot = document.createElement('span'); dot.className = 'live-dot'; dot.setAttribute('aria-hidden', 'true'); summary.appendChild(dot);
      var title = document.createElement('strong'); title.textContent = T('Working', '正在处理'); summary.appendChild(title);
      panel.clock = document.createElement('span'); panel.clock.className = 'live-clock'; summary.appendChild(panel.clock);
      panel.action = document.createElement('span'); panel.action.className = 'live-action'; summary.appendChild(panel.action);
      var chevron = document.createElement('span'); chevron.className = 'live-chevron'; chevron.innerHTML = uiIcon('chevron'); summary.appendChild(chevron); panel.appendChild(summary);
      panel.output = document.createElement('pre'); panel.output.className = 'live-output'; panel.output.setAttribute('aria-label', T('Live terminal output', '实时终端输出')); panel.appendChild(panel.output);
      panel.note = document.createElement('p'); panel.note.className = 'live-note'; panel.note.setAttribute('role', 'status'); panel.appendChild(panel.note);
      panel.viewport = host.querySelector('.conv-scroll, .tile-chat');
      summary.addEventListener('click', function () { var view = panel.viewport; panel.followOnToggle = view && view.clientHeight > 0 && view.scrollHeight - view.scrollTop - view.clientHeight < 48; });
      panel.open = !!state.open;
      panel.ontoggle = function () { state.open = panel.open; paintLive(panel, state); if (panel.followOnToggle && panel.viewport) panel.viewport.scrollTop = panel.viewport.scrollHeight; panel.followOnToggle = false; if (state.open) pollChatLive(id); else cancelRead('live:' + id); };
      host.appendChild(panel);
    }
    if (panel.open !== !!state.open) panel.open = !!state.open;
    paintLive(panel, state);
    if (state.open) pollChatLive(id);
  }
  function paintLive(panel, state) {
    var viewport = panel.viewport, following = viewport && viewport.clientHeight > 0 && viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight < 48;
    panel.clock.textContent = liveDuration(state.agent);
    panel.action.textContent = state.open ? T('Collapse', '收起') : T('Live output', '展开实时输出');
    var text = state.text || (state.error ? T('Live output is unavailable.', '暂时无法读取实时输出。') : state.loading ? T('Loading live output…', '正在加载实时输出…') : T('Waiting for output…', '等待实时输出…'));
    if (panel.output.textContent !== text) { var atBottom = panel.output.scrollHeight - panel.output.scrollTop - panel.output.clientHeight < 32; panel.output.textContent = text; if (atBottom) panel.output.scrollTop = panel.output.scrollHeight; }
    panel.note.hidden = !state.error; panel.note.textContent = state.error ? (state.text ? T('Update failed. Showing the last capture; retrying…', '更新失败，显示上次内容，正在重试…') : T('Retrying automatically…', '正在自动重试…')) : '';
    if (following) viewport.scrollTop = viewport.scrollHeight;
  }
  function paintLivePanels(id, state) {
    document.querySelectorAll('.chat-live').forEach(function (panel) { if (panel.dataset.pane === id) paintLive(panel, state); });
  }
  function pollChatLive(id) {
    var state = chatLive[id]; if (!state || !state.open || !state.working || !state.valid()) return;
    runRead('live:' + id, function (signal) {
      var revision = state.revision, current = currentRead('live:' + id, signal);
      function active() { return current() && state.revision === revision && state.open && state.working && state.valid(); }
      state.loading = true; paintLivePanels(id, state);
      return api('/api/pane?id=' + encodeURIComponent(id), {signal: signal}).then(function (r) {
        if (!active()) return null;
        if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
        if (!r.ok) throw new Error('live capture'); return r.json();
      }).then(function (capture) {
        if (!capture || !active()) return;
        state.text = liveTail(capture.text); state.error = false;
      }).catch(function () { if (active()) state.error = true; }).finally(function () {
        if (state.revision === revision) state.loading = false;
        if (active()) paintLivePanels(id, state);
      });
    }, 15000, function () {
      if (state.open && state.working && state.valid()) { state.loading = false; state.error = true; paintLivePanels(id, state); }
    });
  }
  function pollChatStatus() {
    if ($('chat').hidden || !curPane || !curAgent) return;
    runRead('chat-status', function (signal) {
      var asked = curPane, current = currentRead('chat-status', signal);
      return api('/api/agents', {signal: signal}).then(function (r) {
        if (!current()) return null;
        if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
        if (!r.ok) throw new Error('chat status'); rememberServerClock(r); return r.json();
      }).then(function (agents) {
        if (!Array.isArray(agents) || !current(true) || asked !== curPane) return;
        var next = byId(agents, asked), changed = !next || next.status !== curAgent.status || next.since !== curAgent.since;
        lastAgents = agents; curAgent = next || Object.assign({}, curAgent, {status: ''});
        if (paneMode === 'chat') {
          if (changed && $('chat').querySelector('.conversation-main')) drawChat(lastTurns);
          else { var host = $('chat').querySelector('.conversation-main'); if (host) syncChatLive(host, curAgent, function () { return !$('chat').hidden && paneMode === 'chat' && curPane === asked && curAgent.status === 'working'; }); }
        }
      }).catch(function () { if (current()) setConn(false); });
    });
  }

  // Keyboard navigation stays in the conversation reading column.
  var curTurnIdx = 0;
  function selectTurn(idx, scrollIt) {
    var turns = lastTurns || []; if (!turns.length) return;
    curTurnIdx = Math.max(0, Math.min(turns.length - 1, idx));
    if (scrollIt) { var el = $('cturn-' + curTurnIdx); if (el) el.scrollIntoView({block: 'start', behavior: 'smooth'}); }
  }
  // Shared by focused chat and board tiles: one transcript, one set of affordances.
  function turnSegments(t) {
    return (t.segments && t.segments.length ? t.segments : [{text: t.response || '', steps: t.steps || []}]).filter(function (segment) { return segment.text || segment.steps && segment.steps.length; });
  }
  function conversationTurn(t, currentAgent, idx, expanded, separator, focused, onUpdate) {
    var ct = document.createElement('article'); ct.className = 'cturn'; if (focused) ct.id = 'cturn-' + idx;
    if (separator) ct.appendChild(sepEl(separator));
    if (t.prompt) {
      var ur = document.createElement('div'); ur.className = 'urow';
      var us = document.createElement('div'); us.className = 'message-stack user-message';
      var ub = document.createElement('div'); ub.className = 'ububble'; ub.appendChild(mdRender(t.prompt, {session_id: t.session_id || currentAgent.session_id, home_id: currentAgent.home_id || ''})); us.appendChild(ub);
      us.appendChild(messageMeta(t.prompt, t.time, t.from));
      ur.appendChild(us); ur.appendChild(speakerAvatar(t.from ? t.from.kind : 'user', t.from || {}, t.from ? senderAvatarEl(t.from, 26) : userAvatarEl(26))); ct.appendChild(ur);
    }
    // A stitched HQ history may contain several different agent sessions.
    var a = Object.assign({}, currentAgent, {agent: t.agent || currentAgent.agent, profileTurn: {session_id: t.session_id, time: t.time}});
    turnSegments(t).forEach(function (seg, k) {
      if (seg.text) {
        var ar = document.createElement('div'); ar.className = 'arow'; ar.appendChild(speakerAvatar('agent', a, avatarEl(a, 28, false)));
        var as = document.createElement('div'); as.className = 'message-stack assistant-message';
        var ab = document.createElement('div'); ab.className = 'abubble'; ab.appendChild(mdRender(seg.text, {session_id: t.session_id || currentAgent.session_id, home_id: currentAgent.home_id || ''})); as.appendChild(ab);
        as.appendChild(messageMeta(seg.text)); ar.appendChild(as); ct.appendChild(ar);
      }
      if (seg.steps && seg.steps.length) {
        var key = idx + '-' + k;
        ct.appendChild(toolStepsEl(seg.steps, !!expanded[key], function () {
          expanded[key] = !expanded[key];
          if (onUpdate) onUpdate(); else if (focused) drawChat(lastTurns);
        }, key, expanded));
      }
    });
    return ct;
  }
  function messageMeta(value, timestamp, from) {
    var meta = document.createElement('div'); meta.className = 'message-meta';
    if (from) meta.appendChild(senderNoteEl(from));
    var date = timestamp && new Date(timestamp);
    if (date && !isNaN(date.getTime())) {
      var time = document.createElement('time'); time.setAttribute('datetime', date.toISOString()); time.textContent = sepTime(date); meta.appendChild(time);
    }
    meta.appendChild(bubbleActions(value)); return meta;
  }
  var dialogSequence = 0;
  function browserDialog(title) {
    var previousFocus = document.activeElement, dialog = document.createElement('dialog'); dialog.className = 'browser-dialog';
    var head = document.createElement('header'); head.className = 'dialog-head';
    var heading = document.createElement('h2'); heading.id = 'dialog-title-' + (++dialogSequence); heading.textContent = title; head.appendChild(heading); dialog.setAttribute('aria-labelledby', heading.id);
    var close = document.createElement('button'); close.type = 'button'; close.className = 'dialog-close'; close.innerHTML = uiIcon('close'); close.title = T('Close', '关闭'); close.setAttribute('aria-label', close.title); head.appendChild(close);
    var body = document.createElement('div'); body.className = 'dialog-body';
    var foot = document.createElement('footer'); foot.className = 'dialog-foot'; foot.hidden = true;
    dialog.appendChild(head); dialog.appendChild(body); dialog.appendChild(foot); document.body.appendChild(dialog);
    close.onclick = function () { dialog.close(); };
    dialog.addEventListener('keydown', function (e) { e.stopPropagation(); });
    var result = {el: dialog, body: body, foot: foot, restoreFocus: true, close: function () { dialog.close(); }, onClose: null};
    var cleaned = false;
    function cleanup() { if (cleaned) return; cleaned = true; if (result.onClose) result.onClose(); dialog.remove(); if (result.restoreFocus && previousFocus && previousFocus.isConnected && previousFocus.focus) previousFocus.focus(); }
    // Revocation must discard content and cancel reads before the gate is visible.
    dialog.retire = function () { result.restoreFocus = false; dialog.close(); cleanup(); };
    dialog.addEventListener('close', cleanup);
    dialog.addEventListener('click', function (e) { var rect = dialog.getBoundingClientRect(); if (e.target === dialog && (e.clientX < rect.left || e.clientX > rect.right || e.clientY < rect.top || e.clientY > rect.bottom)) dialog.close(); });
    dialog.showModal(); return result;
  }
  function speakerAvatar(kind, source, avatar) {
    var button = document.createElement('button'); button.type = 'button'; button.className = 'speaker-avatar';
    var name = kind === 'user' ? T('User', '用户') : source.label || (isHQPane(source) ? 'HQ' : source.agent) || T('Agent', 'Agent');
    button.title = T('View profile: ', '查看资料：') + name; button.setAttribute('aria-label', button.title); button.appendChild(avatar);
    button.onclick = function (e) { e.stopPropagation(); showSpeakerProfile(kind, source); };
    return button;
  }
  function showSpeakerProfile(kind, source) {
    var hq = kind === 'hq' || source.kind === 'hq' || isHQPane(source), pane = source.pane || source.pane_id;
    var live = pane ? byId(lastAgents, pane) : source.session_id && lastAgents.find(function (a) { return a.session_id === source.session_id && (a.home_id || '') === (source.home_id || ''); });
    // An old stitched turn identifies its own agent; never assign another tool's live state.
    if (source.profileTurn || source.kind === 'agent' || source.kind === 'hq' || live && source.agent && live.agent && source.agent.toLowerCase() !== live.agent.toLowerCase()) live = null;
    var agent = live || {agent: source.agent}, name = kind === 'user' ? T('User', '用户') : source.label || (hq ? 'gtmux HQ' : source.agent || agent.agent || 'Agent');
    var sheet = browserDialog(T('Profile', '资料')); sheet.el.classList.add('participant-profile');
    var copy = window.GtmuxParticipantPresentation, human = kind === 'user';
    if (human) name = copy.human.name[ZH ? 1 : 0];
    var hero = document.createElement('div'); hero.className = 'profile-hero' + (human ? ' profile-human' : ' profile-agent');
    hero.appendChild(kind === 'user' ? userAvatarEl(56) : avatarEl(Object.assign({}, agent, hq ? {role: 'supervisor'} : {}), 40, false));
    var identity = el('div', 'profile-identity'); hero.appendChild(identity);
    var heading = document.createElement('h3'); heading.textContent = name; identity.appendChild(heading);
    var role = el('p', '', human ? copy.human.subtitle[ZH ? 1 : 0] : hq ? T('gtmux coordinating agent', 'gtmux 协调 Agent') : source.profileTurn || source.kind ? T('Recorded for this message', '这条消息的记录') : T('Current conversation · recorded facts', '当前会话 · 已记录资料'));
    identity.appendChild(role); sheet.body.appendChild(hero);
    var facts = document.createElement('dl'); facts.className = 'profile-facts';
    function fact(label, value) { if (!value) return; var dt = document.createElement('dt'); dt.textContent = label; var dd = document.createElement('dd'); dd.textContent = value; facts.appendChild(dt); facts.appendChild(dd); }
    if (human) {
      var quote = el('blockquote', 'profile-quote'); quote.appendChild(el('p', '', '“' + copy.human.quote[ZH ? 1 : 0] + '”')); quote.appendChild(el('cite', '', copy.human.attribution[ZH ? 1 : 0])); sheet.body.appendChild(quote);
    } else {
      if (hq) {
        sheet.body.appendChild(el('p', 'profile-hq-description', T('Coordinates agents, follows progress, reviews their reports and brings decisions back to you.', '协调各个 Agent 的工作，跟进进度、审阅回执，把需要你决定的事带回来。')));
        fact(source.kind === 'hq' ? T('Agent at delivery', '发信时的 Agent') : T('Agent', 'Agent'), source.agent || T('Not recorded', '未记录'));
        sheet.body.appendChild(facts);
        if (source.kind === 'hq') {
          var currentHQ = el('details', 'profile-provenance'); currentHQ.appendChild(el('summary', '', T('Current HQ', '当前 HQ')));
          var supervisors = lastAgents.filter(isHQPane); currentHQ.appendChild(el('p', '', supervisors.length === 1 ? supervisors[0].agent : T('No confirmed connected instance', '暂无已确认的在线实例')));
          currentHQ.appendChild(el('p', '', T('Current information; the agent at delivery may differ.', '当前信息；发信时的 Agent 可能不同。'))); sheet.body.appendChild(currentHQ);
        }
      }
      if (kind === 'agent') loadSpeakerFacts(sheet, source, heading);
    }
  }
  function loadSpeakerFacts(sheet, source, heading) {
    var turn = source.profileTurn, pane = source.pane_id, session = turn ? turn.session_id : source.session_id;
    var note = el('p', 'profile-note'); note.setAttribute('role','status'); sheet.body.appendChild(note);
    if (!SHARE.all) { note.textContent = T('Recorded profiles are unavailable for this connection.', '此连接无法查看已记录资料。'); return; }
    if (!source.agent || turn && (!turn.session_id || !turn.time) || !pane && !session) { note.textContent = T('No recorded profile is available for this message.', '未记录这条消息当时的资料。'); return; }
    var request = {id: isNative(source) ? '' : pane, session_id: session, home_id: source.home_id, at: turn && turn.time};
    var query = Object.keys(request).filter(function (key) { return request[key]; }).map(function (key) { return key + '=' + encodeURIComponent(request[key]); }).join('&');
    var credential = token, controller = new AbortController(), closed = false, timer, attempt = 0;
    sheet.onClose = function () { closed = true; attempt++; clearTimeout(timer); controller.abort(); };
    var labels = {}; Object.keys(window.GtmuxParticipantPresentation.labels).forEach(function (key) { labels[key] = window.GtmuxParticipantPresentation.labels[key][ZH ? 1 : 0]; });
    var sources = {session_log:T('Session log','本会话日志'),verified_binding:T('Verified client binding','已核验的客户端绑定'),session_rate_limits:T('Rate limits recorded in this session','本会话当时的额度记录'),runtime_package:T('Runtime package','运行实例加载包'),native_hook:T('Native hook','原生 hook')};
    function current(generation) { return !closed && sheet.el.open && credential === token && (generation === undefined || generation === attempt); }
    function failed(generation) {
      if (!current(generation)) return;
      note.textContent = T('Recorded profile unavailable. You can retry.', '暂时无法读取已记录资料，可重试。');
      var retry = el('button', 'dialog-secondary', T('Retry', '重试')); retry.type = 'button'; retry.onclick = read; note.appendChild(retry);
    }
    function read() {
      if (!current()) return;
      var generation = ++attempt;
      controller.abort(); controller = new AbortController(); var requestController = controller;
      note.textContent = T('Loading recorded profile…', '正在读取已记录资料…');
      clearTimeout(timer); var ownTimer = setTimeout(function () { requestController.abort(); failed(generation); }, 15000); timer = ownTimer;
      api('/api/participant-profile?' + query, {signal: requestController.signal}).then(function (response) {
        if (!current(generation)) return null;
        if (response.status === 401) { token = null; gate('expired'); return null; }
        if (!response.ok) throw new Error('profile'); return response.json();
      }).then(function (profile) {
        if (!profile || !current(generation)) return;
        clearTimeout(ownTimer);
        if (typeof profile.agent !== 'string' || typeof profile.name !== 'string' || !Array.isArray(profile.facts) || session && profile.session_id !== session) throw new Error('identity');
        heading.textContent = isHQPane(source) ? 'gtmux HQ' : profile.name || source.agent;
        var values = profile.facts.slice(0, 32).filter(function (value) { return value && typeof value.key === 'string' && Object.prototype.hasOwnProperty.call(labels,value.key) && typeof value.value === 'string' && value.value && typeof value.source === 'string'; });
        values.sort(function (a,b) { return Object.keys(labels).indexOf(a.key) - Object.keys(labels).indexOf(b.key); });
        note.textContent = profile.historical ? T('Recorded for this message', '这条消息的记录') : T('Current conversation · recorded facts', '当前会话 · 已记录资料');
        if (!values.length) { note.textContent = T('Version, model and other details were not recorded for this participant.', '这位参与者的版本、模型等细节未记录。'); renderProfileChain(sheet, source, profile); return; }
        var facts = el('div','profile-grid profile-recorded'), provenance = el('details','profile-provenance');
        provenance.appendChild(el('summary','',T('Sources and observation times','来源与记录时间')));
        values.forEach(function (value) {
          var brands = {openai:'OpenAI',anthropic:'Anthropic',deepseek:'DeepSeek','deepseek-official':'DeepSeek','deepseek-account':'DeepSeek Account',google:'Google'};
          var item = el('dl', 'profile-field'); item.appendChild(el('dt','',labels[value.key])); item.appendChild(el('dd','',value.key === 'provider' ? brands[value.value.toLowerCase()] || value.value : value.value)); facts.appendChild(item);
          var line = el('p','',labels[value.key] + ' · ' + (sources[value.source] || T('Recorded source','已记录来源')));
          if (Number.isFinite(value.observed_at) && value.observed_at > 0) line.appendChild(el('time','', ' · ' + new Date(value.observed_at).toLocaleString()));
          provenance.appendChild(line);
        });
        note.hidden = true; sheet.body.appendChild(facts); renderProfileChain(sheet, source, profile); sheet.body.appendChild(provenance);
      }).catch(function () { clearTimeout(ownTimer); failed(generation); });
    }
    read();
  }
  function renderProfileChain(sheet, source, profile) {
    var section = el('details', 'profile-provenance profile-chain'), content = el('div');
    section.appendChild(el('summary', '', T('Execution chain', '执行链'))); section.appendChild(content); sheet.body.appendChild(section);
    var turn = source.profileTurn, credential = token, closed = false, controller, timer, loaded = false;
    var priorClose = sheet.onClose; sheet.onClose = function () { closed = true; clearTimeout(timer); if (controller) controller.abort(); if (priorClose) priorClose(); };
    function current() { return !closed && sheet.el.open && token === credential; }
    function render(value) {
      if (!current()) return; content.textContent = '';
      var chain = value.historical === false && Array.isArray(value.execution_chain) ? value.execution_chain : [];
      content.appendChild(el('p', '', chain.length ? turn ? T('Latest observations for this conversation, not the processes at this message’s time.', '当前会话的最近观察，不代表这条消息当时的进程。') : T('Recorded launch ancestry; processes may have changed since.', '最近记录的启动链；进程可能已有变化。') : T('No launch chain has been recorded for this conversation.', '此会话尚未记录启动链。')));
      if (chain.length && Number.isFinite(value.chain_observed_at)) content.appendChild(el('p', '', new Date(value.chain_observed_at).toLocaleString()));
      chain.slice(0, 16).forEach(function (process) {
        if (!process || typeof process.name !== 'string') return;
        var command = [typeof process.path === 'string' ? process.path : process.name].concat(Array.isArray(process.arguments) ? process.arguments.filter(function (v) { return typeof v === 'string'; }) : []).join(' ');
        var row = el('div', 'profile-process'); row.appendChild(el('strong', '', process.name));
        var line = el('div', 'profile-command'), status = el('span', 'action-status'), button = el('button', 'profile-copy'); status.setAttribute('role', 'status');
        button.type = 'button'; button.innerHTML = uiIcon('copy'); button.setAttribute('aria-label', T('Copy sanitized command observation', '复制脱敏命令记录'));
        button.onclick = function () { copyWithFeedback(button, command, button.getAttribute('aria-label'), 'copy', status); };
        line.appendChild(el('pre', '', command)); line.appendChild(button); row.appendChild(line); row.appendChild(status); content.appendChild(row);
      });
      if (chain.length) content.appendChild(el('p', '', T('Other arguments omitted. These observations are not replayable commands.', '非展示参数已隐藏，记录不可直接重放。')));
    }
    function failed() {
      if (!current()) return; content.textContent = '';
      content.appendChild(el('p', '', T('Launch-chain observations could not be verified. Try again.', '暂时无法核对启动链，请重试。')));
      var retry = el('button', 'dialog-secondary', T('Retry', '重试')); retry.type = 'button'; retry.onclick = read; content.appendChild(retry);
    }
    function read() {
      loaded = true; clearTimeout(timer); if (controller) controller.abort(); controller = new AbortController();
      content.textContent = T('Reading launch-chain observations…', '正在读取启动链…');
      var request = {id: isNative(source) ? '' : source.pane_id, session_id: turn.session_id, home_id: source.home_id};
      var query = Object.keys(request).filter(function (key) { return request[key]; }).map(function (key) { return key + '=' + encodeURIComponent(request[key]); }).join('&');
      var requestController = controller, ownTimer = setTimeout(function () { if (controller === requestController) { requestController.abort(); failed(); } }, 15000); timer = ownTimer;
      api('/api/participant-profile?' + query, {signal: requestController.signal}).then(function (r) { if (!current() || controller !== requestController) return; if (r.status === 401) { token = null; gate('expired'); return; } if (!r.ok) throw Error('chain'); return r.json(); }).then(function (value) {
        clearTimeout(ownTimer); if (!current() || controller !== requestController || !value) return;
        if (value.historical !== false || value.session_id !== turn.session_id || value.agent !== profile.agent || value.unavailable && !(value.execution_chain || []).length) throw Error('chain identity');
        render(value);
      }).catch(function () { clearTimeout(ownTimer); if (controller === requestController) failed(); });
    }
    var canRead = turn && turn.session_id && (!source.session_id || source.session_id === turn.session_id) && (isNative(source) ? source.session_id : source.pane_id);
    if (!turn) render(profile);
    else if (!canRead) content.textContent = T('Launch chain not recorded for this message.', '这条消息未记录启动链。');
    else { content.textContent = T('Latest observations for this conversation', '当前会话最近记录'); section.addEventListener('toggle', function () { if (section.open && !loaded && current()) read(); }); }
  }
  // One optical grid and stroke weight for navigation, composer and message actions.
  function uiIcon(kind) {
    // Lucide 0.468.0 (ISC); attribution: vendor/lucide.LICENSE.
    var paths = {
      "sun": "<circle cx=\"12\" cy=\"12\" r=\"4\"/><path d=\"M12 2v2m0 16v2M2 12h2m16 0h2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42\"/>",
      "moon": "<path d=\"M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13Z\"/>",
      "monitor": "<rect x=\"2\" y=\"3\" width=\"20\" height=\"14\" rx=\"2\"/><path d=\"M8 21h8m-4-4v4\"/>",
      "terminal": "<rect x=\"3\" y=\"3\" width=\"18\" height=\"18\" rx=\"2\"/><path d=\"m7 8 4 4-4 4m6 0h4\"/>",

      "file": "<path d=\"M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z\"/><path d=\"M14 2v6h6M8 13h8M8 17h6\"/>",
      "tools": "<path d=\"M14 5a6 6 0 0 0-7 7l-5 5 5 5 5-5a6 6 0 0 0 7-7l-4 3-4-4 3-4Z\"/>",
      "trace": "<path d=\"M4 5h16M4 12h16M4 19h16\"/><circle cx=\"8\" cy=\"5\" r=\"2\"/><circle cx=\"16\" cy=\"12\" r=\"2\"/><circle cx=\"10\" cy=\"19\" r=\"2\"/>",
      "diff": "<path d=\"M8 3v12m-3-3 3 3 3-3M16 21V9m-3 3 3-3 3 3\"/>",
      "copy": "<rect width=\"14\" height=\"14\" x=\"8\" y=\"8\" rx=\"2\" ry=\"2\" /> <path d=\"M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2\" />",
      "done": "<path d=\"M20 6 9 17l-5-5\" />",
      "error": "<circle cx=\"12\" cy=\"12\" r=\"10\" /> <line x1=\"12\" x2=\"12\" y1=\"8\" y2=\"12\" /> <line x1=\"12\" x2=\"12.01\" y1=\"16\" y2=\"16\" />",
      "focus": "<path d=\"M8 3H5a2 2 0 0 0-2 2v3\" /> <path d=\"M21 8V5a2 2 0 0 0-2-2h-3\" /> <path d=\"M3 16v3a2 2 0 0 0 2 2h3\" /> <path d=\"M16 21h3a2 2 0 0 0 2-2v-3\" />",
      "close": "<path d=\"M18 6 6 18\" /> <path d=\"m6 6 12 12\" />",
      "sidebarClose": "<rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\" /> <path d=\"M9 3v18\" /> <path d=\"m16 15-3-3 3-3\" />",
      "sidebarOpen": "<rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\" /> <path d=\"M9 3v18\" /> <path d=\"m14 9 3 3-3 3\" />",
      "surface": "<path d=\"M15 3h6v6\" /> <path d=\"M10 14 21 3\" /> <path d=\"M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6\" />",
      "layout": "<rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\" /> <path d=\"M3 9h18\" /> <path d=\"M9 21V9\" />",
      "chevron": "<path d=\"m6 9 6 6 6-6\" />",
      "plus": "<path d=\"M5 12h14\" /> <path d=\"M12 5v14\" />",
      "send": "<path d=\"m5 12 7-7 7 7\" /> <path d=\"M12 19V5\" />",
      "left": "<path d=\"m15 18-6-6 6-6\" />",
      "right": "<path d=\"m9 18 6-6-6-6\" />",
      "up": "<path d=\"m5 12 7-7 7 7\" /> <path d=\"M12 19V5\" />",
      "down": "<path d=\"M12 5v14\" /> <path d=\"m19 12-7 7-7-7\" />",
      "keyboard": "<path d=\"M10 8h.01\" /> <path d=\"M12 12h.01\" /> <path d=\"M14 8h.01\" /> <path d=\"M16 12h.01\" /> <path d=\"M18 8h.01\" /> <path d=\"M6 8h.01\" /> <path d=\"M7 16h10\" /> <path d=\"M8 12h.01\" /> <rect width=\"20\" height=\"16\" x=\"2\" y=\"4\" rx=\"2\" />",
      "eye": "<path d=\"M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0\" /> <circle cx=\"12\" cy=\"12\" r=\"3\" />",
      "search": "<circle cx=\"11\" cy=\"11\" r=\"8\" /> <path d=\"m21 21-4.3-4.3\" />",
      "grip": "<circle cx=\"9\" cy=\"12\" r=\"1\" /> <circle cx=\"9\" cy=\"5\" r=\"1\" /> <circle cx=\"9\" cy=\"19\" r=\"1\" /> <circle cx=\"15\" cy=\"12\" r=\"1\" /> <circle cx=\"15\" cy=\"5\" r=\"1\" /> <circle cx=\"15\" cy=\"19\" r=\"1\" />",
      "back": "<path d=\"m12 19-7-7 7-7\" /> <path d=\"M19 12H5\" />",
      "settings": "<path d=\"M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z\" /> <circle cx=\"12\" cy=\"12\" r=\"3\" />",
      "return": "<polyline points=\"9 10 4 15 9 20\" /> <path d=\"M20 4v7a4 4 0 0 1-4 4H4\" />",
      "backspace": "<path d=\"M10 5a2 2 0 0 0-1.344.519l-6.328 5.74a1 1 0 0 0 0 1.481l6.328 5.741A2 2 0 0 0 10 19h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2z\" /> <path d=\"m12 9 6 6\" /> <path d=\"m18 9-6 6\" />",
      "expand": "<path d=\"m21 21-6-6m6 6v-4.8m0 4.8h-4.8\" /> <path d=\"M3 16.2V21m0 0h4.8M3 21l6-6\" /> <path d=\"M21 7.8V3m0 0h-4.8M21 3l-6 6\" /> <path d=\"M3 7.8V3m0 0h4.8M3 3l6 6\" />",
      "history": "<path d=\"M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8\" /> <path d=\"M3 3v5h5\" /> <path d=\"M12 7v5l4 2\" />",
      "snippets": "<path d=\"M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z\" /> <path d=\"M13 8H7\" /> <path d=\"M17 12H7\" />",
      "camera": "<path d=\"M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z\" /> <circle cx=\"12\" cy=\"13\" r=\"3\" />",
      "file": "<path d=\"M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z\" /> <path d=\"M14 2v4a2 2 0 0 0 2 2h4\" />",
      "image": "<rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\" ry=\"2\" /> <circle cx=\"9\" cy=\"9\" r=\"2\" /> <path d=\"m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21\" />",
      "paste": "<path d=\"M15 2H9a1 1 0 0 0-1 1v2c0 .6.4 1 1 1h6c.6 0 1-.4 1-1V3c0-.6-.4-1-1-1Z\" /> <path d=\"M8 4H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2M16 4h2a2 2 0 0 1 2 2v2M11 14h10\" /> <path d=\"m17 10 4 4-4 4\" />",
      "pen": "<path d=\"M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z\" /> <path d=\"m15 5 4 4\" />",
      "undo": "<path d=\"M9 14 4 9l5-5\" /> <path d=\"M4 9h10.5a5.5 5.5 0 0 1 5.5 5.5a5.5 5.5 0 0 1-5.5 5.5H11\" />",
      "square": "<rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\" />",
      "arrow": "<path d=\"M13 5H19V11\" /> <path d=\"M19 5L5 19\" />",
      "loading": "<path d=\"M21 12a9 9 0 1 1-6.219-8.56\" />",
      "redact": "<path d=\"M12 3v18\" /> <path d=\"M3 12h18\" /> <rect x=\"3\" y=\"3\" width=\"18\" height=\"18\" rx=\"2\" />",
      "clear": "<path d=\"M3 6h18\" /> <path d=\"M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6\" /> <path d=\"M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2\" /> <line x1=\"10\" x2=\"10\" y1=\"11\" y2=\"17\" /> <line x1=\"14\" x2=\"14\" y1=\"11\" y2=\"17\" />"
    };
    return '<svg class="ui-icon" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' + paths[kind] + '</svg>';
  }
  function bubbleActions(value) {
    var bar = document.createElement('div'); bar.className = 'bub-act';
    var status = document.createElement('span'); status.className = 'action-status'; status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
    function add(kind, label, text) {
      var b = document.createElement('button');
      b.type = 'button'; b.innerHTML = uiIcon(kind); b.title = label; b.setAttribute('aria-label', label);
      b.onclick = function (e) {
        e.stopPropagation(); copyWithFeedback(b, text, label, kind, status);
      }; bar.appendChild(b);
    }
    add('copy', T('Copy', '复制'), value);
    bar.appendChild(status);
    return bar;
  }
  function copyWithFeedback(button, value, label, kind, status) {
    var attempt = (button.copyAttempt || 0) + 1; button.copyAttempt = attempt; clearTimeout(button.copyReset);
    return copyText(value).then(function (ok) {
      if (button.copyAttempt !== attempt) return;
      var feedback = ok ? T('Copied', '已复制') : T('Copy failed', '复制失败');
      button.innerHTML = uiIcon(ok ? 'done' : 'error'); button.title = feedback; button.dataset.state = ok ? 'done' : 'error';
      if (status) status.textContent = feedback;
      button.copyReset = setTimeout(function () { button.innerHTML = uiIcon(kind); button.title = label; delete button.dataset.state; if (status) status.textContent = ''; }, 1600);
      return ok;
    });
  }
  function elButton(label, icon, action) {
    var button = el('button'); button.type = 'button'; button.title = label; button.setAttribute('aria-label', label); button.innerHTML = uiIcon(icon);
    button.onclick = function (event) { event.stopPropagation(); action(); }; return button;
  }
  // Tools sit above a session. The base Chat/Terminal DOM, draft and scroll stay mounted.
  function showSessionTools(agent, tile, kind) {
    if (!agent || !hasPane(agent) && !readableConversation(agent)) return;
    if (kind === 'diff' && !hasPane(agent) || kind === 'trace' && !agent.agent && !readableConversation(agent)) return;
    if (activeSessionTool) activeSessionTool.sheet.el.retire();
    var sheet = browserDialog(T('Session tools', '会话工具'));
    var state = {sheet: sheet, trace: {}, closed: false, kind: '', agent: Object.assign({}, agent), tile: tile};
    activeSessionTool = state;
    var epoch = viewEpoch, credential = token, timer = null, signature = '';
    sheet.onClose = function () {
      state.closed = true; clearInterval(timer); cancelRead('session-tool'); resetTrace(state.trace);
      if (activeSessionTool === state) activeSessionTool = null;
      document.body.classList.remove('has-session-tool');
      if (state.reflowPending && sheet.restoreFocus && credential === token && epoch === viewEpoch && $('pane').hidden && $('chat').hidden) home();
      updatePageTitle();
    };
    function valid() { return !state.closed && epoch === viewEpoch && credential === token && (!tile || !tile.removed); }
    function present(value) {
      if (!valid()) return;
      var next = JSON.stringify(value); if (signature === next) return; signature = next;
      if (state.kind === 'trace') {
        var roster = tile ? tile.agent : curAgent;
        var view = traceView(value, state.trace, Object.assign({}, roster || agent, {width: sheet.body.clientWidth, dropped: readableConversation(agent) ? externalDropped : 0}));
        if (view.parentNode !== sheet.body) { sheet.body.replaceChildren(view); }
      } else renderDiff(sheet.body, value.diff || '');
    }
    function poll() {
      if (!valid()) { sheet.el.retire(); return; }
      if (state.kind === 'trace') {
        if (tile && tile.mode === 'chat' && tile.chatSig) { signature = ''; present(tile.lastTurns || []); return; }
        if (!tile && paneMode === 'chat' && chatSig) { signature = ''; present(lastTurns); return; }
      }
      runRead('session-tool', function (signal) {
        var path = state.kind === 'diff' ? '/api/diff?id=' + encodeURIComponent(agent.pane_id)
          : hasPane(agent) ? '/api/transcript?id=' + encodeURIComponent(agent.pane_id) : '/api/session/transcript' + conversationQuery(agent);
        return api(path, {signal: signal}).then(function (response) {
          if (!valid() || signal.aborted) return null;
          if (response.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
          if (response.status === 403) { sheet.body.replaceChildren(el('p', 'chat-empty', T('This connection cannot view these records.', '当前连接没有查看权限。'))); state.trace = resetTrace(state.trace); clearInterval(timer); return null; }
          if (response.status === 404 && state.kind === 'trace') return [];
          if (!response.ok) throw new Error('tool unavailable');
          return response.json();
        }).then(function (value) { if (value !== null && valid() && !signal.aborted) present(value); })
          .catch(function () { if (valid() && !signature) sheet.body.replaceChildren(el('p', 'chat-empty', T('Could not load records. Close and try again.', '暂时无法读取，请关闭后重试。'))); });
      });
    }
    function choose(kind) {
      state.kind = kind; signature = ''; sheet.el.classList.add('session-tool'); updatePageTitle();
      sheet.el.querySelector('h2').textContent = (kind === 'trace' ? T('Trace', '轨迹') : 'Diff') + ' · ' + primary(agent);
      sheet.body.replaceChildren(brandLoaderEl(null)); document.body.classList.add('has-session-tool'); poll(); timer = setInterval(poll, 2500);
    }
    choose(kind);
  }
  function renderDiff(host, diff) {
    var pre = el('pre', 'session-diff');
    if (!diff) pre.textContent = T('No changes, or this session is outside a Git repository.', '没有改动，或当前会话不在 Git 仓库中。');
    else diff.split('\n').forEach(function (line) {
      var row = el('span', line[0] === '+' && line.indexOf('+++') !== 0 ? 'add' : line[0] === '-' && line.indexOf('---') !== 0 ? 'del' : '', line + '\n'); pre.appendChild(row);
    });
    host.replaceChildren(pre);
  }
  function resetTrace(state) { if (state && state.view) state.view.dispose(); return {}; }
  function traceView(turns, state, context) {
    if (!state.view) {
      state.view = window.GtmuxTraceView.create({T: T, markdown: mdRender, actions: bubbleActions, icon: uiIcon, formatTime: sepTime, status: function (agent) { return LABEL[agent.status] ? {text: agent.error ? LABEL.errored : LABEL[agent.status], color: agent.error ? '#F59E0B' : COLORS[agent.status]} : null; }});
      state.el = state.view.el;
    }
    state.view.update(turns, context || curAgent || {}); return state.el;
  }
  function collapseAllSteps() { chatExpanded = {}; drawChat(lastTurns); }
  // waiting approval card — shows the agent's 1/2/3 options (from /api/options).
  // When this caller may type into the pane (WEB §11), the rows are LIVE — one
  // click sends the digit (no Enter, like the phone's ApprovalCard). Otherwise
  // view-only with the reply-elsewhere hint. The server gate stays authoritative.
  function renderChatInteraction() {
    var root = $('chat-interaction');
    var visible = !!curPane && !$('chat').hidden && curAgent && curAgent.status === 'waiting';
    root.hidden = !visible;
    if (!visible) {root.innerHTML = ''; interactionRenderSig = ''; interactionCollapsed = false; interactionQuestion = ''; interactionPeek = false; return;}
    var question = lastOpts.length ? JSON.stringify(lastOpts) : '';
    if (question && question !== interactionQuestion) {interactionQuestion = question; interactionCollapsed = false;}
    var sig = JSON.stringify([curPane, curAgent && curAgent.status, lastOpts, lastInteraction, interactionFailed, interactionPeek, interactionCollapsed, paneCanInput(curPane)]);
    if (sig === interactionRenderSig) return;
    interactionRenderSig = sig;
    var previous = root.querySelector('.interaction-screen');
    var position = previous ? [previous.scrollLeft, previous.scrollTop] : null;
    root.innerHTML = ''; root.appendChild(approvalCard());
    var next = root.querySelector('.interaction-screen');
    if (next && position) {next.scrollLeft = position[0]; next.scrollTop = position[1];}
  }
  function approvalCard() {
    var can = paneCanInput(curPane), pane = curPane;
    var card = document.createElement('div'); card.className = 'appr-card' + (can ? ' appr-live' : '');
    var hd = el('div', 'appr-head');
    var title = el('button', 'interaction-toggle', T('Needs your reply', '需要你回应'));
    title.type = 'button'; title.setAttribute('aria-expanded', String(!interactionCollapsed)); title.onclick = function () {interactionCollapsed = !interactionCollapsed; renderChatInteraction();}; hd.appendChild(title);
    card.appendChild(hd);
    if (interactionCollapsed) return card;
    var opts = lastOpts || [];
    var body = el('div', 'interaction-body');
    opts.forEach(function (o) {
      var row = el('button', 'appr-opt');row.type = 'button';row.disabled = !can;
      row.appendChild(el('span', 'appr-key', String(o.n)));row.appendChild(el('span', 'appr-label', o.label || ''));
      row.onclick = function () {
        if (!can || curPane !== pane || !paneCanInput(pane) || Date.now() - interactionAnsweredAt < 1500 || paneComposer && paneComposer.keys && paneComposer.keys.blocked()) return;
        interactionAnsweredAt = Date.now();
        if (paneComposer && paneComposer.keys) paneComposer.keys.digit(o.n); else sendPane({text: String(o.n)});
        interactionCollapsed = true; lastOpts = []; renderChatInteraction();
      };body.appendChild(row);
    });
    if (opts.length) {var toggle = el('button', 'interaction-peek', interactionPeek ? T('Hide current prompt', '收起当前提示') : T('View current prompt', '查看当前提示'));toggle.type = 'button';toggle.onclick = function () {interactionPeek = !interactionPeek;renderChatInteraction();};body.appendChild(toggle);}
    if (interactionPeek || !opts.length) {
      body.appendChild(el('div', 'appr-empty', T('Current terminal screen', '当前终端画面')));
      if (lastInteraction && !interactionFailed) {
        if (lastInteraction.text) {var pre = el('pre', 'interaction-screen', lastInteraction.text);pre.tabIndex = 0;pre.setAttribute('aria-label', T('Current terminal screen', '当前终端画面'));body.appendChild(pre);}
        else body.appendChild(el('p', 'appr-empty', lastInteraction.unavailable ? T('This Mac does not provide the current screen. Keys remain available.', '这台 Mac 暂未提供当前画面，按键仍可使用。') : T('The current screen has no text. Use keys to continue.', '当前画面没有文字，可用按键继续操作。')));
        if (lastInteraction.truncated) body.appendChild(el('p', 'appr-empty', T('Long screen; only its final portion is shown.', '画面较长，仅显示末尾部分。')));
      } else {
        var retry = el('button', 'interaction-peek', interactionFailed ? T('Prompt unavailable. Retry', '暂时无法读取提示，重试') : T('Reading current prompt…', '正在读取当前提示…'));retry.type = 'button';retry.onclick = pollOptions;body.appendChild(retry);
      }
    }
    card.appendChild(body);
    if (!can) card.appendChild(el('div', 'appr-hint', T('This conversation is read-only.', '此会话为只读。')));
    return card;
  }

  function showConversationDocument(reference, scope) {
    var model = window.GtmuxDocuments, target = model && model.documentReference(reference);
    if (!target || !scope.session_id) return;
    var sheet = browserDialog(target.name || T('File', '文件')), credential = token, epoch = viewEpoch;
    sheet.el.classList.add('conversation-document');
    var controller = new AbortController(), objectURL = '', closed = false;
    sheet.onClose = function () { closed = true; controller.abort(); if (objectURL) URL.revokeObjectURL(objectURL); };
    function valid() { return !closed && credential === token && epoch === viewEpoch; }
    function unavailable(reason) {
      if (!valid()) return;
      sheet.body.replaceChildren(el('p', 'document-error', reason === 'unsupported' ? T('This format cannot be previewed in the browser.', '此格式暂不支持浏览器预览。') : model.documentErrorMessage(reason, ZH)));
      sheet.body.appendChild(clipboardAction(T('Copy path', '复制路径'), '', reference, 'document-path'));
    }
    sheet.body.appendChild(el('p', '', T('Opening file…', '正在打开文件…')));
    if (!SHARE.all) { unavailable('forbidden'); return; }
    api('/api/artifacts/resolve', {method: 'POST', redirect: 'error', signal: controller.signal, headers: {'Content-Type': 'application/json'}, body: JSON.stringify({reference: target.reference, session_id: scope.session_id, home_id: scope.home_id || ''})})
      .then(function (r) { if (r.status === 401 || r.status === 403) throw new Error('forbidden'); if (!r.ok) throw new Error('unpublished'); return r.json(); })
      .then(async function (value) {
        if (!valid()) return;
        var file = model.parseDocument(value, scope.session_id, scope.home_id || ''); if (!file) throw new Error('failed');
        var response = await api('/api/artifacts/content?id=' + encodeURIComponent(file.id) + '&session_id=' + encodeURIComponent(scope.session_id) + '&home_id=' + encodeURIComponent(scope.home_id || ''), {redirect: 'error', signal: controller.signal});
        if (!response.ok) throw new Error(response.status === 401 || response.status === 403 ? 'forbidden' : 'unpublished');
        var limit = file.kind === 'image' ? Math.min(model.MAX_DOCUMENT_BYTES, 8 * 1024 * 1024) : model.MAX_DOCUMENT_BYTES;
        if (file.bytes > limit || !response.body) throw new Error('failed');
        var reader = response.body.getReader(), chunks = [], length = 0;
        try { while (true) { var part = await reader.read(); if (part.done) break; length += part.value.length; if (length > limit || length > file.bytes || !valid()) { await reader.cancel(); throw new Error('failed'); } chunks.push(part.value); } } finally { reader.releaseLock(); }
        if (!valid()) return;
        var bytes = new Uint8Array(length), offset = 0; chunks.forEach(function (chunk) { bytes.set(chunk, offset); offset += chunk.length; });
        var hash = await crypto.subtle.digest('SHA-256', bytes), digest = Array.from(new Uint8Array(hash)).map(function (b) { return b.toString(16).padStart(2, '0'); }).join('');
        if (!valid()) return;
        if (length !== file.bytes || digest !== file.sha256) throw new Error('failed');
        sheet.el.querySelector('h2').textContent = file.name; sheet.body.replaceChildren();
        if (file.kind === 'image' && imageWithinBudget(bytes)) {
          var type = bytes[0] === 137 ? 'image/png' : 'image/jpeg';
          objectURL = URL.createObjectURL(new Blob([bytes], {type: type}));
          var image = el('img', 'document-image'); image.alt = file.name; image.src = objectURL;
          image.onerror = function () { unavailable('failed'); }; sheet.body.appendChild(image);
        } else if (file.kind === 'markdown' || file.kind === 'text') {
          var text = new TextDecoder().decode(bytes.slice(0, 512 * 1024));
          sheet.body.appendChild(file.kind === 'markdown' ? mdRender(text) : el('pre', 'document-text', text));
        } else { unavailable('unsupported'); }
      }).catch(function (error) { unavailable(['forbidden', 'unpublished', 'unsupported'].includes(error.message) ? error.message : 'failed'); });
  }
  // Inspect PNG/JPEG dimensions before allowing a decoder to allocate the bitmap.
  function imageWithinBudget(bytes) {
    var view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength), width = 0, height = 0;
    if (bytes.length >= 24 && view.getUint32(0) === 0x89504e47 && view.getUint32(4) === 0x0d0a1a0a && view.getUint32(12) === 0x49484452) { width = view.getUint32(16); height = view.getUint32(20); }
    else if (bytes.length >= 4 && bytes[0] === 255 && bytes[1] === 216) {
      var i = 2;
      while (i + 9 < bytes.length) {
        if (bytes[i++] !== 255) break; while (bytes[i] === 255) i++;
        var marker = bytes[i++]; if (marker === 0xda || marker === 0xd9) break;
        if (marker === 0x01 || marker >= 0xd0 && marker <= 0xd8) continue;
        if (i + 2 > bytes.length) break; var size = view.getUint16(i); if (size < 2 || i + size > bytes.length) break;
        if ([0xc0,0xc1,0xc2,0xc3,0xc5,0xc6,0xc7,0xc9,0xca,0xcb,0xcd,0xce,0xcf].includes(marker) && size >= 8) { height = view.getUint16(i + 3); width = view.getUint16(i + 5); break; } i += size;
      }
    }
    return width > 0 && height > 0 && width <= 16384 && height <= 16384 && width * height <= 24000000;
  }

  // ---- markdown (vanilla → DOM) -----------------------------------------
  // A tiny Markdown subset matching the phone's markdown.ts: **bold**/*italic*/
  // `code`/[links] inline; heading/para/fenced-code/list/quote/hr/pipe-table
  // blocks. Underscore emphasis is intentionally OFF (snake_case). Built as DOM
  // with textContent (never innerHTML) so agent output can't inject markup.
  function mdInline(s, context) {
    var frag = document.createDocumentFragment(), rest = String(s);
    while (rest.length) {
      var best = null;
      var probe = function (re, kind) {
        var m = re.exec(rest);
        if (m && (!best || m.index < best.idx)) best = {idx: m.index, len: m[0].length, kind: kind, m: m};
      };
      probe(/`([^`]+)`/, 'code');
      probe(/!?\[([^\]]+)\]\((<[^>]+>|[^)\s]+)\)/, 'link');
      probe(/\*\*([^*]+)\*\*/, 'b');
      probe(/\*([^*\n]+)\*/, 'i');
      if (!best) { frag.appendChild(document.createTextNode(rest)); break; }
      if (best.idx > 0) frag.appendChild(document.createTextNode(rest.slice(0, best.idx)));
      var m = best.m, el;
      if (best.kind === 'code') { el = document.createElement('code'); el.className = 'md-ic'; el.textContent = m[1]; }
      else if (best.kind === 'link') {
        var reference = m[2].replace(/^<|>$/g, '');
        if (context && context.session_id && window.GtmuxDocuments && window.GtmuxDocuments.documentReference(reference)) {
          el = document.createElement('button'); el.type = 'button'; el.className = 'conversation-file';
          el.innerHTML = uiIcon('file'); var filename = document.createElement('span'); filename.textContent = m[1]; el.appendChild(filename); el.title = T('Open file: ', '打开文件：') + m[1];
          (function (path, scope) { el.onclick = function () { showConversationDocument(path, scope); }; })(reference, context);
        } else { el = document.createElement('a'); if (/^https?:\/\//i.test(reference) || /^mailto:/i.test(reference)) el.href = reference; el.target = '_blank'; el.rel = 'noopener noreferrer'; el.textContent = m[1]; }
      }
      else if (best.kind === 'b') { el = document.createElement('strong'); el.textContent = m[1]; }
      else { el = document.createElement('em'); el.textContent = m[1]; }
      frag.appendChild(el);
      rest = rest.slice(best.idx + best.len);
    }
    return frag;
  }
  function mdRender(src, context) {
    var root = document.createElement('div'); root.className = 'md';
    var lines = String(src).replace(/\r\n/g, '\n').split('\n'), i = 0;
    var FENCE = /^\s*```(.*)$/, HR = /^\s*([-*_])(\s*\1){2,}\s*$/, HEADING = /^\s*(#{1,6})\s+(.*)$/;
    var QUOTE = /^\s*>\s?/, BULLET = /^\s*[-*+]\s+/, ORDERED = /^\s*\d+\.\s+/;
    var TABLE_SEP = /^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$/;
    function isTableStart(idx) { return lines[idx].indexOf('|') !== -1 && idx + 1 < lines.length && lines[idx + 1].indexOf('|') !== -1 && TABLE_SEP.test(lines[idx + 1]); }
    function trimPipes(s) { s = s.trim(); if (s.charAt(0) === '|') s = s.slice(1); if (s.charAt(s.length - 1) === '|') s = s.slice(0, -1); return s; }
    function splitRow(line) { return trimPipes(line).split('|').map(function (c) { return c.trim(); }); }
    function parseAlign(line) {
      return trimPipes(line).split('|').map(function (c) {
        var t = c.trim(), l = t.charAt(0) === ':', r = t.charAt(t.length - 1) === ':';
        return l && r ? 'center' : r ? 'right' : 'left';
      });
    }
    while (i < lines.length) {
      var line = lines[i], fm = FENCE.exec(line);
      if (fm) {
        var body = []; i++;
        while (i < lines.length && !/^\s*```/.test(lines[i])) { body.push(lines[i]); i++; }
        i++;
        var pre = document.createElement('pre'); pre.className = 'md-pre';
        var code = document.createElement('code'); code.textContent = body.join('\n'); pre.appendChild(code);
        root.appendChild(pre); continue;
      }
      if (line.trim() === '') { i++; continue; }
      if (HR.test(line)) { root.appendChild(document.createElement('hr')); i++; continue; }
      var h = HEADING.exec(line);
      if (h) { var he = document.createElement('div'); he.className = 'md-h md-h' + h[1].length; he.appendChild(mdInline(h[2].trim(), context)); root.appendChild(he); i++; continue; }
      if (QUOTE.test(line)) {
        var q = []; while (i < lines.length && QUOTE.test(lines[i])) { q.push(lines[i].replace(QUOTE, '')); i++; }
        var bq = document.createElement('blockquote'); bq.className = 'md-q'; bq.appendChild(mdInline(q.join(' '), context)); root.appendChild(bq); continue;
      }
      if (BULLET.test(line)) {
        var ul = document.createElement('ul'); ul.className = 'md-list';
        while (i < lines.length && BULLET.test(lines[i])) { var li = document.createElement('li'); li.appendChild(mdInline(lines[i].replace(BULLET, ''), context)); ul.appendChild(li); i++; }
        root.appendChild(ul); continue;
      }
      if (ORDERED.test(line)) {
        var ol = document.createElement('ol'); ol.className = 'md-list';
        while (i < lines.length && ORDERED.test(lines[i])) { var li2 = document.createElement('li'); li2.appendChild(mdInline(lines[i].replace(ORDERED, ''), context)); ol.appendChild(li2); i++; }
        root.appendChild(ol); continue;
      }
      if (isTableStart(i)) {
        var header = splitRow(lines[i]), align = parseAlign(lines[i + 1]); i += 2;
        var rows = []; while (i < lines.length && lines[i].trim() !== '' && lines[i].indexOf('|') !== -1) { rows.push(splitRow(lines[i])); i++; }
        var wrap = document.createElement('div'); wrap.className = 'md-tablewrap';
        var tbl = document.createElement('table'); tbl.className = 'md-table';
        var thead = document.createElement('thead'), htr = document.createElement('tr');
        header.forEach(function (c, ci) { var th = document.createElement('th'); th.style.textAlign = align[ci] || 'left'; th.appendChild(mdInline(c, context)); htr.appendChild(th); });
        thead.appendChild(htr); tbl.appendChild(thead);
        var tb = document.createElement('tbody');
        rows.forEach(function (r) { var tr = document.createElement('tr'); r.forEach(function (c, ci) { var td = document.createElement('td'); td.style.textAlign = align[ci] || 'left'; td.appendChild(mdInline(c, context)); tr.appendChild(td); }); tb.appendChild(tr); });
        tbl.appendChild(tb); wrap.appendChild(tbl); root.appendChild(wrap); continue;
      }
      var para = [];
      while (i < lines.length && lines[i].trim() !== '' && !/^\s*```/.test(lines[i]) && !HR.test(lines[i]) && !HEADING.test(lines[i]) && !QUOTE.test(lines[i]) && !BULLET.test(lines[i]) && !ORDERED.test(lines[i]) && !isTableStart(i)) { para.push(lines[i]); i++; }
      var p = document.createElement('p'); p.className = 'md-p'; p.appendChild(mdInline(para.join(' '), context)); root.appendChild(p);
    }
    return root;
  }

  // ======================================================================
  // Desktop workbench (WEB.md §1–§3): a session tree rail + a freeform board of
  // CONCURRENT pane mirrors. Wide screens only; <900px keeps the single column.
  // Tiles share the existing pane/transcript/diff APIs and scope-gated composer.
  // ======================================================================
  var WB = {on: false, agents: [], tiles: [], railW: 232, railCollapsed: false, snap: false, presetCur: ''};
  var WB_KEY = 'gtmux.board';
  function isWide() { return window.innerWidth >= 900; }
  function wbLoad() {
    try {
      var s = JSON.parse(localStorage.getItem(WB_KEY) || '{}');
      WB.railW = Math.max(170, Math.min(360, s.railW || 232));
      WB.railCollapsed = !!s.railCollapsed; WB.snap = !!s.snap;
      WB.saved = Array.isArray(s.tiles) ? s.tiles : [];
    } catch (e) { WB.saved = []; }
  }
  function wbSave() {
    try {
      localStorage.setItem(WB_KEY, JSON.stringify({
        railW: WB.railW, railCollapsed: WB.railCollapsed, snap: WB.snap,
        tiles: WB.tiles.map(function (t) { return {id: t.id, x: t.x, y: t.y, w: t.w, h: t.h, mode: t.mode}; }),
      }));
    } catch (e) {}
  }

  function startWorkbench() {
    cancelReads(); viewEpoch++;
    WB.on = true; updatePageTitle('workbench');
    ['gate', 'radar', 'panes', 'pane', 'chat', 'chat-input'].forEach(function (id) { $(id).hidden = true; });
    clearInterval(panesTimer); panesTimer = null;
    $('workbench').hidden = false; $('session-toolbar').hidden = true;
    $('bar').hidden = true; // the workbench has its own #wb-bar
    $('back').hidden = true; $('mode').hidden = true; $('gear').hidden = true; hideFocusChrome();
    clearInterval(paneTimer); paneTimer = null; clearInterval(chatTimer); chatTimer = null;
    applyRail();
    $('board').classList.toggle('snap', WB.snap);
    setToggle($('wb-snap'), WB.snap);
    updatePresetLabel();
    WB.tiles.forEach(tileInputSync);
    updateEmpty();
    // restore saved tiles once agents arrive (need their status/agent); poll now.
    pollWB(); clearInterval(radarTimer); radarTimer = setInterval(pollWB, 2000);
  }
  function setToggle(btn, on) { btn.classList.toggle('on', !!on); btn.setAttribute('aria-pressed', String(!!on)); }
  function setWbConn(ok) { renderConn($('wb-conn'), connStateFor(ok)); }

  function pollWB() {
    runRead('workbench', function (signal) {
      var current = currentRead('workbench', signal);
      return api('/api/agents', {signal: signal}).then(function (r) {
        if (!current()) return null;
        if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
        if (!r.ok) throw new Error('agents'); rememberServerClock(r); return r.json();
      }).then(function (agents) {
        if (!agents || !current(true)) return; setWbConn(true); WB.agents = agents; lastAgents = agents;
        if (WB.saved) { restoreTiles(agents); WB.saved = null; }
        renderTree(agents); fetchShare();
        // refresh each tile's header status + waiting border
        WB.tiles.forEach(function (t) { var a = byId(agents, t.id); if (a) { t.agent = a; updateTileHead(t); } });
      }).catch(function () { if (current()) setWbConn(false); });
    }, 15000, function () { setWbConn(false); });
  }
  function flashTile(t) {
    if (!t || !t.el) return;
    t.el.classList.add('flash'); t.el.style.zIndex = ++zTop;
    try { t.el.scrollIntoView({block: 'nearest', behavior: 'smooth'}); } catch (e) {}
    setTimeout(function () { if (t.el) t.el.classList.remove('flash'); }, 1600);
  }
  function byId(agents, id) { for (var i = 0; i < agents.length; i++) if (agents[i].pane_id === id) return agents[i]; return null; }
  function restoreTiles(agents) {
    WB.saved.forEach(function (s) { var a = byId(agents, s.id); if (a) addTile(a, s); });
  }

  // ---- tree -------------------------------------------------------------
  function renderTree(agents) {
    var focused = document.activeElement, focusedPane = focused && focused.dataset && focused.dataset.treePane, focusedSection = focused && focused.dataset && focused.dataset.section;
    var q = ($('rail-search').value || '').trim().toLowerCase();
    var match = function (a) { return !q || (primary(a) + ' ' + (a.agent || '') + ' ' + (a.session || '') + ' ' + (a.pane_id || '')).toLowerCase().indexOf(q) !== -1; };
    // Every section in ORDER needs its own bucket. `errored` had none, so the loop
    // below read `undefined.length` on its pass and threw: the rail rendered the first
    // group and stopped, and the failure also fell through to the poll's catch, which
    // parked the connection dot on "reconnecting" while the board kept updating.
    var by = {waiting: [], errored: [], working: [], idle: []}, natives = [], supervisors = [];
    agents.forEach(function (a) {
      if (!match(a)) return;
      if (isHQPane(a)) { supervisors.push(a); return; }
      if (!hasPane(a)) { natives.push(a); return; }
      // Same rule as the radar: an idle turn that ended on a failure is not finished.
      if (a.status === 'idle' && a.error) { by.errored.push(a); return; }
      by[sectionStatus(a.status)].push(a);
    });
    var root = $('tree'); root.innerHTML = '';
    if (supervisors.length) {
      var hq = rosterSection(root, 'hq', T('Headquarters', '中控'), supervisors.length, true, 'tree-hq');
      hq.parentElement.setAttribute('aria-label', 'gtmux HQ');
      supervisors.forEach(function (a) { hq.appendChild(treeRow(a, false)); });
    }
    var nWait = (by.waiting || []).length + supervisors.filter(function (a) { return a.status === 'waiting'; }).length;
    $('rail-tab-n').textContent = nWait ? nWait : '';
    ORDER.forEach(function (st) {
      var list = by[st]; if (!list.length) return;
      var body = rosterSection(root, st, LABEL[st], list.length, true);
      // group by window (session:window) so a window expands to its panes
      var wins = {}; var order = [];
      list.forEach(function (a) { var w = (a.session || '') + ':' + ((a.loc || '').split(':')[1] || '').split('.')[0]; if (!wins[w]) { wins[w] = []; order.push(w); } wins[w].push(a); });
      order.forEach(function (w) {
        var panes = wins[w];
        if (panes.length > 1) {
          var wh = document.createElement('div'); wh.className = 'tree-win';
          wh.innerHTML = '<span>▾</span><span>' + esc(w) + '</span>'; body.appendChild(wh);
        }
        panes.forEach(function (a) { body.appendChild(treeRow(a, panes.length > 1)); });
      });
    });
    if (natives.length) {
      var external = natives.filter(function (a) { return a.current_client === 'unconfirmed' || a.client === 'chatgpt_desktop'; });
      var elsewhere = natives.filter(function (a) { return external.indexOf(a) < 0; });
      [['native', T('Elsewhere', '不在 tmux'), elsewhere], ['external', T('External conversations', '外部会话'), external]].forEach(function (part) {
        if (!part[2].length) return; var body = rosterSection(root, part[0], part[1], part[2].length, true); part[2].forEach(function (a) { body.appendChild(treeRow(a, false)); });
      });
    }
    if (!root.children.length) { var e = document.createElement('div'); e.className = 'tree-group'; e.textContent = q ? T('No matching sessions', '未找到匹配的会话') : emptyAgentsText(); root.appendChild(e); }
    if (focusedSection) { var sectionButton = root.querySelector('[data-section="' + focusedSection + '"]'); if (sectionButton) sectionButton.focus(); }
    if (focusedPane) root.querySelectorAll('.tree-row').forEach(function (row) { if (row.dataset.treePane === focusedPane) row.focus(); });
  }
  function esc(s) { var d = document.createElement('span'); d.textContent = s == null ? '' : s; return d.innerHTML; }
  function treeRow(a, nested) {
    var st = COLORS[a.status] ? a.status : 'running';
    var row = document.createElement('div'); row.className = 'tree-row' + (nested ? ' nested' : '') + (st === 'waiting' ? ' waiting' : '');
    row.draggable = hasPane(a);
    row.appendChild(avatarEl(a, 24, true));
    var tx = document.createElement('div'); tx.className = 'tr-text';
    var nm = document.createElement('div'); nm.className = 'tr-name'; nm.textContent = primary(a);
    var sb = document.createElement('div'); sb.className = 'tr-sub'; sb.textContent = secondary(a);
    rowSummary(a, sb);
    tx.appendChild(nm); tx.appendChild(sb); row.appendChild(tx);
    if (!hasPane(a)) { row.classList.add('native'); if(readableConversation(a)){row.onclick=function(){openConversation(a);};row.tabIndex=0;row.setAttribute('role','button');row.onkeydown=function(e){if(e.key==='Enter'||e.key===' '){e.preventDefault();openConversation(a);}};} return row; }
    var g = document.createElement('span'); g.className = 'tr-grip'; g.innerHTML = uiIcon('grip'); row.appendChild(g);
    row.addEventListener('dragstart', function (e) { e.dataTransfer.setData('text/pane', a.pane_id); e.dataTransfer.effectAllowed = 'copy'; });
    row.ondblclick = function () { addTile(a); };
    row.dataset.treePane = a.pane_id;
    keyboardLink(row, function () { var tile = addTile(a); if (tile) tile.el.querySelector('[data-action="focus"]').focus(); });
    return row;
  }
  function keyboardLink(element, activate) {
    element.tabIndex = 0; element.setAttribute('role', 'link');
    element.addEventListener('keydown', function (e) {
      if (e.target === element && e.key === 'Enter') { e.preventDefault(); e.stopPropagation(); activate(); }
    });
  }
  function shortAgent(s) { var w = String(s).split(' ')[0]; return w.length > 8 ? w.slice(0, 8) : w; }

  // ---- rail collapse / resize -------------------------------------------
  function applyRail() {
    $('workbench').classList.toggle('rail-collapsed', WB.railCollapsed);
    $('rail-tab').hidden = !WB.railCollapsed;
    if (!WB.railCollapsed) $('rail').style.width = WB.railW + 'px';
  }
  function setupRail() {
    $('rail-collapse').onclick = function () { WB.railCollapsed = true; applyRail(); wbSave(); $('rail-tab').focus(); };
    $('rail-tab').onclick = function () { WB.railCollapsed = false; applyRail(); wbSave(); $('rail-collapse').focus(); };
    $('rail-search').oninput = function () { renderTree(WB.agents); };
    var rz = $('rail-resize'), dragging = false;
    rz.addEventListener('mousedown', function (e) { e.preventDefault(); dragging = true; document.body.style.cursor = 'col-resize'; });
    document.addEventListener('mousemove', function (e) {
      if (!dragging) return;
      WB.railW = Math.max(170, Math.min(360, e.clientX - $('rail').getBoundingClientRect().left));
      $('rail').style.width = WB.railW + 'px';
    });
    document.addEventListener('mouseup', function () { if (dragging) { dragging = false; document.body.style.cursor = ''; wbSave(); } });
    $('wb-snap').onclick = function () { WB.snap = !WB.snap; setToggle($('wb-snap'), WB.snap); $('board').classList.toggle('snap', WB.snap); wbSave(); };
    // layout presets dropdown
    $('wb-preset').onclick = function (e) { e.stopPropagation(); var m = $('wb-preset-menu'); if (m.hidden) showPresetMenu(); else hidePresetMenu(); };
    document.addEventListener('click', function (e) { if (!$('wb-preset-menu').hidden && !$('wb-preset-wrap').contains(e.target)) hidePresetMenu(); });
    $('wb-preset-wrap').addEventListener('keydown', function (e) { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); hidePresetMenu(); $('wb-preset').focus(); } });
    // ⌘K command palette input: ↑/↓ select, Enter pick, Esc close
    var ci = $('cmdk-input');
    ci.addEventListener('input', function () { renderCmdk(ci.value); });
    ci.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { e.preventDefault(); closeCmdk(); }
      else if (e.key === 'Tab') { e.preventDefault(); ci.focus(); }
      else if (e.key === 'ArrowDown') { e.preventDefault(); if (cmdkSel < cmdkRows.length - 1) { cmdkSel++; markCmdk(); } }
      else if (e.key === 'ArrowUp') { e.preventDefault(); if (cmdkSel > 0) { cmdkSel--; markCmdk(); } }
      else if (e.key === 'Enter') { e.preventDefault(); if (cmdkRows[cmdkSel]) pickCmdk(cmdkRows[cmdkSel]); }
    });
    $('cmdk').addEventListener('mousedown', function (e) { if (e.target === $('cmdk')) closeCmdk(); }); // click backdrop
    // accept drops from the tree
    var board = $('board');
    board.addEventListener('dragover', function (e) { if (e.dataTransfer.types.indexOf('text/pane') !== -1) { e.preventDefault(); e.dataTransfer.dropEffect = 'copy'; } });
    board.addEventListener('drop', function (e) {
      var id = e.dataTransfer.getData('text/pane'); if (!id) return; e.preventDefault();
      var a = byId(WB.agents, id); if (!a) return;
      var r = board.getBoundingClientRect();
      addTile(a, {x: e.clientX - r.left - 60, y: e.clientY - r.top - 16});
    });
  }

  // ---- tiles ------------------------------------------------------------
  var SNAP = 20;
  function snapv(v) { return WB.snap ? Math.round(v / SNAP) * SNAP : v; }
  function addTile(a, pos) {
    if (!hasPane(a)) return null;
    pos = pos || {};
    var existing = null; WB.tiles.forEach(function (t) { if (t.id === a.pane_id) existing = t; });
    if (existing) { existing.el.style.zIndex = ++zTop; return existing; }
    var board = $('board'), br = board.getBoundingClientRect();
    var t = {
      id: a.pane_id, agent: a, mode: pos.mode === 'chat' || pos.mode === 'trace' ? 'chat' : 'term',
      x: pos.x != null ? Math.max(0, pos.x) : 14 + (WB.tiles.length % 4) * 28,
      y: pos.y != null ? Math.max(0, pos.y) : 14 + (WB.tiles.length % 4) * 28,
      w: pos.w || 560, h: pos.h || 480,
      term: null, fit: null, lastText: '', pending: null, scrolling: false, scrollIdle: null, timer: null, chatSig: '', expanded: {},
    };
    buildTile(t); WB.tiles.push(t); board.classList.remove('has-empty'); updateEmpty();
    setTileMode(t, t.mode); wbSave();
    return t;
  }
  var zTop = 10;
  function removeTile(t) {
    cancelRead('tile:' + t.id);
    cancelRead('live:' + t.id);
    t.removed = true;
    clearInterval(t.timer); if (t.inputDispose) t.inputDispose(); if (t.term) { try { t.term.dispose(); } catch (e) {} }
    t.el.remove(); WB.tiles = WB.tiles.filter(function (x) { return x !== t; });
    updateEmpty(); wbSave();
  }
  function updateEmpty() {
    var board = $('board'), ex = board.querySelector('.board-empty');
    if (!WB.tiles.length) { if (!ex) {
      var e = el('div', 'board-empty'), welcome = el('div', 'workspace-welcome');
      var mark = el('span', 'workspace-mark'); mark.innerHTML = uiIcon('layout'); welcome.appendChild(mark);
      welcome.appendChild(el('h1', '', T('Start with a session', '从一个会话开始')));
      welcome.appendChild(el('p', 'workspace-help', T('Choose a session from the sidebar, or bring several together here.', '打开左侧会话，在这里一起查看和处理。')));
      var choose = el('button', 'dialog-primary', T('Choose a session', '选择会话')); choose.type = 'button'; choose.onclick = openCmdk; welcome.appendChild(choose);
      var quote = el('blockquote', 'workspace-quote');
      quote.appendChild(el('p', '', T('“A beginning is the time for taking the most delicate care…”', '“初始之际，尤须审慎……”')));
      var cite = el('cite'), source = el('a', '', T('Frank Herbert · Dune · opening epigraph (excerpt)', '弗兰克·赫伯特 ·《沙丘》开篇题记（节译）'));
      source.href = 'https://sites.prh.com/dune/read-an-excerpt'; source.target = '_blank'; source.rel = 'noopener noreferrer'; cite.appendChild(source); quote.appendChild(cite);
      welcome.appendChild(quote); e.appendChild(welcome); board.appendChild(e);
    } }
    else if (ex) ex.remove();
  }
  function buildTile(t) {
    var el = document.createElement('div'); el.className = 'tile'; t.el = el;
    el.style.left = t.x + 'px'; el.style.top = t.y + 'px'; el.style.width = t.w + 'px'; el.style.height = t.h + 'px'; el.style.zIndex = ++zTop;
    var head = document.createElement('div'); head.className = 'tile-head';
    head.appendChild(t.agent.agent ? speakerAvatar('agent', t.agent, avatarEl(t.agent, 24, true)) : avatarEl(t.agent, 24, false));
    var identity = document.createElement('div'); identity.className = 'tile-identity'; head.appendChild(identity);
    var nm = document.createElement('span'); nm.className = 'tile-name'; nm.textContent = primary(t.agent); identity.appendChild(nm);
    var meta = document.createElement('div'); meta.className = 'tile-meta'; identity.appendChild(meta);
    var id = document.createElement('span'); id.className = 'tile-id'; id.textContent = t.id; meta.appendChild(id);
    // Read-only capabilities remain visible when there is no composer.
    var cap = document.createElement('span'); cap.className = 'cap-chip'; cap.hidden = true; t.capEl = cap; meta.appendChild(cap);
    var controls = document.createElement('div'); controls.className = 'tile-controls';
    var modes = document.createElement('span'); modes.className = 'tile-modes';
    // "diff" stays a developer's word in both languages, as git and every review tool use
    // it; its tooltip says what it shows.
    var choices = [['term', T('Terminal', '终端')]];
    if (t.agent.agent) choices.unshift(['chat', T('Chat', '对话')]);
    choices.forEach(function (m) {
      var b = document.createElement('button'); b.textContent = m[1]; b.setAttribute('data-m', m[0]); if (m[2]) b.title = m[2];
      b.onclick = function (e) { e.stopPropagation(); setTileMode(t, m[0]); }; modes.appendChild(b);
    });
    controls.appendChild(modes);
    var tools = document.createElement('div'); tools.className = 'session-actions';
    (t.agent.agent ? ['trace', 'diff'] : ['diff']).forEach(function (kind) {
      var label = kind === 'trace' ? T('Trace', '轨迹') : 'Diff';
      var button = elButton(label, kind, function () { showSessionTools(t.agent, t, kind); });
      button.dataset.tool = kind; button.appendChild(document.createTextNode(label)); tools.appendChild(button);
    });
    controls.appendChild(tools);
    var windowControls = document.createElement('div'); windowControls.className = 'tile-window-controls';
    var max = document.createElement('button'); max.type = 'button'; max.className = 'tile-btn'; max.dataset.action = 'focus'; max.innerHTML = uiIcon('focus'); max.title = T('Open conversation', '打开会话'); max.setAttribute('aria-label', max.title); max.onclick = function (e) { e.stopPropagation(); openAgent(t.agent, t.mode); }; windowControls.appendChild(max);
    var cl = document.createElement('button'); cl.type = 'button'; cl.className = 'tile-btn'; cl.innerHTML = uiIcon('close'); cl.title = T('Close panel', '关闭面板'); cl.setAttribute('aria-label', cl.title); cl.onclick = function (e) { e.stopPropagation(); removeTile(t); }; windowControls.appendChild(cl);
    head.appendChild(windowControls); head.appendChild(controls);
    el.appendChild(head);
    var body = document.createElement('div'); body.className = 'tile-body'; t.body = body; el.appendChild(body);
    // shared-input row (web-shared-input): the SAME composer as the single-pane view,
    // in its compact variant — shown for a term-mode tile the host allowed.
    var tc = makeComposer((function (tid) { return function () { return tid; }; })(t.id), function (text) { if (t.term) tileWrite(t, text); }, true);
    tc.el.hidden = true; t.inputEl = tc.el; t.composer = tc; t.inputSync = tc.sync; t.inputDispose = tc.dispose;
    el.appendChild(tc.el);
    // read-only note — states WHY there's no input row instead of leaving a gap
    var ro = document.createElement('div'); ro.className = 'tile-ro'; ro.hidden = true;
    ro.textContent = T('🔒 Input is not allowed in this pane', '🔒 此窗格不允许输入'); t.roEl = ro; el.appendChild(ro);
    var rz = document.createElement('div'); rz.className = 'tile-resize'; rz.textContent = '⌟'; el.appendChild(rz);
    el.addEventListener('mousedown', function () { el.style.zIndex = ++zTop; });
    dragMove(t, head); dragResize(t, rz);
    $('board').appendChild(el);
    t.headNm = nm; t.headAv = head.querySelector('.avatar');
    updateTileHead(t);
  }
  function updateTileHead(t) {
    var st = COLORS[t.agent.status] ? t.agent.status : 'running';
    t.el.classList.toggle('waiting', st === 'waiting');
    if (t.headNm) t.headNm.textContent = primary(t.agent);
    if (t.composer && !t.inputEl.hidden) t.composer.sync();
    var bdg = t.headAv && t.headAv.querySelector('.badge');
    if (bdg) { bdg.innerHTML = badgeSVG(st, t.agent.error); bdg.setAttribute('aria-label', t.agent.error ? LABEL.errored : LABEL[st]); }
    var host = t.body && t.body.querySelector('.conversation-main');
    if (t.mode === 'chat' && host) syncChatLive(host, t.agent, function () { return WB.on && !t.removed && t.mode === 'chat' && t.agent.status === 'working'; });
  }
  function setTileMode(t, m) {
    if (m !== 'chat' || !t.agent.agent) m = 'term';
    t.mode = m;
    cancelRead('tile:' + t.id);
    cancelRead('live:' + t.id);
    t.readEpoch = (t.readEpoch || 0) + 1;
    Array.prototype.forEach.call(t.el.querySelectorAll('.tile-modes button'), function (b) { var selected = b.getAttribute('data-m') === m; b.classList.toggle('on', selected); b.setAttribute('aria-pressed', String(selected)); });
    clearInterval(t.timer); t.timer = null; t.body.innerHTML = '';
    if (t.term) { try { t.term.dispose(); } catch (e) {} t.term = null; }
    t.body.appendChild(brandLoaderEl(null)); // cleared on first frame (or by a chat/diff re-render)
    if (m === 'term') { tileTermStart(t); }
    else { t.chatSig = ''; tilePollChat(t); t.timer = setInterval(function () { tilePollChat(t); }, 2500); }
    tileInputSync(t); // show the input row only in term mode for an allowed pane
    wbSave();
  }

  // per-tile terminal (own xterm; same incremental write + scroll-lock as #pane)
  function tileTermStart(t) {
    var xt = document.createElement('div'); xt.className = 'xt'; t.body.appendChild(xt);
    t.term = new Terminal({convertEol: true, cursorBlink: false, disableStdin: true, cursorInactiveStyle: 'none', scrollback: 4000, allowProposedApi: true, fontFamily: resolveFont(), fontSize: termSize(), theme: xtermTheme()});
    t.fit = new FitAddon.FitAddon(); t.term.loadAddon(t.fit);
    try { var u = new Unicode11Addon.Unicode11Addon(); t.term.loadAddon(u); t.term.unicode.activeVersion = '11'; } catch (e) {}
    t.term.open(xt); t.lastText = '';
    var mark = function () { t.scrolling = true; clearTimeout(t.scrollIdle); t.scrollIdle = setTimeout(function () { t.scrolling = false; if (t.pending !== null) { var x = t.pending; t.pending = null; tileWrite(t, x); } }, 900); };
    xt.addEventListener('wheel', mark, {passive: true});
    setTimeout(function () { try { t.fit.fit(); } catch (e) {} }, 0);
    tilePollPane(t); t.timer = setInterval(function () { tilePollPane(t); }, 1300);
  }
  function tileWrite(t, text) {
    text = normalize(text || ''); if (text === t.lastText) return;
    if (t.scrolling) { t.pending = text; return; }
    var prev = t.lastText; t.lastText = text;
    if (prev && text.length > prev.length && text.lastIndexOf(prev, 0) === 0) { t.term.write(text.slice(prev.length)); return; }
    var b = t.term.buffer.active, wasBottom = b.viewportY >= b.baseY, fromBottom = b.baseY - b.viewportY;
    t.term.reset();
    t.term.write(text, function () { var nb = t.term.buffer.active; if (wasBottom) t.term.scrollToBottom(); else { try { t.term.scrollToLine(Math.max(0, nb.baseY - fromBottom)); } catch (e) {} } });
  }
  function tilePollPane(t) {
    runRead('tile:' + t.id, function (signal) {
      var epoch = t.readEpoch;
      return api('/api/pane?id=' + encodeURIComponent(t.id), {signal: signal}).then(function (r) { return r.ok ? r.json() : null; })
        .then(function (j) {
          if (!signal.aborted && j && !t.removed && epoch === t.readEpoch && t.mode === 'term' && t.term) {
            t.el.classList.remove('off'); tileWrite(t, j.text);
            var ld = t.body.querySelector('.brandload'); if (ld) ld.remove(); // first frame in
          }
        })
        .catch(function () {});
    });
  }

  function tilePollChat(t) {
    if (t.mode === 'chat') { var host = t.body.querySelector('.conversation-main'); if (host) syncChatLive(host, t.agent, function () { return WB.on && !t.removed && t.mode === 'chat' && t.agent.status === 'working'; }); }
    runRead('tile:' + t.id, function (signal) {
      var epoch = t.readEpoch;
      return api('/api/transcript?id=' + encodeURIComponent(t.id), {signal: signal}).then(function (r) { return (r.status === 404) ? [] : (r.ok ? r.json() : null); })
        .then(function (turns) { if (signal.aborted || turns === null || t.removed || epoch !== t.readEpoch || t.mode !== 'chat') return; var sig = JSON.stringify(turns || []); if (sig === t.chatSig) return; t.chatSig = sig; renderTileChat(t, turns || []); })
        .catch(function () {});
    });
  }

  function renderTileChat(t, turns) {
    t.lastTurns = turns; reconcilePendingPrompts(t.id, turns);
    var preserved = snapshotToolState(t.body);
    var old = t.body.querySelector('.tile-chat');
    var atBottom = !old || old.scrollHeight - old.scrollTop - old.clientHeight < 48;
    var top = old ? old.scrollTop : 0;
    var wrap = document.createElement('div'); wrap.className = 'tile-chat';
    if (!turns.length && !(pendingPrompts[t.id] || []).length) { var e = document.createElement('div'); e.className = 'chat-empty'; e.textContent = T('No conversation yet.', '还没有对话。'); wrap.appendChild(e); }
    var seps = sepLabels(turns);
    turns.forEach(function (tn, idx) { wrap.appendChild(conversationTurn(tn, t.agent || {}, idx, t.expanded || (t.expanded = {}), seps[idx], false, function () { renderTileChat(t, t.lastTurns || []); })); });
    appendPendingPrompts(wrap, t.id, t.agent || {});
    var main = document.createElement('div'); main.className = 'conversation-main'; main.appendChild(wrap);
    t.body.innerHTML = ''; t.body.appendChild(main);
    syncChatLive(main, t.agent || {}, function () { return WB.on && !t.removed && t.mode === 'chat' && t.agent.status === 'working'; });
    wrap.scrollTop = atBottom ? wrap.scrollHeight : top;
    restoreToolState(t.body, preserved);
  }
  // drag-move (head) + resize (corner), with optional snap-to-grid.
  function dragMove(t, handle) {
    handle.addEventListener('mousedown', function (e) {
      if (e.target.closest('button')) return; e.preventDefault();
      if (maxedTile) return; // maximized → head is not draggable
      var sx = e.clientX, sy = e.clientY, ox = t.x, oy = t.y, moved = false;
      function mv(ev) { if (Math.abs(ev.clientX - sx) + Math.abs(ev.clientY - sy) > 4) moved = true; t.x = Math.max(0, snapv(ox + ev.clientX - sx)); t.y = Math.max(0, snapv(oy + ev.clientY - sy)); t.el.style.left = t.x + 'px'; t.el.style.top = t.y + 'px'; }
      function up() { document.removeEventListener('mousemove', mv); document.removeEventListener('mouseup', up); if (moved) wbSave(); else maximizeTile(t); }
      document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up);
    });
  }
  function dragResize(t, handle) {
    handle.addEventListener('mousedown', function (e) {
      e.preventDefault(); e.stopPropagation();
      var sx = e.clientX, sy = e.clientY, ow = t.w, oh = t.h;
      function mv(ev) { t.w = Math.max(180, snapv(ow + ev.clientX - sx)); t.h = Math.max(120, snapv(oh + ev.clientY - sy)); t.el.style.width = t.w + 'px'; t.el.style.height = t.h + 'px'; if (t.fit && t.mode === 'term') { try { t.fit.fit(); } catch (e2) {} } }
      function up() { document.removeEventListener('mousemove', mv); document.removeEventListener('mouseup', up); if (t.fit && t.mode === 'term' && t.lastText) { var x = t.lastText; t.lastText = ''; tileWrite(t, x); } wbSave(); }
      document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up);
    });
  }

  // ---- focus pane (§02): toolbar + jump-to-latest + read-only reply bar ----
  function persistSize() { try { localStorage.setItem('gtmux.fontSize', String(sizePref)); } catch (e) {} }
  function focusVisibleText() {
    var b = term.buffer.active, out = [];
    for (var i = 0; i < term.rows; i++) { var ln = b.getLine(b.viewportY + i); if (ln) out.push(ln.translateToString(true).replace(/\s+$/, '')); }
    return out.join('\n').replace(/\n+$/, '');
  }
  function copyScreen() {
    if (!term) return;
    copyWithFeedback($('copy-screen'), term.getSelection() || focusVisibleText(), T('Copy visible screen', '复制屏幕内容'), 'copy', $('screen-copy-status'));
  }
  function cyclePane(d) {
    var panes = lastAgents.filter(hasPane);
    if (!curAgent || !panes.length) return;
    var i = -1; for (var k = 0; k < panes.length; k++) if (panes[k].pane_id === curAgent.pane_id) { i = k; break; }
    if (i < 0) return; openAgent(panes[(i + d + panes.length) % panes.length], paneMode);
  }
  // jump pill — shown when scrolled up; a dot marks new content while away from bottom.
  function updateJump(newContent) {
    if ($('pane').hidden || paneMode !== 'term' || !term) { $('jump').hidden = true; return; }
    var b = term.buffer.active, atBottom = b.viewportY >= b.baseY - 1;
    if (atBottom) { $('jump').hidden = true; $('jump').querySelector('.jdot').hidden = true; }
    else { $('jump').hidden = false; if (newContent) $('jump').querySelector('.jdot').hidden = false; }
  }
  // waiting options — drive BOTH the term reply bar and the chat approval card.
  function pollOptions() {

    if (!curPane) return;
    var inTerm = !$('pane').hidden && paneMode === 'term';
    var inChat = !$('chat').hidden && paneMode !== 'term';
    if (!inTerm && !inChat) return;
    if (!curAgent || curAgent.status !== 'waiting') {
      cancelRead('options');
      lastInteraction = null; interactionFailed = false; renderChatInteraction();
      if (inTerm) hideReply();
      if (lastOpts.length) { lastOpts = []; lastOptsSig = ''; if (inChat) drawChat(lastTurns); }
      return;
    }
    // The answer is for the pane it was asked about: one that arrives after the reader has
    // moved to another pane is dropped. A request that FAILS clears the choices like an
    // empty answer does: the old ones stayed live and a click still sent their digit
    // (%12's re-verification of #1385).
    runRead('options', function (signal) {
      var asked = curPane, current = currentRead('options', signal);
      var apply = function (opts) {
        if (!current(true) || curPane !== asked || !curAgent || curAgent.status !== 'waiting') return;
        if (inTerm) { renderReply(opts); return; }
        var sig = JSON.stringify(opts); // chat: only redraw the card when options change
        if (sig !== lastOptsSig) { lastOptsSig = sig; lastOpts = opts; drawChat(lastTurns); }
      };
      var route = inChat ? '/api/interaction?id=' : '/api/options?id=';
      return api(route + encodeURIComponent(asked), {signal: signal}).then(function (r) {
        if (inChat && (r.status === 404 || r.status === 501)) return api('/api/options?id=' + encodeURIComponent(asked), {signal: signal}).then(function (old) {if (!old.ok) throw new Error('prompt');return old.json();}).then(function (j) {return {text: '', options: j.options || [], unavailable: true};});
        if (!r.ok) throw new Error('prompt');return r.json();
      }).then(function (j) {
        if (!current(true) || curPane !== asked) return;
        if (inChat) {lastInteraction = j;interactionFailed = false;lastOpts = Date.now() - interactionAnsweredAt < 1500 ? [] : j.options || [];renderChatInteraction();}
        else apply(j.options || []);
      }, function () {
        if (!current(true) || curPane !== asked) return;
        if (inChat) {lastInteraction = null;interactionFailed = true;lastOpts = [];renderChatInteraction();} else apply([]);
      });
    }, 15000, function () { setConn(false); hideReply(); lastInteraction = null; interactionFailed = true; lastOpts = []; lastOptsSig = ''; if (paneMode === 'chat') renderChatInteraction(); });
  }

  function renderReply(opts) {
    var had = !$('reply-bar').hidden;
    $('pane').classList.add('has-reply'); $('reply-bar').hidden = false;
    // A typable pane's 1/2/3 are LIVE buttons (one tap → /api/send, digit with no
    // Enter — same as the phone's ApprovalCard); a read-only pane keeps the
    // view-only chips + the reply-elsewhere hint. Server gate stays authoritative.
    var can = paneCanInput(curPane);
    var lead = document.querySelector('#reply-bar .rb-lead');
    if (lead) lead.textContent = can
      ? T('Reply:', '回复：')
      : T('Read-only · awaiting reply:', '只读 · 等待回复：');
    var hint = document.querySelector('#reply-bar .rb-hint');
    if (hint) hint.hidden = can;
    var box = $('reply-opts'); box.innerHTML = '';
    // Only the choices the server parsed are buttons. With none, this drew 1 Yes /
    // 2 Always / 3 No anyway, and a click typed that digit into whatever was asking: a
    // waiting pane may be asking an open question, and the server answers an empty list
    // exactly when nothing on screen takes a number (2026-10-06 audit, %12).
    if (!opts.length) {
      var none = document.createElement('span'); none.className = 'rb-none';
      none.textContent = can
        ? T('Enter your reply below', '请在下方输入回复')
        : T('No reply options', '暂无回复选项');
      box.appendChild(none);
    }
    opts.forEach(function (o) {
      var s = document.createElement(can ? 'button' : 'span'); if (can) s.type = 'button'; s.className = can ? 'rb-opt live' : 'rb-opt'; s.textContent = o.n + ' ' + (o.label || '');
      if (can) s.onclick = function () { sendPane({text: String(o.n)}); };
      box.appendChild(s);
    });
    if (term) $('reply-size').textContent = term.cols + ' × ' + term.rows;
    if (!had) { try { fit.fit(); } catch (e) {} }
  }
  function hideReply() { if ($('reply-bar').hidden) return; $('reply-bar').hidden = true; $('pane').classList.remove('has-reply'); try { fit.fit(); } catch (e) {} }
  function showFocusChrome(isTerm) {
    $('focus-nav').hidden = false;
    $('focus-ctl').hidden = !isTerm;
    if (!isTerm) { hideReply(); $('jump').hidden = true; }
    // both term (reply bar) and chat (approval card) poll the waiting options.
    lastOpts = []; lastOptsSig = '';
    pollOptions(); clearInterval(optTimer); optTimer = setInterval(pollOptions, 2000);
  }
  function hideFocusChrome() { interactionQuestion = ''; lastInteraction = null; interactionFailed = false; interactionPeek = false; interactionCollapsed = false; interactionRenderSig = ''; $('chat-input').hidden = true; $('chat-interaction').hidden = true; $('focus-nav').hidden = true; $('focus-ctl').hidden = true; hideReply(); $('jump').hidden = true; var cap = $('cap'); if (cap) cap.hidden = true; clearInterval(optTimer); optTimer = null; lastOpts = []; lastOptsSig = ''; }
  // placeSettings opens the appearance panel just under the bar its button sits in. A fixed
  // top sat over the bar's own bottom row, and over the controls once a narrow bar wraps.
  function placeSettings(bar) {
    var b = bar && bar.getBoundingClientRect();
    if (b && b.height) { var top = Math.round(b.bottom + 4) + 'px'; $('settings').style.top = top; $('settings').style.setProperty('--settings-top', top); }
  }
  function setupFocus() {
    ['trace', 'diff'].forEach(function (kind) { $('session-' + kind).onclick = function () { showSessionTools(curAgent, null, kind); }; });
    $('copy-screen').onclick = copyScreen;
    $('pane-prev').onclick = function () { cyclePane(-1); };
    $('pane-next').onclick = function () { cyclePane(1); };
    // Shared composer (multiline + attach/upload + key strip) → POST /api/send. The
    // server gate (guest consent + allowlist) stays authoritative; this just targets
    // the current pane and echoes the post-send screen.
    paneComposer = makeComposer(function () { return curPane; }, function (text) { if (paneMode === 'term') { renderPane(text); hidePaneLoader(); } else { pollChat(); } }, false, function (action) {
      if (action.id === 'codex.view-question' && paneMode !== 'term') {paneMode = 'term'; applyMode();}
    });
    if ($('pane-input')) $('pane-input').appendChild(paneComposer.el);
    $('jump').onclick = function () { if (term) term.scrollToBottom(); updateJump(false); };
  }

  // ---- board: single-click a tile to maximize it to fill the board (§05) ----
  var maxedTile = null;
  // `f` — fullscreen the front-most tile (highest z); toggles if already max.
  function focusTopTile() {
    if (maxedTile) { restoreBoard(); return; }
    if (!WB.tiles.length) return;
    var top = WB.tiles[0];
    WB.tiles.forEach(function (t) { if ((parseInt(t.el.style.zIndex, 10) || 0) >= (parseInt(top.el.style.zIndex, 10) || 0)) top = t; });
    maximizeTile(top);
  }
  function maximizeTile(t) {
    if (maxedTile === t) { restoreBoard(); return; }
    if (maxedTile) restoreBoard();
    maxedTile = t; t.prevRect = {w: t.w, h: t.h};
    var board = $('board'), br = board.getBoundingClientRect();
    board.classList.add('maximized'); t.el.classList.add('max'); t.el.style.zIndex = ++zTop;
    t.el.style.width = (br.width - 16) + 'px'; t.el.style.height = (br.height - 16) + 'px';
    refitTile(t);
    var ex = document.createElement('button'); ex.type = 'button'; ex.className = 'max-exit'; ex.id = 'max-exit'; ex.innerHTML = uiIcon('back') + T('Restore layout', '恢复布局'); ex.onclick = restoreBoard; board.appendChild(ex);
  }
  function restoreBoard() {
    if (!maxedTile) return; var t = maxedTile; maxedTile = null;
    $('board').classList.remove('maximized'); t.el.classList.remove('max');
    t.el.style.width = t.prevRect.w + 'px'; t.el.style.height = t.prevRect.h + 'px';
    var ex = document.getElementById('max-exit'); if (ex) ex.remove();
    refitTile(t);
  }
  function refitTile(t) {
    if (t.fit && t.mode === 'term') setTimeout(function () { try { t.fit.fit(); } catch (e) {} if (t.lastText) { var x = t.lastText; t.lastText = ''; tileWrite(t, x); } }, 0);
  }

  // ---- layout presets (WEB.md §7) ---------------------------------------
  // A preset = a named board layout (which panes, position, size, mode) + rail
  // state. Stored in localStorage; the top-bar dropdown + [ ] switch between them.
  var PRESETS_KEY = 'gtmux.presets';
  function loadPresets() { try { var a = JSON.parse(localStorage.getItem(PRESETS_KEY) || '[]'); return Array.isArray(a) ? a : []; } catch (e) { return []; } }
  function savePresets(a) { try { localStorage.setItem(PRESETS_KEY, JSON.stringify(a)); } catch (e) {} }
  function captureLayout() {
    return {
      tiles: WB.tiles.map(function (t) { return {id: t.id, x: t.x, y: t.y, w: t.w, h: t.h, mode: t.mode}; }),
      railW: WB.railW, railCollapsed: WB.railCollapsed, snap: WB.snap,
    };
  }
  function applyPreset(p) {
    if (!p) return;
    if (maxedTile) restoreBoard();
    WB.tiles.slice().forEach(removeTile); // clear board
    (p.tiles || []).forEach(function (s) { var a = byId(WB.agents, s.id); if (a) addTile(a, s); });
    if (typeof p.railW === 'number') { WB.railW = Math.max(170, Math.min(360, p.railW)); }
    if (typeof p.railCollapsed === 'boolean') WB.railCollapsed = p.railCollapsed;
    if (typeof p.snap === 'boolean') { WB.snap = p.snap; $('board').classList.toggle('snap', WB.snap); setToggle($('wb-snap'), WB.snap); }
    applyRail(); WB.presetCur = p.name; updatePresetLabel(); wbSave();
  }
  function updatePresetLabel() { $('wb-preset-cur').textContent = WB.presetCur ? ' · ' + WB.presetCur : ''; }
  function cyclePreset(dir) {
    var ps = loadPresets(); if (!ps.length) return;
    var i = -1; for (var k = 0; k < ps.length; k++) if (ps[k].name === WB.presetCur) { i = k; break; }
    applyPreset(ps[(i + dir + ps.length) % ps.length]);
  }
  function renderPresetMenu() {
    var menu = $('wb-preset-menu'); menu.innerHTML = '';
    var ps = loadPresets();
    if (!ps.length) { var em = document.createElement('div'); em.className = 'pm-empty'; em.textContent = T('No saved layouts', '暂无已保存的布局'); menu.appendChild(em); }
    ps.forEach(function (p) {
      var row = document.createElement('div'); row.className = 'pm-row' + (p.name === WB.presetCur ? ' on' : '');
      var nm = document.createElement('button'); nm.type = 'button'; nm.className = 'pm-name'; nm.textContent = p.name;
      nm.onclick = function () { applyPreset(p); hidePresetMenu(); }; row.appendChild(nm);
      var del = document.createElement('button'); del.type = 'button'; del.className = 'pm-del'; del.innerHTML = uiIcon('close'); del.title = T('Delete layout: ', '删除布局：') + p.name; del.setAttribute('aria-label', del.title);
      del.onclick = function (e) { e.stopPropagation(); var rest = loadPresets().filter(function (x) { return x.name !== p.name; }); savePresets(rest); if (WB.presetCur === p.name) { WB.presetCur = ''; updatePresetLabel(); } renderPresetMenu(); };
      row.appendChild(del); menu.appendChild(row);
    });
    var save = document.createElement('button'); save.type = 'button'; save.className = 'pm-save'; save.innerHTML = uiIcon('plus') + T('Save current layout…', '保存当前布局…');
    save.onclick = function () {
      var name = (window.prompt(T('Layout name', '布局名称'), WB.presetCur || (T('Layout ', '布局 ') + (ps.length + 1))) || '').trim();
      if (!name) return;
      var arr = loadPresets().filter(function (x) { return x.name !== name; });
      var lay = captureLayout(); lay.name = name; arr.push(lay); savePresets(arr);
      WB.presetCur = name; updatePresetLabel(); renderPresetMenu(); hidePresetMenu();
    };
    menu.appendChild(save);
  }
  function showPresetMenu() { renderPresetMenu(); $('wb-preset-menu').hidden = false; $('wb-preset').setAttribute('aria-expanded', 'true'); }
  function hidePresetMenu() { $('wb-preset-menu').hidden = true; $('wb-preset').setAttribute('aria-expanded', 'false'); }

  // ---- ⌘K command palette (WEB.md §8) -----------------------------------
  var cmdkSel = 0, cmdkRows = [], cmdkTrigger = null;
  function openCmdk() {
    if ($('workbench').hidden) return; // workbench-only
    cmdkTrigger = document.activeElement;
    $('cmdk').hidden = false; var inp = $('cmdk-input'); inp.setAttribute('aria-expanded', 'true'); inp.value = ''; cmdkSel = 0;
    renderCmdk(''); setTimeout(function () { inp.focus(); }, 0);
  }
  function closeCmdk() { var wasOpen = !$('cmdk').hidden; $('cmdk').hidden = true; $('cmdk-input').setAttribute('aria-expanded', 'false'); $('cmdk-input').setAttribute('aria-activedescendant', ''); if (wasOpen && cmdkTrigger && cmdkTrigger.focus) cmdkTrigger.focus(); }
  function renderCmdk(q) {
    q = (q || '').trim().toLowerCase();
    var list = $('cmdk-list'); list.innerHTML = '';
    cmdkRows = (WB.agents || []).filter(hasPane).filter(function (a) {
      return !q || (primary(a) + ' ' + (a.agent || '') + ' ' + (a.session || '') + ' ' + (a.pane_id || '')).toLowerCase().indexOf(q) !== -1;
    }).slice(0, 50);
    if (cmdkSel >= cmdkRows.length) cmdkSel = Math.max(0, cmdkRows.length - 1);
    cmdkRows.forEach(function (a, i) {
      var row = document.createElement('div'); row.className = 'ck-row' + (i === cmdkSel ? ' on' : '');
      row.id = 'cmdk-result-' + i; row.setAttribute('role', 'option'); row.setAttribute('aria-selected', String(i === cmdkSel));
      row.appendChild(avatarEl(a, 22, true));
      var tx = document.createElement('div'); tx.className = 'ck-tx';
      var nm = document.createElement('div'); nm.className = 'ck-nm'; nm.textContent = primary(a); tx.appendChild(nm);
      var sb = document.createElement('div'); sb.className = 'ck-sb'; sb.textContent = a.pane_id + ' · ' + (a.agent || '') + ' · ' + (LABEL[a.status] || a.status || ''); tx.appendChild(sb);
      row.appendChild(tx);
      var dot = document.createElement('span'); dot.className = 'ck-dot'; dot.style.background = COLORS[a.status] || COLORS.running; row.appendChild(dot);
      row.onmousemove = function () { if (cmdkSel !== i) { cmdkSel = i; markCmdk(); } };
      row.onclick = function () { pickCmdk(a); };
      list.appendChild(row);
    });
    $('cmdk-input').setAttribute('aria-activedescendant', cmdkRows.length ? 'cmdk-result-' + cmdkSel : '');
    $('cmdk-status').textContent = cmdkRows.length + T(cmdkRows.length === 1 ? ' session found' : ' sessions found', ' 个匹配会话');
    if (!cmdkRows.length) { var e = document.createElement('div'); e.className = 'ck-empty'; e.textContent = q ? T('no pane matches that', '没有匹配的窗格') : emptyAgentsText(); list.appendChild(e); }
  }
  function markCmdk() { Array.prototype.forEach.call($('cmdk-list').children, function (c, i) { c.classList.toggle('on', i === cmdkSel); c.setAttribute('aria-selected', String(i === cmdkSel)); }); $('cmdk-input').setAttribute('aria-activedescendant', cmdkRows.length ? 'cmdk-result-' + cmdkSel : ''); }
  function pickCmdk(a) { closeCmdk(); var t = addTile(a); flashTile(t); maximizeTile(t); }

  // ---- boot -------------------------------------------------------------
  // setupSettings wires the appearance panel (font + size), persisted locally.
  var settingsTrigger = null;
  function syncSettings() {
    $('size-rng').value = String(termSize()); $('size-val').textContent = termSize() + ' px'; $('size-rng').setAttribute('aria-valuetext', termSize() + ' px');
    $('chat-size-rng').value = String(chatSizePref); $('chat-size-val').textContent = chatSizePref + ' px'; $('chat-size-rng').setAttribute('aria-valuetext', chatSizePref + ' px');
    var hint = $('return-send-hint'); hint.textContent = '';
    var sends = $('return-sends').checked;
    hint.appendChild(el('kbd', '', sends ? 'Shift + Enter' : '⌘/Ctrl + Enter')); hint.appendChild(document.createTextNode(sends ? T(' new line', ' 换行') : T(' sends', ' 发送')));
  }
  function setSettingsOpen(open, trigger) {
    $('settings').hidden = !open;
    if (trigger) settingsTrigger = trigger;
    ['gear', 'wb-gear'].forEach(function (id) { $(id).setAttribute('aria-expanded', String(open && $(id) === settingsTrigger)); });
  }
  function toggleSettings(anchor, trigger) { placeSettings(anchor); setSettingsOpen($('settings').hidden, trigger); if (!$('settings').hidden) { syncSettings(); $('appearance-options').querySelector('input:checked').focus(); } }
  function setupSettings() {
    applyAppearance();
    var gear = $('gear'), panel = $('settings'), sel = $('font-sel'), rng = $('size-rng');
    gear.hidden = false;
    var appearance = $('appearance-options');
    appearance.onchange = function () { appearancePref = appearance.querySelector('input:checked').value; try { localStorage.setItem('gtmux.appearance', appearancePref); } catch (e) {} applyWebAppearance(); };
    sel.value = fontPref;
    function syncSize() { syncSettings(); }
    syncSize();
    panel.setAttribute('aria-label', T('Settings', '设置'));
    var returnSend = $('return-sends'); returnSend.checked = lsGet('gtmux.returnSends', '0') === '1'; syncSettings();
    returnSend.onchange = function () { try { localStorage.setItem('gtmux.returnSends', returnSend.checked ? '1' : '0'); } catch (e) {} document.querySelectorAll('.cx-input').forEach(function (input) { input.setAttribute('enterkeyhint', returnSend.checked ? 'send' : 'enter'); }); syncSettings(); };
    if (panel.dataset.ready) return; panel.dataset.ready = '1';
    panel.querySelectorAll('[data-appearance-icon]').forEach(function (node) { node.innerHTML = uiIcon(node.dataset.appearanceIcon); });
    var settingsBody = panel.querySelector('.settings-body'), navigation = panel.querySelector('nav');
    function markSection(id) { navigation.querySelectorAll('button').forEach(function (button) { if (button.dataset.settingsSection === id) button.setAttribute('aria-current', 'location'); else button.removeAttribute('aria-current'); }); }
    navigation.onclick = function (event) {
      var button = event.target.closest('[data-settings-section]'); if (!button) return;
      var target = $(button.dataset.settingsSection); target.scrollIntoView({block: 'start'}); target.focus({preventScroll: true}); markSection(target.id);
    };
    settingsBody.addEventListener('scroll', function () {
      var top = settingsBody.getBoundingClientRect().top + 28, selected = 'settings-appearance-section';
      settingsBody.querySelectorAll('fieldset').forEach(function (section) { if (section.getBoundingClientRect().top <= top) selected = section.id; }); if (settingsBody.scrollHeight - settingsBody.scrollTop - settingsBody.clientHeight < 4) selected = 'settings-terminal-section'; markSection(selected);
    }, {passive: true});
    $('settings-close').onclick = function () { setSettingsOpen(false); if (settingsTrigger) settingsTrigger.focus(); };
    $('chat-size-rng').oninput = function () { chatSizePref = Number(this.value); try { localStorage.setItem('gtmux.chatFontSize', String(chatSizePref)); } catch (e) {} applyAppearance(); syncSettings(); };
    $('settings-reset').onclick = function () {
      fontPref = 'auto'; sizePref = 0; chatSizePref = 14; appearancePref = 'system';
      ['gtmux.fontPref', 'gtmux.fontSize', 'gtmux.chatFontSize', 'gtmux.appearance', 'gtmux.returnSends'].forEach(function (key) { try { localStorage.removeItem(key); } catch (e) {} });
      sel.value = fontPref; returnSend.checked = false; returnSend.onchange(); applyWebAppearance(); applyAppearance(); syncSettings();
    };
    gear.onclick = function (e) { e.stopPropagation(); toggleSettings($('bar'), gear); syncSize(); };
    document.addEventListener('click', function (e) {
      if (!panel.hidden && !panel.contains(e.target) && e.target !== gear) setSettingsOpen(false);
    });
    panel.addEventListener('keydown', function (e) { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); setSettingsOpen(false); if (settingsTrigger) settingsTrigger.focus(); } });
    sel.onchange = function () {
      fontPref = sel.value;
      try { localStorage.setItem('gtmux.fontPref', fontPref); } catch (e) {}
      applyAppearance();
    };
    rng.oninput = function () {
      sizePref = parseInt(rng.value, 10); syncSettings();
      try { localStorage.setItem('gtmux.fontSize', String(sizePref)); } catch (e) {}
      applyAppearance();
    };
  }

  // setupMode wires the pane-view tab switcher (对话 / 终端).
  function setupMode() {
    var bs = $('mode').querySelectorAll('button');
    Array.prototype.forEach.call(bs, function (b) { b.onclick = function () { setMode(b.getAttribute('data-mode')); }; });
  }

  // home() = the top-level view for the current width: the workbench on wide
  // screens, the single-column radar on narrow. The back button + the responsive
  // switch both route through here.
  function home() { if (isWide()) startWorkbench(); else startRadar(); }

  function boot() {
    window.addEventListener('pagehide', function () { saveCompose(composeCache, true); });
    if (window.visualViewport) {
      var viewport = window.visualViewport;
      var resizeComposerViewport = function () { if (viewport.scale === 1) { document.documentElement.style.setProperty('--viewport-height', Math.round(viewport.height) + 'px'); document.documentElement.style.setProperty('--visual-bottom', Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop) + 'px'); } document.documentElement.classList.toggle('keyboard-open', window.innerHeight - viewport.height > 120); };
      viewport.addEventListener('resize', resizeComposerViewport); resizeComposerViewport();
    }
    setupWebAppearance();
    labelChrome(); // before anything renders, so no frame shows the untranslated markup
    setupSettings();
    $('back').onclick = function () { if (focusFromWB && isWide()) startWorkbench(); else startRadar(); };
    $('panes-btn').onclick = function () { openPanes(); };
    $('panes-back').onclick = function () { startRadar(); };
    $('panes-search').oninput = function () { panesSig = ''; renderPanes(); };
    setupKeyboard();
    setupPin();
    setupMode();
    setupFocus();
    wbLoad();
    setupRail();
    $('wb-gear').onclick = function (e) { e.stopPropagation(); toggleSettings($('wb-bar'), $('wb-gear')); };
    // responsive: cross the 900px threshold → switch top-level layout (only when
    // at a top-level view, not inside a focused pane/chat).
    var lastWide = isWide();
    window.addEventListener('resize', function () {
      var w = isWide(); if (w === lastWide) return; lastWide = w;
      if (activeSessionTool) { activeSessionTool.reflowPending = true; return; }
      // Reflow focused Chat without changing its conversation or mode.
      if (!$('chat').hidden) { chatSig = ''; drawChat(lastTurns); return; }
      if (!$('pane').hidden || !$('gate').hidden) return; // mid-focus / gated
      // All panes is a view of its own at either width: crossing the line keeps the reader
      // in it. It used to go home, and the workbench drew under a browser left showing.
      if (!$('panes').hidden) return;
      home();
    });
    try { token = localStorage.getItem(TOKEN_KEY); } catch (e) {}
    // A share link arrives as `#code=<code>`: the same eight characters someone would
    // read out, redeemed here for the link's own token. Links handed out before that
    // carried the token itself (#g=, legacy #t=) and still work, so both are read.
    var mc = /(?:^|[#&])code=([0-9a-z][0-9a-z-]{3,})/i.exec(location.hash || '');
    var mt = /(?:^|[#&])[gt]=([a-f0-9]{16,})/i.exec(location.hash || '');
    if (mt && mt[1]) { token = mt[1]; try { localStorage.setItem(TOKEN_KEY, token); } catch (e) {} try { history.replaceState(null, '', location.pathname + location.search); } catch (e) {} }
    var m = /(?:^|[#&])c=([a-f0-9]+)/i.exec(location.hash || '');
    // A share code redeems the same way a pairing code does; whichever is in the URL.
    var code = (mc && mc[1]) || (m && m[1]);
    if (code) { try { history.replaceState(null, '', location.pathname + location.search); } catch (e) {} }
    var ready = code ? pair(code).catch(function () { return null; }) : Promise.resolve();
    ready.then(function () {
      if (!token) return gate('unpaired');
      fetchTheme(); // match the user's real terminal (async; the pane picks it up)
      fetchShare(); // learn which panes (if any) this caller may type into
      home();
    });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
})();
