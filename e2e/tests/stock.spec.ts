import { test, expect, columnByName } from '../fixtures/board';
import { StockPage } from '../pages/StockPage';
import { BoardPage } from '../pages/BoardPage';
import { loginContext, type AdminApi } from '../fixtures/auth';

// Фаза 7: экран «Остатки» «/stock» — KPI, таблица рисковых с прогнозом, правка остатка
// (BD-015, BD-016, design-tests.md). Изоляция — своя связка категория/товар/клиент/заказ
// через uniq()+adminApi на общей dev-БД (§TD-6.2); проверки KPI — дельтой до/после,
// не абсолютными числами (накопленные данные прошлых прогонов).
test.use({ colorScheme: 'light' });

async function setup(adminApi: AdminApi, uniq: (s: string) => string, label: string) {
  const client = await adminApi.createClient({ company_name: uniq(`ST клиент ${label}`), phones: [{ phone: '9990000009', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  return { client, columns };
}

function csrfToken(page: import('@playwright/test').Page): Promise<string> {
  return page.locator('meta[name="csrf-token"]').getAttribute('content').then((v) => v ?? '');
}

test.describe('BD: экран «Остатки»', () => {
  test('BD-015: KPI, таблица рисковых с прогнозом; admin правит остаток; executor — без кнопки правки', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const willLowBefore = (await adminApi.getStockSummary()).will_low_count;

    const { client, columns } = await setup(adminApi, uniq, 'BD-015');
    const from = columnByName(columns, 'Приёмка'); // не списывающая — товар считается "в новых заказах" (pending)

    const product = await adminApi.createProduct({ name: uniq('BD-015 товар'), stock: 10, min_stock: 5, box: 'A1', place: 'Стеллаж 1' });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-015 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-015 услуга') });
    const task = await adminApi.createTask({ title: uniq('BD-015 заказ'), client_id: client.id, column_id: from.id });
    // потребность 7 при остатке 10 и минимуме 5 → прогноз 3 (≤ минимума) — «станет минимумом после заказов».
    await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 7 }] });

    const willLowAfterSetup = (await adminApi.getStockSummary()).will_low_count;
    expect(willLowAfterSetup).toBe(willLowBefore + 1);

    const stock = new StockPage(page);
    await stock.goto();

    // KPI-плитка отражает свежую сумму (перезагружена JS'ом при заходе на страницу).
    await expect(stock.kpiWillLow).toHaveText(String(willLowAfterSetup));

    // scope=risky (дефолт) — товар виден с корректными «в новых заказах»/«прогноз».
    const row = stock.row(product.id);
    await expect(row).toBeVisible();
    await expect(row).toContainText(product.name);
    await expect(row).toContainText('10'); // в наличии
    await expect(row).toContainText('5'); // минимум
    await expect(row).toContainText('7'); // в новых заказах
    await expect(row).toContainText('3'); // прогноз
    await expect(row).toContainText('станет минимумом после заказов');

    // admin правит остаток — увеличивает так, что товар выходит из риска.
    await stock.openEdit(product.id);
    await stock.submitValue(20);
    await expect(stock.form).toBeHidden();

    // ушёл из риска — на scope=risky больше не отображается.
    await expect(stock.row(product.id)).toHaveCount(0);
    const willLowAfterEdit = (await adminApi.getStockSummary()).will_low_count;
    expect(willLowAfterEdit).toBe(willLowBefore);

    // scope=all — товар виден с новым остатком, состояние «норма».
    await stock.chooseScope('Все товары');
    await expect(stock.row(product.id)).toBeVisible();
    await expect(stock.row(product.id)).toContainText('20');
    await expect(stock.row(product.id)).toContainText('норма');
    await expect.poll(async () => (await adminApi.getStock({ scope: 'all', q: product.name })).items[0]?.stock).toBe(20);

    // executor: без кнопки правки (testid отсутствует), прямой PUT — 403.
    const login = uniq('exec-bd015');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);
    await stock.goto();
    await stock.chooseScope('Все товары');
    await expect(stock.row(product.id)).toBeVisible();
    await expect(stock.editBtn(product.id)).toHaveCount(0);

    allowHttpErrors((url, status) => url.endsWith(`/api/v1/stock/${product.id}`) && status === 403);
    const res = await page.request.put(`/api/v1/stock/${product.id}`, {
      data: { stock: 1 },
      headers: { 'X-Csrf-Token': await csrfToken(page) },
      failOnStatusCode: false,
    });
    expect(res.status()).toBe(403);
    const body = await res.json();
    expect(body.error?.code ?? body.code).toBe('FORBIDDEN');
  });
});

