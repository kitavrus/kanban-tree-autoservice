import { test, expect, columnByName } from '../fixtures/board';
import type { AdminApi } from '../fixtures/auth';
import { BoardPage } from '../pages/BoardPage';
import { OrderCardPage } from '../pages/OrderCardPage';
import { uniqPlate } from '../pages/NewOrderDialog';

// Фаза 7 (D21): вложения заказа в карточке (js/task-card.js, раздел «Вложения»). Файлы
// генерируются в тесте (валидные сигнатуры PNG/MP4/PDF — тип сервер определяет по
// содержимому); лимиты размера на сервере покрыты live-тестом (attachments_http_live_test.go
// LV-205/206), здесь — только клиентская ранняя проверка. Заказы — через adminApi.createTask
// (уборка _boardCleanup); заказ из UI регистрируется вручную.
test.use({ colorScheme: 'light' });

// 1×1 PNG (валидный — миниатюра реально декодируется браузером).
const PNG_1X1 = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==', 'base64');
const MP4_HEAD = Buffer.concat([Buffer.from('000000186674797069736f6d0000020069736f6d6d703432', 'hex'), Buffer.alloc(2048, 7)]);
const PDF_MIN = Buffer.from('%PDF-1.4\n1 0 obj<< /Type /Catalog >>endobj\ntrailer<< /Root 1 0 R >>\n%%EOF\n');

const png = (name: string) => ({ name, mimeType: 'image/png', buffer: PNG_1X1 });
const mp4 = (name: string) => ({ name, mimeType: 'video/mp4', buffer: MP4_HEAD });
const pdf = (name: string) => ({ name, mimeType: 'application/pdf', buffer: PDF_MIN });

async function setupTask(adminApi: AdminApi, uniq: (s: string) => string, label: string) {
  const client = await adminApi.createClient({ company_name: uniq(`AT клиент ${label}`), phones: [{ phone: '9990000700', note: '' }] });
  const columns = (await adminApi.listColumns()).items;
  const task = await adminApi.createTask({ title: uniq(`${label} заказ`), client_id: client.id, column_id: columnByName(columns, 'Приёмка').id });
  return { client, task };
}

async function openCard(page: import('@playwright/test').Page, clientId: string, taskId: number) {
  const board = new BoardPage(page);
  const card = new OrderCardPage(page);
  await board.goto(`?client_id=${clientId}&task=${taskId}`);
  await card.waitOpen();
  await expect(card.attachmentUpload).toBeVisible();
  return card;
}

test.describe('AT: загрузка и просмотр вложений', () => {
  test('AT-001: фото — миниатюра видна, открывается в полном размере', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-001');
    const card = await openCard(page, client.id, task.id);
    await expect(card.attachmentsEmpty).toBeVisible();

    const name = `${uniq('AT-001 фото')}.png`;
    await card.uploadAttachment(png(name));

    const item = card.attachmentByName(name);
    await expect(item).toHaveCount(1);
    await expect(item).toHaveAttribute('data-kind', 'photo');
    const thumb = item.getByTestId('card-attachment-thumb');
    await expect(thumb).toBeVisible();
    await expect.poll(() => thumb.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth)).toBe(1);
    await expect(card.attachmentsEmpty).toHaveCount(0);
    await expect(card.attachmentUpload).not.toHaveClass(/disabled/);

    const open = item.getByTestId('card-attachment-open');
    await expect(open).toHaveAttribute('target', '_blank');
    const [popup] = await Promise.all([page.waitForEvent('popup'), open.click()]);
    await popup.waitForLoadState();
    expect(popup.url()).toMatch(/\/api\/v1\/attachments\/[0-9a-f-]{36}\/file$/);
    await popup.close();

    // переживает перезагрузку карточки
    await card.close();
    await expect(card.modal).toBeHidden();
    const card2 = await openCard(page, client.id, task.id);
    await expect(card2.attachmentByName(name)).toHaveCount(1);
  });

  test('AT-002: видео — встроенный плеер <video controls>', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-002');
    const card = await openCard(page, client.id, task.id);

    const name = `${uniq('AT-002 видео')}.mp4`;
    await card.uploadAttachment(mp4(name));

    const item = card.attachmentByName(name);
    await expect(item).toHaveAttribute('data-kind', 'video');
    const video = item.getByTestId('card-attachment-video');
    await expect(video).toBeVisible();
    await expect(video).toHaveAttribute('controls', '');
    await expect(video).toHaveAttribute('src', /\/api\/v1\/attachments\/[0-9a-f-]{36}\/file$/);
  });

  test('AT-003: PDF — ссылка на файл, тип и имя из ответа сервера', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-003');
    const card = await openCard(page, client.id, task.id);

    const name = `${uniq('AT-003 акт')}.pdf`;
    await card.uploadAttachment(pdf(name));

    const item = card.attachmentByName(name);
    await expect(item).toHaveAttribute('data-kind', 'pdf');
    const link = item.getByTestId('card-attachment-pdf');
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute('target', '_blank');
    const href = await link.getAttribute('href');
    const res = await page.request.get(href!);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toBe('application/pdf');
    expect(res.headers()['x-content-type-options']).toBe('nosniff');
    expect(res.headers()['content-disposition']).toMatch(/^inline; /);

    const api = await adminApi.getCard(task.id);
    expect(api.attachments.map((a) => [a.original_name, a.kind, a.content_type])).toEqual([[name, 'pdf', 'application/pdf']]);
  });
});

