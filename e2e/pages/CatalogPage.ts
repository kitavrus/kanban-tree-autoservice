import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана «Каталог» — вкладки /catalog/services, /catalog/products,
// /catalog/equipment-types (фаза 4, подфаза A: товары+категории; услуги — заглушка).
export class CatalogPage {
  readonly tabServices: Locator;
  // pagination DS-005/DS-006: пейджер SSR-списков и пометка усечённых категорий.
  readonly pager: Locator;
  readonly pagerShown: Locator;
  readonly pagerPrev: Locator;
  readonly pagerNext: Locator;
  readonly categoriesTruncated: Locator;
  readonly tabProducts: Locator;
  readonly tabTypes: Locator;
  readonly servicesEmpty: Locator;
  readonly flash: Locator;
  readonly catError: Locator;
  readonly search: Locator;
  readonly archivedFilter: Locator;
  readonly table: Locator;
  readonly empty: Locator;
  readonly addButton: Locator;
  readonly categoryPanel: Locator;
  readonly categoryAll: Locator;
  readonly newCategoryName: Locator;
  readonly newCategorySubmit: Locator;
  // форма товара
  readonly form: Locator;
  readonly name: Locator;
  readonly sku: Locator;
  readonly category: Locator;
  readonly unit: Locator;
  readonly price: Locator;
  readonly stock: Locator;
  readonly minStock: Locator;
  readonly box: Locator;
  readonly place: Locator;
  readonly save: Locator;
  readonly formError: Locator;
  readonly deleteButton: Locator;
  readonly archiveButton: Locator;
  // форма услуги
  readonly serviceForm: Locator;
  readonly serviceName: Locator;
  readonly serviceCategory: Locator;
  readonly servicePrice: Locator;
  readonly serviceSave: Locator;
  readonly serviceDeleteButton: Locator;
  readonly serviceArchiveButton: Locator;
  readonly serviceProductAdd: Locator;
  readonly servicesTable: Locator;

  constructor(readonly page: Page) {
    this.tabServices = page.getByTestId('catalog-tab-services');
    this.tabProducts = page.getByTestId('catalog-tab-products');
    this.tabTypes = page.getByTestId('catalog-tab-equipment-types');
    this.servicesEmpty = page.getByTestId('services-empty');
    this.flash = page.getByTestId('flash');
    this.catError = page.getByTestId('cat-error');
    this.search = page.getByTestId('filter-q');
    this.archivedFilter = page.getByTestId('filter-archived');
    this.table = page.getByTestId('products-table');
    this.empty = page.getByTestId('products-empty');
    this.addButton = page.getByTestId('product-add');
    this.categoryPanel = page.getByTestId('category-panel');
    this.categoryAll = page.getByTestId('cat-all');
    this.newCategoryName = page.getByTestId('cat-new-name');
    this.newCategorySubmit = page.getByTestId('cat-new-submit');
    this.form = page.getByTestId('product-form');
    this.name = page.getByTestId('product-name');
    this.sku = page.getByTestId('product-sku');
    this.category = page.getByTestId('product-category');
    this.unit = page.getByTestId('product-unit');
    this.price = page.getByTestId('product-price');
    // Блок «Склады» (warehouses DS-003): строка на каждый активный склад; короткие поля
    // формы — строка склада по умолчанию «Алматы», прочие — stockRow(name).
    const almaty = this.stockRow('Алматы');
    this.stock = almaty.getByTestId('product-stock');
    this.minStock = almaty.getByTestId('product-min-stock');
    this.box = almaty.getByTestId('product-box');
    this.place = almaty.getByTestId('product-place');
    this.save = page.getByTestId('product-save');
    this.formError = page.getByTestId('form-error');
    this.deleteButton = page.getByTestId('product-delete');
    this.archiveButton = page.getByTestId('product-archive');
    this.serviceForm = page.getByTestId('service-form');
    this.serviceName = page.getByTestId('service-name');
    this.serviceCategory = page.getByTestId('service-category');
    this.servicePrice = page.getByTestId('service-price');
    this.serviceSave = page.getByTestId('service-save');
    this.serviceDeleteButton = page.getByTestId('service-delete');
    this.serviceArchiveButton = page.getByTestId('service-archive');
    this.serviceProductAdd = page.getByTestId('service-product-add');
    this.servicesTable = page.getByTestId('services-table');
    this.pager = page.getByTestId('catalog-pager');
    this.pagerShown = page.getByTestId('catalog-shown');
    this.pagerPrev = page.getByTestId('catalog-prev');
    this.pagerNext = page.getByTestId('catalog-next');
    this.categoriesTruncated = page.getByTestId('categories-truncated');
  }

