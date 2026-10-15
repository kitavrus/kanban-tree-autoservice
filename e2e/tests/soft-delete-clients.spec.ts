import { test, expect } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { ClientsPage } from '../pages/ClientsPage';
import { DeletedPage } from '../pages/DeletedPage';
import { EquipmentPage } from '../pages/EquipmentPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { uniqPlate } from '../pages/NewOrderDialog';

// Мягкое удаление — клиенты и оборудование клиентов (docs/features/soft-delete-clients/
// draft-spec.md §8.1–§8.3, §11): клиент с заказом и оборудованием удаляется с причиной, в заказе
// остаются бейджи «удалён», вкладки «Клиенты» и «Оборудование» раздела «Удалённые»,
// восстановление клиента возвращает всё (TS-043). Отказы «оборудование не у клиента» —
// BD-017 / BD-018 (board-guards.spec.ts); окно «Удаление» без причины — CL-008, EQ-006.
// Адаптив и темы раздела «Удалённые» и окна удаления — SD-007 / SD-008 (soft-delete-core.spec.ts).
// Общая dev-БД (§TD-6.2): свои клиенты, единицы и заказы через uniq()+adminApi. Уборка (TS-F9):
// заказы — _boardCleanup (purgeTask), клиенты с единицами — adminApi.purgeClient служебным
// адресом purge (стенд с TEST_PURGE_ENABLED=true).
test.use({ colorScheme: 'light' });

const DATE_TIME = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;

