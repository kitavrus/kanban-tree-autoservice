import { expect, type Locator, type Page } from '@playwright/test';
import { NewOrderDialog } from './NewOrderDialog';

// POM доски заказов «/» (design-go-layers.md §Frontend Modules testid-контракт,
// эталон структуры — e2e/pages/ClientsPage.ts). Комбобоксы (client/column/assignee)
// переносят data-testid на видимую кнопку (js/combobox.js) — выбор через клик по кнопке
// и затем по `.cb-opt` в `combobox-dropdown` (selectOption с исходным <select> не работает).
export class BoardPage {
  readonly root: Locator;
  readonly filters: Locator;
  readonly filterClient: Locator;
  readonly filterQ: Locator;
  readonly filterCategory: Locator;
  readonly filterPaid: Locator;
  readonly filterAssignee: Locator;
  readonly filterDue: Locator;
  readonly filterReset: Locator;
  readonly found: Locator;

  readonly taskAddBtn: Locator;
  readonly columnAddBtn: Locator;

  readonly taskForm: Locator;
  readonly taskTitle: Locator;
  readonly taskClient: Locator;
  readonly taskColumn: Locator;
  readonly taskAssignee: Locator;
  readonly taskDue: Locator;
  readonly taskPaid: Locator;
  readonly taskSave: Locator;
  readonly taskDelete: Locator;
  readonly taskError: Locator;

  readonly columnForm: Locator;
  readonly columnName: Locator;
  readonly columnColor: Locator;
  readonly columnPosition: Locator;
  readonly columnFinal: Locator;
  readonly columnConsumes: Locator;
  readonly columnSave: Locator;
  readonly columnDelete: Locator;
  readonly columnError: Locator;

  readonly toastWarnings: Locator;
  readonly dropdown: Locator;

  /** Диалог «Новый заказ»: госномер, найденный/новый автомобиль, новый клиент. */
  readonly newOrder: NewOrderDialog;

  constructor(readonly page: Page) {
    this.newOrder = new NewOrderDialog(page);
    this.root = page.getByTestId('board-root');
    this.filters = page.getByTestId('board-filters');
    this.filterClient = page.getByTestId('filter-client');
    this.filterQ = page.getByTestId('filter-q');
    this.filterCategory = page.getByTestId('filter-category');
    this.filterPaid = page.getByTestId('filter-paid');
    this.filterAssignee = page.getByTestId('filter-assignee');
    this.filterDue = page.getByTestId('filter-due');
    this.filterReset = page.getByTestId('filter-reset');
    this.found = page.getByTestId('board-total'); // pagination DS-004: счётчик «Заказов: M»

    this.taskAddBtn = page.getByTestId('task-add');
    this.columnAddBtn = page.getByTestId('column-add');

    this.taskForm = page.getByTestId('task-form');
    this.taskTitle = page.getByTestId('task-title');
    this.taskClient = page.getByTestId('task-client');
    this.taskColumn = page.getByTestId('task-column');
    this.taskAssignee = page.getByTestId('task-assignee');
    this.taskDue = page.getByTestId('task-due');
    this.taskPaid = page.getByTestId('task-paid');
    this.taskSave = page.getByTestId('task-save');
    this.taskDelete = page.getByTestId('task-delete');
    this.taskError = page.getByTestId('task-error');

    this.columnForm = page.getByTestId('column-form');
    this.columnName = page.getByTestId('column-name');
    this.columnColor = page.getByTestId('column-color');
    this.columnPosition = page.getByTestId('column-position');
    this.columnFinal = page.getByTestId('column-final');
    this.columnConsumes = page.getByTestId('column-consumes');
    this.columnSave = page.getByTestId('column-save');
    this.columnDelete = page.getByTestId('column-delete');
    this.columnError = page.getByTestId('column-error');

    this.toastWarnings = page.getByTestId('toast-warnings');
    this.dropdown = page.getByTestId('combobox-dropdown');
  }

  async goto(query = '') {
    await this.page.goto(`/${query}`);
    await expect(this.root).toBeVisible();
    // ждём реального рендера колонок — не пустого/loading-состояния (данные грузит JS).
    await expect(this.page.locator('[data-testid^="board-column-"]').first()).toBeVisible();
  }

  column(id: string): Locator {
    return this.page.getByTestId(`board-column-${id}`);
  }

  /** Колонка по названию — сид D32/новые колонки не имеют предсказуемого id заранее. */
  columnByName(name: string): Locator {
    return this.page.locator('[data-testid^="board-column-"]').filter({ has: this.page.locator('.name', { hasText: name }) });
  }

