import { test, expect } from '../fixtures/test';
import { ShellPage } from '../pages/ShellPage';
import { SuppliersPage } from '../pages/SuppliersPage';
import { DeletedPage } from '../pages/DeletedPage';
import { loginContext, type AdminApi, type NewSupplier, type Supplier } from '../fixtures/auth';
import type { Page } from '@playwright/test';

// Справочник «Поставщики» (docs/features/suppliers/draft-spec.md §8.1, DS-007/DS-043) —
// TS-035: admin заводит, правит, архивирует и возвращает поставщика; дубль названия — под
// полем; executor — без пункта меню и с 403; вьюпорт 375×812 без горизонтальной прокрутки.
// Общая dev-БД (§TD-6.2): свои поставщики через uniq(). Уборка (soft-delete-warehouses-suppliers
// TS-F11): созданные тестом записи убираются ФИЗИЧЕСКИ служебным адресом purge — purgeAll по
// префиксу (в том числе заведённые через экран) и autouse partnersCleanup (созданные через
// adminApi и мягко удалённые тестом); раньше они только архивировались и копились.
test.use({ colorScheme: 'light' });

const THEMES = ['light', 'dark'] as const;

function csrfToken(page: Page): Promise<string> {
  return page.locator('meta[name="csrf-token"]').getAttribute('content').then((v) => v ?? '');
}

/** Уборка поставщиков теста по префиксу названия — действующих и архивных (удалённые в список не
 *  входят: их тест регистрирует в partnersCleanup). Статус уборки читается: 200 — убрано, 404 —
 *  записи уже нет; 409 «используется» и любой другой статус — console.warn, молча не глотается. */
async function purgeAll(adminApi: AdminApi, prefix: string) {
  const list = await adminApi.listSuppliers({ q: prefix });
  for (const s of list.items) {
    const status = await adminApi.purge('suppliers', s.id);
    if (status !== 200 && status !== 404) console.warn(`suppliers.spec: уборка поставщика ${s.id} «${s.name}» — статус ${status}`);
  }
}

async function expectNoPageOverflow(sup: SuppliersPage, where: string) {
  await expect
    .poll(() => sup.hasHorizontalOverflow(), { message: `горизонтальный overflow страницы: ${where}` })
    .toBeLessThanOrEqual(0);
}

