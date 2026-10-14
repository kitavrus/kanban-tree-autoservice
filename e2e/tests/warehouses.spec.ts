import { test, expect, columnByName } from '../fixtures/board';
import { StockPage } from '../pages/StockPage';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { ShellPage } from '../pages/ShellPage';
import { CatalogPage } from '../pages/CatalogPage';
import { DeletedPage } from '../pages/DeletedPage';
import { WarehousesPage } from '../pages/WarehousesPage';
import type { AdminApi } from '../fixtures/auth';

// Склады — подфаза A (docs/features/warehouses/draft-spec.md DS-001…DS-005, TS-002, TS-003,
// TS-005, TS-007, TS-014). Общая dev-БД (§TD-6.2): свои товары/клиенты через uniq()+adminApi;
// созданные тестом склады убираются служебным адресом purge — autouse partnersCleanup
// (soft-delete-warehouses-suppliers TS-F11; раньше они только архивировались и копились).
test.use({ colorScheme: 'light' });

function csrfToken(page: import('@playwright/test').Page): Promise<string> {
  return page.locator('meta[name="csrf-token"]').getAttribute('content').then((v) => v ?? '');
}

async function pairStock(adminApi: AdminApi, productName: string, warehouseId: string): Promise<number> {
  const list = await adminApi.getStock({ q: productName, scope: 'all' });
  const cell = list.items[0]?.by_warehouse.find((c) => c.warehouse_id === warehouseId);
  return Number(cell?.stock ?? NaN);
}

test.describe('WH: справочник складов', () => {
  test('WH-001 (TS-002, TS-045): admin — пункт «Склады», добавить, дубль имени, переименовать, в архив, удалить с причиной, восстановить из «Удалённых»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
    partnersCleanup,
  }) => {
    // дубль имени — форма перерисовывается с 422 (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith('/warehouses') && status === 422);
    const shell = new ShellPage(page);
    await shell.goto('/');
    await shell.navLink('Склады').click();
    await expect(page).toHaveURL(/\/warehouses$/);
    await expect(page.getByTestId('warehouses-table')).toBeVisible();
    await expect(page.locator('[data-testid="warehouse-row"][data-name="Алматы"]')).toBeVisible();
    await expect(page.locator('[data-testid="warehouse-row"][data-name="Москва"]')).toBeVisible();

    const name = uniq('WH-001 склад');
    await page.getByTestId('warehouse-new-name').fill(name);
    await page.getByTestId('warehouse-add').click();
    await expect(page.getByTestId('flash')).toHaveText('Склад добавлен');
    const row = page.locator(`[data-testid="warehouse-row"][data-name="${name}"]`);
    await expect(row).toBeVisible();

    // дубль: другой регистр + пробелы → ошибка под полем.
    await page.getByTestId('warehouse-new-name').fill(`  ${name.toUpperCase()}  `);
    await page.getByTestId('warehouse-add').click();
    await expect(page.getByTestId('error-name')).toHaveText('Такой склад уже есть');

    // переименование.
    await page.goto('/warehouses');
    await page.locator(`[data-testid="warehouse-row"][data-name="${name}"]`).getByTestId('warehouse-edit-link').click();
    const renamed = `${name} (2)`;
    await page.getByTestId('warehouse-name').fill(renamed);
    await page.getByTestId('warehouse-save').click();
    await expect(page.getByTestId('flash')).toHaveText('Изменения сохранены');
    const renamedRow = page.locator(`[data-testid="warehouse-row"][data-name="${renamed}"]`);
    await expect(renamedRow).toBeVisible();

    // в архив: пропадает из списка, виден с «Показать архивные» с бейджем.
    await renamedRow.getByTestId('warehouse-toggle-active').click();
    await expect(page.getByTestId('flash')).toHaveText('Склад отправлен в архив');
    await expect(page.locator(`[data-testid="warehouse-row"][data-name="${renamed}"]`)).toHaveCount(0);
    await page.goto('/warehouses?archived=1');
    await expect(page.locator(`[data-testid="warehouse-row"][data-name="${renamed}"]`).getByTestId('warehouse-archived-badge')).toBeVisible();

    // дубль архивного имени — тоже ошибка (уникальность вкл. архивные).
    await page.getByTestId('warehouse-new-name').fill(renamed);
    await page.getByTestId('warehouse-add').click();
    await expect(page.getByTestId('error-name')).toContainText('в архиве');

    const w = await adminApi.warehouseByName(renamed);
    expect(w.is_active).toBe(false);
    partnersCleanup('warehouses', w.id); // заведён через экран; ниже — мягко удаляется

    // «Удалить» с причиной (soft-delete-warehouses-suppliers §8.1): кнопка — рядом с «Вернуть»,
    // окно «Удаление», тост, склада нет в списке — и среди архивных.
    const wh = new WarehousesPage(page);
    await wh.goto(true);
    await expect(wh.toggleActive(renamed)).toHaveText('Вернуть');
    const reason = uniq('WH-001 создан по ошибке');
    await wh.delete(renamed, reason);
    await expect(page).toHaveURL(/\/warehouses\?archived=1&flash=deleted$/);
    await expect(wh.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
    await expect(wh.row(renamed)).toHaveCount(0);
    expect((await adminApi.listWarehouses(true)).items.map((x) => x.id), 'удалённого склада список не отдаёт').not.toContain(w.id);

    // «Удалённые» → «Склады» → «Восстановить»: склад вернулся в архив (признак архива прежний).
    const del = new DeletedPage(page);
    await del.goto(`kind=warehouses&q=${encodeURIComponent(renamed)}`);
    await expect(del.rowTitle(w.id)).toHaveText(renamed);
    await expect(del.rowArchived(w.id)).toHaveText('в архиве');
    await expect(del.rowReason(w.id)).toHaveText(reason);
    await del.restore(w.id);
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.row(w.id)).toHaveCount(0);
    expect(await adminApi.warehouseByName(renamed)).toMatchObject({ id: w.id, is_active: false });
  });

  test('WH-002 (TS-014): executor не видит «Склады» — нет пункта меню, страница и API 403', async ({
    page,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    const login = uniq('exec-wh002');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);

    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.navLink('Остатки')).toBeVisible();
    await expect(shell.navLink('Склады')).toHaveCount(0);

    allowHttpErrors((url, status) => (url.endsWith('/warehouses') || url.includes('/api/v1/warehouses')) && status === 403);
    const resp = await page.goto('/warehouses');
    expect(resp?.status()).toBe(403);

    const api = await page.request.get('/api/v1/warehouses', { failOnStatusCode: false });
    expect(api.status()).toBe(403);
    const post = await page.request.post('/api/v1/warehouses', {
      data: { name: 'чужой склад' },
      headers: { 'X-Csrf-Token': await csrfToken(page) },
      failOnStatusCode: false,
    });
    expect(post.status()).toBe(403);
  });
});

