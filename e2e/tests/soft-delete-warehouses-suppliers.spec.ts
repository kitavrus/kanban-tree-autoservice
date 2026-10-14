import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { CatalogPage } from '../pages/CatalogPage';
import { DeletedPage } from '../pages/DeletedPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { StockPage } from '../pages/StockPage';
import { SuppliersPage } from '../pages/SuppliersPage';
import { TransfersPage } from '../pages/TransfersPage';
import { WarehousesPage } from '../pages/WarehousesPage';

// Мягкое удаление — склады и поставщики
// (docs/features/soft-delete-warehouses-suppliers/draft-spec.md §8.1–§8.3, §11): кнопка «Удалить»
// рядом с «В архив», удаление с причиной, вкладки «Склады» и «Поставщики» раздела «Удалённые»,
// восстановление (TS-045); бейджи «удалён» у поставщика в документах перемещения; остаток на
// удалённом складе в отчёте «Остатки» и в карточке заказа (TS-055, TS-065); строка товара заказа
// на удалённом складе (TS-066); архивный склад и архивный товар в «Остатках» (TS-067). Адаптив и темы
// раздела «Удалённые» и окна удаления — SD-007 / SD-008 (soft-delete-core.spec.ts).
// Общая dev-БД (§TD-6.2): свои записи через uniq()+adminApi. Уборка (TS-F11): склады и поставщики
// — autouse partnersCleanup (служебный адрес purge, стенд с TEST_PURGE_ENABLED=true), товары —
// catalogCleanup, заказы — _boardCleanup.
test.use({ colorScheme: 'light' });

const DATE_TIME = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;
const TOAST_DELETED = 'Удалено. Восстановить можно в разделе «Удалённые»';
const WAREHOUSE_DELETE_TEXT =
  'Удалить можно только пустой склад: без товара, оборудования и незавершённых перемещений. Чтобы только скрыть склад из выбора, используйте «В архив»';
const SUPPLIER_DELETE_TEXT = 'Документы поставщика останутся с пометкой «удалён». Поставщика можно восстановить в разделе «Удалённые»';

