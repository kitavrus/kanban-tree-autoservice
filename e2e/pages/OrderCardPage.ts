import { expect, type Locator, type Page } from '@playwright/test';

// POM карточки заказа (фаза 6, testid-контракт js/task-card.js — KT.taskCard). Разделяет
// шапку (переиспользует testid простой формы фазы 5 — task-title/task-client/task-column/
// task-assignee/task-due/task-paid/task-save/task-delete/task-error) и новые разделы
// оборудования/услуг/товаров/журнала/итогов/печати (префикс card-*). Комбобоксы —
// js/combobox.js переносит testid на видимую кнопку, выбор — клик по кнопке и затем по
// `.cb-opt` в `combobox-dropdown` (эталон — BoardPage.chooseCombobox).
export class OrderCardPage {
  readonly modal: Locator;
  readonly title: Locator;
  readonly error: Locator;

  readonly taskTitle: Locator;
  readonly taskClient: Locator;
  readonly taskColumn: Locator;
  readonly taskAssignee: Locator;
  readonly taskDue: Locator;
  readonly taskPaid: Locator;
  readonly description: Locator;
  readonly saveBtn: Locator;
  readonly deleteBtn: Locator;

  readonly printIntake: Locator;
  readonly printHandover: Locator;

  readonly equipment: Locator;
  readonly equipmentLocked: Locator;
  readonly equipmentRows: Locator;
  readonly equipmentAdd: Locator;
  readonly equipmentNewBtn: Locator;
  readonly equipmentNewType: Locator;
  readonly equipmentNewBrand: Locator;
  readonly equipmentNewModel: Locator;
  readonly equipmentNewSerial: Locator;
  readonly equipmentNewSave: Locator;
  readonly equipmentNewCancel: Locator;
  readonly equipmentNewError: Locator;
  /** Блок «Перемещения заказа» (transfers-partners §8.2). */
  readonly transfers: Locator;
  readonly transfersLocked: Locator;
  readonly transfersEmpty: Locator;
  readonly transferRows: Locator;
  /** «Принять от клиента» — ссылка на форму нового документа либо неактивная кнопка. */
  readonly transferIntake: Locator;
  readonly transferIntakeHint: Locator;
  /** «Вернуть клиенту». */
  readonly transferReturn: Locator;
  readonly transferReturnHint: Locator;
  /** Упразднённый select «Склад приёма» (DS-045) — в форме добавления единицы его нет. */
  readonly equipmentWarehouse: Locator;

  readonly subtasks: Locator;
  readonly subtasksLocked: Locator;
  readonly subtaskRows: Locator;
  readonly subtaskAdd: Locator;

  readonly total: Locator;
  /** «Продано по накладным» над итогом — есть, пока сумма больше нуля (transfers-sales §8.2). */
  readonly transfersAmount: Locator;

  // Вложения (фаза 7, D21)
  readonly attachments: Locator;
  readonly attachmentsLocked: Locator;
  readonly attachmentsEmpty: Locator;
  readonly attachmentItems: Locator;
  readonly attachmentUpload: Locator;
  readonly attachmentInput: Locator;
  readonly attachmentProgress: Locator;
  readonly attachmentError: Locator;

  readonly dropdown: Locator;

