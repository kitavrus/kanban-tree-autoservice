import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { ShellPage } from '../pages/ShellPage';
import { StockPage } from '../pages/StockPage';
import { loginContext, type AdminApi } from '../fixtures/auth';

// Фаза 6: доска заказов «/» — DnD, формы, фильтры (BD-001…BD-014, design-tests.md).
// Каждый тест независим от состояния БД стенда (§TD-6.2): своя связка клиент/категория/
// товар/услуга заводится через uniq()+adminApi, доска открывается отфильтрованной по
// собственному ?client_id= там, где нужна изоляция от чужих заказов на общей БД.
test.use({ colorScheme: 'light' });

// label различает клиентов между тестами файла: testInfo.testId у Playwright не гарантирует
// уникальность первых символов среза (uniq() режет testId.slice(0,6)) между разными test()
// в одном файле — при общем литерале 'BD клиент' у всех тестов, вызывающих setup() первым
// же uniq(), результат мог совпасть текстуально (разные id, одинаковое company_name),
// а комбобокс клиента в форме заказа ищет строго по видимому тексту (BD-009: обнаружено
// на живом стенде — форма выбирала ЧУЖОГО клиента с тем же именем, заказ уходил не туда,
// доска, отфильтрованная по ?client_id= СВОЕГО клиента, показывала 0 заказов).
async function setup(adminApi: AdminApi, uniq: (s: string) => string, label: string) {
  const client = await adminApi.createClient({ company_name: uniq(`BD клиент ${label}`), phones: [{ phone: '9990000001', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  return { client, columns };
}

test.describe('BD: карточка и цвет колонки', () => {
  test('BD-001: карточка с #N, названием, клиентом, ответственным, сроком, оплатой; цвет полосы = цвет колонки', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-001');
    const assignee = await makeUser({ login: uniq('exec-bd001'), name: 'Исполнитель BD-001' });
    const newCol = columnByName(columns, 'Приёмка');
    const title = uniq('Заказ BD-001');
    const task = await adminApi.createTask({
      title,
      client_id: client.id,
      column_id: newCol.id,
      assignee_id: assignee.id,
      due_date: '2030-01-15',
      is_paid: true,
    });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    const card = board.card(task.id);
    await expect(card).toBeVisible();
    await expect(card).toContainText(`#${task.id}`);
    await expect(card).toContainText(title);
    await expect(card).toContainText(client.company_name);
    await expect(card).toContainText('15.01.2030');
    await expect(card).toContainText('Оплачен');
    await expect(card).toHaveAttribute('style', new RegExp(newCol.color, 'i'));
  });
});

test.describe('BD: DnD между колонками', () => {
  test('BD-002: перенос между колонками и перестановка внутри — reload сохраняет положение', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-002');
    const from = columnByName(columns, 'Приёмка');
    const to = columnByName(columns, 'Согласование');
    const t1 = await adminApi.createTask({ title: uniq('BD-002 первый'), client_id: client.id, column_id: from.id });
    const t2 = await adminApi.createTask({ title: uniq('BD-002 второй'), client_id: client.id, column_id: from.id });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.dragCardToColumn(t1.id, to.id);
    await expect(board.columnBody(to.id).getByTestId(`task-card-${t1.id}`)).toBeVisible();
    await expect.poll(async () => (await adminApi.getTask(t1.id)).column_id).toBe(to.id);
    await expect(page.getByRole('status')).toContainText('«Согласование»');

    // t2 переносится и встаёт перед t1 в целевой колонке — проверка вычисления prev_id/next_id.
    await board.dragCardBefore(t2.id, t1.id);
    await expect.poll(async () => (await adminApi.getTask(t2.id)).column_id).toBe(to.id);

    const orderBefore = await board.columnBody(to.id).locator('.task-card').evaluateAll((els) => els.map((e) => e.getAttribute('data-id')));
    await page.reload();
    await expect(board.card(t1.id)).toBeVisible();
    const orderAfter = await board.columnBody(to.id).locator('.task-card').evaluateAll((els) => els.map((e) => e.getAttribute('data-id')));
    expect(orderAfter).toEqual(orderBefore);
  });
});

test.describe('BD: финальная колонка блокирует незавершённые услуги', () => {
  test('BD-003: перенос в «Выдан» с открытой услугой отклонён, после завершения — проходит', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    setSubtaskStatus,
    allowHttpErrors,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-003');
    const from = columnByName(columns, 'Приёмка');
    const done = columnByName(columns, 'Выдан');
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-003 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-003 услуга') });
    const task = await adminApi.createTask({ title: uniq('BD-003 заказ'), client_id: client.id, column_id: from.id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: svc.id, status: 'new' });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}/move`) && status === 409);

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.dragCardToColumn(task.id, done.id);
    // отклонено — карточка вернулась в исходную колонку, диалог со списком услуг.
    await expect(page.getByTestId('ui-dialog')).toBeVisible();
    await expect(page.getByTestId('ui-dialog-text')).toContainText(svc.name);
    await page.getByTestId('ui-dialog-ok').click();
    await expect(board.columnBody(from.id).getByTestId(`task-card-${task.id}`)).toBeVisible();
    await expect.poll(async () => (await adminApi.getTask(task.id)).column_id).toBe(from.id);

    await setSubtaskStatus(subtaskId, 'done');
    await board.dragCardToColumn(task.id, done.id);
    await expect(board.columnBody(done.id).getByTestId(`task-card-${task.id}`)).toBeVisible();
    await expect.poll(async () => (await adminApi.getTask(task.id)).column_id).toBe(done.id);
  });
});

test.describe('BD: списание товаров', () => {
  test('BD-004: перенос в списывающую колонку уменьшает остаток; возврат — восстанавливает', async ({ adminPage: page, adminApi, uniq, insertSubtask }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-004');
    const from = columnByName(columns, 'Приёмка');
    const work = columnByName(columns, 'В ремонте'); // consumes_stock=true (сид D32)
    const waitParts = columnByName(columns, 'Ждёт запчасти'); // тоже consumes_stock=true (сид D32)

    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('BD-004 категория товаров') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('BD-004 товар'), stock: 5 });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-004 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-004 услуга') });
    const task = await adminApi.createTask({ title: uniq('BD-004 заказ'), client_id: client.id, column_id: from.id });
    await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 2 }] });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.dragCardToColumn(task.id, work.id);
    await expect.poll(async () => (await adminApi.getTask(task.id)).column_id).toBe(work.id);
    await expect.poll(() => stockOf(page, product.id)).toBe(3);

    // «В ремонте» → «Ждёт запчасти»: обе списывающие — 0 новых движений, остаток не меняется.
    await board.dragCardToColumn(task.id, waitParts.id);
    await expect.poll(async () => (await adminApi.getTask(task.id)).column_id).toBe(waitParts.id);
    await expect.poll(() => stockOf(page, product.id)).toBe(3);

    // назад в несписывающую «Приёмка» — товары возвращаются.
    await board.dragCardToColumn(task.id, from.id);
    await expect.poll(() => stockOf(page, product.id)).toBe(5);
  });
});

test.describe('BD: минус на складе предупреждает, не блокирует', () => {
  test('BD-005: недостаточный остаток — перенос проходит, toast со ссылкой на «Остатки»', async ({ adminPage: page, adminApi, uniq, insertSubtask }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-005');
    const from = columnByName(columns, 'Приёмка');
    const work = columnByName(columns, 'В ремонте');

    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('BD-005 категория товаров') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('BD-005 товар'), stock: 1 });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-005 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-005 услуга') });
    const task = await adminApi.createTask({ title: uniq('BD-005 заказ'), client_id: client.id, column_id: from.id });
    await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 3 }] });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.dragCardToColumn(task.id, work.id);
    await expect.poll(async () => (await adminApi.getTask(task.id)).column_id).toBe(work.id);
    await expect(board.toastWarnings).toBeVisible();
    await expect(board.toastWarnings).toContainText(product.name);
    await expect(board.toastWarnings.locator('a[href="/stock"]')).toBeVisible();
    await expect.poll(() => stockOf(page, product.id)).toBe(-2);
  });
});

test.describe('BD: управление колонками (admin)', () => {
  test('BD-006: admin создаёт/переименовывает/красит/двигает/удаляет колонку; executor не видит управление', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const board = new BoardPage(page);
    await board.goto();

    const name = uniq('BD-006 колонка');
    await board.openColumnAdd();
    await board.fillColumn({ name, color: '#112233', isFinal: false, consumes: false });
    await board.columnSave.click();
    await expect(board.columnForm).toBeHidden();
    const colId = await board.columnIdByName(name);
    await expect(board.column(colId)).toBeVisible();

    const renamed = uniq('BD-006 переименована');
    await board.openColumnEdit(colId);
    await board.fillColumn({ name: renamed, color: '#445566' });
    await board.columnSave.click();
    await expect(board.columnForm).toBeHidden();
    await expect(board.columnByName(renamed)).toBeVisible();

    // удаление пустой колонки — проходит: общее окно «Удаление» с причиной
    // (soft-delete-board-columns §8.1), удаление мягкое.
    await board.openColumnEdit(colId);
    await board.columnDelete.click();
    await expect(board.deleteDialog).toBeVisible();
    await board.deleteReason.fill('BD-006: колонка больше не нужна');
    await board.deleteDialogOk.click();
    await expect(board.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
    await expect(board.columnForm).toBeHidden();
    await expect(board.columnByName(renamed)).toHaveCount(0);
    await adminApi.purgeColumn(colId); // колонка создана через форму — уборка своими руками

    // удаление непустой — сообщение, колонка на месте.
    const busyName = uniq('BD-006 занятая');
    await board.openColumnAdd();
    await board.fillColumn({ name: busyName, color: '#667788' });
    await board.columnSave.click();
    await expect(board.columnForm).toBeHidden();
    const busyId = await board.columnIdByName(busyName);
    const client = await adminApi.createClient({ company_name: uniq('BD-006 клиент'), phones: [{ phone: '9990000002', note: '' }] });
    await adminApi.createTask({ title: uniq('BD-006 заказ'), client_id: client.id, column_id: busyId });
    await page.reload();
    allowHttpErrors((url, status) => url.includes(`/api/v1/columns/${busyId}`) && status === 409);

    // из меню колонки — реальная попытка удаления (сервер отклоняет 409 COLUMN_NOT_EMPTY):
    // окно «Удаление» с причиной; отказ — красным блоком над кнопками, окно остаётся.
    await board.openDeleteColumn(busyId);
    await board.deleteReason.fill('BD-006: попытка удалить занятую');
    await board.deleteDialogOk.click();
    await expect(board.deleteDialogError).toHaveText('В колонке есть заказы');
    await expect(board.deleteDialog).toBeVisible();
    await board.deleteDialogCancel.click();
    await expect(board.deleteDialog).toBeHidden();
    await expect(board.column(busyId)).toBeVisible();

    // модалка колонки — кнопка «Удалить» тоже заблокирована клиентской проверкой.
    await board.openColumnEdit(busyId);
    await expect(board.columnDelete).toBeDisabled();

    // executor не видит управление колонками.
    const login = uniq('exec-bd006');
    await adminApi.createUser({ login, name: 'Исполнитель BD-006', role: 'executor', password: 'password1' });
    await page.context().clearCookies();
    const shell = new ShellPage(page);
    await loginContext(page.context(), login, 'password1');
    await shell.goto('/');
    await expect(board.columnAddBtn).toHaveCount(0);
    await expect(board.columnMenu(busyId)).toHaveCount(0);
  });
});

test.describe('BD: списывать товары у колонки', () => {
  test('BD-007: включение «списывать» у колонки с заказом уменьшает остаток; выключение — возвращает', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client } = await setup(adminApi, uniq, 'BD-007');
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('BD-007 категория товаров') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('BD-007 товар'), stock: 10 });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-007 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-007 услуга') });

    const col = (await adminApi.createColumn({ name: uniq('BD-007 колонка'), color: '#123123', is_final: false, consumes_stock: false })).column;
    const task = await adminApi.createTask({ title: uniq('BD-007 заказ'), client_id: client.id, column_id: col.id });
    await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 4 }] });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.openColumnEdit(col.id);
    await board.fillColumn({ consumes: true });
    await board.columnSave.click();
    await expect(board.columnForm).toBeHidden();
    await expect.poll(() => stockOf(page, product.id)).toBe(6);

    await board.openColumnEdit(col.id);
    await board.fillColumn({ consumes: false });
    await board.columnSave.click();
    await expect(board.columnForm).toBeHidden();
    await expect.poll(() => stockOf(page, product.id)).toBe(10);
  });
});

test.describe('BD: дефолтные колонки D32', () => {
  test('BD-008: сид-колонки присутствуют с ожидаемыми именами и флагами', async ({ adminPage: page }) => {
    const board = new BoardPage(page);
    await board.goto();
    const expected: Array<[string, boolean, boolean]> = [
      ['Приёмка', false, false],
      ['Диагностика', false, false],
      ['Согласование', false, false],
      ['Ждёт запчасти', false, true],
      ['В ремонте', false, true],
      ['Готов к выдаче', false, true],
      ['Выдан', true, true],
    ];
    for (const [name, isFinal, consumes] of expected) {
      const id = await board.columnIdByName(name);
      await board.openColumnEdit(id);
      await expect(board.columnName).toHaveValue(name);
      expect(await board.columnFinal.isChecked()).toBe(isFinal);
      expect(await board.columnConsumes.isChecked()).toBe(consumes);
      await board.page.getByTestId('column-form').getByRole('button', { name: 'Отмена' }).click();
      await expect(board.columnForm).toBeHidden();
    }
  });
});

test.describe('BD: форма заказа', () => {
  test('BD-009: создание формой, правка, двойной клик «Сохранить» — один запрос', async ({ adminPage: page, adminApi, uniq, _boardCleanup }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-009');
    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    let createCalls = 0;
    page.on('request', (r) => {
      if (r.method() === 'POST' && r.url().endsWith('/api/v1/tasks')) createCalls++;
    });

    const title = uniq('BD-009 заказ');
    await board.openTaskAdd();
    await board.fillTask({ title, client: client.company_name, column: columnByName(columns, 'Приёмка').name });
    // Два клика синхронно в одном browser-тике (а не Playwright dblclick — реальные мышиные
    // события дают гонку между вкладом браузера в обработку disabled и таймингом теста):
    // saving-флаг выставляется синхронно первым же кликом ДО await fetch, поэтому второй
    // click() из того же скрипта детерминированно видит форму уже заблокированной.
    await page.evaluate(() => {
      const btn = document.querySelector('[data-testid="task-save"]') as HTMLButtonElement;
      btn.click();
      btn.click();
    });
    // Карточка заказа (фаза 6, SD-006) остаётся открытой после создания — разделы
    // оборудования/услуг разблокируются той же модалкой, а не новой формой.
    await expect(board.taskForm).toBeVisible();
    await expect(page.locator('#task-modal-title')).toContainText('Заказ #');
    expect(createCalls).toBe(1);
    await page.locator('#task-modal .btn-close').click();
    await expect(board.taskForm).toBeHidden();

    const cardTitle = board.root.locator('.task-card', { hasText: title });
    await expect(cardTitle).toBeVisible();
    const idAttr = await cardTitle.first().getAttribute('data-id');
    const taskId = Number(idAttr);
    _boardCleanup(taskId); // заказ создан формой, мимо adminApi.createTask — на уборку вручную

    // правка той же карточкой — шапка сохраняется кнопкой, карточка остаётся открытой.
    await board.openTaskEdit(taskId);
    const newTitle = uniq('BD-009 переименован');
    await board.fillTask({ title: newTitle });
    await board.taskSave.click();
    await expect(board.taskError).toBeHidden();
    await page.locator('#task-modal .btn-close').click();
    await expect(board.taskForm).toBeHidden();
    await expect(board.card(taskId)).toContainText(newTitle);
  });
});

test.describe('BD: конфликт версии', () => {
  test('BD-010: конкурентная правка — форма показывает «Заказ уже изменён»', async ({ adminPage: page, adminApi, uniq, allowHttpErrors }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-010');
    const task = await adminApi.createTask({ title: uniq('BD-010 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}`) && status === 409);

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);

    // «второй пользователь» правит заказ через API, пока форма первого открыта — версия в форме устаревает.
    await adminApi.updateTask(task.id, {
      title: task.title,
      client_id: task.client_id,
      column_id: task.column_id,
      assignee_id: task.assignee_id,
      due_date: task.due_date,
      is_paid: task.is_paid,
      version: task.version,
    });

    await board.fillTask({ title: uniq('BD-010 попытка правки') });
    await board.taskSave.click();
    await expect(page.getByTestId('ui-dialog-text')).toContainText('уже изменён');
    await page.getByTestId('ui-dialog-ok').click();
  });
});

