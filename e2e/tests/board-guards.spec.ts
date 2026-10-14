import { test, expect } from '../fixtures/board';
import { CatalogPage } from '../pages/CatalogPage';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';

// Фаза 8 (D26) → soft-delete-catalog DS-014, TS-F4: товар и услуга каталога, на которые
// ссылается заказ, больше не удерживаются — удаляются мягко с причиной, в карточке заказа
// остаются с бейджем «удалён» (BD-019a / BD-019b).
// Клиент и оборудование (soft-delete-clients DS-014, TS-F4): заказы их не удерживают —
// удаление мягкое (SDCL-001, soft-delete-clients.spec.ts).
test.use({ colorScheme: 'light' });

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
