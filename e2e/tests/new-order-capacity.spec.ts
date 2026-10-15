import { test, expect } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';

// Загрузка сервиса в диалоге «Новый заказ» (js/task-card.js «Загрузка сервиса», GET
// /api/v1/calendar): строка «Сегодня в ремонте X из N, свободно K» и предупреждение, если от
// сегодня до выбранного срока есть дни без свободных мест. Сохранение не блокируется.
// Вместимость — общая настройка сервиса: тест ставит минимум (5) и возвращает прежнее значение.
test.use({ colorScheme: 'light' });

const HORIZON = 20; // день срока, который тест заполняет до отказа (в пределах окна API)

function ymdText(ymd: string): string {
  const [y, m, d] = ymd.split('-');
  return `${d}.${m}.${y}`;
}

function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
}

test.describe('CAP: загрузка сервиса в новом заказе', () => {
  test('CAP-001: загрузка на сегодня и предупреждение о днях без мест до срока', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const before = await adminApi.getCalendarSettings();
    await adminApi.setCalendarMaxCars(5);
    try {
      const today = (await adminApi.getCalendar()).today; // «сегодня» в поясе пользователя
      const fullDay = addDays(today, HORIZON);

      // заполняем день срока до отказа: заказы, принятые сегодня со сроком fullDay, занимают
      // место с сегодня по fullDay включительно.
      const occupied = (await adminApi.getCalendar(fullDay, fullDay)).days[0].occupied;
      const columns = (await adminApi.listColumns()).items;
      const col = columns.find((c) => !c.is_final)!;
      const client = await adminApi.createClient({ company_name: uniq('CAP-001 Клиент'), phones: [{ phone: '9990000901', note: '' }] });
      for (let i = occupied; i < 5; i++) {
        await adminApi.createTask({ title: uniq(`CAP-001 заказ ${i}`), client_id: client.id, column_id: col.id, due_date: fullDay });
      }
      const window = await adminApi.getCalendar(today, fullDay);
      expect(window.days.at(-1)!.free, 'день срока заполнен').toBe(0);
      const todayDay = window.days[0];
      const fullDates = window.days.filter((d) => d.free <= 0).map((d) => d.date);

      const board = new BoardPage(page);
      const dlg = board.newOrder;
      await board.goto();
      await board.openTaskAdd();

      // строка загрузки на сегодня — по данным календаря.
      await expect(dlg.capacity).toHaveText(`Сегодня в ремонте ${todayDay.occupied} из 5, свободно ${todayDay.free}`);
      await expect(dlg.capacityWarning).toBeHidden();

      // срок в заполненный день — предупреждение с первым днём без мест.
      await dlg.due.fill(fullDay);
      await expect(dlg.capacityWarning).toBeVisible();
      await expect(dlg.capacityWarning).toContainText('До срока нет свободных мест');
      await expect(dlg.capacityWarning).toContainText(ymdText(fullDates[0]));
      // заказ не блокируется.
      await expect(board.taskSave).toBeEnabled();

      // срок в прошлом / без срока — предупреждения нет.
      await dlg.due.fill(addDays(today, -1));
      await expect(dlg.capacityWarning).toBeHidden();
      await dlg.due.fill(fullDay);
      await expect(dlg.capacityWarning).toBeVisible();
      await dlg.due.fill('');
      await expect(dlg.capacityWarning).toBeHidden();
    } finally {
      await adminApi.setCalendarMaxCars(before.max_cars);
    }
  });
});
