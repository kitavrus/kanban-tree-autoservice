/* KT.calendar — экран «Календарь» «/calendar». Сетку загрузки рисует сервер; здесь:
   - сохранение вместимости (admin): PUT /api/v1/calendar/settings {max_cars}, затем
     перезагрузка страницы — счётчики «свободно» пересчитываются сервером;
   - новый заказ: та же карточка заказа, что на доске (js/task-card.js, KT.taskCard).
     «+ Заказ» в шапке — без срока, «+ Заказ со сроком на этот день» — срок = выбранный день;
     клик по заказу (список дня, полоса диаграммы Ганта — [data-open-task]) открывает его
     карточку здесь же, без перехода на доску (обычный клик; с Ctrl/⌘ — ссылка /?task=N).
     Справочники карточки (колонки/исполнители/клиенты) грузятся при первом открытии; после
     закрытия карточки, если заказ создан или изменён, страница перезагружается — сетка и
     список дня пересчитываются сервером. */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  var FIELD_TEXT = {
    required: 'укажите число мест',
    out_of_range: 'допустимо от 5 до 12 машин'
  };

  function showError(msg) {
    var box = document.getElementById('cal-error');
    if (!box) return;
    box.textContent = msg;
    box.classList.toggle('d-none', !msg);
  }

  function save() {
    var input = document.getElementById('cal-max-cars');
    var btn = document.getElementById('cal-save');
    var raw = String(input.value).trim();
    var n = Number(raw);
    if (raw === '' || !Number.isInteger(n)) {
      showError('Мест одновременно: ' + FIELD_TEXT.required);
      return;
    }
    showError('');
    btn.disabled = true;
    KT.api.put('/api/v1/calendar/settings', { max_cars: n }).then(function () {
      window.location.reload();
    }).catch(function (err) {
      btn.disabled = false;
      var f = (err.fields || [])[0];
      showError('Мест одновременно: ' + ((f && FIELD_TEXT[f.message]) || err.message || 'не удалось сохранить'));
    });
  }

  // ---------- новый заказ ----------
  var cardData = null;     // справочники карточки: {columns, assignees, clients, isAdmin, viewerId}
  var cardTouched = false; // карточка что-то сохранила — после закрытия перерисовать страницу

  function loadAll(url) {
    return KT.api.get(url).then(function (r) { return r.items || []; });
  }

  function ensureCardData() {
    if (cardData) return Promise.resolve();
    var root = document.getElementById('cal');
    return Promise.all([
      loadAll('/api/v1/columns?limit=1000'),
      loadAll('/api/v1/assignees?limit=1000'),
      loadAll('/api/v1/clients?limit=200')
    ]).then(function (r) {
      cardData = {
        columns: r[0], assignees: r[1], clients: r[2],
        isAdmin: root.dataset.isAdmin === 'true', viewerId: root.dataset.viewerId || ''
      };
    });
  }

  function toastWarnings(warnings) {
    if (!warnings || !warnings.length || !KT.uiToast) return;
    KT.uiToast('Внимание, товары на минимуме: ' + warnings.map(function (w) { return w.name; }).join(', '), true);
  }

  function openOrder(id) {
    showError('');
    ensureCardData().then(function () {
      KT.taskCard.open(id, null);
    }).catch(function (err) {
      showError('Не удалось открыть заказ: ' + ((err && err.message) || 'ошибка сети'));
    });
  }

  function openNewOrder(dueDate) {
    showError('');
    ensureCardData().then(function () {
      KT.taskCard.open(null, null, { dueDate: dueDate || '' });
    }).catch(function (err) {
      showError('Не удалось открыть новый заказ: ' + ((err && err.message) || 'ошибка сети'));
    });
  }

  function initNewOrder() {
    var modal = document.getElementById('task-modal');
    if (!modal || !KT.taskCard) return;
    KT.taskCard.init({
      getData: function () { return cardData; },
      onChanged: function () { cardTouched = true; },
      toastWarnings: toastWarnings
    });
    modal.addEventListener('hidden.bs.modal', function () {
      if (cardTouched) window.location.reload();
    });
    var add = document.getElementById('task-add-btn');
    if (add) add.addEventListener('click', function () { openNewOrder(''); });
    document.addEventListener('click', function (e) {
      var a = e.target.closest('[data-open-task]');
      if (!a || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
      var id = parseInt(a.dataset.openTask, 10);
      if (!(id > 0)) return;
      e.preventDefault();
      openOrder(id);
    });
    document.querySelectorAll('[data-testid="cal-day-add"]').forEach(function (b) {
      b.addEventListener('click', function () { openNewOrder(b.dataset.due); });
    });
  }

  KT.calendar = { save: save, openNewOrder: openNewOrder, openOrder: openOrder };

  document.addEventListener('DOMContentLoaded', function () {
    initNewOrder();
    var btn = document.getElementById('cal-save');
    var input = document.getElementById('cal-max-cars');
    if (!btn || !input) return;
    btn.addEventListener('click', save);
    input.addEventListener('keydown', function (e) { if (e.key === 'Enter') save(); });
  });
})();