  async gotoServices(query = '') {
    await this.page.goto(`/catalog/services${query}`);
    await expect(this.page.locator('.app-title')).toHaveText('Каталог: услуги');
  }

  async gotoProducts(query = '') {
    await this.page.goto(`/catalog/products${query}`);
    await expect(this.page.locator('.app-title')).toHaveText('Каталог: товары');
  }

  /**
   * pagination DS-005: SSR-списки выдаются страницами по 100 — строку СВОЕЙ записи на общей
   * dev-БД ищем только через фильтр ?q=<уникальное имя>, а не на первой странице без фильтра.
   */
  async gotoProductsByName(name: string) {
    await this.gotoProducts(`?q=${encodeURIComponent(name)}`);
  }

  async gotoServicesByName(name: string) {
    await this.gotoServices(`?q=${encodeURIComponent(name)}`);
  }

  async gotoRoot() {
    await this.page.goto('/catalog');
  }

  async gotoNewProduct() {
    await this.page.goto('/catalog/products/new');
    await expect(this.page.locator('.app-title')).toHaveText('Новый товар');
  }

  row(name: string): Locator {
    return this.page.locator(`[data-testid="product-row"][data-name="${name}"]`);
  }

  rowToggle(name: string): Locator {
    return this.row(name).getByTestId('product-toggle-active');
  }

  /** Строка категории в панели — по видимому названию (текст ссылки cat-link). */
  categoryRow(name: string): Locator {
    return this.page.getByTestId('cat-row').filter({ has: this.page.getByTestId('cat-link').filter({ hasText: name }) });
  }

  categoryRowById(id: string): Locator {
    return this.page.locator(`[data-testid="cat-row"][data-cat-id="${id}"]`);
  }

  categoryLink(name: string): Locator {
    return this.categoryRow(name).getByTestId('cat-link');
  }

  categoryRenameInput(name: string): Locator {
    return this.categoryRow(name).getByTestId('cat-rename-input');
  }

  categoryRenameSubmit(name: string): Locator {
    return this.categoryRow(name).getByTestId('cat-rename-submit');
  }

  categoryToggle(name: string): Locator {
    return this.categoryRow(name).getByTestId('cat-toggle-active');
  }

  categoryDelete(name: string): Locator {
    return this.categoryRow(name).getByTestId('cat-delete');
  }

  async addCategory(name: string) {
    await this.newCategoryName.fill(name);
    await this.newCategorySubmit.click();
  }

  fieldError(field: string): Locator {
    return this.page.getByTestId(`error-${field}`);
  }

  async open(name: string) {
    await this.row(name).getByRole('link').click();
    await expect(this.form).toBeVisible();
  }

  /** Выбор в комбобоксе (D6, combobox.js переносит data-testid на кнопку — эталон UsersPage.chooseRole). */
  private async chooseInCombobox(button: Locator, label: string) {
    await button.click();
    await this.page.getByTestId('combobox-dropdown').locator('.cb-opt', { hasText: label }).click();
  }

  /** Выбор категории по видимому названию (опция может нести суффикс « (архив)» — hasText матчит подстрокой). */
  async chooseCategory(name: string) {
    await this.chooseInCombobox(this.category, name);
  }

  async chooseUnit(unit: string) {
    await this.chooseInCombobox(this.unit, unit);
  }

  /** Строка блока «Склады» формы товара по названию склада. */
  stockRow(warehouseName: string): Locator {
    return this.page.locator(`[data-testid="product-stock-row"][data-warehouse="${warehouseName}"]`);
  }

