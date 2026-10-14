import { test, expect, columnByName } from '../fixtures/board';
import { TransfersPage } from '../pages/TransfersPage';
import { StockPage } from '../pages/StockPage';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import type { AdminApi, NewTransfer } from '../fixtures/auth';
import { contrastOf } from '../fixtures/contrast';

// Накладная клиенту — товар с ценой в документе «возврат клиенту», сумма накладной и итог
// заказа (docs/features/transfers-sales/draft-spec.md §8.1, §8.2; TS-030). Общая dev-БД
// (§TD-6.2): свои клиенты, оборудование, заказы, услуги и товары через uniq()+adminApi, склады
// — сид «Алматы» / «Москва». Заказы с документами убирает _boardCleanup
// (purgeTask: отмена проведения → отмена → удаление документов, затем заказ; transfers-revert).
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;

type Uniq = (s: string) => string;

/** Клиент + заказ (услуга ценой servicePrice, если задана) + count единиц оборудования заказа
 *  на складе «Алматы». */
async function orderAtAlmaty(adminApi: AdminApi, uniq: Uniq, tag: string, opts: { units?: number; servicePrice?: number; assigneeId?: string } = {}) {
  const almaty = await adminApi.warehouseByName('Алматы');
  const client = await adminApi.createClient({ company_name: uniq(`${tag} клиент`), phones: [{ phone: '9990000601', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  const task = await adminApi.createTask({
    title: uniq(`${tag} заказ`), client_id: client.id, column_id: columnByName(columns, 'Новые').id,
    ...(opts.assigneeId ? { assignee_id: opts.assigneeId } : {}),
  });
  const units = [];
  if (opts.units) {
    const type = await adminApi.createEquipmentType({ name: uniq(`${tag} тип`) });
    for (let i = 0; i < opts.units; i++) {
      const eq = await adminApi.createEquipment(client.id, {
        type_id: type.id, brand: `Brand${tag}`, model: uniq(`${tag} модель ${i}`), serial: uniq(`sn-${tag}-${i}`),
      });
      await adminApi.addTaskEquipmentAt(task.id, eq.id, almaty.id);
      units.push(eq);
    }
  }
  if (opts.servicePrice !== undefined) {
    const cat = await adminApi.createCategory({ kind: 'service', name: uniq(`${tag} услуги`) });
    const svc = await adminApi.createService({ category_id: cat.id, name: uniq(`${tag} услуга`), price: opts.servicePrice });
    await adminApi.createSubtask(task.id, svc.id);
  }
  return { almaty, client, task, units };
}

/** Товар каталога с ценой и остатком на «Алматы». */
async function pricedProduct(adminApi: AdminApi, uniq: Uniq, label: string, price: number, stock: number) {
  const cat = await adminApi.createCategory({ kind: 'product', name: uniq(`${label} категория`) });
  return adminApi.createProduct({ category_id: cat.id, name: uniq(`${label} товар`), price, stock });
}

async function pairStock(adminApi: AdminApi, productName: string, warehouseId: string): Promise<number> {
  const list = await adminApi.getStock({ q: productName, scope: 'all' });
  const cell = list.items[0]?.by_warehouse.find((c) => c.warehouse_id === warehouseId);
  return Number(cell?.stock ?? NaN);
}

test.describe('TR: перемещения — накладная клиенту', () => {
  test('TR-017 (TS-030): возврат клиенту с продажей товара — «+ Товар» с ценой каталога, правка цены, «Отправить» → «Принять»: сумма накладной в документе и списке, «Продано по накладным» и итог в карточке заказа, остаток уменьшился; отрицательная и нечисловая цена → ошибка под полем; отмена накладной «в пути» → строка «Продано по накладным» исчезла, остаток вернулся', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { almaty, client, task, units } = await orderAtAlmaty(adminApi, uniq, 'TR017', { units: 1, servicePrice: 1000 });
    const unit = units[0];
    const product = await pricedProduct(adminApi, uniq, 'TR-017', 250.5, 40);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const tr = new TransfersPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.total).toHaveText('Итог заказа: 1000.00');
    await expect(card.transfersAmount).toHaveCount(0); // накладных нет — строки нет
    await expect(card.equipmentLocation(unit.id)).toHaveText('на складе Алматы');

    // «Вернуть клиенту» → форма накладной: единица предвыбрана, есть «+ Товар», колонки цены.
    await card.transferReturn.click();
    await expect(tr.direction).toHaveText('Возврат клиенту');
    await expect(tr.itemRow('equipment', unit.id)).toBeVisible();
    await expect(tr.priceHead).toHaveText('Цена');
    await expect(tr.totalHead).toHaveText('Сумма');
    await expect(tr.amount).toHaveText('0.00');
    await expect(tr.itemRow('equipment', unit.id).getByTestId('st-item-price')).toHaveCount(0); // у единицы цены нет

    // «+ Товар»: цена подставилась из каталога; суммы пересчитываются при вводе, без запроса.
    await tr.addProductRow(product.name, product.id, 2);
    await expect(tr.itemPrice(product.id)).toHaveValue('250.50');
    await expect(tr.itemTotal(product.id)).toHaveText('501.00');
    await expect(tr.amount).toHaveText('501.00');
    await tr.itemPrice(product.id).fill('300,25'); // запятая → точка
    await expect(tr.itemPrice(product.id)).toHaveValue('300.25');
    await expect(tr.itemTotal(product.id)).toHaveText('600.50');
    await expect(tr.amount).toHaveText('600.50');

    // негатив: отрицательная цена → 400, ошибка под полем цены, документ не создан.
    allowHttpErrors((url, status) => url.endsWith('/api/v1/transfers') && status === 400);
    await tr.itemPrice(product.id).fill('-5');
    await tr.send.click();
    await expect(tr.itemPriceError(product.id)).toHaveText('Цена не может быть отрицательной');
    await expect(tr.itemPrice(product.id)).toHaveClass(/is-invalid/);
    await expect(tr.cardTitle).toHaveText('Новое перемещение');
    // негатив: не число — ошибка под полем без запроса; три знака после запятой — от сервера.
    await tr.itemPrice(product.id).fill('abc');
    await tr.send.click();
    await expect(tr.itemPriceError(product.id)).toHaveText('Некорректная цена');
    await tr.itemPrice(product.id).fill('1.999');
    await tr.send.click();
    await expect(tr.itemPriceError(product.id)).toHaveText('Некорректная цена');
    await expect(tr.cardTitle).toHaveText('Новое перемещение');
    expect((await adminApi.getCard(task.id)).transfers.filter((t) => t.direction === 'warehouse_to_client')).toHaveLength(0);
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(40);

    // «Отправить»: «В пути», цена и суммы — значения сервера; товар списан со склада.
    await tr.itemPrice(product.id).fill('300.25');
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    const id = await tr.currentId();
    await expect(tr.amount).toHaveText('600.50');
    await expect(tr.viewItem('product', product.id).getByTestId('st-view-price')).toHaveText('300.25');
    await expect(tr.viewItem('product', product.id).getByTestId('st-view-total')).toHaveText('600.50');
    await expect(tr.viewItem('equipment', unit.id).getByTestId('st-view-price')).toHaveText('');
    await expect(tr.rowAmount(id)).toHaveText('600.50');
    await expect(tr.warnings).toHaveCount(0); // остатка хватает
    const sent = await adminApi.getTransfer(id);
    expect(sent).toMatchObject({ direction: 'warehouse_to_client', status: 'in_transit', task_id: task.id });
    expect(Number(sent.amount)).toBe(600.5);
    const line = sent.items.find((it) => it.kind === 'product')!;
    expect(Number(line.price)).toBe(300.25);
    expect(Number(line.line_total)).toBe(600.5);
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(38);

    // «Остатки»: остаток на «Алматы» уменьшился; проданное клиенту «в пути» не числится.
    const stock = new StockPage(page);
    await stock.goto();
    await stock.chooseScope('Все товары');
    await stock.q.fill(product.name);
    await expect(stock.cell(product.id, 'Алматы')).toContainText('38');
    await expect(stock.row(product.id).getByTestId('stock-cell-transit')).toHaveText('0');

    // карточка заказа: накладная «в пути» уже в итоге.
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.transfersAmount).toHaveText('600.50');
    await expect(card.total).toHaveText('Итог заказа: 1600.50');
    await expect(card.transferAmount(id)).toHaveText('600.50');
    await expect(card.transferStatus(id)).toHaveText('В пути');

    // «Принять»: единица «у клиента», сумма остаётся в итоге, остаток не меняется.
    await card.transferLink(id).click();
    await expect(tr.cardTitle).toHaveText(`Перемещение #${id}`);
    await tr.confirmReceive('Клиент получил оборудование?');
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.amount).toHaveText('600.50');
    await expect(tr.factHead).toHaveCount(0); // факт по товару накладной не вводится
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(38);

    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.equipmentLocation(unit.id)).toHaveText('у клиента');
    await expect(card.transferStatus(id)).toHaveText('Получено');
    await expect(card.transfersAmount).toHaveText('600.50');
    await expect(card.total).toHaveText('Итог заказа: 1600.50');
    const paidCard = await adminApi.getCard(task.id);
    expect(Number(paidCard.task.transfers_amount)).toBe(600.5);
    expect(Number(paidCard.task.total_amount)).toBe(1600.5);
    expect(paidCard.task.is_paid).toBe(false); // отметка «Оплачен» у заказа одна — накладная её не трогает

    // вторая накладная того же заказа — заявка на количество больше остатка: в итог не входит;
    // «Отправить» → предупреждение об отрицательном остатке; отмена «в пути» → сумма ушла из
    // итога, остаток вернулся.
    const second = await adminApi.createTransfer({
      direction: 'warehouse_to_client', from_warehouse_id: almaty.id, task_id: task.id,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 50, price: 10 }],
    } satisfies NewTransfer);
    expect(Number(second.amount)).toBe(500);
    expect(Number((await adminApi.getCard(task.id)).task.transfers_amount)).toBe(600.5);
    await tr.goto(second.id);
    await expect(tr.itemPrice(product.id)).toHaveValue('10.00');
    await expect(tr.amount).toHaveText('500.00');
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.warnings).toContainText(product.name);
    await expect(tr.warnings).toContainText('отрицательный');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(-12);
    expect(Number((await adminApi.getCard(task.id)).task.total_amount)).toBe(2100.5);

    await tr.cancel.click();
    await page.getByTestId('ui-dialog-ok').click();
    await expect(tr.cardStatus).toHaveText('Отменён');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(38);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.transferStatus(second.id)).toHaveText('Отменён');
    await expect(card.transfersAmount).toHaveText('600.50');
    await expect(card.total).toHaveText('Итог заказа: 1600.50');
  });

  test('TR-018 (TS-030): отмена единственной накладной «в пути» — строка «Продано по накладным» исчезает из карточки заказа, остаток возвращается; заявка в итог не входит', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, client, task } = await orderAtAlmaty(adminApi, uniq, 'TR018');
    const product = await pricedProduct(adminApi, uniq, 'TR-018', 15, 10);
    // цена не передана — берётся из каталога (DS-039).
    const doc = await adminApi.createTransfer({
      direction: 'warehouse_to_client', from_warehouse_id: almaty.id, task_id: task.id,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 2 }],
    });
    expect(Number(doc.items[0].price)).toBe(15);
    expect(Number(doc.amount)).toBe(30);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const tr = new TransfersPage(page);
    // заявка: сумма у документа есть, в итоге заказа её нет.
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.transferAmount(doc.id)).toHaveText('30.00');
    await expect(card.transfersAmount).toHaveCount(0);
    await expect(card.total).toHaveText('Итог заказа: 0.00');

    await adminApi.transferAction(doc.id, 'send');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(8);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.transfersAmount).toHaveText('30.00');
    await expect(card.total).toHaveText('Итог заказа: 30.00');

    // отмена «в пути» из документа.
    await card.transferLink(doc.id).click();
    await expect(tr.cardStatus).toHaveText('В пути');
    await tr.cancel.click();
    await page.getByTestId('ui-dialog-ok').click();
    await expect(tr.cardStatus).toHaveText('Отменён');
    await expect(tr.amount).toHaveText('30.00'); // сумма документа остаётся, из итога её убирает статус
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(10);

    await tr.taskLink.click();
    await card.waitOpen();
    await expect(card.transferStatus(doc.id)).toHaveText('Отменён');
    await expect(card.transfersAmount).toHaveCount(0);
    await expect(card.total).toHaveText('Итог заказа: 0.00');
  });

  test('TR-019 (TS-030): исполнитель оформляет накладную — «+ Товар» доступен, цена из каталога правится, «Сохранить» → «Отправить»', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
  }) => {
    const exec = await makeUser({ login: uniq('tr019-exec'), name: uniq('TR-019 Исполнитель') });
    const { almaty, task, units } = await orderAtAlmaty(adminApi, uniq, 'TR019', { units: 1, assigneeId: exec.id });
    const product = await pricedProduct(adminApi, uniq, 'TR-019', 99.9, 5);

    await loginAs(page, exec.login, exec.password);
    const tr = new TransfersPage(page);
    await tr.gotoNew('warehouse_to_client', task.id);
    await expect(tr.itemRow('equipment', units[0].id)).toBeVisible();
    await tr.addProductRow(product.name, product.id, 1);
    await expect(tr.itemPrice(product.id)).toHaveValue('99.90');
    await tr.itemPrice(product.id).fill('120');
    await expect(tr.amount).toHaveText('120.00');
    await tr.save.click();
    await expect(tr.toast).toHaveText('Сохранено');
    await expect(tr.cardStatus).toHaveText('Заявка');
    const id = await tr.currentId();
    await expect(tr.itemPrice(product.id)).toHaveValue('120.00'); // значение сервера
    await expect(tr.amount).toHaveText('120.00');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(5); // заявка остаток не двигает

    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.rowAmount(id)).toHaveText('120.00');
    const sent = await adminApi.getTransfer(id);
    expect(Number(sent.amount)).toBe(120);
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(4);
  });

  test('TR-021 (TS-014, TS-030): сумма строки при дробном количестве округляется как на сервере — 0.15 × 1.5 → 0.23, 0.10 × 0.35 → 0.04: превью в форме, сохранённый документ и «Продано по накладным» совпадают; пустая цена считается по цене каталога', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { client, task } = await orderAtAlmaty(adminApi, uniq, 'TR021');
    const a = await pricedProduct(adminApi, uniq, 'TR-021 a', 0.15, 10);
    const b = await pricedProduct(adminApi, uniq, 'TR-021 b', 0.1, 10);

    const tr = new TransfersPage(page);
    await tr.gotoNew('warehouse_to_client', task.id);
    await tr.choose(tr.from, 'Алматы'); // оборудования на складе нет — склад отправления выбирается вручную

    // превью в форме — те же копейки, что посчитает сервер (float дал бы 0.22 и 0.03).
    await tr.addProductRow(a.name, a.id, 1.5);
    await expect(tr.itemPrice(a.id)).toHaveValue('0.15');
    await expect(tr.itemTotal(a.id)).toHaveText('0.23');
    await tr.addProductRow(b.name, b.id, 0.35);
    await expect(tr.itemPrice(b.id)).toHaveValue('0.10');
    await expect(tr.itemTotal(b.id)).toHaveText('0.04');
    await expect(tr.amount).toHaveText('0.27');

    // пустая цена: превью считает по цене каталога (её подставит сервер), она же — плейсхолдер.
    await tr.itemPrice(a.id).fill('');
    await expect(tr.itemPrice(a.id)).toHaveAttribute('placeholder', '0.15');
    await expect(tr.itemTotal(a.id)).toHaveText('0.23');
    await expect(tr.amount).toHaveText('0.27');

    await tr.save.click();
    await expect(tr.toast).toHaveText('Сохранено');
    const id = await tr.currentId();
    await expect(tr.itemPrice(a.id)).toHaveValue('0.15'); // сервер подставил цену каталога
    await expect(tr.itemTotal(a.id)).toHaveText('0.23');
    await expect(tr.itemTotal(b.id)).toHaveText('0.04');
    await expect(tr.amount).toHaveText('0.27');
    const saved = await adminApi.getTransfer(id);
    expect(saved.items.map((it) => Number(it.line_total))).toEqual([0.23, 0.04]);
    expect(Number(saved.amount)).toBe(0.27);

    // сохранённая заявка открывается с суммами сервера.
    await tr.goto(id);
    await expect(tr.itemTotal(a.id)).toHaveText('0.23');
    await expect(tr.itemTotal(b.id)).toHaveText('0.04');
    await expect(tr.amount).toHaveText('0.27');
    await expect(tr.rowAmount(id)).toHaveText('0.27');

    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.viewItem('product', a.id).getByTestId('st-view-total')).toHaveText('0.23');
    await expect(tr.viewItem('product', b.id).getByTestId('st-view-total')).toHaveText('0.04');
    await expect(tr.amount).toHaveText('0.27');

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.transferAmount(id)).toHaveText('0.27');
    await expect(card.transfersAmount).toHaveText('0.27');
    await expect(card.total).toHaveText('Итог заказа: 0.27');
  });

  test('TR-029 (TS-032): документ заказа — склад, примечание и товар, введённые ДО прихода карточки заказа, сохраняются: форма не перерисовывается, после ответа появляются оборудование заказа и предвыбор; выбор из списка, открытого до ответа, срабатывает; негатив: до ответа оборудования заказа нет, склад пользователя предвыбор не заменяет', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, task, units } = await orderAtAlmaty(adminApi, uniq, 'TR029', { units: 1 });
    const unit = units[0];
    const a = await pricedProduct(adminApi, uniq, 'TR-029 a', 10, 10);
    const b = await pricedProduct(adminApi, uniq, 'TR-029 b', 20, 10);
    const cardUrl = `/api/v1/tasks/${task.id}/card`;

    // ответ карточки заказа формы задерживается, пока тест не отпустит его.
    let release!: () => void;
    let gate = new Promise<void>((resolve) => { release = resolve; });
    // Держатся только запросы новой формы: сохранённая заявка перечитывает карточку заказа сама.
    let holding = true;
    let held = 0;
    await page.route(`**${cardUrl}`, async (route) => {
      if (holding) {
        held += 1;
        await gate;
      }
      await route.continue();
    });
    const releaseCard = async () => {
      const loaded = page.waitForResponse((r) => r.url().includes(cardUrl) && r.status() === 200);
      holding = false;
      release();
      await loaded;
      await expect(tr.taskInfo).toContainText(task.title);
    };

    const tr = new TransfersPage(page);
    await tr.gotoNew('warehouse_to_client', task.id);
    await expect.poll(() => held, { message: 'форма запросила карточку заказа' }).toBe(1);

    // пользователь заполняет форму, пока карточка заказа ещё не пришла.
    await tr.choose(tr.from, 'Алматы');
    await tr.note.fill('TR-029 примечание');
    await tr.addProductRow(a.name, a.id, 2);
    // негатив: карточки заказа ещё нет — ни оборудования заказа, ни предвыбранной единицы.
    await expect(tr.taskEquipmentOptions).toHaveCount(0);
    await expect(tr.itemRow('equipment', unit.id)).toHaveCount(0);
    // список «+ Товар» открыт до прихода ответа.
    await tr.addProduct.click();
    await expect(tr.dropdown).toBeVisible();

    await releaseCard();

    // после ответа: оборудование заказа и предвыбор пришли, а введённое не сброшено.
    await expect(tr.itemRow('equipment', unit.id)).toBeVisible();
    await expect(tr.taskEquipmentOption(unit.id)).toBeDisabled(); // уже в документе
    await expect(tr.from).toContainText('Алматы');
    await expect(tr.note).toHaveValue('TR-029 примечание');
    await expect(tr.itemQty(a.id)).toHaveValue('2');
    // выбор из списка, открытого до ответа, попадает в живую форму.
    await tr.dropdown.locator('.cb-opt', { hasText: b.name }).first().click();
    await expect(tr.itemRow('product', b.id)).toBeVisible();
    await tr.itemQty(b.id).fill('1');

    // форма сохраняется со всем введённым и предвыбранным.
    await tr.save.click();
    await expect(tr.cardStatus).toHaveText('Заявка');
    const id = await tr.currentId();
    const saved = await adminApi.getTransfer(id);
    expect(saved).toMatchObject({ direction: 'warehouse_to_client', status: 'requested', from_warehouse_id: almaty.id, note: 'TR-029 примечание' });
    expect(saved.items.filter((i) => i.kind === 'product').map((i) => i.product_id).sort()).toEqual([a.id, b.id].sort());
    expect(saved.items.filter((i) => i.kind === 'equipment').map((i) => i.equipment_id)).toEqual([unit.id]);
    await adminApi.removeTransferForCleanup(id); // единица снова свободна для второй формы

    // негатив: склад, выбранный пользователем, предвыбор не заменяет — единица с другого склада
    // в документ не попадает.
    await page.goto('about:blank'); // запросы сохранённой заявки завершены — дальше только вторая форма
    gate = new Promise<void>((resolve) => { release = resolve; });
    holding = true;
    held = 0;
    await tr.gotoNew('warehouse_to_client', task.id);
    await expect.poll(() => held, { message: 'вторая форма запросила карточку заказа' }).toBe(1);
    await tr.choose(tr.from, 'Москва');
    await releaseCard();
    await expect(tr.taskEquipmentOption(unit.id)).toBeDisabled(); // единица числится на «Алматы»
    await expect(tr.from).toContainText('Москва');
    await expect(tr.itemRow('equipment', unit.id)).toHaveCount(0);
  });

  test('TR-020 (TS-030, TS-045, TS-046): накладная с колонками цены и итоги карточки заказа — 375×812 без горизонтальной прокрутки, суммы читаемы в светлой и тёмной теме', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, client, task, units } = await orderAtAlmaty(adminApi, uniq, 'TR020', { units: 1, servicePrice: 500 });
    const product = await pricedProduct(adminApi, uniq, 'TR-020 товар с длинным названием для узкого экрана', 12345.67, 20);
    const other = await pricedProduct(adminApi, uniq, 'TR-020 b', 0.5, 20);
    const mk = (send: boolean, withUnit: boolean) =>
      adminApi.createTransfer({
        direction: 'warehouse_to_client', from_warehouse_id: almaty.id, task_id: task.id, send,
        items: [
          ...(withUnit ? [{ kind: 'equipment' as const, equipment_id: units[0].id }] : []),
          { kind: 'product', product_id: product.id, qty_sent: 3, price: 12345.67 },
          { kind: 'product', product_id: other.id, qty_sent: 1.5 },
        ],
      });
    const inTransit = await mk(true, false);
    const requested = await mk(false, true);
    expect(Number(inTransit.amount)).toBe(37037.76); // 37037.01 + 0.75

    await page.setViewportSize({ width: 375, height: 812 });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const tr = new TransfersPage(page);
    for (const theme of THEMES) {
      // последний зарегистрированный init-скрипт задаёт тему (эталон — polish.spec.ts).
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      const where = `${theme} 375×812`;

      // карточка заказа: суммы в блоке «Перемещения заказа» и строка «Продано по накладным».
      await board.goto(`?client_id=${client.id}&task=${task.id}`);
      await card.waitOpen();
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      await card.total.scrollIntoViewIfNeeded();
      await expect(card.transfersAmount).toHaveText('37037.76');
      await expect(card.total).toHaveText('Итог заказа: 37537.76');
      await expect(card.transferAmount(inTransit.id)).toHaveText('37037.76');
      await expect(card.transferAmount(requested.id)).toHaveText('37037.76');
      await card.expectNoHorizontalScroll();
      expect(await contrastOf(card.transfersAmount), `карточка ${where}: «Продано по накладным»`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(card.total), `карточка ${where}: итог заказа`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(card.transferAmount(inTransit.id)), `карточка ${where}: сумма накладной`).toBeGreaterThanOrEqual(4.5);

      // заявка: редактор с колонками «Цена» / «Сумма».
      await tr.goto(requested.id);
      await expect(tr.cardStatus).toHaveText('Заявка');
      await expect(tr.itemPrice(product.id)).toHaveValue('12345.67');
      await expect(tr.itemTotal(product.id)).toHaveText('37037.01');
      await expect(tr.amount).toHaveText('37037.76');
      await tr.expectNoHorizontalScroll();
      expect(await contrastOf(tr.amount), `заявка ${where}: «Сумма накладной»`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.itemTotal(product.id)), `заявка ${where}: сумма строки`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.itemPrice(product.id)), `заявка ${where}: поле цены`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.priceHead), `заявка ${where}: заголовок «Цена»`).toBeGreaterThanOrEqual(4.5);

      // накладная «в пути»: просмотр и строка списка с суммой.
      await tr.goto(inTransit.id);
      await expect(tr.cardStatus).toHaveText('В пути');
      await expect(tr.viewItem('product', product.id).getByTestId('st-view-total')).toHaveText('37037.01');
      await expect(tr.amount).toHaveText('37037.76');
      await tr.expectNoHorizontalScroll();
      expect(await contrastOf(tr.amount), `документ ${where}: «Сумма накладной»`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.viewItem('product', product.id).getByTestId('st-view-price')), `документ ${where}: цена`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.viewItem('product', product.id).getByTestId('st-view-total')), `документ ${where}: сумма строки`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.rowAmount(inTransit.id)), `список ${where}: сумма накладной`).toBeGreaterThanOrEqual(4.5);
    }
  });
});
