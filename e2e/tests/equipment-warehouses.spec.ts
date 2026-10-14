import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { TransfersPage } from '../pages/TransfersPage';
import type { AdminApi } from '../fixtures/auth';
import type { CardEquipment } from '../fixtures/board-types';
import { contrastOf } from '../fixtures/contrast';

// Местоположение оборудования заказа — на самой единице, двигается только документом
// «Перемещение» (docs/features/transfers-core/draft-spec.md §8.2, DS-003, DS-008, DS-026,
// DS-046; TS-036, TS-044, TS-045, TS-046). В карточке заказа — бейдж местоположения и
// «перемещение #N» вместо кнопок «Отправить / Принять / Отменить отправку». Общая dev-БД
// (§TD-6.2): свои клиенты/оборудование/заказы через uniq()+adminApi, заказы удаляет
// _boardCleanup.
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;

async function setup(adminApi: AdminApi, uniq: (s: string) => string, tag: string, equipmentCount = 1) {
  const almaty = await adminApi.warehouseByName('Алматы');
  const moscow = await adminApi.warehouseByName('Москва');
  const client = await adminApi.createClient({ company_name: uniq(`${tag} клиент`), phones: [{ phone: '9990000301', note: '' }] });
  const type = await adminApi.createEquipmentType({ name: uniq(`${tag} тип`) });
  const equipment = [];
  for (let i = 0; i < equipmentCount; i++) {
    equipment.push(await adminApi.createEquipment(client.id, { type_id: type.id, model: uniq(`${tag} модель ${i}`), brand: `Brand${tag}`, serial: `SN-${tag}-${i}` }));
  }
  const columns = (await adminApi.listColumns()).items;
  return { almaty, moscow, client, equipment, columns };
}

function cardUnit(card: { equipment: CardEquipment[] }, equipmentId: string): CardEquipment {
  const row = card.equipment.find((e) => e.equipment_id === equipmentId);
  if (!row) throw new Error(`единица ${equipmentId} не найдена в карточке`);
  return row;
}

