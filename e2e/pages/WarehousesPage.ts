import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана «Склады» «/warehouses» (warehouses DS-001; soft-delete-warehouses-suppliers §8.1):
// серверная страница — список с формой добавления, в строке «В архив» / «Вернуть» и «Удалить»
// (общее окно «Удаление» с обязательной причиной, серверная форма form[data-delete-dialog]).
// Разметка — web/ssr/warehouses/templates/pages/list.html.
export class WarehousesPage {
  readonly table: Locator;
  readonly newName: Locator;
  readonly add: Locator;
  readonly flash: Locator;
  readonly listError: Locator;
  readonly toast: Locator;
  // окно «Удаление»
  readonly deleteDialog: Locator;
  readonly deleteDialogTitle: Locator;
  readonly deleteDialogText: Locator;
  readonly deleteReason: Locator;
  readonly deleteReasonError: Locator;
  readonly deleteDialogOk: Locator;

  constructor(readonly page: Page) {
    this.table = page.getByTestId('warehouses-table');
    this.newName = page.getByTestId('warehouse-new-name');
    this.add = page.getByTestId('warehouse-add');
    this.flash = page.getByTestId('flash');
    this.listError = page.getByTestId('list-error');
    this.toast = page.getByTestId('toast');
    this.deleteDialog = page.locator('[data-testid="ui-dialog"][data-dialog-kind="delete"]');
    this.deleteDialogTitle = page.getByTestId('delete-dialog-title');
    this.deleteDialogText = this.deleteDialog.getByTestId('ui-dialog-text');
    this.deleteReason = page.getByTestId('delete-reason');
    this.deleteReasonError = page.getByTestId('delete-reason-error');
    this.deleteDialogOk = this.deleteDialog.getByTestId('ui-dialog-ok');
  }

  /** Открыть список; archived — с архивными складами. */
  async goto(archived = false) {
    await this.page.goto(`/warehouses${archived ? '?archived=1' : ''}`);
    await expect(this.newName).toBeVisible();
  }

  /** Строка склада по точному названию. */
  row(name: string): Locator {
    return this.page.locator(`[data-testid="warehouse-row"][data-name="${name}"]`);
  }

  toggleActive(name: string): Locator { return this.row(name).getByTestId('warehouse-toggle-active'); }
  deleteBtn(name: string): Locator { return this.row(name).getByTestId('warehouse-delete'); }
  archivedBadge(name: string): Locator { return this.row(name).getByTestId('warehouse-archived-badge'); }

  /** «Удалить» в строке склада → окно «Удаление» открыто. */
  async openDelete(name: string) {
    await this.deleteBtn(name).click();
    await expect(this.deleteDialog).toBeVisible();
  }

  /** «Удалить» в строке → причина → «Удалить» в окне (серверная форма уходит PRG-переходом). */
  async delete(name: string, reason = 'e2e: создан по ошибке') {
    await this.openDelete(name);
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }
}
