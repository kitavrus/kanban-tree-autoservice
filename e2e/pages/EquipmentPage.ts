import { expect, type Locator, type Page } from '@playwright/test';

// POM формы единицы оборудования — /clients/:id/equipment/new и /equipment/:id
// (EQ-001..007). Комбобокс типа (D6) переносит data-testid на видимую кнопку —
// selectType() ведёт диалог поиска, а не работает с исходным <select> напрямую.
export class EquipmentPage {
  readonly form: Locator;
  readonly clientName: Locator;
  readonly typeButton: Locator;
  readonly brand: Locator;
  readonly model: Locator;
  readonly sku: Locator;
  readonly serial: Locator;
  readonly note: Locator;
  readonly specsRows: Locator;
  readonly specAdd: Locator;
  readonly save: Locator;
  readonly formError: Locator;
  readonly deleteButton: Locator;

  constructor(readonly page: Page) {
    this.form = page.getByTestId('equipment-form');
    this.clientName = page.getByTestId('equipment-client-name');
    this.typeButton = page.getByTestId('equipment-type');
    this.brand = page.locator('#brand');
    this.model = page.locator('#model');
    this.sku = page.locator('#sku');
    this.serial = page.locator('#serial');
    this.note = page.locator('#note');
    this.specsRows = page.getByTestId('spec-row');
    this.specAdd = page.getByTestId('spec-add');
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

  fieldError(field: 'type_id' | 'model' | 'brand' | 'sku' | 'serial' | 'note' | 'specs'): Locator {
    return this.page.getByTestId(`error-${field}`);
  }

  specKeyError(index: number): Locator {
    return this.page.getByTestId(`error-specs-${index}`);
  }

  specRow(index: number): Locator {
    return this.specsRows.nth(index);
  }

  specKey(index: number): Locator {
    return this.specRow(index).getByTestId('spec-key');
  }

  specValue(index: number): Locator {
    return this.specRow(index).getByTestId('spec-value');
  }

  specDel(index: number): Locator {
    return this.specRow(index).getByTestId('spec-del');
  }

  /** Выбор типа в комбобоксе (D6) — по видимому названию опции. */
  async selectType(name: string) {
    await this.typeButton.click();
    await this.page.getByTestId('combobox-dropdown').locator('.cb-opt', { hasText: name }).click();
  }

  async fill(e: { brand?: string; model?: string; sku?: string; serial?: string; note?: string }) {
    if (e.brand !== undefined) await this.brand.fill(e.brand);
    if (e.model !== undefined) await this.model.fill(e.model);
    if (e.sku !== undefined) await this.sku.fill(e.sku);
    if (e.serial !== undefined) await this.serial.fill(e.serial);
    if (e.note !== undefined) await this.note.fill(e.note);
  }

  async fillSpec(index: number, s: { key?: string; value?: string }) {
    if (s.key !== undefined) await this.specKey(index).fill(s.key);
    if (s.value !== undefined) await this.specValue(index).fill(s.value);
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
