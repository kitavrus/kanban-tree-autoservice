import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { uniqPlate } from '../pages/NewOrderDialog';

// Фаза 6: карточка заказа `/` — task-card.js (design-tests.md §E2E OC-001…023, эта фаза
// покрывает OC-001, OC-003…OC-013, OC-015, OC-016, OC-019…OC-023; OC-002/OC-014 — фаза 06
// доски, OC-017/018 — order-print.spec.ts). Данные — только API-фикстуры (ADR-101), без pg.
test.use({ colorScheme: 'light' });

async function setup(adminApi: import('../fixtures/auth').AdminApi, uniq: (s: string) => string, label: string) {
  const client = await adminApi.createClient({ company_name: uniq(`OC клиент ${label}`), phones: [{ phone: '9990000100', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  return { client, columns };
}

async function setupCatalog(adminApi: import('../fixtures/auth').AdminApi, uniq: (s: string) => string, label: string) {
  const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq(`OC категория услуг ${label}`) });
  return { svcCat };
}

test.describe('OC: новый заказ — карточка вместо простой формы', () => {
  test('OC-001: разделы неактивны до сохранения, после — активны', async ({ adminPage: page, adminApi, uniq, _boardCleanup }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-001');
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.openTaskAdd();
    // one-car-order: у нового заказа блока «Автомобиль» нет — автомобиль задаётся госномером.
    await expect(card.carSection).toBeHidden();
    await expect(card.subtasksLocked).toBeVisible();

    const plate = uniqPlate();
    await adminApi.createEquipment(client.id, { plate_number: plate });
    await card.newOrder.useFoundCar(plate);
    await card.fillHeader({ column: columnByName(columns, 'Приёмка').name });
    await card.save();

    await expect(card.modal).toBeVisible();
    await expect(card.title).toContainText('#');
    // заказ создан формой, мимо adminApi.createTask — на уборку вручную (номер — из заголовка).
    const created = /#(\d+)/.exec((await card.title.textContent()) ?? '');
    expect(created, 'номер заказа в заголовке карточки').toBeTruthy();
    _boardCleanup(Number(created![1]));
    await expect(card.carSection).toBeVisible();
    await expect(card.carPlate).toHaveText(plate);
    await expect(card.subtasksLocked).toHaveCount(0);
  });
});

test.describe('OC: deep-link /?task=N', () => {
  test('OC-002: открытие по ссылке, закрытие убирает task из URL, несуществующий — «Заказ не найден»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-002');
    const task = await adminApi.createTask({ title: uniq('OC-002 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);

    // deep-link несёт и client_id (обычный фильтр), и task (адрес открытой карточки) —
    // закрытие должно убрать ТОЛЬКО task, сохранив client_id (board.js parseFilters/
    // stripTaskFromUrl).
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.carRow).toBeVisible();
    await expect(card.subtasks).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`[?&]task=${task.id}\\b`));

    await card.close();
    await expect(card.modal).toBeHidden();
    await expect(page).not.toHaveURL(/[?&]task=/);
    await expect(page).toHaveURL(new RegExp(`client_id=${client.id}`));

    allowHttpErrors((url, status) => url.endsWith('/api/v1/tasks/999999/card') && status === 404);
    await board.goto('?task=999999');
    await expect(page.getByTestId('ui-dialog-text')).toContainText('Заказ не найден');
    await page.getByTestId('ui-dialog-ok').click();
    await expect(card.modal).toBeHidden();
    await expect(page).not.toHaveURL(/[?&]task=/);
  });
});