  async columnIdByName(name: string): Promise<string> {
    const testid = await this.columnByName(name).getAttribute('data-testid');
    if (!testid) throw new Error(`колонка «${name}» не найдена на доске`);
    return testid.replace('board-column-', '');
  }

  columnBody(id: string): Locator {
    return this.page.getByTestId(`column-body-${id}`);
  }

  /** pagination DS-001: строка «показаны N из M» внизу колонки. */
  columnShown(id: string): Locator {
    return this.page.getByTestId(`column-shown-${id}`);
  }

  /** pagination DS-004: счётчик M в шапке колонки (заказов колонки под фильтрами). */
  columnCount(id: string): Locator {
    return this.page.getByTestId(`column-count-${id}`);
  }

  /** pagination DS-001: кнопка «Показать ещё» внизу колонки. */
  columnMore(id: string): Locator {
    return this.page.getByTestId(`column-more-${id}`);
  }

  /** Метка «к выплате: N» на карточке доски (executor-payouts D68). */
  executorUnpaidBadge(id: number | string): Locator {
    return this.card(id).getByTestId('task-card-executor-unpaid');
  }

  /** Фильтр «Выплата исполнителю» (executor-payouts D67; комбобокс — testid на кнопке). */
  get filterExecutorPaid(): Locator {
    return this.page.getByTestId('filter-executor-paid');
  }

  card(id: number | string): Locator {
    return this.page.getByTestId(`task-card-${id}`);
  }

  cardMenu(id: number | string): Locator {
    return this.page.getByTestId(`task-card-menu-${id}`);
  }

  moveToOption(id: number | string, columnId: string): Locator {
    return this.page.getByTestId(`task-move-to-${columnId}`);
  }

  columnMenu(id: string): Locator {
    return this.page.getByTestId(`column-menu-${id}`);
  }

  columnEditLink(id: string): Locator {
    return this.page.getByTestId(`column-edit-${id}`);
  }

  columnDeleteLink(id: string): Locator {
    return this.page.getByTestId(`column-delete-${id}`);
  }

  columnAddTaskBtn(id: string): Locator {
    return this.page.getByTestId(`column-add-task-${id}`);
  }

  /** Переносит карточку через меню «⋯ → Переместить в…» (ADR-106, работает и на мобильном). */
  async moveViaMenu(taskId: number | string, targetColumnId: string) {
    await this.cardMenu(taskId).click();
    await this.moveToOption(taskId, targetColumnId).click();
  }

  /** Нативный HTML5 DnD — перетаскивает карточку в конец колонки.
   *
   * `locator.dragTo()` (реальные мышиные события) в этом проекте ненадёжен для колонок,
   * требующих горизонтальной прокрутки `.board-columns` (5-я+ колонка, особенно когда на
   * общей dev-БД накопились колонки от прошлых прогонов) — drop иногда не долетает до
   * цели. Вместо этого — ручной синтетический dispatch dragstart/dragover/drop с общим
   * DataTransfer (board.js слушает именно эти события, реальная мышь не нужна); board.js
   * сам вычисляет prev_id/next_id по clientY внутри dragover, поэтому координаты в evaluate
   * должны соответствовать логике «после последней карточки» / «перед target».
   */
  async dragCardToColumn(taskId: number | string, targetColumnId: string) {
    await this.dispatchDrag(`[data-testid="task-card-${taskId}"]`, `[data-testid="column-body-${targetColumnId}"]`, 'end');
  }

  /** Перетаскивает карточку перед другой карточкой той же/другой колонки. */
  async dragCardBefore(taskId: number | string, beforeTaskId: number | string) {
    await this.dispatchDrag(`[data-testid="task-card-${taskId}"]`, `[data-testid="task-card-${beforeTaskId}"]`, 'before');
  }

  private async dispatchDrag(cardSelector: string, targetSelector: string, position: 'end' | 'before') {
    await this.page.evaluate(
      ([cardSel, targetSel, pos]) => {
        const card = document.querySelector(cardSel) as HTMLElement | null;
        const target = document.querySelector(targetSel) as HTMLElement | null;
        if (!card || !target) throw new Error(`drag: не найден card=${cardSel} target=${targetSel}`);
        card.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        target.scrollIntoView({ block: 'nearest', inline: 'nearest' });

        const dt = new DataTransfer();
        let x: number;
        let y: number;
        let dropEl: Element;
        if (pos === 'end') {
          const r = target.getBoundingClientRect();
          x = r.left + r.width / 2;
          y = r.bottom - 4; // ниже середины любой карточки → вставка в конец
          dropEl = target;
        } else {
          const r = target.getBoundingClientRect();
          x = r.left + r.width / 2;
          y = r.top + 2; // выше середины target-карточки → вставка перед ней
          dropEl = target.closest('[data-testid^="column-body-"]') ?? (target.parentElement as Element);
        }
        const cardRect = card.getBoundingClientRect();

        function fire(el: Element, type: string, clientX: number, clientY: number) {
          const ev = new DragEvent(type, { bubbles: true, cancelable: true, clientX, clientY });
          Object.defineProperty(ev, 'dataTransfer', { value: dt });
          el.dispatchEvent(ev);
        }
        fire(card, 'dragstart', cardRect.left + cardRect.width / 2, cardRect.top + cardRect.height / 2);
        fire(dropEl, 'dragover', x, y);
        fire(dropEl, 'drop', x, y);
        fire(card, 'dragend', x, y);
      },
      [cardSelector, targetSelector, position] as const,
    );
  }

