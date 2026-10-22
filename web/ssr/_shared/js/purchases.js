/* Закупки по дефициту (docs/features/purchase-orders/draft-spec.md §8.1–§8.3): список,
   «Новый заказ» и карточка поверх JSON API /api/v1/purchase-orders (KT.api). Страница
   выбирается атрибутом data-po-page корневого элемента. Ошибки: поле — текст под полем,
   глобальные (409, сеть) — тост вверху справа (KT.uiToast), подтверждения — модальные окна
   (UI-конвенции DS-073). Время — в поясе пользователя (KT.time). */
(function () {
  'use strict';
  var KT = window.KT || {};
  var API = '/api/v1/purchase-orders';

  var STATUS = {
    draft: { t: 'Черновик', c: 'text-bg-secondary' },
    sent: { t: 'Отправлен', c: 'text-bg-primary' },
    ready: { t: 'Собран', c: 'text-bg-info' },
    picked_up: { t: 'Забрали', c: 'text-bg-success' },
    cancelled: { t: 'Отменён', c: 'text-bg-dark' }
  };
  var REASON = {
    for_orders: { t: 'Не хватает под заказы', c: 'text-bg-danger' },
    below_min: { t: 'Ниже минимума', c: 'text-bg-warning' },
    manual: { t: 'Вручную', c: 'text-bg-light border' }
  };
  // Коды ошибок полей → текст (validators.Message на сервере даёт тот же текст в message).
  var FIELD_TEXT = {
    items: { required: 'Отметьте хотя бы одну строку', too_many: 'Не больше 200 строк в заказе' },
    qty: 'Количество от 0.001 до 99 999.999',
    product_id: 'Запчасть недоступна',
    supplier_id: 'Поставщик недоступен',
    note: 'Не больше 1000 символов',
    reason: 'Причина — от 1 до 500 символов'
  };

  function $(sel, root) { return (root || document).querySelector(sel); }
  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }
  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function dt(iso) { return iso ? KT.time.fmtDateTime(iso) : ''; }
  function toast(msg, isError) { KT.uiToast(msg, isError); }
  function failure(err, fallback) {
    if (err instanceof KT.ApiError) toast(err.message || fallback, true);
    else toast('Нет связи — повторите', true);
  }
  function badge(map, key, testid) {
    var b = el('span', 'badge ' + ((map[key] && map[key].c) || 'text-bg-secondary'), (map[key] && map[key].t) || key);
    if (testid) b.setAttribute('data-testid', testid);
    return b;
  }
  // Количество: «4.000» → «4», «2.500» → «2.5» (на экране; на wire остаётся строка).
  function num(s) {
    if (s == null || s === '') return '';
    var v = String(s);
    return v.indexOf('.') >= 0 ? v.replace(/0+$/, '').replace(/\.$/, '') : v;
  }
  // Деньги: «28400.00» → «28 400,00» (неразрывный пробел, запятая) — как numeric.Amount.RU в PDF.
  function money(s) {
    if (s == null || s === '') return '';
    var m = /^(-?)(\d+)(?:\.(\d+))?$/.exec(String(s));
    if (!m) return String(s);
    var frac = ((m[3] || '') + '00').slice(0, Math.max(2, (m[3] || '').length));
    return m[1] + m[2].replace(/\B(?=(\d{3})+(?!\d))/g, ' ') + ',' + frac;
  }
  function fieldBase(name) { var m = /^items\[(\d+)\]\.(.+)$/.exec(name); return m ? { index: +m[1], name: m[2] } : null; }

  /* Окно с причиной (отмена §8.3 п.5, удаление п.6): кнопка неактивна, пока причина пуста;
     400 reason — текст под полем, окно открыто; иное — закрыть и отдать ошибку. */
  function reasonModal(opts) {
    return new Promise(function (resolve) {
      var root = el('div', 'modal fade');
      root.tabIndex = -1;
      root.setAttribute('data-testid', 'po-reason-dialog');
      root.innerHTML = '<div class="modal-dialog modal-dialog-centered"><div class="modal-content">' +
        '<div class="modal-header"><h2 class="modal-title fs-5"></h2><button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Закрыть"></button></div>' +
        '<div class="modal-body"><p class="po-reason-text"></p><label class="form-label" for="po-reason-input">Причина *</label>' +
        '<textarea id="po-reason-input" class="form-control" rows="3" maxlength="500" data-testid="po-reason-input"></textarea>' +
        '<div class="invalid-feedback d-block" data-testid="po-reason-error"></div></div>' +
        '<div class="modal-footer"><button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal" data-testid="po-reason-cancel">Назад</button>' +
        '<button type="button" class="btn btn-danger" disabled data-testid="po-reason-ok"></button></div></div></div>';
      $('.modal-title', root).textContent = opts.title;
      $('.po-reason-text', root).textContent = opts.text || '';
      var ok = $('[data-testid="po-reason-ok"]', root);
      ok.textContent = opts.ok;
      var ta = $('textarea', root), errEl = $('[data-testid="po-reason-error"]', root);
      ta.addEventListener('input', function () { ok.disabled = ta.value.trim() === ''; errEl.textContent = ''; ta.classList.remove('is-invalid'); });
      document.body.append(root);
      var modal = new bootstrap.Modal(root);
      var outcome = null;
      ok.addEventListener('click', function () {
        ok.disabled = true;
        opts.submit(ta.value).then(function (resp) {
          outcome = { result: resp };
          modal.hide();
        }).catch(function (err) {
          ok.disabled = false;
          if (err instanceof KT.ApiError && err.status === 400) {
            var f = (err.fields || []).filter(function (x) { return x.name === 'reason'; })[0];
            if (f) { errEl.textContent = FIELD_TEXT.reason; ta.classList.add('is-invalid'); return; }
          }
          outcome = { error: err };
          modal.hide();
        });
      });
      root.addEventListener('shown.bs.modal', function () { ta.focus(); });
      root.addEventListener('hidden.bs.modal', function () { modal.dispose(); root.remove(); resolve(outcome); });
      modal.show();
    });
  }

  /* Поиск запчасти «Добавить запчасть» (§8.2 п.1): GET /api/v1/products?q=…, от 2 символов. */
  function productSearch(input, onPick) {
    KT.autocomplete(input, {
      min: 2, delay: 300, testid: 'po-add-list', optionTestid: 'po-add-option',
      fetch: function (q) {
        return KT.api.get('/api/v1/products?q=' + encodeURIComponent(q.trim()) + '&limit=20&offset=0').then(function (r) {
          return (r.items || []).map(function (p) {
            return { html: esc(p.name) + (p.sku ? ' <span class="text-body-secondary small">' + esc(p.sku) + '</span>' : ''), item: p };
          });
        }).catch(function (err) { toast('Поиск недоступен', true); throw err; });
      },
      onSelect: function (p) { input.value = ''; onPick(p); }
    });
  }

  // ======================= §8.1 список: канбан по статусам =======================
  // Колонки — статусы заказа; переходы между ними — только кнопками в карточке (там
  // проверки версии и прав), поэтому перетаскивания нет. Быстрые фильтры, поставщик и
  // поиск применяются на клиенте: доска грузит все заказы страницами по 100.
  var COLUMNS = [
    { s: 'draft', c: '#6c757d' },
    { s: 'sent', c: '#0d6efd' },
    { s: 'ready', c: '#0dcaf0' },
    { s: 'picked_up', c: '#198754' },
    { s: 'cancelled', c: '#343a40' }
  ];
  var VIEWS = [
    { v: '', t: 'Все', test: function () { return true; } },
    { v: 'new', t: 'Новые', hint: 'Черновики — ещё не отправлены поставщику', test: function (o) { return o.status === 'draft'; } },
    { v: 'active', t: 'Выполняются', hint: 'Отправлены или собраны поставщиком', test: function (o) { return o.status === 'sent' || o.status === 'ready'; } },
    { v: 'discrepancy', t: 'С расхождениями', hint: 'Поставщик ответил количеством, отличным от заказанного', cls: 'danger',
      test: function (o) { return !!o.has_discrepancy; } },
    { v: 'changed', t: 'Изменены', hint: 'Изменены после отправки — поставщик ещё не ответил заново', cls: 'warning',
      test: function (o) { return !!o.is_changed; } },
    { v: 'done', t: 'Завершены', hint: 'Забрали или отменены', test: function (o) { return o.status === 'picked_up' || o.status === 'cancelled'; } }
  ];
  var BAZAAR = 'bazaar';

  function initList() {
    var params = new URLSearchParams(location.search);
    // Длинный поиск раздувает URL (лимит заголовков сервера) — режем до 100 символов, как поле.
    var Q_MAX = 100;
    var state = { view: params.get('view') || '', supplier: params.get('supplier') || '', q: (params.get('q') || '').slice(0, Q_MAX) };
    if (!VIEWS.some(function (x) { return x.v === state.view; })) state.view = '';
    var all = [];
    var supSel = $('#po-supplier');
    var qInput = $('#po-q');
    qInput.value = state.q;
    if (params.get('toast') === 'deleted') toast('Черновик удалён — восстановить можно в «Удалённых»');

    supSel.addEventListener('change', function () { state.supplier = supSel.value; render(); });
    var qTimer = null;
    qInput.addEventListener('input', function () {
      clearTimeout(qTimer);
      qTimer = setTimeout(function () { state.q = qInput.value.trim().slice(0, Q_MAX); render(); }, 200);
    });
    $('#po-reset').addEventListener('click', function () {
      state.view = ''; state.supplier = ''; state.q = '';
      supSel.value = ''; qInput.value = '';
      render();
    });

    function syncUrl() {
      var p = new URLSearchParams();
      if (state.view) p.set('view', state.view);
      if (state.supplier) p.set('supplier', state.supplier);
      if (state.q) p.set('q', state.q);
      history.replaceState(null, '', '/purchases' + (p.toString() ? '?' + p : ''));
    }
    function supplierKey(o) { return o.supplier_id || BAZAAR; }

    // Поставщики — из самих заказов: так в фильтре есть и архивные, у которых остались заказы.
    function fillSuppliers() {
      var seen = {};
      var opts = [];
      all.forEach(function (o) {
        var k = supplierKey(o);
        if (seen[k]) { seen[k].n++; return; }
        seen[k] = { k: k, name: o.recipient_name, n: 1 };
        opts.push(seen[k]);
      });
      opts.sort(function (a, b) {
        if (a.k === BAZAAR) return 1;
        if (b.k === BAZAAR) return -1;
        return a.name.localeCompare(b.name, 'ru');
      });
      if (state.supplier && !seen[state.supplier]) state.supplier = '';
      supSel.textContent = '';
      supSel.append(new Option('Все поставщики', ''));
      opts.forEach(function (x) { supSel.append(new Option(x.name + ' (' + x.n + ')', x.k)); });
      supSel.value = state.supplier;
    }

    function matchesBase(o) {
      if (state.supplier && supplierKey(o) !== state.supplier) return false;
      if (state.q) {
        var q = state.q.toLowerCase();
        var hay = [o.number_display, String(o.number), o.recipient_name, o.created_by_name].join(' ').toLowerCase();
        if (hay.indexOf(q) < 0) return false;
      }
      return true;
    }

    function renderViews(base) {
      var box = $('#po-views');
      box.textContent = '';
      VIEWS.forEach(function (v) {
        var n = base.filter(v.test).length;
        var active = state.view === v.v;
        var b = el('button', 'po-view btn btn-sm ' + (active ? 'btn-' + (v.cls || 'primary') : 'btn-outline-' + (v.cls || 'secondary')));
        b.type = 'button';
        b.setAttribute('aria-pressed', active ? 'true' : 'false');
        b.setAttribute('data-testid', 'po-view-' + (v.v || 'all'));
        if (v.hint) b.title = v.hint;
        b.append(document.createTextNode(v.t + ' '));
        var cnt = el('span', 'badge rounded-pill ' + (active ? 'text-bg-light' : 'text-bg-secondary'), String(n));
        cnt.setAttribute('data-testid', 'po-view-count');
        b.append(cnt);
        b.addEventListener('click', function () { state.view = active ? '' : v.v; render(); });
        box.append(b);
      });
    }

    function card(o, color) {
      var a = el('a', 'task-card po-card text-reset text-decoration-none d-block');
      a.href = '/purchases/' + encodeURIComponent(o.id);
      a.style.setProperty('--c', color);
      a.style.cursor = 'pointer';
      a.setAttribute('data-testid', 'po-row');
      a.dataset.id = o.id;
      var title = el('div', 'task-card-title d-flex align-items-center gap-2');
      var num = el('span', null, o.number_display);
      num.setAttribute('data-testid', 'po-row-number');
      title.append(num);
      var st = badge(STATUS, o.status, 'po-row-status');
      st.classList.add('ms-auto', 'fw-normal');
      title.append(st);
      a.append(title);
      var rec = el('div', 'task-card-client');
      var rn = el('span', 'name', o.recipient_name);
      rn.setAttribute('data-testid', 'po-row-recipient');
      rec.append(rn);
      a.append(rec);
      if (o.has_discrepancy || o.is_changed) {
        var flags = el('div', 'd-flex flex-wrap gap-1 mb-1');
        if (o.has_discrepancy) {
          var d = el('span', 'badge text-bg-danger', 'Расхождение');
          d.setAttribute('data-testid', 'po-row-discrepancy');
          flags.append(d);
        }
        if (o.is_changed) {
          var ch = el('span', 'badge text-bg-warning', 'Изменён');
          ch.setAttribute('data-testid', 'po-row-changed');
          flags.append(ch);
        }
        a.append(flags);
      }
      var tot = el('div', 'task-card-total');
      tot.setAttribute('data-testid', 'po-row-total');
      tot.textContent = o.items_count + ' поз.' + (o.total_amount ? ' · ' + money(o.total_amount) + ' ₽' : '');
      a.append(tot);
      var foot = el('div', 'task-card-foot text-body-secondary');
      var when = o.picked_up_at || o.sent_at || o.created_at;
      var whenLabel = o.picked_up_at ? 'Забрали ' : (o.sent_at ? 'Отпр. ' : '');
      foot.append(el('span', 'text-truncate', o.created_by_name), el('span', 'ms-auto text-nowrap', whenLabel + dt(when)));
      a.append(foot);
      return a;
    }

    function render() {
      syncUrl();
      var base = all.filter(matchesBase);
      renderViews(base);
      var view = VIEWS.filter(function (x) { return x.v === state.view; })[0] || VIEWS[0];
      var shown = base.filter(view.test);
      var board = $('#po-board');
      board.textContent = '';
      var isEmpty = all.length === 0;
      $('#po-empty').classList.toggle('d-none', !isEmpty);
      board.classList.toggle('d-none', isEmpty);
      $('#po-total').textContent = isEmpty ? '' : 'Показано: ' + shown.length + ' из ' + all.length;
      if (isEmpty) return;
      COLUMNS.forEach(function (col) {
        var items = shown.filter(function (o) { return o.status === col.s; });
        var c = el('section', 'board-col');
        c.style.setProperty('--c', col.c);
        c.setAttribute('data-testid', 'po-col');
        c.dataset.status = col.s;
        var head = el('div', 'board-col-head');
        head.append(el('span', 'dot'), el('span', 'name', STATUS[col.s].t));
        var cnt = el('span', 'count', String(items.length));
        cnt.setAttribute('data-testid', 'po-col-count');
        head.append(cnt);
        var sum = items.reduce(function (acc, o) { return acc + (o.total_amount ? parseFloat(o.total_amount) : 0); }, 0);
        if (sum > 0) head.append(el('span', 'column-total ms-auto', money(sum.toFixed(2)) + ' ₽'));
        c.append(head);
        var body = el('div', 'board-col-body');
        if (!items.length) body.append(el('div', 'text-body-secondary small text-center py-3', '—'));
        items.forEach(function (o) { body.append(card(o, col.c)); });
        c.append(body);
        board.append(c);
      });
    }

    function loadAll(offset, acc) {
      return KT.api.get(API + '?limit=100&offset=' + offset).then(function (resp) {
        var items = resp.items || [];
        acc = acc.concat(items);
        if (items.length && acc.length < resp.total) return loadAll(offset + items.length, acc);
        return acc;
      });
    }
    loadAll(0, []).then(function (rows) {
      all = rows;
      fillSuppliers();
      render();
    }).catch(function () {
      $('#po-board').textContent = '';
      $('#po-board').append(el('div', 'empty-state', 'Не удалось загрузить заказы'));
      toast('Не удалось загрузить список', true);
    });
  }

  // ======================= §8.2 новый заказ =======================
  function initNew(root) {
    var lines = []; // {product_id,name,sku,unit,stock,min_stock,pending,reason,tasks,in_open_orders,checked,qty}
    var body = $('#po-lines-body');
    var saving = false;

    function lineOf(pid) { for (var i = 0; i < lines.length; i++) if (lines[i].product_id === pid) return lines[i]; return null; }
    function clearErrors() {
      ['#po-supplier-error', '#po-note-error', '#po-items-error'].forEach(function (s) { $(s).textContent = ''; });
      body.querySelectorAll('.is-invalid').forEach(function (n) { n.classList.remove('is-invalid'); });
      body.querySelectorAll('[data-line-error]').forEach(function (n) { n.textContent = ''; });
      body.querySelectorAll('tr.table-danger').forEach(function (n) { n.classList.remove('table-danger'); });
    }
    function render() {
      body.textContent = '';
      lines.forEach(function (l) {
        var tr = el('tr');
        tr.setAttribute('data-testid', 'po-line');
        tr.dataset.productId = l.product_id;
        var cb = el('input', 'form-check-input');
        cb.type = 'checkbox';
        cb.checked = l.checked;
        cb.setAttribute('aria-label', 'Заказать ' + l.name);
        cb.setAttribute('data-testid', 'po-line-check');
        cb.addEventListener('change', function () { l.checked = cb.checked; });
        var td0 = el('td'); td0.append(cb);

        var td1 = el('td', 'text-break');
        td1.append(el('div', 'fw-semibold', l.name));
        if (l.sku) td1.append(el('div', 'small text-body-secondary', l.sku));
        (l.in_open_orders || []).forEach(function (o) {
          var hint = el('div', 'po-hint');
          hint.setAttribute('data-testid', 'po-line-open-hint');
          hint.append('уже в открытом заказе ');
          var a = el('a', null, o.number_display);
          a.href = '/purchases/' + encodeURIComponent(o.id);
          hint.append(a, ' — ' + num(o.qty) + ' ' + (l.unit || 'шт.'));
          td1.append(hint);
        });
        var td2 = el('td', 'text-end', l.stock == null ? '' : num(l.stock));
        var td3 = el('td', 'text-end', l.min_stock == null ? '' : num(l.min_stock));
        var td4 = el('td');
        if (l.pending != null) td4.append(el('span', null, num(l.pending)));
        (l.tasks || []).forEach(function (t) {
          var a = el('a', 'd-block small', '#' + t.id + ' ' + t.title);
          a.href = '/?task=' + encodeURIComponent(t.id);
          td4.append(a);
        });
        var td5 = el('td'); td5.append(badge(REASON, l.reason, 'po-line-reason'));
        var td6 = el('td');
        var qty = el('input', 'form-control form-control-sm po-qty');
        qty.type = 'text';
        qty.inputMode = 'decimal';
        qty.value = num(l.qty);
        qty.setAttribute('aria-label', 'К заказу');
        qty.setAttribute('data-testid', 'po-line-qty');
        qty.addEventListener('input', function () { l.qty = qty.value.trim().replace(',', '.'); });
        var qerr = el('div', 'invalid-feedback d-block');
        qerr.setAttribute('data-line-error', '');
        qerr.setAttribute('data-testid', 'po-line-error');
        td6.append(qty, qerr);
        l.qtyInput = qty; l.row = tr; l.errEl = qerr;
        [td0, td1, td2, td3, td4, td5, td6].forEach(function (td) { tr.append(td); });
        body.append(tr);
      });
      $('#po-lines-empty').classList.toggle('d-none', lines.length > 0);
      $('#po-lines-wrap').classList.toggle('d-none', lines.length === 0);
    }
    ['for_orders', 'below_min'].forEach(function (r) {
      var t = $('#po-toggle-' + r);
      t.addEventListener('change', function () {
        lines.forEach(function (l) { if (l.reason === r) l.checked = t.checked; });
        render();
      });
    });
    productSearch($('#po-add'), function (p) {
      var ex = lineOf(p.id);
      if (ex) { ex.qtyInput.focus(); ex.qtyInput.select(); return; }
      lines.push({ product_id: p.id, name: p.name, sku: p.sku, unit: p.unit, reason: 'manual', checked: true, qty: '1', tasks: [], in_open_orders: [] });
      render();
    });
    KT.api.get(API + '/suggestions').then(function (resp) {
      lines = (resp.items || []).map(function (s) {
        return { product_id: s.product_id, name: s.name, sku: s.sku, unit: s.unit, stock: s.stock, min_stock: s.min_stock,
          pending: s.pending, reason: s.reason, tasks: s.tasks, in_open_orders: s.in_open_orders, checked: s.checked, qty: s.to_order };
      });
      render();
    }).catch(function () { render(); toast('Не удалось рассчитать дефицит', true); });

    $('#po-save-draft').addEventListener('click', function () {
      if (saving) return;
      clearErrors();
      var sup = $('#po-supplier').value;
      if (!sup) { $('#po-supplier-error').textContent = 'Выберите получателя'; return; }
      var picked = lines.filter(function (l) { return l.checked; });
      if (picked.length === 0) { $('#po-items-error').textContent = FIELD_TEXT.items.required; return; }
      var body = {
        supplier_id: sup === 'bazaar' ? null : sup,
        note: $('#po-note').value,
        items: picked.map(function (l) { return { product_id: l.product_id, qty: String(l.qty), reason: l.reason }; })
      };
      saving = true;
      var btn = $('#po-save-draft');
      btn.disabled = true;
      KT.api.post(API, body).then(function (o) {
        location.href = '/purchases/' + encodeURIComponent(o.id) + '?created=' + encodeURIComponent(o.number_display);
      }).catch(function (err) {
        saving = false;
        btn.disabled = false;
        if (!(err instanceof KT.ApiError) || err.status !== 400 || !(err.fields || []).length) { failure(err, 'Не удалось сохранить'); return; }
        err.fields.forEach(function (f) {
          var it = fieldBase(f.name);
          if (it) {
            var l = picked[it.index];
            if (!l) return;
            if (it.name === 'qty') { l.qtyInput.classList.add('is-invalid'); l.errEl.textContent = FIELD_TEXT.qty; }
            else { l.row.classList.add('table-danger'); l.errEl.textContent = FIELD_TEXT.product_id; }
            return;
          }
          if (f.name === 'supplier_id') $('#po-supplier-error').textContent = FIELD_TEXT.supplier_id;
          else if (f.name === 'note') $('#po-note-error').textContent = FIELD_TEXT.note;
          else if (f.name === 'items') $('#po-items-error').textContent = FIELD_TEXT.items[f.message] || err.message;
          else toast(err.message, true);
        });
      });
    });
  }

  // ======================= §8.3 карточка =======================
  function initView(root) {
    var id = root.dataset.poId;
    var isAdmin = root.dataset.isAdmin === 'true';
    var order = null, items = [], dirty = false, busy = false;

    var params = new URLSearchParams(location.search);
    if (params.get('created')) {
      toast('Черновик ' + params.get('created') + ' создан');
      history.replaceState(null, '', location.pathname);
    }
    window.addEventListener('beforeunload', function (e) { if (dirty) { e.preventDefault(); e.returnValue = ''; } });

    function markDirty() { dirty = true; }
    function load() {
      return KT.api.get(API + '/' + encodeURIComponent(id)).then(function (o) { show(o); }).catch(function (err) {
        if (err instanceof KT.ApiError && err.status === 404) { dirty = false; location.reload(); return; }
        failure(err, 'Не удалось загрузить заказ');
      });
    }
    function show(o) {
      order = o;
      dirty = false;
      items = o.items.map(function (it) { return Object.assign({}, it); });
      $('#po-number').textContent = o.number_display;
      var b = $('#po-status-badge');
      b.className = 'badge ' + STATUS[o.status].c;
      b.textContent = STATUS[o.status].t;
      b.dataset.status = o.status;
      $('#po-recipient').textContent = o.recipient_name;
      $('#po-recipient-phone').textContent = o.recipient_phone || '';
      var d = ['Создан ' + dt(o.created_at) + ' — ' + o.created_by_name];
      if (o.sent_at) d.push('Отправлен ' + dt(o.sent_at));
      if (o.ready_at) d.push('Собран ' + dt(o.ready_at));
      if (o.picked_up_at) d.push('Забрали ' + dt(o.picked_up_at));
      if (o.cancelled_at) d.push('Отменён ' + dt(o.cancelled_at));
      $('#po-dates').textContent = d.join(' · ');
      var cr = $('#po-cancel-reason');
      cr.classList.toggle('d-none', !o.cancel_reason);
      cr.textContent = o.cancel_reason ? 'Причина отмены: ' + o.cancel_reason : '';
      var note = $('#po-note');
      note.value = o.note;
      note.readOnly = !o.can_edit;
      note.oninput = markDirty;
      $('#po-add-wrap').classList.toggle('d-none', !o.can_edit);
      renderItems();
      renderButtons();
    }
    function answered() { return items.some(function (it) { return it.supplier_qty != null; }); }
    function renderItems() {
      var head = $('#po-items-head');
      var showAnswers = answered();
      var cols = ['№', 'Запчасть', 'Тип', 'Просим'];
      if (showAnswers) cols = cols.concat(['Соберёт', 'Цена', 'Сумма', 'Комментарий']);
      if (order.can_edit) cols.push('');
      head.textContent = '';
      cols.forEach(function (c) { head.append(el('th', null, c)); });
      var body = $('#po-items-body');
      body.textContent = '';
      items.forEach(function (it, i) {
        var tr = el('tr', it.has_discrepancy ? 'po-discrepancy' : '');
        tr.setAttribute('data-testid', 'po-item');
        tr.dataset.productId = it.product_id;
        if (it.has_discrepancy) tr.dataset.discrepancy = 'true';
        tr.append(el('td', null, String(i + 1)));
        var name = el('td', 'text-break');
        name.append(el('div', 'fw-semibold', it.name));
        name.append(el('div', 'small text-body-secondary', [it.sku, it.unit].filter(Boolean).join(' · ')));
        if (it.customer_changed) name.append(el('span', 'badge text-bg-warning', 'Изменено'));
        tr.append(name);
        var tdr = el('td'); tdr.append(badge(REASON, it.reason)); tr.append(tdr);
        var tdq = el('td');
        if (order.can_edit) {
          var q = el('input', 'form-control form-control-sm po-qty');
          q.type = 'text'; q.inputMode = 'decimal'; q.value = num(it.qty);
          q.setAttribute('aria-label', 'Просим');
          q.setAttribute('data-testid', 'po-item-qty');
          q.addEventListener('input', function () { it.qty = q.value.trim().replace(',', '.'); markDirty(); });
          var qe = el('div', 'invalid-feedback d-block');
          qe.setAttribute('data-testid', 'po-item-error');
          it.qtyInput = q; it.errEl = qe; it.row = tr;
          tdq.append(q, qe);
        } else {
          tdq.textContent = num(it.qty);
          tdq.setAttribute('data-testid', 'po-item-qty-text');
        }
        tr.append(tdq);
        if (showAnswers) {
          var sq = el('td', null, num(it.supplier_qty)); sq.setAttribute('data-testid', 'po-item-supplier-qty');
          var sp = el('td', null, money(it.supplier_price)); sp.setAttribute('data-testid', 'po-item-supplier-price');
          var la = el('td', null, money(it.line_amount)); la.setAttribute('data-testid', 'po-item-line-amount');
          var sn = el('td', 'text-break small', it.supplier_note || ''); sn.setAttribute('data-testid', 'po-item-supplier-note');
          tr.append(sq, sp, la, sn);
        }
        if (order.can_edit) {
          var tdx = el('td');
          var x = el('button', 'btn btn-sm btn-outline-danger', '×');
          x.type = 'button';
          x.setAttribute('aria-label', 'Удалить строку');
          x.setAttribute('data-testid', 'po-item-remove');
          x.addEventListener('click', function () { items.splice(items.indexOf(it), 1); markDirty(); renderItems(); });
          tdx.append(x);
          tr.append(tdx);
        }
        body.append(tr);
      });
      $('#po-total').textContent = order.total_amount ? 'Итого: ' + money(order.total_amount) : '';
    }
    function button(text, cls, testid, fn) {
      var b = el('button', 'btn ' + cls, text);
      b.type = 'button';
      b.setAttribute('data-testid', testid);
      b.addEventListener('click', function () { if (!busy) fn(b); });
      return b;
    }
    function renderButtons() {
      var box = $('#po-buttons');
      box.textContent = '';
      var o = order, bazaar = !o.supplier_id;
      var add = function (b) { box.append(b); };
      if (o.status === 'draft') {
        add(button('Сохранить', 'btn-outline-primary', 'po-save', function () { save().catch(function () {}); }));
        if (!bazaar) add(button('Отправить поставщику', 'btn-primary', 'po-send', send));
        if (bazaar) add(button('Забрали', 'btn-outline-success', 'po-pickup', pickUp));
        add(button('Отменить', 'btn-outline-secondary', 'po-cancel', cancel));
        if (isAdmin) add(button('Удалить', 'btn-outline-danger', 'po-delete', del));
      } else if (o.status === 'sent') {
        add(button('Сохранить', 'btn-outline-primary', 'po-save', function () { save().catch(function () {}); }));
        add(button('Отозвать', 'btn-outline-secondary', 'po-recall', recall));
        if (!o.supplier_has_login) add(button('Забрали', 'btn-outline-success', 'po-pickup', pickUp));
        add(button('Отменить', 'btn-outline-secondary', 'po-cancel', cancel));
      } else if (o.status === 'ready') {
        add(button('Забрали', 'btn-success', 'po-pickup', pickUp));
        add(button('Отменить', 'btn-outline-secondary', 'po-cancel', cancel));
      }
    }
    function clearErrors() {
      $('#po-note-error').textContent = '';
      $('#po-items-error').textContent = '';
      items.forEach(function (it) {
        if (it.qtyInput) it.qtyInput.classList.remove('is-invalid');
        if (it.errEl) it.errEl.textContent = '';
        if (it.row) it.row.classList.remove('table-danger');
      });
    }
    // conflict — 409/404 действия: тост; статус/версия — перечитать карточку.
    function conflict(err, fallback) {
      if (err instanceof KT.ApiError && err.status === 404) { dirty = false; location.reload(); return; }
      failure(err, fallback);
      if (err instanceof KT.ApiError && err.status === 409) {
        if (err.code === 'PURCHASE_ORDER_VERSION_CONFLICT') showReload();
        else load();
      }
    }
    function showReload() {
      if ($('#po-reload')) return;
      var a = el('div', 'alert alert-warning d-flex align-items-center gap-2 mt-2');
      a.id = 'po-reload';
      a.append(el('span', null, 'Заказ изменился, обновите страницу'));
      var b = el('button', 'btn btn-sm btn-warning ms-auto', 'Обновить');
      b.type = 'button';
      b.setAttribute('data-testid', 'po-reload');
      b.addEventListener('click', function () { a.remove(); load(); });
      a.append(b);
      root.prepend(a);
    }
    function run(btn, p) {
      busy = true;
      if (btn) btn.disabled = true;
      return p.then(function (r) { busy = false; if (btn) btn.disabled = false; return r; },
        function (e) { busy = false; if (btn) btn.disabled = false; throw e; });
    }
    function save() {
      clearErrors();
      var body = {
        version: order.version,
        note: $('#po-note').value,
        items: items.map(function (it) { return { id: it.id || null, product_id: it.product_id, qty: String(it.qty), reason: it.reason }; })
      };
      if (body.items.length === 0) { $('#po-items-error').textContent = 'Добавьте хотя бы одну строку'; return Promise.reject(null); }
      return run(null, KT.api.put(API + '/' + encodeURIComponent(id), body)).then(function (o) {
        show(o);
        toast('Сохранено');
        return o;
      }).catch(function (err) {
        if (err instanceof KT.ApiError && err.status === 400 && (err.fields || []).length) {
          err.fields.forEach(function (f) {
            var fi = fieldBase(f.name);
            var it = fi && items[fi.index];
            if (it && it.errEl) {
              if (fi.name === 'qty') { it.qtyInput.classList.add('is-invalid'); it.errEl.textContent = FIELD_TEXT.qty; }
              else { it.row.classList.add('table-danger'); it.errEl.textContent = err.message; }
            } else if (f.name === 'note') $('#po-note-error').textContent = FIELD_TEXT.note;
            else $('#po-items-error').textContent = err.message;
          });
        } else if (err) conflict(err, 'Не удалось сохранить');
        throw null;
      });
    }
    function transition(btn, path, okText) {
      var go = function () {
        return run(btn, KT.api.post(API + '/' + encodeURIComponent(id) + '/' + path, { version: order.version })).then(function (o) {
          show(o);
          toast(okText);
        }).catch(function (err) { conflict(err, 'Не удалось выполнить действие'); });
      };
      return dirty ? save().then(go, function () {}) : go();
    }
    function send(btn) { transition(btn, 'send', 'Отправлено поставщику'); }
    function recall(btn) { transition(btn, 'recall', 'Заказ отозван'); }
    function pickUp(btn) {
      KT.uiDialog({ title: 'Забрали', text: 'Отметить, что забрали? Остатки не изменятся — приход введите как обычно', ok: 'Да' }).then(function (yes) {
        if (yes) transition(btn, 'pick-up', 'Заказ закрыт');
      });
    }
    function cancel() {
      reasonModal({ title: 'Отменить заказ', text: order.number_display, ok: 'Отменить заказ', submit: function (reason) {
        return KT.api.post(API + '/' + encodeURIComponent(id) + '/cancel', { version: order.version, reason: reason });
      } }).then(function (out) {
        if (!out) return;
        if (out.error) { conflict(out.error, 'Не удалось отменить'); return; }
        show(out.result);
        toast('Заказ отменён');
      });
    }
    function del() {
      reasonModal({ title: 'Удалить черновик', text: order.number_display, ok: 'Удалить', submit: function (reason) {
        return KT.api.delWith(API + '/' + encodeURIComponent(id), { version: order.version, reason: reason });
      } }).then(function (out) {
        if (!out) return;
        if (out.error) { conflict(out.error, 'Не удалось удалить'); return; }
        dirty = false;
        location.href = '/purchases?toast=deleted';
      });
    }
    productSearch($('#po-add'), function (p) {
      for (var i = 0; i < items.length; i++) {
        if (items[i].product_id === p.id) { if (items[i].qtyInput) { items[i].qtyInput.focus(); items[i].qtyInput.select(); } return; }
      }
      items.push({ id: null, product_id: p.id, name: p.name, sku: p.sku, unit: p.unit, qty: '1', reason: 'manual' });
      markDirty();
      renderItems();
    });
    load();
  }

  document.addEventListener('DOMContentLoaded', function () {
    var root = document.querySelector('[data-po-page]');
    if (!root) return;
    ({ list: initList, 'new': initNew, view: initView })[root.dataset.poPage](root);
  });
})();
