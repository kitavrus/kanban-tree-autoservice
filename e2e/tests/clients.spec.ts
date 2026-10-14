import { test, expect } from '../fixtures/test';
import { ClientsPage } from '../pages/ClientsPage';
import { EquipmentPage } from '../pages/EquipmentPage';
import { EquipmentTypesPage } from '../pages/EquipmentTypesPage';

// Фаза 6: экран «Клиенты» (admin) — AC-001..006, ADR-005 (конфликт версии),
// ADR-103 (confirm-удаление). Сортировка списка фиксированная (AC-004, Tech Spec В-4 —
// по компании, без UI-переключателя) — CL-006 проверяет именно это, sort-параметр в API
// не нужен (fix-loop 1, ответ оркестратора).
// CL-011 (XSS), CL-020 (конкурентность), CL-021 (адаптив) — Phase 9.
test.use({ colorScheme: 'light' });

test.describe('CL: создание и валидация', () => {
  test('CL-001: админ создаёт клиента с несколькими телефонами через форму (PRG + flash)', async ({ adminPage: page, uniq }) => {
    const name = uniq('Ромашка');
    const clients = new ClientsPage(page);
    await clients.goto();
    await clients.addButton.click();
    await expect(page.locator('.app-title')).toHaveText('Новый клиент');

    await clients.fill({ companyName: name, address: 'г. Москва, ул. Ленина, 1', contactName: 'Иванов Иван', contactPosition: 'Директор' });
    await clients.fillPhone(0, { phone: '+7 (999) 123-45-67', note: 'основной' });
    await clients.phoneAdd.click();
    await clients.fillPhone(1, { phone: '8001234567', note: 'доп.' });
    await clients.save.click();

    // PRG — на карточку созданного клиента (design-go-layers.md §SSR Routes), не на список.
    await expect(page).toHaveURL(/\/clients\/[^/]+\?flash=created$/);
    await expect(clients.flash).toHaveText('Клиент создан');

    await clients.goto(`?q=${name}`);
    await expect(clients.row(name)).toContainText(name);
    await expect(clients.row(name)).toContainText('+7 (999) 123-45-67');
    await expect(clients.row(name)).toContainText('8001234567');
    await expect(clients.row(name)).toContainText('Иванов Иван');
  });

  test('CL-002: обязательные поля, формат и лимит телефонов — ошибки под полями, ввод сохраняется', async ({ adminPage: page, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/clients') && status === 422);
    const clients = new ClientsPage(page);
    await clients.gotoNew();

    await expect(clients.companyName).toHaveAttribute('maxlength', '200');
    await expect(clients.address).toHaveAttribute('maxlength', '500');
    await expect(clients.contactName).toHaveAttribute('maxlength', '200');
    await expect(clients.contactPosition).toHaveAttribute('maxlength', '200');
    await expect(clients.phoneInput(0)).toHaveAttribute('maxlength', '32');
    await expect(clients.phoneNote(0)).toHaveAttribute('maxlength', '200');

    // пустая форма — company_name и phones обязательны (пустая строка телефона отбрасывается).
    await clients.save.click();
    await expect(clients.fieldError('company_name')).toHaveText('Обязательное поле');
    await expect(clients.fieldError('phones')).toHaveText('Укажите хотя бы один телефон');
    await expect(clients.companyName).toHaveClass(/is-invalid/);

    // телефон без цифр — invalid_format, введённое сохраняется.
    await clients.fill({ companyName: 'Валидация Клиент' });
    await clients.fillPhone(0, { phone: 'abc', note: 'заметка' });
    await clients.save.click();
    await expect(clients.phoneFieldError(0)).toHaveText('Номер должен содержать цифры');
    await expect(clients.fieldError('company_name')).toHaveCount(0);
    await expect(clients.companyName).toHaveValue('Валидация Клиент');
    await expect(clients.phoneInput(0)).toHaveValue('abc');
    await expect(clients.phoneNote(0)).toHaveValue('заметка');

    // больше 20 телефонов — too_many.
    await clients.fillPhone(0, { phone: '1' });
    for (let i = 0; i < 20; i++) await clients.phoneAdd.click();
    await expect(clients.phonesRows).toHaveCount(21);
    for (let i = 1; i < 21; i++) await clients.fillPhone(i, { phone: String(1000 + i) });
    await clients.save.click();
    await expect(clients.fieldError('phones')).toHaveText('Не больше 20 телефонов');
  });

  test('CL-003: исправление после ошибки валидации — форма сохраняется', async ({ adminPage: page, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, status) => url.endsWith('/clients') && status === 422);
    const name = uniq('Исправленный Клиент');
    const clients = new ClientsPage(page);
    await clients.gotoNew();
    await clients.save.click();
    await expect(clients.fieldError('company_name')).toBeVisible();

    await clients.fill({ companyName: name });
    await clients.fillPhone(0, { phone: '9990001122' });
    await clients.save.click();

    await expect(page).toHaveURL(/\/clients\/[^/]+\?flash=created$/);
    await expect(clients.flash).toHaveText('Клиент создан');

    await clients.goto(`?q=${name}`);
    await expect(clients.row(name)).toBeVisible();
  });

  test('CL-004: добавление и удаление строк телефонов — последняя строка очищается, не удаляется', async ({ adminPage: page, uniq }) => {
    const name = uniq('Телефоны Клиент');
    const clients = new ClientsPage(page);
    await clients.gotoNew();
    await clients.fill({ companyName: name });
    await clients.fillPhone(0, { phone: '1110000000', note: 'первый' });

    await clients.phoneAdd.click();
    await expect(clients.phonesRows).toHaveCount(2);
    await clients.fillPhone(1, { phone: '2220000000', note: 'второй' });

    await clients.phoneAdd.click();
    await expect(clients.phonesRows).toHaveCount(3);
    await clients.fillPhone(2, { phone: '3330000000' });

    // удаление средней строки — остальные сдвигаются, значения не путаются.
    await clients.phoneDel(1).click();
    await expect(clients.phonesRows).toHaveCount(2);
    await expect(clients.phoneInput(0)).toHaveValue('1110000000');
    await expect(clients.phoneInput(1)).toHaveValue('3330000000');

    // осталась ровно одна строка (data-row-min=1) — «удаление» очищает, а не убирает её.
    await clients.phoneDel(1).click();
    await expect(clients.phonesRows).toHaveCount(1);
    await clients.phoneDel(0).click();
    await expect(clients.phonesRows).toHaveCount(1);
    await expect(clients.phoneInput(0)).toHaveValue('');

    await clients.fillPhone(0, { phone: '1110000000', note: 'первый' });
    await clients.save.click();
    await expect(clients.flash).toHaveText('Клиент создан');

    await clients.goto(`?q=${name}`);
    await expect(clients.row(name)).toContainText('1110000000');
  });
});

