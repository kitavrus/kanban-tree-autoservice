/* KT.refreshNavBadge — счётчик рисковых товаров в пункте меню «Остатки» (ADR-004, AC-016):
   GET /api/v1/stock/summary → risk_count → [data-nav-badge="/stock"] (hidden при 0).
   Подключается после api.js; на страницах, где элемента нет (гость/раздел скрыт), — no-op.
   Инвалидация после мутаций доски/остатков — вызовом KT.refreshNavBadge() из board.js/
   stock.js (design-go-layers.md §Frontend Modules). */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  KT.refreshNavBadge = function () {
    var el = document.querySelector('[data-nav-badge="/stock"]');
    if (!el || !KT.api) return;
    KT.api.get('/api/v1/stock/summary').then(function (summary) {
      var n = (summary && summary.risk_count) || 0;
      el.textContent = n;
      el.hidden = n === 0;
    }).catch(function () {
      /* бейдж — не критичная функциональность страницы, ошибку тихо игнорируем */
    });
  };

  document.addEventListener('DOMContentLoaded', function () {
    KT.refreshNavBadge();
  });
})();
