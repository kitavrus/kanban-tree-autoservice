/* История оборудования (/equipment/:id/history): правка комплектности/состояния при
   приёме в заказ — inline-форма в строке «Принято». Сохраняет через JSON API карточки
   заказа PUT /api/v1/tasks/:id/equipment/:equipment_id (та же валидация и блокировка
   заказа, что в карточке на доске). */
(function () {
  'use strict';

  var FIELD_TEXT = { too_long: 'Не больше 1000 символов' };

  function intakeText(completeness, condition) {
    var parts = [];
    if (completeness) parts.push('комплектность — ' + completeness);
    if (condition) parts.push('состояние — ' + condition);
    return parts.length ? parts.join('; ') : 'без замечаний';
  }

  function init(box) {
    var view = box.querySelector('[data-intake-view]');
    var form = box.querySelector('[data-intake-form]');
    var text = box.querySelector('[data-intake-text]');
    var errBox = box.querySelector('[data-intake-error]');
    var saveBtn = form.querySelector('[type="submit"]');
    var saved = { completeness: form.completeness.value, condition: form.condition.value };

    function clearErrors() {
      errBox.hidden = true;
      errBox.textContent = '';
      ['completeness', 'condition'].forEach(function (name) {
        form[name].classList.remove('is-invalid');
        form.querySelector('[data-field-error="' + name + '"]').textContent = '';
      });
    }

    function open(on) {
      clearErrors();
      if (on) {
        form.completeness.value = saved.completeness;
        form.condition.value = saved.condition;
      }
      form.hidden = !on;
      view.classList.toggle('d-none', on); // не hidden: .d-flex (!important) его перебивает
      if (on) form.completeness.focus();
    }

    box.querySelector('[data-intake-edit]').addEventListener('click', function () { open(true); });
    box.querySelector('[data-intake-cancel]').addEventListener('click', function () { open(false); });
    form.addEventListener('keydown', function (e) { if (e.key === 'Escape') open(false); });

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      clearErrors();
      saveBtn.disabled = true;
      var body = { completeness: form.completeness.value.trim(), condition: form.condition.value.trim() };
      var url = '/api/v1/tasks/' + encodeURIComponent(box.dataset.taskId) +
        '/equipment/' + encodeURIComponent(box.dataset.equipmentId);
      KT.api.put(url, body).then(function () {
        saved = body;
        text.textContent = intakeText(body.completeness, body.condition);
        open(false);
      }).catch(function (err) {
        var shown = false;
        (err.fields || []).forEach(function (f) {
          var name = f.name || f.Name;
          var input = form[name];
          if (!input) return;
          input.classList.add('is-invalid');
          form.querySelector('[data-field-error="' + name + '"]').textContent =
            FIELD_TEXT[f.message || f.Message] || 'Недопустимое значение';
          shown = true;
        });
        if (!shown) {
          errBox.textContent = err.status === 404
            ? 'Автомобиль больше не числится в этом заказе — обновите страницу'
            : (err.message || 'Не удалось сохранить');
          errBox.hidden = false;
        }
      }).then(function () { saveBtn.disabled = false; });
    });
  }

  document.querySelectorAll('[data-intake]').forEach(init);
})();
