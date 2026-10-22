import { expect, type Locator, type Page } from '@playwright/test';

// POM закупок (docs/features/purchase-orders/draft-spec.md §8.1–§8.3) и кабинета поставщика
// (§8.6–§8.7). testid-контракт — web/ssr/purchase-orders/templates + js/purchases.js,
// js/supplier-orders.js.
export class PurchaseOrdersPage {
  constructor(readonly page: Page) {}

  // ---- §8.1 список ----
  async gotoList(query = '') {
    await this.page.goto(`/purchases${query}`);
    await expect(this.page.locator('.app-title')).toHaveText('Закупки');
  }
  get supplierFilter(): Locator { return this.page.getByTestId('po-filter-supplier'); }
  get search(): Locator { return this.page.getByTestId('po-filter-q'); }
  /** Быстрый фильтр: '' — «Все», new, active, discrepancy, changed, done. */
  view(v: string): Locator { return this.page.getByTestId(`po-view-${v || 'all'}`); }
  viewCount(v: string): Locator { return this.view(v).getByTestId('po-view-count'); }
  column(status: string): Locator { return this.page.locator(`[data-testid="po-col"][data-status="${status}"]`); }
  get rows(): Locator { return this.page.getByTestId('po-row'); }
  row(orderId: string): Locator { return this.page.locator(`[data-testid="po-row"][data-id="${orderId}"]`); }
  get newButton(): Locator { return this.page.getByTestId('po-new'); }

  // ---- §8.2 новый заказ ----
  async gotoNew() {
    await this.page.goto('/purchases/new');
    await expect(this.page.locator('.app-title')).toHaveText('Новый заказ');
  }
  get recipient(): Locator { return this.page.getByTestId('po-recipient'); }
  get note(): Locator { return this.page.getByTestId('po-note'); }
  toggle(reason: 'for_orders' | 'below_min'): Locator { return this.page.getByTestId(`po-toggle-${reason}`); }
  line(productId: string): Locator { return this.page.locator(`[data-testid="po-line"][data-product-id="${productId}"]`); }
  lineCheck(productId: string): Locator { return this.line(productId).getByTestId('po-line-check'); }
  lineQty(productId: string): Locator { return this.line(productId).getByTestId('po-line-qty'); }
  lineHint(productId: string): Locator { return this.line(productId).getByTestId('po-line-open-hint'); }
  get saveDraft(): Locator { return this.page.getByTestId('po-save-draft'); }
  get itemsError(): Locator { return this.page.getByTestId('po-error-items'); }

  /** Снять все галочки дефицита (оба переключателя типа) — на общем стенде дефицит не только наш. */
  async uncheckAll() {
    for (const r of ['for_orders', 'below_min'] as const) {
      if (await this.toggle(r).isChecked()) await this.toggle(r).uncheck();
    }
  }

  /** Поиск «Добавить запчасть» и выбор пункта. */
  async addProduct(name: string) {
    await this.page.getByTestId('po-add-product').fill(name);
    await this.page.getByTestId('po-add-option').filter({ hasText: name }).first().click();
  }

  /** Сохранить черновик → карточка; id заказа из адреса. */
  async saveAndOpen(): Promise<string> {
    await this.saveDraft.click();
    await this.page.waitForURL(/\/purchases\/[0-9a-f-]{36}/);
    return new URL(this.page.url()).pathname.split('/').pop()!;
  }

  // ---- §8.3 карточка ----
  async gotoView(orderId: string) {
    await this.page.goto(`/purchases/${orderId}`);
    await expect(this.number).not.toBeEmpty();
  }
  get number(): Locator { return this.page.getByTestId('po-number'); }
  get status(): Locator { return this.page.getByTestId('po-status'); }
  get cancelReason(): Locator { return this.page.getByTestId('po-cancel-reason'); }
  get items(): Locator { return this.page.getByTestId('po-item'); }
  item(productId: string): Locator { return this.page.locator(`[data-testid="po-item"][data-product-id="${productId}"]`); }
  itemQty(productId: string): Locator { return this.item(productId).getByTestId('po-item-qty'); }
  get total(): Locator { return this.page.getByTestId('po-total'); }
  button(name: 'save' | 'send' | 'recall' | 'pickup' | 'cancel' | 'delete' | 'print'): Locator {
    return this.page.getByTestId(`po-${name}`);
  }
  get toast(): Locator { return this.page.getByTestId('toast'); }
  get toastError(): Locator { return this.page.getByTestId('toast-error'); }

  /** Подтверждение KT.uiDialog («Забрали»/«Собрано»). */
  async confirm() { await this.page.getByTestId('ui-dialog-ok').click(); }

  get reasonInput(): Locator { return this.page.getByTestId('po-reason-input'); }
  get reasonOk(): Locator { return this.page.getByTestId('po-reason-ok'); }

  // ---- §8.6–§8.7 кабинет поставщика ----
  async gotoSupplierList() {
    await this.page.goto('/supplier/orders');
    await expect(this.page.getByTestId('so-list-root')).toBeVisible();
  }
  supplierCard(orderId: string): Locator { return this.page.locator(`[data-testid="so-card"][data-id="${orderId}"]`); }
  async gotoSupplierOrder(orderId: string) {
    await this.page.goto(`/supplier/orders/${orderId}`);
  }
  get supplierStatus(): Locator { return this.page.getByTestId('so-status'); }
  get supplierItems(): Locator { return this.page.getByTestId('so-item'); }
  /** Карточка строки кабинета по названию запчасти. */
  supplierItem(name: string): Locator { return this.supplierItems.filter({ hasText: name }); }
  get supplierReady(): Locator { return this.page.getByTestId('so-ready'); }
  get supplierTotal(): Locator { return this.page.getByTestId('so-total'); }
  get supplierNote(): Locator { return this.page.getByTestId('so-note'); }
  get notFound(): Locator { return this.page.getByTestId('so-not-found'); }

  /** Ответ по строке кабинета: поля сохраняются по уходу из поля (DS-063). */
  async answer(name: string, a: { qty?: string; price?: string; note?: string }) {
    const card = this.supplierItem(name);
    if (a.qty !== undefined) { await card.getByTestId('so-item-qty').fill(a.qty); await card.getByTestId('so-item-qty').blur(); }
    if (a.price !== undefined) { await card.getByTestId('so-item-price').fill(a.price); await card.getByTestId('so-item-price').blur(); }
    if (a.note !== undefined) { await card.getByTestId('so-item-note').fill(a.note); await card.getByTestId('so-item-note').blur(); }
    await expect(card.getByTestId('so-item-state')).toHaveText('Сохранено ✓');
  }
}