  constructor(readonly page: Page) {
    this.modal = page.getByTestId('task-form');
    this.title = page.locator('#task-modal-title');
    this.error = page.getByTestId('task-error');

    this.taskTitle = page.getByTestId('task-title');
    this.taskClient = page.getByTestId('task-client');
    this.taskColumn = page.getByTestId('task-column');
    this.taskAssignee = page.getByTestId('task-assignee');
    this.taskDue = page.getByTestId('task-due');
    this.taskPaid = page.getByTestId('task-paid');
    this.description = page.getByTestId('card-description');
    this.saveBtn = page.getByTestId('task-save');
    this.deleteBtn = page.getByTestId('task-delete');

    this.printIntake = page.getByTestId('card-print-intake');
    this.printHandover = page.getByTestId('card-print-handover');

    this.equipment = page.getByTestId('card-equipment');
    this.equipmentLocked = page.getByTestId('card-equipment-locked');
    this.equipmentRows = page.getByTestId('card-equipment-row');
    this.equipmentAdd = page.getByTestId('card-equipment-add');
    this.equipmentNewBtn = page.getByTestId('card-equipment-new');
    this.equipmentNewType = page.getByTestId('card-equipment-new-type');
    this.equipmentNewBrand = page.getByTestId('card-equipment-new-brand');
    this.equipmentNewModel = page.getByTestId('card-equipment-new-model');
    this.equipmentNewSerial = page.getByTestId('card-equipment-new-serial');
    this.equipmentNewSave = page.getByTestId('card-equipment-new-save');
    this.equipmentNewCancel = page.getByTestId('card-equipment-new-cancel');
    this.equipmentNewError = page.getByTestId('card-equipment-new-error');
    this.equipmentWarehouse = page.getByTestId('card-equipment-warehouse');
    this.transfers = page.getByTestId('card-transfers');
    this.transfersLocked = page.getByTestId('card-transfers-locked');
    this.transfersEmpty = page.getByTestId('card-transfers-empty');
    this.transferRows = page.getByTestId('card-transfer-row');
    this.transferIntake = page.getByTestId('card-transfer-intake');
    this.transferIntakeHint = page.getByTestId('card-transfer-intake-hint');
    this.transferReturn = page.getByTestId('card-transfer-return');
    this.transferReturnHint = page.getByTestId('card-transfer-return-hint');

    this.subtasks = page.getByTestId('card-subtasks');
    this.subtasksLocked = page.getByTestId('card-subtasks-locked');
    this.subtaskRows = page.getByTestId('card-subtask-row');
    this.subtaskAdd = page.getByTestId('card-subtask-add');

    this.total = page.getByTestId('card-total');
    this.transfersAmount = page.getByTestId('card-transfers-amount');

    this.attachments = page.getByTestId('card-attachments');
    this.attachmentsLocked = page.getByTestId('card-attachments-locked');
    this.attachmentsEmpty = page.getByTestId('card-attachments-empty');
    this.attachmentItems = page.getByTestId('card-attachment');
    this.attachmentUpload = page.getByTestId('card-attachment-upload');
    this.attachmentInput = page.getByTestId('card-attachment-input');
    this.attachmentProgress = page.getByTestId('card-attachment-progress');
    this.attachmentError = page.getByTestId('card-attachment-error');

    this.dropdown = page.getByTestId('combobox-dropdown');
  }

  async chooseCombobox(button: Locator, optionText: string) {
    await button.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-opt', { hasText: optionText }).first().click();
  }

  /** Выбор в remote-search комбобоксе (услуги/товары/оборудование клиента) — печатает
   *  запрос в поле поиска дропдауна, ждёт результат по тексту. */
  async chooseRemote(button: Locator, query: string, optionText: string) {
    await button.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-search').fill(query);
    await this.dropdown.locator('.cb-opt', { hasText: optionText }).first().click();
  }

  async waitOpen() {
    await expect(this.modal).toBeVisible();
  }

  async fillHeader(t: { title?: string; client?: string; column?: string; assignee?: string; due?: string; description?: string }) {
    if (t.title !== undefined) await this.taskTitle.fill(t.title);
    if (t.client !== undefined) await this.chooseCombobox(this.taskClient, t.client);
    if (t.column !== undefined) await this.chooseCombobox(this.taskColumn, t.column);
    if (t.assignee !== undefined) await this.chooseCombobox(this.taskAssignee, t.assignee);
    if (t.due !== undefined) await this.taskDue.fill(t.due);
    if (t.description !== undefined) await this.description.fill(t.description);
  }

  async save() {
    await this.saveBtn.click();
  }

  /** Бейдж «удалён» у склада строки товара услуги (soft-delete-warehouses-suppliers §8.3 п.3). */
  productWarehouseDeleted(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-product-warehouse-deleted');
  }

  subtaskRow(subtaskId: string): Locator {
    return this.page.locator(`[data-testid="card-subtask-row"][data-subtask-id="${subtaskId}"]`);
  }

  subtaskStatus(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-status');
  }

  subtaskAssignee(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-assignee');
  }

  subtaskDue(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-due');
  }

  subtaskPrice(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-price');
  }

