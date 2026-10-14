import { test, expect } from '../fixtures/test';
import { ShellPage } from '../pages/ShellPage';

// Фаза 1 (plan.md §6), под входом с фазы 2: общий SSR-layout и общие JS-компоненты, перенесённые из
// prototype/index.html (тема, диалоги вместо нативных, выпадающий список с поиском D6).
// Системная тема зафиксирована светлой — порядок переключения детерминирован.
test.use({ colorScheme: 'light' });

test.describe('SH: каркас приложения', () => {
  test('SH-001: главная — layout, меню, заглушка доски', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');

    await expect(page).toHaveTitle('Доска заказов — Kanban-tree');
    await expect(shell.title).toHaveText('Доска заказов');
    await expect(shell.navLink('Доска')).toHaveAttribute('aria-current', 'page');
    // реализованные разделы admin видит, включая «Остатки» (фаза 5); бейдж риск-товаров
    // рендерится всегда (hidden при 0, ADR-004) — конкретное число не проверяем.
    for (const title of ['Доска', 'Остатки', 'Сотрудники']) await expect(shell.navLink(title)).toBeVisible();
    await expect(shell.navLink('Остатки').getByTestId('nav-badge-stock')).toBeAttached();
    await expect(page.getByTestId('board-root')).toBeVisible();
    await expect(page.locator('meta[name="csrf-token"]')).toHaveCount(1);
  });

  test('SH-002: тема system → light → dark, сохраняется после перезагрузки', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');
    await page.evaluate(() => localStorage.removeItem('kt-theme'));
    await page.reload();

    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'light');
    await expect(shell.themeToggle).toHaveAttribute('data-theme-pref', 'system');

    await shell.themeToggle.click();
    await expect(shell.themeToggle).toHaveAttribute('data-theme-pref', 'light');
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'light');

    await shell.themeToggle.click();
    await expect(shell.themeToggle).toHaveAttribute('data-theme-pref', 'dark');
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');

    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');

    await shell.themeToggle.click(); // dark → system (светлая)
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'light');
  });

  test('SH-003: тёмная тема системы подхватывается без выбора пользователя', async ({ adminPage: page }) => {
    await page.emulateMedia({ colorScheme: 'dark' });
    const shell = new ShellPage(page);
    await shell.goto('/');
    await page.evaluate(() => localStorage.removeItem('kt-theme'));
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');
  });
});

test.describe('SH: диалоги вместо нативных prompt/confirm/alert', () => {
  test('SH-010: uiConfirm — OK → true, Отмена → false, Esc → false', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');

    await shell.openDialog('confirm', 'Удалить заказ?');
    await expect(shell.dialog.getByTestId('ui-dialog-text')).toHaveText('Удалить заказ?');
    await expect(shell.dialogOk).toHaveText('Удалить');
    await expect(shell.dialogOk).toHaveClass(/btn-danger/);
    await shell.dialogOk.click();
    expect(await shell.dialogResult()).toBe(true);

    await shell.openDialog('confirm', 'Удалить?');
    await shell.dialogCancel.click();
    expect(await shell.dialogResult()).toBe(false);

    await shell.openDialog('confirm', 'Удалить?');
    await page.keyboard.press('Escape');
    expect(await shell.dialogResult()).toBe(false);
  });

  test('SH-011: uiPrompt — значение по Enter, null при отмене', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');

    await shell.openDialog('prompt', 'Название категории', 'Фильтры');
    await expect(shell.dialogInput).toBeFocused();
    await expect(shell.dialogInput).toHaveValue('Фильтры');
    await shell.dialogInput.fill('  Запчасти 🔧 ');
    await shell.dialogInput.press('Enter');
    expect(await shell.dialogResult()).toBe('  Запчасти 🔧 ');

    await shell.openDialog('prompt', 'Название');
    await shell.dialogCancel.click();
    expect(await shell.dialogResult()).toBeNull();
  });

  test('SH-012: uiAlert — одна кнопка; HTML в тексте не интерпретируется', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');

    await shell.openDialog('alert', '<img src=x onerror="window.__xss=1">У клиента есть заказы');
    await expect(shell.dialogCancel).toHaveCount(0);
    await expect(shell.dialog.locator('img')).toHaveCount(0);
    await expect(shell.dialog.getByTestId('ui-dialog-text')).toContainText('<img src=x');
    await shell.dialogOk.click();
    expect(await shell.dialogResult()).toBe(true);
    expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined();
  });
});

