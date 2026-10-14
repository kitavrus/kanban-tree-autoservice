import { test, expect, columnByName } from '../fixtures/board';
import { AdminApi } from '../fixtures/auth';
import { contrastOf } from '../fixtures/contrast';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { DeletedPage } from '../pages/DeletedPage';
import { ShellPage } from '../pages/ShellPage';

// Мягкое удаление — ядро (docs/features/soft-delete-core/draft-spec.md §8.1–§8.3, §11):
// удаление заказа с причиной (TS-034), раздел «Удалённые» (TS-035), восстановление (TS-036,
// TS-037), фильтры (TS-038), прямые ссылки на удалённый заказ (TS-039), адаптив (TS-046),
// темы (TS-047). Общая dev-БД (§TD-6.2): свои клиенты, заказы, товары через uniq()+adminApi;
// заказы (и действующие, и мягко удалённые) физически убирает _boardCleanup (purgeTask —
// служебный адрес, стенд с TEST_PURGE_ENABLED=true).
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;
const DATE_TIME = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;
const TOAST_DELETED = 'Удалено. Восстановить можно в разделе «Удалённые»';
const PNG_1X1 = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==', 'base64');

type Uniq = (s: string) => string;

/** Клиент + заказ в колонке columnName. */
async function order(adminApi: AdminApi, uniq: Uniq, tag: string, columnName = 'Новые') {
  const client = await adminApi.createClient({ company_name: uniq(`${tag} клиент`), phones: [{ phone: '9990000701', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  const column = columnByName(columns, columnName);
  const task = await adminApi.createTask({ title: uniq(`${tag} заказ`), client_id: client.id, column_id: column.id });
  return { client, columns, column, task };
}

/** Товар с остатком stock + услуга каталога. */
async function goods(adminApi: AdminApi, uniq: Uniq, tag: string, stock: number) {
  const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq(`${tag} категория товаров`) });
  const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq(`${tag} товар`), stock });
  const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq(`${tag} категория услуг`) });
  const service = await adminApi.createService({ category_id: svcCat.id, name: uniq(`${tag} услуга`) });
  return { product, service };
}

async function stockOf(adminApi: AdminApi, productName: string): Promise<number> {
  const list = await adminApi.getStock({ q: productName, scope: 'all' });
  return Number(list.items[0]?.stock ?? NaN);
}

/** «Сегодня» (ГГГГ-ММ-ДД) в поясе zone, сдвиг в днях. */
function dayIn(zone: string, shiftDays = 0): string {
  const now = new Date(Date.now() + shiftDays * 86_400_000);
  return new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(now);
}

