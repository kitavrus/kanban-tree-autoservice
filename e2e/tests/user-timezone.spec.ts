import type { Page } from '@playwright/test';
import { test, expect, columnByName } from '../fixtures/board';
import { loginContext, type AdminApi, type User } from '../fixtures/auth';
import {
  TZ_ALMATY,
  TZ_BROWSER,
  TZ_FIXTURE,
  TZ_LABEL,
  TZ_MOSCOW,
  minutesBetween,
  pinFixtureMoment,
  type ZoneView,
} from '../fixtures/timezone';
import { BoardPage } from '../pages/BoardPage';
import { uniqPlate } from '../pages/NewOrderDialog';
import { OrderCardPage } from '../pages/OrderCardPage';
import { ExecutorPayoutsPage } from '../pages/ExecutorPayoutsPage';
import { ShellPage } from '../pages/ShellPage';
import { UsersPage } from '../pages/UsersPage';

// Часовой пояс пользователя (docs/features/user-timezone/draft-spec.md §8.1, §11:
// TS-040, TS-041, TS-042; фикстура времени TS-F5 — e2e/fixtures/timezone.ts).
//
// Браузер запущен в ТРЕТЬЕМ поясе (America/New_York): любой экран, который форматирует
// время «по браузеру», а не по поясу пользователя, эти тесты роняет. Общая dev-БД
// (§TD-6.2): свои клиент/услуга/сотрудники через uniq()+adminApi.
test.use({ colorScheme: 'light', timezoneId: TZ_BROWSER });

const DATE_RE = /^\d{2}\.\d{2}\.\d{4}$/;
const DATE_TIME_RE = /^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/;

type Seed = { adminApi: AdminApi; uniq: (s: string) => string; insertSubtask: (s: { taskId: number; serviceId: string }) => Promise<string> };

