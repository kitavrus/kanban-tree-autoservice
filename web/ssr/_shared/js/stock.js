/* KT.stock — экран «Остатки» «/stock» (rich-экран поверх JSON API, D30; эталон логики —
   renderStock в prototype/index.html, переложенный на Bootstrap 5 + KT.api/KT.enhanceSelect
   этого проекта).

   Данные — GET /api/v1/stock/summary?warehouse_id= (KPI-плитки по парам товар×склад) +
   GET /api/v1/stock?scope=&q=&warehouse_id= (таблица: колонки складов, «Итого», «В пути»;
   склады — r.warehouses, warehouses DS-004). Правка остатка — PUT /api/v1/stock/:product_id
   {stock, warehouse_id}, доступна только admin (кнопка правки не рендерится
   в DOM для executor — testid отсутствует, а не просто скрыт, тот же приём, что и
   task-delete в board.js). После успешной правки — KT.refreshNavBadge() (ADR-004).

   Весь пользовательский текст выводится через esc() (HTML-экранирование, Security §). */
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

  var state = {
    rows: [],
    rowsTotal: 0,
    summary: null,
    scope: 'risky',
    q: '',
    warehouseId: '',
    warehouses: [],
    isAdmin: false,
    editingProductId: null
  };

  // ---------- загрузка ----------
  function loadSummary() {
    var q = state.warehouseId ? '?warehouse_id=' + encodeURIComponent(state.warehouseId) : '';
    return KT.api.get('/api/v1/stock/summary' + q).then(function (r) {
      state.summary = r;
      renderKpis();
    });
  }

  function loadList() {
    var p = new URLSearchParams();
    p.set('scope', state.scope);
    if (state.q) p.set('q', state.q);
    if (state.warehouseId) p.set('warehouse_id', state.warehouseId);
    p.set('limit', '1000');
    return KT.api.get('/api/v1/stock?' + p.toString()).then(function (r) {
      state.rows = r.items;
      state.rowsTotal = r.total;
      setWarehouses(r.warehouses || []);
      renderTable();
    });
  }

  function reloadAll() {
    return Promise.all([loadSummary(), loadList()]);
  }

  // ---------- KPI ----------
  function renderKpis() {
    var s = state.summary;
    if (!s) return;
    $('#stock-kpi-total').textContent = s.products_total;
    $('#stock-kpi-low').textContent = s.low_count;
    $('#stock-kpi-out').textContent = s.out_count;
    $('#stock-kpi-will-low').textContent = s.will_low_count;
  }

  // ---------- склады: фильтр + колонки ----------
  // Склады отчёта: активные и — с ненулевым остатком — архивные (is_archived) и мягко удалённые
  // (is_deleted) с серым бейджем в заголовке колонки (soft-delete-warehouses-suppliers §8.3 п.4,
  // DS-042, DS-049).
  function warehouseMark(w) {
    return w.is_deleted ? 'удалён' : (w.is_archived ? 'в архиве' : '');
  }
  function warehouseLabel(w) {
    var mark = warehouseMark(w);
    return w.name + (mark ? ' (' + mark + ')' : '');
  }

  function setWarehouses(list) {
    var same = list.length === state.warehouses.length && list.every(function (w, i) {
      var o = state.warehouses[i];
      return w.id === o.id && w.name === o.name && !!w.is_archived === !!o.is_archived && !!w.is_deleted === !!o.is_deleted;
    });
    state.warehouses = list;
    if (same) return;
    // склад фильтра ушёл из отчёта (остаток на архивном/удалённом складе списан) — фильтр
    // сбрасывается, иначе следующий запрос списка получил бы 400 warehouse_id: not_found
    if (state.warehouseId && !list.some(function (w) { return w.id === state.warehouseId; })) state.warehouseId = '';
    var sel = $('#stock-warehouse');
    sel.innerHTML = '<option value="">Все склады</option>' + list.map(function (w) {
      return '<option value="' + esc(w.id) + '">' + esc(warehouseLabel(w)) + '</option>';
    }).join('');
    sel.value = state.warehouseId;
    if (sel._cbSync) sel._cbSync(); // combobox.js: подпись кнопки — по выбранной опции
    var anchor = $('[data-wh-cols]');
    $$('th[data-wh-col]').forEach(function (th) { th.remove(); });
    list.forEach(function (w) {
      var th = document.createElement('th');
      th.setAttribute('data-wh-col', w.id);
      th.setAttribute('data-testid', 'stock-col-wh');
      th.textContent = w.name;
      var mark = warehouseMark(w);
      if (mark) {
        var b = document.createElement('span');
        b.className = 'badge text-bg-secondary ms-1';
        b.setAttribute('data-testid', w.is_deleted ? 'stock-col-wh-deleted' : 'stock-col-wh-archived');
        b.textContent = mark;
        th.appendChild(b);
      }
      anchor.parentNode.insertBefore(th, anchor);
    });
    anchor.classList.add('d-none');
  }

  // Сброс фильтра «Склад», если на выбранном в нём недействующем (архивном либо удалённом) складе
  // остаток только что списан до нуля (soft-delete-warehouses-suppliers §8.3: такой склад без
  // остатка в отчёте и в фильтре не показывается).
  function dropFilterIfWrittenOff(warehouseId, newStock) {
    if (newStock !== 0 || state.warehouseId !== warehouseId) return;
    var w = state.warehouses.filter(function (x) { return x.id === warehouseId; })[0];
    if (!w || !(w.is_deleted || w.is_archived)) return;
    dropFilter(warehouseId);
  }
  // Снять фильтр «Склад», если в нём выбран warehouseId.
  function dropFilter(warehouseId) {
    if (state.warehouseId !== warehouseId) return;
    state.warehouseId = '';
    var sel = $('#stock-warehouse');
    sel.value = '';
    if (sel._cbSync) sel._cbSync();
  }

  function cellState(c) {
    if (Number(c.stock) < 0) return 'stock-out';
    if (c.state === 'out') return 'stock-out';
    if (c.state === 'low') return 'stock-low';
    return '';
  }

  function warehouseName(id) {
    var w = state.warehouses.filter(function (x) { return x.id === id; })[0];
    return w ? w.name : '';
  }

  // ---------- таблица ----------
  function stateBadge(row) {
    switch (row.state) {
      case 'out': return '<span class="stock-out">нет в наличии</span>';
      case 'low': return '<span class="stock-low">⚠ на минимуме</span>';
      case 'will_low': return '<span class="stock-low">станет минимумом после заказов</span>';
      default: return '<span class="text-body-secondary">норма</span>';
    }
  }

  function rowHTML(r) {
    var whereabouts = [r.place, r.box ? ('коробка ' + r.box) : ''].filter(Boolean).join(' · ') || '—';
    var stockCls = Number(r.stock) < 0 ? 'stock-out' : (r.state === 'low' ? 'stock-low' : '');
    var forecastCls = r.state === 'will_low' ? 'stock-low' : '';
    var whCells = (r.by_warehouse || []).map(function (c) {
      var dim = state.warehouseId && state.warehouseId !== c.warehouse_id ? ' text-body-tertiary' : '';
      return '  <td class="' + cellState(c) + dim + '" data-testid="stock-cell-wh" data-warehouse="' + esc(warehouseName(c.warehouse_id)) + '">' + esc(String(c.stock)) + '</td>';
    }).join('');
    return '' +
      '<tr data-testid="stock-row-' + esc(r.product_id) + '">' +
      // товар удалён из каталога, но на складе числится его остаток (soft-delete-catalog §8.3
      // п.5, DS-042) — серый бейдж «удалён» справа от названия; корректировка работает как обычно
      '  <td><b>' + esc(r.name) + '</b>' + (r.is_deleted ? ' <span class="badge text-bg-secondary" data-testid="stock-row-deleted">удалён</span>' : '') +
      // архивный товар с остатком на удалённом складе — серый бейдж «в архиве» (остаток только списывается)
      (r.is_archived ? ' <span class="badge text-bg-secondary" data-testid="stock-row-archived">в архиве</span>' : '') + '</td>' +
      '  <td class="d-none d-md-table-cell text-body-secondary small">' + esc(whereabouts) + '</td>' +
      whCells +
      '  <td class="' + stockCls + '" data-testid="stock-cell-total">' + esc(String(r.stock)) + ' ' + esc(r.unit) + (Number(r.stock) < 0 ? ' (долг)' : '') + '</td>' +
      '  <td class="d-none d-md-table-cell" data-testid="stock-cell-transit">' + esc(String(r.in_transit == null ? 0 : r.in_transit)) + '</td>' +
      '  <td>' + esc(String(r.min_stock)) + '</td>' +
      '  <td class="d-none d-md-table-cell">' + esc(String(r.pending)) + '</td>' +
      '  <td class="d-none d-md-table-cell ' + forecastCls + '">' + esc(String(r.forecast)) + '</td>' +
      '  <td>' + stateBadge(r) + '</td>' +
      '  <td class="text-end">' + (state.isAdmin ?
        '<button type="button" class="btn btn-sm btn-outline-secondary stock-edit-btn" data-product-id="' + esc(r.product_id) + '" data-testid="stock-edit-' + esc(r.product_id) + '">Изменить</button>' : '') +
      '  </td>' +
      '</tr>';
  }

  function renderTable() {
    $('#stock-tbody').innerHTML = state.rows.map(rowHTML).join('');
    $('#stock-empty').classList.toggle('d-none', state.rows.length > 0);
    // Список грузится одной страницей (limit=1000): если товаров больше — говорим об этом,
    // а не обрезаем молча.
    var shown = $('#stock-shown');
    var cut = state.rowsTotal > state.rows.length;
    shown.classList.toggle('d-none', !cut);
    shown.textContent = cut ? 'Показаны первые ' + state.rows.length + ' из ' + state.rowsTotal + ', уточните поиск' : '';
    bindRowButtons();
  }

  function bindRowButtons() {
    $$('.stock-edit-btn').forEach(function (btn) {
      btn.addEventListener('click', function () { openEditModal(btn.dataset.productId); });
    });
  }

  function rowById(productId) {
    return state.rows.filter(function (r) { return r.product_id === productId; })[0];
  }

  // ---------- фильтры ----------
  function bindScope() {
    $('#stock-scope').addEventListener('change', function (e) {
      state.scope = e.target.value;
      loadList();
    });
  }

  function bindWarehouse() {
    $('#stock-warehouse').addEventListener('change', function (e) {
      state.warehouseId = e.target.value;
      reloadAll();
    });
  }

  function bindSearch() {
    var timer = null;
    $('#stock-q').addEventListener('input', function (e) {
      var v = e.target.value;
      clearTimeout(timer);
      timer = setTimeout(function () {
        state.q = v;
        loadList();
      }, 250);
    });
  }

  // ---------- модалка правки остатка (admin) ----------
  var stockModal = null;
  var stockSaving = false;

  function openEditModal(productId) {
    var row = rowById(productId);
    if (!row) return;
    state.editingProductId = productId;
    $('#stock-modal-title').textContent = 'Остаток «' + row.name + '»';
    var whSel = $('#stock-modal-warehouse');
    var cellOf = function (whId) {
      return (row.by_warehouse || []).filter(function (c) { return c.warehouse_id === whId; })[0];
    };
    var hasStock = function (w) {
      var c = cellOf(w.id);
      return !!c && Number(c.stock) !== 0;
    };
    // Склады окна (DS-048, DS-049): у удалённого товара — только склады с его ненулевым остатком
    // (активные, архивные, удалённые): остаток можно только списать. У действующего товара —
    // активные склады, как раньше, плюс удалённые склады с его остатком (там тоже только списание).
    var options = state.warehouses.filter(function (w) {
      if (row.is_deleted) return hasStock(w);
      // архивный товар в отчёте — только ради остатка на удалённом складе: списать можно лишь его
      if (row.is_archived) return w.is_deleted && hasStock(w);
      return (!w.is_archived && !w.is_deleted) || (w.is_deleted && hasStock(w));
    });
    whSel.innerHTML = options.map(function (w) {
      return '<option value="' + esc(w.id) + '">' + esc(warehouseLabel(w)) + '</option>';
    }).join('');
    var preferred = options.some(function (w) { return w.id === state.warehouseId; }) ? state.warehouseId : '';
    whSel.value = preferred || (options[0] && options[0].id) || '';
    var syncValue = function () {
      var c = cellOf(whSel.value);
      $('#stock-value').value = c ? c.stock : 0;
    };
    whSel.onchange = syncValue;
    if (whSel._cbSync) whSel._cbSync();
    syncValue();
    $('#stock-error').classList.add('d-none');
    $('#stock-error').textContent = '';
    $('#stock-save-btn').disabled = false;
    stockSaving = false;
    if (!stockModal) stockModal = new bootstrap.Modal($('#stock-modal'));
    stockModal.show();
  }

  function showError(msg) {
    var el = $('#stock-error');
    el.textContent = msg;
    el.classList.remove('d-none');
  }

  function bindForm() {
    $('#stock-form-el').addEventListener('submit', function (e) {
      e.preventDefault();
      if (stockSaving) return; // защита от двойного клика/двойного submit

      var raw = $('#stock-value').value.trim();
      var n = Number(raw);
      if (raw === '' || Number.isNaN(n)) {
        showError('Введите число');
        return;
      }

      stockSaving = true;
      $('#stock-save-btn').disabled = true;

      var editedWarehouseId = $('#stock-modal-warehouse').value;
      KT.api.put('/api/v1/stock/' + state.editingProductId, { stock: n, warehouse_id: editedWarehouseId })
        .then(function () {
          stockModal.hide();
          // остаток на архивном либо удалённом складе списан до нуля, а этот склад выбран в
          // фильтре: склад мог уйти из отчёта, и запрос списка с ним получил бы 400
          // warehouse_id: not_found — фильтр сбрасывается ДО перезагрузки
          dropFilterIfWrittenOff(editedWarehouseId, n);
          // строка могла выйти из scope=risky (перестала быть рисковой) или наоборот —
          // проще перезагрузить и таблицу, и KPI, чем патчить локально.
          return reloadAll();
        })
        .then(function () {
          KT.refreshNavBadge();
        })
        .catch(function (err) {
          stockSaving = false;
          $('#stock-save-btn').disabled = false;
          // остаток на удалённом складе уже нулевой — склада для корректировки больше нет
          // (soft-delete-warehouses-suppliers §8.3): тост, окно закрывается, отчёт перечитывается
          if (err && err.fields && err.fields.some(function (f) { return f.name === 'warehouse_id' && f.message === 'not_found'; })) {
            stockModal.hide();
            KT.uiToast('Склад не найден', true);
            dropFilter(editedWarehouseId); // склада в отчёте больше нет — и в фильтре тоже
            reloadAll();
            return;
          }
          if (err && err.fields && err.fields.length) {
            showError(err.fields.map(function (f) { return f.message; }).join('; '));
            return;
          }
          // товара для корректировки больше нет (удалён, остаток уже нулевой — DS-042): тост,
          // окно закрывается, отчёт перечитывается без этой строки
          if (err && err.status === 404) {
            stockModal.hide();
            KT.uiToast(err.message || 'Товар не найден', true);
            reloadAll();
            return;
          }
          showError((err && err.message) || 'Не удалось сохранить остаток');
        });
    });
  }

  // ---------- инициализация ----------
  function init() {
    var root = $('#stock');
    if (!root) return; // страница без экрана остатков
    state.isAdmin = root.dataset.isAdmin === 'true';

    bindScope();
    bindWarehouse();
    bindSearch();
    bindForm();

    reloadAll().catch(function (err) {
      root.innerHTML = '<div class="empty-state">Не удалось загрузить остатки' + (err && err.message ? ': ' + esc(err.message) : '') + '</div>';
    });
  }

  KT.stock = { init: init };
  document.addEventListener('DOMContentLoaded', init);
})();
