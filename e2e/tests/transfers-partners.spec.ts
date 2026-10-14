import type { Page } from '@playwright/test';
import { test, expect, columnByName } from '../fixtures/board';
import { TransfersPage } from '../pages/TransfersPage';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { loginContext, type AdminApi } from '../fixtures/auth';
import { contrastOf } from '../fixtures/contrast';
import { TZ_ALMATY, TZ_BROWSER, TZ_FIXTURE, TZ_MOSCOW, pinFixtureMoment } from '../fixtures/timezone';

// Документ «Перемещение» с клиентом и поставщиком (docs/features/transfers-partners/
// draft-spec.md §8.1, §8.2): приём в ремонт и возврат клиенту из карточки заказа (TS-029),
// приход от поставщика (TS-032, хвост TS-035), фильтры списка с «периодом» (TS-039, TS-023).
// Права исполнителя (TS-034) — TR-007 в transfers.spec.ts; хвост TS-036 («Склад приёма»
// убран) — EW-001 / EW-005 в equipment-warehouses.spec.ts. Общая dev-БД (§TD-6.2): свои
// клиенты, оборудование, заказы, товары и поставщики через uniq()+adminApi, склады — сид
// «Алматы» / «Москва». Заказы с документами убирает _boardCleanup
// (purgeTask: отмена проведения → отмена → удаление документов, затем заказ; transfers-revert).
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;

type Uniq = (s: string) => string;

/** Клиент + заказ + count единиц оборудования в заказе — все «у клиента» (DS-045). */
async function orderWithUnits(adminApi: AdminApi, uniq: Uniq, tag: string, count: number, assigneeId?: string) {
  const almaty = await adminApi.warehouseByName('Алматы');
  const moscow = await adminApi.warehouseByName('Москва');
  const client = await adminApi.createClient({ company_name: uniq(`${tag} клиент`), phones: [{ phone: '9990000501', note: '' }] });
  const type = await adminApi.createEquipmentType({ name: uniq(`${tag} тип`) });
  const columns = (await adminApi.listColumns()).items;
  const task = await adminApi.createTask({
    title: uniq(`${tag} заказ`), client_id: client.id, column_id: columnByName(columns, 'Новые').id, ...(assigneeId ? { assignee_id: assigneeId } : {}),
  });
  const units = [];
  for (let i = 0; i < count; i++) {
    const eq = await adminApi.createEquipment(client.id, {
      type_id: type.id, brand: `Brand${tag}`, model: uniq(`${tag} модель ${i}`), serial: uniq(`sn-${tag}-${i}`),
    });
    await adminApi.addTaskEquipment(task.id, eq.id);
    units.push(eq);
  }
  return { almaty, moscow, client, task, units };
}

async function newProduct(adminApi: AdminApi, uniq: Uniq, label: string, stock: number) {
  const cat = await adminApi.createCategory({ kind: 'product', name: uniq(`${label} категория`) });
  return adminApi.createProduct({ category_id: cat.id, name: uniq(`${label} товар`), stock }); // остаток — на «Алматы»
}

async function pairStock(adminApi: AdminApi, productName: string, warehouseId: string): Promise<number> {
  const list = await adminApi.getStock({ q: productName, scope: 'all' });
  const cell = list.items[0]?.by_warehouse.find((c) => c.warehouse_id === warehouseId);
  return Number(cell?.stock ?? NaN);
}

/** Календарные даты в поясе ПОЛЬЗОВАТЕЛЯ — тем же KT.time, что и экран (не Date теста). */
async function userDays(page: Page, offsets: number[]): Promise<string[]> {
  return page.evaluate((list) => {
    const time = (window as unknown as { KT: { time: { today(): string; addDays(ymd: string, n: number): string } } }).KT.time;
    const today = time.today();
    return list.map((n) => (n === 0 ? today : time.addDays(today, n)));
  }, offsets);
}

