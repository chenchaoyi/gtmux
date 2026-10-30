/* A bounded reader for the public transcript index; input stays in Chat/Terminal. */
(function () {
  'use strict';
  window.GtmuxTraceView = {create: function (options) {
    var T = options.T, model = window.GtmuxTrace, pageSize = 200;
    var current = {events: [], sessions: []}, filtered = [], presented = [], context = {}, selected = '', page = 0;
    var expandedTurns = {}, expandedCalls = {}, defaults = {turns: true, calls: true}, disclosureHold = false;
    var inspectorOpen = null, follow = true, inspecting = false, initialized = false, rowNodes = new Map(), lastDetail = '', lastDetailKey = '', deferred = null, lastInput = null, lastScope = '';
    var root = el('div', 'trace-view'), toolbar = el('div', 'trace-toolbar');
    var status = el('span', 'trace-status'); status.title = T('Current session status', '当前会话状态'); toolbar.appendChild(status);
    var count = el('span', 'trace-count'); count.setAttribute('role', 'status'); toolbar.appendChild(count);
    var search = el('input', 'trace-search'); search.type = 'search'; search.placeholder = T('Search trace', '搜索轨迹'); search.setAttribute('aria-label', search.placeholder); toolbar.appendChild(search);
    var kind = el('select', 'trace-filter'); kind.setAttribute('aria-label', T('Record type', '记录类型'));
    [['all', T('All events', '全部记录')], ['user', T('Messages', '用户消息')], ['assistant', T('Agent replies', 'Agent 回复')], ['tool', T('Tool calls', '工具调用')], ['context', T('Context', '上下文')]].forEach(function (item) { kind.appendChild(option(item[0], item[1])); }); toolbar.appendChild(kind);
    var foldTurns = button('trace-fold-turns', T('Collapse all turns', '收起所有轮次'), function () { foldAll('turns'); }); toolbar.appendChild(foldTurns);
    var foldCalls = button('trace-fold-calls', T('Collapse all calls', '收起所有调用'), function () { foldAll('calls'); }); toolbar.appendChild(foldCalls);
    var session = el('select', 'trace-session'); session.setAttribute('aria-label', T('Conversation history', '历史会话')); toolbar.appendChild(session);
    var latest = button('trace-latest', T('Jump to latest', '回到最新记录'), function () { follow = true; inspecting = false; disclosureHold = false; selected = filtered.length ? filtered[filtered.length - 1].key : ''; reveal(filtered[filtered.length - 1]); page = Math.max(0, Math.floor((presented.length - 1) / pageSize)); if (compact()) panel(false); draw(true); list.scrollTop = list.scrollHeight; }); latest.innerHTML = options.icon('down');
    var toggle = button('trace-toggle', T('Show details', '查看详情'), function () { panel(!inspectorOpen); if (inspectorOpen) detail.focus(); }); toolbar.appendChild(toggle); root.appendChild(toolbar);
    var overview = el('div', 'trace-overview'), map = el('div', 'trace-map'); map.setAttribute('aria-hidden', 'true'); overview.appendChild(map);
    var scrub = el('input', 'trace-scrubber'); scrub.type = 'range'; scrub.min = '0'; scrub.step = '1'; scrub.setAttribute('aria-label', T('Locate a record', '定位记录')); overview.appendChild(scrub); root.appendChild(overview);
    var body = el('div', 'trace-body'), list = el('div', 'trace-list'); list.setAttribute('aria-label', T('Trace events', '轨迹记录')); body.appendChild(list);
    var detail = el('section', 'trace-inspector'); detail.id = 'trace-detail-' + Math.random().toString(36).slice(2); detail.tabIndex = -1; detail.setAttribute('aria-label', T('Event details', '记录详情')); toggle.setAttribute('aria-controls', detail.id);
    var head = el('div', 'trace-detail-head'), title = el('h3', 'trace-detail-title'); head.appendChild(title);
    var actions = el('div', 'trace-detail-actions'); head.appendChild(actions);
    var close = button('trace-close', T('Close details', '关闭详情'), function () { inspecting = false; panel(false); (selectedRow() || toggle).focus({preventScroll: true}); }); close.innerHTML = options.icon('close'); head.appendChild(close); detail.appendChild(head);
    var metadata = el('p', 'trace-meta'); detail.appendChild(metadata);
    var path = el('div', 'trace-workdir'); detail.appendChild(path);
    var modes = el('div', 'trace-detail-modes'); modes.setAttribute('role', 'tablist'); modes.setAttribute('aria-label', T('Text presentation', '正文显示方式'));
    var overviewTab = button('trace-overview-mode', T('Overview', '概述'), function () { detailMode = 'overview'; renderDetail(true); });
    var preview = button('trace-preview-mode', T('Preview', '预览'), function () { detailMode = 'preview'; renderDetail(true); });
    var source = button('trace-source-mode', T('Raw content', '原始内容'), function () { detailMode = 'raw'; renderDetail(true); }); var origin = button('trace-origin-mode', T('Source', '来源'), function () { detailMode = 'origin'; renderDetail(true); });
    [overviewTab, preview, source, origin].forEach(function (tab) { tab.setAttribute('role', 'tab'); modes.appendChild(tab); }); detail.appendChild(modes);
    var detailMode = 'preview', warning = el('p', 'trace-truncated'); detail.appendChild(warning);
    var content = el('div', 'trace-detail-content'); content.id = detail.id + '-content'; content.setAttribute('role', 'tabpanel'); [overviewTab, preview, source, origin].forEach(function (tab, i) { tab.id = detail.id + '-tab-' + i; tab.setAttribute('aria-controls', content.id); }); detail.appendChild(content); body.appendChild(detail); body.appendChild(latest); root.appendChild(body);
    var footer = el('details', 'trace-limits'), summary = el('summary', '', T('About these records', '记录范围')); footer.appendChild(summary);
    footer.appendChild(el('p', '', T('Conversation text, recorded context and tool input. Tool output, call timing and token usage are not recorded. Turn numbers refer to loaded history.', '展示会话正文、已记录的上下文与调用参数，不含工具输出、调用耗时或 Token 用量。轮次按当前载入的历史编号。'))); root.appendChild(footer);

    function el(tag, name, text) { var node = document.createElement(tag); if (name) node.className = name; if (text !== undefined) node.textContent = text; return node; }
    function option(value, label) { var node = el('option', '', label); node.value = value; return node; }
    function button(name, label, action) { var node = el('button', name, label); node.type = 'button'; node.title = label; node.setAttribute('aria-label', label); node.onclick = action; return node; }
    function label(event) { if (event.kind === 'context') return model.contextLabel(event, T('en', 'zh')); return event.title || (event.kind === 'tool' ? T('Tool call', '工具调用') : event.kind === 'user' ? T('User message', '用户消息') : event.agent || T('Agent reply', 'Agent 回复')); }
    function turnLabel(event) { return T('Turn ', '第 ') + event.turn + T('', ' 轮'); }
    function compact() { return (root.clientWidth || context.width || window.innerWidth) < 700; }
    function updateStatus() {
      var historical = session.value && session.value !== 'all' && current.sessions.length && session.value !== current.sessions[current.sessions.length - 1].key;
      var presentation = options.status ? options.status(context) : null;
      status.hidden = !historical && !presentation;
      status.textContent = historical ? T('History', '历史会话') : presentation ? presentation.text : '';
      status.style.setProperty('--trace-state', historical ? 'var(--fg3)' : presentation ? presentation.color : 'var(--fg3)');
      latest.hidden = historical || !current.events.length || follow || compact() && inspectorOpen;
      latest.style.left = Math.max(12, list.offsetLeft + list.offsetWidth - 60) + 'px';
    }
    function syncCompact() { var narrow = compact(), covered = narrow && inspectorOpen; root.classList.toggle('trace-compact', narrow); list.hidden = covered; list.inert = covered; updateStatus(); }
    function panel(open) {
      inspectorOpen = !!open; detail.hidden = !inspectorOpen; root.classList.toggle('inspector-closed', !inspectorOpen); root.classList.toggle('inspector-open', inspectorOpen);
      toggle.textContent = inspectorOpen ? T('Hide details', '收起详情') : T('Show details', '查看详情'); toggle.title = toggle.textContent; toggle.setAttribute('aria-label', toggle.title); toggle.setAttribute('aria-expanded', String(inspectorOpen));
      syncCompact();
    }
    function selectionHeld() {
      var value = window.getSelection && window.getSelection();
      return !!(value && !value.isCollapsed && content.contains(value.anchorNode)) || actions.contains(document.activeElement) || modes.contains(document.activeElement) || content.contains(document.activeElement);
    }
    function renderDetail(force) {
      var event = filtered.find(function (item) { return item.key === selected; });
      if (!event) { detail.hidden = true; toggle.disabled = true; return; }
      toggle.disabled = false; detail.hidden = !inspectorOpen;
      var signature = JSON.stringify([event, detailMode]);
      if (!force && signature === lastDetail) return;
      if (!force && event.key === lastDetailKey && selectionHeld()) {
        if (!deferred) deferred = setTimeout(function flush() { deferred = null; if (!root.isConnected) return; if (selectionHeld()) deferred = setTimeout(flush, 250); else renderDetail(false); }, 250);
        return;
      }
      lastDetail = signature; lastDetailKey = event.key;
      var scroll = detail.scrollTop;
      title.textContent = label(event);
      metadata.textContent = [event.agent, turnLabel(event), event.session, event.time && !isNaN(new Date(event.time).getTime()) ? T('Turn started ', '本轮开始于 ') + options.formatTime(new Date(event.time)) : ''].filter(Boolean).join(' · ');
      path.textContent = event.step && event.step.workdir ? T('Working directory: ', '工作目录：') + event.step.workdir : ''; path.hidden = !path.textContent;
      modes.hidden = false; [overviewTab, preview, source, origin].forEach(function (tab, i) { var active = detailMode === ['overview', 'preview', 'raw', 'origin'][i]; tab.setAttribute('aria-selected', String(active)); if (active) content.setAttribute('aria-labelledby', tab.id); tab.tabIndex = active ? 0 : -1; });
      warning.textContent = (event.context && event.context.truncated || event.step && event.step.truncated) ? T('Only the recorded portion is available.', '内容较长，仅展示已记录的部分。') : event.step && !event.step.input ? T('Only a call summary was recorded.', '此调用仅记录了摘要。') : ''; warning.hidden = !warning.textContent;
      content.replaceChildren();
      if (detailMode === 'overview') {
        var facts = el('dl', 'trace-facts'); model.overviewFacts(event, T('en', 'zh')).forEach(function (fact) { facts.appendChild(el('dt', '', fact.label)); facts.appendChild(el('dd', '', fact.value)); }); content.appendChild(facts); content.appendChild(el('p', 'trace-excerpt', event.body.replace(/\s+/g, ' ').slice(0, 240)));
      } else if (detailMode === 'origin') {
        content.appendChild(el('p', 'trace-origin-note', T('Source information from the conversation record.', '会话记录中的来源信息。'))); var originText = el('pre', 'trace-origin', JSON.stringify(model.sourceInfo(event), null, 2)); originText.tabIndex = 0; content.appendChild(originText);
      } else if (event.kind !== 'tool' && !(event.kind === 'context' && event.title === 'Environment') && detailMode === 'preview') { var prose = el('div', 'trace-prose'); prose.appendChild(options.markdown(model.previewText(event), {session_id: event.session, home_id: context.home_id || ''})); content.appendChild(prose); }
      else { var code = el('pre', 'trace-source' + (event.step && !event.step.input ? ' trace-summary' : ''), (detailMode === 'preview' ? model.previewText(event) : event.body) || T('No detail recorded.', '未记录详情。')); code.tabIndex = 0; content.appendChild(code); }
      actions.replaceChildren(options.actions(detailMode === 'origin' ? JSON.stringify(model.sourceInfo(event), null, 2) : event.body || label(event))); detail.scrollTop = scroll;
    }
    function inspect(event, reveal) {
      if (!event) return;
      selected = event.key;
      rowNodes.forEach(function (row, key) { row.classList.toggle('on', key === selected); row.setAttribute('aria-pressed', String(key === selected)); });
      if (reveal) { follow = false; inspecting = true; panel(true); }
      var position = filtered.findIndex(function (item) { return item.key === selected; });
      scrub.value = String(Math.max(0, position)); scrub.setAttribute('aria-valuetext', (position + 1) + ' / ' + filtered.length + ' · ' + turnLabel(event) + ' · ' + label(event));
      updateStatus(); renderDetail(!!reveal);
      if (reveal && compact()) detail.focus({preventScroll: true});
    }
    function rowFor(event) {
      var row = rowNodes.get(event.key);
      if (!row) {
        row = el('button', 'trace-event ' + event.kind); row.type = 'button'; row.dataset.key = event.key;
        var identity = el('span', 'trace-kind'); identity.appendChild(el('span', '', event.kind === 'tool' ? T('Tool', '工具') : event.kind === 'user' ? T('Message', '消息') : event.kind === 'context' ? T('Context', '上下文') : T('Agent', 'Agent'))); row.appendChild(identity);
        var text = el('span', 'trace-row-text'); text.appendChild(el('span', 'trace-row-title')); text.appendChild(el('span', 'trace-preview')); row.appendChild(text);
        rowNodes.set(event.key, row);
      }
      row.querySelector('.trace-row-title').textContent = label(event);
      row.querySelector('.trace-preview').textContent = event.body.replace(/\s+/g, ' ');
      row.setAttribute('aria-controls', detail.id); row.setAttribute('aria-pressed', String(event.key === selected)); row.classList.toggle('on', event.key === selected);
      row.onclick = function () { inspect(event, true); };
      return row;
    }
    function disclosureIcon(open) {
      var node = el('span', 'trace-chevron'); node.setAttribute('aria-hidden', 'true');
      node.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="m9 18 6-6-6-6"/></svg>';
      node.classList.toggle('open', !!open); return node;
    }
    function searching() { return !!search.value.trim() || kind.value !== 'all' || session.value !== 'all'; }
    function projection() { presented = model.timelineRows(model.groupTurns(filtered), expandedTurns, expandedCalls, searching(), defaults); }
    function positionFor(event) {
      if (!event) return 0;
      var index = presented.findIndex(function (row) { return row.type === 'event' && row.event.key === event.key; });
      if (index < 0) index = presented.findIndex(function (row) { return row.type === 'calls' && row.events.some(function (item) { return item.key === event.key; }); });
      if (index < 0) index = presented.findIndex(function (row) { return row.type === 'turn' && row.group.key === event.turnKey; });
      return Math.max(0, index);
    }
    function selectedRow() {
      var row = rowNodes.get(selected);
      if (row && row.isConnected) return row;
      var parent = presented[positionFor(filtered.find(function (event) { return event.key === selected; }))];
      return parent && Array.from(list.children).find(function (node) { return node.dataset.rowKey === parent.key; });
    }
    function reveal(event) {
      if (!event) return;
      expandedTurns[event.turnKey] = true;
      var groups = model.groupTurns(filtered), group = groups.find(function (value) { return value.key === event.turnKey; });
      if (group) {
        var start = null;
        group.events.forEach(function (item) { if (item.kind !== 'tool') start = null; else { if (!start) start = item; if (item.key === event.key) expandedCalls['calls:' + start.key] = true; } });
      }
      projection();
    }
    function foldAll(level) {
      if (searching()) return;
      var open = level === 'turns' ? model.groupTurns(filtered).every(function (group) { return expandedTurns[group.key] ?? defaults.turns; }) : model.timelineRows(model.groupTurns(filtered), {}, expandedCalls, false, {turns: true, calls: defaults.calls}).filter(function (row) { return row.type === 'calls'; }).every(function (row) { return row.open; });
      defaults[level] = !open;
      if (level === 'turns') expandedTurns = {}; else expandedCalls = {};
      follow = false; disclosureHold = true; draw(false);
    }
    function groupNode(row) {
      var turn = row.type === 'turn', group = turn ? row.group : null;
      var text = turn ? [group.agent, turnLabel(group), current.sessions.length > 1 ? group.session : ''].filter(Boolean).join(' · ') : row.events.length + T(row.events.length === 1 ? ' tool call' : ' tool calls', ' 个工具调用');
      var node = button(turn ? 'trace-turn' : 'trace-calls', text, function () {
        follow = false; disclosureHold = true;
        if (turn) expandedTurns[group.key] = !row.open; else expandedCalls[row.key] = !row.open;
        draw(false);
        var restored = Array.from(list.children).find(function (value) { return value.dataset.rowKey === row.key; });
        if (restored) restored.focus({preventScroll: true});
      });
      node.replaceChildren(disclosureIcon(row.open), el('span', 'trace-group-label', text));
      node.disabled = searching(); node.setAttribute('aria-expanded', String(row.open)); node.dataset.rowKey = row.key;
      node.dataset.turnKey = turn ? group.key : row.events[0].turnKey;
      if (turn && group.events[0].boundary) node.classList.add('trace-session-break');
      if (turn && !row.open) {
        var summary = group.prompt ? group.prompt.body : group.events[0].body;
        node.appendChild(el('span', 'trace-group-summary', summary.replace(/\s+/g, ' ')));
        node.appendChild(el('span', 'trace-group-count', group.calls + T(' calls', ' 次调用') + ' · ' + group.events.length + T(' records', ' 条记录')));
      } else if (!turn) node.appendChild(el('span', 'trace-call-names', [...new Set(row.events.map(function (event) { return event.title; }))].join(', ')));
      return node;
    }
    function pager(text, action) { var node = button('trace-page', text, action); list.appendChild(node); }
    function renderList(scrollToSelected) {
      var active = document.activeElement, focusedKey = active && active.dataset && active.dataset.rowKey;
      var top = list.scrollTop, anchor = null, listTop = list.getBoundingClientRect().top;
      if (!scrollToSelected) {
        var visible = Array.from(list.querySelectorAll('[data-row-key]')).find(function (row) { return row.getBoundingClientRect().bottom > listTop; });
        if (visible) {
          anchor = {key: visible.dataset.rowKey, turn: visible.dataset.turnKey, offset: visible.getBoundingClientRect().top - listTop};
          var anchorIndex = presented.findIndex(function (row) { return row.key === anchor.key; });
          if (anchorIndex < 0) anchorIndex = presented.findIndex(function (row) { return row.key === 'turn:' + anchor.turn; });
          if (anchorIndex >= 0) page = Math.floor(anchorIndex / pageSize);
        }
      }
      var from = page * pageSize, until = Math.min(presented.length, from + pageSize);
      list.replaceChildren();
      if (!filtered.length) {
        list.appendChild(el('p', 'chat-empty', kind.value === 'context' && !current.events.some(function (event) { return event.kind === 'context'; }) ? T('No readable context was recorded in this conversation log.', '此会话记录未包含可读取的上下文。') : current.events.length ? T('No matching records.', '没有匹配的记录。') : T('No conversation records yet.', '暂无会话记录。')));
        if (current.events.length) pager(T('Clear filters', '清除筛选'), function () { search.value = ''; kind.value = session.value = 'all'; changedFilter(); });
      }
      if (from > 0) pager(T('Previous ', '查看前 ') + pageSize + T(' records', ' 条'), function () { page--; renderList(true); list.scrollTop = 0; });
      var liveKeys = new Set();
      presented.slice(from, until).forEach(function (row) {
        var node;
        if (row.type === 'event') { node = rowFor(row.event); node.dataset.rowKey = row.key; node.dataset.turnKey = row.event.turnKey; liveKeys.add(row.key); }
        else node = groupNode(row);
        list.appendChild(node);
      });
      if (until < presented.length) pager(T('Next ', '查看后 ') + pageSize + T(' records', ' 条'), function () { page++; renderList(true); list.scrollTop = 0; });
      function find(key) { return Array.from(list.children).find(function (node) { return node.dataset.rowKey === key; }); }
      list.scrollTop = top;
      if (anchor) {
        var anchored = find(anchor.key) || find('turn:' + anchor.turn);
        if (anchored) list.scrollTop += anchored.getBoundingClientRect().top - list.getBoundingClientRect().top - anchor.offset;
      }
      var focused = focusedKey && find(focusedKey); if (focused) focused.focus({preventScroll: true});
      if (scrollToSelected) {
        var target = selectedRow();
        if (target && target.isConnected) target.scrollIntoView({block: 'nearest'});
      }
      rowNodes.forEach(function (row, key) { if (!liveKeys.has(key)) rowNodes.delete(key); });
    }
    function draw(scrollToSelected) {
      filtered = model.filter(current.events, search.value, kind.value, session.value);
      if (follow || !filtered.some(function (event) { return event.key === selected; })) selected = filtered.length ? filtered[filtered.length - 1].key : '';
      projection();
      var position = filtered.findIndex(function (event) { return event.key === selected; });
      if (follow || scrollToSelected) page = Math.max(0, Math.floor(positionFor(filtered[position]) / pageSize));
      page = Math.min(page, Math.max(0, Math.floor((presented.length - 1) / pageSize)));
      count.textContent = (filtered.length === current.events.length ? current.events.length : filtered.length + ' / ' + current.events.length) + T(' records', ' 条');
      count.title = current.turns + T(' loaded turns', ' 轮已载入会话');
      var turnsOpen = model.groupTurns(filtered).every(function (group) { return expandedTurns[group.key] ?? defaults.turns; });
      var callsOpen = model.timelineRows(model.groupTurns(filtered), {}, expandedCalls, false, {turns: true, calls: defaults.calls}).filter(function (row) { return row.type === 'calls'; }).every(function (row) { return row.open; });
      [[foldTurns, turnsOpen, T('Turns', '轮次'), 'turns'], [foldCalls, callsOpen, T('Calls', '调用'), 'calls']].forEach(function (value) {
        var control = value[0], open = value[1]; control.replaceChildren(disclosureIcon(open), el('span', '', value[2]));
        control.title = value[3] === 'turns' ? (open ? T('Collapse all turns', '收起所有轮次') : T('Expand all turns', '展开所有轮次')) : (open ? T('Collapse all calls', '收起所有调用') : T('Expand all calls', '展开所有调用'));
        control.setAttribute('aria-label', control.title); control.setAttribute('aria-expanded', String(open));
        control.disabled = searching() || !filtered.length || value[3] === 'calls' && !filtered.some(function (event) { return event.kind === 'tool'; });
      });
      updateStatus(); map.replaceChildren(); model.overview(filtered, 120).forEach(function (bucket) { map.appendChild(el('span', 'trace-mark ' + bucket.kind)); });
      scrub.max = String(Math.max(0, filtered.length - 1)); scrub.disabled = !filtered.length; overview.hidden = filtered.length < 2;
      renderList(scrollToSelected); inspect(filtered[position], false);
      if (!filtered.length) { panel(false); detail.hidden = true; toggle.disabled = true; }
    }
    function changedFilter() { follow = false; inspecting = false; selected = ''; page = 0; lastDetail = ''; if (compact()) panel(false); draw(true); }
    search.oninput = kind.onchange = session.onchange = changedFilter;
    function stopFollowing() { follow = false; updateStatus(); }
    list.addEventListener('wheel', function () { disclosureHold = false; stopFollowing(); }, {passive: true});
    list.addEventListener('touchstart', function () { disclosureHold = false; stopFollowing(); }, {passive: true});
    list.addEventListener('scroll', function () {
      var tail = page === Math.max(0, Math.floor((presented.length - 1) / pageSize)) && list.scrollHeight - list.scrollTop - list.clientHeight < 60;
      if (!disclosureHold && !inspecting && !selectionHeld() && !search.value && kind.value === 'all' && session.value === 'all') follow = tail;
      updateStatus();
    }, {passive: true});
    list.onkeydown = function (event) {
      if (!event.target.matches('.trace-event') || !['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return;
      var index = filtered.findIndex(function (item) { return item.key === event.target.dataset.key; });
      index = event.key === 'Home' ? 0 : event.key === 'End' ? filtered.length - 1 : Math.max(0, Math.min(filtered.length - 1, index + (event.key === 'ArrowUp' ? -1 : 1)));
      event.preventDefault(); stopFollowing(); selected = filtered[index].key; reveal(filtered[index]); page = Math.floor(positionFor(filtered[index]) / pageSize); renderList(true); inspect(filtered[index], false); rowNodes.get(selected).focus({preventScroll: true});
    };
    scrub.oninput = function () { follow = false; var event = filtered[Number(scrub.value)]; if (!event) return; selected = event.key; reveal(event); page = Math.floor(positionFor(event) / pageSize); renderList(true); inspect(event, false); var row = rowNodes.get(event.key); if (row) row.scrollIntoView({block: 'nearest'}); };
    modes.onkeydown = function (event) {
      if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
      var tabs = [overviewTab, preview, source, origin], index = tabs.indexOf(document.activeElement);
      if (index < 0) return;
      index = event.key === 'Home' ? 0 : event.key === 'End' ? 3 : (index + (event.key === 'ArrowLeft' ? 3 : 1)) % 4;
      event.preventDefault(); tabs[index].click(); tabs[index].focus({preventScroll: true});
    };
    detail.onkeydown = function (event) { if (event.key === 'Escape') { event.stopPropagation(); close.click(); } };
    var resize = new ResizeObserver(syncCompact); resize.observe(root);
    return {el: root, update: function (turns, nextContext) {
      context = nextContext || {};
      var scope = JSON.stringify([context.pane_id, context.session_id, context.home_id, context.agent]);
      if (lastInput === turns && lastScope === scope) { updateStatus(); return; }
      if (lastScope && lastScope !== scope) { initialized = false; selected = ''; page = 0; expandedTurns = {}; expandedCalls = {}; defaults = {turns: true, calls: true}; disclosureHold = false; search.value = ''; kind.value = 'all'; session.value = 'all'; lastDetail = ''; lastDetailKey = ''; }
      lastInput = turns; lastScope = scope;
      current = model.build(turns, context);
      var priorSession = session.value || 'all', sessionSignature = JSON.stringify(current.sessions.map(function (entry) { return [entry.key, entry.agent, entry.session]; }));
      if (session.dataset.signature !== sessionSignature) {
        session.dataset.signature = sessionSignature; session.replaceChildren(option('all', T('All sessions', '全部会话')));
        current.sessions.forEach(function (entry) { session.appendChild(option(entry.key, [entry.agent, entry.session].filter(Boolean).join(' · '))); });
        session.value = current.sessions.some(function (entry) { return entry.key === priorSession; }) ? priorSession : 'all';
      }
      session.hidden = current.sessions.length < 2;
      if (!initialized) { initialized = true; follow = true; inspecting = false; panel(!compact()); }
      var present = new Set(current.events.map(function (event) { return event.key; })); rowNodes.forEach(function (row, key) { if (!present.has(key)) rowNodes.delete(key); });
      draw(follow || !selected);
    }, dispose: function () { clearTimeout(deferred); resize.disconnect(); rowNodes.clear(); }};
  }};
})();
