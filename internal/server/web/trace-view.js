/* A bounded reader for the public transcript index; input stays in Chat/Terminal. */
(function () {
  'use strict';
  window.GtmuxTraceView = {create: function (options) {
    var T = options.T, model = window.GtmuxTrace, pageSize = 200;
    var current = {events: [], sessions: []}, filtered = [], context = {}, selected = '', page = 0;
    var inspectorOpen = null, follow = false, initialized = false, rowNodes = new Map(), lastDetail = '', deferred = null, lastInput = null, lastScope = '';
    var root = el('div', 'trace-view'), toolbar = el('div', 'trace-toolbar');
    var status = el('span', 'trace-status'); status.title = T('Current session status', '当前会话状态'); toolbar.appendChild(status);
    var count = el('span', 'trace-count'); count.setAttribute('role', 'status'); toolbar.appendChild(count);
    var search = el('input', 'trace-search'); search.type = 'search'; search.placeholder = T('Search trace', '搜索轨迹'); search.setAttribute('aria-label', search.placeholder); toolbar.appendChild(search);
    var kind = el('select', 'trace-filter'); kind.setAttribute('aria-label', T('Record type', '记录类型'));
    [['all', T('All events', '全部记录')], ['user', T('Messages', '用户消息')], ['assistant', T('Agent replies', 'Agent 回复')], ['tool', T('Tool calls', '工具调用')]].forEach(function (item) { kind.appendChild(option(item[0], item[1])); }); toolbar.appendChild(kind);
    var session = el('select', 'trace-session'); session.setAttribute('aria-label', T('Conversation history', '历史会话')); toolbar.appendChild(session);
    var latest = button('trace-latest', T('Follow latest', '跟随最新'), function () { follow = !follow; if (follow) { selected = ''; page = Math.max(0, Math.floor((filtered.length - 1) / pageSize)); } draw(true); }); toolbar.appendChild(latest);
    var toggle = button('trace-toggle', T('Show details', '查看详情'), function () { panel(!inspectorOpen); if (inspectorOpen) detail.focus(); }); toolbar.appendChild(toggle); root.appendChild(toolbar);
    var overview = el('div', 'trace-overview'), map = el('div', 'trace-map'); map.setAttribute('aria-hidden', 'true'); overview.appendChild(map);
    var scrub = el('input', 'trace-scrubber'); scrub.type = 'range'; scrub.min = '0'; scrub.step = '1'; scrub.setAttribute('aria-label', T('Locate a record', '定位记录')); overview.appendChild(scrub); root.appendChild(overview);
    var body = el('div', 'trace-body'), list = el('div', 'trace-list'); list.setAttribute('aria-label', T('Trace events', '轨迹记录')); body.appendChild(list);
    var detail = el('section', 'trace-inspector'); detail.id = 'trace-detail-' + Math.random().toString(36).slice(2); detail.tabIndex = -1; detail.setAttribute('aria-label', T('Event details', '记录详情')); toggle.setAttribute('aria-controls', detail.id);
    var head = el('div', 'trace-detail-head'), title = el('h3', 'trace-detail-title'); head.appendChild(title);
    var actions = el('div', 'trace-detail-actions'); head.appendChild(actions);
    var close = button('trace-close', T('Close details', '关闭详情'), function () { panel(false); var row = rowNodes.get(selected); (row && row.isConnected ? row : toggle).focus({preventScroll: true}); }); close.innerHTML = options.icon('close'); head.appendChild(close); detail.appendChild(head);
    var metadata = el('p', 'trace-meta'); detail.appendChild(metadata);
    var path = el('div', 'trace-workdir'); detail.appendChild(path);
    var modes = el('div', 'trace-detail-modes'); modes.setAttribute('role', 'group'); modes.setAttribute('aria-label', T('Text presentation', '正文显示方式'));
    var preview = button('trace-preview-mode', T('Preview', '预览'), function () { detailMode = 'preview'; renderDetail(true); });
    var source = button('trace-source-mode', T('Source', '原文'), function () { detailMode = 'source'; renderDetail(true); }); modes.appendChild(preview); modes.appendChild(source); detail.appendChild(modes);
    var detailMode = 'preview', warning = el('p', 'trace-truncated'); detail.appendChild(warning);
    var content = el('div', 'trace-detail-content'); detail.appendChild(content); body.appendChild(detail); root.appendChild(body);
    var footer = el('details', 'trace-limits'), summary = el('summary', '', T('About these records', '记录范围')); footer.appendChild(summary);
    footer.appendChild(el('p', '', T('Conversation text and recorded tool input. Tool output, call timing and token usage are not recorded. Turn numbers refer to loaded history.', '展示会话正文与已记录的调用参数，不含工具输出、调用耗时或 Token 用量。轮次按当前载入的历史编号。'))); root.appendChild(footer);

    function el(tag, name, text) { var node = document.createElement(tag); if (name) node.className = name; if (text !== undefined) node.textContent = text; return node; }
    function option(value, label) { var node = el('option', '', label); node.value = value; return node; }
    function button(name, label, action) { var node = el('button', name, label); node.type = 'button'; node.title = label; node.setAttribute('aria-label', label); node.onclick = action; return node; }
    function label(event) { return event.title || (event.kind === 'tool' ? T('Tool call', '工具调用') : event.kind === 'user' ? T('User message', '用户消息') : event.agent || T('Agent reply', 'Agent 回复')); }
    function turnLabel(event) { return T('Turn ', '第 ') + event.turn + T('', ' 轮'); }
    function compact() { return (root.clientWidth || context.width || window.innerWidth) < 700; }
    function updateStatus() {
      var historical = session.value && session.value !== 'all' && current.sessions.length && session.value !== current.sessions[current.sessions.length - 1].key;
      var presentation = options.status ? options.status(context) : null;
      status.hidden = !historical && !presentation;
      status.textContent = historical ? T('History', '历史会话') : presentation ? presentation.text : '';
      status.style.setProperty('--trace-state', historical ? 'var(--fg3)' : presentation ? presentation.color : 'var(--fg3)');
      latest.hidden = historical || !current.events.length;
    }
    function syncCompact() { var narrow = compact(), covered = narrow && inspectorOpen; root.classList.toggle('trace-compact', narrow); list.hidden = covered; list.inert = covered; }
    function panel(open) {
      inspectorOpen = !!open; detail.hidden = !inspectorOpen; root.classList.toggle('inspector-closed', !inspectorOpen); root.classList.toggle('inspector-open', inspectorOpen);
      toggle.textContent = inspectorOpen ? T('Hide details', '收起详情') : T('Show details', '查看详情'); toggle.title = toggle.textContent; toggle.setAttribute('aria-label', toggle.title); toggle.setAttribute('aria-expanded', String(inspectorOpen));
      syncCompact();
    }
    function selectionHeld() {
      var value = window.getSelection && window.getSelection();
      return !!(value && !value.isCollapsed && content.contains(value.anchorNode)) || actions.contains(document.activeElement) || content.contains(document.activeElement);
    }
    function renderDetail(force) {
      var event = filtered.find(function (item) { return item.key === selected; });
      if (!event) { detail.hidden = true; toggle.disabled = true; return; }
      toggle.disabled = false; detail.hidden = !inspectorOpen;
      var signature = JSON.stringify([event, detailMode]);
      if (!force && signature === lastDetail) return;
      if (!force && selectionHeld()) {
        if (!deferred) deferred = setTimeout(function flush() { deferred = null; if (!root.isConnected) return; if (selectionHeld()) deferred = setTimeout(flush, 250); else renderDetail(false); }, 250);
        return;
      }
      lastDetail = signature;
      var scroll = detail.scrollTop;
      title.textContent = label(event);
      metadata.textContent = [event.agent, turnLabel(event), event.session, event.time && !isNaN(new Date(event.time).getTime()) ? T('Turn started ', '本轮开始于 ') + options.formatTime(new Date(event.time)) : ''].filter(Boolean).join(' · ');
      path.textContent = event.step && event.step.workdir ? T('Working directory: ', '工作目录：') + event.step.workdir : ''; path.hidden = !path.textContent;
      modes.hidden = event.kind === 'tool'; preview.setAttribute('aria-pressed', String(detailMode === 'preview')); source.setAttribute('aria-pressed', String(detailMode === 'source'));
      warning.textContent = event.step && event.step.truncated ? T('Only the recorded portion of this input is available.', '调用参数较长，仅展示已记录的部分。') : event.step && !event.step.input ? T('Only a call summary was recorded.', '此调用仅记录了摘要。') : ''; warning.hidden = !warning.textContent;
      content.replaceChildren();
      if (event.kind !== 'tool' && detailMode === 'preview') { var prose = el('div', 'trace-prose'); prose.appendChild(options.markdown(event.body, {session_id: event.session, home_id: context.home_id || ''})); content.appendChild(prose); }
      else { var code = el('pre', 'trace-source' + (event.step && !event.step.input ? ' trace-summary' : ''), event.body || T('No detail recorded.', '未记录详情。')); code.tabIndex = 0; content.appendChild(code); }
      actions.replaceChildren(options.actions(event.body || label(event))); detail.scrollTop = scroll;
    }
    function inspect(event, reveal) {
      if (!event) return;
      selected = event.key;
      rowNodes.forEach(function (row, key) { row.classList.toggle('on', key === selected); row.setAttribute('aria-pressed', String(key === selected)); });
      if (reveal) { follow = false; panel(true); }
      var position = filtered.findIndex(function (item) { return item.key === selected; });
      scrub.value = String(Math.max(0, position)); scrub.setAttribute('aria-valuetext', (position + 1) + ' / ' + filtered.length + ' · ' + turnLabel(event) + ' · ' + label(event));
      latest.setAttribute('aria-pressed', String(follow)); renderDetail(!!reveal);
      if (reveal && compact()) detail.focus({preventScroll: true});
    }
    function rowFor(event) {
      var row = rowNodes.get(event.key);
      if (!row) {
        row = el('button', 'trace-event ' + event.kind); row.type = 'button'; row.dataset.key = event.key;
        var identity = el('span', 'trace-kind'); identity.appendChild(el('span', '', event.kind === 'tool' ? T('Tool', '工具') : event.kind === 'user' ? T('Message', '消息') : T('Agent', 'Agent'))); row.appendChild(identity);
        var text = el('span', 'trace-row-text'); text.appendChild(el('span', 'trace-row-title')); text.appendChild(el('span', 'trace-preview')); row.appendChild(text);
        rowNodes.set(event.key, row);
      }
      row.querySelector('.trace-row-title').textContent = label(event);
      row.querySelector('.trace-preview').textContent = event.body.replace(/\s+/g, ' ');
      row.setAttribute('aria-controls', detail.id); row.setAttribute('aria-pressed', String(event.key === selected)); row.classList.toggle('on', event.key === selected);
      row.onclick = function () { inspect(event, true); };
      return row;
    }
    function pager(text, action) { var node = button('trace-page', text, action); list.appendChild(node); }
    function renderList(scrollToSelected) {
      var active = document.activeElement, focusedKey = active && active.dataset && active.dataset.key;
      var top = list.scrollTop, from = page * pageSize, until = Math.min(filtered.length, from + pageSize);
      list.replaceChildren();
      if (!filtered.length) {
        list.appendChild(el('p', 'chat-empty', current.events.length ? T('No matching records.', '没有匹配的记录。') : T('No conversation records yet.', '暂无会话记录。')));
        if (current.events.length) pager(T('Clear filters', '清除筛选'), function () { search.value = ''; kind.value = session.value = 'all'; changedFilter(); });
      }
      if (from > 0) pager(T('Previous ', '查看前 ') + pageSize + T(' records', ' 条'), function () { page--; renderList(true); list.scrollTop = 0; });
      var previous = null;
      filtered.slice(from, until).forEach(function (event) {
        if (!previous || previous.sessionKey !== event.sessionKey || previous.turn !== event.turn) {
          var heading = el('div', 'trace-turn', [event.agent, turnLabel(event), current.sessions.length > 1 ? event.session : ''].filter(Boolean).join(' · '));
          if (previous && previous.sessionKey !== event.sessionKey || event.boundary && (!previous || previous.turn !== event.turn)) heading.classList.add('trace-session-break');
          list.appendChild(heading);
        }
        list.appendChild(rowFor(event)); previous = event;
      });
      if (until < filtered.length) pager(T('Next ', '查看后 ') + pageSize + T(' records', ' 条'), function () { page++; renderList(true); list.scrollTop = 0; });
      list.scrollTop = top;
      if (focusedKey && rowNodes.get(focusedKey) && rowNodes.get(focusedKey).isConnected) rowNodes.get(focusedKey).focus({preventScroll: true});
      if (scrollToSelected) {
        var target = rowNodes.get(selected), targetKey = selected;
        if (target && target.isConnected) target.scrollIntoView({block: 'nearest'});
        else window.requestAnimationFrame(function () { if (selected === targetKey && target && target.isConnected) target.scrollIntoView({block: 'nearest'}); });
      }
      var visibleKeys = new Set(filtered.slice(from, until).map(function (event) { return event.key; }));
      rowNodes.forEach(function (row, key) { if (!visibleKeys.has(key)) rowNodes.delete(key); });
    }
    function draw(scrollToSelected) {
      filtered = model.filter(current.events, search.value, kind.value, session.value);
      if (follow || !filtered.some(function (event) { return event.key === selected; })) selected = filtered.length ? filtered[filtered.length - 1].key : '';
      var position = filtered.findIndex(function (event) { return event.key === selected; });
      if (follow || scrollToSelected) page = Math.max(0, Math.floor(position / pageSize));
      page = Math.min(page, Math.max(0, Math.floor((filtered.length - 1) / pageSize)));
      count.textContent = (filtered.length === current.events.length ? current.events.length : filtered.length + ' / ' + current.events.length) + T(' records', ' 条');
      count.title = current.turns + T(' loaded turns', ' 轮已载入会话');
      latest.setAttribute('aria-pressed', String(follow)); latest.hidden = !current.events.length;
      updateStatus();
      map.replaceChildren(); model.overview(filtered, 120).forEach(function (bucket) { map.appendChild(el('span', 'trace-mark ' + bucket.kind)); });
      scrub.max = String(Math.max(0, filtered.length - 1)); scrub.disabled = !filtered.length; overview.hidden = filtered.length < 2;
      renderList(scrollToSelected); inspect(filtered[position], false);
      if (!filtered.length) { panel(false); detail.hidden = true; toggle.disabled = true; }
    }
    function changedFilter() { follow = false; selected = ''; page = 0; lastDetail = ''; if (compact()) panel(false); draw(true); }
    search.oninput = kind.onchange = session.onchange = changedFilter;
    function stopFollowing() { follow = false; latest.setAttribute('aria-pressed', 'false'); }
    list.addEventListener('wheel', stopFollowing, {passive: true});
    list.addEventListener('touchstart', stopFollowing, {passive: true});
    list.onkeydown = function (event) {
      if (!event.target.matches('.trace-event') || !['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return;
      var index = filtered.findIndex(function (item) { return item.key === event.target.dataset.key; });
      index = event.key === 'Home' ? 0 : event.key === 'End' ? filtered.length - 1 : Math.max(0, Math.min(filtered.length - 1, index + (event.key === 'ArrowUp' ? -1 : 1)));
      event.preventDefault(); stopFollowing(); selected = filtered[index].key; page = Math.floor(index / pageSize); renderList(true); inspect(filtered[index], false); rowNodes.get(selected).focus({preventScroll: true});
    };
    scrub.oninput = function () { follow = false; var event = filtered[Number(scrub.value)]; if (!event) return; page = Math.floor(Number(scrub.value) / pageSize); renderList(false); inspect(event, false); var row = rowNodes.get(event.key); if (row) row.scrollIntoView({block: 'nearest'}); };
    detail.onkeydown = function (event) { if (event.key === 'Escape') { event.stopPropagation(); close.click(); } };
    var resize = new ResizeObserver(syncCompact); resize.observe(root);
    return {el: root, update: function (turns, nextContext) {
      context = nextContext || {};
      var scope = JSON.stringify([context.pane_id, context.session_id, context.home_id, context.agent]);
      if (lastInput === turns && lastScope === scope) { updateStatus(); return; }
      lastInput = turns; lastScope = scope;
      current = model.build(turns, context);
      var priorSession = session.value || 'all', sessionSignature = JSON.stringify(current.sessions.map(function (entry) { return [entry.key, entry.agent, entry.session]; }));
      if (session.dataset.signature !== sessionSignature) {
        session.dataset.signature = sessionSignature; session.replaceChildren(option('all', T('All sessions', '全部会话')));
        current.sessions.forEach(function (entry) { session.appendChild(option(entry.key, [entry.agent, entry.session].filter(Boolean).join(' · '))); });
        session.value = current.sessions.some(function (entry) { return entry.key === priorSession; }) ? priorSession : 'all';
      }
      session.hidden = current.sessions.length < 2;
      if (!initialized) { initialized = true; follow = context.status === 'working'; panel(!compact()); }
      var present = new Set(current.events.map(function (event) { return event.key; })); rowNodes.forEach(function (row, key) { if (!present.has(key)) rowNodes.delete(key); });
      draw(follow || !selected);
    }, dispose: function () { clearTimeout(deferred); resize.disconnect(); rowNodes.clear(); }};
  }};
})();