test.describe('SUP: справочник «Поставщики»', () => {
  test('SUP-001 (TS-035, TS-045): admin — меню «Поставщики», добавить, открыть, в архив, показать архивные, вернуть из архива, удалить с причиной, восстановить из «Удалённых»', async ({
    adminPage: page,
    adminApi,
    uniq,
    partnersCleanup,
  }) => {
    const prefix = uniq('sup-001');
    const data: Required<NewSupplier> = {
      name: `${prefix} ооо картридж`,
      contact_name: 'Иван Петров',
      phone: '+7 700 123-45-67',
      address: 'Алматы, пр. Абая 1',
      note: 'оптовые поставки',
    };
    const sup = new SuppliersPage(page);
    const shell = new ShellPage(page);
    try {
      // «Справочники → Поставщики».
      await shell.goto('/');
      await shell.navLink('Поставщики').click();
      await expect(page).toHaveURL(/\/suppliers$/);
      await expect(shell.title).toHaveText('Поставщики');
      await expect(sup.root).toBeVisible();

      // «Добавить» → заполнить → «Сохранить».
      await sup.openNew();
      await expect(page).toHaveURL(/\/suppliers\?new=1$/);
      await sup.fill(data);
      await sup.save.click();
      await sup.expectSaved();
      await expect(page).toHaveURL(/\/suppliers$/);

      // строка в таблице со всеми колонками, без бейджа архива (поиск — чтобы не зависеть
      // от числа чужих поставщиков на общем стенде: страница списка — 50 строк).
      await sup.searchFor(prefix);
      const row = sup.row(data.name);
      await expect(row).toBeVisible();
      await expect(row).toContainText(data.contact_name);
      await expect(row).toContainText(data.phone);
      await expect(row).toContainText(data.address);
      await expect(sup.archivedBadge(data.name)).toHaveCount(0);
      const created = await adminApi.supplierByName(data.name);
      expect(created).toMatchObject({ ...data, is_active: true } satisfies Partial<Supplier>);
      partnersCleanup('suppliers', created.id); // заведён через экран; ниже — мягко удаляется

      // открыть — форма с сохранёнными значениями, адрес ?id=<uuid>.
      await sup.openRow(data.name);
      await expect(page).toHaveURL(new RegExp(`/suppliers\\?id=${created.id}$`));
      await expect(sup.contactName).toHaveValue(data.contact_name);
      await expect(sup.phone).toHaveValue(data.phone);
      await expect(sup.address).toHaveValue(data.address);
      await expect(sup.note).toHaveValue(data.note);
      await expect(sup.toggleActive).toHaveText('В архив');

      // «В архив»: из списка по умолчанию (только действующие) строка пропадает.
      await sup.toggleActive.click();
      await sup.expectSaved();
      await expect(sup.row(data.name)).toHaveCount(0);
      expect((await adminApi.supplierByName(data.name)).is_active).toBe(false);

      // «Показать архивные» — строка с бейджем «в архиве».
      await sup.showArchived(true);
      await expect(sup.archivedBadge(data.name)).toBeVisible();
      await expect(sup.archivedBadge(data.name)).toHaveText('в архиве');

      // «Вернуть из архива» — бейдж исчез, поля не потеряны.
      await sup.openRow(data.name);
      await expect(sup.archivedNote).toBeVisible();
      await expect(sup.toggleActive).toHaveText('Вернуть из архива');
      await sup.toggleActive.click();
      await sup.expectSaved();
      await expect(sup.row(data.name)).toBeVisible();
      await expect(sup.archivedBadge(data.name)).toHaveCount(0);
      expect(await adminApi.supplierByName(data.name)).toMatchObject({ ...data, is_active: true } satisfies Partial<Supplier>);

      // после выключения toggle действующий поставщик остаётся в списке.
      await sup.showArchived(false);
      await expect(sup.row(data.name)).toBeVisible();

      // «Удалить» с причиной (soft-delete-warehouses-suppliers §8.1): окно «Удаление», тост,
      // поставщика нет в списке — и среди архивных.
      await sup.openRow(data.name);
      const reason = `${prefix} создан по ошибке`;
      await sup.confirmDelete(reason);
      await expect(sup.toast.last()).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
      await expect(sup.form).toBeHidden();
      await expect(sup.row(data.name)).toHaveCount(0);
      await sup.showArchived(true);
      await expect(sup.row(data.name)).toHaveCount(0);
      expect((await adminApi.listSuppliers({ q: data.name })).total, 'удалённого поставщика список не отдаёт').toBe(0);

      // «Удалённые» → «Поставщики» → «Восстановить»: поставщик вернулся действующим, поля не потеряны.
      const del = new DeletedPage(page);
      await del.goto(`kind=suppliers&q=${encodeURIComponent(data.name)}`);
      await expect(del.rowTitle(created.id)).toHaveText(data.name);
      await expect(del.rowSubtitle(created.id)).toHaveText(data.contact_name);
      await expect(del.rowReason(created.id)).toHaveText(reason);
      await del.restore(created.id);
      await expect(del.toast).toHaveText('Восстановлено');
      await expect(del.row(created.id)).toHaveCount(0);
      expect(await adminApi.supplierByName(data.name)).toMatchObject({ ...data, id: created.id, is_active: true } satisfies Partial<Supplier>);
    } finally {
      await purgeAll(adminApi, prefix);
    }
  });

  test('SUP-002 (TS-035): негатив — дубль названия под полем «Поставщик с таким названием уже есть»; обязательное название; телефон без цифр', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    // ошибки валидации формы — ожидаемые 400 ровно на то, что шлёт сценарий: POST создания
    // (URL без query) и PUT правки (/:id). GET списка (всегда с ?…) сюда не попадает.
    allowHttpErrors(
      (url, status) =>
        status === 400 && (url.endsWith('/api/v1/suppliers') || /\/api\/v1\/suppliers\/[0-9a-f-]{36}$/.test(url)),
    );
    const prefix = uniq('sup-002');
    const existing = await adminApi.createSupplier({ name: `${prefix} поставщик` });
    const archived = await adminApi.createSupplier({ name: `${prefix} архивный` });
    await adminApi.updateSupplier(archived.id, { name: archived.name, is_active: false });
    const sup = new SuppliersPage(page);
    try {
      await sup.goto();
      await sup.searchFor(prefix);
      await expect(sup.rows).toHaveCount(1);
      await sup.openNew();

      // пустое название → под полем, форма не закрылась, запись не создана.
      await sup.save.click();
      await expect(sup.fieldError('name')).toHaveText('Укажите название (до 200 символов)');
      await expect(sup.name).toHaveClass(/is-invalid/);
      await expect(sup.form).toBeVisible();
      await expect(sup.save).toBeEnabled();

      // название из пробелов — то же (trim на сервере).
      await sup.fill({ name: '    ' });
      await sup.save.click();
      await expect(sup.fieldError('name')).toHaveText('Укажите название (до 200 символов)');

      // дубль: другой регистр + пробелы по краям → под полем «Название».
      await sup.fill({ name: `  ${existing.name.toUpperCase()}  `, contact_name: 'Кто-то', phone: '12345' });
      await sup.save.click();
      await expect(sup.fieldError('name')).toHaveText('Поставщик с таким названием уже есть');
      await expect(sup.name).toHaveClass(/is-invalid/);
      // введённое не потеряно.
      await expect(sup.contactName).toHaveValue('Кто-то');
      await expect(sup.phone).toHaveValue('12345');

      // дубль названия архивного поставщика — тоже ошибка (уникальность вкл. архивные).
      await sup.fill({ name: archived.name });
      await sup.save.click();
      await expect(sup.fieldError('name')).toHaveText('Поставщик с таким названием уже есть');

      // телефон без цифр → под полем «Телефон»; ошибка названия снята.
      await sup.fill({ name: `${prefix} новый`, phone: 'нет телефона' });
      await sup.save.click();
      await expect(sup.fieldError('phone')).toHaveText('Проверьте телефон');
      await expect(sup.phone).toHaveClass(/is-invalid/);
      await expect(sup.name).not.toHaveClass(/is-invalid/);
      expect((await adminApi.listSuppliers({ q: prefix })).total, 'невалидные формы ничего не создали').toBe(2);

      // исправили — сохраняется.
      await sup.fill({ phone: '8 (700) 000-00-00' });
      await sup.save.click();
      await sup.expectSaved();
      await expect(sup.row(`${prefix} новый`)).toBeVisible();

      // правка: переименование в чужое название → дубль под полем; своё название — не дубль.
      await sup.openRow(`${prefix} новый`);
      await sup.fill({ name: existing.name });
      await sup.save.click();
      await expect(sup.fieldError('name')).toHaveText('Поставщик с таким названием уже есть');
      await sup.fill({ name: `${prefix} НОВЫЙ` });
      await sup.save.click();
      await sup.expectSaved();
      await expect(sup.row(`${prefix} НОВЫЙ`)).toBeVisible();

      // «Отмена» закрывает форму без запроса и без изменений.
      await sup.openRow(existing.name);
      await sup.fill({ name: `${prefix} не сохранится` });
      await sup.cancel.click();
      await expect(sup.form).toBeHidden();
      await expect(sup.row(existing.name)).toBeVisible();
      expect((await adminApi.listSuppliers({ q: `${prefix} не сохранится` })).total).toBe(0);
    } finally {
      await purgeAll(adminApi, prefix);
    }
  });

  test('SUP-003 (TS-035): executor — пункта меню «Поставщики» нет, прямой URL /suppliers и API → 403', async ({
    page,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    const login = uniq('exec-sup003');
    const user = await makeUser({ login });
    await loginAs(page, login, user.password);

    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.navLink('Доска')).toBeVisible();
    await expect(shell.navLink('Поставщики')).toHaveCount(0);

    allowHttpErrors((url, status) => (url.includes('/suppliers') || url.includes('/api/v1/suppliers')) && status === 403);
    // прямой URL — страница отказа в доступе, экран справочника не отрисован.
    const resp = await page.goto('/suppliers');
    expect(resp?.status()).toBe(403);
    await expect(page.getByTestId('status-page')).toBeVisible();
    await expect(page.getByTestId('status-page')).toContainText('403');
    await expect(page.getByTestId('suppliers-root')).toHaveCount(0);

    const list = await page.request.get('/api/v1/suppliers', { failOnStatusCode: false });
    expect(list.status()).toBe(403);
    const body: NewSupplier = { name: 'чужой поставщик' };
    const post = await page.request.post('/api/v1/suppliers', {
      data: body,
      headers: { 'X-Csrf-Token': await csrfToken(page) },
      failOnStatusCode: false,
    });
    expect(post.status()).toBe(403);
    const put = await page.request.put('/api/v1/suppliers/00000000-0000-0000-0000-000000000000', {
      data: { ...body, is_active: false },
      headers: { 'X-Csrf-Token': await csrfToken(page) },
      failOnStatusCode: false,
    });
    expect(put.status()).toBe(403);
  });

  test('SUP-004 (TS-035): поиск, toggle архивных, пустой список, правка, адрес формы (?new=1, ?id=) и reload', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const prefix = uniq('sup-004');
    const alpha = await adminApi.createSupplier({ name: `${prefix} альфа`, contact_name: 'Анна', phone: '111', address: 'Алматы' });
    const beta = await adminApi.createSupplier({ name: `${prefix} бета` });
    const gamma = await adminApi.createSupplier({ name: `${prefix} гамма` });
    await adminApi.updateSupplier(gamma.id, { name: gamma.name, is_active: false });
    const sup = new SuppliersPage(page);
    try {
      await sup.goto();

      // поиск по подстроке без учёта регистра: только действующие, по названию.
      await sup.searchFor(prefix.toUpperCase());
      await expect(sup.rows).toHaveCount(2);
      await expect(sup.rows.nth(0)).toHaveAttribute('data-name', alpha.name);
      await expect(sup.rows.nth(1)).toHaveAttribute('data-name', beta.name);

      // + архивные: действующие раньше архивных.
      await sup.showArchived(true);
      await expect(sup.rows).toHaveCount(3);
      await expect(sup.rows.nth(2)).toHaveAttribute('data-name', gamma.name);
      await expect(sup.archivedBadge(gamma.name)).toBeVisible();
      await expect(sup.archivedBadge(alpha.name)).toHaveCount(0);

      // комбинация фильтров — «И».
      await sup.searchFor(`${prefix} гам`);
      await expect(sup.rows).toHaveCount(1);
      await sup.showArchived(false);
      await expect(sup.rows).toHaveCount(0);

      // пусто: «Поставщиков нет» + «Добавить» открывает форму создания.
      await expect(sup.empty).toBeVisible();
      await expect(sup.empty).toContainText('Поставщиков нет');
      await sup.emptyAdd.click();
      await expect(sup.form).toBeVisible();
      await expect(sup.formTitle).toHaveText('Новый поставщик');
      await expect(page).toHaveURL(/[?&]new=1$/);
      await sup.cancel.click();
      await expect(sup.form).toBeHidden();

      // правка: все поля сохраняются, строка обновлена.
      await sup.searchFor(prefix);
      await sup.openRow(alpha.name);
      const edited: Required<NewSupplier> = {
        name: `${prefix} альфа плюс`,
        contact_name: 'Борис',
        phone: '222-333',
        address: 'Астана, Кабанбай батыра 5',
        note: 'звонить после 10',
      };
      await sup.fill(edited);
      await sup.save.click();
      await sup.expectSaved();
      await expect(sup.row(edited.name)).toContainText(edited.contact_name);
      await expect(sup.row(edited.name)).toContainText(edited.phone);
      await expect(sup.row(alpha.name)).toHaveCount(0);
      expect(await adminApi.supplierByName(edited.name)).toMatchObject({ ...edited, id: alpha.id, is_active: true } satisfies Partial<Supplier>);

      // адрес формы правки переживает reload: список с сервера, форма с сохранёнными данными.
      await sup.openRow(edited.name);
      await expect(page).toHaveURL(new RegExp(`[?&]id=${alpha.id}$`));
      await page.reload();
      await expect(sup.form).toBeVisible();
      await expect(sup.name).toHaveValue(edited.name);
      await expect(sup.note).toHaveValue(edited.note);

      // прямая ссылка на архивного (нет в списке по умолчанию) — форма открывается.
      await sup.goto(`?id=${gamma.id}`);
      await expect(sup.form).toBeVisible();
      await expect(sup.name).toHaveValue(gamma.name);
      await expect(sup.toggleActive).toHaveText('Вернуть из архива');

      // ?new=1 — пустая форма создания.
      await sup.goto('?new=1');
      await expect(sup.form).toBeVisible();
      await expect(sup.name).toHaveValue('');
      await expect(sup.toggleActive).toBeHidden();

      // несуществующий id в адресе — тост, форма закрыта, адрес очищен.
      await sup.goto('?id=00000000-0000-0000-0000-000000000000');
      await expect(sup.toastError).toHaveText('Поставщик не найден');
      await expect(sup.form).toBeHidden();
      await expect(page).toHaveURL(/\/suppliers$/);
    } finally {
      await purgeAll(adminApi, prefix);
    }
  });

  test('SUP-005 (TS-035): вьюпорт 375×812 — нет горизонтальной прокрутки (список и форма, светлая и тёмная тема)', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    // сценарий сам отправляет пустую форму создания (по разу на тему), чтобы замерить
    // overflow формы с ошибкой под полем: POST /api/v1/suppliers → 400 name: required.
    // Разрешён только этот ответ — URL без query (GET списка всегда с ?…) и без /:id (PUT).
    allowHttpErrors((url, status) => url.endsWith('/api/v1/suppliers') && status === 400);
    const prefix = uniq('sup-005');
    // длинные значения — худший случай для вёрстки таблицы и формы.
    const long = await adminApi.createSupplier({
      name: `${prefix} ${'оченьдлинноеназваниепоставщикабезпробелов'.repeat(3)}`,
      contact_name: 'Константинопольский Александр Владимирович',
      phone: '+7 (700) 123-45-67, +7 (727) 987-65-43 доб. 1234',
      address: 'Республика Казахстан, город Алматы, Бостандыкский район, проспект Абая, дом 150/230, офис 1024',
      note: 'примечание '.repeat(40).trim(),
    });
    await adminApi.createSupplier({ name: `${prefix} короткий` });
    const sup = new SuppliersPage(page);
    await page.setViewportSize({ width: 375, height: 812 });
    try {
      for (const theme of THEMES) {
        // последний зарегистрированный init-скрипт задаёт тему (эталон — polish.spec.ts).
        await page.addInitScript((t) => localStorage.setItem('kt-theme', t), theme);
        const where = `/suppliers ${theme} 375×812`;
        await sup.goto();
        await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
        await sup.searchFor(prefix);
        await expect(sup.rows).toHaveCount(2);
        await expectNoPageOverflow(sup, `${where}: список`);

        await sup.openRow(long.name);
        await expect(sup.save).toBeVisible();
        await expectNoPageOverflow(sup, `${where}: форма правки`);
        await sup.cancel.click();

        await sup.openNew();
        await sup.save.click();
        await expect(sup.fieldError('name')).toHaveText('Укажите название (до 200 символов)');
        await expectNoPageOverflow(sup, `${where}: форма с ошибкой`);
        await sup.cancel.click();

        await sup.searchFor(`${prefix} нет такого`);
        await expect(sup.empty).toBeVisible();
        await expectNoPageOverflow(sup, `${where}: пустой список`);
      }
    } finally {
      await purgeAll(adminApi, prefix);
    }
  });

  test('SUP-006 (TS-035, DS-043): два администратора правят одного поставщика — сохраняется последняя запись', async ({
    adminPage: page,
    adminApi,
    runAdmin,
    browser,
    baseURL,
    uniq,
  }) => {
    const prefix = uniq('sup-006');
    const s = await adminApi.createSupplier({ name: `${prefix} общий`, contact_name: 'Исходный' });
    const other = await browser.newContext({ baseURL, locale: 'ru-RU' });
    try {
      await loginContext(other, runAdmin.login, runAdmin.password);
      const first = new SuppliersPage(page);
      const second = new SuppliersPage(await other.newPage());
      // оба открыли одну и ту же версию.
      await first.goto(`?id=${s.id}`);
      await second.goto(`?id=${s.id}`);
      await expect(first.name).toHaveValue(s.name);
      await expect(second.name).toHaveValue(s.name);

      await first.fill({ name: `${prefix} общий (первый)`, contact_name: 'Первый' });
      await first.save.click();
      await first.expectSaved();

      // второй сохраняет поверх со своей (устаревшей) формой — без ошибки, last-write-wins.
      await second.fill({ contact_name: 'Второй', phone: '777' });
      await second.save.click();
      await second.expectSaved();

      const final = await adminApi.listSuppliers({ q: prefix });
      expect(final.total, 'дубль не появился').toBe(1);
      expect(final.items[0]).toMatchObject({
        id: s.id,
        name: s.name,
        contact_name: 'Второй',
        phone: '777',
        is_active: true,
      } satisfies Partial<Supplier>);

      // первый после reload видит итог с сервера.
      await first.goto();
      await first.searchFor(prefix);
      await expect(first.rows).toHaveCount(1);
      await expect(first.row(s.name)).toContainText('Второй');
    } finally {
      await other.close();
      await purgeAll(adminApi, prefix);
    }
  });
});
