import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';

// Фаза 6: печать актов из карточки заказа (design-tests.md OC-017/018, AC-015/AC-016).
// Содержимое актов (реквизиты FIRM_*, XSS-экранирование, 404 на чужом kind) уже покрыто
// backend-сьютом internal/features/board/print_http_live_test.go — здесь только UI: кнопки
// открывают акт в НОВОЙ вкладке правильным URL, страница печати остаётся светлой темой
// независимо от темы UI.
test.use({ colorScheme: 'light' });

async function setup(adminApi: import('../fixtures/auth').AdminApi, uniq: (s: string) => string, label: string) {
  const client = await adminApi.createClient({ company_name: uniq(`OC-print клиент ${label}`), phones: [{ phone: '9990000200', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  return { client, columns };
}

test.describe('OC: печать актов из карточки', () => {
  test('OC-017: «Акт приёма» открывает /tasks/N/print/intake в новой вкладке, светлая тема', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-017');
    const task = await adminApi.createTask({ title: uniq('OC-017 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await expect(card.printIntake).toHaveAttribute('target', '_blank');
    await expect(card.printIntake).toHaveAttribute('href', `/tasks/${task.id}/print/intake`);

    const [popup] = await Promise.all([page.context().waitForEvent('page'), card.printIntake.click()]);
    await popup.waitForLoadState();
    expect(popup.url()).toContain(`/tasks/${task.id}/print/intake`);
    await expect(popup.locator('body')).toContainText(`Акт приёма №${task.id}`);
    const bg = await popup.evaluate(() => getComputedStyle(document.body).backgroundColor);
    // светлая тема — фон НЕ тёмный (RGB-компоненты не все < 60, эталон печати print.css).
    const m = bg.match(/\d+/g);
    expect(m).not.toBeNull();
    const dark = m!.slice(0, 3).every((v) => Number(v) < 60);
    expect(dark).toBe(false);
    await popup.close();
  });

  test('OC-018: «Акт выдачи» открывает /tasks/N/print/handover в новой вкладке', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, columns } = await setup(adminApi, uniq, 'OC-018');
    const task = await adminApi.createTask({ title: uniq('OC-018 заказ'), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskEdit(task.id);
    await card.waitOpen();

    await expect(card.printHandover).toHaveAttribute('target', '_blank');
    await expect(card.printHandover).toHaveAttribute('href', `/tasks/${task.id}/print/handover`);

    const [popup] = await Promise.all([page.context().waitForEvent('page'), card.printHandover.click()]);
    await popup.waitForLoadState();
    expect(popup.url()).toContain(`/tasks/${task.id}/print/handover`);
    await expect(popup.locator('body')).toContainText(`Акт выдачи №${task.id}`);
    await popup.close();
  });
});
