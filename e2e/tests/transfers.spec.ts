import type { Page } from '@playwright/test';
import { test, expect, columnByName } from '../fixtures/board';
import { TransfersPage } from '../pages/TransfersPage';
import { StockPage } from '../pages/StockPage';
import { ShellPage } from '../pages/ShellPage';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { loginContext, type AdminApi, type NewTransfer, type Transfer, type TransferList } from '../fixtures/auth';
import { contrastOf } from '../fixtures/contrast';

// Экран «Перемещения» /transfers — документ «склад → склад» со смешанными строками: товары и
// единицы оборудования клиентов (docs/features/transfers-core/draft-spec.md §8.1; TS-031,
// TS-043, TS-045, TS-046 + UI-части TS-005, TS-006, TS-009). Направления с клиентом и
// поставщиком, фильтры «Направление / Поставщик / Заказ / Период» — transfers-partners.spec.ts;
// здесь из той части — TR-007 (TS-034, права исполнителя). Общая dev-БД (§TD-6.2): свои
// товары, клиенты, оборудование и заказы через uniq()+adminApi, склады — сид «Алматы»/«Москва».
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;

function csrfToken(page: Page): Promise<string> {
  return page.locator('meta[name="csrf-token"]').getAttribute('content').then((v) => v ?? '');
}

async function pairStock(adminApi: AdminApi, productName: string, warehouseId: string): Promise<number> {
  const list = await adminApi.getStock({ q: productName, scope: 'all' });
  const cell = list.items[0]?.by_warehouse.find((c) => c.warehouse_id === warehouseId);
  return Number(cell?.stock ?? NaN);
}

async function newProduct(adminApi: AdminApi, uniq: (s: string) => string, label: string, stock: number) {
  const cat = await adminApi.createCategory({ kind: 'product', name: uniq(`${label} категория`) });
  return adminApi.createProduct({ category_id: cat.id, name: uniq(`${label} товар`), stock }); // остаток — на «Алматы»
}

/** Клиент + заказ + единицы оборудования, принятые на «Алматы» документом «клиент → склад»
 *  (transfers-partners DS-045: само добавление в заказ местоположение не меняет). */
