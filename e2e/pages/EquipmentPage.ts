import { expect, type Locator, type Page } from '@playwright/test';

// POM формы автомобиля — /clients/:id/equipment/new и /equipment/:id. Автомобиль — только
// госномер: единственное поле «Госномер *» (пусто → 422 «Укажите госномер»).
export class EquipmentPage {
  readonly form: Locator;
  readonly clientName: Locator;
  readonly plate: Locator;
  readonly save: Locator;
  readonly formError: Locator;
  readonly deleteButton: Locator;

  constructor(readonly page: Page) {
    this.form = page.getByTestId('equipment-form');
    this.clientName = page.getByTestId('equipment-client-name');
    this.plate = this.form.getByTestId('equipment-plate-number');
    this.save = page.getByTestId('equipment-save');
    this.formError = page.getByTestId('form-error');
    this.deleteButton = page.getByTestId('equipment-delete');
  }

  async gotoNew(clientId: string) {
    await this.page.goto(`/clients/${clientId}/equipment/new`);
    await expect(this.page.locator('.app-title')).toHaveText('Новое оборудование');
  }

  async gotoEdit(equipmentId: string) {
    await this.page.goto(`/equipment/${equipmentId}`);
    await expect(this.page.locator('.app-title')).toHaveText('Оборудование');
  }

  fieldError(field: 'plate_number'): Locator {
    return this.page.getByTestId(`error-${field}`);
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

  /** «Удалить» → окно «Удаление» открыто (форма единицы не отправлена). */
  async openDelete() {
    await this.deleteButton.click();
    await expect(this.deleteDialog).toBeVisible();
  }

  /** Мягкое удаление единицы: «Удалить» → причина в окне → «Удалить» (форма уходит обычным POST). */
  async confirmDelete(reason = 'e2e: создан по ошибке') {
    await this.openDelete();
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }
}
