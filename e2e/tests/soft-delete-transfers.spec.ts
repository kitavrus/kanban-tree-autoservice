import { test, expect, columnByName } from '../fixtures/board';
import type { AdminApi, TransferDeleteBody } from '../fixtures/auth';
import { contrastOf } from '../fixtures/contrast';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { TransfersPage } from '../pages/TransfersPage';
import { DeletedPage } from '../pages/DeletedPage';

// Мягкое удаление — документы перемещения и заказы с документами
// (docs/features/soft-delete-transfers/draft-spec.md §8.1–§8.3, §11): заказ с документами
// (TS-040), удалить и восстановить документ (TS-041); отказы по удалённым записям состава —
// отправка (TS-062), отмена проведения (TS-060), восстановление заявки с единицей, убранной из
// заказа (TS-056). Адаптив и темы вкладки «Перемещения»
// раздела «Удалённые» — расширением SD-007 / SD-008 (soft-delete-core.spec.ts); окно удаления
// заказа с документами — TR-024 (transfers-revert.spec.ts). Общая dev-БД (§TD-6.2): свои
// клиенты, оборудование, заказы и товары через uniq()+adminApi, склады — сид «Алматы» /
// «Москва». Уборка — _boardCleanup: заказы вместе с документами (purgeTask), мягко удалённые
// документы без заказа — служебным адресом purge (стенд с TEST_PURGE_ENABLED=true).
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;
const DATE_TIME = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;
const TOAST_DELETED = 'Удалено. Восстановить можно в разделе «Удалённые»';
const TOAST_DELETED_REFS = 'В перемещении есть удалённые записи — сначала восстановите их';

type Uniq = (s: string) => string;

/** Клиент + заказ + count единиц оборудования в заказе — все «у клиента». */
async function orderWithUnits(adminApi: AdminApi, uniq: Uniq, tag: string, count: number, assigneeId?: string) {
  const almaty = await adminApi.warehouseByName('Алматы');
  const moscow = await adminApi.warehouseByName('Москва');
  const client = await adminApi.createClient({ company_name: uniq(`${tag} клиент`), phones: [{ phone: '9990000801', note: '' }] });
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
  return { almaty, moscow, client, columns, task, units };
}

async function newProduct(adminApi: AdminApi, uniq: Uniq, label: string, stock: number) {
  const cat = await adminApi.createCategory({ kind: 'product', name: uniq(`${label} категория`) });
  return adminApi.createProduct({ category_id: cat.id, name: uniq(`${label} товар`), stock }); // остаток — на «Алматы»
}

async function unitState(adminApi: AdminApi, serial: string) {
  const found = (await adminApi.searchEquipment({ q: serial })).items.find((e) => e.serial === serial);
  expect(found, `единица ${serial} найдена`).toBeTruthy();
  return { location: found!.location_status, warehouse: found!.warehouse_name, transfer: found!.active_transfer_id };
}