test.describe('SD: мягкое удаление заказа и раздел «Удалённые»', () => {
  test('SD-001 (TS-034): «Удалить» в карточке → окно «Удаление»: пустая причина → «Укажите причину удаления», заказ на месте; «Отмена» причину не сохраняет; с причиной → карточка закрылась, заказа нет на доске, тост «Удалено…», заказ в «Удалённых»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { client, task } = await order(adminApi, uniq, 'SD-001');
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    // окно: заголовок с номером и названием, пояснение, счётчик символов.
    const text = await card.openDeleteTask();
    await expect(card.deleteDialogTitle).toHaveText(`Удалить «#${task.id} ${task.title}»?`);
    await expect(text).toHaveText('Товары заказа вернутся на склад. Заказ можно восстановить в разделе «Удалённые».');
    await expect(card.deleteReasonCounter).toHaveText('0 / 500');

    // негатив: пустая причина и причина из пробелов — проверяет сервер, окно остаётся.
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}`) && status === 400);
    await card.deleteDialogOk.click();
    await expect(card.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(card.deleteReason).toHaveClass(/is-invalid/);
    await card.submitDelete('   ');
    await expect(card.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(card.deleteDialog).toBeVisible();
    expect(await adminApi.taskHttpStatus(task.id), 'заказ на месте').toBe(200);

    // ввод снимает ошибку; длиннее 500 символов — отказ сервера под полем.
    await card.deleteReason.fill('я'.repeat(501));
    await expect(card.deleteReasonCounter).toHaveText('501 / 500');
    await expect(card.deleteReason).not.toHaveClass(/is-invalid/);
    await card.deleteDialogOk.click();
    await expect(card.deleteReasonError).toHaveText('Причина — не длиннее 500 символов');

    // «Отмена» — окно закрылось, причина не сохранена, заказ на месте.
    await card.cancelUiDialog();
    await expect(card.deleteDialog).toBeHidden();
    await expect(card.modal).toBeVisible();
    await card.openDeleteTask();
    await expect(card.deleteReason).toHaveValue('');
    expect(await adminApi.taskHttpStatus(task.id)).toBe(200);

    // с причиной — заказ удалён.
    await card.submitDelete('  Создан по ошибке  ');
    await expect(card.deleteDialog).toBeHidden();
    await expect(card.modal).toBeHidden();
    await expect(page.getByTestId('toast')).toHaveText(TOAST_DELETED);
    await expect(board.card(task.id)).toHaveCount(0);
    expect(await adminApi.taskHttpStatus(task.id), 'заказ удалён').toBe(404);

    const deleted = await adminApi.listDeleted('tasks', { q: `#${task.id}` });
    expect(deleted.items.map((i) => i.id)).toEqual([String(task.id)]);
    expect(deleted.items[0].delete_reason, 'крайние пробелы обрезаны').toBe('Создан по ошибке');
  });

  test('SD-002 (TS-035): меню «Удалённые» → вкладка «Заказы» со счётчиком; строка — «#N название», клиент, причина, кто удалил, время «ДД.ММ.ГГГГ ЧЧ:ММ»; негатив: исполнителю — «Нет доступа» и пункта меню нет', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    const { client, task } = await order(adminApi, uniq, 'SD-002');
    const reason = uniq('SD-002 причина: дубль заказа');
    await adminApi.deleteTask(task.id, reason);

    const shell = new ShellPage(page);
    const del = new DeletedPage(page);
    await shell.goto('/');
    await shell.navLink('Удалённые').click();
    await expect(page).toHaveURL(/\/deleted(\?|$)/);
    await expect(del.root).toBeVisible();
    await del.loaded();
    await expect(shell.navLink('Удалённые')).toHaveAttribute('aria-current', 'page');
    await expect(page).toHaveURL(/kind=tasks/);

    // вкладка «Заказы» активна, счётчик совпадает с API.
    await expect(del.tab('tasks')).toHaveClass(/active/);
    await expect(del.tab('tasks')).toContainText('Заказы');
    const summary = await adminApi.deletedSummary();
    await expect(del.tabCount('tasks')).toHaveText(String(summary.counts.tasks));
    expect(summary.counts.tasks).toBeGreaterThan(0);

    // строка заказа (поиск по номеру — на общей БД в разделе бывают чужие записи).
    await del.search(`#${task.id}`);
    await expect(del.rows).toHaveCount(1);
    await expect(del.rowTitle(task.id)).toHaveText(`#${task.id} ${task.title}`);
    await expect(del.rowSubtitle(task.id)).toHaveText(client.company_name);
    await expect(del.rowReason(task.id)).toHaveText(reason);
    await expect(del.rowBy(task.id)).toHaveText('Админ Прогона');
    await expect(del.rowAt(task.id)).toHaveText(DATE_TIME);
    await expect(del.restoreBtn(task.id)).toHaveText('Восстановить');
    await expect(del.shown).toHaveText('Показано 1–1 из 1');

    // негатив: исполнитель — страница отказа, пункта меню нет.
    const login = uniq('sd002-exec');
    const exec = await makeUser({ login });
    await loginAs(page, login, exec.password);
    allowHttpErrors((url, status) => url.includes('/deleted') && status === 403);
    const res = await page.goto('/deleted');
    expect(res?.status()).toBe(403);
    await expect(page.locator('body')).toContainText('Нет доступа');
    await expect(del.root).toHaveCount(0);
    await shell.goto('/');
    await expect(shell.navLink('Удалённые')).toHaveCount(0);
    for (const path of ['/api/v1/deleted?kind=tasks', '/api/v1/deleted/summary']) {
      const api = await page.request.get(path);
      expect(api.status(), path).toBe(403);
      expect((await api.json()).message, path).toBe('Нет доступа');
    }
  });

  test('SD-003 (TS-036): «Восстановить» → подтверждение → строка исчезла, счётчик уменьшился, тост «Восстановлено», заказ снова на доске в своей колонке, остатки как до удаления; негатив: повтор во второй вкладке → «Запись уже восстановлена»', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { client, columns, task } = await order(adminApi, uniq, 'SD-003');
    const work = columnByName(columns, 'В работе');
    const { product, service } = await goods(adminApi, uniq, 'SD-003', 5);
    await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 2 }] });
    await adminApi.moveTask(task.id, { column_id: work.id });
    expect(await stockOf(adminApi, product.name), 'товар списан').toBe(3);
    await adminApi.deleteTask(task.id, uniq('SD-003 причина'));
    expect(await stockOf(adminApi, product.name), 'удаление вернуло товар').toBe(5);

    const del = new DeletedPage(page);
    await del.goto(`kind=tasks&q=${encodeURIComponent(`#${task.id}`)}`);
    await expect(del.row(task.id)).toBeVisible();
    const countBefore = Number(await del.tabCount('tasks').textContent());

    // вторая вкладка открыта ДО восстановления — у неё строка ещё в списке.
    const second = await page.context().newPage();
    const del2 = new DeletedPage(second);
    await del2.goto(`kind=tasks&q=${encodeURIComponent(`#${task.id}`)}`);
    await expect(del2.row(task.id)).toBeVisible();

    // «Отмена» в подтверждении ничего не делает.
    await del.restoreBtn(task.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «#${task.id} ${task.title}»?`);
    await del.dialogCancel.click();
    await expect(del.dialog).toBeHidden();
    await expect(del.row(task.id)).toBeVisible();

    await del.restore(task.id);
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.row(task.id)).toHaveCount(0);
    await expect(del.empty).toHaveText('Ничего не найдено');
    await expect(del.tabCount('tasks')).toHaveText(String(countBefore - 1));
    await expect(del.warnings).toBeHidden(); // товара хватает — предупреждений нет

    expect((await adminApi.getTask(task.id)).column_id, 'заказ в своей колонке').toBe(work.id);
    expect(await stockOf(adminApi, product.name), 'товар списан снова').toBe(3);
    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.column(work.id).getByTestId(`task-card-${task.id}`)).toBeVisible();

    // негатив: повторное восстановление во второй вкладке — 409, строка исчезает.
    await del2.restore(task.id);
    await expect(del2.toastError).toHaveText('Запись уже восстановлена');
    await expect(del2.row(task.id)).toHaveCount(0);
    expect(await stockOf(adminApi, product.name), 'повтор остаток не меняет').toBe(3);
    await second.close();
  });

  test('SD-004 (TS-037): восстановление при нехватке товара — жёлтый блок с товаром и остатком, ссылка «Открыть заказ #N»; блок не исчезает сам и закрывается крестиком; негатив: заказ всё равно восстановлен', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const { columns, task } = await order(adminApi, uniq, 'SD-004');
    const { product, service } = await goods(adminApi, uniq, 'SD-004', 5);
    await insertSubtask({ taskId: task.id, serviceId: service.id, products: [{ productId: product.id, qty: 2 }] });
    await adminApi.moveTask(task.id, { column_id: columnByName(columns, 'В работе').id });
    await adminApi.deleteTask(task.id, uniq('SD-004 причина'));
    await adminApi.adjustStock(product.id, 0); // товар ушёл со склада, пока заказ был удалён

    const del = new DeletedPage(page);
    await del.goto(`kind=tasks&q=${encodeURIComponent(`#${task.id}`)}`);
    await del.restore(task.id);
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.row(task.id)).toHaveCount(0);

    await expect(del.warnings).toBeVisible();
    await expect(del.warnings).toContainText(`Восстановлено: #${task.id} ${task.title}`);
    await expect(del.warningLines).toHaveCount(1);
    await expect(del.warningLines).toHaveText(`Не хватает товара «${product.name}»: остаток −2`);
    await expect(del.warningOpen).toHaveText(`Открыть заказ #${task.id}`);
    await expect(del.warningOpen).toHaveAttribute('href', `/?task=${task.id}`);

    // негатив: восстановление не отклонено — заказ действует, остаток ушёл в минус.
    expect(await adminApi.taskHttpStatus(task.id), 'заказ восстановлен').toBe(200);
    expect(await stockOf(adminApi, product.name)).toBe(-2);

    // блок не исчезает сам (тост живёт 5 секунд) и закрывается крестиком.
    await expect(del.toast).toHaveCount(0, { timeout: 8000 });
    await expect(del.warnings).toBeVisible();
    await del.warningsClose.click();
    await expect(del.warnings).toBeHidden();

    // ссылка из блока открывает карточку восстановленного заказа.
    await del.goto(`kind=tasks&q=${encodeURIComponent(`#${task.id}`)}`);
    await page.goto(`/?task=${task.id}`);
    await new OrderCardPage(page).waitOpen();
  });

  test('SD-005 (TS-038): фильтры — поиск по причине, период, «Кто удалил», «Сбросить»; список сужается и возвращается, фильтры в адресной строке и переживают перезагрузку; негатив: «по» раньше «с» → «Проверьте период»', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    allowHttpErrors,
  }) => {
    const { client, columns } = await order(adminApi, uniq, 'SD-005');
    const col = columnByName(columns, 'Новые').id;
    const mark = uniq('sd005');
    const mine = await adminApi.createTask({ title: uniq('SD-005 мой'), client_id: client.id, column_id: col });
    const dup = await adminApi.createTask({ title: uniq('SD-005 дубль'), client_id: client.id, column_id: col });
    const other = await adminApi.createTask({ title: uniq('SD-005 чужой'), client_id: client.id, column_id: col });
    await adminApi.deleteTask(mine.id, `${mark} создан по ошибке`);
    await adminApi.deleteTask(dup.id, `${mark} ДУБЛЬ заказа`);

    // третий заказ удаляет другой администратор.
    const otherLogin = uniq('sd005-admin');
    const otherAdmin = await makeUser({ login: otherLogin, name: uniq('Второй Админ SD-005'), role: 'admin' });
    const otherApi = await AdminApi.login(otherLogin, otherAdmin.password);
    await otherApi.deleteTask(other.id, `${mark} клиент отказался`);
    await otherApi.dispose();

    const del = new DeletedPage(page);
    await del.goto('kind=tasks');
    for (const t of [mine, dup, other]) await expect(del.row(t.id)).toBeVisible();

    // поиск по причине (без учёта регистра) — в адресной строке.
    await del.search(`${mark} дубль`);
    await expect(del.rows).toHaveCount(1);
    await expect(del.row(dup.id)).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`q=${encodeURIComponent(`${mark} дубль`).replace(/%20/g, '(%20|\\+)')}`));
    await del.search(mark);
    await expect(del.rows).toHaveCount(3);

    // период по поясу пользователя (Asia/Almaty): сегодня — все три; вчера — ничего.
    const today = dayIn('Asia/Almaty');
    const yesterday = dayIn('Asia/Almaty', -1);
    await del.filterFrom.fill(today);
    await del.filterTo.fill(today);
    await expect(page).toHaveURL(new RegExp(`deleted_from=${today}.*deleted_to=${today}`));
    await expect(del.rows).toHaveCount(3);
    await del.filterFrom.fill(yesterday);
    await del.filterTo.fill(yesterday);
    await expect(del.empty).toHaveText('Ничего не найдено');
    await expect(del.rows).toHaveCount(0);

    // негатив: «по» раньше «с» → текст под полями, таблица пустая.
    allowHttpErrors((url, status) => url.includes('/api/v1/deleted?') && status === 400);
    await del.filterFrom.fill(today);
    await expect(del.filterPeriodError).toHaveText('Проверьте период');
    await expect(del.filterTo).toHaveClass(/is-invalid/);
    await del.filterTo.fill(today);
    await expect(del.filterPeriodError).toBeHidden();
    await expect(del.rows).toHaveCount(3);

    // «Кто удалил» — второй администратор; сочетание с поиском и периодом — по «И».
    await del.filterBy.selectOption({ label: otherAdmin.name });
    await expect(del.rows).toHaveCount(1);
    await expect(del.row(other.id)).toBeVisible();
    await expect(del.rowBy(other.id)).toHaveText(otherAdmin.name);
    await expect(page).toHaveURL(new RegExp(`deleted_by=${otherAdmin.id}`));

    // перезагрузка: вкладка и все фильтры восстановлены из адресной строки.
    await page.reload();
    await del.loaded();
    await expect(del.filterQ).toHaveValue(mark);
    await expect(del.filterFrom).toHaveValue(today);
    await expect(del.filterTo).toHaveValue(today);
    await expect(del.filterBy).toHaveValue(otherAdmin.id);
    await expect(del.rows).toHaveCount(1);
    await expect(del.row(other.id)).toBeVisible();

    // «Сбросить» — четыре фильтра очищены, список вернулся.
    await del.filterReset.click();
    await expect(del.filterQ).toHaveValue('');
    await expect(del.filterFrom).toHaveValue('');
    await expect(del.filterTo).toHaveValue('');
    await expect(del.filterBy).toHaveValue('');
    await expect(page).not.toHaveURL(/q=|deleted_from=|deleted_to=|deleted_by=/);
    for (const t of [mine, dup, other]) await expect(del.row(t.id)).toBeVisible();

    // слишком длинная строка поиска — текст под полем.
    await del.search('я'.repeat(101));
    await expect(del.filterQError).toHaveText('Строка поиска — не длиннее 100 символов');
  });

  test('SD-006 (TS-039): прямые ссылки на удалённый заказ — /?task=N → «Заказ не найден», печатная форма и вложение → 404 (и администратору, и исполнителю); негатив: после восстановления все три открываются', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    const { task } = await order(adminApi, uniq, 'SD-006');
    const uploaded = await adminApi.uploadAttachment(task.id, { name: `${uniq('sd006')}.png`, mimeType: 'image/png', buffer: PNG_1X1 });
    const fileUrl = uploaded.card.attachments[0].url;
    const printUrl = `/tasks/${task.id}/print/intake`;
    expect((await page.request.get(fileUrl)).status(), 'до удаления файл отдаётся').toBe(200);
    await adminApi.deleteTask(task.id, uniq('SD-006 причина'));

    allowHttpErrors((url, status) => status === 404 && (url.includes(`/api/v1/tasks/${task.id}`) || url.includes(printUrl) || url.includes(fileUrl)));
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);

    const expectGone = async (who: string) => {
      await board.goto(`?task=${task.id}`);
      await expect(page.getByTestId('ui-dialog-text'), `${who}: карточка`).toHaveText('Заказ не найден');
      await page.getByTestId('ui-dialog-ok').click();
      await expect(card.modal).toBeHidden();
      await expect(page).not.toHaveURL(/task=/);
      const print = await page.goto(printUrl);
      expect(print?.status(), `${who}: печатная форма`).toBe(404);
      expect((await page.request.get(fileUrl)).status(), `${who}: вложение`).toBe(404);
    };
    await expectGone('администратор');

    const login = uniq('sd006-exec');
    const exec = await makeUser({ login });
    await loginAs(page, login, exec.password);
    await expectGone('исполнитель');

    // негатив: после восстановления все три ссылки открываются (исполнителю тоже).
    expect((await adminApi.restore('tasks', task.id)).status).toBe(200);
    await board.goto(`?task=${task.id}`);
    await card.waitOpen();
    const print = await page.goto(printUrl);
    expect(print?.status()).toBe(200);
    await expect(page.locator('body')).toContainText(`Акт приёма №${task.id}`);
    expect((await page.request.get(fileUrl)).status()).toBe(200);
  });

  test('SD-007 (TS-046): адаптив — «/deleted» на 360×640 без горизонтальной прокрутки страницы (таблица прокручивается в своём блоке); окно «Удаление» помещается в экран', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { client, task } = await order(adminApi, uniq, 'SD-007');
    const gone = await adminApi.createTask({ title: uniq('SD-007 удалённый заказ с длинным названием для проверки переноса строки'), client_id: client.id, column_id: task.column_id });
    await adminApi.deleteTask(gone.id, uniq('SD-007 ' + 'оченьдлиннаяпричинабезпробелов'.repeat(6)));

    await page.setViewportSize({ width: 360, height: 640 });
    const del = new DeletedPage(page);
    await del.goto(`kind=tasks&q=${encodeURIComponent(`#${gone.id}`)}`);
    await expect(del.row(gone.id)).toBeVisible();
    await del.expectNoHorizontalScroll();

    // подтверждение восстановления помещается.
    await del.restoreBtn(gone.id).scrollIntoViewIfNeeded();
    await del.restoreBtn(gone.id).click();
    await expect(del.dialog).toBeVisible();
    await del.expectNoHorizontalScroll();
    await del.dialogCancel.click();
    await expect(del.dialog).toBeHidden();

    // вкладка «Колонки» (soft-delete-board-columns §8.2, TS-046 расширением): то же на 360×640.
    const goneCol = (await adminApi.createColumn({ name: uniq('SD-007 колонка').slice(0, 60), color: '#445566', is_final: true, consumes_stock: false })).column;
    await adminApi.deleteColumn(goneCol.id, uniq('SD-007 ' + 'оченьдлиннаяпричинаколонкибезпробелов'.repeat(5)));
    await del.goto(`kind=columns&q=${encodeURIComponent(goneCol.name)}`);
    await expect(del.tab('columns')).toHaveClass(/active/);
    await expect(del.row(goneCol.id)).toBeVisible();
    await del.expectNoHorizontalScroll();

    // окно «Удаление» из карточки заказа помещается в экран по ширине, кнопки доступны.
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await card.openDeleteTask();
    const box = await card.deleteDialog.locator('.modal-content').boundingBox();
    expect(box, 'окно отрисовано').not.toBeNull();
    expect(box!.x, 'левый край окна в экране').toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width, 'правый край окна в экране').toBeLessThanOrEqual(360);
    await expect(card.deleteReason).toBeVisible();
    await expect(card.deleteDialogOk).toBeInViewport();
    await del.expectNoHorizontalScroll();
    await card.cancelUiDialog();
  });

  test('SD-008 (TS-047): темы — «/deleted» (вкладка со счётчиком, строка, кнопка, жёлтый блок) и окно «Удаление» (пояснение, ошибка поля, красная кнопка) читаемы в светлой и тёмной теме', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const { client, columns, task } = await order(adminApi, uniq, 'SD-008');
    const work = columnByName(columns, 'В работе').id;
    const { product, service } = await goods(adminApi, uniq, 'SD-008', 5);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}`) && status === 400);

    const del = new DeletedPage(page);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    for (const theme of THEMES) {
      // свой удалённый заказ с нехваткой товара на каждую тему — жёлтый блок после восстановления.
      const short = await adminApi.createTask({ title: uniq(`SD-008 ${theme}`), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
      await insertSubtask({ taskId: short.id, serviceId: service.id, products: [{ productId: product.id, qty: 2 }] });
      await adminApi.adjustStock(product.id, 5);
      await adminApi.moveTask(short.id, { column_id: work });
      await adminApi.deleteTask(short.id, uniq(`SD-008 причина ${theme}`));
      await adminApi.adjustStock(product.id, 0);

      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      await del.goto(`kind=tasks&q=${encodeURIComponent(`#${short.id}`)}`);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);

      expect(await contrastOf(del.tab('tasks')), `${theme}: вкладка со счётчиком`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.rowTitle(short.id)), `${theme}: что удалено`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.rowSubtitle(short.id)), `${theme}: клиент`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.rowReason(short.id)), `${theme}: причина`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.restoreBtn(short.id)), `${theme}: «Восстановить»`).toBeGreaterThanOrEqual(3);

      await del.restore(short.id);
      await expect(del.warnings).toBeVisible();
      expect(await contrastOf(del.warningLines.first()), `${theme}: жёлтый блок`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.warningOpen), `${theme}: ссылка в жёлтом блоке`).toBeGreaterThanOrEqual(4.5);

      // вкладка «Колонки» (soft-delete-board-columns §8.2, TS-047 расширением).
      const goneCol = (await adminApi.createColumn({ name: uniq(`SD-008 колонка ${theme}`).slice(0, 60), color: '#445566', is_final: true, consumes_stock: false })).column;
      await adminApi.deleteColumn(goneCol.id, uniq(`SD-008 причина колонки ${theme}`));
      await del.goto(`kind=columns&q=${encodeURIComponent(goneCol.name)}`);
      await expect(del.rowTitle(goneCol.id)).toHaveText(goneCol.name);
      expect(await contrastOf(del.tab('columns')), `${theme}: вкладка «Колонки»`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.rowTitle(goneCol.id)), `${theme}: колонка`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.rowSubtitle(goneCol.id)), `${theme}: «финальная»`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(del.restoreBtn(goneCol.id)), `${theme}: «Восстановить» колонки`).toBeGreaterThanOrEqual(3);

      // окно «Удаление»: пояснение, текст ошибки поля, красная кнопка.
      await board.goto(`?client_id=${client.id}&task=${task.id}`);
      await card.waitOpen();
      const text = await card.openDeleteTask();
      expect(await contrastOf(card.deleteDialogTitle), `${theme}: заголовок окна`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(text), `${theme}: пояснение`).toBeGreaterThanOrEqual(4.5);
      await card.deleteDialogOk.click();
      await expect(card.deleteReasonError).toHaveText('Укажите причину удаления');
      expect(await contrastOf(card.deleteReasonError), `${theme}: ошибка поля`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(card.deleteDialogOk), `${theme}: красная «Удалить»`).toBeGreaterThanOrEqual(3);
      await card.cancelUiDialog();
      await expect(card.deleteDialog).toBeHidden();
    }
    expect(await adminApi.taskHttpStatus(task.id), 'заказ на месте').toBe(200);
  });
});