  /** Выбор опции в комбобоксе по видимому тексту (js/combobox.js — testid уходит на кнопку). */
  async chooseCombobox(button: Locator, optionText: string) {
    await button.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-opt', { hasText: optionText }).first().click();
  }

  async openTaskAdd() {
    await this.taskAddBtn.click();
    await expect(this.taskForm).toBeVisible();
  }

  async openTaskEdit(taskId: number | string) {
    await this.card(taskId).click();
    await expect(this.taskForm).toBeVisible();
  }

  async openColumnAdd() {
    await this.columnAddBtn.click();
    await expect(this.columnForm).toBeVisible();
  }

  async openColumnEdit(columnId: string) {
    await this.columnMenu(columnId).click();
    await this.columnEditLink(columnId).click();
    await expect(this.columnForm).toBeVisible();
  }

  async fillTask(t: { title?: string; client?: string; column?: string; assignee?: string; due?: string }) {
    if (t.title !== undefined) await this.taskTitle.fill(t.title);
    if (t.client !== undefined) await this.chooseCombobox(this.taskClient, t.client);
    if (t.column !== undefined) await this.chooseCombobox(this.taskColumn, t.column);
    if (t.assignee !== undefined) await this.chooseCombobox(this.taskAssignee, t.assignee);
    if (t.due !== undefined) await this.taskDue.fill(t.due);
  }

  async fillColumn(c: { name?: string; color?: string; position?: number; isFinal?: boolean; consumes?: boolean }) {
    if (c.name !== undefined) await this.columnName.fill(c.name);
    if (c.color !== undefined) await this.columnColor.fill(c.color);
    if (c.position !== undefined) await this.columnPosition.fill(String(c.position));
    if (c.isFinal !== undefined) {
      if ((await this.columnFinal.isChecked()) !== c.isFinal) await this.columnFinal.click();
    }
    if (c.consumes !== undefined) {
      if ((await this.columnConsumes.isChecked()) !== c.consumes) await this.columnConsumes.click();
    }
  }

  // ---- окно «Удаление» колонки (soft-delete-board-columns §8.1; общее KT.uiDeleteDialog) ----
  get deleteDialog(): Locator { return this.page.locator('[data-testid="ui-dialog"][data-dialog-kind="delete"]'); }
  get deleteDialogTitle(): Locator { return this.page.getByTestId('delete-dialog-title'); }
  get deleteDialogText(): Locator { return this.deleteDialog.getByTestId('ui-dialog-text'); }
  get deleteReason(): Locator { return this.page.getByTestId('delete-reason'); }
  get deleteReasonError(): Locator { return this.page.getByTestId('delete-reason-error'); }
  get deleteDialogError(): Locator { return this.page.getByTestId('delete-dialog-error'); }
  get deleteDialogOk(): Locator { return this.deleteDialog.getByTestId('ui-dialog-ok'); }
  get deleteDialogCancel(): Locator { return this.deleteDialog.getByTestId('ui-dialog-cancel'); }
  get toast(): Locator { return this.page.getByTestId('toast'); }
  get toastError(): Locator { return this.page.getByTestId('toast-error'); }
  /** Колонки доски в порядке отображения (слева направо). */
  get columns(): Locator { return this.page.locator('[data-testid^="board-column-"]'); }

  /** Меню колонки → «Удалить» → открыто окно «Удаление» с причиной. */
  async openDeleteColumn(columnId: string) {
    await this.columnMenu(columnId).click();
    await this.columnDeleteLink(columnId).click();
    await expect(this.deleteDialog).toBeVisible();
  }

  /** «Удалить» заказ → окно «Удаление» с обязательной причиной (soft-delete-core §8.2) → «Удалить». */
  async confirmDeleteTask(reason = 'e2e: создан по ошибке') {
    await this.taskDelete.click();
    await expect(this.page.getByTestId('ui-dialog')).toBeVisible();
    await this.page.getByTestId('delete-reason').fill(reason);
    await this.page.getByTestId('ui-dialog-ok').click();
  }
}
