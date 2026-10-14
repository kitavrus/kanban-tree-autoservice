import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { CatalogPage } from '../pages/CatalogPage';

// pagination (docs/features/pagination/draft-spec.md): доска — ленивая догрузка по колонке
// (DS-001) и счётчик заказов (DS-004); каталог — SSR-пейджер товаров (DS-005); форма
// услуги — серверный поиск товара в комбобоксе комплекта (DS-007). TS-007…TS-010.
// Каждый тест заводит свои данные (uniq()+adminApi) и смотрит только на них: доска —
// отфильтрованная по собственному ?client_id=, каталог — по собственной категории
// (§TD-6.2: dev-БД общая и раздута прошлыми прогонами, глобальные количества не стабильны).
test.use({ colorScheme: 'light' });

const PER_COLUMN = 50; // board.js PER_COLUMN (DS-001)

test.describe('PG: доска — догрузка по колонке и счётчик', () => {
  test('TS-007/TS-008: колонка с 52 заказами → 50 карточек и «показаны 50 из 52»; «Показать ещё» догружает остальные', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    test.setTimeout(120_000); // 52 заказа через API (фикстура) — дольше дефолта
    const client = await adminApi.createClient({ company_name: uniq('PG-007 клиент'), phones: [{ phone: '9990000701', note: '' }] });
    const col = columnByName((await adminApi.listColumns()).items, 'Новые');
    const total = PER_COLUMN + 2;
    const ids: number[] = [];
    for (let i = 0; i < total; i++) {
      const t = await adminApi.createTask({ title: uniq(`PG-007 заказ ${String(i + 1).padStart(3, '0')}`), client_id: client.id, column_id: col.id });
      ids.push(t.id);
    }

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);

    const body = board.columnBody(col.id);
    await expect(body.locator('.task-card')).toHaveCount(PER_COLUMN);
    // первые 50 по position,id — порядок создания; 51-й и 52-й ещё не загружены.
    await expect(board.card(ids[0])).toBeVisible();
    await expect(board.card(ids[PER_COLUMN - 1])).toBeAttached();
    await expect(board.card(ids[PER_COLUMN])).toHaveCount(0);
    await expect(board.columnShown(col.id)).toHaveText(`показаны ${PER_COLUMN} из ${total}`);
    // TS-008 (DS-004): на доске видно меньше, чем подходит под фильтр.
    await expect(board.found).toHaveText(`Заказов: ${total} · показаны ${PER_COLUMN}`);

    const more = page.waitForResponse((r) => r.url().includes('/api/v1/tasks?') && r.url().includes(`column_id=${col.id}`) && r.url().includes(`offset=${PER_COLUMN}`));
    await board.columnMore(col.id).click();
    expect((await more).status()).toBe(200);

    await expect(body.locator('.task-card')).toHaveCount(total);
    await expect(board.card(ids[total - 1])).toBeAttached();
    await expect(board.columnShown(col.id)).toHaveCount(0);
    await expect(board.columnMore(col.id)).toHaveCount(0);
    await expect(board.found).toHaveText(`Заказов: ${total}`);
  });

  test('TS-008: board-total показывает число заказов под фильтром', async ({ adminPage: page, adminApi, uniq }) => {
    const client = await adminApi.createClient({ company_name: uniq('PG-008 клиент'), phones: [{ phone: '9990000801', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    await adminApi.createTask({ title: uniq('PG-008 a'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    await adminApi.createTask({ title: uniq('PG-008 b'), client_id: client.id, column_id: columnByName(columns, 'Новые').id });
    await adminApi.createTask({ title: uniq('PG-008 c'), client_id: client.id, column_id: columnByName(columns, 'Согласование').id });

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.found).toHaveText('Заказов: 3');
    await expect(page.locator('[data-testid^="column-more-"]')).toHaveCount(0);
  });

  test('TS-011: счётчики колонок пересчитываются при переносе и возвращаются при откате', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
  }) => {
    const client = await adminApi.createClient({ company_name: uniq('PG-011 клиент'), phones: [{ phone: '9990001101', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const fresh = columnByName(columns, 'Новые');
    const agree = columnByName(columns, 'Согласование');
    const done = columnByName(columns, 'Готово'); // финальная: открытая услуга → 409 и откат
    const moved = await adminApi.createTask({ title: uniq('PG-011 переносимый'), client_id: client.id, column_id: fresh.id });
    const blocked = await adminApi.createTask({ title: uniq('PG-011 с открытой услугой'), client_id: client.id, column_id: fresh.id });
    // заказ уже в «Готово»: у колонки есть M, и оптимистичный +1 при отклонённом переносе
    // виден в счётчике (иначе счётчик = число карточек и откат M не проверялся бы).
    await adminApi.createTask({ title: uniq('PG-011 готовый'), client_id: client.id, column_id: done.id });
    const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq('PG-011 категория') });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('PG-011 услуга') });
    await insertSubtask({ taskId: blocked.id, serviceId: svc.id, status: 'new' });
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${blocked.id}/move`) && status === 409);

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.columnCount(fresh.id)).toHaveText('2');
    await expect(board.columnCount(agree.id)).toHaveText('0');
    await expect(board.columnCount(done.id)).toHaveText('1');
    await expect(board.found).toHaveText('Заказов: 3');

    // Успешный перенос: M «откуда» −1, «куда» +1; после ответа сервера и reload — то же.
    await board.dragCardToColumn(moved.id, agree.id);
    await expect.poll(async () => (await adminApi.getTask(moved.id)).column_id).toBe(agree.id);
    await expect(board.columnCount(fresh.id)).toHaveText('1');
    await expect(board.columnCount(agree.id)).toHaveText('1');
    await expect(board.found).toHaveText('Заказов: 3');
    await page.reload();
    await expect(board.columnCount(fresh.id)).toHaveText('1');
    await expect(board.columnCount(agree.id)).toHaveText('1');

    // Отклонённый перенос (409): оптимистичный пересчёт откатывается.
    await board.dragCardToColumn(blocked.id, done.id);
    await expect(page.getByTestId('ui-dialog')).toBeVisible();
    await page.getByTestId('ui-dialog-ok').click();
    await expect(board.columnBody(fresh.id).getByTestId(`task-card-${blocked.id}`)).toBeVisible();
    await expect(board.columnCount(fresh.id)).toHaveText('1');
    await expect(board.columnCount(done.id)).toHaveText('1');
    await expect(board.columnCount(agree.id)).toHaveText('1');
    await expect(board.found).toHaveText('Заказов: 3');
  });

  test('TS-012: колонка вне обрезанной первичной выдачи — «Показать заказы» догружает её с начала', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const client = await adminApi.createClient({ company_name: uniq('PG-012 клиент'), phones: [{ phone: '9990001201', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const fresh = columnByName(columns, 'Новые');
    const agree = columnByName(columns, 'Согласование');
    const kept = await adminApi.createTask({ title: uniq('PG-012 видимый'), client_id: client.id, column_id: fresh.id });
    const hidden1 = await adminApi.createTask({ title: uniq('PG-012 хвост 1'), client_id: client.id, column_id: agree.id });
    const hidden2 = await adminApi.createTask({ title: uniq('PG-012 хвост 2'), client_id: client.id, column_id: agree.id });

    // Ветка срабатывает, когда первичная выдача упёрлась в limit=1000 и до хвостовых колонок
    // не дошла. Тысячу заказов не заводим: в ответе первичной загрузки (per_column=) убираем
    // карточки «Согласования», total оставляем серверным — для board.js это та же обрезка.
    // Догрузка по кнопке идёт на настоящий сервер.
    await page.route(
      (url) => url.pathname === '/api/v1/tasks' && url.searchParams.has('per_column'),
      async (route) => {
        const resp = await route.fetch();
        const body = await resp.json();
        body.items = body.items.filter((t: { column_id: string }) => t.column_id !== agree.id);
        await route.fulfill({ response: resp, json: body });
      },
      { times: 1 },
    );

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await expect(board.card(kept.id)).toBeVisible();
    await expect(board.found).toHaveText('Заказов: 3 · показаны 1');
    await expect(board.columnShown(agree.id)).toHaveText('заказы колонки не загружены');
    await expect(board.columnMore(agree.id)).toHaveText('Показать заказы');
    await expect(board.columnMore(fresh.id)).toHaveCount(0);

    const more = page.waitForResponse((r) => r.url().includes('/api/v1/tasks?') && r.url().includes(`column_id=${agree.id}`) && r.url().includes('offset=0'));
    await board.columnMore(agree.id).click();
    expect((await more).status()).toBe(200);

    await expect(board.card(hidden1.id)).toBeVisible();
    await expect(board.card(hidden2.id)).toBeVisible();
    await expect(board.columnCount(agree.id)).toHaveText('2');
    await expect(board.columnMore(agree.id)).toHaveCount(0);
    await expect(board.columnShown(agree.id)).toHaveCount(0);
  });
});

test.describe('PG: каталог — пейджер товаров', () => {
  test('TS-009: >100 товаров → пейджер; «Вперёд» меняет строки, «Назад» возвращает; поиск сбрасывает на первую страницу', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    test.setTimeout(120_000); // 102 товара через API в СВОЕЙ категории (пейджер — по ?category=)
    const cat = await adminApi.createCategory({ kind: 'product', name: uniq('PG-009 категория') });
    const prefix = uniq('PG-009 товар');
    const n = 102; // минимум для второй страницы — меньше засоряем общую БД
    for (let i = 1; i <= n; i++) {
      await adminApi.createProduct({ category_id: cat.id, name: `${prefix} ${String(i).padStart(3, '0')}` });
    }
    const nameAt = (i: number) => `${prefix} ${String(i).padStart(3, '0')}`;

    const catalog = new CatalogPage(page);
    await catalog.gotoProducts(`?category=${cat.id}`);
    await expect(catalog.pagerShown).toHaveText(`Показаны 1–100 из ${n}`);
    await expect(page.getByTestId('product-row')).toHaveCount(100);
    await expect(catalog.row(nameAt(1))).toBeVisible();
    await expect(catalog.pagerPrev).toHaveCount(0);

    await catalog.pagerNext.click();
    await expect(page).toHaveURL(/offset=100/);
    await expect(catalog.pagerShown).toHaveText(`Показаны 101–${n} из ${n}`);
    await expect(page.getByTestId('product-row')).toHaveCount(n - 100);
    await expect(catalog.row(nameAt(101))).toBeVisible();
    await expect(catalog.row(nameAt(1))).toHaveCount(0);
    await expect(catalog.pagerNext).toHaveCount(0);

    await catalog.pagerPrev.click();
    await expect(page).not.toHaveURL(/offset=/);
    await expect(catalog.pagerShown).toHaveText(`Показаны 1–100 из ${n}`);
    await expect(catalog.row(nameAt(1))).toBeVisible();

    // поиск со второй страницы → снова первая (форма фильтра не несёт offset).
    await catalog.pagerNext.click();
    await expect(page).toHaveURL(/offset=100/);
    await catalog.search.fill(prefix);
    await catalog.search.press('Enter');
    await expect(page).not.toHaveURL(/offset=/);
    await expect(page).toHaveURL(/category=/);
    await expect(catalog.pagerShown).toHaveText(`Показаны 1–100 из ${n}`);
    await expect(catalog.row(nameAt(1))).toBeVisible();
  });
});

test.describe('PG: форма услуги — серверный поиск товара', () => {
  test('TS-010: товар находится вводом в комбобокс строки комплекта через GET /api/v1/products?q=', async ({
    adminPage: page,
    adminApi,
    uniq,
    catalogCleanup,
  }) => {
    const pcat = await adminApi.createCategory({ kind: 'product', name: uniq('PG-010 кат товаров') });
    const productName = uniq('PG-010 товар');
    const product = await adminApi.createProduct({ category_id: pcat.id, name: productName });
    const scatName = uniq('PG-010 кат услуг');
    await adminApi.createCategory({ kind: 'service', name: scatName });
    const serviceName = uniq('PG-010 услуга');

    const catalog = new CatalogPage(page);
    await catalog.gotoNewService();
    await catalog.fillService({ categoryName: scatName, name: serviceName });

    // Товар «за пределами предзагруженных 2000»: убираем его опцию из нативного select
    // строки — найти его теперь можно только серверным поиском (DS-007).
    await page.evaluate((id) => {
      document.querySelectorAll(`select[name="product_id"] option[value="${id}"]`).forEach((o) => o.remove());
    }, product.id);

    const row = catalog.productRow(0);
    await row.getByTestId('service-product-select').click();
    const dropdown = page.getByTestId('combobox-dropdown');
    await expect(dropdown).toBeVisible();
    const search = page.waitForResponse((r) => r.url().includes('/api/v1/products?') && r.url().includes('q='));
    await dropdown.locator('.cb-search').fill(productName);
    expect((await search).status()).toBe(200);
    await dropdown.locator('.cb-opt', { hasText: productName }).first().click();
    await catalog.fillProductRowQty(0, '2');

    await catalog.serviceSave.click();
    await expect(page).toHaveURL(/flash=service_created/);
    // услуга создана через экран — без уборки держит товар и категорию теста.
    const created = await adminApi.findCatalogByName('services', serviceName);
    if (created) catalogCleanup('services', created.id);
    await catalog.gotoServices(`?q=${encodeURIComponent(serviceName)}`);
    await expect(catalog.serviceRow(serviceName)).toContainText(productName);
  });
});
