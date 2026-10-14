import { test, expect } from '../fixtures/test';
import { CatalogPage } from '../pages/CatalogPage';
import { ShellPage } from '../pages/ShellPage';

// Фаза 4: «Каталог» — вкладки, категории, товары (подфаза A), услуги + комплект (подфаза B).
// Каждый тест независим от состояния БД стенда (§TD-6.2) — свои данные заводятся через
// uniq()/adminApi, проверка «есть/пусто» — только в рамках собственного uniq()-фильтра.
// Уборка (soft-delete-catalog TS-F10): записи, созданные через adminApi, убирает autouse-фикстура
// catalogCleanup служебным адресом purge; созданные через экран тест регистрирует сам.
test.use({ colorScheme: 'light' });

test.describe('CT: каталог — вкладки и меню', () => {
  test('CT-001: /catalog → услуги; вкладки переключаются; меню «Каталог» подсвечено', async ({ adminPage: page, adminApi, uniq }) => {
    const catalog = new CatalogPage(page);
    const shell = new ShellPage(page);

    // своя запись — проверяем видимость через собственный uniq()-фильтр, а не пустоту
    // общего списка (на постоянной БД стенда список услуг мог быть непустым и до этого теста).
    const catName = uniq('CT-001 категория услуг');
    const cat = await adminApi.createCategory({ kind: 'service', name: catName });
    const svcName = uniq('CT-001 услуга');
    await adminApi.createService({ category_id: cat.id, name: svcName });

    await catalog.gotoRoot();
    await expect(page).toHaveURL(/\/catalog\/services$/);
    await expect(catalog.tabServices).toHaveClass(/active/);
    await expect(shell.navLink('Каталог')).toHaveClass(/active/);

    await catalog.gotoServices(`?q=${encodeURIComponent(svcName)}`);
    await expect(catalog.serviceRow(svcName)).toBeVisible();

    await catalog.tabProducts.click();
    await expect(page).toHaveURL(/\/catalog\/products$/);
    await expect(catalog.tabProducts).toHaveClass(/active/);

    // вкладки «Типы оборудования» больше нет; у товаров нет панели категорий.
    await expect(page.getByTestId('catalog-tab-equipment-types')).toHaveCount(0);
    await expect(catalog.categoryPanel).toHaveCount(0);

    await catalog.tabServices.click();
    await expect(page).toHaveURL(/\/catalog\/services$/);
  });
});

test.describe('CT: категории', () => {
  test('CT-002: создать категорию услуг, дубль → ошибка, переименовать, фильтр в URL', async ({ adminPage: page, adminApi, uniq, catalogCleanup }) => {
    // дубль категории — SSR всегда 303 (флаг ошибки в query, не HTTP-статус) — сеть чистая.
    // Панель категорий есть только у услуг (у товаров категорий нет).
    const catalog = new CatalogPage(page);
    const name = uniq('Расходники');
    await catalog.gotoServices();

    await catalog.addCategory(name);
    await expect(page).toHaveURL(/flash=category_created/);
    await expect(catalog.flash).toHaveText('Категория создана');
    await expect(catalog.categoryRow(name)).toBeVisible();
    const createdCategory = await adminApi.findCatalogByName('catalog-categories', name);
    expect(createdCategory, 'категория создана через экран').toBeTruthy();
    catalogCleanup('catalog-categories', createdCategory!.id);

    // дубль (без учёта регистра/пробелов, D16) — сообщение об ошибке, категория не задвоена.
    await catalog.addCategory(`  ${name.toUpperCase()}  `);
    await expect(catalog.catError).toBeVisible();
    await expect(catalog.categoryRow(name)).toHaveCount(1);

    // переименование
    const renamed = uniq('Крепёж');
    await catalog.categoryRenameInput(name).fill(renamed);
    await catalog.categoryRenameSubmit(name).click();
    await expect(page).toHaveURL(/flash=category_saved/);
    await expect(catalog.categoryRow(renamed)).toBeVisible();

    // фильтр слева — попадает в URL и переживает reload.
    await catalog.categoryLink(renamed).click();
    await expect(page).toHaveURL(/[?&]category=/);
    const url = page.url();
    await page.reload();
    await expect(page).toHaveURL(url);
  });
});

