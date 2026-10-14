import { test, expect } from '../fixtures/test';
import { ClientsPage } from '../pages/ClientsPage';
import { EquipmentPage } from '../pages/EquipmentPage';

// Фаза 8: форма единицы оборудования — /clients/:id/equipment/new, /equipment/:id
// (AC-007..AC-010, AC-012 equipment-часть, ADR-002 подстановка шаблона характеристик,
// ADR-004 архивный тип). EQ-021 (адаптив) — Phase 9.
test.use({ colorScheme: 'light' });

test.describe('EQ: создание и валидация', () => {
  test('EQ-001: создание оборудования — выбор типа через комбобокс, карточка клиента показывает единицу и счётчик', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const typeName = uniq('Компрессор');
    await adminApi.createEquipmentType({ name: typeName, spec_keys: ['Мощность'] });
    const company = uniq('Оборудование Клиент');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000001', note: '' }] });

    const clients = new ClientsPage(page);
    await clients.goto(`?q=${company}`);
    await clients.open(company);
    await page.getByTestId('equipment-add').click();
    await expect(page.locator('.app-title')).toHaveText('Новое оборудование');

    const eq = new EquipmentPage(page);
    await expect(eq.clientName).toHaveText(company);
    await eq.selectType(typeName);
    await eq.fill({ brand: 'Danfoss', model: 'M100', sku: 'SKU1', serial: 'SN1' });
    await eq.save.click();

    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_created$`));
    await expect(page.getByTestId('equipment-table')).toContainText('M100');
    await expect(page.getByTestId('equipment-table')).toContainText(typeName);

    await clients.goto(`?q=${company}`);
    await expect(clients.equipmentCount(company)).toHaveText('1');
  });

  test('EQ-002: тип и модель обязательны — ошибки под полями, исправление проходит', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.includes('/equipment') && status === 422);
    const company = uniq('Валидация Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000002', note: '' }] });
    const typeName = uniq('Тип Валидации');
    await adminApi.createEquipmentType({ name: typeName });

    const eq = new EquipmentPage(page);
    await eq.gotoNew(client.id);

    await expect(eq.model).toHaveAttribute('maxlength', '200');
    await expect(eq.brand).toHaveAttribute('maxlength', '200');
    await expect(eq.sku).toHaveAttribute('maxlength', '200');
    await expect(eq.serial).toHaveAttribute('maxlength', '200');
    await expect(eq.note).toHaveAttribute('maxlength', '1000');

    // пустая форма — тип и модель обязательны.
    await eq.save.click();
    await expect(eq.fieldError('type_id')).toHaveText('Выберите тип');
    await expect(eq.fieldError('model')).toHaveText('Обязательное поле');

    // исправление — форма проходит.
    await eq.selectType(typeName);
    await eq.fill({ model: 'Модель X' });
    await eq.save.click();
    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_created$`));
  });
});