/** Оплаченный заказ с оборудованием и выполненной услугой, выплаченной исполнителю. */
async function seedPaidOrder({ adminApi, uniq, insertSubtask }: Seed, label: string, executorId: string) {
  const client = await adminApi.createClient({ company_name: uniq(`TZ клиент ${label}`), phones: [{ phone: '9990000300', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  const cat = await adminApi.createCategory({ kind: 'service', name: uniq(`TZ услуги ${label}`) });
  const service = await adminApi.createService({ category_id: cat.id, name: uniq(`TZ услуга ${label}`), price: 1000 });
  const equipment = await adminApi.createEquipment(client.id, { plate_number: uniqPlate() });
  const task = await adminApi.createTask({
    title: uniq(`TZ заказ ${label}`),
    client_id: client.id,
    column_id: columnByName(columns, 'Приёмка').id,
    assignee_id: executorId,
    is_paid: true,
  });
  await adminApi.addTaskEquipment(task.id, equipment.id);
  const subId = await insertSubtask({ taskId: task.id, serviceId: service.id });
  await adminApi.updateSubtask(subId, { status: 'done', price: 1000, assignee_id: executorId, equipment_id: equipment.id, executor_payout: 500 });
  await adminApi.markExecutorPayouts([subId], true);
  return { client, columns, equipment, task, subId };
}

/** Карточка сотрудника для PUT /api/v1/users/:id (полная замена) с новым поясом. */
function withTimezone(u: User, timezone: string) {
  return { login: u.login, name: u.name, role: u.role, phone: u.phone, is_active: true, timezone };
}

test.describe('TZ: свой часовой пояс', () => {
  test('TZ-001 (TS-040): исполнитель меняет пояс в сайдбаре — время сдвигается на −2 часа, значение переживает reload и повторный вход; 375×812 и обе темы', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    insertSubtask,
    browser,
    baseURL,
  }) => {
    const login = uniq('tz001');
    const exec = await makeUser({ login, name: uniq('Исполнитель TZ-001') });
    expect(exec.timezone, 'умолчание для нового сотрудника').toBe(TZ_ALMATY);
    const { subId } = await seedPaidOrder({ adminApi, uniq, insertSubtask }, 'TZ-001', exec.id);

    await loginAs(page, login, exec.password);
    const shell = new ShellPage(page);
    const ep = new ExecutorPayoutsPage(page);
    await ep.goto(`assignee=${exec.id}`);
    const paidAt = ep.row(subId).getByTestId('ep-row-paid-at');

    // Алматы — умолчание: select показывает его, время записи — в нём.
    await shell.expectTimezone(TZ_ALMATY);
    await expect(shell.timezoneSelect).toHaveText(TZ_LABEL[TZ_ALMATY]);
    await expect(paidAt).toHaveText(DATE_TIME_RE);
    const inAlmaty = await paidAt.innerText();

    // Смена пояса: PUT → страница перезагружается сама, все даты — в новом поясе.
    const saved = page.waitForResponse((r) => r.url().endsWith('/api/v1/users/me/timezone') && r.request().method() === 'PUT');
    const reloaded = page.waitForEvent('load');
    await shell.chooseTimezone(TZ_LABEL[TZ_MOSCOW]);
    expect((await saved).status()).toBe(200);
    await reloaded;
    await shell.expectTimezone(TZ_MOSCOW);
    await expect(shell.timezoneSelect).toHaveText(TZ_LABEL[TZ_MOSCOW]);
    await expect(shell.timezoneSelect).toBeEnabled();
    await expect(paidAt).toHaveText(DATE_TIME_RE);
    const inMoscow = await paidAt.innerText();
    expect(minutesBetween(inAlmaty, inMoscow), `та же запись: Алматы «${inAlmaty}», Москва «${inMoscow}»`).toBe(TZ_FIXTURE.almatyAheadMinutes);

    // Сохранено на сервере: reload и повторный вход (другой контекст) показывают Москву.
    expect((await adminApi.findByLogin(login)).timezone).toBe(TZ_MOSCOW);
    await page.reload();
    await shell.expectTimezone(TZ_MOSCOW);
    await expect(paidAt).toHaveText(inMoscow);

    const again = await browser.newContext({ baseURL, locale: 'ru-RU', timezoneId: TZ_BROWSER });
    await loginContext(again, login, exec.password);
    const againPage = await again.newPage();
    const againShell = new ShellPage(againPage);
    await againShell.goto('/');
    await againShell.expectTimezone(TZ_MOSCOW);
    await expect(againShell.timezoneSelect).toHaveText(TZ_LABEL[TZ_MOSCOW]);
    await again.close();

    // Темы (US-1 themes:both): select читаем — текст есть и не сливается с фоном кнопки.
    for (const scheme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: scheme });
      await shell.goto('/');
      await expect(page.locator('html')).toHaveAttribute('data-bs-theme', scheme);
      await expect(shell.timezoneSelect).toBeVisible();
      await expect(shell.timezoneSelect).toHaveText(TZ_LABEL[TZ_MOSCOW]);
      const colors = await shell.timezoneSelect.evaluate((el) => {
        const cs = getComputedStyle(el);
        return { color: cs.color, background: cs.backgroundColor };
      });
      expect(colors.color, `тема ${scheme}: цвет текста ≠ фон`).not.toBe(colors.background);
    }
    await page.emulateMedia({ colorScheme: 'light' });

    // Телефон 375×812 (US-1 adaptive:min): сайдбар с select'ом — без горизонтальной прокрутки.
    await page.setViewportSize({ width: 375, height: 812 });
    await shell.goto('/');
    expect(await shell.hasHorizontalOverflow()).toBe(false);
    await shell.menuToggle.click();
    await expect(shell.sidebar).toBeInViewport();
    await expect(shell.timezoneSelect).toBeInViewport();
    expect(await shell.hasHorizontalOverflow()).toBe(false);
    const fits = await shell.timezoneSelect.evaluate((el) => {
      const r = el.getBoundingClientRect();
      const side = el.closest('[data-testid="sidebar"]')!.getBoundingClientRect();
      return r.left >= side.left && r.right <= side.right && r.right <= window.innerWidth;
    });
    expect(fits, 'select не вылезает за сайдбар и экран').toBe(true);
    // Комбобокс на телефоне открывается в пределах экрана.
    await shell.timezoneSelect.click();
    await expect(shell.dropdown).toBeInViewport();
    await expect(shell.dropdown.locator('.cb-opt').first()).toHaveText(TZ_LABEL[TZ_ALMATY]);
    await expect(shell.dropdown.locator('.cb-opt').nth(1)).toHaveText(TZ_LABEL[TZ_MOSCOW]);
    expect(await shell.hasHorizontalOverflow()).toBe(false);
  });

  test('TZ-002 (TS-040, TS-021): неизвестный пояс подменой значения — тост «Неизвестный часовой пояс», select возвращается к прежнему', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.endsWith('/api/v1/users/me/timezone') && status === 400);
    const login = uniq('tz002');
    const exec = await makeUser({ login, name: uniq('Исполнитель TZ-002') });
    await loginAs(page, login, exec.password);
    const shell = new ShellPage(page);
    await shell.goto('/');
    await shell.expectTimezone(TZ_ALMATY);

    const rejected = page.waitForResponse((r) => r.url().endsWith('/api/v1/users/me/timezone') && r.request().method() === 'PUT');
    await shell.timezoneNative.evaluate((el) => {
      const select = el as HTMLSelectElement;
      const option = document.createElement('option');
      option.value = 'Mars/Olympus';
      option.textContent = 'Марс';
      select.append(option);
      select.value = option.value;
      select.dispatchEvent(new Event('change', { bubbles: true }));
    });
    expect((await rejected).status()).toBe(400);

    await expect(shell.timezoneToast).toHaveText('Неизвестный часовой пояс');
    await expect(shell.timezoneNative).toHaveValue(TZ_ALMATY);
    await expect(shell.timezoneSelect).toHaveText(TZ_LABEL[TZ_ALMATY]);
    await expect(shell.timezoneSelect).toBeEnabled();
    await shell.expectTimezone(TZ_ALMATY); // страница не перезагружалась в чужой пояс
    expect((await adminApi.findByLogin(login)).timezone, 'запись не изменена').toBe(TZ_ALMATY);
  });
});

