/* KT.rowList — динамические списки строк 1..N (телефоны клиента, характеристики
   оборудования — ADR-002). Разметка:
     <div id="phones-rows" data-row-list data-row-min="1">
       <div>...строка: input[name=...] + [data-row-del]...</div>
       ...
       <template data-row-template><div>...пустая строка...</div></template>
     </div>
     <button data-row-add="phones-rows">+ Строка</button>
   data-row-min="N" — ниже этого числа строк последняя строка НЕ удаляется, а очищается
   (её значения сбрасываются в пустые) — «последняя строка телефонов очищается, не
   удаляется» (ADR-002). KT.rowList.fill(el, rows) — программно заполнить список
   массивом объектов {имя_поля: значение}; имя поля = атрибут name инпута строки. */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  function template(list) {
    return Array.prototype.find.call(list.children, function (el) {
      return el.tagName === 'TEMPLATE';
    }) || null;
  }

  function rowsOf(list) {
    var tpl = template(list);
    return Array.prototype.filter.call(list.children, function (el) { return el !== tpl; });
  }

  function clearRow(row) {
    row.querySelectorAll('input, textarea, select').forEach(function (el) { el.value = ''; });
  }

  function bindRow(list, row) {
    var del = row.querySelector('[data-row-del]');
    if (!del || del.dataset.rowBound) return;
    del.dataset.rowBound = '1';
    del.addEventListener('click', function (e) {
      e.preventDefault();
      var min = parseInt(list.getAttribute('data-row-min') || '0', 10);
      if (rowsOf(list).length <= min) { clearRow(row); return; }
      row.remove();
    });
  }

  function newRow(list) {
    var tpl = template(list);
    if (!tpl || !tpl.content) return null;
    var frag = tpl.content.cloneNode(true);
    var row = frag.firstElementChild;
    list.insertBefore(frag, tpl);
    if (row) bindRow(list, row);
    return row;
  }

  function addRow(list) {
    var row = newRow(list);
    if (!row) return row;
    var focusEl = row.querySelector('input, textarea, select');
    if (focusEl) focusEl.focus();
    return row;
  }

  /** fill — очищает список и создаёт по одной строке на элемент rows (объект
      {имя_поля: значение}); пустой rows — одна пустая строка (нечего удалять до 0). */
  function fill(list, rows) {
    rowsOf(list).forEach(function (r) { r.remove(); });
    (rows && rows.length ? rows : [{}]).forEach(function (data) {
      var row = newRow(list);
      if (!row) return;
      Object.keys(data || {}).forEach(function (k) {
        var f = row.querySelector('[name="' + k + '"]');
        if (f) f.value = data[k];
      });
    });
  }

  document.querySelectorAll('[data-row-list]').forEach(function (list) {
    rowsOf(list).forEach(function (row) { bindRow(list, row); });
  });
  document.querySelectorAll('[data-row-add]').forEach(function (btn) {
    btn.addEventListener('click', function (e) {
      e.preventDefault();
      var list = document.getElementById(btn.getAttribute('data-row-add'));
      if (list) addRow(list);
    });
  });

  KT.rowList = { fill: fill, addRow: addRow };
})();
