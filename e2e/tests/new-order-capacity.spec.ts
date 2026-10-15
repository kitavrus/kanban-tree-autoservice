import { test, expect } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';

// Загрузка сервиса в диалоге «Новый заказ» (js/task-card.js «Загрузка сервиса», GET
// /api/v1/calendar): строка «Сегодня (или DD.MM.YYYY — день начала) в ремонте X из N, свободно
// K» и предупреждение, если в плановые дни заказа (с «Начать с» по начало + «Дней на работу» − 1;
// прошедшие дни не проверяются) есть дни без свободных мест. Сохранение не блокируется.
// Вместимость — общая настройка сервиса: тест ставит минимум (5) и возвращает прежнее значение.
// БД общая (заказы с прошедшим планом занимают место во всех будущих днях), поэтому ожидания по
// загрузке берутся из того же API календаря.
test.use({ colorScheme: 'light' });

const HORIZON = 20; // день, который тест заполняет до отказа (в пределах окна API)

function ymdText(ymd: string): string {
  const [y, m, d] = ymd.split('-');
  return `${d}.${m}.${y}`;
}

function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
}

test.describe('CAP: загрузка сервиса в новом заказе', () => {
  test('CAP-001: загрузка на день начала и предупреждение о плановых днях без мест', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const before = await adminApi.getCalendarSettings();
    await adminApi.setCalendarMaxCars(5);
    try {
      const today = (await adminApi.getCalendar()).today; // «сегодня» в поясе пользователя
      const fullDay = addDays(today, HORIZON);

      // заполняем fullDay до отказа однодневными заказами с началом в этот день — соседние дни
      // ими не заняты.
      const occupied = (await adminApi.getCalendar(fullDay, fullDay)).days[0].occupied;
      const columns = (await adminApi.listColumns()).items;
      const col = columns.find((c) => !c.is_final)!;
      const client = await adminApi.createClient({ company_name: uniq('CAP-001 Клиент'), phones: [{ phone: '9990000901', note: '' }] });
      for (let i = occupied; i < 5; i++) {
        await adminApi.createTask({ title: uniq(`CAP-001 заказ ${i}`), client_id: client.id, column_id: col.id, start_date: fullDay, duration_days: 1 });
      }
      const full = (await adminApi.getCalendar(fullDay, fullDay)).days[0];
      expect(full.free, 'день заполнен').toBe(0);
      const todayDay = (await adminApi.getCalendar(today, today)).days[0];
      const tail = await adminApi.getCalendar(addDays(fullDay, -2), fullDay);
      const tailFull = tail.days.filter((d) => d.free <= 0).map((d) => d.date);

      const board = new BoardPage(page);
      const dlg = board.newOrder;
      await board.goto();
      await board.openTaskAdd();

      // по умолчанию — начало сегодня, 1 день: строка загрузки на сегодня по данным календаря.
      await expect(dlg.start).toHaveValue(today);
      await expect(dlg.duration).toHaveValue('1');
      await expect(dlg.capacity).toHaveText(`Сегодня в ремонте ${todayDay.occupied} из 5, свободно ${todayDay.free}`);
      if (todayDay.free > 0) await expect(dlg.capacityWarning).toBeHidden();
      else await expect(dlg.capacityWarning).toContainText(ymdText(today));

      // начало в заполненный день — строка загрузки этого дня и предупреждение с ним.
      await dlg.start.fill(fullDay);
      await expect(dlg.capacity).toHaveText(`${ymdText(fullDay)} в ремонте ${full.occupied} из 5, свободно 0`);
      await expect(dlg.capacityWarning).toBeVisible();
      await expect(dlg.capacityWarning).toContainText('В плановые дни нет свободных мест');
      await expect(dlg.capacityWarning).toContainText(ymdText(fullDay));
      // заказ не блокируется.
      await expect(board.taskSave).toBeEnabled();

      // план целиком в прошлом — предупреждения нет.
      await dlg.start.fill(addDays(today, -10));
      await expect(dlg.capacityWarning).toBeHidden();

      // план из трёх дней, кончающийся в заполненный день, — первый день без мест в окне.
      await dlg.start.fill(addDays(fullDay, -2));
      await dlg.duration.fill('3');
      await dlg.duration.blur();
      await expect(dlg.plannedEnd).toHaveText(`Плановое окончание: ${ymdText(fullDay)}`);
      await expect(dlg.capacityWarning).toContainText(ymdText(tailFull[0]));

      // некорректный план (61 день) — подсказки и предупреждения нет.
      await dlg.duration.fill('61');
      await dlg.duration.blur();
      await expect(dlg.plannedEnd).toHaveText('');
      await expect(dlg.capacityWarning).toBeHidden();
    } finally {
      await adminApi.setCalendarMaxCars(before.max_cars);
    }
  });
});
