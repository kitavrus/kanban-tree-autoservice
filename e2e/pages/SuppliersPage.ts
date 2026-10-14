import { expect, type Locator, type Page } from '@playwright/test';
import type { NewSupplier } from '../fixtures/auth';

// POM экрана «Поставщики» «/suppliers» (suppliers DS-007, draft-spec §8.1,
// web/ssr/_shared/js/suppliers.js): таблица с поиском и toggle архивных + форма справа
// (?new=1 — создание, ?id=<uuid> — правка). Все действия — через JSON API, результат —
// тост справа вверху.
export class SuppliersPage {
  readonly root: Locator;
  readonly addButton: Locator;
  readonly search: Locator;
  readonly archivedToggle: Locator;
  readonly table: Locator;
  readonly rows: Locator;
  readonly empty: Locator;
  readonly emptyAdd: Locator;
  // форма
  readonly form: Locator;
  readonly formTitle: Locator;
  readonly name: Locator;
  readonly contactName: Locator;
  readonly phone: Locator;
  readonly address: Locator;
  readonly note: Locator;
  readonly save: Locator;
  readonly toggleActive: Locator;
  readonly cancel: Locator;
  readonly archivedNote: Locator;
  /** «Удалить» внизу формы поставщика слева (soft-delete-suppliers §8.1). */
  readonly delete: Locator;
  // окно «Удаление»
  readonly deleteDialog: Locator;
  readonly deleteDialogTitle: Locator;
  readonly deleteDialogText: Locator;
  readonly deleteReason: Locator;
  readonly deleteReasonError: Locator;
  readonly deleteDialogOk: Locator;
  // тосты
  readonly toast: Locator;
  readonly toastError: Locator;

  constructor(readonly page: Page) {
    this.root = page.getByTestId('suppliers-root');
    this.addButton = page.getByTestId('supplier-add');
    this.search = page.getByTestId('supplier-search');
    this.archivedToggle = page.getByTestId('supplier-filter-archived');
    this.table = page.getByTestId('suppliers-table');
    this.rows = page.getByTestId('supplier-row');
    this.empty = page.getByTestId('suppliers-empty');
    this.emptyAdd = page.getByTestId('supplier-empty-add');
    this.form = page.getByTestId('supplier-form');
    this.formTitle = page.getByTestId('supplier-form-title');
    this.name = page.getByTestId('supplier-name');
    this.contactName = page.getByTestId('supplier-contact-name');
    this.phone = page.getByTestId('supplier-phone');
    this.address = page.getByTestId('supplier-address');
    this.note = page.getByTestId('supplier-note');
    this.save = page.getByTestId('supplier-save');
    this.toggleActive = page.getByTestId('supplier-toggle-active');
    this.cancel = page.getByTestId('supplier-cancel');
    this.archivedNote = page.getByTestId('supplier-archived-note');
    this.delete = page.getByTestId('supplier-delete');
    this.deleteDialog = page.locator('[data-testid="ui-dialog"][data-dialog-kind="delete"]');
    this.deleteDialogTitle = page.getByTestId('delete-dialog-title');
    this.deleteDialogText = this.deleteDialog.getByTestId('ui-dialog-text');
    this.deleteReason = page.getByTestId('delete-reason');
    this.deleteReasonError = page.getByTestId('delete-reason-error');
    this.deleteDialogOk = this.deleteDialog.getByTestId('ui-dialog-ok');
    this.toast = page.getByTestId('toast');
    this.toastError = page.getByTestId('toast-error');
  }

  async goto(query = '') {
    await this.page.goto(`/suppliers${query}`);
    await expect(this.root).toBeVisible();
  }

  /** Строка таблицы по точному названию. */
  row(name: string): Locator {
    return this.page.locator(`[data-testid="supplier-row"][data-name="${name}"]`);
  }

  archivedBadge(name: string): Locator {
    return this.row(name).getByTestId('supplier-archived-badge');
  }

  /** Текст ошибки под полем формы (ключ — wire-имя поля). */
  fieldError(field: keyof NewSupplier): Locator {
    return this.page.getByTestId(`error-${field.replace('_', '-')}`);
  }

  async openNew() {
    await this.addButton.click();
    await expect(this.form).toBeVisible();
    await expect(this.toggleActive).toBeHidden();
  }

  async openRow(name: string) {
    await this.row(name).click();
    await expect(this.form).toBeVisible();
    await expect(this.name).toHaveValue(name);
  }

  /** Заполняет поля формы wire-значениями поставщика (незаданные — не трогает). */
  async fill(s: Partial<NewSupplier>) {
    if (s.name !== undefined) await this.name.fill(s.name);
    if (s.contact_name !== undefined) await this.contactName.fill(s.contact_name);
    if (s.phone !== undefined) await this.phone.fill(s.phone);
    if (s.address !== undefined) await this.address.fill(s.address);
    if (s.note !== undefined) await this.note.fill(s.note);
  }

  /** Поиск по названию (debounce 300 мс — ждём по результату, не по времени). */
  async searchFor(q: string) {
    await this.search.fill(q);
  }

  async showArchived(on: boolean) {
    await this.archivedToggle.setChecked(on);
  }

  /** «Удалить» в форме → окно «Удаление» открыто. */
  async openDelete() {
    await this.delete.click();
    await expect(this.deleteDialog).toBeVisible();
  }

  /** «Удалить» в форме → причина → «Удалить» в окне. */
  async confirmDelete(reason = 'e2e: создан по ошибке') {
    await this.openDelete();
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }

  async expectSaved() {
    await expect(this.toast.last()).toHaveText('Сохранено');
    await expect(this.form).toBeHidden();
  }

  async hasHorizontalOverflow(): Promise<number> {
    return this.page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  }
}
