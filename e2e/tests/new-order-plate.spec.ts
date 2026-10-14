import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { uniqPlate } from '../pages/NewOrderDialog';

// Диалог «Новый заказ» по госномеру (NO-001…NO-003): первое поле — госномер; найденный
// автомобиль подставляет и блокирует клиента; несколько совпадений — выбор; не найден —
// «Новый автомобиль» (+ «Новый клиент»). Сохранение — один атомарный POST /api/v1/tasks,
// название заказа генерирует сервер «{Марка} {Модель} {ГОСНОМЕР}».
test.use({ colorScheme: 'light' });

async function createdTaskId(card: OrderCardPage): Promise<number> {
  await expect(card.title).toContainText('Заказ #');
  const m = /#(\d+)/.exec((await card.title.textContent()) ?? '');
  expect(m, 'номер заказа в заголовке карточки').toBeTruthy();
  return Number(m![1]);
}

test.describe('NO: новый заказ по госномеру', () => {
  test('NO-001: найденный автомобиль — клиент подставлен и заблокирован, колонка из «+ Добавить заказ»', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const client = await adminApi.createClient({ company_name: uniq('NO-001 Петрова Н. С.'), phones: [{ phone: '9990000801', note: '' }] });
    const plate = uniqPlate();
    const car = await adminApi.createEquipment(client.id, { plate_number: plate });
    const columns = (await adminApi.listColumns()).items;
    const col = columnByName(columns, 'Диагностика');

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${client.id}`);

    // «+ Добавить заказ» внизу колонки — тот же диалог, колонка выбрана.
    await board.columnAddTaskBtn(col.id).click();
    await expect(board.taskForm).toBeVisible();
    await expect(board.taskColumn).toContainText('Диагностика');
    await expect(dlg.plate).toBeVisible();
    await expect(dlg.titleField).toBeHidden();

    // ввод в нижнем регистре, с пробелом и дефисом — поиск без учёта регистра/пробелов/дефисов.
    const typed = `${plate.slice(0, 3).toLowerCase()} -${plate.slice(3).toLowerCase()}`;
    await dlg.useFoundCar(typed);
    await expect(dlg.plate).toHaveValue(typed.toUpperCase());
    await expect(dlg.carFound).toContainText(`${plate} · ${client.company_name}`);
    await expect(dlg.clientLocked).toBeVisible();
    await expect(dlg.client).toBeDisabled();
    await expect(dlg.client).toContainText(client.company_name);
    await expect(dlg.newCar).toBeHidden();

    await board.taskSave.click();
    const id = await createdTaskId(card);
    _boardCleanup(id);

    // карточка созданного заказа: название — редактируемое поле, автомобиль уже в заказе.
    await expect(dlg.plate).toBeHidden();
    await expect(dlg.titleField).toHaveValue(plate);
    await expect(card.equipmentRows).toHaveCount(1);
    await expect(card.equipmentRow(car.id)).toBeVisible();

    const task = await adminApi.getTask(id);
    expect(task.client_id).toBe(client.id);
    expect(task.column_id).toBe(col.id);
    expect(task.title).toBe(plate);
  });

  test('NO-002: один госномер у нескольких клиентов — выбор автомобиля задаёт клиента', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const first = await adminApi.createClient({ company_name: uniq('NO-002 Первый'), phones: [{ phone: '9990000802', note: '' }] });
    const second = await adminApi.createClient({ company_name: uniq('NO-002 Второй'), phones: [{ phone: '9990000803', note: '' }] });
    const plate = uniqPlate();
    await adminApi.createEquipment(first.id, { plate_number: plate });
    const resold = await adminApi.createEquipment(second.id, { plate_number: plate });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${second.id}`);
    await board.openTaskAdd();

    await dlg.fillPlate(plate);
    await expect(dlg.carOptions).toHaveCount(2);
    await expect(dlg.clientLocked).toBeHidden();
    await page.getByRole('radio', { name: new RegExp(second.company_name) }).check();
    await expect(dlg.clientLocked).toBeVisible();
    await expect(dlg.client).toContainText(second.company_name);

    await board.taskSave.click();
    const id = await createdTaskId(card);
    _boardCleanup(id);
    const task = await adminApi.getTask(id);
    expect(task.client_id).toBe(second.id);
    expect((await adminApi.getCard(id)).equipment.map((e) => e.equipment_id)).toEqual([resold.id]);
  });

  test('NO-003: не найден — новый автомобиль и новый клиент одним сохранением', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const anchor = await adminApi.createClient({ company_name: uniq('NO-003 якорь доски'), phones: [{ phone: '9990000804', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const plate = uniqPlate();
    const clientName = uniq('NO-003 Кузнецов Пётр');

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${anchor.id}`);
    await board.openTaskAdd();

    await dlg.fillNewCar(plate.toLowerCase());
    // «Новый автомобиль» — только госномер: полей марки, модели, VIN нет.
    await expect(dlg.newCar.locator('input')).toHaveCount(0);
    await expect(dlg.plate).toHaveValue(plate);
    await expect(dlg.carFound).toBeHidden();
    // клиент не выбран — поля «Новый клиент» открыты сразу, без флажка.
    await expect(dlg.newClientToggle).toBeChecked();
    await expect(dlg.newClientName).toBeVisible();
    await dlg.fillNewClient(clientName, '+7 912 000-11-22');
    await board.fillTask({ column: columnByName(columns, 'Приёмка').name });

    await board.taskSave.click();
    const id = await createdTaskId(card);
    _boardCleanup(id);
    await expect(dlg.titleField).toHaveValue(plate);
    await expect(card.equipmentRows).toHaveCount(1);

    const task = await adminApi.getTask(id);
    const client = await adminApi.getClient(task.client_id);
    expect(client.company_name).toBe(clientName);
    expect(client.phones.map((p) => p.phone)).toEqual(['+7 912 000-11-22']);
    const eq = (await adminApi.getCard(id)).equipment;
    expect(eq).toHaveLength(1);
    expect(eq[0]).toMatchObject({ plate_number: plate });

    // клиент заведён диалогом (мимо adminApi) — уборка вручную: заказ, затем клиент с автомобилем.
    await adminApi.purgeClient(task.client_id, [id]);
  });
});
