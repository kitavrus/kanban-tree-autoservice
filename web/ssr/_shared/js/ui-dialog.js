/* Модальные диалоги вместо нативных prompt/confirm/alert — во встроенных браузерах
   нативные подавляются (эталон — uiDialog в prototype/index.html). Разметка и фокус —
   Bootstrap Modal. Все функции возвращают Promise:
     KT.uiDialog({title, text, input, ok, cancel, danger}) → true|false | строка|null
     KT.uiDialog({title, text, select: [{value, text}], value}) → value выбранной опции|null
       (нативный <select data-native>, не комбобокс; warehouses DS-008 — выбор склада)
     KT.uiPrompt(title, value) → строка|null
     KT.uiConfirm(text, ok='Удалить') → true|false
     KT.uiAlert(text, title='Нельзя') → true
   Текст выводится через textContent — HTML в сообщениях не интерпретируется. */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  // Очередь диалогов: следующий показывается только ПОСЛЕ того, как предыдущий полностью
  // скрыт и удалён из DOM (hidden.bs.modal). Без неё повторный вызов во время fade-out
  // предыдущего (напр. «Отмена» → сразу снова «Удалить») давал два узла [data-testid=
  // ui-dialog] разом и два backdrop'а (AT-005 фазы 7).
  var queue = Promise.resolve();

  function uiDialog(opts) {
    var run = queue.then(function () { return showDialog(opts); });
    queue = run.then(function () {}, function () {});
    return run;
  }

  function showDialog(opts) {
    var o = Object.assign({ text: '', input: null, select: null, ok: 'OK', cancel: 'Отмена', danger: false }, opts);
    return new Promise(function (resolve) {
      var root = el('div', 'modal fade');
      root.tabIndex = -1;
      root.setAttribute('data-testid', 'ui-dialog');
      var dialog = el('div', 'modal-dialog modal-dialog-centered');
      var content = el('div', 'modal-content');
      var head = el('div', 'modal-header');
      head.append(el('h2', 'modal-title fs-5', o.title));
      var x = el('button', 'btn-close');
      x.type = 'button';
      x.setAttribute('data-bs-dismiss', 'modal');
      x.setAttribute('aria-label', 'Закрыть');
      head.append(x);

      var body = el('div', 'modal-body');
      if (o.text) {
        var p = el('p', 'mb-0', o.text);
        p.style.whiteSpace = 'pre-line';
        p.setAttribute('data-testid', 'ui-dialog-text');
        body.append(p);
      }
      var inp = null;
      if (o.input !== null) {
        inp = el('input', 'form-control' + (o.text ? ' mt-3' : ''));
        inp.value = o.input;
        inp.setAttribute('data-testid', 'ui-dialog-input');
        body.append(inp);
      } else if (o.select) {
        inp = el('select', 'form-select' + (o.text ? ' mt-3' : ''));
        inp.setAttribute('data-native', '1');
        inp.setAttribute('data-testid', 'ui-dialog-select');
        o.select.forEach(function (opt) {
          var op = el('option', null, opt.text);
          op.value = opt.value;
          inp.append(op);
        });
        if (o.value != null) inp.value = o.value;
        body.append(inp);
      }

      var foot = el('div', 'modal-footer');
      if (o.cancel) {
        var c = el('button', 'btn btn-outline-secondary', o.cancel);
        c.type = 'button';
        c.setAttribute('data-bs-dismiss', 'modal');
        c.setAttribute('data-testid', 'ui-dialog-cancel');
        foot.append(c);
      }
      var ok = el('button', 'btn ' + (o.danger ? 'btn-danger' : 'btn-primary'), o.ok);
      ok.type = 'button';
      ok.setAttribute('data-testid', 'ui-dialog-ok');
      foot.append(ok);

      content.append(head, body, foot);
      dialog.append(content);
      root.append(dialog);
      document.body.append(root);

      var result = inp ? null : false;
      var modal = new bootstrap.Modal(root);
      var finish = function () { result = inp ? inp.value : true; modal.hide(); };
      ok.addEventListener('click', finish);
      if (inp) inp.addEventListener('keydown', function (e) { if (e.key === 'Enter') { e.preventDefault(); finish(); } });
      root.addEventListener('shown.bs.modal', function () { if (inp) { inp.focus(); if (inp.select && inp.tagName === 'INPUT') inp.select(); } else ok.focus(); });
      root.addEventListener('hidden.bs.modal', function () { modal.dispose(); root.remove(); resolve(result); });
      if (KT.closeCombobox) KT.closeCombobox();
      modal.show();
    });
  }

  /* Окно «Удаление» с обязательной причиной (soft-delete-core §8.2, DS-020) — общее для всех
     видов записей:
       KT.uiDeleteDialog({title, text, note, submit(reason) → Promise, onOffline()}) → Promise
         → null                — отмена (кнопка, крестик, Esc);
         → {result: ответ}     — submit выполнен, окно закрыто;
         → {error: ApiError}   — отказ, после которого окно закрывается (404, 403, прочее).
     Окно остаётся открытым: 400 по полю reason — красный текст под полем; 409 — красный блок
     над кнопками с текстом сервера; потеря связи — onOffline() (тост), причина сохраняется.
     Пустая причина на клиенте не проверяется — проверяет сервер.
     opts.reason / opts.reasonError — начальный текст причины и ошибка под полем: серверная
     форма удаления (form[data-delete-dialog], ниже) открывается заново с отказом по причине
     (soft-delete-clients §8.1). */
  var REASON_MAX = 500;
  var REASON_ERRORS = {
    required: 'Укажите причину удаления',
    too_long: 'Причина — не длиннее 500 символов',
    invalid_text: 'Недопустимые символы в причине'
  };

  function uiDeleteDialog(opts) {
    var run = queue.then(function () { return showDeleteDialog(opts); });
    queue = run.then(function () {}, function () {});
    return run;
  }

  function showDeleteDialog(opts) {
    return new Promise(function (resolve) {
      var root = el('div', 'modal fade');
      root.tabIndex = -1;
      root.setAttribute('data-testid', 'ui-dialog');
      root.setAttribute('data-dialog-kind', 'delete');
      var dialog = el('div', 'modal-dialog modal-dialog-centered');
      var content = el('div', 'modal-content');
      var head = el('div', 'modal-header');
      var h = el('h2', 'modal-title fs-5', opts.title);
      h.style.overflowWrap = 'anywhere';
      h.setAttribute('data-testid', 'delete-dialog-title');
      head.append(h);
      var x = el('button', 'btn-close');
      x.type = 'button';
      x.setAttribute('data-bs-dismiss', 'modal');
      x.setAttribute('aria-label', 'Закрыть');
      head.append(x);

      var body = el('div', 'modal-body');
      if (opts.text) {
        var p = el('p', null, opts.text);
        p.setAttribute('data-testid', 'ui-dialog-text');
        body.append(p);
      }
      if (opts.note) { // вторая строка пояснения (soft-delete-transfers §8.3)
        var note = el('p', null, opts.note);
        note.setAttribute('data-testid', 'ui-dialog-note');
        body.append(note);
      }
      var label = el('label', 'form-label', 'Причина *');
      label.htmlFor = 'kt-delete-reason';
      var ta = el('textarea', 'form-control');
      ta.id = 'kt-delete-reason';
      ta.rows = 3;
      ta.setAttribute('data-testid', 'delete-reason');
      var fieldErr = el('div', 'invalid-feedback');
      fieldErr.setAttribute('data-testid', 'delete-reason-error');
      var counter = el('div', 'form-text text-end', '0 / ' + REASON_MAX);
      counter.setAttribute('data-testid', 'delete-reason-counter');
      var blockErr = el('div', 'alert alert-danger d-none mt-3 mb-0');
      blockErr.setAttribute('role', 'alert');
      blockErr.setAttribute('data-testid', 'delete-dialog-error');
      body.append(label, ta, fieldErr, counter, blockErr);

      var foot = el('div', 'modal-footer');
      var c = el('button', 'btn btn-outline-secondary', 'Отмена');
      c.type = 'button';
      c.setAttribute('data-bs-dismiss', 'modal');
      c.setAttribute('data-testid', 'ui-dialog-cancel');
      var ok = el('button', 'btn btn-danger', opts.ok || 'Удалить');
      ok.type = 'button';
      ok.setAttribute('data-testid', 'ui-dialog-ok');
      foot.append(c, ok);

      content.append(head, body, foot);
      dialog.append(content);
      root.append(dialog);
      document.body.append(root);

      var outcome = null;
      var modal = new bootstrap.Modal(root);
      function clearErrors() {
        ta.classList.remove('is-invalid');
        fieldErr.textContent = '';
        blockErr.classList.add('d-none');
        blockErr.textContent = '';
      }
      ta.addEventListener('input', function () {
        counter.textContent = Array.from(ta.value).length + ' / ' + REASON_MAX;
        ta.classList.remove('is-invalid');
        fieldErr.textContent = '';
      });
      ok.addEventListener('click', function () {
        if (ok.disabled) return;
        ok.disabled = true;
        clearErrors();
        Promise.resolve().then(function () { return opts.submit(ta.value); }).then(function (resp) {
          outcome = { result: resp };
          modal.hide();
        }).catch(function (err) {
          ok.disabled = false;
          var api = KT.ApiError && err instanceof KT.ApiError;
          if (!api) { if (opts.onOffline) opts.onOffline(err); return; }
          var field = err.status === 400 && (err.fields || []).filter(function (f) { return f.name === 'reason'; })[0];
          if (field) {
            fieldErr.textContent = REASON_ERRORS[field.message] || field.message;
            ta.classList.add('is-invalid');
            ta.focus();
            return;
          }
          if (err.status === 409) {
            blockErr.textContent = err.message;
            blockErr.classList.remove('d-none');
            return;
          }
          outcome = { error: err };
          modal.hide();
        });
      });
      if (opts.reason) {
        ta.value = opts.reason;
        counter.textContent = Array.from(ta.value).length + ' / ' + REASON_MAX;
      }
      if (opts.reasonError) {
        fieldErr.textContent = opts.reasonError;
        ta.classList.add('is-invalid');
      }
      root.addEventListener('shown.bs.modal', function () { ta.focus(); });
      root.addEventListener('hidden.bs.modal', function () { modal.dispose(); root.remove(); resolve(outcome); });
      if (KT.closeCombobox) KT.closeCombobox();
      modal.show();
    });
  }

  /* KT.uiToast(text, isError) — тост в правом верхнем углу, 5 секунд (UI-конвенции DS-033);
     текст — через textContent. */
  function uiToast(text, isError) {
    var host = document.getElementById('kt-toasts');
    if (!host) {
      host = el('div', 'toast-container position-fixed top-0 end-0 p-3');
      host.id = 'kt-toasts';
      host.style.zIndex = '1090';
      host.style.maxWidth = 'min(380px, 100vw)';
      host.setAttribute('aria-live', 'polite');
      document.body.append(host);
    }
    var t = el('div', 'alert shadow mb-2 ' + (isError ? 'alert-danger' : 'alert-success'), text);
    t.setAttribute('role', isError ? 'alert' : 'status');
    t.setAttribute('data-testid', isError ? 'toast-error' : 'toast');
    host.append(t);
    setTimeout(function () { t.remove(); }, 5000);
  }

  KT.uiDeleteDialog = uiDeleteDialog;
  KT.uiToast = uiToast;
  KT.uiDialog = uiDialog;
  KT.uiPrompt = function (title, value) { return uiDialog({ title: title, input: value == null ? '' : String(value) }); };
  KT.uiConfirm = function (text, ok) { return uiDialog({ title: 'Подтверждение', text: text, ok: ok || 'Удалить', danger: true }); };
  KT.uiAlert = function (text, title) { return uiDialog({ title: title || 'Нельзя', text: text, cancel: null }); };

  /* Серверная форма мягкого удаления (soft-delete-clients §8.1, DS-032):
       form[data-delete-dialog] с data-delete-title / data-delete-text и скрытым полем reason.
     Сабмит перехватывается — открывается общее окно «Удаление»; причина из окна подставляется
     в скрытое поле, форма уходит обычным POST (PRG), кнопка окна остаётся заблокированной до
     перехода. data-delete-error — сервер вернул форму с отказом по причине: окно открывается
     сразу, с прежней причиной и текстом под полем. */
  function openDeleteForm(form) {
    var field = form.querySelector('input[name="reason"]');
    if (!field) return;
    uiDeleteDialog({
      title: form.getAttribute('data-delete-title') || 'Удаление',
      text: form.getAttribute('data-delete-text') || '',
      reason: field.value,
      reasonError: form.getAttribute('data-delete-error') || '',
      submit: function (reason) {
        field.value = reason;
        form.dataset.confirmed = '1';
        HTMLFormElement.prototype.submit.call(form);
        return new Promise(function () {}); // страница уходит на PRG-переход
      }
    });
  }
  function openFailedDeleteForms() {
    Array.prototype.forEach.call(document.querySelectorAll('form[data-delete-dialog][data-delete-error]'), openDeleteForm);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', openFailedDeleteForms);
  else openFailedDeleteForms();

  /* Тост после PRG-перехода: элемент [data-kt-toast] с текстом (серверные формы). */
  function showPageToasts() {
    Array.prototype.forEach.call(document.querySelectorAll('[data-kt-toast]'), function (n) {
      uiToast(n.getAttribute('data-kt-toast'), false);
    });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', showPageToasts);
  else showPageToasts();

  /* Делегированный submit-обработчик форм (ADR-103, Tech Spec «Двойной submit»):
       form[data-confirm="текст"] — сабмит перехватывается, показывается KT.uiConfirm;
       подтвердили → реальный сабмит через HTMLFormElement.prototype.submit (минуя
       повторную отправку события submit, иначе бесконечный цикл); отменили → ничего
       не отправляется. form[data-once] — на настоящем сабмите (после возможного
       confirm) кнопка отправки блокируется, чтобы повторный клик/Enter не отправил
       форму дважды, пока идёт PRG-редирект. */
  document.addEventListener('submit', function (e) {
    var form = e.target;
    if (!(form instanceof HTMLFormElement)) return;

    if (form.matches('[data-delete-dialog]') && !form.dataset.confirmed) {
      e.preventDefault();
      openDeleteForm(form);
      return;
    }

    if (form.matches('[data-confirm]') && !form.dataset.confirmed) {
      e.preventDefault();
      KT.uiConfirm(form.getAttribute('data-confirm')).then(function (ok) {
        if (!ok) return;
        form.dataset.confirmed = '1';
        HTMLFormElement.prototype.submit.call(form);
      });
      return;
    }

    if (form.matches('[data-once]') && !form.dataset.submitted) {
      form.dataset.submitted = '1';
      var btn = form.querySelector('button[type="submit"]');
      if (btn) btn.disabled = true;
    }
  });
})();
