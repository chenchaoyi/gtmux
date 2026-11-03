/* One loaded-conversation navigator for focused chat and workbench tiles. */
(function () {
  'use strict';
  window.GtmuxConversationMap = {
    mount: function (main, scroll, turns, T, markdownPreview, options) {
      if (!window.GtmuxTrace || !window.GtmuxTrace.conversationOutline)
        return function () {};
      options = options || {};
      var outline = window.GtmuxTrace.conversationOutline(
          turns,
          T('en', 'zh'),
          options.context
        ),
        nodes = Array.from(scroll.querySelectorAll('.cturn'));
      if (outline.length < 8) return function () {};
      var nav = document.createElement('div');
      nav.className = 'conversation-map';
      var track = document.createElement('div');
      track.className = 'conversation-map-track';
      track.tabIndex = 0;
      track.setAttribute('role', 'slider');
      track.setAttribute('aria-orientation', 'vertical');
      track.setAttribute(
        'aria-label',
        T('Conversation overview — loaded turns', '对话总览 — 已加载轮次')
      );
      track.setAttribute('aria-valuemin', '1');
      track.setAttribute('aria-valuemax', String(outline.length));
      var tip = document.createElement('div');
      tip.className = 'conversation-map-preview';
      tip.hidden = true;
      var meta = document.createElement('div'),
        prompt = document.createElement('div'),
        summary = document.createElement('div'),
        jump = document.createElement('button');
      meta.className = 'conversation-map-meta';
      prompt.className = 'conversation-map-prompt';
      summary.className = 'conversation-map-summary';
      jump.type = 'button';
      jump.textContent = T('Go to turn', '跳到这一轮');
      var heading = document.createElement('div'),
        bookmark = document.createElement('button'),
        feedback = document.createElement('span');
      heading.className = 'conversation-map-heading';
      bookmark.type = 'button';
      bookmark.className = 'conversation-map-bookmark';
      bookmark.innerHTML = options.bookmarkIcon || '';
      feedback.className = 'conversation-map-feedback';
      feedback.setAttribute('role', 'status');
      heading.append(meta, bookmark);
      tip.append(heading, prompt, summary, jump, feedback);
      nav.append(track, tip);
      main.appendChild(nav);
      main.classList.add('has-conversation-map');
      var selected = 0,
        current = 0,
        touch = false,
        keyboardMode = false,
        hideTimer = 0,
        frame = 0,
        positions = [],
        marks = [],
        previewIndex = -1;
      var saved = new Set(),
        storageKey = options.storageKey,
        volatile = false;
      function readBookmarks() {
        try {
          var raw = localStorage.getItem(storageKey);
          if (!raw) {
            saved = new Set();
          } else if (raw.length <= 262144) {
            var values = JSON.parse(raw);
            if (Array.isArray(values) && values.length <= 2000)
              saved = new Set(
                values.filter(function (v) {
                  return typeof v === 'string' && v.length < 200;
                })
              );
          }
        } catch (e) {
          volatile = true;
        }
      }
      if (storageKey) readBookmarks();
      function marked(item) {
        return saved.has(item.bookmarkKey);
      }
      var samples = Math.min(outline.length, 120);
      for (var i = 0; i < samples; i++) {
        var mark = document.createElement('i');
        mark.setAttribute('aria-hidden', 'true');
        mark.dataset.index = String(
          Math.round((i * (outline.length - 1)) / (samples - 1))
        );
        mark.style.top = (i * 100) / (samples - 1) + '%';
        track.appendChild(mark);
        marks.push(mark);
      }
      function name(item) {
        return T('Turn ', '第 ') + item.turn + T('', ' 轮');
      }
      function syncMarks() {
        marks
          .filter(function (mark) {
            return mark.dataset.extra;
          })
          .forEach(function (mark) {
            mark.remove();
          });
        marks = marks.filter(function (mark) {
          return !mark.dataset.extra;
        });
        var represented = new Set(
          marks.map(function (mark) {
            return Number(mark.dataset.index);
          })
        );
        outline.forEach(function (item, index) {
          if (marked(item) && !represented.has(index)) {
            var mark = document.createElement('i');
            mark.dataset.index = String(index);
            mark.dataset.extra = 'true';
            mark.setAttribute('aria-hidden', 'true');
            mark.style.top = (index * 100) / (outline.length - 1) + '%';
            track.appendChild(mark);
            marks.push(mark);
          }
        });
      }
      function bookmarkState() {
        var active = marked(outline[selected]);
        bookmark.classList.toggle('marked', active);
        bookmark.setAttribute('aria-pressed', String(active));
        bookmark.title = active
          ? T('Remove bookmark', '取消书签')
          : T('Bookmark this turn', '为这一轮加书签');
        bookmark.setAttribute('aria-label', bookmark.title);
        bookmark.disabled = !storageKey || !outline[selected].bookmarkKey;
        if (bookmark.disabled) {
          bookmark.title = T(
            'This server does not provide a stable turn identity',
            '此记录缺少稳定的轮次标识'
          );
          bookmark.setAttribute('aria-label', bookmark.title);
        }
      }
      bookmark.onclick = function () {
        var key = outline[selected].bookmarkKey;
        if (!storageKey || !key) return;
        if (!saved.has(key) && saved.size >= 2000) {
          feedback.textContent = T(
            'Bookmark limit reached. Remove one to add another.',
            '书签已达上限，请先取消一个书签。'
          );
          return;
        }
        if (saved.has(key)) saved.delete(key);
        else saved.add(key);
        try {
          localStorage.setItem(storageKey, JSON.stringify(Array.from(saved)));
          volatile = false;
        } catch (e) {
          volatile = true;
        }
        feedback.textContent = volatile
          ? T(
              'Saved for this page only; browser storage is unavailable.',
              '浏览器存储不可用，书签仅保留到关闭页面。'
            )
          : '';
        syncMarks();
        paint();
        bookmarkState();
        window.dispatchEvent(
          new CustomEvent('gtmux-bookmarks', { detail: storageKey })
        );
      };
      function bookmarksChanged(e) {
        if ((e.type === 'storage' ? e.key : e.detail) !== storageKey) return;
        readBookmarks();
        syncMarks();
        paint();
        bookmarkState();
      }
      window.addEventListener('storage', bookmarksChanged);
      window.addEventListener('gtmux-bookmarks', bookmarksChanged);
      function paint() {
        marks.forEach(function (mark) {
          var index = Number(mark.dataset.index),
            distance = Math.abs(index - selected),
            zoom = tip.hidden
              ? 0
              : Math.max(0, 1 - distance / Math.max(3, outline.length / 30));
          mark.style.setProperty('--map-lens', String(zoom));
          mark.classList.toggle('marked', marked(outline[index]));
          mark.classList.toggle(
            'current',
            Math.abs(index - current) <
              Math.max(1, outline.length / samples / 2)
          );
        });
        track.setAttribute('aria-valuenow', String(selected + 1));
        track.setAttribute(
          'aria-valuetext',
          name(outline[selected]) + ' · ' + outline[selected].prompt
        );
      }
      function preview(index) {
        clearTimeout(hideTimer);
        selected = Math.max(0, Math.min(outline.length - 1, index));
        var item = outline[selected];
        meta.textContent = [item.agent, name(item)].filter(Boolean).join(' · ');
        if (previewIndex !== selected) {
          previewIndex = selected;
          if (markdownPreview) {
            prompt.replaceChildren(
              markdownPreview(
                item.promptMarkdown ||
                  item.prompt ||
                  T('No message text', '无消息正文')
              )
            );
            summary.replaceChildren(
              markdownPreview(item.summaryMarkdown || item.summary)
            );
          } else {
            prompt.textContent =
              item.prompt || T('No message text', '无消息正文');
            summary.textContent = item.summary;
          }
        }
        bookmarkState();
        tip.hidden = false;
        tip.style.top =
          Math.max(
            8,
            Math.min(
              main.clientHeight - tip.offsetHeight - 8,
              track.offsetTop +
                (selected / Math.max(1, outline.length - 1)) *
                  track.clientHeight -
                65
            )
          ) + 'px';
        paint();
      }
      function hide() {
        tip.hidden = true;
        selected = current;
        paint();
      }
      function go() {
        var node = nodes[outline[selected].sourceIndex];
        if (!node) return;
        scroll.scrollTop =
          scroll.scrollTop +
          node.getBoundingClientRect().top -
          scroll.getBoundingClientRect().top -
          14;
        current = selected;
        if (keyboardMode) {
          node.tabIndex = -1;
          node.focus({ preventScroll: true });
        }
        node.classList.add('conversation-map-target');
        setTimeout(function () {
          node.classList.remove('conversation-map-target');
        }, 1200);
        hide();
      }
      function measure() {
        tip.style.width =
          Math.min(320, Math.max(120, main.clientWidth - 48)) + 'px';
        positions = outline.map(function (item) {
          var node = nodes[item.sourceIndex];
          return node
            ? node.getBoundingClientRect().top -
                scroll.getBoundingClientRect().top +
                scroll.scrollTop
            : 0;
        });
        update();
      }
      function update() {
        cancelAnimationFrame(frame);
        frame = requestAnimationFrame(function () {
          if (!main.isConnected) return;
          var target = scroll.scrollTop + scroll.clientHeight * 0.25,
            lo = 0,
            hi = positions.length;
          while (lo < hi) {
            var mid = (lo + hi) >> 1;
            if (positions[mid] <= target) lo = mid + 1;
            else hi = mid;
          }
          current = Math.max(0, lo - 1);
          if (tip.hidden) selected = current;
          paint();
        });
      }
      track.onpointermove = function (e) {
        if (keyboardMode && !e.movementX && !e.movementY) return;
        keyboardMode = false;
        if (e.pointerType !== 'touch')
          preview(
            Math.round(
              ((e.clientY - track.getBoundingClientRect().top) /
                track.clientHeight) *
                (outline.length - 1)
            )
          );
      };
      track.onpointerdown = function (e) {
        touch = e.pointerType === 'touch';
        if (touch)
          preview(
            Math.round(
              ((e.clientY - track.getBoundingClientRect().top) /
                track.clientHeight) *
                (outline.length - 1)
            )
          );
      };
      track.onclick = function () {
        if (!touch) go();
      };
      jump.onclick = go;
      track.onfocus = function () {
        if (tip.hidden) preview(current);
      };
      track.onkeydown = function (e) {
        keyboardMode = true;
        var value = selected;
        if (e.key === 'ArrowUp') value--;
        else if (e.key === 'ArrowDown') value++;
        else if (e.key === 'Home') value = 0;
        else if (e.key === 'End') value = outline.length - 1;
        else if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          go();
          return;
        } else if (e.key === 'Escape') {
          hide();
          return;
        } else return;
        e.preventDefault();
        preview(value);
      };
      nav.onkeydown = function (e) {
        if (e.key === 'Escape') {
          e.stopPropagation();
          track.focus({ preventScroll: true });
          hide();
        }
      };
      nav.onpointerleave = function (e) {
        if (touch || e.pointerType === 'touch' || nav.contains(e.relatedTarget))
          return;
        hideTimer = setTimeout(hide, 200);
      };
      nav.onpointerenter = function () {
        clearTimeout(hideTimer);
      };
      nav.onfocusout = function (e) {
        if (!nav.contains(e.relatedTarget)) hide();
      };
      function outside(e) {
        if (!nav.contains(e.target)) hide();
      }
      document.addEventListener('pointerdown', outside);
      scroll.addEventListener('scroll', update, { passive: true });
      var resize =
        typeof ResizeObserver === 'undefined'
          ? null
          : new ResizeObserver(measure);
      if (resize) {
        resize.observe(scroll);
        if (scroll.firstElementChild) resize.observe(scroll.firstElementChild);
      }
      syncMarks();
      measure();
      var previous = options.state;
      if (previous) {
        var restored = outline.findIndex(function (item) {
          return (item.bookmarkKey || item.key) === previous.key;
        });
        if (restored >= 0) {
          keyboardMode = previous.keyboard;
          touch = previous.touch;
          if (previous.open) preview(restored);
          if (previous.focus === 'track') track.focus({ preventScroll: true });
          else if (previous.focus === 'bookmark')
            bookmark.focus({ preventScroll: true });
          else if (previous.focus === 'jump')
            jump.focus({ preventScroll: true });
        }
      }
      var dispose = function () {
        window.removeEventListener('storage', bookmarksChanged);
        window.removeEventListener('gtmux-bookmarks', bookmarksChanged);
        clearTimeout(hideTimer);
        cancelAnimationFrame(frame);
        if (resize) resize.disconnect();
        document.removeEventListener('pointerdown', outside);
        scroll.removeEventListener('scroll', update);
        nav.remove();
        main.classList.remove('has-conversation-map');
      };
      dispose.snapshot = function () {
        return {
          key: outline[selected].bookmarkKey || outline[selected].key,
          open: !tip.hidden,
          keyboard: keyboardMode,
          touch: touch,
          focus:
            document.activeElement === track
              ? 'track'
              : document.activeElement === bookmark
              ? 'bookmark'
              : document.activeElement === jump
              ? 'jump'
              : '',
        };
      };
      return dispose;
    },
  };
})();