test.describe('WH: остатки по складам', () => {
  test('WH-003 (TS-003, TS-007): «Остатки» — колонки складов, «Итого», «В пути», фильтр склада, правка на «Москве»', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const almaty = await adminApi.warehouseByName('Алматы');
    const moscow = await adminApi.warehouseByName('Москва');
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('WH-003 категория') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('WH-003 товар'), stock: 10, min_stock: 2 });
    await adminApi.adjustStock(product.id, 1, moscow.id); // Москва: 1 при минимуме 0 → норма

    const stock = new StockPage(page);
    await stock.goto();
    await stock.chooseScope('Все товары');
    await stock.q.fill(product.name);
    const row = stock.row(product.id);
    await expect(row).toBeVisible();
    await expect(page.locator('[data-testid="stock-col-wh"]', { hasText: 'Алматы' })).toBeVisible();
    await expect(page.locator('[data-testid="stock-col-wh"]', { hasText: 'Москва' })).toBeVisible();
    await expect(stock.cell(product.id, 'Алматы')).toHaveText('10');
    await expect(stock.cell(product.id, 'Москва')).toHaveText('1');
    await expect(row.getByTestId('stock-cell-total')).toContainText('11');
    await expect(row.getByTestId('stock-cell-transit')).toHaveText('0');

    // правка остатка на «Москве» через модалку — «Алматы» не меняется.
    await stock.openEdit(product.id);
    await stock.modalWarehouse.click();
    await stock.dropdown.locator('.cb-opt', { hasText: 'Москва' }).first().click();
    await expect(stock.value).toHaveValue('1');
    await stock.submitValue(7);
    await expect(stock.form).toBeHidden();
    await expect(stock.cell(product.id, 'Москва')).toHaveText('7');
    await expect(stock.cell(product.id, 'Алматы')).toHaveText('10');
    await expect(row.getByTestId('stock-cell-total')).toContainText('17');
    await expect.poll(() => pairStock(adminApi, product.name, moscow.id)).toBe(7);
    await expect.poll(() => pairStock(adminApi, product.name, almaty.id)).toBe(10);

    // фильтр «Москва»: «Итого» — только выбранный склад.
    await stock.chooseWarehouse('Москва');
    await expect(row.getByTestId('stock-cell-total')).toContainText('7');
    await expect(row.getByTestId('stock-cell-total')).not.toContainText('17');

    // ниже минимума — по паре: Алматы 10 → 1 при минимуме 2 → рисковый на «Алматы», не на «Москве».
    await adminApi.adjustStock(product.id, 1, almaty.id);
    const risky = await adminApi.getStock({ scope: 'risky', q: product.name, warehouse_id: almaty.id });
    expect(risky.items.map((i) => i.product_id)).toContain(product.id);
    const riskyMsk = await adminApi.getStock({ scope: 'risky', q: product.name, warehouse_id: moscow.id });
    expect(riskyMsk.items.map((i) => i.product_id)).not.toContain(product.id);
  });

  test('WH-004 (DS-003): карточка товара — блок «Склады», остаток на «Москве» из формы', async ({ adminPage: page, adminApi, uniq }) => {
    const moscow = await adminApi.warehouseByName('Москва');
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('WH-004 категория') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('WH-004 товар'), stock: 3, min_stock: 1, place: 'Полка A' });

    const catalog = new CatalogPage(page);
    await page.goto(`/catalog/products/${product.id}`);
    await expect(page.getByTestId('product-stocks')).toBeVisible();
    await expect(catalog.stock).toHaveValue('3.000');
    await expect(catalog.place).toHaveValue('Полка A');
    const msk = catalog.stockRow('Москва');
    await msk.getByTestId('product-stock').fill('4');
    await msk.getByTestId('product-place').fill('Москва, стеллаж 2');
    await page.getByTestId('product-save').click();
    await expect(page).toHaveURL(/\/catalog\/products\?flash=saved/);

    await expect.poll(() => pairStock(adminApi, product.name, moscow.id)).toBe(4);
    const row = page.locator(`[data-testid="product-row"][data-name="${product.name}"]`);
    await page.goto(`/catalog/products?q=${encodeURIComponent(product.name)}`);
    await expect(row.getByTestId('product-stock')).toContainText('7');
    await expect(row.locator('[data-testid="product-stock-wh"][data-warehouse="Москва"]')).toContainText('4');
  });

  test('WH-005 (TS-005): склад строки товара в карточке заказа — смена у списанной строки переносит списание', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const almaty = await adminApi.warehouseByName('Алматы');
    const moscow = await adminApi.warehouseByName('Москва');
    const client = await adminApi.createClient({ company_name: uniq('WH-005 клиент'), phones: [{ phone: '9990000051', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('WH-005 категория товаров') });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('WH-005 категория услуг') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('WH-005 товар'), stock: 10, min_stock: 0 });
    await adminApi.adjustStock(product.id, 10, moscow.id);
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('WH-005 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('WH-005 заказ'), client_id: client.id, column_id: columnByName(columns, 'В работе').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 3 }] });

    // строка без склада — склад по умолчанию (первый активный — «Алматы»), списано с него.
    await expect.poll(() => pairStock(adminApi, product.name, almaty.id)).toBe(7);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    const row = card.subtaskRow(subtaskId).getByTestId('card-product-row');
    await expect(row).toHaveCount(1);
    const productRowId = (await row.getAttribute('data-product-id'))!;
    await expect(card.productWarehouse(productRowId)).toContainText('Алматы');

    await card.chooseCombobox(card.productWarehouse(productRowId), 'Москва');
    await expect(card.productWarehouse(productRowId)).toContainText('Москва');
    await expect(card.productHint(productRowId)).toContainText('в наличии 10');

    await expect.poll(() => pairStock(adminApi, product.name, almaty.id)).toBe(10); // возврат на старый склад
    await expect.poll(() => pairStock(adminApi, product.name, moscow.id)).toBe(7); // списание с нового
    const cardData = await adminApi.getCard(task.id);
    expect(cardData.subtasks[0].products[0].warehouse_id).toBe(moscow.id);
  });
});