test.describe('CT: товары', () => {
  test('CT-003: создать/править товар — поля, остаток у минимума выделен, отрицательный — «долг»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
    catalogCleanup,
  }) => {
    // required-валидация формы (SSR /catalog/products) отвечает 422 на саму навигацию.
    allowHttpErrors((url, status) => url.endsWith('/catalog/products') && status === 422);
    const catalog = new CatalogPage(page);
    const name = uniq('Фильтр масляный');
    await catalog.gotoNewProduct();
    await catalog.fill({ name, sku: 'F-100', unit: 'шт', price: '199.9', stock: '1', minStock: '2', box: 'К-01', place: 'Стеллаж A' });
    await catalog.save.click();

    await expect(page).toHaveURL(/\/catalog\/products\?flash=created$/);
    await expect(catalog.flash).toHaveText('Товар создан');
    const createdProduct = await adminApi.findCatalogByName('products', name);
    expect(createdProduct, 'товар создан через экран').toBeTruthy();
    catalogCleanup('products', createdProduct!.id);
    // редирект после создания не сохраняет фильтр — список без q может быть обрезан
    // лимитом на разросшемся стенде (§TD-6.2), проверяем свою запись через ?q=.
    await catalog.gotoProducts(`?q=${encodeURIComponent(name)}`);
    // остаток (1) ≤ минимума (2) — выделен.
    await expect(catalog.row(name).getByTestId('stock-low')).toBeVisible();

    // required-валидация: пустое название.
    await catalog.gotoNewProduct();
    await catalog.fill({ name: '' });
    await catalog.save.click();
    await expect(catalog.fieldError('name')).toHaveText('Обязательное поле');

    // правка на отрицательный остаток — «долг».
    await catalog.gotoProductsByName(name);
    await catalog.open(name);
    await catalog.fill({ stock: '-3' });
    await catalog.save.click();
    await expect(page).toHaveURL(/flash=saved/);
    // редирект после сохранения — первая страница без фильтра (pagination DS-005: 100 на
    // страницу), своя запись может быть дальше — проверяем через ?q=.
    await catalog.gotoProductsByName(name);
    await expect(catalog.row(name).getByTestId('stock-debt')).toBeVisible();
  });
});

test.describe('CT: услуги', () => {
  test('CT-004: услуга «по факту» с комплектом — добавить/удалить строки, количество, отображение в таблице', async ({
    adminPage: page,
    adminApi,
    uniq,
    catalogCleanup,
  }) => {
    const catalog = new CatalogPage(page);
    const svcCatName = uniq('Категория услуг');
    await adminApi.createCategory({ kind: 'service', name: svcCatName });
    const p1 = uniq('Прокладка ГБЦ');
    await adminApi.createProduct({ name: p1 });
    const p2 = uniq('Масло моторное');
    await adminApi.createProduct({ name: p2 });

    const name = uniq('Замена двигателя');
    await catalog.gotoNewService();
    await catalog.fillService({ categoryName: svcCatName, name });
    // «по факту» — цена не заполняется.
    await expect(catalog.servicePrice).toHaveValue('');

    await catalog.chooseProductInRow(0, p1);
    await catalog.fillProductRowQty(0, '2');
    await catalog.serviceProductAdd.click();
    await catalog.chooseProductInRow(1, p2);
    await catalog.fillProductRowQty(1, '1.5');
    // добавить и сразу удалить третью строку — комплект не должен её унести с собой.
    await catalog.serviceProductAdd.click();
    await catalog.removeProductRow(2);

    await catalog.serviceSave.click();
    await expect(page).toHaveURL(/\/catalog\/services\?flash=service_created$/);
    await expect(catalog.flash).toHaveText('Услуга создана');
    const createdService = await adminApi.findCatalogByName('services', name);
    expect(createdService, 'услуга создана через экран').toBeTruthy();
    catalogCleanup('services', createdService!.id);

    // редирект после создания не сохраняет фильтр — список без q может быть обрезан
    // лимитом на разросшемся стенде (§TD-6.2), проверяем свою запись через ?q=.
    await catalog.gotoServices(`?q=${encodeURIComponent(name)}`);
    const row = catalog.serviceRow(name);
    await expect(row).toBeVisible();
    await expect(row.getByTestId('service-price')).toHaveText('по факту');
    const productsCell = await row.getByTestId('service-products').textContent();
    expect(productsCell).toContain(`${p1} ×2.000`);
    expect(productsCell).toContain(`${p2} ×1.500`);
  });
});

