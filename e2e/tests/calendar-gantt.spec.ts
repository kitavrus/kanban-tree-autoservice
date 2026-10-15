import { test, expect } from '../fixtures/board';
import { OrderCardPage } from '../pages/OrderCardPage';

// Диаграмма Ганта выбранного дня на «Календаре» (internal/features/calendar/gantt.go): машины,
// стоящие в ремонте в этот день, полосами по шкале из 28 дней (выбранный — 8-й столбец):
// со дня приёмки по срок / выдачу; без срока — до правого края. Клик по полосе открывает
// карточку заказа на этой же странице.
test.use({ colorScheme: 'light' });

function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
}
function short(ymd: string): string {
  const [, m, d] = ymd.split('-');
  return `${d}.${m}`;
}

test.describe('GANTT: диаграмма выбранного дня', () => {
  test('GANTT-001: полоса со сроком и бессрочная, клик открывает карточку', async ({ adminPage: page, adminApi, uniq }) => {
    const today = (await adminApi.getCalendar()).today;
    const due = addDays(today, 4);
    const columns = (await adminApi.listColumns()).items;
    const col = columns.find((c) => !c.is_final)!;
    const client = await adminApi.createClient({ company_name: uniq('GANTT-001 Клиент'), phones: [{ phone: '9990000961', note: '' }] });
    const withDue = await adminApi.createTask({ title: uniq('GANTT-001 со сроком'), client_id: client.id, column_id: col.id, due_date: due });
    const noDue = await adminApi.createTask({ title: uniq('GANTT-001 без срока'), client_id: client.id, column_id: col.id });

    await page.goto(`/calendar?month=${today.slice(0, 7)}&day=${today}`);
    const gantt = page.getByTestId('cal-gantt');
    await expect(gantt).toBeVisible();
    await expect(page.getByTestId('cal-gantt-load')).toHaveCount(28);

    // со сроком: сегодня (8-й столбец) .. срок — 5 дней.
    const r1 = gantt.locator(`[data-testid="cal-gantt-row"][data-task-id="${withDue.id}"]`);
    await expect(r1).toHaveAttribute('data-col', '8');
    await expect(r1).toHaveAttribute('data-span', '5');
    await expect(r1.getByTestId('cal-gantt-period')).toContainText(`${short(today)} → срок ${short(due)}`);
    await expect(r1.getByTestId('cal-gantt-period')).toContainText('5 дн.');

    // без срока: от сегодня до правого края шкалы (28 − 7 = 21 столбец).
    const r2 = gantt.locator(`[data-testid="cal-gantt-row"][data-task-id="${noDue.id}"]`);
    await expect(r2).toHaveAttribute('data-span', '21');
    await expect(r2.getByTestId('cal-gantt-period')).toContainText('срок не указан');

    // клик по полосе — карточка заказа здесь же, без перехода на доску.
    await r1.getByTestId('cal-gantt-bar').click();
    const card = new OrderCardPage(page);
    await expect(card.title).toHaveText(`Заказ #${withDue.id}`);
    await expect(page).toHaveURL(/\/calendar/);
    await card.close();
  });
});