test.describe('AT: ошибки загрузки', () => {
  test('AT-004: подмена типа (HTML под .jpg) — понятная ошибка, список не меняется', async ({ adminPage: page, adminApi, uniq, allowHttpErrors }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-004');
    const card = await openCard(page, client.id, task.id);
    allowHttpErrors((url, status) => url.endsWith(`/api/v1/tasks/${task.id}/attachments`) && status === 400);

    await card.uploadAttachment({ name: `${uniq('AT-004')}.jpg`, mimeType: 'image/jpeg', buffer: Buffer.from('<!DOCTYPE html><script>alert(1)</script>') });

    await expect(card.attachmentError).toBeVisible();
    await expect(card.attachmentError).toContainText('Неподдерживаемый тип файла');
    await expect(card.attachmentsEmpty).toBeVisible();
    await expect(card.attachmentUpload).not.toHaveClass(/disabled/);
    expect((await adminApi.getCard(task.id)).attachments).toEqual([]);
  });

  test('AT-008: файл больше лимита — отказ на клиенте без запроса', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-008');
    const card = await openCard(page, client.id, task.id);
    let requested = false;
    page.on('request', (r) => { if (r.url().endsWith('/attachments') && r.method() === 'POST') requested = true; });

    const big = Buffer.alloc(20 * 1024 * 1024 + 1, 0);
    PNG_1X1.copy(big);
    await card.uploadAttachment({ name: `${uniq('AT-008')}.png`, mimeType: 'image/png', buffer: big });

    await expect(card.attachmentError).toContainText('слишком большой');
    await expect(card.attachmentsEmpty).toBeVisible();
    expect(requested).toBe(false);
  });
});

test.describe('AT: прогресс и блокировка кнопки', () => {
  test('AT-009: во время загрузки кнопка заблокирована, виден прогресс', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-009');
    const card = await openCard(page, client.id, task.id);

    // Запрос удерживается маршрутом, пока проверяем «занятое» состояние раздела.
    let release!: () => void;
    const held = new Promise<void>((r) => { release = r; });
    await page.route(`**/api/v1/tasks/${task.id}/attachments`, async (route) => {
      await held;
      await route.continue();
    });

    const name = `${uniq('AT-009')}.png`;
    await card.uploadAttachment(png(name));
    await expect(card.attachmentUpload).toHaveClass(/disabled/);
    await expect(card.attachmentUpload).toHaveAttribute('aria-disabled', 'true');
    await expect(card.attachmentInput).toBeDisabled();
    await expect(card.attachmentProgress).toBeVisible();
    await expect(card.page.getByTestId('card-attachment-uploading-name')).toHaveText(name);

    release();
    await expect(card.attachmentByName(name)).toHaveCount(1);
    await expect(card.attachmentUpload).not.toHaveClass(/disabled/);
    await expect(card.attachmentProgress).toBeHidden();
    await page.unroute(`**/api/v1/tasks/${task.id}/attachments`);
  });
});