test.describe('TR: перемещения — клиент и поставщик', () => {
  test('TR-012 (TS-029): исполнитель принимает оборудование в ремонт из карточки заказа («Принять сразу») и возвращает клиенту («Отправить» → «Принять»); единица, уже стоящая в заявке, → «уже в перемещении #N»', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    const exec = await makeUser({ login: uniq('tr012-exec'), name: uniq('TR-012 Исполнитель') });
    const { almaty, client, task, units } = await orderWithUnits(adminApi, uniq, 'TR012', 1, exec.id);
    const unit = units[0];
    // незавершённая заявка на приём той же единицы — для негатива.
    const holder = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id, items: [{ kind: 'equipment', equipment_id: unit.id }],
    });
    const defaultWh = await adminApi.defaultWarehouse();

    await loginAs(page, exec.login, exec.password);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const tr = new TransfersPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();

    // блок «Перемещения заказа»: документ-заявка, кнопки по местоположению единиц.
    await expect(card.equipmentLocation(unit.id)).toHaveText('у клиента');
    await expect(card.transferRows).toHaveCount(1);
    await expect(card.transferLink(holder.id)).toHaveText(`#${holder.id}`);
    await expect(card.transferLink(holder.id)).toHaveAttribute('href', `/transfers?id=${holder.id}`);
    await expect(card.transferRow(holder.id).getByTestId('card-transfer-direction')).toHaveText('Приём от клиента');
    await expect(card.transferRow(holder.id).getByTestId('card-transfer-warehouse')).toHaveText('Алматы');
    await expect(card.transferStatus(holder.id)).toHaveText('Заявка');
    await expect(card.transferRow(holder.id).getByTestId('card-transfer-count')).toHaveText('1');
    await expect(card.equipmentTransfer(unit.id)).toHaveAttribute('href', `/transfers?id=${holder.id}`); // ссылка и у исполнителя
    await expect(card.transferReturn).toBeDisabled();
    await expect(card.transferReturnHint).toHaveAttribute('title', 'Нет оборудования на складе');
    await expect(card.transferIntake).toHaveAttribute('href', `/transfers?new=1&direction=client_to_warehouse&task_id=${task.id}`);

    // «Принять от клиента» → форма нового документа: заказ, клиент, единица и склад предвыбраны.
    await card.transferIntake.click();
    await expect(page).toHaveURL(new RegExp(`/transfers\\?new=1&direction=client_to_warehouse&task_id=${task.id}$`));
    await expect(tr.cardTitle).toHaveText('Новое перемещение');
    await expect(tr.direction).toHaveText('Приём от клиента');
    await expect(tr.task).toHaveValue(String(task.id));
    await expect(tr.taskInfo).toContainText(task.title);
    await expect(tr.taskInfo).toContainText(client.company_name);
    await expect(tr.to).toContainText(defaultWh.name);
    await expect(tr.from).toHaveCount(0); // склада отправления у приёма нет
    await expect(tr.addProduct).toHaveCount(0); // от клиента — только его оборудование
    // негатив 1: единица уже в незавершённой заявке — в документ не предвыбрана и вручную не
    // добавляется: в «+ Оборудование» неактивна, с пометкой «в перемещении #N».
    await expect(tr.itemRow('equipment', unit.id)).toHaveCount(0);
    await expect(tr.taskEquipmentOption(unit.id)).toBeDisabled();
    await expect(tr.taskEquipmentOption(unit.id).getByTestId('st-task-equipment-busy')).toHaveText(`в перемещении #${holder.id}`);

    // заявку отменили — после reload форма открывается снова (параметры в URL), единица
    // свободна и предвыбрана.
    await adminApi.transferAction(holder.id, 'cancel');
    await page.reload();
    await expect(tr.direction).toHaveText('Приём от клиента');
    await expect(tr.itemRow('equipment', unit.id)).toContainText(unit.model);

    // негатив 2: пока форма открыта, единицу заняла другая заявка → сохранение отвечает 409,
    // красный текст в строке «уже в перемещении #N» со ссылкой; документ не создан.
    const rival = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id, items: [{ kind: 'equipment', equipment_id: unit.id }],
    });
    allowHttpErrors((url, status) => url.endsWith('/api/v1/transfers') && status === 409);
    await tr.receiveNow.click();
    await expect(tr.itemError('equipment', unit.id)).toContainText(`уже в перемещении #${rival.id}`);
    await expect(tr.itemError('equipment', unit.id).getByTestId('st-item-transfer-link')).toHaveAttribute('href', `/transfers?id=${rival.id}`);
    await expect(tr.cardTitle).toHaveText('Новое перемещение'); // документ не создан
    expect((await adminApi.getCard(task.id)).transfers).toHaveLength(2);

    // и эту заявку отменили — приём проходит.
    await adminApi.transferAction(rival.id, 'cancel');
    await page.reload();
    await expect(tr.direction).toHaveText('Приём от клиента');
    await expect(tr.itemRow('equipment', unit.id)).toBeVisible();
    await expect(tr.taskEquipmentOption(unit.id).getByTestId('st-task-equipment-location')).toHaveText('у клиента');
    await tr.choose(tr.to, 'Алматы');
    await expect(tr.send).toBeVisible(); // три шага доступны и здесь
    await tr.receiveNow.click();
    await expect(tr.cardStatus).toHaveText('Получено');
    const intakeId = await tr.currentId();
    await expect(page).toHaveURL(new RegExp(`/transfers\\?id=${intakeId}$`));
    await expect(tr.direction).toHaveText('Приём от клиента');
    await expect(tr.taskLink).toHaveText(`Заказ #${task.id}`);
    await expect(tr.taskLink).toHaveAttribute('href', `/?task=${task.id}`);
    await expect(tr.viewItem('equipment', unit.id)).toContainText(unit.model);
    await expect(tr.factHead).toHaveCount(0); // у документа с клиентом факта нет
    await expect(tr.row(intakeId).getByTestId('st-row-route')).toHaveText(`#${intakeId} · Клиент → Алматы`);
    await expect(tr.row(intakeId).getByTestId('st-row-party')).toHaveText(`${client.company_name} · заказ #${task.id}`);

    const intake = await adminApi.getTransfer(intakeId);
    expect(intake).toMatchObject({
      direction: 'client_to_warehouse', status: 'received', task_id: task.id, client_id: client.id,
      to_warehouse_id: almaty.id, from_warehouse_id: null, supplier_id: null,
    });

    // карточка заказа: документ «Получено», единица «на складе Алматы».
    await tr.taskLink.click();
    await card.waitOpen();
    await expect(card.transferStatus(intakeId)).toHaveText('Получено');
    await expect(card.equipmentLocation(unit.id)).toHaveText('на складе Алматы');
    await expect(card.equipmentTransfer(unit.id)).toHaveCount(0);
    await expect(card.transferIntake).toBeDisabled();
    await expect(card.transferIntakeHint).toHaveAttribute('title', 'Нет оборудования у клиента');

    // «Вернуть клиенту» → «Отправить» → «Принять».
    await card.transferReturn.click();
    await expect(page).toHaveURL(new RegExp(`/transfers\\?new=1&direction=warehouse_to_client&task_id=${task.id}$`));
    await expect(tr.direction).toHaveText('Возврат клиенту');
    await expect(tr.from).toContainText('Алматы'); // склад первой единицы на складе
    await expect(tr.to).toHaveCount(0);
    await expect(tr.receiveNow).toHaveCount(0); // «Принять сразу» — только у приёма от клиента
    await expect(tr.itemRow('equipment', unit.id)).toBeVisible();
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    const backId = await tr.currentId();
    expect((await adminApi.getCard(task.id)).equipment[0]).toMatchObject({
      location_status: 'in_transit', warehouse_id: null, active_transfer_id: backId, transit_to_name: 'клиент',
    });
    await tr.confirmReceive('Клиент получил оборудование?');
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.receive).toHaveCount(0);

    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.equipmentLocation(unit.id)).toHaveText('у клиента');
    await expect(card.transferRows).toHaveCount(4);
    await expect(card.transferRows.first()).toHaveAttribute('data-id', String(backId)); // новые сверху
    await expect(card.transferRow(backId).getByTestId('card-transfer-direction')).toHaveText('Возврат клиенту');
    await expect(card.transferStatus(backId)).toHaveText('Получено');
    await expect(card.transferStatus(holder.id)).toHaveText('Отменён');
    await expect(card.transferIntake).toBeEnabled(); // единица снова у клиента
  });

  test('TR-013 (TS-029, TS-045, TS-046): блок «Перемещения заказа» и форма документа с клиентом — 375×812 без горизонтальной прокрутки, читаемы в светлой и тёмной теме', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, client, task, units } = await orderWithUnits(adminApi, uniq, 'TR013', 3);
    const [atClient, onStock, leaving] = units;
    const received = await adminApi.receiveFromClient(task.id, almaty.id, [onStock.id, leaving.id]);
    const inTransit = await adminApi.createTransfer({
      direction: 'warehouse_to_client', from_warehouse_id: almaty.id, task_id: task.id, send: true,
      note: 'возврат после ремонта '.repeat(12).trim(), items: [{ kind: 'equipment', equipment_id: leaving.id }],
    });
    const requested = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id, items: [{ kind: 'equipment', equipment_id: atClient.id }],
    });
    await adminApi.transferAction(requested.id, 'cancel');

    await page.setViewportSize({ width: 375, height: 812 });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const tr = new TransfersPage(page);
    for (const theme of THEMES) {
      // последний зарегистрированный init-скрипт задаёт тему (эталон — polish.spec.ts).
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      const where = `${theme} 375×812`;

      // карточка заказа: блок «Перемещения заказа».
      await board.goto(`?client_id=${client.id}&task=${task.id}`);
      await card.waitOpen();
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      await card.transfers.scrollIntoViewIfNeeded();
      await expect(card.transferRows).toHaveCount(3);
      await expect(card.equipmentLocation(leaving.id)).toHaveText('в пути → клиент');
      await card.expectNoHorizontalScroll();
      for (const [doc, label] of [[received, 'Получено'], [inTransit, 'В пути'], [requested, 'Отменён']] as const) {
        await expect(card.transferStatus(doc.id)).toHaveText(label);
        expect(await contrastOf(card.transferStatus(doc.id)), `карточка ${where}: бейдж «${label}»`).toBeGreaterThanOrEqual(3);
        expect(await contrastOf(card.transferLink(doc.id)), `карточка ${where}: ссылка #${doc.id}`).toBeGreaterThanOrEqual(3);
      }
      expect(await contrastOf(card.transferIntake), `карточка ${where}: «Принять от клиента»`).toBeGreaterThanOrEqual(3);
      expect(await contrastOf(card.transferReturn), `карточка ${where}: «Вернуть клиенту»`).toBeGreaterThanOrEqual(3);
      expect(await contrastOf(card.equipmentLocation(leaving.id)), `карточка ${where}: «в пути → клиент»`).toBeGreaterThanOrEqual(3);

      // форма нового документа с клиентом.
      await tr.gotoNew('client_to_warehouse', task.id);
      await expect(tr.itemRow('equipment', atClient.id)).toBeVisible();
      await expect(tr.taskEquipmentOption(onStock.id)).toBeDisabled(); // не в источнике — неактивна
      await tr.expectNoHorizontalScroll();
      expect(await contrastOf(tr.direction), `форма ${where}: направление`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.taskInfo), `форма ${where}: заказ и клиент`).toBeGreaterThanOrEqual(3);
      expect(await contrastOf(tr.receiveNow), `форма ${where}: «Принять сразу»`).toBeGreaterThanOrEqual(3);
      for (const eq of [atClient, onStock, leaving]) {
        const badge = tr.taskEquipmentOption(eq.id).getByTestId('st-task-equipment-location');
        expect(await contrastOf(badge), `форма ${where}: бейдж местоположения`).toBeGreaterThanOrEqual(3);
      }

      // документ «в пути → клиент» и строка списка с бейджем направления.
      await tr.goto(inTransit.id);
      await expect(tr.cardStatus).toHaveText('В пути');
      await tr.expectNoHorizontalScroll();
      expect(await contrastOf(tr.row(inTransit.id).getByTestId('st-row-direction')), `список ${where}: бейдж направления`).toBeGreaterThanOrEqual(3);
      expect(await contrastOf(tr.taskLink), `документ ${where}: ссылка на заказ`).toBeGreaterThanOrEqual(3);
    }
  });

  test('TR-014 (TS-032, TS-035): приход от поставщика — заказано 10, «Отправить» без движения остатка, «Принять» с фактом 12: «Получено», остаток +12; второй приход с фактом 7 — остаток +7; «+ Оборудование» нет, архивного поставщика в select нет', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
    transfersCleanup,
  }) => {
    const almaty = await adminApi.warehouseByName('Алматы');
    const product = await newProduct(adminApi, uniq, 'TR-014', 3);
    const supplier = await adminApi.createSupplier({ name: uniq('TR-014 поставщик') });
    const archived = await adminApi.createSupplier({ name: uniq('TR-014 архивный поставщик') });
    await adminApi.updateSupplier(archived.id, { name: archived.name, is_active: false });

    const tr = new TransfersPage(page);
    await tr.goto();
    await tr.newDocument('supplier_to_warehouse');
    // негатив: в приходе от поставщика — только товары.
    await expect(tr.equipmentSearch).toHaveCount(0);
    await expect(tr.taskEquipment).toHaveCount(0);
    await expect(tr.task).toHaveCount(0);
    await expect(tr.from).toHaveCount(0);
    await expect(tr.receiveNow).toHaveCount(0);
    await expect(tr.qtyHead).toHaveText('Заказано');

    // негатив: без поставщика — ошибка под полем.
    allowHttpErrors((url, status) => url.endsWith('/api/v1/transfers') && status === 400);
    await tr.choose(tr.to, 'Алматы');
    await tr.addProductRow(product.name, product.id, 10);
    await tr.save.click();
    await expect(tr.supplierError).toHaveText('Укажите поставщика');
    await expect(tr.cardTitle).toHaveText('Новое перемещение');

    // хвост TS-035: архивного поставщика в select нет, действующий — есть.
    await tr.supplier.click();
    await expect(tr.dropdown).toBeVisible();
    await tr.dropdown.locator('.cb-search').fill('TR-014');
    await expect(tr.dropdown.locator('.cb-opt', { hasText: supplier.name })).toHaveCount(1);
    await expect(tr.dropdown.locator('.cb-opt', { hasText: archived.name })).toHaveCount(0);
    await tr.dropdown.locator('.cb-opt', { hasText: supplier.name }).first().click();
    await expect(tr.supplier).toContainText(supplier.name);

    // «Отправить»: документ «В пути», остаток не меняется (товар ещё не наш), на «Остатках» — «в пути».
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    const id = await tr.currentId();
    transfersCleanup(id); // документ создан через экран — без уборки держит товар и поставщика
    await expect(tr.warnings).toHaveCount(0);
    await expect(tr.direction).toHaveText('Приход от поставщика');
    await expect(page.getByTestId('st-view-supplier')).toHaveText(supplier.name);
    await expect(page.getByTestId('st-view-to')).toHaveText('Алматы');
    await expect(page.getByTestId('st-view-from')).toHaveCount(0);
    await expect(tr.row(id).getByTestId('st-row-direction')).toHaveText('Приход от поставщика');
    await expect(tr.row(id).getByTestId('st-row-route')).toHaveText(`#${id} · Поставщик → Алматы`);
    await expect(tr.row(id).getByTestId('st-row-party')).toHaveText(supplier.name);
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(3);
    const stockRow = (await adminApi.getStock({ q: product.name, scope: 'all', warehouse_id: almaty.id })).items[0];
    expect(Number(stockRow.in_transit)).toBe(10);

    const sent = await adminApi.getTransfer(id);
    expect(sent).toMatchObject({ direction: 'supplier_to_warehouse', status: 'in_transit', supplier_id: supplier.id, to_warehouse_id: almaty.id, from_warehouse_id: null, task_id: null });
    const line = sent.items[0];

    // «Принять»: факт больше заказанного — разрешён только этому направлению.
    await tr.receive.click();
    await expect(tr.receiveModal).toBeVisible();
    await expect(page.getByTestId('st-receive-sent-head')).toHaveText('Заказано');
    await expect(tr.receiveQty(line.id)).toHaveValue('10');
    await tr.receiveQty(line.id).fill('12');
    await tr.receiveConfirm.click();
    await expect(tr.receiveModal).toBeHidden();
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.qtyHead).toHaveText('Заказано');
    await expect(tr.factHead).toHaveText('Получено');
    await expect(tr.viewItem('product', product.id).getByTestId('st-view-sent')).toHaveText('10');
    const fact = tr.viewItem('product', product.id).getByTestId('st-view-received');
    await expect(fact).toHaveText('12');
    await expect(fact).toHaveAttribute('data-diff', 'true'); // расхождение подсвечено и «в плюс»
    await expect(fact).not.toHaveAttribute('data-loss', 'true');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(15); // 3 + 12
    const after = (await adminApi.getStock({ q: product.name, scope: 'all', warehouse_id: almaty.id })).items[0];
    expect(Number(after.in_transit)).toBe(0);

    // второй приход: факт МЕНЬШЕ заказанного (недопоставка) — остаток растёт на факт, расхождение
    // помечено и как недостача.
    const short = await adminApi.createTransfer({
      direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id, send: true,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 10 }],
    });
    const shortLine = (await adminApi.getTransfer(short.id)).items[0];
    await tr.goto(short.id);
    await expect(tr.cardStatus).toHaveText('В пути');
    await tr.receive.click();
    await expect(tr.receiveModal).toBeVisible();
    await tr.receiveQty(shortLine.id).fill('7');
    await tr.receiveConfirm.click();
    await expect(tr.receiveModal).toBeHidden();
    await expect(tr.cardStatus).toHaveText('Получено');
    const shortFact = tr.viewItem('product', product.id).getByTestId('st-view-received');
    await expect(shortFact).toHaveText('7');
    await expect(shortFact).toHaveAttribute('data-diff', 'true');
    await expect(shortFact).toHaveAttribute('data-loss', 'true');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(22); // 15 + 7

    // поставщик ушёл в архив — в проведённом документе он остаётся.
    await adminApi.updateSupplier(supplier.id, { name: supplier.name, is_active: false });
    await tr.goto(id);
    await expect(page.getByTestId('st-view-supplier')).toHaveText(supplier.name);
  });

  test('TR-028 (TS-032): приход от поставщика — «Куда» и товар, выбранные ДО прихода списка поставщиков, сохраняются: форма не перерисовывается, после ответа обновляется только список «Поставщик»; негатив: до ответа поставщика в списке нет', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const product = await newProduct(adminApi, uniq, 'TR-028', 3);
    const supplier = await adminApi.createSupplier({ name: uniq('TR-028 поставщик') });

    // ответ списка активных поставщиков формы задерживается, пока тест не отпустит его.
    let release!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    let held = 0;
    await page.route('**/api/v1/suppliers?is_active=true*', async (route) => {
      held += 1;
      await gate;
      await route.continue();
    });

    const tr = new TransfersPage(page);
    await tr.goto();
    await tr.newDocument('supplier_to_warehouse');
    await expect.poll(() => held, { message: 'форма запросила активных поставщиков' }).toBe(1);

    // пользователь заполняет форму, пока поставщики ещё не пришли.
    await tr.choose(tr.to, 'Алматы');
    await tr.addProductRow(product.name, product.id, 10);
    // негатив: списка поставщиков ещё нет — в «Поставщик» только пустой пункт.
    await expect(page.locator('#st-supplier option', { hasText: supplier.name })).toHaveCount(0);

    const loaded = page.waitForResponse((r) => r.url().includes('/api/v1/suppliers?is_active=true') && r.status() === 200);
    release();
    await loaded;

    // после ответа: поставщик появился в списке, а введённое не сброшено.
    await tr.choose(tr.supplier, supplier.name);
    await expect(tr.supplier).toContainText(supplier.name);
    await expect(tr.to).toContainText('Алматы');
    await expect(tr.itemRow('product', product.id)).toBeVisible();
    await expect(tr.itemQty(product.id)).toHaveValue('10');

    // форма сохраняется со всеми тремя значениями.
    await tr.save.click();
    await expect(tr.cardStatus).toHaveText('Заявка');
    const id = await tr.currentId();
    const saved = await adminApi.getTransfer(id);
    const almaty = await adminApi.warehouseByName('Алматы');
    expect(saved).toMatchObject({ direction: 'supplier_to_warehouse', status: 'requested', supplier_id: supplier.id, to_warehouse_id: almaty.id });
    expect(saved.items.map((i) => i.product_id)).toEqual([product.id]);
    await adminApi.removeTransferForCleanup(id);
  });

  test('TR-015 (TS-039): фильтры списка — статус + направление + период, поставщик, заказ; «Сбросить» возвращает всё; «по» раньше «с» → «Некорректный период»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { almaty, moscow, task, units } = await orderWithUnits(adminApi, uniq, 'TR015', 1);
    const product = await newProduct(adminApi, uniq, 'TR-015', 5);
    const supplier = await adminApi.createSupplier({ name: uniq('TR-015 поставщик') });
    const items = [{ kind: 'product' as const, product_id: product.id, qty_sent: 1 }];
    const w2wTransit = await adminApi.createTransfer({ from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, send: true, items });
    const supplyRequested = await adminApi.createTransfer({ direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id, items });
    const supplyTransit = await adminApi.createTransfer({ direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id, send: true, items });
    const intake = await adminApi.receiveFromClient(task.id, almaty.id, [units[0].id]);
    const all = [w2wTransit, supplyRequested, supplyTransit, intake];

    const tr = new TransfersPage(page);
    await tr.goto();
    for (const doc of all) await expect(tr.row(doc.id)).toBeVisible();

    // статус.
    await tr.pick(tr.filterStatus, 'В пути');
    await expect(tr.row(supplyRequested.id)).toHaveCount(0);
    await expect(tr.row(intake.id)).toHaveCount(0);
    await expect(tr.row(w2wTransit.id)).toBeVisible();
    await expect(tr.row(supplyTransit.id)).toBeVisible();

    // + направление (комбинация по «И»).
    await tr.pick(tr.filterDirection, 'Приход от поставщика');
    await expect(tr.row(w2wTransit.id)).toHaveCount(0);
    await expect(tr.row(supplyTransit.id)).toBeVisible();
    for (const row of await tr.rows.all()) {
      await expect(row).toHaveAttribute('data-direction', 'supplier_to_warehouse');
      await expect(row.getByTestId('st-status-badge')).toHaveAttribute('data-status', 'in_transit');
    }

    // + период: даты — календарные дни пользователя (KT.time), документ создан только что.
    // Период, который должен содержать документ, — «вчера…завтра»: дни считаются один раз, и
    // прогон на стыке суток пользователя (документ создан «вчера» либо уже «завтра») не ломается.
    const [farAgo, tenAgo, twoAgo, yesterday, tomorrow] = await userDays(page, [-20, -10, -2, -1, 1]);
    await tr.filterFrom.fill(yesterday);
    await tr.filterTo.fill(tomorrow);
    await expect(tr.row(supplyTransit.id)).toBeVisible();
    await expect(tr.periodError).toBeHidden();
    // сначала «с», потом «по»: каждое поле шлёт запрос само, промежуточный период обязан быть
    // корректным (иначе — незаявленный 400 «по раньше с»).
    await tr.filterFrom.fill(tenAgo);
    await tr.filterTo.fill(twoAgo);
    await expect(tr.row(supplyTransit.id)).toHaveCount(0); // период целиком в прошлом
    await expect(tr.periodError).toBeHidden();

    // негатив (§8.1: «400 (период): красный текст под полями периода»): «по» раньше «с» — экран
    // шлёт запрос, сервер отвечает 400 created_to: before_from. Ожидаем ровно этот запрос списка.
    allowHttpErrors((url, status) => {
      if (status !== 400 || !url.includes('/api/v1/transfers?')) return false;
      const q = new URL(url).searchParams;
      return q.get('created_from') === tenAgo && q.get('created_to') === farAgo;
    });
    await tr.filterTo.fill(farAgo);
    await expect(tr.periodError).toHaveText('Некорректный период');
    await expect(tr.filterFrom).toHaveClass(/is-invalid/);
    await expect(tr.filterTo).toHaveClass(/is-invalid/);

    // период исправлен («−10 дней…завтра») — список перечитан, сообщение и подсветка сняты.
    await tr.filterTo.fill(tomorrow);
    await expect(tr.periodError).toBeHidden();
    await expect(tr.filterFrom).not.toHaveClass(/is-invalid/);
    await expect(tr.filterTo).not.toHaveClass(/is-invalid/);
    await expect(tr.row(supplyTransit.id)).toBeVisible();
    // и снова некорректный — чтобы «Сбросить» ниже снимал ошибку.
    await tr.filterTo.fill(farAgo);
    await expect(tr.periodError).toHaveText('Некорректный период');

    // «Сбросить» возвращает всё.
    await tr.filterReset.click();
    for (const doc of all) await expect(tr.row(doc.id)).toBeVisible();
    await expect(tr.periodError).toBeHidden();
    await expect(tr.filterStatus).toContainText('Все статусы');
    await expect(tr.filterDirection).toContainText('Все направления');
    await expect(tr.filterFrom).toHaveValue('');
    await expect(tr.filterTo).toHaveValue('');

    // поставщик.
    await tr.choose(tr.filterSupplier, supplier.name);
    await expect(tr.row(supplyRequested.id)).toBeVisible();
    await expect(tr.row(supplyTransit.id)).toBeVisible();
    await expect(tr.row(w2wTransit.id)).toHaveCount(0);
    await expect(tr.row(intake.id)).toHaveCount(0);
    await expect(tr.rows).toHaveCount(2);
    await tr.filterReset.click();
    await expect(tr.filterSupplier).toContainText('Все поставщики');

    // заказ (номер, debounce).
    await tr.filterTask.fill(String(task.id));
    await expect(tr.rows).toHaveCount(1);
    await expect(tr.row(intake.id).getByTestId('st-row-party')).toContainText(`заказ #${task.id}`);
    // негатив: не номер → сервер 400 task_id, тост, список прежний.
    allowHttpErrors((url, status) => status === 400 && url.includes('/api/v1/transfers?') && new URL(url).searchParams.get('task_id') === 'abc');
    await tr.filterTask.fill('abc');
    await expect(tr.toastError).toHaveText('Некорректное значение фильтра');
    await expect(tr.row(intake.id)).toBeVisible();
    await tr.filterReset.click();
    await expect(tr.filterTask).toHaveValue('');
    for (const doc of all) await expect(tr.row(doc.id)).toBeVisible();
  });
});

