import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { ExecutorPayoutsPage } from '../pages/ExecutorPayoutsPage';
import { ShellPage } from '../pages/ShellPage';
import type { AdminApi } from '../fixtures/auth';

// Выплаты исполнителям (docs/plan.md D62–D71; сценарии TS-021…TS-023 —
// docs/features/executor-payouts/0-setup/light-progress.md): отметка/сумма/дата в карточке
// заказа, фильтр и метка доски, экран-отчёт «Выплаты» с массовой отметкой. Общая dev-БД
// (§TD-6.2): свои клиент/услуга/исполнители через uniq()+adminApi; отчёт и доска всегда
// сужаются до своих данных (assignee=<id> / client_id=<id>).
test.use({ colorScheme: 'light' });

type Ctx = { adminApi: AdminApi; uniq: (s: string) => string };

async function setup({ adminApi, uniq }: Ctx, label: string) {
  const client = await adminApi.createClient({ company_name: uniq(`EP клиент ${label}`), phones: [{ phone: '9990000200', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  const cat = await adminApi.createCategory({ kind: 'service', name: uniq(`EP услуги ${label}`) });
  const service = await adminApi.createService({ category_id: cat.id, name: uniq(`EP услуга ${label}`), price: 1000 });
  const task = await adminApi.createTask({ title: uniq(`EP заказ ${label}`), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
  return { client, columns, service, task };
}

async function subtaskOf(adminApi: AdminApi, taskId: number, subtaskId: string) {
  const card = await adminApi.getCard(taskId);
  const sub = card.subtasks.find((s) => s.id === subtaskId);
  if (!sub) throw new Error(`услуга ${subtaskId} не найдена в заказе ${taskId}`);
  return sub;
}

test.describe('EP: выплата исполнителю в карточке заказа', () => {
  test('EP-001 (TS-021): отметка «Выплачено исполнителю» + сумма + дата; без исполнителя отметка недоступна; смена исполнителя у выплаченной — отказ', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const exec = await makeUser({ login: uniq('ep001'), name: uniq('Исполнитель EP-001') });
    const { client, service, task } = await setup({ adminApi, uniq }, 'EP-001');
    // Статус и цена — явным PUT: фикстура insertSubtask({status}) шлёт строку без price
    // (полная замена → цена сбрасывается в «по факту»), а тесту нужна цена 1000.
    const subId = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(subId, { status: 'done', price: 1000 });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.subtaskPrice(subId)).toHaveValue('1000');

    // без исполнителя отметить выплату нельзя; сумму задать можно (не зависит от флага).
    await expect(card.subtaskExecutorPaid(subId)).toBeDisabled();
    await expect(card.subtaskExecutorPaid(subId)).not.toBeChecked();
    await expect(card.subtaskExecutorPaidAt(subId)).toHaveText('');
    await card.subtaskExecutorPayout(subId).fill('1500.5');
    await card.subtaskExecutorPayout(subId).press('Tab');
    await expect.poll(async () => Number((await subtaskOf(adminApi, task.id, subId)).executor_payout)).toBe(1500.5);
    await expect(card.subtaskExecutorPayout(subId)).toHaveValue('1500.5');
    expect((await subtaskOf(adminApi, task.id, subId)).is_executor_paid).toBe(false);

    // невалидная сумма — подсказка у поля, значение откатывается, на сервер не уходит.
    await card.subtaskExecutorPayout(subId).fill('-5');
    await card.subtaskExecutorPayout(subId).press('Tab');
    await expect(card.subtaskPayoutWarning(subId)).toBeVisible();
    await expect(card.subtaskExecutorPayout(subId)).toHaveValue('1500.5');
    await card.subtaskExecutorPayout(subId).fill('1.234');
    await card.subtaskExecutorPayout(subId).press('Tab');
    await expect(card.subtaskPayoutWarning(subId)).toBeVisible();
    expect(Number((await subtaskOf(adminApi, task.id, subId)).executor_payout)).toBe(1500.5);

    // назначили исполнителя — отметка доступна.
    await card.chooseCombobox(card.subtaskAssignee(subId), exec.name);
    await expect(card.subtaskExecutorPaid(subId)).toBeEnabled();
    await card.subtaskExecutorPaid(subId).click();
    await expect(card.subtaskExecutorPaid(subId)).toBeChecked();
    await expect(card.subtaskExecutorPaidAt(subId)).toHaveText(/\d{2}\.\d{2}\.\d{4}/);
    await expect.poll(async () => (await subtaskOf(adminApi, task.id, subId)).is_executor_paid).toBe(true);
    const paid = await subtaskOf(adminApi, task.id, subId);
    expect(paid.executor_paid_at).not.toBeNull();
    expect(Number(paid.executor_payout)).toBe(1500.5);
    expect(Number(paid.price)).toBe(1000); // цена для клиента — отдельное поле

    // сумму можно править и после отметки — дата отметки не меняется.
    await card.subtaskExecutorPayout(subId).fill('1700');
    await card.subtaskExecutorPayout(subId).press('Tab');
    await expect.poll(async () => Number((await subtaskOf(adminApi, task.id, subId)).executor_payout)).toBe(1700);
    expect((await subtaskOf(adminApi, task.id, subId)).executor_paid_at).toBe(paid.executor_paid_at);

    // обратная сторона: правка цены и статуса в карточке (PUT — полная замена строки) не
    // стирает ни сумму выплаты, ни отметку, ни её дату; цена при правках выплаты тоже цела.
    await card.subtaskPrice(subId).fill('1200');
    await card.subtaskPrice(subId).press('Tab');
    await expect.poll(async () => Number((await subtaskOf(adminApi, task.id, subId)).price)).toBe(1200);
    await card.chooseCombobox(card.subtaskStatus(subId), 'В работе');
    await expect.poll(async () => (await subtaskOf(adminApi, task.id, subId)).status).toBe('in_progress');
    const edited = await subtaskOf(adminApi, task.id, subId);
    expect(Number(edited.price)).toBe(1200);
    expect(Number(edited.executor_payout)).toBe(1700);
    expect(edited.is_executor_paid).toBe(true);
    expect(edited.executor_paid_at).toBe(paid.executor_paid_at);
    expect(edited.assignee_id).toBe(exec.id);
    await expect(card.subtaskExecutorPaid(subId)).toBeChecked();
    await expect(card.subtaskExecutorPayout(subId)).toHaveValue('1700');

    // снять исполнителя у выплаченной услуги нельзя — 409, карточка откатывается.
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/subtasks/${subId}`) && status === 409);
    await card.chooseCombobox(card.subtaskAssignee(subId), 'Не назначен');
    await expect(page.getByTestId('ui-dialog-text')).toContainText('сначала снимите отметку');
    await page.getByTestId('ui-dialog-ok').click();
    await expect(card.subtaskAssignee(subId)).toContainText(exec.name);
    await expect(card.subtaskExecutorPaid(subId)).toBeChecked();
    expect((await subtaskOf(adminApi, task.id, subId)).assignee_id).toBe(exec.id);

    // сняли отметку — дата исчезла, исполнителя снова можно снять.
    await card.subtaskExecutorPaid(subId).click();
    await expect(card.subtaskExecutorPaid(subId)).not.toBeChecked();
    await expect(card.subtaskExecutorPaidAt(subId)).toHaveText('');
    await expect.poll(async () => (await subtaskOf(adminApi, task.id, subId)).executor_paid_at).toBeNull();
    await card.chooseCombobox(card.subtaskAssignee(subId), 'Не назначен');
    await expect(card.subtaskExecutorPaid(subId)).toBeDisabled();
    await expect.poll(async () => (await subtaskOf(adminApi, task.id, subId)).assignee_id).toBeNull();

    // состояние переживает перезагрузку страницы.
    await page.reload();
    await card.waitOpen();
    await expect(card.subtaskExecutorPayout(subId)).toHaveValue('1700');
    await expect(card.subtaskExecutorPaid(subId)).not.toBeChecked();
  });

  test('EP-002 (TS-021, TS-002): исполнитель сам отмечает выплату в карточке', async ({ page, adminApi, uniq, makeUser, loginAs, insertSubtask }) => {
    const login = uniq('ep002');
    const exec = await makeUser({ login, name: uniq('Исполнитель EP-002') });
    const { client, service, task } = await setup({ adminApi, uniq }, 'EP-002');
    const subId = await insertSubtask({ taskId: task.id, serviceId: service.id, status: 'done' });
    await adminApi.updateSubtask(subId, { status: 'done', assignee_id: exec.id, price: 1000 });

    await loginAs(page, login, exec.password);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();

    await card.subtaskExecutorPayout(subId).fill('800');
    await card.subtaskExecutorPayout(subId).press('Tab');
    await expect.poll(async () => Number((await subtaskOf(adminApi, task.id, subId)).executor_payout)).toBe(800);
    await card.subtaskExecutorPaid(subId).click();
    await expect(card.subtaskExecutorPaid(subId)).toBeChecked();
    await expect.poll(async () => (await subtaskOf(adminApi, task.id, subId)).is_executor_paid).toBe(true);
  });
});

test.describe('EP: карточка не затирает отметку, поставленную из отчёта', () => {
  test('EP-008 (TS-025): карточка открыта → отметка/сумма меняются из отчёта → правка другого поля в карточке их не откатывает', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
  }) => {
    const exec = await makeUser({ login: uniq('ep008'), name: uniq('Исполнитель EP-008') });
    const { client, service, task } = await setup({ adminApi, uniq }, 'EP-008');
    const subId = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(subId, { status: 'done', assignee_id: exec.id, price: 1000, executor_payout: 300 });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.subtaskExecutorPaid(subId)).not.toBeChecked();
    await expect(card.subtaskExecutorPayout(subId)).toHaveValue('300');

    // другой пользователь отмечает выплату в отчёте — открытая карточка об этом не знает.
    await adminApi.markExecutorPayouts([subId], true);
    const marked = await subtaskOf(adminApi, task.id, subId);
    expect(marked.is_executor_paid).toBe(true);

    // правка цены из устаревшей карточки: отметка, дата и сумма целы; карточка после
    // ответа сервера показывает актуальную отметку.
    await card.subtaskPrice(subId).fill('1100');
    await card.subtaskPrice(subId).press('Tab');
    await expect.poll(async () => Number((await subtaskOf(adminApi, task.id, subId)).price)).toBe(1100);
    const afterPrice = await subtaskOf(adminApi, task.id, subId);
    expect(afterPrice.is_executor_paid).toBe(true);
    expect(afterPrice.executor_paid_at).toBe(marked.executor_paid_at);
    expect(Number(afterPrice.executor_payout)).toBe(300);
    await expect(card.subtaskExecutorPaid(subId)).toBeChecked();

    // зеркально: отметку сняли, сумму поменяли в другом месте — правка статуса в карточке
    // не возвращает ни отметку, ни старую сумму.
    await adminApi.markExecutorPayouts([subId], false);
    await adminApi.updateSubtask(subId, { status: 'done', assignee_id: exec.id, price: 1100, executor_payout: 450 });
    await card.chooseCombobox(card.subtaskStatus(subId), 'В работе');
    await expect.poll(async () => (await subtaskOf(adminApi, task.id, subId)).status).toBe('in_progress');
    const afterStatus = await subtaskOf(adminApi, task.id, subId);
    expect(afterStatus.is_executor_paid).toBe(false);
    expect(afterStatus.executor_paid_at).toBeNull();
    expect(Number(afterStatus.executor_payout)).toBe(450);
    await expect(card.subtaskExecutorPaid(subId)).not.toBeChecked();
    await expect(card.subtaskExecutorPayout(subId)).toHaveValue('450');
  });
});

test.describe('EP: доска — фильтр и метка выплаты исполнителю', () => {
  test('EP-003 (TS-022): фильтр «Выплата исполнителю» все / выплачено / не выплачено и метка «к выплате» на карточке', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
  }) => {
    const exec = await makeUser({ login: uniq('ep003'), name: uniq('Исполнитель EP-003') });
    const { client, columns, service, task: unpaidTask } = await setup({ adminApi, uniq }, 'EP-003');
    const colId = columnByName(columns, 'Приёмка').id;
    const paidTask = await adminApi.createTask({ title: uniq('EP-003 выплаченный'), client_id: client.id, column_id: colId });
    const plainTask = await adminApi.createTask({ title: uniq('EP-003 без исполнителя'), client_id: client.id, column_id: colId });

    // unpaidTask: две выполненные услуги с исполнителем без выплаты + одна в работе.
    for (const status of ['done', 'done', 'in_progress'] as const) {
      const id = await insertSubtask({ taskId: unpaidTask.id, serviceId: service.id });
      await adminApi.updateSubtask(id, { status, assignee_id: exec.id });
    }
    // paidTask: услуга с исполнителем выплачена.
    const paidSub = await insertSubtask({ taskId: paidTask.id, serviceId: service.id });
    await adminApi.updateSubtask(paidSub, { status: 'done', assignee_id: exec.id, is_executor_paid: true, executor_payout: 500 });
    // plainTask: услуга без исполнителя — ни «выплачено», ни «не выплачено».
    await insertSubtask({ taskId: plainTask.id, serviceId: service.id, status: 'done' });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.card(unpaidTask.id)).toBeVisible();
    await expect(board.card(paidTask.id)).toBeVisible();
    await expect(board.card(plainTask.id)).toBeVisible();

    // метка: только выполненные услуги с исполнителем без выплаты.
    await expect(board.executorUnpaidBadge(unpaidTask.id)).toHaveText('💸 к выплате: 2');
    await expect(board.executorUnpaidBadge(paidTask.id)).toHaveCount(0);
    await expect(board.executorUnpaidBadge(plainTask.id)).toHaveCount(0);

    await board.chooseCombobox(board.filterExecutorPaid, 'Исполнителю не выплачено');
    await expect(page).toHaveURL(/executor_paid=unpaid/);
    await expect(board.card(unpaidTask.id)).toBeVisible();
    await expect(board.card(paidTask.id)).toHaveCount(0);
    await expect(board.card(plainTask.id)).toHaveCount(0);

    await board.chooseCombobox(board.filterExecutorPaid, 'Исполнителю выплачено');
    await expect(page).toHaveURL(/executor_paid=paid/);
    await expect(board.card(paidTask.id)).toBeVisible();
    await expect(board.card(unpaidTask.id)).toHaveCount(0);
    await expect(board.card(plainTask.id)).toHaveCount(0);

    // фильтр живёт в адресе: перезагрузка сохраняет выбор и выдачу.
    await page.reload();
    await expect(board.card(paidTask.id)).toBeVisible();
    await expect(board.card(unpaidTask.id)).toHaveCount(0);
    await expect(board.filterExecutorPaid).toContainText('Исполнителю выплачено');

    await board.chooseCombobox(board.filterExecutorPaid, 'Выплата исполнителю: все');
    await expect(page).not.toHaveURL(/executor_paid=/);
    await expect(board.card(unpaidTask.id)).toBeVisible();
    await expect(board.card(paidTask.id)).toBeVisible();
    await expect(board.card(plainTask.id)).toBeVisible();

    // «Сбросить» снимает и этот фильтр.
    await board.chooseCombobox(board.filterExecutorPaid, 'Исполнителю не выплачено');
    await expect(board.card(paidTask.id)).toHaveCount(0);
    await board.filterReset.click();
    await expect(page).not.toHaveURL(/executor_paid=/);
    await expect(board.filterExecutorPaid).toContainText('Выплата исполнителю: все');

    // мусор в адресе — не ошибка, фильтр «все».
    await board.goto(`?client_id=${client.id}&executor_paid=zzz`);
    await expect(board.card(unpaidTask.id)).toBeVisible();
    await expect(board.card(paidTask.id)).toBeVisible();

    // отметили выплату — метка уменьшилась, затем исчезла.
    const card = await adminApi.getCard(unpaidTask.id);
    const done = card.subtasks.filter((s) => s.status === 'done').map((s) => s.id);
    await adminApi.markExecutorPayouts([done[0]], true);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.executorUnpaidBadge(unpaidTask.id)).toHaveText('💸 к выплате: 1');
    await adminApi.markExecutorPayouts([done[1]], true);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.executorUnpaidBadge(unpaidTask.id)).toHaveCount(0);
  });
});

test.describe('EP: экран-отчёт «Выплаты»', () => {
  test('EP-004 (TS-023): пункт навигации, группа исполнителя, итоги, фильтры, массовая отметка и снятие', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
  }) => {
    const execA = await makeUser({ login: uniq('ep004a'), name: uniq('Исполнитель EP-004 A') });
    const execB = await makeUser({ login: uniq('ep004b'), name: uniq('Исполнитель EP-004 B') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-004');

    const doneSub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(doneSub, { status: 'done', assignee_id: execA.id, price: 1000, executor_payout: 100 });
    const newSub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(newSub, { status: 'new', assignee_id: execA.id, price: null, executor_payout: 50.5 });
    const noPayoutSub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(noPayoutSub, { status: 'done', assignee_id: execA.id, price: 300 });
    const otherSub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(otherSub, { status: 'done', assignee_id: execB.id, executor_payout: 70, is_executor_paid: true });
    const orphanSub = await insertSubtask({ taskId: task.id, serviceId: service.id, status: 'done' }); // без исполнителя

    // пункт навигации.
    const shell = new ShellPage(page);
    await shell.goto('/');
    await shell.navLink('Выплаты').click();
    await expect(page).toHaveURL(/\/executor-payouts$/);
    const ep = new ExecutorPayoutsPage(page);
    await expect(ep.root).toBeVisible();
    await expect(ep.filterAssignee).toContainText('Все исполнители'); // admin — по умолчанию все

    // группа исполнителя A: строки, поля, итоги.
    await ep.goto(`assignee=${execA.id}`);
    await expect(ep.groups).toHaveCount(1);
    await expect(ep.groupName(execA.id)).toHaveText(execA.name);
    await expect(ep.groupRows(execA.id)).toHaveCount(3);
    await expect(ep.row(orphanSub)).toHaveCount(0);
    await expect(ep.row(otherSub)).toHaveCount(0);
    await expect(ep.row(doneSub).getByTestId('ep-row-task')).toHaveText(`#${task.id}`);
    await expect(ep.row(doneSub).getByTestId('ep-row-service')).toHaveText(service.name);
    await expect(ep.row(doneSub).getByTestId('ep-row-status')).toHaveText('Выполнена');
    await expect(ep.row(doneSub).getByTestId('ep-row-price')).toHaveText('1000.00');
    await expect(ep.rowPayoutInput(doneSub)).toHaveValue('100.00');
    await expect(ep.rowPaid(doneSub)).toContainText('Не выплачено');
    await expect(ep.row(newSub).getByTestId('ep-row-status')).toHaveText('Новая');
    await expect(ep.row(newSub).getByTestId('ep-row-price')).toHaveText('по факту');
    await expect(ep.rowPayoutInput(noPayoutSub)).toHaveValue(''); // сумма не задана
    await expect(ep.groupToPay(execA.id)).toHaveText('150.50'); // 100 + 50.5 + 0 (сумма не задана)
    await expect(ep.groupPaid(execA.id)).toHaveText('0.00');
    await expect(ep.totalToPay).toHaveText('150.50');
    await expect(ep.totalPaid).toHaveText('0.00');

    // без выбора кнопки массовой отметки неактивны.
    await expect(ep.markPaid).toBeDisabled();
    await expect(ep.markUnpaid).toBeDisabled();
    await expect(ep.selectedCount).toHaveText('Выбрано: 0');

    // отметить одну строку выплаченной.
    await ep.rowCheck(doneSub).check();
    await expect(ep.selectedCount).toHaveText('Выбрано: 1');
    await expect(ep.markPaid).toBeEnabled();
    await ep.markPaid.click();
    await expect(ep.rowPaid(doneSub)).toContainText('Выплачено');
    await expect(ep.row(doneSub).getByTestId('ep-row-paid-at')).toHaveText(/\d{2}\.\d{2}\.\d{4}/);
    await expect(ep.groupToPay(execA.id)).toHaveText('50.50');
    await expect(ep.groupPaid(execA.id)).toHaveText('100.00');
    await expect(ep.selectedCount).toHaveText('Выбрано: 0');
    expect((await subtaskOf(adminApi, task.id, doneSub)).is_executor_paid).toBe(true);
    expect((await subtaskOf(adminApi, task.id, newSub)).is_executor_paid).toBe(false);

    // выбрана только уже выплаченная услуга — запрос не уходит, подсказка, выбор сброшен.
    await ep.rowCheck(doneSub).check();
    await ep.markPaid.click();
    await expect(ep.info).toContainText('уже отмечены выплаченными');
    await expect(ep.selectedCount).toHaveText('Выбрано: 0');
    await expect(ep.error).toBeHidden();
    await expect(ep.rowPaid(doneSub)).toContainText('Выплачено');

    // фильтр «Только выполненные».
    await ep.filterOnlyDone.check();
    await expect(page).toHaveURL(/only_done=true/);
    await expect(ep.groupRows(execA.id)).toHaveCount(2);
    await expect(ep.row(newSub)).toHaveCount(0);
    await expect(ep.groupToPay(execA.id)).toHaveText('0.00');

    // + статус «Не выплачено».
    await ep.choose(ep.filterStatus, 'Не выплачено');
    await expect(page).toHaveURL(/status=unpaid/);
    await expect(ep.groupRows(execA.id)).toHaveCount(1);
    await expect(ep.row(noPayoutSub)).toBeVisible();
    await expect(ep.groupPaid(execA.id)).toHaveText('0.00');

    // статус «Выплачено».
    await ep.choose(ep.filterStatus, 'Выплачено');
    await expect(ep.groupRows(execA.id)).toHaveCount(1);
    await expect(ep.row(doneSub)).toBeVisible();

    // фильтры живут в адресе — перезагрузка сохраняет выбор.
    await page.reload();
    await expect(ep.filterOnlyDone).toBeChecked();
    await expect(ep.filterStatus).toContainText('Выплачено');
    await expect(ep.row(doneSub)).toBeVisible();

    // сбросили фильтры; чекбокс группы выбирает все строки → «Снять отметку».
    await ep.filterOnlyDone.uncheck();
    await ep.choose(ep.filterStatus, 'Выплата: все');
    await expect(ep.groupRows(execA.id)).toHaveCount(3);
    await ep.groupCheck(execA.id).check();
    await expect(ep.selectedCount).toHaveText('Выбрано: 3');
    await ep.markPaid.click();
    await expect(ep.groupToPay(execA.id)).toHaveText('0.00');
    await expect(ep.groupPaid(execA.id)).toHaveText('150.50');
    await ep.groupCheck(execA.id).check();
    await ep.markUnpaid.click();
    await expect(ep.groupToPay(execA.id)).toHaveText('150.50');
    await expect(ep.groupPaid(execA.id)).toHaveText('0.00');
    for (const id of [doneSub, newSub, noPayoutSub]) {
      const s = await subtaskOf(adminApi, task.id, id);
      expect(s.is_executor_paid).toBe(false);
      expect(s.executor_paid_at).toBeNull();
    }

    // выбор исполнителя в фильтре: B — своя группа и свои итоги; пустая выдача — заглушка.
    await ep.choose(ep.filterAssignee, execB.name);
    await expect(page).toHaveURL(new RegExp(`assignee=${execB.id}`));
    await expect(ep.groups).toHaveCount(1);
    await expect(ep.groupPaid(execB.id)).toHaveText('70.00');
    await expect(ep.groupToPay(execB.id)).toHaveText('0.00');
    await expect(ep.empty).toBeHidden();
    await ep.choose(ep.filterStatus, 'Не выплачено');
    await expect(ep.groups).toHaveCount(0);
    await expect(ep.empty).toBeVisible();

    // ссылка строки ведёт в карточку заказа на доске.
    await ep.goto(`assignee=${execA.id}`);
    await ep.row(doneSub).getByTestId('ep-row-task').click();
    await expect(page).toHaveURL(new RegExp(`[?&]task=${task.id}\\b`));
    await new OrderCardPage(page).waitOpen();
  });

  test('EP-005 (TS-023, TS-012): исполнитель видит по умолчанию «мои», может переключиться на всех и отметить выплату', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    insertSubtask,
  }) => {
    const login = uniq('ep005me');
    const me = await makeUser({ login, name: uniq('Исполнитель EP-005 Я') });
    const other = await makeUser({ login: uniq('ep005other'), name: uniq('Исполнитель EP-005 Другой') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-005');
    const mine = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(mine, { status: 'done', assignee_id: me.id, executor_payout: 250 });
    const foreign = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(foreign, { status: 'done', assignee_id: other.id, executor_payout: 90 });

    await loginAs(page, login, me.password);
    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.navLink('Выплаты')).toBeVisible(); // пункт виден и исполнителю

    const ep = new ExecutorPayoutsPage(page);
    await ep.goto();
    await expect(ep.filterAssignee).toContainText('Мои');
    await expect(ep.groups).toHaveCount(1);
    await expect(ep.groupName(me.id)).toHaveText(me.name);
    await expect(ep.row(mine)).toBeVisible();
    await expect(ep.row(foreign)).toHaveCount(0);
    await expect(ep.totalToPay).toHaveText('250.00');

    // исполнитель отмечает свою услугу выплаченной.
    await ep.rowCheck(mine).check();
    await ep.markPaid.click();
    await expect(ep.rowPaid(mine)).toContainText('Выплачено');
    await expect(ep.groupPaid(me.id)).toHaveText('250.00');
    expect((await subtaskOf(adminApi, task.id, mine)).is_executor_paid).toBe(true);

    // переключение на «Все исполнители» — видны и чужие услуги.
    await ep.choose(ep.filterAssignee, 'Все исполнители');
    await expect(page).toHaveURL(/assignee=all/);
    await expect(ep.group(other.id)).toBeVisible();
    await expect(ep.row(foreign)).toBeVisible();
    await expect(ep.group(me.id)).toBeVisible();
  });

  test('EP-006 (TS-023, TS-015): массовая отметка атомарна — услуга лишилась исполнителя → ошибка, ничего не отмечено', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const exec = await makeUser({ login: uniq('ep006'), name: uniq('Исполнитель EP-006') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-006');
    const keep = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(keep, { status: 'done', assignee_id: exec.id, executor_payout: 40 });
    const lost = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(lost, { status: 'done', assignee_id: exec.id, executor_payout: 60 });

    const ep = new ExecutorPayoutsPage(page);
    await ep.goto(`assignee=${exec.id}`);
    await ep.groupCheck(exec.id).check();
    await expect(ep.selectedCount).toHaveText('Выбрано: 2');

    // пока список открыт, у одной услуги сняли исполнителя (другая вкладка/пользователь).
    await adminApi.updateSubtask(lost, { status: 'done', assignee_id: null, executor_payout: 60 });

    allowHttpErrors((url, status) => url.endsWith('/api/v1/executor-payouts/mark') && status === 400);
    await ep.markPaid.click();
    await expect(ep.error).toBeVisible();
    await expect(ep.error).toContainText('у услуги нет исполнителя');
    await expect(ep.error).toContainText('Ничего не изменено');

    // ни одна услуга не отмечена; список перечитан — услуга без исполнителя исчезла.
    expect((await subtaskOf(adminApi, task.id, keep)).is_executor_paid).toBe(false);
    expect((await subtaskOf(adminApi, task.id, lost)).is_executor_paid).toBe(false);
    await expect(ep.row(lost)).toHaveCount(0);
    await expect(ep.rowPaid(keep)).toContainText('Не выплачено');
    await expect(ep.groupToPay(exec.id)).toHaveText('40.00');

    // повтор с актуальным списком проходит.
    await ep.rowCheck(keep).check();
    await ep.markPaid.click();
    await expect(ep.rowPaid(keep)).toContainText('Выплачено');
    await expect(ep.error).toBeHidden();
  });

  test('EP-009 (TS-033): сумма выплаты правится прямо в отчёте — сохраняется, итоги пересчитаны, очистка поля → null, выбор и отметка не сбрасываются', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
  }) => {
    const exec = await makeUser({ login: uniq('ep009'), name: uniq('Исполнитель EP-009') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-009');
    const unpaidSub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(unpaidSub, { status: 'done', assignee_id: exec.id, price: 1000, executor_payout: 100 });
    const paidSub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(paidSub, { status: 'done', assignee_id: exec.id, price: 700, is_executor_paid: true });
    const paidBefore = await subtaskOf(adminApi, task.id, paidSub);

    const ep = new ExecutorPayoutsPage(page);
    await ep.goto(`assignee=${exec.id}`);
    await expect(ep.rowPayoutInput(unpaidSub)).toHaveValue('100.00');
    await expect(ep.rowPayoutInput(paidSub)).toHaveValue('');
    await expect(ep.groupToPay(exec.id)).toHaveText('100.00');
    await expect(ep.groupPaid(exec.id)).toHaveText('0.00');

    // выбор чекбокса переживает правку суммы.
    await ep.rowCheck(paidSub).check();
    await expect(ep.selectedCount).toHaveText('Выбрано: 1');

    // Enter сохраняет: значение, итоги группы и общие пересчитаны; отметка не появилась.
    await ep.rowPayoutInput(unpaidSub).fill('250.5');
    await ep.rowPayoutInput(unpaidSub).press('Enter');
    await expect(ep.groupToPay(exec.id)).toHaveText('250.50');
    await expect(ep.totalToPay).toHaveText('250.50');
    await expect(ep.rowPayoutInput(unpaidSub)).toHaveValue('250.50');
    await expect(ep.rowPaid(unpaidSub)).toContainText('Не выплачено');
    const afterUnpaid = await subtaskOf(adminApi, task.id, unpaidSub);
    expect(Number(afterUnpaid.executor_payout)).toBe(250.5);
    expect(afterUnpaid.is_executor_paid).toBe(false);
    expect(Number(afterUnpaid.price)).toBe(1000); // цена для клиента не тронута
    await expect(ep.selectedCount).toHaveText('Выбрано: 1');
    await expect(ep.rowCheck(paidSub)).toBeChecked();
    await expect(page).toHaveURL(new RegExp(`assignee=${exec.id}`)); // фильтр не сброшен

    // уход из поля (blur) тоже сохраняет; у выплаченной услуги отметка и дата целы.
    await ep.rowPayoutInput(paidSub).fill('80');
    await ep.rowPayoutInput(paidSub).press('Tab');
    await expect(ep.groupPaid(exec.id)).toHaveText('80.00');
    await expect(ep.totalPaid).toHaveText('80.00');
    await expect(ep.rowPayoutInput(paidSub)).toHaveValue('80.00');
    const afterPaid = await subtaskOf(adminApi, task.id, paidSub);
    expect(Number(afterPaid.executor_payout)).toBe(80);
    expect(afterPaid.is_executor_paid).toBe(true);
    expect(afterPaid.executor_paid_at).toBe(paidBefore.executor_paid_at);
    expect(Number(afterPaid.price)).toBe(700);

    // невалидная сумма — сообщение, значение откатилось, на сервер не ушло.
    await ep.rowPayoutInput(paidSub).fill('-5');
    await ep.rowPayoutInput(paidSub).press('Enter');
    await expect(ep.error).toBeVisible();
    await expect(ep.error).toContainText('Сумма выплаты не сохранена');
    await expect(ep.rowPayoutInput(paidSub)).toHaveValue('80.00');
    await ep.rowPayoutInput(paidSub).fill('1.234');
    await ep.rowPayoutInput(paidSub).press('Enter');
    await expect(ep.rowPayoutInput(paidSub)).toHaveValue('80.00');
    await ep.rowPayoutInput(paidSub).fill('abc');
    await ep.rowPayoutInput(paidSub).press('Enter');
    await expect(ep.rowPayoutInput(paidSub)).toHaveValue('80.00'); // мусор — откат, НЕ очистка
    await ep.rowPayoutInput(paidSub).fill('99999999999');
    await ep.rowPayoutInput(paidSub).press('Enter');
    await expect(ep.error).toContainText('слишком большая сумма');
    await expect(ep.rowPayoutInput(paidSub)).toHaveValue('80.00');
    expect(Number((await subtaskOf(adminApi, task.id, paidSub)).executor_payout)).toBe(80);

    // поле — текстовое (как в карточке заказа): запятая как десятичный разделитель.
    await expect(ep.rowPayoutInput(paidSub)).toHaveAttribute('type', 'text');
    await ep.rowPayoutInput(unpaidSub).fill('1500,5');
    await ep.rowPayoutInput(unpaidSub).press('Enter');
    await expect(ep.rowPayoutInput(unpaidSub)).toHaveValue('1500.50');
    await expect(ep.groupToPay(exec.id)).toHaveText('1500.50');
    await expect(ep.error).toBeHidden();
    expect(Number((await subtaskOf(adminApi, task.id, unpaidSub)).executor_payout)).toBe(1500.5);

    // очистка поля → сумма не задана (null), в итогах — 0; сообщение об ошибке ушло.
    await ep.rowPayoutInput(unpaidSub).fill('');
    await ep.rowPayoutInput(unpaidSub).press('Enter');
    await expect(ep.groupToPay(exec.id)).toHaveText('0.00');
    await expect(ep.rowPayoutInput(unpaidSub)).toHaveValue('');
    await expect(ep.error).toBeHidden();
    await expect.poll(async () => (await subtaskOf(adminApi, task.id, unpaidSub)).executor_payout).toBeNull();

    // значение переживает перезагрузку страницы.
    await ep.rowPayoutInput(unpaidSub).fill('12.34');
    await ep.rowPayoutInput(unpaidSub).press('Enter');
    await expect(ep.groupToPay(exec.id)).toHaveText('12.34');
    await ep.goto(`assignee=${exec.id}`);
    await expect(ep.rowPayoutInput(unpaidSub)).toHaveValue('12.34');
    await expect(ep.rowPayoutInput(paidSub)).toHaveValue('80.00');
    await expect(ep.groupToPay(exec.id)).toHaveText('12.34');
    await expect(ep.groupPaid(exec.id)).toHaveText('80.00');

    // массовая отметка после правки сумм работает с актуальными строками.
    await ep.rowCheck(unpaidSub).check();
    await ep.markPaid.click();
    await expect(ep.groupPaid(exec.id)).toHaveText('92.34');
    await expect(ep.groupToPay(exec.id)).toHaveText('0.00');
  });

  test('EP-010 (TS-033, TS-002): исполнитель правит сумму выплаты в отчёте', async ({ page, adminApi, uniq, makeUser, loginAs, insertSubtask }) => {
    const login = uniq('ep010');
    const me = await makeUser({ login, name: uniq('Исполнитель EP-010') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-010');
    const sub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(sub, { status: 'done', assignee_id: me.id });

    await loginAs(page, login, me.password);
    const ep = new ExecutorPayoutsPage(page);
    await ep.goto();
    await ep.rowPayoutInput(sub).fill('333');
    await ep.rowPayoutInput(sub).press('Enter');
    await expect(ep.groupToPay(me.id)).toHaveText('333.00');
    expect(Number((await subtaskOf(adminApi, task.id, sub)).executor_payout)).toBe(333);
  });

  test('EP-011 (TS-034): сохранение суммы не пересоздаёт остальные строки — ввод и фокус в соседнем поле не теряются', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
  }) => {
    const exec = await makeUser({ login: uniq('ep011'), name: uniq('Исполнитель EP-011') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-011');
    const subA = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(subA, { status: 'done', assignee_id: exec.id });
    const subB = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(subB, { status: 'done', assignee_id: exec.id });

    const ep = new ExecutorPayoutsPage(page);
    await ep.goto(`assignee=${exec.id}`);

    // PUT суммы строки A придерживаем, пока пользователь уже печатает в строке B.
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => { release = resolve; });
    await page.route(`**/api/v1/executor-payouts/${subA}`, async (route) => {
      await held;
      await route.continue();
    });
    const savedA = page.waitForResponse((r) => r.url().endsWith(`/api/v1/executor-payouts/${subA}`) && r.request().method() === 'PUT');
    const totalsAfterA = page.waitForResponse((r) => r.url().includes('/api/v1/executor-payouts/summary') && r.status() === 200);

    await ep.rowPayoutInput(subA).fill('111');
    await ep.rowPayoutInput(subB).click(); // blur A → PUT (летит); фокус в B
    await expect(ep.rowPayoutInput(subA)).toBeDisabled();
    await page.keyboard.type('22');

    release();
    expect((await savedA).status()).toBe(200);
    await totalsAfterA;
    await expect(ep.groupToPay(exec.id)).toHaveText('111.00'); // итоги обновлены точечно

    // ответ пришёл — поле B не пересоздано: фокус на месте, набранное цело, можно печатать дальше.
    await expect(ep.rowPayoutInput(subB)).toBeFocused();
    await expect(ep.rowPayoutInput(subB)).toHaveValue('22');
    await page.keyboard.type('2');
    await expect(ep.rowPayoutInput(subB)).toHaveValue('222');
    await expect(ep.rowPayoutInput(subA)).toHaveValue('111.00');
    await expect(ep.rowPayoutInput(subA)).toBeEnabled();

    await page.keyboard.press('Enter'); // уход из B → сохранение
    await expect(ep.groupToPay(exec.id)).toHaveText('333.00');
    await expect(ep.totalToPay).toHaveText('333.00');
    await expect(ep.rowPayoutInput(subB)).toHaveValue('222.00');
    expect(Number((await subtaskOf(adminApi, task.id, subA)).executor_payout)).toBe(111);
    expect(Number((await subtaskOf(adminApi, task.id, subB)).executor_payout)).toBe(222);
  });

  test('EP-012 (TS-034): Tab из поля суммы — фокус уходит на следующий элемент и остаётся там после сохранения', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    insertSubtask,
  }) => {
    const exec = await makeUser({ login: uniq('ep012'), name: uniq('Исполнитель EP-012') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-012');
    const subA = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(subA, { status: 'done', assignee_id: exec.id });
    const subB = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(subB, { status: 'done', assignee_id: exec.id });

    const ep = new ExecutorPayoutsPage(page);
    await ep.goto(`assignee=${exec.id}`);

    // Tab из суммы A: следующий фокусируемый элемент — чекбокс строки B.
    const totalsAfterA = page.waitForResponse((r) => r.url().includes('/api/v1/executor-payouts/summary') && r.status() === 200);
    await ep.rowPayoutInput(subA).fill('40');
    await ep.rowPayoutInput(subA).press('Tab');
    await expect(ep.rowCheck(subB)).toBeFocused();
    await totalsAfterA;
    await expect(ep.groupToPay(exec.id)).toHaveText('40.00');
    await expect(ep.rowCheck(subB)).toBeFocused(); // строки не пересозданы — фокус не сброшен

    // дальше с клавиатуры: пробел отмечает строку B, Tab-ы доводят до её суммы.
    await page.keyboard.press('Space');
    await expect(ep.selectedCount).toHaveText('Выбрано: 1');
    await page.keyboard.press('Tab'); // ссылка на заказ
    await page.keyboard.press('Tab'); // сумма B
    await expect(ep.rowPayoutInput(subB)).toBeFocused();
    await page.keyboard.type('60,5');
    await page.keyboard.press('Tab');
    await expect(ep.groupToPay(exec.id)).toHaveText('100.50');
    await expect(ep.rowPayoutInput(subB)).toHaveValue('60.50');
    await expect(ep.selectedCount).toHaveText('Выбрано: 1'); // выбор цел
    await expect(ep.rowCheck(subB)).toBeChecked();
    expect(Number((await subtaskOf(adminApi, task.id, subA)).executor_payout)).toBe(40);
    expect(Number((await subtaskOf(adminApi, task.id, subB)).executor_payout)).toBe(60.5);
  });

  test('EP-007 (TS-023): экран не скроллится вбок на узком вьюпорте', async ({ adminPage: page, adminApi, uniq, makeUser, insertSubtask }) => {
    const exec = await makeUser({ login: uniq('ep007'), name: uniq('Исполнитель EP-007') });
    const { service, task } = await setup({ adminApi, uniq }, 'EP-007');
    const sub = await insertSubtask({ taskId: task.id, serviceId: service.id });
    await adminApi.updateSubtask(sub, { status: 'done', assignee_id: exec.id, executor_payout: 10 });

    await page.setViewportSize({ width: 375, height: 700 });
    const ep = new ExecutorPayoutsPage(page);
    await ep.goto(`assignee=${exec.id}`);
    await expect(ep.row(sub)).toBeVisible();
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth))
      .toBeLessThanOrEqual(0);
  });
});
