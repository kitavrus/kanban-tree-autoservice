import { test, expect } from '../fixtures/test';
import { LoginPage } from '../pages/LoginPage';
import { UsersPage } from '../pages/UsersPage';
import { columnByName } from '../fixtures/board';

// Фаза 2: экран «Сотрудники» (admin) — создание, правка, блокировка, фильтры, D29, D28.
test.use({ colorScheme: 'light' });

test.describe('US: создание и правка', () => {
  test('US-001: админ создаёт исполнителя через форму, тот входит', async ({ adminPage: page, uniq, browser }) => {
    const login = uniq('volkov');
    const users = new UsersPage(page);
    await users.goto();
    await users.addButton.click();
    await expect(page.locator('.app-title')).toHaveText('Новый сотрудник');
    await expect(users.role).toHaveText('Исполнитель', { useInnerText: true });
    await expect(users.active).toBeChecked();

    await users.fill({ name: 'Волков Олег 🔧', login: `  ${login.toUpperCase()} `, phone: '+7 (900) 123-45-67', password: 'password1' });
    await users.save.click();

    await expect(page).toHaveURL(/\/users\?flash=created$/);
    await expect(users.flash).toHaveText('Сотрудник создан');
    // Список без фильтра показывает не больше 200 строк — на наполненной БД ищем свою
    // запись через ?q (эталон — US-004).
    await users.goto(`?q=${login}`);
    await expect(users.row(login)).toContainText('Волков Олег 🔧');
    await expect(users.row(login)).toContainText('Исполнитель');
    await expect(users.row(login)).toContainText('+7 (900) 123-45-67');

    const ctx = await browser.newContext();
    const other = new LoginPage(await ctx.newPage());
    await other.goto();
    await other.signIn(login, 'password1');
    await expect(other.page.getByTestId('sidebar')).toContainText('Волков Олег 🔧');
    await ctx.close();
  });

  test('US-002: валидация каждого поля — ошибки под полями, введённое сохраняется', async ({ adminPage: page, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/users') && status === 422);
    const users = new UsersPage(page);
    await users.gotoNew();

    await users.save.click();
    await expect(users.fieldError('name')).toHaveText('Обязательное поле');
    await expect(users.fieldError('login')).toHaveText('Обязательное поле');
    await expect(users.fieldError('password')).toHaveText('Обязательное поле');
    await expect(users.name).toHaveClass(/is-invalid/);
    await expect(users.fieldError('phone')).toHaveCount(0);

    await users.fill({ name: 'Имя', login: 'ab', phone: '8-900-abc', password: '1234567' });
    await users.save.click();
    await expect(users.fieldError('name')).toHaveCount(0);
    await expect(users.fieldError('login')).toHaveText('Минимум 3 символа');
    await expect(users.fieldError('phone')).toHaveText('Только цифры, пробелы и символы + ( ) -');
    await expect(users.fieldError('password')).toHaveText('Минимум 8 символов');
    await expect(users.name).toHaveValue('Имя');
    await expect(users.phone).toHaveValue('8-900-abc');
    await expect(users.password).toHaveValue(''); // пароль не возвращается в форму

    await users.fill({ login: 'сидоров', phone: '', password: 'п'.repeat(37) }); // 74 байта > предела bcrypt
    await users.save.click();
    await expect(users.fieldError('login')).toHaveText('Только латинские буквы, цифры, точка, дефис и подчёркивание');
    await expect(users.fieldError('password')).toHaveText('Слишком длинный пароль: до 72 латинских символов (кириллица — до 36)');

    await users.fill({ name: 'Я'.repeat(200), login: 'a'.repeat(50) });
    await expect(users.name).toHaveValue('Я'.repeat(200));
    await expect(users.login).toHaveAttribute('maxlength', '50');
  });

  test('US-003: занятый логин (без учёта регистра) — ошибка под полем', async ({ adminPage: page, makeUser, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/users') && status === 409);
    const taken = await makeUser({ login: uniq('taken') });
    const users = new UsersPage(page);
    await users.gotoNew();
    await users.fill({ name: 'Дубль', login: taken.login.toUpperCase(), password: 'password1' });
    await users.save.click();
    await expect(users.fieldError('login')).toHaveText('Логин уже занят');
    await expect(users.name).toHaveValue('Дубль');
  });

  test('US-004: правка — роль, телефон, ФИО; пустой пароль не меняет старый', async ({ adminPage: page, makeUser, uniq, browser }) => {
    const u = await makeUser({ login: uniq('edit'), phone: '111' });
    const users = new UsersPage(page);
    await users.goto(`?q=${u.login}`);
    await users.open(u.login);
    await expect(users.password).toHaveValue('');
    await expect(page.getByText('Новый пароль (пусто — не менять)')).toBeVisible();

    await users.fill({ name: 'Новое Имя', phone: '' });
    await users.chooseRole('Администратор');
    await users.save.click();
    await expect(users.flash).toHaveText('Изменения сохранены');
    // Список без фильтра показывает не больше 200 строк — на наполненной БД ищем свою через ?q.
    await users.goto(`?q=${u.login}`);
    await expect(users.row(u.login)).toContainText('Новое Имя');
    await expect(users.row(u.login)).toContainText('Администратор');

    const ctx = await browser.newContext();
    const other = new LoginPage(await ctx.newPage());
    await other.goto();
    await other.signIn(u.login, u.password);
    await expect(other.page.getByTestId('sidebar')).toContainText('Администратор');
    await ctx.close();
  });

  test('US-005: смена пароля — новый работает, старый нет', async ({ adminPage: page, makeUser, uniq, browser }) => {
    const u = await makeUser({ login: uniq('pwd') });
    const users = new UsersPage(page);
    await users.goto(`?q=${u.login}`);
    await users.open(u.login);
    await users.fill({ password: 'brand-new-pass' });
    await users.save.click();
    await expect(users.flash).toBeVisible();

    const ctx = await browser.newContext();
    const other = new LoginPage(await ctx.newPage());
    await other.goto();
    await other.signIn(u.login, u.password);
    await expect(other.error).toHaveText('Неверный логин или пароль');
    await other.signIn(u.login, 'brand-new-pass');
    await expect(other.page).toHaveURL(/\/$/);
    await ctx.close();
  });
});

test.describe('US: блокировка, D29, фильтры', () => {
  test('US-010: блокировка — метка в списке, фильтр «Заблокированные», разблокировка', async ({ adminPage: page, makeUser, uniq }) => {
    const u = await makeUser({ login: uniq('block') });
    const users = new UsersPage(page);
    await users.goto(`?q=${u.login}`);
    await expect(users.row(u.login).getByTestId('user-blocked')).toHaveCount(0);
    await users.open(u.login);
    await users.active.uncheck();
    await users.save.click();
    await expect(users.flash).toBeVisible();

    await users.goto(`?q=${u.login}&status=blocked`);
    await expect(users.row(u.login).getByTestId('user-blocked')).toHaveText('заблокирован');
    await users.goto(`?q=${u.login}&status=active`);
    await expect(users.empty).toHaveText('Никого не нашли по фильтрам.');

    await users.goto(`?q=${u.login}`);
    await users.open(u.login);
    await users.active.check();
    await users.save.click();
    await expect(users.row(u.login).getByTestId('user-blocked')).toHaveCount(0);
  });

  test('US-011: себя не заблокировать (D29)', async ({ adminPage: page, runAdmin, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.includes('/users/') && status === 409);
    const users = new UsersPage(page);
    await users.goto(`?q=${runAdmin.login}`);
    await users.open(runAdmin.login);
    await users.active.uncheck();
    await users.save.click();
    await expect(users.formError).toHaveText('Нельзя заблокировать самого себя');
    await expect(users.active).not.toBeChecked(); // введённое сохраняется
  });

  test('US-012: поиск и фильтры живут в URL — reload и «назад» сохраняют', async ({ adminPage: page, makeUser, uniq }) => {
    const tag = uniq('tag'); // общая метка в ФИО обоих — поиск находит обоих
    const a = await makeUser({ login: uniq('flt-exec'), name: `Фильтр ${tag} Исп` });
    const b = await makeUser({ login: uniq('flt-admin'), name: `Фильтр ${tag} Адм`, role: 'admin' });
    const users = new UsersPage(page);
    await users.goto();

    await users.search.fill(tag);
    await users.search.press('Enter');
    await expect(users.row(a.login)).toBeVisible();
    await expect(users.row(b.login)).toBeVisible();

    await users.roleFilter.click();
    await page.getByTestId('combobox-dropdown').locator('.cb-opt', { hasText: 'Администратор' }).click();
    await expect(page).toHaveURL(/role=admin/);
    await expect(users.row(b.login)).toBeVisible();
    await expect(users.row(a.login)).toHaveCount(0);

    await page.reload();
    await expect(users.search).toHaveValue(tag);
    await expect(users.roleFilter).toHaveText('Администратор', { useInnerText: true });
    await expect(users.row(a.login)).toHaveCount(0);

    await page.goBack();
    await expect(users.row(a.login)).toBeVisible();

    await page.getByTestId('filter-reset').click();
    await expect(page).toHaveURL(/\/users$/);
  });

  test('US-013: несуществующий сотрудник — страница 404', async ({ adminPage: page, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.includes('/users/') && status === 404);
    await page.goto('/users/00000000-0000-0000-0000-000000000000');
    await expect(page.getByTestId('status-page')).toContainText('Не найдено');
    await page.goto('/users/not-a-uuid');
    await expect(page.getByTestId('status-page')).toContainText('404');
  });
});

test.describe('US: конкурентность и вёрстка', () => {
  test('US-020: два админа правят одну карточку — сохраняется последняя правка (D28)', async ({ adminPage: page, makeUser, uniq, browser, runAdmin, loginAs }) => {
    const u = await makeUser({ login: uniq('lww') });
    const ctx2 = await browser.newContext({ colorScheme: 'light' });
    const page2 = await ctx2.newPage();
    await loginAs(page2, runAdmin.login, runAdmin.password);

    const one = new UsersPage(page);
    const two = new UsersPage(page2);
    await one.goto(`?q=${u.login}`);
    await one.open(u.login);
    await two.goto(`?q=${u.login}`);
    await two.open(u.login);

    await one.fill({ name: 'Правка Первого' });
    await two.fill({ name: 'Правка Второго', phone: '222' });
    await one.save.click();
    await expect(one.flash).toBeVisible();
    await two.save.click();
    await expect(two.flash).toBeVisible();

    await one.goto(`?q=${u.login}`);
    await expect(one.row(u.login)).toContainText('Правка Второго');
    await expect(one.row(u.login)).toContainText('222');
    await ctx2.close();
  });

  test('US-021: 360px и 1280px — список и форма без горизонтального переполнения', async ({ adminPage: page, makeUser, uniq }) => {
    const u = await makeUser({ login: uniq('wide-' + 'x'.repeat(12)), name: 'Очень-Длинная-Фамилия-Без-Пробелов Иван' });
    const users = new UsersPage(page);
    for (const width of [360, 1280]) {
      await page.setViewportSize({ width, height: 800 });
      await users.goto(`?q=${u.login}`);
      await expect(users.row(u.login)).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
      await users.gotoNew();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
    }
  });
});

test.describe('US: нагрузка сотрудника', () => {
  test('US-014: «Активных заказов» — заказы вне финальной колонки, где сотрудник ответственный', async ({ adminPage: page, adminApi, makeUser, uniq }) => {
    const tag = uniq('load'); // общая метка в ФИО — поиск находит обоих
    const busy = await makeUser({ login: uniq('load-busy'), name: `Нагрузка ${tag} Занят` });
    const idle = await makeUser({ login: uniq('load-idle'), name: `Нагрузка ${tag} Свободен` });
    const client = await adminApi.createClient({ company_name: uniq('US-014 клиент'), phones: [{ phone: '9990000014', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const open = columnByName(columns, 'Новые');
    const final = columns.find((c) => c.is_final);
    expect(final, 'нужна финальная колонка сида D32').toBeTruthy();
    await adminApi.createTask({ title: uniq('US-014 первый'), client_id: client.id, column_id: open.id, assignee_id: busy.id });
    await adminApi.createTask({ title: uniq('US-014 второй'), client_id: client.id, column_id: open.id, assignee_id: busy.id });
    await adminApi.createTask({ title: uniq('US-014 готов'), client_id: client.id, column_id: final!.id, assignee_id: busy.id });

    const users = new UsersPage(page);
    await users.goto(`?q=${tag}`);
    await expect(users.table.locator('thead')).toContainText('Активных заказов');
    await expect(users.row(busy.login).getByTestId('user-active-tasks')).toHaveText('2');
    await expect(users.row(idle.login).getByTestId('user-active-tasks')).toHaveText('0');
  });
});
