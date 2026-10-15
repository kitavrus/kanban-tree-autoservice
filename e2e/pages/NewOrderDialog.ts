import { expect, type Locator, type Page } from '@playwright/test';

/** Уникальный госномер теста: только буквы/цифры (правило clients.PlateNumberProblem), ≤20. */
export function uniqPlate(): string {
  return `T${Date.now().toString(36)}${Math.floor(Math.random() * 1296).toString(36)}`.toUpperCase().slice(0, 20);
}

// POM диалога «Новый заказ» (js/task-card.js, раздел «Новый заказ по госномеру»): первое
// поле — госномер, поиск автомобиля GET /api/v1/equipment?plate_number=; найден — клиент
// подставляется и блокируется; не найден — мини-форма «Новый автомобиль» (+ «Новый клиент»).
// Комбобоксы — js/combobox.js переносит testid на видимую кнопку.
export class NewOrderDialog {
  readonly plate: Locator;
  readonly plateStatus: Locator;
  readonly plateError: Locator;
  readonly carFound: Locator;
  readonly carOptions: Locator;
  readonly carError: Locator;
  readonly newCar: Locator;
  readonly client: Locator;
  readonly clientLocked: Locator;
  readonly newClientToggle: Locator;
  readonly newClientName: Locator;
  readonly newClientPhone: Locator;
  readonly titleField: Locator;
  readonly dropdown: Locator;
  readonly plateSuggestions: Locator;
  readonly plateSuggestion: Locator;
  readonly clientNewAction: Locator;
  readonly start: Locator;
  readonly duration: Locator;
  readonly plannedEnd: Locator;
  readonly startError: Locator;
  readonly durationError: Locator;
  readonly capacity: Locator;
  readonly capacityWarning: Locator;

  constructor(readonly page: Page) {
    // План работ (order-start-duration): «Начать с» + «Дней на работу», подсказка планового окончания.
    this.start = page.getByTestId('task-start');
    this.duration = page.getByTestId('task-duration');
    this.plannedEnd = page.getByTestId('task-planned-end');
    this.startError = page.getByTestId('task-start-error');
    this.durationError = page.getByTestId('task-duration-error');
    this.capacity = page.getByTestId('task-capacity');
    this.capacityWarning = page.getByTestId('task-capacity-warning');
    this.plate = page.getByTestId('task-plate');
    this.plateStatus = page.getByTestId('task-plate-status');
    this.plateError = page.getByTestId('task-plate-error');
    this.carFound = page.getByTestId('task-car-found');
    this.carOptions = page.getByTestId('task-car-option');
    this.carError = page.getByTestId('task-car-error');
    this.newCar = page.getByTestId('task-new-car');
    this.client = page.getByTestId('task-client');
    this.clientLocked = page.getByTestId('task-client-locked');
    this.newClientToggle = page.getByTestId('task-new-client-toggle');
    this.newClientName = page.getByTestId('task-new-client-name');
    this.newClientPhone = page.getByTestId('task-new-client-phone');
    this.titleField = page.getByTestId('task-title');
    this.dropdown = page.getByTestId('combobox-dropdown');
    this.plateSuggestions = page.getByTestId('task-plate-suggestions');
    this.plateSuggestion = page.getByTestId('task-plate-suggestion');
    this.clientNewAction = page.getByTestId('task-client-new-action');
  }

  /** Ввод госномера; ждёт окончания поиска (статус не «Поиск…»). */
  async fillPlate(plate: string) {
    await this.plate.fill(plate);
    await this.plate.blur();
    await expect(this.plateStatus).not.toHaveText(/Поиск|Введите/);
  }

  /** Госномер существующего автомобиля: ждёт сводку «найден». */
  async useFoundCar(plate: string) {
    await this.fillPlate(plate);
    await expect(this.carFound).toBeVisible();
  }

  /** Госномер нового автомобиля: ждёт блок «Новый автомобиль» (полей в нём нет — автомобиль = госномер). */
  async fillNewCar(plate: string) {
    await this.fillPlate(plate);
    await expect(this.newCar).toBeVisible();
  }

  /** Существующий клиент нового автомобиля (комбобокс «Клиент»). */
  async chooseClient(name: string) {
    await this.client.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-opt', { hasText: name }).first().click();
  }

  /** Поиск клиента в комбобоксе «Клиент» по фрагменту (имя / телефон) и выбор пункта. */
  async searchClient(fragment: string, name: string) {
    await this.client.click();
    await expect(this.dropdown).toBeVisible();
    await this.dropdown.locator('.cb-search').fill(fragment);
    await this.dropdown.locator('.cb-opt', { hasText: name }).first().click();
    await expect(this.dropdown).toBeHidden();
  }

  /** Новый клиент нового автомобиля (поля открываются сами, если клиент не выбран). */
  async fillNewClient(name: string, phone: string) {
    await this.newClientToggle.check();
    await this.newClientName.fill(name);
    await this.newClientPhone.fill(phone);
  }
}