test.describe('SDWS: мягкое удаление складов и поставщиков', () => {
  test('SDWS-001 (TS-045): склад — «Удалить» рядом с «В архив»; без причины — «Укажите причину удаления»; пустой склад удаляется → тост, строки нет; «Удалённые» → «Склады»: причина, кто, когда; название удалённого свободно, «Восстановить» при занятом названии → «Название занято — переименуйте действующую запись»; после переименования — «Восстановлено»; архивный склад возвращается в архив; негатив: склад с товаром — «На складе есть товар — сначала переместите или спишите остаток»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const name = uniq('SDWS-001 склад');
    const empty = await adminApi.createWarehouse(name);
    const busyName = uniq('SDWS-001 склад с товаром');
    const busy = await adminApi.createWarehouse(busyName);
    const archivedName = uniq('SDWS-001 архивный склад');
    const archived = await adminApi.createWarehouse(archivedName);
    await adminApi.updateWarehouse(archived.id, { name: archivedName, is_active: false });
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('SDWS-001 категория') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('SDWS-001 товар'), stock: 0 });
    await adminApi.adjustStock(product.id, 2, busy.id);
    // отказ по причине — серверная форма перерисовывает список с 422 (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith(`/warehouses/${empty.id}/delete`) && status === 422);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/warehouses/${empty.id}/restore`) && status === 409);

    // ---- §8.1: «Удалить» в строке склада рядом с «В архив» ----
    const wh = new WarehousesPage(page);
    await wh.goto();
    await expect(wh.toggleActive(name)).toHaveText('В архив');
    await expect(wh.deleteBtn(name)).toHaveText('Удалить');
    await wh.openDelete(name);
    await expect(wh.deleteDialogTitle).toHaveText(`Удалить «${name}»?`);
    await expect(wh.deleteDialogText).toHaveText(WAREHOUSE_DELETE_TEXT);

    // без причины: окно показано снова, под полем — текст; склад не удалён.
    await wh.deleteDialogOk.click();
    await expect(wh.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(wh.deleteDialogTitle).toHaveText(`Удалить «${name}»?`);
    expect((await adminApi.warehouseByName(name)).id, 'склад на месте').toBe(empty.id);

    const reason = uniq('SDWS-001 создан по ошибке');
    await wh.deleteReason.fill(reason);
    await wh.deleteDialogOk.click();
    await expect(page).toHaveURL(/\/warehouses\?flash=deleted$/);
    await expect(wh.toast).toHaveText(TOAST_DELETED);
    await expect(wh.row(name)).toHaveCount(0);
    await wh.goto(true);
    await expect(wh.row(name), 'удалённого склада нет и среди архивных').toHaveCount(0);

    // ---- негатив: склад с товаром не удаляется — красный блок над списком ----
    await wh.goto();
    await wh.delete(busyName, uniq('SDWS-001 закрыт'));
    await expect(page).toHaveURL(/\/warehouses\?error=delete_stock$/);
    await expect(wh.listError).toHaveText('На складе есть товар — сначала переместите или спишите остаток');
    await expect(wh.row(busyName)).toBeVisible();
    expect((await adminApi.listDeleted('warehouses', { q: busyName })).total, 'склад с товаром не удалён').toBe(0);

    // ---- название удалённого склада свободно: новый с тем же названием создаётся ----
    await wh.newName.fill(name);
    await wh.add.click();
    await expect(wh.flash).toHaveText('Склад добавлен');
    const twin = await adminApi.warehouseByName(name);
    expect(twin.id, 'новый склад с названием удалённого').not.toBe(empty.id);

    // ---- §8.2: «Удалённые» → «Склады» ----
    const del = new DeletedPage(page);
    await del.goto(`kind=warehouses&q=${encodeURIComponent(name)}`);
    await expect(del.tab('warehouses')).toHaveClass(/active/);
    await expect(del.tab('warehouses')).toContainText('Склады');
    await expect(del.tab('suppliers')).toContainText('Поставщики');
    await expect(del.rows).toHaveCount(1);
    await expect(del.rowTitle(empty.id)).toHaveText(name);
    await expect(del.rowArchived(empty.id)).toHaveCount(0);
    await expect(del.rowReason(empty.id)).toHaveText(reason);
    await expect(del.rowBy(empty.id)).toHaveText('Админ Прогона');
    await expect(del.rowAt(empty.id)).toHaveText(DATE_TIME);
    const summary = await adminApi.deletedSummary();
    await expect(del.tabCount('warehouses')).toHaveText(String(summary.counts.warehouses));

    // восстановление при занятом названии — тост, строка остаётся.
    await del.restoreBtn(empty.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «${name}»?`);
    await del.dialogOk.click();
    await expect(del.toastError).toHaveText('Название занято — переименуйте действующую запись');
    await expect(del.row(empty.id)).toBeVisible();

    // действующий склад переименован — удалённый восстанавливается.
    await adminApi.updateWarehouse(twin.id, { name: uniq('SDWS-001 склад новый'), is_active: true });
    await del.restore(empty.id);
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.row(empty.id)).toHaveCount(0);
    await expect(del.tabCount('warehouses')).toHaveText(String(summary.counts.warehouses - 1));
    await wh.goto();
    await expect(wh.row(name)).toBeVisible();
    await expect(wh.toggleActive(name)).toHaveText('В архив');

    // ---- архив и удаление независимы: архивный склад возвращается в архив ----
    await wh.goto(true);
    await expect(wh.archivedBadge(archivedName)).toHaveText('в архиве');
    await wh.delete(archivedName, uniq('SDWS-001 закрыт'));
    await expect(page).toHaveURL(/\/warehouses\?archived=1&flash=deleted$/);
    await expect(wh.toast).toHaveText(TOAST_DELETED);
    await expect(wh.row(archivedName)).toHaveCount(0);
    await del.goto(`kind=warehouses&q=${encodeURIComponent(archivedName)}`);
    await expect(del.rowArchived(archived.id)).toHaveText('в архиве');
    await del.restoreBtn(archived.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «${archivedName}»? Запись вернётся в архив`);
    await del.dialogOk.click();
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.empty).toBeVisible();
    expect((await adminApi.warehouseByName(archivedName)).is_active, 'склад вернулся в архив').toBe(false);

    // уборка: остаток на складе с товаром списан (иначе склад не убрать), склад-двойник — purge.
    await adminApi.adjustStock(product.id, 0, busy.id);
    expect(await adminApi.purge('warehouses', twin.id), 'уборка склада-двойника').toBe(200);
  });

  test('SDWS-002 (TS-045): поставщик с документом — «Удалить» в форме; без причины — «Укажите причину удаления»; с причиной → тост, поставщика нет в списке; в документе и в списке перемещений — бейдж «удалён»; «Удалённые» → «Поставщики»: контактное лицо, причина, кто, когда; «Восстановить» → «Восстановлено», бейдж в документе исчез', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const name = uniq('SDWS-002 поставщик');
    const supplier = await adminApi.createSupplier({ name, contact_name: 'Иван Петров' });
    const almaty = await adminApi.warehouseByName('Алматы');
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('SDWS-002 категория') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('SDWS-002 товар'), stock: 0 });
    const doc = await adminApi.createTransfer({
      direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id, send: true,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 2 }],
    });
    expect(doc.supplier_is_deleted, 'до удаления пометки нет').toBe(false);
    // пустая причина — 400 по полю, окно остаётся (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/suppliers/${supplier.id}`) && status === 400);

    try {
      // ---- §8.1: «Удалить» внизу формы поставщика ----
      const sup = new SuppliersPage(page);
      await sup.goto();
      await sup.searchFor(name);
      await sup.openRow(name);
      await expect(sup.delete).toHaveText('Удалить');
      await sup.openDelete();
      await expect(sup.deleteDialogTitle).toHaveText(`Удалить «${name}»?`);
      await expect(sup.deleteDialogText).toHaveText(SUPPLIER_DELETE_TEXT);
      await sup.deleteDialogOk.click();
      await expect(sup.deleteReasonError).toHaveText('Укажите причину удаления');
      await expect(sup.deleteDialog).toBeVisible();

      const reason = uniq('SDWS-002 не работаем');
      await sup.deleteReason.fill(reason);
      await sup.deleteDialogOk.click();
      await expect(sup.toast.last()).toHaveText(TOAST_DELETED);
      await expect(sup.form).toBeHidden();
      await expect(sup.row(name)).toHaveCount(0);
      await sup.showArchived(true);
      await expect(sup.row(name), 'удалённого поставщика нет и среди архивных').toHaveCount(0);

      // ---- §8.3: документ удалённого поставщика — бейдж «удалён» в списке и в документе ----
      const tr = new TransfersPage(page);
      await tr.goto(doc.id);
      await expect(tr.viewSupplier).toContainText(name);
      await expect(tr.viewSupplierDeleted).toHaveText('удалён');
      await expect(tr.rowSupplierDeleted(doc.id)).toHaveText('удалён');
      expect((await adminApi.getTransfer(doc.id)).supplier_is_deleted).toBe(true);

      // ---- §8.2: «Удалённые» → «Поставщики» ----
      const del = new DeletedPage(page);
      await del.goto(`kind=suppliers&q=${encodeURIComponent(name)}`);
      await expect(del.tab('suppliers')).toHaveClass(/active/);
      await expect(del.rows).toHaveCount(1);
      await expect(del.rowTitle(supplier.id)).toHaveText(name);
      await expect(del.rowSubtitle(supplier.id)).toHaveText('Иван Петров');
      await expect(del.rowReason(supplier.id)).toHaveText(reason);
      await expect(del.rowBy(supplier.id)).toHaveText('Админ Прогона');
      await expect(del.rowAt(supplier.id)).toHaveText(DATE_TIME);
      const summary = await adminApi.deletedSummary();
      await expect(del.tabCount('suppliers')).toHaveText(String(summary.counts.suppliers));

      await del.restoreBtn(supplier.id).click();
      await expect(del.dialogText).toHaveText(`Восстановить «${name}»?`);
      await del.dialogOk.click();
      await expect(del.toast).toHaveText('Восстановлено');
      await expect(del.row(supplier.id)).toHaveCount(0);
      await expect(del.tabCount('suppliers')).toHaveText(String(summary.counts.suppliers - 1));

      // поставщик снова в списке, бейдж в документе исчез.
      await sup.goto();
      await sup.searchFor(name);
      await expect(sup.row(name)).toBeVisible();
      await tr.goto(doc.id);
      await expect(tr.viewSupplier).toContainText(name);
      await expect(tr.viewSupplierDeleted).toHaveCount(0);
      await expect(tr.rowSupplierDeleted(doc.id)).toHaveCount(0);
    } finally {
      // уборка: документ без заказа держит поставщика от служебной уборки — отменить и убрать.
      await adminApi.transferAction(doc.id, 'cancel');
      expect(await adminApi.purgeTransfer(doc.id), 'уборка документа').toBe(200);
    }
  });

  test('SDWS-003 (TS-055, TS-065): остаток на удалённом складе — в «Остатках» колонка склада с бейджем «удалён», в карточке заказа у склада строки бейдж «удалён»; корректировка с увеличением → «Склад удалён — остаток на нём можно только списать»; списание до нуля → колонка склада исчезает', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const whName = uniq('SDWS-003 склад');
    const wh = await adminApi.createWarehouse(whName);
    const client = await adminApi.createClient({ company_name: uniq('SDWS-003 клиент'), phones: [{ phone: '9990000063', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('SDWS-003 категория товаров') });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('SDWS-003 категория услуг') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('SDWS-003 товар'), stock: 10, min_stock: 0 });
    await adminApi.adjustStock(product.id, 3, wh.id);
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('SDWS-003 услуга'), price: null, products: [] });
    // заказ в списывающей колонке забирает со склада весь товар — склад пуст и удаляется.
    const task = await adminApi.createTask({ title: uniq('SDWS-003 заказ'), client_id: client.id, column_id: columnByName(columns, 'В работе').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.createSubtaskProduct(subtaskId, { product_id: product.id, qty: 3, warehouse_id: wh.id });
    await adminApi.softDelete('warehouses', wh.id, uniq('SDWS-003 склад закрыт'));
    expect((await adminApi.getStock({ scope: 'all', q: product.name })).warehouses.map((w) => w.id), 'пустого удалённого склада в отчёте нет').not.toContain(wh.id);
    // заказ уходит из списывающей колонки — товар возвращается на удалённый склад.
    expect((await adminApi.moveTask(task.id, { column_id: columnByName(columns, 'Новые').id })).status).toBe(200);
    // увеличение остатка на удалённом складе — 409 под полем окна (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/stock/${product.id}`) && status === 409);

    // ---- «Остатки»: колонка удалённого склада с бейджем ----
    const stock = new StockPage(page);
    await stock.goto();
    await stock.q.fill(product.name);
    await expect(stock.row(product.id), 'товар с остатком на удалённом складе виден и в режиме «рисковые»').toBeVisible();
    await expect(stock.colDeleted(whName)).toHaveText('удалён');
    await expect(stock.cell(product.id, whName)).toHaveText('3');
    await expect(stock.cell(product.id, 'Алматы')).toHaveText('10');
    await expect(stock.row(product.id).getByTestId('stock-cell-total')).toContainText('13');
    await expect(stock.rowDeleted(product.id), 'сам товар действует').toHaveCount(0);

    // ---- карточка заказа: бейдж «удалён» у склада строки ----
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.productWarehouseDeleted(subtaskId)).toHaveText('удалён');
    expect((await adminApi.getCard(task.id)).warehouses.find((w) => w.id === wh.id)?.is_deleted).toBe(true);

    // ---- корректировка на удалённом складе — только списание ----
    await stock.goto();
    await stock.q.fill(product.name);
    await stock.openEdit(product.id);
    await stock.modalWarehouse.click();
    await stock.dropdown.locator('.cb-opt', { hasText: whName }).first().click();
    await expect(stock.value).toHaveValue('3');
    await stock.submitValue(5);
    await expect(stock.error).toHaveText('Склад удалён — остаток на нём можно только списать');
    await expect(stock.form).toBeVisible();
    await stock.submitValue(0);
    await expect(stock.form).toBeHidden();
    await expect(stock.col(whName), 'после списания до нуля колонка удалённого склада исчезла').toHaveCount(0);
    await expect.poll(async () => (await adminApi.getStock({ scope: 'all', q: product.name })).warehouses.map((w) => w.id)).not.toContain(wh.id);
  });

  test('SDWS-004 (TS-066): карточка заказа, склад строки товара удалён — увеличение количества → у самой строки «Склад строки удалён — выберите другой склад», количество откатывается; уменьшение проходит; смена склада на действующий проходит — бейдж «удалён» исчезает, после неё количество увеличивается', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const whName = uniq('SDWS-004 склад');
    const wh = await adminApi.createWarehouse(whName);
    const almaty = await adminApi.warehouseByName('Алматы');
    const client = await adminApi.createClient({ company_name: uniq('SDWS-004 клиент'), phones: [{ phone: '9990000064', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('SDWS-004 категория товаров') });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('SDWS-004 категория услуг') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('SDWS-004 товар'), stock: 10, min_stock: 0 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('SDWS-004 услуга'), price: null, products: [] });
    // заказ в несписывающей колонке: строка ссылается на склад, но товар с него не списан —
    // склад пуст и удаляется, остатки тест не двигает.
    const task = await adminApi.createTask({ title: uniq('SDWS-004 заказ'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id });
    const added = await adminApi.createSubtaskProduct(subtaskId, { product_id: product.id, qty: 3, warehouse_id: wh.id });
    const rowId = added.card.subtasks.find((s) => s.id === subtaskId)!.products.find((p) => p.product_id === product.id)!.id;
    await adminApi.softDelete('warehouses', wh.id, uniq('SDWS-004 склад закрыт'));
    const rowOf = async () => (await adminApi.getCard(task.id)).subtasks.find((s) => s.id === subtaskId)!.products.find((p) => p.id === rowId)!;
    // увеличение количества на удалённом складе — 409 у строки (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/subtask-products/${rowId}`) && status === 409);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.productWarehouseDeleted(subtaskId)).toHaveText('удалён');
    await expect(card.productQty(rowId)).toHaveValue('3');

    // ---- увеличение: отказ у самой строки, значение откатывается к серверному ----
    await card.productQty(rowId).fill('5');
    await card.productQty(rowId).blur();
    await expect(card.productQtyError(rowId)).toHaveText('Склад строки удалён — выберите другой склад');
    await expect(card.productQty(rowId)).toHaveValue('3');
    await expect(card.modal, 'карточка остаётся открытой, без общего окна ошибки').toBeVisible();
    await expect(page.getByTestId('ui-dialog')).toHaveCount(0);
    expect(Number((await rowOf()).qty), 'количество на сервере не изменилось').toBe(3);

    // ---- уменьшение проходит ----
    await card.productQty(rowId).fill('2');
    await card.productQty(rowId).blur();
    await expect.poll(async () => Number((await rowOf()).qty), 'уменьшение сохранено').toBe(2);
    await expect(card.productQtyError(rowId)).toBeHidden();
    await expect(card.productQty(rowId)).toHaveValue('2');
    expect((await rowOf()).warehouse_id, 'склад строки прежний').toBe(wh.id);

    // ---- смена склада на действующий проходит: бейдж исчезает, количество снова растёт ----
    await card.chooseCombobox(card.productWarehouse(rowId), 'Алматы');
    await expect.poll(async () => (await rowOf()).warehouse_id, 'склад строки сменён').toBe(almaty.id);
    await expect(card.productWarehouse(rowId)).toContainText('Алматы');
    await expect(card.productWarehouseDeleted(subtaskId)).toHaveCount(0);
    await card.productQty(rowId).fill('5');
    await card.productQty(rowId).blur();
    await expect.poll(async () => Number((await rowOf()).qty), 'на действующем складе количество увеличивается').toBe(5);
    await expect(card.productQtyError(rowId)).toBeHidden();
  });

  test('SDWS-005 (TS-065, TS-067): «Остатки» — колонка архивного склада с бейджем «в архиве», удалённого — «удалён»; архивный товар с остатком на удалённом складе показан с бейджем «в архиве»; окно корректировки действующего товара архивный склад не предлагает, архивного товара — только удалённый склад с его остатком; увеличение → «Склад удалён — остаток на нём можно только списать»; списание до нуля → строка исчезает', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const delName = uniq('SDWS-005 удалённый склад');
    const whDel = await adminApi.createWarehouse(delName);
    const archName = uniq('SDWS-005 архивный склад');
    const whArch = await adminApi.createWarehouse(archName);
    const client = await adminApi.createClient({ company_name: uniq('SDWS-005 клиент'), phones: [{ phone: '9990000065', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('SDWS-005 категория товаров') });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('SDWS-005 категория услуг') });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('SDWS-005 услуга'), price: null, products: [] });
    // действующий товар с остатком на складе, который уходит в архив.
    const live = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('SDWS-005 товар действующий'), stock: 4, min_stock: 0 });
    await adminApi.adjustStock(live.id, 2, whArch.id);
    await adminApi.updateWarehouse(whArch.id, { name: archName, is_active: false });
    // товар, который уйдёт в архив: его остаток окажется на удалённом складе (заказ в списывающей
    // колонке забирает всё со склада → склад удалён пустым → заказ уходит из колонки).
    const archivedName = uniq('SDWS-005 товар архивный');
    const archived = await adminApi.createProduct({ category_id: prodCat.id, name: archivedName, stock: 0, min_stock: 0 });
    await adminApi.adjustStock(archived.id, 3, whDel.id);
    const task = await adminApi.createTask({ title: uniq('SDWS-005 заказ'), client_id: client.id, column_id: columnByName(columns, 'В работе').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.createSubtaskProduct(subtaskId, { product_id: archived.id, qty: 3, warehouse_id: whDel.id });
    await adminApi.softDelete('warehouses', whDel.id, uniq('SDWS-005 склад закрыт'));
    expect((await adminApi.moveTask(task.id, { column_id: columnByName(columns, 'Новые').id })).status).toBe(200);
    // товар — в архив боевым действием справочника.
    const catalog = new CatalogPage(page);
    await catalog.gotoProductsByName(archivedName);
    await catalog.rowToggle(archivedName).click();
    await expect(page).toHaveURL(/flash=archived/);
    const row = (await adminApi.getStock({ scope: 'all', q: archivedName })).items.find((r) => r.product_id === archived.id);
    expect(row, 'архивный товар с остатком на удалённом складе есть в отчёте').toMatchObject({ is_archived: true, is_deleted: false });
    // увеличение остатка на удалённом складе — 409 под полем окна (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/stock/${archived.id}`) && status === 409);

    try {
      // ---- колонка архивного склада с пометкой; окно действующего товара его не предлагает ----
      const stock = new StockPage(page);
      await stock.goto();
      await stock.chooseScope('Все товары');
      await stock.q.fill(live.name);
      await expect(stock.row(live.id)).toBeVisible();
      await expect(stock.colArchived(archName)).toHaveText('в архиве');
      await expect(stock.colDeleted(archName)).toHaveCount(0);
      await expect(stock.cell(live.id, archName)).toHaveText('2');
      await expect(stock.rowArchived(live.id), 'сам товар действует').toHaveCount(0);
      await stock.openEdit(live.id);
      await expect(stock.modalWarehouseOptions.filter({ hasText: 'Алматы' })).toHaveCount(1);
      await expect(stock.modalWarehouseOptions.filter({ hasText: archName }), 'архивный склад в окне не предлагается').toHaveCount(0);
      await expect(stock.modalWarehouseOptions.filter({ hasText: delName }), 'удалённый склад без остатка товара не предлагается').toHaveCount(0);
      await stock.form.getByRole('button', { name: 'Отмена' }).click();
      await expect(stock.form).toBeHidden();

      // ---- архивный товар на удалённом складе: бейдж у товара и у колонки склада ----
      await stock.q.fill(archivedName);
      await expect(stock.row(archived.id)).toBeVisible();
      await expect(stock.rowArchived(archived.id)).toHaveText('в архиве');
      await expect(stock.rowDeleted(archived.id)).toHaveCount(0);
      await expect(stock.colDeleted(delName)).toHaveText('удалён');
      await expect(stock.colArchived(delName)).toHaveCount(0);
      await expect(stock.cell(archived.id, delName)).toHaveText('3');

      // ---- окно корректировки: только удалённый склад с остатком; только списание ----
      await stock.openEdit(archived.id);
      await expect(stock.modalWarehouseOptions).toHaveText([`${delName} (удалён)`]);
      await expect(stock.value).toHaveValue('3');
      await stock.submitValue(5);
      await expect(stock.error).toHaveText('Склад удалён — остаток на нём можно только списать');
      await expect(stock.form).toBeVisible();
      expect(Number((await adminApi.getStock({ scope: 'all', q: archivedName })).items[0].stock), 'остаток не изменился').toBe(3);
      await stock.submitValue(0);
      await expect(stock.form).toBeHidden();
      await expect(stock.row(archived.id), 'после списания архивного товара в отчёте нет').toHaveCount(0);
      await expect(stock.col(delName)).toHaveCount(0);
      await expect.poll(async () => (await adminApi.getStock({ scope: 'all', q: archivedName })).items).toHaveLength(0);
    } finally {
      // уборка: остаток на архивном складе списан (иначе склад не убрать); у действующего товара
      // корректировка на архивном складе закрыта — склад сначала возвращается из архива.
      await adminApi.updateWarehouse(whArch.id, { name: archName, is_active: true });
      await adminApi.adjustStock(live.id, 0, whArch.id);
    }
  });
});
