/* KT.calendar — экран «Календарь» «/calendar». Сетку загрузки рисует сервер; здесь только
   сохранение вместимости (admin): PUT /api/v1/calendar/settings {max_cars}, затем
   перезагрузка страницы — счётчики «свободно» пересчитываются сервером. */
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

  KT.calendar = { save: save };

  document.addEventListener('DOMContentLoaded', function () {
    var btn = document.getElementById('cal-save');
    var input = document.getElementById('cal-max-cars');
    if (!btn || !input) return;
    btn.addEventListener('click', save);
    input.addEventListener('keydown', function (e) { if (e.key === 'Enter') save(); });
  });
})();
