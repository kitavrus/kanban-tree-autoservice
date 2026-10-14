import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана «Остатки» «/stock» (design-go-layers.md §Frontend Modules testid-контракт,
// эталон структуры — e2e/pages/BoardPage.ts). Переключатель scope — обычный <select>,
// превращённый js/combobox.js в комбобокс: testid уходит на видимую кнопку, выбор — клик
// по кнопке и затем по `.cb-opt` в `combobox-dropdown` (как у фильтров доски).
export class StockPage {
  readonly root: Locator;
  readonly kpiTotal: Locator;
  readonly kpiLow: Locator;
  readonly kpiOut: Locator;
  readonly kpiWillLow: Locator;
  readonly scope: Locator;
  readonly warehouse: Locator;
  readonly q: Locator;
  readonly table: Locator;
  readonly empty: Locator;
  readonly dropdown: Locator;

  readonly form: Locator;
  readonly modalWarehouse: Locator;
  readonly value: Locator;
  readonly save: Locator;
  readonly error: Locator;

  constructor(readonly page: Page) {
    this.root = page.getByTestId('stock-root');
    this.kpiTotal = page.getByTestId('stock-kpi-total');
    this.kpiLow = page.getByTestId('stock-kpi-low');
    this.kpiOut = page.getByTestId('stock-kpi-out');
    this.kpiWillLow = page.getByTestId('stock-kpi-will-low');
    this.scope = page.getByTestId('stock-scope');
    this.warehouse = page.getByTestId('stock-warehouse');
    this.q = page.getByTestId('stock-q');
    this.table = page.getByTestId('stock-table');
    this.empty = page.getByTestId('stock-empty');
    this.dropdown = page.getByTestId('combobox-dropdown');

    this.form = page.getByTestId('stock-form');
    this.modalWarehouse = page.getByTestId('stock-modal-warehouse');
    this.value = page.getByTestId('stock-value');
    this.save = page.getByTestId('stock-save');
    this.error = page.getByTestId('stock-error');
  }

  async goto() {
    await this.page.goto('/stock');
    await expect(this.root).toBeVisible();
  }

  row(productId: string): Locator {
    return this.page.getByTestId(`stock-row-${productId}`);
  }

  /** Бейдж «удалён» у товара, удалённого из каталога, чей остаток числится на складе (DS-042). */
  rowDeleted(productId: string): Locator {
    return this.row(productId).getByTestId('stock-row-deleted');
  }

  /** Бейдж «в архиве» у архивного товара, чей остаток числится на удалённом складе. */
  rowArchived(productId: string): Locator {
    return this.row(productId).getByTestId('stock-row-archived');
  }

  editBtn(productId: string): Locator {
    return this.page.getByTestId(`stock-edit-${productId}`);
  }

  /** Выбор опции в комбобоксе переключателя scope по видимому тексту. */
  async chooseScope(optionText: string) {
    await this.scope.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-opt', { hasText: optionText }).first().click();
  }

  /** Фильтр «Склад» (warehouses DS-004) — тот же combobox, что scope. */
  async chooseWarehouse(optionText: string) {
    await this.warehouse.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-opt', { hasText: optionText }).first().click();
  }

  /** Заголовок колонки склада по названию (с бейджем «в архиве» / «удалён» у недействующих складов
   *  с остатком — soft-delete-warehouses-suppliers §8.3 п.4). */
  col(warehouseName: string): Locator {
    return this.page.locator('[data-testid="stock-col-wh"]', { hasText: warehouseName });
  }
  colDeleted(warehouseName: string): Locator { return this.col(warehouseName).getByTestId('stock-col-wh-deleted'); }
  colArchived(warehouseName: string): Locator { return this.col(warehouseName).getByTestId('stock-col-wh-archived'); }

  /** Ячейка склада в строке товара (колонки складов). */
  cell(productId: string, warehouseName: string): Locator {
    return this.row(productId).locator(`[data-testid="stock-cell-wh"][data-warehouse="${warehouseName}"]`);
  }

  /** Склады, которые предлагает окно корректировки (опции исходного select'а под комбобоксом):
   *  у удалённого и архивного товара — только склады с его остатком (DS-048, DS-049). */
  get modalWarehouseOptions(): Locator {
    return this.page.locator('#stock-modal-warehouse option');
  }

  async openEdit(productId: string) {
    await this.editBtn(productId).click();
    await expect(this.form).toBeVisible();
  }

  async submitValue(value: string | number) {
    await this.value.fill(String(value));
    await this.save.click();
  }
}
