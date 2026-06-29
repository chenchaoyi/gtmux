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
  var userScrolling = false, scrollIdle = null, pendingText = null, resizeIdle = null;
  var iconCache = {}; // agentName -> objectURL | 'none' | Promise
  var BUNDLED = ['Hack', 'JetBrains Mono', 'Fira Code', 'IBM Plex Mono'];
  function lsGet(k, d) { try { return localStorage.getItem(k) || d; } catch (e) { return d; } }
  var fontPref = lsGet('gtmux.fontPref', 'auto');          // 'auto' | 'system' | a bundled family
  var sizePref = parseInt(lsGet('gtmux.fontSize', '0'), 10) || 0;  // 0 = follow terminal/default
  var paneMode = lsGet('gtmux.paneMode', 'term');          // pane view tab: 'term' | 'chat'

  // ---- auth -------------------------------------------------------------
  function authHeaders() { return token ? {Authorization: 'Bearer ' + token} : {}; }
  function api(path, opts) {
    opts = opts || {};
    opts.headers = Object.assign({}, opts.headers || {}, authHeaders());
    return fetch(path, opts);
  }
  function pair(code) {
    return fetch('/api/enroll', {
      method: 'POST', headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({enrollCode: code, name: 'browser'}),
    }).then(function (r) { if (!r.ok) throw new Error('pair'); return r.json(); })
      .then(function (j) { token = j.token; try { localStorage.setItem(TOKEN_KEY, token); } catch (e) {} });
  }

  // ---- helpers ----------------------------------------------------------
  function show(which) { ['gate', 'radar', 'pane', 'chat'].forEach(function (id) { $(id).hidden = id !== which; }); }
  function gate(msg) {
    $('gate-msg').textContent = msg; show('gate');
    $('mode').hidden = true; $('back').hidden = true;
    clearInterval(radarTimer); clearInterval(paneTimer); clearInterval(chatTimer);
    radarTimer = paneTimer = chatTimer = null;
  }
  function setConn(live) { $('conn').className = 'conn ' + (live ? 'live' : 'off'); $('conn').textContent = live ? 'live' : 'offline'; }
  function primary(a) { return a.task || a.session || a.loc || a.pane_id || ''; }
  function secondary(a) { var b = a.session || a.loc || ''; return a.pane_id ? b + ' · ' + a.pane_id : b; }

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
    text.appendChild(p); text.appendChild(s);

    var right = document.createElement('div'); right.className = 'right';
    var t = relTime(a.since || a.activity_at);
    if (t) { var tm = document.createElement('span'); tm.className = 'time'; tm.textContent = t; right.appendChild(tm); }
    var ag = document.createElement('span'); ag.className = 'agent'; ag.textContent = a.agent || ''; right.appendChild(ag);
    var ch = document.createElement('span'); ch.className = 'chev'; ch.textContent = '›'; right.appendChild(ch);

    row.appendChild(av); row.appendChild(text); row.appendChild(right);
    row.onclick = function () { openAgent(a); };
    return row;
  }

  function renderRadar(agents) {
    // only repaint when something actually changed (avoids list flicker every poll)
    var sig = JSON.stringify(agents.map(function (a) { return [a.pane_id, a.status, a.task, a.since, a.icon]; }));
    if (sig === lastSig) return;
    lastSig = sig;
    var by = {waiting: [], working: [], idle: [], running: []};
    agents.forEach(function (a) { (by[a.status] || by.running).push(a); });
    var root = $('radar'); root.innerHTML = '';
    ORDER.forEach(function (st) {
      var list = by[st]; if (!list.length) return;
      var lbl = document.createElement('div'); lbl.className = 'group-label';
      lbl.textContent = LABEL[st] + '  ' + list.length; root.appendChild(lbl);
      list.forEach(function (a) { root.appendChild(rowEl(a)); });
    });
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
      if (!$('radar').hidden) {
        if (e.key === 'j' || e.key === 'ArrowDown') { e.preventDefault(); moveSel(1); }
        else if (e.key === 'k' || e.key === 'ArrowUp') { e.preventDefault(); moveSel(-1); }
        else if (e.key === 'Enter') { var rows = radarRows(); if (rows[selIdx]) { e.preventDefault(); rows[selIdx].click(); } }
      } else if (!$('pane').hidden || !$('chat').hidden) {
        if (e.key === 'Escape') { e.preventDefault(); $('back').click(); }
        else if (e.key === 'c') { e.preventDefault(); setMode('chat'); }
        else if (e.key === 't') { e.preventDefault(); setMode('term'); }
      }
    });
  }

  function pollRadar() {
    api('/api/agents').then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('Pairing expired — open a fresh link.'); return null; }
      if (!r.ok) throw new Error('agents'); return r.json();
    }).then(function (agents) { if (!agents) return; setConn(true); renderRadar(agents); })
      .catch(function () { setConn(false); });
  }
  function startRadar() {
    show('radar'); $('back').hidden = true; $('mode').hidden = true; $('title').textContent = 'gtmux'; $('sub').textContent = '';
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
    if (!term) return;
    term.options.theme = xtermTheme();
    term.options.fontFamily = resolveFont();
    term.options.fontSize = termSize();
    document.body.style.background = (theme && theme.background) || GHOSTTY.bg;
    try { fit.fit(); } catch (e) {}
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
      term.write(text.slice(prev.length));
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
    });
  }

  function pollPane() {
    if (!curPane) return;
    api('/api/pane?id=' + encodeURIComponent(curPane)).then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('Pairing expired — open a fresh link.'); return null; }
      if (!r.ok) throw new Error('pane'); return r.json();
    }).then(function (j) { if (!j) return; setConn(true); writePane(j.text); })
      .catch(function () { setConn(false); });
  }
  // openAgent enters the pane view for an agent; the selected tab (terminal mirror
  // vs chat history) is remembered across agents (gtmux.paneMode).
  function openAgent(a) {
    curPane = a.pane_id; curAgent = a; $('back').hidden = false; $('mode').hidden = false;
    $('title').textContent = primary(a); $('sub').textContent = (a.agent || '') + ' · ' + secondary(a);
    clearInterval(radarTimer); radarTimer = null;
    applyMode();
  }
  // applyMode shows the active pane tab and (re)starts ONLY that tab's poll loop.
  function applyMode() {
    syncModeButtons();
    if (paneMode === 'chat') {
      clearInterval(paneTimer); paneTimer = null;
      show('chat'); chatSig = ''; lastTurns = []; chatExpanded = {};
      pollChat(); clearInterval(chatTimer); chatTimer = setInterval(pollChat, 2500);
    } else {
      clearInterval(chatTimer); chatTimer = null;
      show('pane'); ensureTerm(); lastText = ''; pendingText = null; userScrolling = false;
      try { fit.fit(); } catch (e) {}
      pollPane(); clearInterval(paneTimer); paneTimer = setInterval(pollPane, 1200);
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
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('Pairing expired — open a fresh link.'); return null; }
      if (r.status === 404 || r.status === 503) { setConn(true); return []; } // no resume record / not available
      if (!r.ok) throw new Error('transcript'); return r.json();
    }).then(function (turns) { if (turns === null) return; setConn(true); renderChat(turns || []); })
      .catch(function () { setConn(false); });
  }
  function renderChat(turns) {
    lastTurns = turns;
    // only repaint when content changed (keeps scroll position + expanded steps)
    var sig = JSON.stringify(turns.map(function (t) { return [t.prompt, t.response, (t.steps || []).length]; }));
    if (sig === chatSig) return;
    chatSig = sig;
    drawChat(turns);
  }
  function drawChat(turns) {
    var root = $('chat');
    var atBottom = root.scrollHeight - root.scrollTop - root.clientHeight < 48;
    var prevTop = root.scrollTop;
    root.innerHTML = '';
    var col = document.createElement('div'); col.className = 'chat-col';
    var a = curAgent || {};

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
      var ct = document.createElement('div'); ct.className = 'cturn';
      if (t.prompt) {
        var ur = document.createElement('div'); ur.className = 'urow';
        var ub = document.createElement('div'); ub.className = 'ububble'; ub.textContent = t.prompt;
        ur.appendChild(ub); ct.appendChild(ur);
      }
      if (t.steps && t.steps.length) {
        var open = !!chatExpanded[idx];
        var tog = document.createElement('button'); tog.className = 'steps-toggle';
        tog.textContent = (open ? '▾ ' : '▸ ') + t.steps.length + ' step' + (t.steps.length > 1 ? 's' : '');
        tog.onclick = (function (k) { return function () { chatExpanded[k] = !chatExpanded[k]; drawChat(lastTurns); }; })(idx);
        ct.appendChild(tog);
        if (open) {
          t.steps.forEach(function (s) {
            var row = document.createElement('div'); row.className = 'step-row';
            var sn = document.createElement('span'); sn.className = 'step-name'; sn.textContent = s.title || ''; row.appendChild(sn);
            if (s.detail) { var sd = document.createElement('span'); sd.className = 'step-detail'; sd.textContent = s.detail; row.appendChild(sd); }
            ct.appendChild(row);
          });
        }
      }
      if (t.response) {
        var ar = document.createElement('div'); ar.className = 'arow';
        ar.appendChild(avatarEl(a, 26, false));
        var ab = document.createElement('div'); ab.className = 'abubble'; ab.appendChild(mdRender(t.response));
        ar.appendChild(ab); ct.appendChild(ar);
      }
      col.appendChild(ct);
    });
    root.appendChild(col);
    root.scrollTop = atBottom ? root.scrollHeight : prevTop;
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

  function boot() {
    $('back').onclick = startRadar;
    setupKeyboard();
    setupMode();
    try { token = localStorage.getItem(TOKEN_KEY); } catch (e) {}
    var m = /(?:^|[#&])c=([a-f0-9]+)/i.exec(location.hash || '');
    var code = m && m[1];
    if (code) { try { history.replaceState(null, '', location.pathname + location.search); } catch (e) {} }
    var ready = code ? pair(code).catch(function () { return null; }) : Promise.resolve();
    ready.then(function () {
      if (!token) return gate('Open the pairing link from `gtmux serve` / `gtmux tunnel`, or from the phone app ("open on computer").');
      fetchTheme(); // match the user's real terminal (async; the pane picks it up)
      setupSettings();
      startRadar();
    });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
})();