test.describe('TZ: пояс применяется во всей системе', () => {
  /** JS-экраны с моментом TS-F5: карточка заказа, «Выплаты». */
  async function expectJsScreens(
    page: Page,
    view: ZoneView,
    d: { clientId: string; taskId: number; subId: string; executorId: string },
  ) {
    await pinFixtureMoment(page);
    const shell = new ShellPage(page);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);

    // Карточка заказа: дата оплаты исполнителю (дата+время) и дата записи журнала услуг.
    await board.goto(`?client_id=${d.clientId}&task=${d.taskId}`);
    await card.waitOpen();
    await shell.expectTimezone(view.zone);
    await expect(card.subtaskExecutorPaidAt(d.subId)).toHaveText(view.dateTime);
    await expect(card.subtaskRow(d.subId).getByTestId('card-log-date')).toHaveText(view.date);

    // «Выплаты»: дата и время отметки.
    const ep = new ExecutorPayoutsPage(page);
    await ep.goto(`assignee=${d.executorId}`);
    await expect(ep.row(d.subId).getByTestId('ep-row-paid-at')).toHaveText(view.dateTime);
  }

  /** SSR-экраны с настоящим «сейчас»: история оборудования и две печатные формы. */
  async function readSsrScreens(page: Page, d: { taskId: number; equipmentId: string }) {
    await page.goto(`/equipment/${d.equipmentId}/history`);
    const order = page.locator(`[data-testid="history-order"][data-task-id="${d.taskId}"]`);
    await expect(order.getByTestId('history-order-date')).toHaveText(DATE_RE);
    await expect(order.getByTestId('history-log-time')).toHaveText(DATE_TIME_RE);
    const orderDate = await order.getByTestId('history-order-date').innerText();
    const logTime = await order.getByTestId('history-log-time').innerText();

    await page.goto(`/tasks/${d.taskId}/print/intake`);
    await expect(page.getByTestId('print-date')).toHaveText(/^Дата: \d{2}\.\d{2}\.\d{4}$/);
    const intakeDate = await page.getByTestId('print-date').innerText();

    await page.goto(`/tasks/${d.taskId}/print/handover`);
    await expect(page.getByTestId('print-date')).toHaveText(/^Дата: \d{2}\.\d{2}\.\d{4}$/);
    await expect(page.getByTestId('print-payment')).toHaveText(/Оплачено \d{2}\.\d{2}\.\d{4}/);
    const handoverDate = await page.getByTestId('print-date').innerText();
    const payment = (await page.getByTestId('print-payment').innerText()).trim();
    return { orderDate, logTime, intakeDate, handoverDate, payment };
  }

  test('TZ-010 (TS-041): карточка заказа, «Выплаты», история оборудования и печать — в поясе пользователя, не браузера', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    insertSubtask,
    browser,
    baseURL,
  }) => {
    test.slow(); // два пользователя × семь экранов
    const almatyUser = await makeUser({ login: uniq('tz010-almaty'), name: uniq('Админ Алматы TZ-010'), role: 'admin', timezone: TZ_ALMATY });
    const moscowUser = await makeUser({ login: uniq('tz010-moscow'), name: uniq('Админ Москва TZ-010'), role: 'admin', timezone: TZ_MOSCOW });
    const exec = await makeUser({ login: uniq('tz010-exec'), name: uniq('Исполнитель TZ-010') });
    expect(moscowUser.timezone).toBe(TZ_MOSCOW);

    const { client, equipment, task, subId } = await seedPaidOrder({ adminApi, uniq, insertSubtask }, 'TZ-010', exec.id);
    const data = { clientId: client.id, taskId: task.id, subId, executorId: exec.id, equipmentId: equipment.id };

    // Запись журнала услуги — настоящая, через UI (пользователь Алматы).
    await loginAs(page, almatyUser.login, almatyUser.password);
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}&task=${task.id}`);
    await card.waitOpen();
    const logText = uniq('TZ-010 запись журнала');
    await card.logNewInput(subId).fill(logText);
    await card.logAddBtn(subId).click();
    await expect(card.subtaskRow(subId)).toContainText(logText);

    const moscowContext = await browser.newContext({ baseURL, locale: 'ru-RU', timezoneId: TZ_BROWSER, colorScheme: 'light' });
    await loginContext(moscowContext, moscowUser.login, moscowUser.password);
    const moscowPage = await moscowContext.newPage();

    // SSR — до «прикалывания» момента: настоящее время записи у двух пользователей.
    const ssrAlmaty = await readSsrScreens(page, data);
    const ssrMoscow = await readSsrScreens(moscowPage, data);
    expect(minutesBetween(ssrAlmaty.logTime, ssrMoscow.logTime), `журнал в истории: Алматы «${ssrAlmaty.logTime}», Москва «${ssrMoscow.logTime}»`).toBe(
      TZ_FIXTURE.almatyAheadMinutes,
    );

    // JS-экраны — точный момент TS-F5: 00:30 7 октября по Алматы = 22:30 6 октября по Москве.
    await expectJsScreens(page, TZ_FIXTURE.almaty, data);
    await expectJsScreens(moscowPage, TZ_FIXTURE.moscow, data);

    // SSR, даты без времени (печать, дата заказа): администратор разводит пользователей в
    // пояса UTC+14 и UTC−12 — между ними 26 часов, календарная дата различается ВСЕГДА,
    // поэтому проверка не зависит от времени прогона. Заодно: пояс, изменённый
    // администратором, применяется при следующей загрузке страницы (§8.1 «Состояния»).
    await adminApi.updateUser(almatyUser.id, withTimezone(almatyUser, 'Pacific/Kiritimati'));
    await adminApi.updateUser(moscowUser.id, withTimezone(moscowUser, 'Etc/GMT+12'));
    const east = await readSsrScreens(page, data);
    const west = await readSsrScreens(moscowPage, data);
    expect(east.intakeDate, 'дата печати «Приём»').not.toBe(west.intakeDate);
    expect(east.handoverDate, 'дата печати «Выдача»').not.toBe(west.handoverDate);
    expect(east.payment, 'дата оплаты в «Выдаче»').not.toBe(west.payment);
    expect(east.orderDate, 'дата заказа в истории оборудования').not.toBe(west.orderDate);
    expect(minutesBetween(east.logTime, west.logTime)).toBe(26 * 60);

    await moscowContext.close();
  });

  test('TZ-011 (TS-041): метка срока «сегодня / просрочен» на доске считается по поясу пользователя, не браузера', async ({
    page,
    adminApi,
    uniq,
    makeUser,
    loginAs,
    browser,
    baseURL,
  }) => {
    const almatyUser = await makeUser({ login: uniq('tz011-almaty'), name: uniq('Админ Алматы TZ-011'), role: 'admin', timezone: TZ_ALMATY });
    const moscowUser = await makeUser({ login: uniq('tz011-moscow'), name: uniq('Админ Москва TZ-011'), role: 'admin', timezone: TZ_MOSCOW });
    const client = await adminApi.createClient({ company_name: uniq('TZ-011 клиент'), phones: [{ phone: '9990000301', note: '' }] });
    const column = columnByName((await adminApi.listColumns()).items, 'Приёмка');
    const mk = (label: string, due: string) => adminApi.createTask({ title: uniq(`TZ-011 ${label}`), client_id: client.id, column_id: column.id, due_date: due });
    // «Сейчас» = момент TS-F5: в Алматы уже 7 октября, в Москве и в браузере (Нью-Йорк) — 6-е.
    const due6 = await mk('срок 6 октября', TZ_FIXTURE.moscow.today);
    const due8 = await mk('срок 8 октября', '2026-10-08');

    const check = async (p: Page, expected: { due6: string; due8: string }) => {
      await p.clock.setFixedTime(new Date(TZ_FIXTURE.moment));
      const board = new BoardPage(p);
      await board.goto(`?client_id=${client.id}`);
      await expect(board.card(due6.id).getByTestId('task-card-due')).toHaveAttribute('data-due', expected.due6);
      await expect(board.card(due8.id).getByTestId('task-card-due')).toHaveAttribute('data-due', expected.due8);
      // Сама дата срока — календарная, по поясу не сдвигается.
      await expect(board.card(due6.id).getByTestId('task-card-due')).toContainText('06.10.2026');
      await expect(board.card(due8.id).getByTestId('task-card-due')).toContainText('08.10.2026');
    };

    // Алматы: сегодня 7-е → срок 6-го просрочен, 8-е — «завтра».
    await loginAs(page, almatyUser.login, almatyUser.password);
    await check(page, { due6: 'overdue', due8: 'soon' });
    await expect(new BoardPage(page).card(due6.id)).toHaveClass(/overdue/);

    // Москва: сегодня 6-е → срок 6-го — «сегодня» (не просрочен), 8-е — послезавтра.
    const moscowContext = await browser.newContext({ baseURL, locale: 'ru-RU', timezoneId: TZ_BROWSER, colorScheme: 'light' });
    await loginContext(moscowContext, moscowUser.login, moscowUser.password);
    const moscowPage = await moscowContext.newPage();
    await check(moscowPage, { due6: 'soon', due8: 'ok' });
    await expect(new BoardPage(moscowPage).card(due6.id)).not.toHaveClass(/overdue/);
    await moscowContext.close();
  });
});

test.describe('TZ: администратор задаёт пояс сотруднику', () => {
  test('TZ-020 (TS-042): форма сотрудника — по умолчанию «Алматы (UTC+5)»; админ меняет пояс → сотрудник видит даты и select в заданном поясе; невалидный пояс — ошибка под полем', async ({
    adminPage: page,
    adminApi,
    uniq,
    insertSubtask,
    allowHttpErrors,
    browser,
    baseURL,
  }) => {
    allowHttpErrors((url, status) => /\/users\/[0-9a-f-]{36}$/.test(url) && status === 422);
    const users = new UsersPage(page);
    const login = uniq('tz020');
    const name = uniq('Сотрудник TZ-020');

    // Создание: поле есть, по умолчанию — Алматы.
    await users.gotoNew();
    await expect(users.timezone).toHaveText(TZ_LABEL[TZ_ALMATY]);
    await expect(users.timezoneNative).toHaveValue(TZ_ALMATY);
    await users.fill({ name, login, password: 'password1' });
    await users.save.click();
    await expect(users.flash).toHaveText('Сотрудник создан');
    const created = await adminApi.findByLogin(login);
    expect(created.timezone).toBe(TZ_ALMATY);

    const { subId } = await seedPaidOrder({ adminApi, uniq, insertSubtask }, 'TZ-020', created.id);

    // Правка: форма показывает текущий пояс; админ ставит Москву и сохраняет.
    await users.goto(`?q=${login}`);
    await users.open(login);
    await expect(users.timezone).toHaveText(TZ_LABEL[TZ_ALMATY]);
    await users.chooseTimezone(TZ_LABEL[TZ_MOSCOW]);
    await users.save.click();
    await expect(users.flash).toHaveText('Изменения сохранены');
    expect((await adminApi.findByLogin(login)).timezone).toBe(TZ_MOSCOW);

    // Сотрудник входит: select в сайдбаре — заданный пояс, даты — в нём (момент TS-F5).
    const own = await browser.newContext({ baseURL, locale: 'ru-RU', timezoneId: TZ_BROWSER, colorScheme: 'light' });
    await loginContext(own, login, 'password1');
    const ownPage = await own.newPage();
    await pinFixtureMoment(ownPage);
    const ep = new ExecutorPayoutsPage(ownPage);
    await ep.goto(`assignee=${created.id}`);
    const ownShell = new ShellPage(ownPage);
    await ownShell.expectTimezone(TZ_MOSCOW);
    await expect(ownShell.timezoneSelect).toHaveText(TZ_LABEL[TZ_MOSCOW]);
    await expect(ep.row(subId).getByTestId('ep-row-paid-at')).toHaveText(TZ_FIXTURE.moscow.dateTime);
    await own.close();

    // Негатив: значение select'а подменено на несуществующий пояс → 422, текст под полем,
    // введённое не теряется, пояс сотрудника прежний.
    await users.goto(`?q=${login}`);
    await users.open(login);
    await expect(users.timezone).toHaveText(TZ_LABEL[TZ_MOSCOW]);
    await users.fill({ name: `${name} правка` });
    await users.forgeTimezone('Mars/Olympus');
    await users.save.click();
    await expect(users.fieldError('timezone')).toHaveText('Неизвестный часовой пояс');
    await expect(users.name).toHaveValue(`${name} правка`);
    const after = await adminApi.findByLogin(login);
    expect(after.timezone).toBe(TZ_MOSCOW);
    expect(after.name, 'невалидная форма ничего не сохранила').toBe(name);
  });
});