test.describe('CL: список, поиск, правка, удаление', () => {
  test('CL-005: поиск живёт в URL — reload и «назад» сохраняют', async ({ adminPage: page, adminApi, uniq }) => {
    const tag = uniq('search-tag');
    await adminApi.createClient({ company_name: `Найди ${tag} Один`, phones: [{ phone: '1112223344', note: '' }] });
    await adminApi.createClient({ company_name: `Другой Клиент ${tag}`, phones: [{ phone: '5556667788', note: '' }] });
    const clients = new ClientsPage(page);
    await clients.goto();

    await clients.search.fill(tag);
    await clients.search.press('Enter');
    await expect(page).toHaveURL(new RegExp(`q=${tag}`));
    await expect(clients.table).toContainText(tag);

    await page.reload();
    await expect(clients.search).toHaveValue(tag);

    await page.goto('/clients');
    await expect(clients.search).toHaveValue('');

    await page.goBack();
    await expect(clients.search).toHaveValue(tag);

    await page.getByTestId('filter-reset').click();
    await expect(page).toHaveURL(/\/clients$/);
  });

  test('CL-006: список отсортирован по названию компании, регистронезависимо (AC-004); колонка «Оборудование» — счётчик единиц', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const tag = uniq('order');
    // созданы в обратном алфавитном порядке, с разным регистром первой буквы — проверяем
    // и сам порядок, и регистронезависимость (`ORDER BY lower(company_name)`, repo.go).
    await adminApi.createClient({ company_name: `cherry ${tag}`, phones: [{ phone: '1000000001', note: '' }] });
    await adminApi.createClient({ company_name: `Apple ${tag}`, phones: [{ phone: '1000000002', note: '' }] });
    await adminApi.createClient({ company_name: `banana ${tag}`, phones: [{ phone: '1000000003', note: '' }] });

    const clients = new ClientsPage(page);
    await clients.goto(`?q=${tag}`);
    const rows = page.getByTestId('client-row');
    await expect(rows).toHaveCount(3);

    const companies = await rows.evaluateAll((els) => els.map((el) => el.getAttribute('data-company')));
    expect(companies).toEqual([`Apple ${tag}`, `banana ${tag}`, `cherry ${tag}`]);

    // единиц оборудования пока 0 — API оборудования появляется в Phase 7, привязка к
    // клиенту и ненулевой счётчик в списке — Phase 8 (EQ-*).
    for (const company of companies) {
      await expect(clients.equipmentCount(company!)).toHaveText('0');
    }
  });

  test('CL-007: правка карточки — поля и телефон обновляются', async ({ adminPage: page, adminApi, uniq }) => {
    const company = uniq('Правка Клиент');
    await adminApi.createClient({
      company_name: company,
      address: 'Старый адрес',
      phones: [{ phone: '1000000000', note: '' }],
    });
    const clients = new ClientsPage(page);
    await clients.goto(`?q=${company}`);
    await clients.open(company);
    await expect(clients.companyName).toHaveValue(company);
    await expect(clients.address).toHaveValue('Старый адрес');

    await clients.fill({ address: 'Новый адрес', contactName: 'Новый Представитель' });
    await clients.fillPhone(0, { phone: '2000000000', note: 'обновлён' });
    await clients.save.click();

    // PRG — на карточку (design-go-layers.md §SSR Routes), не на список.
    await expect(page).toHaveURL(/\/clients\/[^/]+\?flash=saved$/);
    await expect(clients.flash).toHaveText('Изменения сохранены');

    await clients.goto(`?q=${company}`);
    await expect(clients.row(company)).toContainText('2000000000');
    await expect(clients.row(company)).toContainText('Новый Представитель');
  });

  test('CL-008 (TS-043): удаление клиента — окно «Удаление» с причиной, PRG на список и тост; негатив: без причины клиент не удаляется — «Укажите причину удаления»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const company = uniq('Удаляемый Клиент');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '1234567890', note: '' }] });
    // отказ по причине — серверная форма возвращается с 422 (soft-delete-clients §8.1).
    allowHttpErrors((url, status) => url.endsWith(`/clients/${client.id}/delete`) && status === 422);
    try {
      const clients = new ClientsPage(page);
      await clients.goto(`?q=${company}`);
      await clients.open(company);

      // негатив: пустая причина — форма открывается снова с окном и текстом под полем.
      await clients.openDelete();
      await expect(clients.deleteDialogTitle).toHaveText(`Удалить «${company}»?`);
      await clients.deleteDialogOk.click();
      await expect(clients.deleteReasonError).toHaveText('Укажите причину удаления');
      await expect(clients.deleteDialog).toBeVisible();
      expect((await adminApi.getClient(client.id)).id, 'клиент не удалён').toBe(client.id);

      await clients.deleteReason.fill(uniq('CL-008 создан по ошибке'));
      await clients.deleteDialogOk.click();
      await expect(page).toHaveURL(/\/clients\?flash=deleted&n=0$/);
      await expect(clients.toast).toHaveText('Удалено вместе с оборудованием: 0. Восстановить можно в разделе «Удалённые»');
      await expect(clients.row(company)).toHaveCount(0);
      expect((await adminApi.listDeleted('clients', { q: company })).total).toBe(1);
    } finally {
      await adminApi.purgeClient(client.id); // TS-F9: мягко удалённый клиент не копится в «Удалённых»
    }
  });
});

