import { test, expect } from '../fixtures/board';
import { CatalogPage } from '../pages/CatalogPage';
import { DeletedPage } from '../pages/DeletedPage';
import { EquipmentPage } from '../pages/EquipmentPage';
import { EquipmentTypesPage } from '../pages/EquipmentTypesPage';
import { StockPage } from '../pages/StockPage';

// Мягкое удаление — каталог (товары, услуги, категории) и типы оборудования
// (docs/features/soft-delete-catalog/draft-spec.md §8.1–§8.3, §11): удаление с причиной рядом с
// архивом, вкладки «Товары», «Услуги», «Категории», «Типы оборудования» раздела «Удалённые»,
// свободное название удалённой категории и отказ восстановления при занятом (TS-044), остаток
// удалённого товара в отчёте «Остатки» (TS-054) и только его списание (TS-063), тип оборудования
// с единицами (TS-061).
// Удаление товара и услуги, стоящих в заказе, и бейджи в карточке заказа — BD-019a / BD-019b
// (board-guards.spec.ts); окно «Удаление» без причины — CT-005, ET-003. Адаптив и темы раздела
// «Удалённые» и окна удаления — SD-007 / SD-008 (soft-delete-core.spec.ts).
// Общая dev-БД (§TD-6.2): свои записи через uniq()+adminApi. Уборка (TS-F10): записи каталога —
// autouse catalogCleanup (служебный адрес purge, стенд с TEST_PURGE_ENABLED=true), заказы —
// _boardCleanup, клиент с единицей — adminApi.purgeClient.
test.use({ colorScheme: 'light' });

const DATE_TIME = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;
const TOAST_DELETED = 'Удалено. Восстановить можно в разделе «Удалённые»';

