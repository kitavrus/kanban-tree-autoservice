/* KT.deleted — экран «Удалённые» «/deleted» (admin, soft-delete-core §8.1, DS-011) поверх
   JSON API /api/v1/deleted и /api/v1/deleted/summary.

   Вкладки по видам записей со счётчиками (TABS — реестр; части 2–6 мастер-спеки добавляют
   свои строки), фильтры «Поиск» (debounce 300 мс), «Удалено с / по», «Кто удалил», кнопка
   «Сбросить», таблица, пагинация по 50. Вкладка, фильтры и страница списка живут в адресной
   строке (?kind=&q=&deleted_from=&deleted_to=&deleted_by=&offset=) и переживают перезагрузку.
   «Восстановить» — подтверждение KT.uiDialog → POST restore-адреса вида; предупреждения по
   остатку и notices — жёлтый блок над таблицей (не исчезает сам). Время — KT.time (пояс
   пользователя, «ДД.ММ.ГГГГ ЧЧ:ММ»). Весь текст — через textContent. */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  var API = '/api/v1/deleted';
  var PAGE_LIMIT = 50;
  var DEBOUNCE_MS = 300;

  // Реестр вкладок: вид записей → подпись, ключ счётчика summary, адрес восстановления,
  // текст 404, (если у записи есть экран) ссылка «Открыть» и (если отличается от общего) текст
  // тоста об успешном восстановлении.
  var TABS = [
    {
      kind: 'tasks', label: 'Заказы', countKey: 'tasks',
      restoreUrl: function (id) { return '/api/v1/tasks/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Заказ не найден',
      openLink: function (id) { return { href: '/?task=' + encodeURIComponent(id), text: 'Открыть заказ #' + id }; }
    },
    {
      // часть 2 (soft-delete-transfers §8.2): документы перемещения. Отказы восстановления 409
      // EQUIPMENT_IN_TRANSFER, TRANSFER_CLIENT_MISMATCH и TRANSFER_EQUIPMENT_NOT_IN_TASK — тост с
      // текстом сервера, строка остаётся.
      kind: 'transfers', label: 'Перемещения', countKey: 'transfers',
      restoreUrl: function (id) { return '/api/v1/transfers/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Перемещение не найдено',
      openLink: function (id) { return { href: '/transfers?id=' + encodeURIComponent(id), text: 'Открыть перемещение #' + id }; }
    },
    // часть 6 (soft-delete-warehouses-suppliers §8.2): склады и поставщики — перед вкладкой
    // «Колонки». Запись возвращается с прежним признаком архива; отказ 409 NAME_TAKEN — тост с
    // текстом сервера, строка остаётся. Своего экрана «Открыть» у удалённой записи нет.
    {
      kind: 'warehouses', label: 'Склады', countKey: 'warehouses',
      restoreUrl: function (id) { return '/api/v1/warehouses/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Склад не найден', confirmText: archivedConfirm
    },
    {
      kind: 'suppliers', label: 'Поставщики', countKey: 'suppliers',
      restoreUrl: function (id) { return '/api/v1/suppliers/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Поставщик не найден', confirmText: archivedConfirm
    },
    {
      // часть 3 (soft-delete-board-columns §8.2): колонки доски. Восстановленная колонка встаёт
      // в конец доски — об этом тост; своего экрана у колонки нет (ссылки «Открыть» нет).
      kind: 'columns', label: 'Колонки', countKey: 'columns',
      restoreUrl: function (id) { return '/api/v1/columns/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Колонка не найдена',
      restored: 'Восстановлено. Колонка — в конце доски'
    },
    {
      // часть 4 (soft-delete-clients §8.2): клиенты. Клиент возвращается вместе с единицами,
      // ушедшими с ним, — об этом вопрос подтверждения и тост с их числом.
      kind: 'clients', label: 'Клиенты', countKey: 'clients',
      restoreUrl: function (id) { return '/api/v1/clients/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Клиент не найден',
      confirmText: function (it) { return 'Восстановить «' + it.title + '» вместе с оборудованием?'; },
      restored: function (resp) { return 'Восстановлено вместе с оборудованием: ' + (resp && resp.equipment_restored || 0); }
    },
    {
      // часть 4: единицы оборудования. Отказ 409 CLIENT_DELETED «Сначала восстановите клиента» —
      // тост с текстом сервера, строка остаётся.
      kind: 'equipment', label: 'Оборудование', countKey: 'equipment',
      restoreUrl: function (id) { return '/api/v1/equipment/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Оборудование не найдено'
    },
    // часть 5 (soft-delete-catalog §8.2): товары, услуги, категории каталога и типы
    // оборудования. Запись возвращается с прежним признаком архива (бейдж «в архиве» и строка
    // «Запись вернётся в архив» в подтверждении). Отказ 409 NAME_TAKEN (категория, тип) — тост
    // с текстом сервера, строка остаётся.
    {
      kind: 'products', label: 'Товары', countKey: 'products',
      restoreUrl: function (id) { return '/api/v1/products/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Товар не найден', confirmText: archivedConfirm
    },
    {
      kind: 'services', label: 'Услуги', countKey: 'services',
      restoreUrl: function (id) { return '/api/v1/services/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Услуга не найдена', confirmText: archivedConfirm
    },
    {
      kind: 'catalog-categories', label: 'Категории', countKey: 'catalog_categories',
      restoreUrl: function (id) { return '/api/v1/catalog-categories/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Категория не найдена', confirmText: archivedConfirm
    },
    {
      kind: 'equipment-types', label: 'Типы оборудования', countKey: 'equipment_types',
      restoreUrl: function (id) { return '/api/v1/equipment-types/' + encodeURIComponent(id) + '/restore'; },
      notFound: 'Тип оборудования не найден', confirmText: archivedConfirm
    }
  ];

  // Вопрос подтверждения для видов с архивом (DS-009): запись, удалённая из архива, вернётся
  // в архив — об этом вторая фраза.
  function archivedConfirm(it) {
    return 'Восстановить «' + it.title + '»?' + (it.is_archived ? ' Запись вернётся в архив' : '');
  }

  // Подписи бейджа «удалено вместе с …» по значению deleted_with строки (DS-015).
  var DELETED_WITH = { client: 'вместе с клиентом' };

  function $(sel, root) { return (root || document).querySelector(sel); }
  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }
  function tabOf(kind) {
    for (var i = 0; i < TABS.length; i++) if (TABS[i].kind === kind) return TABS[i];
    return null;
  }

  var state = {
    kind: TABS[0].kind,
    q: '', from: '', to: '', by: '',
    offset: 0,
    items: [], total: 0,
    counts: {},
    seq: 0
  };

  // ---------- адресная строка ----------
  function readUrl() {
    var p = new URLSearchParams(window.location.search);
    // неизвестный вид из адреса уходит на сервер как есть → 400 «Неизвестный вид записей»
    state.kind = p.get('kind') || TABS[0].kind;
    state.q = p.get('q') || '';
    state.from = p.get('deleted_from') || '';
    state.to = p.get('deleted_to') || '';
    state.by = p.get('deleted_by') || '';
    var off = parseInt(p.get('offset'), 10);
    state.offset = off > 0 ? off : 0;
  }
  function writeUrl() {
    var p = new URLSearchParams();
    p.set('kind', state.kind);
    if (state.q) p.set('q', state.q);
    if (state.from) p.set('deleted_from', state.from);
    if (state.to) p.set('deleted_to', state.to);
    if (state.by) p.set('deleted_by', state.by);
    if (state.offset) p.set('offset', String(state.offset));
    window.history.replaceState(null, '', window.location.pathname + '?' + p.toString());
  }
  function listUrl() {
    var p = new URLSearchParams();
    p.set('kind', state.kind);
    if (state.q) p.set('q', state.q);
    if (state.from) p.set('deleted_from', state.from);
    if (state.to) p.set('deleted_to', state.to);
    if (state.by) p.set('deleted_by', state.by);
    p.set('limit', String(PAGE_LIMIT));
    p.set('offset', String(state.offset));
    return API + '?' + p.toString();
  }
  function hasFilters() { return !!(state.q || state.from || state.to || state.by); }

  // ---------- ошибки ----------
  var FIELD_TOAST = {
    kind: { required: 'Выберите вкладку', '*': 'Неизвестный вид записей' },
    deleted_by: { '*': 'Проверьте фильтр «Кто удалил»' },
    limit: { '*': 'Обновите страницу' },
    offset: { '*': 'Обновите страницу' }
  };
  function clearFilterErrors() {
    $('#del-q').classList.remove('is-invalid');
    $('#del-q-error').textContent = '';
    $('#del-from').classList.remove('is-invalid');
    $('#del-to').classList.remove('is-invalid');
    $('#del-period-error').classList.add('d-none');
    $('#del-period-error').textContent = '';
  }
  // failure — ошибка запроса списка/счётчиков: поле фильтра → текст под ним, остальное → тост.
  function failure(err) {
    if (!(err instanceof KT.ApiError)) { KT.uiToast('Нет связи с сервером', true); return; }
    if (err.status === 403) { KT.uiToast('Нет доступа', true); return; }
    if (err.status === 400 && err.fields.length) {
      var toasted = {};
      err.fields.forEach(function (f) {
        if (f.name === 'q') {
          $('#del-q').classList.add('is-invalid');
          $('#del-q-error').textContent = 'Строка поиска — не длиннее 100 символов';
        } else if (f.name === 'deleted_from' || f.name === 'deleted_to') {
          $(f.name === 'deleted_from' ? '#del-from' : '#del-to').classList.add('is-invalid');
          $('#del-period-error').textContent = 'Проверьте период';
          $('#del-period-error').classList.remove('d-none');
        } else {
          var map = FIELD_TOAST[f.name] || {};
          var text = map[f.message] || map['*'] || err.message;
          if (!toasted[text]) { toasted[text] = true; KT.uiToast(text, true); }
        }
      });
      return;
    }
    KT.uiToast(err.message, true);
  }

  // ---------- вкладки ----------
  function renderTabs() {
    var host = $('#del-tabs');
    host.textContent = '';
    TABS.forEach(function (t) {
      var li = el('li', 'nav-item');
      li.setAttribute('role', 'presentation');
      var b = el('button', 'nav-link' + (t.kind === state.kind ? ' active' : ''));
      b.type = 'button';
      b.setAttribute('role', 'tab');
      b.setAttribute('aria-selected', t.kind === state.kind ? 'true' : 'false');
      b.setAttribute('data-testid', 'deleted-tab-' + t.kind);
      b.append(document.createTextNode(t.label + ' ('));
      var n = el('span', null, String(state.counts[t.countKey] || 0));
      n.setAttribute('data-testid', 'deleted-tab-count-' + t.kind);
      b.append(n, document.createTextNode(')'));
      b.addEventListener('click', function () {
        if (state.kind === t.kind) return;
        state.kind = t.kind;
        state.offset = 0; // фильтры сохраняются
        renderTabs();
        writeUrl();
        loadList();
      });
      li.append(b);
      host.append(li);
    });
  }
  function loadSummary() {
    return KT.api.get(API + '/summary').then(function (r) {
      state.counts = r.counts || {};
      renderTabs();
    });
  }

  // ---------- таблица ----------
  function renderList() {
    var tbody = $('#del-tbody');
    tbody.textContent = '';
    var tab = tabOf(state.kind);
    state.items.forEach(function (it) {
      var tr = el('tr');
      tr.setAttribute('data-testid', 'deleted-row');
      tr.setAttribute('data-id', it.id);

      var what = el('td');
      var title = el('div', 'fw-medium');
      title.style.overflowWrap = 'anywhere';
      var titleText = el('span', null, it.title);
      titleText.setAttribute('data-testid', 'deleted-row-title');
      title.append(titleText);
      what.append(title);
      // единица ушла вместе с клиентом (DS-015) — серый бейдж справа от названия
      if (it.deleted_with && DELETED_WITH[it.deleted_with]) {
        var withBadge = el('span', 'badge text-bg-secondary ms-2', DELETED_WITH[it.deleted_with]);
        withBadge.setAttribute('data-testid', 'deleted-row-with');
        title.append(withBadge);
      }
      // запись удалена из архива (DS-009) — серый бейдж «в архиве» справа от названия
      if (it.is_archived) {
        var archBadge = el('span', 'badge text-bg-secondary ms-2', 'в архиве');
        archBadge.setAttribute('data-testid', 'deleted-row-archived');
        title.append(archBadge);
      }
      if (it.subtitle) {
        var sub = el('div', 'small text-body-secondary', it.subtitle);
        sub.style.overflowWrap = 'anywhere';
        sub.setAttribute('data-testid', 'deleted-row-subtitle');
        what.append(sub);
      }

      var reason = el('td', null, it.delete_reason);
      reason.style.whiteSpace = 'pre-wrap';
      reason.style.overflowWrap = 'anywhere';
      reason.style.minWidth = '160px';
      reason.setAttribute('data-testid', 'deleted-row-reason');

      var who = el('td', null, it.deleted_by ? it.deleted_by.name : '');
      who.setAttribute('data-testid', 'deleted-row-by');
      var when = el('td', 'text-nowrap', KT.time.fmtDateTime(it.deleted_at));
      when.setAttribute('data-testid', 'deleted-row-at');

      var act = el('td', 'text-end');
      if (tab) {
        var btn = el('button', 'btn btn-outline-secondary btn-sm', 'Восстановить');
        btn.type = 'button';
        btn.setAttribute('data-testid', 'deleted-restore');
        btn.addEventListener('click', function () { restore(tab, it, btn); });
        act.append(btn);
      }
      tr.append(what, reason, who, when, act);
      tbody.append(tr);
    });

    var empty = state.total === 0;
    $('#del-table-wrap').classList.toggle('d-none', empty);
    $('#del-empty').classList.toggle('d-none', !empty);
    $('#del-empty').textContent = hasFilters() ? 'Ничего не найдено' : 'Удалённых записей нет';

    $('#del-pager').classList.toggle('d-none', empty);
    var first = state.items.length ? state.offset + 1 : 0;
    var last = state.offset + state.items.length;
    $('#del-shown').textContent = 'Показано ' + first + '–' + last + ' из ' + state.total;
    $('#del-prev').disabled = state.offset <= 0;
    $('#del-next').disabled = last >= state.total;
  }

  // loadList — список текущей вкладки. Ответы приходят не по порядку (debounce поиска) —
  // применяется только последний запрошенный (seq). При ошибке таблица остаётся прежней,
  // кроме 4xx — там по спецификации таблица пустая.
  function loadList() {
    var seq = ++state.seq;
    clearFilterErrors();
    $('#del-loading').classList.remove('d-none');
    return KT.api.get(listUrl()).then(function (r) {
      if (seq !== state.seq) return;
      $('#del-loading').classList.add('d-none');
      state.items = r.items || [];
      state.total = r.total || 0;
      // страница за пределами выдачи (записи восстановили) — вернуться на последнюю
      if (!state.items.length && state.total > 0 && state.offset > 0) {
        state.offset = Math.max(0, Math.floor((state.total - 1) / PAGE_LIMIT) * PAGE_LIMIT);
        writeUrl();
        return loadList();
      }
      renderList();
    }).catch(function (err) {
      if (seq !== state.seq) return;
      $('#del-loading').classList.add('d-none');
      failure(err);
      if (err instanceof KT.ApiError) { state.items = []; state.total = 0; renderList(); }
    });
  }

  // ---------- восстановление ----------
  function showWarnings(tab, it, resp) {
    var lines = [];
    (resp.warnings || []).forEach(function (w) {
      var stock = String(w.stock).replace(/^-/, '−');
      lines.push(w.kind === 'negative'
        ? 'Не хватает товара «' + w.name + '» на складе «' + w.warehouse_name + '»: остаток ' + stock
        : 'Товар «' + w.name + '» на складе «' + w.warehouse_name + '» на минимуме: остаток ' + stock);
    });
    (resp.notices || []).forEach(function (n) { lines.push(n.message); });
    if (!lines.length) return;

    var body = $('#del-warnings-body');
    body.textContent = '';
    var head = el('div', 'fw-medium', 'Восстановлено: ' + it.title);
    head.style.overflowWrap = 'anywhere';
    body.append(head);
    lines.forEach(function (text) {
      var line = el('div', null, text);
      line.setAttribute('data-testid', 'deleted-warning-line');
      body.append(line);
    });
    var link = tab.openLink && tab.openLink(it.id);
    if (link) {
      var a = el('a', 'alert-link text-warning-emphasis', link.text);
      a.href = link.href;
      a.setAttribute('data-testid', 'deleted-warning-open');
      body.append(a);
    }
    $('#del-warnings').classList.remove('d-none');
  }

  // dropRow — запись ушла из списка (восстановлена здесь либо другим администратором).
  function dropRow() {
    loadSummary().catch(function () {});
    loadList();
  }

  function restore(tab, it, btn) {
    var question = tab.confirmText ? tab.confirmText(it) : 'Восстановить «' + it.title + '»?';
    KT.uiDialog({ title: 'Восстановление', text: question, ok: 'Восстановить' }).then(function (ok) {
      if (!ok) return;
      btn.disabled = true;
      KT.api.post(tab.restoreUrl(it.id)).then(function (resp) {
        KT.uiToast(typeof tab.restored === 'function' ? tab.restored(resp) : (tab.restored || 'Восстановлено'));
        showWarnings(tab, it, resp);
        dropRow();
      }).catch(function (err) {
        btn.disabled = false;
        if (!(err instanceof KT.ApiError)) { KT.uiToast('Нет связи с сервером', true); return; }
        if (err.status === 409 && err.code === 'NOT_DELETED') { KT.uiToast('Запись уже восстановлена', true); dropRow(); return; }
        if (err.status === 404) { KT.uiToast(tab.notFound, true); dropRow(); return; }
        if (err.status === 403) { KT.uiToast('Нет доступа', true); return; }
        KT.uiToast(err.message, true);
      });
    });
  }

  // ---------- фильтры ----------
  function applyFiltersToForm() {
    $('#del-q').value = state.q;
    $('#del-from').value = state.from;
    $('#del-to').value = state.to;
    $('#del-by').value = state.by;
  }
  function filtersChanged() {
    state.offset = 0;
    writeUrl();
    loadList();
  }
  function bindFilters() {
    var timer = null;
    $('#del-q').addEventListener('input', function () {
      clearTimeout(timer);
      timer = setTimeout(function () {
        state.q = $('#del-q').value.trim();
        filtersChanged();
      }, DEBOUNCE_MS);
    });
    $('#del-from').addEventListener('change', function () { state.from = $('#del-from').value; filtersChanged(); });
    $('#del-to').addEventListener('change', function () { state.to = $('#del-to').value; filtersChanged(); });
    $('#del-by').addEventListener('change', function () { state.by = $('#del-by').value; filtersChanged(); });
    $('#del-reset').addEventListener('click', function () {
      clearTimeout(timer);
      state.q = ''; state.from = ''; state.to = ''; state.by = '';
      applyFiltersToForm();
      filtersChanged();
    });
    $('#del-prev').addEventListener('click', function () {
      state.offset = Math.max(0, state.offset - PAGE_LIMIT);
      writeUrl();
      loadList();
    });
    $('#del-next').addEventListener('click', function () {
      state.offset += PAGE_LIMIT;
      writeUrl();
      loadList();
    });
    $('#del-warnings-close').addEventListener('click', function () { $('#del-warnings').classList.add('d-none'); });
  }

  // «Кто удалил» — активные администраторы (GET /api/v1/assignees); значение из адресной
  // строки, которого нет в списке (сотрудник деактивирован), остаётся отдельным пунктом.
  function loadAdmins() {
    return KT.api.get('/api/v1/assignees?limit=200').then(function (r) {
      var sel = $('#del-by');
      (r.items || []).filter(function (u) { return u.role === 'admin'; }).forEach(function (u) {
        var o = el('option', null, u.name);
        o.value = u.id;
        sel.append(o);
      });
    }).catch(function () { /* фильтр остаётся с одним пунктом «все» */ }).then(function () {
      var sel = $('#del-by');
      if (state.by && ![].some.call(sel.options, function (o) { return o.value === state.by; })) {
        var o = el('option', null, 'Сотрудник не в списке');
        o.value = state.by;
        sel.append(o);
      }
      sel.value = state.by;
    });
  }

  function init() {
    if (!$('#del')) return;
    readUrl();
    renderTabs();
    applyFiltersToForm();
    bindFilters();
    writeUrl();
    loadAdmins();
    loadSummary().catch(failure).then(loadList);
  }

  KT.deleted = { init: init };
  document.addEventListener('DOMContentLoaded', init);
})();