test.describe('EW: местоположение оборудования заказа', () => {
  test('EW-001 (TS-036): карточка без кнопок перемещения — бейдж местоположения и «перемещение #N»; «Убрать» скрыта у единицы в незавершённом документе', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, moscow, client, equipment, columns } = await setup(adminApi, uniq, 'EW001', 3);
    const [viaUi, inRequest, inTransit] = equipment;
    const task = await adminApi.createTask({ title: uniq('EW-001 заказ'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    await adminApi.addTaskEquipmentAt(task.id, inRequest.id, almaty.id);
    await adminApi.addTaskEquipmentAt(task.id, inTransit.id, almaty.id);
    const request = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'equipment', equipment_id: inRequest.id }],
    });
    const transit = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true, items: [{ kind: 'equipment', equipment_id: inTransit.id }],
    });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    // хвост TS-036 (transfers-partners DS-045): select «Склад приёма» из формы добавления убран —
    // добавленная единица остаётся «у клиента», на склад её приводит документ «клиент → склад».
    await expect(card.equipmentAdd).toBeVisible();
    await expect(card.equipmentWarehouse).toHaveCount(0);
    await expect(card.equipmentWarehouseHint).toHaveCount(0);
    await card.chooseCombobox(card.equipmentAdd, viaUi.model);
    await expect(card.equipmentRows).toHaveCount(3);
    await expect(card.equipmentLocation(viaUi.id)).toHaveText('у клиента');
    await expect(card.equipmentLocation(viaUi.id)).toHaveAttribute('data-location-status', 'at_client');
    await expect(card.equipmentTransfer(viaUi.id)).toHaveCount(0);
    await expect(card.equipmentDelete(viaUi.id)).toBeVisible();

    // заявка: местоположение прежнее, ссылка на документ, «Убрать» скрыта.
    await expect(card.equipmentLocation(inRequest.id)).toHaveText('на складе Алматы');
    await expect(card.equipmentTransfer(inRequest.id)).toHaveText(`перемещение #${request.id}`);
    await expect(card.equipmentTransfer(inRequest.id)).toHaveAttribute('href', `/transfers?id=${request.id}`);
    await expect(card.equipmentDelete(inRequest.id)).toHaveCount(0);

    // в пути: «в пути → Москва», ссылка, «Убрать» скрыта.
    await expect(card.equipmentLocation(inTransit.id)).toHaveText('в пути → Москва');
    await expect(card.equipmentLocation(inTransit.id)).toHaveAttribute('data-location-status', 'in_transit');
    await expect(card.equipmentTransfer(inTransit.id)).toHaveText(`перемещение #${transit.id}`);
    await expect(card.equipmentDelete(inTransit.id)).toHaveCount(0);

    // кнопок «Отправить / Принять / Отменить отправку» и журнала перемещений нет ни у одной единицы.
    for (const eq of equipment) {
      await expect(card.equipmentMoveButtons(eq.id)).toHaveCount(0);
      await expect(card.equipmentMovements(eq.id)).toHaveCount(0);
    }

    const data = await adminApi.getCard(task.id);
    expect(cardUnit(data, viaUi.id)).toMatchObject({ location_status: 'at_client', warehouse_id: null, active_transfer_id: null, transit_to_name: null });
    expect(cardUnit(data, inRequest.id)).toMatchObject({ location_status: 'at_warehouse', warehouse_id: almaty.id, active_transfer_id: request.id, transit_to_name: null });
    expect(cardUnit(data, inTransit.id)).toMatchObject({ location_status: 'in_transit', warehouse_id: null, active_transfer_id: transit.id, transit_to_name: 'Москва' });
    expect(data.task.has_in_transit).toBe(true);

    // ссылка «перемещение #N» открывает документ на экране «Перемещения».
    await card.equipmentTransfer(inTransit.id).click();
    const tr = new TransfersPage(page);
    await expect(page).toHaveURL(new RegExp(`/transfers\\?id=${transit.id}$`));
    await expect(tr.cardTitle).toHaveText(`Перемещение #${transit.id}`);
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.viewItem('equipment', inTransit.id)).toContainText(inTransit.model);
  });

  test('EW-002 (TS-036, TS-018): исполнитель — «перемещение #N» документа «склад → склад» текстом, без ссылки; убрать единицу в документе нельзя (409), после отмены документа — можно', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
  }) => {
    const { almaty, moscow, client, equipment, columns } = await setup(adminApi, uniq, 'EW002');
    const eq = equipment[0];
    const exec = await makeUser({ login: uniq('ew002-exec'), name: uniq('EW-002 Исполнитель') });
    const task = await adminApi.createTask({
      title: uniq('EW-002 заказ'), client_id: client.id, column_id: columnByName(columns, 'Новые').id, assignee_id: exec.id,
    });
    await adminApi.addTaskEquipmentAt(task.id, eq.id, almaty.id);
    const doc = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true, items: [{ kind: 'equipment', equipment_id: eq.id }],
    });

    await loginAs(page, exec.login, exec.password);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await expect(card.equipmentLocation(eq.id)).toHaveText('в пути → Москва');
    await expect(card.equipmentTransfer(eq.id)).toHaveText(`перемещение #${doc.id}`);
    // transfers-partners DS-018: документ «склад → склад» исполнителю недоступен — «перемещение
    // #N» у него текстом, не ссылкой (ссылка на документ с клиентом — TR-012).
    await expect(card.equipmentTransfer(eq.id)).not.toHaveAttribute('href', /.*/);
    expect((await adminApi.getCard(task.id)).equipment[0].active_transfer_direction).toBe('warehouse_to_warehouse');
    await expect(card.equipmentDelete(eq.id)).toHaveCount(0);
    await expect(card.equipmentMoveButtons(eq.id)).toHaveCount(0);

    // API под сессией исполнителя: единица в незавершённом документе не убирается.
    const csrf = (await page.locator('meta[name="csrf-token"]').getAttribute('content')) ?? '';
    const del = await page.request.delete(`/api/v1/tasks/${task.id}/equipment/${eq.id}`, { headers: { 'X-Csrf-Token': csrf }, failOnStatusCode: false });
    expect(del.status()).toBe(409);
    expect(await del.json()).toMatchObject({
      code: 'EQUIPMENT_IN_TRANSFER',
      message: `Оборудование в перемещении #${doc.id} — сначала отмените его`,
      details: { transfer_id: doc.id },
    });

    // негатив TS-018: после отмены документа единица свободна и убирается из заказа через UI.
    await adminApi.transferAction(doc.id, 'cancel');
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.equipmentLocation(eq.id)).toHaveText('на складе Алматы'); // вернулась на склад отправления
    await expect(card.equipmentTransfer(eq.id)).toHaveCount(0);
    await card.equipmentDelete(eq.id).click();
    await card.confirmUiDialog();
    await expect(card.equipmentRows).toHaveCount(0);
  });

  test('EW-003 (DS-046, D58): склад по умолчанию строки товара — склад единицы оборудования услуги', async ({ adminPage: page, adminApi, uniq }) => {
    const { moscow, client, equipment, columns } = await setup(adminApi, uniq, 'EW003');
    const eq = equipment[0];
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('EW-003 категория товаров') });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('EW-003 категория услуг') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('EW-003 товар'), stock: 5, min_stock: 0 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('EW-003 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('EW-003 заказ'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    await adminApi.addTaskEquipmentAt(task.id, eq.id, moscow.id);
    const created = await adminApi.createSubtask(task.id, service.id);
    const subtaskId = created.card.subtasks[0].id;
    await adminApi.updateSubtask(subtaskId, { status: 'new', equipment_id: eq.id });
    const withProduct = await adminApi.createSubtaskProduct(subtaskId, { product_id: product.id, qty: 1 });
    const productRowId = withProduct.card.subtasks[0].products[0].id;
    expect(withProduct.card.subtasks[0].products[0].warehouse_id).toBe(moscow.id);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.productWarehouse(productRowId)).toContainText('Москва');
  });

  test('EW-004 (TS-044): доска — фильтр «Склад» (на складе / едет на склад) и бейдж «в пути» по документу; после приёмки бейджа нет', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, moscow, client, equipment, columns } = await setup(adminApi, uniq, 'EW004', 3);
    const col = columnByName(columns, 'Новые').id;
    const atMoscow = await adminApi.createTask({ title: uniq('EW-004 в Москве'), client_id: client.id, column_id: col });
    const toMoscow = await adminApi.createTask({ title: uniq('EW-004 едет в Москву'), client_id: client.id, column_id: col });
    const atAlmaty = await adminApi.createTask({ title: uniq('EW-004 в Алматы'), client_id: client.id, column_id: col });
    await adminApi.addTaskEquipmentAt(atMoscow.id, equipment[0].id, moscow.id);
    await adminApi.addTaskEquipmentAt(toMoscow.id, equipment[1].id, almaty.id);
    await adminApi.addTaskEquipmentAt(atAlmaty.id, equipment[2].id, almaty.id);
    // документ с единицей заказа отправлен.
    const doc = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true, items: [{ kind: 'equipment', equipment_id: equipment[1].id }],
    });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.card(atAlmaty.id)).toBeVisible();
    await expect(board.inTransitBadge(toMoscow.id)).toBeVisible();
    await expect(board.inTransitBadge(atMoscow.id)).toHaveCount(0);
    await expect(board.inTransitBadge(atAlmaty.id)).toHaveCount(0);

    // под фильтром склада назначения виден и заказ с единицей в пути.
    await board.chooseCombobox(board.filterWarehouse, 'Москва');
    await expect(page).toHaveURL(new RegExp(`warehouse_id=${moscow.id}`));
    await expect(board.card(atMoscow.id)).toBeVisible();
    await expect(board.card(toMoscow.id)).toBeVisible();
    await expect(board.inTransitBadge(toMoscow.id)).toBeVisible();
    await expect(board.card(atAlmaty.id)).toHaveCount(0);

    await page.reload();
    await expect(board.card(toMoscow.id)).toBeVisible();
    await expect(board.card(atAlmaty.id)).toHaveCount(0);
    await expect(board.filterWarehouse).toContainText('Москва');

    // под складом отправления единицы в пути уже нет.
    await board.chooseCombobox(board.filterWarehouse, 'Алматы');
    await expect(board.card(atAlmaty.id)).toBeVisible();
    await expect(board.card(toMoscow.id)).toHaveCount(0);

    // негатив: после приёмки документа бейджа «в пути» нет, заказ — под складом назначения.
    await adminApi.transferAction(doc.id, 'receive');
    await board.chooseCombobox(board.filterWarehouse, 'Москва');
    await expect(board.card(toMoscow.id)).toBeVisible();
    await expect(board.inTransitBadge(toMoscow.id)).toHaveCount(0);
    expect((await adminApi.getTask(toMoscow.id)).has_in_transit).toBe(false);

    await board.filterReset.click();
    await expect(page).not.toHaveURL(/warehouse_id=/);
    await expect(board.card(atMoscow.id)).toBeVisible();
    await expect(board.card(atAlmaty.id)).toBeVisible();
    await expect(board.inTransitBadge(toMoscow.id)).toHaveCount(0);
  });

  test('EW-005 (TS-036, TS-027): в форме добавления единицы нет «Склада приёма» — единица остаётся там, где числится', async ({ adminPage: page, adminApi, uniq }) => {
    const { moscow, client, equipment, columns } = await setup(adminApi, uniq, 'EW005', 2);
    const [onStock, fresh] = equipment;
    const col = columnByName(columns, 'Новые').id;
    const first = await adminApi.createTask({ title: uniq('EW-005 первый заказ'), client_id: client.id, column_id: col });
    const second = await adminApi.createTask({ title: uniq('EW-005 второй заказ'), client_id: client.id, column_id: col });
    await adminApi.addTaskEquipmentAt(first.id, onStock.id, moscow.id); // принята на «Москву» документом первого заказа

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(second.id);
    await card.waitOpen();
    // хвост TS-036: select «Склад приёма» и подпись под ним убраны и из «Нового оборудования».
    await expect(card.equipmentWarehouse).toHaveCount(0);
    await expect(card.equipmentWarehouseHint).toHaveCount(0);
    await card.equipmentNewBtn.click();
    await expect(card.equipmentNewModel).toBeVisible();
    await expect(card.equipmentWarehouse).toHaveCount(0);
    await card.equipmentNewCancel.click();

    await card.chooseCombobox(card.equipmentAdd, onStock.model);
    await expect(card.equipmentRows).toHaveCount(1);
    await expect(card.equipmentLocation(onStock.id)).toHaveText('на складе Москва'); // прежнее местоположение
    await card.chooseCombobox(card.equipmentAdd, fresh.model);
    await expect(card.equipmentRows).toHaveCount(2);
    await expect(card.equipmentLocation(fresh.id)).toHaveText('у клиента'); // не «принята» добавлением

    const data = await adminApi.getCard(second.id);
    expect(cardUnit(data, onStock.id)).toMatchObject({ location_status: 'at_warehouse', warehouse_id: moscow.id });
    expect(cardUnit(data, fresh.id)).toMatchObject({ location_status: 'at_client', warehouse_id: null, active_transfer_id: null });
    expect(data.transfers).toEqual([]); // документов у второго заказа нет
  });

  test('EW-006 (DS-046): история оборудования — текущее местоположение и список документов единицы', async ({ adminPage: page, adminApi, uniq }) => {
    const { almaty, moscow, client, equipment, columns } = await setup(adminApi, uniq, 'EW006');
    const eq = equipment[0];

    await page.goto(`/equipment/${eq.id}/history`);
    await expect(page.getByTestId('history-location')).toHaveText('у клиента');
    await expect(page.getByTestId('history-transfers')).toHaveCount(0);

    const task = await adminApi.createTask({ title: uniq('EW-006 заказ'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    await adminApi.addTaskEquipmentAt(task.id, eq.id, almaty.id);
    const doc = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true, items: [{ kind: 'equipment', equipment_id: eq.id }],
    });

    await page.reload();
    await expect(page.getByTestId('history-location')).toHaveText('в пути → Москва');
    await expect(page.getByTestId('history-active-transfer')).toHaveText(`перемещение #${doc.id}`);
    const row = page.locator(`[data-testid="history-transfer"][data-transfer-id="${doc.id}"]`);
    await expect(row).toContainText('Алматы → Москва');
    await expect(row.getByTestId('history-transfer-status')).toHaveText('В пути');
    await expect(row.getByTestId('history-transfer-date')).toHaveText(/^\d{2}\.\d{2}\.\d{4}$/);
    // склада единицы «в каждом заказе» больше нет.
    await expect(page.locator(`[data-testid="history-order"][data-task-id="${task.id}"]`).getByTestId('history-order-location')).toHaveCount(0);

    await adminApi.transferAction(doc.id, 'receive');
    await page.reload();
    await expect(page.getByTestId('history-location')).toHaveText('на складе Москва');
    await expect(page.getByTestId('history-active-transfer')).toHaveCount(0);
    await expect(row.getByTestId('history-transfer-status')).toHaveText('Получено');

    await row.getByTestId('history-transfer-link').click();
    await expect(page).toHaveURL(new RegExp(`/transfers\\?id=${doc.id}$`));
    await expect(new TransfersPage(page).cardStatus).toHaveText('Получено');
  });

  test('EW-007 (TS-045, TS-046): карточка заказа 375×812 в светлой и тёмной теме — без горизонтальной прокрутки, бейджи местоположения читаемы', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, moscow, client, equipment, columns } = await setup(adminApi, uniq, 'EW007', 3);
    const [atClient, atWarehouse, inTransit] = equipment;
    const colId = columnByName(columns, 'Новые').id;
    const task = await adminApi.createTask({ title: uniq('EW-007 заказ'), client_id: client.id, column_id: colId });
    await adminApi.addTaskEquipmentAt(task.id, atWarehouse.id, almaty.id);
    await adminApi.addTaskEquipmentAt(task.id, inTransit.id, almaty.id);
    await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true, items: [{ kind: 'equipment', equipment_id: inTransit.id }],
    });

    await page.setViewportSize({ width: 375, height: 812 });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    for (const theme of THEMES) {
      // последний зарегистрированный init-скрипт задаёт тему (эталон — polish.spec.ts).
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      const where = `карточка заказа ${theme} 375×812`;
      await board.goto(`?client_id=${client.id}&task=${task.id}`);
      await card.waitOpen();
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      await expect(card.equipmentLocation(atWarehouse.id)).toHaveText('на складе Алматы');
      await expect(card.equipmentLocation(inTransit.id)).toHaveText('в пути → Москва');

      // TS-045: нет горизонтальной прокрутки страницы с открытой карточкой.
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow, `${where}: горизонтальная прокрутка`).toBeLessThanOrEqual(0);

      // TS-046: бейджи местоположения и «перемещение #N» читаемы.
      for (const [id, label] of [[atWarehouse.id, 'на складе'], [inTransit.id, 'в пути']] as const) {
        expect(await contrastOf(card.equipmentLocation(id)), `${where}: бейдж «${label}»`).toBeGreaterThanOrEqual(3);
      }
      expect(await contrastOf(card.equipmentTransfer(inTransit.id)), `${where}: «перемещение #N»`).toBeGreaterThanOrEqual(3);
    }

    // бейдж «у клиента»: местоположение единицы видно на странице истории (в заказ единица
    // «у клиента» через API не попадает — добавление принимает её на склад).
    for (const theme of THEMES) {
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      await page.goto(`/equipment/${atClient.id}/history`);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      const badge = page.getByTestId('history-location');
      await expect(badge).toHaveText('у клиента');
      expect(await contrastOf(badge), `история ${theme}: бейдж «у клиента»`).toBeGreaterThanOrEqual(3);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow, `история ${theme} 375×812: горизонтальная прокрутка`).toBeLessThanOrEqual(0);
    }
  });
});