test.describe('SDCL: мягкое удаление клиентов и оборудования', () => {
  test('SDCL-001 (TS-043): клиент с заказом и двумя единицами удаляется с причиной → список без клиента, тост «Удалено вместе с автомобилями: 2…»; на доске и в карточке заказа бейдж «удалён» у клиента и обеих единиц; в «Удалённых» вкладка «Клиенты» — строка клиента, вкладка «Оборудование» — единицы с бейджем «вместе с клиентом»; «Восстановить» клиента возвращает всё, бейджи исчезают; негатив: «Восстановить» у единицы удалённого клиента → «Сначала восстановите клиента», строка остаётся', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const company = uniq('SDCL-001 клиент');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990004301', note: '' }] });
    const first = await adminApi.createEquipment(client.id, { plate_number: uniqPlate() });
    const second = await adminApi.createEquipment(client.id, { plate_number: uniqPlate() });
    const column = (await adminApi.listColumns()).items.find((c) => !c.is_final)!;
    const task = await adminApi.createTask({ title: uniq('SDCL-001 заказ'), client_id: client.id, column_id: column.id });
    await adminApi.addTaskEquipment(task.id, first.id);
    await adminApi.addTaskEquipment(task.id, second.id);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/equipment/${first.id}/restore`) && status === 409);

    try {
      // §8.1: форма клиента → окно «Удаление» с пояснением и числом единиц.
      const clients = new ClientsPage(page);
      await clients.goto(`?q=${encodeURIComponent(company)}`);
      await expect(clients.equipmentCount(company)).toHaveText('2');
      await clients.open(company);
      await clients.openDelete();
      await expect(clients.deleteDialogTitle).toHaveText(`Удалить «${company}»?`);
      await expect(clients.deleteDialogText).toHaveText(
        'Автомобили клиента (2 шт.) будут удалены вместе с ним. Заказы клиента останутся на доске с пометкой «удалён». Удалить можно, только когда все автомобили у клиента',
      );
      const reason = uniq('SDCL-001 клиент закрылся');
      await clients.deleteReason.fill(reason);
      await clients.deleteDialogOk.click();
      await expect(page).toHaveURL(/\/clients\?flash=deleted&n=2$/);
      await expect(clients.toast).toHaveText('Удалено вместе с автомобилями: 2. Восстановить можно в разделе «Удалённые»');
      await clients.goto(`?q=${encodeURIComponent(company)}`);
      await expect(clients.row(company)).toHaveCount(0);
      await expect(clients.empty).toBeVisible();

      // §8.3: заказ остаётся на доске — бейдж «удалён» у клиента на карточке доски.
      const board = new BoardPage(page);
      await board.goto(`?client_id=${client.id}`);
      await expect(board.card(task.id)).toBeVisible();
      await expect(board.card(task.id).getByTestId('task-card-client-deleted')).toHaveText('удалён');

      // карточка заказа: бейдж у клиента и у обеих единиц блока «Оборудование».
      await board.openTaskEdit(task.id);
      const card = new OrderCardPage(page);
      await card.waitOpen();
      await expect(page.getByTestId('task-client-deleted')).toBeVisible();
      await expect(page.getByTestId('task-client-deleted')).toHaveText('удалён');
      for (const unit of [first, second]) {
        await expect(card.equipmentRow(unit.id).getByTestId('card-equipment-deleted')).toHaveText('удалён');
      }
      // удалённому клиенту оборудование не добавить, истории удалённых единиц нет (DS-026):
      // вместо выбора — пояснение, кнопок «История работ» у удалённых единиц нет.
      await expect(page.getByTestId('card-equipment-client-deleted')).toHaveText('Клиент заказа удалён — добавить автомобиль нельзя');
      await expect(page.getByTestId('card-equipment-add')).toHaveCount(0);
      await expect(page.getByTestId('card-equipment-new')).toHaveCount(0);
      await expect(page.getByTestId('card-equipment-history-toggle')).toHaveCount(0);
      const apiCard = await adminApi.getCard(task.id);
      expect(apiCard.task.client_is_deleted).toBe(true);
      expect(apiCard.equipment.map((e) => e.is_deleted)).toEqual([true, true]);

      // §8.2: «Удалённые» → вкладка «Клиенты».
      const del = new DeletedPage(page);
      await del.goto(`kind=clients&q=${encodeURIComponent(company)}`);
      await expect(del.tab('clients')).toHaveClass(/active/);
      await expect(del.tab('clients')).toContainText('Клиенты');
      await expect(del.tab('equipment')).toContainText('Автомобили');
      await expect(del.rows).toHaveCount(1);
      await expect(del.rowTitle(client.id)).toHaveText(company);
      await expect(del.rowSubtitle(client.id)).toHaveCount(0);
      await expect(del.rowReason(client.id)).toHaveText(reason);
      await expect(del.rowBy(client.id)).toHaveText('Админ Прогона');
      await expect(del.rowAt(client.id)).toHaveText(DATE_TIME);
      const summary = await adminApi.deletedSummary();
      await expect(del.tabCount('clients')).toHaveText(String(summary.counts.clients));
      await expect(del.tabCount('equipment')).toHaveText(String(summary.counts.equipment));

      // вкладка «Оборудование»: обе единицы с бейджем «вместе с клиентом» и причиной клиента.
      await del.tab('equipment').click();
      await expect(del.tab('equipment')).toHaveClass(/active/);
      await expect(del.rows).toHaveCount(2);
      for (const unit of [first, second]) {
        await expect(del.rowTitle(unit.id)).toHaveText(unit.plate_number);
        await expect(del.rowSubtitle(unit.id)).toHaveText(company);
        await expect(del.rowWith(unit.id)).toHaveText('вместе с клиентом');
        await expect(del.rowReason(unit.id)).toHaveText(reason);
      }

      // негатив: единицу удалённого клиента отдельно не восстановить — тост, строка остаётся.
      await del.restoreBtn(first.id).click();
      await expect(del.dialogText).toHaveText(`Восстановить «${first.plate_number}»?`);
      await del.dialogOk.click();
      await expect(del.toastError).toHaveText('Сначала восстановите клиента');
      await expect(del.row(first.id)).toBeVisible();
      expect((await adminApi.listDeleted('equipment', { q: company })).total, 'единицы остались в удалённых').toBe(2);

      // восстановление клиента возвращает его и обе единицы.
      await del.tab('clients').click();
      await expect(del.row(client.id)).toBeVisible();
      await del.restoreBtn(client.id).click();
      await expect(del.dialogText).toHaveText(`Восстановить «${company}» вместе с автомобилями?`);
      await del.dialogOk.click();
      await expect(del.toast).toHaveText('Восстановлено вместе с автомобилями: 2');
      await expect(del.row(client.id)).toHaveCount(0);
      await expect(del.tabCount('clients')).toHaveText(String(summary.counts.clients - 1));
      await expect(del.tabCount('equipment')).toHaveText(String(summary.counts.equipment - 2));
      expect((await adminApi.listDeleted('equipment', { q: company })).total).toBe(0);
      expect((await adminApi.getClient(client.id)).equipment_count).toBe(2);

      // бейджи исчезли: доска и карточка заказа.
      await board.goto(`?client_id=${client.id}`);
      await expect(board.card(task.id)).toBeVisible();
      await expect(board.card(task.id).getByTestId('task-card-client-deleted')).toHaveCount(0);
      await board.openTaskEdit(task.id);
      await card.waitOpen();
      await expect(card.equipmentRow(first.id)).toBeVisible();
      await expect(page.getByTestId('task-client-deleted')).toBeHidden();
      await expect(page.getByTestId('card-equipment-deleted')).toHaveCount(0);
      await expect(page.getByTestId('card-equipment-client-deleted')).toHaveCount(0);
      await expect(page.getByTestId('card-equipment-new')).toBeVisible();
    } finally {
      await adminApi.purgeClient(client.id, [task.id]);
    }
  });

  test('SDCL-002 (TS-043): единица заказа удаляется отдельно с причиной → карточка клиента без единицы, тост; в карточке заказа бейдж «удалён» только у неё; в «Удалённых» вкладка «Оборудование» — госномер, клиент, без бейджа «вместе с клиентом»; «Восстановить» → тост «Восстановлено», единица снова у клиента; негатив: клиент при этом не удалён и бейджа у клиента нет', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('SDCL-002 клиент');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990004302', note: '' }] });
    const gone = await adminApi.createEquipment(client.id, { plate_number: uniqPlate() });
    const kept = await adminApi.createEquipment(client.id, { plate_number: uniqPlate() });
    const column = (await adminApi.listColumns()).items.find((c) => !c.is_final)!;
    const task = await adminApi.createTask({ title: uniq('SDCL-002 заказ'), client_id: client.id, column_id: column.id });
    await adminApi.addTaskEquipment(task.id, gone.id);
    await adminApi.addTaskEquipment(task.id, kept.id);

    try {
      // §8.1: форма единицы → окно «Удаление»; единица в заказе удалению не мешает (DS-014).
      const eq = new EquipmentPage(page);
      await eq.gotoEdit(gone.id);
      const reason = uniq('SDCL-002 списана клиентом');
      await eq.confirmDelete(reason);
      await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_deleted$`));
      await expect(eq.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
      await expect(page.getByTestId('equipment-table')).toContainText(kept.plate_number);
      await expect(page.getByTestId('equipment-table')).not.toContainText(gone.plate_number);

      // §8.3: в карточке заказа бейдж только у удалённой единицы; клиент — без бейджа.
      const board = new BoardPage(page);
      await board.goto(`?client_id=${client.id}`);
      await expect(board.card(task.id).getByTestId('task-card-client-deleted')).toHaveCount(0);
      await board.openTaskEdit(task.id);
      const card = new OrderCardPage(page);
      await card.waitOpen();
      await expect(card.equipmentRow(gone.id).getByTestId('card-equipment-deleted')).toHaveText('удалён');
      await expect(card.equipmentRow(kept.id)).toBeVisible();
      await expect(card.equipmentRow(kept.id).getByTestId('card-equipment-deleted')).toHaveCount(0);
      await expect(page.getByTestId('task-client-deleted')).toBeHidden();

      // §8.2: вкладка «Оборудование» — без бейджа «вместе с клиентом».
      const del = new DeletedPage(page);
      await del.goto(`kind=equipment&q=${encodeURIComponent(gone.plate_number)}`);
      await expect(del.rows).toHaveCount(1);
      await expect(del.rowTitle(gone.id)).toHaveText(gone.plate_number);
      await expect(del.rowSubtitle(gone.id)).toHaveText(company);
      await expect(del.rowWith(gone.id)).toHaveCount(0);
      await expect(del.rowReason(gone.id)).toHaveText(reason);
      await expect(del.rowAt(gone.id)).toHaveText(DATE_TIME);

      await del.restore(gone.id);
      await expect(del.toast).toHaveText('Восстановлено');
      await expect(del.row(gone.id)).toHaveCount(0);
      await expect(del.empty).toBeVisible();
      expect((await adminApi.getClient(client.id)).equipment_count, 'единица снова у клиента').toBe(2);
    } finally {
      await adminApi.purgeClient(client.id, [task.id]);
    }
  });
});
