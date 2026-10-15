/* KT.board — доска заказов «/» (rich-экран поверх JSON API, D30; эталон логики —
   renderBoardView/cardHTML/renderBoard/moveTask/openColumn в prototype/index.html,
   переложенные на Bootstrap 5 + KT.api/KT.uiDialog/KT.enhanceSelect этого проекта).

   Данные — GET /api/v1/columns + ОДИН GET /api/v1/tasks (ADR-003), фильтры живут в
   location.search. DnD — нативный HTML5 (dragstart/dragover/drop) с оптимистичным
   переносом карточки и откатом при ошибке; на любом экране (в т.ч. ≤576px, где drag
   неудобен) перенос доступен и через меню карточки «⋯ → Переместить в…» (ADR-106).

   Весь пользовательский текст выводится через esc() (HTML-экранирование) — без
   интерпретации HTML в данных (Security §, DoD фазы). */
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
  // moneySum — сумма денежных величин В КОПЕЙКАХ (ADR-005): накопление в float-рублях
  // накапливает погрешность на десятках карточек колонки, поэтому каждое слагаемое
  // округляется до целых копеек ДО суммирования, а результат форматируется обратно.
  function moneySum(amounts) {
    return amounts.reduce(function (sum, v) { return sum + Math.round(num(v) * 100); }, 0);
  }
  function fmtMoneyCents(cents) { return (cents / 100).toFixed(2); }

  var COLORS = ['#94A3B8', '#7C3AED', '#2563EB', '#D97706', '#059669', '#DB2777', '#DC2626', '#0891B2'];

  // ---------- состояние ----------
  var state = {
    columns: [],
    tasks: [],
    assignees: [],
    clients: [],
    categories: [],
    isAdmin: false,
    viewerId: '',
    filters: null,
    editingTaskId: null,
    editingColumnId: null,
    lastKnownTasks: null, // снапшот для отката DnD
    // pagination DS-001/DS-004: columnTotals — M по колонке (column_total сервера, число
    // заказов колонки под фильтрами; при переносе карточки пересчитывается локально);
    // colLoaded — сколько карточек колонки получено С СЕРВЕРА в её порядке (offset
    // «Показать ещё»; перенесённые сюда локально карточки не считаются — дубли по id
    // отсекаются при догрузке); listsCapped — справочник упёрся в страховочный потолок (DS-003).
    columnTotals: {},
    colLoaded: {},
    tasksTotal: 0,
    listsCapped: false,
    // loadGen — поколение загрузки доски (ревью MEDIUM-1): растёт в каждом loadTasks;
    // ответ «Показать ещё», отправленный при старом поколении, игнорируется.
    loadGen: 0,
    // tasksTruncated — первичная выдача обрезана limit=1000 ПОСЛЕ окна per_column (ревью
    // MEDIUM-2): у хвостовых колонок может не прийти ни одной карточки — для колонок без
    // записи в columnTotals тогда показывается «Показать заказы» (догрузка с offset 0).
    tasksTruncated: false
  };

  // pagination DS-001: окно первичной загрузки и размер догрузки на колонку.
  var PER_COLUMN = 50;
  // pagination DS-003: страховочный потолок постраничной загрузки справочников.
  var MAX_LIST_PAGES = 20;

  // boardAbort — все фоновые GET доски (loadStatic/loadTasks/поиск клиента) уходят с этим
  // signal. Отменяем их, как только страница реально начинает уходить — иначе фоновый
  // GET (напр. loadTasks доски), отправленный ДО клика «Выйти», успевает получить ответ
  // 401 УЖЕ после разлогина — HTTP-шум на чужой (по сути) странице (BD-021, найдено сводным
  // E2E-гейтом на AU-005). Отменённый fetch не долетает до 'response' — Playwright §TD-4
  // «чистая сеть» перестаёт его видеть.
  // 'beforeunload'/'pagehide' — общая подстраховка, но на localhost они срабатывают СЛИШКОМ
  // ПОЗДНО: браузер сначала ждёт ответ на сам POST /logout и только потом начинает
  // навигацию/выгрузку документа, а к этому моменту параллельный быстрый localhost-GET
  // (напр. тот же loadTasks) уже успевает получить ответ. Единственный НАДЁЖНЫЙ момент —
  // submit самой формы logout (form[action="/logout"]): он происходит СИНХРОННО в том же
  // тике, что и клик «Выйти», ДО того, как браузер вообще отправил POST — abort() отсюда
  // гарантированно опережает и /logout, и любой другой ещё не завершившийся фоновый запрос.
  var boardAbort = new AbortController();
  document.addEventListener('submit', function (e) {
    var f = e.target;
    if (f && f.tagName === 'FORM' && f.getAttribute('action') === '/logout') boardAbort.abort();
  }, true);
  addEventListener('beforeunload', function () { boardAbort.abort(); });
  addEventListener('pagehide', function () { boardAbort.abort(); });
  function reqOpts() { return { signal: boardAbort.signal }; }
  function isAbortError(err) { return err && (err.name === 'AbortError' || (err.code == null && /abort/i.test(err.message || ''))); }

  function assigneeDefault() { return state.isAdmin ? 'all' : 'me'; }

  // parseFilters — `task` (deep-link ADR-104, см. openDeepLinkTask/init) сюда сознательно
  // НЕ входит: это не фильтр доски, а адрес открытой карточки, и filtersSearch/tasksQuery
  // ниже строят query-string только из перечисленных ниже ключей — «лишний» task не
  // просачивается в GET /tasks и не подставляется обратно при pushFiltersToUrl().
  function parseFilters() {
    var p = new URLSearchParams(location.search);
    return {
      client_id: p.get('client_id') || '',
      q: p.get('q') || '',
      category_id: p.get('category_id') || '',
      paid: p.get('paid') || 'all',
      // неизвестное значение из адреса → «все» (сервер на мусор ответил бы 400).
      executor_paid: ({ paid: 'paid', unpaid: 'unpaid' })[p.get('executor_paid')] || 'all',
      assignee: p.get('assignee') || assigneeDefault(),
      due: p.get('due') || 'all'
    };
  }

  function filtersSearch(f) {
    var p = new URLSearchParams();
    if (f.client_id) p.set('client_id', f.client_id);
    if (f.q) p.set('q', f.q);
    if (f.category_id) p.set('category_id', f.category_id);
    if (f.paid && f.paid !== 'all') p.set('paid', f.paid);
    if (f.executor_paid && f.executor_paid !== 'all') p.set('executor_paid', f.executor_paid);
    if (f.assignee && f.assignee !== assigneeDefault()) p.set('assignee', f.assignee);
    if (f.due && f.due !== 'all') p.set('due', f.due);
    var s = p.toString();
    return s ? '?' + s : '';
  }

  function tasksQuery(f) {
    var p = new URLSearchParams();
    if (f.client_id) p.set('client_id', f.client_id);
    if (f.q) p.set('q', f.q);
    if (f.category_id) p.set('category_id', f.category_id);
    if (f.paid && f.paid !== 'all') p.set('paid', f.paid);
    // executor-payouts D67: фильтр «Выплата исполнителю» (по услугам заказа с исполнителем).
    if (f.executor_paid && f.executor_paid !== 'all') p.set('executor_paid', f.executor_paid);
    p.set('assignee', f.assignee || assigneeDefault());
    if (f.due && f.due !== 'all') p.set('due', f.due);
    // pagination DS-001: окно per_column — не более PER_COLUMN первых (position,id)
    // карточек каждой колонки ОДНИМ запросом (глобальная постраничность по колонкам слева
    // направо оставляла бы правые колонки пустыми); остальное — «Показать ещё» по колонке.
    // limit=1000 — верхняя граница сервера (tasks_dto.go §TaskListResponse), применяется
    // ПОСЛЕ окна.
    p.set('per_column', String(PER_COLUMN));
    p.set('limit', '1000');
    return '?' + p.toString();
  }

  // columnMoreQuery — «Показать ещё» (DS-001): те же фильтры + column_id, следующая порция.
  function columnMoreQuery(f, columnId, offset) {
    var p = new URLSearchParams(tasksQuery(f).slice(1));
    p.delete('per_column');
    p.set('column_id', columnId);
    p.set('limit', String(PER_COLUMN));
    p.set('offset', String(offset));
    return '?' + p.toString();
  }

  function pushFiltersToUrl() {
    var q = filtersSearch(state.filters);
    history.replaceState(null, '', location.pathname + q);
  }

  // ---------- загрузка ----------
  // GET /api/v1/clients клэмпит limit сервером до 200 (clients_service.go maxLimit) —
  // грузим ТОЛЬКО первую (алфавитную) страницу для мгновенно отзывчивого списка «Все
  // клиенты» в фильтре/форме заказа; клиенты дальше по алфавиту (за пределами 200,
  // BD-009) находятся при вводе через серверный поиск remoteClientSearch — combobox.js
  // сам вызывает его, когда в комбобоксе есть текст (см. wireClientCombobox ниже). Раньше
  // здесь была догрузка ВСЕХ клиентов постранично (до 25 запросов) — она грузила доску
  // десятками фоновых запросов, росла с числом клиентов и стала причиной гонки с logout
  // (BD-021): запрос второй страницы получал 401 уже после разлогина.
  // loadAllPages — pagination DS-003: справочник целиком, страницами limit=<max сервера>
  // до offset >= total. Страховочный потолок MAX_LIST_PAGES — при его достижении
  // console.warn и пометка в тулбаре (state.listsCapped), доска рендерится с тем, что есть.
  function loadAllPages(url, pageSize) {
    var items = [];
    var sep = url.indexOf('?') >= 0 ? '&' : '?';
    function page(n) {
      return KT.api.get(url + sep + 'limit=' + pageSize + '&offset=' + (n * pageSize), reqOpts()).then(function (r) {
        items = items.concat(r.items || []);
        var more = (r.items || []).length > 0 && items.length < r.total;
        if (!more) return items;
        if (n + 1 >= MAX_LIST_PAGES) {
          console.warn('board: ' + url + ' — достигнут потолок ' + MAX_LIST_PAGES + ' страниц, загружено ' + items.length + ' из ' + r.total);
          state.listsCapped = true;
          return items;
        }
        return page(n + 1);
      });
    }
    return page(0);
  }

  function loadStatic() {
    state.listsCapped = false;
    return Promise.all([
      // pagination DS-003: колонки/исполнители/категории — ВСЕ страницы (лимиты сервера не
      // меняются: columnsMaxLimit/assigneesMaxLimit=1000, catalog maxLimit=2000). Раньше —
      // одна страница limit=1000/200: на общей dev-БД колонок уже бывало >100 (BD-006) —
      // хвост отрезался молча.
      loadAllPages('/api/v1/columns', 1000),
      loadAllPages('/api/v1/assignees', 1000),
      KT.api.get('/api/v1/clients?limit=200', reqOpts()),
      loadAllPages('/api/v1/catalog-categories?kind=service', 2000)
    ]).then(function (r) {
      state.columns = r[0];
      state.assignees = r[1];
      state.clients = r[2].items;
      state.categories = r[3];
    });
  }

  function loadTasks() {
    var gen = ++state.loadGen;
    moreInFlight = {}; // догрузки старого поколения больше не блокируют кнопки
    return KT.api.get('/api/v1/tasks' + tasksQuery(state.filters), reqOpts()).then(function (r) {
      if (gen !== state.loadGen) return; // перекрыт более свежим loadTasks
      state.tasks = r.items;
      state.tasksTotal = r.total;
      // pagination DS-001/DS-002: column_total одинаков у всех карточек колонки.
      state.columnTotals = {};
      state.colLoaded = {};
      state.tasks.forEach(function (t) {
        state.columnTotals[t.column_id] = t.column_total;
        state.colLoaded[t.column_id] = (state.colLoaded[t.column_id] || 0) + 1;
        t._srvCol = t.column_id;
      });
      state.tasksTruncated = state.tasks.length < state.tasksTotal;
    });
  }

  // loadMoreColumn — «Показать ещё» (DS-001): следующая порция колонки, дописывается в
  // конец колонки. offset = colLoaded (только карточки, пришедшие с сервера для этой
  // колонки); уже известные по id карточки (перенесённые сюда локально) отбрасываются.
  var moreInFlight = {};
  function loadMoreColumn(columnId) {
    if (moreInFlight[columnId]) return Promise.resolve();
    moreInFlight[columnId] = true;
    var gen = state.loadGen;
    var offset = state.colLoaded[columnId] || 0;
    return KT.api.get('/api/v1/tasks' + columnMoreQuery(state.filters, columnId, offset), reqOpts()).then(function (r) {
      // ревью MEDIUM-1: доска перезагружена (фильтр/конфликт версии), пока шла догрузка —
      // ответ относится к старому набору, в новый state.tasks его не подмешиваем.
      if (gen !== state.loadGen) return;
      var known = {};
      state.tasks.forEach(function (t) { known[t.id] = true; });
      // вставка — сразу за последней карточкой колонки (порядок state.tasks = порядок доски).
      var lastIdx = -1;
      state.tasks.forEach(function (t, i) { if (t.column_id === columnId) lastIdx = i; });
      var fresh = r.items.filter(function (t) { return !known[t.id]; });
      fresh.forEach(function (t) { t._srvCol = columnId; });
      if (lastIdx < 0) state.tasks = state.tasks.concat(fresh);
      else state.tasks.splice.apply(state.tasks, [lastIdx + 1, 0].concat(fresh));
      state.colLoaded[columnId] = offset + r.items.length;
      if (r.items.length) state.columnTotals[columnId] = r.items[0].column_total;
      else state.columnTotals[columnId] = columnShown(columnId); // за хвостом ничего нет
      renderBoard();
    }).catch(function (err) {
      if (isAbortError(err) || gen !== state.loadGen) return;
      KT.uiAlert((err && err.message) || 'Не удалось загрузить заказы колонки', 'Ошибка');
    }).then(function () {
      if (gen !== state.loadGen) return; // moreInFlight уже сброшен новым loadTasks
      moreInFlight[columnId] = false;
      // ревью LOW-1: после ошибки кнопка снова активна (успех перерисовал доску сам).
      var btn = $('.board-more-btn[data-column-id="' + columnId + '"]');
      if (btn) btn.disabled = false;
    });
  }

  function columnShown(columnId) {
    return state.tasks.filter(function (t) { return t.column_id === columnId; }).length;
  }

  // remoteClientSearch — серверный поиск клиента по вводу (GET /api/v1/clients?q=…,
  // ADR-003: ищет и по названию, и по телефону). Найденные клиенты добавляются в
  // state.clients (dedup по id), чтобы пережить следующий fillFilterOptions/
  // fillTaskFormOptions (они перестраивают <select> целиком из state.clients).
  function remoteClientSearch(q) {
    return KT.api.get('/api/v1/clients?q=' + encodeURIComponent(q) + '&limit=50', reqOpts()).then(function (r) {
      r.items.forEach(function (c) {
        if (!state.clients.some(function (x) { return x.id === c.id; })) state.clients.push(c);
      });
      return r.items.map(function (c) { return { value: c.id, text: c.company_name }; });
    });
  }

  // wireClientCombobox — навешивает серверный поиск на комбобокс клиента (фильтр и форма
  // заказа); combobox.js вызывает sel._cbFetchOptions(q), когда в его поле ввода есть текст.
  function wireClientCombobox(sel) {
    if (sel) sel._cbFetchOptions = remoteClientSearch;
  }

  function reloadAndRender() {
    // AbortError (страница уходит, boardAbort.abort() на pagehide) — не настоящая ошибка,
    // просто некому больше показывать доску; проглатываем, а не роняем в unhandled rejection.
    return loadTasks().then(renderBoard).catch(function (err) {
      if (isAbortError(err)) return;
      throw err;
    });
  }

  // ---------- фильтры ----------
  function fillFilterOptions() {
    var clientSel = $('#filter-client');
    // клиент фильтра в удалённых (state.filterClient) — отдельным пунктом с пометкой, только
    // пока фильтр стоит на нём (soft-delete-clients §8.3)
    var fc = state.filterClient && state.filterClient.id === state.filters.client_id ? state.filterClient : null;
    // клиента восстановили и он снова в справочнике — отдельный пункт не нужен
    if (fc && state.clients.some(function (c) { return c.id === fc.id; })) fc = state.filterClient = null;
    clientSel.innerHTML = '<option value="">Все клиенты</option>' +
      (fc ? '<option value="' + esc(fc.id) + '">' + esc(fc.company_name) + ' (удалён)</option>' : '') +
      state.clients.map(function (c) {
        return '<option value="' + esc(c.id) + '">' + esc(c.company_name) + '</option>';
      }).join('');
    wireClientCombobox(clientSel);

    var catSel = $('#filter-category');
    catSel.innerHTML = '<option value="">Все виды работ</option>' + state.categories.map(function (c) {
      return '<option value="' + esc(c.id) + '">' + esc(c.name) + '</option>';
    }).join('');

    var aSel = $('#filter-assignee');
    aSel.innerHTML = '<option value="all">Все исполнители</option><option value="me">Мои</option>' + state.assignees.map(function (a) {
      return '<option value="' + esc(a.id) + '">' + esc(a.name) + '</option>';
    }).join('');
  }

  function applyFiltersToForm() {
    var f = state.filters;
    $('#filter-client').value = f.client_id;
    $('#filter-q').value = f.q;
    $('#filter-category').value = f.category_id;
    $('#filter-paid').value = f.paid;
    $('#filter-executor-paid').value = f.executor_paid;
    $('#filter-assignee').value = f.assignee;
    $('#filter-due').value = f.due;
    // Все 5 select-фильтров превращены в комбобоксы (js/combobox.js) — после программной
    // смены .value видимая кнопка не обновляется сама, нужен ручной _cbSync() на каждом
    // (баг фазы 06: filter-paid/filter-due не синхронизировались после reload — BD-013).
    ['#filter-client', '#filter-category', '#filter-paid', '#filter-executor-paid', '#filter-assignee', '#filter-due'].forEach(function (sel) {
      var el = $(sel);
      if (el && el._cbSync) el._cbSync();
    });
  }

  function bindFilters() {
    function onChange(id, key) {
      $(id).addEventListener('change', function (e) {
        state.filters[key] = e.target.value;
        pushFiltersToUrl();
        reloadAndRender();
      });
    }
    onChange('#filter-client', 'client_id');
    onChange('#filter-category', 'category_id');
    onChange('#filter-paid', 'paid');
    onChange('#filter-executor-paid', 'executor_paid');
    onChange('#filter-assignee', 'assignee');
    onChange('#filter-due', 'due');

    var qTimer = null;
    $('#filter-q').addEventListener('input', function (e) {
      var v = e.target.value;
      clearTimeout(qTimer);
      qTimer = setTimeout(function () {
        state.filters.q = v;
        pushFiltersToUrl();
        reloadAndRender();
      }, 250);
    });

    $('#filter-reset').addEventListener('click', function () {
      state.filters = { client_id: '', q: '', category_id: '', paid: 'all', executor_paid: 'all', assignee: assigneeDefault(), due: 'all' };
      applyFiltersToForm();
      pushFiltersToUrl();
      reloadAndRender();
    });
  }

  // ---------- рендер доски ----------
  function due2Class(t) {
    var col = columnById(t.column_id);
    if (col && col.is_final) return '';
    // «Сегодня» — текущая дата в поясе ПОЛЬЗОВАТЕЛЯ, не браузера (DS-032). due_date —
    // плановое окончание заказа (order-start-duration: start_date + duration_days − 1, у заказа
    // всегда есть), календарная дата без времени (не сдвигается); ISO-даты сравниваются как строки.
    var today = KT.time.today();
    var due = String(t.due_date).slice(0, 10);
    if (due < today) return 'overdue';
    if (due <= KT.time.addDays(today, 1)) return 'soon';
    return '';
  }
  function fmtDate(iso) {
    if (!iso) return '';
    var p = iso.split('-');
    return p[2] + '.' + p[1] + '.' + p[0];
  }
  function columnById(id) { return state.columns.filter(function (c) { return c.id === id; })[0]; }
  function assigneeById(id) { return state.assignees.filter(function (a) { return a.id === id; })[0]; }

  // moveMenuItemsHTML — «Переместить в…» на карточку заказа (ADR-106). Строится ЛЕНИВО,
  // по факту открытия конкретного меню (см. bindBoardDelegates), а не для КАЖДОЙ карточки
  // при каждом renderBoard(): на общей dev-БД счёт карточек и колонок идёт на сотни/тысячи
  // (limit подняли до 1000 в обоих loadTasks/loadStatic), и eager-генерация count(cards) ×
  // count(columns) пунктов списка на каждый рендер доски раздувала DOM до сотен тысяч
  // узлов и вешала главный поток на десятки секунд (найдено фикс-лупом фазы 05: BD-006
  // падал таймаутом на «нестабильной» кнопке диалога — реальная причина была здесь, а не в
  // самом диалоге).
  function moveMenuItemsHTML(t) {
    return state.columns.filter(function (c) { return c.id !== t.column_id; }).map(function (c) {
      return '<li><a class="dropdown-item task-move-link" href="#" data-task-id="' + t.id + '" data-column-id="' + c.id + '" data-testid="task-move-to-' + c.id + '">Переместить в «' + esc(c.name) + '»</a></li>';
    }).join('');
  }

  function cardHTML(t) {
    var ds = due2Class(t);
    var late = t.subtasks_late || 0;
    var totalAmount = num(t.total_amount);
    return '' +
      '<div class="task-card' + (ds === 'overdue' ? ' overdue' : '') + '" draggable="true" data-id="' + t.id + '" data-testid="task-card-' + t.id + '" style="--c:' + esc(columnColor(t.column_id)) + '">' +
      '  <div class="d-flex align-items-start">' +
      '    <div class="task-card-title">#' + t.id + ' ' + esc(t.title) + '</div>' +
      '    <div class="dropdown task-card-menu-btn">' +
      '      <button type="button" class="btn btn-sm btn-outline-secondary" data-bs-toggle="dropdown" aria-expanded="false" data-task-id="' + t.id + '" data-testid="task-card-menu-' + t.id + '">⋯</button>' +
      '      <ul class="dropdown-menu dropdown-menu-end" data-task-move-menu="' + t.id + '"></ul>' +
      '    </div>' +
      '  </div>' +
      '  <div class="task-card-client">🏢 <span class="name">' + esc(t.client_name) + '</span>' +
      // клиент заказа мягко удалён (soft-delete-clients §8.3 п.1) — серый бейдж справа от имени
      (t.client_is_deleted ? '<span class="badge text-bg-secondary ms-1" data-testid="task-card-client-deleted">удалён</span>' : '') +
      '<span class="pay ' + (t.is_paid ? 'pay-yes' : 'pay-no') + '">' + (t.is_paid ? '✓ Оплачен' : 'Не оплачен') + '</span></div>' +
      // executor-payouts D68: метка — есть выполненные услуги с исполнителем без выплаты.
      (t.executor_unpaid_done > 0 ? '<div class="mb-1"><span class="badge text-bg-info" data-testid="task-card-executor-unpaid" title="Выполненные услуги, за которые исполнителю ещё не выплачено">💸 к выплате: ' + esc(t.executor_unpaid_done) + '</span></div>' : '') +
      (t.subtasks_total ? '<div class="small text-body-secondary mb-1">☑ ' + t.subtasks_done + '/' + t.subtasks_total + (late ? ' · <span class="text-danger">⚠ ' + late + '</span>' : '') + '</div>' : '') +
      // total_amount показываем, только если >0 (AC-013) — заказ без услуг/цен не засоряет
      // карточку нулём; last_log — последняя запись журнала ЛЮБОЙ услуги заказа (AC-012),
      // текст уже обрезан сервером до 120 символов + «…».
      (totalAmount > 0 ? '<div class="small task-card-total" data-testid="task-card-total">Итог: ' + fmtMoneyCents(Math.round(totalAmount * 100)) + '</div>' : '') +
      (t.last_log ? '<div class="small text-body-secondary task-card-last-log" data-testid="task-card-last-log">💬 ' + esc(t.last_log.text) + ' — ' + esc(t.last_log.author_name) + '</div>' : '') +
      '  <div class="task-card-foot">' +
      '<span class="due due-' + esc(ds || 'ok') + '" data-testid="task-card-due" data-due="' + esc(ds || 'ok') + '" title="Плановое окончание">📅 ' + fmtDate(t.due_date) + '</span>' +
      (t.assignee_name ? '<span class="avatar" title="' + esc(t.assignee_name) + '">' + esc(initials(t.assignee_name)) + '</span>' : '') +
      '  </div>' +
      '</div>';
  }
  function initials(name) {
    return String(name || '').trim().split(/\s+/).map(function (w) { return w[0]; }).slice(0, 2).join('').toUpperCase();
  }
  function columnColor(columnId) {
    var c = columnById(columnId);
    return c ? c.color : '#94A3B8';
  }

  function renderBoard() {
    var board = $('#board');
    var tasksByColumn = {};
    state.columns.forEach(function (c) { tasksByColumn[c.id] = []; });
    state.tasks.forEach(function (t) { if (tasksByColumn[t.column_id]) tasksByColumn[t.column_id].push(t); });

    board.innerHTML = state.columns.map(function (col) {
      var tasks = tasksByColumn[col.id] || [];
      // pagination DS-001: M — заказов колонки под фильтрами (сервер + локальные переносы),
      // N — показано; при N < M — «показаны N из M» и «Показать ещё».
      var colM = Math.max(state.columnTotals[col.id] || 0, tasks.length);
      // ревью MEDIUM-2: колонка не попала в обрезанную первичную выдачу — M неизвестен.
      var colUnknown = state.tasksTruncated && !Object.prototype.hasOwnProperty.call(state.columnTotals, col.id);
      // column-total (ADR-005) — сумма total_amount ВИДИМЫХ карточек колонки (после
      // клиентских фильтров, не серверный агрегат): считается тут же, из уже загруженного
      // state.tasks, без расширения конверта GET /tasks {items,total,limit,offset}.
      var colTotalCents = moneySum(tasks.map(function (t) { return t.total_amount; }));
      return '' +
        '<section class="board-col" data-column-id="' + col.id + '" data-testid="board-column-' + col.id + '" style="--c:' + esc(col.color) + '">' +
        '  <div class="board-col-head">' +
        '    <i class="dot"></i><span class="name">' + esc(col.name) + '</span><span class="count" data-testid="column-count-' + col.id + '">' + colM + '</span>' +
        (colTotalCents > 0 ? '<span class="column-total" data-testid="column-total">' + fmtMoneyCents(colTotalCents) + '</span>' : '') +
        (col.is_final ? '<span class="final-mark" title="Финальная колонка">✓</span>' : '') +
        (state.isAdmin ? '<div class="dropdown"><button type="button" class="btn btn-sm btn-outline-secondary" data-bs-toggle="dropdown" data-testid="column-menu-' + col.id + '">⋯</button>' +
          '<ul class="dropdown-menu dropdown-menu-end">' +
          '<li><a class="dropdown-item column-edit-link" href="#" data-column-id="' + col.id + '" data-testid="column-edit-' + col.id + '">Изменить</a></li>' +
          '<li><a class="dropdown-item column-delete-link text-danger" href="#" data-column-id="' + col.id + '" data-testid="column-delete-' + col.id + '">Удалить</a></li>' +
          '</ul></div>' : '') +
        '  </div>' +
        '  <div class="board-col-body" data-column-id="' + col.id + '" data-testid="column-body-' + col.id + '">' +
        tasks.map(cardHTML).join('') +
        '  </div>' +
        (colUnknown ?
          '  <div class="board-col-more">' +
          '    <span class="text-body-secondary small" data-testid="column-shown-' + col.id + '">' + (tasks.length ? 'показаны ' + tasks.length : 'заказы колонки не загружены') + '</span>' +
          '    <button type="button" class="btn btn-sm btn-outline-secondary board-more-btn" data-column-id="' + col.id + '" data-testid="column-more-' + col.id + '">Показать заказы</button>' +
          '  </div>' :
        tasks.length < colM ?
          '  <div class="board-col-more">' +
          '    <span class="text-body-secondary small" data-testid="column-shown-' + col.id + '">показаны ' + tasks.length + ' из ' + colM + '</span>' +
          '    <button type="button" class="btn btn-sm btn-outline-secondary board-more-btn" data-column-id="' + col.id + '" data-testid="column-more-' + col.id + '">Показать ещё</button>' +
          '  </div>' : '') +
        '  <button type="button" class="btn btn-sm board-add-task" data-column-id="' + col.id + '" data-testid="column-add-task-' + col.id + '">+ Добавить заказ</button>' +
        '</section>';
    }).join('') + (state.isAdmin ? '<button type="button" class="board-add-column" id="board-add-column-btn" data-testid="board-add-column">+ Добавить колонку</button>' : '');

    // pagination DS-004: «Заказов: M», а если на доске видно меньше — «Заказов: M · показаны N».
    var shown = state.tasks.length;
    var totalEl = $('#board-total');
    if (totalEl) {
      totalEl.textContent = 'Заказов: ' + state.tasksTotal + (shown < state.tasksTotal ? ' · показаны ' + shown : '') +
        (state.listsCapped ? ' · справочники доски загружены не полностью' : '');
    }

    bindColumnMenus();
    bindTaskCards();
    bindDnd();
  }

  function bindColumnMenus() {
    $$('.column-edit-link').forEach(function (a) {
      a.addEventListener('click', function (e) { e.preventDefault(); openColumnForm(columnById(a.dataset.columnId)); });
    });
    $$('.column-delete-link').forEach(function (a) {
      a.addEventListener('click', function (e) { e.preventDefault(); deleteColumn(a.dataset.columnId); });
    });
    $$('.board-add-task').forEach(function (b) {
      b.addEventListener('click', function () { openTaskForm(null, b.dataset.columnId); });
    });
    var addCol = $('#board-add-column-btn');
    if (addCol) addCol.addEventListener('click', function () { openColumnForm(null); });
  }

  function bindTaskCards() {
    $$('.task-card').forEach(function (el) {
      el.addEventListener('click', function (e) {
        if (e.target.closest('.dropdown')) return;
        openTaskForm(taskById(+el.dataset.id));
      });
    });
    // «Переместить в…» — делегированные обработчики (bindBoardDelegates, once), не здесь:
    // .task-move-link создаётся лениво при открытии меню, per-card биндинг тут был бы
    // бесполезен (элементов ещё нет в момент renderBoard).
  }

  // bindBoardDelegates — ОДИН раз на #board (переживает renderBoard, т.к. innerHTML меняет
  // только детей): лениво строит «Переместить в…» по факту открытия меню карточки
  // (Bootstrap 'show.bs.dropdown') и обрабатывает клик по уже построенному пункту.
  function bindBoardDelegates(board) {
    board.addEventListener('show.bs.dropdown', function (e) {
      var btn = e.target;
      if (!btn || !btn.matches('[data-testid^="task-card-menu-"]')) return;
      var t = taskById(+btn.dataset.taskId);
      if (!t) return;
      var ul = btn.parentElement.querySelector('[data-task-move-menu="' + t.id + '"]');
      if (ul) ul.innerHTML = moveMenuItemsHTML(t);
    });
    board.addEventListener('click', function (e) {
      var more = e.target.closest('.board-more-btn');
      if (more) {
        if (moreInFlight[more.dataset.columnId]) return; // ревью LOW-1: не гасим кнопку повторным кликом
        more.disabled = true;
        loadMoreColumn(more.dataset.columnId);
        return;
      }
      var a = e.target.closest('.task-move-link');
      if (!a) return;
      e.preventDefault();
      moveTaskToEnd(+a.dataset.taskId, a.dataset.columnId);
    });
  }

  function taskById(id) { return state.tasks.filter(function (t) { return t.id === id; })[0]; }

  // ---------- DnD ----------
  var dragId = null;
  var dragFromColumn = null;

  function bindDnd() {
    $$('.task-card').forEach(function (el) {
      el.addEventListener('dragstart', function (e) {
        dragId = +el.dataset.id;
        var t = taskById(dragId);
        dragFromColumn = t ? t.column_id : null;
        e.dataTransfer.setData('text/plain', String(dragId));
        e.dataTransfer.effectAllowed = 'move';
        el.classList.add('dragging');
      });
      el.addEventListener('dragend', function () {
        el.classList.remove('dragging');
        $$('.board-drop-placeholder').forEach(function (p) { p.remove(); });
        $$('.drag-over').forEach(function (b) { b.classList.remove('drag-over'); });
      });
    });

    $$('.board-col-body').forEach(function (body) {
      body.addEventListener('dragover', function (e) {
        e.preventDefault();
        $$('.drag-over').forEach(function (b) { if (b !== body) b.classList.remove('drag-over'); });
        body.classList.add('drag-over');
        var after = $$('.task-card:not(.dragging)', body).filter(function (c) {
          var r = c.getBoundingClientRect();
          return e.clientY < r.top + r.height / 2;
        })[0] || null;
        var ph = $('.board-drop-placeholder') || document.createElement('div');
        ph.className = 'board-drop-placeholder';
        if (after) body.insertBefore(ph, after); else body.appendChild(ph);
      });
      body.addEventListener('drop', function (e) {
        e.preventDefault();
        var ph = $('.board-drop-placeholder');
        if (!ph || dragId == null) return;
        var prevEl = ph.previousElementSibling && ph.previousElementSibling.classList.contains('task-card') ? ph.previousElementSibling : null;
        var nextEl = ph.nextElementSibling && ph.nextElementSibling.classList.contains('task-card') ? ph.nextElementSibling : null;
        var prevId = prevEl ? +prevEl.dataset.id : null;
        var nextId = nextEl ? +nextEl.dataset.id : null;
        var columnId = body.dataset.columnId;
        var taskId = dragId;
        ph.remove();
        dragId = null;
        performMove(taskId, columnId, prevId, nextId);
      });
    });
  }

  function moveTaskToEnd(taskId, columnId) {
    var t = taskById(taskId);
    if (!t || t.column_id === columnId) return;
    var body = $('.board-col-body[data-column-id="' + columnId + '"]');
    var last = body ? $$('.task-card', body).pop() : null;
    performMove(taskId, columnId, last ? +last.dataset.id : null, null);
  }

  function performMove(taskId, columnId, prevId, nextId) {
    // снапшот снимается ЗДЕСЬ, прямо перед собственной оптимистичной правкой этого move
    // (не в dragstart/moveTaskToEnd) — иначе два быстрых move подряд затирают снапшот друг
    // друга: rollback первого отката откатывал бы к состоянию ПОСЛЕ оптимистичной правки
    // второго (баг из code review LOW-4).
    state.lastKnownTasks = JSON.parse(JSON.stringify({ tasks: state.tasks, columnTotals: state.columnTotals, colLoaded: state.colLoaded }));
    // оптимистичный перенос — локально переносим карточку сразу.
    var idx = state.tasks.findIndex(function (t) { return t.id === taskId; });
    if (idx >= 0) {
      var t = state.tasks[idx];
      var fromColumn = t.column_id;
      t.column_id = columnId;
      // pagination DS-004: локальный пересчёт M колонок и серверного префикса (colLoaded).
      if (fromColumn !== columnId) {
        // колонка без записи (не попала в обрезанную выдачу, MEDIUM-2) остаётся «неизвестной» —
        // иначе +1 создал бы запись и спрятал «Показать заказы».
        if (Object.prototype.hasOwnProperty.call(state.columnTotals, fromColumn)) state.columnTotals[fromColumn] = Math.max(0, state.columnTotals[fromColumn] - 1);
        if (Object.prototype.hasOwnProperty.call(state.columnTotals, columnId)) state.columnTotals[columnId] += 1;
        if (t._srvCol === fromColumn) state.colLoaded[fromColumn] = Math.max(0, (state.colLoaded[fromColumn] || 0) - 1);
        t._srvCol = null;
      }
      state.tasks.splice(idx, 1);
      var insertAt = state.tasks.length;
      if (nextId != null) {
        var ni = state.tasks.findIndex(function (x) { return x.id === nextId; });
        if (ni >= 0) insertAt = ni;
      } else if (prevId != null) {
        var pi = state.tasks.findIndex(function (x) { return x.id === prevId; });
        if (pi >= 0) insertAt = pi + 1;
      }
      state.tasks.splice(insertAt, 0, t);
      renderBoard();
      if (fromColumn !== columnId) {
        var col = columnById(columnId);
        showBoardToast('#' + taskId + ': статус → «' + (col ? col.name : '') + '»', false);
      }
    }

    KT.api.post('/api/v1/tasks/' + taskId + '/move', { column_id: columnId, prev_id: prevId, next_id: nextId })
      .then(function (resp) {
        state.lastKnownTasks = null;
        applyTaskPatch(resp.task);
        if (resp.warnings && resp.warnings.length) toastWarnings(resp.warnings);
        KT.refreshNavBadge();
      })
      .catch(function (err) { handleMutationError(err, true); });
  }

  function applyTaskPatch(task) {
    var idx = state.tasks.findIndex(function (t) { return t.id === task.id; });
    // ревью LOW-2: TaskResponse мутаций несёт column_total=0 (не вычисляется) — не
    // затираем им значение из GET /tasks.
    var patch = Object.assign({}, task);
    delete patch.column_total;
    if (idx >= 0) state.tasks[idx] = Object.assign({}, state.tasks[idx], patch);
    renderBoard();
  }

  function rollback() {
    if (state.lastKnownTasks) {
      state.tasks = state.lastKnownTasks.tasks;
      state.columnTotals = state.lastKnownTasks.columnTotals;
      state.colLoaded = state.lastKnownTasks.colLoaded;
      state.lastKnownTasks = null;
    }
    renderBoard();
  }

  function handleMutationError(err, isMove) {
    if (isMove) rollback();
    if (err && err.code === 'SUBTASKS_NOT_DONE') {
      var subs = (err.details && err.details.subtasks) || [];
      var list = subs.map(function (s) { return s.service_name + ' (' + statusLabel(s.status) + ')'; }).join('\n• ');
      KT.uiAlert((list ? '• ' + list : '') , 'Есть незавершённые услуги');
      return;
    }
    if (err && err.code === 'TASK_VERSION_CONFLICT') {
      KT.uiAlert(err.message || 'Заказ уже изменён, обновите', 'Конфликт версии');
      reloadAndRender();
      return;
    }
    // Колонку удалил другой администратор, пока доска открыта (soft-delete-board-columns §8.1):
    // тост, карточка уже возвращена на место (rollback); колонка исчезнет после перезагрузки.
    if (err && err.code === 'COLUMN_NOT_FOUND') {
      KT.uiToast('Колонка не найдена', true);
      return;
    }
    KT.uiAlert((err && err.message) || 'Не удалось выполнить операцию', 'Ошибка');
  }
  function statusLabel(s) {
    return { new: 'новая', in_progress: 'в работе', done: 'завершена' }[s] || s;
  }

  function showBoardToast(msg, isWarning) {
    $$('.board-toast').forEach(function (el) { el.remove(); });
    var el = document.createElement('div');
    el.className = 'board-toast alert ' + (isWarning ? 'alert-warning' : 'alert-secondary') + ' shadow';
    el.setAttribute('role', 'status');
    el.textContent = msg;
    document.body.appendChild(el);
    setTimeout(function () { el.remove(); }, isWarning ? 6000 : 2500);
  }

  function toastWarnings(warnings) {
    $$('.board-toast').forEach(function (el) { el.remove(); });
    var el = document.createElement('div');
    el.className = 'board-toast alert alert-warning shadow';
    el.setAttribute('data-testid', 'toast-warnings');
    el.setAttribute('role', 'alert');
    var text = document.createElement('div');
    text.textContent = '⚠ Внимание, товары на минимуме: ' + warnings.map(function (w) {
      return w.name + ' (' + (w.kind === 'negative' ? 'не хватает' : 'на минимуме') + ')';
    }).join(', ');
    el.appendChild(text);
    var link = document.createElement('a');
    link.href = '/stock';
    link.className = 'alert-link';
    link.textContent = 'Открыть «Остатки»';
    el.appendChild(link);
    document.body.appendChild(el);
    setTimeout(function () { el.remove(); }, 8000);
  }

  // ---------- форма заказа — карточка заказа (фаза 6, KT.taskCard, js/task-card.js) ----------
  // Простая форма фазы 5 заменена полной карточкой (шапка + оборудование + услуги/товары +
  // журнал + итоги + печать актов); board.js только открывает/закрывает её и передаёт
  // общие справочники (клиенты/колонки/исполнители) через getData().

  // loadStatic грузит только первую страницу клиентов (limit=200) — не гарантирует, что
  // клиент открываемого заказа/фильтра в неё попал. Без этого клиент в форме молча
  // остаётся невыбранным (select.value не находит option), и PUT/POST уходит с пустым
  // client_id → 400 VALIDATION. Подгружаем недостающего клиента отдельным запросом и
  // добавляем в список перед тем, как выставлять .value.
  //
  // soft-delete-clients §8.3, DS-026: клиент фильтра может быть мягко удалён — для справочника
  // (GET /api/v1/clients/:id) его не существует (404), а его заказы остаются на доске. Поэтому
  // клиент сначала ищется среди заказов (GET /api/v1/tasks?client_id=…&limit=1 — имя и признак
  // client_is_deleted приходят в строке заказа): удалённый попадает только в фильтр
  // (state.filterClient, пункт «… (удалён)»), в выбор клиента нового заказа — нет; действующий
  // добавляется в state.clients. К справочнику идём, только если заказов у клиента нет.
  function ensureClientOption(clientId) {
    if (!clientId) return Promise.resolve();
    var known = state.clients.some(function (c) { return c.id === clientId; });
    if (known || (state.filterClient && state.filterClient.id === clientId)) return Promise.resolve();
    return KT.api.get('/api/v1/tasks?client_id=' + encodeURIComponent(clientId) + '&limit=1', reqOpts()).then(function (r) {
      var t = (r.items || [])[0];
      if (t && t.client_is_deleted) {
        state.filterClient = { id: clientId, company_name: t.client_name };
        return;
      }
      if (t) {
        state.clients = [{ id: clientId, company_name: t.client_name }].concat(state.clients);
        return;
      }
      return KT.api.get('/api/v1/clients/' + clientId, reqOpts()).then(function (c) {
        state.clients = [c].concat(state.clients);
      });
    }).catch(function () { /* клиента нет — оставляем список как есть */ });
  }

  // openTaskForm — клиент открываемого заказа известен из самой строки доски (client_name,
  // client_is_deleted), запрос справочника не нужен: действующий, не попавший в первую страницу
  // state.clients, добавляется из строки; удалённого в список выбора не добавляем — карточка
  // (KT.taskCard) сама покажет его отдельным пунктом с бейджем «удалён».
  function openTaskForm(task, defaultColumnId) {
    if (task && !task.client_is_deleted && !state.clients.some(function (c) { return c.id === task.client_id; })) {
      state.clients = [{ id: task.client_id, company_name: task.client_name }].concat(state.clients);
    }
    KT.taskCard.open(task ? task.id : null, defaultColumnId);
  }

  // ---------- deep-link /?task=N (ADR-104) ----------
  // stripTaskFromUrl — убирает `task` из адресной строки, СОХРАНЯЯ остальные фильтры
  // (client_id/q/…), которые могли быть в URL вместе с deep-link'ом. Вызывается и при
  // обычном закрытии карточки, и (через тот же 'hidden.bs.modal') после автозакрытия
  // task-card.js на 404 «Заказ не найден» — второй путь ничего не знает про URL сам.
  function stripTaskFromUrl() {
    var p = new URLSearchParams(location.search);
    if (!p.has('task')) return;
    p.delete('task');
    var s = p.toString();
    history.replaceState(null, '', location.pathname + (s ? '?' + s : ''));
  }

  // openDeepLinkTask — открывает карточку заказа по `?task=N` из адресной строки. Сообщение
  // «Заказ не найден» на несуществующий/чужой id и уборка `task` из URL после — уже
  // реализованы в KT.taskCard.loadCard (404 → uiAlert + close() → 'hidden.bs.modal' →
  // stripTaskFromUrl выше), здесь достаточно просто инициировать открытие.
  function openDeepLinkTask() {
    var id = parseInt(new URLSearchParams(location.search).get('task'), 10);
    if (id > 0) KT.taskCard.open(id, null); else stripTaskFromUrl();
  }

  // ---------- форма колонки ----------
  var columnModal = null;

  function openColumnForm(col) {
    state.editingColumnId = col ? col.id : null;
    $('#column-modal-title').textContent = col ? 'Колонка «' + col.name + '»' : 'Новая колонка';
    $('#column-name').value = col ? col.name : '';
    $('#column-color').value = col ? col.color : COLORS[0];
    $('#column-position').value = col ? col.position : (state.columns.length + 1);
    $('#column-final').checked = !!(col && col.is_final);
    $('#column-consumes').checked = !!(col && col.consumes_stock);
    $('#column-error').classList.add('d-none');
    $('#column-error').textContent = '';
    // hasTasks — «есть заказы в колонке» для клиентского гарда кнопки «Удалить»: НЕ по
    // одному state.tasks (список доски обрезан limit=500 и фильтрами — на выросшем стенде
    // задачи дальней/новой колонки могли не попасть в первые 500 строк, хотя физически
    // существуют, из-за чего гард молча включал кнопку для непустой колонки, BD-006).
    // col.tasks_count — точный COUNT(*) с сервера (columns_repo.go List), не режется
    // limit/фильтрами доски, но обновляется только при полном reload/column CRUD; OR с
    // state.tasks подстраховывает противоположный случай — заказ перенесён В колонку
    // локально (DnD/меню) в этой же сессии без полного reload, col.tasks_count ещё не
    // обновился, но в state.tasks перенос уже отражён.
    var hasTasks = !!(col && (col.tasks_count > 0 || state.tasks.some(function (t) { return t.column_id === col.id; })));
    $('#column-delete-btn').classList.toggle('d-none', !col);
    $('#column-delete-btn').disabled = !!hasTasks;
    $('#column-delete-btn').title = hasTasks ? 'Сначала перенесите заказы из колонки' : '';
    $('#column-save-btn').disabled = false;

    var sw = $('#column-swatches');
    sw.innerHTML = COLORS.map(function (c) {
      return '<button type="button" class="swatch' + (c.toUpperCase() === $('#column-color').value.toUpperCase() ? ' on' : '') + '" data-color="' + c + '" style="background:' + c + '" aria-label="' + c + '"></button>';
    }).join('');
    $$('.swatch', sw).forEach(function (b) {
      b.addEventListener('click', function () {
        $('#column-color').value = b.dataset.color;
        $$('.swatch', sw).forEach(function (s) { s.classList.toggle('on', s === b); });
      });
    });

    if (!columnModal) columnModal = new bootstrap.Modal($('#column-modal'));
    columnModal.show();
  }

  function showColumnError(msg) {
    var el = $('#column-error');
    el.textContent = msg;
    el.classList.remove('d-none');
  }

  function bindColumnForm() {
    $('#column-form-el').addEventListener('submit', function (e) {
      e.preventDefault();
      var btn = $('#column-save-btn');
      if (btn.disabled) return;
      btn.disabled = true;

      var payload = {
        name: $('#column-name').value.trim(),
        color: $('#column-color').value.trim(),
        is_final: $('#column-final').checked,
        consumes_stock: $('#column-consumes').checked,
        position: $('#column-position').value ? +$('#column-position').value : null
      };
      var req = state.editingColumnId
        ? KT.api.put('/api/v1/columns/' + state.editingColumnId, payload)
        : KT.api.post('/api/v1/columns', payload);
      req.then(function (resp) {
        columnModal.hide();
        return loadStatic().then(function () {
          // loadStatic() перечитал state.clients заново (только первая страница) — если
          // клиент текущего фильтра был найден раньше через remoteClientSearch/
          // ensureClientOption и лежал за пределами первой страницы, он потерялся бы молча
          // (BD-009 повторно, теперь через column CRUD).
          return ensureClientOption(state.filters.client_id);
        }).then(function () {
          fillFilterOptions();
          return reloadAndRender();
        }).then(function () {
          if (resp.warnings && resp.warnings.length) toastWarnings(resp.warnings);
        });
      }).catch(function (err) {
        btn.disabled = false;
        if (err && err.fields && err.fields.length) {
          showColumnError(err.fields.map(function (f) { return f.message; }).join('; '));
          return;
        }
        showColumnError((err && err.message) || 'Не удалось сохранить колонку');
      });
    });

    $('#column-add-btn').addEventListener('click', function () { openColumnForm(null); });

    $('#column-delete-btn').addEventListener('click', function () {
      var id = state.editingColumnId;
      if (!id) return;
      deleteColumn(id).then(function (ok) { if (ok) columnModal.hide(); });
    });
  }

  // Удаление колонки — мягкое, с причиной (soft-delete-board-columns §8.1): общее окно
  // «Удаление» (KT.uiDeleteDialog). 400 по причине и 409 (COLUMN_NOT_EMPTY,
  // LAST_NON_FINAL_COLUMN) окно показывает само и остаётся открытым.
  // Возвращает Promise<boolean> — true, если колонки на доске больше нет (удалена сейчас либо
  // уже удалена другим администратором) — для закрытия формы вызывающим.
  function deleteColumn(id) {
    var col = columnById(id);
    function refresh() {
      return loadStatic()
        .then(function () { return ensureClientOption(state.filters.client_id); })
        .then(function () { fillFilterOptions(); return reloadAndRender(); });
    }
    return KT.uiDeleteDialog({
      title: 'Удалить «' + (col ? col.name : '') + '»?',
      text: 'Колонку можно восстановить в разделе «Удалённые»',
      submit: function (reason) { return KT.api.delWith('/api/v1/columns/' + id, { reason: reason }); },
      onOffline: function () { KT.uiToast('Нет связи с сервером', true); }
    }).then(function (out) {
      if (!out) return false; // отмена
      if (out.error) {
        var err = out.error;
        if (err.status === 404) { // колонку удалил другой администратор
          KT.uiToast('Колонка не найдена', true);
          return refresh().then(function () { return true; }, function () { return true; });
        }
        KT.uiToast(err.status === 403 ? 'Нет доступа' : (err.message || 'Не удалось удалить колонку'), true);
        return false;
      }
      KT.uiToast('Удалено. Восстановить можно в разделе «Удалённые»');
      return refresh().then(function () { return true; }, function () { return true; });
    });
  }

  // ---------- инициализация ----------
  // cardTouchedBoard — карточка сообщила об изменении (правка строки/шапки), но доска
  // ПОЛНОСТЬЮ (GET /tasks?limit=1000 с коррелированными total_amount/last_log на каждый
  // заказ) не перечитывается на каждое такое сообщение — только один раз, при закрытии
  // карточки (MEDIUM-4 code review): автосохранение строки идёт часто (по каждому change),
  // reloadAndRender на каждое из них — лишняя нагрузка без видимого эффекта (карточка и так
  // открыта поверх доски).
  var cardTouchedBoard = false;

  function init() {
    var root = $('#board');
    if (!root) return; // страница без доски
    state.isAdmin = root.dataset.isAdmin === 'true';
    state.viewerId = root.dataset.viewerId || '';
    state.filters = parseFilters();
    bindBoardDelegates(root);

    KT.taskCard.init({
      getData: function () { return { columns: state.columns, clients: state.clients, assignees: state.assignees, isAdmin: state.isAdmin, viewerId: state.viewerId }; },
      onChanged: function () { cardTouchedBoard = true; },
      toastWarnings: toastWarnings
    });
    // 'hidden.bs.modal' — общий момент закрытия карточки ЛЮБЫМ путём (крестик/Esc/фон,
    // а также автозакрытие task-card.js на 404 «Заказ не найден»); board.js слушает его
    // независимо от task-card.js, чтобы убрать deep-link `task` из URL и (один раз)
    // перечитать доску, если карточка что-то поменяла.
    $('#task-modal').addEventListener('hidden.bs.modal', function () {
      stripTaskFromUrl();
      if (cardTouchedBoard) { cardTouchedBoard = false; reloadAndRender(); }
    });
    $('#task-add-btn').addEventListener('click', function () { openTaskForm(null); });

    loadStatic().then(function () {
      // клиент фильтра из URL мог не попасть в первую страницу state.clients (BD-009) —
      // догружаем его отдельно, иначе #filter-client молча остаётся на «Все клиенты».
      return ensureClientOption(state.filters.client_id);
    }).then(function () {
      fillFilterOptions();
      applyFiltersToForm();
      bindFilters();
      if (state.isAdmin) bindColumnForm();
      return reloadAndRender();
    }).then(function () {
      // deep-link открываем ПОСЛЕ полного рендера доски — колонки/клиенты/исполнители уже
      // в state, карточка заказа рендерится с корректными справочниками (не пустыми).
      openDeepLinkTask();
    }).catch(function (err) {
      if (isAbortError(err)) return; // страница уходит (logout/навигация) — доска больше не нужна
      root.innerHTML = '<div class="empty-state">Не удалось загрузить доску' + (err && err.message ? ': ' + esc(err.message) : '') + '</div>';
    });
  }

  KT.board = { init: init };
  document.addEventListener('DOMContentLoaded', init);
})();
