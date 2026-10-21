import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана «Сотрудники»: список /users и форма /users/new, /users/:id.
export class UsersPage {
  readonly table: Locator;
  readonly addButton: Locator;
  readonly flash: Locator;
  readonly search: Locator;
  readonly roleFilter: Locator;
  readonly statusFilter: Locator;
  readonly empty: Locator;
  // форма
  readonly form: Locator;
  readonly name: Locator;
  readonly login: Locator;
  readonly role: Locator;
  readonly phone: Locator;
  /** «Часовой пояс» — combobox-кнопка (testid переносится на неё) и скрытый нативный select. */
  readonly timezone: Locator;
  readonly timezoneNative: Locator;
  readonly password: Locator;
  readonly active: Locator;
  readonly save: Locator;
  readonly formError: Locator;

  constructor(readonly page: Page) {
    this.table = page.getByTestId('users-table');
    this.addButton = page.getByTestId('user-add');
    this.flash = page.getByTestId('flash');
    this.search = page.getByTestId('filter-q');
    this.roleFilter = page.getByTestId('filter-role');
    this.statusFilter = page.getByTestId('filter-status');
    this.empty = page.getByTestId('users-empty');
    this.form = page.getByTestId('user-form');
    this.name = page.getByLabel('ФИО *');
    this.login = page.getByLabel('Логин *');
    this.role = page.getByTestId('role'); // кнопка комбобокса (testid переносится на неё)
    this.phone = page.getByLabel('Телефон');
    this.timezone = page.getByTestId('user-timezone');
    this.timezoneNative = page.locator('#timezone');
    this.password = page.locator('#password');
    this.active = page.getByLabel('Активен (может входить в систему)');
    this.save = page.getByTestId('user-save');
    this.formError = page.getByTestId('form-error');
  }

  async goto(query = '') {
    await this.page.goto(`/users${query}`);
    await expect(this.page.locator('.app-title')).toHaveText('Сотрудники');
  }

  async gotoNew() {
    await this.page.goto('/users/new');
    await expect(this.page.locator('.app-title')).toHaveText('Новый сотрудник');
  }

  row(login: string): Locator {
    return this.page.locator(`[data-testid="user-row"][data-login="${login}"]`);
  }

  fieldError(field: 'name' | 'login' | 'role' | 'phone' | 'password' | 'timezone' | 'supplier_id'): Locator {
    return this.page.getByTestId(`error-${field}`);
  }

  /** Выбор роли в комбобоксе (D6). */
  async chooseRole(label: 'Исполнитель' | 'Администратор' | 'Поставщик') {
    await this.role.click();
    await this.page.getByTestId('combobox-dropdown').locator('.cb-opt', { hasText: label }).click();
  }

  /** «Поставщик» учётки роли supplier (purchase-orders §8.5): нативный select, виден только у
   *  роли «Поставщик». */
  get supplier(): Locator {
    return this.page.getByTestId('user-supplier');
  }

  /** Выбор пояса в комбобоксе по подписи («Москва (UTC+3)»). */
  async chooseTimezone(label: string) {
    await this.timezone.click();
    await this.page.getByTestId('combobox-dropdown').locator('.cb-opt', { hasText: label }).click();
    await expect(this.timezone).toHaveText(label);
  }

  /** Подмена значения select'а на то, чего нет в перечне (проверка серверной валидации). */
  async forgeTimezone(value: string) {
    await this.timezoneNative.evaluate((el, v) => {
      const select = el as HTMLSelectElement;
      const option = document.createElement('option');
      option.value = v;
      option.textContent = v;
      select.append(option);
      select.value = v;
    }, value);
    await expect(this.timezoneNative).toHaveValue(value);
  }

  async open(login: string) {
    await this.row(login).getByRole('link').click();
    await expect(this.form).toBeVisible();
  }

  async fill(u: { name?: string; login?: string; phone?: string; password?: string }) {
    if (u.name !== undefined) await this.name.fill(u.name);
    if (u.login !== undefined) await this.login.fill(u.login);
    if (u.phone !== undefined) await this.phone.fill(u.phone);
    if (u.password !== undefined) await this.password.fill(u.password);
  }
}