test.describe('SH: выпадающий список с поиском (D6)', () => {
  const SELECT = `
    <label for="e2e-sel">Товар</label>
    <select id="e2e-sel" name="product" class="form-select" data-testid="e2e-sel">
      <option value="">— выберите —</option>
      <optgroup label="Фильтры">
        <option value="1" data-hint="FL-01 · 120 ₽" data-search="воздушный">Фильтр салона</option>
        <option value="2" data-hint="FL-02">Фильтр масляный</option>
      </optgroup>
      <optgroup label="Запчасти">
        <option value="3" data-hint="LN-10">Линза фокусирующая</option>
      </optgroup>
    </select>
    <select id="e2e-native" class="form-select" data-native><option value="a">A</option></select>`;

  test('SH-020: select превращается в комбобокс, поиск по тексту/подсказке/data-search', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');
    await shell.injectSelect(SELECT);

    const btn = page.getByTestId('e2e-sel');
    await expect(btn).toHaveJSProperty('tagName', 'BUTTON');
    await expect(btn).toHaveText('— выберите —');
    await expect(page.locator('select#e2e-sel')).toBeHidden();
    // [data-native] не трогаем
    await expect(page.locator('select#e2e-native')).toBeVisible();

    await btn.click();
    await expect(shell.dropdown).toBeVisible();
    await expect(shell.dropdown.locator('.cb-group')).toHaveText(['Фильтры', 'Запчасти']);
    await expect(shell.dropdown.locator('.cb-opt')).toHaveCount(4);

    const search = shell.dropdown.getByRole('textbox', { name: 'Поиск' });
    await search.fill('воздуш'); // только data-search
    await expect(shell.dropdown.locator('.cb-opt')).toHaveCount(1);
    await expect(shell.dropdown.locator('.cb-opt')).toContainText('Фильтр салона');

    await search.fill('ln-10'); // по подсказке, без учёта регистра
    await expect(shell.dropdown.locator('.cb-opt')).toHaveText(/Линза фокусирующая/);
    await expect(shell.dropdown.locator('.cb-opt mark')).toHaveText('LN-10');

    await search.fill('несуществующее');
    await expect(shell.dropdown.locator('.cb-empty')).toHaveText('Ничего не найдено');

    await search.press('Escape');
    await expect(shell.dropdown).toHaveCount(0);
    await expect(btn).toBeFocused();
  });

  test('SH-021: выбор клавиатурой и мышью меняет value и шлёт change', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');
    await shell.injectSelect(SELECT);
    await page.evaluate(() => {
      (window as unknown as { __changes: string[] }).__changes = [];
      document.getElementById('e2e-sel')!.addEventListener('change', (e) =>
        (window as unknown as { __changes: string[] }).__changes.push((e.target as HTMLSelectElement).value));
    });
    const btn = page.getByTestId('e2e-sel');

    await btn.focus();
    await page.keyboard.press('Enter');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    await expect(btn).toHaveText('Фильтр масляный');
    await expect(page.locator('select#e2e-sel')).toHaveValue('2');

    await btn.click();
    await shell.dropdown.locator('.cb-opt', { hasText: 'Линза фокусирующая' }).click();
    await expect(btn).toHaveText('Линза фокусирующая');
    await expect(page.locator('select#e2e-sel')).toHaveValue('3');

    // повторный выбор того же значения change не шлёт
    await btn.click();
    await shell.dropdown.locator('.cb-opt', { hasText: 'Линза фокусирующая' }).click();
    expect(await page.evaluate(() => (window as unknown as { __changes: string[] }).__changes)).toEqual(['2', '3']);

    // печать буквы на кнопке открывает список с этой буквой в поиске
    await btn.focus();
    await page.keyboard.press('l'); // «l» есть в подсказках FL-01, FL-02, LN-10
    await expect(shell.dropdown.getByRole('textbox', { name: 'Поиск' })).toHaveValue('l');
    await expect(shell.dropdown.locator('.cb-opt')).toHaveCount(3);
    await page.mouse.click(5, 5); // клик мимо закрывает
    await expect(shell.dropdown).toHaveCount(0);
  });
});

test.describe('SH: выпадающий список — экранирование', () => {
  test('SH-022: HTML в тексте опции, подсказке и группе не интерпретируется', async ({ adminPage: page }) => {
    const shell = new ShellPage(page);
    await shell.goto('/');
    await shell.injectSelect(`
      <select class="form-select" data-testid="xss-sel">
        <optgroup label="&lt;img src=x onerror=&quot;window.__xss=1&quot;&gt;">
          <option value="1" data-hint="&lt;img src=x onerror=&quot;window.__xss=2&quot;&gt;">&lt;b&gt;жирный&lt;/b&gt;</option>
        </optgroup>
      </select>`);
    const btn = page.getByTestId('xss-sel');
    await btn.click();
    await expect(shell.dropdown.locator('.cb-opt')).toContainText('<b>жирный</b>');
    await shell.dropdown.getByRole('textbox', { name: 'Поиск' }).fill('img');
    await expect(shell.dropdown.locator('.cb-opt small')).toContainText('<img src=x');
    await expect(shell.dropdown.locator('img, b')).toHaveCount(0);
    await shell.dropdown.locator('.cb-opt').click();
    await expect(btn).toHaveText('<b>жирный</b>');
    await expect(btn.locator('b')).toHaveCount(0);
    expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined();
  });
});

test.describe('SH: адаптив (US-1 adaptive:min)', () => {
  test('SH-030: 1280px — нет горизонтального переполнения, сайдбар виден', async ({ adminPage: page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    const shell = new ShellPage(page);
    await shell.goto('/');
    await expect(shell.sidebar).toBeInViewport();
    await expect(shell.menuToggle).toBeHidden();
    expect(await shell.hasHorizontalOverflow()).toBe(false);
  });

  test('SH-031: 360px — меню за кнопкой ☰, закрывается по фону, без переполнения', async ({ adminPage: page }) => {
    await page.setViewportSize({ width: 360, height: 640 });
    const shell = new ShellPage(page);
    await shell.goto('/');
    expect(await shell.hasHorizontalOverflow()).toBe(false);
    await expect(shell.sidebar).not.toBeInViewport();

    await shell.menuToggle.click();
    await expect(shell.sidebar).toBeInViewport();
    await page.mouse.click(350, 320); // backdrop справа от сайдбара
    await expect(shell.sidebar).not.toBeInViewport();
  });
});