async function unitsAtAlmaty(adminApi: AdminApi, uniq: (s: string) => string, tag: string, count = 1) {
  const almaty = await adminApi.warehouseByName('Алматы');
  const moscow = await adminApi.warehouseByName('Москва');
  const client = await adminApi.createClient({ company_name: uniq(`${tag} клиент`), phones: [{ phone: '9990000401', note: '' }] });
  const type = await adminApi.createEquipmentType({ name: uniq(`${tag} тип`) });
  const columns = (await adminApi.listColumns()).items;
  const task = await adminApi.createTask({ title: uniq(`${tag} заказ`), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
  const units = [];
  for (let i = 0; i < count; i++) {
    const eq = await adminApi.createEquipment(client.id, {
      type_id: type.id, brand: `Brand${tag}`, model: uniq(`${tag} модель ${i}`), serial: uniq(`sn-${tag}-${i}`),
    });
    await adminApi.addTaskEquipmentAt(task.id, eq.id, almaty.id);
    units.push(eq);
  }
  return { almaty, moscow, client, task, units };
}

test.describe('TR: перемещения — документ склад → склад', () => {
  test('TR-001 (TS-031): товар + оборудование клиента — отправка, приёмка с фактом меньше отправленного; в карточке заказа единица «на складе Москва»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    // негатив сценария: факт больше отправленного → POST …/receive → 400.
    allowHttpErrors((url, status) => /\/api\/v1\/transfers\/\d+\/receive$/.test(url) && status === 400);
    const { almaty, moscow, client, task, units } = await unitsAtAlmaty(adminApi, uniq, 'TR001');
    const unit = units[0];
    const product = await newProduct(adminApi, uniq, 'TR-001', 10);

    const shell = new ShellPage(page);
    await shell.goto('/');
    await shell.navLink('Перемещения').click();
    await expect(page).toHaveURL(/\/transfers$/);

    const tr = new TransfersPage(page);
    await tr.newDocument('warehouse_to_warehouse');
    await expect(tr.cardTitle).toHaveText('Новое перемещение');
    // «+ Оборудование» недоступно, пока не выбран склад отправления.
    await expect(tr.equipmentSearch).toBeDisabled();
    await expect(tr.equipmentHint).toHaveText('Сначала выберите склад отправления');
    await tr.choose(tr.from, 'Алматы');
    await tr.choose(tr.to, 'Москва');
    await expect(tr.equipmentSearch).toBeEnabled();
    await tr.note.fill('TR-001 принтер на диагностику + расходники');

    await tr.addProductRow(product.name, product.id, 4);
    await tr.addEquipmentRow(unit.serial, unit.id);
    await expect(tr.itemRows).toHaveCount(2);
    await expect(tr.itemRow('equipment', unit.id).getByTestId('st-item-kind')).toHaveText('оборудование');
    await expect(tr.itemRow('equipment', unit.id)).toContainText(unit.model);
    await expect(tr.itemRow('equipment', unit.id)).toContainText(client.company_name);
    await expect(tr.itemRow('equipment', unit.id).getByTestId('st-item-qty')).toHaveCount(0); // количества у единицы нет
    await expect(tr.itemRow('product', product.id).getByTestId('st-item-kind')).toHaveText('товар');

    // «Отправить» нового документа — создать и отправить одним действием.
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    const id = await tr.currentId();
    await expect(page).toHaveURL(new RegExp(`id=${id}`));
    await expect(tr.row(id).getByTestId('st-status-badge')).toHaveText('В пути');
    await expect(tr.row(id).getByTestId('st-row-count')).toHaveText('2');
    await expect(tr.viewItem('product', product.id).getByTestId('st-view-sent')).toHaveText('4');
    await expect(tr.save).toHaveCount(0);
    await expect(tr.cancel).toHaveText('Отменить отправку');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(6);

    const sent = await adminApi.getTransfer(id);
    expect(sent.status).toBe('in_transit');
    const productLine = sent.items.find((i) => i.kind === 'product')!;
    expect(sent.items.find((i) => i.kind === 'equipment')?.equipment_id).toBe(unit.id);
    let card = await adminApi.getCard(task.id);
    expect(card.equipment[0]).toMatchObject({ location_status: 'in_transit', active_transfer_id: id, transit_to_name: 'Москва', warehouse_id: null });

    // «Принять»: факт — только по товару; единица — «принимается целиком».
    await tr.receive.click();
    await expect(tr.receiveModal).toBeVisible();
    await expect(tr.receiveRows).toHaveCount(2);
    await expect(tr.receiveRows.filter({ hasText: unit.model }).getByTestId('st-receive-whole')).toHaveText('принимается целиком');
    await expect(tr.receiveRows.filter({ hasText: unit.model }).getByTestId('st-receive-qty')).toHaveCount(0);
    await expect(tr.receiveQty(productLine.id)).toHaveValue('4');

    // негатив: факт больше отправленного.
    await tr.receiveQty(productLine.id).fill('5');
    await tr.receiveConfirm.click();
    await expect(tr.receiveQtyError(productLine.id)).toHaveText('Факт больше отправленного');
    await expect(tr.receiveModal).toBeVisible();
    expect((await adminApi.getTransfer(id)).status).toBe('in_transit');

    await tr.receiveQty(productLine.id).fill('3');
    await tr.receiveConfirm.click();
    await expect(tr.receiveModal).toBeHidden();
    await expect(tr.cardStatus).toHaveText('Получено');
    const fact = tr.viewItem('product', product.id).getByTestId('st-view-received');
    await expect(fact).toHaveText('3');
    await expect(fact).toHaveAttribute('data-loss', 'true'); // расхождение подсвечено
    await expect(fact).toHaveClass(/text-danger/);
    await expect(tr.receive).toHaveCount(0);
    await expect(tr.cancel).toHaveCount(0);
    await expect(tr.cardMeta).toContainText('Получен: ');
    expect(await pairStock(adminApi, product.name, moscow.id)).toBe(3);

    // карточка заказа: единица «на складе Москва», документа за ней больше нет.
    card = await adminApi.getCard(task.id);
    expect(card.equipment[0]).toMatchObject({ location_status: 'at_warehouse', warehouse_id: moscow.id, active_transfer_id: null, transit_to_name: null });
    const board = new BoardPage(page);
    const orderCard = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await orderCard.waitOpen();
    await expect(orderCard.equipmentLocation(unit.id)).toHaveText('на складе Москва');
    await expect(orderCard.equipmentTransfer(unit.id)).toHaveCount(0);
  });

  test('TR-002: товарный документ — заявка → правка строк → отправка («В пути» на остатках, предупреждение) → приёмка', async ({
    adminPage: page,
    adminApi,
    uniq,
    transfersCleanup,
  }) => {
    const almaty = await adminApi.warehouseByName('Алматы');
    const moscow = await adminApi.warehouseByName('Москва');
    const p1 = await newProduct(adminApi, uniq, 'TR-002 a', 10);
    const p2 = await newProduct(adminApi, uniq, 'TR-002 b', 1);

    const tr = new TransfersPage(page);
    await tr.goto();
    await tr.newDocument('warehouse_to_warehouse');
    await tr.choose(tr.from, 'Алматы');
    await tr.choose(tr.to, 'Москва');
    await tr.addProductRow(p1.name, p1.id, 4);
    await tr.addProductRow(p2.name, p2.id, 2);
    await tr.save.click();
    await expect(tr.toast).toHaveText('Сохранено');
    await expect(tr.cardStatus).toHaveText('Заявка');
    const id = await tr.currentId();
    transfersCleanup(id); // документ создан через экран — без уборки держит товары теста
    await expect(page).toHaveURL(new RegExp(`id=${id}`));
    await expect(tr.row(id)).toBeVisible();
    expect(await pairStock(adminApi, p1.name, almaty.id)).toBe(10); // заявка не двигает остатки

    // правка строки заявки.
    await tr.itemQty(p1.id).fill('6');
    await tr.save.click();
    await expect.poll(async () => Number((await adminApi.getTransfer(id)).items[0].qty_sent)).toBe(6);

    // отправка: минус на источнике; p2 уходит в минус — жёлтый блок предупреждений.
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.warnings).toContainText(p2.name);
    await expect(tr.warnings).toContainText('отрицательный');
    expect(await pairStock(adminApi, p1.name, almaty.id)).toBe(4);
    expect(await pairStock(adminApi, p2.name, almaty.id)).toBe(-1);

    // «В пути» на экране «Остатки»: все склады и фильтр склада назначения.
    const stock = new StockPage(page);
    await stock.goto();
    await stock.chooseScope('Все товары');
    await stock.q.fill(p1.name);
    await expect(stock.row(p1.id).getByTestId('stock-cell-transit')).toHaveText('6');
    await stock.chooseWarehouse('Москва');
    await expect(stock.row(p1.id).getByTestId('stock-cell-transit')).toHaveText('6');
    await stock.chooseWarehouse('Алматы');
    await expect(stock.row(p1.id).getByTestId('stock-cell-transit')).toHaveText('0');

    // приёмка: факт по умолчанию = отправлено.
    await tr.goto(id);
    await tr.receive.click();
    await expect(tr.receiveModal).toBeVisible();
    await tr.receiveConfirm.click();
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.viewItem('product', p1.id).getByTestId('st-view-received')).toHaveText('6');
    await expect(tr.viewItem('product', p1.id).getByTestId('st-view-received')).not.toHaveAttribute('data-loss', 'true');
    expect(await pairStock(adminApi, p1.name, moscow.id)).toBe(6);
    expect(Number((await adminApi.getStock({ q: p1.name, scope: 'all' })).items[0].in_transit)).toBe(0);
  });

  test('TR-003 (TS-009): отмена «в пути» — возврат на склад отправления; отмена заявки; склады совпадают → ошибка под полем', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.endsWith('/api/v1/transfers') && status === 400);
    const { almaty, moscow, task, units } = await unitsAtAlmaty(adminApi, uniq, 'TR003');
    const unit = units[0];
    const p = await newProduct(adminApi, uniq, 'TR-003', 10);
    const sent = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true,
      items: [{ kind: 'product', product_id: p.id, qty_sent: 3 }, { kind: 'equipment', equipment_id: unit.id }],
    } satisfies NewTransfer);
    const req = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'product', product_id: p.id, qty_sent: 1 }],
    });
    expect(await pairStock(adminApi, p.name, almaty.id)).toBe(7);

    const tr = new TransfersPage(page);
    await tr.goto(sent.id);
    await expect(tr.cardStatus).toHaveText('В пути');

    // «Отмена» в диалоге — ничего не происходит.
    await tr.cancel.click();
    await expect(page.getByTestId('ui-dialog-text')).toHaveText(`Отменить перемещение #${sent.id}?`);
    await page.getByTestId('ui-dialog-cancel').click();
    await expect(page.getByTestId('ui-dialog')).toHaveCount(0);
    await expect(tr.cardStatus).toHaveText('В пути');

    await tr.cancel.click();
    await page.getByTestId('ui-dialog-ok').click();
    await expect(tr.cardStatus).toHaveText('Отменён');
    await expect(tr.cardMeta).toContainText('Отменён: ');
    expect(await pairStock(adminApi, p.name, almaty.id)).toBe(10);
    expect((await adminApi.getTransfer(sent.id)).status).toBe('cancelled');
    // единица вернулась на склад отправления и свободна.
    expect((await adminApi.getCard(task.id)).equipment[0]).toMatchObject({
      location_status: 'at_warehouse', warehouse_id: almaty.id, active_transfer_id: null,
    });

    // отмена заявки — без движений.
    await tr.row(req.id).click();
    await expect(tr.cardTitle).toHaveText(`Перемещение #${req.id}`);
    await expect(tr.cancel).toHaveText('Отменить');
    await tr.cancel.click();
    await page.getByTestId('ui-dialog-ok').click();
    await expect(tr.cardStatus).toHaveText('Отменён');
    expect(await pairStock(adminApi, p.name, almaty.id)).toBe(10);

    // склады совпадают → ошибка под «Складом назначения», документ не создан.
    await tr.newDocument('warehouse_to_warehouse');
    await tr.choose(tr.from, 'Алматы');
    await tr.choose(tr.to, 'Алматы');
    await tr.addProductRow(p.name, p.id, 1);
    await tr.save.click();
    await expect(tr.toError).toHaveText('Склады должны различаться');
    await expect(tr.cardTitle).toHaveText('Новое перемещение');
  });

  test('TR-004 (TS-043): два администратора открыли одну заявку — первый отправил, второй сохраняет → тост «Перемещение уже изменено, обновите», документ перечитан', async ({
    adminPage: page,
    adminApi,
    runAdmin,
    browser,
    baseURL,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => /\/api\/v1\/transfers\/\d+$/.test(url) && status === 409);
    const almaty = await adminApi.warehouseByName('Алматы');
    const moscow = await adminApi.warehouseByName('Москва');
    const p = await newProduct(adminApi, uniq, 'TR-004', 5);
    const req = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, note: 'TR-004 исходное примечание',
      items: [{ kind: 'product', product_id: p.id, qty_sent: 2 }],
    });

    const other = await browser.newContext({ baseURL, locale: 'ru-RU' });
    try {
      await loginContext(other, runAdmin.login, runAdmin.password);
      const first = new TransfersPage(await other.newPage());
      const second = new TransfersPage(page);
      // оба открыли одну и ту же версию заявки.
      await first.goto(req.id);
      await second.goto(req.id);
      await expect(first.cardStatus).toHaveText('Заявка');
      await expect(second.cardStatus).toHaveText('Заявка');

      await first.send.click();
      await expect(first.cardStatus).toHaveText('В пути');

      // второй правит устаревшую заявку и сохраняет.
      await second.note.fill('TR-004 правка второго');
      await second.itemQty(p.id).fill('4');
      await second.save.click();
      await expect(second.toastError).toHaveText('Перемещение уже изменено, обновите');
      await expect(second.cardStatus).toHaveText('В пути'); // документ перечитан
      await expect(second.save).toHaveCount(0);
      await expect(second.row(req.id).getByTestId('st-status-badge')).toHaveText('В пути');

      // негатив: данные первого не затёрты.
      const final: Transfer = await adminApi.getTransfer(req.id);
      expect(final.status).toBe('in_transit');
      expect(final.note).toBe('TR-004 исходное примечание');
      expect(Number(final.items[0].qty_sent)).toBe(2);
      expect(await pairStock(adminApi, p.name, almaty.id)).toBe(3); // списано один раз, 2 шт.
    } finally {
      await other.close();
    }
  });

  test('TR-005 (TS-009): устаревший документ — «Отправить» уже отправленной заявки → тост, показано актуальное состояние', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.includes('/api/v1/transfers/') && status === 409);
    const almaty = await adminApi.warehouseByName('Алматы');
    const moscow = await adminApi.warehouseByName('Москва');
    const p = await newProduct(adminApi, uniq, 'TR-005', 5);
    const req = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'product', product_id: p.id, qty_sent: 2 }],
    });

    const tr = new TransfersPage(page);
    await tr.goto(req.id);
    await expect(tr.cardStatus).toHaveText('Заявка');
    await adminApi.transferAction(req.id, 'send'); // отправили в другом месте

    await tr.send.click();
    await expect(tr.toastError).toHaveText('Перемещение уже изменено, обновите');
    await expect(tr.cardStatus).toHaveText('В пути');
    expect(await pairStock(adminApi, p.name, almaty.id)).toBe(3); // списано один раз
  });

  test('TR-006: фильтры списка — статус, склад, «Сбросить»; счётчик «N из total»', async ({ adminPage: page, adminApi, uniq }) => {
    const almaty = await adminApi.warehouseByName('Алматы');
    const moscow = await adminApi.warehouseByName('Москва');
    const p = await newProduct(adminApi, uniq, 'TR-006', 5);
    const inTransit = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true, items: [{ kind: 'product', product_id: p.id, qty_sent: 1 }],
    });
    const req = await adminApi.createTransfer({
      from_warehouse_id: moscow.id, to_warehouse_id: almaty.id, items: [{ kind: 'product', product_id: p.id, qty_sent: 1 }],
    });

    const tr = new TransfersPage(page);
    await tr.goto();
    await expect(tr.row(inTransit.id)).toBeVisible();
    await expect(tr.row(req.id)).toBeVisible();
    await expect(tr.shown).toHaveText(/^\d+ из \d+$/);
    await expect(tr.placeholder).toHaveText('Выберите перемещение');

    await tr.chooseStatusFilter('В пути');
    await expect(tr.row(req.id)).toHaveCount(0);
    await expect(tr.row(inTransit.id).getByTestId('st-status-badge')).toHaveText('В пути');
    for (const badge of await tr.rows.getByTestId('st-status-badge').all()) {
      await expect(badge).toHaveAttribute('data-status', 'in_transit');
    }

    await tr.choose(tr.filterWarehouse, 'Москва');
    await expect(tr.row(inTransit.id)).toBeVisible(); // склад — отправитель ИЛИ получатель

    await tr.filterReset.click();
    await expect(tr.row(req.id)).toBeVisible();
    await expect(tr.row(inTransit.id)).toBeVisible();
    await expect(tr.filterStatus).toContainText('Все статусы');
    await expect(tr.filterWarehouse).toContainText('Все склады');
  });

  test('TR-007 (TS-034): права исполнителя — пункт меню и экран доступны; в меню «Новое перемещение» два направления, в списке только документы с клиентом, фильтра «Поставщик» нет; прямой URL документа «склад → склад» → «Перемещение не найдено»', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    // чужие направления и документ с клиентом создаёт администратор.
    const { almaty, moscow, client, task, units } = await unitsAtAlmaty(adminApi, uniq, 'TR007');
    const p = await newProduct(adminApi, uniq, 'TR-007', 5);
    const supplier = await adminApi.createSupplier({ name: uniq('TR-007 поставщик') });
    const w2w = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'product', product_id: p.id, qty_sent: 1 }],
    });
    const supply = await adminApi.createTransfer({
      direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id,
      items: [{ kind: 'product', product_id: p.id, qty_sent: 2 }],
    });
    const back = await adminApi.createTransfer({
      direction: 'warehouse_to_client', from_warehouse_id: almaty.id, task_id: task.id,
      items: [{ kind: 'equipment', equipment_id: units[0].id }],
    });

    const login = uniq('exec-tr007');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);

    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.navLink('Остатки')).toBeVisible();
    await shell.navLink('Перемещения').click();
    await expect(page).toHaveURL(/\/transfers$/);

    const tr = new TransfersPage(page);
    await expect(tr.root).toBeVisible();
    // список: только документы с клиентом.
    await expect(tr.row(back.id)).toBeVisible();
    await expect(tr.row(back.id).getByTestId('st-row-direction')).toHaveText('Возврат клиенту');
    await expect(tr.row(back.id).getByTestId('st-row-route')).toHaveText(`#${back.id} · Алматы → Клиент`);
    await expect(tr.row(back.id).getByTestId('st-row-party')).toHaveText(`${client.company_name} · заказ #${task.id}`);
    await expect(tr.row(w2w.id)).toHaveCount(0);
    await expect(tr.row(supply.id)).toHaveCount(0);
    for (const row of await tr.rows.all()) {
      await expect(row).toHaveAttribute('data-direction', /^(client_to_warehouse|warehouse_to_client)$/);
    }

    // меню «Новое перемещение» — два направления; фильтра «Поставщик» нет; «Направление» — два значения.
    await tr.newBtn.click();
    await expect(tr.newDirections).toHaveText(['Приём от клиента', 'Возврат клиенту']);
    await tr.newBtn.click(); // закрыть меню
    await expect(tr.newDirections.first()).toBeHidden();
    await expect(tr.filterSupplier).toHaveCount(0);
    await tr.filterDirection.click();
    await expect(tr.dropdown).toBeVisible();
    await expect(tr.dropdown.locator('.cb-opt', { hasText: 'Приём от клиента' })).toHaveCount(1);
    await expect(tr.dropdown.locator('.cb-opt', { hasText: 'Склад → склад' })).toHaveCount(0);
    await expect(tr.dropdown.locator('.cb-opt', { hasText: 'Приход от поставщика' })).toHaveCount(0);
    await tr.dropdown.locator('.cb-opt', { hasText: 'Возврат клиенту' }).first().click(); // выбор закрывает список
    await expect(tr.row(back.id)).toBeVisible();
    await tr.filterReset.click();

    // исполнитель открывает документ с клиентом — доступен.
    await tr.row(back.id).click();
    await expect(tr.cardTitle).toHaveText(`Перемещение #${back.id}`);
    await expect(tr.direction).toHaveText('Возврат клиенту');
    await expect(tr.taskLink).toHaveText(`Заказ #${task.id}`);

    // негатив: прямой URL документа чужого направления → тост «Перемещение не найдено».
    allowHttpErrors((url, status) => /\/api\/v1\/transfers\/\d+$/.test(url) && status === 404);
    for (const foreign of [w2w, supply]) {
      await page.goto(`/transfers?id=${foreign.id}`);
      await expect(tr.toastError).toHaveText('Перемещение не найдено');
      await expect(tr.placeholder).toBeVisible();
      await expect(tr.cardTitle).toHaveCount(0);
      await expect(page).toHaveURL(/\/transfers$/); // id из адреса убран
    }

    // API под сессией исполнителя: чужое направление — 403 на создание, 404 на чтение; свой
    // список чужих направлений не содержит даже с фильтром.
    allowHttpErrors((url, status) => url.endsWith('/api/v1/transfers') && status === 403);
    const csrf = await csrfToken(page);
    const forbidden: NewTransfer = {
      direction: 'warehouse_to_warehouse', from_warehouse_id: almaty.id, to_warehouse_id: moscow.id,
      items: [{ kind: 'product', product_id: p.id, qty_sent: 1 }],
    };
    const post = await page.request.post('/api/v1/transfers', { data: forbidden, headers: { 'X-Csrf-Token': csrf }, failOnStatusCode: false });
    expect(post.status()).toBe(403);
    expect((await page.request.get(`/api/v1/transfers/${supply.id}`, { failOnStatusCode: false })).status()).toBe(404);
    const filtered = await page.request.get('/api/v1/transfers?direction=supplier_to_warehouse&limit=500');
    expect(filtered.status()).toBe(200);
    const list: TransferList = await filtered.json();
    expect(list.items).toEqual([]);
    expect(list.total).toBe(0);

    // DS-030: старые адреса убраны без редиректов.
    allowHttpErrors((url, status) => url.includes('/stock-transfers') && status === 404);
    expect((await page.request.get('/api/v1/stock-transfers', { failOnStatusCode: false })).status()).toBe(404);
    expect((await page.goto('/stock-transfers'))?.status()).toBe(404);
    // поиск единиц (§6.8) исполнителю доступен — чтение оборудования клиентов.
    expect((await page.request.get('/api/v1/equipment?limit=1', { failOnStatusCode: false })).status()).toBe(200);
  });

  test('TR-008 (TS-005): единица, занятая другой заявкой, в поиске «+ Оборудование» неактивна с пометкой «в перемещении #N»', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, moscow, units } = await unitsAtAlmaty(adminApi, uniq, 'TR008', 2);
    const [busy, free] = units;
    const holder = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'equipment', equipment_id: busy.id }],
    });

    const tr = new TransfersPage(page);
    await tr.goto();
    await tr.newDocument('warehouse_to_warehouse');
    await tr.choose(tr.from, 'Алматы');
    await tr.choose(tr.to, 'Москва');

    await tr.equipmentSearch.fill('BrandTR008');
    await expect(tr.equipmentOption(busy.id)).toBeVisible();
    await expect(tr.equipmentOption(busy.id)).toBeDisabled();
    await expect(tr.equipmentOption(busy.id).getByTestId('st-equipment-busy')).toHaveText(`в перемещении #${holder.id}`);
    await expect(tr.equipmentOption(free.id)).toBeEnabled();

    await tr.equipmentOption(free.id).click();
    await expect(tr.itemRow('equipment', free.id)).toBeVisible();
    await expect(tr.itemRow('equipment', busy.id)).toHaveCount(0);
    await tr.save.click();
    await expect(tr.cardStatus).toHaveText('Заявка');
    const id = await tr.currentId();
    const search = await adminApi.searchEquipment({ q: 'BrandTR008', warehouse_id: almaty.id, location_status: 'at_warehouse' });
    expect(search.items.find((e) => e.id === free.id)?.active_transfer_id).toBe(id);
    expect(search.items.find((e) => e.id === busy.id)?.active_transfer_id).toBe(holder.id);

    // в своей заявке единица — не «занята»: повторно в поиске она помечена «уже в документе».
    await tr.equipmentSearch.fill(free.serial);
    await expect(tr.equipmentOption(free.id)).toBeDisabled();
    await expect(tr.equipmentOption(free.id)).toContainText('уже в документе');
  });

  test('TR-009 (TS-006): отправка единицы не со склада отправления → красный текст в строке, документ остаётся заявкой', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => /\/api\/v1\/transfers\/\d+\/send$/.test(url) && status === 409);
    const { almaty, moscow, task, units } = await unitsAtAlmaty(adminApi, uniq, 'TR009');
    const unit = units[0];
    const req = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'equipment', equipment_id: unit.id }],
    });

    const tr = new TransfersPage(page);
    await tr.goto(req.id);
    // маршрут развернули: единица числится на «Алматы», а склад отправления теперь «Москва».
    await tr.choose(tr.to, 'Алматы');
    await tr.choose(tr.from, 'Москва');
    await tr.send.click();
    await expect(tr.itemError('equipment', unit.id)).toHaveText('Оборудование находится не на складе отправления');
    await expect(tr.cardStatus).toHaveText('Заявка');

    const after = await adminApi.getTransfer(req.id);
    expect(after.status).toBe('requested');
    expect(after.from_warehouse_id).toBe(moscow.id); // правка сохранена, отправка — нет
    expect((await adminApi.getCard(task.id)).equipment[0]).toMatchObject({
      location_status: 'at_warehouse', warehouse_id: almaty.id, active_transfer_id: req.id,
    });
  });

  test('TR-010 (TS-045): вьюпорт 375×812 — документ со строками обоих видов без горизонтальной прокрутки (заявка, в пути, приёмка)', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, moscow, units } = await unitsAtAlmaty(adminApi, uniq, 'TR010');
    const p = await adminApi.createProduct({
      category_id: (await adminApi.createCategory({ kind: 'product', name: uniq('TR-010 категория') })).id,
      name: uniq(`TR-010 ${'оченьдлинноеназваниетоварабезпробелов'.repeat(2)}`),
      stock: 1,
    });
    const doc = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, note: 'примечание '.repeat(30).trim(),
      items: [{ kind: 'product', product_id: p.id, qty_sent: 3 }, { kind: 'equipment', equipment_id: units[0].id }],
    });

    await page.setViewportSize({ width: 375, height: 812 });
    const tr = new TransfersPage(page);
    await tr.goto();
    await tr.expectNoHorizontalScroll(); // список
    await tr.goto(doc.id);
    await expect(tr.itemRows).toHaveCount(2);
    await tr.expectNoHorizontalScroll(); // заявка (редактор)

    await tr.send.click(); // остаток 1 − 3 → предупреждение
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.warnings).toBeVisible();
    await tr.expectNoHorizontalScroll(); // в пути + жёлтый блок

    await tr.receive.click();
    await expect(tr.receiveModal).toBeVisible();
    await tr.expectNoHorizontalScroll(); // модалка «Приёмка»
    await tr.receiveConfirm.click();
    await expect(tr.cardStatus).toHaveText('Получено');
    await tr.expectNoHorizontalScroll(); // получено (колонка «Факт»)
  });

  test('TR-011 (TS-046): светлая и тёмная тема — бейджи статусов читаемы, жёлтый блок предупреждений контрастен', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, moscow, units } = await unitsAtAlmaty(adminApi, uniq, 'TR011');
    const p = await newProduct(adminApi, uniq, 'TR-011', 1);
    const mk = (send: boolean, extra: NewTransfer['items'] = []) =>
      adminApi.createTransfer({
        from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send,
        items: [{ kind: 'product', product_id: p.id, qty_sent: 2 }, ...extra],
      });
    const requested = await mk(false, [{ kind: 'equipment', equipment_id: units[0].id }]);
    const cancelled = await mk(false);
    await adminApi.transferAction(cancelled.id, 'cancel');
    const received = await mk(true);
    await adminApi.transferAction(received.id, 'receive', { items: [{ id: (await adminApi.getTransfer(received.id)).items[0].id, qty_received: 1 }] });

    const tr = new TransfersPage(page);
    for (const theme of THEMES) {
      // последний зарегистрированный init-скрипт задаёт тему (эталон — polish.spec.ts).
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      const where = `/transfers ${theme}`;

      // заявка со строками обоих видов → отправка из UI: жёлтый блок предупреждений.
      const fresh = await mk(false);
      await tr.goto(fresh.id);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      expect(await contrastOf(tr.cardStatus), `${where}: бейдж «Заявка»`).toBeGreaterThanOrEqual(3);
      await tr.send.click();
      await expect(tr.cardStatus).toHaveText('В пути');
      await expect(tr.warnings).toBeVisible();
      expect(await contrastOf(tr.warnings), `${where}: блок предупреждений`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.cardStatus), `${where}: бейдж «В пути»`).toBeGreaterThanOrEqual(3);

      for (const [doc, label] of [[requested, 'Заявка'], [received, 'Получено'], [cancelled, 'Отменён']] as const) {
        await tr.goto(doc.id);
        await expect(tr.cardStatus).toHaveText(label);
        expect(await contrastOf(tr.cardStatus), `${where}: бейдж «${label}»`).toBeGreaterThanOrEqual(3);
        expect(await contrastOf(tr.row(doc.id).getByTestId('st-status-badge')), `${where}: бейдж «${label}» в списке`).toBeGreaterThanOrEqual(3);
      }
      // строки документа: тип строки и подсветка расхождения.
      await tr.goto(requested.id);
      for (const kind of await tr.itemRows.getByTestId('st-item-kind').all()) {
        expect(await contrastOf(kind), `${where}: бейдж типа строки`).toBeGreaterThanOrEqual(3);
      }
      await tr.goto(received.id);
      const loss = tr.viewItem('product', p.id).getByTestId('st-view-received');
      await expect(loss).toHaveAttribute('data-loss', 'true');
      expect(await contrastOf(loss), `${where}: расхождение факта`).toBeGreaterThanOrEqual(3);
    }
  });
});
