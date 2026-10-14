import { test, expect, columnByName } from '../fixtures/board';
import { TransfersPage } from '../pages/TransfersPage';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import type { AdminApi } from '../fixtures/auth';
import { contrastOf } from '../fixtures/contrast';

// Отмена проведения и удаление документа «Перемещение» (docs/features/transfers-revert/
// draft-spec.md §8.1, §8.2): «Отменить проведение» с причиной и повторная приёмка (TS-033),
// удаление заказа с документами (TS-037 → soft-delete-transfers TS-014: отказ снят), удаление
// документа (TS-038; теперь мягкое, с причиной). Общая dev-БД
// (§TD-6.2): свои клиенты, оборудование, заказы, товары и поставщики через uniq()+adminApi,
// склады — сид «Алматы» / «Москва». Заказы вместе с документами убирает _boardCleanup
// (purgeTask).
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;

type Uniq = (s: string) => string;

/** Клиент + заказ + count единиц оборудования в заказе — все «у клиента». */
async function orderWithUnits(adminApi: AdminApi, uniq: Uniq, tag: string, count: number, assigneeId?: string) {
  const almaty = await adminApi.warehouseByName('Алматы');
  const moscow = await adminApi.warehouseByName('Москва');
  const client = await adminApi.createClient({ company_name: uniq(`${tag} клиент`), phones: [{ phone: '9990000601', note: '' }] });
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

test.describe('TR: перемещения — отмена проведения и удаление', () => {
  test('TR-022 (TS-033): «Отменить проведение» прихода от поставщика — пустая причина → «Укажите причину»; с причиной → «В пути», строка «Проведение отменено: …», остаток откатился; повторная приёмка с другим фактом — остаток по новому факту, строка об откате осталась', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const almaty = await adminApi.warehouseByName('Алматы');
    const product = await newProduct(adminApi, uniq, 'TR-022', 3);
    const supplier = await adminApi.createSupplier({ name: uniq('TR-022 поставщик') });
    const doc = await adminApi.createTransfer({
      direction: 'supplier_to_warehouse', to_warehouse_id: almaty.id, supplier_id: supplier.id, send: true,
      items: [{ kind: 'product', product_id: product.id, qty_sent: 10 }],
    });
    const line = doc.items[0];
    await adminApi.transferAction(doc.id, 'receive', { items: [{ id: line.id, qty_received: 10 }] });
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(13);

    const tr = new TransfersPage(page);
    await tr.goto(doc.id);
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.unreceiveBtn).toBeVisible();
    await expect(tr.unreceiveBtn).toHaveText('Отменить проведение');
    await expect(tr.cardUnreceived).toHaveCount(0); // отката ещё не было
    await expect(tr.deleteBtn).toHaveCount(0); // у «Получено» кнопки «Удалить» нет (TS-038)

    // негатив: причина обязательна — ошибка под полем, диалог открыт, документ не изменён.
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${doc.id}/unreceive`) && status === 400);
    await tr.openUnreceive(doc.id);
    await tr.submitUnreceive('   ');
    await expect(tr.unreceiveReasonError).toHaveText('Укажите причину');
    await expect(tr.unreceiveModal).toBeVisible();
    expect((await adminApi.getTransfer(doc.id)).status).toBe('received');
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(13);

    // причина указана: документ «В пути», сведения об откате, остаток откатился.
    const reason = uniq('TR-022 приняли без пересчёта');
    await tr.submitUnreceive(`  ${reason}  `);
    await expect(tr.unreceiveModal).toBeHidden();
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.toast.filter({ hasText: 'Проведение отменено' })).toBeVisible();
    await expect(tr.cardUnreceived).toContainText('Проведение отменено: Админ Прогона');
    await expect(tr.cardUnreceived).toContainText(reason);
    await expect(tr.rowUnreceived(doc.id)).toBeVisible();
    await expect(tr.rowUnreceived(doc.id)).toHaveAttribute('title', 'Проведение отменено');
    await expect(tr.unreceiveBtn).toHaveCount(0); // «В пути»: откатывать нечего
    await expect(tr.receive).toBeVisible();
    await expect(tr.warnings).toHaveCount(0); // остаток 3 — не минус и не ниже минимума
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(3);
    const back = await adminApi.getTransfer(doc.id);
    expect(back).toMatchObject({ status: 'in_transit', unreceive_reason: reason });
    expect(back.unreceived_at).not.toBeNull();
    expect(back.items[0].qty_received).toBeNull();

    // повторная приёмка с другим фактом: остаток — по новому факту, строка об откате осталась.
    await tr.receive.click();
    await expect(tr.receiveModal).toBeVisible();
    await tr.receiveQty(line.id).fill('7');
    await tr.receiveConfirm.click();
    await expect(tr.receiveModal).toBeHidden();
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.viewItem('product', product.id).getByTestId('st-view-received')).toHaveText('7');
    await expect(tr.cardUnreceived).toContainText(reason);
    await expect(tr.rowUnreceived(doc.id)).toHaveCount(0); // значок — только пока документ «В пути»
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(10);
    expect((await adminApi.getTransfer(doc.id)).unreceive_reason).toBe(reason);
  });

  test('TR-023 (TS-033): единицу увёз более поздний документ — в диалоге «Сначала отмените проведение перемещения #N» со ссылкой, документ не изменён; диалог при 375×812 без горизонтальной прокрутки, читаем в светлой и тёмной теме', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { almaty, moscow, task, units } = await orderWithUnits(adminApi, uniq, 'TR023', 1);
    const unit = units[0];
    await adminApi.receiveFromClient(task.id, almaty.id, [unit.id]); // единица на «Алматы»
    const move = async (from: string, to: string) => {
      const d = await adminApi.createTransfer({ from_warehouse_id: from, to_warehouse_id: to, send: true, items: [{ kind: 'equipment', equipment_id: unit.id }] });
      return adminApi.transferAction(d.id, 'receive');
    };
    const first = await move(almaty.id, moscow.id); // привёз единицу в Москву
    const later = await move(moscow.id, almaty.id); // более поздний документ увёз её обратно

    allowHttpErrors((url, status) => url.endsWith(`/api/v1/transfers/${first.id}/unreceive`) && status === 409);
    await page.setViewportSize({ width: 375, height: 812 });
    const tr = new TransfersPage(page);
    for (const theme of THEMES) {
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      const where = `${theme} 375×812`;
      await tr.goto(first.id);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      await expect(tr.cardStatus).toHaveText('Получено');
      await tr.unreceiveBtn.scrollIntoViewIfNeeded();
      expect(await contrastOf(tr.unreceiveBtn), `документ ${where}: «Отменить проведение»`).toBeGreaterThanOrEqual(3);

      await tr.openUnreceive(first.id);
      await tr.expectNoHorizontalScroll();
      expect(await contrastOf(tr.unreceiveTitle), `диалог ${where}: заголовок`).toBeGreaterThanOrEqual(4.5);
      expect(await contrastOf(tr.unreceiveConfirm), `диалог ${where}: кнопка`).toBeGreaterThanOrEqual(3);

      // откат — в обратном порядке: мешающий документ назван ссылкой, диалог остаётся открыт.
      await tr.submitUnreceive('проверка порядка');
      await expect(tr.unreceiveError).toHaveText(`Сначала отмените проведение перемещения #${later.id}`);
      await expect(tr.unreceiveBlockedLink).toHaveText(`#${later.id}`);
      await expect(tr.unreceiveBlockedLink).toHaveAttribute('href', `/transfers?id=${later.id}`);
      await expect(tr.unreceiveModal).toBeVisible();
      await tr.expectNoHorizontalScroll();
      expect(await contrastOf(tr.unreceiveBlockedLink), `диалог ${where}: ссылка на мешающий документ`).toBeGreaterThanOrEqual(3);
    }
    const same = await adminApi.getTransfer(first.id);
    expect(same).toMatchObject({ status: 'received', unreceived_at: null, unreceive_reason: '' });

    // ссылка ведёт на мешающий документ; после его отката первый документ откатывается.
    await tr.unreceiveBlockedLink.click();
    await expect(tr.cardTitle).toHaveText(`Перемещение #${later.id}`);
    await tr.openUnreceive(later.id);
    await tr.submitUnreceive('сначала поздний');
    await expect(tr.unreceiveModal).toBeHidden();
    await expect(tr.cardStatus).toHaveText('В пути');
    await expect(tr.cardUnreceived).toContainText('сначала поздний');
    await tr.expectNoHorizontalScroll();
    expect(await contrastOf(tr.cardUnreceived), 'документ dark 375×812: строка об откате').toBeGreaterThanOrEqual(3);
    await adminApi.transferAction(later.id, 'cancel'); // единица снова в Москве
    await adminApi.unreceiveTransfer(first.id, 'теперь можно');
    expect((await adminApi.getTransfer(first.id)).status).toBe('in_transit');
  });

  test('TR-024 (TS-014; soft-delete-transfers DS-006): удаление заказа с проведённым и отменённым документами — отказа «Сначала отмените перемещения» больше нет: в окне «Удаление» вторая строка о перемещениях, заказ удаляется, документы остаются с пометкой «заказ удалён»; у заказа без документов второй строки нет', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, client, task, units } = await orderWithUnits(adminApi, uniq, 'TR024', 2);
    const received = await adminApi.receiveFromClient(task.id, almaty.id, [units[0].id]);
    const cancelled = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id, items: [{ kind: 'equipment', equipment_id: units[1].id }],
    });
    await adminApi.transferAction(cancelled.id, 'cancel');

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const tr = new TransfersPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.transferRows).toHaveCount(2);

    // окно «Удаление»: пояснение + вторая строка о документах заказа (§8.3); отказа 409 нет.
    const dialogText = await card.openDeleteTask();
    await expect(card.deleteDialogTitle).toContainText(`#${task.id}`);
    await expect(dialogText).toContainText('Товары заказа вернутся на склад');
    await expect(card.deleteDialogNote).toHaveText('Перемещения заказа останутся в разделе «Перемещения» с пометкой «заказ удалён»');
    await card.submitDelete('удаляем заказ с документами');
    await expect(card.deleteDialog).toBeHidden();
    await expect(card.modal).toBeHidden();
    expect(await adminApi.taskHttpStatus(task.id), 'заказ удалён').toBe(404);
    expect((await adminApi.listDeleted('tasks', { q: `#${task.id}` })).items.map((i) => i.id)).toEqual([String(task.id)]);

    // документы не изменились и остались у удалённого заказа.
    const after = await adminApi.listTransfers({ task_id: task.id });
    expect(after.total, 'документы остались у удалённого заказа').toBe(2);
    expect(after.items.every((t) => t.task_is_deleted)).toBe(true);
    expect((await adminApi.getTransfer(received.id)).status).toBe('received');
    expect((await adminApi.getTransfer(cancelled.id)).status).toBe('cancelled');

    await tr.goto();
    await expect(tr.list).toBeVisible();
    await expect(tr.rowTaskDeleted(received.id)).toHaveText('заказ удалён');
    await expect(tr.rowTaskDeleted(cancelled.id)).toHaveText('заказ удалён');

    // негатив: у заказа без документов второй строки пояснения нет.
    const columns = (await adminApi.listColumns()).items;
    const plain = await adminApi.createTask({ title: uniq('TR024 без документов'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    await board.goto(`?client_id=${client.id}&task=${plain.id}`);
    await card.waitOpen();
    await card.openDeleteTask();
    await expect(card.deleteDialogNote).toHaveCount(0);
    await card.cancelUiDialog();
  });

  test('TR-025 (TS-038): «Удалить» у заявки — окно «Удаление» с причиной, строка исчезла, тост «Удалено», единица снова свободна; у «В пути» и «Получено» кнопки нет; у «Отменён» — есть; красная кнопка читаема в обеих темах', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const { almaty, moscow, task, units } = await orderWithUnits(adminApi, uniq, 'TR025', 1);
    const unit = units[0];
    await adminApi.receiveFromClient(task.id, almaty.id, [unit.id]); // единица на «Алматы»
    const product = await newProduct(adminApi, uniq, 'TR-025', 5);
    const request = await adminApi.createTransfer({
      from_warehouse_id: almaty.id, to_warehouse_id: moscow.id,
      items: [{ kind: 'equipment', equipment_id: unit.id }, { kind: 'product', product_id: product.id, qty_sent: 2 }],
    });
    const cancelled = await adminApi.createTransfer({ from_warehouse_id: almaty.id, to_warehouse_id: moscow.id, items: [{ kind: 'product', product_id: product.id, qty_sent: 1 }] });
    await adminApi.transferAction(cancelled.id, 'cancel');

    const tr = new TransfersPage(page);
    for (const theme of THEMES) {
      await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
      await tr.goto(request.id);
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
      await expect(tr.cardStatus).toHaveText('Заявка');
      await expect(tr.deleteBtn).toBeVisible();
      await expect(tr.deleteBtn).toHaveText('Удалить');
      expect(await contrastOf(tr.deleteBtn), `заявка ${theme}: красная «Удалить»`).toBeGreaterThanOrEqual(3);
    }
    await expect(tr.unreceiveBtn).toHaveCount(0);

    // отказ в окне «Удаление» — документ на месте; причина и подтверждение — документ удалён
    // (мягко: soft-delete-transfers DS-018).
    await tr.deleteDocument(request.id, false);
    await expect(tr.cardTitle).toHaveText(`Перемещение #${request.id}`);
    await expect(tr.row(request.id)).toHaveCount(1);
    await tr.deleteDocument(request.id);
    await expect(tr.toast.filter({ hasText: 'Удалено' })).toBeVisible();
    await expect(tr.placeholder).toBeVisible();
    await expect(tr.row(request.id)).toHaveCount(0);
    await expect(page).toHaveURL(/\/transfers$/);
    expect(await pairStock(adminApi, product.name, almaty.id)).toBe(5); // заявка остаток не двигала

    // единица заявки свободна — встаёт в новый документ (UI: «+ Оборудование» находит её).
    await tr.newDocument('warehouse_to_warehouse');
    await tr.choose(tr.from, 'Алматы');
    await tr.choose(tr.to, 'Москва');
    await tr.addEquipmentRow(unit.serial, unit.id);
    await tr.send.click();
    await expect(tr.cardStatus).toHaveText('В пути');
    const moving = await tr.currentId();

    // негатив: у «В пути» и «Получено» кнопки «Удалить» нет.
    await expect(tr.deleteBtn).toHaveCount(0);
    await adminApi.transferAction(moving, 'receive');
    await tr.goto(moving);
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.deleteBtn).toHaveCount(0);
    await expect(tr.unreceiveBtn).toBeVisible();

    // «Отменён» — удаляется.
    await tr.goto(cancelled.id);
    await expect(tr.cardStatus).toHaveText('Отменён');
    await expect(tr.deleteBtn).toBeVisible();
    await tr.deleteDocument(cancelled.id);
    await expect(tr.toast.filter({ hasText: 'Удалено' })).toBeVisible();
    await expect(tr.row(cancelled.id)).toHaveCount(0);
  });

  test('TR-026 (TS-033, TS-038): исполнитель не видит «Отменить проведение» и «Удалить» у документов с клиентом, сведения об откате видит', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
  }) => {
    const exec = await makeUser({ login: uniq('tr026-exec'), name: uniq('TR-026 Исполнитель') });
    const { almaty, task, units } = await orderWithUnits(adminApi, uniq, 'TR026', 2, exec.id);
    const received = await adminApi.receiveFromClient(task.id, almaty.id, [units[0].id]);
    await adminApi.unreceiveTransfer(received.id, 'TR-026 откат администратором');
    await adminApi.transferAction(received.id, 'receive');
    const request = await adminApi.createTransfer({
      direction: 'client_to_warehouse', to_warehouse_id: almaty.id, task_id: task.id, items: [{ kind: 'equipment', equipment_id: units[1].id }],
    });

    await loginAs(page, exec.login, exec.password);
    const tr = new TransfersPage(page);
    await tr.goto(received.id);
    await expect(tr.cardStatus).toHaveText('Получено');
    await expect(tr.unreceiveBtn).toHaveCount(0);
    await expect(tr.deleteBtn).toHaveCount(0);
    await expect(tr.cardUnreceived).toContainText('TR-026 откат администратором');

    await tr.goto(request.id);
    await expect(tr.cardStatus).toHaveText('Заявка');
    await expect(tr.cancel).toBeVisible(); // свои действия исполнителю доступны
    await expect(tr.deleteBtn).toHaveCount(0);
  });

  test('TR-027 (D84): смена клиента у заказа с документом перемещения — ошибка в шапке «У заказа есть перемещения — сменить клиента нельзя: #N» со ссылкой на документ, клиент в форме прежний, заказ не изменён', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const { almaty, client, task, units } = await orderWithUnits(adminApi, uniq, 'TR027', 1);
    const other = await adminApi.createClient({ company_name: uniq('TR027 другой клиент'), phones: [{ phone: '9990000627', note: '' }] });
    const doc = await adminApi.receiveFromClient(task.id, almaty.id, [units[0].id]);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    await expect(card.transferRows).toHaveCount(1);
    await expect(card.taskClient).toContainText(client.company_name);

    // выбор другого клиента: предупреждение об оборудовании подтверждаем, сохраняем → 409.
    await card.chooseRemote(card.taskClient, other.company_name, other.company_name);
    await card.confirmUiDialog();
    await expect(card.taskClient).toContainText(other.company_name);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}`) && status === 409);
    await card.save();

    await expect(card.error).toHaveText(`У заказа есть перемещения — сменить клиента нельзя: #${doc.id}`);
    await expect(card.errorTransferLinks).toHaveCount(1);
    await expect(card.errorTransferLinks).toHaveText(`#${doc.id}`);
    await expect(card.errorTransferLinks).toHaveAttribute('href', `/transfers?id=${doc.id}`);
    await expect(card.taskClient, 'клиент в форме вернулся к сохранённому').toContainText(client.company_name);
    await expect(card.modal).toBeVisible();

    expect((await adminApi.getTask(task.id)).client_id, 'клиент заказа не изменён').toBe(client.id);
    expect((await adminApi.getCard(task.id)).equipment, 'оборудование заказа не тронуто').toHaveLength(1);
    expect((await adminApi.getTransfer(doc.id)).status).toBe('received');
  });
});
