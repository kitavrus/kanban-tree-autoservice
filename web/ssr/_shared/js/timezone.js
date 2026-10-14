/* KT.time — единый форматтер даты и времени в ЧАСОВОМ ПОЯСЕ ПОЛЬЗОВАТЕЛЯ (DS-031, DS-032).
   Пояс (IANA) рендерит SSR-layout атрибутом <body data-timezone>; пояс браузера на показ
   не влияет. Неизвестное браузеру имя пояса → Asia/Almaty, страница не ломается.

     KT.time.zone            — действующий пояс
     KT.time.fmtDate(iso)    — «07.10.2026»
     KT.time.fmtDateTime(iso)— «07.10.2026 00:30»
     KT.time.today()         — текущая календарная дата в поясе пользователя, «2026-10-07»
     KT.time.addDays(ymd, n) — календарная дата ± n дней, «2026-10-08»

   Экранам запрещено форматировать время сами (getHours/toLocaleString без timeZone) —
   выйдет пояс браузера.

   Плюс select «Часовой пояс» в сайдбаре ([data-timezone-select]): PUT
   /api/v1/users/me/timezone → перезагрузка страницы; ошибка → тост и прежнее значение. */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};
  var DEFAULT_ZONE = 'Asia/Almaty';
  var TOAST_MS = 5000;

  function supported(zone) {
    try { new Intl.DateTimeFormat('ru-RU', { timeZone: zone }); return true; } catch (e) { return false; }
  }

  var raw = (document.body && document.body.getAttribute('data-timezone')) || '';
  var zone = raw && supported(raw) ? raw : DEFAULT_ZONE;
  // en-GB + h23: стабильные числовые части без «24:00» и без локальных разделителей.
  var partsFmt = new Intl.DateTimeFormat('en-GB', {
    timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hourCycle: 'h23'
  });

  // parts — {year, month, day, hour, minute} момента d в поясе пользователя; null — не дата.
  function parts(value) {
    if (value === null || value === undefined || value === '') return null;
    var d = value instanceof Date ? value : new Date(value);
    if (isNaN(d.getTime())) return null;
    var out = {};
    partsFmt.formatToParts(d).forEach(function (p) { out[p.type] = p.value; });
    return out;
  }

  function fmtDate(value) {
    var p = parts(value);
    return p ? p.day + '.' + p.month + '.' + p.year : '';
  }

  function fmtDateTime(value) {
    var p = parts(value);
    return p ? p.day + '.' + p.month + '.' + p.year + ' ' + p.hour + ':' + p.minute : '';
  }

  function today() {
    var p = parts(new Date());
    return p.year + '-' + p.month + '-' + p.day;
  }

  // addDays — календарная арифметика без пояса (UTC-полночь): дата не «съезжает».
  function addDays(ymd, n) {
    var a = String(ymd).split('-');
    return new Date(Date.UTC(+a[0], +a[1] - 1, +a[2] + n)).toISOString().slice(0, 10);
  }

  KT.time = { zone: zone, fmtDate: fmtDate, fmtDateTime: fmtDateTime, today: today, addDays: addDays };

  // ---------- select «Часовой пояс» в сайдбаре ----------
  function toastError(msg) {
    var box = document.getElementById('app-toasts');
    if (!box) {
      box = document.createElement('div');
      box.id = 'app-toasts';
      box.className = 'toast-container position-fixed top-0 end-0 p-3';
      box.style.zIndex = '1090';
      box.style.maxWidth = 'min(380px, 100vw)';
      box.setAttribute('aria-live', 'polite');
      document.body.appendChild(box);
    }
    var el = document.createElement('div');
    el.className = 'alert alert-danger shadow py-2 mb-2';
    el.setAttribute('role', 'alert');
    el.setAttribute('data-testid', 'timezone-toast-error');
    el.textContent = msg;
    box.appendChild(el);
    setTimeout(function () { el.remove(); }, TOAST_MS);
  }

  var select = document.querySelector('[data-timezone-select]');
  if (!select) return;
  var saved = select.value;

  // Select превращён в комбобокс (combobox.js, D6): видимая кнопка берёт disabled/подпись
  // из скрытого select только в _cbSync — зовём его после каждой программной правки.
  function setState(value, disabled) {
    if (value !== null) select.value = value;
    select.disabled = disabled;
    if (select._cbSync) select._cbSync();
  }

  select.addEventListener('change', function () {
    var next = select.value;
    if (next === saved) return;
    setState(null, true);
    KT.api.put('/api/v1/users/me/timezone', { timezone: next }).then(function () {
      // Все даты страницы уже отрисованы в старом поясе — перерисовать проще и надёжнее reload'ом.
      window.location.reload();
    }).catch(function (err) {
      if (err instanceof KT.ApiError && err.status === 401) { window.location.reload(); return; } // → /login
      setState(saved, false);
      if (err instanceof KT.ApiError && err.status === 400) {
        var required = err.fields.some(function (f) { return (f.name || f.Name) === 'timezone' && (f.message || f.Message) === 'required'; });
        toastError(required ? 'Выберите часовой пояс' : 'Неизвестный часовой пояс');
      } else if (err instanceof KT.ApiError) toastError(err.message);
      else toastError('Нет связи');
    });
  });
})();
