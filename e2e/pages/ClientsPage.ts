import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана «Клиенты»: список /clients и форма /clients/new, /clients/:id
// (создание/правка/удаление, конфликт версии — ADR-005).
export class ClientsPage {
  readonly table: Locator;
  readonly addButton: Locator;
  readonly flash: Locator;
  readonly search: Locator;
  readonly empty: Locator;
  readonly total: Locator;
  // форма
  readonly form: Locator;
  readonly companyName: Locator;
  readonly address: Locator;
  readonly contactName: Locator;
  readonly contactPosition: Locator;
  readonly phonesRows: Locator;
  readonly phoneAdd: Locator;
  readonly save: Locator;
  readonly formError: Locator;
  readonly conflictReload: Locator;
  readonly deleteButton: Locator;
  readonly equipmentEmpty: Locator;

  constructor(readonly page: Page) {
    this.table = page.getByTestId('clients-table');
    this.addButton = page.getByTestId('client-add');
    this.flash = page.getByTestId('flash');
    this.search = page.getByTestId('filter-q');
    this.empty = page.getByTestId('clients-empty');
    this.total = page.getByTestId('clients-total');
    this.form = page.getByTestId('client-form');
    this.companyName = page.locator('#company_name');
    this.address = page.locator('#address');
    this.contactName = page.locator('#contact_name');
    this.contactPosition = page.locator('#contact_position');
    this.phonesRows = page.getByTestId('phone-row');
    this.phoneAdd = page.getByTestId('phone-add');
    this.save = page.getByTestId('client-save');
    this.formError = page.getByTestId('form-error');
    this.conflictReload = page.getByTestId('client-conflict-reload');
    this.deleteButton = page.getByTestId('client-delete');
    this.equipmentEmpty = page.getByTestId('equipment-empty');
  }

  async goto(query = '') {
    await this.page.goto(`/clients${query}`);
    await expect(this.page.locator('.app-title')).toHaveText('Клиенты');
  }

  async gotoNew() {
    await this.page.goto('/clients/new');
    await expect(this.page.locator('.app-title')).toHaveText('Новый клиент');
  }

  row(company: string): Locator {
    return this.page.locator(`[data-testid="client-row"][data-company="${company}"]`);
  }

  equipmentCount(company: string): Locator {
    return this.row(company).getByTestId('client-equipment-count');
  }

  fieldError(field: 'company_name' | 'address' | 'contact_name' | 'contact_position' | 'phones'): Locator {
    return this.page.getByTestId(`error-${field}`);
  }

  phoneFieldError(index: number): Locator {
    return this.page.getByTestId(`error-phones-${index}`);
  }

  phoneRow(index: number): Locator {
    return this.phonesRows.nth(index);
  }

  phoneInput(index: number): Locator {
    return this.phoneRow(index).getByTestId('phone-input');
  }

  phoneNote(index: number): Locator {
    return this.phoneRow(index).getByTestId('phone-note');
  }

  phoneDel(index: number): Locator {
    return this.phoneRow(index).getByTestId('phone-del');
  }

  async open(company: string) {
    await this.row(company).getByRole('link').click();
    await expect(this.form).toBeVisible();
  }

  async fill(c: { companyName?: string; address?: string; contactName?: string; contactPosition?: string }) {
    if (c.companyName !== undefined) await this.companyName.fill(c.companyName);
    if (c.address !== undefined) await this.address.fill(c.address);
    if (c.contactName !== undefined) await this.contactName.fill(c.contactName);
    if (c.contactPosition !== undefined) await this.contactPosition.fill(c.contactPosition);
  }

  async fillPhone(index: number, p: { phone?: string; note?: string }) {
    if (p.phone !== undefined) await this.phoneInput(index).fill(p.phone);
    if (p.note !== undefined) await this.phoneNote(index).fill(p.note);
  }

  // Общее окно «Удаление» с причиной (soft-delete-clients §8.1; KT.uiDeleteDialog).
  get deleteDialog(): Locator { return this.page.locator('[data-testid="ui-dialog"][data-dialog-kind="delete"]'); }
  get deleteDialogTitle(): Locator { return this.page.getByTestId('delete-dialog-title'); }
  get deleteDialogText(): Locator { return this.deleteDialog.getByTestId('ui-dialog-text'); }
  get deleteReason(): Locator { return this.page.getByTestId('delete-reason'); }
  get deleteReasonError(): Locator { return this.page.getByTestId('delete-reason-error'); }
  get deleteDialogOk(): Locator { return this.deleteDialog.getByTestId('ui-dialog-ok'); }
  get deleteDialogCancel(): Locator { return this.deleteDialog.getByTestId('ui-dialog-cancel'); }
  get toast(): Locator { return this.page.getByTestId('toast'); }

  /** «Удалить» → окно «Удаление» открыто (форма клиента не отправлена). */
  async openDelete() {
    await this.deleteButton.click();
    await expect(this.deleteDialog).toBeVisible();
  }

  /** Мягкое удаление клиента: «Удалить» → причина в окне → «Удалить» (форма уходит обычным POST). */
  async confirmDelete(reason = 'e2e: создан по ошибке') {
    await this.openDelete();
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }
}
