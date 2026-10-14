/* KT.suppliers — экран «Поставщики» «/suppliers» (admin, suppliers DS-007, draft-spec §8.1)
   поверх JSON API /api/v1/suppliers.

   Слева — таблица с поиском по названию и toggle «Показать архивные» (выкл →
   is_active=true; вкл → параметр не передаётся), debounce 300 мс. Справа — форма:
   ?new=1 — создание (POST), ?id=<uuid> — правка (PUT, там же «В архив» / «Вернуть из
   архива»). Ошибка поля — текст под полем + is-invalid (details.fields[]); успех и
   глобальная ошибка — тост справа вверху на 5 секунд; на время запроса кнопки формы
   заблокированы. Правка — last-write-wins (DS-043). Весь текст — через esc(). */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  var API = '/api/v1/suppliers';
  var PAGE_LIMIT = 50;
  var LOOKUP_LIMIT = 500;
  var DEBOUNCE_MS = 300;
  var TOAST_MS = 5000;
  var FIELDS = ['name', 'contact_name', 'phone', 'address', 'note'];

  // Тексты ошибок полей (§6.2): поле → код → текст; '*' — любой прочий код.
  var FIELD_TEXT = {
    name: { duplicate: 'Поставщик с таким названием уже есть', '*': 'Укажите название (до 200 символов)' },
    contact_name: { '*': 'Слишком длинное имя контакта' },
    phone: { '*': 'Проверьте телефон' },
    address: { '*': 'Слишком длинный адрес' },
    note: { '*': 'Слишком длинное примечание' }
  };

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function $(sel, root) { return (root || document).querySelector(sel); }
  function $$(sel, root) { return [].slice.call((root || document).querySelectorAll(sel)); }

  var state = {
    list: [],
    total: 0,
    q: '',
    showArchived: false,
    current: null, // SupplierResponse (правка) | null (создание)
    formOpen: false,
    busy: false,
    listSeq: 0
  };

  // ---------- тосты ----------
  function toast(msg, isError) {
    var el = document.createElement('div');
    el.className = 'alert ' + (isError ? 'alert-danger' : 'alert-success') + ' shadow py-2 mb-2';
    el.setAttribute('role', isError ? 'alert' : 'status');
    el.setAttribute('data-testid', isError ? 'toast-error' : 'toast');
    el.textContent = msg;
    $('#sup-toasts').appendChild(el);
    setTimeout(function () { el.remove(); }, TOAST_MS);
  }

  // Ошибка без ответа сервера (fetch отклонён) — «Нет связи»; иначе сообщение API.
  function toastFailure(err) {
    if (err instanceof KT.ApiError) toast(err.message, true);
    else toast('Нет связи', true);
  }

  // ---------- адрес ----------
  function setUrl(params) {
    var u = new URL(window.location.href);
    u.searchParams.delete('id');
    u.searchParams.delete('new');
    Object.keys(params || {}).forEach(function (k) { u.searchParams.set(k, params[k]); });
    window.history.replaceState(null, '', u.pathname + u.search);
  }

  // ---------- список ----------
  function listUrl() {
    var p = new URLSearchParams();
    if (state.q) p.set('q', state.q);
    if (!state.showArchived) p.set('is_active', 'true');
    p.set('limit', String(PAGE_LIMIT));
    p.set('offset', '0');
    return API + '?' + p.toString();
  }

  function loadList() {
    var seq = ++state.listSeq;
    return KT.api.get(listUrl()).then(function (r) {
      if (seq !== state.listSeq) return; // пришёл ответ на устаревший фильтр
      state.list = r.items || [];
      state.total = r.total || 0;
      renderList();
    }).catch(function (err) {
      if (seq !== state.listSeq) return;
      if (err instanceof KT.ApiError && err.status === 400) toast('Некорректный фильтр', true);
      else toastFailure(err);
    });
  }

  function renderList() {
    var curId = state.formOpen && state.current ? state.current.id : '';
    $('#sup-tbody').innerHTML = state.list.map(function (s) {
      return '<tr data-testid="supplier-row" data-id="' + esc(s.id) + '" data-name="' + esc(s.name) + '"' +
        ' role="button" tabindex="0"' + (s.id === curId ? ' class="table-active"' : '') + '>' +
        '<td><div class="fw-semibold text-break">' + esc(s.name) + '</div>' +
        (s.is_active ? '' : '<span class="badge-off" data-testid="supplier-archived-badge">в архиве</span>') + '</td>' +
        '<td class="text-break">' + esc(s.contact_name) + '</td>' +
        '<td class="text-break">' + esc(s.phone) + '</td>' +
        '<td class="text-break">' + esc(s.address) + '</td>' +
        '</tr>';
    }).join('');
    var empty = state.list.length === 0;
    $('#sup-table-wrap').classList.toggle('d-none', empty);
    $('#sup-empty').classList.toggle('d-none', !empty);
    var shown = $('#sup-shown');
    if (state.total > state.list.length) {
      shown.textContent = 'Показано ' + state.list.length + ' из ' + state.total;
      shown.classList.remove('d-none');
    } else {
      shown.classList.add('d-none');
    }
  }

  // ---------- форма ----------
  function clearErrors() {
    $$('#sup-form .is-invalid').forEach(function (el) { el.classList.remove('is-invalid'); });
    $$('#sup-form [data-error-for]').forEach(function (el) { el.textContent = ''; });
  }

  // Ошибки полей под полями; ошибка не про поле формы (is_active) — «Обновите страницу».
  function showFieldErrors(fields) {
    clearErrors();
    var other = false;
    (fields || []).forEach(function (f) {
      var name = f.name || f.Name;
      var code = f.message || f.Message;
      var texts = FIELD_TEXT[name];
      var input = $('#sup-form [name="' + name + '"]');
      if (!texts || !input) { other = true; return; }
      input.classList.add('is-invalid');
      $('#sup-form [data-error-for="' + name + '"]').textContent = texts[code] || texts['*'];
    });
    if (other) toast('Обновите страницу', true);
    var first = $('#sup-form .is-invalid');
    if (first) first.focus();
  }

  function setBusy(busy) {
    state.busy = busy;
    ['#sup-save', '#sup-toggle-active', '#sup-cancel', '#sup-delete'].forEach(function (sel) { $(sel).disabled = busy; });
  }

  function openForm(supplier) {
    // запрос формы в полёте: его завершение закрыло бы только что открытую другую форму.
    if (state.busy) return;
    state.current = supplier || null;
    state.formOpen = true;
    clearErrors();
    FIELDS.forEach(function (name) {
      $('#sup-form [name="' + name + '"]').value = supplier ? (supplier[name] || '') : '';
    });
    $('#sup-form-title').textContent = supplier ? 'Поставщик' : 'Новый поставщик';
    var toggle = $('#sup-toggle-active');
    toggle.classList.toggle('d-none', !supplier);
    $('#sup-delete').classList.toggle('d-none', !supplier);
    if (supplier) toggle.textContent = supplier.is_active ? 'В архив' : 'Вернуть из архива';
    $('#sup-archived-note').classList.toggle('d-none', !supplier || supplier.is_active);
    $('#sup-form-col').classList.remove('d-none');
    setUrl(supplier ? { id: supplier.id } : { 'new': '1' });
    renderList();
    $('#sup-name').focus();
  }

  function closeForm() {
    state.current = null;
    state.formOpen = false;
    $('#sup-form-col').classList.add('d-none');
    setUrl(null);
    renderList();
  }

  function formValues() {
    var v = {};
    FIELDS.forEach(function (name) { v[name] = $('#sup-form [name="' + name + '"]').value; });
    return v;
  }

  // Общий разбор ошибки мутации: 400 — под полями, 404 — тост + перезапрос списка.
  function handleMutationError(err) {
    if (err instanceof KT.ApiError && err.status === 400 && err.fields.length) {
      showFieldErrors(err.fields);
      return;
    }
    if (err instanceof KT.ApiError && err.status === 404) {
      toast('Поставщик не найден', true);
      closeForm();
      loadList();
      return;
    }
    toastFailure(err);
  }

  function mutate(request) {
    if (state.busy) return;
    setBusy(true);
    // busy покрывает только сам запрос формы: к моменту перезапроса списка форму уже можно
    // открывать снова (openForm игнорирует клики, пока busy).
    request().then(function () {
      setBusy(false);
      closeForm();
      toast('Сохранено', false);
      loadList();
    }, function (err) {
      setBusy(false);
      handleMutationError(err);
    });
  }

  function save() {
    var body = formValues();
    var cur = state.current;
    if (!cur) {
      mutate(function () { return KT.api.post(API, body); });
      return;
    }
    body.is_active = cur.is_active;
    mutate(function () { return KT.api.put(API + '/' + encodeURIComponent(cur.id), body); });
  }

  // «В архив» / «Вернуть из архива» — сохранённые значения поставщика + перевёрнутый
  // is_active: несохранённые правки формы архивированием не записываются.
  function toggleActive() {
    var cur = state.current;
    if (!cur) return;
    var body = { is_active: !cur.is_active };
    FIELDS.forEach(function (name) { body[name] = cur[name] || ''; });
    mutate(function () { return KT.api.put(API + '/' + encodeURIComponent(cur.id), body); });
  }

  // «Удалить» (soft-delete-warehouses-suppliers §8.1): общее окно «Удаление» с обязательной
  // причиной; DELETE /api/v1/suppliers/:id {reason}. 400 по причине — под полем окна, окно
  // остаётся; 404 — тост, форма закрывается, список перезапрашивается; 403 — тост «Нет доступа».
  function removeSupplier() {
    var cur = state.current;
    if (!cur || state.busy) return;
    KT.uiDeleteDialog({
      title: 'Удалить «' + cur.name + '»?',
      text: 'Документы поставщика останутся с пометкой «удалён». Поставщика можно восстановить в разделе «Удалённые»',
      submit: function (reason) { return KT.api.delWith(API + '/' + encodeURIComponent(cur.id), { reason: reason }); },
      onOffline: function () { toast('Нет связи с сервером', true); }
    }).then(function (out) {
      if (!out) return; // отмена
      if (out.error) {
        var err = out.error;
        if (err.status === 404) {
          toast('Поставщик не найден', true);
          closeForm();
          loadList();
          return;
        }
        toast(err.status === 403 ? 'Нет доступа' : (err.message || 'Не удалось удалить поставщика'), true);
        return;
      }
      closeForm();
      toast('Удалено. Восстановить можно в разделе «Удалённые»', false);
      loadList();
    });
  }

  // Поиск поставщика по id в общем списке без фильтров (отдельного GET /:id у API нет, §6):
  // страницы по LOOKUP_LIMIT листаются, пока запись не найдена и offset < total.
  // fetchPage(offset) → Promise<{items,total}>; результат — запись либо null.
  function findById(id, fetchPage, offset) {
    offset = offset || 0;
    return fetchPage(offset).then(function (r) {
      var items = r.items || [];
      var s = items.filter(function (x) { return x.id === id; })[0];
      if (s) return s;
      var next = offset + items.length;
      // пустая страница — стоп даже при total > offset (защита от вечного цикла).
      if (items.length === 0 || next >= (r.total || 0)) return null;
      return findById(id, fetchPage, next);
    });
  }

  function lookupPage(offset) {
    return KT.api.get(API + '?limit=' + LOOKUP_LIMIT + '&offset=' + offset);
  }

  // Открыть поставщика по id из адреса: сперва в загруженном списке, иначе — постранично
  // по всему справочнику (включая архивных).
  function openById(id) {
    var found = state.list.filter(function (s) { return s.id === id; })[0];
    if (found) { openForm(found); return Promise.resolve(); }
    return findById(id, lookupPage, 0).then(function (s) {
      if (s) { openForm(s); return; }
      toast('Поставщик не найден', true);
      setUrl(null);
    }).catch(toastFailure);
  }

  // ---------- события ----------
  function rowOf(target) {
    var tr = target.closest ? target.closest('tr[data-id]') : null;
    if (!tr) return null;
    return state.list.filter(function (s) { return s.id === tr.getAttribute('data-id'); })[0] || null;
  }

  function init() {
    if (!$('#sup')) return;
    var timer = null;
    function scheduleReload() {
      clearTimeout(timer);
      timer = setTimeout(function () {
        state.q = $('#sup-q').value.trim();
        state.showArchived = $('#sup-archived').checked;
        loadList();
      }, DEBOUNCE_MS);
    }
    $('#sup-q').addEventListener('input', scheduleReload);
    $('#sup-archived').addEventListener('change', scheduleReload);

    $('#sup-add').addEventListener('click', function () { openForm(null); });
    $('#sup-empty-add').addEventListener('click', function () { openForm(null); });
    $('#sup-cancel').addEventListener('click', closeForm);
    $('#sup-toggle-active').addEventListener('click', toggleActive);
    $('#sup-delete').addEventListener('click', removeSupplier);
    $('#sup-form').addEventListener('submit', function (e) { e.preventDefault(); save(); });

    $('#sup-tbody').addEventListener('click', function (e) {
      var s = rowOf(e.target);
      if (s) openForm(s);
    });
    $('#sup-tbody').addEventListener('keydown', function (e) {
      if (e.key !== 'Enter' && e.key !== ' ') return;
      var s = rowOf(e.target);
      if (!s) return;
      e.preventDefault();
      openForm(s);
    });

    var params = new URLSearchParams(window.location.search);
    var id = params.get('id');
    var isNew = params.get('new') === '1';
    loadList().then(function () {
      if (id) return openById(id);
      if (isNew) openForm(null);
    });
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
