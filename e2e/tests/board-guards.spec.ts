import { test, expect } from '../fixtures/board';
import { ClientsPage } from '../pages/ClientsPage';
import { EquipmentPage } from '../pages/EquipmentPage';
import { CatalogPage } from '../pages/CatalogPage';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';

// Фаза 8 (D26) → soft-delete-catalog DS-014, TS-F4: товар и услуга каталога, на которые
// ссылается заказ, больше не удерживаются — удаляются мягко с причиной, в карточке заказа
// остаются с бейджем «удалён» (BD-019a / BD-019b).
// Клиент и оборудование (soft-delete-clients DS-014, DS-041, TS-F4): заказы и документы их
// больше не удерживают — удаление мягкое (SDCL-001, soft-delete-clients.spec.ts); отказ остался
// один — оборудование не у клиента (на нашем складе, в пути либо в незавершённом перемещении).
test.use({ colorScheme: 'light' });

test.describe('BD: нельзя удалить клиента, пока его оборудование на складе (DS-041)', () => {
  test('BD-017 (TS-053): удаление клиента с оборудованием на нашем складе — «Оборудование клиента на складе или в пути — сначала верните его клиенту», клиент остаётся', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const company = uniq('BD-017 клиент с оборудованием на складе');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990001111', note: '' }] });
    const type = await adminApi.createEquipmentType({ name: uniq('BD-017 тип') });
    const equipment = await adminApi.createEquipment(client.id, { type_id: type.id, model: uniq('BD-017 модель') });
    const columns = (await adminApi.listColumns()).items;
    const task = await adminApi.createTask({ title: uniq('BD-017 заказ'), client_id: client.id, column_id: columns[0].id });
    const warehouse = await adminApi.defaultWarehouse();
    await adminApi.addTaskEquipmentAt(task.id, equipment.id, warehouse.id); // принята на склад документом

    allowHttpErrors((url, status) => url.endsWith(`/clients/${client.id}/delete`) && status === 409);

    const clients = new ClientsPage(page);
    await clients.goto(`?q=${company}`);
    await clients.open(company);
    await clients.confirmDelete(uniq('BD-017 попытка'));

    // карточка перерисована с красным блоком (POST не редиректит на список — форма остаётся
    // с данными клиента, не голый JSON из FiberErrorHandler).
    await expect(clients.formError).toHaveText('Оборудование клиента на складе или в пути — сначала верните его клиенту');
    await expect(clients.companyName).toHaveValue(company);

    // клиент по-прежнему присутствует в списке.
    await clients.goto(`?q=${company}`);
    await expect(clients.row(company)).toBeVisible();
  });
});

test.describe('BD: нельзя удалить оборудование, пока оно на складе (DS-041)', () => {
  test('BD-018 (TS-053): удаление единицы на нашем складе — «Оборудование на складе или в пути — сначала верните его клиенту», оборудование остаётся', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const company = uniq('BD-018 клиент оборудования');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990002222', note: '' }] });
    const type = await adminApi.createEquipmentType({ name: uniq('BD-018 тип') });
    const equipment = await adminApi.createEquipment(client.id, { type_id: type.id, model: uniq('BD-018 модель') });
    const columns = (await adminApi.listColumns()).items;
    const task = await adminApi.createTask({ title: uniq('BD-018 заказ'), client_id: client.id, column_id: columns[0].id });
    const warehouse = await adminApi.defaultWarehouse();
    await adminApi.addTaskEquipmentAt(task.id, equipment.id, warehouse.id); // принята на склад документом

    allowHttpErrors((url, status) => url.endsWith(`/equipment/${equipment.id}/delete`) && status === 409);

    const eq = new EquipmentPage(page);
    await eq.gotoEdit(equipment.id);
    await eq.confirmDelete(uniq('BD-018 попытка'));

    // оборудование остаётся на карточке — форма показывает красный блок.
    await expect(eq.formError).toHaveText('Оборудование на складе или в пути — сначала верните его клиенту');

    // оборудование по-прежнему присутствует у клиента.
    const clients = new ClientsPage(page);
    await clients.goto(`?q=${company}`);
    await clients.open(company);
    await expect(page.getByTestId('equipment-table')).toContainText(equipment.model);
  });
});

