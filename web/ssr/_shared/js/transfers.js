/* KT.transfers — экран «Перемещения» «/transfers» (admin и executor, transfers-partners §8.1):
   master-detail поверх JSON API /api/v1/transfers.

   Слева — список с фильтрами «Статус», «Склад», «Направление», «Поставщик» (admin), «Заказ»,
   «Период» ({items,total,limit,offset}, по 50, «Показать ещё»); справа — документ (?id=N в
   адресе; новый из карточки заказа — ?new=1&direction=…&task_id=N). Направление документа
   (DS-002) задаёт точки маршрута и состав строк (DIR):
     - приём от клиента / возврат клиенту — по заказу, строки — единицы оборудования заказа;
     - склад → склад — товары (комбобокс по каталогу) и единицы (поиск GET /api/v1/equipment);
     - приход от поставщика — только товары, количество = «Заказано».
   Исполнителю доступны только документы с клиентом (DS-018): меню и фильтры урезаны сервером.
   Заявка (requested) и новый документ — редактор; кнопки «Сохранить» / «Отправить» /
   «Отменить», для нового приёма от клиента — «Принять сразу» (создать, отправить и принять
   одним запросом, DS-013). В пути — «Принять» (товарные направления — модалка с фактом;
   документы с клиентом — диалог без полей) и «Отменить отправку»; «Получено» и «Отменён» —
   только просмотр.

   UI-конвенции (DS-052): ошибка поля — под полем/в строке (is-invalid), глобальная ошибка и
   успех — тост справа вверху на 5 секунд, подтверждение — KT.uiDialog, кнопки блокируются
   на время запроса. Дата и время — только KT.time (пояс пользователя); даты фильтра «Период»
   уходят на сервер календарными датами YYYY-MM-DD — границы дня считает сервер по поясу
   пользователя (DS-034). Числа выводятся как пришли в JSON, весь текст — через esc(). */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  var PAGE = 50;
  var TOAST_MS = 5000;
  var SEARCH_DEBOUNCE_MS = 300;
  var API = '/api/v1/transfers';

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function $(sel, root) { return (root || document).querySelector(sel); }
  function $$(sel, root) { return [].slice.call((root || document).querySelectorAll(sel)); }
  function num(v) { return String(v == null ? 0 : v); }
  // money — сумма с двумя знаками, как в карточке заказа (task-card.js).
  function money(v) { var n = Number(v); return (isFinite(n) ? n : 0).toFixed(2); }
  // Суммы накладной — целочисленно (BigInt), как round(numeric, 2) у сервера: float даёт
  // расхождение на копейку при дробном количестве (0.15 × 1.5 → 0.23, не 0.22).
  // toScaled — неотрицательное десятичное «123.45» в целое с frac знаками после запятой
  // (цена — копейки, frac = 2; количество — тысячные, frac = 3); не число, отрицательное либо
  // лишние знаки — null (превью считает такую строку нулём, ошибку покажет сервер).
  function toScaled(v, frac) {
    var m = /^(\d+)(?:\.(\d*))?$/.exec(String(v == null ? '' : v).trim());
    if (!m || (m[2] || '').length > frac) return null;
    var tail = m[2] || '';
    while (tail.length < frac) tail += '0';
    return BigInt(m[1] + tail);
  }
  // lineCents — «Сумма» строки в копейках: цена × количество, округление half-up до копейки.
  function lineCents(price, qty) {
    var pc = toScaled(price, 2), qm = toScaled(qty, 3);
    if (pc === null || qm === null) return BigInt(0);
    return (pc * qm + BigInt(500)) / BigInt(1000);
  }
  // fmtCents — копейки (BigInt) → «123.45».
  function fmtCents(c) {
    var t = c.toString();
    while (t.length < 3) t = '0' + t;
    return t.slice(0, -2) + '.' + t.slice(-2);
  }

  // DIR — направления (DS-002): подпись, точки маршрута, допустимые строки (DS-004).
  // equipment: 'task' — единицы оборудования заказа, 'search' — поиск по складу отправления.
  var DIR = {
    client_to_warehouse: { label: 'Приём от клиента', to: true, task: true, equipment: 'task' },
    // priced — накладная: товарные строки несут цену продажи, у документа есть сумма (DS-011)
    warehouse_to_client: { label: 'Возврат клиенту', from: true, task: true, products: true, priced: true, equipment: 'task' },
    warehouse_to_warehouse: { label: 'Склад → склад', from: true, to: true, products: true, equipment: 'search' },
    supplier_to_warehouse: { label: 'Приход от поставщика', to: true, supplier: true, products: true }
  };
  function dirOf(t) { return DIR[t && t.direction] || DIR.warehouse_to_warehouse; }

  var STATUS = {
    requested: { label: 'Заявка', cls: 'text-bg-primary' },
    in_transit: { label: 'В пути', cls: 'text-bg-warning' },
    received: { label: 'Получено', cls: 'text-bg-success' },
    cancelled: { label: 'Отменён', cls: 'text-bg-secondary' }
  };

  // Тексты ошибок полей (§6.2, §6.4, §6.6): по имени поля, затем по коду.
  var WAREHOUSE_TEXT = {
    same_warehouse: 'Склады должны различаться',
    not_found: 'Склад не найден',
    not_allowed: 'Этот склад не указывается для выбранного направления'
  };
  var TASK_TEXT = { not_allowed: 'Заказ указывается только для документов с клиентом' };
  var SUPPLIER_TEXT = {
    archived: 'Поставщик в архиве',
    not_found: 'Поставщик не найден',
    not_allowed: 'Поставщик указывается только для прихода от поставщика'
  };
  // Тексты строки по паре «поле: код» (§6.2) — приоритетнее общих ROW_TEXT.
  var ROW_PAIR_TEXT = {
    'kind:not_allowed': 'Этот тип строки недопустим для направления',
    'equipment_id:not_in_task': 'Оборудование не из этого заказа'
  };
  var ROW_TEXT = {
    kind: 'Некорректный тип строки',
    product_id: 'Проверьте товар',
    equipment_id: 'Проверьте оборудование',
    qty_sent: 'Количество должно быть больше нуля'
  };
  // Цена продажи товарной строки накладной (transfers-sales §6.1).
  var PRICE_TEXT = {
    invalid: 'Некорректная цена',
    negative: 'Цена не может быть отрицательной',
    not_allowed: 'Цена указывается только у товара в накладной клиенту'
  };
  var ROW_CODE_TEXT = { duplicate: 'уже есть в другой строке', archived: 'товар в архиве', not_found: 'не найдено' };
  // причина отмены проведения (transfers-revert §8.1)
  var UNRECEIVE_TEXT = {
    required: 'Укажите причину',
    too_long: 'Причина — не более 500 символов',
    invalid_text: 'Недопустимые символы'
  };
  var RECEIVE_TEXT = {
    exceeds_sent: 'Факт больше отправленного',
    negative: 'Факт не может быть отрицательным',
    invalid: 'Факт не может быть отрицательным',
    invalid_format: 'Факт не может быть отрицательным',
    required: 'Укажите факт',
    too_precise: 'Не больше 3 знаков после запятой',
    too_large: 'Слишком большое число'
  };

  var state = {
    list: [],
    total: 0,
    isAdmin: false,
    // фильтры списка (§8.1 п.2)
    status: '',
    warehouseId: '',
    direction: '',
    supplierId: '',
    taskId: '',
    createdFrom: '',
    createdTo: '',
    warehouses: [], // активные склады: [{id, name}]
    products: [],
    suppliers: [], // активные поставщики для шапки документа: [{id, name}]
    taskCard: null, // карточка заказа документа с клиентом (оборудование заказа) | null
    taskLoading: false,
    current: null, // документ | {new: true}
    items: [], // строки редактора (модель; количество читается из DOM)
    warnings: [],
    warningsAtDestination: false, // предупреждения отмены проведения — по складу назначения
    taskError: '',
    busy: false
  };
  var receiveModal = null;
  var unreceiveModal = null;
  var searchTimer = null;
  var searchSeq = 0;
  var taskTimer = null;
  var taskSeq = 0;
  var filterTaskTimer = null;

  // ---------- тосты ----------
  function toast(msg, isError) {
    var box = $('#st-toasts');
    if (!box || !msg) return;
    var el = document.createElement('div');
    el.className = 'alert ' + (isError ? 'alert-danger' : 'alert-success') + ' shadow py-2 mb-2';
    el.setAttribute('role', isError ? 'alert' : 'status');
    el.setAttribute('data-testid', isError ? 'st-toast-error' : 'st-toast');
    el.textContent = msg;
    box.appendChild(el);
    setTimeout(function () { el.remove(); }, TOAST_MS);
  }

  function failureText(err) {
    if (err && err.status) return err.message || 'Не удалось выполнить действие';
    return 'Нет связи'; // fetch отклонён — сети нет; правки формы остаются в полях
  }

  // ---------- загрузка ----------
  // Активные товары для «+ Товар» — постранично (API отдаёт не больше 500 за запрос).
  function loadProducts(offset, acc) {
    return KT.api.get('/api/v1/products?limit=500&offset=' + offset).then(function (r) {
      var items = acc.concat(r.items || []);
      if ((r.items || []).length > 0 && items.length < r.total && items.length < 10000) return loadProducts(items.length, items);
      return items;
    });
  }

  function readWarehouses() {
    state.warehouses = $$('#st-filter-warehouse option[value]').filter(function (o) {
      return o.value && o.dataset.active === 'true';
    }).map(function (o) { return { id: o.value, name: o.dataset.name }; });
  }

  // filterParams — фильтры списка, комбинация по «И». Период — календарные даты как есть.
  function filterParams() {
    var p = new URLSearchParams();
    if (state.status) p.set('status', state.status);
    if (state.warehouseId) p.set('warehouse_id', state.warehouseId);
    if (state.direction) p.set('direction', state.direction);
    if (state.supplierId) p.set('supplier_id', state.supplierId);
    if (state.taskId) p.set('task_id', state.taskId);
    if (state.createdFrom) p.set('created_from', state.createdFrom);
    if (state.createdTo) p.set('created_to', state.createdTo);
    return p;
  }

  function listQuery(offset) {
    var p = filterParams();
    p.set('limit', String(PAGE));
    p.set('offset', String(offset));
    return API + '?' + p.toString();
  }

  // periodError — красный текст под полями периода (400 created_from / created_to).
  function periodError(msg) {
    var el = $('#st-filter-period-error');
    el.textContent = msg || '';
    el.classList.toggle('d-none', !msg);
    $('#st-filter-from').classList.toggle('is-invalid', !!msg);
    $('#st-filter-to').classList.toggle('is-invalid', !!msg);
  }

  function isPeriodError(err) {
    return !!(err && err.status === 400 && (err.fields || []).some(function (f) {
      return f.name === 'created_from' || f.name === 'created_to';
    }));
  }

  // loadList — первая страница (reset) либо следующая («Показать ещё»). Список затеняется на
  // время запроса; 400 периода — текст под полями, 400 остальных фильтров — тост.
  function loadList(append) {
    var box = $('#st-list');
    box.classList.add('opacity-50');
    return KT.api.get(listQuery(append ? state.list.length : 0)).then(function (r) {
      periodError('');
      state.list = append ? state.list.concat(r.items || []) : (r.items || []);
      state.total = r.total || 0;
      renderList();
    }).catch(function (err) {
      if (isPeriodError(err)) { periodError('Некорректный период'); return; }
      toast(err && err.status === 400 ? 'Некорректное значение фильтра' : failureText(err), true);
    }).then(function () { box.classList.remove('opacity-50'); });
  }

  // reloadList — перечитать столько строк, сколько уже показано (после мутации документа).
  function reloadList() {
    var want = Math.max(PAGE, state.list.length);
    var p = filterParams();
    p.set('limit', String(Math.min(want, 500)));
    p.set('offset', '0');
    return KT.api.get(API + '?' + p.toString()).then(function (r) {
      state.list = r.items || [];
      state.total = r.total || 0;
      renderList();
    }).catch(function () { /* список обновится при следующем действии */ });
  }

  // loadSupplierFilter — фильтр «Поставщик» (только admin): все, архивные с пометкой.
  function loadSupplierFilter() {
    var sel = $('#st-filter-supplier');
    if (!sel) return Promise.resolve();
    return KT.api.get('/api/v1/suppliers?limit=500').then(function (r) {
      sel.innerHTML = '<option value="">Все поставщики</option>' + (r.items || []).map(function (sp) {
        return '<option value="' + esc(sp.id) + '">' + esc(sp.name) + (sp.is_active ? '' : ' (архив)') + '</option>';
      }).join('');
      sel.value = state.supplierId;
      if (sel._cbSync) sel._cbSync();
    }).catch(function () { /* фильтр останется пустым */ });
  }

  // loadActiveSuppliers — select «Поставщик» шапки: только активные, на момент открытия формы.
  function loadActiveSuppliers() {
    return KT.api.get('/api/v1/suppliers?is_active=true&limit=500').then(function (r) {
      state.suppliers = (r.items || []).map(function (sp) { return { id: sp.id, name: sp.name }; });
    }).catch(function () { state.suppliers = []; });
  }

  function setCurrent(t) {
    state.current = t;
    state.items = (t && t.items ? t.items : []).map(function (it) {
      return {
        kind: it.kind, product_id: it.product_id, product_name: it.product_name, unit: it.unit,
        product_is_deleted: !!it.product_is_deleted,
        equipment_id: it.equipment_id, equipment_name: it.equipment_name, equipment_serial: it.equipment_serial,
        equipment_client_name: it.equipment_client_name, qty: it.qty_sent == null ? '' : num(it.qty_sent),
        price: it.price == null ? '' : money(it.price),
        // значения сервера: пока строка не изменена, «Сумма» показывается серверная
        serverQty: it.qty_sent == null ? '' : num(it.qty_sent),
        serverPrice: it.price == null ? '' : money(it.price),
        serverTotal: it.line_total == null ? null : money(it.line_total)
      };
    });
  }

  // prepareEditor — данные редактора заявки по направлению: активные поставщики либо карточка
  // заказа (оборудование заказа для «+ Оборудование»).
  function prepareEditor(t) {
    state.taskCard = null;
    state.taskLoading = false;
    taskSeq++;
    if (!t || !(t.new || t.status === 'requested')) return Promise.resolve();
    var d = dirOf(t);
    if (d.supplier) return loadActiveSuppliers();
    // у заявки удалённого заказа карточки заказа нет (404): её строки правятся как есть,
    // оборудование заказа для «+ Оборудование» не загружается (soft-delete-transfers DS-007).
    if (d.task && t.task_id && !(t.task_is_deleted && !t.new)) return loadTaskCard(t.task_id);
    return Promise.resolve();
  }

  function loadTransfer(id) {
    return KT.api.get(API + '/' + encodeURIComponent(id)).then(function (t) {
      setCurrent(t);
      setUrl(t.id);
      return prepareEditor(t).then(function () {
        renderCard();
        renderList();
      });
    });
  }

  // openTransfer — выбор документа / ?id=N: 404 → тост, список перезапрашивается.
  function openTransfer(id) {
    state.warnings = [];
    return loadTransfer(id).catch(function (err) {
      if (err && err.status === 404) {
        toast('Перемещение не найдено', true);
        setCurrent(null);
        setUrl(null);
        renderCard();
        return loadList(false);
      }
      toast(failureText(err), true);
    });
  }

  // setUrl — ?id=N открытого документа; у нового документа с клиентом — ?new=1&direction=…
  // &task_id=N (после reload форма откроется снова, §3 Offline).
  function setUrl(id) {
    var u = new URL(window.location.href);
    var t = state.current;
    ['id', 'new', 'direction', 'task_id'].forEach(function (k) { u.searchParams.delete(k); });
    if (id) {
      u.searchParams.set('id', id);
    } else if (t && t.new && dirOf(t).task) {
      u.searchParams.set('new', '1');
      u.searchParams.set('direction', t.direction);
      if (t.task_id) u.searchParams.set('task_id', t.task_id);
    }
    window.history.replaceState(null, '', u.pathname + u.search);
  }

  // ---------- список ----------
  function fmtDateTime(s) { return KT.time.fmtDateTime(s); }
  function fmtDate(s) { return KT.time.fmtDate(s); }

  function badge(status) {
    var s = STATUS[status] || { label: status, cls: 'text-bg-secondary' };
    return '<span class="badge ' + s.cls + '" data-testid="st-status-badge" data-status="' + esc(status) + '">' + esc(s.label) + '</span>';
  }

  // routeHtml — маршрут строки списка: «Клиент → Алматы», «Алматы → Клиент», «Алматы → Москва»,
  // «Поставщик → Алматы» — с серыми бейджами «удалён» у мягко удалённых складов
  // (soft-delete-warehouses-suppliers §8.3 п.1): склад отправления / назначения.
  function routeHtml(t) {
    var d = dirOf(t);
    var from = d.from ? esc(t.from_warehouse_name || '') + (t.from_warehouse_is_deleted ? ' ' + deletedBadge('st-row-from-deleted') : '')
      : (d.supplier ? 'Поставщик' : 'Клиент');
    var to = d.to ? esc(t.to_warehouse_name || '') + (t.to_warehouse_is_deleted ? ' ' + deletedBadge('st-row-to-deleted') : '') : 'Клиент';
    return from + ' → ' + to;
  }

  // partyText — контрагент под маршрутом: клиент и «заказ #N» либо название поставщика.
  function partyText(t) {
    var d = dirOf(t);
    if (d.task) return [t.client_name, t.task_id ? 'заказ #' + t.task_id : ''].filter(Boolean).join(' · ');
    if (d.supplier) return t.supplier_name || '';
    return '';
  }

  // deletedBadge — серый бейдж «удалён» у клиента документа и у строки-единицы
  // (soft-delete-clients §8.3 п.4, UI-конвенции DS-033).
  function deletedBadge(testid) {
    return '<span class="badge text-bg-secondary" data-testid="' + testid + '">удалён</span>';
  }

  function directionBadge(t) {
    return '<span class="badge text-bg-light border" data-testid="st-row-direction" data-direction="' + esc(t.direction) + '">' + esc(dirOf(t).label) + '</span>';
  }

  function renderList() {
    var box = $('#st-list');
    var curId = state.current && state.current.id;
    box.innerHTML = state.list.map(function (t) {
      var active = t.id === curId;
      var party = partyText(t);
      return '<button type="button" class="list-group-item list-group-item-action' + (active ? ' active' : '') + '"' +
        ' data-testid="st-row" data-id="' + esc(t.id) + '" data-direction="' + esc(t.direction) + '"' + (active ? ' aria-current="true"' : '') + '>' +
        '<div class="d-flex justify-content-between align-items-center gap-2">' +
        '<span class="fw-semibold text-break" data-testid="st-row-route">#' + esc(t.id) + ' · ' + routeHtml(t) + '</span>' +
        '<span class="d-flex flex-wrap justify-content-end align-items-center gap-1">' + directionBadge(t) + badge(t.status) +
        // документ, возвращённый откатом в «В пути» (transfers-revert §8.1 п.3)
        (t.status === 'in_transit' && t.unreceived_at ? '<span data-testid="st-row-unreceived" title="Проведение отменено" aria-label="Проведение отменено">↩</span>' : '') +
        '</span></div>' +
        (party ? '<div class="small text-break" data-testid="st-row-party">' + esc(party) +
          // документ мягко удалённого заказа (soft-delete-transfers §8.1 п.1)
          (t.client_is_deleted ? ' ' + deletedBadge('st-row-client-deleted') : '') +
          // поставщик документа мягко удалён (soft-delete-warehouses-suppliers §8.3 п.1)
          (t.supplier_is_deleted ? ' ' + deletedBadge('st-row-supplier-deleted') : '') +
          (t.task_is_deleted ? ' ' + taskDeletedBadge('st-row-task-deleted') : '') + '</div>' : '') +
        // сумма накладной — справа под маршрутом, только у «возврата клиенту» с товарами (§8.1 п.1)
        (dirOf(t).priced && Number(t.amount) > 0 ? '<div class="small fw-semibold text-end" data-testid="st-row-amount">' + esc(money(t.amount)) + '</div>' : '') +
        '<div class="small' + (active ? '' : ' text-body-secondary') + '">' +
        '<span data-testid="st-row-date">' + esc(fmtDate(t.created_at)) + '</span> · строк: <span data-testid="st-row-count">' + esc(t.items_count) + '</span>' +
        (t.note ? ' · ' + esc(t.note) : '') + '</div>' +
        '</button>';
    }).join('');
    $('#st-empty').classList.toggle('d-none', state.list.length > 0);
    $('#st-more').classList.toggle('d-none', state.list.length >= state.total);
    var shown = $('#st-shown');
    shown.textContent = state.list.length + ' из ' + state.total;
    shown.classList.toggle('d-none', state.total === 0);
  }

  // ---------- документ ----------
  function warehouseOptions(selected) {
    var list = state.warehouses.slice();
    if (selected && !list.some(function (w) { return w.id === selected; })) {
      // склад заявки ушёл в архив — показать его, чтобы не потерять значение
      var cur = state.current || {};
      var name = cur.from_warehouse_id === selected ? cur.from_warehouse_name : cur.to_warehouse_name;
      list.push({ id: selected, name: (name || selected) + ' (архив)' });
    }
    return '<option value="">— выберите склад —</option>' + list.map(function (w) {
      return '<option value="' + esc(w.id) + '"' + (w.id === selected ? ' selected' : '') + '>' + esc(w.name) + '</option>';
    }).join('');
  }

  function supplierOptions(selected) {
    var list = state.suppliers.slice();
    if (selected && !list.some(function (sp) { return sp.id === selected; })) {
      // поставщик заявки ушёл в архив либо удалён — показать его, чтобы не потерять значение
      // (TS-020; удалённый поставщик не мешает довести начатый документ, DS-047)
      var curDoc = state.current || {};
      list.push({ id: selected, name: (curDoc.supplier_name || selected) + (curDoc.supplier_is_deleted ? ' (удалён)' : ' (архив)') });
    }
    return '<option value="">— выберите поставщика —</option>' + list.map(function (sp) {
      return '<option value="' + esc(sp.id) + '"' + (sp.id === selected ? ' selected' : '') + '>' + esc(sp.name) + '</option>';
    }).join('');
  }

  // refreshSupplierOptions — список активных поставщиков пришёл после отрисовки формы: меняются
  // ТОЛЬКО опции «Поставщик» (уже сделанный выбор сохраняется). Вся карточка не перерисовывается:
  // иначе пересоздавались бы «Куда», «+ Товар» и строки — введённое пользователем до прихода
  // ответа терялось, а выбор из уже открытого списка уходил в отсоединённый select.
  function refreshSupplierOptions() {
    var sel = $('#st-supplier');
    if (!sel) return;
    var chosen = sel.value || (state.current && state.current.supplier_id) || '';
    sel.innerHTML = supplierOptions(chosen);
    sel.value = chosen;
    if (sel._cbSync) sel._cbSync();
  }

  function productOptions() {
    var used = {};
    state.items.forEach(function (it) { if (it.kind === 'product') used[it.product_id] = true; });
    return '<option value="">+ Товар</option>' + state.products.filter(function (p) { return !used[p.id]; }).map(function (p) {
      return '<option value="' + esc(p.id) + '">' + esc(p.name) + (p.unit ? ', ' + esc(p.unit) : '') + '</option>';
    }).join('');
  }

  // refreshProductOptions — каталог товаров пришёл после отрисовки формы: меняются ТОЛЬКО опции
  // «+ Товар», остальная форма не трогается.
  function refreshProductOptions() {
    var sel = $('#st-add-product');
    if (!sel) return;
    sel.innerHTML = productOptions();
    sel.value = '';
    if (sel._cbSync) sel._cbSync();
  }

  function kindBadge(kind) {
    return kind === 'equipment'
      ? '<span class="badge text-bg-info" data-testid="st-item-kind">оборудование</span>'
      : '<span class="badge text-bg-light border" data-testid="st-item-kind">товар</span>';
  }

  function itemName(it) {
    if (it.kind === 'equipment') {
      var sub = [];
      if (it.equipment_serial) sub.push('S/N ' + esc(it.equipment_serial));
      if (it.equipment_client_name) sub.push(esc(it.equipment_client_name));
      return '<div data-testid="st-item-name">' + esc(it.equipment_name) +
        (it.equipment_is_deleted ? ' ' + deletedBadge('st-item-equipment-deleted') : '') + '</div>' +
        (sub.length ? '<div class="small text-body-secondary">' + sub.join(' · ') + '</div>' : '');
    }
    // товар строки удалён из каталога (soft-delete-catalog §8.3 п.4) — серый бейдж «удалён»
    return '<div data-testid="st-item-name">' + esc(it.product_name) + (it.unit ? ', ' + esc(it.unit) : '') +
      (it.product_is_deleted ? ' ' + deletedBadge('st-item-product-deleted') : '') + '</div>';
  }

  // priceCellsHTML — ячейки «Цена» и «Сумма» строки редактора накладной: у товара — текстовый
  // инпут цены и сумма строки, у строки-единицы обе пустые (§8.1 п.2).
  // catalogPrice — цена товара строки в загруженном каталоге («» — товара в каталоге нет).
  function catalogPrice(it) {
    var p = state.products.filter(function (x) { return x.id === it.product_id; })[0];
    return p ? money(p.price) : '';
  }

  // rowCents — «Сумма» строки редактора в копейках: строка не менялась после загрузки —
  // line_total сервера; цена не указана — по цене каталога (её сервер и подставит, DS-039).
  function rowCents(it, price, qty) {
    if (it.serverTotal != null && price === it.serverPrice && qty === it.serverQty) {
      var saved = toScaled(it.serverTotal, 2);
      if (saved !== null) return saved;
    }
    return lineCents(price === '' ? catalogPrice(it) : price, qty);
  }

  function priceCellsHTML(it) {
    if (it.kind !== 'product') return '<td></td><td></td>';
    var price = it.price == null ? '' : it.price;
    return '<td style="min-width:110px;width:130px">' +
      '<input type="text" inputmode="decimal" autocomplete="off" class="form-control form-control-sm text-end" data-testid="st-item-price" aria-label="Цена" placeholder="' + esc(catalogPrice(it)) + '" value="' + esc(price) + '">' +
      '<div class="text-danger small d-none" data-testid="st-item-price-error" data-item-error data-price-error></div></td>' +
      '<td class="text-end text-nowrap" data-testid="st-item-total">' + esc(fmtCents(rowCents(it, price, it.qty == null ? '' : it.qty))) + '</td>';
  }

  function editRowHTML(it, i) {
    var priced = dirOf(state.current).priced;
    var key = it.kind === 'equipment' ? it.equipment_id : it.product_id;
    return '<tr data-testid="st-item-row" data-kind="' + esc(it.kind) + '" data-key="' + esc(key) + '" data-index="' + i + '">' +
      '<td>' + kindBadge(it.kind) + '</td>' +
      '<td>' + itemName(it) + '<div class="text-danger small d-none" data-testid="st-item-error" data-item-error></div></td>' +
      '<td style="width:130px">' + (it.kind === 'product'
        ? '<input type="number" step="any" min="0" class="form-control form-control-sm" data-testid="st-item-qty" aria-label="Количество" value="' + esc(it.qty) + '">'
        : '<span class="text-body-secondary">—</span>') + '</td>' +
      (priced ? priceCellsHTML(it) : '') +
      '<td style="width:44px" class="text-end"><button type="button" class="btn btn-sm btn-outline-danger" data-testid="st-item-remove" title="Убрать строку" aria-label="Убрать строку">×</button></td>' +
      '</tr>';
  }

  // viewRowHTML — строка просмотра. Расхождение факта с количеством подсвечено в любую
  // сторону (data-diff; приход от поставщика — и недопоставка, и излишек, DS-015); недостача
  // дополнительно помечена data-loss.
  function viewRowHTML(it, showFact, priced) {
    var hasFact = showFact && it.kind === 'product' && it.qty_received != null;
    var diff = hasFact && Number(it.qty_received) !== Number(it.qty_sent);
    var loss = hasFact && Number(it.qty_received) < Number(it.qty_sent);
    return '<tr data-testid="st-view-item" data-kind="' + esc(it.kind) + '" data-key="' + esc(it.kind === 'equipment' ? it.equipment_id : it.product_id) + '">' +
      '<td>' + kindBadge(it.kind) + '</td>' +
      '<td>' + itemName(it) + '</td>' +
      '<td data-testid="st-view-sent">' + (it.kind === 'product' ? esc(num(it.qty_sent)) : '<span class="text-body-secondary">—</span>') + '</td>' +
      (showFact ? '<td data-testid="st-view-received"' + (diff ? ' class="text-danger fw-semibold" data-diff="true"' + (loss ? ' data-loss="true"' : '') + ' title="Расхождение с количеством документа"' : '') + '>' +
        (it.kind === 'product' ? esc(num(it.qty_received)) : '<span class="text-body-secondary">—</span>') + '</td>' : '') +
      (priced ? '<td class="text-end text-nowrap" data-testid="st-view-price">' + (it.price == null ? '' : esc(money(it.price))) + '</td>' +
        '<td class="text-end text-nowrap" data-testid="st-view-total">' + (it.line_total == null ? '' : esc(money(it.line_total))) + '</td>' : '') +
      '</tr>';
  }

  // Жёлтый блок (DS-052): app.css перекрашивает .alert-warning в нейтральный серо-синий
  // (--bs-secondary), поэтому цвета предупреждения заданы родными warning-токенами Bootstrap
  // 5.3 (bg-subtle / text-emphasis / border-subtle) — они свои у светлой и тёмной темы.
  function warningsHTML() {
    if (!state.warnings.length) return '';
    return '<div class="alert alert-warning bg-warning-subtle text-warning-emphasis border-warning-subtle" data-testid="st-warnings"><div class="fw-semibold mb-1">' + (state.warningsAtDestination ? 'Остатки на складе назначения:' : 'Остатки на складе отправления:') + '</div><ul class="mb-0">' +
      state.warnings.map(function (w) {
        return '<li>' + esc(w.name) + ' (' + esc(w.warehouse_name) + '): ' + esc(num(w.stock)) +
          (w.kind === 'negative' ? ' — отрицательный остаток' : ' — на минимуме или ниже (' + esc(num(w.min_stock)) + ')') + '</li>';
      }).join('') + '</ul></div>';
  }

  function headerHTML(t) {
    if (t.new) return '<h2 class="fs-5 mb-3" data-testid="st-card-title">Новое перемещение</h2>';
    return '<div class="d-flex justify-content-between align-items-center gap-2 mb-3">' +
      '<h2 class="fs-5 mb-0" data-testid="st-card-title">Перемещение #' + esc(t.id) + '</h2>' +
      '<span data-testid="st-card-status">' + badge(t.status) + '</span></div>';
  }

  // metaHTML — сведения: создан / отправлен / получен / отменён — кто и когда (§8.1 п.8).
  function metaHTML(t) {
    if (t.new) return '';
    var meta = ['Создан: ' + esc(t.created_by_name) + ', ' + esc(fmtDateTime(t.created_at))];
    if (t.sent_at) meta.push('Отправлен: ' + esc(t.sent_by_name || '') + ', ' + esc(fmtDateTime(t.sent_at)));
    if (t.received_at) meta.push('Получен: ' + esc(t.received_by_name || '') + ', ' + esc(fmtDateTime(t.received_at)));
    if (t.cancelled_at) meta.push('Отменён: ' + esc(t.cancelled_by_name || '') + ', ' + esc(fmtDateTime(t.cancelled_at)));
    // последняя отмена проведения — остаётся и после повторной приёмки (transfers-revert DS-044)
    if (t.unreceived_at) {
      meta.push('<span class="text-break" data-testid="st-card-unreceived">Проведение отменено: ' + esc(t.unreceived_by_name || '') + ', ' +
        esc(fmtDateTime(t.unreceived_at)) + ' — ' + esc(t.unreceive_reason || '') + '</span>');
    }
    return '<div class="small text-body-secondary mt-3" data-testid="st-card-meta">' + meta.join('<br>') + '</div>';
  }

  // qtyHead — заголовок колонки количества: в приходе от поставщика — «Заказано» (§8.1 п.7).
  // priceHeadHTML — заголовки колонок «Цена» и «Сумма» накладной.
  function priceHeadHTML(t) {
    return dirOf(t).priced ? '<th class="text-end" data-testid="st-price-head">Цена</th><th class="text-end" data-testid="st-total-head">Сумма</th>' : '';
  }

  // amountHTML — «Сумма накладной» справа под таблицей (§8.1 п.4): в заявке пересчитывается на
  // клиенте при вводе (updateAmount), после сохранения — значение сервера.
  function amountHTML(t) {
    if (!dirOf(t).priced) return '';
    return '<div class="text-end fw-semibold mb-3" data-testid="st-amount-row">Сумма накладной: <span data-testid="st-amount">' + esc(money(t.amount)) + '</span></div>';
  }

  function qtyHead(t) { return dirOf(t).supplier ? 'Заказано' : 'Кол-во'; }

  function directionHTML(t) {
    return '<div class="mb-2"><span class="text-body-secondary">Направление:</span> ' +
      '<span class="fw-semibold" data-testid="st-direction" data-direction="' + esc(t.direction) + '">' + esc(dirOf(t).label) + '</span></div>';
  }

  // taskDeletedBadge — серый бейдж «заказ удалён» справа от номера заказа (soft-delete-transfers
  // §8.1, UI-конвенции DS-033).
  function taskDeletedBadge(testid) {
    return '<span class="badge text-bg-secondary" data-testid="' + testid + '">заказ удалён</span>';
  }

  // taskLinkHTML — заказ документа: ссылка на карточку + название и клиент (только чтение).
  // Заказ мягко удалён — номер текстом без ссылки (карточки у него нет) и бейдж «заказ удалён».
  function taskLinkHTML(t) {
    var head = t.task_is_deleted
      ? '<span data-testid="st-task-text">Заказ #' + esc(t.task_id) + '</span> ' + taskDeletedBadge('st-task-deleted')
      : '<a href="/?task=' + encodeURIComponent(t.task_id) + '" data-testid="st-task-link">Заказ #' + esc(t.task_id) + '</a>';
    return head +
      (t.task_title ? ' «' + esc(t.task_title) + '»' : '') +
      (t.client_name ? ' · <span data-testid="st-task-client">' + esc(t.client_name) + '</span>' +
        (t.client_is_deleted ? ' ' + deletedBadge('st-task-client-deleted') : '') : '');
  }

  // taskFieldHTML — поле «Заказ» (документы с клиентом): у нового документа — номер, под ним
  // название заказа и клиент; после создания — только чтение (заказ не меняется, DS-012).
  function taskFieldHTML(t) {
    if (!t.new) return '<div class="mb-2" data-testid="st-task-readonly">' + taskLinkHTML(t) + '</div>';
    return '<div class="mb-2"><label class="form-label" for="st-task">Заказ *</label>' +
      '<input type="text" inputmode="numeric" class="form-control" id="st-task" data-testid="st-task" style="max-width:200px" placeholder="Номер заказа" autocomplete="off" value="' + esc(t.task_id || '') + '">' +
      '<div class="form-text" id="st-task-info" data-testid="st-task-info"></div>' +
      '<div class="text-danger small d-none" data-field-error="task_id" data-testid="st-task-error"></div></div>';
  }

  function warehouseFieldHTML(id, field, label, value) {
    return '<div class="col-sm-6"><label class="form-label" for="' + id + '">' + label + ' *</label>' +
      '<select class="form-select" id="' + id + '" data-testid="' + id + '">' + warehouseOptions(value) + '</select>' +
      '<div class="text-danger small d-none" data-field-error="' + field + '" data-testid="' + id + '-error"></div></div>';
  }

  function editorHTML(t) {
    var d = dirOf(t);
    var html = directionHTML(t);
    if (d.task) html += taskFieldHTML(t);
    if (d.supplier) {
      html += '<div class="mb-2"><label class="form-label" for="st-supplier">Поставщик *</label>' +
        '<select class="form-select" id="st-supplier" data-testid="st-supplier">' + supplierOptions(t.supplier_id) + '</select>' +
        '<div class="text-danger small d-none" data-field-error="supplier_id" data-testid="st-supplier-error"></div></div>';
    }
    html += '<div class="row g-2 mb-2">' +
      (d.from ? warehouseFieldHTML('st-from', 'from_warehouse_id', 'Склад отправления', t.from_warehouse_id) : '') +
      (d.to ? warehouseFieldHTML('st-to', 'to_warehouse_id', 'Склад назначения', t.to_warehouse_id) : '') +
      '</div>' +
      '<div class="mb-3"><label class="form-label" for="st-note">Примечание</label>' +
      '<textarea class="form-control" id="st-note" data-testid="st-note" rows="2" maxlength="1000">' + esc(t.note || '') + '</textarea>' +
      '<div class="text-danger small d-none" data-field-error="note" data-testid="st-note-error"></div></div>' +
      '<div class="text-danger small mb-1 d-none" data-field-error="items" data-testid="st-items-error"></div>' +
      '<div class="table-responsive"><table class="table table-sm align-middle mb-2" data-testid="st-items">' +
      '<thead><tr><th>Тип</th><th>Наименование</th><th data-testid="st-qty-head">' + qtyHead(t) + '</th>' + priceHeadHTML(t) + '<th></th></tr></thead><tbody id="st-items-tbody"></tbody></table></div>' +
      amountHTML(t) +
      '<div class="row g-2 mb-3">';
    if (d.products) {
      html += '<div class="col-sm-6"><select class="form-select form-select-sm" id="st-add-product" data-testid="st-add-product" aria-label="Добавить товар"></select></div>';
    }
    if (d.equipment === 'search') {
      html += '<div class="col-sm-6 position-relative" data-testid="st-add-equipment">' +
        '<input type="search" class="form-control form-control-sm" id="st-equipment-search" data-testid="st-equipment-search" placeholder="+ Оборудование" autocomplete="off" aria-label="Добавить оборудование">' +
        '<div class="form-text" id="st-equipment-hint" data-testid="st-equipment-hint"></div>' +
        '<div class="text-danger small d-none" id="st-equipment-error" data-testid="st-equipment-error"></div>' +
        '<div class="list-group position-absolute shadow d-none" id="st-equipment-results" data-testid="st-equipment-results" style="z-index:1050;left:0;right:0;max-height:260px;overflow:auto"></div>' +
        '</div>';
    }
    if (d.equipment === 'task') {
      // «+ Оборудование» документа с клиентом — список оборудования заказа (§8.1 п.5)
      html += '<div class="col-12"><div class="small fw-semibold mb-1">+ Оборудование заказа</div>' +
        '<div class="list-group" id="st-task-equipment" data-testid="st-task-equipment"></div></div>';
    }
    html += '</div>' +
      '<div class="d-flex flex-wrap justify-content-end gap-2">' +
      (t.new ? '' : '<button type="button" class="btn btn-outline-secondary me-auto" data-testid="st-cancel" data-act="cancel">Отменить</button>') +
      (t.new ? '' : deleteButtonHTML()) +
      '<button type="button" class="btn btn-outline-primary" data-testid="st-save" data-act="save">Сохранить</button>';
    if (t.new && t.direction === 'client_to_warehouse') {
      // клиент принёс сам: создать, отправить и принять одним действием (DS-013)
      html += '<button type="button" class="btn btn-outline-primary" data-testid="st-send" data-act="send">Отправить</button>' +
        '<button type="button" class="btn btn-primary" data-testid="st-receive-now" data-act="receive-now">Принять сразу</button>';
    } else {
      html += '<button type="button" class="btn btn-primary" data-testid="st-send" data-act="send">Отправить</button>';
    }
    return html + '</div>';
  }

  function viewHTML(t) {
    var d = dirOf(t);
    var showFact = t.status === 'received' && !d.task;
    var html = directionHTML(t) + '<dl class="row mb-2">' +
      (d.task ? '<dt class="col-sm-4">Заказ</dt><dd class="col-sm-8 text-break" data-testid="st-view-task">' + taskLinkHTML(t) + '</dd>' : '') +
      (d.supplier ? '<dt class="col-sm-4">Поставщик</dt><dd class="col-sm-8 text-break" data-testid="st-view-supplier">' + esc(t.supplier_name || '') +
        (t.supplier_is_deleted ? ' ' + deletedBadge('st-view-supplier-deleted') : '') + '</dd>' : '') +
      (d.from ? '<dt class="col-sm-4">Склад отправления</dt><dd class="col-sm-8" data-testid="st-view-from">' + esc(t.from_warehouse_name || '') +
        (t.from_warehouse_is_deleted ? ' ' + deletedBadge('st-view-from-deleted') : '') + '</dd>' : '') +
      (d.to ? '<dt class="col-sm-4">Склад назначения</dt><dd class="col-sm-8" data-testid="st-view-to">' + esc(t.to_warehouse_name || '') +
        (t.to_warehouse_is_deleted ? ' ' + deletedBadge('st-view-to-deleted') : '') + '</dd>' : '') +
      (t.note ? '<dt class="col-sm-4">Примечание</dt><dd class="col-sm-8 text-break" data-testid="st-view-note">' + esc(t.note) + '</dd>' : '') +
      '</dl>' +
      '<div class="table-responsive"><table class="table table-sm align-middle" data-testid="st-items">' +
      '<thead><tr><th>Тип</th><th>Наименование</th><th data-testid="st-qty-head">' + qtyHead(t) + '</th>' +
      (showFact ? '<th data-testid="st-fact-head">' + (d.supplier ? 'Получено' : 'Факт') + '</th>' : '') + priceHeadHTML(t) + '</tr></thead><tbody>' +
      t.items.map(function (it) { return viewRowHTML(it, showFact, d.priced); }).join('') + '</tbody></table></div>' +
      amountHTML(t);
    if (t.status === 'in_transit') {
      html += '<div class="d-flex flex-wrap justify-content-end gap-2">' +
        '<button type="button" class="btn btn-outline-secondary me-auto" data-testid="st-cancel" data-act="cancel">Отменить отправку</button>' +
        '<button type="button" class="btn btn-primary" data-testid="st-receive" data-act="receive">Принять</button>' +
        '</div>';
    }
    // transfers-revert §8.1: «Отменить проведение» (Получено) и «Удалить» (Отменён) — только admin
    if (state.isAdmin && t.status === 'received') {
      html += '<div class="d-flex flex-wrap justify-content-end gap-2">' +
        '<button type="button" class="btn btn-outline-secondary" data-testid="st-unreceive" data-act="unreceive">Отменить проведение</button></div>';
    }
    if (state.isAdmin && t.status === 'cancelled') {
      html += '<div class="d-flex flex-wrap justify-content-end gap-2">' + deleteButtonHTML() + '</div>';
    }
    return html;
  }

  // deleteButtonHTML — «Удалить» (только admin; «Заявка» и «Отменён», transfers-revert DS-025;
  // удаление мягкое, с причиной — soft-delete-transfers DS-018).
  function deleteButtonHTML() {
    return state.isAdmin ? '<button type="button" class="btn btn-danger" data-testid="st-delete" data-act="delete">Удалить</button>' : '';
  }

  function renderCard() {
    var t = state.current;
    $('#st-placeholder').classList.toggle('d-none', !!t);
    var card = $('#st-card');
    card.classList.toggle('d-none', !t);
    if (!t) { card.innerHTML = ''; return; }
    var editable = t.new || t.status === 'requested';
    card.innerHTML = headerHTML(t) + warningsHTML() + (editable ? editorHTML(t) : viewHTML(t)) + metaHTML(t);
    if (editable) {
      renderRows();
      syncEquipmentSearch();
      renderTaskInfo();
      renderTaskEquipment();
    }
  }

  function emptyRowsText(t) {
    var d = dirOf(t);
    if (d.equipment === 'task' && d.products) return 'Строк нет — добавьте оборудование заказа или товар';
    if (d.equipment === 'task') return 'Строк нет — добавьте оборудование заказа';
    if (!d.equipment) return 'Строк нет — добавьте товар';
    return 'Строк нет — добавьте товар или оборудование';
  }

  // renderRows — строки редактора из модели + обновление «+ Товар» (без уже добавленных) и
  // списка оборудования заказа (добавленные единицы в нём неактивны).
  function renderRows() {
    var tbody = $('#st-items-tbody');
    if (!tbody) return;
    tbody.innerHTML = state.items.map(editRowHTML).join('') ||
      '<tr data-testid="st-items-empty"><td colspan="' + (dirOf(state.current).priced ? 6 : 4) + '" class="text-body-secondary small">' + esc(emptyRowsText(state.current)) + '</td></tr>';
    refreshProductOptions();
    renderTaskEquipment();
    updateAmount();
  }

  // readQty — перенести введённые количества и цены из DOM в модель (перед перерисовкой строк).
  function readQty() {
    $$('#st-items-tbody tr[data-index]').forEach(function (tr) {
      var inp = $('[data-testid="st-item-qty"]', tr);
      var price = $('[data-testid="st-item-price"]', tr);
      var it = state.items[Number(tr.dataset.index)];
      if (inp && it) it.qty = inp.value.trim();
      if (price && it) it.price = price.value.trim();
    });
  }

  // updateAmount — «Сумма» строк и «Сумма накладной» редактора из введённого, без запроса
  // (§8.1 «Ввод цены»).
  function updateAmount() {
    var out = $('#st-card [data-testid="st-amount"]');
    if (!out || !$('#st-items-tbody')) return;
    var cents = BigInt(0);
    $$('#st-items-tbody tr[data-index]').forEach(function (tr) {
      var it = state.items[Number(tr.dataset.index)];
      var qty = $('[data-testid="st-item-qty"]', tr);
      var price = $('[data-testid="st-item-price"]', tr);
      var cell = $('[data-testid="st-item-total"]', tr);
      if (!it || !qty || !price || !cell) return;
      var sum = rowCents(it, price.value.trim(), qty.value.trim());
      cell.textContent = fmtCents(sum);
      cents += sum;
    });
    out.textContent = fmtCents(cents);
  }

  function addProduct(id) {
    var p = state.products.filter(function (x) { return x.id === id; })[0];
    if (!p) return;
    readQty();
    // накладная: цена подставляется из каталога и правится в строке (DS-039)
    state.items.push({
      kind: 'product', product_id: p.id, product_name: p.name, unit: p.unit, qty: '',
      price: dirOf(state.current).priced ? money(p.price) : ''
    });
    renderRows();
    var rows = $$('#st-items-tbody [data-testid="st-item-qty"]');
    if (rows.length) rows[rows.length - 1].focus();
  }

  function addEquipment(e) {
    if (state.items.some(function (it) { return it.kind === 'equipment' && it.equipment_id === e.id; })) return;
    readQty();
    state.items.push({
      kind: 'equipment', equipment_id: e.id,
      equipment_name: [e.type_name, e.brand, e.model].filter(Boolean).join(' '),
      equipment_serial: e.serial, equipment_client_name: e.client_name
    });
    renderRows();
  }

  // ---------- заказ документа с клиентом и его оборудование (§8.1, интерактив 2) ----------
  function taskFieldMessage(msg) {
    var el = $('#st-card [data-field-error="task_id"]');
    if (!el) return;
    el.textContent = msg || '';
    el.classList.toggle('d-none', !msg);
    var inp = $('#st-task');
    if (inp) inp.classList.toggle('is-invalid', !!msg);
  }

  // renderTaskInfo — под полем «Заказ»: название заказа и клиент.
  function renderTaskInfo() {
    var el = $('#st-task-info');
    if (!el) return;
    var c = state.taskCard;
    el.textContent = c ? '«' + c.task.title + '» · ' + c.task.client_name : (state.taskLoading ? 'Загрузка…' : '');
  }

  function unitName(eq) { return [eq.type_name, eq.brand, eq.model].filter(Boolean).join(' '); }

  function unitLocationText(eq) {
    switch (eq.location_status) {
      case 'at_warehouse': return 'на складе ' + (eq.warehouse_name || '');
      case 'in_transit': return 'в пути → ' + (eq.transit_to_name || '');
      default: return 'у клиента';
    }
  }

  // unitAtSource — единица числится в источнике документа: приём — «у клиента»; возврат — на
  // складе отправления (склад ещё не выбран — на любом: выбор единицы подставит её склад).
  function unitAtSource(eq) {
    var t = state.current;
    if (t.direction === 'client_to_warehouse') return eq.location_status === 'at_client';
    var from = $('#st-from') ? $('#st-from').value : '';
    return eq.location_status === 'at_warehouse' && (!from || eq.warehouse_id === from);
  }

  function unitBusy(eq) {
    return eq.active_transfer_id != null && eq.active_transfer_id !== (state.current && state.current.id);
  }

  // renderTaskEquipment — «+ Оборудование» документа с клиентом: оборудование заказа с
  // бейджем местоположения; единицы не из источника, занятые другим документом (DS-019) и
  // уже добавленные — неактивны.
  function renderTaskEquipment() {
    var box = $('#st-task-equipment');
    if (!box) return;
    var c = state.taskCard;
    if (!c) {
      box.innerHTML = '<div class="list-group-item text-body-secondary small" data-testid="st-task-equipment-none">' +
        (state.taskLoading ? 'Загрузка…'
          : (state.current && state.current.task_is_deleted ? 'Заказ удалён — добавить его оборудование нельзя'
            : 'Укажите заказ — здесь появится его оборудование')) + '</div>';
      return;
    }
    var added = {};
    state.items.forEach(function (it) { if (it.kind === 'equipment') added[it.equipment_id] = true; });
    var list = c.equipment || [];
    // мягко удалённые единицы заказа в выборе не предлагаются (soft-delete-clients §8.3 п.5)
    list = list.filter(function (eq) { return !eq.is_deleted; });
    box.innerHTML = list.length ? list.map(function (eq) {
      var busy = unitBusy(eq);
      var off = state.busy || busy || added[eq.equipment_id] || !unitAtSource(eq);
      return '<button type="button" class="list-group-item list-group-item-action' + (off ? ' disabled' : '') + '"' +
        ' data-testid="st-task-equipment-option" data-equipment-id="' + esc(eq.equipment_id) + '"' + (off ? ' disabled aria-disabled="true"' : '') + '>' +
        '<div class="d-flex flex-wrap justify-content-between align-items-center gap-2">' +
        '<span class="text-break">' + esc(unitName(eq)) + (eq.serial ? ' · S/N ' + esc(eq.serial) : '') + '</span>' +
        '<span class="badge text-bg-light border" data-testid="st-task-equipment-location" data-location-status="' + esc(eq.location_status) + '">' + esc(unitLocationText(eq)) + '</span></div>' +
        (busy ? '<div class="small" data-testid="st-task-equipment-busy">в перемещении #' + esc(eq.active_transfer_id) + '</div>' : '') +
        (!busy && added[eq.equipment_id] ? '<div class="small">уже в документе</div>' : '') +
        '</button>';
    }).join('') : '<div class="list-group-item text-body-secondary small" data-testid="st-task-equipment-none">В заказе нет оборудования</div>';
  }

  function unitItem(eq) {
    return {
      kind: 'equipment', equipment_id: eq.equipment_id, equipment_name: unitName(eq),
      equipment_serial: eq.serial, equipment_client_name: state.taskCard.task.client_name
    };
  }

  function addTaskUnit(equipmentId) {
    var c = state.taskCard;
    var eq = c && (c.equipment || []).filter(function (x) { return x.equipment_id === equipmentId; })[0];
    if (!eq || state.items.some(function (it) { return it.kind === 'equipment' && it.equipment_id === equipmentId; })) return;
    var from = $('#st-from');
    if (from && !from.value && eq.warehouse_id) {
      // возврат клиенту: склад отправления — склад выбранной единицы
      from.value = eq.warehouse_id;
      if (from._cbSync) from._cbSync();
    }
    state.items.push(unitItem(eq));
    renderRows();
  }

  // prefillFromTask — новый документ из карточки заказа (§8.2): приём — единицы «у клиента» и
  // первый активный склад; возврат — единицы на складе, склад отправления — склад первой из
  // них (единицы с другого склада не выбираются). Единица, уже стоящая в незавершённом
  // документе, НЕ предвыбирается (форма иначе гарантированно получала бы 409): она видна в
  // «+ Оборудование» неактивной, с пометкой «в перемещении #N».
  // Форма к приходу карточки заказа уже на экране и доступна пользователю, поэтому предвыбор
  // только ДОПОЛНЯЕТ её: склад, выбранный пользователем, не заменяется (возврат — единицы
  // берутся с него), добавленные им строки остаются. Возвращает true, если добавлены строки.
  function prefillFromTask() {
    var t = state.current;
    var units = (state.taskCard.equipment || []).filter(function (eq) { return !unitBusy(eq); });
    var picked;
    if (t.direction === 'client_to_warehouse') {
      picked = units.filter(function (eq) { return eq.location_status === 'at_client'; });
      if (state.warehouses.length) presetWarehouse($('#st-to'), state.warehouses[0].id);
    } else {
      var onStock = units.filter(function (eq) { return eq.location_status === 'at_warehouse'; });
      var from = $('#st-from');
      var wh = (from && from.value) || (onStock.length ? onStock[0].warehouse_id : '');
      picked = onStock.filter(function (eq) { return eq.warehouse_id === wh; });
      if (picked.length) presetWarehouse(from, wh);
    }
    picked = picked.filter(function (eq) {
      return !state.items.some(function (it) { return it.kind === 'equipment' && it.equipment_id === eq.equipment_id; });
    });
    state.items = state.items.concat(picked.map(unitItem));
    return picked.length > 0;
  }

  // presetWarehouse — подставить склад в ещё не заполненное поле формы.
  function presetWarehouse(sel, id) {
    if (!sel || sel.value || !id) return;
    sel.value = id;
    if (sel._cbSync) sel._cbSync();
  }

  // applyTaskCard — карточка заказа нового документа пришла ПОСЛЕ отрисовки формы: обновляются
  // только подпись заказа, ошибка поля и «+ Оборудование заказа»; предвыбор (prefill) дополняет
  // форму. Вся карточка не перерисовывается: иначе пересоздавались бы склады, примечание и
  // строки — введённое пользователем до прихода ответа терялось, а выбор из уже открытого
  // списка уходил в отсоединённый select.
  function applyTaskCard(prefill) {
    taskFieldMessage(state.taskError || '');
    renderTaskInfo();
    if (!(prefill && state.taskCard && prefillFromTask())) { renderTaskEquipment(); return; }
    // строки перерисовываются только из-за добавленных единиц; ввод и фокус сохраняются
    var active = document.activeElement;
    var row = active && active.closest ? active.closest('#st-items-tbody tr[data-index]') : null;
    var focus = row ? { index: row.dataset.index, testid: active.getAttribute('data-testid') } : null;
    readQty();
    renderRows();
    var again = focus && $('#st-items-tbody tr[data-index="' + focus.index + '"] [data-testid="' + focus.testid + '"]');
    if (again) again.focus();
  }

  // loadTaskCard — карточка заказа документа (название, клиент, оборудование): 404 — под полем
  // «Заказ не найден».
  function loadTaskCard(taskId) {
    var seq = ++taskSeq;
    state.taskCard = null;
    state.taskLoading = true;
    return KT.api.get('/api/v1/tasks/' + encodeURIComponent(taskId) + '/card').then(function (c) {
      if (seq !== taskSeq) return;
      state.taskLoading = false;
      state.taskCard = c;
    }).catch(function (err) {
      if (seq !== taskSeq) return;
      state.taskLoading = false;
      state.taskCard = null;
      state.taskError = err && err.status === 404 ? 'Заказ не найден' : failureText(err);
    });
  }

  // onTaskInput — номер заказа нового документа изменён: прежние единицы (они из другого
  // заказа) снимаются, карточка нового заказа загружается.
  function onTaskInput() {
    var t = state.current;
    if (!t || !t.new) return;
    var raw = $('#st-task').value.trim();
    var id = /^[1-9]\d{0,17}$/.test(raw) ? Number(raw) : 0;
    t.task_id = id || '';
    state.items = state.items.filter(function (it) { return it.kind !== 'equipment'; });
    state.taskCard = null;
    state.taskError = '';
    taskSeq++;
    setUrl(null);
    if (!id) {
      state.taskLoading = false;
      taskFieldMessage(raw ? 'Заказ не найден' : '');
      renderTaskInfo();
      renderRows();
      return;
    }
    taskFieldMessage('');
    var p = loadTaskCard(id);
    renderTaskInfo();
    renderRows();
    p.then(function () {
      if (state.current !== t) return;
      taskFieldMessage(state.taskError || '');
      renderTaskInfo();
      renderRows();
    });
  }

  // ---------- «+ Оборудование»: поиск единиц на складе отправления (§8.1, интерактив 3) ----------
  function syncEquipmentSearch() {
    var input = $('#st-equipment-search');
    if (!input) return;
    var from = $('#st-from').value;
    input.disabled = !from;
    $('#st-equipment-hint').textContent = from ? '' : 'Сначала выберите склад отправления';
    if (!from) { input.value = ''; hideEquipmentResults(); }
  }

  function hideEquipmentResults() {
    var box = $('#st-equipment-results');
    if (box) { box.classList.add('d-none'); box.innerHTML = ''; }
  }

  function equipmentError(msg) {
    var el = $('#st-equipment-error');
    if (!el) return;
    el.textContent = msg || '';
    el.classList.toggle('d-none', !msg);
    $('#st-equipment-search').classList.toggle('is-invalid', !!msg);
  }

  function searchEquipment() {
    var input = $('#st-equipment-search');
    var from = $('#st-from') && $('#st-from').value;
    if (!input || !from) return;
    var seq = ++searchSeq;
    var p = new URLSearchParams();
    p.set('warehouse_id', from);
    p.set('location_status', 'at_warehouse');
    if (input.value.trim()) p.set('q', input.value.trim());
    p.set('limit', '20');
    KT.api.get('/api/v1/equipment?' + p.toString()).then(function (r) {
      if (seq !== searchSeq) return; // пришёл ответ на устаревший ввод
      equipmentError('');
      renderEquipmentResults(r.items || []);
    }).catch(function (err) {
      if (seq !== searchSeq) return;
      hideEquipmentResults();
      var tooLong = err && err.status === 400 && (err.fields || []).some(function (f) { return f.name === 'q' && f.message === 'too_long'; });
      if (tooLong) equipmentError('Слишком длинный запрос');
      else toast(failureText(err), true);
    });
  }

  function renderEquipmentResults(items) {
    var box = $('#st-equipment-results');
    var curId = state.current && state.current.id;
    var added = {};
    state.items.forEach(function (it) { if (it.kind === 'equipment') added[it.equipment_id] = true; });
    box.innerHTML = items.length ? items.map(function (e) {
      // занятые другим документом — неактивны, с пометкой (DS-019)
      var busy = e.active_transfer_id != null && e.active_transfer_id !== curId;
      var off = busy || added[e.id];
      var label = [e.client_name, [e.type_name, e.brand, e.model].filter(Boolean).join(' '), e.serial].filter(Boolean).join(' · ');
      return '<button type="button" class="list-group-item list-group-item-action' + (off ? ' disabled' : '') + '"' +
        ' data-testid="st-equipment-option" data-equipment-id="' + esc(e.id) + '"' + (off ? ' disabled aria-disabled="true"' : '') + '>' +
        '<div>' + esc(label) + '</div>' +
        (busy ? '<div class="small" data-testid="st-equipment-busy">в перемещении #' + esc(e.active_transfer_id) + '</div>' : '') +
        (!busy && added[e.id] ? '<div class="small">уже в документе</div>' : '') +
        '</button>';
    }).join('') : '<div class="list-group-item text-body-secondary small" data-testid="st-equipment-none">На складе отправления оборудования нет</div>';
    box._items = items;
    box.classList.remove('d-none');
  }

  // ---------- ошибки ----------
  function clearErrors() {
    $$('#st-card .is-invalid').forEach(function (n) { n.classList.remove('is-invalid'); });
    $$('#st-card [data-field-error], #st-card [data-item-error]').forEach(function (n) {
      n.textContent = '';
      n.classList.add('d-none');
    });
  }

  function fieldError(name, msg) {
    var el = $('#st-card [data-field-error="' + name + '"]');
    if (!el) return false;
    el.textContent = msg;
    el.classList.remove('d-none');
    return true;
  }

  // rowError — красный текст в строке таблицы (html — уже экранированная разметка).
  function rowError(tr, html) {
    if (!tr) return false;
    var el = $('[data-item-error]', tr);
    if (!el) return false;
    el.innerHTML = el.innerHTML ? el.innerHTML + '; ' + html : html;
    el.classList.remove('d-none');
    return true;
  }

  function rowByEquipment(id) {
    return $$('#st-items-tbody tr[data-kind="equipment"]').filter(function (tr) { return tr.dataset.key === id; })[0];
  }

  // showFieldErrors — 400 details.fields[]: точки маршрута/примечание — под полем, строки — в
  // строке; ошибка поля, которого нет в форме направления, — тостом.
  function showFieldErrors(fields) {
    var rows = $$('#st-items-tbody tr[data-index]');
    var rest = [];
    function under(name, text, controlSel) {
      if (!fieldError(name, text)) { rest.push(text); return; }
      var ctl = controlSel && $(controlSel);
      if (ctl) ctl.classList.add('is-invalid');
    }
    fields.forEach(function (f) {
      var m = /^items\[(\d+)\]\.(\w+)$/.exec(f.name);
      if (m) {
        var tr = rows[Number(m[1])];
        var text = ROW_PAIR_TEXT[m[2] + ':' + f.message];
        if (!text) {
          text = ROW_TEXT[m[2]] || 'Проверьте строку';
          if (ROW_CODE_TEXT[f.message] && m[2] !== 'qty_sent') text += ' (' + ROW_CODE_TEXT[f.message] + ')';
        }
        if (m[2] === 'price') {
          // цена: под полем цены строки; not_allowed в UI недостижим — страховка тостом (§8.1)
          text = PRICE_TEXT[f.message] || PRICE_TEXT.invalid;
          var priceInp = tr && $('[data-testid="st-item-price"]', tr);
          var priceErr = tr && $('[data-price-error]', tr);
          if (f.message === 'not_allowed' || !priceInp || !priceErr) { rest.push(text); return; }
          priceErr.textContent = text;
          priceErr.classList.remove('d-none');
          priceInp.classList.add('is-invalid');
          return;
        }
        if (!rowError(tr, esc(text))) rest.push(text);
        var inp = tr && m[2] === 'qty_sent' && $('[data-testid="st-item-qty"]', tr);
        if (inp) inp.classList.add('is-invalid');
        return;
      }
      switch (f.name) {
        case 'from_warehouse_id':
        case 'to_warehouse_id':
          under(f.name, WAREHOUSE_TEXT[f.message] || 'Проверьте склад', f.name === 'from_warehouse_id' ? '#st-from' : '#st-to');
          break;
        case 'task_id':
          under('task_id', state.current && !state.current.new ? 'Направление и заказ изменить нельзя' : (TASK_TEXT[f.message] || 'Укажите заказ'), '#st-task');
          break;
        case 'supplier_id':
          under('supplier_id', SUPPLIER_TEXT[f.message] || 'Укажите поставщика', '#st-supplier');
          break;
        case 'direction':
          rest.push(f.message === 'not_allowed' ? 'Направление и заказ изменить нельзя' : 'Укажите направление');
          break;
        case 'receive':
          rest.push('Принять сразу можно только приём от клиента');
          break;
        case 'note':
          under('note', 'Слишком длинное примечание', '#st-note');
          break;
        case 'items':
          fieldError('items', f.message === 'not_allowed' ? 'Для этого направления факт не вводится' : 'Добавьте строки (не более 200)');
          break;
        case 'version':
          rest.push('Обновите страницу');
          break;
        default:
          rest.push(f.name + ': ' + f.message);
      }
    });
    if (rest.length) toast(rest.join('; '), true);
  }

  // handleError — ошибка мутации документа. 409 по версии/статусу — тост и перечитывание
  // документа и списка; 409 по единице — красный текст в её строке.
  function handleError(err) {
    if (err && err.status === 400 && err.fields && err.fields.length) {
      showFieldErrors(err.fields);
      return;
    }
    var details = (err && err.details) || {};
    if (err && err.code === 'EQUIPMENT_IN_TRANSFER') {
      var n = details.transfer_id;
      var html = 'уже в перемещении <a href="/transfers?id=' + encodeURIComponent(n) + '" data-testid="st-item-transfer-link">#' + esc(n) + '</a>';
      if (!rowError(rowByEquipment(details.equipment_id), html)) toast(err.message, true);
      return;
    }
    if (err && err.code === 'TRANSFER_EQUIPMENT_NOT_AT_SOURCE') {
      // текст по направлению отдаёт сервер: «…не на складе отправления» / «…уже не у клиента»
      if (!rowError(rowByEquipment(details.equipment_id), esc(err.message || 'Оборудование находится не на складе отправления'))) toast(err.message, true);
      return;
    }
    toast(failureText(err), true);
    if (err && (err.status === 409 || err.status === 404) && state.current && !state.current.new) {
      // документ изменён или уже в другом статусе — показать актуальное состояние
      var id = state.current.id;
      state.warnings = [];
      loadTransfer(id).catch(function () { /* 404 — документ пропал; список ниже */ });
      reloadList();
    }
  }

  // ---------- действия ----------
  // collectForm — тело запроса по направлению: только положенные ему точки маршрута (лишняя →
  // 400 not_allowed). direction и task_id уходят только при создании — потом не меняются.
  function collectForm() {
    readQty();
    var t = state.current;
    var d = dirOf(t);
    var body = { note: $('#st-note').value };
    if (t.new) body.direction = t.direction;
    if (d.from) body.from_warehouse_id = $('#st-from').value;
    if (d.to) body.to_warehouse_id = $('#st-to').value;
    if (d.supplier) body.supplier_id = $('#st-supplier').value;
    // пустой/нечисловой номер заказа не передаётся — сервер ответит task_id: required
    if (d.task && t.new && t.task_id) body.task_id = t.task_id;
    body.items = state.items.map(function (it) {
      if (it.kind === 'equipment') return { kind: 'equipment', equipment_id: it.equipment_id };
      var n = Number(it.qty);
      var row = { kind: 'product', product_id: it.product_id };
      // пустое/нечисловое количество не передаётся — сервер ответит qty_sent: required
      if (it.qty !== '' && !Number.isNaN(n)) row.qty_sent = n;
      // цена — только в накладной; пустая не передаётся — сервер подставит цену каталога (DS-039)
      if (d.priced && it.price !== '' && it.price != null) row.price = Number(it.price);
      return row;
    });
    return body;
  }

  function after(t, okText, atDestination) {
    state.warnings = t.warnings && t.warnings.length ? t.warnings : [];
    state.warningsAtDestination = !!atDestination;
    setCurrent(t);
    setUrl(t.id);
    if (okText) toast(okText, false);
    if (KT.refreshNavBadge) KT.refreshNavBadge();
    return prepareEditor(t).then(function () {
      renderCard();
      return reloadList();
    });
  }

  function setBusy(on) {
    state.busy = on;
    $$('#st-card button, #st-card select, #st-card input, #st-card textarea').forEach(function (b) { b.disabled = on; });
    $$('#st-card select').forEach(function (s) { if (s._cbSync) s._cbSync(); });
    if (!on) {
      syncEquipmentSearch();
      renderTaskEquipment(); // неактивные единицы заказа снова неактивны
    }
  }

  function withBusy(p) {
    clearErrors();
    setBusy(true);
    return p.then(function () { setBusy(false); }, function (err) { setBusy(false); handleError(err); });
  }

  // priceFormatErrors — цена, которая не число («abc»), в JSON-число не превращается: ошибка
  // показывается под полем без запроса, в форме ответа сервера (items[i].price: invalid).
  function priceFormatErrors() {
    readQty();
    if (!dirOf(state.current).priced) return [];
    var out = [];
    state.items.forEach(function (it, i) {
      if (it.kind !== 'product' || it.price === '' || it.price == null) return;
      if (!/^-?\d+(\.\d+)?$/.test(it.price)) out.push({ name: 'items[' + i + '].price', message: 'invalid' });
    });
    return out;
  }

  function save() {
    var t = state.current;
    var bad = priceFormatErrors();
    if (bad.length) return Promise.reject({ status: 400, fields: bad });
    var body = collectForm();
    if (t.new) return KT.api.post(API, body);
    body.version = t.version;
    return KT.api.put(API + '/' + t.id, body);
  }

  function doSave() {
    return withBusy(save().then(function (t) { return after(t, 'Сохранено'); }));
  }

  function doSend() {
    var t = state.current;
    var p;
    var bad = priceFormatErrors();
    if (bad.length) {
      p = Promise.reject({ status: 400, fields: bad });
    } else if (t.new) {
      var body = collectForm();
      body.send = true;
      p = KT.api.post(API, body);
    } else {
      // несохранённые правки заявки сначала сохраняются, затем отправка
      p = save().then(function (saved) {
        setCurrent(saved);
        return KT.api.post(API + '/' + saved.id + '/send', { version: saved.version });
      });
    }
    return withBusy(p.then(function (r) { return after(r, 'Перемещение отправлено'); }));
  }

  // doReceiveNow — «Принять сразу» (новый приём от клиента): создать, отправить и принять одной
  // транзакцией (receive: true, DS-013).
  function doReceiveNow() {
    var body = collectForm();
    body.receive = true;
    return withBusy(KT.api.post(API, body).then(function (r) { return after(r, 'Оборудование принято на склад'); }));
  }

  function doCancel() {
    var t = state.current;
    return KT.uiDialog({
      title: 'Отмена перемещения', text: 'Отменить перемещение #' + t.id + '?',
      ok: t.status === 'in_transit' ? 'Отменить отправку' : 'Отменить перемещение', cancel: 'Отмена', danger: true
    }).then(function (ok) {
      if (!ok) return;
      return withBusy(KT.api.post(API + '/' + t.id + '/cancel', { version: t.version })
        .then(function (r) { return after(r, 'Перемещение отменено'); }));
    });
  }

  // ---------- удаление и отмена проведения (transfers-revert §8.1) ----------
  // doDelete — «Удалить» (admin; «Заявка» / «Отменён»): мягкое удаление с причиной
  // (soft-delete-transfers §8.1) через общее окно «Удаление» (KT.uiDeleteDialog, часть 1).
  // Окно само показывает ошибку причины (400, под полем) и отказы 409 (статус, версия — красный
  // блок) и остаётся открытым; 200 — документ закрывается, строка уходит из списка; 404 / 403
  // — тост. После отказа 409 документ и список перечитываются при закрытии окна.
  function doDelete() {
    var t = state.current;
    var stale = false;
    clearErrors();
    return KT.uiDeleteDialog({
      title: 'Удалить «Перемещение #' + t.id + '»?',
      text: 'Документ можно восстановить в разделе «Удалённые»',
      submit: function (reason) {
        return KT.api.delWith(API + '/' + t.id, { reason: reason, version: t.version }).catch(function (err) {
          if (err && err.status === 409) stale = true;
          throw err;
        });
      },
      onOffline: function () { toast('Нет связи', true); }
    }).then(function (out) {
      var refresh = function () {
        state.warnings = [];
        loadTransfer(t.id).catch(function () { /* 404 — документ пропал; список ниже */ });
        return reloadList();
      };
      if (!out) return stale ? refresh() : undefined; // отмена — причина не сохраняется
      if (out.error) {
        var err = out.error;
        if (err.status === 404) {
          toast('Перемещение не найдено', true);
          state.warnings = [];
          setCurrent(null);
          setUrl(null);
          renderCard();
          return reloadList();
        }
        if (err.status === 400 && (err.fields || []).some(function (f) { return f.name === 'version'; })) {
          toast('Обновите страницу', true);
          return refresh();
        }
        toast(err.status === 403 ? 'Нет доступа' : failureText(err), true);
        return;
      }
      state.busy = false;
      state.warnings = [];
      setCurrent(null);
      setUrl(null);
      renderCard();
      toast('Удалено. Восстановить можно в разделе «Удалённые»', false);
      if (KT.refreshNavBadge) KT.refreshNavBadge();
      return reloadList();
    });
  }

  function unreceiveError(id, html) {
    var el = $(id);
    el.innerHTML = html || '';
    el.classList.toggle('d-none', !html);
    if (id === '#st-unreceive-reason-error') $('#st-unreceive-reason').classList.toggle('is-invalid', !!html);
  }

  // openUnreceive — диалог «Отменить проведение» с обязательной причиной.
  function openUnreceive() {
    var t = state.current;
    $('#st-unreceive-title').textContent = 'Отменить проведение перемещения #' + t.id + '?';
    $('#st-unreceive-reason').value = '';
    unreceiveError('#st-unreceive-reason-error', '');
    unreceiveError('#st-unreceive-error', '');
    $('#st-unreceive-confirm').disabled = false;
    if (!unreceiveModal) unreceiveModal = new bootstrap.Modal($('#st-unreceive-modal'));
    unreceiveModal.show();
  }

  function bindUnreceive() {
    var form = $('#st-unreceive-form');
    if (!form) return;
    var sending = false;
    $('#st-unreceive-modal').addEventListener('shown.bs.modal', function () { $('#st-unreceive-reason').focus(); });
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      if (sending) return;
      var t = state.current;
      unreceiveError('#st-unreceive-reason-error', '');
      unreceiveError('#st-unreceive-error', '');
      sending = true;
      $('#st-unreceive-confirm').disabled = true;
      KT.api.post(API + '/' + t.id + '/unreceive', { version: t.version, reason: $('#st-unreceive-reason').value })
        .then(function (r) {
          unreceiveModal.hide();
          return after(r, 'Проведение отменено', true);
        })
        .catch(function (err) {
          var details = (err && err.details) || {};
          if (err && err.status === 400 && err.fields && err.fields.length) {
            var other = [];
            err.fields.forEach(function (f) {
              if (f.name === 'reason') unreceiveError('#st-unreceive-reason-error', esc(UNRECEIVE_TEXT[f.message] || f.message));
              else other.push('Обновите страницу');
            });
            if (other.length) unreceiveError('#st-unreceive-error', esc(other.join('; ')));
            return;
          }
          if (err && err.code === 'TRANSFER_UNRECEIVE_BLOCKED') {
            // мешающий документ — ссылкой: откат идёт в обратном порядке (DS-023)
            var n = details.transfer_id;
            var link = n ? '<a href="/transfers?id=' + encodeURIComponent(n) + '" data-testid="st-unreceive-blocked-link">#' + esc(n) + '</a>' : '';
            var msg = esc(err.message || '');
            unreceiveError('#st-unreceive-error', n ? msg.replace('#' + esc(n), link) : msg);
            return;
          }
          // 403, 404, 409 (версия, статус), сеть — тост; документ перечитывается
          unreceiveModal.hide();
          handleError(err);
        })
        .then(function () {
          sending = false;
          $('#st-unreceive-confirm').disabled = false;
        });
    });
  }

  // ---------- приёмка ----------
  // receiveClientDoc — документ с клиентом: факт не вводится, подтверждение — диалог без полей.
  function receiveClientDoc() {
    var t = state.current;
    var toClient = t.direction === 'warehouse_to_client';
    return KT.uiDialog({
      title: 'Приёмка', text: toClient ? 'Клиент получил оборудование?' : 'Оборудование на складе?',
      ok: 'Принять', cancel: 'Отмена'
    }).then(function (ok) {
      if (!ok) return;
      return withBusy(KT.api.post(API + '/' + t.id + '/receive', { version: t.version })
        .then(function (r) { return after(r, toClient ? 'Оборудование возвращено клиенту' : 'Оборудование принято на склад'); }));
    });
  }

  function openReceive() {
    var t = state.current;
    if (dirOf(t).task) { receiveClientDoc(); return; }
    var fromSupplier = !!dirOf(t).supplier;
    $('#st-receive-sent-head').textContent = fromSupplier ? 'Заказано' : 'Отправлено';
    $('#st-receive-hint').textContent = fromSupplier
      ? 'Укажите фактически полученное количество: оно может быть меньше или больше заказанного. Остаток склада назначения вырастет на факт.'
      : 'Укажите фактически полученное количество товаров. Недостача будет списана как потеря при перемещении. Оборудование принимается целиком.';
    $('#st-receive-tbody').innerHTML = t.items.map(function (it) {
      if (it.kind === 'equipment') {
        return '<tr data-testid="st-receive-row" data-kind="equipment">' +
          '<td>' + itemName(it) + '</td><td><span class="text-body-secondary">—</span></td>' +
          '<td class="text-body-secondary small" data-testid="st-receive-whole">принимается целиком</td></tr>';
      }
      return '<tr data-testid="st-receive-row" data-kind="product" data-item-id="' + esc(it.id) + '">' +
        '<td>' + itemName(it) + '</td>' +
        '<td>' + esc(num(it.qty_sent)) + '</td>' +
        '<td><input type="number" step="any" min="0" class="form-control form-control-sm" data-testid="st-receive-qty"' +
        ' data-item-id="' + esc(it.id) + '" value="' + esc(num(it.qty_sent)) + '" aria-label="Факт">' +
        '<div class="text-danger small d-none" data-testid="st-receive-qty-error" data-receive-error></div></td></tr>';
    }).join('');
    var err = $('#st-receive-error');
    err.classList.add('d-none');
    err.textContent = '';
    $('#st-receive-confirm').disabled = false;
    if (!receiveModal) receiveModal = new bootstrap.Modal($('#st-receive-modal'));
    receiveModal.show();
  }

  function receiveFieldError(input, msg) {
    input.classList.add('is-invalid');
    var el = $('[data-receive-error]', input.closest('td'));
    el.textContent = msg;
    el.classList.remove('d-none');
  }

  function bindReceive() {
    var receiving = false;
    $('#st-receive-form').addEventListener('submit', function (e) {
      e.preventDefault();
      if (receiving) return;
      var t = state.current;
      var errEl = $('#st-receive-error');
      errEl.classList.add('d-none');
      var inputs = $$('#st-receive-tbody input');
      inputs.forEach(function (inp) {
        inp.classList.remove('is-invalid');
        var el = $('[data-receive-error]', inp.closest('td'));
        el.textContent = '';
        el.classList.add('d-none');
      });
      var bad = false;
      var items = inputs.map(function (inp) {
        var raw = inp.value.trim();
        var n = Number(raw);
        if (raw === '' || Number.isNaN(n)) { bad = true; receiveFieldError(inp, RECEIVE_TEXT.required); }
        return { id: inp.dataset.itemId, qty_received: n };
      });
      if (bad) return;
      receiving = true;
      $('#st-receive-confirm').disabled = true;
      KT.api.post(API + '/' + t.id + '/receive', { version: t.version, items: items })
        .then(function (r) {
          receiveModal.hide();
          return after(r, 'Перемещение получено');
        })
        .catch(function (err) {
          if (err && err.status === 400 && err.fields && err.fields.length) {
            var other = [];
            err.fields.forEach(function (f) {
              var m = /^items\[(\d+)\]\.(\w+)$/.exec(f.name);
              var inp = m && inputs[Number(m[1])];
              if (inp && m[2] === 'qty_received') receiveFieldError(inp, RECEIVE_TEXT[f.message] || f.message);
              else if (inp) receiveFieldError(inp, 'Проверьте строки приёмки');
              else other.push(f.name === 'version' ? 'Обновите страницу' : 'Проверьте строки приёмки');
            });
            if (other.length) {
              errEl.textContent = other.join('; ');
              errEl.classList.remove('d-none');
            }
            return;
          }
          // 409 (версия, статус, STOCK_OVERFLOW) и сеть — тост; документ перечитывается
          receiveModal.hide();
          handleError(err);
        })
        .then(function () {
          receiving = false;
          $('#st-receive-confirm').disabled = false;
        });
    });
  }

  // ---------- события ----------
  // newDocument — форма нового документа направления direction. opts.taskId — заказ (переход
  // из карточки заказа), opts.prefill — предвыбрать единицы и склад (§8.2).
  function newDocument(direction, opts) {
    opts = opts || {};
    if (!DIR[direction] || !(state.isAdmin || DIR[direction].task)) return Promise.resolve();
    state.warnings = [];
    state.taskError = '';
    var t = { new: true, direction: direction, from_warehouse_id: '', to_warehouse_id: '', supplier_id: '', task_id: opts.taskId || '', note: '', items: [] };
    setCurrent(t);
    setUrl(null);
    var withTask = !!(DIR[direction].task && t.task_id);
    var p = withTask ? loadTaskCard(t.task_id)
      : (DIR[direction].supplier ? loadActiveSuppliers() : Promise.resolve());
    if (!withTask) { state.taskCard = null; state.taskLoading = false; }
    var seq = taskSeq;
    renderCard();
    renderList();
    return p.then(function () {
      if (state.current !== t) return;
      // Форма уже нарисована и доступна пользователю — повторно она целиком не рисуется: поля и
      // строки, которые пользователь успел заполнить, не трогаются. Документу заказа догружается
      // карточка заказа (оборудование, предвыбор), приходу от поставщика — список поставщиков.
      if (withTask) {
        // номер заказа за это время изменён — форму ведёт onTaskInput со своей загрузкой
        if (seq === taskSeq) applyTaskCard(!!opts.prefill);
        return;
      }
      if (DIR[direction].supplier) refreshSupplierOptions();
    });
  }

  function setFilterValue(sel, value) {
    sel.value = value;
    if (sel._cbSync) sel._cbSync();
  }

  function bind() {
    $('#st-filter-status').addEventListener('change', function (e) {
      state.status = e.target.value;
      loadList(false);
    });
    $('#st-filter-warehouse').addEventListener('change', function (e) {
      state.warehouseId = e.target.value;
      loadList(false);
    });
    $('#st-filter-direction').addEventListener('change', function (e) {
      state.direction = e.target.value;
      loadList(false);
    });
    var supplierFilter = $('#st-filter-supplier');
    if (supplierFilter) supplierFilter.addEventListener('change', function (e) {
      state.supplierId = e.target.value;
      loadList(false);
    });
    $('#st-filter-task').addEventListener('input', function (e) {
      // номер заказа — debounce 300 мс (§8.1, интерактив 1)
      clearTimeout(filterTaskTimer);
      var value = e.target.value.trim();
      filterTaskTimer = setTimeout(function () {
        state.taskId = value;
        loadList(false);
      }, SEARCH_DEBOUNCE_MS);
    });
    $('#st-filter-from').addEventListener('change', function (e) {
      state.createdFrom = e.target.value;
      loadList(false);
    });
    $('#st-filter-to').addEventListener('change', function (e) {
      state.createdTo = e.target.value;
      loadList(false);
    });
    $('#st-filter-reset').addEventListener('click', function () {
      clearTimeout(filterTaskTimer);
      state.status = '';
      state.warehouseId = '';
      state.direction = '';
      state.supplierId = '';
      state.taskId = '';
      state.createdFrom = '';
      state.createdTo = '';
      setFilterValue($('#st-filter-status'), '');
      setFilterValue($('#st-filter-warehouse'), '');
      setFilterValue($('#st-filter-direction'), '');
      if (supplierFilter) setFilterValue(supplierFilter, '');
      $('#st-filter-task').value = '';
      $('#st-filter-from').value = '';
      $('#st-filter-to').value = '';
      periodError('');
      loadList(false);
    });
    $('#st-more').addEventListener('click', function () { loadList(true); });
    $('#st').addEventListener('click', function (e) {
      var pick = e.target.closest('[data-testid="st-new-direction"]');
      if (pick) { newDocument(pick.dataset.direction); return; }
      if (e.target.closest('[data-st-new]')) {
        // «Новое перемещение» пустого списка — то же меню направлений (после всплытия клика,
        // иначе документный обработчик Bootstrap тут же закроет меню)
        setTimeout(function () { bootstrap.Dropdown.getOrCreateInstance($('#st-new')).show(); }, 0);
      }
    });
    $('#st-list').addEventListener('click', function (e) {
      var row = e.target.closest('[data-testid="st-row"]');
      if (row) openTransfer(row.dataset.id);
    });

    var card = $('#st-card');
    card.addEventListener('click', function (e) {
      if (state.busy) return;
      var opt = e.target.closest('[data-testid="st-equipment-option"]');
      if (opt) {
        if (opt.disabled) return;
        var found = ($('#st-equipment-results')._items || []).filter(function (x) { return x.id === opt.dataset.equipmentId; })[0];
        if (found) addEquipment(found);
        $('#st-equipment-search').value = '';
        hideEquipmentResults();
        return;
      }
      var unit = e.target.closest('[data-testid="st-task-equipment-option"]');
      if (unit) {
        if (!unit.disabled) { readQty(); addTaskUnit(unit.dataset.equipmentId); }
        return;
      }
      var rm = e.target.closest('[data-testid="st-item-remove"]');
      if (rm) {
        readQty();
        state.items.splice(Number(rm.closest('tr').dataset.index), 1);
        renderRows();
        return;
      }
      var act = e.target.closest('[data-act]');
      if (!act) return;
      switch (act.dataset.act) {
        case 'save': doSave(); break;
        case 'send': doSend(); break;
        case 'receive-now': doReceiveNow(); break;
        case 'cancel': doCancel(); break;
        case 'receive': openReceive(); break;
        case 'unreceive': openUnreceive(); break;
        case 'delete': doDelete(); break;
      }
    });
    card.addEventListener('change', function (e) {
      if (e.target.id === 'st-add-product') {
        if (e.target.value) addProduct(e.target.value);
        return;
      }
      if (e.target.id === 'st-from') {
        // единицы ищутся на складе отправления — прежняя выдача неактуальна
        hideEquipmentResults();
        syncEquipmentSearch();
        renderTaskEquipment();
      }
    });
    card.addEventListener('input', function (e) {
      var testid = e.target.getAttribute && e.target.getAttribute('data-testid');
      if (testid === 'st-item-price' || testid === 'st-item-qty') {
        // запятая в цене заменяется на точку; суммы пересчитываются сразу (§8.1 «Ввод цены»)
        if (testid === 'st-item-price' && e.target.value.indexOf(',') !== -1) e.target.value = e.target.value.replace(/,/g, '.');
        updateAmount();
        return;
      }
      if (e.target.id === 'st-task') {
        clearTimeout(taskTimer);
        taskTimer = setTimeout(onTaskInput, SEARCH_DEBOUNCE_MS);
        return;
      }
      if (e.target.id !== 'st-equipment-search') return;
      clearTimeout(searchTimer);
      searchTimer = setTimeout(searchEquipment, SEARCH_DEBOUNCE_MS);
    });
    card.addEventListener('focusin', function (e) {
      if (e.target.id === 'st-equipment-search' && !e.target.disabled) searchEquipment();
    });
    card.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && e.target.id === 'st-equipment-search') hideEquipmentResults();
    });
    document.addEventListener('click', function (e) {
      if (!e.target.closest('[data-testid="st-add-equipment"]')) hideEquipmentResults();
    });
    bindReceive();
    bindUnreceive();
  }

  // ---------- инициализация ----------
  function init() {
    var root = $('#st');
    if (!root) return;
    state.isAdmin = root.dataset.isAdmin === 'true';
    readWarehouses();
    bind();
    var q = new URLSearchParams(window.location.search);
    var id = q.get('id');
    var newTask = parseInt(q.get('task_id'), 10);
    // каталог товаров нужен и исполнителю: «+ Товар» в накладной клиенту (transfers-sales DS-011)
    var products = loadProducts(0, []);
    loadSupplierFilter();
    products.then(function (items) { state.products = items; }, function () { state.products = []; })
      .then(function () {
        // форму нового документа могли открыть из меню ДО прихода каталога — «+ Товар» в ней пуст
        refreshProductOptions();
        return loadList(false);
      })
      .then(function () {
        if (id) return openTransfer(id);
        // новый документ из карточки заказа: ?new=1&direction=…&task_id=N (§8.2)
        if (q.get('new') === '1') return newDocument(q.get('direction'), { taskId: newTask > 0 ? newTask : '', prefill: true });
      });
  }

  KT.transfers = { init: init };
  document.addEventListener('DOMContentLoaded', init);
})();
