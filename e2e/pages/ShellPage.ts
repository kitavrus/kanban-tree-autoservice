import { expect, type Locator, type Page } from '@playwright/test';

// POM общего layout (web/ssr/_shared/templates/layouts/base.html) и общих
// JS-компонентов (KT.uiDialog / KT.enhanceSelect) — §TD-6.1.
export class ShellPage {
  readonly sidebar: Locator;
  readonly title: Locator;
  readonly themeToggle: Locator;
  /** Select «Часовой пояс» в сайдбаре — combobox-кнопка (combobox.js переносит testid). */
  readonly timezoneSelect: Locator;
  /** Скрытый нативный select под комбобоксом — источник value. */
  readonly timezoneNative: Locator;
  readonly timezoneToast: Locator;
  /** ☰ в шапке: на широком экране сворачивает/разворачивает меню, на узком — drawer. */
  readonly menuToggle: Locator;
  /** Логотип «Kanban-tree» в сайдбаре — тот же переключатель (скрыть меню). */
  readonly sidebarBrand: Locator;
  readonly dialog: Locator;
  readonly dialogOk: Locator;
  readonly dialogCancel: Locator;
  readonly dialogInput: Locator;
  readonly dropdown: Locator;

  constructor(readonly page: Page) {
    this.sidebar = page.getByTestId('sidebar');
    this.title = page.locator('.app-title');
    this.themeToggle = page.getByTestId('theme-toggle');
    this.timezoneSelect = page.getByTestId('timezone-select');
    this.timezoneNative = page.locator('#app-timezone');
    this.timezoneToast = page.getByTestId('timezone-toast-error');
    this.menuToggle = page.getByTestId('menu-toggle');
    this.sidebarBrand = page.getByTestId('sidebar-brand');
    this.dialog = page.getByTestId('ui-dialog');
    this.dialogOk = page.getByTestId('ui-dialog-ok');
    this.dialogCancel = page.getByTestId('ui-dialog-cancel');
    this.dialogInput = page.getByTestId('ui-dialog-input');
    this.dropdown = page.getByTestId('combobox-dropdown');
  }

  async goto(path = '/') {
    await this.page.goto(path);
    await expect(this.sidebar).toBeAttached();
  }

  navLink(title: string): Locator {
    return this.sidebar.getByRole('link', { name: title });
  }

  async theme(): Promise<string | null> {
    return this.page.locator('html').getAttribute('data-bs-theme');
  }

  /** Запускает диалог в странице; результат забирается через dialogResult(). */
  async openDialog(kind: 'confirm' | 'prompt' | 'alert', text: string, value = '') {
    await this.page.evaluate(
      ([k, t, v]) => {
        const KT = (window as unknown as { KT: Record<string, (...a: string[]) => Promise<unknown>> }).KT;
        const call = k === 'confirm' ? KT.uiConfirm(t) : k === 'prompt' ? KT.uiPrompt(t, v) : KT.uiAlert(t);
        (window as unknown as { __dlg: Promise<unknown> }).__dlg = call;
      },
      [kind, text, value] as const,
    );
    await expect(this.dialog).toBeVisible();
    // анимация показа завершена — фокус внутри диалога (Esc/Enter работают с этого момента)
    await expect(this.dialog.locator(':focus')).toHaveCount(1);
  }

  async dialogResult(): Promise<unknown> {
    const r = await this.page.evaluate(() => (window as unknown as { __dlg: Promise<unknown> }).__dlg);
    await expect(this.dialog).toHaveCount(0);
    return r;
  }

  /** Вставляет в контент страницы <select> — комбобокс должен подхватить его сам. */
  async injectSelect(html: string) {
    await this.page.evaluate((h) => {
      const box = document.createElement('div');
      box.id = 'e2e-box';
      box.innerHTML = h;
      document.querySelector('.app-content')!.append(box);
    }, html);
  }

  /** Пояс пользователя, по которому страница форматирует время (<body data-timezone>). */
  async expectTimezone(zone: string) {
    await expect(this.page.locator('body')).toHaveAttribute('data-timezone', zone);
    await expect(this.timezoneNative).toHaveValue(zone);
  }

  /** Выбор пояса в сайдбаре по подписи («Москва (UTC+3)») — PUT + перезагрузка страницы. */
  async chooseTimezone(label: string) {
    await this.timezoneSelect.click();
    await this.dropdown.locator('.cb-opt', { hasText: label }).click();
  }

  async hasHorizontalOverflow(): Promise<boolean> {
    return this.page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  }

  /** Сколько ширины окна НЕ досталось контенту (.app-main): 0 — меню свёрнуто, иначе ширина сайдбара. */
  async contentGap(): Promise<number> {
    return this.page.evaluate(() =>
      Math.round(document.documentElement.clientWidth - document.querySelector('.app-main')!.getBoundingClientRect().width));
  }
}
