/* KT.stock — экран «Остатки» «/stock» (rich-экран поверх JSON API, D30; эталон логики —
   renderStock в prototype/index.html, переложенный на Bootstrap 5 + KT.api/KT.enhanceSelect
   этого проекта).

   Данные — GET /api/v1/stock/summary (KPI-плитки) + GET /api/v1/stock?scope=&q= (таблица:
   одна строка на товар). Правка остатка — PUT /api/v1/stock/:product_id {stock}, доступна
   только admin (кнопка правки не рендерится
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
    isAdmin: false,
    editingProductId: null
  };

  // ---------- загрузка ----------
  function loadSummary() {
    return KT.api.get('/api/v1/stock/summary').then(function (r) {
      state.summary = r;
      renderKpis();
    });
  }

  function loadList() {
    var p = new URLSearchParams();
    p.set('scope', state.scope);
    if (state.q) p.set('q', state.q);
    p.set('limit', '1000');
    return KT.api.get('/api/v1/stock?' + p.toString()).then(function (r) {
      state.rows = r.items;
      state.rowsTotal = r.total;
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
    return '' +
      '<tr data-testid="stock-row-' + esc(r.product_id) + '">' +
      // товар удалён из каталога, но на складе числится его остаток (soft-delete-catalog §8.3
      // п.5, DS-042) — серый бейдж «удалён» справа от названия; остаток можно только списать
      '  <td><b>' + esc(r.name) + '</b>' + (r.is_deleted ? ' <span class="badge text-bg-secondary" data-testid="stock-row-deleted">удалён</span>' : '') + '</td>' +
      '  <td class="d-none d-md-table-cell text-body-secondary small">' + esc(whereabouts) + '</td>' +
      '  <td class="' + stockCls + '" data-testid="stock-cell-total">' + esc(String(r.stock)) + ' ' + esc(r.unit) + (Number(r.stock) < 0 ? ' (долг)' : '') + '</td>' +
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
    $('#stock-value').value = row.stock;
    // удалённый товар (DS-042): остаток только списывается — подсказка в окне, сервер на
    // увеличение/смену знака ответит 409 PRODUCT_DELETED_WRITE_OFF_ONLY (текст — в окне)
    $('#stock-write-off-hint').classList.toggle('d-none', !row.is_deleted);
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

      KT.api.put('/api/v1/stock/' + state.editingProductId, { stock: n })
        .then(function () {
          stockModal.hide();
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
    bindSearch();
    bindForm();

    reloadAll().catch(function (err) {
      root.innerHTML = '<div class="empty-state">Не удалось загрузить остатки' + (err && err.message ? ': ' + esc(err.message) : '') + '</div>';
    });
  }

  KT.stock = { init: init };
  document.addEventListener('DOMContentLoaded', init);
})();
