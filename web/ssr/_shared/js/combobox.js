/* Выпадающий список с поиском (D6; эталон — enhanceSelect в prototype/index.html).
   Любой <select class="form-select"> (кроме [multiple] и [data-native]) автоматически
   превращается в комбобокс — и при загрузке, и при появлении в DOM (MutationObserver).
   Нативный select остаётся в DOM скрытым: value, name в форме и событие change
   работают как раньше. Доп. текст для поиска — option[data-search], подпись под
   пунктом — option[data-hint]. Ручной вызов — KT.enhanceSelect(select).
   select._cbAction — пункт-действие в конце списка ({text, testid, run(query)} либо функция,
   возвращающая такой объект или null): например «+ Новый клиент», когда ничего не подошло.
   KT.autocomplete(input, opts) — выпадающие подсказки под текстовым полем (ниже). */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};
  var SELECTOR = 'select.form-select:not([multiple]):not([data-native]):not([data-cb])';
  var closeOpen = null;
  var uidSeq = 0;

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function highlight(text, q) {
    // toLowerCase может менять длину строки (редкие символы) — тогда без подсветки
    if (!q || text.toLowerCase().length !== text.length) return esc(text);
    var i = text.toLowerCase().indexOf(q);
    if (i < 0) return esc(text);
    return esc(text.slice(0, i)) + '<mark>' + esc(text.slice(i, i + q.length)) + '</mark>' + esc(text.slice(i + q.length));
  }

  function enhanceSelect(sel) {
    if (sel.dataset.cb) return;
    sel.dataset.cb = '1';
    var wrap = document.createElement('div');
    wrap.className = 'cb';
    sel.before(wrap);
    var btn = document.createElement('button');
    btn.type = 'button';
    // размер/состояние (form-select-sm, is-invalid) переносим на кнопку
    btn.className = [].slice.call(sel.classList).concat('cb-btn').join(' ');
    btn.disabled = sel.disabled;
    if (sel.id) btn.setAttribute('aria-labelledby', sel.id);
    btn.setAttribute('aria-haspopup', 'listbox');
    // testid — на видимый контрол (кнопку); у скрытого select он больше не нужен
    if (sel.dataset.testid) { btn.setAttribute('data-testid', sel.dataset.testid); sel.removeAttribute('data-testid'); }
    wrap.append(btn, sel);
    sel.hidden = true;
    // label[for=id] должен фокусировать кнопку, а не скрытый select
    if (sel.id) document.querySelectorAll('label[for="' + CSS.escape(sel.id) + '"]').forEach(function (l) {
      l.addEventListener('click', function (e) { e.preventDefault(); btn.focus(); });
    });

    function sync() {
      var o = sel.selectedOptions[0];
      btn.innerHTML = '<span class="cb-label' + (!o || !o.value ? ' is-placeholder' : '') + '">' + esc(o ? o.textContent : '') + '</span>';
      btn.disabled = sel.disabled;
    }
    sel._cbSync = sync;
    sync();
    sel.addEventListener('change', sync);

    btn.addEventListener('click', function () { open(''); });
    btn.addEventListener('keydown', function (e) {
      if (['ArrowDown', 'Enter', ' '].indexOf(e.key) >= 0) { e.preventDefault(); open(''); }
      else if (e.key.length === 1 && !e.metaKey && !e.ctrlKey && !e.altKey) { e.preventDefault(); open(e.key); }
    });

    function open(initial) {
      if (closeOpen) closeOpen();
      var dd = document.createElement('div');
      dd.className = 'cb-dd';
      dd.setAttribute('data-testid', 'combobox-dropdown');
      var listId = 'cb-list-' + (++uidSeq);
      dd.innerHTML = '<input class="form-control form-control-sm cb-search" placeholder="Поиск…" autocomplete="off" aria-label="Поиск"' +
        ' role="combobox" aria-expanded="true" aria-autocomplete="list" aria-controls="' + listId + '">' +
        '<div class="cb-list" role="listbox" id="' + listId + '"></div>';
      // внутри открытой Bootstrap-модалки — туда же, иначе focus trap модалки не даст ввести текст
      (btn.closest('.modal') || document.body).append(dd);
      var inp = dd.querySelector('input'), list = dd.querySelector('.cb-list');
      var items = [], active = 0;

      var r = btn.getBoundingClientRect();
      var w = Math.max(r.width, 280);
      dd.style.left = Math.max(8, Math.min(r.left, innerWidth - w - 8)) + 'px';
      dd.style.width = Math.min(w, innerWidth - 16) + 'px';
      if (innerHeight - r.bottom < 300 && r.top > innerHeight - r.bottom) dd.style.bottom = (innerHeight - r.top + 4) + 'px';
      else dd.style.top = (r.bottom + 4) + 'px';

      function mark() {
        list.querySelectorAll('.cb-opt').forEach(function (el, i) {
          el.classList.toggle('active', i === active);
          el.setAttribute('aria-selected', i === active ? 'true' : 'false');
        });
        var a = list.querySelector('.cb-opt.active');
        if (a) { a.scrollIntoView({ block: 'nearest' }); inp.setAttribute('aria-activedescendant', a.id); }
        else inp.removeAttribute('aria-activedescendant');
      }
      function optHtml(i, inner, extraClass) {
        return '<div class="cb-opt' + (extraClass || '') + '" role="option" id="' + listId + '-' + i + '" data-i="' + i + '">' + inner + '</div>';
      }
      // actionHtml — пункт-действие select._cbAction в конце списка (входит в навигацию ↑/↓).
      function actionHtml() {
        var a = typeof sel._cbAction === 'function' ? sel._cbAction() : sel._cbAction;
        if (!a) return '';
        var i = items.length;
        items.push({ action: a });
        return '<div class="cb-opt cb-action" role="option" id="' + listId + '-' + i + '" data-i="' + i + '"' +
          (a.testid ? ' data-testid="' + esc(a.testid) + '"' : '') + '>' + esc(a.text) + '</div>';
      }
      function renderFlatItems(opts, q) {
        // Плоский рендер (без optgroup) — общий хвост для локального и серверного поиска.
        items = opts;
        var html = opts.map(function (o, i) {
          return optHtml(i, highlight(o.textContent, q) + (o.dataset.hint ? '<small>' + highlight(o.dataset.hint, q) + '</small>' : ''), o.value === sel.value ? ' sel' : '');
        }).join('');
        items = opts.slice();
        list.innerHTML = (html || '<div class="cb-empty">Ничего не найдено</div>') + actionHtml();
        active = q ? 0 : Math.max(0, items.findIndex(function (o) { return o.value === sel.value; }));
        mark();
      }
      function drawLocal(q) {
        var html = '';
        items = [];
        [].forEach.call(sel.children, function (node) {
          var group = node.tagName === 'OPTGROUP';
          var opts = (group ? [].slice.call(node.children) : [node]).filter(function (o) {
            return !q || (o.textContent + ' ' + (o.dataset.search || '') + ' ' + (o.dataset.hint || '')).toLowerCase().indexOf(q) >= 0;
          });
          if (!opts.length) return;
          if (group) html += '<div class="cb-group">' + esc(node.label) + '</div>';
          opts.forEach(function (o) {
            html += optHtml(items.length, highlight(o.textContent, q) + (o.dataset.hint ? '<small>' + highlight(o.dataset.hint, q) + '</small>' : ''), o.value === sel.value ? ' sel' : '');
            items.push(o);
          });
        });
        list.innerHTML = (html || '<div class="cb-empty">Ничего не найдено</div>') + actionHtml();
        active = q ? 0 : Math.max(0, items.findIndex(function (o) { return o.value === sel.value; }));
        mark();
      }
      // remoteToken — защита от гонки: ответ устаревшего запроса (юзер уже допечатал
      // дальше) игнорируется, если пришёл после более свежего запроса.
      var remoteToken = 0, remoteTimer = null;
      function drawRemote(q) {
        clearTimeout(remoteTimer);
        var token = ++remoteToken;
        list.innerHTML = '<div class="cb-empty">Поиск…</div>';
        remoteTimer = setTimeout(function () {
          sel._cbFetchOptions(q).then(function (results) {
            if (token !== remoteToken) return;
            var opts = results.map(function (r) { return ensureOption(sel, r.value, r.text); });
            renderFlatItems(opts, q);
          }).catch(function () {
            if (token !== remoteToken) return;
            list.innerHTML = '<div class="cb-empty">Ошибка поиска</div>';
          });
        }, 250);
      }
      function draw() {
        var q = inp.value.trim().toLowerCase();
        if (sel._cbFetchOptions && q) { drawRemote(q); return; }
        drawLocal(q);
      }
      function choose(i) {
        var o = items[i];
        if (!o) return;
        if (o.action) { var q = inp.value.trim(); close(); o.action.run(q); return; }
        close();
        if (sel.value !== o.value) { sel.value = o.value; sel.dispatchEvent(new Event('change', { bubbles: true })); }
        if (btn.isConnected) btn.focus();
      }
      function onOutside(e) { if (!dd.contains(e.target) && !btn.contains(e.target)) close(); }
      // openedAt — короткая «защита» сразу после открытия: клик по кнопке, которая была
      // не полностью видна (частично за нижним краем экрана — обычный случай на длинных
      // страницах: список клиентов на сотни строк, доска со множеством карточек), браузер
      // сначала докручивает страницу к кнопке САМ (программный scrollIntoView реального
      // клика или Playwright-действия), и это НАСТОЯЩЕЕ событие `scroll` документа иногда
      // долетает уже ПОСЛЕ open() — без защиты список закрывался тем же кликом, которым
      // был открыт (найдено регрессией SH-020/022: список клиентов вырос настолько, что
      // внедрённый тестом select уходил на тысячи пикселей вниз). Игнорируем scroll-закрытие
      // первые 300мс — реальный «пользователь скроллит мимо» после этого работает как раньше.
      var openedAt = Date.now();
      function onScroll(e) { if (Date.now() - openedAt < 300) return; if (!dd.contains(e.target)) close(); }
      function close() {
        clearTimeout(remoteTimer);
        remoteToken++; // гасит уже летящий remote-запрос — его ответ больше некуда рисовать
        dd.remove();
        document.removeEventListener('mousedown', onOutside, true);
        document.removeEventListener('scroll', onScroll, true);
        removeEventListener('resize', close);
        closeOpen = null;
      }
      closeOpen = close;
      document.addEventListener('mousedown', onOutside, true);
      document.addEventListener('scroll', onScroll, true);
      addEventListener('resize', close);

      list.addEventListener('mousedown', function (e) { var o = e.target.closest('.cb-opt'); if (o) { e.preventDefault(); choose(+o.dataset.i); } });
      list.addEventListener('mousemove', function (e) { var o = e.target.closest('.cb-opt'); if (o && +o.dataset.i !== active) { active = +o.dataset.i; mark(); } });
      inp.addEventListener('input', draw);
      inp.addEventListener('keydown', function (e) {
        if (e.key === 'ArrowDown') { e.preventDefault(); active = Math.min(active + 1, items.length - 1); mark(); }
        else if (e.key === 'ArrowUp') { e.preventDefault(); active = Math.max(active - 1, 0); mark(); }
        else if (e.key === 'Enter') { e.preventDefault(); choose(active); }
        // stopPropagation — Escape закрывает список, а не модалку под ним
        else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(); btn.focus(); }
        else if (e.key === 'Tab') close();
      });
      inp.value = initial;
      draw();
      inp.focus();
    }
  }

  // autocomplete — подсказки под текстовым полем input (WAI-ARIA combobox: role=combobox на
  // поле, role=listbox у списка, aria-activedescendant у активного пункта). opts:
  //   fetch(value) → Promise<[{html, item}]> — подсказки (html — уже экранированная разметка);
  //   onSelect(item) — выбор пункта (мышь, Enter);
  //   minLength(value) → число символов для поиска (по умолчанию длина value), min — порог (2);
  //   delay — debounce, мс (200); testid / optionTestid — data-testid списка и пунктов.
  // Клавиатура: ↓/↑ — по пунктам, Enter — выбрать активный (без активного — обычный Enter
  // поля), Esc — закрыть список (модалку под ним не закрывает). Ответы устаревшего ввода
  // отбрасываются. Возвращает {close}.
  function autocomplete(input, opts) {
    var min = opts.min || 2, delay = opts.delay == null ? 200 : opts.delay;
    var listId = 'ac-list-' + (++uidSeq);
    var dd = null, items = [], active = -1, timer = null, token = 0;
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-autocomplete', 'list');
    input.setAttribute('aria-expanded', 'false');
    input.setAttribute('aria-controls', listId);

    function close() {
      clearTimeout(timer);
      token++;
      if (dd) { dd.remove(); dd = null; }
      items = []; active = -1;
      input.setAttribute('aria-expanded', 'false');
      input.removeAttribute('aria-activedescendant');
      document.removeEventListener('scroll', onScroll, true);
      removeEventListener('resize', close);
    }
    function onScroll(e) { if (dd && !dd.contains(e.target)) close(); }
    function mark() {
      if (!dd) return;
      dd.querySelectorAll('.cb-opt').forEach(function (el, i) {
        el.classList.toggle('active', i === active);
        el.setAttribute('aria-selected', i === active ? 'true' : 'false');
      });
      var a = dd.querySelector('.cb-opt.active');
      if (a) { a.scrollIntoView({ block: 'nearest' }); input.setAttribute('aria-activedescendant', a.id); }
      else input.removeAttribute('aria-activedescendant');
    }
    function render(results) {
      if (!results.length) { close(); return; }
      if (!dd) {
        dd = document.createElement('div');
        dd.className = 'cb-dd ac-dd';
        if (opts.testid) dd.setAttribute('data-testid', opts.testid);
        dd.innerHTML = '<div class="cb-list" role="listbox" id="' + listId + '"></div>';
        (input.closest('.modal') || document.body).append(dd);
        dd.addEventListener('mousedown', function (e) {
          e.preventDefault(); // фокус остаётся в поле
          var o = e.target.closest('.cb-opt');
          if (o) choose(+o.dataset.i);
        });
        dd.addEventListener('mousemove', function (e) {
          var o = e.target.closest('.cb-opt');
          if (o && +o.dataset.i !== active) { active = +o.dataset.i; mark(); }
        });
        document.addEventListener('scroll', onScroll, true);
        addEventListener('resize', close);
      }
      var r = input.getBoundingClientRect();
      var w = Math.max(r.width, 280);
      dd.style.left = Math.max(8, Math.min(r.left, innerWidth - w - 8)) + 'px';
      dd.style.width = Math.min(w, innerWidth - 16) + 'px';
      dd.style.top = (r.bottom + 4) + 'px';
      items = results;
      dd.firstChild.innerHTML = results.map(function (res, i) {
        return '<div class="cb-opt" role="option" id="' + listId + '-' + i + '" data-i="' + i + '"' +
          (opts.optionTestid ? ' data-testid="' + esc(opts.optionTestid) + '"' : '') + '>' + res.html + '</div>';
      }).join('');
      active = -1;
      input.setAttribute('aria-expanded', 'true');
      mark();
    }
    function choose(i) {
      var res = items[i];
      if (!res) return;
      close();
      opts.onSelect(res.item);
    }
    input.addEventListener('input', function () {
      clearTimeout(timer);
      var value = input.value;
      var len = opts.minLength ? opts.minLength(value) : value.trim().length;
      var t = ++token;
      if (len < min) { close(); return; }
      timer = setTimeout(function () {
        opts.fetch(value).then(function (results) {
          if (t !== token || document.activeElement !== input) return;
          render(results || []);
        }).catch(function () { if (t === token) close(); });
      }, delay);
    });
    input.addEventListener('keydown', function (e) {
      if (!dd) return;
      if (e.key === 'ArrowDown') { e.preventDefault(); e.stopImmediatePropagation(); active = Math.min(active + 1, items.length - 1); mark(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); e.stopImmediatePropagation(); active = Math.max(active - 1, 0); mark(); }
      else if (e.key === 'Enter' && active >= 0) { e.preventDefault(); e.stopImmediatePropagation(); choose(active); }
      else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); e.stopImmediatePropagation(); close(); }
      else if (e.key === 'Tab') close();
    });
    input.addEventListener('blur', close);
    return { close: close };
  }
  KT.autocomplete = autocomplete;

  // ensureOption — добавляет <option> в скрытый select, если такого value ещё нет
  // (используется удалённым поиском ниже — найденную по вводу запись нужно сделать
  // настоящей option, иначе sel.value = o.value молча не применится, DOM select
  // игнорирует значения без соответствующего option).
  function ensureOption(sel, value, text) {
    var existing = sel.querySelector('option[value="' + CSS.escape(String(value)) + '"]');
    if (existing) { existing.textContent = text; return existing; }
    var o = document.createElement('option');
    o.value = value;
    o.textContent = text;
    sel.appendChild(o);
    return o;
  }
  KT.comboboxEnsureOption = ensureOption;

  function scan(root) {
    if (root.matches && root.matches(SELECTOR)) enhanceSelect(root);
    if (root.querySelectorAll) root.querySelectorAll(SELECTOR).forEach(enhanceSelect);
  }

  KT.enhanceSelect = enhanceSelect;
  KT.closeCombobox = function () { if (closeOpen) closeOpen(); };

  scan(document);
  new MutationObserver(function (muts) {
    muts.forEach(function (m) { m.addedNodes.forEach(function (n) { if (n.nodeType === 1) scan(n); }); });
  }).observe(document.body, { childList: true, subtree: true });
})();