test.describe('SDCT: мягкое удаление каталога и типов оборудования', () => {
  test('SDCT-001 (TS-044): категория удаляется с причиной → её товар остаётся с бейджем «категория удалена»; новая категория с тем же названием создаётся; «Восстановить» старую → «Название занято — переименуйте действующую запись», строка остаётся; после переименования новой — «Восстановлено»; архивный товар после удаления и восстановления остаётся в архиве (бейдж «в архиве», «Запись вернётся в архив»); негатив: товар с остатком — «У товара есть остаток на складе — сначала спишите его», товар не удалён', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
    catalogCleanup,
  }) => {
    const catName = uniq('SDCT-001 расходники');
    const category = await adminApi.createCategory({ kind: 'product', name: catName });
    const keepCatName = uniq('SDCT-001 запчасти');
    const keepCat = await adminApi.createCategory({ kind: 'product', name: keepCatName });
    const inCatName = uniq('SDCT-001 товар категории');
    const inCat = await adminApi.createProduct({ category_id: category.id, name: inCatName });
    const archivedName = uniq('SDCT-001 архивный товар');
    const archived = await adminApi.createProduct({ category_id: keepCat.id, name: archivedName });
    const stockName = uniq('SDCT-001 товар с остатком');
    const withStock = await adminApi.createProduct({ category_id: keepCat.id, name: stockName, stock: 3 });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/catalog-categories/${category.id}/restore`) && status === 409);
    allowHttpErrors((url, status) => url.endsWith(`/catalog/products/${withStock.id}/delete`) && status === 409);

    // ---- §8.1: удаление категории из панели — окно с пояснением про товары и услуги ----
    const catalog = new CatalogPage(page);
    await catalog.gotoProductsByName(inCatName);
    await expect(catalog.categoryDeletedBadge(inCatName)).toHaveCount(0);
    await catalog.categoryDelete(catName).click();
    await expect(catalog.deleteDialogTitle).toHaveText(`Удалить «${catName}»?`);
    await expect(catalog.deleteDialogText).toHaveText('Товары и услуги категории останутся в каталоге с пометкой «категория удалена»');
    const catReason = uniq('SDCT-001 категория не нужна');
    await catalog.deleteReason.fill(catReason);
    await catalog.deleteDialogOk.click();
    await expect(page).toHaveURL(/\/catalog\/products\?flash=category_deleted$/);
    await expect(catalog.toast).toHaveText(TOAST_DELETED);
    await expect(catalog.categoryRowById(category.id)).toHaveCount(0);

    // товар удалённой категории остался в каталоге — с бейджем; в форме категория «удалена».
    await catalog.gotoProductsByName(inCatName);
    await expect(catalog.row(inCatName)).toBeVisible();
    await expect(catalog.categoryDeletedBadge(inCatName)).toHaveText('категория удалена');
    await catalog.open(inCatName);
    await expect(catalog.formCategoryDeletedBadge).toHaveText('удалена');
    await expect(catalog.category).toContainText(`${catName} (удалена)`);
    expect((await adminApi.findCatalogByName('products', inCatName))?.id).toBe(inCat.id);

    // ---- название удалённой категории свободно: новая с тем же названием создаётся ----
    await catalog.gotoProducts();
    await catalog.addCategory(catName);
    await expect(page).toHaveURL(/flash=category_created/);
    await expect(catalog.flash).toHaveText('Категория создана');
    const fresh = await adminApi.findCatalogByName('catalog-categories', catName);
    expect(fresh, 'новая категория с названием удалённой').toBeTruthy();
    expect(fresh!.id).not.toBe(category.id);
    catalogCleanup('catalog-categories', fresh!.id);

    // ---- §8.2: «Удалённые» → «Категории»; восстановление при занятом названии ----
    const del = new DeletedPage(page);
    await del.goto(`kind=catalog-categories&q=${encodeURIComponent(catName)}`);
    await expect(del.tab('catalog-categories')).toHaveClass(/active/);
    await expect(del.tab('catalog-categories')).toContainText('Категории');
    await expect(del.tab('products')).toContainText('Товары');
    await expect(del.tab('services')).toContainText('Услуги');
    await expect(del.tab('equipment-types')).toContainText('Типы оборудования');
    await expect(del.rows).toHaveCount(1);
    await expect(del.rowTitle(category.id)).toHaveText(catName);
    await expect(del.rowSubtitle(category.id)).toHaveText('товары');
    await expect(del.rowArchived(category.id)).toHaveCount(0);
    await expect(del.rowReason(category.id)).toHaveText(catReason);
    await expect(del.rowBy(category.id)).toHaveText('Админ Прогона');
    await expect(del.rowAt(category.id)).toHaveText(DATE_TIME);
    const summary = await adminApi.deletedSummary();
    await expect(del.tabCount('catalog-categories')).toHaveText(String(summary.counts.catalog_categories));

    await del.restoreBtn(category.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «${catName}»?`);
    await del.dialogOk.click();
    await expect(del.toastError).toHaveText('Название занято — переименуйте действующую запись');
    await expect(del.row(category.id)).toBeVisible();
    expect((await adminApi.listDeleted('catalog-categories', { q: catName })).total, 'категория осталась в удалённых').toBe(1);

    // новая переименована — старая восстанавливается.
    const renamed = uniq('SDCT-001 расходники новые');
    await catalog.gotoProducts();
    await catalog.categoryRenameInput(catName).fill(renamed);
    await catalog.categoryRenameSubmit(catName).click();
    await expect(page).toHaveURL(/flash=category_saved/);
    await del.goto(`kind=catalog-categories&q=${encodeURIComponent(catName)}`);
    await del.restore(category.id);
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.row(category.id)).toHaveCount(0);
    await expect(del.tabCount('catalog-categories')).toHaveText(String(summary.counts.catalog_categories - 1));
    await catalog.gotoProductsByName(inCatName);
    await expect(catalog.categoryRowById(category.id)).toBeVisible();
    await expect(catalog.categoryDeletedBadge(inCatName)).toHaveCount(0);

    // ---- архив и удаление независимы: архивный товар возвращается в архив ----
    await catalog.gotoProductsByName(archivedName);
    await catalog.rowToggle(archivedName).click();
    await expect(page).toHaveURL(/flash=archived/);
    await catalog.gotoProducts(`?archived=1&q=${encodeURIComponent(archivedName)}`);
    await expect(catalog.archivedBadge(archivedName)).toHaveText('в архиве');
    await catalog.open(archivedName);
    const productReason = uniq('SDCT-001 снят с продажи');
    await catalog.confirmDelete(productReason);
    await expect(page).toHaveURL(/\/catalog\/products\?flash=deleted$/);
    await expect(catalog.toast).toHaveText(TOAST_DELETED);
    await catalog.gotoProducts(`?archived=1&q=${encodeURIComponent(archivedName)}`);
    await expect(catalog.row(archivedName)).toHaveCount(0);

    await del.goto(`kind=products&q=${encodeURIComponent(archivedName)}`);
    await expect(del.rows).toHaveCount(1);
    await expect(del.rowTitle(archived.id)).toHaveText(archivedName);
    await expect(del.rowSubtitle(archived.id)).toHaveText(keepCatName);
    await expect(del.rowArchived(archived.id)).toHaveText('в архиве');
    await expect(del.rowReason(archived.id)).toHaveText(productReason);
    await del.restoreBtn(archived.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «${archivedName}»? Запись вернётся в архив`);
    await del.dialogOk.click();
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.row(archived.id)).toHaveCount(0);
    await expect(del.empty).toBeVisible();
    // восстановлен архивным: в обычном списке его нет, в списке с архивом — с бейджем.
    await catalog.gotoProductsByName(archivedName);
    await expect(catalog.row(archivedName)).toHaveCount(0);
    await catalog.gotoProducts(`?archived=1&q=${encodeURIComponent(archivedName)}`);
    await expect(catalog.archivedBadge(archivedName)).toHaveText('в архиве');

    // ---- негатив: товар с остатком не удаляется — красный блок в форме товара ----
    await catalog.gotoProductsByName(stockName);
    await catalog.open(stockName);
    await catalog.confirmDelete(uniq('SDCT-001 не нужен'));
    await expect(catalog.formError).toHaveText('У товара есть остаток на складе — сначала спишите его');
    await expect(catalog.form).toBeVisible();
    await catalog.gotoProductsByName(stockName);
    await expect(catalog.row(stockName)).toBeVisible();
    expect((await adminApi.listDeleted('products', { q: stockName })).total, 'товар с остатком не удалён').toBe(0);
  });

  test('SDCT-002 (TS-054): товар списан заказом до нуля и удалён, затем заказ уходит из списывающей колонки → в «Остатках» строка товара с бейджем «удалён» и возвращённым остатком; корректировка остатка до нуля — строка исчезает из отчёта; негатив: пока остаток нулевой, удалённого товара в отчёте нет', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('SDCT-002 категория товаров') });
    const productName = uniq('SDCT-002 товар');
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: productName, stock: 2 });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('SDCT-002 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('SDCT-002 услуга') });
    const client = await adminApi.createClient({ company_name: uniq('SDCT-002 клиент'), phones: [{ phone: '9990005402', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const consuming = columns.find((c) => c.consumes_stock && !c.is_final)!;
    const plain = columns.find((c) => !c.consumes_stock && !c.is_final)!;
    const task = await adminApi.createTask({ title: uniq('SDCT-002 заказ'), client_id: client.id, column_id: consuming.id });

    try {
      // заказ в списывающей колонке забрал весь остаток — товар удаляется (остаток нулевой).
      await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 2 }] });
      await adminApi.softDelete('products', product.id, uniq('SDCT-002 снят с продажи'));

      // негатив: удалённого товара с нулевым остатком в отчёте нет.
      expect((await adminApi.getStock({ scope: 'all', q: productName })).items, 'нулевой остаток — строки нет').toHaveLength(0);
      const stock = new StockPage(page);
      await stock.goto();
      await stock.q.fill(productName);
      await expect(stock.empty).toBeVisible();
      await expect(stock.row(product.id)).toHaveCount(0);

      // заказ уходит из списывающей колонки — остаток возвращается удалённому товару.
      const moved = await adminApi.moveTask(task.id, { column_id: plain.id });
      expect(moved.status, 'перенос заказа').toBe(200);
      const rows = (await adminApi.getStock({ q: productName })).items;
      expect(rows.map((r) => [r.product_id, r.is_deleted, Number(r.stock)])).toEqual([[product.id, true, 2]]);

      await stock.goto();
      await stock.q.fill(productName);
      await expect(stock.row(product.id)).toBeVisible();
      await expect(stock.rowDeleted(product.id)).toHaveText('удалён');
      await expect(stock.row(product.id).getByTestId('stock-cell-total')).toContainText('2');

      // корректировка остатка работает как обычно: списали до нуля — строка исчезла.
      await stock.openEdit(product.id);
      await stock.submitValue(0);
      await expect(stock.form).toBeHidden();
      await expect(stock.row(product.id)).toHaveCount(0);
      await expect(stock.empty).toBeVisible();
      expect((await adminApi.getStock({ scope: 'all', q: productName })).items).toHaveLength(0);
    } finally {
      await adminApi.purgeClient(client.id, [task.id]);
    }
  });

  test('SDCT-003 (TS-061): тип оборудования с единицей клиента удаляется с причиной → список без типа, тост; в форме единицы и в карточке клиента у типа бейдж «удалён», в выборе типа новой единицы его нет; в «Удалённых» вкладка «Типы оборудования» — строка типа; «Восстановить» → «Восстановлено», бейдж исчезает; негатив: единица при удалении типа не удалена', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const typeName = uniq('SDCT-003 тип');
    const type = await adminApi.createEquipmentType({ name: typeName });
    const client = await adminApi.createClient({ company_name: uniq('SDCT-003 клиент'), phones: [{ phone: '9990005403', note: '' }] });
    const unit = await adminApi.createEquipment(client.id, { type_id: type.id, brand: 'Raycus', model: uniq('SDCT-003 единица') });

    try {
      // §8.1: форма типа → окно «Удаление»; единица клиента удалению не мешает (DS-014).
      const types = new EquipmentTypesPage(page);
      await types.goto(`?q=${encodeURIComponent(typeName)}`);
      await expect(types.rowCount(typeName)).toHaveText('1'); // счётчик единиц типа — действующие
      await types.open(typeName);
      const reason = uniq('SDCT-003 тип заведён по ошибке');
      await types.confirmDelete(reason);
      await expect(page).toHaveURL(/\/catalog\/equipment-types\?flash=deleted$/);
      await expect(types.toast).toHaveText(TOAST_DELETED);
      await types.goto(`?archived=1&q=${encodeURIComponent(typeName)}`);
      await expect(types.row(typeName)).toHaveCount(0);

      // §8.3: единица осталась действующей — у типа бейдж «удалён» в форме и в карточке клиента.
      expect((await adminApi.getClient(client.id)).equipment_count, 'единица не удалена').toBe(1);
      const eq = new EquipmentPage(page);
      await eq.gotoEdit(unit.id);
      await expect(page.getByTestId('type-deleted-badge')).toHaveText('удалён');
      await expect(eq.typeButton).toContainText(`${typeName} (удалён)`);
      await page.goto(`/clients/${client.id}`);
      await expect(page.getByTestId('equipment-table').getByTestId('type-deleted-badge')).toHaveText('удалён');
      // в выборе типа новой единицы удалённого типа нет.
      await eq.gotoNew(client.id);
      await eq.typeButton.click();
      await expect(page.getByTestId('combobox-dropdown').getByText(typeName)).toHaveCount(0);
      await page.keyboard.press('Escape');

      // §8.2: «Удалённые» → «Типы оборудования».
      const del = new DeletedPage(page);
      await del.goto(`kind=equipment-types&q=${encodeURIComponent(typeName)}`);
      await expect(del.tab('equipment-types')).toHaveClass(/active/);
      await expect(del.rows).toHaveCount(1);
      await expect(del.rowTitle(type.id)).toHaveText(typeName);
      await expect(del.rowSubtitle(type.id)).toHaveCount(0);
      await expect(del.rowArchived(type.id)).toHaveCount(0);
      await expect(del.rowReason(type.id)).toHaveText(reason);
      await expect(del.rowAt(type.id)).toHaveText(DATE_TIME);

      await del.restoreBtn(type.id).click();
      await expect(del.dialogText).toHaveText(`Восстановить «${typeName}»?`);
      await del.dialogOk.click();
      await expect(del.toast).toHaveText('Восстановлено');
      await expect(del.row(type.id)).toHaveCount(0);
      await expect(del.empty).toBeVisible();

      // тип снова в справочнике со своей единицей; бейдж у единицы исчез.
      await types.goto(`?q=${encodeURIComponent(typeName)}`);
      await expect(types.rowCount(typeName)).toHaveText('1');
      await eq.gotoEdit(unit.id);
      await expect(eq.form).toBeVisible();
      await expect(page.getByTestId('type-deleted-badge')).toHaveCount(0);
    } finally {
      await adminApi.purgeClient(client.id);
    }
  });

  test('SDCT-004 (TS-063): остаток удалённого товара — только списание: увеличение → «Товар удалён — его остаток можно только списать», окно остаётся, остаток прежний; списание части проходит; списание до нуля — строка исчезает', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('SDCT-004 категория товаров') });
    const productName = uniq('SDCT-004 товар');
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: productName, stock: 4 });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('SDCT-004 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('SDCT-004 услуга') });
    const client = await adminApi.createClient({ company_name: uniq('SDCT-004 клиент'), phones: [{ phone: '9990006304', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const consuming = columns.find((c) => c.consumes_stock && !c.is_final)!;
    const plain = columns.find((c) => !c.consumes_stock && !c.is_final)!;
    const task = await adminApi.createTask({ title: uniq('SDCT-004 заказ'), client_id: client.id, column_id: consuming.id });
    const stockOf = async () => Number((await adminApi.getStock({ scope: 'all', q: productName })).items[0].stock);
    // увеличение остатка удалённого товара — 409 под полем окна (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/stock/${product.id}`) && status === 409);

    try {
      // заказ забрал весь остаток → товар удалён → заказ ушёл из списывающей колонки: остаток
      // вернулся удалённому товару.
      await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 4 }] });
      await adminApi.softDelete('products', product.id, uniq('SDCT-004 снят с продажи'));
      expect((await adminApi.moveTask(task.id, { column_id: plain.id })).status, 'перенос заказа').toBe(200);
      expect(await stockOf()).toBe(4);

      const stock = new StockPage(page);
      await stock.goto();
      await stock.q.fill(productName);
      await expect(stock.rowDeleted(product.id)).toHaveText('удалён');

      // окно корректировки: текущий остаток в поле.
      await stock.openEdit(product.id);
      await expect(stock.value).toHaveValue('4');

      // увеличение — отказ под полем, окно остаётся, остаток прежний.
      await stock.submitValue(6);
      await expect(stock.error).toHaveText('Товар удалён — его остаток можно только списать');
      await expect(stock.form).toBeVisible();
      await expect(stock.save).toBeEnabled();
      expect(await stockOf(), 'остаток не изменился').toBe(4);

      // списание части проходит.
      await stock.submitValue(1);
      await expect(stock.form).toBeHidden();
      await expect(stock.row(product.id).getByTestId('stock-cell-total')).toContainText('1');
      await expect(stock.rowDeleted(product.id)).toHaveText('удалён');
      expect(await stockOf()).toBe(1);

      // списание до нуля — строка исчезает из отчёта.
      await stock.openEdit(product.id);
      await expect(stock.error).toBeHidden();
      await stock.submitValue(0);
      await expect(stock.form).toBeHidden();
      await expect(stock.row(product.id)).toHaveCount(0);
      expect((await adminApi.getStock({ scope: 'all', q: productName })).items).toHaveLength(0);
    } finally {
      await adminApi.purgeClient(client.id, [task.id]);
    }
  });
});
