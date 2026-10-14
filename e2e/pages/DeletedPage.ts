import { expect, type Locator, type Page } from '@playwright/test';
import type { DeletedKind } from '../fixtures/board-types';

// Экран «Удалённые» «/deleted» (soft-delete-core §8.1): вкладки по видам записей со счётчиками,
// фильтры (поиск, период, кто удалил), таблица, «Восстановить», жёлтый блок предупреждений,
// пагинация. Разметка — web/ssr/softdelete/templates/pages/list.html + js/deleted.js.
export class DeletedPage {
  readonly root: Locator;
  readonly tabs: Locator;
  readonly filterQ: Locator;
  readonly filterQError: Locator;
  readonly filterFrom: Locator;
  readonly filterTo: Locator;
  readonly filterPeriodError: Locator;
  readonly filterBy: Locator;
  readonly filterReset: Locator;
  readonly table: Locator;
  readonly rows: Locator;
  readonly empty: Locator;
  readonly pager: Locator;
  readonly prev: Locator;
  readonly next: Locator;
  readonly shown: Locator;
  readonly warnings: Locator;
  readonly warningLines: Locator;
  readonly warningOpen: Locator;
  readonly warningsClose: Locator;
  readonly toast: Locator;
  readonly toastError: Locator;
  readonly dialog: Locator;
  readonly dialogText: Locator;
  readonly dialogOk: Locator;
  readonly dialogCancel: Locator;

  constructor(readonly page: Page) {
    this.root = page.getByTestId('deleted-root');
    this.tabs = page.getByTestId('deleted-tabs');
    this.filterQ = page.getByTestId('deleted-filter-q');
    this.filterQError = page.getByTestId('deleted-filter-q-error');
    this.filterFrom = page.getByTestId('deleted-filter-from');
    this.filterTo = page.getByTestId('deleted-filter-to');
    this.filterPeriodError = page.getByTestId('deleted-filter-period-error');
    this.filterBy = page.getByTestId('deleted-filter-by');
    this.filterReset = page.getByTestId('deleted-filter-reset');
    this.table = page.getByTestId('deleted-table');
    this.rows = page.getByTestId('deleted-row');
    this.empty = page.getByTestId('deleted-empty');
    this.pager = page.getByTestId('deleted-pager');
    this.prev = page.getByTestId('deleted-prev');
    this.next = page.getByTestId('deleted-next');
    this.shown = page.getByTestId('deleted-shown');
    this.warnings = page.getByTestId('deleted-warnings');
    this.warningLines = page.getByTestId('deleted-warning-line');
    this.warningOpen = page.getByTestId('deleted-warning-open');
    this.warningsClose = page.getByTestId('deleted-warnings-close');
    this.toast = page.getByTestId('toast');
    this.toastError = page.getByTestId('toast-error');
    this.dialog = page.getByTestId('ui-dialog');
    this.dialogText = page.getByTestId('ui-dialog-text');
    this.dialogOk = page.getByTestId('ui-dialog-ok');
    this.dialogCancel = page.getByTestId('ui-dialog-cancel');
  }

  /** Открыть экран; query — без «?», напр. `kind=tasks&q=…`. Ждёт первой загрузки списка. */
  async goto(query = '') {
    await this.page.goto(`/deleted${query ? `?${query}` : ''}`);
    await expect(this.root).toBeVisible();
    await this.loaded();
  }

  /** Список загружен: видна таблица либо пустое состояние. Оба узла всегда есть в DOM (один
   *  скрыт d-none), поэтому ждём именно ВИДИМЫЙ: `.or().first()` брал бы первый по порядку в
   *  DOM — таблицу, скрытую при пустом списке. */
  async loaded() {
    await expect(this.page.locator('[data-testid="deleted-table"]:visible, [data-testid="deleted-empty"]:visible').first()).toBeVisible();
  }

  tab(kind: DeletedKind): Locator {
    return this.page.getByTestId(`deleted-tab-${kind}`);
  }

  tabCount(kind: DeletedKind): Locator {
    return this.page.getByTestId(`deleted-tab-count-${kind}`);
  }

  row(id: number | string): Locator {
    return this.page.locator(`[data-testid="deleted-row"][data-id="${id}"]`);
  }

  rowTitle(id: number | string): Locator { return this.row(id).getByTestId('deleted-row-title'); }
  rowSubtitle(id: number | string): Locator { return this.row(id).getByTestId('deleted-row-subtitle'); }
  /** Бейдж «вместе с клиентом» у единицы, ушедшей в удалённые с клиентом (soft-delete-clients §8.2). */
  rowWith(id: number | string): Locator { return this.row(id).getByTestId('deleted-row-with'); }
  /** Бейдж «в архиве» у записи, удалённой из архива (soft-delete-catalog §8.2, DS-009). */
  rowArchived(id: number | string): Locator { return this.row(id).getByTestId('deleted-row-archived'); }
  rowReason(id: number | string): Locator { return this.row(id).getByTestId('deleted-row-reason'); }
  rowBy(id: number | string): Locator { return this.row(id).getByTestId('deleted-row-by'); }
  rowAt(id: number | string): Locator { return this.row(id).getByTestId('deleted-row-at'); }
  restoreBtn(id: number | string): Locator { return this.row(id).getByTestId('deleted-restore'); }

  /** «Восстановить» в строке → подтверждение → «Восстановить». */
  async restore(id: number | string) {
    await this.restoreBtn(id).click();
    await expect(this.dialog).toBeVisible();
    await this.dialogOk.click();
  }

  /** Поиск: ввод + ожидание debounce-запроса списка. */
  async search(text: string) {
    const loaded = this.page.waitForResponse((r) => r.url().includes('/api/v1/deleted?') && r.request().method() === 'GET');
    await this.filterQ.fill(text);
    await loaded;
  }

  async expectNoHorizontalScroll() {
    const overflow = await this.page.evaluate(() => {
      const el = document.documentElement;
      return { scrollWidth: el.scrollWidth, clientWidth: el.clientWidth };
    });
    expect(overflow.scrollWidth, 'горизонтальная прокрутка страницы').toBeLessThanOrEqual(overflow.clientWidth);
  }
}