  async fill(p: {
    categoryName?: string;
    name?: string;
    sku?: string;
    unit?: string;
    price?: string;
    stock?: string;
    minStock?: string;
    box?: string;
    place?: string;
  }) {
    if (p.categoryName !== undefined) await this.chooseCategory(p.categoryName);
    if (p.name !== undefined) await this.name.fill(p.name);
    if (p.sku !== undefined) await this.sku.fill(p.sku);
    if (p.unit !== undefined) await this.chooseUnit(p.unit);
    if (p.price !== undefined) await this.price.fill(p.price);
    if (p.stock !== undefined) await this.stock.fill(p.stock);
    if (p.minStock !== undefined) await this.minStock.fill(p.minStock);
    if (p.box !== undefined) await this.box.fill(p.box);
    if (p.place !== undefined) await this.place.fill(p.place);
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

  /** «Удалить» в форме товара → окно «Удаление». */
  async openDelete() {
    await this.deleteButton.click();
    await expect(this.deleteDialog).toBeVisible();
  }

  /** Мягкое удаление товара: «Удалить» → причина в окне → «Удалить» (форма уходит обычным POST). */
  async confirmDelete(reason = 'e2e: создан по ошибке') {
    await this.openDelete();
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }

  /** Мягкое удаление категории из панели: «Удалить» в строке → причина → «Удалить». */
  async confirmCategoryDelete(name: string, reason = 'e2e: создана по ошибке') {
    await this.categoryDelete(name).click();
    await expect(this.deleteDialog).toBeVisible();
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }

  /** Бейдж «в архиве» строки товара / услуги списка. */
  archivedBadge(name: string): Locator {
    return this.row(name).getByTestId('product-archived-badge');
  }

  /** Бейдж «категория удалена» строки товара списка (soft-delete-catalog §8.1 п.3). */
  categoryDeletedBadge(name: string): Locator {
    return this.row(name).getByTestId('category-deleted-badge');
  }

  /** Бейдж «удалена» у поля «Категория» формы товара / услуги (§8.1 п.4). */
  get formCategoryDeletedBadge(): Locator { return this.page.getByTestId('category-deleted-badge'); }

  /** Бейдж «удалён» у строки состава услуги (§8.1 п.5). */
  productDeletedBadge(i: number): Locator {
    return this.productRow(i).getByTestId('product-deleted-badge');
  }

  // ---- услуги ----

  async gotoNewService() {
    await this.page.goto('/catalog/services/new');
    await expect(this.page.locator('.app-title')).toHaveText('Новая услуга');
  }

  serviceRow(name: string): Locator {
    return this.page.locator(`[data-testid="service-row"][data-name="${name}"]`);
  }

  serviceRowToggle(name: string): Locator {
    return this.serviceRow(name).getByTestId('service-toggle-active');
  }

  async openService(name: string) {
    await this.serviceRow(name).getByRole('link').click();
    await expect(this.serviceForm).toBeVisible();
  }

  async chooseServiceCategory(name: string) {
    await this.chooseInCombobox(this.serviceCategory, name);
  }

  /** Строка комплекта по индексу (0-based) — товар через combobox + количество. */
  productRow(i: number): Locator {
    return this.page.getByTestId('product-row-item').nth(i);
  }

  async chooseProductInRow(i: number, name: string) {
    await this.chooseInCombobox(this.productRow(i).getByTestId('service-product-select'), name);
  }

  async fillProductRowQty(i: number, qty: string) {
    await this.productRow(i).getByTestId('service-product-qty').fill(qty);
  }

  async removeProductRow(i: number) {
    await this.productRow(i).getByTestId('service-product-del').click();
  }

  async fillService(p: { categoryName?: string; name?: string; price?: string }) {
    if (p.categoryName !== undefined) await this.chooseServiceCategory(p.categoryName);
    if (p.name !== undefined) await this.serviceName.fill(p.name);
    if (p.price !== undefined) await this.servicePrice.fill(p.price);
  }

  /** Мягкое удаление услуги: «Удалить» → причина в окне → «Удалить». */
  async confirmServiceDelete(reason = 'e2e: создана по ошибке') {
    await this.serviceDeleteButton.click();
    await expect(this.deleteDialog).toBeVisible();
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }
}