test.describe('BD: оплата — только администратор', () => {
  test('BD-011: admin отмечает оплату; у executor переключатель disabled, прямой PUT — 403', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-011');
    const task = await adminApi.createTask({ title: uniq('BD-011 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await expect(board.taskPaid).not.toBeDisabled();
    await board.taskPaid.check();
    await board.taskSave.click();
    await expect(board.taskError).toBeHidden();
    await expect.poll(async () => (await adminApi.getTask(task.id)).is_paid).toBe(true);
    await page.locator('#task-modal .btn-close').click();
    await expect(board.taskForm).toBeHidden();

    const login = uniq('exec-bd011');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);
    await board.goto(`?client_id=${client.id}`);
    // заказ без ответственного — под дефолтным «Мои» (AC-014) исполнитель его не видит.
    await board.chooseCombobox(board.filterAssignee, 'Все исполнители');
    await board.openTaskEdit(task.id);
    await expect(board.taskPaid).toBeDisabled();
    await expect(board.taskPaid).toBeChecked();

    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}`) && status === 403);
    // Актуальная версия: с устаревшей сервер сначала отвечает 409 TASK_VERSION_CONFLICT (AC-010).
    const current = await adminApi.getTask(task.id);
    const res = await page.request.put(`/api/v1/tasks/${task.id}`, {
      data: { title: current.title, client_id: current.client_id, column_id: current.column_id, is_paid: false, version: current.version },
      headers: { 'X-Csrf-Token': await csrfToken(page) },
      failOnStatusCode: false,
    });
    expect(res.status()).toBe(403);
    const body = await res.json();
    expect(body.error?.code ?? body.code).toBe('PAID_ADMIN_ONLY');
  });
});

test.describe('BD: удаление заказа', () => {
  test('BD-012: admin удаляет заказ с подтверждением — товары возвращаются; у executor кнопки нет', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    makeUser,
    loginAs,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-012');
    const from = columnByName(columns, 'Приёмка');
    const work = columnByName(columns, 'В ремонте');
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('BD-012 категория товаров') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('BD-012 товар'), stock: 5 });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-012 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-012 услуга') });
    // создаём заказ в несписывающей колонке — Reconcile пересчитывает списание только на
    // Create/Move; подзадача сидит в БД без него (raw insert, ADR-103), поэтому списание
    // происходит именно при move в «В ремонте» ниже, а не при создании заказа.
    const task = await adminApi.createTask({ title: uniq('BD-012 заказ'), client_id: client.id, column_id: from.id });
    await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 2 }] });
    await adminApi.moveTask(task.id, { column_id: work.id });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect.poll(() => stockOf(page, product.id)).toBe(3);

    await board.openTaskEdit(task.id);
    await board.confirmDeleteTask();
    await expect(board.taskForm).toBeHidden();
    await expect(board.card(task.id)).toHaveCount(0);
    await expect.poll(() => stockOf(page, product.id)).toBe(5);

    const login = uniq('exec-bd012');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);
    const task2 = await adminApi.createTask({ title: uniq('BD-012 заказ 2'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
    await board.goto(`?client_id=${client.id}`);
    // заказ без ответственного — под дефолтным «Мои» (AC-014) исполнитель его не видит.
    await board.chooseCombobox(board.filterAssignee, 'Все исполнители');
    await board.openTaskEdit(task2.id);
    await expect(board.taskDelete).toHaveCount(0);
  });
});

test.describe('BD: фильтры доски', () => {
  test('BD-013: фильтры сочетаются, отражены в URL, reload сохраняет, «Сбросить» очищает', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-013');
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-013 категория') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-013 услуга') });
    const matching = await adminApi.createTask({ title: uniq('BD-013 подходит'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id, is_paid: true });
    const other = await adminApi.createTask({ title: uniq('BD-013 не подходит'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id, is_paid: false });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.chooseCombobox(board.filterPaid, 'Оплачен');
    await expect.poll(() => new URL(page.url()).searchParams.get('paid')).toBe('paid');
    await expect(board.card(matching.id)).toBeVisible();
    await expect(board.card(other.id)).toHaveCount(0);

    await board.filterQ.fill(String(matching.id));
    // debounce фильтра q (250мс) — web-first: expect.poll сам переопрашивает URL, без waitForTimeout.
    await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe(String(matching.id));

    const url = page.url();
    await page.reload();
    await expect(page).toHaveURL(url);
    await expect(board.filterPaid).toContainText('Оплачен');
    await expect(board.card(matching.id)).toBeVisible();

    await board.filterReset.click();
    await expect.poll(() => new URL(page.url()).search).toBe('');
    await expect(board.card(matching.id)).toBeVisible();
    await expect(board.card(other.id)).toBeVisible();
  });
});

test.describe('BD: поиск по услуге, товару и оборудованию', () => {
  test('BD-022: поиск находит заказ по услуге, товару, модели и SN оборудования', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    insertTaskEquipment,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-022');
    const col = columnByName(columns, 'Приёмка');
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-022 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-022 Замена турбины') });
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('BD-022 категория товаров') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('BD-022 Фильтр'), stock: 5 });
    const eqType = await adminApi.createEquipmentType({ name: uniq('BD-022 тип') });
    const model = uniq('RX-900');
    const serial = uniq('SN77AB');
    const equipment = await adminApi.createEquipment(client.id, { type_id: eqType.id, model, serial });

    const target = await adminApi.createTask({ title: uniq('BD-022 искомый'), client_id: client.id, column_id: col.id });
    const other = await adminApi.createTask({ title: uniq('BD-022 другой'), client_id: client.id, column_id: col.id });
    await insertSubtask({ taskId: target.id, serviceId: svc.id, products: [{ productId: product.id, qty: 1 }] });
    await insertTaskEquipment(target.id, equipment.id);

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.card(other.id)).toBeVisible();

    for (const q of [svc.name, product.name, model, serial]) {
      await board.filterQ.fill(q);
      await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe(q);
      await expect(board.card(target.id)).toBeVisible();
      await expect(board.card(other.id)).toHaveCount(0);
    }
  });
});

test.describe('BD: «Мои» по умолчанию для исполнителя', () => {
  test('BD-014: executor по умолчанию видит только свои заказы, «Все» показывает чужие', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-014');
    const col = columnByName(columns, 'Приёмка');
    const login = uniq('exec-bd014');
    const executor = await makeUser({ login });

    const mine = await adminApi.createTask({ title: uniq('BD-014 моя'), client_id: client.id, column_id: col.id, assignee_id: executor.id });
    const foreign = await adminApi.createTask({ title: uniq('BD-014 чужая'), client_id: client.id, column_id: col.id });

    await loginAs(page, login, executor.password);
    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await expect(board.card(mine.id)).toBeVisible();
    await expect(board.card(foreign.id)).toHaveCount(0);
    expect(new URL(page.url()).searchParams.get('assignee')).toBeNull();

    await board.chooseCombobox(board.filterAssignee, 'Все исполнители');
    await expect.poll(() => new URL(page.url()).searchParams.get('assignee')).toBe('all');
    await expect(board.card(mine.id)).toBeVisible();
    await expect(board.card(foreign.id)).toBeVisible();
  });
});

test.describe('BD: адаптив 360px и тёмная тема', () => {
  test('BD-020: доска/форма заказа/«Остатки» без overflow страницы на 360×800 в обеих темах; перенос через меню без DnD', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-020');
    const from = columnByName(columns, 'Приёмка');
    const to = columnByName(columns, 'Согласование');
    const task = await adminApi.createTask({ title: uniq('BD-020 заказ'), client_id: client.id, column_id: from.id });

    const board = new BoardPage(page);
    const stock = new StockPage(page);
    const noOverflow = () => page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);

    for (const theme of ['light', 'dark'] as const) {
      if (theme === 'dark') await page.addInitScript(() => localStorage.setItem('kt-theme', 'dark'));
      await page.setViewportSize({ width: 360, height: 800 });

      await board.goto(`?client_id=${client.id}`);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      // страница целиком НЕ скроллится вбок (US-1 adaptive:min) — только .board-columns
      // внутри себя (её собственный scrollWidth больше вьюпорта, т.к. колонок больше одной
      // 85vw-ширины на 360px).
      expect(await noOverflow()).toBe(false);
      const boardScrolls = await board.root.evaluate((el) => el.scrollWidth > el.clientWidth);
      expect(boardScrolls).toBe(true);

      await board.openTaskAdd();
      expect(await noOverflow()).toBe(false);
      await board.taskForm.getByRole('button', { name: 'Отмена' }).click();
      await expect(board.taskForm).toBeHidden();

      await stock.goto();
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      expect(await noOverflow()).toBe(false);
    }

    // ADR-106: перенос карточки через меню «⋯ → Переместить в…» работает без DnD — на
    // мобильном экране это единственный способ переноса.
    await board.goto(`?client_id=${client.id}`);
    await board.moveViaMenu(task.id, to.id);
    await expect(board.columnBody(to.id).getByTestId(`task-card-${task.id}`)).toBeVisible();
    await expect.poll(async () => (await adminApi.getTask(task.id)).column_id).toBe(to.id);
  });
});

test.describe('BD: итог заказа и колонки на доске (AC-013)', () => {
  test('BD-021: task-card-total/last-log на карточках, column-total = сумма видимых; без overflow на 375px в обеих темах', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-021');
    const newCol = columnByName(columns, 'Приёмка');
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-021 категория услуг') });
    const svcA = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-021 услуга A'), price: 100 });
    const svcB = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-021 услуга B'), price: 250.5 });

    // taskZero — заказ без услуг: total_amount=0 → task-card-total на нём НЕ рендерится
    // (board.js cardHTML, «если >0»).
    const taskA = await adminApi.createTask({ title: uniq('BD-021 заказ A'), client_id: client.id, column_id: newCol.id });
    const subtaskA = await adminApi.createSubtask(taskA.id, svcA.id);
    const taskB = await adminApi.createTask({ title: uniq('BD-021 заказ B'), client_id: client.id, column_id: newCol.id });
    await adminApi.createSubtask(taskB.id, svcB.id);
    const taskZero = await adminApi.createTask({ title: uniq('BD-021 заказ без услуг'), client_id: client.id, column_id: newCol.id });

    const board = new BoardPage(page);
    // отфильтровано по СВОЕМУ клиенту (§TD-6.2) — column-total считается по видимым
    // (после фильтра) карточкам, на общей dev-БД в колонке «Приёмка» есть и чужие заказы.
    await board.goto(`?client_id=${client.id}`);

    // журнал работ услуги (AC-012, для task-card-last-log) — в AdminApi нет метода для него
    // (не нужен больше ни одному e2e-тесту), поэтому прямой запрос от уже залогиненной
    // adminPage с её CSRF-токеном (эталон — PUT /tasks/:id в BD-010 этого файла).
    await page.request.post(`/api/v1/subtasks/${subtaskA.card.subtasks[0].id}/logs`, {
      data: { text: uniq('BD-021 запись журнала') },
      headers: { 'X-Csrf-Token': await csrfToken(page) },
    });
    await board.goto(`?client_id=${client.id}`);

    await expect(board.card(taskA.id).getByTestId('task-card-total')).toHaveText('Итог: 100.00');
    await expect(board.card(taskA.id).getByTestId('task-card-last-log')).toBeVisible();
    await expect(board.card(taskB.id).getByTestId('task-card-total')).toHaveText('Итог: 250.50');
    await expect(board.card(taskZero.id).getByTestId('task-card-total')).toHaveCount(0);
    await expect(board.columnByName('Приёмка').getByTestId('column-total')).toHaveText('350.50');

    // UI-scope gate (US-1 adaptive:min · themes:both, design-tests.md DoD фазы 06): новые
    // элементы карточки/колонки видимы и БЕЗ горизонтального overflow страницы на 375px в
    // обеих темах — эталон цикла тем/вьюпорта BD-020 выше в этом файле.
    const noOverflow = () => page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
    for (const theme of ['light', 'dark'] as const) {
      if (theme === 'dark') await page.addInitScript(() => localStorage.setItem('kt-theme', 'dark'));
      await page.setViewportSize({ width: 375, height: 812 });
      await board.goto(`?client_id=${client.id}`);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      await expect(board.card(taskA.id).getByTestId('task-card-total')).toBeVisible();
      await expect(board.card(taskA.id).getByTestId('task-card-last-log')).toBeVisible();
      await expect(board.columnByName('Приёмка').getByTestId('column-total')).toBeVisible();
      expect(await noOverflow()).toBe(false);
    }
  });
});

// ---------- утилиты ----------
async function stockOf(page: import('@playwright/test').Page, productId: string): Promise<number> {
  const res = await page.request.get(`/api/v1/stock?scope=all&limit=1000`);
  const body: { items: Array<{ product_id: string; stock: number | string }> } = await res.json();
  const row = body.items.find((r) => r.product_id === productId);
  return row ? Number(row.stock) : NaN;
}

async function csrfToken(page: import('@playwright/test').Page): Promise<string> {
  return (await page.locator('meta[name="csrf-token"]').getAttribute('content')) ?? '';
}