test.describe('BD: товар и услуга каталога удаляются при наличии заказов (soft-delete-catalog DS-014)', () => {
  test('BD-019a (TS-027): товар в услуге действующего заказа удаляется с причиной — список без товара, тост; в карточке заказа у товара бейдж «удалён», строка остаётся', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq('BD-019a категория товаров') });
    const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq('BD-019a товар') });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-019a категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-019a услуга') });
    const client = await adminApi.createClient({ company_name: uniq('BD-019a клиент'), phones: [{ phone: '9990003333', note: '' }] });
    const column = (await adminApi.listColumns()).items.find((c) => !c.is_final && !c.consumes_stock)!;
    const task = await adminApi.createTask({ title: uniq('BD-019a заказ'), client_id: client.id, column_id: column.id });
    await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 1 }] });

    const catalog = new CatalogPage(page);
    // pagination DS-005: список постраничный — своя запись только через ?q=.
    await catalog.gotoProductsByName(product.name);
    await catalog.open(product.name);
    await catalog.confirmDelete(uniq('BD-019a снят с продажи'));

    // заказ удалению не мешает — отказа «используется» больше нет.
    await expect(page).toHaveURL(/\/catalog\/products\?flash=deleted$/);
    await expect(catalog.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
    await catalog.gotoProductsByName(product.name);
    await expect(catalog.row(product.name)).toHaveCount(0);

    // в карточке заказа строка товара осталась — с бейджем «удалён».
    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    const card = new OrderCardPage(page);
    await card.waitOpen();
    await expect(page.getByTestId('card-product-deleted')).toHaveText('удалён');
    await expect(page.getByTestId('card-subtask-service-deleted')).toHaveCount(0);
    const apiCard = await adminApi.getCard(task.id);
    expect(apiCard.subtasks[0].products.map((p) => p.product_is_deleted)).toEqual([true]);
  });

  test('BD-019b (TS-030): услуга каталога в действующем заказе удаляется с причиной — список без услуги, тост; в карточке заказа у услуги бейдж «удалён», услуга заказа остаётся', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
  }) => {
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-019b категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-019b услуга') });
    const client = await adminApi.createClient({ company_name: uniq('BD-019b клиент'), phones: [{ phone: '9990004444', note: '' }] });
    const column = (await adminApi.listColumns()).items.find((c) => !c.is_final && !c.consumes_stock)!;
    const task = await adminApi.createTask({ title: uniq('BD-019b заказ'), client_id: client.id, column_id: column.id });
    const subtaskId = await insertSubtask({ taskId: task.id, serviceId: svc.id });

    const catalog = new CatalogPage(page);
    // ?q= — без фильтра список услуг на общей dev-БД (годы e2e-прогонов) отдаёт первую
    // страницу, куда свежесозданная услуга не гарантированно попадает (тот же класс
    // проблемы объёма данных, что и у клиентов/колонок/заказов — BD-006/BD-009/BD-013,
    // code-plan-status.md фазы 05); эталон — clients.goto(`?q=${company}`) выше в этом файле.
    await catalog.gotoServices(`?q=${encodeURIComponent(svc.name)}`);
    await catalog.openService(svc.name);
    await catalog.confirmServiceDelete(uniq('BD-019b не оказываем'));

    await expect(page).toHaveURL(/\/catalog\/services\?flash=service_deleted$/);
    await expect(catalog.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
    await catalog.gotoServices(`?q=${encodeURIComponent(svc.name)}`);
    await expect(catalog.serviceRow(svc.name)).toHaveCount(0);

    // услуга заказа осталась — с бейджем «удалён» у названия.
    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    const card = new OrderCardPage(page);
    await card.waitOpen();
    await expect(card.subtaskRow(subtaskId).getByTestId('card-subtask-service-deleted')).toHaveText('удалён');
    const apiCard = await adminApi.getCard(task.id);
    expect(apiCard.subtasks.map((s) => s.service_is_deleted)).toEqual([true]);
  });
});
