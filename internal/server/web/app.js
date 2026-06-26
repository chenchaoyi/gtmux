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
  var token = null, radarTimer = null, paneTimer = null;
  var term = null, fit = null, lastText = '', curPane = null, lastSig = '', theme = null;
  var iconCache = {}; // agentName -> objectURL | 'none' | Promise
  var BUNDLED = ['Hack', 'JetBrains Mono', 'Fira Code', 'IBM Plex Mono'];
  function lsGet(k, d) { try { return localStorage.getItem(k) || d; } catch (e) { return d; } }
  var fontPref = lsGet('gtmux.fontPref', 'auto');          // 'auto' | 'system' | a bundled family
  var sizePref = parseInt(lsGet('gtmux.fontSize', '0'), 10) || 0;  // 0 = follow terminal/default

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
  function show(which) { ['gate', 'radar', 'pane'].forEach(function (id) { $(id).hidden = id !== which; }); }
  function gate(msg) { $('gate-msg').textContent = msg; show('gate'); }
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

  // ---- radar ------------------------------------------------------------
  function rowEl(a) {
    var st = COLORS[a.status] ? a.status : 'running';
    var row = document.createElement('div');
    row.className = 'row' + (st === 'waiting' ? ' waiting' : '');

    var av = document.createElement('div'); av.className = 'avatar';
    var img = document.createElement('img'); img.alt = ''; img.style.display = 'none';
    var mono = document.createElement('span'); mono.className = 'mono'; mono.textContent = agentMark(a.agent);
    var bdg = document.createElement('span'); bdg.className = 'badge'; bdg.innerHTML = badgeSVG(st);
    av.appendChild(img); av.appendChild(mono); av.appendChild(bdg);
    if (a.icon) loadIcon(a.agent, img, mono);

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
    row.onclick = function () { openPane(a); };
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
  }

  function pollRadar() {
    api('/api/agents').then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('Pairing expired — open a fresh link.'); return null; }
      if (!r.ok) throw new Error('agents'); return r.json();
    }).then(function (agents) { if (!agents) return; setConn(true); renderRadar(agents); })
      .catch(function () { setConn(false); });
  }
  function startRadar() {
    show('radar'); $('back').hidden = true; $('title').textContent = 'gtmux'; $('sub').textContent = '';
    curPane = null; lastSig = ''; clearInterval(paneTimer); paneTimer = null;
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
    window.addEventListener('resize', function () { try { fit.fit(); } catch (e) {} });
  }
  function normalize(t) { return t.indexOf('⏺') === -1 ? t : t.split('⏺').join('●'); }

  // Incremental: when the snapshot only GREW (history appended — the common case),
  // write just the new tail — no reset, so no flash. Only a real redraw resets.
  function writePane(text) {
    text = normalize(text || '');
    if (text === lastText) return;
    var prev = lastText; lastText = text;
    if (prev && text.length > prev.length && text.lastIndexOf(prev, 0) === 0) {
      term.write(text.slice(prev.length));
      return;
    }
    var b = term.buffer.active, wasBottom = b.viewportY >= b.baseY;
    term.reset();
    term.write(text, function () { if (wasBottom) term.scrollToBottom(); });
  }

  function pollPane() {
    if (!curPane) return;
    api('/api/pane?id=' + encodeURIComponent(curPane)).then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); gate('Pairing expired — open a fresh link.'); return null; }
      if (!r.ok) throw new Error('pane'); return r.json();
    }).then(function (j) { if (!j) return; setConn(true); writePane(j.text); })
      .catch(function () { setConn(false); });
  }
  function openPane(a) {
    curPane = a.pane_id; show('pane'); $('back').hidden = false;
    $('title').textContent = primary(a); $('sub').textContent = (a.agent || '') + ' · ' + secondary(a);
    clearInterval(radarTimer); radarTimer = null;
    ensureTerm(); lastText = '';
    try { fit.fit(); } catch (e) {}
    pollPane(); clearInterval(paneTimer); paneTimer = setInterval(pollPane, 1200);
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

  function boot() {
    $('back').onclick = startRadar;
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