test.describe('SDT: мягкое удаление документов перемещения', () => {
  test('SDT-001 (TS-040): заказ с документом «в пути» удаляется — документ остаётся в «Перемещениях» с бейджем «заказ удалён» (номер заказа без ссылки), «Принять» работает; бейдж читаем в обеих темах; удалённого заказа нет в выборе нового документа; заказ удалили при открытой форме → тост «Заказ удалён — сначала восстановите его», форма остаётся', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { almaty, client, columns, task, units } = await orderWithUnits(adminApi, uniq, 'SDT001', 1);
    const unit = units[0];
    const doc = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id, send: true,
      items: [{ kind: 'equipment', equipment_id: unit.id }],
    });
    expect(doc.status).toBe('in_transit');
    expect(doc.task_is_deleted).toBe(false);

    // заказ с документом «в пути» удаляется из карточки: отказа нет, в окне — строка о перемещениях.
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const tr = new TransfersPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await card.openDeleteTask();
    await expect(card.deleteDialogNote).toHaveText('Перемещения заказа останутся в разделе «Перемещения» с пометкой «заказ удалён»');
    await card.submitDelete(uniq('SDT-001 заказ создан по ошибке'));
    await expect(card.modal).toBeHidden();
    expect(await adminApi.taskHttpStatus(task.id), 'заказ удалён').toBe(404);
    const kept = await adminApi.getTransfer(doc.id);
    expect(kept).toMatchObject({ status: 'in_transit', version: doc.version, task_id: task.id, task_is_deleted: true });
    expect(await unitState(adminApi, unit.serial)).toMatchObject({ location: 'in_transit', transfer: doc.id });

    // «Перемещения»: документ в списке и в шапке — с бейджем, номер заказа — текст без ссылки.
    for (const theme of THEMES) {
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      await tr.goto(doc.id);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      await expect(tr.rowTaskDeleted(doc.id)).toHaveText('заказ удалён');
      await expect(tr.row(doc.id).getByTestId('st-row-party')).toContainText(`заказ #${task.id}`);
      await expect(tr.taskDeletedBadge).toHaveText('заказ удалён');
      await expect(tr.taskText).toHaveText(`Заказ #${task.id}`);
      await expect(tr.taskLink).toHaveCount(0);
      expect(await contrastOf(tr.taskDeletedBadge), `${theme}: бейдж в документе`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.taskText), `${theme}: номер заказа`).toBeGreaterThanOrEqual(4.5);
    }

    // документ удалённого заказа доводится до конца: «Принять» работает как обычно.
    await expect(tr.cardStatus).toHaveText('В пути');
    await tr.confirmReceive('Оборудование на складе?');
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.taskDeletedBadge).toBeVisible();
    expect((await adminApi.getTransfer(doc.id)).status).toBe('received');
    expect(await unitState(adminApi, unit.serial)).toMatchObject({ location: 'at_warehouse', warehouse: 'Алматы', transfer: null });

    // негатив 1: удалённого заказа нет в выборе нового документа — «Заказ не найден».
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}/card`) && status === 404);
    await tr.newDocument('client_to_warehouse');
    await tr.task.fill(String(task.id));
    await expect(tr.taskError).toHaveText('Заказ не найден');
    await expect(tr.taskInfo).toHaveText('');

    // негатив 2: заказ удалили, пока форма нового документа открыта → 409 TASK_DELETED тостом.
    const type = await adminApi.createEquipmentType({ name: uniq('SDT001 тип второй') });
    const late = await adminApi.createTask({ title: uniq('SDT001 заказ удалят при открытой форме'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    const lateUnit = await adminApi.createEquipment(client.id, { type_id: type.id, brand: 'BrandSDT001', model: uniq('SDT001 модель поздняя'), serial: uniq('sn-SDT001-late') });
    await adminApi.addTaskEquipment(late.id, lateUnit.id);
    await tr.gotoNew('client_to_warehouse', late.id);
    await expect(tr.itemRow('equipment', lateUnit.id)).toBeVisible();
    await tr.choose(tr.to, 'Алматы');
    await adminApi.deleteTask(late.id, uniq('SDT-001 удалён при открытой форме'));
    allowHttpErrors((url, status) => url.endsWith('/api/v1/transfers') && status === 409);
    await tr.save.click();
    await expect(tr.toastError.filter({ hasText: 'Заказ удалён — сначала восстановите его' })).toBeVisible();
    await expect(tr.cardTitle).toHaveText('Новое перемещение'); // форма остаётся
    await expect(tr.itemRow('equipment', lateUnit.id)).toBeVisible();
    expect((await adminApi.listTransfers({ task_id: late.id })).total, 'документ не создан').toBe(0);
  });

  test('SDT-002 (TS-041): заявка → «Удалить»: пустая причина → «Укажите причину удаления»; с причиной → тост, строки нет, прямая ссылка — «Перемещение не найдено»; «Удалённые» → вкладка «Перемещения» со счётчиком → «Восстановить» → документ снова в списке со своими строками; документ отправили при открытом окне — «Перемещение уже изменено, обновите», у документа «в пути» — «Перемещение в пути — сначала отмените его»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { almaty, moscow, task, units } = await orderWithUnits(adminApi, uniq, 'SDT002', 1);
    const unit = units[0];
    await adminApi.receiveFromClient(task.id, almaty.id, [unit.id]); // единица на «Алматы»
    const product = await newProduct(adminApi, uniq, 'SDT-002', 5);
    const request = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id,
      items: [{ kind: 'equipment', equipment_id: unit.id }, { kind: 'product', product_id: product.id, qty_sent: 2 }],
    });
    const reason = uniq('SDT-002 причина: документ создан по ошибке');

    const tr = new TransfersPage(page);
    const del = new DeletedPage(page);
    await tr.goto(request.id);
    await expect(tr.cardStatus).toHaveText('Заявка');

    // пустая причина — ошибка под полем, окно открыто, документ на месте; «Отмена» ничего не удаляет.
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${request.id}`) && status === 400);
    await tr.openDelete(request.id);
    await tr.submitDelete('   ');
    await expect(tr.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(tr.uiDialog).toBeVisible();
    await tr.cancelDelete();
    await expect(tr.cardTitle).toHaveText(`Перемещение #${request.id}`);
    expect((await adminApi.getTransfer(request.id)).version, 'документ не изменён').toBe(request.version);

    // с причиной — документ уходит в «Удалённые», единица заявки свободна, остаток не менялся.
    await tr.deleteDocument(request.id, true, reason);
    await expect(tr.toast.filter({ hasText: TOAST_DELETED })).toBeVisible();
    await expect(tr.placeholder).toBeVisible();
    await expect(tr.row(request.id)).toHaveCount(0);
    await expect(page).toHaveURL(/\/transfers$/);
    expect(await adminApi.transferHttpStatus(request.id), 'удалённый документ для обычных адресов не существует').toBe(404);
    expect(await unitState(adminApi, unit.serial)).toMatchObject({ location: 'at_warehouse', warehouse: 'Алматы', transfer: null });

    // прямая ссылка на удалённый документ — «Перемещение не найдено».
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${request.id}`) && status === 404);
    await page.goto(`/transfers?id=${request.id}`);
    await expect(tr.toastError.filter({ hasText: 'Перемещение не найдено' })).toBeVisible();
    await expect(tr.placeholder).toBeVisible();

    // «Удалённые» → вкладка «Перемещения»: счётчик, строка, «Восстановить».
    await del.goto();
    const summary = await adminApi.deletedSummary();
    await expect(del.tab('transfers')).toContainText('Перемещения');
    await expect(del.tabCount('transfers')).toHaveText(String(summary.counts.transfers));
    await del.tab('transfers').click();
    await expect(page).toHaveURL(/kind=transfers/);
    await del.search(`#${request.id}`);
    await expect(del.rows).toHaveCount(1);
    await expect(del.rowTitle(request.id)).toHaveText(`Перемещение #${request.id}`);
    await expect(del.rowSubtitle(request.id)).toHaveText('Склад → склад · Заявка');
    await expect(del.rowReason(request.id)).toHaveText(reason);
    await expect(del.rowBy(request.id)).not.toHaveText('');
    await expect(del.rowAt(request.id)).toHaveText(DATE_TIME);
    const listed = await adminApi.listDeleted('transfers', { q: `#${request.id}` });
    expect(listed.items.map((i) => i.id)).toEqual([String(request.id)]);
    expect(listed.items[0]).toMatchObject({ kind: 'transfers', is_archived: false, deleted_with: null, delete_reason: reason });

    const countBefore = Number(await del.tabCount('transfers').textContent());
    await del.restoreBtn(request.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «Перемещение #${request.id}»?`);
    await del.dialogOk.click();
    await expect(del.toast.filter({ hasText: 'Восстановлено' })).toBeVisible();
    await expect(del.row(request.id)).toHaveCount(0);
    await expect(del.tabCount('transfers')).toHaveText(String(countBefore - 1));
    await expect(del.warnings).toBeHidden(); // у документа предупреждений восстановления нет

    // документ снова в списке со своими строками, единица снова занята им.
    await tr.goto(request.id);
    await expect(tr.cardStatus).toHaveText('Заявка');
    await expect(tr.row(request.id)).toHaveCount(1);
    await expect(tr.itemRows).toHaveCount(2);
    await expect(tr.itemRow('equipment', unit.id)).toBeVisible();
    await expect(tr.itemRow('product', product.id)).toBeVisible();
    await expect(tr.itemQty(product.id)).toHaveValue('2');
    const restored = await adminApi.getTransfer(request.id);
    expect(restored).toMatchObject({ status: 'requested', version: request.version + 2 });
    expect(await unitState(adminApi, unit.serial)).toMatchObject({ location: 'at_warehouse', transfer: request.id });

    // негатив: документ отправили, пока окно удаления открыто. Кнопки «Удалить» у «В пути» нет,
    // поэтому запрос уходит из уже открытого окна. Без подмены — как у пользователя: в окне
    // устаревшая версия → «Перемещение уже изменено, обновите», окно и причина остаются.
    await tr.openDelete(request.id);
    const sent = await adminApi.transferAction(request.id, 'send');
    expect(sent.status).toBe('in_transit');
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${request.id}`) && status === 409);
    const staleReason = uniq('SDT-002 удаляем по устаревшей версии');
    await tr.submitDelete(staleReason);
    await expect(tr.deleteDialogError).toHaveText('Перемещение уже изменено, обновите');
    await expect(tr.uiDialog).toBeVisible();
    await expect(tr.deleteReason).toHaveValue(staleReason);
    expect((await adminApi.getTransfer(request.id)).status, 'отказ документ не тронул').toBe('in_transit');
    // чтобы сервер дошёл до проверки статуса (версия сверяется раньше), в запрос подставляется
    // актуальная версия (abuse case 4).
    await page.route(`**/api/v1/transfers/${request.id}`, async (route) => {
      if (route.request().method() !== 'DELETE') return route.continue();
      const body = route.request().postDataJSON() as TransferDeleteBody;
      const fresh: TransferDeleteBody = { reason: body.reason, version: sent.version };
      await route.continue({ postData: JSON.stringify(fresh) });
    });
    await tr.submitDelete(uniq('SDT-002 удаляем отправленный'));
    await expect(tr.deleteDialogError).toHaveText('Перемещение в пути — сначала отмените его');
    await expect(tr.uiDialog).toBeVisible(); // окно остаётся открытым
    await page.unroute(`**/api/v1/transfers/${request.id}`);
    await tr.cancelDelete();
    // после отказа документ перечитан: «В пути», кнопки «Удалить» нет, остаток списан отправкой один раз.
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.deleteBtn).toHaveCount(0);
    expect((await adminApi.getTransfer(request.id)).status).toBe('in_transit');
    expect(await adminApi.transferHttpStatus(request.id)).toBe(200);
    // уборка: документ без заказа — отмена отправки и физическое удаление служебным адресом.
    await adminApi.removeTransferForCleanup(request.id);
  });

  test('SDT-003 (TS-041): восстановление заявки, чья единица уже в другом документе, — тост «Оборудование уже в перемещении #N», строка остаётся; устаревшая версия в окне удаления — «Перемещение уже изменено, обновите»; повторное восстановление из второй вкладки — «Запись уже восстановлена», строка исчезает', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
    context,
  }) => {
    const { almaty, moscow, task, units } = await orderWithUnits(adminApi, uniq, 'SDT003', 1);
    const unit = units[0];
    await adminApi.receiveFromClient(task.id, almaty.id, [unit.id]);
    const gone = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'equipment', equipment_id: unit.id }],
    });

    // устаревшая версия: документ правят, пока окно удаления открыто → 409 в окне, документ цел.
    const tr = new TransfersPage(page);
    await tr.goto(gone.id);
    await tr.openDelete(gone.id);
    await adminApi.transferAction(gone.id, 'cancel');
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${gone.id}`) && status === 409);
    await tr.submitDelete(uniq('SDT-003 устаревшая версия'));
    await expect(tr.deleteDialogError).toHaveText('Перемещение уже изменено, обновите');
    await tr.cancelDelete();
    await expect(tr.cardStatus).toHaveText('Отменён'); // документ перечитан
    expect(await adminApi.transferHttpStatus(gone.id)).toBe(200);
    await adminApi.removeTransferForCleanup(gone.id);

    // заявка удалена, её единицу заняла другая заявка → восстановление отказано.
    const request = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'equipment', equipment_id: unit.id }],
    });
    await adminApi.softDelete('transfers', request.id, uniq('SDT-003 причина'));
    const blocker = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'equipment', equipment_id: unit.id }],
    });

    const del = new DeletedPage(page);
    await del.goto(`kind=transfers&q=${encodeURIComponent(`#${request.id}`)}`);
    await expect(del.row(request.id)).toBeVisible();
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${request.id}/restore`) && status === 409);
    await del.restore(request.id);
    await expect(del.toastError.filter({ hasText: `Оборудование уже в перемещении #${blocker.id}` })).toBeVisible();
    await expect(del.row(request.id)).toBeVisible(); // строка остаётся
    await expect(del.restoreBtn(request.id)).toBeEnabled();
    expect(await adminApi.transferHttpStatus(request.id), 'документ остался удалённым').toBe(404);
    expect(await unitState(adminApi, unit.serial)).toMatchObject({ transfer: blocker.id });

    // мешающая заявка убрана; документ восстановили в другой вкладке → здесь «Запись уже восстановлена».
    await adminApi.removeTransferForCleanup(blocker.id);
    const other = await context.newPage();
    const del2 = new DeletedPage(other);
    await del2.goto(`kind=transfers&q=${encodeURIComponent(`#${request.id}`)}`);
    await del2.restore(request.id);
    await expect(del2.toast.filter({ hasText: 'Восстановлено' })).toBeVisible();
    await other.close();

    await del.restore(request.id);
    await expect(del.toastError.filter({ hasText: 'Запись уже восстановлена' })).toBeVisible();
    await expect(del.row(request.id)).toHaveCount(0);
    expect((await adminApi.getTransfer(request.id)).status).toBe('requested');
    expect(await unitState(adminApi, unit.serial)).toMatchObject({ transfer: request.id });
    await adminApi.removeTransferForCleanup(request.id);
  });

  test('SDT-004 (TS-040): исполнитель видит бейдж «заказ удалён» у документа своего направления и доводит его до конца; кнопки «Удалить» у него нет, раздел «Удалённые» — «Нет доступа»', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    const exec = await makeUser({ login: uniq('sdt004-exec'), name: uniq('SDT-004 Исполнитель') });
    const { almaty, task, units } = await orderWithUnits(adminApi, uniq, 'SDT004', 1, exec.id);
    const doc = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id, items: [{ kind: 'equipment', equipment_id: units[0].id }],
    });
    await adminApi.deleteTask(task.id, uniq('SDT-004 заказ удалён'));

    await loginAs(page, exec.login, exec.password);
    const tr = new TransfersPage(page);
    await tr.goto(doc.id);
    await expect(tr.rowTaskDeleted(doc.id)).toHaveText('заказ удалён');
    await expect(tr.taskDeletedBadge).toHaveText('заказ удалён');
    await expect(tr.taskLink).toHaveCount(0);
    await expect(tr.deleteBtn).toHaveCount(0); // удаляет только администратор

    // заявку удалённого заказа исполнитель отправляет и принимает как обычно.
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    await tr.confirmReceive('Оборудование на складе?');
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.taskDeletedBadge).toBeVisible();
    expect(await adminApi.getTransfer(doc.id)).toMatchObject({ status: 'received', task_is_deleted: true });

    // раздел «Удалённые» исполнителю закрыт.
    allowHttpErrors((url, status) => url.includes('/deleted') && status === 403);
    const resp = await page.goto('/deleted?kind=transfers');
    expect(resp?.status()).toBe(403);
    await expect(page.locator('body')).toContainText('Нет доступа');
    const api = await page.request.get('/api/v1/deleted?kind=transfers');
    expect(api.status()).toBe(403);
  });

  test('SDT-005 (TS-062): заявка, товар которой удалили после её создания, не отправляется — тост «В перемещении есть удалённые записи — сначала восстановите их», документ остаётся «Заявка»; после восстановления товара «Отправить» проходит', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const almaty = await adminApi.warehouseByName('Алматы');
    const supplier = await adminApi.createSupplier({ name: uniq('SDT-005 поставщик') });
    const product = await newProduct(adminApi, uniq, 'SDT-005', 0);
    const request = await adminApi.createTransfer({
      direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 2 }],
    });
    await adminApi.softDelete('products', product.id, uniq('SDT-005 товар снят с продажи'));
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${request.id}/send`) && status === 409);

    try {
      const tr = new TransfersPage(page);
      await tr.goto(request.id);
      await expect(tr.cardStatus).toHaveText('Заявка');
      await expect(tr.itemRow('product', product.id)).toBeVisible();

      await tr.send.click();
      await expect(tr.toastError.filter({ hasText: TOAST_DELETED_REFS })).toBeVisible();
      await expect(tr.cardStatus, 'документ перечитан и остался заявкой').toHaveText('Заявка');
      await expect(tr.send).toBeVisible();
      expect((await adminApi.getTransfer(request.id)).status, 'отказ документ не отправил').toBe('requested');

      // товар восстановлен — та же кнопка отправляет документ.
      expect((await adminApi.restoreCatalog('products', product.id)).status).toBe(200);
      await tr.send.click();
      await expect(tr.cardStatus).toHaveText('В пути');
      expect((await adminApi.getTransfer(request.id)).status).toBe('in_transit');
    } finally {
      // уборка: документ без заказа держит товар и поставщика от служебной уборки.
      await adminApi.restoreCatalog('products', product.id);
      await adminApi.removeTransferForCleanup(request.id);
    }
  });

  test('SDT-006 (TS-060): «Отменить проведение» документа с удалённым товаром и документа с удалённой единицей оборудования — диалог закрывается, тост «В перемещении есть удалённые записи — сначала восстановите их», документ остаётся «Получено»; после восстановления товара отмена проведения проходит', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    // ---- документ «поставщик → склад» получен, товар израсходован до нуля и удалён ----
    const supplier = await adminApi.createSupplier({ name: uniq('SDT-006 поставщик') });
    const product = await newProduct(adminApi, uniq, 'SDT-006', 0);
    const { almaty, client, task, units } = await orderWithUnits(adminApi, uniq, 'SDT006', 1);
    const unit = units[0];
    const supply = await adminApi.createTransfer({
      direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id, send: true,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 2 }],
    });
    await adminApi.transferAction(supply.id, 'receive', { items: [{ id: supply.items[0].id, qty_received: 2 }] });
    await adminApi.adjustStock(product.id, 0);
    await adminApi.softDelete('products', product.id, uniq('SDT-006 товар снят с продажи'));
    // ---- единица принята на склад, возвращена клиенту документом «склад → клиент» и удалена ----
    await adminApi.receiveFromClient(task.id, almaty.id, [unit.id]);
    const back = await adminApi.createTransfer({
      direction: 'warehouse_to_client', from_warehouse_id: almaty.id, task_id: task.id, send: true,
      items: [{ kind: 'equipment', equipment_id: unit.id }],
    });
    await adminApi.transferAction(back.id, 'receive');
    expect(await unitState(adminApi, unit.serial)).toMatchObject({ location: 'at_client', transfer: null });
    await adminApi.softDelete('equipment', unit.id, uniq('SDT-006 единица списана'));
    allowHttpErrors((url, status) => /\/api\/v1\/transfers\/\d+\/unreceive$/.test(url) && status === 409);

    try {
      const tr = new TransfersPage(page);
      for (const doc of [supply, back]) {
        const before = await adminApi.getTransfer(doc.id);
        await tr.goto(doc.id);
        await expect(tr.cardStatus).toHaveText('Получено');
        await tr.openUnreceive(doc.id);
        await tr.submitUnreceive(uniq('SDT-006 ошибка в факте'));
        await expect(tr.toastError.filter({ hasText: TOAST_DELETED_REFS })).toBeVisible();
        await expect(tr.unreceiveModal).toBeHidden();
        await expect(tr.cardStatus, `документ #${doc.id} остался «Получено»`).toHaveText('Получено');
        await expect(tr.unreceiveBtn).toBeVisible();
        expect(await adminApi.getTransfer(doc.id), 'отказ документ не тронул').toMatchObject({ status: 'received', version: before.version });
      }

      // товар восстановлен (остаток возвращён к факту приёмки) — отмена проведения проходит.
      expect((await adminApi.restoreCatalog('products', product.id)).status).toBe(200);
      await adminApi.adjustStock(product.id, 2);
      await tr.goto(supply.id);
      await tr.openUnreceive(supply.id);
      await tr.submitUnreceive(uniq('SDT-006 ошибка в факте'));
      await expect(tr.toast.filter({ hasText: 'Проведение отменено' })).toBeVisible();
      await expect(tr.cardStatus).toHaveText('В пути');
    } finally {
      // уборка: удалённые записи возвращаются — иначе документы не откатить и не убрать.
      await adminApi.restoreCatalog('products', product.id);
      await adminApi.restoreEquipment(unit.id);
      await adminApi.removeTransferForCleanup(supply.id);
      await adminApi.purgeClient(client.id, [task.id]);
    }
  });

  test('SDT-007 (TS-056): восстановление заявки из «Удалённых», единицу которой убрали из заказа, — тост «Оборудование убрано из заказа — восстановить перемещение нельзя», строка остаётся, документ удалён; единицу вернули в заказ → «Восстановлено», заявка снова занимает единицы', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { almaty, task, units } = await orderWithUnits(adminApi, uniq, 'SDT007', 2);
    const [kept, removed] = units;
    const request = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id,
      items: units.map((u) => ({ kind: 'equipment' as const, equipment_id: u.id })),
    });
    await adminApi.softDelete('transfers', request.id, uniq('SDT-007 заявка создана по ошибке'));
    // единица удалённой заявки свободна — убирается из заказа боевым API карточки.
    await adminApi.removeTaskEquipment(task.id, removed.id);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${request.id}/restore`) && status === 409);

    const del = new DeletedPage(page);
    await del.goto(`kind=transfers&q=${encodeURIComponent(`#${request.id}`)}`);
    await expect(del.row(request.id)).toBeVisible();
    await del.restore(request.id);
    await expect(del.toastError.filter({ hasText: 'Оборудование убрано из заказа — восстановить перемещение нельзя' })).toBeVisible();
    await expect(del.row(request.id), 'строка остаётся').toBeVisible();
    await expect(del.restoreBtn(request.id)).toBeEnabled();
    expect(await adminApi.transferHttpStatus(request.id), 'документ остался удалённым').toBe(404);
    expect(await unitState(adminApi, kept.serial), 'оставшаяся в заказе единица не занята').toMatchObject({ location: 'at_client', transfer: null });

    // единицу вернули в заказ — та же строка восстанавливается.
    await adminApi.addTaskEquipment(task.id, removed.id);
    await del.restore(request.id);
    await expect(del.toast.filter({ hasText: 'Восстановлено' })).toBeVisible();
    await expect(del.row(request.id)).toHaveCount(0);
    expect((await adminApi.getTransfer(request.id)).status).toBe('requested');
    for (const u of units) expect(await unitState(adminApi, u.serial)).toMatchObject({ transfer: request.id });
  });
});
