import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import type { AdminApi } from '../fixtures/auth';
import type { Page } from '@playwright/test';

// Фаза 8 (plan.md §6, polish): сводный smoke адаптива (adaptive:min, US-1/§TD-6.6) и тем
// (themes:both) по ВСЕМ SSR-экранам. Для каждого экрана × {360, 1280} × {light, dark}:
//   - страница целиком не скроллится вбок (scrollWidth ≤ clientWidth);
//   - виден основной testid экрана;
//   - тема применена (html[data-bs-theme]) и фон body действительно тёмный/светлый —
//     захардкоженный светлый фон в тёмной теме ловится здесь; акты печати — всегда светлые.
// Смена темы — как CL-021/BD-020: localStorage kt-theme до загрузки страницы (init-скрипты
// выполняются в порядке регистрации — последний зарегистрированный задаёт тему).
// Данные — свои сущности через adminApi (uniq), заказы убирает autouse _boardCleanup.
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;
type Theme = (typeof THEMES)[number];
const VIEWPORTS = [
  { width: 360, height: 800 },
  { width: 1280, height: 800 },
] as const;

async function useTheme(page: Page, theme: Theme) {
  await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
}

/** Тёмный ли фон body (все RGB-компоненты < 60 — эталон OC-017). */
async function bodyIsDark(page: Page): Promise<boolean> {
  const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  const m = bg.match(/\d+/g);
  expect(m, `body background-color: ${bg}`).not.toBeNull();
  return m!.slice(0, 3).every((v) => Number(v) < 60);
}

// Замеры — через expect.poll (ревью фазы 8, LOW-3): при асинхронной вёрстке (догрузка
// данных JS после toBeVisible) одноразовый evaluate мог бы поймать промежуточное состояние.
async function expectNoPageOverflow(page: Page, where: string) {
  await expect
    .poll(
      () => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth),
      { message: `горизонтальный overflow страницы: ${where}` },
    )
    .toBeLessThanOrEqual(0);
}

/** Общая проверка экрана приложения (с сайдбаром/без): тема + overflow + основной testid. */
async function checkScreen(page: Page, theme: Theme, where: string, testid: string) {
  await expect(page.getByTestId(testid).first()).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
  await expect.poll(() => bodyIsDark(page), { message: `фон body в теме ${theme}: ${where}` }).toBe(theme === 'dark');
  await expectNoPageOverflow(page, where);
}

/** Прогон набора путей по матрице тема × вьюпорт. */
async function sweep(page: Page, screens: Array<{ path: string; testid: string }>) {
  for (const theme of THEMES) {
    await useTheme(page, theme);
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      for (const s of screens) {
        await page.goto(s.path);
        await checkScreen(page, theme, `${s.path} @${vp.width} ${theme}`, s.testid);
      }
    }
  }
}

async function newClient(adminApi: AdminApi, uniq: (s: string) => string, label: string) {
  return adminApi.createClient({
    company_name: uniq(`PL клиент ${label}`),
    address: 'г. Москва, ул. Длинная-предлинная, дом 1, строение 2, офис 345',
    contact_name: 'Иванов Иван Иванович',
    phones: [{ phone: '9990000801', note: 'основной' }],
  });
}

/** Заказ с оборудованием и услугой с товаром — карточка с таблицей товаров и журналом. */
async function richTask(adminApi: AdminApi, uniq: (s: string) => string, label: string) {
  const client = await newClient(adminApi, uniq, label);
  const type = await adminApi.createEquipmentType({ name: uniq(`PL тип ${label}`), spec_keys: ['Мощность'] });
  const eq = await adminApi.createEquipment(client.id, {
    type_id: type.id,
    brand: 'Brand',
    model: 'Model-X',
    serial: 'SN-0123456789ABCDEFGHIJ0123456789ABCDEFGHIJ',
  });
  const prodCat = await adminApi.createCategory({ kind: 'product', name: uniq(`PL кат. товаров ${label}`) });
  const product = await adminApi.createProduct({ category_id: prodCat.id, name: uniq(`PL товар ${label}`), price: 150, stock: 10, min_stock: 2 });
  const svcCat = await adminApi.createCategory({ kind: 'service', name: uniq(`PL кат. услуг ${label}`) });
  const svc = await adminApi.createService({
    category_id: svcCat.id,
    name: uniq(`PL услуга ${label}`),
    price: 1000,
    products: [{ product_id: product.id, qty: 2 }],
  });
  const columns = (await adminApi.listColumns()).items;
  const task = await adminApi.createTask({
    title: uniq(`PL заказ ${label} с длинным названием для проверки переноса в шапке карточки`),
    client_id: client.id,
    column_id: columnByName(columns, 'Новые').id,
    description: 'Описание заказа',
  });
  await adminApi.addTaskEquipment(task.id, eq.id);
  await adminApi.createSubtask(task.id, svc.id);
  return { client, type, eq, prodCat, product, svcCat, svc, task };
}

