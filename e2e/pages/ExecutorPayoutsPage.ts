import { expect, type Locator, type Page } from '@playwright/test';

// POM экрана-отчёта «Выплаты» «/executor-payouts» (executor-payouts D69/D70,
// js/executor-payouts.js). Селекты фильтров превращаются js/combobox.js в комбобоксы:
// testid уходит на видимую кнопку, выбор — клик по кнопке и по `.cb-opt` в
// `combobox-dropdown` (эталон — StockPage).
export class ExecutorPayoutsPage {
  readonly root: Locator;
  readonly filterAssignee: Locator;
  readonly filterStatus: Locator;
  readonly filterOnlyDone: Locator;
  readonly selectedCount: Locator;
  readonly markPaid: Locator;
  readonly markUnpaid: Locator;
  readonly error: Locator;
  /** Подсказка «нечего менять» (выбраны только услуги, уже стоящие в нужном состоянии). */
  readonly info: Locator;
  readonly totalToPay: Locator;
  readonly totalPaid: Locator;
  readonly groups: Locator;
  readonly rows: Locator;
  readonly empty: Locator;
  readonly dropdown: Locator;

  constructor(readonly page: Page) {
    this.root = page.getByTestId('ep-root');
    this.filterAssignee = page.getByTestId('ep-filter-assignee');
    this.filterStatus = page.getByTestId('ep-filter-status');
    this.filterOnlyDone = page.getByTestId('ep-filter-only-done');
    this.selectedCount = page.getByTestId('ep-selected-count');
    this.markPaid = page.getByTestId('ep-mark-paid');
    this.markUnpaid = page.getByTestId('ep-mark-unpaid');
    this.error = page.getByTestId('ep-error');
    this.info = page.getByTestId('ep-info');
    this.totalToPay = page.getByTestId('ep-total-to-pay');
    this.totalPaid = page.getByTestId('ep-total-paid');
    this.groups = page.getByTestId('ep-group');
    this.rows = page.getByTestId('ep-row');
    this.empty = page.getByTestId('ep-empty');
    this.dropdown = page.getByTestId('combobox-dropdown');
  }

  /** query — без «?», напр. `assignee=<uuid>&status=unpaid`. Ждёт конца первой загрузки. */
  async goto(query = '') {
    const loaded = this.page.waitForResponse((r) => r.url().includes('/api/v1/executor-payouts/summary') && r.status() === 200);
    await this.page.goto(query ? `/executor-payouts?${query}` : '/executor-payouts');
    await expect(this.root).toBeVisible();
    await loaded;
  }

  /** Выбор опции по ТОЧНОМУ тексту: «Выплачено» — подстрока «Не выплачено» (hasText
   *  строкой нечувствителен к регистру и выбрал бы первую из них). */
  async choose(button: Locator, label: string) {
    await button.click();
    await expect(this.dropdown).toBeVisible();
    const exact = new RegExp(`^\\s*${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*$`);
    await this.dropdown.locator('.cb-opt', { hasText: exact }).first().click();
  }

  group(assigneeId: string): Locator {
    return this.page.locator(`[data-testid="ep-group"][data-assignee-id="${assigneeId}"]`);
  }

  groupName(assigneeId: string): Locator {
    return this.group(assigneeId).getByTestId('ep-group-name');
  }

  groupToPay(assigneeId: string): Locator {
    return this.group(assigneeId).getByTestId('ep-group-to-pay');
  }

  groupPaid(assigneeId: string): Locator {
    return this.group(assigneeId).getByTestId('ep-group-paid');
  }

  groupCheck(assigneeId: string): Locator {
    return this.group(assigneeId).getByTestId('ep-group-check');
  }

  groupRows(assigneeId: string): Locator {
    return this.group(assigneeId).getByTestId('ep-row');
  }

  row(subtaskId: string): Locator {
    return this.page.locator(`[data-testid="ep-row"][data-subtask-id="${subtaskId}"]`);
  }

  rowCheck(subtaskId: string): Locator {
    return this.row(subtaskId).getByTestId('ep-row-check');
  }

  rowPaid(subtaskId: string): Locator {
    return this.row(subtaskId).getByTestId('ep-row-paid');
  }

  rowPayout(subtaskId: string): Locator {
    return this.row(subtaskId).getByTestId('ep-row-payout');
  }

  /** Инлайн-поле суммы выплаты строки (D72): сохранение по Enter/blur, пусто — «не задана». */
  rowPayoutInput(subtaskId: string): Locator {
    return this.row(subtaskId).getByTestId('ep-row-payout-input');
  }
}
