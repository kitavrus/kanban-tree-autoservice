import { test, expect } from '../fixtures/board';
import type { AdminApi } from '../fixtures/auth';
import { BoardPage } from '../pages/BoardPage';
import { DeletedPage } from '../pages/DeletedPage';

// Мягкое удаление — колонки доски (docs/features/soft-delete-board-columns/draft-spec.md §8.1,
// §8.2, §11): удалить колонку с причиной и восстановить из «Удалённых» (TS-042), восстановить
// заказ удалённой колонки (TS-042, DS-016). Адаптив и темы вкладки «Колонки» — расширением
// SD-007 / SD-008 (soft-delete-core.spec.ts); управление колонками у исполнителя недоступно —
// BD-006 (board.spec.ts). Общая dev-БД (§TD-6.2): свои колонки, клиенты и заказы через
// uniq()+adminApi. Уборка — _boardCleanup: заказы (purgeTask), затем колонки служебным адресом
// purge — и действующие, и мягко удалённые (стенд с TEST_PURGE_ENABLED=true).
test.use({ colorScheme: 'light' });

const DATE_TIME = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;
const TOAST_DELETED = 'Удалено. Восстановить можно в разделе «Удалённые»';

type Uniq = (s: string) => string;

/** Своя колонка теста в конце доски; название — не длиннее 60 символов. */
async function newColumn(adminApi: AdminApi, uniq: Uniq, label: string, isFinal = false) {
  const res = await adminApi.createColumn({ name: uniq(label).slice(0, 60), color: '#445566', is_final: isFinal, consumes_stock: false });
  return res.column;
}