test.describe('TR: фильтр «период» — пояс пользователя, не браузера', () => {
  // Пояс браузера — третий (Нью-Йорк): экран не должен считать даты «по браузеру».
  test.use({ timezoneId: TZ_BROWSER });

  test('TR-016 (TS-023): момент TS-F5 — дата документа у пользователей Алматы и Москвы разная; фильтр «период» уходит на сервер календарными датами пользователя', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    browser,
    baseURL,
    allowHttpErrors,
  }) => {
    // Граница суток в БД (документ создан 2026-10-06T19:30:00Z: Алматы — 7 октября найден,
    // 6-го нет; Москва — наоборот) покрыта Go live-тестом TestTS023_PeriodFilter_UserTimezone:
    // в E2E момент создания задать нельзя (pg удалён, ADR-101). Здесь — экранная половина:
    // дата в списке показана в поясе пользователя (момент TS-F5 приколот в ответах GET), а
    // введённые даты периода уходят в запрос как есть, без пересчёта через пояс браузера.
    const almatyUser = await makeUser({ login: uniq('tr016-almaty'), name: uniq('Админ Алматы TR-016'), role: 'admin', timezone: TZ_ALMATY });
    const moscowUser = await makeUser({ login: uniq('tr016-moscow'), name: uniq('Админ Москва TR-016'), role: 'admin', timezone: TZ_MOSCOW });
    const almaty = await adminApi.warehouseByName('Алматы');
    const product = await newProduct(adminApi, uniq, 'TR-016', 2);
    const supplier = await adminApi.createSupplier({ name: uniq('TR-016 поставщик') });
    const doc = await adminApi.createTransfer({
      direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 1 }],
    });

    await loginAs(page, almatyUser.login, almatyUser.password);
    await pinFixtureMoment(page);
    const tr = new TransfersPage(page);
    await tr.goto();
    await expect(tr.row(doc.id).getByTestId('st-row-date')).toHaveText(TZ_FIXTURE.almaty.date); // 07.10.2026

    // «Период с» = 7 октября: в запросе — та же календарная дата (не 6-е «по Нью-Йорку»).
    const fromRequest = page.waitForRequest((r) => r.url().includes('/api/v1/transfers?') && r.url().includes('created_from='));
    await tr.filterFrom.fill(TZ_FIXTURE.almaty.today);
    const fromUrl = new URL((await fromRequest).url());
    expect(fromUrl.searchParams.get('created_from')).toBe(TZ_FIXTURE.almaty.today);
    expect(fromUrl.searchParams.get('created_to')).toBeNull();
    await expect(tr.row(doc.id)).toBeVisible(); // документ создан позже начала 7 октября по Алматы

    // негатив: «по» = 6 октября раньше «с» = 7 октября → «Некорректный период».
    allowHttpErrors((url, status) => {
      if (status !== 400 || !url.includes('/api/v1/transfers?')) return false;
      const q = new URL(url).searchParams;
      return q.get('created_from') === TZ_FIXTURE.almaty.today && q.get('created_to') === TZ_FIXTURE.moscow.today;
    });
    await tr.filterTo.fill(TZ_FIXTURE.moscow.today);
    await expect(tr.periodError).toHaveText('Некорректный период');

    // только «по» = 6 октября: конец дня включительно — документ, созданный позже, не найден.
    const toRequest = page.waitForRequest((r) => r.url().includes('/api/v1/transfers?') && !r.url().includes('created_from='));
    await tr.filterFrom.fill('');
    const toUrl = new URL((await toRequest).url());
    expect(toUrl.searchParams.get('created_to')).toBe(TZ_FIXTURE.moscow.today);
    await expect(tr.periodError).toBeHidden();
    await expect(tr.row(doc.id)).toHaveCount(0);

    // пользователь с поясом Москвы: тот же момент — 6 октября.
    const moscowContext = await browser.newContext({ baseURL, locale: 'ru-RU', timezoneId: TZ_BROWSER, colorScheme: 'light' });
    try {
      await loginContext(moscowContext, moscowUser.login, moscowUser.password);
      const moscowPage = await moscowContext.newPage();
      await pinFixtureMoment(moscowPage);
      const trMoscow = new TransfersPage(moscowPage);
      await trMoscow.goto();
      await expect(trMoscow.row(doc.id).getByTestId('st-row-date')).toHaveText(TZ_FIXTURE.moscow.date); // 06.10.2026
      await trMoscow.filterFrom.fill(TZ_FIXTURE.moscow.today);
      await expect(trMoscow.row(doc.id)).toBeVisible();
      await trMoscow.goto(doc.id);
      await expect(trMoscow.cardMeta).toContainText(TZ_FIXTURE.moscow.dateTime);
      await expect(trMoscow.cardMeta).not.toContainText(TZ_FIXTURE.almaty.dateTime);
    } finally {
      await moscowContext.close();
    }
  });
});
