import { test, expect } from '../fixtures/board';
import { OrderCardPage } from '../pages/OrderCardPage';

// Диаграмма Ганта выбранного дня на «Календаре» (internal/features/calendar/gantt.go): машины,
// стоящие в ремонте в этот день, полосами по шкале из 28 дней (выбранный — 8-й столбец):
// с «Начать с» по плановое окончание (начало + «Дней на работу» − 1) / выдачу; плановое
// окончание прошло, машина не выдана — до правого края. Клик по полосе открывает карточку
// заказа на этой же странице.
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
  test('GANTT-001: полоса по плану и просроченная бессрочная, клик открывает карточку', async ({ adminPage: page, adminApi, uniq }) => {
    const today = (await adminApi.getCalendar()).today;
    const due = addDays(today, 4);
    const columns = (await adminApi.listColumns()).items;
    const col = columns.find((c) => !c.is_final)!;
    const client = await adminApi.createClient({ company_name: uniq('GANTT-001 Клиент'), phones: [{ phone: '9990000961', note: '' }] });
    const planned = await adminApi.createTask({ title: uniq('GANTT-001 по плану'), client_id: client.id, column_id: col.id, start_date: today, duration_days: 5 });
    // план 3 дня назад на 2 дня — плановое окончание позавчера, машина не выдана.
    const overdue = await adminApi.createTask({
      title: uniq('GANTT-001 просрочен'), client_id: client.id, column_id: col.id, start_date: addDays(today, -3), duration_days: 2,
    });

    await page.goto(`/calendar?month=${today.slice(0, 7)}&day=${today}`);
    const gantt = page.getByTestId('cal-gantt');
    await expect(gantt).toBeVisible();
    await expect(page.getByTestId('cal-gantt-load')).toHaveCount(28);

    // по плану: сегодня (8-й столбец) .. плановое окончание — 5 дней, отметка окончания в 12-м.
    const r1 = gantt.locator(`[data-testid="cal-gantt-row"][data-task-id="${planned.id}"]`);
    await expect(r1).toHaveAttribute('data-col', '8');
    await expect(r1).toHaveAttribute('data-span', '5');
    await expect(r1.getByTestId('cal-gantt-period')).toContainText(`${short(today)} → срок ${short(due)}`);
    await expect(r1.getByTestId('cal-gantt-period')).toContainText('5 дн.');
    await expect(r1.getByTestId('cal-gantt-due')).toHaveAttribute('style', /grid-column: 12\b/);
    await expect(r1.getByTestId('cal-gantt-due')).toHaveAttribute('title', 'Плановое окончание');

    // просрочен: с начала (5-й столбец) до правого края шкалы (28 − 4 = 24 столбца).
    const r2 = gantt.locator(`[data-testid="cal-gantt-row"][data-task-id="${overdue.id}"]`);
    await expect(r2).toHaveAttribute('data-col', '5');
    await expect(r2).toHaveAttribute('data-span', '24');
    await expect(r2.getByTestId('cal-gantt-period')).toContainText(`срок ${short(addDays(today, -2))} просрочен`);

    // клик по полосе — карточка заказа здесь же, без перехода на доску.
    await r1.getByTestId('cal-gantt-bar').click();
    const card = new OrderCardPage(page);
    await expect(card.title).toHaveText(`Заказ #${planned.id}`);
    await expect(page).toHaveURL(/\/calendar/);
    await card.close();
  });

  test('GANTT-002: заказ с будущим «Начать с» — место только в плановые дни', async ({ adminPage: page, adminApi, uniq }) => {
    const today = (await adminApi.getCalendar()).today;
    const start = addDays(today, 9);
    const col = (await adminApi.listColumns()).items.find((c) => !c.is_final)!;
    const client = await adminApi.createClient({ company_name: uniq('GANTT-002 Клиент'), phones: [{ phone: '9990000962', note: '' }] });
    const t = await adminApi.createTask({ title: uniq('GANTT-002 позже'), client_id: client.id, column_id: col.id, start_date: start, duration_days: 3 });

    // накануне начала машины ещё нет — ни в загрузке дня, ни на диаграмме.
    await page.goto(`/calendar?month=${start.slice(0, 7)}&day=${addDays(start, -1)}`);
    await expect(page.getByTestId('cal-gantt')).toBeVisible();
    await expect(page.locator(`[data-testid="cal-gantt-row"][data-task-id="${t.id}"]`)).toHaveCount(0);

    // в день начала — полоса ровно 3 дня с 8-го столбца.
    await page.goto(`/calendar?month=${start.slice(0, 7)}&day=${start}`);
    const row = page.locator(`[data-testid="cal-gantt-row"][data-task-id="${t.id}"]`);
    await expect(row).toHaveAttribute('data-col', '8');
    await expect(row).toHaveAttribute('data-span', '3');
    await expect(row).toHaveAttribute('data-days', '3');
    await expect(row.getByTestId('cal-gantt-period')).toContainText(`${short(start)} → срок ${short(addDays(start, 2))}`);
  });
});
