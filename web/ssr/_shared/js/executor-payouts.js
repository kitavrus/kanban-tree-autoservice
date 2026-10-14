/* KT.executorPayouts — экран-отчёт «Выплаты» «/executor-payouts» (executor-payouts D62–D70)
   поверх JSON API /api/v1/executor-payouts.

   Группы «исполнитель → его услуги» (заказ #, услуга, статус, цена, сумма выплаты,
   выплачено/дата); итоги по исполнителю «к выплате / выплачено» приходят с сервера
   (/summary — те же фильтры, без постраничности списка). Фильтры — исполнитель (по
   умолчанию: admin — все, executor — «мои»), статус выплаты, «только выполненные»; они
   живут в адресе страницы. Массовая отметка — чекбоксы строк/групп + «Отметить
   выплаченными» / «Снять отметку» (POST /mark, атомарно на сервере).
   Сумма выплаты правится инлайн-полем строки (PUT /executor-payouts/:id — меняет только
   сумму): сохранение по blur/Enter, если значение изменилось; после успеха итоги
   перезапрашиваются, выбор чекбоксов и фильтры не сбрасываются.
   Суммы складываются в копейках, весь текст — через esc(). */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function $(sel, root) { return (root || document).querySelector(sel); }
  function $$(sel, root) { return [].slice.call((root || document).querySelectorAll(sel)); }
  function cents(v) { var n = Number(v); return isFinite(n) ? Math.round(n * 100) : 0; }
  function fmtCents(c) { return (c / 100).toFixed(2); }
  function money(v) { return fmtCents(cents(v)); }

  var PAGE = 500;       // размер страницы списка
  var MARK_MAX = 500;   // предел subtask_ids одного POST /mark (сервер: markMax)
  var MAX_ITEMS = 5000; // предел строк на экране — дальше просим сузить фильтры

  var STATUS = {
    'new': { label: 'Новая', cls: 'text-bg-secondary' },
    in_progress: { label: 'В работе', cls: 'text-bg-primary' },
    done: { label: 'Выполнена', cls: 'text-bg-success' }
  };
  var PAYOUT_TEXT = {
    required: 'сумма не передана',
    invalid: 'недопустимое значение',
    invalid_format: 'введите число',
    negative: 'сумма не может быть отрицательной',
    too_precise: 'не больше 2 знаков после запятой',
    too_large: 'слишком большая сумма'
  };
  var FIELD_TEXT = {
    required: 'не выбрано ни одной услуги',
    invalid: 'недопустимое значение',
    too_many: 'за один раз можно отметить не более 500 услуг',
    duplicate: 'услуга выбрана дважды',
    not_found: 'услуга уже удалена',
    no_assignee: 'у услуги нет исполнителя'
  };

  var state = {
    isAdmin: false,
    filters: { assignee: 'all', status: 'all', only_done: false },
    assignees: [],
    items: [],
    total: 0,
    summary: [],
    selected: {}, // subtask_id → true
    busy: false
  };

  function assigneeDefault() { return state.isAdmin ? 'all' : 'me'; }

  // ---------- фильтры ↔ адрес ----------
  function parseFilters() {
    var p = new URLSearchParams(window.location.search);
    return {
      assignee: p.get('assignee') || assigneeDefault(),
      status: p.get('status') || 'all',
      only_done: p.get('only_done') === 'true'
    };
  }
  function pushFiltersToUrl() {
    var f = state.filters;
    var p = new URLSearchParams();
    if (f.assignee && f.assignee !== assigneeDefault()) p.set('assignee', f.assignee);
    if (f.status && f.status !== 'all') p.set('status', f.status);
    if (f.only_done) p.set('only_done', 'true');
    var s = p.toString();
    window.history.replaceState(null, '', window.location.pathname + (s ? '?' + s : ''));
  }
  function apiQuery() {
    var f = state.filters;
    var p = new URLSearchParams();
    p.set('assignee', f.assignee || assigneeDefault());
    if (f.status && f.status !== 'all') p.set('status', f.status);
    if (f.only_done) p.set('only_done', 'true');
    return p;
  }

  // ---------- загрузка ----------
  function loadAssignees() {
    return KT.api.get('/api/v1/assignees?limit=1000').then(function (r) {
      state.assignees = r.items || [];
      var sel = $('#ep-filter-assignee');
      sel.innerHTML = '<option value="all">Все исполнители</option><option value="me">Мои</option>' +
        state.assignees.map(function (a) { return '<option value="' + esc(a.id) + '">' + esc(a.name) + '</option>'; }).join('');
    });
  }

  function loadItems(offset, acc) {
    var p = apiQuery();
    p.set('limit', String(PAGE));
    p.set('offset', String(offset));
    return KT.api.get('/api/v1/executor-payouts?' + p.toString()).then(function (r) {
      var items = acc.concat(r.items || []);
      state.total = r.total || 0;
      if ((r.items || []).length > 0 && items.length < state.total && items.length < MAX_ITEMS) return loadItems(items.length, items);
      return items;
    });
  }

  // load — полная загрузка списка и итогов. Сначала ждёт незавершённые сохранения сумм
  // (whenSavesDone): иначе свежий список мог бы прийти раньше ответа PUT и показать старую
  // сумму.
  function load() {
    return whenSavesDone().then(function () {
      var p = apiQuery();
      p.set('limit', '1000');
      return Promise.all([
        loadItems(0, []),
        KT.api.get('/api/v1/executor-payouts/summary?' + p.toString())
      ]);
    }).then(function (r) {
      state.items = r[0];
      state.summary = r[1].items || [];
      // выбор переживает перезагрузку только для строк, которые всё ещё на экране
      var known = {};
      state.items.forEach(function (it) { known[it.subtask_id] = true; });
      Object.keys(state.selected).forEach(function (id) { if (!known[id]) delete state.selected[id]; });
      render();
    });
  }

  function reload() {
    if (!payoutErrorIsFresh()) showError(''); // не затирать только что показанную ошибку суммы
    return load().catch(function (err) {
      $('#ep-groups').innerHTML = '';
      showError('Не удалось загрузить выплаты' + (err && err.message ? ': ' + err.message : ''));
    });
  }

  // ---------- отрисовка ----------
  // Дата и время — только через KT.time (пояс пользователя, не браузера; DS-032).
  function fmtDateTime(s) { return KT.time.fmtDateTime(s); }

  function showError(msg) {
    var el = $('#ep-error');
    el.textContent = msg || '';
    el.classList.toggle('d-none', !msg);
  }

  function showInfo(msg) {
    var el = $('#ep-info');
    el.textContent = msg || '';
    el.classList.toggle('d-none', !msg);
  }

  // payoutValue — значение поля суммы строки: '' — сумма не задана.
  function payoutValue(it) { return it.executor_payout == null ? '' : money(it.executor_payout); }

  function rowHTML(it) {
    var st = STATUS[it.status] || { label: it.status, cls: 'text-bg-secondary' };
    return '<tr data-testid="ep-row" data-subtask-id="' + esc(it.subtask_id) + '" data-paid="' + (it.is_executor_paid ? 'true' : 'false') + '">' +
      '<td style="width:36px"><input type="checkbox" class="form-check-input" data-testid="ep-row-check" data-subtask-id="' + esc(it.subtask_id) + '"' +
        (state.selected[it.subtask_id] ? ' checked' : '') + ' aria-label="Выбрать услугу"></td>' +
      '<td><a href="/?task=' + encodeURIComponent(it.task_id) + '" data-testid="ep-row-task">#' + esc(it.task_id) + '</a> ' +
        '<span class="text-body-secondary small">' + esc(it.task_title) + ' · ' + esc(it.client_name) + '</span></td>' +
      '<td data-testid="ep-row-service">' + esc(it.service_name) + '</td>' +
      '<td><span class="badge ' + st.cls + '" data-testid="ep-row-status" data-status="' + esc(it.status) + '">' + esc(st.label) + '</span></td>' +
      '<td class="text-end" data-testid="ep-row-price">' + (it.price == null ? '<span class="text-body-secondary">по факту</span>' : money(it.price)) + '</td>' +
      // D72: сумма выплаты правится прямо здесь (blur/Enter); пусто — «не задана».
      // type="text" + inputmode="decimal" — как то же поле в карточке заказа: у number-поля
      // с автосохранением спиннер и колесо мыши меняли бы сумму, а запятая давала badInput.
      '<td class="text-end" data-testid="ep-row-payout"><input type="text" inputmode="decimal" autocomplete="off"' +
        ' class="form-control form-control-sm text-end ms-auto" style="width:110px" data-testid="ep-row-payout-input"' +
        ' data-subtask-id="' + esc(it.subtask_id) + '" value="' + esc(payoutValue(it)) + '" placeholder="—" aria-label="Сумма выплаты исполнителю"></td>' +
      '<td data-testid="ep-row-paid">' + (it.is_executor_paid
        ? '<span class="badge text-bg-success">Выплачено</span> <span class="small text-body-secondary" data-testid="ep-row-paid-at">' + esc(fmtDateTime(it.executor_paid_at)) + '</span>'
        : '<span class="badge text-bg-warning">Не выплачено</span>') + '</td>' +
      '</tr>';
  }

  function groupHTML(g) {
    var rows = state.items.filter(function (it) { return it.assignee_id === g.assignee_id; });
    var allSelected = rows.length > 0 && rows.every(function (it) { return state.selected[it.subtask_id]; });
    return '<div class="card mb-3" data-testid="ep-group" data-assignee-id="' + esc(g.assignee_id) + '">' +
      '<div class="card-header d-flex flex-wrap align-items-center gap-3">' +
        '<div class="form-check mb-0"><input type="checkbox" class="form-check-input" data-testid="ep-group-check" data-assignee-id="' + esc(g.assignee_id) + '"' +
          (allSelected ? ' checked' : '') + (rows.length ? '' : ' disabled') + ' aria-label="Выбрать все услуги исполнителя"></div>' +
        '<strong data-testid="ep-group-name">' + esc(g.assignee_name) + '</strong>' +
        '<span class="ms-auto small">К выплате: <strong data-testid="ep-group-to-pay">' + money(g.to_pay) + '</strong>' +
          ' <span class="text-body-secondary" data-testid="ep-group-unpaid-count">(' + esc(g.unpaid_count) + ')</span></span>' +
        '<span class="small">Выплачено: <strong data-testid="ep-group-paid">' + money(g.paid) + '</strong>' +
          ' <span class="text-body-secondary" data-testid="ep-group-paid-count">(' + esc(g.paid_count) + ')</span></span>' +
      '</div>' +
      '<div class="table-responsive"><table class="table table-sm align-middle mb-0">' +
        '<thead><tr><th></th><th>Заказ</th><th>Услуга</th><th>Статус</th><th class="text-end">Цена</th><th class="text-end">Выплата</th><th>Выплачено</th></tr></thead>' +
        '<tbody>' + rows.map(rowHTML).join('') + '</tbody>' +
      '</table></div>' +
      '</div>';
  }

  function groupEl(assigneeId) {
    return $$('[data-testid="ep-group"]').filter(function (el) { return el.dataset.assigneeId === assigneeId; })[0];
  }
  function payoutInput(subtaskId) {
    return $$('[data-testid="ep-row-payout-input"]').filter(function (el) { return el.dataset.subtaskId === subtaskId; })[0];
  }

  // renderTotals — ТОЧЕЧНОЕ обновление итогов: шапки уже нарисованных групп («К выплате /
  // Выплачено» и счётчики) и общие итоги. Строки не пересоздаются — фокус и набранный, но
  // ещё не сохранённый текст в полях сумм остаются на месте (правка суммы, D72).
  function renderTotals() {
    var toPay = 0, paid = 0;
    state.summary.forEach(function (g) {
      toPay += cents(g.to_pay);
      paid += cents(g.paid);
      var el = groupEl(g.assignee_id);
      if (!el) return;
      $('[data-testid="ep-group-to-pay"]', el).textContent = money(g.to_pay);
      $('[data-testid="ep-group-paid"]', el).textContent = money(g.paid);
      $('[data-testid="ep-group-unpaid-count"]', el).textContent = '(' + g.unpaid_count + ')';
      $('[data-testid="ep-group-paid-count"]', el).textContent = '(' + g.paid_count + ')';
    });
    $('#ep-total-to-pay').textContent = fmtCents(toPay);
    $('#ep-total-paid').textContent = fmtCents(paid);
  }

  // render — полная перерисовка списка (загрузка, фильтр, массовая отметка). Поле суммы, в
  // котором сейчас стоит курсор, переживает её: набранный текст, фокус и выделение
  // возвращаются в ту же строку, если она осталась в выдаче.
  function render() {
    var act = document.activeElement, keep = null;
    if (act && act.matches && act.matches('[data-testid="ep-row-payout-input"]')) {
      keep = { id: act.dataset.subtaskId, value: act.value, start: act.selectionStart, end: act.selectionEnd };
    }
    $('#ep-groups').innerHTML = state.summary.map(groupHTML).join('');
    $('#ep-empty').classList.toggle('d-none', state.summary.length > 0);
    if (keep) {
      var inp = payoutInput(keep.id);
      if (inp) {
        inp.value = keep.value;
        inp.focus();
        try { inp.setSelectionRange(keep.start, keep.end); } catch (e) { /* выделение не критично */ }
      }
    }
    renderTotals();

    var shown = $('#ep-shown');
    var cut = state.items.length < state.total;
    shown.textContent = cut ? 'Показано ' + state.items.length + ' из ' + state.total + ' услуг — сузьте фильтры, чтобы увидеть остальные.' : '';
    shown.classList.toggle('d-none', !cut);
    renderSelection();
  }

  function selectedIds() { return Object.keys(state.selected); }

  function renderSelection() {
    var n = selectedIds().length;
    $('#ep-selected-count').textContent = 'Выбрано: ' + n;
    $('#ep-mark-paid').disabled = state.busy || n === 0;
    $('#ep-mark-unpaid').disabled = state.busy || n === 0;
  }

  // ---------- массовая отметка ----------
  function errorText(err) {
    if (err && err.fields && err.fields.length) {
      var kinds = {};
      err.fields.forEach(function (f) { kinds[FIELD_TEXT[f.message] || f.message] = true; });
      return 'Отметка не сохранена: ' + Object.keys(kinds).join('; ') + '. Ничего не изменено — обновите список и повторите.';
    }
    return 'Отметка не сохранена' + (err && err.message ? ': ' + err.message : '');
  }

  // В запрос идут только выбранные услуги, чья отметка отличается от целевой: уже
  // выплаченные при «Отметить» (и невыплаченные при «Снять») сервер и так не тронул бы, а
  // в лимит одного запроса (MARK_MAX) они бы упёрлись — на экране до MAX_ITEMS строк.
  // Больше лимита — понятное сообщение; на части запрос не режется (отметка атомарна, D70).
  function mark(paid) {
    if (state.busy || !selectedIds().length) return;
    // клик по кнопке увёл фокус из поля суммы с невалидным вводом: сообщение об ошибке суммы
    // только что показано — не затираем его отметкой, пользователь поправит и нажмёт снова.
    if (payoutErrorIsFresh()) return;
    var ids = state.items.filter(function (it) {
      return state.selected[it.subtask_id] && !!it.is_executor_paid !== paid;
    }).map(function (it) { return it.subtask_id; });
    if (!ids.length) {
      state.selected = {};
      render();
      showError('');
      showInfo(paid ? 'Все выбранные услуги уже отмечены выплаченными.' : 'У выбранных услуг отметки о выплате нет.');
      return;
    }
    if (ids.length > MARK_MAX) {
      showInfo('');
      showError('За один раз можно ' + (paid ? 'отметить' : 'снять отметку у') + ' не более ' + MARK_MAX + ' услуг, а к изменению выбрано ' + ids.length +
        '. Выберите меньше услуг или сузьте фильтры — ничего не изменено.');
      return;
    }
    showInfo('');
    state.busy = true;
    renderSelection();
    showError('');
    // сначала дожидаемся сохранения сумм, начатого уходом из поля (blur → PUT), — отметка и
    // последующая полная перезагрузка списка не должны обогнать его.
    whenSavesDone().then(function () {
      return KT.api.post('/api/v1/executor-payouts/mark', { subtask_ids: ids, is_executor_paid: paid });
    }).then(function () {
      state.selected = {};
      return load();
    }).catch(function (err) {
      showError(errorText(err));
      return load().catch(function () { /* ошибка отметки уже показана */ });
    }).then(function () {
      state.busy = false;
      renderSelection();
    });
  }

  // ---------- правка суммы выплаты в строке (D72) ----------
  function itemById(id) { return state.items.filter(function (it) { return it.subtask_id === id; })[0]; }

  function loadSummary() {
    var p = apiQuery();
    p.set('limit', '1000');
    return KT.api.get('/api/v1/executor-payouts/summary?' + p.toString()).then(function (r) {
      state.summary = r.items || [];
    });
  }

  function payoutErrorText(err) {
    if (err && err.fields && err.fields.length) {
      return 'Сумма выплаты не сохранена: ' + err.fields.map(function (f) { return PAYOUT_TEXT[f.message] || f.message; }).join('; ') + '.';
    }
    return 'Сумма выплаты не сохранена' + (err && err.message ? ': ' + err.message : '') + '.';
  }

  // Незавершённые сохранения сумм: load() и mark() ждут их, чтобы полная перерисовка не
  // показала старую сумму поверх ещё летящего PUT.
  var savesInFlight = 0, saveWaiters = [];
  function saveDone() {
    savesInFlight = Math.max(0, savesInFlight - 1);
    if (savesInFlight > 0) return;
    var w = saveWaiters;
    saveWaiters = [];
    w.forEach(function (resolve) { resolve(); });
  }
  function whenSavesDone() {
    return savesInFlight === 0 ? Promise.resolve() : new Promise(function (resolve) { saveWaiters.push(resolve); });
  }

  var payoutErrorAt = 0;
  function showPayoutError(msg) { payoutErrorAt = Date.now(); showError(msg); }
  function payoutErrorIsFresh() { return Date.now() - payoutErrorAt < 1000; }

  // parsePayout — тот же разбор, что у поля «Сумма выплаты исполнителю» в карточке заказа
  // (task-card.js, поле executor_payout: запятая → точка, пусто — «не задана», число ≥ 0 с
  // не более чем 2 знаками); общего хелпера в _shared/js нет — продублировано осознанно,
  // правила менять в обоих местах. Плюс граница NUMERIC(12,2): не больше 10 цифр целой
  // части — отдельным сообщением, без похода на сервер.
  function parsePayout(text) {
    var raw = String(text).trim().replace(',', '.');
    if (raw === '') return { ok: true, value: null };
    var n = Number(raw);
    if (!isFinite(n) || n < 0 || !/^\d+(\.\d{1,2})?$/.test(raw)) {
      return { ok: false, msg: 'Сумма выплаты — число ≥ 0, не больше 2 знаков после запятой' };
    }
    if (raw.split('.')[0].replace(/^0+(?=\d)/, '').length > 10) return { ok: false, msg: 'слишком большая сумма' };
    return { ok: true, value: n };
  }

  // savePayout — уход из поля суммы (blur; Enter тоже уводит фокус). Пусто → null;
  // невалидный ввод → сообщение и откат (НЕ null); значение не изменилось → запрос не уходит.
  // После ответа обновляются ТОЛЬКО это поле и итоги (renderTotals) — остальные строки не
  // пересоздаются, фокус и набранное в других полях не теряются.
  function savePayout(input) {
    var id = input.dataset.subtaskId;
    var it = itemById(id);
    if (!it || input.disabled) return;
    var parsed = parsePayout(input.value);
    if (!parsed.ok) {
      showPayoutError('Сумма выплаты не сохранена: ' + parsed.msg + '.');
      input.value = payoutValue(it);
      return;
    }
    var value = parsed.value;
    var current = it.executor_payout == null ? null : cents(it.executor_payout);
    if ((value == null ? null : cents(value)) === current) {
      input.value = payoutValue(it); // привести вид («100» → «100.00»), без запроса
      return;
    }
    showError('');
    showInfo('');
    input.disabled = true;
    savesInFlight++;
    KT.api.put('/api/v1/executor-payouts/' + encodeURIComponent(id), { executor_payout: value }).then(function (updated) {
      saveDone();
      // строку ищем по subtask_id в ТЕКУЩЕМ списке: за время запроса он мог быть
      // перезагружен (фильтр, массовая отметка); если строки уже нет — обновим только итоги.
      var cur = itemById(id);
      if (cur) state.items[state.items.indexOf(cur)] = updated;
      var inp = payoutInput(id);
      if (inp) {
        inp.disabled = false;
        if (document.activeElement !== inp) inp.value = payoutValue(updated);
      }
      // итоги — с сервера (те же фильтры); при сбое перезапроса обновятся при следующей загрузке.
      return loadSummary().then(renderTotals, function () { /* строка уже актуальна */ });
    }).catch(function (err) {
      saveDone();
      showPayoutError(payoutErrorText(err));
      if (err && err.status === 404) return load().catch(function () { /* ошибка уже показана */ }); // строки больше нет в отчёте
      var inp = payoutInput(id);
      var cur = itemById(id) || it;
      if (inp) {
        inp.disabled = false;
        if (document.activeElement !== inp) inp.value = payoutValue(cur);
      }
    });
  }

  // ---------- события ----------
  function applyFiltersToForm() {
    var f = state.filters;
    var aSel = $('#ep-filter-assignee');
    aSel.value = f.assignee;
    if (aSel.value !== f.assignee) { aSel.value = assigneeDefault(); f.assignee = assigneeDefault(); }
    var sSel = $('#ep-filter-status');
    sSel.value = f.status;
    if (sSel.value !== f.status) { sSel.value = 'all'; f.status = 'all'; }
    $('#ep-filter-only-done').checked = !!f.only_done;
    [aSel, sSel].forEach(function (el) { if (el._cbSync) el._cbSync(); });
  }

  function bind() {
    function onFilter() {
      state.selected = {};
      showInfo('');
      pushFiltersToUrl();
      reload();
    }
    $('#ep-filter-assignee').addEventListener('change', function (e) { state.filters.assignee = e.target.value; onFilter(); });
    $('#ep-filter-status').addEventListener('change', function (e) { state.filters.status = e.target.value; onFilter(); });
    $('#ep-filter-only-done').addEventListener('change', function (e) { state.filters.only_done = e.target.checked; onFilter(); });

    $('#ep-groups').addEventListener('keydown', function (e) {
      if (e.key !== 'Enter') return;
      var inp = e.target.closest('[data-testid="ep-row-payout-input"]');
      if (inp) { e.preventDefault(); inp.blur(); } // blur → focusout → savePayout
    });
    // focusout, а не change: change не приходит, если текст вернули в поле программно
    // (render() восстанавливает набранное после полной перерисовки) — а сохранить его надо.
    $('#ep-groups').addEventListener('focusout', function (e) {
      var payout = e.target.closest && e.target.closest('[data-testid="ep-row-payout-input"]');
      if (payout) savePayout(payout);
    });
    $('#ep-groups').addEventListener('change', function (e) {
      if (e.target.closest('[data-testid="ep-row-payout-input"]')) return; // сумма — по focusout
      var row = e.target.closest('[data-testid="ep-row-check"]');
      if (row) {
        if (row.checked) state.selected[row.dataset.subtaskId] = true; else delete state.selected[row.dataset.subtaskId];
        var group = row.closest('[data-testid="ep-group"]');
        var boxes = $$('[data-testid="ep-row-check"]', group);
        $('[data-testid="ep-group-check"]', group).checked = boxes.length > 0 && boxes.every(function (b) { return b.checked; });
        renderSelection();
        return;
      }
      var grp = e.target.closest('[data-testid="ep-group-check"]');
      if (grp) {
        $$('[data-testid="ep-row-check"]', grp.closest('[data-testid="ep-group"]')).forEach(function (b) {
          b.checked = grp.checked;
          if (grp.checked) state.selected[b.dataset.subtaskId] = true; else delete state.selected[b.dataset.subtaskId];
        });
        renderSelection();
      }
    });

    $('#ep-mark-paid').addEventListener('click', function () { mark(true); });
    $('#ep-mark-unpaid').addEventListener('click', function () { mark(false); });
  }

  // ---------- инициализация ----------
  function init() {
    var root = $('#ep');
    if (!root) return;
    state.isAdmin = root.dataset.isAdmin === 'true';
    state.filters = parseFilters();
    bind();
    loadAssignees()
      .catch(function () { /* фильтр останется с «Все»/«Мои» */ })
      .then(function () {
        applyFiltersToForm();
        return reload();
      });
  }

  KT.executorPayouts = { init: init };
  document.addEventListener('DOMContentLoaded', init);
})();
