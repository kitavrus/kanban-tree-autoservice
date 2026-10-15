import { expect, type Locator, type Page } from '@playwright/test';
import { NewOrderDialog } from './NewOrderDialog';

// POM карточки заказа (фаза 6, testid-контракт js/task-card.js — KT.taskCard). Разделяет
// шапку (переиспользует testid простой формы фазы 5 — task-title/task-client/task-column/
// task-assignee/task-start/task-duration/task-paid/task-save/task-delete/task-error) и новые разделы
// автомобиля/услуг/товаров/журнала/итогов/печати (префикс card-*). Автомобиль заказа
// (one-car-order) — ровно один, в карточке только читается: «госномер · владелец · История». Комбобоксы —
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
  readonly taskStart: Locator;
  readonly taskDuration: Locator;
  readonly plannedEnd: Locator;
  readonly taskPaid: Locator;
  readonly description: Locator;
  readonly saveBtn: Locator;
  readonly deleteBtn: Locator;

  readonly printIntake: Locator;
  readonly printHandoverPdf: Locator;

  /** Блок «Автомобиль» существующего заказа (у нового заказа скрыт). */
  readonly carSection: Locator;
  readonly carRow: Locator;
  readonly carPlate: Locator;
  readonly carOwner: Locator;
  readonly carDeleted: Locator;
  readonly carHistoryToggle: Locator;
  readonly carHistoryPanel: Locator;
  /** Подсказка «Владелец автомобиля» под полем «Клиент» (поле только для чтения). */
  readonly clientLockedHint: Locator;

  readonly subtasks: Locator;
  readonly subtasksLocked: Locator;
  readonly subtaskRows: Locator;
  readonly subtaskAdd: Locator;

  readonly total: Locator;

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

  /** Диалог «Новый заказ»: госномер, найденный/новый автомобиль, новый клиент. */
  readonly newOrder: NewOrderDialog;

  constructor(readonly page: Page) {
    this.newOrder = new NewOrderDialog(page);
    this.modal = page.getByTestId('task-form');
    this.title = page.locator('#task-modal-title');
    this.error = page.getByTestId('task-error');

    this.taskTitle = page.getByTestId('task-title');
    this.taskClient = page.getByTestId('task-client');
    this.taskColumn = page.getByTestId('task-column');
    this.taskAssignee = page.getByTestId('task-assignee');
    // План работ (order-start-duration): «Начать с», «Дней на работу», подсказка окончания.
    this.taskStart = page.getByTestId('task-start');
    this.taskDuration = page.getByTestId('task-duration');
    this.plannedEnd = page.getByTestId('task-planned-end');
    this.taskPaid = page.getByTestId('task-paid');
    this.description = page.getByTestId('card-description');
    this.saveBtn = page.getByTestId('task-save');
    this.deleteBtn = page.getByTestId('task-delete');

    this.printIntake = page.getByTestId('card-print-intake');
    this.printHandoverPdf = page.getByTestId('card-print-handover-pdf');

    this.carSection = page.getByTestId('card-car-section');
    this.carRow = page.getByTestId('card-car-row');
    this.carPlate = this.carRow.getByTestId('card-car-plate');
    this.carOwner = this.carRow.getByTestId('card-car-owner');
    this.carDeleted = this.carRow.getByTestId('card-car-deleted');
    this.carHistoryToggle = this.carRow.getByTestId('card-car-history-toggle');
    this.carHistoryPanel = this.carRow.getByTestId('card-car-history-panel');
    this.clientLockedHint = page.getByTestId('task-client-locked');

    this.subtasks = page.getByTestId('card-subtasks');
    this.subtasksLocked = page.getByTestId('card-subtasks-locked');
    this.subtaskRows = page.getByTestId('card-subtask-row');
    this.subtaskAdd = page.getByTestId('card-subtask-add');

    this.total = page.getByTestId('card-total');

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

  /** Выбор в remote-search комбобоксе (услуги/товары) — печатает
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

  async fillHeader(t: { title?: string; client?: string; column?: string; assignee?: string; start?: string; days?: number; description?: string }) {
    if (t.title !== undefined) await this.taskTitle.fill(t.title);
    if (t.client !== undefined) await this.chooseCombobox(this.taskClient, t.client);
    if (t.column !== undefined) await this.chooseCombobox(this.taskColumn, t.column);
    if (t.assignee !== undefined) await this.chooseCombobox(this.taskAssignee, t.assignee);
    if (t.start !== undefined) await this.taskStart.fill(t.start);
    if (t.days !== undefined) await this.taskDuration.fill(String(t.days));
    if (t.description !== undefined) await this.description.fill(t.description);
  }

  async save() {
    await this.saveBtn.click();
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

  /** Выбора автомобиля в услуге нет (one-car-order) — локатор для проверки отсутствия. */
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

  /** Строка автомобиля заказа с этим equipment_id (видна — значит это автомобиль заказа). */
  carRowOf(equipmentId: string): Locator {
    return this.page.locator(`[data-testid="card-car-row"][data-equipment-id="${equipmentId}"]`);
  }

  /** Заказ в раскрытой «Истории работ» автомобиля по номеру. */
  carHistoryOrder(taskId: number): Locator {
    return this.carHistoryPanel.getByTestId('card-car-history-order').filter({ hasText: `#${taskId}` });
  }

  /** Раскрыть «Историю работ» автомобиля и дождаться загрузки. */
  async openCarHistory() {
    await this.carHistoryToggle.click();
    await expect(this.carHistoryToggle).toHaveAttribute('aria-expanded', 'true');
    await expect(this.carHistoryPanel).not.toContainText('Загрузка…');
  }

  /** Модалка карточки заказа — без горизонтальной прокрутки (TS-045). */
  async expectNoHorizontalScroll() {
    const overflow = await this.modal.evaluate((el) => {
      const body = el.querySelector('.modal-body') ?? el;
      return { scrollWidth: body.scrollWidth, clientWidth: body.clientWidth, doc: document.documentElement.scrollWidth, win: document.documentElement.clientWidth };
    });
    expect(overflow.scrollWidth, 'горизонтальная прокрутка карточки заказа').toBeLessThanOrEqual(overflow.clientWidth);
    expect(overflow.doc, 'горизонтальная прокрутка страницы').toBeLessThanOrEqual(overflow.win);
  }

  /** Вложение по имени файла (имена в тестах уникальны через uniq()). */
  attachmentByName(name: string): Locator {
    return this.attachmentItems.filter({ has: this.page.getByTestId('card-attachment-name').getByText(name, { exact: true }) });
  }

  async uploadAttachment(file: { name: string; mimeType: string; buffer: Buffer }) {
    await this.attachmentInput.setInputFiles(file);
  }

  // ---- окно «Удаление» с причиной (soft-delete-core §8.2) ----
  get deleteDialog(): Locator { return this.page.getByTestId('ui-dialog'); }
  get deleteDialogTitle(): Locator { return this.page.getByTestId('delete-dialog-title'); }
  get deleteReason(): Locator { return this.page.getByTestId('delete-reason'); }
  get deleteReasonError(): Locator { return this.page.getByTestId('delete-reason-error'); }
  get deleteReasonCounter(): Locator { return this.page.getByTestId('delete-reason-counter'); }
  /** Красный блок над кнопками: отказ 409 с текстом сервера. */
  get deleteDialogError(): Locator { return this.page.getByTestId('delete-dialog-error'); }
  /** Вторая строка пояснения окна (ui-dialog-note). */
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