test.describe('OC: шапка заказа', () => {
  test('OC-003: title/описание/срок сохраняются кнопкой и переживают reload', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-003');
    const task = await adminApi.createTask({ title: uniq('OC-003 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    const newTitle = uniq('OC-003 переименован');
    const desc = 'Первый абзац\n\nВторой абзац';
    await card.fillHeader({ title: newTitle, due: '2031-02-20', description: desc });
    await card.save();
    await expect(card.error).toBeHidden();

    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.taskTitle).toHaveValue(newTitle);
    await expect(card.taskDue).toHaveValue('2031-02-20');
    await expect(card.description).toHaveValue(desc);
  });

  test('OC-004: конкурентная правка шапки — вторая вкладка видит «уже изменён», данные первой на месте', async ({
    adminPage: page,
    adminApi,
    uniq,
    context,
    allowHttpErrors,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-004');
    const task = await adminApi.createTask({ title: uniq('OC-004 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}`) && status === 409);

    const boardA = new BoardPage(page);
    const cardA = new OrderCardPage(page);
    await boardA.goto(`?client_id=${client.id}`);
    await boardA.openTaskEdit(task.id);
    await cardA.waitOpen();

    const pageB = await context.newPage();
    const boardB = new BoardPage(pageB);
    const cardB = new OrderCardPage(pageB);
    await boardB.goto(`?client_id=${client.id}`);
    await boardB.openTaskEdit(task.id);
    await cardB.waitOpen();

    const titleA = uniq('OC-004 от A');
    await cardA.fillHeader({ title: titleA });
    await cardA.save();
    await expect(cardA.error).toBeHidden();

    const titleB = uniq('OC-004 от B');
    await cardB.fillHeader({ title: titleB });
    await cardB.save();
    await expect(pageB.getByTestId('ui-dialog-text')).toContainText('уже изменён');
    await pageB.getByTestId('ui-dialog-ok').click();

    await expect.poll(async () => (await adminApi.getTask(task.id)).title).toBe(titleA);
    await pageB.close();
  });
});

test.describe('OC: оплата — только администратор', () => {
  test('OC-005: у executor чекбокс оплаты disabled, сохранение шапки не сбрасывает is_paid', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-005');
    const task = await adminApi.createTask({
      title: uniq('OC-005 заказ'),
      client_id: client.id,
      column_id: columnByName(columns, 'Приёмка').id,
      is_paid: true,
    });
    const login = uniq('exec-oc005');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.chooseCombobox(board.filterAssignee, 'Все исполнители');
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.taskPaid).toBeDisabled();
    await expect(card.taskPaid).toBeChecked();

    await card.fillHeader({ title: uniq('OC-005 переименован') });
    await card.save();
    await expect(card.error).toBeHidden();
    await expect.poll(async () => (await adminApi.getTask(task.id)).is_paid).toBe(true);
  });
});

test.describe('OC: автомобиль заказа (one-car-order)', () => {
  test('OC-006: строка «госномер · владелец», клиент и автомобиль не редактируются, шапка сохраняется без них', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-006');
    const plate = uniqPlate();
    const eq = await adminApi.createEquipment(client.id, { plate_number: plate });
    const task = await adminApi.createCarTask(eq, { title: uniq('OC-006 заказ'), column_id: columnByName(columns, 'Приёмка').id });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('OC-006 категория') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-006 Диагностика'), price: 500 });
    const sub = (await adminApi.createSubtask(task.id, svc.id)).card.subtasks[0];

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    // строка автомобиля: госномер и владелец (клиент заказа), без полей ввода и кнопок смены.
    await expect(card.carRowOf(eq.id)).toBeVisible();
    await expect(card.carPlate).toHaveText(plate);
    await expect(card.carOwner).toHaveText(client.company_name);
    await expect(card.carDeleted).toHaveCount(0);
    await expect(card.carSection.locator('input, select, textarea')).toHaveCount(0);
    await expect(card.carHistoryToggle).toBeVisible();
    // клиент — только для чтения: «Владелец автомобиля».
    await expect(card.taskClient).toBeDisabled();
    await expect(card.taskClient).toContainText(client.company_name);
    await expect(card.clientLockedHint).toBeVisible();
    // выбора автомобиля в услуге нет.
    await expect(card.subtaskRow(sub.id)).toBeVisible();
    await expect(card.subtaskEquipment(sub.id)).toHaveCount(0);

    // правка шапки сохраняется; клиент и автомобиль заказа прежние.
    const newTitle = uniq('OC-006 переименован');
    await card.fillHeader({ title: newTitle });
    await card.save();
    await expect(card.error).toBeHidden();
    await expect.poll(async () => (await adminApi.getTask(task.id)).title).toBe(newTitle);
    const after = await adminApi.getCard(task.id);
    expect(after.task.client_id).toBe(client.id);
    expect(after.car).toMatchObject({ equipment_id: eq.id, plate_number: plate, is_deleted: false });
    await expect(card.carPlate).toHaveText(plate);
    await expect(card.taskClient).toBeDisabled();
  });
});

test.describe('OC: история автомобиля', () => {
  test('OC-008: «История работ» — другие заказы этого автомобиля, текущий не показывается', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-008');
    const eq = await adminApi.createEquipment(client.id, { plate_number: uniqPlate() });
    const col = columnByName(columns, 'Приёмка').id;

    // первый заказ автомобиля — истории других заказов нет.
    const oldTitle = uniq('OC-008 старый заказ');
    const oldTask = await adminApi.createCarTask(eq, { title: oldTitle, column_id: col });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(oldTask.id);
    await card.waitOpen();
    await card.openCarHistory();
    await expect(card.carHistoryPanel.getByTestId('card-car-history-empty')).toBeVisible();
    await card.close();

    // второй заказ того же автомобиля — в истории первый (номер и название), не сам.
    const task = await adminApi.createCarTask(eq, { title: uniq('OC-008 заказ'), column_id: col });
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await card.openCarHistory();
    await expect(card.carHistoryOrder(oldTask.id)).toContainText(oldTitle);
    await expect(card.carHistoryOrder(task.id)).toHaveCount(0);
    await expect(card.carHistoryPanel.getByTestId('card-car-history-page')).toHaveAttribute('href', `/equipment/${eq.id}/history`);
  });
});

test.describe('OC: услуги заказа', () => {
  test('OC-009: услуга — цена/комплект из каталога, правки без кнопки «Сохранить», reload сохраняет', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-009');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-009');
    const product = await adminApi.createProduct({ name: uniq('OC-009 товар'), stock: 50, min_stock: 1 });
    const service = await adminApi.createService({
      category_id: svcCat.id,
      name: uniq('OC-009 услуга'),
      price: 750,
      products: [{ product_id: product.id, qty: 2 }],
    });
    const assignee = await makeUser({ login: uniq('exec-oc009'), name: 'Исполнитель OC-009' });
    const task = await adminApi.createTask({ title: uniq('OC-009 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await card.chooseRemote(card.subtaskAdd, service.name, service.name);
    await expect(card.subtaskRows).toHaveCount(1);
    const subtaskId = (await card.subtaskRows.first().getAttribute('data-subtask-id'))!;
    await expect(card.subtaskPrice(subtaskId)).toHaveValue('750');
    await expect(card.subtaskRow(subtaskId).getByTestId('card-product-row')).toHaveCount(1);

    await card.subtaskPrice(subtaskId).fill('');
    await card.subtaskPrice(subtaskId).blur();
    await card.chooseCombobox(card.subtaskAssignee(subtaskId), assignee.name);
    await card.subtaskDue(subtaskId).fill('2031-03-15');
    await card.chooseCombobox(card.subtaskStatus(subtaskId), 'В работе');

    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.subtaskPrice(subtaskId)).toHaveValue('');
    await expect(card.subtaskDue(subtaskId)).toHaveValue('2031-03-15');
    await expect(card.subtaskRow(subtaskId)).toContainText(assignee.name);
    await expect(card.subtaskRow(subtaskId)).toContainText('В работе');
  });
});

test.describe('OC: товары услуги и остаток', () => {
  test('OC-010: qty/цена, подсказка остатка в трёх состояниях', async ({ adminPage: page, adminApi, uniq, insertSubtask }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-010');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-010');
    const product = await adminApi.createProduct({ name: uniq('OC-010 товар'), stock: 10, min_stock: 3 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-010 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('OC-010 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await card.chooseRemote(card.subtaskProductAdd(subtaskId), product.name, product.name);
    const row = card.subtaskRow(subtaskId).getByTestId('card-product-row');
    await expect(row).toHaveCount(1);
    const productRowId = (await row.getAttribute('data-product-id'))!;

    await expect(card.productHint(productRowId)).toContainText('в наличии 10');

    await card.productQty(productRowId).fill('8');
    await card.productQty(productRowId).blur();
    await expect(card.productHint(productRowId)).toContainText('останется 2 — минимум 3');

    await card.productQty(productRowId).fill('20');
    await card.productQty(productRowId).blur();
    await expect(card.productHint(productRowId)).toContainText('в наличии только 10');

    await card.productPrice(productRowId).fill('15.5');
    await card.productPrice(productRowId).blur();
    await expect(card.productPrice(productRowId)).toHaveAttribute('aria-busy', 'false');
  });

  test('OC-011: заказ «В ремонте» — предупреждение склада, «Остатки» уменьшились, удаление услуги возвращает', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-011');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-011');
    const product = await adminApi.createProduct({ name: uniq('OC-011 товар'), stock: 5, min_stock: 1 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-011 услуга'), price: null, products: [] });
    const inWork = columnByName(columns, 'В ремонте');
    const task = await adminApi.createTask({ title: uniq('OC-011 заказ'), client_id: client.id, column_id: inWork.id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await card.chooseRemote(card.subtaskProductAdd(subtaskId), product.name, product.name);
    const row = card.subtaskRow(subtaskId).getByTestId('card-product-row');
    const productRowId = (await row.getAttribute('data-product-id'))!;
    await card.productQty(productRowId).fill('4');
    await card.productQty(productRowId).blur();

    await expect(page.getByTestId('toast-warnings')).toBeVisible();

    const afterQty = await adminApi.getStock({ q: product.name, scope: 'all' });
    expect(Number(afterQty.items[0].stock)).toBe(1);

    await card.subtaskDelete(subtaskId).click();
    await card.confirmUiDialog();
    await expect(card.subtaskRows).toHaveCount(0);

    const afterDelete = await adminApi.getStock({ q: product.name, scope: 'all' });
    expect(Number(afterDelete.items[0].stock)).toBe(5);
  });
});

test.describe('OC: статус услуги и финальная колонка', () => {
  test('OC-012: финальная колонка блокирует добавление услуги и возврат из «Выполнена»', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-012');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-012');
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-012 услуга'), price: null, products: [] });
    const final = columns.find((c) => c.is_final)!;
    // услуга создаётся ДО переноса в финальную колонку (design-tests.md §Fixtures) —
    // POST /tasks/:id/subtasks в финальной колонке сам заблокирован (409 SUBTASK_FINAL_COLUMN).
    const task = await adminApi.createTask({ title: uniq('OC-012 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, status: 'done' });
    await adminApi.moveTask(task.id, { column_id: final.id });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/subtasks/${subtaskId}`) && status === 409);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await expect(card.subtaskAdd).toBeDisabled();

    await card.chooseCombobox(card.subtaskStatus(subtaskId), 'В работе');
    await expect(page.getByTestId('ui-dialog-text')).toContainText('финальной колонке');
    await page.getByTestId('ui-dialog-ok').click();
    await expect(card.subtaskStatus(subtaskId)).toContainText('Выполнена');
  });
});

test.describe('OC: журнал работ', () => {
  test('OC-013: добавить/исправить/удалить свою запись; чужая у executor — без кнопок', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-013');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-013');
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-013 услуга'), price: null, products: [] });
    const login = uniq('exec-oc013');
    const user = await makeUser({ login });
    const task = await adminApi.createTask({
      title: uniq('OC-013 заказ'),
      client_id: client.id,
      column_id: columnByName(columns, 'Приёмка').id,
      assignee_id: user.id,
    });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id });

    await loginAs(page, login, user.password);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    const text = uniq('OC-013 запись журнала');
    await card.logNewInput(subtaskId).fill(text);
    await card.logAddBtn(subtaskId).click();
    await expect(card.subtaskRow(subtaskId)).toContainText(text);

    const logRowAfterAdd = card.subtaskRow(subtaskId).getByTestId('card-log-row');
    const logId = (await logRowAfterAdd.first().getAttribute('data-log-id'))!;
    await expect(card.logEdit(logId)).toBeVisible();
    await expect(card.logDelete(logId)).toBeVisible();

    const newText = uniq('OC-013 исправлено');
    await card.logEdit(logId).click();
    await page.getByTestId('card-subtask-log-edit-input').fill(newText);
    await page.getByTestId('card-subtask-log-save').click();
    await expect(card.logText(logId)).toHaveText(newText);

    await card.logDelete(logId).click();
    await card.confirmUiDialog();
    await expect(card.logRow(logId)).toHaveCount(0);
  });
});

test.describe('OC: одновременная правка товара', () => {
  test('OC-015: две вкладки правят qty одного товара — после reload обеих последнее значение сохранено', async ({
    adminPage: page,
    adminApi,
    uniq,
    context,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-015');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-015');
    const product = await adminApi.createProduct({ name: uniq('OC-015 товар'), stock: 100, min_stock: 1 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-015 услуга'), price: null, products: [] });
    // «В ремонте» — списывающая колонка (consumes_stock), иначе qty не резервирует склад и
    // финальная проверка «Остатки» ничего не показала бы.
    const task = await adminApi.createTask({ title: uniq('OC-015 заказ'), client_id: client.id, column_id: columnByName(columns, 'В ремонте').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 1 }] });
    const cardData = await adminApi.getCard(task.id);
    const productRowId = cardData.subtasks.find((s) => s.id === subtaskId)!.products[0].id;

    const boardA = new BoardPage(page);
    const cardA = new OrderCardPage(page);
    await boardA.goto(`?client_id=${client.id}`);
    await boardA.openTaskEdit(task.id);
    await cardA.waitOpen();

    const pageB = await context.newPage();
    const boardB = new BoardPage(pageB);
    const cardB = new OrderCardPage(pageB);
    await boardB.goto(`?client_id=${client.id}`);
    await boardB.openTaskEdit(task.id);
    await cardB.waitOpen();

    await cardA.productQty(productRowId).fill('3');
    await cardA.productQty(productRowId).blur();
    await expect(cardA.productQty(productRowId)).toHaveAttribute('aria-busy', 'false');

    await cardB.productQty(productRowId).fill('7');
    await cardB.productQty(productRowId).blur();
    await expect(cardB.productQty(productRowId)).toHaveAttribute('aria-busy', 'false');

    await boardA.goto(`?client_id=${client.id}`);
    await boardA.openTaskEdit(task.id);
    await cardA.waitOpen();
    await expect(cardA.productQty(productRowId)).toHaveValue('7');

    const stock = await adminApi.getStock({ q: product.name, scope: 'all' });
    expect(Number(stock.items[0].stock)).toBe(93);
    await pageB.close();
  });

  test('OC-016: параллельное удаление услуги при правке её товара — «Строка уже удалена»', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-016');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-016');
    const product = await adminApi.createProduct({ name: uniq('OC-016 товар'), stock: 20, min_stock: 1 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-016 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('OC-016 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 1 }] });
    const cardData = await adminApi.getCard(task.id);
    const productRowId = cardData.subtasks.find((s) => s.id === subtaskId)!.products[0].id;
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/subtask-products/${productRowId}`) && status === 404);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    // «вкладка B» удаляет услугу через API прямо перед тем, как PUT товара уходит в сеть —
    // ответ гарантированно 404 SUBTASK_PRODUCT_NOT_FOUND (услуга удалена каскадом).
    await page.route(`**/api/v1/subtask-products/${productRowId}`, async (route) => {
      if (route.request().method() === 'PUT') await adminApi.deleteSubtask(subtaskId);
      await route.continue();
    });

    await card.productQty(productRowId).fill('5');
    await card.productQty(productRowId).blur();

    await expect(page.getByTestId('ui-dialog-text')).toContainText('уже удалена');
    await page.getByTestId('ui-dialog-ok').click();
    await expect(card.subtaskRows).toHaveCount(0);
  });
});

test.describe('OC: права исполнителя', () => {
  test('OC-019: у executor нет кнопки удаления заказа, остальное редактируется', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-019');
    const login = uniq('exec-oc019');
    const user = await makeUser({ login });
    const task = await adminApi.createTask({
      title: uniq('OC-019 заказ'),
      client_id: client.id,
      column_id: columnByName(columns, 'Приёмка').id,
      assignee_id: user.id,
    });

    await loginAs(page, login, user.password);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await expect(card.deleteBtn).toHaveCount(0);
    const newTitle = uniq('OC-019 переименован');
    await card.fillHeader({ title: newTitle });
    await card.save();
    await expect(card.error).toBeHidden();
  });
});

test.describe('OC: адаптивность 375px', () => {
  test('OC-020: модалка во всю ширину, без горизонтального скролла страницы', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-020');
    const task = await adminApi.createTask({ title: uniq('OC-020 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });

    await page.setViewportSize({ width: 375, height: 812 });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1);
    expect(overflow).toBe(false);
  });
});

test.describe('OC: TD-4 валидация количества товара', () => {
  test('OC-021: qty "-1"/"abc" — ошибка у поля, откат значения, склад не изменился', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-021');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-021');
    const product = await adminApi.createProduct({ name: uniq('OC-021 товар'), stock: 10, min_stock: 1 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-021 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('OC-021 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 2 }] });
    const cardData = await adminApi.getCard(task.id);
    const productRowId = cardData.subtasks.find((s) => s.id === subtaskId)!.products[0].id;

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await card.productQty(productRowId).fill('-1');
    await card.productQty(productRowId).blur();
    await expect(card.productQtyError(productRowId)).toBeVisible();
    await expect(card.productQty(productRowId)).toHaveValue('2');

    await card.productQty(productRowId).fill('abc');
    await card.productQty(productRowId).blur();
    await expect(card.productQtyError(productRowId)).toBeVisible();
    await expect(card.productQty(productRowId)).toHaveValue('2');

    const stock = await adminApi.getStock({ q: product.name, scope: 'all' });
    expect(Number(stock.items[0].stock)).toBe(10);
  });
});

test.describe('OC: self-race ADR-105', () => {
  test('OC-022: статус→срок→цена при задержке первого PUT — не более 1 запроса в полёте, все значения сохранены', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-022');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-022');
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-022 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('OC-022 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    let inFlight = 0;
    let maxInFlight = 0;
    let responded = false;
    await page.route(`**/api/v1/subtasks/${subtaskId}`, async (route) => {
      if (route.request().method() !== 'PUT') { await route.continue(); return; }
      inFlight++;
      maxInFlight = Math.max(maxInFlight, inFlight);
      if (!responded) {
        responded = true;
        await new Promise((r) => setTimeout(r, 1500));
      }
      await route.continue();
      inFlight--;
    });

    await card.chooseCombobox(card.subtaskStatus(subtaskId), 'В работе');
    await card.subtaskDue(subtaskId).fill('2031-04-10');
    await card.subtaskPrice(subtaskId).fill('321');
    await card.subtaskPrice(subtaskId).blur();

    await expect(card.subtaskPrice(subtaskId)).toHaveAttribute('aria-busy', 'false', { timeout: 10000 });
    expect(maxInFlight).toBeLessThanOrEqual(1);

    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.subtaskRow(subtaskId)).toContainText('В работе');
    await expect(card.subtaskDue(subtaskId)).toHaveValue('2031-04-10');
    await expect(card.subtaskPrice(subtaskId)).toHaveValue('321');
  });

  test('OC-023: qty 1→2→3 при задержке первого ответа — финальный qty=3, «Остатки» = старт − 3', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-023');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-023');
    const product = await adminApi.createProduct({ name: uniq('OC-023 товар'), stock: 50, min_stock: 1 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-023 услуга'), price: null, products: [] });
    const inWork = columnByName(columns, 'В ремонте');
    const task = await adminApi.createTask({ title: uniq('OC-023 заказ'), client_id: client.id, column_id: inWork.id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 1 }] });
    const cardData = await adminApi.getCard(task.id);
    const productRowId = cardData.subtasks.find((s) => s.id === subtaskId)!.products[0].id;

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    let inFlight = 0;
    let maxInFlight = 0;
    let responded = false;
    await page.route(`**/api/v1/subtask-products/${productRowId}`, async (route) => {
      if (route.request().method() !== 'PUT') { await route.continue(); return; }
      inFlight++;
      maxInFlight = Math.max(maxInFlight, inFlight);
      if (!responded) {
        responded = true;
        await new Promise((r) => setTimeout(r, 1500));
      }
      await route.continue();
      inFlight--;
    });

    await card.productQty(productRowId).fill('2');
    await card.productQty(productRowId).blur();
    await card.productQty(productRowId).fill('3');
    await card.productQty(productRowId).blur();

    await expect(card.productQty(productRowId)).toHaveAttribute('aria-busy', 'false', { timeout: 10000 });
    expect(maxInFlight).toBeLessThanOrEqual(1);

    const stock = await adminApi.getStock({ q: product.name, scope: 'all' });
    expect(Number(stock.items[0].stock)).toBe(47);
  });
});

// Code review 8-code-review/output.md (fix-loop 1, MEDIUM-1/MEDIUM-2): гонки применения
// ответов task-card.js (openGeneration/requestSeq/appliedSeq) и откат несохранённой шапки
// (headerDirty) row-мутацией.
test.describe('OC: гонки применения ответов между карточками/строками (fix-loop 1)', () => {
  test('OC-024: закрыли карточку A с ответом в полёте, открыли B — поздний ответ A не перезаписывает B', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-024');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-024');
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-024 услуга'), price: null, products: [] });
    const newCol = columnByName(columns, 'Приёмка');
    const taskA = await adminApi.createTask({ title: uniq('OC-024 заказ A'), client_id: client.id, column_id: newCol.id });
    const taskB = await adminApi.createTask({ title: uniq('OC-024 заказ B'), client_id: client.id, column_id: newCol.id });
    const subtaskId = await insertSubtask({ taskId: taskA.id, serviceId: service.id });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.openTaskEdit(taskA.id);
    await card.waitOpen();
    await expect(card.title).toContainText(`#${taskA.id}`);

    // Запрос реально доходит до сервера и коммитится сразу (route.fetch()) — придерживаем
    // ТОЛЬКО доставку готового ответа в браузер, имитируя «ответ пришёл после закрытия A».
    await page.route(`**/api/v1/subtasks/${subtaskId}`, async (route) => {
      if (route.request().method() !== 'PUT') { await route.continue(); return; }
      const response = await route.fetch();
      await new Promise((r) => setTimeout(r, 1500));
      await route.fulfill({ response });
    });

    await card.chooseCombobox(card.subtaskStatus(subtaskId), 'В работе');
    // Не ждём ответа — закрываем карточку A ДО того, как задержанный PUT долетит обратно.
    await card.close();
    await expect(card.modal).toBeHidden();

    await board.openTaskEdit(taskB.id);
    await card.waitOpen();
    await expect(card.title).toContainText(`#${taskB.id}`);

    // Дожидаемся, пока задержанный ответ A реально долетит до браузера — именно в этот
    // момент без фикса applyCard(A) переписал бы `card`/шапку данными заказа A поверх
    // уже открытой B.
    await page.waitForResponse((r) => r.url().includes(`/api/v1/subtasks/${subtaskId}`) && r.request().method() === 'PUT');

    await expect(card.title).toContainText(`#${taskB.id}`);
    await expect(card.taskTitle).toHaveValue(taskB.title);
    await expect(card.subtaskRows).toHaveCount(0); // у B своих услуг нет — карточка A их не «подмешала»
  });

  test('OC-025: правка шапки без сохранения переживает мутацию строки услуги/товара', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-025');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-025');
    const product = await adminApi.createProduct({ name: uniq('OC-025 товар'), stock: 10, min_stock: 1 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-025 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('OC-025 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 1 }] });
    const cardData = await adminApi.getCard(task.id);
    const productRowId = cardData.subtasks.find((s) => s.id === subtaskId)!.products[0].id;

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    const newTitle = uniq('OC-025 несохранённая правка');
    await card.taskTitle.fill(newTitle);
    // Правка строки услуги — сама успешно сохраняется (re-fetch-after-write) и раньше
    // «перерисовывала» шапку значениями с сервера поверх несохранённого title.
    // aria-busy проверяем на card-subtask-price (не card-subtask-status) — combobox.js
    // переносит testid статуса на видимую кнопку БЕЗ data-row-key (эталон — OC-022/023,
    // которые точно так же используют price/qty-поле как индикатор завершения запроса).
    await card.chooseCombobox(card.subtaskStatus(subtaskId), 'В работе');
    await expect(card.subtaskPrice(subtaskId)).toHaveAttribute('aria-busy', 'false', { timeout: 10000 });
    await expect(card.taskTitle).toHaveValue(newTitle);

    // Товарная мутация — та же защита (renderAll вызывается и из неё).
    await card.productQty(productRowId).fill('4');
    await card.productQty(productRowId).blur();
    await expect(card.productQty(productRowId)).toHaveAttribute('aria-busy', 'false', { timeout: 10000 });
    await expect(card.taskTitle).toHaveValue(newTitle);

    // Save — несохранённый title реально уходит на сервер, а не откатанное значение.
    await card.save();
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();
    await expect(card.taskTitle).toHaveValue(newTitle);
    await expect(card.subtaskRow(subtaskId)).toContainText('В работе');
    await expect(card.productQty(productRowId)).toHaveValue('4');
  });

  test('OC-026: ответы двух строк не в порядке отправки — поздний устаревший не откатывает более свежую строку', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-026');
    const { svcCat } = await setupCatalog(adminApi, uniq, 'OC-026');
    const product = await adminApi.createProduct({ name: uniq('OC-026 товар'), stock: 20, min_stock: 1 });
    const service = await adminApi.createService({ category_id: svcCat.id, name: uniq('OC-026 услуга'), price: null, products: [] });
    const task = await adminApi.createTask({ title: uniq('OC-026 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 1 }] });
    const cardData = await adminApi.getCard(task.id);
    const productRowId = cardData.subtasks.find((s) => s.id === subtaskId)!.products[0].id;

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    // Строка «услуга» (статус) коммитится на сервере сразу (route.fetch()), но её ОТВЕТ
    // придерживаем — доставляем клиенту ПОСЛЕ ответа на независимую строку «товар»,
    // отправленную следом и ничем не задержанную. Без seq-guard'а этот поздний ответ
    // (снятый ДО изменения qty) откатил бы уже сохранённое qty в отображении.
    await page.route(`**/api/v1/subtasks/${subtaskId}`, async (route) => {
      if (route.request().method() !== 'PUT') { await route.continue(); return; }
      const response = await route.fetch();
      await new Promise((r) => setTimeout(r, 1500));
      await route.fulfill({ response });
    });

    await card.chooseCombobox(card.subtaskStatus(subtaskId), 'В работе');
    await card.productQty(productRowId).fill('3');
    await card.productQty(productRowId).blur();

    // aria-busy у card-subtask-price — тот же data-row-key (subtask:<id>), что и у статуса
    // (combobox.js уносит testid статуса на кнопку без data-row-key, см. OC-025).
    await expect(card.productQty(productRowId)).toHaveAttribute('aria-busy', 'false', { timeout: 5000 });
    await expect(card.subtaskPrice(subtaskId)).toHaveAttribute('aria-busy', 'false', { timeout: 10000 });

    await expect(card.productQty(productRowId)).toHaveValue('3');
    await expect(card.subtaskRow(subtaskId)).toContainText('В работе');
  });
});
