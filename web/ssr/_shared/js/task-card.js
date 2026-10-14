/* KT.taskCard — карточка заказа поверх доски (фаза 6, D25/SD-006/ADR-101/ADR-105).
   Заменяет простую форму фазы 5 (`#task-modal`): шапка (title/client/column/assignee/
   due/paid/description) сохраняется кнопкой; разделы «Оборудование»/«Услуги» заблокированы
   у нового заказа до первого сохранения шапки (SD-006); строки услуг/товаров/оборудования
   сохраняются по каждому change без отдельной кнопки (ADR-105 rowQueue — не более одного
   запроса строки в полёте, накопленные изменения уходят следующим PUT).

   Данные — GET /api/v1/tasks/:id/card (ADR-001, OrderCardResponse); каждая row-мутация
   возвращает {card, warnings} — карточка перечитывается сервером заново (re-fetch-after-
   write), клиент просто заменяет своё состояние ответом.

   Весь пользовательский текст — через esc() (HTML-экранирование), без интерпретации HTML. */
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
  function num(v) { var n = Number(v); return isFinite(n) ? n : 0; }
  function money(v) { return num(v).toFixed(2); }

  // ---------- модуль подключается board.js через init({getData, onChanged}) ----------
  var host = { getData: function () { return { columns: [], clients: [], assignees: [], isAdmin: false, viewerId: '' }; }, onChanged: function () {} };

  // ---------- состояние карточки ----------
  var modal = null;
  var card = null;           // последний известный OrderCardResponse (или null — новый заказ)
  var taskId = null;         // null — режим «новый заказ»
  var defaultColumnId = null;
  var saving = false;        // защита от двойного submit шапки
  var newEquipmentOpen = false;
  var headerDirty = false;   // есть несохранённая правка шапки — renderAll не трогает поля шапки
  // upload (фаза 7) — идущая загрузка вложения {gen, name, percent} либо null; переживает
  // renderAll (секция перерисовывается после любой row-мутации), кнопка заблокирована.
  var upload = null;

  // rowQueue (ADR-105): key -> {inFlight, pending, baseGetter, requestFn, lastBody, lastBodySeq}
  var rowQueue = {};

  // openGeneration/requestSeq/appliedSeq (code review MEDIUM-1) — защита от гонок ответов:
  // (а) карточка A закрыта, открыта B, поздний ответ A не должен применяться к B —
  // openGeneration меняется в open(), ответ с чужим поколением игнорируется; (б) ответы
  // разных ключей (subtask:X, product:Y) могут прийти не в порядке отправки запросов —
  // requestSeq присваивается КАЖДОМУ запросу в момент отправки (beginRequest), ответ с
  // seq не выше уже применённого (appliedSeq) отбрасывается как устаревший, вместо того
  // чтобы откатить более свежее состояние другой строки.
  var openGeneration = 0;
  var requestSeq = 0;
  var appliedSeq = 0;

  function beginRequest() { return { gen: openGeneration, seq: ++requestSeq }; }

  // applyCardIfCurrent — применяет карточку из ответа мутации/loadCard, только если её
  // «билет» всё ещё относится к текущей открытой карточке и не устарел относительно уже
  // применённого ответа. Возвращает true, если применил (вызывающий код тогда может
  // показать warnings/дёрнуть host.onChanged) — иначе ответ молча отброшен.
  function applyCardIfCurrent(resp, ticket) {
    if (!ticket || ticket.gen !== openGeneration) return false;
    if (ticket.seq <= appliedSeq) return false;
    appliedSeq = ticket.seq;
    card = resp;
    renderAll();
    return true;
  }

  function reqKeySubtask(id) { return 'subtask:' + id; }
  function reqKeyProduct(id) { return 'product:' + id; }
  function reqKeyEquipment(id) { return 'equipment:' + id; }
  function reqKeyLog(id) { return 'log:' + id; }

  function enqueueRow(key, patch, baseGetter, requestFn) {
    var q = rowQueue[key];
    if (!q) { q = rowQueue[key] = { inFlight: false, pending: null, lastBody: null, lastBodySeq: 0 }; }
    q.pending = Object.assign(q.pending || {}, patch);
    q.baseGetter = baseGetter; // function(sourceCard) — читает поля ЭТОЙ строки из переданной карточки
    q.requestFn = requestFn;
    setRowBusy(key, true);
    if (!q.inFlight) flushRow(key);
  }

  function flushRow(key) {
    var q = rowQueue[key];
    if (!q || q.pending == null) return;
    // База тела PUT — ПОСЛЕДНЕЕ подтверждённое сервером тело ЭТОЙ строки (q.lastBody), а
    // НЕ текущий глобальный `card` (code review fix-loop 2, п.1): seq-guard в
    // applyCardIfCurrent отбрасывает ответы по порядку ОТПРАВКИ, а не commit'а на сервере —
    // если ответ ЭТОЙ ЖЕ строки был отброшен как «устаревший» (пришёл позже ответа другой
    // строки, применённого раньше), `card` не содержит уже подтверждённое сервером
    // изменение этой строки, и baseGetter(card) собрал бы тело со старым значением, а
    // следующий PUT с полной заменой откатил бы его. lastBody — своё состояние строки,
    // не зависящее от того, применился ли общий снимок карточки к `card`.
    var base = q.lastBody || q.baseGetter(card);
    var body = Object.assign({}, base, q.pending);
    q.pending = null;
    q.inFlight = true;
    // Re-assert busy: предыдущий ответ этой же цепочки уже мог перерисовать строку
    // (applyCard → innerHTML), стерев атрибут aria-busy, выставленный enqueueRow раньше.
    setRowBusy(key, true);
    var ticket = beginRequest();
    q.requestFn(body).then(function (resp) {
      q.inFlight = false;
      // lastBody обновляем из ЭТОГО ответа (его resp.card гарантированно отражает
      // состояние строки сразу после её собственного commit'а на сервере) — независимо от
      // того, применился ли resp.card к глобальному `card` (applyCardIfCurrent мог его
      // отбросить по seq). Сравнение с lastBodySeq — для одного ключа ответы и так приходят
      // строго по порядку отправки (следующий flushRow стартует только после .then этого),
      // но проверка не даёт случайно откатить lastBody более свежим по смыслу, но более
      // старым по seq вызовом (напр. повторный вход в этот .then невозможен, но дёшево).
      if (ticket.seq > q.lastBodySeq) {
        q.lastBody = q.baseGetter(resp.card);
        q.lastBodySeq = ticket.seq;
      }
      if (applyCardIfCurrent(resp.card, ticket)) {
        if (resp.warnings && resp.warnings.length) host.toastWarnings(resp.warnings);
        host.onChanged();
      }
      if (q.pending != null) flushRow(key); else setRowBusy(key, false);
    }).catch(function (err) {
      q.inFlight = false;
      q.pending = null; // состояние сомнительно — не повторяем автоматически
      setRowBusy(key, false);
      if (ticket.gen === openGeneration) onRowError(key, err); // карточка уже закрыта/сменилась — ошибка чужая
    });
  }

  function setRowBusy(key, busy) {
    $$('[data-row-key="' + cssesc(key) + '"]').forEach(function (el) {
      el.setAttribute('aria-busy', busy ? 'true' : 'false');
    });
  }
  function cssesc(s) { return window.CSS && CSS.escape ? CSS.escape(s) : s.replace(/["\\]/g, '\\$&'); }

  // onRowError — 404 (родитель/строка удалена параллельно) → «Строка уже удалена» + перечитать
  // карточку; 400 с полями → показать текст у поля (card-warning), если у строки есть такой
  // слот, иначе (409/403/прочее) — общий KT.uiAlert + перечитать карточку (откат к серверным
  // значениям).
  // fieldErrorText — текст ошибки поля строки (код сервера).
  function fieldErrorText(f) {
    return f.message;
  }

  function onRowError(key, err) {
    var code = err && err.code;
    if (code === 'SUBTASK_NOT_FOUND' || code === 'SUBTASK_PRODUCT_NOT_FOUND' || code === 'SUBTASK_LOG_NOT_FOUND' || code === 'TASK_EQUIPMENT_NOT_FOUND') {
      KT.uiAlert('Строка уже удалена', 'Не удалось сохранить').then(function () { loadCard(taskId); });
      return;
    }
    if (err && err.status === 400 && err.fields && err.fields.length) {
      var rowEl = document.querySelector('[data-row-key="' + cssesc(key) + '"]');
      // key вида "subtask:X" — rowEl это ВСЯ карточка услуги (со вложенными товарами):
      // обычный querySelector('[data-testid="card-warning"]') нашёл бы слот ПЕРВОГО
      // товара внутри неё (LOW code review) — у услуги свой отдельный слот-testid.
      var warnSelector = key.indexOf('subtask:') === 0 ? '[data-testid="card-subtask-warning"]' : '[data-testid="card-warning"]';
      var warn = rowEl && rowEl.querySelector(warnSelector);
      if (warn) {
        warn.textContent = err.fields.map(fieldErrorText).join('; ');
        warn.classList.remove('d-none');
        loadCard(taskId); // откат значений к последнему состоянию сервера
        return;
      }
    }
    if (err && err.fields && err.fields.length) {
      KT.uiAlert(err.fields.map(fieldErrorText).join('; '), 'Не удалось сохранить').then(function () { loadCard(taskId); });
      return;
    }
    KT.uiAlert((err && err.message) || 'Не удалось сохранить изменение', 'Ошибка').then(function () { loadCard(taskId); });
  }

  // ---------- открытие/закрытие ----------
  function open(id, defColId) {
    taskId = id || null;
    defaultColumnId = defColId || null;
    card = null;
    rowQueue = {};
    newEquipmentOpen = false;
    eqHistory = {};
    headerDirty = false;
    upload = null;
    openGeneration++; // новое поколение — ответы предыдущей карточки больше не применяются
    requestSeq = 0;
    appliedSeq = 0;
    renderShell();
    if (!modal) modal = new bootstrap.Modal($('#task-modal'));
    modal.show();
    if (taskId) loadCard(taskId);
  }

  function close() { if (modal) modal.hide(); }

  function loadCard(id) {
    var ticket = beginRequest();
    return KT.api.get('/api/v1/tasks/' + id + '/card').then(function (resp) {
      // Тот же guard, что и у applyCardIfCurrent (code review fix-loop 2, п.2): loadCard
      // тоже может прийти ПОЗЖЕ, чем уже применённый ответ row-мутации (напр. вызван из
      // onRowError) — без проверки seq он откатил бы карточку на устаревший снимок.
      if (ticket.gen !== openGeneration || ticket.seq <= appliedSeq) return;
      appliedSeq = ticket.seq;
      headerDirty = false; // полный авторитетный ответ сервера — сбрасываем несохранённую правку
      card = resp;
      taskId = resp.task.id;
      renderAll();
    }).catch(function (err) {
      if (ticket.gen !== openGeneration) return;
      if (err && err.status === 404) {
        KT.uiAlert('Заказ не найден', 'Ошибка').then(close);
        return;
      }
      KT.uiAlert((err && err.message) || 'Не удалось загрузить карточку', 'Ошибка').then(close);
    });
  }

  // refetchCard — перечитать открытую карточку после отказа мутации (состояние на сервере
  // изменилось мимо этой вкладки). В отличие от loadCard не сбрасывает несохранённую правку
  // шапки и молчит при ошибке сети — пользователь уже видит сообщение об отказе.
  function refetchCard() {
    var ticket = beginRequest();
    return KT.api.get('/api/v1/tasks/' + taskId + '/card').then(function (resp) {
      applyCardIfCurrent(resp, ticket);
    }).catch(function () { /* карточка обновится при следующем действии */ });
  }

  // applyCard — ответ row-мутации: карточка перечитана сервером заново, локальное
  // состояние просто заменяется (re-fetch-after-write); фокус активного поля не теряется —
  // rerenderPreserveFocus в render* хранит значение и позицию курсора активного поля.
  // ticket (beginRequest()) — см. applyCardIfCurrent: без него ответ применяется безусловно,
  // поэтому обработчики row-мутаций ДОЛЖНЫ передавать сюда билет, полученный ДО запроса.
  function applyCard(resp, ticket) {
    return applyCardIfCurrent(resp, ticket);
  }

  // ---------- каркас модалки (шапка + контейнеры секций) ----------
  function renderShell() {
    var data = host.getData();
    $('#task-modal-title').textContent = taskId ? ('Заказ #' + taskId) : 'Новый заказ';
    $('#task-error').classList.add('d-none');
    $('#task-error').textContent = '';

    fillHeaderOptions(data);
    $('#task-title').value = '';
    $('#task-client').value = '';
    clientMarks(false);
    setNewOrderMode(!taskId);
    resetCar();
    $('#task-column').value = defaultColumnId || (data.columns[0] && data.columns[0].id) || '';
    $('#task-assignee').value = '';
    $('#task-due').value = '';
    $('#task-description').value = '';
    var paid = $('#task-paid');
    paid.checked = false;
    paid.disabled = !data.isAdmin;
    ['#task-client', '#task-column', '#task-assignee'].forEach(function (sel) { var el = $(sel); if (el && el._cbSync) el._cbSync(); });

    var canDelete = false; // новый заказ — удалять нечего
    var delBtn = $('#task-delete-btn');
    delBtn.classList.toggle('d-none', !canDelete);
    delBtn.removeAttribute('data-testid');

    $('#task-save-btn').disabled = false;
    saving = false;

    $('#card-print-actions').innerHTML = '';
    renderLockedSections();
    $('#card-total').innerHTML = '';
  }

  function fillHeaderOptions(data) {
    var clientSel = $('#task-client');
    clientSel.innerHTML = '<option value="">Выберите клиента</option>' + data.clients.map(function (c) {
      return '<option value="' + esc(c.id) + '">' + esc(c.company_name) + '</option>';
    }).join('');
    clientSel._cbFetchOptions = function (q) {
      return KT.api.get('/api/v1/clients?q=' + encodeURIComponent(q) + '&limit=50').then(function (r) {
        return r.items.map(function (c) { return { value: c.id, text: c.company_name }; });
      });
    };

    var colSel = $('#task-column');
    colSel.innerHTML = data.columns.map(function (c) { return '<option value="' + esc(c.id) + '">' + esc(c.name) + '</option>'; }).join('');

    var aSel = $('#task-assignee');
    aSel.innerHTML = '<option value="">Не назначен</option>' + data.assignees.map(function (a) {
      return '<option value="' + esc(a.id) + '">' + esc(a.name) + '</option>';
    }).join('');
  }

  // deletedBadge — серый бейдж «удалён» (soft-delete-clients §8.3, UI-конвенции DS-033).
  function deletedBadge(testid) {
    return ' <span class="badge text-bg-secondary" data-testid="' + testid + '">удалён</span>';
  }

  // clientMarks — пометки у поля «Клиент»: бейдж «удалён» (клиент заказа в удалённых) и сброс
  // ошибки под полем.
  function clientMarks(deleted) {
    var b = $('#task-client-deleted');
    if (b) b.classList.toggle('d-none', !deleted);
    var e = $('#task-client-error');
    if (e) { e.textContent = ''; e.classList.add('d-none'); }
  }

  function showClientError(text) {
    var e = $('#task-client-error');
    if (!e) { showHeaderError(text); return; }
    e.textContent = text;
    e.classList.remove('d-none');
  }

  // ensureClientOption — клиент заказа мягко удалён: в списке выбора (GET /api/v1/clients) его
  // нет, но у действующего заказа он остаётся — отдельным пунктом, чтобы шапка сохранялась без
  // смены клиента (soft-delete-clients §8.3 п.3).
  function ensureClientOption(t) {
    var sel = $('#task-client');
    if ([].some.call(sel.options, function (o) { return o.value === t.client_id; })) return;
    var o = document.createElement('option');
    o.value = t.client_id;
    o.textContent = t.client_name;
    sel.appendChild(o);
  }

  function renderLockedSections() {
    $('#card-equipment').innerHTML = '<p class="text-body-secondary small" data-testid="card-equipment-locked">Сохраните заказ, чтобы добавить оборудование</p>';
    $('#card-subtasks').innerHTML = '<p class="text-body-secondary small" data-testid="card-subtasks-locked">Сохраните заказ, чтобы добавить услуги</p>';
    $('#card-attachments').innerHTML = '<p class="text-body-secondary small" data-testid="card-attachments-locked">Сохраните заказ, чтобы добавить фото, видео или PDF</p>';
  }

  // ---------- полный рендер (после loadCard/applyCard) ----------
  function renderAll() {
    if (!card) return;
    var data = host.getData();
    var t = card.task;
    $('#task-modal-title').textContent = 'Заказ #' + t.id;

    // headerDirty (code review MEDIUM-2): renderAll вызывается ПОСЛЕ КАЖДОЙ row-мутации
    // (applyCard) — если шапка правится, а между вводом и «Сохранить» успевает пройти
    // хоть одна row-мутация (смена статуса услуги и т.п.), обычный re-render молча
    // затирал бы несохранённые поля шапки значениями с сервера (activeElement-guard не
    // спасает: поле уже могло потерять фокус). Пока шапка «грязная» (до loadCard —
    // либо новой карточки, либо успешного «Сохранить»), поля шапки не трогаем вовсе.
    if (!headerDirty) {
      setNewOrderMode(false);
      fillHeaderOptions(data);
      ensureClientOption(t);
      clientMarks(!!t.client_is_deleted);
      if (document.activeElement !== $('#task-title')) $('#task-title').value = t.title;
      if (document.activeElement !== $('#task-client')) $('#task-client').value = t.client_id;
      if (document.activeElement !== $('#task-column')) $('#task-column').value = t.column_id;
      if (document.activeElement !== $('#task-assignee')) $('#task-assignee').value = t.assignee_id || '';
      if (document.activeElement !== $('#task-due')) $('#task-due').value = t.due_date || '';
      if (document.activeElement !== $('#task-description')) $('#task-description').value = descriptionHtmlToText(t.description_html);
      var paid = $('#task-paid');
      paid.checked = !!t.is_paid;
      paid.disabled = !data.isAdmin;
      ['#task-client', '#task-column', '#task-assignee'].forEach(function (sel) { var el = $(sel); if (el && el._cbSync) el._cbSync(); });
    }

    var canDelete = !!data.isAdmin;
    var delBtn = $('#task-delete-btn');
    delBtn.classList.toggle('d-none', !canDelete);
    if (canDelete) delBtn.setAttribute('data-testid', 'task-delete'); else delBtn.removeAttribute('data-testid');

    $('#task-save-btn').disabled = false;
    saving = false;

    renderPrintActions(t.id);
    renderEquipment();
    renderSubtasks();
    renderAttachments();
    renderTotal();
  }

  function descriptionHtmlToText(html) {
    if (!html) return '';
    var doc = new DOMParser().parseFromString(html, 'text/html');
    return [].slice.call(doc.body.children).map(function (p) {
      return [].slice.call(p.childNodes).map(function (n) {
        return n.nodeType === 3 ? n.textContent : (n.nodeName === 'BR' ? '\n' : n.textContent);
      }).join('');
    }).join('\n\n');
  }

  function renderPrintActions(id) {
    $('#card-print-actions').innerHTML =
      '<a class="btn btn-outline-secondary btn-sm" target="_blank" rel="noopener" href="/tasks/' + id + '/print/intake" data-testid="card-print-intake">Акт приёма</a>' +
      '<a class="btn btn-outline-secondary btn-sm" target="_blank" rel="noopener" href="/tasks/' + id + '/print/handover" data-testid="card-print-handover">Акт выдачи</a>';
  }

  // ---------- фокус: сохранить значение/курсор активного поля секции при полном ре-рендере ----------
  function rerenderPreserveFocus(container, html) {
    var af = document.activeElement;
    var info = null;
    if (af && container.contains(af) && af.hasAttribute && af.hasAttribute('data-field')) {
      var rowEl = af.closest('[data-row-key]');
      info = {
        field: af.getAttribute('data-field'),
        rowKey: rowEl ? rowEl.getAttribute('data-row-key') : null,
        value: af.value,
        selStart: (typeof af.selectionStart === 'number') ? af.selectionStart : null,
        selEnd: (typeof af.selectionEnd === 'number') ? af.selectionEnd : null
      };
    }
    container.innerHTML = html;
    if (info) {
      var sel = (info.rowKey ? '[data-row-key="' + cssesc(info.rowKey) + '"] ' : '') + '[data-field="' + cssesc(info.field) + '"]';
      var el = container.querySelector(sel);
      if (el) {
        el.value = info.value;
        el.focus();
        if (info.selStart != null && el.setSelectionRange) { try { el.setSelectionRange(info.selStart, info.selEnd); } catch (e) { /* select/date — не поддерживают selectionRange */ } }
      }
    }
  }

  // ==================================================================
  // Оборудование заказа (AC-004/AC-005/AC-006)
  // ==================================================================

  var clientEquipmentCache = null; // {clientId, items} — эталон опций «Добавить оборудование клиента»
  // eqHistory[equipmentId] = {open, loading, items, error} — раскрытая «История работ»
  // единицы (GET /api/v1/equipment/:id/history); переживает перерисовки блока, сбрасывается
  // при открытии другой карточки.
  var eqHistory = {};

  // renderEquipment — доступный клиенту список оборудования нужен ДО рендера <select>
  // (не после, отдельным запросом внутри wireEquipment): иначе окно между вставкой пустого
  // select в DOM и приходом ответа даёт combobox.js открыть список без опций (гонка,
  // combobox.js не перерисовывает уже открытый список при появлении новых <option>).
  //
  // Клиент заказа мягко удалён (soft-delete-clients §8.3, DS-026): его справочник оборудования
  // для обычных адресов не существует (404), а действующих единиц у него нет — запрос не
  // выполняется, блок рисуется только по данным карточки.
  function renderEquipment() {
    var clientId = card.task.client_id;
    if (card.task.client_is_deleted) {
      renderEquipmentWith([]);
      return;
    }
    if (clientEquipmentCache && clientEquipmentCache.clientId === clientId) {
      renderEquipmentWith(clientEquipmentCache.items);
      return;
    }
    KT.api.get('/api/v1/clients/' + clientId + '/equipment?limit=200').then(function (r) {
      clientEquipmentCache = { clientId: clientId, items: r.items };
      renderEquipmentWith(r.items);
    }).catch(function () { renderEquipmentWith([]); });
  }

  function renderEquipmentWith(clientEquipment) {
    var container = $('#card-equipment');
    if (!container) return; // модалка уже закрыта раньше ответа сети
    var already = {}; card.equipment.forEach(function (e) { already[e.equipment_id] = true; });
    var addOptions = clientEquipment.filter(function (e) { return !already[e.id]; });
    // клиент заказа удалён — добавлять ему оборудование нельзя (POST /clients/:id/equipment → 404,
    // DS-026): вместо выбора и формы новой единицы — пояснение.
    var adding = card.task.client_is_deleted
      ? '<p class="text-body-secondary small mb-0" data-testid="card-equipment-client-deleted">Клиент заказа удалён — добавить оборудование нельзя</p>'
      : equipmentAddHtml(addOptions) +
        (newEquipmentOpen ? newEquipmentFormHtml() : '<button type="button" class="btn btn-outline-secondary btn-sm mt-2" data-testid="card-equipment-new">+ Новое оборудование</button>');
    var html = '<div>' +
      card.equipment.map(equipmentRowHtml).join('') +
      adding +
      '</div>';
    rerenderPreserveFocus(container, html);
    wireEquipment(container);
  }

  function equipmentRowHtml(eq) {
    var key = reqKeyEquipment(eq.equipment_id);
    var history = (eq.history || []).map(function (h) {
      return '<div class="small text-body-secondary" data-testid="card-equipment-history">Уже был в ремонте: #' + h.task_id + ' «' + esc(h.title) + '» от ' + fmtDate(h.created_at) + '</div>';
    }).join('');
    return '' +
      '<div class="card mb-2" data-testid="card-equipment-row" data-row-key="' + key + '" data-equipment-id="' + esc(eq.equipment_id) + '">' +
      '  <div class="card-body py-2">' +
      '    <div class="d-flex justify-content-between align-items-start gap-2">' +
      '      <div class="card-eq-title">' + plateBadge(eq.plate_number) + '<strong>' + esc(eq.type_name) + '</strong> ' + esc(eq.brand) + ' ' + esc(eq.model) + (eq.serial ? ' · SN ' + esc(eq.serial) : '') +
      // единица мягко удалена (soft-delete-clients §8.3 п.3) — в заказе остаётся с бейджем
      (eq.is_deleted ? deletedBadge('card-equipment-deleted') : '') + '</div>' +
      '      <button type="button" class="btn btn-outline-danger btn-sm" data-testid="card-equipment-remove" data-equipment-id="' + esc(eq.equipment_id) + '">Убрать</button>' +
      '    </div>' +
      '    <div class="row g-2 mt-1">' +
      '      <div class="col-6"><label class="form-label small mb-0">Комплектность</label>' +
      '        <input class="form-control form-control-sm" data-field="completeness" data-row-key="' + key + '" data-testid="card-equipment-completeness" maxlength="1000" value="' + esc(eq.completeness) + '"></div>' +
      '      <div class="col-6"><label class="form-label small mb-0">Состояние</label>' +
      '        <input class="form-control form-control-sm" data-field="condition" data-row-key="' + key + '" data-testid="card-equipment-condition" maxlength="1000" value="' + esc(eq.condition) + '"></div>' +
      '    </div>' +
      (history ? '<div class="mt-1">' + history + '</div>' : '') +
      // история удалённой единицы для обычных адресов не существует (404, DS-026) — без кнопки
      (eq.is_deleted ? '' : equipmentHistoryHtml(eq.equipment_id)) +
      '  </div>' +
      '</div>';
  }

  var SERVICE_STATUS = { 'new': ['Новая', 'text-bg-secondary'], in_progress: ['В работе', 'text-bg-primary'], done: ['Выполнена', 'text-bg-success'] };

  // Дата и время — только через KT.time (пояс пользователя, не браузера; DS-032).
  function fmtDateTime(iso) { return KT.time.fmtDateTime(iso); }

  // equipmentHistoryHtml — кнопка «История работ» + раскрытая панель: другие заказы с этой
  // единицей (текущий исключён — его работы и так в карточке) с услугами по ней, товарами
  // и журналом. Для admin — ссылка на полную страницу /equipment/:id/history.
  function equipmentHistoryHtml(eqId) {
    var st = eqHistory[eqId] || {};
    var btn = '<button type="button" class="btn btn-link btn-sm p-0 mt-1" data-testid="card-equipment-history-toggle" data-equipment-id="' + esc(eqId) + '" aria-expanded="' + (st.open ? 'true' : 'false') + '">' +
      (st.open ? '▾' : '▸') + ' История работ</button>';
    if (!st.open) return '<div>' + btn + '</div>';
    var body;
    if (st.loading) body = '<div class="small text-body-secondary">Загрузка…</div>';
    else if (st.error) body = '<div class="small text-danger">' + esc(st.error) + '</div>';
    else {
      var others = (st.items || []).filter(function (o) { return String(o.task_id) !== String(taskId); });
      body = others.length ? others.map(historyOrderHtml).join('')
        : '<div class="small text-body-secondary" data-testid="card-equipment-history-empty">Других работ по этому оборудованию не было</div>';
      if (host.getData().isAdmin) {
        body += '<a class="small" href="/equipment/' + esc(eqId) + '/history" target="_blank" rel="noopener" data-testid="card-equipment-history-page">Открыть полную историю ↗</a>';
      }
    }
    return '<div>' + btn + '<div class="border-start ps-2 mt-1" data-testid="card-equipment-history-panel">' + body + '</div></div>';
  }

  function historyOrderHtml(o) {
    var intake = o.accepted && (o.completeness || o.condition)
      ? '<div class="text-body-secondary">Принято: ' + esc([o.completeness && ('комплектность — ' + o.completeness), o.condition && ('состояние — ' + o.condition)].filter(Boolean).join('; ')) + '</div>' : '';
    var services = (o.services || []).map(function (s) {
      var stl = SERVICE_STATUS[s.status] || SERVICE_STATUS['new'];
      var products = (s.products || []).map(function (p) {
        return '<div class="ps-2">• ' + esc(p.name) + ' — ' + esc(String(p.qty)) + (p.unit ? ' ' + esc(p.unit) : '') + (p.price != null ? ', ' + esc(String(p.price)) + ' ₽' : '') + '</div>';
      }).join('');
      var logs = (s.logs || []).map(function (l) {
        return '<div class="ps-2 text-body-secondary">' + fmtDateTime(l.created_at) + ' · ' + esc(l.author_name) + ': <span class="text-body" style="white-space:pre-wrap">' + esc(l.text) + '</span></div>';
      }).join('');
      return '<div class="mt-1" data-testid="card-equipment-history-service">' +
        '<span class="fw-semibold">' + esc(s.name) + '</span> <span class="badge ' + stl[1] + '">' + stl[0] + '</span>' +
        (s.assignee_name ? ' <span class="text-body-secondary">' + esc(s.assignee_name) + '</span>' : '') +
        (s.price != null ? ' · ' + esc(String(s.price)) + ' ₽' : '') + products + logs + '</div>';
    }).join('');
    return '<div class="small mb-2" data-testid="card-equipment-history-order">' +
      '<a href="/?task=' + o.task_id + '" data-testid="card-equipment-history-link">#' + o.task_id + ' ' + esc(o.title) + '</a>' +
      ' <span class="badge" style="background-color:' + esc(o.column_color) + '">' + esc(o.column_name) + '</span>' +
      ' <span class="text-body-secondary">от ' + fmtDate(o.created_at) + '</span>' +
      intake + (services || '<div class="text-body-secondary">Услуг по этому оборудованию не было</div>') + '</div>';
  }

  function toggleEquipmentHistory(eqId) {
    var st = eqHistory[eqId] = eqHistory[eqId] || {};
    st.open = !st.open;
    if (st.open && !st.items) {
      st.loading = true; st.error = '';
      var gen = openGeneration;
      KT.api.get('/api/v1/equipment/' + encodeURIComponent(eqId) + '/history').then(function (r) {
        st.items = r.items;
      }).catch(function (err) {
        st.error = (err && err.message) || 'Не удалось загрузить историю';
      }).then(function () {
        st.loading = false;
        if (gen === openGeneration) renderEquipment();
      });
    }
    renderEquipment();
  }

  // plateBadge — госномер автомобиля бейджем перед названием единицы ('' — госномера нет).
  function plateBadge(plate) {
    return plate ? '<span class="badge text-bg-light border me-1 text-nowrap" data-testid="card-equipment-plate">' + esc(plate) + '</span> ' : '';
  }

  // PLATE_ERROR_TEXT — коды ошибок поля plate_number (details.fields[].message) → текст.
  var PLATE_ERROR_TEXT = {
    too_long: 'Госномер: не больше 20 символов',
    invalid_format: 'Госномер: только буквы, цифры, пробел и дефис',
    invalid_text: 'Госномер: недопустимые символы'
  };

  function equipmentAddHtml(options) {
    return '<select class="form-select form-select-sm" data-testid="card-equipment-add"><option value="">+ Добавить оборудование клиента…</option>' +
      (options || []).map(function (e) {
        return '<option value="' + esc(e.id) + '">' + (e.plate_number ? esc(e.plate_number) + ' — ' : '') + esc(e.type_name) + ' ' + esc(e.brand) + ' ' + esc(e.model) + (e.serial ? ' · ' + esc(e.serial) : '') + '</option>';
      }).join('') + '</select>';
  }

  function newEquipmentFormHtml() {
    return '' +
      '<div class="card mt-2" data-testid="card-equipment-new-form">' +
      '  <div class="card-body py-2">' +
      '    <div class="row g-2">' +
      '      <div class="col-12"><label class="form-label small mb-0">Тип *</label>' +
      '        <select class="form-select form-select-sm" data-testid="card-equipment-new-type"><option value="">Выберите тип</option></select></div>' +
      '      <div class="col-6"><label class="form-label small mb-0">Госномер</label><input class="form-control form-control-sm text-uppercase" maxlength="20" placeholder="А123ВС 777" data-testid="card-equipment-new-plate"></div>' +
      '      <div class="col-6"><label class="form-label small mb-0">Бренд</label><input class="form-control form-control-sm" data-testid="card-equipment-new-brand"></div>' +
      '      <div class="col-6"><label class="form-label small mb-0">Модель *</label><input class="form-control form-control-sm" data-testid="card-equipment-new-model"></div>' +
      '      <div class="col-6"><label class="form-label small mb-0">Серийный №</label><input class="form-control form-control-sm" data-testid="card-equipment-new-serial"></div>' +
      '    </div>' +
      '    <div class="alert alert-danger d-none mt-2 py-1 px-2 small" data-testid="card-equipment-new-error"></div>' +
      '    <div class="d-flex gap-2 mt-2">' +
      '      <button type="button" class="btn btn-primary btn-sm" data-testid="card-equipment-new-save">Добавить</button>' +
      '      <button type="button" class="btn btn-outline-secondary btn-sm" data-testid="card-equipment-new-cancel">Отмена</button>' +
      '    </div>' +
      '  </div>' +
      '</div>';
  }

  function fmtDate(iso) { return KT.time.fmtDate(iso); }

  function wireEquipment(container) {
    $$('[data-field="completeness"], [data-field="condition"]', container).forEach(function (input) {
      input.addEventListener('change', function () {
        var eqId = input.closest('[data-testid="card-equipment-row"]').dataset.equipmentId;
        var eq = card.equipment.filter(function (e) { return e.equipment_id === eqId; })[0];
        var field = input.dataset.field;
        var patch = {}; patch[field] = input.value;
        enqueueRow(reqKeyEquipment(eqId), patch, function (source) {
          var cur = (source || card).equipment.filter(function (e) { return e.equipment_id === eqId; })[0] || eq;
          return { completeness: cur.completeness, condition: cur.condition };
        }, function (body) {
          return KT.api.put('/api/v1/tasks/' + taskId + '/equipment/' + eqId, body);
        });
      });
    });

    $$('[data-testid="card-equipment-history-toggle"]', container).forEach(function (btn) {
      btn.addEventListener('click', function () { toggleEquipmentHistory(btn.dataset.equipmentId); });
    });

    $$('[data-testid="card-equipment-remove"]', container).forEach(function (btn) {
      btn.addEventListener('click', function () {
        var eqId = btn.dataset.equipmentId;
        KT.uiConfirm('Убрать оборудование из заказа?').then(function (ok) {
          if (!ok) return;
          var ticket = beginRequest();
          KT.api.del('/api/v1/tasks/' + taskId + '/equipment/' + eqId).then(function (resp) {
            if (!applyCard(resp.card, ticket)) return;
            if (resp.warnings && resp.warnings.length) toastWarningsHost(resp.warnings);
            host.onChanged();
          }).catch(function (err) {
            KT.uiAlert((err && err.message) || 'Не удалось убрать оборудование', 'Ошибка');
            // 409: единица занята услугой — карточка устарела; перечитываем, чтобы показать
            // актуальное состояние.
            if (err && err.status === 409) refetchCard();
          });
        });
      });
    });

    var addSel = $('[data-testid="card-equipment-add"]', container);
    if (addSel) {
      addSel.addEventListener('change', function () {
        var eqId = addSel.value;
        if (!eqId) return;
        var ticket = beginRequest();
        KT.api.post('/api/v1/tasks/' + taskId + '/equipment', { equipment_id: eqId, completeness: '', condition: '' }).then(function (resp) {
          if (!applyCard(resp.card, ticket)) return;
          host.onChanged();
        }).catch(function (err) {
          // единицу удалили, пока карточка открыта (soft-delete-clients §8.3): ошибка в шапке
          var gone = err && err.status === 400 && (err.fields || []).some(function (f) { return f.name === 'equipment_id' && f.message === 'not_found'; });
          if (gone) {
            clientEquipmentCache = null;
            showHeaderError('Оборудование не найдено');
            renderEquipment();
            return;
          }
          KT.uiAlert((err && err.message) || 'Не удалось добавить оборудование', 'Ошибка');
        });
      });
    }

    var newBtn = $('[data-testid="card-equipment-new"]', container);
    if (newBtn) newBtn.addEventListener('click', function () { newEquipmentOpen = true; renderEquipment(); });
    var cancelBtn = $('[data-testid="card-equipment-new-cancel"]', container);
    if (cancelBtn) cancelBtn.addEventListener('click', function () { newEquipmentOpen = false; renderEquipment(); });

    // Список типов оборудования может быть больше «первой страницы» — как каталог услуг/
    // товаров, тип ищем remote-поиском по вводу (а не одной статичной подгрузкой limit=200),
    // иначе только что созданный тип может не попасть в список (эталон — combobox.js
    // remoteClientSearch в board.js).
    var typeSel = $('[data-testid="card-equipment-new-type"]', container);
    if (typeSel) {
      typeSel._cbFetchOptions = function (q) {
        return KT.api.get('/api/v1/equipment-types?q=' + encodeURIComponent(q) + '&limit=50').then(function (r) {
          return r.items.map(function (t) { return { value: t.id, text: t.name }; });
        });
      };
    }
    var saveBtn = $('[data-testid="card-equipment-new-save"]', container);
    if (saveBtn) saveBtn.addEventListener('click', function () {
      // typeSel — та же ссылка на <select>, захваченная ДО того, как combobox.js перенёс
      // data-testid на видимую кнопку (js/combobox.js enhanceSelect) — повторный поиск по
      // testid здесь нашёл бы уже саму кнопку (HTMLButtonElement.value всегда '', баг
      // найден вживую: клик «Добавить» без выбранного типа, хотя тип был выбран в UI).
      var typeId = typeSel ? typeSel.value : '';
      var brand = $('[data-testid="card-equipment-new-brand"]', container).value.trim();
      var model = $('[data-testid="card-equipment-new-model"]', container).value.trim();
      var serial = $('[data-testid="card-equipment-new-serial"]', container).value.trim();
      var plate = $('[data-testid="card-equipment-new-plate"]', container).value.trim();
      var errEl = $('[data-testid="card-equipment-new-error"]', container);
      if (!typeId || !model) {
        errEl.textContent = 'Укажите тип и модель';
        errEl.classList.remove('d-none');
        return;
      }
      errEl.classList.add('d-none');
      var ticket = beginRequest();
      KT.api.post('/api/v1/clients/' + card.task.client_id + '/equipment', { type_id: typeId, plate_number: plate, brand: brand, model: model, sku: '', serial: serial, specs: [], note: '' })
        .then(function (equipment) {
          return KT.api.post('/api/v1/tasks/' + taskId + '/equipment', { equipment_id: equipment.id, completeness: '', condition: '' });
        }).then(function (resp) {
          newEquipmentOpen = false;
          clientEquipmentCache = null; // новая единица появилась у клиента — перечитать список
          if (!applyCard(resp.card, ticket)) return;
          host.onChanged();
        }).catch(function (err) {
          errEl.textContent = (err && err.fields && err.fields.length) ? err.fields.map(function (f) { return (f.name === 'plate_number' && PLATE_ERROR_TEXT[f.message]) || f.message; }).join('; ') : ((err && err.message) || 'Не удалось создать оборудование');
          errEl.classList.remove('d-none');
        });
    });
  }

  function toastWarningsHost(w) { host.toastWarnings(w); }

  // ==================================================================
  // Услуги/товары/журнал (AC-007/AC-008/AC-009/AC-010/AC-011/AC-012/AC-013)
  // ==================================================================

  var SUBTASK_STATUSES = [{ v: 'new', t: 'Новая' }, { v: 'in_progress', t: 'В работе' }, { v: 'done', t: 'Выполнена' }];

  // Остаток товара — card.stock по product_id.
  function stockOf(productId) {
    return (card.stock || []).filter(function (s) { return s.product_id === productId; })[0] || { stock: 0, min_stock: 0, task_reserved: 0 };
  }
  // stockHint — AC-009: avail = stock + task_reserved (этого заказа), need = Σ qty ЭТОГО
  // товара по ВСЕМ услугам карточки (включая текущую строку).
  function stockHint(productId) {
    var st = stockOf(productId);
    var avail = num(st.stock) + num(st.task_reserved);
    var need = 0;
    card.subtasks.forEach(function (s) {
      s.products.forEach(function (p) { if (p.product_id === productId) need += num(p.qty); });
    });
    var rest = avail - need;
    if (rest < 0) return 'в наличии только ' + avail;
    if (rest < num(st.min_stock)) return 'останется ' + rest + ' — минимум ' + num(st.min_stock);
    return 'в наличии ' + avail;
  }

  function renderSubtasks() {
    var container = $('#card-subtasks');
    var locked = card.column.is_final;
    var html = '<div>' +
      card.subtasks.map(subtaskRowHtml).join('') +
      (locked
        ? '<button type="button" class="btn btn-outline-secondary btn-sm mt-2" data-testid="card-subtask-add" disabled title="Заказ в финальной колонке: нельзя добавить услугу">+ Добавить услугу</button>'
        : '<select class="form-select form-select-sm mt-2" data-testid="card-subtask-add"><option value="">+ Добавить услугу…</option></select>') +
      '</div>';
    rerenderPreserveFocus(container, html);
    wireSubtasks(container);
  }

  function subtaskRowHtml(sub) {
    var key = reqKeySubtask(sub.id);
    var payoutId = 'card-subtask-executor-paid-' + sub.id;
    // мягко удалённую единицу услуге заново не назначить (soft-delete-clients §8.3 п.5): в
    // списке она остаётся, только если уже назначена этой услуге, — с бейджем у подписи поля.
    var subEquipment = card.equipment.filter(function (e) { return e.equipment_id === sub.equipment_id; })[0];
    var subEquipmentDeleted = !!(subEquipment && subEquipment.is_deleted);
    var equipmentOptions = '<option value="">Без оборудования</option>' + card.equipment.filter(function (e) {
      return !e.is_deleted || e.equipment_id === sub.equipment_id;
    }).map(function (e) {
      return '<option value="' + esc(e.equipment_id) + '"' + (sub.equipment_id === e.equipment_id ? ' selected' : '') + '>' + (e.plate_number ? esc(e.plate_number) + ' — ' : '') + esc(e.type_name) + ' ' + esc(e.brand) + ' ' + esc(e.model) + '</option>';
    }).join('');
    var assigneeOptions = '<option value="">Не назначен</option>' + host.getData().assignees.map(function (a) {
      return '<option value="' + esc(a.id) + '"' + (sub.assignee_id === a.id ? ' selected' : '') + '>' + esc(a.name) + (a.is_active === false ? ' (неактивен)' : '') + '</option>';
    }).join('');
    return '' +
      '<div class="card mb-2" data-testid="card-subtask-row" data-row-key="' + key + '" data-subtask-id="' + esc(sub.id) + '">' +
      '  <div class="card-body py-2">' +
      '    <div class="d-flex justify-content-between align-items-start gap-2 flex-wrap">' +
      // услуга удалена из каталога (soft-delete-catalog §8.3 п.1) — серый бейдж «удалён»;
      // услуга заказа правится как обычно
      '      <strong>' + esc(sub.service_name) + (sub.service_is_deleted ? deletedBadge('card-subtask-service-deleted') : '') + '</strong>' +
      '      <button type="button" class="btn btn-outline-danger btn-sm" data-testid="card-subtask-delete" data-subtask-id="' + esc(sub.id) + '">Удалить</button>' +
      '    </div>' +
      '    <div class="row g-2 mt-1">' +
      '      <div class="col-6 col-md-3"><label class="form-label small mb-0">Статус</label>' +
      '        <select class="form-select form-select-sm" data-field="status" data-row-key="' + key + '" data-testid="card-subtask-status">' +
             SUBTASK_STATUSES.map(function (s) { return '<option value="' + s.v + '"' + (sub.status === s.v ? ' selected' : '') + '>' + s.t + '</option>'; }).join('') +
      '        </select></div>' +
      '      <div class="col-6 col-md-3"><label class="form-label small mb-0">Исполнитель</label>' +
      '        <select class="form-select form-select-sm" data-field="assignee_id" data-row-key="' + key + '" data-testid="card-subtask-assignee">' + assigneeOptions + '</select></div>' +
      '      <div class="col-6 col-md-3"><label class="form-label small mb-0">Срок</label>' +
      '        <input type="date" class="form-control form-control-sm" data-field="due_date" data-row-key="' + key + '" data-testid="card-subtask-due" value="' + esc(sub.due_date || '') + '"></div>' +
      '      <div class="col-6 col-md-3"><label class="form-label small mb-0">Цена (пусто — по факту)</label>' +
      '        <input type="text" inputmode="decimal" class="form-control form-control-sm" data-field="price" data-row-key="' + key + '" data-testid="card-subtask-price" value="' + (sub.price == null ? '' : esc(sub.price)) + '">' +
      '        <div class="invalid-feedback d-block d-none small" data-testid="card-subtask-warning"></div></div>' +
      '      <div class="col-12"><label class="form-label small mb-0">Оборудование' + (subEquipmentDeleted ? deletedBadge('card-subtask-equipment-deleted') : '') + '</label>' +
      '        <select class="form-select form-select-sm" data-field="equipment_id" data-row-key="' + key + '" data-testid="card-subtask-equipment">' + equipmentOptions + '</select></div>' +
      // executor-payouts D62–D66: выплата исполнителю — отметка, сумма (отдельная от цены) и
      // дата отметки. Отметить можно только при назначенном исполнителе; уже стоящую
      // отметку снять можно всегда (в т.ч. если сотрудника удалили).
      '      <div class="col-12 col-md-6 d-flex align-items-end" data-testid="card-subtask-payout">' +
      '        <div class="form-check mb-1">' +
      '          <input type="checkbox" class="form-check-input" id="' + esc(payoutId) + '" data-field="is_executor_paid" data-row-key="' + key + '" data-testid="card-subtask-executor-paid"' +
                 (sub.is_executor_paid ? ' checked' : '') + (!sub.assignee_id && !sub.is_executor_paid ? ' disabled title="Сначала назначьте исполнителя"' : '') + '>' +
      '          <label class="form-check-label small" for="' + esc(payoutId) + '">Выплачено исполнителю</label>' +
      '          <span class="small text-body-secondary ms-1" data-testid="card-subtask-executor-paid-at">' + (sub.is_executor_paid && sub.executor_paid_at ? esc(fmtDateTime(sub.executor_paid_at)) : '') + '</span>' +
      '        </div></div>' +
      '      <div class="col-12 col-md-6"><label class="form-label small mb-0">Сумма выплаты исполнителю</label>' +
      '        <input type="text" inputmode="decimal" class="form-control form-control-sm" data-field="executor_payout" data-row-key="' + key + '" data-testid="card-subtask-executor-payout" value="' + (sub.executor_payout == null ? '' : esc(sub.executor_payout)) + '">' +
      '        <div class="invalid-feedback d-block d-none small" data-testid="card-subtask-payout-warning"></div></div>' +
      '    </div>' +
      '    <div class="mt-2">' + subtaskProductsHtml(sub) + '</div>' +
      '    <div class="mt-2">' + subtaskLogsHtml(sub) + '</div>' +
      '    <div class="text-end small mt-1"><strong data-testid="card-subtask-total">Итог: ' + money(sub.total) + '</strong></div>' +
      '  </div>' +
      '</div>';
  }

  function subtaskProductsHtml(sub) {
    var rows = sub.products.map(function (p) {
      var pkey = reqKeyProduct(p.id);
      return '' +
        '<tr data-testid="card-product-row" data-row-key="' + pkey + '" data-product-id="' + esc(p.id) + '">' +
        '  <td>' + esc(p.name) + ' <span class="text-body-secondary small">(' + esc(p.unit) + ')</span>' +
        (p.product_is_deleted ? deletedBadge('card-product-deleted') : '') + '</td>' +
        '  <td style="max-width:90px">' +
        '    <input type="text" inputmode="decimal" class="form-control form-control-sm" data-field="qty" data-row-key="' + pkey + '" data-testid="card-product-qty" value="' + esc(p.qty) + '">' +
        '    <div class="invalid-feedback d-block d-none small" data-testid="card-warning"></div>' +
        '  </td>' +
        '  <td style="max-width:100px"><input type="text" inputmode="decimal" class="form-control form-control-sm" data-field="price" data-row-key="' + pkey + '" data-testid="card-product-price" value="' + (p.price == null ? '' : esc(p.price)) + '"></td>' +
        '  <td class="small text-body-secondary" data-testid="card-product-hint">' + esc(stockHint(p.product_id)) + '</td>' +
        '  <td data-testid="card-subtask-product-total">' + money(p.line_total) + '</td>' +
        '  <td><button type="button" class="btn btn-outline-danger btn-sm" data-testid="card-product-delete" data-product-row-id="' + esc(p.id) + '">×</button></td>' +
        '</tr>';
    }).join('');
    return '' +
      '<div class="table-responsive"><table class="table table-sm align-middle mb-1">' +
      '  <thead><tr><th>Товар</th><th>Кол-во</th><th>Цена</th><th>Остаток</th><th>Сумма</th><th></th></tr></thead>' +
      '  <tbody>' + rows + '</tbody>' +
      '</table></div>' +
      '<select class="form-select form-select-sm" data-testid="card-product-add" data-subtask-id="' + esc(sub.id) + '"><option value="">+ Добавить товар…</option></select>';
  }

  function subtaskLogsHtml(sub) {
    var rows = sub.logs.map(function (l) {
      var lkey = reqKeyLog(l.id);
      return '' +
        '<div class="border-top py-1" data-testid="card-log-row" data-row-key="' + lkey + '" data-log-id="' + esc(l.id) + '">' +
        '  <div class="small text-body-secondary">' + esc(l.author_name) + ' · <span data-testid="card-log-date">' + fmtDate(l.created_at) + '</span></div>' +
        '  <div data-testid="card-log-text">' + esc(l.text) + '</div>' +
        (l.can_edit
          ? '  <div class="d-flex gap-2 mt-1">' +
            '    <button type="button" class="btn btn-link btn-sm p-0" data-testid="card-log-edit" data-log-id="' + esc(l.id) + '">Изменить</button>' +
            '    <button type="button" class="btn btn-link btn-sm p-0 text-danger" data-testid="card-log-delete" data-log-id="' + esc(l.id) + '">Удалить</button>' +
            '  </div>'
          : '') +
        '</div>';
    }).join('');
    return '' +
      '<div data-testid="card-subtask-logs">' + rows + '</div>' +
      '<div class="d-flex gap-2 mt-1">' +
      '  <input class="form-control form-control-sm" data-testid="card-log-new" data-subtask-id="' + esc(sub.id) + '" maxlength="4000" placeholder="Новая запись журнала">' +
      '  <button type="button" class="btn btn-outline-secondary btn-sm" data-testid="card-log-add" data-subtask-id="' + esc(sub.id) + '">Добавить</button>' +
      '</div>';
  }

  // source (опционально) — карточка, из которой читать; по умолчанию текущая глобальная
  // `card`. baseGetter'ы rowQueue (fix-loop 2, п.1) вызывают их и с `resp.card` конкретного
  // ответа строки — важно уметь читать НЕ только из глобального состояния.
  function currentSubtask(id, source) { return (source || card).subtasks.filter(function (s) { return s.id === id; })[0]; }
  function currentProduct(subId, prodRowId, source) {
    var s = currentSubtask(subId, source);
    return s && s.products.filter(function (p) { return p.id === prodRowId; })[0];
  }

  function wireSubtasks(container) {
    // ---- статус/исполнитель/срок/цена/оборудование (LWW — PUT /subtasks/:id) ----
    $$('[data-testid="card-subtask-row"]', container).forEach(function (row) {
      var subId = row.dataset.subtaskId;
      var priceWarn = row.querySelector('[data-testid="card-subtask-warning"]');
      $$('[data-field]', row).forEach(function (input) {
        if (input.closest('[data-testid="card-product-row"]')) return; // товары — отдельно ниже
        input.addEventListener('change', function () {
          var field = input.dataset.field;
          var value = input.value;
          if (field === 'assignee_id' || field === 'equipment_id') value = value || null;
          if (field === 'due_date') value = value || null;
          if (field === 'price') {
            // LOW code review: num() молча превращал «abc»/отрицательное в 0 и сохранял —
            // цена услуги валидируется так же, как qty/price товара (пусто — «по факту»).
            var raw = input.value.trim();
            if (raw === '') {
              value = null;
            } else {
              var n = Number(raw);
              if (!isFinite(n) || n < 0) {
                input.classList.add('is-invalid');
                if (priceWarn) { priceWarn.textContent = 'Цена должна быть ≥ 0'; priceWarn.classList.remove('d-none'); }
                var s0 = currentSubtask(subId);
                input.value = (s0 && s0.price != null) ? s0.price : '';
                return;
              }
              value = n;
            }
            input.classList.remove('is-invalid');
            if (priceWarn) priceWarn.classList.add('d-none');
          }
          // executor-payouts: отметка выплаты — булево из чекбокса; сумма выплаты
          // валидируется как цена (пусто — «не задана»).
          if (field === 'is_executor_paid') value = input.checked;
          if (field === 'executor_payout') {
            var payoutWarn = row.querySelector('[data-testid="card-subtask-payout-warning"]');
            var rawPayout = input.value.trim().replace(',', '.');
            if (rawPayout === '') {
              value = null;
            } else {
              var pn = Number(rawPayout);
              if (!isFinite(pn) || pn < 0 || !/^\d+(\.\d{1,2})?$/.test(rawPayout)) {
                input.classList.add('is-invalid');
                if (payoutWarn) { payoutWarn.textContent = 'Сумма выплаты — число ≥ 0, не больше 2 знаков после запятой'; payoutWarn.classList.remove('d-none'); }
                var s1 = currentSubtask(subId);
                input.value = (s1 && s1.executor_payout != null) ? s1.executor_payout : '';
                return;
              }
              value = pn;
            }
            input.classList.remove('is-invalid');
            if (payoutWarn) payoutWarn.classList.add('d-none');
          }
          var patch = {}; patch[field] = value;
          enqueueRow(reqKeySubtask(subId), patch, function (source) {
            var s = currentSubtask(subId, source) || {};
            // База тела — только поля полной замены. is_executor_paid / executor_payout в базу
            // НЕ входят (code review HIGH-1): у них есть второй писатель — отчёт «Выплаты»
            // (POST /executor-payouts/mark), а карточка сервер не опрашивает. Устаревший
            // снимок, отправленный вместе с правкой срока/статуса/цены, молча снял бы чужую
            // отметку (или вернул снятую). Эти поля уходят только через patch — когда
            // пользователь правит именно их; сервер поле, которого нет в теле, не трогает.
            return { status: s.status, assignee_id: s.assignee_id, due_date: s.due_date, price: s.price == null ? null : num(s.price), equipment_id: s.equipment_id };
          }, function (body) {
            return KT.api.put('/api/v1/subtasks/' + subId, body);
          });
        });
      });
    });

    $$('[data-testid="card-subtask-delete"]', container).forEach(function (btn) {
      btn.addEventListener('click', function () {
        var subId = btn.dataset.subtaskId;
        KT.uiConfirm('Услуга удалится вместе с товарами и журналом.').then(function (ok) {
          if (!ok) return;
          var ticket = beginRequest();
          KT.api.del('/api/v1/subtasks/' + subId).then(function (resp) {
            delete rowQueue[reqKeySubtask(subId)];
            if (!applyCard(resp.card, ticket)) return;
            if (resp.warnings && resp.warnings.length) toastWarningsHost(resp.warnings);
            host.onChanged();
          }).catch(function (err) {
            KT.uiAlert((err && err.message) || 'Не удалось удалить услугу', 'Ошибка');
          });
        });
      });
    });

    var addSvc = $('[data-testid="card-subtask-add"]', container);
    if (addSvc && !addSvc.disabled) {
      addSvc._cbFetchOptions = function (q) {
        return KT.api.get('/api/v1/services?q=' + encodeURIComponent(q) + '&limit=50').then(function (r) {
          return r.items.map(function (s) { return { value: s.id, text: s.name }; });
        });
      };
      addSvc.addEventListener('change', function () {
        var svcId = addSvc.value;
        if (!svcId) return;
        var ticket = beginRequest();
        KT.api.post('/api/v1/tasks/' + taskId + '/subtasks', { service_id: svcId }).then(function (resp) {
          if (!applyCard(resp.card, ticket)) return;
          host.onChanged();
        }).catch(function (err) {
          KT.uiAlert((err && err.message) || 'Не удалось добавить услугу', 'Ошибка');
        });
      });
    }

    // ---- товары услуги ----
    $$('[data-testid="card-product-row"]', container).forEach(function (rowEl) {
      var prodRowId = rowEl.dataset.productId;
      var subId = rowEl.closest('[data-testid="card-subtask-row"]').dataset.subtaskId;

      var qtyInput = $('[data-testid="card-product-qty"]', rowEl);
      var rowWarning = $('[data-testid="card-warning"]', rowEl);
      qtyInput.addEventListener('change', function () {
        var raw = qtyInput.value.trim();
        var n = Number(raw);
        if (raw === '' || !isFinite(n) || n <= 0) {
          qtyInput.classList.add('is-invalid');
          rowWarning.textContent = 'Должно быть больше 0';
          rowWarning.classList.remove('d-none');
          var p = currentProduct(subId, prodRowId);
          qtyInput.value = p ? p.qty : '';
          return;
        }
        qtyInput.classList.remove('is-invalid');
        rowWarning.classList.add('d-none');
        enqueueRow(reqKeyProduct(prodRowId), { qty: n }, function (source) {
          var p = currentProduct(subId, prodRowId, source) || {};
          return { qty: num(p.qty), price: p.price == null ? null : num(p.price) };
        }, function (body) {
          return KT.api.put('/api/v1/subtask-products/' + prodRowId, body);
        });
      });

      var priceInput = $('[data-testid="card-product-price"]', rowEl);
      priceInput.addEventListener('change', function () {
        var raw = priceInput.value.trim();
        var value = null;
        if (raw !== '') {
          var n = Number(raw);
          if (!isFinite(n) || n < 0) {
            priceInput.classList.add('is-invalid');
            rowWarning.textContent = 'Цена должна быть ≥ 0';
            rowWarning.classList.remove('d-none');
            var p0 = currentProduct(subId, prodRowId);
            priceInput.value = (p0 && p0.price != null) ? p0.price : '';
            return;
          }
          value = n;
        }
        priceInput.classList.remove('is-invalid');
        rowWarning.classList.add('d-none');
        enqueueRow(reqKeyProduct(prodRowId), { price: value }, function (source) {
          var p = currentProduct(subId, prodRowId, source) || {};
          return { qty: num(p.qty), price: p.price == null ? null : num(p.price) };
        }, function (body) {
          return KT.api.put('/api/v1/subtask-products/' + prodRowId, body);
        });
      });

      $('[data-testid="card-product-delete"]', rowEl).addEventListener('click', function () {
        delete rowQueue[reqKeyProduct(prodRowId)];
        var ticket = beginRequest();
        KT.api.del('/api/v1/subtask-products/' + prodRowId).then(function (resp) {
          if (!applyCard(resp.card, ticket)) return;
          if (resp.warnings && resp.warnings.length) toastWarningsHost(resp.warnings);
          host.onChanged();
        }).catch(function (err) {
          KT.uiAlert((err && err.message) || 'Не удалось удалить товар', 'Ошибка');
        });
      });
    });

    $$('[data-testid="card-product-add"]', container).forEach(function (sel) {
      var subId = sel.dataset.subtaskId;
      sel._cbFetchOptions = function (q) {
        return KT.api.get('/api/v1/products?q=' + encodeURIComponent(q) + '&limit=50').then(function (r) {
          return r.items.map(function (p) { return { value: p.id, text: p.name }; });
        });
      };
      sel.addEventListener('change', function () {
        var prodId = sel.value;
        if (!prodId) return;
        var ticket = beginRequest();
        KT.api.post('/api/v1/subtasks/' + subId + '/products', { product_id: prodId, qty: 1 }).then(function (resp) {
          if (!applyCard(resp.card, ticket)) return;
          if (resp.warnings && resp.warnings.length) toastWarningsHost(resp.warnings);
          host.onChanged();
        }).catch(function (err) {
          KT.uiAlert((err && err.message) || 'Не удалось добавить товар', 'Ошибка');
        });
      });
    });

    // ---- журнал ----
    $$('[data-testid="card-log-row"]', container).forEach(function (rowEl) {
      var logId = rowEl.dataset.logId;
      var editBtn = $('[data-testid="card-log-edit"]', rowEl);
      if (editBtn) editBtn.addEventListener('click', function () { startEditLog(rowEl, logId); });
      var delBtn = $('[data-testid="card-log-delete"]', rowEl);
      if (delBtn) delBtn.addEventListener('click', function () {
        KT.uiConfirm('Удалить запись журнала?').then(function (ok) {
          if (!ok) return;
          var ticket = beginRequest();
          KT.api.del('/api/v1/subtask-logs/' + logId).then(function (resp) {
            if (!applyCard(resp.card, ticket)) return;
            host.onChanged();
          }).catch(function (err) {
            if (err && err.code === 'SUBTASK_LOG_FORBIDDEN') { KT.uiAlert(err.message, 'Нельзя'); return; }
            KT.uiAlert((err && err.message) || 'Не удалось удалить запись', 'Ошибка');
          });
        });
      });
    });

    $$('[data-testid="card-log-add"]', container).forEach(function (btn) {
      var subId = btn.dataset.subtaskId;
      btn.addEventListener('click', function () {
        var input = $('[data-testid="card-log-new"][data-subtask-id="' + cssesc(subId) + '"]', container);
        var text = input.value.trim();
        if (!text) return;
        var ticket = beginRequest();
        KT.api.post('/api/v1/subtasks/' + subId + '/logs', { text: text }).then(function (resp) {
          if (!applyCard(resp.card, ticket)) return;
          host.onChanged();
        }).catch(function (err) {
          KT.uiAlert((err && err.message) || 'Не удалось добавить запись', 'Ошибка');
        });
      });
    });
  }

  function startEditLog(rowEl, logId) {
    var log = null;
    card.subtasks.some(function (s) { log = s.logs.filter(function (l) { return l.id === logId; })[0]; return !!log; });
    if (!log) return;
    rowEl.innerHTML =
      '<textarea class="form-control form-control-sm" data-testid="card-subtask-log-edit-input" maxlength="4000">' + esc(log.text) + '</textarea>' +
      '<div class="d-flex gap-2 mt-1">' +
      '  <button type="button" class="btn btn-primary btn-sm" data-testid="card-subtask-log-save">Сохранить</button>' +
      '  <button type="button" class="btn btn-outline-secondary btn-sm" data-testid="card-subtask-log-cancel">Отмена</button>' +
      '</div>';
    $('[data-testid="card-subtask-log-save"]', rowEl).addEventListener('click', function () {
      var text = $('[data-testid="card-subtask-log-edit-input"]', rowEl).value.trim();
      if (!text) return;
      var ticket = beginRequest();
      KT.api.put('/api/v1/subtask-logs/' + logId, { text: text }).then(function (resp) {
        if (!applyCard(resp.card, ticket)) return;
        host.onChanged();
      }).catch(function (err) {
        if (err && err.code === 'SUBTASK_LOG_FORBIDDEN') { KT.uiAlert(err.message, 'Нельзя'); return; }
        KT.uiAlert((err && err.message) || 'Не удалось сохранить запись', 'Ошибка');
      });
    });
    $('[data-testid="card-subtask-log-cancel"]', rowEl).addEventListener('click', function () { renderSubtasks(); });
  }

  // ==================================================================
  // Вложения заказа (фаза 7, D21): фото — миниатюра (открытие в полный размер в новой
  // вкладке), видео — <video controls>, PDF — ссылка. Тип определяет сервер по содержимому;
  // здесь — только ранняя подсказка по размеру (сервер всё равно проверяет, 413/400).
  // Загрузка — XHR (fetch не даёт прогресса отправки), ответ — {card, warnings}.
  // ==================================================================

  var ATT_MAX_VIDEO = 500 * 1024 * 1024;
  var ATT_MAX_OTHER = 20 * 1024 * 1024;
  var ATT_ACCEPT = 'image/jpeg,image/png,image/gif,image/webp,video/mp4,video/webm,video/quicktime,application/pdf';

  function fmtSize(n) {
    n = num(n);
    if (n >= 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' МБ';
    if (n >= 1024) return Math.round(n / 1024) + ' КБ';
    return n + ' Б';
  }

  function attachmentMediaHtml(a) {
    var url = esc(a.url);
    if (a.kind === 'photo') {
      return '<a class="att-media" href="' + url + '" target="_blank" rel="noopener" data-testid="card-attachment-open" title="Открыть в полном размере">' +
        '<img src="' + url + '" alt="' + esc(a.original_name) + '" loading="lazy" data-testid="card-attachment-thumb"></a>';
    }
    if (a.kind === 'video') {
      return '<div class="att-media"><video controls preload="metadata" src="' + url + '" data-testid="card-attachment-video"></video></div>';
    }
    return '<a class="att-media att-pdf-icon" href="' + url + '" target="_blank" rel="noopener" data-testid="card-attachment-pdf" title="Открыть PDF">📄</a>';
  }

  function attachmentItemHtml(a) {
    return '<div class="att-item" data-testid="card-attachment" data-attachment-id="' + esc(a.id) + '" data-kind="' + esc(a.kind) + '">' +
      attachmentMediaHtml(a) +
      '<div class="att-meta">' +
        '<span class="att-name" title="' + esc(a.original_name) + '" data-testid="card-attachment-name">' + esc(a.original_name) + '</span>' +
        '<div class="att-sub">' + esc(fmtSize(a.size)) + ' · ' + esc(a.author_name) + '</div>' +
      '</div>' +
      (a.can_delete ? '<button type="button" class="btn btn-sm btn-dark att-delete" data-testid="card-attachment-delete" data-attachment-id="' + esc(a.id) + '" aria-label="Удалить вложение ' + esc(a.original_name) + '">✕</button>' : '') +
    '</div>';
  }

  function renderAttachments() {
    var container = $('#card-attachments');
    if (!container || !card) return;
    var items = card.attachments || [];
    var busy = !!(upload && upload.gen === openGeneration);
    var list = items.length
      ? '<div class="att-gallery" data-testid="card-attachments-list">' + items.map(attachmentItemHtml).join('') + '</div>'
      : '<p class="text-body-secondary small" data-testid="card-attachments-empty">Вложений пока нет</p>';
    var pct = busy ? upload.percent : 0;
    container.innerHTML = list +
      '<div class="att-upload">' +
        '<label class="btn btn-outline-secondary btn-sm mb-0' + (busy ? ' disabled' : '') + '" data-testid="card-attachment-upload"' + (busy ? ' aria-disabled="true"' : '') + '>' +
          (busy ? 'Загрузка…' : '📎 Загрузить фото, видео или PDF') +
          '<input type="file" hidden accept="' + ATT_ACCEPT + '" data-testid="card-attachment-input"' + (busy ? ' disabled' : '') + '>' +
        '</label>' +
        '<div class="progress' + (busy ? '' : ' d-none') + '" role="progressbar" aria-label="Загрузка вложения" aria-valuemin="0" aria-valuemax="100" aria-valuenow="' + pct + '" data-testid="card-attachment-progress">' +
          '<div class="progress-bar" style="width:' + pct + '%">' + pct + '%</div>' +
        '</div>' +
        '<span class="small text-body-secondary' + (busy ? '' : ' d-none') + '" data-testid="card-attachment-uploading-name">' + (busy ? esc(upload.name) : '') + '</span>' +
      '</div>' +
      '<div class="text-danger small mt-1 d-none" role="alert" data-testid="card-attachment-error"></div>';
    wireAttachments(container);
  }

  function showAttachmentError(msg) {
    var el = $('[data-testid="card-attachment-error"]');
    if (!el) return;
    el.textContent = msg;
    el.classList.remove('d-none');
  }

  function setUploadProgress(pct) {
    if (!upload) return;
    upload.percent = pct;
    var bar = $('[data-testid="card-attachment-progress"]');
    if (!bar) return;
    bar.setAttribute('aria-valuenow', String(pct));
    var inner = bar.querySelector('.progress-bar');
    inner.style.width = pct + '%';
    inner.textContent = pct + '%';
  }

  function csrfToken() {
    var m = document.querySelector('meta[name="csrf-token"]');
    return m ? m.content : '';
  }

  // xhrUpload — POST multipart с прогрессом; ошибки — KT.ApiError (тот же wire, что KT.api).
  function xhrUpload(tid, file, onProgress) {
    return new Promise(function (resolve, reject) {
      var xhr = new XMLHttpRequest();
      xhr.open('POST', '/api/v1/tasks/' + encodeURIComponent(tid) + '/attachments');
      xhr.setRequestHeader('Accept', 'application/json');
      xhr.setRequestHeader('X-Csrf-Token', csrfToken());
      xhr.upload.onprogress = function (e) {
        if (e.lengthComputable) onProgress(Math.min(99, Math.floor(e.loaded * 100 / e.total)));
      };
      xhr.onload = function () {
        var data = null;
        try { data = JSON.parse(xhr.responseText); } catch (e) { data = null; }
        if (xhr.status >= 200 && xhr.status < 300 && data) { onProgress(100); resolve(data); return; }
        reject(new KT.ApiError(xhr.status, data || { message: 'Не удалось загрузить файл (HTTP ' + xhr.status + ')' }));
      };
      xhr.onerror = function () { reject(new KT.ApiError(0, { message: 'Сеть недоступна — файл не загружен' })); };
      var fd = new FormData();
      fd.append('file', file, file.name);
      xhr.send(fd);
    });
  }

  function startUpload(file) {
    if (!taskId || !card || upload) return;
    var tooBig = file.type.indexOf('video/') === 0 ? file.size > ATT_MAX_VIDEO : file.size > ATT_MAX_OTHER;
    if (file.size > ATT_MAX_VIDEO || tooBig) {
      showAttachmentError('Файл «' + file.name + '» слишком большой: фото и PDF — до 20 МБ, видео — до 500 МБ');
      return;
    }
    var tid = taskId;
    var ticket = beginRequest();
    upload = { gen: openGeneration, name: file.name, percent: 0 };
    renderAttachments();
    xhrUpload(tid, file, setUploadProgress).then(function (resp) {
      if (ticket.gen !== openGeneration) return; // карточку уже закрыли/сменили
      upload = null;
      // Загрузка долгая: пока она шла, могли примениться более поздние ответы row-мутаций
      // (seq выше) — тогда снимок из этого ответа устарел, перечитываем карточку целиком.
      if (!applyCard(resp.card, ticket)) { loadCard(tid); return; }
      host.onChanged();
    }).catch(function (err) {
      if (ticket.gen !== openGeneration) return;
      upload = null;
      renderAttachments();
      if (err && err.status === 404 && err.code === 'TASK_NOT_FOUND') {
        KT.uiAlert('Заказ уже удалён', 'Не удалось загрузить').then(close);
        return;
      }
      var msg = (err && err.fields && err.fields.length) ? err.fields.map(function (f) { return f.message; }).join('; ') : ((err && err.message) || 'Не удалось загрузить файл');
      showAttachmentError(msg);
    });
  }

  function wireAttachments(container) {
    var input = $('[data-testid="card-attachment-input"]', container);
    if (input) input.addEventListener('change', function () {
      var file = input.files && input.files[0];
      input.value = '';
      if (file) startUpload(file);
    });

    $$('[data-testid="card-attachment-delete"]', container).forEach(function (btn) {
      btn.addEventListener('click', function () {
        var id = btn.dataset.attachmentId;
        var a = (card.attachments || []).filter(function (x) { return x.id === id; })[0];
        KT.uiConfirm('Удалить вложение «' + (a ? a.original_name : '') + '»?').then(function (ok) {
          if (!ok) return;
          var ticket = beginRequest();
          KT.api.del('/api/v1/attachments/' + id).then(function (resp) {
            if (!applyCard(resp.card, ticket)) return;
            host.onChanged();
          }).catch(function (err) {
            if (err && err.code === 'ATTACHMENT_NOT_FOUND') {
              KT.uiAlert('Вложение уже удалено', 'Не удалось удалить').then(function () { loadCard(taskId); });
              return;
            }
            KT.uiAlert((err && err.message) || 'Не удалось удалить вложение', 'Ошибка');
          });
        });
      });
    });
  }

  // renderTotal — итог заказа (услуги + товары).
  function renderTotal() {
    $('#card-total').innerHTML =
      '<strong data-testid="card-total">Итог заказа: ' + money(card.task.total_amount) + '</strong>';
  }

  // ==================================================================
  // Новый заказ по госномеру: поиск автомобиля, «Новый автомобиль», новый клиент.
  // Сохранение — ОДИН POST /api/v1/tasks (plate_number + equipment_id | new_equipment
  // [+ new_client]): сервер атомарно создаёт клиента/автомобиль/заказ и кладёт автомобиль в
  // заказ; название «{Марка} {Модель} {ГОСНОМЕР}» генерирует сервер.
  // ==================================================================

  var PLATE_LOOKUP_MIN = 4;      // символов госномера (без пробелов/дефисов) для поиска
  var PLATE_LOOKUP_DELAY = 350;  // debounce ввода, мс
  // car.status: idle (номер не введён) | searching | found (один) | multi (несколько —
  // выбрать) | notfound (форма «Новый автомобиль») | error (поиск не удался — повтор при сохранении)
  var car = { status: 'idle', key: '', items: [], chosenId: '' };
  var lookupTimer = null;
  var lookupSeq = 0;
  var lookupPromise = null;
  var clientFromCar = false; // поле «Клиент» заполнено владельцем найденного автомобиля

  // FIELD_ERROR_TEXT — коды details.fields[].message → текст под полем формы нового заказа.
  var FIELD_ERROR_TEXT = {
    required: 'Обязательное поле',
    too_long: 'Слишком длинное значение',
    too_many: 'Слишком много значений',
    invalid: 'Некорректное значение',
    invalid_format: 'Неверный формат',
    invalid_text: 'Недопустимые символы',
    not_found: 'Не найдено',
    archived: 'Тип в архиве — выберите другой',
    conflict: 'Противоречивые данные',
    plate_mismatch: 'Госномер не совпадает с найденным автомобилем — проверьте номер',
    foreign_client: 'Автомобиль принадлежит другому клиенту'
  };

  function fieldText(f) {
    if (f.name === 'plate_number' && PLATE_ERROR_TEXT[f.message]) return PLATE_ERROR_TEXT[f.message];
    return FIELD_ERROR_TEXT[f.message] || f.message;
  }

  // plateKey — ключ сравнения госномера (как sqlutil.PlateCompact на сервере).
  function plateKey(v) { return String(v || '').replace(/[\s-]+/g, '').toUpperCase(); }
  // plateTidy — нормализация поля при уходе фокуса (как clients.NormalizePlateNumber).
  function plateTidy(v) { return String(v || '').replace(/\s+/g, ' ').trim().toUpperCase(); }

  function setNewOrderMode(isNew) {
    $('#task-plate-wrap').classList.toggle('d-none', !isNew);
    $('#task-title-wrap').classList.toggle('d-none', isNew);
    if (!isNew) {
      clearTimeout(lookupTimer);
      lookupSeq++;
      $('#task-car').classList.add('d-none');
      $('#task-new-client-toggle-wrap').classList.add('d-none');
      $('#task-new-client').classList.add('d-none');
      $('#task-client-locked').classList.add('d-none');
      setClientLocked(false);
    }
  }

  function setClientLocked(locked) {
    var sel = $('#task-client');
    sel.disabled = locked;
    if (sel._cbSync) sel._cbSync();
  }

  function resetCar() {
    clearTimeout(lookupTimer);
    lookupSeq++;
    lookupPromise = null;
    car = { status: 'idle', key: '', items: [], chosenId: '' };
    clientFromCar = false;
    $('#task-plate').value = '';
    ['#task-new-car-brand', '#task-new-car-model', '#task-new-car-vin', '#task-new-client-name', '#task-new-client-phone'].forEach(function (sel) { $(sel).value = ''; });
    var typeSel = $('#task-new-car-type');
    typeSel.value = '';
    if (typeSel._cbSync) typeSel._cbSync();
    $('#task-new-client-toggle').checked = false;
    clearFieldErrors();
    renderCar();
  }

  function clearFieldErrors() {
    $$('#task-modal [data-error-for]').forEach(function (el) { el.textContent = ''; el.classList.add('d-none'); });
  }

  function carLabel(it) {
    return [it.brand, it.model].filter(Boolean).join(' ') + ' · ' + it.plate_number + ' · ' + it.client_name;
  }

  function chosenCar() {
    return car.items.filter(function (it) { return it.id === car.chosenId; })[0] || null;
  }

  // renderCar — блок под госномером и поле «Клиент» по состоянию поиска.
  function renderCar() {
    var data = host.getData();
    var status = $('#task-plate-status');
    var box = $('#task-car');
    var found = $('#task-car-found');
    var choices = $('#task-car-choices');
    var newCar = $('#task-new-car');
    var chosen = chosenCar();
    var newClient = car.status === 'notfound' && data.isAdmin && $('#task-new-client-toggle').checked;

    status.textContent = {
      idle: 'Введите госномер — автомобиль найдётся автоматически',
      searching: 'Поиск автомобиля…',
      found: 'Автомобиль найден',
      multi: 'Найдено автомобилей: ' + car.items.length + ' — выберите нужный',
      notfound: 'Автомобиль не найден — заполните данные нового',
      error: 'Не удалось проверить госномер — повторим при сохранении'
    }[car.status] || '';

    box.classList.toggle('d-none', !(car.status === 'found' || car.status === 'multi' || car.status === 'notfound'));
    found.classList.toggle('d-none', car.status !== 'found');
    if (car.status === 'found' && chosen) {
      found.innerHTML = plateBadge(chosen.plate_number) + '<span data-testid="task-car-found-text">' + esc(carLabel(chosen)) + '</span>';
    }
    choices.classList.toggle('d-none', car.status !== 'multi');
    if (car.status === 'multi') {
      choices.innerHTML = car.items.map(function (it) {
        return '<div class="form-check"><input class="form-check-input" type="radio" name="task-car-choice" id="task-car-' + esc(it.id) + '" value="' + esc(it.id) + '" data-testid="task-car-option"' + (it.id === car.chosenId ? ' checked' : '') + '>' +
          '<label class="form-check-label" for="task-car-' + esc(it.id) + '">' + esc(carLabel(it)) + '</label></div>';
      }).join('');
    }
    newCar.classList.toggle('d-none', car.status !== 'notfound');

    // клиент: найденный/выбранный автомобиль — его владелец (поле заблокировано).
    var owner = (car.status === 'found' || car.status === 'multi') && chosen;
    var sel = $('#task-client');
    if (owner) {
      KT.comboboxEnsureOption(sel, chosen.client_id, chosen.client_name);
      sel.value = chosen.client_id;
      clientFromCar = true;
    } else if (clientFromCar) {
      sel.value = ''; // владелец прежнего найденного автомобиля — не выбор пользователя
      clientFromCar = false;
    }
    $('#task-client-locked').classList.toggle('d-none', !owner);
    $('#task-new-client-toggle-wrap').classList.toggle('d-none', !(car.status === 'notfound' && data.isAdmin));
    $('#task-new-client').classList.toggle('d-none', !newClient);
    setClientLocked(!!owner || newClient);
  }

  // runLookup — GET /api/v1/equipment?plate_number=…; ответы устаревших вводов отбрасываются.
  function runLookup() {
    clearTimeout(lookupTimer);
    var value = $('#task-plate').value;
    var key = plateKey(value);
    var seq = ++lookupSeq;
    car = { status: 'searching', key: key, items: [], chosenId: '' };
    renderCar();
    lookupPromise = KT.api.get('/api/v1/equipment?plate_number=' + encodeURIComponent(value) + '&limit=20').then(function (r) {
      if (seq !== lookupSeq) return;
      var items = r.items || [];
      car = {
        status: items.length === 0 ? 'notfound' : (items.length === 1 ? 'found' : 'multi'),
        key: key, items: items, chosenId: items.length === 1 ? items[0].id : ''
      };
      if (car.status === 'notfound') preloadCarTypes();
      renderCar();
    }).catch(function () {
      if (seq !== lookupSeq) return;
      car = { status: 'error', key: key, items: [], chosenId: '' };
      renderCar();
    });
    return lookupPromise;
  }

  function scheduleLookup() {
    clearTimeout(lookupTimer);
    var key = plateKey($('#task-plate').value);
    if (key === car.key && car.status !== 'idle' && car.status !== 'error') return;
    if (key.length < PLATE_LOOKUP_MIN) {
      lookupSeq++;
      car = { status: 'idle', key: key, items: [], chosenId: '' };
      renderCar();
      return;
    }
    lookupTimer = setTimeout(runLookup, PLATE_LOOKUP_DELAY);
  }

  // ensureLookup — перед сохранением: результат поиска относится к ТЕКУЩЕМУ номеру.
  function ensureLookup() {
    var key = plateKey($('#task-plate').value);
    if (!key) return Promise.resolve();
    if (key === car.key && car.status === 'searching' && lookupPromise) return lookupPromise;
    if (key !== car.key || car.status === 'idle' || car.status === 'error') return runLookup();
    return Promise.resolve();
  }

  var carTypesLoaded = false;
  // preloadCarTypes — первая страница активных типов для «Новый автомобиль» (поиск по вводу —
  // _cbFetchOptions); единственный тип выбирается сразу.
  function preloadCarTypes() {
    if (carTypesLoaded) return;
    carTypesLoaded = true;
    KT.api.get('/api/v1/equipment-types?limit=200').then(function (r) {
      var sel = $('#task-new-car-type');
      (r.items || []).forEach(function (t) { KT.comboboxEnsureOption(sel, t.id, t.name); });
      if (!sel.value && r.items && r.items.length === 1) sel.value = r.items[0].id;
      if (sel._cbSync) sel._cbSync();
    }).catch(function () { carTypesLoaded = false; });
  }

  // newOrderPayload — тело POST /api/v1/tasks нового заказа; null — локальная ошибка (показана).
  function newOrderPayload() {
    var due = $('#task-due').value;
    var assignee = $('#task-assignee').value;
    var body = {
      plate_number: plateTidy($('#task-plate').value),
      column_id: $('#task-column').value,
      assignee_id: assignee || null,
      due_date: due || null,
      is_paid: $('#task-paid').checked,
      description: $('#task-description').value
    };
    if (!plateKey(body.plate_number)) {
      showFieldErrors([{ name: 'plate_number', message: 'required' }]);
      return null;
    }
    if (car.status === 'found' || car.status === 'multi') {
      var chosen = chosenCar();
      if (!chosen) {
        showFieldErrors([{ name: 'equipment_id', message: 'required' }], 'Выберите автомобиль');
        return null;
      }
      body.equipment_id = chosen.id;
      body.client_id = chosen.client_id;
      return body;
    }
    body.new_equipment = {
      type_id: $('#task-new-car-type').value,
      brand: $('#task-new-car-brand').value.trim(),
      model: $('#task-new-car-model').value.trim(),
      serial: $('#task-new-car-vin').value.trim().toUpperCase()
    };
    if (host.getData().isAdmin && $('#task-new-client-toggle').checked) {
      var phone = $('#task-new-client-phone').value.trim();
      body.new_client = { company_name: $('#task-new-client-name').value.trim(), phones: phone ? [{ phone: phone, note: '' }] : [] };
    } else {
      body.client_id = $('#task-client').value;
    }
    return body;
  }

  // showFieldErrors — текст под полем по пути details.fields[].name; без слота — в шапку.
  function showFieldErrors(fields, overrideText) {
    var rest = [];
    fields.forEach(function (f) {
      var text = overrideText || fieldText(f);
      if (f.name === 'client_id') { showClientError(f.message === 'not_found' ? 'Клиент не найден' : text); return; }
      var el = $('#task-modal [data-error-for="' + cssesc(f.name) + '"]');
      if (!el) { rest.push(text + ' (' + f.name + ')'); return; }
      el.textContent = text;
      el.classList.remove('d-none');
    });
    if (rest.length) showHeaderError(rest.join('; '));
  }

  function bindCarLookup() {
    var plate = $('#task-plate');
    plate.addEventListener('input', function () {
      var pos = plate.selectionStart;
      var up = plate.value.toUpperCase();
      if (up !== plate.value) { plate.value = up; try { plate.setSelectionRange(pos, pos); } catch (e) { /* noop */ } }
      var err = $('#task-modal [data-error-for="plate_number"]');
      err.textContent = ''; err.classList.add('d-none');
      scheduleLookup();
    });
    plate.addEventListener('blur', function () {
      plate.value = plateTidy(plate.value);
      scheduleLookup();
    });
    plate.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); plate.blur(); }
    });
    $('#task-car-choices').addEventListener('change', function (e) {
      if (e.target && e.target.name === 'task-car-choice') {
        car.chosenId = e.target.value;
        var err = $('#task-modal [data-error-for="equipment_id"]');
        err.textContent = ''; err.classList.add('d-none');
        renderCar();
      }
    });
    $('#task-new-client-toggle').addEventListener('change', function () {
      renderCar();
      if ($('#task-new-client-toggle').checked) $('#task-new-client-name').focus();
    });
    var typeSel = $('#task-new-car-type');
    typeSel._cbFetchOptions = function (q) {
      return KT.api.get('/api/v1/equipment-types?q=' + encodeURIComponent(q) + '&limit=50').then(function (r) {
        return r.items.map(function (t) { return { value: t.id, text: t.name }; });
      });
    };
  }

  // ==================================================================
  // Шапка заказа (AC-001/AC-002/AC-003/AC-004/AC-017)
  // ==================================================================

  function headerPayload() {
    var due = $('#task-due').value;
    var assignee = $('#task-assignee').value;
    return {
      title: $('#task-title').value.trim(),
      client_id: $('#task-client').value,
      column_id: $('#task-column').value,
      assignee_id: assignee || null,
      due_date: due || null,
      is_paid: $('#task-paid').checked,
      description: $('#task-description').value
    };
  }

  function showHeaderError(msg) {
    var el = $('#task-error');
    el.textContent = msg;
    el.classList.remove('d-none');
  }

  function bindStatic() {
    bindCarLookup();
    // headerDirty (MEDIUM-2): любая правка шапки — «грязная» до следующего успешного
    // loadCard (после «Сохранить» или полной перезагрузки карточки). Слушатели вешаются
    // ОДИН раз здесь — сами элементы шапки не пересоздаются при re-render (rerenderPreserveFocus
    // применяется только к секциям строк, не к шапке), поэтому переживают renderAll.
    ['#task-title', '#task-description'].forEach(function (sel) {
      var el = $(sel); if (el) el.addEventListener('input', function () { headerDirty = true; });
    });
    ['#task-client', '#task-column', '#task-assignee', '#task-due', '#task-paid'].forEach(function (sel) {
      var el = $(sel); if (el) el.addEventListener('change', function () { headerDirty = true; });
    });

    $('#task-client').addEventListener('change', function () {
      if (!taskId || !card) return;
      var newClientId = $('#task-client').value;
      if (newClientId === card.task.client_id) return;
      if (card.equipment.length > 0) {
        KT.uiConfirm('Оборудование другого клиента будет убрано из заказа.').then(function (ok) {
          if (!ok) { $('#task-client').value = card.task.client_id; if ($('#task-client')._cbSync) $('#task-client')._cbSync(); }
        });
      }
    });

    $('#task-save-btn').addEventListener('click', function () {
      if (saving) return;
      saving = true;
      $('#task-save-btn').disabled = true;
      $('#task-error').classList.add('d-none');
      var clientErr = $('#task-client-error');
      if (clientErr) { clientErr.textContent = ''; clientErr.classList.add('d-none'); }

      clearFieldErrors();
      var req;
      if (taskId) {
        var payload = headerPayload();
        payload.version = card ? card.task.version : null;
        req = KT.api.put('/api/v1/tasks/' + taskId, payload);
      } else {
        // новый заказ: дождаться поиска по ТЕКУЩЕМУ госномеру (debounce мог не сработать),
        // затем один атомарный POST. null — локальная ошибка формы уже показана.
        var gen = openGeneration;
        req = ensureLookup().then(function () {
          if (gen !== openGeneration) return Promise.reject({ stale: true });
          var body = newOrderPayload();
          if (!body) return Promise.reject({ local: true });
          return KT.api.post('/api/v1/tasks', body);
        });
      }
      req.then(function (resp) {
        saving = false;
        $('#task-save-btn').disabled = false;
        host.onChanged();
        // SD-006: и создание, и правка шапки — модалка остаётся открытой, карточка
        // перечитывается заново, разделы оборудования/услуг разблокируются.
        return loadCard(resp.task.id);
      }).catch(function (err) {
        saving = false;
        $('#task-save-btn').disabled = false;
        if (err && (err.local || err.stale)) return;
        if (!taskId && err && err.fields && err.fields.length) {
          showFieldErrors(err.fields);
          return;
        }
        if (err && err.code === 'TASK_VERSION_CONFLICT') {
          KT.uiAlert(err.message || 'Заказ уже изменён, обновите', 'Конфликт версии').then(function () { if (taskId) loadCard(taskId); });
          return;
        }
        if (err && err.fields && err.fields.length) {
          // клиента удалили, пока форма открыта (soft-delete-clients §8.3) — текст под полем
          var clientGone = err.fields.some(function (f) { return f.name === 'client_id' && f.message === 'not_found'; });
          if (clientGone) showClientError('Клиент не найден');
          var rest = err.fields.filter(function (f) { return !(f.name === 'client_id' && f.message === 'not_found'); });
          if (rest.length) showHeaderError(rest.map(function (f) { return f.message; }).join('; '));
          return;
        }
        showHeaderError((err && err.message) || 'Не удалось сохранить заказ');
      });
    });

    $('#task-delete-btn').addEventListener('click', function () {
      if (!taskId) return;
      var id = taskId;
      // Мягкое удаление с причиной (soft-delete-core §8.2, §8.3): окно «Удаление» само
      // показывает ошибку причины (400) и остаётся открытым.
      var title = (card && card.task && card.task.title) || '';
      KT.uiDeleteDialog({
        title: 'Удалить «#' + id + (title ? ' ' + title : '') + '»?',
        text: 'Товары заказа вернутся на склад. Заказ можно восстановить в разделе «Удалённые».',
        submit: function (reason) { return KT.api.delWith('/api/v1/tasks/' + id, { reason: reason }); },
        onOffline: function () { KT.uiToast('Нет связи с сервером', true); }
      }).then(function (out) {
        if (!out) return; // отмена — причина не сохраняется
        if (out.error) {
          var err = out.error;
          if (err.status === 404) { // заказ удалил другой администратор
            close();
            host.onChanged();
            KT.uiToast('Заказ не найден', true);
            return;
          }
          KT.uiToast(err.status === 403 ? 'Нет доступа' : (err.message || 'Не удалось удалить заказ'), true);
          return;
        }
        close();
        host.onChanged();
        KT.uiToast('Удалено. Восстановить можно в разделе «Удалённые»');
        if (out.result.warnings && out.result.warnings.length) toastWarningsHost(out.result.warnings);
      });
    });

    $('#task-modal').addEventListener('hidden.bs.modal', function () {
      card = null; taskId = null; rowQueue = {}; newEquipmentOpen = false; headerDirty = false; upload = null;
      clearTimeout(lookupTimer); lookupSeq++;
      openGeneration++; // закрыли без переоткрытия — ответы запросов в полёте больше не применяем
    });
  }

  KT.taskCard = {
    init: function (opts) {
      host.getData = opts.getData;
      host.onChanged = opts.onChanged || function () {};
      host.toastWarnings = opts.toastWarnings || function () {};
      bindStatic();
    },
    open: open,
    close: close
  };
})();
