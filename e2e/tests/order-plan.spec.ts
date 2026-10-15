import { test, expect, columnByName } from '../fixtures/board';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { uniqPlate } from '../pages/NewOrderDialog';

// План работ заказа (order-start-duration): в диалоге «Новый заказ» и в шапке карточки вместо
// «Срок» — «Начать с» + «Дней на работу» (1–60); срок заказа на доске (метка 📅) — плановое
// окончание = начало + дни − 1. Календарь загрузки: машина занимает место ровно в плановые дни
// (заказ с будущим началом — только с него), диаграмма Ганта — полоса этих дней.
test.use({ colorScheme: 'light' });

function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
}

function ymdText(ymd: string): string {
  const [y, m, d] = ymd.split('-');
  return `${d}.${m}.${y}`;
}

test.describe('PLAN: «Начать с» и «Дней на работу»', () => {
  test('PLAN-001: новый заказ с будущим началом на 3 дня занимает место ровно эти дни; правка дней пересчитывает окончание и метку доски', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const client = await adminApi.createClient({ company_name: uniq('PLAN-001 Соколов А. А.'), phones: [{ phone: '9990000971', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const col = columnByName(columns, 'Приёмка');
    const today = (await adminApi.getCalendar()).today;
    const start = addDays(today, 12);

    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${client.id}`);
    await board.columnAddTaskBtn(col.id).click();
    await expect(board.taskForm).toBeVisible();

    // по умолчанию — сегодня, 1 день; подсказка планового окончания пересчитывается на вводе.
    await expect(dlg.start).toHaveValue(today);
    await expect(dlg.duration).toHaveValue('1');
    await expect(dlg.plannedEnd).toHaveText(`Плановое окончание: ${ymdText(today)}`);

    await dlg.fillNewCar(uniqPlate());
    await dlg.chooseClient(client.company_name);
    await dlg.start.fill(start);
    await dlg.duration.fill('3');
    await dlg.duration.blur();
    await expect(dlg.plannedEnd).toHaveText(`Плановое окончание: ${ymdText(addDays(start, 2))}`);
    await expect(dlg.capacity).toContainText(`${ymdText(start)} в ремонте`);

    await board.taskSave.click();
    await expect(card.title).toContainText('Заказ #');
    const id = Number(/#(\d+)/.exec((await card.title.textContent()) ?? '')![1]);
    _boardCleanup(id);
    // карточка созданного заказа — план в шапке.
    await expect(card.taskStart).toHaveValue(start);
    await expect(card.taskDuration).toHaveValue('3');

    const task = await adminApi.getTask(id);
    expect(`${task.start_date}|${task.duration_days}|${task.due_date}`).toBe(`${start}|3|${addDays(start, 2)}`);

    // календарь: место занято ровно с начала по плановое окончание.
    const win = await adminApi.getCalendar(addDays(start, -1), addDays(start, 3));
    const busy = win.days.filter((d) => d.task_ids.includes(id)).map((d) => d.date);
    expect(busy).toEqual([start, addDays(start, 1), addDays(start, 2)]);
    await card.close();

    // диаграмма Ганта дня начала — полоса 3 дня с 8-го столбца.
    await page.goto(`/calendar?month=${start.slice(0, 7)}&day=${start}`);
    const row = page.locator(`[data-testid="cal-gantt-row"][data-task-id="${id}"]`);
    await expect(row).toHaveAttribute('data-col', '8');
    await expect(row).toHaveAttribute('data-span', '3');
    await expect(row).toHaveAttribute('data-days', '3');

    // правка «Дней на работу» в карточке (открыта с диаграммы) — сохраняется кнопкой.
    await row.getByTestId('cal-gantt-bar').click();
    await expect(card.title).toHaveText(`Заказ #${id}`);
    await expect(card.taskDuration).toHaveValue('3');
    await card.fillHeader({ days: 5 });
    await expect(card.plannedEnd).toHaveText(`Плановое окончание: ${ymdText(addDays(start, 4))}`);
    await card.save();
    await expect(card.error).toBeHidden();
    await expect.poll(async () => (await adminApi.getTask(id)).due_date).toBe(addDays(start, 4));
    await card.close();
    // закрытие карточки после правки перерисовывает календарь — полоса стала 5 дней.
    await expect(page.locator(`[data-testid="cal-gantt-row"][data-task-id="${id}"]`)).toHaveAttribute('data-span', '5');

    // доска: метка 📅 — новое плановое окончание.
    await board.goto(`?client_id=${client.id}`);
    await expect(board.card(id).getByTestId('task-card-due')).toContainText(ymdText(addDays(start, 4)));
    await expect(board.card(id).getByTestId('task-card-due')).toHaveAttribute('data-due', 'ok');
  });

  test('PLAN-003: фильтр доски «Горит/просрочено» и метка 📅 — по плановому окончанию', async ({ adminPage: page, adminApi, uniq }) => {
    const client = await adminApi.createClient({ company_name: uniq('PLAN-003 Клиент'), phones: [{ phone: '9990000973', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const open = columnByName(columns, 'Приёмка');
    const final = columns.find((c) => c.is_final)!;
    const today = (await adminApi.getCalendar()).today;
    const mk = (label: string, column: string, startOffset: number, days: number) =>
      adminApi.createTask({ title: uniq(`PLAN-003 ${label}`), client_id: client.id, column_id: column, start_date: addDays(today, startOffset), duration_days: days });
    // контрольные заказы — с явным планом (фикстура по умолчанию даёт «сегодня, 1 день» = «Горит»).
    const future = await mk('позже', open.id, 5, 3); // окончание через 7 дней — не горит
    const endsToday = await mk('кончается сегодня', open.id, -2, 3);
    const endsTomorrow = await mk('кончается завтра', open.id, 0, 2);
    const overdue = await mk('просрочен', open.id, -4, 2); // окончание 3 дня назад, не выдан
    const issued = await mk('выдан', final.id, -4, 1); // выдан — не горит, хоть план и прошёл

    const board = new BoardPage(page);
    await board.goto(`?client_id=${client.id}`);
    const due = (id: number) => board.card(id).getByTestId('task-card-due');
    await expect(due(future.id)).toHaveAttribute('data-due', 'ok');
    await expect(due(future.id)).toContainText(ymdText(addDays(today, 7)));
    await expect(due(endsToday.id)).toHaveAttribute('data-due', 'soon');
    await expect(due(endsToday.id)).toContainText(ymdText(today));
    await expect(due(endsTomorrow.id)).toHaveAttribute('data-due', 'soon');
    await expect(due(overdue.id)).toHaveAttribute('data-due', 'overdue');
    await expect(board.card(overdue.id)).toHaveClass(/overdue/);
    await expect(due(issued.id)).toHaveAttribute('data-due', 'ok');

    // «Горит» — окончание сегодня/завтра и просроченные открытые.
    await board.chooseCombobox(board.filterDue, 'Горит (сегодня/завтра, просрочено)');
    await expect.poll(() => new URL(page.url()).searchParams.get('due')).toBe('soon');
    for (const t of [endsToday, endsTomorrow, overdue]) await expect(board.card(t.id)).toBeVisible();
    for (const t of [future, issued]) await expect(board.card(t.id)).toHaveCount(0);

    // «Только просроченные» — плановое окончание прошло, машина не выдана.
    await board.chooseCombobox(board.filterDue, 'Только просроченные');
    await expect.poll(() => new URL(page.url()).searchParams.get('due')).toBe('overdue');
    await expect(board.card(overdue.id)).toBeVisible();
    for (const t of [future, endsToday, endsTomorrow, issued]) await expect(board.card(t.id)).toHaveCount(0);

    await board.chooseCombobox(board.filterDue, 'Любой срок');
    for (const t of [future, endsToday, endsTomorrow, overdue, issued]) await expect(board.card(t.id)).toBeVisible();
  });

  test('PLAN-002: «Начать с» и «Дней на работу» обязательны, дни — от 1 до 60 (ошибки под полями, заказ не создаётся)', async ({
    adminPage: page,
    adminApi,
    uniq,
    _boardCleanup,
  }) => {
    const client = await adminApi.createClient({ company_name: uniq('PLAN-002 Клиент'), phones: [{ phone: '9990000972', note: '' }] });
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    const dlg = board.newOrder;
    await board.goto(`?client_id=${client.id}`);
    await board.openTaskAdd();
    await dlg.fillNewCar(uniqPlate());
    await dlg.chooseClient(client.company_name);

    await dlg.start.fill('');
    await dlg.duration.fill('');
    await board.taskSave.click();
    await expect(dlg.startError).toHaveText('Обязательное поле');
    await expect(dlg.durationError).toHaveText('Обязательное поле');
    await expect(card.title).toHaveText('Новый заказ');

    for (const bad of ['0', '61', '2.5']) {
      await dlg.duration.fill(bad);
      await board.taskSave.click();
      await expect(dlg.durationError, `дни = ${bad}`).toHaveText('От 1 до 60 дней');
    }
    await expect(card.title).toHaveText('Новый заказ');

    // исправленный план — ошибки уходят, заказ создаётся.
    await dlg.start.fill('2031-07-01');
    await dlg.duration.fill('60');
    await expect(dlg.durationError).toBeHidden();
    await expect(dlg.plannedEnd).toHaveText('Плановое окончание: 29.08.2031');
    await board.taskSave.click();
    await expect(card.title).toContainText('Заказ #');
    const id = Number(/#(\d+)/.exec((await card.title.textContent()) ?? '')![1]);
    _boardCleanup(id);
    const task = await adminApi.getTask(id);
    expect(`${task.start_date}|${task.duration_days}|${task.due_date}`).toBe('2031-07-01|60|2031-08-29');
  });
});
