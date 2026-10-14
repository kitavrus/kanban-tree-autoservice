import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана «Перемещения» «/transfers» (transfers-core §8.1, transfers-partners §8.1,
// js/transfers.js): документ одного из четырёх направлений — приём от клиента / возврат
// клиенту (по заказу, единицы оборудования заказа), склад → склад (товары и единицы), приход
// от поставщика (товары). «Новое перемещение» — меню выбора направления.
// Селекты (фильтры, склады документа, «+ Товар») превращаются js/combobox.js в комбобоксы:
// testid уходит на видимую кнопку, выбор — клик по кнопке, поиск в `.cb-search` и клик по
// `.cb-opt` в `combobox-dropdown` (эталон — StockPage/CatalogPage). «+ Оборудование» — поле
// поиска с выпадающим списком единиц склада отправления (GET /api/v1/equipment).
export type Direction = 'client_to_warehouse' | 'warehouse_to_client' | 'warehouse_to_warehouse' | 'supplier_to_warehouse';

export const DIRECTION_LABEL: Record<Direction, string> = {
  client_to_warehouse: 'Приём от клиента',
  warehouse_to_client: 'Возврат клиенту',
  warehouse_to_warehouse: 'Склад → склад',
  supplier_to_warehouse: 'Приход от поставщика',
};

export class TransfersPage {
  readonly root: Locator;
  readonly list: Locator;
  readonly rows: Locator;
  readonly filterStatus: Locator;
  readonly filterWarehouse: Locator;
  readonly filterDirection: Locator;
  /** Фильтр «Поставщик» — только у администратора. */
  readonly filterSupplier: Locator;
  readonly filterTask: Locator;
  readonly filterFrom: Locator;
  readonly filterTo: Locator;
  readonly periodError: Locator;
  readonly filterReset: Locator;
  readonly shown: Locator;
  readonly more: Locator;
  /** «Новое перемещение» — открывает меню направлений (newDocument выбирает пункт). */
  readonly newBtn: Locator;
  readonly newDirections: Locator;
  readonly direction: Locator;
  readonly task: Locator;
  readonly taskInfo: Locator;
  readonly taskError: Locator;
  readonly taskLink: Locator;
  readonly supplier: Locator;
  readonly supplierError: Locator;
  readonly taskEquipment: Locator;
  readonly taskEquipmentOptions: Locator;
  readonly receiveNow: Locator;
  readonly qtyHead: Locator;
  /** Колонки «Цена» / «Сумма» и итог «Сумма накладной» — только у «возврата клиенту» (transfers-sales §8.1). */
  readonly priceHead: Locator;
  readonly totalHead: Locator;
  readonly amount: Locator;
  readonly factHead: Locator;
  readonly uiDialog: Locator;
  readonly placeholder: Locator;
  readonly cardTitle: Locator;
  readonly cardStatus: Locator;
  readonly cardMeta: Locator;
  readonly from: Locator;
  readonly to: Locator;
  readonly toError: Locator;
  readonly note: Locator;
  readonly itemsTable: Locator;
  readonly itemsError: Locator;
  readonly itemRows: Locator;
  readonly addProduct: Locator;
  readonly equipmentSearch: Locator;
  readonly equipmentHint: Locator;
  readonly equipmentOptions: Locator;
  readonly save: Locator;
  readonly send: Locator;
  readonly cancel: Locator;
  readonly receive: Locator;
  readonly warnings: Locator;
  readonly toast: Locator;
  readonly toastError: Locator;
  readonly receiveModal: Locator;
  readonly receiveRows: Locator;
  readonly receiveConfirm: Locator;
  readonly dropdown: Locator;
  /** «Отменить проведение» (admin, «Получено») и «Удалить» (admin, «Заявка» / «Отменён») —
   *  transfers-revert §8.1. */
  readonly unreceiveBtn: Locator;
  readonly deleteBtn: Locator;
  /** Диалог «Отменить проведение»: обязательная причина, ошибка под полем, мешающий документ. */
  readonly unreceiveModal: Locator;
  readonly unreceiveTitle: Locator;
  readonly unreceiveReason: Locator;
  readonly unreceiveReasonError: Locator;
  readonly unreceiveError: Locator;
  readonly unreceiveBlockedLink: Locator;
  readonly unreceiveConfirm: Locator;
  /** Строка сведений «Проведение отменено: кто, когда — причина». */
  readonly cardUnreceived: Locator;

  constructor(readonly page: Page) {
    this.root = page.getByTestId('st-root');
    this.list = page.getByTestId('st-list');
    this.rows = this.list.getByTestId('st-row');
    this.filterStatus = page.getByTestId('st-filter-status');
    this.filterWarehouse = page.getByTestId('st-filter-warehouse');
    this.filterDirection = page.getByTestId('st-filter-direction');
    this.filterSupplier = page.getByTestId('st-filter-supplier');
    this.filterTask = page.getByTestId('st-filter-task');
    this.filterFrom = page.getByTestId('st-filter-from');
    this.filterTo = page.getByTestId('st-filter-to');
    this.periodError = page.getByTestId('st-filter-period-error');
    this.filterReset = page.getByTestId('st-filter-reset');
    this.shown = page.getByTestId('st-shown');
    this.more = page.getByTestId('st-more');
    this.newBtn = page.getByTestId('st-new');
    this.newDirections = page.getByTestId('st-new-direction');
    this.direction = page.getByTestId('st-direction');
    this.task = page.getByTestId('st-task');
    this.taskInfo = page.getByTestId('st-task-info');
    this.taskError = page.getByTestId('st-task-error');
    this.taskLink = page.getByTestId('st-task-link');
    this.supplier = page.getByTestId('st-supplier');
    this.supplierError = page.getByTestId('st-supplier-error');
    this.taskEquipment = page.getByTestId('st-task-equipment');
    this.taskEquipmentOptions = page.getByTestId('st-task-equipment-option');
    this.receiveNow = page.getByTestId('st-receive-now');
    this.qtyHead = page.getByTestId('st-qty-head');
    this.priceHead = page.getByTestId('st-price-head');
    this.totalHead = page.getByTestId('st-total-head');
    this.amount = page.getByTestId('st-amount');
    this.factHead = page.getByTestId('st-fact-head');
    this.uiDialog = page.getByTestId('ui-dialog');
    this.placeholder = page.getByTestId('st-placeholder');
    this.cardTitle = page.getByTestId('st-card-title');
    this.cardStatus = page.getByTestId('st-card-status').getByTestId('st-status-badge');
    this.cardMeta = page.getByTestId('st-card-meta');
    this.from = page.getByTestId('st-from');
    this.to = page.getByTestId('st-to');
    this.toError = page.getByTestId('st-to-error');
    this.note = page.getByTestId('st-note');
    this.itemsTable = page.getByTestId('st-items');
    this.itemsError = page.getByTestId('st-items-error');
    this.itemRows = page.getByTestId('st-item-row');
    this.addProduct = page.getByTestId('st-add-product');
    this.equipmentSearch = page.getByTestId('st-equipment-search');
    this.equipmentHint = page.getByTestId('st-equipment-hint');
    this.equipmentOptions = page.getByTestId('st-equipment-option');
    this.save = page.getByTestId('st-save');
    this.send = page.getByTestId('st-send');
    this.cancel = page.getByTestId('st-cancel');
    this.receive = page.getByTestId('st-receive');
    this.warnings = page.getByTestId('st-warnings');
    this.toast = page.getByTestId('st-toast');
    this.toastError = page.getByTestId('st-toast-error');
    this.receiveModal = page.getByTestId('st-receive-modal');
    this.receiveRows = page.getByTestId('st-receive-row');
    this.receiveConfirm = page.getByTestId('st-receive-confirm');
    this.dropdown = page.getByTestId('combobox-dropdown');
    this.unreceiveBtn = page.getByTestId('st-unreceive');
    this.deleteBtn = page.getByTestId('st-delete');
    this.unreceiveModal = page.getByTestId('st-unreceive-modal');
    this.unreceiveTitle = page.getByTestId('st-unreceive-title');
    this.unreceiveReason = page.getByTestId('st-unreceive-reason');
    this.unreceiveReasonError = page.getByTestId('st-unreceive-reason-error');
    this.unreceiveError = page.getByTestId('st-unreceive-error');
    this.unreceiveBlockedLink = page.getByTestId('st-unreceive-blocked-link');
    this.unreceiveConfirm = page.getByTestId('st-unreceive-confirm');
    this.cardUnreceived = page.getByTestId('st-card-unreceived');
  }

  /** Значок «↩» в строке списка: документ возвращён откатом в «В пути». */
  rowUnreceived(id: number): Locator {
    return this.row(id).getByTestId('st-row-unreceived');
  }

  /** «Отменить проведение» → диалог с причиной (открыт и ждёт ввода). */
  async openUnreceive(id: number) {
    await this.unreceiveBtn.click();
    await expect(this.unreceiveModal).toBeVisible();
    await expect(this.unreceiveTitle).toHaveText(`Отменить проведение перемещения #${id}?`);
  }

  /** Причина + подтверждение в открытом диалоге «Отменить проведение». */
  async submitUnreceive(reason: string) {
    await this.unreceiveReason.fill(reason);
    await this.unreceiveConfirm.click();
  }

  // ---- окно «Удаление» с причиной (общее, soft-delete-core §8.2; документ — soft-delete-transfers §8.1) ----
  get deleteDialogTitle(): Locator { return this.page.getByTestId('delete-dialog-title'); }
  get deleteDialogText(): Locator { return this.uiDialog.getByTestId('ui-dialog-text'); }
  get deleteReason(): Locator { return this.page.getByTestId('delete-reason'); }
  get deleteReasonError(): Locator { return this.page.getByTestId('delete-reason-error'); }
  /** Красный блок над кнопками: отказ 409 (статус, версия). */
  get deleteDialogError(): Locator { return this.page.getByTestId('delete-dialog-error'); }

  /** «Удалить» → окно «Удаление» открыто: заголовок и пояснение проверяются. */
  async openDelete(id: number) {
    await this.deleteBtn.click();
    await expect(this.uiDialog).toBeVisible();
    await expect(this.deleteDialogTitle).toHaveText(`Удалить «Перемещение #${id}»?`);
    await expect(this.deleteDialogText).toHaveText('Документ можно восстановить в разделе «Удалённые»');
  }

  /** Причина + «Удалить» в открытом окне «Удаление» (окно может остаться открытым — отказ). */
  async submitDelete(reason: string) {
    await this.deleteReason.fill(reason);
    await this.uiDialog.getByTestId('ui-dialog-ok').click();
  }

  async cancelDelete() {
    await this.uiDialog.getByTestId('ui-dialog-cancel').click();
    await expect(this.uiDialog).toHaveCount(0);
  }

  /** «Удалить» → окно «Удаление» → причина и подтверждение либо отказ (документ на месте). */
  async deleteDocument(id: number, confirm = true, reason = 'e2e: создан по ошибке') {
    await this.openDelete(id);
    if (!confirm) return this.cancelDelete();
    await this.submitDelete(reason);
    await expect(this.uiDialog).toHaveCount(0);
  }

  /** Бейджи «удалён» у склада и поставщика документа — в строке списка и в карточке документа
   *  (soft-delete-warehouses-suppliers §8.3 п.1). */
  rowSupplierDeleted(id: number): Locator { return this.row(id).getByTestId('st-row-supplier-deleted'); }
  rowToDeleted(id: number): Locator { return this.row(id).getByTestId('st-row-to-deleted'); }
  rowFromDeleted(id: number): Locator { return this.row(id).getByTestId('st-row-from-deleted'); }
  get viewSupplier(): Locator { return this.page.getByTestId('st-view-supplier'); }
  get viewSupplierDeleted(): Locator { return this.page.getByTestId('st-view-supplier-deleted'); }
  get viewTo(): Locator { return this.page.getByTestId('st-view-to'); }
  get viewToDeleted(): Locator { return this.page.getByTestId('st-view-to-deleted'); }

  /** Бейдж «заказ удалён» в строке списка и в шапке документа (soft-delete-transfers §8.1). */
  rowTaskDeleted(id: number): Locator { return this.row(id).getByTestId('st-row-task-deleted'); }
  get taskDeletedBadge(): Locator { return this.page.getByTestId('st-task-deleted'); }
  /** Номер удалённого заказа — текстом, без ссылки. */
  get taskText(): Locator { return this.page.getByTestId('st-task-text'); }

  async goto(id?: number) {
    await this.page.goto(id ? `/transfers?id=${id}` : '/transfers');
    await expect(this.root).toBeVisible();
    if (id) await expect(this.cardTitle).toHaveText(`Перемещение #${id}`);
  }

  /** Форма нового документа с клиентом по ссылке из карточки заказа (§8.2). */
  async gotoNew(direction: Direction, taskId: number) {
    await this.page.goto(`/transfers?new=1&direction=${direction}&task_id=${taskId}`);
    await expect(this.root).toBeVisible();
    await expect(this.direction).toHaveText(DIRECTION_LABEL[direction]);
  }

  /** «Новое перемещение» → пункт меню направления → форма нового документа. */
  async newDocument(direction: Direction) {
    await this.newBtn.click();
    const item = this.page.locator(`[data-testid="st-new-direction"][data-direction="${direction}"]`);
    await expect(item).toBeVisible();
    await item.click();
    await expect(this.cardTitle).toHaveText('Новое перемещение');
    await expect(this.direction).toHaveText(DIRECTION_LABEL[direction]);
  }

  row(id: number): Locator {
    return this.list.locator(`[data-testid="st-row"][data-id="${id}"]`);
  }

  /** Комбобокс без поиска по вводу (короткие списки: статус, направление). */
  async pick(button: Locator, label: string) {
    await button.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-opt', { hasText: label }).first().click();
  }

  /** Единица оборудования заказа в «+ Оборудование» документа с клиентом. */
  taskEquipmentOption(equipmentId: string): Locator {
    return this.page.locator(`[data-testid="st-task-equipment-option"][data-equipment-id="${equipmentId}"]`);
  }

  /** Номер заказа нового документа с клиентом: ввод → debounce → название заказа под полем. */
  async fillTask(taskId: number, title: string) {
    await this.task.fill(String(taskId));
    await expect(this.taskInfo).toContainText(title);
  }

  /** Подтверждение приёмки документа с клиентом — диалог без полей. */
  async confirmReceive(text: string) {
    await this.receive.click();
    await expect(this.uiDialog.getByTestId('ui-dialog-text')).toHaveText(text);
    await this.uiDialog.getByTestId('ui-dialog-ok').click();
  }

  async choose(button: Locator, label: string) {
    await button.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-search').fill(label);
    await this.dropdown.locator('.cb-opt', { hasText: label }).first().click();
  }

  async chooseStatusFilter(label: string) {
    await this.filterStatus.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-opt', { hasText: label }).first().click();
  }

  /** Строка редактора заявки: товар — по id товара, единица — по id оборудования. */
  itemRow(kind: 'product' | 'equipment', key: string): Locator {
    return this.page.locator(`[data-testid="st-item-row"][data-kind="${kind}"][data-key="${key}"]`);
  }

  itemQty(productId: string): Locator {
    return this.itemRow('product', productId).getByTestId('st-item-qty');
  }

  /** Цена продажи товарной строки накладной (редактор заявки). */
  itemPrice(productId: string): Locator {
    return this.itemRow('product', productId).getByTestId('st-item-price');
  }

  itemPriceError(productId: string): Locator {
    return this.itemRow('product', productId).getByTestId('st-item-price-error');
  }

  /** «Сумма» товарной строки накладной (редактор заявки; пересчитывается при вводе). */
  itemTotal(productId: string): Locator {
    return this.itemRow('product', productId).getByTestId('st-item-total');
  }

  /** Сумма накладной в строке списка — только у «возврата клиенту» с товарами. */
  rowAmount(id: number): Locator {
    return this.row(id).getByTestId('st-row-amount');
  }

  itemError(kind: 'product' | 'equipment', key: string): Locator {
    return this.itemRow(kind, key).getByTestId('st-item-error');
  }

  /** «+ Товар»: комбобокс по каталогу → новая строка-товар с количеством. */
  async addProductRow(productName: string, productId: string, qty: number | string) {
    await this.choose(this.addProduct, productName);
    await expect(this.itemRow('product', productId)).toBeVisible();
    await this.itemQty(productId).fill(String(qty));
  }

  equipmentOption(equipmentId: string): Locator {
    return this.page.locator(`[data-testid="st-equipment-option"][data-equipment-id="${equipmentId}"]`);
  }

  /** «+ Оборудование»: поиск по вводу (debounce) → клик по единице → новая строка-единица. */
  async addEquipmentRow(query: string, equipmentId: string) {
    await this.equipmentSearch.fill(query);
    await expect(this.equipmentOption(equipmentId)).toBeVisible();
    await this.equipmentOption(equipmentId).click();
    await expect(this.itemRow('equipment', equipmentId)).toBeVisible();
  }

  /** Строка документа в режиме просмотра (в пути / получено / отменён). */
  viewItem(kind: 'product' | 'equipment', key: string): Locator {
    return this.page.locator(`[data-testid="st-view-item"][data-kind="${kind}"][data-key="${key}"]`);
  }

  /** Поле «Факт» товарной строки в модалке «Приёмка» (по id строки документа). */
  receiveQty(itemId: string): Locator {
    return this.page.locator(`[data-testid="st-receive-qty"][data-item-id="${itemId}"]`);
  }

  receiveQtyError(itemId: string): Locator {
    return this.page.locator(`[data-testid="st-receive-row"][data-item-id="${itemId}"]`).getByTestId('st-receive-qty-error');
  }

  /** Номер перемещения из заголовка документа «Перемещение #N». */
  async currentId(): Promise<number> {
    // После сохранения заголовок «Новое перемещение» сменяется на «Перемещение #N» не мгновенно.
    await expect(this.cardTitle).toHaveText(/#\d+/);
    const text = (await this.cardTitle.textContent()) ?? '';
    const m = /#(\d+)/.exec(text);
    expect(m, `номер в заголовке «${text}»`).toBeTruthy();
    return Number(m![1]);
  }

  /** TS-045: у документа нет горизонтальной прокрутки. */
  async expectNoHorizontalScroll() {
    const overflow = await this.page.evaluate(() => {
      const el = document.documentElement;
      return { scrollWidth: el.scrollWidth, clientWidth: el.clientWidth };
    });
    expect(overflow.scrollWidth, 'горизонтальная прокрутка страницы').toBeLessThanOrEqual(overflow.clientWidth);
  }
}
