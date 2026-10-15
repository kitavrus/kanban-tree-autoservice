import { test, expect } from '../fixtures/board';
import { NewOrderDialog, uniqPlate } from '../pages/NewOrderDialog';
import { OrderCardPage } from '../pages/OrderCardPage';

// Новый заказ со страницы «Календарь» (js/calendar.js → KT.taskCard, та же карточка, что на
// доске): «+ Заказ со сроком на этот день» открывает диалог со сроком = выбранный день; после
// сохранения и закрытия карточки страница перерисовывается и заказ виден на диаграмме дня.
test.use({ colorScheme: 'light' });

function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
}

test.describe('CALNO: новый заказ из календаря', () => {
  test('CALNO-001: заказ со сроком на выбранный день появляется на диаграмме дня', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const client = await adminApi.createClient({ company_name: uniq('CALNO-001 Орлов П. П.'), phones: [{ phone: '9990000951', note: '' }] });
    const plate = uniqPlate();
    const car = await adminApi.createEquipment(client.id, { plate_number: plate });
    const today = (await adminApi.getCalendar()).today;
    const day = addDays(today, 3);

    const dlg = new NewOrderDialog(page);
    const card = new OrderCardPage(page);
    await page.goto(`/calendar?month=${day.slice(0, 7)}&day=${day}`);
    await expect(page.getByTestId('cal-gantt-title')).toBeVisible();

    await page.getByTestId('cal-day-add').click();
    await expect(page.getByTestId('task-form')).toBeVisible();
    await expect(card.title).toHaveText('Новый заказ');
    await expect(dlg.due).toHaveValue(day);
    await expect(dlg.capacity).toContainText('Сегодня в ремонте');

    await dlg.useFoundCar(plate);
    await expect(dlg.client).toContainText(client.company_name);
    await page.getByTestId('task-save').click();
    await expect(card.title).toContainText('Заказ #');
    const id = Number(/#(\d+)/.exec((await card.title.textContent()) ?? '')![1]);
    _boardCleanup(id);

    const task = await adminApi.getTask(id);
    expect(task.due_date).toBe(day);
    expect(task.client_id).toBe(client.id);
    expect((await adminApi.getCard(id)).equipment.map((e) => e.equipment_id)).toEqual([car.id]);

    // закрытие карточки → перерисовка страницы: заказ на диаграмме выбранного дня.
    await card.close();
    await expect(page.locator(`[data-testid="cal-gantt-row"][data-task-id="${id}"]`)).toBeVisible();
  });

  test('CALNO-002: «+ Заказ» в шапке — без срока; у прошедшего дня кнопки «со сроком» нет', async ({ adminPage: page, adminApi }) => {
    const today = (await adminApi.getCalendar()).today;
    const past = addDays(today, -1);
    const dlg = new NewOrderDialog(page);

    await page.goto(`/calendar?month=${past.slice(0, 7)}&day=${past}`);
    await expect(page.getByTestId('cal-gantt-title')).toBeVisible();
    await expect(page.getByTestId('cal-day-add')).toHaveCount(0);

    await page.getByTestId('task-add').click();
    await expect(page.getByTestId('task-form')).toBeVisible();
    await expect(dlg.plate).toBeVisible();
    await expect(dlg.due).toHaveValue('');
    await expect(dlg.capacityWarning).toBeHidden();
  });
});
