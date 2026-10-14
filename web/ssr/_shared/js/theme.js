/* Тема (system → light → dark по кругу), ключ localStorage kt-theme. Начальное
   значение ставит инлайн-снипет в layouts/base.html до загрузки стилей. Плюс
   мобильный сайдбар: [data-sidebar-toggle] открывает, backdrop закрывает. */
(function () {
  'use strict';
  var KEY = 'kt-theme';
  var ORDER = ['system', 'light', 'dark'];
  var mq = window.matchMedia ? matchMedia('(prefers-color-scheme: dark)') : null;

  function pref() {
    try { return localStorage.getItem(KEY) || 'system'; } catch (e) { return 'system'; }
  }
  function apply(p) {
    var dark = p === 'dark' || (p === 'system' && !!mq && mq.matches);
    document.documentElement.setAttribute('data-bs-theme', dark ? 'dark' : 'light');
    document.querySelectorAll('[data-theme-toggle]').forEach(function (b) {
      b.title = 'Тема: ' + ({ system: 'как в системе', light: 'светлая', dark: 'тёмная' })[p];
      b.dataset.themePref = p;
    });
  }
  function cycle() {
    var next = ORDER[(ORDER.indexOf(pref()) + 1) % ORDER.length];
    try { localStorage.setItem(KEY, next); } catch (e) { /* приватный режим — тема до перезагрузки */ }
    apply(next);
  }

  if (mq && mq.addEventListener) mq.addEventListener('change', function () { if (pref() === 'system') apply('system'); });

  document.addEventListener('click', function (e) {
    if (e.target.closest('[data-theme-toggle]')) { cycle(); return; }
    var shell = document.querySelector('.app-shell');
    if (!shell) return;
    if (e.target.closest('[data-sidebar-toggle]')) shell.classList.toggle('sidebar-open');
    else if (e.target.closest('[data-sidebar-close]')) shell.classList.remove('sidebar-open');
  });

  apply(pref());
})();