test.describe('SDC: мягкое удаление колонок доски', () => {
  test('SDC-001 (TS-042): «Удалить» в меню колонки → окно «Удаление»: пустая причина → «Укажите причину удаления», колонка на месте; с причиной → колонка исчезла с доски, тост «Удалено…»; в «Удалённых» вкладка «Колонки» — название, причина, кто удалил, время; «Восстановить» → тост «Восстановлено. Колонка — в конце доски», колонка последняя на доске; негатив: колонка с заказом — «В колонке есть заказы», окно остаётся', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const empty = await newColumn(adminApi, uniq, 'SDC-001 пустая');
    const busy = await newColumn(adminApi, uniq, 'SDC-001 занятая');
    const client = await adminApi.createClient({ company_name: uniq('SDC-001 клиент'), phones: [{ phone: '9990000901', note: '' }] });
    await adminApi.createTask({ title: uniq('SDC-001 заказ'), client_id: client.id, column_id: busy.id });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/columns/${empty.id}`) && status === 400);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/columns/${busy.id}`) && status === 409);

    const board = new BoardPage(page);
    await board.goto();
    await expect(board.column(empty.id)).toBeVisible();

    // окно «Удаление»: заголовок с названием, пояснение; пустая причина — ошибка под полем.
    await board.openDeleteColumn(empty.id);
    await expect(board.deleteDialogTitle).toHaveText(`Удалить «${empty.name}»?`);
    await expect(board.deleteDialogText).toHaveText('Колонку можно восстановить в разделе «Удалённые»');
    await board.deleteDialogOk.click();
    await expect(board.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(board.deleteDialog).toBeVisible();
    await expect(board.column(empty.id)).toBeVisible();

    // с причиной — колонка уходит с доски.
    const reason = uniq('SDC-001 лишний этап');
    await board.deleteReason.fill(reason);
    await board.deleteDialogOk.click();
    await expect(board.toast).toHaveText(TOAST_DELETED);
    await expect(board.deleteDialog).toBeHidden();
    await expect(board.column(empty.id)).toHaveCount(0);

    // негатив: колонка с действующим заказом — отказ красным блоком, окно остаётся, колонка на месте.
    await board.openDeleteColumn(busy.id);
    await board.deleteReason.fill(uniq('SDC-001 попытка'));
    await board.deleteDialogOk.click();
    await expect(board.deleteDialogError).toHaveText('В колонке есть заказы');
    await expect(board.deleteDialog).toBeVisible();
    await board.deleteDialogCancel.click();
    await expect(board.deleteDialog).toBeHidden();
    await expect(board.column(busy.id)).toBeVisible();

    // «Удалённые» → вкладка «Колонки»: строка удалённой колонки.
    const del = new DeletedPage(page);
    await del.goto(`kind=columns&q=${encodeURIComponent(empty.name)}`);
    await expect(del.tab('columns')).toHaveClass(/active/);
    await expect(del.tab('columns')).toContainText('Колонки');
    await expect(del.rows).toHaveCount(1);
    await expect(del.rowTitle(empty.id)).toHaveText(empty.name);
    await expect(del.rowSubtitle(empty.id)).toHaveCount(0); // нефинальная — без подписи
    await expect(del.rowReason(empty.id)).toHaveText(reason);
    await expect(del.rowBy(empty.id)).toHaveText('Админ Прогона');
    await expect(del.rowAt(empty.id)).toHaveText(DATE_TIME);
    const summary = await adminApi.deletedSummary();
    await expect(del.tabCount('columns')).toHaveText(String(summary.counts.columns));

    // «Восстановить» → подтверждение → строка исчезла, счётчик уменьшился, колонка в конце доски.
    await del.restoreBtn(empty.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «${empty.name}»?`);
    await del.dialogOk.click();
    await expect(del.toast).toHaveText('Восстановлено. Колонка — в конце доски');
    await expect(del.row(empty.id)).toHaveCount(0);
    await expect(del.empty).toBeVisible();
    await expect(del.tabCount('columns')).toHaveText(String(summary.counts.columns - 1));
    expect((await adminApi.listDeleted('columns', { q: empty.name })).total).toBe(0);

    const columns = (await adminApi.listColumns()).items;
    expect(columns[columns.length - 1].id, 'восстановленная колонка — последняя').toBe(empty.id);
    await board.goto();
    await expect(board.columns.last()).toHaveAttribute('data-testid', `board-column-${empty.id}`);
    await expect(board.columnByName(empty.name)).toBeVisible();
  });

  test('SDC-002 (TS-042): восстановление заказа, колонку которого удалили, — жёлтый блок «Колонка заказа удалена — заказ поставлен в колонку «…»» со ссылкой «Открыть заказ #N», заказ на доске в первой нефинальной колонке; негатив: сама колонка остаётся в «Удалённых» и на доске не появляется', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const col = await newColumn(adminApi, uniq, 'SDC-002 колонка');
    const client = await adminApi.createClient({ company_name: uniq('SDC-002 клиент'), phones: [{ phone: '9990000902', note: '' }] });
    const task = await adminApi.createTask({ title: uniq('SDC-002 заказ'), client_id: client.id, column_id: col.id });
    await adminApi.deleteTask(task.id, uniq('SDC-002 причина заказа'));
    // удалённый заказ колонку не удерживает (DS-019): колонка удаляется.
    await adminApi.deleteColumn(col.id, uniq('SDC-002 причина колонки'));
    const first = (await adminApi.listColumns()).items.find((c) => !c.is_final);
    expect(first, 'на доске есть нефинальная колонка').toBeTruthy();

    const del = new DeletedPage(page);
    await del.goto(`kind=tasks&q=${encodeURIComponent(`#${task.id}`)}`);
    await expect(del.row(task.id)).toBeVisible();
    await del.restore(task.id);
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.warnings).toBeVisible();
    await expect(del.warningLines).toHaveText([`Колонка заказа удалена — заказ поставлен в колонку «${first!.name}»`]);
    await expect(del.warningOpen).toHaveText(`Открыть заказ #${task.id}`);
    await expect(del.warningOpen).toHaveAttribute('href', `/?task=${task.id}`);
    await expect(del.row(task.id)).toHaveCount(0);

    // негатив: колонка заказа сама не восстановлена.
    expect((await adminApi.listDeleted('columns', { q: col.name })).total).toBe(1);
    await del.tab('columns').click();
    await del.search(col.name);
    await expect(del.row(col.id)).toBeVisible();

    // заказ — на доске, в первой нефинальной колонке; удалённой колонки на доске нет.
    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.column(first!.id).getByTestId(`task-card-${task.id}`)).toBeVisible();
    await expect(board.column(col.id)).toHaveCount(0);
  });

  test('SDC-003 (TS-042): колонку удалил другой администратор, пока доска открыта, — «Удалить» в окне → тост «Колонка не найдена», окно закрыто, колонка исчезла с доски; перенос заказа в удалённую колонку через меню → тост «Колонка не найдена», заказ остаётся в своей колонке', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const gone = await newColumn(adminApi, uniq, 'SDC-003 удалят при открытом окне');
    const target = await newColumn(adminApi, uniq, 'SDC-003 удалят при открытой доске');
    const home = (await adminApi.listColumns()).items.find((c) => !c.is_final)!;
    const client = await adminApi.createClient({ company_name: uniq('SDC-003 клиент'), phones: [{ phone: '9990000903', note: '' }] });
    const task = await adminApi.createTask({ title: uniq('SDC-003 заказ'), client_id: client.id, column_id: home.id });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/columns/${gone.id}`) && status === 404);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}/move`) && status === 404);

    // ---- удаление уже удалённой колонки: 404 → тост, доска перечитана ----
    const board = new BoardPage(page);
    await board.goto();
    await board.openDeleteColumn(gone.id);
    await adminApi.deleteColumn(gone.id, uniq('SDC-003 удалил другой администратор'));
    await board.deleteReason.fill(uniq('SDC-003 лишний этап'));
    await board.deleteDialogOk.click();
    await expect(board.toastError).toHaveText('Колонка не найдена');
    await expect(board.deleteDialog).toBeHidden();
    await expect(board.column(gone.id), 'доска перечитана — колонки нет').toHaveCount(0);
    expect((await adminApi.listDeleted('columns', { q: gone.name })).total, 'колонка удалена один раз').toBe(1);

    // ---- перенос заказа в колонку, удалённую при открытой доске ----
    await board.goto(`?client_id=${client.id}`);
    await expect(board.column(home.id).getByTestId(`task-card-${task.id}`)).toBeVisible();
    await expect(board.column(target.id)).toBeVisible();
    await adminApi.deleteColumn(target.id, uniq('SDC-003 удалил другой администратор'));
    await board.moveViaMenu(task.id, target.id);
    await expect(board.toastError).toHaveText('Колонка не найдена');
    await expect(board.column(home.id).getByTestId(`task-card-${task.id}`), 'карточка возвращена на место').toBeVisible();
    await expect(page.getByTestId('ui-dialog'), 'общего окна ошибки нет').toHaveCount(0);
    expect((await adminApi.getTask(task.id)).column_id, 'заказ остался в своей колонке').toBe(home.id);
  });

  // Отказ 409 LAST_NON_FINAL_COLUMN настоящим запросом на общем стенде не получить: для него
  // пришлось бы удалить либо сделать финальными все нефинальные колонки сида, на которых стоят
  // остальные спеки (§TD-6.2 — тест трогает только свои записи). Сам отказ сервера покрыт
  // live-тестами API; здесь проверяется ветка экрана — ответ DELETE подменён точным wire-форматом
  // pkg/errorspkg (код и текст — internal/features/board/errors.go).
  test('SDC-004 (TS-042): отказ «последняя нефинальная» в окне удаления — красный блок «Нужна хотя бы одна нефинальная колонка», окно и причина остаются, колонка на месте; «Отмена» закрывает окно', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const col = await newColumn(adminApi, uniq, 'SDC-004 последняя нефинальная');
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/columns/${col.id}`) && status === 409);
    await page.route(`**/api/v1/columns/${col.id}`, async (route) => {
      if (route.request().method() !== 'DELETE') return route.continue();
      await route.fulfill({
        status: 409, contentType: 'application/json',
        body: JSON.stringify({ code: 'LAST_NON_FINAL_COLUMN', message: 'Нужна хотя бы одна нефинальная колонка' }),
      });
    });

    const board = new BoardPage(page);
    await board.goto();
    await board.openDeleteColumn(col.id);
    const reason = uniq('SDC-004 лишний этап');
    await board.deleteReason.fill(reason);
    await board.deleteDialogOk.click();
    await expect(board.deleteDialogError).toHaveText('Нужна хотя бы одна нефинальная колонка');
    await expect(board.deleteDialog).toBeVisible();
    await expect(board.deleteReason).toHaveValue(reason);
    await expect(board.deleteDialogOk).toBeEnabled();
    await board.deleteDialogCancel.click();
    await expect(board.deleteDialog).toBeHidden();
    await expect(board.column(col.id)).toBeVisible();
    await expect(board.toastError).toHaveCount(0);
    expect((await adminApi.listColumns()).items.map((c) => c.id), 'колонка на месте').toContain(col.id);
  });
});
