import { test, expect } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { uniqPlate } from '../pages/NewOrderDialog';

// Диалог «Новый заказ» — связь госномер ↔ клиент в обе стороны и автодополнение (NO-004…NO-009):
// исполнитель заводит нового клиента; клиент выбран первым — его автомобиль подставляется
// (один) либо выбирается (несколько); смена номера переключает владельца; подсказки по
// фрагменту госномера (GET /api/v1/equipment?q=) и поиск клиента по фрагменту имени.
test.use({ colorScheme: 'light' });

async function createdTaskId(card: OrderCardPage): Promise<number> {
  await expect(card.title).toContainText('Заказ #');
  const m = /#(\d+)/.exec((await card.title.textContent()) ?? '');
  expect(m, 'номер заказа в заголовке карточки').toBeTruthy();
  return Number(m![1]);
}

test.describe('NO: госномер ↔ клиент, автодополнение', () => {
  test('NO-004: исполнитель оформляет заказ на нового клиента и новый автомобиль', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    _boardCleanup,
  }) => {
    const carType = await adminApi.createEquipmentType({ name: uniq('NO-004 легковой') });
    const login = uniq('exec-no004');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);
    const plate = uniqPlate();
    const clientName = uniq('NO-004 Орлова Е. В.');

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto();
    await board.openTaskAdd();

    await dlg.fillNewCar(plate, { type: carType.name, brand: 'Renault', model: 'Duster' });
    // клиент не выбран — «Новый клиент» открыт сразу и исполнителю.
    await expect(dlg.newClientToggle).toBeChecked();
    await expect(dlg.newClientName).toBeVisible();
    await dlg.newClientName.fill(clientName);
    await dlg.newClientPhone.fill('+7 913 111-22-33');

    await board.taskSave.click();
    const id = await createdTaskId(card);
    _boardCleanup(id);
    await expect(dlg.titleField).toHaveValue(`Renault Duster ${plate}`);

    const task = await adminApi.getTask(id);
    const client = await adminApi.getClient(task.client_id);
    expect(client.company_name).toBe(clientName);
    expect(client.phones.map((p) => p.phone)).toEqual(['+7 913 111-22-33']);
    const eq = (await adminApi.getCard(id)).equipment;
    expect(eq).toHaveLength(1);
    expect(eq[0]).toMatchObject({ plate_number: plate, model: 'Duster' });
    await adminApi.purgeClient(task.client_id, [id]);
  });

  test('NO-005: клиент выбран первым по фрагменту имени — единственный автомобиль подставляет госномер', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const name = uniq('NO-005 Смирнова А. П.');
    const client = await adminApi.createClient({ company_name: name, phones: [{ phone: '9990000805', note: '' }] });
    const carType = await adminApi.createEquipmentType({ name: uniq('NO-005 легковой') });
    const plate = uniqPlate();
    const car = await adminApi.createEquipment(client.id, { type_id: carType.id, plate_number: plate, brand: 'Mazda', model: '3' });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskAdd();

    await dlg.searchClient(name.slice(-22), name);
    await expect(dlg.plate).toHaveValue(plate);
    await expect(dlg.carFound).toContainText(`Mazda 3 · ${plate} · ${name}`);
    await expect(dlg.clientLocked).toBeVisible();
    await expect(dlg.client).toBeDisabled();
    await expect(dlg.newCar).toBeHidden();

    await board.taskSave.click();
    const id = await createdTaskId(card);
    _boardCleanup(id);
    expect((await adminApi.getTask(id)).client_id).toBe(client.id);
    expect((await adminApi.getCard(id)).equipment.map((e) => e.equipment_id)).toEqual([car.id]);
  });

  test('NO-006: у клиента несколько автомобилей — выбор из списка заполняет госномер', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const name = uniq('NO-006 Волков И. И.');
    const client = await adminApi.createClient({ company_name: name, phones: [{ phone: '9990000806', note: '' }] });
    const carType = await adminApi.createEquipmentType({ name: uniq('NO-006 легковой') });
    const plate1 = uniqPlate();
    const plate2 = uniqPlate() + 'B';
    await adminApi.createEquipment(client.id, { type_id: carType.id, plate_number: plate1, brand: 'Toyota', model: 'Camry' });
    const car2 = await adminApi.createEquipment(client.id, { type_id: carType.id, plate_number: plate2, brand: 'Toyota', model: 'RAV4' });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskAdd();

    await dlg.searchClient(name.slice(-22), name);
    await expect(dlg.carOptions).toHaveCount(2);
    await expect(dlg.plateStatus).toContainText('Автомобилей у клиента: 2');
    await expect(dlg.plate).toHaveValue('');
    await page.getByRole('radio', { name: new RegExp(`RAV4 · ${plate2}`) }).check();
    await expect(dlg.plate).toHaveValue(plate2);
    await expect(dlg.clientLocked).toBeVisible();
    await expect(dlg.client).toContainText(name);

    await board.taskSave.click();
    const id = await createdTaskId(card);
    _boardCleanup(id);
    expect((await adminApi.getTask(id)).client_id).toBe(client.id);
    expect((await adminApi.getCard(id)).equipment.map((e) => e.equipment_id)).toEqual([car2.id]);
  });

  test('NO-007: смена госномера переключает владельца, стирание — снимает блокировку; «+ Новый клиент» из списка', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const nameA = uniq('NO-007 Первый Владелец');
    const nameB = uniq('NO-007 Второй Владелец');
    const a = await adminApi.createClient({ company_name: nameA, phones: [{ phone: '9990000807', note: '' }] });
    const b = await adminApi.createClient({ company_name: nameB, phones: [{ phone: '9990000808', note: '' }] });
    const carType = await adminApi.createEquipmentType({ name: uniq('NO-007 легковой') });
    const plateA = uniqPlate();
    const plateB = uniqPlate() + 'C';
    await adminApi.createEquipment(a.id, { type_id: carType.id, plate_number: plateA, brand: 'Ford', model: 'Focus' });
    await adminApi.createEquipment(b.id, { type_id: carType.id, plate_number: plateB, brand: 'Opel', model: 'Astra' });

    const board = new BoardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${a.id}`);
    await board.openTaskAdd();

    await dlg.useFoundCar(plateA);
    await expect(dlg.client).toContainText(nameA);
    await expect(dlg.client).toBeDisabled();

    // правка номера — прежняя сводка и блокировка снимаются сразу, до ответа поиска.
    await dlg.plate.focus();
    await dlg.plate.press('End');
    await dlg.plate.press('Backspace');
    await expect(dlg.carFound).toBeHidden();
    await expect(dlg.clientLocked).toBeHidden();
    await expect(dlg.client).toBeEnabled();

    // номер другого владельца — клиент переключается на него.
    await dlg.useFoundCar(plateB);
    await expect(dlg.carFound).toContainText(`Opel Astra · ${plateB} · ${nameB}`);
    await expect(dlg.client).toContainText(nameB);
    await expect(dlg.client).toBeDisabled();

    // номер стёрт — клиент разблокирован и сброшен (он был владельцем автомобиля).
    await dlg.plate.fill('');
    await expect(dlg.client).toBeEnabled();
    await expect(dlg.client).not.toContainText(nameB);
    await expect(dlg.carFound).toBeHidden();

    // «+ Новый клиент» в конце списка клиентов — текст поиска становится ФИО.
    const fresh = uniq('NO-007 Никого Такого');
    await dlg.client.click();
    await expect(dlg.dropdown).toBeVisible();
    await dlg.dropdown.locator('.cb-search').fill(fresh);
    await expect(dlg.dropdown).toContainText('Ничего не найдено');
    await dlg.clientNewAction.click();
    await expect(dlg.newClientToggle).toBeChecked();
    await expect(dlg.newClientName).toHaveValue(fresh);
  });

  test('NO-008: подсказки по фрагменту госномера — выбор мышью задаёт автомобиль и владельца', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const name = uniq('NO-008 Кузьмина О. О.');
    const client = await adminApi.createClient({ company_name: name, phones: [{ phone: '9990000809', note: '' }] });
    const carType = await adminApi.createEquipmentType({ name: uniq('NO-008 легковой') });
    const plate = uniqPlate();
    const car = await adminApi.createEquipment(client.id, { type_id: carType.id, plate_number: plate, brand: 'Nissan', model: 'Qashqai' });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskAdd();

    // фрагмент из середины номера, в нижнем регистре и с дефисом.
    const fragment = `${plate.slice(2, 5)}-${plate.slice(5, 9)}`.toLowerCase();
    await dlg.plate.click();
    await dlg.plate.pressSequentially(fragment, { delay: 20 });
    await expect(dlg.plateSuggestions).toBeVisible();
    const option = dlg.plateSuggestion.filter({ hasText: plate });
    await expect(option).toContainText(`${plate} · Nissan Qashqai · ${name}`);
    await expect(dlg.plate).toHaveAttribute('role', 'combobox');
    await expect(dlg.plate).toHaveAttribute('aria-expanded', 'true');
    await option.click();

    await expect(dlg.plateSuggestions).toBeHidden();
    await expect(dlg.plate).toHaveValue(plate);
    await expect(dlg.carFound).toContainText(`Nissan Qashqai · ${plate} · ${name}`);
    await expect(dlg.client).toContainText(name);
    await expect(dlg.client).toBeDisabled();

    await board.taskSave.click();
    const id = await createdTaskId(card);
    _boardCleanup(id);
    expect((await adminApi.getTask(id)).client_id).toBe(client.id);
    expect((await adminApi.getCard(id)).equipment.map((e) => e.equipment_id)).toEqual([car.id]);
  });

  test('NO-009: подсказки госномера с клавиатуры — ↓ и Enter, Esc закрывает только список', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const name = uniq('NO-009 Белов Д. Д.');
    const client = await adminApi.createClient({ company_name: name, phones: [{ phone: '9990000810', note: '' }] });
    const carType = await adminApi.createEquipmentType({ name: uniq('NO-009 легковой') });
    const plate = uniqPlate();
    await adminApi.createEquipment(client.id, { type_id: carType.id, plate_number: plate, brand: 'Skoda', model: 'Rapid' });

    const board = new BoardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskAdd();

    const prefix = plate.slice(0, -1);
    await dlg.plate.click();
    await dlg.plate.pressSequentially(prefix, { delay: 20 });
    await expect(dlg.plateSuggestion.first()).toContainText(plate);
    await dlg.plate.press('Escape');
    await expect(dlg.plateSuggestions).toBeHidden();
    await expect(board.taskForm).toBeVisible(); // Esc не закрыл модалку

    await dlg.plate.press('Backspace');
    await expect(dlg.plateSuggestion.first()).toContainText(plate);
    await dlg.plate.press('ArrowDown');
    await expect(dlg.plate).toHaveAttribute('aria-activedescendant', /.+/);
    await dlg.plate.press('Enter');
    await expect(dlg.plate).toHaveValue(plate);
    await expect(dlg.carFound).toContainText(`Skoda Rapid · ${plate} · ${name}`);
    await expect(dlg.client).toBeDisabled();
  });
});