  /** executor-payouts D62–D66: отметка «Выплачено исполнителю», дата отметки, сумма выплаты. */
  subtaskExecutorPaid(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-executor-paid');
  }

  subtaskExecutorPaidAt(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-executor-paid-at');
  }

  subtaskExecutorPayout(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-executor-payout');
  }

  subtaskPayoutWarning(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-payout-warning');
  }

  subtaskEquipment(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-equipment');
  }

  subtaskTotal(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-total');
  }

  subtaskDelete(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-subtask-delete');
  }

  subtaskProductAdd(subtaskId: string): Locator {
    return this.subtaskRow(subtaskId).getByTestId('card-product-add');
  }

  productRow(productRowId: string): Locator {
    return this.page.locator(`[data-testid="card-product-row"][data-product-id="${productRowId}"]`);
  }

  productQty(productRowId: string): Locator {
    return this.productRow(productRowId).getByTestId('card-product-qty');
  }

  productQtyError(productRowId: string): Locator {
    return this.productRow(productRowId).getByTestId('card-warning');
  }

  productPrice(productRowId: string): Locator {
    return this.productRow(productRowId).getByTestId('card-product-price');
  }

  productHint(productRowId: string): Locator {
    return this.productRow(productRowId).getByTestId('card-product-hint');
  }

  /** Склад строки товара (warehouses DS-005) — combobox-кнопка (combobox.js переносит testid). */
  productWarehouse(productRowId: string): Locator {
    return this.productRow(productRowId).getByTestId('card-product-warehouse');
  }

  productDelete(productRowId: string): Locator {
    return this.productRow(productRowId).getByTestId('card-product-delete');
  }

  logRow(logId: string): Locator {
    return this.page.locator(`[data-testid="card-log-row"][data-log-id="${logId}"]`);
  }

  logText(logId: string): Locator {
    return this.logRow(logId).getByTestId('card-log-text');
  }

  logEdit(logId: string): Locator {
    return this.logRow(logId).getByTestId('card-log-edit');
  }

  logDelete(logId: string): Locator {
    return this.logRow(logId).getByTestId('card-log-delete');
  }

  logNewInput(subtaskId: string): Locator {
    return this.page.locator(`[data-testid="card-log-new"][data-subtask-id="${subtaskId}"]`);
  }

  logAddBtn(subtaskId: string): Locator {
    return this.page.locator(`[data-testid="card-log-add"][data-subtask-id="${subtaskId}"]`);
  }

  equipmentRow(equipmentId: string): Locator {
    return this.page.locator(`[data-testid="card-equipment-row"][data-equipment-id="${equipmentId}"]`);
  }

