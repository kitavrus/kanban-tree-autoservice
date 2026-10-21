/* Кабинет поставщика (docs/features/purchase-orders/draft-spec.md §8.6–§8.7) поверх
   /api/v1/supplier/purchase-orders. Телефон в первую очередь: карточки, крупные поля,
   числовая клавиатура (inputmode=decimal). Ответ по строке сохраняется отдельно — по уходу
   из изменённого поля (DS-063); «Собрано» — после подтверждения. */
(function () {
  'use strict';
  var KT = window.KT || {};
  var API = '/api/v1/supplier/purchase-orders';
  var STATUS = {
    sent: { t: 'Новый', c: 'text-bg-primary' },
    ready: { t: 'Собран', c: 'text-bg-info' },
    picked_up: { t: 'Забрали', c: 'text-bg-success' }
  };
  var FIELD_TEXT = {
    supplier_qty: 'Количество от 0 до 99 999.999',
    supplier_price: 'Цена от 0 до 9 999 999.99',
    supplier_note: 'Комментарий — не больше 500 символов'
  };

  function $(sel, root) { return (root || document).querySelector(sel); }
  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }
  function num(s) {
    if (s == null || s === '') return '';
    var v = String(s);
    return v.indexOf('.') >= 0 ? v.replace(/0+$/, '').replace(/\.$/, '') : v;
  }
  function toast(msg, isError) { KT.uiToast(msg, isError); }
  function setBadge(b, status) {
    var s = STATUS[status] || { t: status, c: 'text-bg-secondary' };
    b.className = 'badge ' + s.c;
    b.textContent = s.t;
    b.dataset.status = status;
  }
  function toList(msg) {
    if (msg) { try { sessionStorage.setItem('so-toast', msg); } catch (e) { /* приватный режим */ } }
    location.href = '/supplier/orders';
  }

  // ======================= §8.6 список =======================
  function initList() {
    var status = new URLSearchParams(location.search).get('status') || '';
    try {
      var t = sessionStorage.getItem('so-toast');
      if (t) { sessionStorage.removeItem('so-toast'); toast(t, true); }
    } catch (e) { /* приватный режим */ }
    document.querySelectorAll('#so-tabs [data-status]').forEach(function (b) {
      b.addEventListener('click', function () { status = b.dataset.status; load(); });
    });
    function load() {
      document.querySelectorAll('#so-tabs [data-status]').forEach(function (b) {
        b.classList.toggle('active', b.dataset.status === status);
      });
      history.replaceState(null, '', '/supplier/orders' + (status ? '?status=' + status : ''));
      KT.api.get(API + '?limit=50&offset=0' + (status ? '&status=' + encodeURIComponent(status) : '')).then(render).catch(function (err) {
        if (err instanceof KT.ApiError && err.status === 400) { toast('Неизвестный статус', true); status = ''; load(); return; }
        toast('Не удалось загрузить', true);
      });
    }
    function render(resp) {
      var box = $('#so-cards');
      box.textContent = '';
      (resp.items || []).forEach(function (o) {
        var col = el('div', 'col-12 col-md-6');
        var card = el('a', 'card so-card text-reset text-decoration-none h-100');
        card.href = '/supplier/orders/' + encodeURIComponent(o.id);
        card.setAttribute('data-testid', 'so-card');
        card.dataset.id = o.id;
        var bodyEl = el('div', 'card-body');
        var top = el('div', 'd-flex flex-wrap align-items-center gap-2 mb-1');
        top.append(el('span', 'fw-semibold fs-5', o.number_display));
        var b = el('span'); setBadge(b, o.status); b.setAttribute('data-testid', 'so-card-status'); top.append(b);
        if (o.is_changed) {
          var ch = el('span', 'badge text-bg-warning', 'Изменён');
          ch.setAttribute('data-testid', 'so-card-changed');
          top.append(ch);
        }
        bodyEl.append(top);
        bodyEl.append(el('div', 'small text-body-secondary', o.sent_at ? KT.time.fmtDateTime(o.sent_at) : ''));
        var meta = el('div', 'd-flex flex-wrap gap-2 mt-1');
        var ans = el('span', null, 'отвечено ' + o.answered_count + ' из ' + o.items_count);
        ans.setAttribute('data-testid', 'so-card-answered');
        meta.append(ans);
        if (o.total_amount) meta.append(el('span', 'ms-auto fw-semibold', o.total_amount));
        bodyEl.append(meta);
        card.append(bodyEl);
        col.append(card);
        box.append(col);
      });
      $('#so-empty').classList.toggle('d-none', (resp.items || []).length > 0);
    }
    load();
  }

  // ======================= §8.7 карточка =======================
  function initView(root) {
    var id = root.dataset.soId;
    var order = null;
    var inflight = [];

    function load() {
      return KT.api.get(API + '/' + encodeURIComponent(id)).then(show).catch(function (err) {
        if (err instanceof KT.ApiError && err.status === 404) { toList('Заказ отозван заказчиком'); return; }
        toast('Не удалось загрузить', true);
      });
    }
    function show(o) {
      order = o;
      $('#so-number').textContent = o.number_display;
      setBadge($('#so-status'), o.status);
      $('#so-sent').textContent = o.sent_at ? 'Отправлен ' + KT.time.fmtDateTime(o.sent_at) : '';
      var note = $('#so-note');
      note.classList.toggle('d-none', !o.note);
      note.classList.toggle('so-note-changed', o.note_changed);
      $('#so-note-changed').classList.toggle('d-none', !o.note_changed);
      $('#so-note-text').textContent = o.note;
      var box = $('#so-items');
      box.textContent = '';
      o.items.forEach(function (it) { box.append(itemCard(it)); });
      $('#so-total').textContent = o.total_amount || '—';
      $('#so-ready').classList.toggle('d-none', !o.can_answer);
    }
    function field(label, testid, value, opts) {
      var wrap = el('div', opts.col);
      var id2 = 'so-f-' + Math.random().toString(36).slice(2);
      var l = el('label', 'form-label small mb-1', label);
      l.htmlFor = id2;
      var inp = el('input', 'form-control');
      inp.id = id2;
      inp.type = 'text';
      if (opts.decimal) inp.inputMode = 'decimal';
      if (opts.max) inp.maxLength = opts.max;
      inp.value = value;
      inp.readOnly = !order.can_answer;
      inp.setAttribute('data-testid', testid);
      var err = el('div', 'invalid-feedback d-block');
      err.setAttribute('data-testid', testid + '-error');
      wrap.append(l, inp, err);
      return { wrap: wrap, input: inp, err: err };
    }
    function itemCard(it) {
      var card = el('div', 'card so-item mb-2');
      card.setAttribute('data-testid', 'so-item');
      card.dataset.itemId = it.id;
      var body = el('div', 'card-body');
      var head = el('div', 'd-flex flex-wrap align-items-start gap-2');
      var title = el('div', 'flex-grow-1');
      title.append(el('div', 'fw-semibold', it.name));
      if (it.sku) title.append(el('div', 'small text-body-secondary', it.sku));
      var asked = el('div', null, 'Просят: ' + num(it.qty) + ' ' + (it.unit || 'шт'));
      asked.setAttribute('data-testid', 'so-item-asked');
      title.append(asked);
      head.append(title);
      var changed = el('span', 'badge text-bg-warning' + (it.customer_changed ? '' : ' d-none'), 'Изменено заказчиком');
      changed.setAttribute('data-testid', 'so-item-changed');
      head.append(changed);
      body.append(head);

      var row = el('div', 'row g-2 mt-1');
      var q = field('Соберу, шт.', 'so-item-qty', num(it.supplier_qty), { col: 'col-6 col-md-3', decimal: true });
      var p = field('Цена за ед.', 'so-item-price', it.supplier_price || '', { col: 'col-6 col-md-3', decimal: true });
      var n = field('Комментарий', 'so-item-note', it.supplier_note || '', { col: 'col-12 col-md-6', max: 500 });
      row.append(q.wrap, p.wrap, n.wrap);
      body.append(row);

      var foot = el('div', 'd-flex flex-wrap align-items-center gap-2 mt-2');
      var amount = el('span', 'fw-semibold', it.line_amount ? 'Сумма: ' + it.line_amount : '');
      amount.setAttribute('data-testid', 'so-item-amount');
      var state = el('span', 'small ms-auto', '');
      state.setAttribute('data-testid', 'so-item-state');
      foot.append(amount, state);
      body.append(foot);
      card.append(body);

      var cur = { version: it.version };
      function discrepancy() {
        var v = q.input.value.trim().replace(',', '.');
        card.classList.toggle('border-warning', v !== '' && parseFloat(v) !== parseFloat(it.qty));
      }
      discrepancy();
      function save() {
        [q, p, n].forEach(function (f) { f.err.textContent = ''; f.input.classList.remove('is-invalid'); });
        card.classList.remove('so-missing');
        var val = function (f) { var v = f.input.value.trim().replace(',', '.'); return v === '' ? null : v; };
        var payload = { version: cur.version, supplier_qty: val(q), supplier_price: val(p), supplier_note: n.input.value };
        [q, p, n].forEach(function (f) { f.input.disabled = true; });
        state.className = 'small ms-auto text-body-secondary';
        state.textContent = 'Сохраняю…';
        var pr = KT.api.put(API + '/' + encodeURIComponent(id) + '/items/' + encodeURIComponent(it.id), payload).then(function (r) {
          cur.version = r.version;
          amount.textContent = r.line_amount ? 'Сумма: ' + r.line_amount : '';
          $('#so-total').textContent = r.total_amount || '—';
          changed.classList.toggle('d-none', !r.customer_changed);
          state.className = 'small ms-auto text-success';
          state.textContent = 'Сохранено ✓';
        }).catch(function (err) {
          if (!(err instanceof KT.ApiError)) {
            state.className = 'small ms-auto text-danger';
            state.textContent = 'Не сохранено — повторить';
            state.style.cursor = 'pointer';
            state.onclick = save;
            return;
          }
          state.textContent = '';
          if (err.status === 400) {
            (err.fields || []).forEach(function (f) {
              var t = f.name === 'supplier_qty' ? q : f.name === 'supplier_price' ? p : f.name === 'supplier_note' ? n : null;
              if (t) { t.err.textContent = FIELD_TEXT[f.name]; t.input.classList.add('is-invalid'); }
            });
            return;
          }
          if (err.status === 404 || (err.status === 409 && /отозван/.test(err.message))) { toList('Заказ отозван заказчиком'); return; }
          toast(err.message, true);
          load();
        }).then(function () {
          [q, p, n].forEach(function (f) { f.input.disabled = !order.can_answer; });
        });
        inflight.push(pr);
        return pr;
      }
      [q, p, n].forEach(function (f) {
        f.input.addEventListener('change', save);
        f.input.addEventListener('input', discrepancy);
      });
      return card;
    }

    $('#so-ready').addEventListener('click', function () {
      var btn = this;
      Promise.all(inflight).then(function () {
        return KT.uiDialog({ title: 'Собрано', text: 'Отметить «Собрано»? После этого изменить ответ нельзя', ok: 'Да' });
      }).then(function (yes) {
        if (!yes) return;
        btn.disabled = true;
        KT.api.post(API + '/' + encodeURIComponent(id) + '/ready', {}).then(function (o) {
          btn.disabled = false;
          show(o);
          toast('Отмечено «Собрано»');
        }).catch(function (err) {
          btn.disabled = false;
          if (!(err instanceof KT.ApiError)) { toast('Нет связи — повторите', true); return; }
          if (err.status === 404) { toList('Заказ отозван заказчиком'); return; }
          toast(err.message, true);
          if (err.code === 'PURCHASE_ORDER_ITEMS_INCOMPLETE') {
            var ids = (err.details && err.details.item_ids) || [];
            var noPrice = /цену/i.test(err.message);
            ids.forEach(function (iid) {
              var card = document.querySelector('[data-testid="so-item"][data-item-id="' + iid + '"]');
              if (!card) return;
              card.classList.add('so-missing');
              var t = card.querySelector('[data-testid="' + (noPrice ? 'so-item-price' : 'so-item-qty') + '-error"]');
              if (t) t.textContent = noPrice ? 'Укажите цену' : 'Заполните';
            });
            return;
          }
          load();
        });
      });
    });
    load();
  }

  document.addEventListener('DOMContentLoaded', function () {
    var root = document.querySelector('[data-so-page]');
    if (!root) return;
    if (root.dataset.soPage === 'list') initList(root);
    else initView(root);
  });
})();
