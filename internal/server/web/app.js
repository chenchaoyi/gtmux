/* gtmux browser mirror — view-only. Pairs via a one-time #c=<code> link, then
 * polls /api/agents (radar) and /api/pane (live terminal, xterm.js). No input. */
(function () {
  'use strict';
  var TOKEN_KEY = 'gtmux.token';
  var COLORS = { waiting: '#EF4444', working: '#06B6D4', idle: '#22C55E', running: '#8E8E93' };
  var ORDER = ['waiting', 'working', 'idle', 'running'];
  var LABEL = { waiting: 'needs you', working: 'working', idle: 'idle', running: 'running' };

  var $ = function (id) { return document.getElementById(id); };
  var token = null;
  var radarTimer = null, paneTimer = null;
  var term = null, fit = null, lastText = '', curPane = null;

  // ---- auth -------------------------------------------------------------
  function authHeaders() { return token ? { Authorization: 'Bearer ' + token } : {}; }

  function api(path, opts) {
    opts = opts || {};
    opts.headers = Object.assign({}, opts.headers || {}, authHeaders());
    return fetch(path, opts);
  }

  // Redeem a one-time enroll code (from the #c= fragment) for a device token.
  function pair(code) {
    return fetch('/api/enroll', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enrollCode: code, name: 'browser' }),
    }).then(function (r) {
      if (!r.ok) throw new Error('pair-failed');
      return r.json();
    }).then(function (j) {
      token = j.token;
      try { localStorage.setItem(TOKEN_KEY, token); } catch (e) {}
    });
  }

  // ---- views ------------------------------------------------------------
  function show(which) {
    ['gate', 'radar', 'pane'].forEach(function (id) { $(id).hidden = id !== which; });
  }
  function gate(msg) { $('gate-msg').textContent = msg; show('gate'); }
  function setConn(live) {
    $('conn').className = 'conn ' + (live ? 'live' : 'off');
    $('conn').textContent = live ? 'live' : 'offline';
  }

  // ---- radar ------------------------------------------------------------
  function primary(a) { return a.task || a.session || a.loc || a.pane_id || ''; }
  function secondary(a) {
    var base = a.session || a.loc || '';
    return a.pane_id ? base + ' · ' + a.pane_id : base;
  }

  function renderRadar(agents) {
    var byStatus = { waiting: [], working: [], idle: [], running: [] };
    agents.forEach(function (a) { (byStatus[a.status] || byStatus.running).push(a); });
    var root = $('radar');
    root.innerHTML = '';
    ORDER.forEach(function (st) {
      var list = byStatus[st];
      if (!list.length) return;
      var lbl = document.createElement('div');
      lbl.className = 'group-label';
      lbl.textContent = LABEL[st] + '  ' + list.length;
      root.appendChild(lbl);
      list.forEach(function (a) {
        var row = document.createElement('div');
        row.className = 'row';
        var dot = document.createElement('span');
        dot.className = 'dot' + (st === 'waiting' ? ' sq' : '');
        dot.style.background = COLORS[st] || COLORS.running;
        var text = document.createElement('div');
        text.className = 'text';
        var p = document.createElement('div'); p.className = 'primary'; p.textContent = primary(a);
        var s = document.createElement('div'); s.className = 'secondary'; s.textContent = secondary(a);
        text.appendChild(p); text.appendChild(s);
        var ag = document.createElement('span'); ag.className = 'agent'; ag.textContent = a.agent || '';
        row.appendChild(dot); row.appendChild(text); row.appendChild(ag);
        row.onclick = function () { openPane(a); };
        root.appendChild(row);
      });
    });
    if (!root.children.length) {
      var empty = document.createElement('div');
      empty.className = 'group-label';
      empty.textContent = 'no agents';
      root.appendChild(empty);
    }
  }

  function pollRadar() {
    api('/api/agents').then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); return gate('Pairing expired — open a fresh link.'); }
      if (!r.ok) throw new Error('agents');
      return r.json();
    }).then(function (agents) {
      if (!agents) return;
      setConn(true);
      renderRadar(agents);
    }).catch(function () { setConn(false); });
  }

  function startRadar() {
    show('radar');
    $('back').hidden = true;
    $('title').textContent = 'gtmux';
    $('sub').textContent = '';
    curPane = null;
    clearInterval(paneTimer); paneTimer = null;
    pollRadar();
    clearInterval(radarTimer);
    radarTimer = setInterval(pollRadar, 2000);
  }

  // ---- pane mirror ------------------------------------------------------
  function ensureTerm() {
    if (term) return;
    term = new Terminal({
      convertEol: true, cursorBlink: false, disableStdin: true,
      cursorInactiveStyle: 'none', scrollback: 5000, allowProposedApi: true,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace', fontSize: 13,
      theme: { background: '#0B0B0F', foreground: '#D6D6DA' },
    });
    fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    try { var u = new Unicode11Addon.Unicode11Addon(); term.loadAddon(u); term.unicode.activeVersion = '11'; } catch (e) {}
    term.open($('term'));
    window.addEventListener('resize', function () { try { fit.fit(); } catch (e) {} });
  }

  // iOS/WebKit (and Safari) render U+23FA as the red record-button emoji; swap for
  // U+25CF so it's a clean ANSI-colored dot, matching the mobile renderer.
  function normalize(t) { return t.indexOf('⏺') === -1 ? t : t.split('⏺').join('●'); }

  function writePane(text) {
    text = normalize(text || '');
    if (text === lastText) return;
    lastText = text;
    term.reset();
    term.write(text);
  }

  function pollPane() {
    if (!curPane) return;
    api('/api/pane?id=' + encodeURIComponent(curPane)).then(function (r) {
      if (r.status === 401) { token = null; localStorage.removeItem(TOKEN_KEY); return gate('Pairing expired — open a fresh link.'); }
      if (!r.ok) throw new Error('pane');
      return r.json();
    }).then(function (j) {
      if (!j) return;
      setConn(true);
      writePane(j.text);
    }).catch(function () { setConn(false); });
  }

  function openPane(a) {
    curPane = a.pane_id;
    show('pane');
    $('back').hidden = false;
    $('title').textContent = primary(a);
    $('sub').textContent = (a.agent || '') + ' · ' + secondary(a);
    clearInterval(radarTimer); radarTimer = null;
    ensureTerm();
    lastText = '';
    try { fit.fit(); } catch (e) {}
    pollPane();
    clearInterval(paneTimer);
    paneTimer = setInterval(pollPane, 1200);
  }

  // ---- boot -------------------------------------------------------------
  function boot() {
    $('back').onclick = startRadar;
    try { token = localStorage.getItem(TOKEN_KEY); } catch (e) {}
    var m = /(?:^|[#&])c=([a-f0-9]+)/i.exec(location.hash || '');
    var code = m && m[1];
    // strip the code from the address bar regardless
    if (code) { try { history.replaceState(null, '', location.pathname + location.search); } catch (e) {} }

    var ready = code ? pair(code).catch(function () { return null; }) : Promise.resolve();
    ready.then(function () {
      if (!token) return gate('Open the pairing link from `gtmux serve` / `gtmux tunnel`, or from the phone app ("open on computer").');
      startRadar();
    });
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
})();