test.describe('AT: удаление и права', () => {
  test('AT-005: удаление с подтверждением — отмена оставляет, подтверждение удаляет', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-005');
    const name = `${uniq('AT-005')}.png`;
    await adminApi.uploadAttachment(task.id, png(name));
    const card = await openCard(page, client.id, task.id);

    const item = card.attachmentByName(name);
    await expect(item).toHaveCount(1);
    await item.getByTestId('card-attachment-delete').click();
    await card.cancelUiDialog();
    await expect(item).toHaveCount(1);

    await item.getByTestId('card-attachment-delete').click();
    await card.confirmUiDialog();
    await expect(item).toHaveCount(0);
    await expect(card.attachmentsEmpty).toBeVisible();
    expect((await adminApi.getCard(task.id)).attachments).toEqual([]);
  });

  test('AT-006: новый заказ — вложения недоступны до первого сохранения', async ({ adminPage: page, adminApi, uniq, _boardCleanup }) => {
    const client = await adminApi.createClient({ company_name: uniq('AT клиент AT-006'), phones: [{ phone: '9990000701', note: '' }] });
    const columns = (await adminApi.listColumns()).items;
    const board = new BoardPage(page);
    const card = new OrderCardPage(page);
    await board.goto(`?client_id=${client.id}`);

    await board.openTaskAdd();
    await expect(card.attachmentsLocked).toBeVisible();
    await expect(card.attachmentUpload).toHaveCount(0);

    const plate = uniqPlate();
    const carType = await adminApi.createEquipmentType({ name: uniq('AT-006 тип') });
    await adminApi.createEquipment(client.id, { type_id: carType.id, plate_number: plate, brand: 'Kia', model: 'Rio' });
    await card.newOrder.useFoundCar(plate);
    await card.fillHeader({ column: columnByName(columns, 'Приёмка').name });
    await card.save();
    await expect(card.title).toContainText('#');
    const id = Number((await card.title.textContent())!.replace(/\D+/g, ''));
    _boardCleanup(id);

    await expect(card.attachmentsLocked).toHaveCount(0);
    await expect(card.attachmentsEmpty).toBeVisible();
    const name = `${uniq('AT-006')}.png`;
    await card.uploadAttachment(png(name));
    await expect(card.attachmentByName(name)).toHaveCount(1);
  });

  test('AT-007: исполнитель видит чужое вложение без кнопки удаления, своё — удаляет', async ({ page, adminApi, uniq, makeUser, loginAs }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-007');
    const adminFile = `${uniq('AT-007 админ')}.png`;
    await adminApi.uploadAttachment(task.id, png(adminFile));
    const exec = await makeUser({ login: uniq('exec') });
    await loginAs(page, exec.login, exec.password);
    const card = await openCard(page, client.id, task.id);

    const foreign = card.attachmentByName(adminFile);
    await expect(foreign).toHaveCount(1);
    await expect(foreign.getByTestId('card-attachment-thumb')).toBeVisible();
    await expect(foreign.getByTestId('card-attachment-delete')).toHaveCount(0);

    const own = `${uniq('AT-007 своё')}.pdf`;
    await card.uploadAttachment(pdf(own));
    const ownItem = card.attachmentByName(own);
    await expect(ownItem.getByTestId('card-attachment-delete')).toBeVisible();
    await ownItem.getByTestId('card-attachment-delete').click();
    await card.confirmUiDialog();
    await expect(ownItem).toHaveCount(0);
    await expect(foreign).toHaveCount(1);
  });
});

test.describe('AT: вёрстка', () => {
  test('AT-010: раздел вложений без горизонтального переполнения на 360px', async ({ adminPage: page, adminApi, uniq }) => {
    const { client, task } = await setupTask(adminApi, uniq, 'AT-010');
    await adminApi.uploadAttachment(task.id, png(`${uniq('AT-010 очень длинное имя файла фотографии')}.png`));
    await adminApi.uploadAttachment(task.id, pdf(`${uniq('AT-010 акт')}.pdf`));
    await page.setViewportSize({ width: 360, height: 640 });
    const card = await openCard(page, client.id, task.id);
    await expect(card.attachmentItems).toHaveCount(2);
    const overflow = await card.attachments.evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  });
});