test.describe('EQ: правка, шаблон характеристик, свои характеристики', () => {
  test('EQ-003: правка карточки оборудования — поля обновляются, PRG на карточку клиента', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Правка Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000003', note: '' }] });
    const type = await adminApi.createEquipmentType({ name: uniq('Тип Правки') });
    const equipment = await adminApi.createEquipment(client.id, { type_id: type.id, model: 'Старая модель' });

    const eq = new EquipmentPage(page);
    await eq.gotoEdit(equipment.id);
    await expect(eq.model).toHaveValue('Старая модель');
    await expect(eq.clientName).toHaveText(company);

    await eq.fill({ model: 'Новая модель', brand: 'НовыйБренд' });
    await eq.save.click();

    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_saved$`));
    await expect(page.getByTestId('equipment-table')).toContainText('Новая модель');
  });

  test('EQ-004: смена типа подставляет шаблон характеристик — заполненные значения сохраняются, пустые заменяются (AC-009)', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Шаблон Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000004', note: '' }] });
    const typeA = uniq('Тип А');
    const typeB = uniq('Тип Б');
    await adminApi.createEquipmentType({ name: typeA, spec_keys: ['Мощность', 'Напряжение'] });
    await adminApi.createEquipmentType({ name: typeB, spec_keys: ['Объём'] });

    const eq = new EquipmentPage(page);
    await eq.gotoNew(client.id);
    await eq.fill({ model: 'Модель шаблона' });

    await eq.selectType(typeA);
    await expect(eq.specsRows).toHaveCount(2);
    await expect(eq.specKey(0)).toHaveValue('Мощность');
    await expect(eq.specKey(1)).toHaveValue('Напряжение');
    await eq.fillSpec(0, { value: '5 кВт' });

    // смена типа: заполненная «Мощность» сохраняется, пустая «Напряжение» заменяется
    // характеристикой шаблона нового типа «Объём».
    await eq.selectType(typeB);
    await expect(eq.specsRows).toHaveCount(2);
    await expect(eq.specKey(0)).toHaveValue('Мощность');
    await expect(eq.specValue(0)).toHaveValue('5 кВт');
    await expect(eq.specKey(1)).toHaveValue('Объём');

    await eq.save.click();
    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_created$`));

    // сохранение подтверждаем повторным открытием формы правки.
    await page.getByTestId('equipment-table').getByRole('link', { name: 'Модель шаблона' }).click();
    await expect(page.locator('.app-title')).toHaveText('Оборудование');
    await expect(eq.specsRows).toHaveCount(2);
    await expect(eq.specKey(0)).toHaveValue('Мощность');
    await expect(eq.specValue(0)).toHaveValue('5 кВт');
    await expect(eq.specKey(1)).toHaveValue('Объём');
  });

  test('EQ-005: своя характеристика — добавление/удаление строки; значение без названия не проходит тихо (key_required)', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.includes('/equipment') && status === 422);
    const company = uniq('Своя Характеристика');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000005', note: '' }] });
    const typeName = uniq('Тип Своей Характеристики');
    await adminApi.createEquipmentType({ name: typeName });

    const eq = new EquipmentPage(page);
    await eq.gotoNew(client.id);
    await eq.fill({ model: 'Модель Своя' });
    await eq.selectType(typeName);
    await expect(eq.specsRows).toHaveCount(1);

    await eq.specAdd.click();
    await expect(eq.specsRows).toHaveCount(2);
    await eq.specDel(1).click();
    await expect(eq.specsRows).toHaveCount(1);

    // значение без ключа — сервер не принимает тихо, а возвращает key_required.
    await eq.fillSpec(0, { value: '5' });
    await eq.save.click();
    await expect(eq.specsRows).toHaveCount(1);
    await expect(eq.specKeyError(0)).toHaveText('Укажите характеристику');
    await expect(eq.specValue(0)).toHaveValue('5');

    await eq.fillSpec(0, { key: 'Своя характеристика' });
    await eq.save.click();
    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_created$`));
  });
});

test.describe('EQ: удаление, архивный тип', () => {
  test('EQ-006 (TS-043): удаление единицы оборудования — окно «Удаление» с причиной, PRG на карточку клиента и тост; негатив: без причины единица не удаляется', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const company = uniq('Удаление Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000006', note: '' }] });
    const type = await adminApi.createEquipmentType({ name: uniq('Тип Удаления') });
    const equipment = await adminApi.createEquipment(client.id, { type_id: type.id, model: 'Удаляемая модель' });
    allowHttpErrors((url, status) => url.endsWith(`/equipment/${equipment.id}/delete`) && status === 422);
    try {
      const eq = new EquipmentPage(page);
      await eq.gotoEdit(equipment.id);

      // негатив: пустая причина — форма единицы открывается снова с окном и текстом под полем.
      await eq.openDelete();
      await expect(eq.deleteDialogTitle).toHaveText('Удалить «Удаляемая модель»?');
      await expect(eq.deleteDialogText).toHaveText('Единицу можно восстановить в разделе «Удалённые»');
      await eq.deleteDialogOk.click();
      await expect(eq.deleteReasonError).toHaveText('Укажите причину удаления');
      await expect(eq.deleteDialog).toBeVisible();
      expect((await adminApi.getClient(client.id)).equipment_count, 'единица не удалена').toBe(1);

      await eq.deleteReason.fill(uniq('EQ-006 списана'));
      await eq.deleteDialogOk.click();
      await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_deleted$`));
      await expect(eq.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
      await expect(page.getByTestId('equipment-empty')).toBeVisible();
      expect((await adminApi.getClient(client.id)).equipment_count, 'счётчик — только действующие единицы').toBe(0);
    } finally {
      await adminApi.purgeClient(client.id); // TS-F9: клиент уходит вместе с удалённой единицей
    }
  });

  test('EQ-007: архивный тип недоступен для новой единицы, но сохраняется у существующей (ADR-004)', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Архивный Тип Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000007', note: '' }] });
    const typeName = uniq('Тип В Архиве');
    const type = await adminApi.createEquipmentType({ name: typeName });
    const equipment = await adminApi.createEquipment(client.id, { type_id: type.id, model: 'Оборудование Архивного Типа' });
    await adminApi.updateEquipmentType(type.id, { name: typeName, spec_keys: [], is_active: false });

    const eq = new EquipmentPage(page);
    await eq.gotoNew(client.id);
    await eq.typeButton.click();
    await expect(page.getByTestId('combobox-dropdown').locator('.cb-opt', { hasText: typeName })).toHaveCount(0);
    await page.keyboard.press('Escape');

    await eq.gotoEdit(equipment.id);
    await expect(eq.typeButton).toContainText(typeName);
    await expect(eq.typeButton).toContainText('в архиве');
  });
});

// Phase 9: хардение — adaptive:min.
test.describe('EQ: адаптив', () => {
  test('EQ-021: 360×640 и 1280×800 — форма единицы (несколько характеристик) без горизонтального overflow', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Адаптив Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990005555', note: '' }] });
    const typeName = uniq('Тип Адаптив Оборудование Много Характеристик');
    await adminApi.createEquipmentType({
      name: typeName,
      spec_keys: [
        'Очень-Длинное-Название-Характеристики-Без-Пробелов-Раз',
        'Очень-Длинное-Название-Характеристики-Без-Пробелов-Два',
        'Мощность',
        'Напряжение',
        'Объём бака',
      ],
    });

    const eq = new EquipmentPage(page);
    for (const [width, height] of [
      [360, 640],
      [1280, 800],
    ] as const) {
      await page.setViewportSize({ width, height });
      await eq.gotoNew(client.id);
      await eq.selectType(typeName);
      await expect(eq.specsRows).toHaveCount(5);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
    }
  });
});
