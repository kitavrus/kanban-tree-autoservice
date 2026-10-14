import { test, expect } from '../fixtures/test';
import { ClientsPage } from '../pages/ClientsPage';
import { EquipmentPage } from '../pages/EquipmentPage';
import { uniqPlate } from '../pages/NewOrderDialog';

// Форма автомобиля — /clients/:id/equipment/new, /equipment/:id. Автомобиль — только госномер:
// единственное поле «Госномер *» (марки, модели, VIN, характеристик, примечания и типа больше нет).
// EQ-021 (адаптив) — Phase 9.
test.use({ colorScheme: 'light' });

test.describe('EQ: создание и валидация', () => {
  test('EQ-001: создание автомобиля — карточка клиента показывает госномер ссылкой и счётчик', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Оборудование Клиент');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000001', note: '' }] });
    const plate = uniqPlate();

    const clients = new ClientsPage(page);
    await clients.goto(`?q=${company}`);
    await clients.open(company);
    await page.getByTestId('equipment-add').click();
    await expect(page.locator('.app-title')).toHaveText('Новое оборудование');

    const eq = new EquipmentPage(page);
    await expect(eq.clientName).toHaveText(company);
    // в форме только госномер.
    await expect(page.locator('#brand, #model, #sku, #serial, #note')).toHaveCount(0);
    await expect(page.getByTestId('spec-row')).toHaveCount(0);
    await expect(page.getByTestId('equipment-type')).toHaveCount(0);
    await eq.plate.fill(plate.toLowerCase());
    await eq.save.click();

    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_created$`));
    // таблица «Оборудование клиента» — госномер (ссылка на /equipment/:id) и действия.
    const row = page.locator(`[data-testid="equipment-row"][data-plate-number="${plate}"]`);
    await expect(row).toBeVisible();
    await expect(row.getByTestId('equipment-plate-number')).toHaveText(plate);
    await expect(row.getByTestId('equipment-plate-number').getByRole('link')).toHaveAttribute('href', /^\/equipment\/[^/]+$/);

    await clients.goto(`?q=${company}`);
    await expect(clients.equipmentCount(company)).toHaveText('1');
  });

  test('EQ-002: госномер обязателен — «Укажите госномер» под полем, исправление проходит', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.includes('/equipment') && status === 422);
    const company = uniq('Валидация Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000002', note: '' }] });

    const eq = new EquipmentPage(page);
    await eq.gotoNew(client.id);
    await expect(eq.plate).toHaveAttribute('maxlength', '20');

    await eq.save.click();
    await expect(eq.fieldError('plate_number')).toHaveText('Укажите госномер');
    await expect(eq.plate).toHaveClass(/is-invalid/);

    await eq.plate.fill(uniqPlate());
    await eq.save.click();
    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_created$`));
  });
});

test.describe('EQ: правка', () => {
  test('EQ-003: правка госномера — PRG на карточку клиента, новый госномер в таблице', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Правка Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000003', note: '' }] });
    const oldPlate = uniqPlate();
    const equipment = await adminApi.createEquipment(client.id, { plate_number: oldPlate });

    const eq = new EquipmentPage(page);
    await eq.gotoEdit(equipment.id);
    await expect(eq.plate).toHaveValue(oldPlate);
    await expect(eq.clientName).toHaveText(company);

    const newPlate = uniqPlate();
    await eq.plate.fill(newPlate);
    await eq.save.click();

    await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_saved$`));
    await expect(page.locator(`[data-testid="equipment-row"][data-plate-number="${newPlate}"]`)).toBeVisible();
    await expect(page.locator(`[data-testid="equipment-row"][data-plate-number="${oldPlate}"]`)).toHaveCount(0);
  });
});

test.describe('EQ: удаление', () => {
  test('EQ-006 (TS-043): удаление автомобиля — окно «Удаление» с причиной, PRG на карточку клиента и тост; негатив: без причины автомобиль не удаляется', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    const company = uniq('Удаление Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990000006', note: '' }] });
    const plate = uniqPlate();
    const equipment = await adminApi.createEquipment(client.id, { plate_number: plate });
    allowHttpErrors((url, status) => url.endsWith(`/equipment/${equipment.id}/delete`) && status === 422);
    try {
      const eq = new EquipmentPage(page);
      await eq.gotoEdit(equipment.id);

      // негатив: пустая причина — форма открывается снова с окном и текстом под полем.
      await eq.openDelete();
      await expect(eq.deleteDialogTitle).toHaveText(`Удалить «${plate}»?`);
      await expect(eq.deleteDialogText).toHaveText('Единицу можно восстановить в разделе «Удалённые»');
      await eq.deleteDialogOk.click();
      await expect(eq.deleteReasonError).toHaveText('Укажите причину удаления');
      await expect(eq.deleteDialog).toBeVisible();
      expect((await adminApi.getClient(client.id)).equipment_count, 'автомобиль не удалён').toBe(1);

      await eq.deleteReason.fill(uniq('EQ-006 списан'));
      await eq.deleteDialogOk.click();
      await expect(page).toHaveURL(new RegExp(`/clients/${client.id}\\?flash=eq_deleted$`));
      await expect(eq.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
      await expect(page.getByTestId('equipment-empty')).toBeVisible();
      expect((await adminApi.getClient(client.id)).equipment_count, 'счётчик — только действующие').toBe(0);
    } finally {
      await adminApi.purgeClient(client.id); // TS-F9: клиент уходит вместе с удалённым автомобилем
    }
  });
});

// Phase 9: хардение — adaptive:min.
test.describe('EQ: адаптив', () => {
  test('EQ-021: 360×640 и 1280×800 — форма автомобиля без горизонтального overflow', async ({
    adminPage: page,
    adminApi,
    uniq,
  }) => {
    const company = uniq('Адаптив Оборудование');
    const client = await adminApi.createClient({ company_name: company, phones: [{ phone: '9990005555', note: '' }] });
    const equipment = await adminApi.createEquipment(client.id, { plate_number: uniqPlate() });

    const eq = new EquipmentPage(page);
    for (const [width, height] of [
      [360, 640],
      [1280, 800],
    ] as const) {
      await page.setViewportSize({ width, height });
      await eq.gotoNew(client.id);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
      await eq.gotoEdit(equipment.id);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
    }
  });
});
