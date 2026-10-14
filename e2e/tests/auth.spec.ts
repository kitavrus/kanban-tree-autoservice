import { test, expect } from '../fixtures/test';
import { LoginPage } from '../pages/LoginPage';
import { ShellPage } from '../pages/ShellPage';

// Фаза 2 (plan.md §2): вход/выход, гарды, роли в меню, убийство сессий при блокировке.
test.use({ colorScheme: 'light' });

test.describe('AU: вход и выход', () => {
  test('AU-001: без сессии — на /login с возвратом на исходную страницу', async ({ page, makeUser, uniq }) => {
    const u = await makeUser({ login: uniq('back'), role: 'admin' });
    await page.goto('/users?q=x');
    await expect(page).toHaveURL(/\/login\?redirect=%2Fusers%3Fq%3Dx$/);
    const login = new LoginPage(page);
    await login.signIn(u.login, u.password);
    await expect(page).toHaveURL(/\/users\?q=x$/);
    await expect(page.getByTestId('filter-q')).toHaveValue('x');
  });

  test('AU-002: неверный пароль и пустые поля — ошибка, логин остаётся в форме', async ({ page, makeUser, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/login') && (status === 401 || status === 400));
    const u = await makeUser({ login: uniq('wrong') });
    const login = new LoginPage(page);
    await login.goto();

    await login.submit.click();
    await expect(login.error).toHaveText('Введите логин и пароль');

    await login.signIn(u.login, 'not-the-password');
    await expect(login.error).toHaveText('Неверный логин или пароль');
    await expect(login.login).toHaveValue(u.login);
    await expect(login.password).toHaveValue('');
    await expect(login.password).toBeFocused();

    await login.signIn('nobody-' + u.login, 'whatever-pass');
    await expect(login.error).toHaveText('Неверный логин или пароль');
  });

  test('AU-003: логин без учёта регистра и пробелов', async ({ page, makeUser, uniq }) => {
    const u = await makeUser({ login: uniq('case') });
    const login = new LoginPage(page);
    await login.goto();
    await login.signIn(`  ${u.login.toUpperCase()} `, u.password);
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByTestId('sidebar')).toContainText(u.name);
  });

  test('AU-004: заблокированный видит причину только при верном пароле', async ({ page, makeUser, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/login') && (status === 403 || status === 401));
    const u = await makeUser({ login: uniq('blocked'), is_active: false });
    const login = new LoginPage(page);
    await login.goto();
    await login.signIn(u.login, 'wrong-password');
    await expect(login.error).toHaveText('Неверный логин или пароль');
    await login.signIn(u.login, u.password);
    await expect(login.error).toContainText('заблокирована');
  });

  test('AU-005: выход — на /login, назад по истории на доску не пускает', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');
    await page.getByRole('button', { name: 'Выйти' }).click();
    await expect(page).toHaveURL(/\/login$/);
    await page.goBack();
    await page.reload();
    await expect(page).toHaveURL(/\/login\?redirect=%2F$/);
  });

  test('AU-006: уже вошедшего /login отправляет дальше', async ({ adminPage: page }) => {
    await page.goto('/login?redirect=%2Fusers');
    await expect(page).toHaveURL(/\/users$/);
    await page.goto('/login?redirect=https%3A%2F%2Fevil.example%2F');
    await expect(page).toHaveURL(/(localhost|127\.0\.0\.1):\d+\/$/);
  });
});

test.describe('AU: роли и сессии', () => {
  test('AU-010: исполнитель не видит «Сотрудники», прямой заход — «Нет доступа»', async ({ page, makeUser, uniq, loginAs, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/users') && status === 403);
    const u = await makeUser({ login: uniq('exec') });
    await loginAs(page, u.login, u.password);
    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.navLink('Доска')).toBeVisible();
    await expect(shell.navLink('Сотрудники')).toHaveCount(0);
    await expect(shell.sidebar).toContainText('Исполнитель');

    await page.goto('/users');
    await expect(page.getByTestId('status-page')).toContainText('Нет доступа');
    await expect(page.getByTestId('status-page')).toContainText('403');
  });

  test('AU-011: блокировка выкидывает сотрудника из открытой вкладки', async ({ browser, makeUser, uniq, adminApi }) => {
    const u = await makeUser({ login: uniq('kick') });
    const ctx = await browser.newContext({ colorScheme: 'light' });
    const exec = await ctx.newPage();
    const login = new LoginPage(exec);
    await login.goto();
    await login.signIn(u.login, u.password);
    await expect(exec.getByTestId('sidebar')).toContainText(u.name);

    await adminApi.updateUser(u.id, { login: u.login, name: u.name, role: u.role, phone: u.phone, is_active: false });

    await exec.reload();
    await expect(exec).toHaveURL(/\/login\?redirect=%2F$/);
    await login.signIn(u.login, u.password);
    await expect(login.error).toContainText('заблокирована');
    await ctx.close();
  });

  test('AU-012: смена роли действует без перелогина', async ({ page, makeUser, uniq, loginAs, adminApi }) => {
    const u = await makeUser({ login: uniq('promote') });
    await loginAs(page, u.login, u.password);
    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.navLink('Сотрудники')).toHaveCount(0);

    await adminApi.updateUser(u.id, { login: u.login, name: u.name, role: 'admin', phone: u.phone, is_active: true });
    await page.reload();
    await expect(shell.navLink('Сотрудники')).toBeVisible();
    await expect(shell.sidebar).toContainText('Администратор');
  });
});

test.describe('AU: вёрстка входа (adaptive:min)', () => {
  test('AU-020: 360px и 1280px — без горизонтального переполнения, тема переключается', async ({ page }) => {
    const login = new LoginPage(page);
    for (const width of [360, 1280]) {
      await page.setViewportSize({ width, height: 700 });
      await login.goto();
      await expect(login.form).toBeInViewport();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
    }
    await page.evaluate(() => localStorage.removeItem('kt-theme'));
    await page.reload();
    await page.getByTestId('theme-toggle').click();
    await page.getByTestId('theme-toggle').click();
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');
  });
});
