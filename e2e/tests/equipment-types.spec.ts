import { test, expect } from '../fixtures/test';
import { EquipmentTypesPage } from '../pages/EquipmentTypesPage';

// Фаза 4: экран «Каталог → Типы оборудования» (admin) — AC-011, AC-012, ADR-006/007.
// ET-021 (адаптив) — Phase 9.
// Уборка (soft-delete-catalog TS-F10): типы, созданные через adminApi, убирает autouse-фикстура
// catalogCleanup служебным адресом purge; созданные через экран тест регистрирует сам.
test.use({ colorScheme: 'light' });

test.describe('ET: типы оборудования', () => {
  test('ET-001: админ создаёт тип через форму, тот виден в списке', async ({ adminPage: page, adminApi, uniq, catalogCleanup }) => {
    const name = uniq('Чиллер');
    const types = new EquipmentTypesPage(page);
    await types.goto();
    await types.addButton.click();
    await expect(page.locator('.app-title')).toHaveText('Новый тип оборудования');

    await types.fill({ name, specKeys: 'Мощность, Хладагент, Объём бака' });
    await types.save.click();

    await expect(page).toHaveURL(/\/catalog\/equipment-types\?flash=created$/);
    await expect(types.flash).toHaveText('Тип оборудования создан');
    const created = await adminApi.findCatalogByName('equipment-types', name);
    expect(created, 'тип создан через экран').toBeTruthy();
    catalogCleanup('equipment-types', created!.id);
    // редирект после создания не сохраняет q — на наполненной БД список без фильтра
    // может быть обрезан лимитом (§TD-6.2), проверяем свою запись через ?q=.
    await types.goto(`?q=${encodeURIComponent(name)}`);
    await expect(types.row(name)).toContainText(name);
    await expect(types.row(name)).toContainText('Мощность, Хладагент, Объём бака');
  });

  test('ET-002: дубль имени активного и архивного типа — ошибка под полем + ссылка «Показать архивные»', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
  }) => {
    allowHttpErrors((url, status) => url.endsWith('/catalog/equipment-types') && (status === 409 || status === 422));
    const name = uniq('Компрессор');
    const active = await adminApi.createEquipmentType({ name });
    const types = new EquipmentTypesPage(page);

    await types.gotoNew();
    await types.fill({ name });
    await types.save.click();
    await expect(types.fieldError('name')).toHaveText('Такой тип уже есть');
    await expect(types.name).toHaveValue(name); // введённое сохраняется

    await adminApi.updateEquipmentType(active.id, { name, is_active: false });
    await types.gotoNew();
    await types.fill({ name });
    await types.save.click();
    await expect(types.fieldError('name')).toHaveText('Такой тип уже есть в архиве — верните его из архива');
    await expect(types.archivedLink).toBeVisible();
    await types.archivedLink.click();
    await expect(page).toHaveURL(/archived=1/);
    await expect(types.row(name)).toBeVisible();
  });

  test('ET-003 (TS-030): тип удаляется с причиной — список без типа, тост «Удалено. Восстановить можно в разделе «Удалённые»»; окно без причины — «Укажите причину удаления», тип не удалён; архивирование из списка — отдельное действие', async ({
    adminPage: page,
    adminApi,
    uniq,
    allowHttpErrors,
    catalogCleanup,
  }) => {
    const free = uniq('Свободный тип');
    const types = new EquipmentTypesPage(page);
    await types.gotoNew();
    await types.fill({ name: free });
    await types.save.click();
    await expect(page).toHaveURL(/flash=created$/);
    const created = await adminApi.findCatalogByName('equipment-types', free);
    expect(created, 'тип создан через экран').toBeTruthy();
    catalogCleanup('equipment-types', created!.id);
    // серверная форма удаления без причины отвечает 422 на саму навигацию.
    allowHttpErrors((url, status) => url.endsWith(`/catalog/equipment-types/${created!.id}/delete`) && status === 422);
    // редирект после создания не сохраняет q — на наполненной БД список без фильтра
    // может быть обрезан лимитом (§TD-6.2).
    await types.goto(`?q=${encodeURIComponent(free)}`);
    await types.open(free);

    await types.openDelete();
    await expect(types.deleteDialogTitle).toHaveText(`Удалить «${free}»?`);
    await expect(types.deleteDialogText).toHaveText(
      'Запись можно восстановить в разделе «Удалённые». Чтобы только скрыть её из выбора, используйте «В архив»',
    );
    // негатив: без причины окно остаётся с ошибкой под полем, тип не удалён.
    await types.deleteDialogOk.click();
    await expect(types.deleteReasonError).toHaveText('Укажите причину удаления');
    await expect(types.deleteDialog).toBeVisible();
    expect((await adminApi.findCatalogByName('equipment-types', free))?.id, 'тип не удалён').toBe(created!.id);

    await types.deleteReason.fill(uniq('ET-003 создан по ошибке'));
    await types.deleteDialogOk.click();
    await expect(page).toHaveURL(/\/catalog\/equipment-types\?flash=deleted$/);
    await expect(types.toast).toHaveText('Удалено. Восстановить можно в разделе «Удалённые»');
    await types.goto(`?archived=1&q=${encodeURIComponent(free)}`);
    await expect(types.row(free)).toHaveCount(0);

    const toArchive = uniq('Тип для архива');
    await types.gotoNew();
    await types.fill({ name: toArchive });
    await types.save.click();
    await expect(page).toHaveURL(/flash=created$/);
    const archived = await adminApi.findCatalogByName('equipment-types', toArchive);
    expect(archived, 'тип создан через экран').toBeTruthy();
    catalogCleanup('equipment-types', archived!.id);
    await types.goto(`?q=${toArchive}`);
    await types.rowToggle(toArchive).click();
    await expect(types.flash).toHaveText('Тип отправлен в архив');
    // архивирование сохраняет фильтр, с которым отправлена форма (design-go-layers.md
    // §SSR Routes) — активный список без `archived=1` не форсируется в него; тип пропадает
    // из активного списка.
    expect(page.url()).not.toContain('archived=1');
    await expect(types.row(toArchive)).toHaveCount(0);

    await types.goto(`?archived=1&q=${toArchive}`);
    await expect(types.row(toArchive).getByTestId('type-archived-badge')).toHaveText('в архиве');
  });

  test('ET-004: «Показать архивные» + «Вернуть» — archived=1 в URL переживает reload', async ({ adminPage: page, adminApi, uniq }) => {
    const name = uniq('Архивный тип');
    const created = await adminApi.createEquipmentType({ name });
    await adminApi.updateEquipmentType(created.id, { name, is_active: false });

    const types = new EquipmentTypesPage(page);
    await types.goto();
    await expect(types.row(name)).toHaveCount(0);

    await types.archivedFilter.check();
    await expect(page).toHaveURL(/archived=1/);
    await expect(types.row(name)).toBeVisible();

    await page.reload();
    await expect(types.archivedFilter).toBeChecked();
    await expect(types.row(name)).toBeVisible();

    await page.goBack();
    await expect(types.row(name)).toHaveCount(0);

    await page.goForward();
    await types.rowToggle(name).click();
    await expect(types.flash).toHaveText('Тип возвращён из архива');
    await expect(types.row(name)).toBeVisible();
  });
});

// Phase 9: хардение — adaptive:min.
test.describe('ET: адаптив', () => {
  test('ET-021: 360×640 и 1280×800 — список типов оборудования без горизонтального overflow', async ({ adminPage: page, adminApi, uniq }) => {
    const name = uniq('Очень-Длинное-Название-Типа-Оборудования-Без-Пробелов-XL');
    await adminApi.createEquipmentType({
      name,
      spec_keys: ['Очень-Длинная-Характеристика-Раз-Без-Пробелов', 'Очень-Длинная-Характеристика-Два-Без-Пробелов', 'Три'],
    });

    const types = new EquipmentTypesPage(page);
    for (const [width, height] of [
      [360, 640],
      [1280, 800],
    ] as const) {
      await page.setViewportSize({ width, height });
      await types.goto(`?q=${name}`);
      await expect(types.row(name)).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
    }
  });
});
