import { test, expect } from '../fixtures/board';
import { DeletedPage } from '../pages/DeletedPage';
import { SuppliersPage } from '../pages/SuppliersPage';

// Мягкое удаление — поставщики (docs/features/soft-delete-suppliers/draft-spec.md §8.1–§8.2, §11):
// кнопка «Удалить» в форме поставщика, удаление с причиной, вкладка «Поставщики» раздела
// «Удалённые», восстановление (TS-045). Адаптив и темы раздела «Удалённые» и окна удаления —
// SD-007 / SD-008 (soft-delete-core.spec.ts).
// Общая dev-БД (§TD-6.2): свои записи через uniq()+adminApi. Уборка (TS-F11): поставщики —
// autouse _cleanup (служебный адрес purge, стенд с TEST_PURGE_ENABLED=true).
test.use({ colorScheme: 'light' });

const DATE_TIME = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;
const TOAST_DELETED = 'Удалено. Восстановить можно в разделе «Удалённые»';
const SUPPLIER_DELETE_TEXT = 'Поставщика можно восстановить в разделе «Удалённые»';

test.describe('SDS: мягкое удаление поставщиков', () => {
  test('SDS-001 (TS-045): поставщик — «Удалить» в форме; без причины — «Укажите причину удаления»; с причиной → тост, поставщика нет в списке; «Удалённые» → «Поставщики»: контактное лицо, причина, кто, когда; «Восстановить» → «Восстановлено», поставщик снова в списке', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const name = uniq('SDS-001 поставщик');
    const supplier = await adminApi.createSupplier({ name, contact_name: 'Иван Петров' });
    // пустая причина — 400 по полю, окно остаётся (ожидаемо).
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/suppliers/${supplier.id}`) && status === 400);

    // ---- §8.1: «Удалить» внизу формы поставщика ----
    const sup = new SuppliersPage(page);
    await sup.goto();
    await sup.searchFor(name);
    await sup.openRow(name);
    await expect(sup.delete).toHaveText('Удалить');
    await sup.openDelete();
    await expect(sup.deleteDialogTitle).toHaveText(`Удалить «${name}»?`);
    await expect(sup.deleteDialogText).toHaveText(SUPPLIER_DELETE_TEXT);
    await sup.deleteDialogOk.click();
    await expect(sup.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(sup.deleteDialog).toBeVisible();

    const reason = uniq('SDS-001 не работаем');
    await sup.deleteReason.fill(reason);
    await sup.deleteDialogOk.click();
    await expect(sup.toast.last()).toHaveText(TOAST_DELETED);
    await expect(sup.form).toBeHidden();
    await expect(sup.row(name)).toHaveCount(0);
    await sup.showArchived(true);
    await expect(sup.row(name), 'удалённого поставщика нет и среди архивных').toHaveCount(0);

    // ---- §8.2: «Удалённые» → «Поставщики» ----
    const del = new DeletedPage(page);
    await del.goto(`kind=suppliers&q=${encodeURIComponent(name)}`);
    await expect(del.tab('suppliers')).toHaveClass(/active/);
    await expect(del.rows).toHaveCount(1);
    await expect(del.rowTitle(supplier.id)).toHaveText(name);
    await expect(del.rowSubtitle(supplier.id)).toHaveText('Иван Петров');
    await expect(del.rowReason(supplier.id)).toHaveText(reason);
    await expect(del.rowBy(supplier.id)).toHaveText('Админ Прогона');
    await expect(del.rowAt(supplier.id)).toHaveText(DATE_TIME);
    const summary = await adminApi.deletedSummary();
    await expect(del.tabCount('suppliers')).toHaveText(String(summary.counts.suppliers));

    await del.restoreBtn(supplier.id).click();
    await expect(del.dialogText).toHaveText(`Восстановить «${name}»?`);
    await del.dialogOk.click();
    await expect(del.toast).toHaveText('Восстановлено');
    await expect(del.row(supplier.id)).toHaveCount(0);
    await expect(del.tabCount('suppliers')).toHaveText(String(summary.counts.suppliers - 1));

    // поставщик снова в списке.
    await sup.goto();
    await sup.searchFor(name);
    await expect(sup.row(name)).toBeVisible();
  });
});
