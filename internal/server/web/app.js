/* gtmux browser mirror — view-only. Pairs via a one-time #c=<code> link, then
 * polls /api/agents (radar, app-styled) and /api/pane (live terminal, xterm.js,
 * Ghostty colors). Incremental writes (append the new tail, not a full reset) so
 * the terminal doesn't flash. No input. */
(function () {
  'use strict';
  var TOKEN_KEY = 'gtmux.token';
  var COLORS = {waiting: '#EF4444', working: '#06B6D4', idle: '#22C55E', running: '#8E8E93'};
  var ORDER = ['waiting', 'working', 'idle', 'running'];
  var LABEL = {waiting: 'needs you', working: 'working', idle: 'idle', running: 'running'};
  // terminal defaults taken from the user's Ghostty config (Hack 15, #17171a/#d4d2cc).
  var GHOSTTY = {bg: '#17171a', fg: '#d4d2cc', cursor: '#bbc1ff', sel: '#2a2a33', font: 'Hack, Menlo, Monaco, "Courier New", monospace', size: 15};
  var MARKS = {'claude code': 'CC', claude: 'CC', codex: 'Cx', gemini: 'G', aider: 'Ai', opencode: 'oc', cursor: 'Cu', crush: 'Cr', amp: 'Am', cline: 'Cl'};

  var $ = function (id) { return document.getElementById(id); };
  var token = null, radarTimer = null, paneTimer = null, selIdx = -1;
  var term = null, fit = null, lastText = '', curPane = null, lastSig = '', theme = null;
  var curAgent = null, chatTimer = null, chatSig = '', lastTurns = [], chatExpanded = {};
  var lastAgents = [], optTimer = null; // for focus prev/next + the waiting reply bar
  var lastOpts = [], lastOptsSig = ''; // waiting options, shared by reply bar (term) + approval card (chat)
  var userScrolling = false, scrollIdle = null, pendingText = null, resizeIdle = null;
  var iconCache = {}; // agentName -> objectURL | 'none' | Promise
  var BUNDLED = ['Hack', 'JetBrains Mono', 'Fira Code', 'IBM Plex Mono'];
  function lsGet(k, d) { try { return localStorage.getItem(k) || d; } catch (e) { return d; } }
  var fontPref = lsGet('gtmux.fontPref', 'auto');          // 'auto' | 'system' | a bundled family
  var sizePref = parseInt(lsGet('gtmux.fontSize', '0'), 10) || 0;  // 0 = follow terminal/default
  var paneMode = lsGet('gtmux.paneMode', 'term');          // pane view tab: 'term' | 'chat'

  // ---- auth -------------------------------------------------------------
  // BASE is the path prefix this mirror is served under. The multi-tenant Direct
  // tunnel routes each Mac at https://host/p<port>/…, so the page loads at /p<port>/
  // and every /api/… call must carry that prefix — an absolute /api/… would hit the
  // VPS's legacy fallback (the WRONG Mac) and pairing would fail. Empty for a plain
  // serve/Cloudflare tunnel served at the root.
  var BASE = (location.pathname || '/').replace(/\/+$/, '');
  function authHeaders() { return token ? {Authorization: 'Bearer ' + token} : {}; }
  function api(path, opts) {
    opts = opts || {};
    opts.headers = Object.assign({}, opts.headers || {}, authHeaders());
    return fetch(BASE + path, opts);
  }
  function pair(code) {
    return fetch(BASE + '/api/enroll', {
      method: 'POST', headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({enrollCode: code, name: 'browser'}),
    }).then(function (r) { if (!r.ok) throw new Error('pair'); return r.json(); })
      .then(function (j) { token = j.token; try { localStorage.setItem(TOKEN_KEY, token); } catch (e) {} });
  }

  // ---- helpers ----------------------------------------------------------
  function show(which) { ['gate', 'radar', 'pane', 'chat', 'workbench'].forEach(function (id) { $(id).hidden = id !== which; }); if (which !== 'workbench') WB.on = false; }
  // GATE states. Each says what the situation IS and the one thing to do about it, in
  // both languages — the old screen was a single English sentence with no subject, and it
  // pointed at a phone-app affordance ("open on computer") that does not exist. The
  // instruction below is the one `gtmux pair` actually prints.
  var GATE = {
    unpaired: {
      zh: '这个浏览器还没有和你的 Mac 配对。',
      en: "This browser isn't paired with your Mac yet.",
      steps: [
        {zh: '在你的 Mac 上运行：', en: 'On your Mac, run:', code: 'gtmux pair'},
        {zh: '然后打开它列出的第 2 项「Browser」链接。', en: 'Then open the link it prints under "2) Browser".'}
      ],
      note: {zh: '别人分享给你的访客链接可以直接打开，不用配对。',
             en: 'A guest link someone shared with you works as-is — no pairing needed.'}
    },
    expired: {
      zh: '这个链接已经失效了。',
      en: 'This link has expired.',
      steps: [
        {zh: '配对码是一次性的、5 分钟内有效。在你的 Mac 上重新生成：',
         en: 'A pairing code is one-time and expires in 5 minutes. Mint a fresh one on your Mac:', code: 'gtmux pair'},
        {zh: '然后打开它列出的第 2 项「Browser」链接。', en: 'Then open the link it prints under "2) Browser".'}
      ]
    }
  };

  // gate renders one of those states. Built as DOM (not a string) so a command renders as
  // a command — the old copy carried markdown backticks into a rendered page, where they
  // showed up as literal ` characters.
  function gate(which) {
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
    show('gate');
    $('mode').hidden = true; $('back').hidden = true;
    clearInterval(radarTimer); clearInterval(paneTimer); clearInterval(chatTimer);
    radarTimer = paneTimer = chatTimer = null;
  }

  // el builds a text node element (no innerHTML anywhere in the gate).
  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    e.textContent = text;
    return e;
  }
  // Connection indicator (铁律: server 名 + 状态点,不用 "live"). 3 states:
  // 已连接绿 / 重连琥珀(首次失败) / 离线红(持续失败). Both the radar/pane bar
  // (#conn) and the workbench bar (#wb-conn) render the same shared state.
  function serverLabel() {
    var h = location.hostname || 'server';
    if (h === 'localhost' || /^[0-9.]+$/.test(h) || h.indexOf(':') !== -1) return h; // IP / localhost as-is
    return h.split('.')[0]; // gtmux-qclyu2s2.ccy.dev → gtmux-qclyu2s2
  }
  var connFails = 0;
  function connStateFor(ok) { if (ok) { connFails = 0; return 'live'; } connFails++; return connFails === 1 ? 'retry' : 'off'; }
  var CONN_TITLE = {live: '已连接 / connected', retry: '重连中 / reconnecting…', off: '离线 / offline'};
  function renderConn(el, st) { el.className = 'conn ' + st; el.textContent = serverLabel(); el.title = CONN_TITLE[st] || ''; }
  function setConn(ok) { renderConn($('conn'), connStateFor(ok)); }
  function isNative(a) { return a.source === 'native'; }
  function primary(a) {
    if (a.task) return a.task;
    if (isNative(a)) return a.project || a.terminal || '';
    return a.session || a.loc || a.pane_id || '';
  }
  function secondary(a) {
    if (isNative(a)) return a.terminal || a.agent || ''; // sensed agent, no pane locator
    var b = a.session || a.loc || '';
    return a.pane_id ? b + ' · ' + a.pane_id : b;
  }

  function agentMark(name) {
    var k = (name || '').trim().toLowerCase();
    if (MARKS[k]) return MARKS[k];
    for (var key in MARKS) { if (k.indexOf(key) !== -1) return MARKS[key]; }
    var c = (name || '').trim();
    return c ? c.slice(0, 2) : '?';
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
  function badgeSVG(st) {
    var c = COLORS[st] || COLORS.running, W = '#fff', shape, glyph = '';
    if (st === 'waiting') {
      shape = '<rect x="1" y="1" width="14" height="14" rx="4" fill="' + c + '"/>';
      glyph = '<rect x="5.1" y="4.6" width="1.7" height="6.8" rx="0.85" fill="' + W + '"/><rect x="9.2" y="4.6" width="1.7" height="6.8" rx="0.85" fill="' + W + '"/>';
    } else {
      shape = '<circle cx="8" cy="8" r="7" fill="' + c + '"/>';
      if (st === 'working') glyph = '<circle cx="8" cy="8" r="3.4" stroke="' + W + '" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-dasharray="13 6"/>';
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
    if (withBadge) { var bdg = document.createElement('span'); bdg.className = 'badge'; bdg.innerHTML = badgeSVG(COLORS[a.status] ? a.status : 'running'); av.appendChild(bdg); }
    if (a.icon) loadIcon(a.agent, img, mono);
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

  // ---- radar ------------------------------------------------------------
  function rowEl(a) {
    var st = COLORS[a.status] ? a.status : 'running';
    var row = document.createElement('div');
    row.className = 'row' + (st === 'waiting' ? ' waiting' : '');

    var av = avatarEl(a, 0, true);

    var text = document.createElement('div'); text.className = 'text';
    var p = document.createElement('div'); p.className = 'primary'; p.textContent = primary(a);
    var s = document.createElement('div'); s.className = 'secondary'; s.textContent = secondary(a);
    if (a.task && primary(a) !== a.task) {
      var tk = document.createElement('span'); tk.className = 'task'; tk.textContent = '  ' + a.task; s.appendChild(tk);
    }
    // Idle modifiers (amber, NEVER red — red is `waiting`): errored ⚠ (ended on an
    // error) and background-running ⧗ (idle but bg work still in flight). Mutually
    // exclusive; errored wins. The status badge/section stay idle.
    var AMBER = '#F59E0B';
    if (a.error) {
      var em = document.createElement('span'); em.className = 'task'; em.style.color = AMBER;
      em.textContent = '  ⚠ ' + (a.error_text || 'errored'); s.appendChild(em);
    } else if (a.bg) {
      var bn = (a.bg_count && a.bg_count > 1) ? a.bg_count : '';
      var bm = document.createElement('span'); bm.className = 'task'; bm.style.color = AMBER;
      bm.textContent = '  ⧗' + bn + ' ' + (a.bg_text || 'background running'); s.appendChild(bm);
    }
    text.appendChild(p); text.appendChild(s);

    var right = document.createElement('div'); right.className = 'right';
    var t = relTime(a.since || a.activity_at);
    if (t) { var tm = document.createElement('span'); tm.className = 'time'; tm.textContent = t; right.appendChild(tm); }
    var ag = document.createElement('span'); ag.className = 'agent'; ag.textContent = a.agent || ''; right.appendChild(ag);
    // Native (non-tmux) agents are SENSED read-only: no pane to open → a "native" tag
    // instead of the chevron, and no click target (mirrors the app/menu-bar).
    if (isNative(a)) {
      var nt = document.createElement('span'); nt.className = 'native-tag'; nt.textContent = 'native'; right.appendChild(nt);
      row.classList.add('native');
    } else {
      var ch = document.createElement('span'); ch.className = 'chev'; ch.textContent = '›'; right.appendChild(ch);
      row.onclick = function () { openAgent(a); };
    }

    row.appendChild(av); row.appendChild(text); row.appendChild(right);
    return row;
  }

  function renderRadar(agents) {
    // only repaint when something actually changed (avoids list flicker every poll)
    var sig = JSON.stringify(agents.map(function (a) { return [a.pane_id, a.source, a.project, a.terminal, a.status, a.task, a.since, a.icon, a.error, a.error_text, a.bg, a.bg_count, a.bg_text]; }));
    if (sig === lastSig) return;
    lastSig = sig;
    // tmux agents bucket by status; native (non-tmux) agents are SENSED read-only, so
    // they get their own "Elsewhere" section at the end (mirrors the app / menu-bar).
    var by = {waiting: [], working: [], idle: [], running: []};
    var natives = [];
    agents.forEach(function (a) {
      if (isNative(a)) { natives.push(a); return; }
      (by[a.status] || by.running).push(a);
    });
    var root = $('radar'); root.innerHTML = '';
    function section(label, list) {
      if (!list.length) return;
      var lbl = document.createElement('div'); lbl.className = 'group-label';
      lbl.textContent = label + '  ' + list.length; root.appendChild(lbl);
      list.forEach(function (a) { root.appendChild(rowEl(a)); });
    }
    ORDER.forEach(function (st) { section(LABEL[st], by[st]); });
    section('Elsewhere', natives);
    if (!root.children.length) { var e = document.createElement('div'); e.className = 'group-label'; e.textContent = 'no agents'; root.appendChild(e); }
    if (selIdx >= 0) { selIdx = Math.min(selIdx, radarRows().length - 1); highlightSel(); }
  }

  // ---- desktop keyboard nav (j/k or ↑↓ select · Enter open · Esc back) -----
  function radarRows() { return Array.prototype.slice.call($('radar').querySelectorAll('.row')); }
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
      var tag = (e.target && e.target.tagName) || '';
      if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA') return;
      if (!$('cmdk').hidden) { return; } // cmdk has its own input handler
      if (!$('workbench').hidden) {
        if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) { e.preventDefault(); openCmdk(); }
        else if (e.metaKey || e.ctrlKey) { return; } // let other ⌘ combos pass
        else if (e.key === 'Escape' && maxedTile) { e.preventDefault(); restoreBoard(); }
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
    api('/api/agents').then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
      if (!r.ok) throw new Error('agents'); return r.json();
    }).then(function (agents) { if (!agents) return; setConn(true); lastAgents = agents; renderRadar(agents); })
      .catch(function () { setConn(false); });
  }
  function startRadar() {
    show('radar'); $('bar').hidden = false; $('back').hidden = true; $('mode').hidden = true; hideFocusChrome(); $('title').textContent = 'gtmux'; $('sub').textContent = '';
    curPane = null; curAgent = null; lastSig = '';
    clearInterval(paneTimer); paneTimer = null; clearInterval(chatTimer); chatTimer = null;
    pollRadar(); clearInterval(radarTimer); radarTimer = setInterval(pollRadar, 2000);
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
    document.body.style.background = (theme && theme.background) || GHOSTTY.bg;
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
      t.term.options.fontSize = Math.max(10, termSize() - 3);
      try { t.fit.fit(); } catch (e) {}
    });
  }
  function fetchTheme() {
    return api('/api/theme').then(function (r) { return r.ok ? r.json() : null; }).then(function (j) {
      if (!j) return;
      theme = j; applyAppearance();
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
        try { fit.fit(); } catch (e) {}
        if (lastText) { var t = lastText; lastText = ''; writePane(t); }
      }, 120);
    });
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
    var el = brandLoaderEl('正在拉取屏幕…'); el.id = 'pane-load';
    $('pane').appendChild(el);
  }
  function hidePaneLoader() { var e = $('pane-load'); if (e) e.remove(); }
  function showChatLoader() {
    hideChatLoader();
    var el = brandLoaderEl('正在拉取对话…'); el.id = 'chat-load';
    $('chat').appendChild(el);
  }
  function hideChatLoader() { var e = $('chat-load'); if (e) e.remove(); }

  // --- shared input (web-shared-input) -----------------------------------
  // The server gate is authoritative; this only mirrors it. GET /api/share tells us,
  // for THIS caller, whether input is on and which panes are allowed (or all, for the
  // owner). We show the input bar ONLY for an allowed pane; a blocked send still 403s.
  var SHARE = {input: false, all: false, panes: {}, viewCount: 0, typeCount: 0};
  function fetchShare() {
    api('/api/share').then(function (r) { return r && r.ok ? r.json() : null; }).then(function (j) {
      if (!j) return;
      SHARE = {input: !!j.input, all: !!j.all, panes: {},
               viewCount: (j.view_panes || []).length, typeCount: (j.panes || []).length};
      (j.panes || []).forEach(function (p) { SHARE.panes[p] = true; });
      SHARE.known = true;
      updateInputBar();
      updateGuestScopeStrip();
      updateIdentity();
    }).catch(function () {});
  }

  // Identity chips (WEB §11): owner and guest top bars must read differently —
  // owner = 全权 (full, every pane typable), guest = 协作视图 (this link's scope).
  function updateIdentity() {
    ['identity', 'wb-identity'].forEach(function (id) {
      var el = document.getElementById(id); if (!el) return;
      el.hidden = !SHARE.known;
      if (!SHARE.known) return;
      el.classList.toggle('id-owner', !!SHARE.all);
      el.textContent = SHARE.all ? '全权 · ' + (location.hostname || 'local') : '协作视图 · 访客';
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
    // zh to match the rest of the page (the web mirror is Chinese-primary; this line
    // was the sole English holdout — bilingual 铁律).
    el.textContent = '协作视图 · 访客 · ' + SHARE.viewCount + ' 个会话可见 · ' +
      SHARE.typeCount + ' 个可输入 —— 由 host 授权，可随时吊销';
  }
  function paneCanInput(id) { return !!id && SHARE.input && (SHARE.all || !!SHARE.panes[id]); }
  // setCapChip paints a ⌨可输入/👁只读 capability chip (WEB §11 — always explicit,
  // color+glyph, never an unexplained missing input box).
  function setCapChip(el, can) {
    if (!el) return;
    el.classList.toggle('cap-in', can);
    el.classList.toggle('cap-ro', !can);
    el.textContent = can ? '⌨ 可输入' : '👁 只读';
  }
  function updateInputBar() {
    var bar = $('pane-input'); if (!bar) return;
    var inPaneTerm = !$('pane').hidden && paneMode === 'term';
    var can = paneCanInput(curPane);
    bar.hidden = !(inPaneTerm && can);
    // read-only panes state WHY there is no input row (one line, no empty textbox)
    var ro = $('pane-ro'); if (ro) ro.hidden = !(inPaneTerm && SHARE.known && !can);
    $('pane').classList.toggle('has-input', inPaneTerm && SHARE.known);
    // the single-pane top-bar capability chip (term + chat share the pane focus)
    var cap = $('cap');
    if (cap) {
      var focused = !!curPane && (!$('pane').hidden || !$('chat').hidden);
      cap.hidden = !(focused && SHARE.known);
      setCapChip(cap, can);
    }
    if (WB && WB.tiles) WB.tiles.forEach(tileInputSync); // the workbench tiles too
  }
  // postSend is the shared /api/send call used by BOTH the single-pane bar and the
  // workbench tiles. The server gate (guest consent + allowlist) is authoritative.
  function postSend(id, body) {
    body.id = id;
    return api('/api/send', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)});
  }
  function sendPane(body) {
    var pin = $('pin');
    return postSend(curPane, body).then(function (r) {
      if (!r) return;
      if (r.status === 403) { if (pin) { pin.value = ''; pin.placeholder = 'input not shared for this pane'; } return; }
      if (r.status === 401) { token = null; try { localStorage.removeItem(TOKEN_KEY); } catch (e) {} gate('expired'); return; }
      if (!r.ok) return;
      return r.json();
    }).then(function (j) { if (j && typeof j.text === 'string') { writePane(j.text); hidePaneLoader(); } });
  }
  // tileInputSync shows a tile's input row iff it is in term mode and the host allowed
  // this pane — and keeps the head's ⌨/👁 capability chip + the read-only note in
  // sync; tileSendThen renders the post-send echo into the tile's own terminal.
  function tileInputSync(t) {
    if (!t) return;
    var can = paneCanInput(t.id);
    if (t.inputEl) t.inputEl.hidden = !(t.mode === 'term' && can);
    if (t.roEl) t.roEl.hidden = !(t.mode === 'term' && SHARE.known && !can);
    if (t.capEl) { t.capEl.hidden = !SHARE.known; setCapChip(t.capEl, can); }
  }
  function tileSendThen(t, pin) {
    return function (r) {
      if (!r) return;
      if (r.status === 403) { if (pin) { pin.value = ''; pin.placeholder = 'not shared'; } return; }
      if (!r.ok) return;
      r.json().then(function (j) { if (j && typeof j.text === 'string' && t.term) tileWrite(t, j.text); });
    };
  }

  function pollPane() {
    if (!curPane) return;
    api('/api/pane?id=' + encodeURIComponent(curPane)).then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
      if (!r.ok) throw new Error('pane'); return r.json();
    }).then(function (j) { if (!j) return; setConn(true); writePane(j.text); hidePaneLoader(); })
      .catch(function () { setConn(false); });
  }
  // openAgent enters the pane view for an agent; the selected tab (terminal mirror
  // vs chat history) is remembered across agents (gtmux.paneMode).
  var focusFromWB = false;
  function openAgent(a) {
    focusFromWB = !$('workbench').hidden; // remember where to return (workbench vs radar)
    curPane = a.pane_id; curAgent = a; $('bar').hidden = false; $('back').hidden = false; $('mode').hidden = false; $('gear').hidden = false;
    $('title').textContent = primary(a); $('sub').textContent = (a.agent || '') + ' · ' + secondary(a);
    clearInterval(radarTimer); radarTimer = null;
    applyMode();
  }
  // applyMode shows the active pane tab and (re)starts ONLY that tab's poll loop.
  function applyMode() {
    syncModeButtons();
    if (paneMode === 'chat') {
      clearInterval(paneTimer); paneTimer = null;
      show('chat'); chatSig = ''; lastTurns = []; chatExpanded = {}; curTurnIdx = 1e9;
      showChatLoader();
      showFocusChrome(false);
      pollChat(); clearInterval(chatTimer); chatTimer = setInterval(pollChat, 2500);
      updateInputBar(); // keep the top-bar ⌨/👁 capability chip in sync in chat mode
    } else {
      clearInterval(chatTimer); chatTimer = null;
      show('pane'); ensureTerm(); lastText = ''; pendingText = null; userScrolling = false;
      showPaneLoader();
      try { fit.fit(); } catch (e) {}
      showFocusChrome(true);
      pollPane(); clearInterval(paneTimer); paneTimer = setInterval(pollPane, 1200);
      updateInputBar(); // show the input row iff this caller may type into this pane
    }
  }
  function setMode(m) {
    if (m === paneMode || !curPane) return;
    paneMode = m;
    try { localStorage.setItem('gtmux.paneMode', m); } catch (e) {}
    applyMode();
  }
  function syncModeButtons() {
    var bs = $('mode').querySelectorAll('button');
    Array.prototype.forEach.call(bs, function (b) { b.classList.toggle('on', b.getAttribute('data-mode') === paneMode); });
  }

  // ---- chat (对话 mode) -------------------------------------------------
  // Renders the parsed transcript (GET /api/transcript) as prompt → collapsed
  // steps → final response, mirroring the phone's ChatView. View-only.
  function pollChat() {
    if (!curPane) return;
    api('/api/transcript?id=' + encodeURIComponent(curPane)).then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
      if (r.status === 404 || r.status === 503) { setConn(true); return []; } // no resume record / not available
      if (!r.ok) throw new Error('transcript'); return r.json();
    }).then(function (turns) { if (turns === null) return; setConn(true); hideChatLoader(); renderChat(turns || []); })
      .catch(function () { setConn(false); });
  }
  function renderChat(turns) {
    lastTurns = turns;
    // only repaint when content changed (keeps scroll position + expanded steps)
    var sig = JSON.stringify(turns.map(function (t) { return [t.prompt, t.response, (t.segments || []).map(function (s) { return (s.steps || []).length; })]; }));
    if (sig === chatSig) return;
    chatSig = sig;
    drawChat(turns);
  }
  // §03 wide chat: a turn-outline rail (left) + a centered conversation column with
  // hover copy/quote on agent bubbles + a waiting approval card. Narrow: no rail.
  function drawChat(turns) {
    var root = $('chat'), wide = isWide() && turns.length;
    var oldScroll = root.querySelector('.conv-scroll');
    var atBottom = !oldScroll || (oldScroll.scrollHeight - oldScroll.scrollTop - oldScroll.clientHeight < 48);
    var prevTop = oldScroll ? oldScroll.scrollTop : 0;
    root.innerHTML = ''; root.classList.toggle('wide', !!wide);
    var a = curAgent || {};

    if (wide) root.appendChild(buildOutline(turns));
    var scroll = document.createElement('div'); scroll.className = 'conv-scroll';
    var col = document.createElement('div'); col.className = 'chat-col';

    var sr = document.createElement('div'); sr.className = 'chat-state';
    sr.appendChild(avatarEl(a, 30, false));
    var stx = document.createElement('div'); stx.className = 'cs-text';
    var nm = document.createElement('div'); nm.className = 'cs-name'; nm.textContent = a.agent || ''; stx.appendChild(nm);
    var stl = document.createElement('div'); stl.className = 'cs-status';
    var dot = document.createElement('span'); dot.className = 'cs-dot'; dot.style.background = COLORS[a.status] || COLORS.running; stl.appendChild(dot);
    var lb = document.createElement('span'); lb.textContent = LABEL[a.status] || a.status || ''; stl.appendChild(lb);
    stx.appendChild(stl); sr.appendChild(stx); col.appendChild(sr);

    if (!turns.length) {
      var e = document.createElement('div'); e.className = 'chat-empty';
      e.textContent = 'No conversation history yet. History comes from the agent’s session log (needs the gtmux hooks); it appears once you start talking. Switch to Terminal for the current screen.';
      col.appendChild(e);
    }
    turns.forEach(function (t, idx) {
      var ct = document.createElement('div'); ct.className = 'cturn'; ct.id = 'cturn-' + idx;
      if (t.prompt) {
        var ur = document.createElement('div'); ur.className = 'urow';
        var ub = document.createElement('div'); ub.className = 'ububble'; ub.textContent = t.prompt;
        ur.appendChild(ub); ur.appendChild(userAvatarEl(26)); ct.appendChild(ur);
      }
      // each segment = an assistant text bubble + the tool steps that followed it;
      // render in order so intermediate process sits BETWEEN separate bubbles. Every
      // agent bubble carries the avatar (a turn can split into many bubbles across
      // tool calls; one-per-turn left the follow-ups looking orphaned).
      var segs = (t.segments && t.segments.length) ? t.segments : (t.response ? [{text: t.response}] : []);
      segs.forEach(function (seg, k) {
        if (seg.text) {
          var ar = document.createElement('div'); ar.className = 'arow';
          ar.appendChild(avatarEl(a, 26, false));
          var ab = document.createElement('div'); ab.className = 'abubble'; ab.appendChild(mdRender(seg.text));
          ab.appendChild(bubbleActions(seg.text)); // hover: 复制 / 引用 (desktop)
          ar.appendChild(ab); ct.appendChild(ar);
        }
        if (seg.steps && seg.steps.length) {
          var sk = idx + '-' + k;
          var open = !!chatExpanded[sk];
          var tog = document.createElement('button'); tog.className = 'steps-toggle';
          tog.textContent = (open ? '▾ ' : '▸ ') + seg.steps.length + ' step' + (seg.steps.length > 1 ? 's' : '');
          tog.onclick = (function (key) { return function () { chatExpanded[key] = !chatExpanded[key]; drawChat(lastTurns); }; })(sk);
          ct.appendChild(tog);
          if (open) {
            seg.steps.forEach(function (s) {
              var row = document.createElement('div'); row.className = 'step-row';
              var sn = document.createElement('span'); sn.className = 'step-name'; sn.textContent = s.title || ''; row.appendChild(sn);
              if (s.detail) { var sd = document.createElement('span'); sd.className = 'step-detail'; sd.textContent = s.detail; row.appendChild(sd); }
              ct.appendChild(row);
            });
          }
        }
      });
      col.appendChild(ct);
    });
    // waiting → an inline approval card (read-only 1/2/3 from /api/options).
    if (a.status === 'waiting') col.appendChild(approvalCard());
    scroll.appendChild(col); root.appendChild(scroll);
    scroll.scrollTop = atBottom ? scroll.scrollHeight : prevTop;
    if (curTurnIdx >= turns.length) curTurnIdx = Math.max(0, turns.length - 1);
    if (wide) syncOutline();
  }

  // ---- §03 wide-chat helpers --------------------------------------------
  var curTurnIdx = 0; // highlighted turn in the outline (j/k nav)
  function buildOutline(turns) {
    var rail = document.createElement('div'); rail.className = 'turn-rail';
    var hd = document.createElement('div'); hd.className = 'to-head'; hd.textContent = 'Turns'; rail.appendChild(hd);
    var list = document.createElement('div'); list.className = 'to-list'; list.id = 'to-list';
    turns.forEach(function (t, idx) {
      var it = document.createElement('button'); it.className = 'to-item'; it.dataset.idx = idx;
      var n = document.createElement('span'); n.className = 'to-n'; n.textContent = (idx + 1); it.appendChild(n);
      var tx = document.createElement('span'); tx.className = 'to-tx';
      tx.textContent = (t.prompt || (t.segments && t.segments[0] && t.segments[0].text) || t.response || '…').replace(/\s+/g, ' ').trim();
      it.appendChild(tx);
      it.onclick = function () { selectTurn(idx, true); };
      list.appendChild(it);
    });
    rail.appendChild(list);
    var ft = document.createElement('div'); ft.className = 'to-foot'; ft.textContent = 'j/k 跳转 · c 折叠全部'; rail.appendChild(ft);
    return rail;
  }
  function syncOutline() {
    var list = $('to-list'); if (!list) return;
    var items = list.querySelectorAll('.to-item');
    for (var i = 0; i < items.length; i++) items[i].classList.toggle('on', +items[i].dataset.idx === curTurnIdx);
  }
  function selectTurn(idx, scrollIt) {
    var turns = lastTurns || []; if (!turns.length) return;
    curTurnIdx = Math.max(0, Math.min(turns.length - 1, idx));
    syncOutline();
    if (scrollIt) { var el = $('cturn-' + curTurnIdx); if (el) el.scrollIntoView({block: 'start', behavior: 'smooth'}); }
  }
  // hover floating 复制 / 引用 on an agent bubble (desktop pointer only).
  function bubbleActions(text) {
    var bar = document.createElement('div'); bar.className = 'bub-act';
    var mk = function (label, fn) { var b = document.createElement('button'); b.textContent = label; b.onclick = function (e) { e.stopPropagation(); fn(); }; return b; };
    bar.appendChild(mk('⧉ 复制', function () { copyText(text); }));
    bar.appendChild(mk('❝ 引用', function () { copyText(String(text).split('\n').map(function (l) { return '> ' + l; }).join('\n')); }));
    return bar;
  }
  function copyText(s) {
    try { navigator.clipboard.writeText(s); } catch (e) {}
  }
  function collapseAllSteps() { chatExpanded = {}; drawChat(lastTurns); }
  // waiting approval card — shows the agent's 1/2/3 options (from /api/options).
  // When this caller may type into the pane (WEB §11), the rows are LIVE — one
  // click sends the digit (no Enter, like the phone's ApprovalCard). Otherwise
  // view-only with the reply-elsewhere hint. The server gate stays authoritative.
  function approvalCard() {
    var can = paneCanInput(curPane);
    var card = document.createElement('div'); card.className = can ? 'appr-card appr-live' : 'appr-card';
    var hd = document.createElement('div'); hd.className = 'appr-head';
    var d = document.createElement('span'); d.className = 'appr-dot'; hd.appendChild(d);
    var ht = document.createElement('span'); ht.textContent = '需要你批准'; hd.appendChild(ht); card.appendChild(hd);
    var opts = lastOpts || [];
    if (!opts.length) {
      var ph = document.createElement('div'); ph.className = 'appr-empty';
      ph.textContent = can ? '在终端里有一个待确认的选择 · 切到「终端」回应' : '在终端里有一个待确认的选择 · 用手机/Mac 回应';
      card.appendChild(ph);
    } else {
      opts.forEach(function (o) {
        var row = document.createElement('div'); row.className = 'appr-opt';
        var k = document.createElement('span'); k.className = 'appr-key'; k.textContent = o.n; row.appendChild(k);
        var lb = document.createElement('span'); lb.className = 'appr-label'; lb.textContent = o.label || ''; row.appendChild(lb);
        if (can) row.onclick = function () { sendPane({text: String(o.n)}); };
        card.appendChild(row);
      });
    }
    if (!can) {
      var hint = document.createElement('div'); hint.className = 'appr-hint'; hint.textContent = 'view-only · 用手机/Mac 发送,或扫码接管'; card.appendChild(hint);
    }
    return card;
  }

  // ---- markdown (vanilla → DOM) -----------------------------------------
  // A tiny Markdown subset matching the phone's markdown.ts: **bold**/*italic*/
  // `code`/[links] inline; heading/para/fenced-code/list/quote/hr/pipe-table
  // blocks. Underscore emphasis is intentionally OFF (snake_case). Built as DOM
  // with textContent (never innerHTML) so agent output can't inject markup.
  function mdInline(s) {
    var frag = document.createDocumentFragment(), rest = String(s);
    while (rest.length) {
      var best = null;
      var probe = function (re, kind) {
        var m = re.exec(rest);
        if (m && (!best || m.index < best.idx)) best = {idx: m.index, len: m[0].length, kind: kind, m: m};
      };
      probe(/`([^`]+)`/, 'code');
      probe(/\[([^\]]+)\]\(([^)\s]+)\)/, 'link');
      probe(/\*\*([^*]+)\*\*/, 'b');
      probe(/\*([^*\n]+)\*/, 'i');
      if (!best) { frag.appendChild(document.createTextNode(rest)); break; }
      if (best.idx > 0) frag.appendChild(document.createTextNode(rest.slice(0, best.idx)));
      var m = best.m, el;
      if (best.kind === 'code') { el = document.createElement('code'); el.className = 'md-ic'; el.textContent = m[1]; }
      else if (best.kind === 'link') { el = document.createElement('a'); el.href = m[2]; el.target = '_blank'; el.rel = 'noopener noreferrer'; el.textContent = m[1]; }
      else if (best.kind === 'b') { el = document.createElement('strong'); el.textContent = m[1]; }
      else { el = document.createElement('em'); el.textContent = m[1]; }
      frag.appendChild(el);
      rest = rest.slice(best.idx + best.len);
    }
    return frag;
  }
  function mdRender(src) {
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
      if (h) { var he = document.createElement('div'); he.className = 'md-h md-h' + h[1].length; he.appendChild(mdInline(h[2].trim())); root.appendChild(he); i++; continue; }
      if (QUOTE.test(line)) {
        var q = []; while (i < lines.length && QUOTE.test(lines[i])) { q.push(lines[i].replace(QUOTE, '')); i++; }
        var bq = document.createElement('blockquote'); bq.className = 'md-q'; bq.appendChild(mdInline(q.join(' '))); root.appendChild(bq); continue;
      }
      if (BULLET.test(line)) {
        var ul = document.createElement('ul'); ul.className = 'md-list';
        while (i < lines.length && BULLET.test(lines[i])) { var li = document.createElement('li'); li.appendChild(mdInline(lines[i].replace(BULLET, ''))); ul.appendChild(li); i++; }
        root.appendChild(ul); continue;
      }
      if (ORDERED.test(line)) {
        var ol = document.createElement('ol'); ol.className = 'md-list';
        while (i < lines.length && ORDERED.test(lines[i])) { var li2 = document.createElement('li'); li2.appendChild(mdInline(lines[i].replace(ORDERED, ''))); ol.appendChild(li2); i++; }
        root.appendChild(ol); continue;
      }
      if (isTableStart(i)) {
        var header = splitRow(lines[i]), align = parseAlign(lines[i + 1]); i += 2;
        var rows = []; while (i < lines.length && lines[i].trim() !== '' && lines[i].indexOf('|') !== -1) { rows.push(splitRow(lines[i])); i++; }
        var wrap = document.createElement('div'); wrap.className = 'md-tablewrap';
        var tbl = document.createElement('table'); tbl.className = 'md-table';
        var thead = document.createElement('thead'), htr = document.createElement('tr');
        header.forEach(function (c, ci) { var th = document.createElement('th'); th.style.textAlign = align[ci] || 'left'; th.appendChild(mdInline(c)); htr.appendChild(th); });
        thead.appendChild(htr); tbl.appendChild(thead);
        var tb = document.createElement('tbody');
        rows.forEach(function (r) { var tr = document.createElement('tr'); r.forEach(function (c, ci) { var td = document.createElement('td'); td.style.textAlign = align[ci] || 'left'; td.appendChild(mdInline(c)); tr.appendChild(td); }); tb.appendChild(tr); });
        tbl.appendChild(tb); wrap.appendChild(tbl); root.appendChild(wrap); continue;
      }
      var para = [];
      while (i < lines.length && lines[i].trim() !== '' && !/^\s*```/.test(lines[i]) && !HR.test(lines[i]) && !HEADING.test(lines[i]) && !QUOTE.test(lines[i]) && !BULLET.test(lines[i]) && !ORDERED.test(lines[i]) && !isTableStart(i)) { para.push(lines[i]); i++; }
      var p = document.createElement('p'); p.className = 'md-p'; p.appendChild(mdInline(para.join(' '))); root.appendChild(p);
    }
    return root;
  }

  // ======================================================================
  // Desktop workbench (WEB.md §1–§3): a session tree rail + a freeform board of
  // CONCURRENT pane mirrors. Wide screens only; <900px keeps the single column.
  // View-only, no new backend — each tile is another /api/pane|transcript|diff.
  // ======================================================================
  var WB = {on: false, agents: [], tiles: [], railW: 232, railCollapsed: false, snap: false, surface: false, presetCur: '', prevStatus: null};
  var WB_KEY = 'gtmux.board';
  function isWide() { return window.innerWidth >= 900; }
  function wbLoad() {
    try {
      var s = JSON.parse(localStorage.getItem(WB_KEY) || '{}');
      WB.railW = Math.max(170, Math.min(360, s.railW || 232));
      WB.railCollapsed = !!s.railCollapsed; WB.snap = !!s.snap; WB.surface = !!s.surface;
      WB.saved = Array.isArray(s.tiles) ? s.tiles : [];
    } catch (e) { WB.saved = []; }
  }
  function wbSave() {
    try {
      localStorage.setItem(WB_KEY, JSON.stringify({
        railW: WB.railW, railCollapsed: WB.railCollapsed, snap: WB.snap, surface: WB.surface,
        tiles: WB.tiles.map(function (t) { return {id: t.id, x: t.x, y: t.y, w: t.w, h: t.h, mode: t.mode}; }),
      }));
    } catch (e) {}
  }

  function startWorkbench() {
    WB.on = true;
    ['gate', 'radar', 'pane', 'chat'].forEach(function (id) { $(id).hidden = true; });
    $('workbench').hidden = false;
    $('bar').hidden = true; // the workbench has its own #wb-bar
    $('back').hidden = true; $('mode').hidden = true; $('gear').hidden = true; hideFocusChrome();
    clearInterval(paneTimer); paneTimer = null; clearInterval(chatTimer); chatTimer = null;
    applyRail();
    $('board').classList.toggle('snap', WB.snap);
    setToggle($('wb-snap'), WB.snap); setToggle($('wb-surface'), WB.surface);
    updatePresetLabel();
    updateEmpty();
    // restore saved tiles once agents arrive (need their status/agent); poll now.
    pollWB(); clearInterval(radarTimer); radarTimer = setInterval(pollWB, 2000);
  }
  function setToggle(btn, on) { btn.classList.toggle('on', !!on); }
  function setWbConn(ok) { renderConn($('wb-conn'), connStateFor(ok)); }

  function pollWB() {
    api('/api/agents').then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('expired'); return null; }
      if (!r.ok) throw new Error('agents'); return r.json();
    }).then(function (agents) {
      if (!agents) return; setWbConn(true); WB.agents = agents; lastAgents = agents;
      if (WB.saved) { restoreTiles(agents); WB.saved = null; }
      renderTree(agents);
      // refresh each tile's header status + waiting border
      WB.tiles.forEach(function (t) { var a = byId(agents, t.id); if (a) { t.agent = a; updateTileHead(t); } });
      if (WB.surface) autoSurface(agents);
      var ps = {}; agents.forEach(function (a) { ps[a.pane_id] = a.status; }); WB.prevStatus = ps;
    }).catch(function () { setWbConn(false); });
  }
  // auto-surface: a pane newly turning waiting floats onto the board and pulses
  // once → the board doubles as a radar (WEB.md §7). Skips the first poll so we
  // don't flood the board with everything already waiting at load.
  function autoSurface(agents) {
    if (!WB.prevStatus) return;
    agents.forEach(function (a) {
      if (a.status !== 'waiting' || WB.prevStatus[a.pane_id] === 'waiting') return;
      if (WB.tiles.some(function (t) { return t.id === a.pane_id; })) return;
      flashTile(addTile(a));
    });
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
    var q = ($('rail-search').value || '').trim().toLowerCase();
    var match = function (a) { return !q || (primary(a) + ' ' + (a.agent || '') + ' ' + (a.session || '') + ' ' + (a.pane_id || '')).toLowerCase().indexOf(q) !== -1; };
    var by = {waiting: [], working: [], idle: [], running: []};
    agents.forEach(function (a) { if (match(a)) (by[a.status] || by.running).push(a); });
    var root = $('tree'); root.innerHTML = '';
    var nWait = (by.waiting || []).length;
    $('rail-tab-n').textContent = nWait ? nWait : '';
    ORDER.forEach(function (st) {
      var list = by[st]; if (!list.length) return;
      var lbl = document.createElement('div'); lbl.className = 'tree-group' + (st === 'waiting' ? ' waiting' : '');
      lbl.textContent = LABEL[st] + ' ' + list.length; root.appendChild(lbl);
      // group by window (session:window) so a window expands to its panes
      var wins = {}; var order = [];
      list.forEach(function (a) { var w = (a.session || '') + ':' + ((a.loc || '').split(':')[1] || '').split('.')[0]; if (!wins[w]) { wins[w] = []; order.push(w); } wins[w].push(a); });
      order.forEach(function (w) {
        var panes = wins[w];
        if (panes.length > 1) {
          var wh = document.createElement('div'); wh.className = 'tree-win';
          wh.innerHTML = '<span>▾</span><span>' + esc(w) + '</span>'; root.appendChild(wh);
        }
        panes.forEach(function (a) { root.appendChild(treeRow(a, panes.length > 1)); });
      });
    });
    if (!root.children.length) { var e = document.createElement('div'); e.className = 'tree-group'; e.textContent = q ? 'no match' : 'no agents'; root.appendChild(e); }
  }
  function esc(s) { var d = document.createElement('span'); d.textContent = s == null ? '' : s; return d.innerHTML; }
  function treeRow(a, nested) {
    var st = COLORS[a.status] ? a.status : 'running';
    var row = document.createElement('div'); row.className = 'tree-row' + (nested ? ' nested' : '') + (st === 'waiting' ? ' waiting' : '');
    row.draggable = true;
    row.appendChild(avatarEl(a, 24, true));
    var tx = document.createElement('div'); tx.className = 'tr-text';
    var nm = document.createElement('div'); nm.className = 'tr-name'; nm.textContent = primary(a);
    var sb = document.createElement('div'); sb.className = 'tr-sub'; sb.textContent = a.pane_id + (a.agent ? ' · ' + shortAgent(a.agent) : '');
    tx.appendChild(nm); tx.appendChild(sb); row.appendChild(tx);
    var g = document.createElement('span'); g.className = 'tr-grip'; g.textContent = '⠿'; row.appendChild(g);
    row.addEventListener('dragstart', function (e) { e.dataTransfer.setData('text/pane', a.pane_id); e.dataTransfer.effectAllowed = 'copy'; });
    row.ondblclick = function () { addTile(a); };
    return row;
  }
  function shortAgent(s) { var w = String(s).split(' ')[0]; return w.length > 8 ? w.slice(0, 8) : w; }

  // ---- rail collapse / resize -------------------------------------------
  function applyRail() {
    $('workbench').classList.toggle('rail-collapsed', WB.railCollapsed);
    $('rail-tab').hidden = !WB.railCollapsed;
    if (!WB.railCollapsed) $('rail').style.width = WB.railW + 'px';
  }
  function setupRail() {
    $('rail-collapse').onclick = function () { WB.railCollapsed = true; applyRail(); wbSave(); };
    $('rail-tab').onclick = function () { WB.railCollapsed = false; applyRail(); wbSave(); };
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
    $('wb-surface').onclick = function () { WB.surface = !WB.surface; setToggle($('wb-surface'), WB.surface); wbSave(); };
    // layout presets dropdown
    $('wb-preset').onclick = function (e) { e.stopPropagation(); var m = $('wb-preset-menu'); if (m.hidden) showPresetMenu(); else hidePresetMenu(); };
    document.addEventListener('click', function (e) { if (!$('wb-preset-menu').hidden && !$('wb-preset-wrap').contains(e.target)) hidePresetMenu(); });
    // ⌘K command palette input: ↑/↓ select, Enter pick, Esc close
    var ci = $('cmdk-input');
    ci.addEventListener('input', function () { renderCmdk(ci.value); });
    ci.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { e.preventDefault(); closeCmdk(); }
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
    pos = pos || {};
    var existing = null; WB.tiles.forEach(function (t) { if (t.id === a.pane_id) existing = t; });
    if (existing) { existing.el.style.zIndex = ++zTop; return existing; }
    var board = $('board'), br = board.getBoundingClientRect();
    var t = {
      id: a.pane_id, agent: a, mode: pos.mode || 'term',
      x: pos.x != null ? Math.max(0, pos.x) : 14 + (WB.tiles.length % 4) * 28,
      y: pos.y != null ? Math.max(0, pos.y) : 14 + (WB.tiles.length % 4) * 28,
      w: pos.w || 360, h: pos.h || 240,
      term: null, fit: null, lastText: '', pending: null, scrolling: false, scrollIdle: null, timer: null, chatSig: '', expanded: {},
    };
    buildTile(t); WB.tiles.push(t); board.classList.remove('has-empty'); updateEmpty();
    setTileMode(t, t.mode); wbSave();
    return t;
  }
  var zTop = 10;
  function removeTile(t) {
    clearInterval(t.timer); if (t.term) { try { t.term.dispose(); } catch (e) {} }
    t.el.remove(); WB.tiles = WB.tiles.filter(function (x) { return x !== t; });
    updateEmpty(); wbSave();
  }
  function updateEmpty() {
    var board = $('board'), ex = board.querySelector('.board-empty');
    if (!WB.tiles.length) { if (!ex) { var e = document.createElement('div'); e.className = 'board-empty'; e.textContent = '从左侧把 pane 拖到这里 · 或双击树中的 pane'; board.appendChild(e); } }
    else if (ex) ex.remove();
  }
  function buildTile(t) {
    var el = document.createElement('div'); el.className = 'tile'; t.el = el;
    el.style.left = t.x + 'px'; el.style.top = t.y + 'px'; el.style.width = t.w + 'px'; el.style.height = t.h + 'px'; el.style.zIndex = ++zTop;
    var head = document.createElement('div'); head.className = 'tile-head';
    head.appendChild(avatarEl(t.agent, 20, true));
    var nm = document.createElement('span'); nm.className = 'tile-name'; nm.textContent = primary(t.agent); head.appendChild(nm);
    var id = document.createElement('span'); id.className = 'tile-id'; id.textContent = t.id; head.appendChild(id);
    // input-capability chip (WEB §11): every tile head states ⌨可输入 / 👁只读
    var cap = document.createElement('span'); cap.className = 'cap-chip'; cap.hidden = true; t.capEl = cap; head.appendChild(cap);
    var sp = document.createElement('span'); sp.className = 'th-spacer'; head.appendChild(sp);
    var modes = document.createElement('span'); modes.className = 'tile-modes';
    [['term', '终端'], ['chat', '对话'], ['diff', 'diff']].forEach(function (m) {
      var b = document.createElement('button'); b.textContent = m[1]; b.setAttribute('data-m', m[0]);
      b.onclick = function (e) { e.stopPropagation(); setTileMode(t, m[0]); }; modes.appendChild(b);
    });
    head.appendChild(modes);
    var max = document.createElement('button'); max.className = 'tile-btn'; max.textContent = '⤢'; max.title = '全屏'; max.onclick = function (e) { e.stopPropagation(); openAgent(t.agent); }; head.appendChild(max);
    var cl = document.createElement('button'); cl.className = 'tile-btn'; cl.textContent = '×'; cl.title = '关闭'; cl.onclick = function (e) { e.stopPropagation(); removeTile(t); }; head.appendChild(cl);
    el.appendChild(head);
    var body = document.createElement('div'); body.className = 'tile-body'; t.body = body; el.appendChild(body);
    // shared-input row (web-shared-input): shown for a term-mode tile the host allowed.
    var tin = document.createElement('div'); tin.className = 'tile-input'; tin.hidden = true; t.inputEl = tin;
    var tpin = document.createElement('input'); tpin.type = 'text'; tpin.placeholder = 'type…'; tpin.autocomplete = 'off'; tpin.spellcheck = false;
    tpin.addEventListener('keydown', function (e) { if (e.key === 'Enter') { e.preventDefault(); var v = tpin.value; tpin.value = ''; tpin.placeholder = 'type…'; postSend(t.id, {text: v, enter: true}).then(tileSendThen(t, tpin)); } });
    tin.appendChild(tpin);
    [['C-c', '^C'], ['Escape', 'esc'], ['Tab', '⇥'], ['Up', '↑'], ['Down', '↓']].forEach(function (k) {
      var b = document.createElement('button'); b.textContent = k[1]; b.title = k[0];
      b.onclick = function (e) { e.stopPropagation(); postSend(t.id, {key: k[0]}).then(tileSendThen(t, tpin)); tpin.focus(); };
      tin.appendChild(b);
    });
    el.appendChild(tin);
    // read-only note — states WHY there's no input row instead of leaving a gap
    var ro = document.createElement('div'); ro.className = 'tile-ro'; ro.hidden = true;
    ro.textContent = '🔒 host 未授予此 pane 的输入权限'; t.roEl = ro; el.appendChild(ro);
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
    var bdg = t.headAv && t.headAv.querySelector('.badge');
    if (bdg) bdg.innerHTML = badgeSVG(st);
  }
  function setTileMode(t, m) {
    t.mode = m;
    Array.prototype.forEach.call(t.el.querySelectorAll('.tile-modes button'), function (b) { b.classList.toggle('on', b.getAttribute('data-m') === m); });
    clearInterval(t.timer); t.timer = null; t.body.innerHTML = '';
    if (t.term) { try { t.term.dispose(); } catch (e) {} t.term = null; }
    t.body.appendChild(brandLoaderEl(null)); // cleared on first frame (or by a chat/diff re-render)
    if (m === 'term') { tileTermStart(t); }
    else if (m === 'chat') { t.chatSig = ''; tilePollChat(t); t.timer = setInterval(function () { tilePollChat(t); }, 2500); }
    else { tilePollDiff(t); t.timer = setInterval(function () { tilePollDiff(t); }, 3000); }
    tileInputSync(t); // show the input row only in term mode for an allowed pane
    wbSave();
  }

  // per-tile terminal (own xterm; same incremental write + scroll-lock as #pane)
  function tileTermStart(t) {
    var xt = document.createElement('div'); xt.className = 'xt'; t.body.appendChild(xt);
    t.term = new Terminal({convertEol: true, cursorBlink: false, disableStdin: true, cursorInactiveStyle: 'none', scrollback: 4000, allowProposedApi: true, fontFamily: resolveFont(), fontSize: Math.max(10, termSize() - 3), theme: xtermTheme()});
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
    api('/api/pane?id=' + encodeURIComponent(t.id)).then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        if (j && t.mode === 'term' && t.term) {
          t.el.classList.remove('off'); tileWrite(t, j.text);
          var ld = t.body.querySelector('.brandload'); if (ld) ld.remove(); // first frame in
        }
      })
      .catch(function () {});
  }
  function tilePollChat(t) {
    api('/api/transcript?id=' + encodeURIComponent(t.id)).then(function (r) { return (r.status === 404 || r.status === 503) ? [] : (r.ok ? r.json() : null); })
      .then(function (turns) { if (turns === null || t.mode !== 'chat') return; var sig = JSON.stringify((turns || []).map(function (x) { return [x.prompt, x.response]; })); if (sig === t.chatSig) return; t.chatSig = sig; renderTileChat(t, turns || []); })
      .catch(function () {});
  }
  function renderTileChat(t, turns) {
    var wrap = document.createElement('div'); wrap.className = 'tile-chat';
    if (!turns.length) { var e = document.createElement('div'); e.className = 'chat-empty'; e.textContent = 'No history yet.'; wrap.appendChild(e); }
    turns.forEach(function (tn) {
      var ct = document.createElement('div'); ct.className = 'cturn';
      if (tn.prompt) { var ur = document.createElement('div'); ur.className = 'urow'; var ub = document.createElement('div'); ub.className = 'ububble'; ub.textContent = tn.prompt; ur.appendChild(ub); ur.appendChild(userAvatarEl(22)); ct.appendChild(ur); }
      var segs = (tn.segments && tn.segments.length) ? tn.segments : (tn.response ? [{text: tn.response}] : []);
      segs.forEach(function (s) { if (s.text) { var ar = document.createElement('div'); ar.className = 'arow'; var ab = document.createElement('div'); ab.className = 'abubble'; ab.appendChild(mdRender(s.text)); ar.appendChild(ab); ct.appendChild(ar); } });
      wrap.appendChild(ct);
    });
    t.body.innerHTML = ''; t.body.appendChild(wrap); wrap.scrollTop = wrap.scrollHeight;
  }
  function tilePollDiff(t) {
    api('/api/diff?id=' + encodeURIComponent(t.id)).then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) { if (!j || t.mode !== 'diff') return; renderTileDiff(t, j.diff || ''); })
      .catch(function () {});
  }
  function renderTileDiff(t, diff) {
    var pre = document.createElement('pre'); pre.className = 'tile-diff';
    if (!diff) { pre.textContent = '(cwd 不是 git 仓库 / 无改动)'; t.body.innerHTML = ''; t.body.appendChild(pre); return; }
    diff.split('\n').forEach(function (ln) {
      var span = document.createElement('span');
      if (ln.charAt(0) === '+' && ln.indexOf('+++') !== 0) span.className = 'add';
      else if (ln.charAt(0) === '-' && ln.indexOf('---') !== 0) span.className = 'del';
      span.textContent = ln + '\n'; pre.appendChild(span);
    });
    t.body.innerHTML = ''; t.body.appendChild(pre);
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
  function flashBtn(b) { var o = b.textContent; b.textContent = '✓'; setTimeout(function () { b.textContent = o; }, 700); }
  function copyScreen() {
    if (!term) return;
    var text = term.getSelection() || focusVisibleText(), done = function () { flashBtn($('copy-screen')); };
    if (navigator.clipboard) navigator.clipboard.writeText(text).then(done).catch(done); else done();
  }
  function cyclePane(d) {
    if (!curAgent || !lastAgents.length) return;
    var i = -1; for (var k = 0; k < lastAgents.length; k++) if (lastAgents[k].pane_id === curAgent.pane_id) { i = k; break; }
    if (i < 0) return; openAgent(lastAgents[(i + d + lastAgents.length) % lastAgents.length]);
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
    var inChat = !$('chat').hidden && paneMode === 'chat';
    if (!inTerm && !inChat) return;
    if (!curAgent || curAgent.status !== 'waiting') {
      if (inTerm) hideReply();
      if (lastOpts.length) { lastOpts = []; lastOptsSig = ''; if (inChat) drawChat(lastTurns); }
      return;
    }
    api('/api/options?id=' + encodeURIComponent(curPane)).then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        var opts = (j && j.options) ? j.options : [];
        if (inTerm) { renderReply(opts); return; }
        var sig = JSON.stringify(opts); // chat: only redraw the card when options change
        if (sig !== lastOptsSig) { lastOptsSig = sig; lastOpts = opts; drawChat(lastTurns); }
      }).catch(function () {});
  }
  function renderReply(opts) {
    var had = !$('reply-bar').hidden;
    $('pane').classList.add('has-reply'); $('reply-bar').hidden = false;
    // A typable pane's 1/2/3 are LIVE buttons (one tap → /api/send, digit with no
    // Enter — same as the phone's ApprovalCard); a read-only pane keeps the
    // view-only chips + the reply-elsewhere hint. Server gate stays authoritative.
    var can = paneCanInput(curPane);
    var lead = document.querySelector('#reply-bar .rb-lead');
    if (lead) lead.textContent = can ? '在此 pane 回应：' : 'view-only · 在此 pane 回应：';
    var hint = document.querySelector('#reply-bar .rb-hint');
    if (hint) hint.hidden = can;
    var box = $('reply-opts'); box.innerHTML = '';
    (opts.length ? opts : [{n: 1, label: 'Yes'}, {n: 2, label: 'Always'}, {n: 3, label: 'No'}]).forEach(function (o) {
      var s = document.createElement('span'); s.className = can ? 'rb-opt live' : 'rb-opt'; s.textContent = o.n + ' ' + (o.label || '');
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
  function hideFocusChrome() { $('focus-nav').hidden = true; $('focus-ctl').hidden = true; hideReply(); $('jump').hidden = true; var cap = $('cap'); if (cap) cap.hidden = true; clearInterval(optTimer); optTimer = null; lastOpts = []; lastOptsSig = ''; }
  function setupFocus() {
    $('font-dn').onclick = function () { sizePref = Math.max(10, termSize() - 1); persistSize(); applyAppearance(); };
    $('font-up').onclick = function () { sizePref = Math.min(22, termSize() + 1); persistSize(); applyAppearance(); };
    $('copy-screen').onclick = copyScreen;
    $('pane-prev').onclick = function () { cyclePane(-1); };
    $('pane-next').onclick = function () { cyclePane(1); };
    // Shared-input row: text+Enter and the allowlisted control keys → POST /api/send.
    var pin = $('pin');
    if (pin) {
      pin.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); var t = pin.value; pin.value = ''; pin.placeholder = 'type into this pane…'; sendPane({text: t, enter: true}); }
      });
    }
    if ($('pin-send')) $('pin-send').onclick = function () { var t = pin ? pin.value : ''; if (pin) pin.value = ''; sendPane({text: t, enter: true}); };
    Array.prototype.forEach.call(document.querySelectorAll('.pin-k'), function (b) {
      b.onclick = function () { sendPane({key: b.getAttribute('data-key')}); if (pin) pin.focus(); };
    });
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
    var ex = document.createElement('div'); ex.className = 'max-exit'; ex.id = 'max-exit'; ex.textContent = '‹ 还原'; ex.onclick = restoreBoard; board.appendChild(ex);
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
    if (!ps.length) { var em = document.createElement('div'); em.className = 'pm-empty'; em.textContent = '还没有预设'; menu.appendChild(em); }
    ps.forEach(function (p) {
      var row = document.createElement('div'); row.className = 'pm-row' + (p.name === WB.presetCur ? ' on' : '');
      var nm = document.createElement('span'); nm.className = 'pm-name'; nm.textContent = p.name;
      nm.onclick = function () { applyPreset(p); hidePresetMenu(); }; row.appendChild(nm);
      var del = document.createElement('button'); del.className = 'pm-del'; del.textContent = '×'; del.title = '删除';
      del.onclick = function (e) { e.stopPropagation(); var rest = loadPresets().filter(function (x) { return x.name !== p.name; }); savePresets(rest); if (WB.presetCur === p.name) { WB.presetCur = ''; updatePresetLabel(); } renderPresetMenu(); };
      row.appendChild(del); menu.appendChild(row);
    });
    var save = document.createElement('div'); save.className = 'pm-save'; save.textContent = '＋ 存为当前布局…';
    save.onclick = function () {
      var name = (window.prompt('预设名称 / Preset name', WB.presetCur || ('布局 ' + (ps.length + 1))) || '').trim();
      if (!name) return;
      var arr = loadPresets().filter(function (x) { return x.name !== name; });
      var lay = captureLayout(); lay.name = name; arr.push(lay); savePresets(arr);
      WB.presetCur = name; updatePresetLabel(); renderPresetMenu(); hidePresetMenu();
    };
    menu.appendChild(save);
  }
  function showPresetMenu() { renderPresetMenu(); $('wb-preset-menu').hidden = false; }
  function hidePresetMenu() { $('wb-preset-menu').hidden = true; }

  // ---- ⌘K command palette (WEB.md §8) -----------------------------------
  var cmdkSel = 0, cmdkRows = [];
  function openCmdk() {
    if ($('workbench').hidden) return; // workbench-only
    $('cmdk').hidden = false; var inp = $('cmdk-input'); inp.value = ''; cmdkSel = 0;
    renderCmdk(''); setTimeout(function () { inp.focus(); }, 0);
  }
  function closeCmdk() { $('cmdk').hidden = true; }
  function renderCmdk(q) {
    q = (q || '').trim().toLowerCase();
    var list = $('cmdk-list'); list.innerHTML = '';
    cmdkRows = (WB.agents || []).filter(function (a) {
      return !q || (primary(a) + ' ' + (a.agent || '') + ' ' + (a.session || '') + ' ' + (a.pane_id || '')).toLowerCase().indexOf(q) !== -1;
    }).slice(0, 50);
    if (cmdkSel >= cmdkRows.length) cmdkSel = Math.max(0, cmdkRows.length - 1);
    cmdkRows.forEach(function (a, i) {
      var row = document.createElement('div'); row.className = 'ck-row' + (i === cmdkSel ? ' on' : '');
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
    if (!cmdkRows.length) { var e = document.createElement('div'); e.className = 'ck-empty'; e.textContent = q ? '无匹配' : '无 agent'; list.appendChild(e); }
  }
  function markCmdk() { Array.prototype.forEach.call($('cmdk-list').children, function (c, i) { c.classList.toggle('on', i === cmdkSel); }); }
  function pickCmdk(a) { closeCmdk(); var t = addTile(a); flashTile(t); maximizeTile(t); }

  // ---- boot -------------------------------------------------------------
  // setupSettings wires the appearance panel (font + size), persisted locally.
  function setupSettings() {
    var gear = $('gear'), panel = $('settings'), sel = $('font-sel'), rng = $('size-rng'), sz = $('size-val');
    gear.hidden = false;
    sel.value = fontPref;
    function syncSize() { rng.value = String(termSize()); sz.textContent = termSize(); }
    syncSize();
    gear.onclick = function (e) { e.stopPropagation(); panel.hidden = !panel.hidden; syncSize(); };
    document.addEventListener('click', function (e) {
      if (!panel.hidden && !panel.contains(e.target) && e.target !== gear) panel.hidden = true;
    });
    sel.onchange = function () {
      fontPref = sel.value;
      try { localStorage.setItem('gtmux.fontPref', fontPref); } catch (e) {}
      applyAppearance();
    };
    rng.oninput = function () {
      sizePref = parseInt(rng.value, 10); sz.textContent = sizePref;
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
    $('back').onclick = function () { if (focusFromWB && isWide()) startWorkbench(); else startRadar(); };
    setupKeyboard();
    setupMode();
    setupFocus();
    wbLoad();
    setupRail();
    $('wb-gear').onclick = function (e) { e.stopPropagation(); var p = $('settings'); p.hidden = !p.hidden; };
    // responsive: cross the 900px threshold → switch top-level layout (only when
    // at a top-level view, not inside a focused pane/chat).
    var lastWide = isWide();
    window.addEventListener('resize', function () {
      var w = isWide(); if (w === lastWide) return; lastWide = w;
      // mid-focus chat: reflow to add/remove the wide turn-outline rail (§03).
      if (!$('chat').hidden) { chatSig = ''; drawChat(lastTurns); return; }
      if (!$('pane').hidden || !$('gate').hidden) return; // mid-focus / gated
      home();
    });
    try { token = localStorage.getItem(TOKEN_KEY); } catch (e) {}
    // A GUEST share link carries its token directly (#g=<token>; legacy #t= still
    // accepted) — use it as-is (a lasting, revocable credential), unlike the
    // one-time pairing code (#c=).
    var mt = /(?:^|[#&])[gt]=([a-f0-9]{16,})/i.exec(location.hash || '');
    if (mt && mt[1]) { token = mt[1]; try { localStorage.setItem(TOKEN_KEY, token); } catch (e) {} try { history.replaceState(null, '', location.pathname + location.search); } catch (e) {} }
    var m = /(?:^|[#&])c=([a-f0-9]+)/i.exec(location.hash || '');
    var code = m && m[1];
    if (code) { try { history.replaceState(null, '', location.pathname + location.search); } catch (e) {} }
    var ready = code ? pair(code).catch(function () { return null; }) : Promise.resolve();
    ready.then(function () {
      if (!token) return gate('unpaired');
      fetchTheme(); // match the user's real terminal (async; the pane picks it up)
      fetchShare(); // learn which panes (if any) this caller may type into
      setupSettings();
      home();
    });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
})();