  equipmentCompleteness(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).getByTestId('card-equipment-completeness');
  }

  equipmentCondition(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).getByTestId('card-equipment-condition');
  }

  equipmentDelete(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).getByTestId('card-equipment-remove');
  }

  equipmentHistory(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).getByTestId('card-equipment-history');
  }

  // ---- местоположение оборудования заказа (transfers-core DS-003, DS-008, §8.2) ----

  /** Бейдж местоположения: «у клиента» / «на складе X» / «в пути → Y». */
  equipmentLocation(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).getByTestId('card-equipment-location');
  }

  /** «перемещение #N» единицы в незавершённом документе: ссылка (admin) либо текст (executor). */
  equipmentTransfer(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).getByTestId('card-equipment-transfer');
  }

  /** Подпись под упразднённым select «Склад приёма» — её тоже нет. */
  get equipmentWarehouseHint(): Locator {
    return this.page.getByTestId('card-equipment-warehouse-hint');
  }

  /** Строка документа в блоке «Перемещения заказа». */
  transferRow(transferId: number): Locator {
    return this.page.locator(`[data-testid="card-transfer-row"][data-id="${transferId}"]`);
  }

  transferLink(transferId: number): Locator {
    return this.transferRow(transferId).getByTestId('card-transfer-link');
  }

  /** Сумма накладной в строке блока «Перемещения заказа» (только если больше нуля). */
  transferAmount(transferId: number): Locator {
    return this.transferRow(transferId).getByTestId('card-transfer-amount');
  }

  transferStatus(transferId: number): Locator {
    return this.transferRow(transferId).getByTestId('card-transfer-status');
  }

  /** Блок «Перемещения заказа» и модалка — без горизонтальной прокрутки (TS-045). */
  async expectNoHorizontalScroll() {
    const overflow = await this.modal.evaluate((el) => {
      const body = el.querySelector('.modal-body') ?? el;
      return { scrollWidth: body.scrollWidth, clientWidth: body.clientWidth, doc: document.documentElement.scrollWidth, win: document.documentElement.clientWidth };
    });
    expect(overflow.scrollWidth, 'горизонтальная прокрутка карточки заказа').toBeLessThanOrEqual(overflow.clientWidth);
    expect(overflow.doc, 'горизонтальная прокрутка страницы').toBeLessThanOrEqual(overflow.win);
  }

  /** Снятые D57-кнопки строки единицы (DS-003): «Отправить / Принять / Отменить отправку». */
  equipmentMoveButtons(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).locator(
      '[data-testid="card-equipment-send"], [data-testid="card-equipment-receive"], [data-testid="card-equipment-cancel-transit"]',
    );
  }

  /** Снятый журнал перемещений единицы в заказе (DS-029). */
  equipmentMovements(equipmentId: string): Locator {
    return this.equipmentRow(equipmentId).locator('[data-testid="card-equipment-movements"], [data-testid="card-equipment-movement"]');
  }

  /** Вложение по имени файла (имена в тестах уникальны через uniq()). */
  attachmentByName(name: string): Locator {
    return this.attachmentItems.filter({ has: this.page.getByTestId('card-attachment-name').getByText(name, { exact: true }) });
  }

  async uploadAttachment(file: { name: string; mimeType: string; buffer: Buffer }) {
    await this.attachmentInput.setInputFiles(file);
  }

  /** Ссылки на документы в отказе смены клиента заказа (409 TASK_CLIENT_LOCKED_BY_TRANSFERS). */
  get errorTransferLinks(): Locator {
    return this.error.getByTestId('task-error-transfer-link');
  }

  // ---- окно «Удаление» с причиной (soft-delete-core §8.2) ----
  get deleteDialog(): Locator { return this.page.getByTestId('ui-dialog'); }
  get deleteDialogTitle(): Locator { return this.page.getByTestId('delete-dialog-title'); }
  get deleteReason(): Locator { return this.page.getByTestId('delete-reason'); }
  get deleteReasonError(): Locator { return this.page.getByTestId('delete-reason-error'); }
  get deleteReasonCounter(): Locator { return this.page.getByTestId('delete-reason-counter'); }
  /** Красный блок над кнопками: отказ 409 с текстом сервера. */
  get deleteDialogError(): Locator { return this.page.getByTestId('delete-dialog-error'); }
  /** Вторая строка пояснения: у заказа есть документы перемещения (soft-delete-transfers §8.3). */
  get deleteDialogNote(): Locator { return this.page.getByTestId('ui-dialog-note'); }
  get deleteDialogOk(): Locator { return this.page.getByTestId('ui-dialog-ok'); }

  /** «Удалить» заказ → окно «Удаление» открыто (пояснение — ui-dialog-text). */
  async openDeleteTask(): Promise<Locator> {
    await this.deleteBtn.click();
    await expect(this.deleteDialog).toBeVisible();
    return this.page.getByTestId('ui-dialog-text');
  }

  /** Ввести причину в открытом окне «Удаление» и нажать «Удалить». */
  async submitDelete(reason: string) {
    await this.deleteReason.fill(reason);
    await this.deleteDialogOk.click();
  }

  async confirmDeleteTask(reason = 'e2e: создан по ошибке') {
    await this.openDeleteTask();
    await this.submitDelete(reason);
  }

  async confirmUiDialog() {
    await expect(this.page.getByTestId('ui-dialog')).toBeVisible();
    await this.page.getByTestId('ui-dialog-ok').click();
  }

  async cancelUiDialog() {
    await expect(this.page.getByTestId('ui-dialog')).toBeVisible();
    await this.page.getByTestId('ui-dialog-cancel').click();
  }

  async close() {
    await this.page.locator('#task-modal .btn-close').click();
  }
}