test.describe('CL: права исполнителя', () => {
  test('CL-010: исполнитель не видит «Клиенты» в меню и получает «Нет доступа» по прямым URL', async ({
    page,
    makeUser,
    uniq,
    loginAs,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.includes('/clients') && status === 403);
    const exec = await makeUser({ login: uniq('exec-clients') });
    await loginAs(page, exec.login, exec.password);

    await page.goto('/');
    await expect(page.getByTestId('sidebar').getByRole('link', { name: 'Клиенты' })).toHaveCount(0);

    await page.goto('/clients');
    await expect(page.getByTestId('status-page')).toContainText('Нет доступа');
    await page.goto('/clients/new');
    await expect(page.getByTestId('status-page')).toContainText('Нет доступа');
  });
});

// Phase 9: хардening — конкурентность (CL-020), XSS (CL-011), adaptive/themes (CL-021).
test.describe('CL: хардening', () => {
  test('CL-011: HTML/скрипт в компании, телефоне, модели, характеристике и названии типа — отображается текстом, не исполняется', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const payload = '<img src=x onerror=window.xssTriggered=1>';
    const clientTag = uniq('xss-client');
    const typeTag = uniq('xss-type');
    const company = `${payload} ${clientTag}`;
    const typeName = `${payload} ${typeTag}`;

    const client = await adminApi.createClient({
      company_name: company,
      phones: [{ phone: '9990002222', note: payload }],
    });
    const type = await adminApi.createEquipmentType({ name: typeName, spec_keys: [payload] });
    await adminApi.createEquipment(client.id, {
      type_id: type.id,
      model: payload,
      specs: [{ k: payload, v: payload }],
    });

    const noXss = () => expect(page.evaluate(() => (window as unknown as { xssTriggered?: unknown }).xssTriggered)).resolves.toBeUndefined();

    // список клиентов — компания и примечание телефона текстом.
    const clients = new ClientsPage(page);
    await clients.goto(`?q=${clientTag}`);
    await expect(clients.row(company)).toContainText(company);
    await expect(clients.row(company)).toContainText(payload);
    await noXss();

    // карточка клиента — значение поля формы и таблица оборудования текстом.
    await clients.open(company);
    await expect(clients.companyName).toHaveValue(company);
    await expect(page.getByTestId('equipment-table')).toContainText(payload);
    await noXss();

    // карточка единицы оборудования — модель, тип (комбобокс), характеристика текстом.
    await page.getByTestId('equipment-table').getByRole('link').first().click();
    await expect(page.locator('.app-title')).toHaveText('Оборудование');
    const eq = new EquipmentPage(page);
    await expect(eq.model).toHaveValue(payload);
    await expect(eq.typeButton).toContainText(payload);
    await expect(eq.specKey(0)).toHaveValue(payload);
    await expect(eq.specValue(0)).toHaveValue(payload);
    await noXss();

    // список типов оборудования — название типа текстом.
    const types = new EquipmentTypesPage(page);
    await types.goto(`?q=${typeTag}`);
    await expect(types.row(typeName)).toContainText(typeName);
    await noXss();
  });

  test('CL-020: два браузерных контекста правят одну карточку — победитель сохраняется, проигравший видит конфликт и не теряет ввод', async ({
    adminPage: page,
    adminApi,
    uniq,
    browser,
    runAdmin,
    loginAs,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.includes('/clients/') && status === 409);
    const company = uniq('Конкурентный Клиент');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990004444', note: '' }] });

    const ctx2 = await browser.newContext({ colorScheme: 'light' });
    const page2 = await ctx2.newPage();
    await loginAs(page2, runAdmin.login, runAdmin.password);

    const one = new ClientsPage(page);
    const two = new ClientsPage(page2);
    await one.goto(`?q=${company}`);
    await one.open(company);
    await two.goto(`?q=${company}`);
    await two.open(company);

    // оба открыли карточку со старой version — правят разные поля, сохраняются по очереди.
    await one.fill({ companyName: `${company} Победитель` });
    await two.fill({ companyName: `${company} Проигравший` });

    await one.save.click();
    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=saved$`));

    // второй сохраняет со старым version — 409, форма перерисована с его вводом, не потеряна.
    await two.save.click();
    await expect(page2).toHaveURL(new RegExp(`/clients/${client.id}$`));
    await expect(two.formError).toContainText('Карточку уже изменил другой пользователь — ваши правки не сохранены');
    await expect(two.conflictReload).toBeVisible();
    await expect(two.companyName).toHaveValue(`${company} Проигравший`);

    // карточка в БД содержит правку победителя, не проигравшего.
    const winner = await adminApi.getClient(client.id);
    expect(winner.company_name).toBe(`${company} Победитель`);

    // «Открыть актуальную версию» ведёт к данным победителя.
    await two.conflictReload.click();
    await expect(two.companyName).toHaveValue(`${company} Победитель`);

    await ctx2.close();
  });

  test('CL-021: 360×640 и 1280×800 — список и карточка (телефоны + оборудование) без overflow; тёмная тема без overflow', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Адаптив Клиент Оборудование');
    const client = await adminApi.createClient({
      company_name: company,
      address: 'г. Москва, очень длинный адрес для проверки переноса строк в узкой карточке, дом 1, офис 100500',
      contact_name: 'Очень-Длинное-Имя-Представителя-Без-Пробелов Иванович',
      contact_position: 'Генеральный директор по особо важным вопросам снабжения и логистики',
      phones: [
        { phone: '+7 (999) 111-22-33', note: 'основной номер телефона с длинным примечанием' },
        { phone: '+7 (999) 444-55-66', note: 'запасной номер' },
        { phone: '9997778899', note: '' },
      ],
    });
    const type = await adminApi.createEquipmentType({ name: uniq('Тип Адаптив Клиент') });
    await adminApi.createEquipment(client.id, { type_id: type.id, model: 'Очень-Длинное-Название-Модели-Без-Пробелов-XL9000' });
    await adminApi.createEquipment(client.id, { type_id: type.id, model: 'Вторая единица оборудования клиента' });

    const clients = new ClientsPage(page);
    const noOverflow = () => page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);

    for (const [width, height] of [
      [360, 640],
      [1280, 800],
    ] as const) {
      await page.setViewportSize({ width, height });
      await clients.goto(`?q=${company}`);
      await expect(clients.row(company)).toBeVisible();
      expect(await noOverflow()).toBe(false);

      await clients.open(company);
      await expect(page.getByTestId('equipment-table')).toBeVisible();
      expect(await noOverflow()).toBe(false);
    }

    // тёмная тема (themes:both) — ключевые элементы видимы, без overflow.
    await page.addInitScript(() => localStorage.setItem('kt-theme', 'dark'));
    await page.setViewportSize({ width: 360, height: 640 });
    await clients.goto(`?q=${company}`);
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');
    await expect(clients.row(company)).toBeVisible();
    expect(await noOverflow()).toBe(false);

    await clients.open(company);
    await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');
    await expect(clients.form).toBeVisible();
    await expect(page.getByTestId('equipment-table')).toBeVisible();
    expect(await noOverflow()).toBe(false);
  });
});