test.describe('PL: адаптив и темы по всем экранам', () => {
  test('PL-001: /login — без overflow, форма видна, обе темы', async ({ page }) => {
    await sweep(page, [{ path: '/login', testid: 'login-form' }]);
  });

  test('PL-002: доска / — страница не скроллится вбок (колонки — внутри себя), обе темы', async ({ adminPage: page, adminApi, uniq }) => {
    const { client } = await richTask(adminApi, uniq, 'PL-002');
    const board = new BoardPage(page);
    for (const theme of THEMES) {
      await useTheme(page, theme);
      for (const vp of VIEWPORTS) {
        await page.setViewportSize(vp);
        await board.goto(`?client_id=${client.id}`);
        await checkScreen(page, theme, `/ @${vp.width} ${theme}`, 'board-root');
        await expect(page.getByTestId('board-filters')).toBeVisible();
      }
    }
  });

  test('PL-003: карточка заказа (модалка) — ни страница, ни тело модалки не скроллятся вбок, обе темы', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    test.setTimeout(90_000);
    const { client, task } = await richTask(adminApi, uniq, 'PL-003');
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    for (const theme of THEMES) {
      await useTheme(page, theme);
      for (const vp of VIEWPORTS) {
        const where = `карточка #${task.id} @${vp.width} ${theme}`;
        await page.setViewportSize(vp);
        await board.goto(`?client_id=${client.id}`);
        await board.openTaskEdit(task.id);
        await card.waitOpen();
        // дерево карточки грузится JS — ждём реального рендера секций.
        await expect(page.getByTestId('card-equipment-row').first()).toBeVisible();
        await expect(page.getByTestId('card-subtask-row').first()).toBeVisible();
        await expect(page.getByTestId('card-print-intake')).toBeVisible();
        await expect(page.getByTestId('task-save')).toBeVisible();
        await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
        await expectNoPageOverflow(page, where);
        for (const sel of ['#task-modal .modal-header', '#task-modal .modal-body']) {
          const o = await page.locator(sel).evaluate((el) => ({ scroll: el.scrollWidth, client: el.clientWidth }));
          expect(o.scroll, `горизонтальный overflow ${sel}: ${where}`).toBeLessThanOrEqual(o.client);
        }
        await card.close();
        await expect(board.taskForm).toBeHidden();
      }
    }
  });

  test('PL-004: акты печати — светлые в любой теме, без overflow на 360/1280', async ({ adminPage: page, adminApi, uniq }) => {
    const { task } = await richTask(adminApi, uniq, 'PL-004');
    for (const theme of THEMES) {
      await useTheme(page, theme);
      for (const vp of VIEWPORTS) {
        await page.setViewportSize(vp);
        for (const kind of ['intake', 'handover'] as const) {
          const where = `/tasks/${task.id}/print/${kind} @${vp.width} ${theme}`;
          await page.goto(`/tasks/${task.id}/print/${kind}`);
          await expect(page.getByTestId('print-title')).toBeVisible();
          await expect(page.getByTestId('print-equipment')).toBeVisible();
          await expect.poll(() => bodyIsDark(page), { message: `акт всегда светлый: ${where}` }).toBe(false);
          await expectNoPageOverflow(page, where);
        }
      }
    }
  });

  test('PL-005: «Остатки» /stock — без overflow, обе темы', async ({ adminPage: page }) => {
    await sweep(page, [{ path: '/stock', testid: 'stock-root' }]);
  });

  test('PL-006: клиенты — список, новый, карточка клиента с оборудованием, обе темы', async ({ adminPage: page, adminApi, uniq }) => {
    test.setTimeout(90_000);
    const { client } = await richTask(adminApi, uniq, 'PL-006');
    await sweep(page, [
      { path: `/clients?q=${encodeURIComponent(client.company_name)}`, testid: 'clients-table' },
      { path: '/clients/new', testid: 'client-form' },
      { path: `/clients/${client.id}`, testid: 'equipment-table' },
    ]);
  });

  test('PL-007: оборудование клиента — новое и редактирование, обе темы', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, eq } = await richTask(adminApi, uniq, 'PL-007');
    await sweep(page, [
      { path: `/clients/${client.id}/equipment/new`, testid: 'equipment-form' },
      { path: `/equipment/${eq.id}`, testid: 'equipment-form' },
    ]);
  });

  test('PL-008: каталог услуг — список, новая, редактирование, обе темы', async ({ adminPage: page, adminApi, uniq }) => {
    test.setTimeout(90_000);
    const { svc } = await richTask(adminApi, uniq, 'PL-008');
    await sweep(page, [
      { path: `/catalog/services?q=${encodeURIComponent(svc.name)}`, testid: 'services-table' },
      { path: '/catalog/services/new', testid: 'service-form' },
      { path: `/catalog/services/${svc.id}`, testid: 'service-form' },
    ]);
  });

  test('PL-009: каталог товаров — список, новый, редактирование, обе темы', async ({ adminPage: page, adminApi, uniq }) => {
    test.setTimeout(90_000);
    const { product } = await richTask(adminApi, uniq, 'PL-009');
    await sweep(page, [
      { path: `/catalog/products?q=${encodeURIComponent(product.name)}`, testid: 'products-table' },
      { path: '/catalog/products/new', testid: 'product-form' },
      { path: `/catalog/products/${product.id}`, testid: 'product-form' },
    ]);
  });

  test('PL-010: типы оборудования — список, новый, редактирование, обе темы', async ({ adminPage: page, adminApi, uniq }) => {
    test.setTimeout(90_000);
    const type = await adminApi.createEquipmentType({ name: uniq('PL-010 тип'), spec_keys: ['Напряжение', 'Мощность'] });
    await sweep(page, [
      { path: `/catalog/equipment-types?q=${encodeURIComponent(type.name)}`, testid: 'types-table' },
      { path: '/catalog/equipment-types/new', testid: 'type-form' },
      { path: `/catalog/equipment-types/${type.id}`, testid: 'type-form' },
    ]);
  });

  test('PL-011: сотрудники — список, новый, редактирование, обе темы', async ({ adminPage: page, makeUser, uniq }) => {
    test.setTimeout(90_000);
    const u = await makeUser({ login: uniq('pl011'), role: 'executor' });
    await sweep(page, [
      { path: `/users?q=${encodeURIComponent(u.login)}`, testid: 'users-table' },
      { path: '/users/new', testid: 'user-form' },
      { path: `/users/${u.id}`, testid: 'user-form' },
    ]);
  });
});
