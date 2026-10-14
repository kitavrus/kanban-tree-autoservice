import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана «Каталог: типы оборудования» — /catalog/equipment-types (+ /new, /:id).
export class EquipmentTypesPage {
  readonly table: Locator;
  readonly addButton: Locator;
  readonly flash: Locator;
  readonly search: Locator;
  readonly archivedFilter: Locator;
  readonly empty: Locator;
  // форма
  readonly form: Locator;
  readonly name: Locator;
  readonly specKeys: Locator;
  readonly save: Locator;
  readonly formError: Locator;
  readonly deleteButton: Locator;
  readonly archiveButton: Locator;
  readonly archivedLink: Locator;

  constructor(readonly page: Page) {
    this.table = page.getByTestId('types-table');
    this.addButton = page.getByTestId('type-add');
    this.flash = page.getByTestId('flash');
    this.search = page.getByTestId('filter-q');
    this.archivedFilter = page.getByTestId('filter-archived');
    this.empty = page.getByTestId('types-empty');
    this.form = page.getByTestId('type-form');
    this.name = page.getByTestId('type-name');
    this.specKeys = page.getByTestId('type-spec-keys');
    this.save = page.getByTestId('type-save');
    this.formError = page.getByTestId('form-error');
    this.deleteButton = page.getByTestId('type-delete');
    this.archiveButton = page.getByTestId('type-archive');
    this.archivedLink = page.getByTestId('show-archived-link');
  }

  async goto(query = '') {
    await this.page.goto(`/catalog/equipment-types${query}`);
    await expect(this.page.locator('.app-title')).toHaveText('Каталог: типы оборудования');
  }

  async gotoNew() {
    await this.page.goto('/catalog/equipment-types/new');
    await expect(this.page.locator('.app-title')).toHaveText('Новый тип оборудования');
  }

  row(name: string): Locator {
    return this.page.locator(`[data-testid="type-row"][data-name="${name}"]`);
  }

  /** Ячейка «Единиц» строки списка — число действующих единиц типа (soft-delete-catalog TS-061). */
  rowCount(name: string): Locator {
    return this.row(name).getByTestId('type-equipment-count');
  }

  /** Кнопка «В архив»/«Вернуть» в строке списка. */
  rowToggle(name: string): Locator {
    return this.row(name).getByTestId('type-toggle-active');
  }

  fieldError(field: 'name' | 'spec_keys'): Locator {
    return this.page.getByTestId(`error-${field}`);
  }

  async open(name: string) {
    await this.row(name).getByRole('link').click();
    await expect(this.form).toBeVisible();
  }

  async fill(t: { name?: string; specKeys?: string }) {
    if (t.name !== undefined) await this.name.fill(t.name);
    if (t.specKeys !== undefined) await this.specKeys.fill(t.specKeys);
  }


  // Общее окно «Удаление» с причиной (soft-delete-catalog §8.1; KT.uiDeleteDialog).
  get deleteDialog(): Locator { return this.page.locator('[data-testid="ui-dialog"][data-dialog-kind="delete"]'); }
  get deleteDialogTitle(): Locator { return this.page.getByTestId('delete-dialog-title'); }
  get deleteDialogText(): Locator { return this.deleteDialog.getByTestId('ui-dialog-text'); }
  get deleteReason(): Locator { return this.page.getByTestId('delete-reason'); }
  get deleteReasonError(): Locator { return this.page.getByTestId('delete-reason-error'); }
  get deleteDialogOk(): Locator { return this.deleteDialog.getByTestId('ui-dialog-ok'); }
  get deleteDialogCancel(): Locator { return this.deleteDialog.getByTestId('ui-dialog-cancel'); }
  /** Тост после PRG-перехода («Удалено. Восстановить можно в разделе «Удалённые»»). */
  get toast(): Locator { return this.page.getByTestId('toast'); }

  /** «Удалить» в форме типа → окно «Удаление». */
  async openDelete() {
    await this.deleteButton.click();
    await expect(this.deleteDialog).toBeVisible();
  }

  /** Мягкое удаление типа: «Удалить» → причина в окне → «Удалить» (форма уходит обычным POST). */
  async confirmDelete(reason = 'e2e: создан по ошибке') {
    await this.openDelete();
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }
}
