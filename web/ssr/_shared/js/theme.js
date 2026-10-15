/* Тема (system → light → dark по кругу), ключ localStorage kt-theme. Начальное
   значение ставит инлайн-снипет в layouts/base.html до загрузки стилей. Плюс
   сайдбар: [data-sidebar-toggle] (☰ в шапке, логотип) на широком экране сворачивает/
   разворачивает меню (data-sidebar="collapsed" на <html>, ключ kt-sidebar — тоже
   ставится снипетом до стилей), на узком (≤900px, мобильный drawer) открывает/закрывает
   его; backdrop закрывает drawer. */
(function () {
  'use strict';
  var KEY = 'kt-theme';
  var ORDER = ['system', 'light', 'dark'];
  var SIDEBAR_KEY = 'kt-sidebar';
  var root = document.documentElement;
  var mq = window.matchMedia ? matchMedia('(prefers-color-scheme: dark)') : null;
  // тот же брейкпоинт, что у мобильного сайдбара в app.css
  var narrow = window.matchMedia ? matchMedia('(max-width: 900px)') : null;

  function pref() {
    try { return localStorage.getItem(KEY) || 'system'; } catch (e) { return 'system'; }
  }
  function apply(p) {
    var dark = p === 'dark' || (p === 'system' && !!mq && mq.matches);
    root.setAttribute('data-bs-theme', dark ? 'dark' : 'light');
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

  function isNarrow() { return !!narrow && narrow.matches; }
  function collapsed() { return root.getAttribute('data-sidebar') === 'collapsed'; }
  function setCollapsed(c) {
    if (c) root.setAttribute('data-sidebar', 'collapsed'); else root.removeAttribute('data-sidebar');
    try {
      if (c) localStorage.setItem(SIDEBAR_KEY, 'collapsed'); else localStorage.removeItem(SIDEBAR_KEY);
    } catch (e) { /* приватный режим — состояние до перезагрузки */ }
  }
  // aria-expanded/подсказка переключателей — по тому режиму, что сейчас на экране
  function syncSidebar() {
    var shell = document.querySelector('.app-shell');
    if (!shell) return;
    var open = isNarrow() ? shell.classList.contains('sidebar-open') : !collapsed();
    document.querySelectorAll('[data-sidebar-toggle]').forEach(function (b) {
      b.setAttribute('aria-expanded', open ? 'true' : 'false');
      b.title = open ? 'Скрыть меню' : 'Показать меню';
    });
  }

  if (mq && mq.addEventListener) mq.addEventListener('change', function () { if (pref() === 'system') apply('system'); });
  if (narrow && narrow.addEventListener) narrow.addEventListener('change', syncSidebar);

  document.addEventListener('click', function (e) {
    if (e.target.closest('[data-theme-toggle]')) { cycle(); return; }
    var shell = document.querySelector('.app-shell');
    if (!shell) return;
    if (e.target.closest('[data-sidebar-toggle]')) {
      if (isNarrow()) shell.classList.toggle('sidebar-open'); else setCollapsed(!collapsed());
    } else if (e.target.closest('[data-sidebar-close]')) shell.classList.remove('sidebar-open');
    else return;
    syncSidebar();
  });

  apply(pref());
  syncSidebar();
})();