test.describe('BD: бейдж «Остатков» живой', () => {
  test('BD-016: бейдж виден executor и admin; после move с уходом в минимум — +1 без перезагрузки', async ({
    adminPage: page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    insertSubtask,
  }) => {
    const { client, columns } = await setup(adminApi, uniq, 'BD-016');
    const from = columnByName(columns, 'Приёмка');
    const work = columnByName(columns, 'В ремонте'); // consumes_stock=true (сид D32)

    const product = await adminApi.createProduct({ name: uniq('BD-016 товар'), stock: 6, min_stock: 5 });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('BD-016 категория услуг') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('BD-016 услуга') });
    const task = await adminApi.createTask({ title: uniq('BD-016 заказ'), client_id: client.id, column_id: from.id });

    // baseline — из API (не из DOM: сразу после goto() фоновый fetch бейджа ещё может
    // не завершиться, чтение textContent() без retry гонялось бы с ним).
    const before = (await adminApi.getStockSummary()).risk_count;

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    const badge = page.getByTestId('nav-badge-stock');

    // подзадача заводится ПОСЛЕ открытия доски (raw pg-фикстура, ADR-103) — открытая
    // страница ничего не знает об этом изменении до следующего вызова KT.refreshNavBadge().
    // остаток 6, минимум 5, потребность 2 → после списания 4 (≤ минимума) — уходит в риск.
    await insertSubtask({ taskId: task.id, serviceId: svc.id, products: [{ productId: product.id, qty: 2 }] });

    // перенос в списывающую колонку через UI — board.js вызывает KT.refreshNavBadge()
    // после успешного move (design-go-layers.md §Frontend Modules) без перезагрузки страницы.
    await board.dragCardToColumn(task.id, work.id);
    await expect.poll(async () => (await adminApi.getTask(task.id)).column_id).toBe(work.id);

    const after = (await adminApi.getStockSummary()).risk_count;
    expect(after).toBe(before + 1);
    await expect.poll(async () => Number((await badge.textContent()) || '0')).toBe(after);
    await expect(badge).toBeVisible();

    // executor тоже видит бейдж (риск уже есть — гарантирован нашим товаром).
    const login = uniq('exec-bd016');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);
    await page.goto('/stock');
    await expect(page.getByTestId('nav-badge-stock')).toBeVisible();
    await expect(page.getByTestId('nav-badge-stock')).toHaveText(String(after));
  });
});

test.describe('BD: «Остатки» не обрезают список молча', () => {
  test('BD-023: товаров больше, чем пришло одной страницей, — пометка «Показаны первые N из M»', async ({ adminPage: page }) => {
    const stock = new StockPage(page);
    await stock.goto();
    await expect(stock.table).toBeVisible();
    await expect(page.getByTestId('stock-shown')).toBeHidden();

    // Список грузится одной страницей limit=1000. Тысячу товаров не заводим: в ответе
    // сервера total увеличиваем на 5 — для stock.js это та же обрезка.
    let shown = 0;
    await page.route(
      (url) => url.pathname === '/api/v1/stock',
      async (route) => {
        const resp = await route.fetch();
        const body = await resp.json();
        shown = body.items.length;
        body.total = shown + 5;
        await route.fulfill({ response: resp, json: body });
      },
    );
    await page.reload();
    await expect(page.getByTestId('stock-shown')).toBeVisible();
    await expect(page.getByTestId('stock-shown')).toHaveText(`Показаны первые ${shown} из ${shown + 5}, уточните поиск`);
  });
});