test.describe('CT: удаление и архив', () => {
  test('CT-005 (TS-027): товар удаляется с причиной — и свободный, и стоящий в комплекте услуги (ссылки не мешают): список без товара, тост «Удалено. Восстановить можно в разделе «Удалённые»», в составе услуги — бейдж «удалён»; окно без причины — «Укажите причину удаления», товар не удалён; «В архив» — отдельное действие, архивный товар в комплекте новой услуги не предлагается', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const catalog = new CatalogPage(page);
    const svcCatName = uniq('Категория услуги для удаления');
    const svcCat = await adminApi.createCategory({ kind: 'service', name: svcCatName });

    const usedName = uniq('Товар в комплекте');
    const used = await adminApi.createProduct({ name: usedName });
    const freeName = uniq('Свободный товар');
    const free = await adminApi.createProduct({ name: freeName });
    const archivedName = uniq('Товар для архива');
    await adminApi.createProduct({ name: archivedName });
    const svc = await adminApi.createService({ category_id: svcCat.id, name: uniq('Услуга с комплектом'), products: [{ product_id: used.id, qty: 1 }] });
    // серверная форма удаления без причины отвечает 422 на саму навигацию.
    allowHttpErrors((url, status) => url.endsWith(`/catalog/products/${free.id}/delete`) && status === 422);

    // pagination DS-005: список постраничный — своя запись только через ?q=.
    await catalog.gotoProductsByName(freeName);
    await catalog.open(freeName);
    await catalog.openDelete();
    await expect(catalog.deleteDialogTitle).toHaveText(`Удалить «${freeName}»?`);
    await expect(catalog.deleteDialogText).toHaveText(
      'Запись можно восстановить в разделе «Удалённые». Чтобы только скрыть её из выбора, используйте «В архив»',
    );
    // негатив: причина из пробелов — окно остаётся с ошибкой под полем, товар не удалён.
    await catalog.deleteReason.fill('   ');
    await catalog.deleteDialogOk.click();
    await expect(catalog.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(catalog.deleteDialog).toBeVisible();
    expect((await adminApi.findCatalogByName('products', freeName))?.id, 'товар не удалён').toBe(free.id);

    await catalog.deleteReason.fill(uniq('CT-005 создан по ошибке'));
    await catalog.deleteDialogOk.click();
    await expect(page).toHaveURL(/\/catalog\/products\?flash=deleted$/);
    await expect(catalog.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
    await catalog.gotoProductsByName(freeName);
    await expect(catalog.row(freeName)).toHaveCount(0);
    // и в списке с архивом удалённого нет — удаление и архив независимы.
    await catalog.gotoProducts(`?archived=1&q=${encodeURIComponent(freeName)}`);
    await expect(catalog.row(freeName)).toHaveCount(0);

    // товар из комплекта услуги — тоже удаляется: отказа «используется» больше нет.
    await catalog.gotoProductsByName(usedName);
    await catalog.open(usedName);
    await catalog.confirmDelete();
    await expect(page).toHaveURL(/\/catalog\/products\?flash=deleted$/);
    await expect(catalog.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
    await catalog.gotoProductsByName(usedName);
    await expect(catalog.row(usedName)).toHaveCount(0);
    // строка состава услуги осталась — с бейджем «удалён»; в новой строке удалённый не предлагается.
    await page.goto(`/catalog/services/${svc.id}`);
    await expect(catalog.serviceForm).toBeVisible();
    await expect(catalog.productDeletedBadge(0)).toHaveText('удалён');
    await catalog.serviceProductAdd.click();
    await catalog.productRow(1).getByTestId('service-product-select').click();
    await expect(page.getByTestId('combobox-dropdown').getByText(usedName)).toHaveCount(0);
    await page.keyboard.press('Escape');

    // «В архив» — отдельное действие: архивный товар виден в справочнике с бейджем, но в
    // комплекте новой услуги не предлагается.
    await catalog.gotoProductsByName(archivedName);
    await catalog.open(archivedName);
    await catalog.archiveButton.click();
    await expect(page).toHaveURL(/flash=archived/);
    await catalog.gotoProducts(`?archived=1&q=${encodeURIComponent(archivedName)}`);
    await expect(catalog.archivedBadge(archivedName)).toHaveText('в архиве');
    await catalog.gotoNewService();
    await catalog.productRow(0).getByTestId('service-product-select').click();
    await expect(page.getByTestId('combobox-dropdown').getByText(archivedName)).toHaveCount(0);
    await page.keyboard.press('Escape');
  });
});

test.describe('CT: поиск и архив переживают reload', () => {
  test('CT-006 (товары): ?q= и «Показать архивные» сохраняются при reload', async ({ adminPage: page, adminApi, uniq }) => {
    const catalog = new CatalogPage(page);
    const name = uniq('Товар для поиска');
    await adminApi.createProduct({ name });

    await catalog.gotoProducts();
    await catalog.search.fill(name);
    await catalog.search.press('Enter');
    // GET-форма кодирует пробел как `+`, а не `%20` — сравниваем декодированное значение
    // параметра, а не сырую строку URL (web-first: expect.poll ждёт навигацию формы).
    await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe(name);
    await expect(catalog.row(name)).toBeVisible();

    await page.reload();
    await expect(catalog.search).toHaveValue(name);
    await expect(catalog.row(name)).toBeVisible();

    await catalog.rowToggle(name).click();
    // архивирование редиректит на список БЕЗ q (флаг ошибки/успеха в query, не фильтр) —
    // на разросшемся стенде (§TD-6.2) архивный список без фильтра может быть обрезан
    // лимитом, поэтому фильтр переустанавливаем явно перед переключением «архивные».
    await catalog.gotoProducts(`?q=${encodeURIComponent(name)}`);
    await catalog.archivedFilter.check();
    await expect(page).toHaveURL(/archived=1/);
    await expect(catalog.row(name)).toBeVisible();

    await page.reload();
    await expect(catalog.archivedFilter).toBeChecked();
    await expect(catalog.row(name)).toBeVisible();
  });

  test('CT-006 (услуги): ?q= и «Показать архивные» сохраняются при reload', async ({ adminPage: page, adminApi, uniq }) => {
    const catalog = new CatalogPage(page);
    const catName = uniq('Категория поиска услуг');
    const cat = await adminApi.createCategory({ kind: 'service', name: catName });
    const name = uniq('Услуга для поиска');
    await adminApi.createService({ category_id: cat.id, name });

    await catalog.gotoServices();
    await catalog.search.fill(name);
    await catalog.search.press('Enter');
    await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe(name);
    await expect(catalog.serviceRow(name)).toBeVisible();

    await page.reload();
    await expect(catalog.search).toHaveValue(name);
    await expect(catalog.serviceRow(name)).toBeVisible();

    await catalog.serviceRowToggle(name).click();
    // архивирование редиректит на список БЕЗ q — см. комментарий в CT-006 (товары) выше.
    await catalog.gotoServices(`?q=${encodeURIComponent(name)}`);
    await catalog.archivedFilter.check();
    await expect(page).toHaveURL(/archived=1/);
    await expect(catalog.serviceRow(name)).toBeVisible();

    await page.reload();
    await expect(catalog.archivedFilter).toBeChecked();
    await expect(catalog.serviceRow(name)).toBeVisible();
  });
});

test.describe('CT: права', () => {
  test('CT-007: исполнитель не видит «Каталог» в меню, /catalog/products → «Нет доступа»', async ({ page, makeUser, loginAs, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/catalog/products') && status === 403);
    const login = uniq('exec-catalog');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);

    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.navLink('Каталог')).toHaveCount(0);

    await page.goto('/catalog/products');
    await expect(page.getByTestId('status-page')).toContainText('Нет доступа');
  });
});

// Phase 9: хардение — adaptive:min.
test.describe('CT: адаптив и тема', () => {
  test('CT-008: 360×1280 без горизонтального overflow (товары и услуги), тёмная тема', async ({ adminPage: page, adminApi, uniq }) => {
    const product = await adminApi.createProduct({ name: uniq('Товар адаптива'), sku: 'ADPT-1' });
    const svcCatName = uniq('Категория адаптива услуг');
    const svcCat = await adminApi.createCategory({ kind: 'service', name: svcCatName });
    const svc = await adminApi.createService({
      category_id: svcCat.id,
      name: uniq('Услуга адаптива'),
      products: [{ product_id: product.id, qty: 1 }],
    });

    const catalog = new CatalogPage(page);
    const shell = new ShellPage(page);
    for (const [width, height] of [
      [360, 640],
      [1280, 800],
    ] as const) {
      await page.setViewportSize({ width, height });
      await catalog.gotoProducts();
      expect(await shell.hasHorizontalOverflow()).toBe(false);

      await catalog.gotoServices();
      expect(await shell.hasHorizontalOverflow()).toBe(false);

      await page.goto(`/catalog/services/${svc.id}`);
      await expect(catalog.serviceForm).toBeVisible();
      expect(await shell.hasHorizontalOverflow()).toBe(false);
    }

    await shell.themeToggle.click();
    await shell.themeToggle.click();
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');
    expect(await shell.hasHorizontalOverflow()).toBe(false);
  });
});
