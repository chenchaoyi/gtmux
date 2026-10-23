(function (scope) {
  'use strict';
  var modsOrder = ['control', 'alt', 'shift'];
  function ordered(mods) { return modsOrder.filter(function (m) { return mods.indexOf(m) >= 0; }); }
  function supports(key, mods) { var wanted = ordered(mods).join(','); return key.modifier_sets.some(function (set) { return ordered(set).join(',') === wanted; }); }
  function create(opts) {
    var zh = opts.zh, cap = null, target = '', mods = [], group = 'navigation', busy = false, uncertain = false, pending = null, label = '', generation = 0, reading = false, clockOffset = 0;
    var button = document.createElement('button'); button.type = 'button'; button.className = 'tk-entry'; button.textContent = zh ? '按键' : 'Keys'; button.title = zh ? '打开终端按键' : 'Open terminal keys';
    var notice = document.createElement('div'); notice.className = 'tk-notice'; notice.hidden = true;
    var overlay = null, grid, feedback, modButtons = [], lastFocus;
    function b(text, action, parent) { var el = document.createElement('button'); el.type = 'button'; el.textContent = text; el.onclick = action; parent.appendChild(el); return el; }
    function usable() { return !!cap && cap.can_input && opts.canInput() && getTarget() === target; }
    function getTarget() { return opts.getId() || ''; }
    function blocked() { return busy || uncertain; }
    function now() { return Date.now() + clockOffset; }
    function clearTarget() { generation++; target = getTarget(); cap = null; mods = []; pending = null; uncertain = false; busy = false; reading = false; label = ''; close(); drawNotice(); }
    function refresh() {
      if (getTarget() !== target) clearTarget();
      if (!target || reading) return Promise.resolve();
      var epoch = generation; reading = true;
      return opts.request('/api/input-capabilities?id=' + encodeURIComponent(target) + (cap ? '&catalog_version=' + cap.version + '&catalog_boot=' + encodeURIComponent(cap.context.boot) : '')).then(function (r) { var date = r && r.headers && r.headers.get('Date'); if (date && Number.isFinite(Date.parse(date))) clockOffset = Date.parse(date) - Date.now(); return r && r.ok ? r.json() : null; }).then(function (next) {
        if (epoch !== generation) return;
        if (next && next.version === 1 && next.context && Array.isArray(next.actions)) {
          next.keys = next.keys || (cap && cap.keys); next.legacy_events = next.legacy_events || (cap && cap.legacy_events); cap = next;
        } else cap = null;
        if (!cap || !cap.can_input) mods = [];
        drawNotice(); if (overlay) render();
      }).catch(function () { if (epoch === generation) {cap = null; drawNotice(); if (overlay) render();} }).finally(function () {if (epoch === generation) reading = false;});
    }
    function drawNotice() {
      notice.innerHTML = ''; notice.hidden = true;
      if (usable()) (cap.actions || []).forEach(function (a) { notice.hidden = false; var el = b((a.count || 1) + (zh ? ' 个问题 · 查看' : a.count > 1 ? ' questions · View' : ' question · View'), function () { if (fire(a.event, a) && opts.onAction) opts.onAction(a); }, notice); el.disabled = blocked(); });
      if (label) {notice.hidden = false;var p = document.createElement('span');p.textContent = label;p.setAttribute('role', 'status');notice.appendChild(p);}
      if (uncertain) b(zh ? '查看按键结果' : 'View key result', open, notice);
    }
    function say(text) { label = text; drawNotice(); if (feedback) feedback.textContent = text; }
    function eventLabel(event) { return ordered(event.modifiers).map(function (m) {return m === 'control' ? 'Ctrl' : m === 'alt' ? 'Alt' : 'Shift';}).concat([event.key]).join(' + '); }
    function deliver(body, text) {
      if (busy) return;
      var epoch = generation, id = target;
      busy = true; drawNotice(); if (overlay) render();
      return opts.send(id, body).then(function (r) {
        return (r ? r.json().catch(function () {return {};}) : Promise.resolve({})).then(function (j) {
          if (epoch !== generation) return;
          if (r && r.ok) { uncertain = false; pending = null; if (typeof j.text === 'string') opts.echo(j.text, j.cols); drawNotice(); say(text + ' · ' + (zh ? '已发送到终端' : 'Sent to terminal')); }
          else {
            uncertain = !r || r.status >= 500 || String(j.error || '').indexOf('unconfirmed') >= 0; pending = uncertain ? body : null;
            drawNotice(); say(uncertain ? (zh ? '结果未确认，请先核对当前画面' : 'Result unconfirmed. Check the current screen first') : (j.error || (zh ? '按键未能发送' : 'The key could not be sent')));
          }
        });
      }).catch(function () { if (epoch === generation) {uncertain = true; pending = body; drawNotice(); say(zh ? '结果未确认，请先核对当前画面' : 'Result unconfirmed. Check the current screen first');} }).finally(function () {
        if (epoch !== generation) return;
        busy = false; if (overlay) render(); refresh();
      });
    }
    function fire(event, action) {
      if (!usable() || blocked()) return;
      if (cap.context.expires_at * 1000 <= now()) {say(zh ? '目标信息已过期，请重新选择' : 'Target context expired. Select again'); refresh(); return;}
      var body = {send_id: 'key-' + Date.now() + '-' + Math.random().toString(36).slice(2), input_context: cap.context};
      if (action) body.action = {id: action.id, revision: action.revision}; else body.key_event = event;
      return deliver(body, eventLabel(event));
    }
    function close() {mods = []; if (overlay) {overlay.remove(); overlay = null;} if (lastFocus && lastFocus.isConnected) lastFocus.focus(); feedback = null;}
    function render() {
      if (!overlay) return;
      grid.innerHTML = '';
      grid.dataset.group = group;
      modButtons.forEach(function (el, i) {var selected = mods.indexOf(modsOrder[i]) >= 0; el.textContent = (selected ? '✓ ' : '') + ['Ctrl', 'Alt', 'Shift'][i]; el.setAttribute('aria-pressed', String(selected)); el.disabled = !usable() || blocked();});
      if (!cap) {var empty = document.createElement('p'); empty.textContent = zh ? '通用按键暂不可用，原有快捷键仍可使用' : 'General keys unavailable; existing shortcuts remain available';grid.appendChild(empty);}
      if (group === 'characters') {var aliases = document.createElement('p');aliases.className = 'tk-aliases';aliases.textContent = 'Ctrl+I = Tab · Ctrl+M = Return · Ctrl+[ = Esc';grid.appendChild(aliases);}
      ((cap && cap.keys) || []).filter(function (k) {return k.group === group;}).forEach(function (k) {
        var event = {key: k.key, modifiers: ordered(mods)}, valid = supports(k, mods);
        var el = b(zh ? k.label_zh : k.label, function () {mods = [];fire(event);render();}, grid);
        el.disabled = !usable() || blocked() || !valid;el.title = eventLabel(event) + (valid ? '' : zh ? '：当前终端不支持此组合' : ': unsupported combination');el.setAttribute('aria-label', el.title);el.dataset.key = k.key;
      });
      feedback.innerHTML = ''; var text = document.createElement('p'); text.setAttribute('role', 'status'); text.textContent = busy ? (zh ? '发送中…' : 'Sending…') : label;feedback.appendChild(text);
      if (uncertain) {
        if (pending && cap && pending.input_context.boot === cap.context.boot && pending.input_context.pane_binding === cap.context.pane_binding && pending.input_context.expires_at * 1000 > now()) b(zh ? '重试同一按键' : 'Retry same key', function () {deliver(pending, label);}, feedback).disabled = busy;
        b(zh ? '不重试，继续操作' : 'Continue without retry', function () {uncertain = false;pending = null;label = '';drawNotice();render();}, feedback).disabled = busy;
      }
    }
    function open() {
      lastFocus = document.activeElement;mods = [];
      if (overlay) return;
      overlay = document.createElement('div');overlay.className = 'tk-backdrop';
      overlay.onclick = function (e) {if (e.target === overlay) close();};
      var panel = document.createElement('section');panel.className = 'tk-panel';panel.setAttribute('role', 'dialog');panel.setAttribute('aria-modal', 'true');panel.setAttribute('aria-label', zh ? '终端按键' : 'Terminal keys');
      var head = document.createElement('header');var title = document.createElement('strong');title.textContent = (zh ? '终端按键 ' : 'Terminal keys ') + getTarget();head.appendChild(title);var closeButton = b(zh ? '关闭' : 'Close', close, head);panel.appendChild(head);
      var modifierRow = document.createElement('div');modifierRow.className = 'tk-modifiers';modButtons = modsOrder.map(function (m) {return b(m, function () {mods = mods.indexOf(m) >= 0 ? mods.filter(function (x) {return x !== m;}) : mods.concat([m]);render();}, modifierRow);});panel.appendChild(modifierRow);
      var categories = document.createElement('div');categories.className = 'tk-categories';
      [['navigation', zh ? '导航' : 'Navigation'], ['function', zh ? '功能' : 'Function'], ['characters', zh ? '字符' : 'Characters']].forEach(function (g) {var tab = b(g[1], function () {group = g[0];Array.prototype.forEach.call(categories.children, function (child) {child.setAttribute('aria-pressed', String(child === tab));});render();}, categories);tab.setAttribute('aria-pressed', String(group === g[0]));});panel.appendChild(categories);
      grid = document.createElement('div');grid.className = 'tk-grid';panel.appendChild(grid);feedback = document.createElement('div');feedback.className = 'tk-feedback';panel.appendChild(feedback);overlay.appendChild(panel);document.body.appendChild(overlay);
      overlay.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') {e.preventDefault();close();}
        if (e.key !== 'Tab') return;
        var focusable = panel.querySelectorAll('button:not([disabled])'), first = focusable[0], last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {e.preventDefault();last.focus();} else if (!e.shiftKey && document.activeElement === last) {e.preventDefault();first.focus();}
      });
      render();closeButton.focus();refresh();
    }
    button.onclick = open;
    function visibility() { if (document.hidden) {mods = []; if (overlay) render();} else refresh(); }
    document.addEventListener('visibilitychange', visibility);
    function dispose() {generation++; clearInterval(timer); document.removeEventListener('visibilitychange', visibility); close();}

    var timer = setInterval(function () {if (!button.isConnected) {dispose();return;} if (document.hidden) {mods = [];return;} if (getTarget() !== target) clearTarget(); if (!opts.canInput()) close(); if (button.offsetParent !== null) refresh();}, 8000);
    Promise.resolve().then(function () {if (button.isConnected) refresh();});
    function sync(visible) {var changed = getTarget() !== target;if (changed) clearTarget();if (visible && opts.canInput() && changed) refresh();if (!visible || !opts.canInput()) {mods = []; close(); drawNotice();}}
    return {open: open, key: function (event) {if (!usable()) return false;fire(event);return true;}, sync: sync, button: button, notice: notice, blocked: blocked, refresh: refresh, dispose: dispose,
      legacy: function (key) {if (blocked() || !opts.canInput()) return true;var event = cap && cap.legacy_events && cap.legacy_events[key];if (event) fire(event);else if (!cap) deliver({key: key, send_id: 'key-' + Date.now() + '-' + Math.random().toString(36).slice(2)}, key);return true;},
      digit: function (n) {if (blocked() || !opts.canInput() || !Number.isInteger(n) || n < 1 || n > 9) return;if (cap) fire({key: String(n), modifiers: []});else deliver({text: String(n), send_id: 'key-' + Date.now() + '-' + Math.random().toString(36).slice(2)}, String(n));}};
  }
  var api = {create: create, supports: supports};
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else scope.GtmuxTerminalKeys = api;
})(typeof window === 'undefined' ? globalThis : window);
