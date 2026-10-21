import { test, expect, type PoOrder, type PoError } from '../fixtures/purchase-orders';
import { PurchaseOrdersPage } from '../pages/PurchaseOrdersPage';
import { UsersPage } from '../pages/UsersPage';
import { SuppliersPage } from '../pages/SuppliersPage';
import { DeletedPage } from '../pages/DeletedPage';

// Закупки по дефициту (docs/features/purchase-orders/draft-spec.md §11, TS-014…TS-026):
// экраны «Закупки» (§8.1–§8.4), «Пользователи» (§8.5), кабинет поставщика (§8.6–§8.7),
// «Поставщики» и «Удалённые». Данные — фикстура po (TS-F1…TS-F3), уборка автоматическая.

const isConflict = (status: number) => status === 409;

test.describe('Закупки по дефициту', () => {
  test('TS-014: полный круг закупки у поставщика (DS-011, DS-008)', async ({ adminPage: page, po, uniq }) => {
    const acc = await po.supplier(uniq('АвтоДеталь Плюс'), true);
    const p1 = await po.product(uniq('колодки'), { stock: 0, min_stock: 5 });
    const p2 = await po.product(uniq('фильтр'), { stock: 0, min_stock: 5 });
    const p3 = await po.product(uniq('ремень'), { stock: 0, min_stock: 5 });
    const ui = new PurchaseOrdersPage(page);

    // admin → «Новый заказ» → поставщик → три наши строки, одну галочку снять → черновик.
    await ui.gotoNew();
    await ui.recipient.selectOption(acc.supplier.id);
    await ui.uncheckAll();
    for (const p of [p1, p2, p3]) {
      await expect(ui.lineQty(p.id)).toHaveValue('5');
      await ui.lineCheck(p.id).check();
    }
    await ui.lineCheck(p3.id).uncheck();
    const id = await ui.saveAndOpen();
    po.track(id);
    await expect(ui.status).toHaveText('Черновик');
    await expect(ui.items).toHaveCount(2);
    await expect(ui.toast.last()).toContainText('создан');

    await ui.button('send').click();
    await expect(ui.status).toHaveText('Отправлен');

    // поставщик: одна строка «Соберу» меньше просимого; «Собрано» с пустой строкой — отказ.
    const sp = await po.pageAs(acc.login, acc.password);
    const sup = new PurchaseOrdersPage(sp);
    await sup.gotoSupplierList();
    await sup.supplierCard(id).click();
    await expect(sup.supplierItems).toHaveCount(2);
    await sup.answer(p1.name, { qty: '3', price: '100' });
    await sup.supplierReady.click();
    await sup.confirm();
    await expect(sp.getByTestId('toast-error').last()).toHaveText('Заполните «Соберу» во всех строках');
    await expect(sup.supplierItem(p2.name)).toHaveClass(/so-missing/);
    await expect(sup.supplierStatus).toHaveText('Новый');

    await sup.answer(p2.name, { qty: '5', price: '10' });
    await sup.supplierReady.click();
    await sup.confirm();
    await expect(sup.supplierStatus).toHaveText('Собран');
    await expect(sup.supplierReady).toBeHidden();

    // admin: расхождение подсвечено, сумма верна → «Забрали».
    await ui.gotoView(id);
    await expect(ui.status).toHaveText('Собран');
    await expect(ui.item(p1.id)).toHaveAttribute('data-discrepancy', 'true');
    await expect(ui.item(p2.id)).not.toHaveAttribute('data-discrepancy', 'true');
    await expect(ui.total).toHaveText('Итого: 350.00');
    await ui.button('pickup').click();
    await ui.confirm();
    await expect(ui.status).toHaveText('Забрали');
    await expect(ui.button('save')).toHaveCount(0);

    // остаток запчасти не изменился (DS-010)
    const stock = await po.admin.raw('GET', `/api/v1/products?q=${encodeURIComponent(p1.name)}`);
    expect((stock.body as { items: Array<{ id: string; stock: number }> }).items.find((x) => x.id === p1.id)?.stock).toBe(0);
  });

  test('TS-015: Базар — без отправки, печать с ☐, «Забрали» (DS-058)', async ({ adminPage: page, po, uniq }) => {
    const p = await po.product(uniq('масло'), { stock: 0, min_stock: 2 });
    const ui = new PurchaseOrdersPage(page);
    await ui.gotoNew();
    await ui.recipient.selectOption('bazaar');
    await ui.uncheckAll();
    await ui.lineCheck(p.id).check();
    const id = await ui.saveAndOpen();
    po.track(id);
    await expect(page.getByTestId('po-recipient')).toContainText('Базар');
    await expect(ui.button('send')).toHaveCount(0);
    await expect(ui.button('pickup')).toBeVisible();

    const [download] = await Promise.all([page.waitForEvent('download'), ui.button('print').click()]);
    expect(download.suggestedFilename()).toMatch(/^З-\d{4,}\.pdf$/);

    // негатив: send через API → 409 NO_SUPPLIER
    const o = await po.admin.get(id);
    const r = await po.admin.act(id, 'send', o.version);
    expect(r.status).toBe(409);
    expect((r.body as PoError).code).toBe('PURCHASE_ORDER_NO_SUPPLIER');

    await ui.button('pickup').click();
    await ui.confirm();
    await expect(ui.status).toHaveText('Забрали');
  });

  test('TS-016: правка отправленного заказа — «Изменено заказчиком» (DS-034, DS-048)', async ({ adminPage: page, po, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, st) => url.includes('/api/v1/purchase-orders/') && isConflict(st));
    const acc = await po.supplier(uniq('Поставщик'), true);
    const p1 = await po.product(uniq('диск'));
    const p2 = await po.product(uniq('колодка'));
    let o = await po.create(po.admin, { supplier_id: acc.supplier.id, note: 'старое', items: [{ product_id: p1.id, qty: '4' }, { product_id: p2.id, qty: '2' }] });
    o = await po.admin.send(o);
    const supApi = await po.as(acc.login, acc.password);
    for (const it of o.items) await supApi.answer(o.id, it.id, { version: it.version, supplier_qty: it.qty, supplier_price: '50' });

    const ui = new PurchaseOrdersPage(page);
    await ui.gotoView(o.id);
    await ui.itemQty(p1.id).fill('7');
    await ui.page.getByTestId('po-note').fill('Забрать до 18:00');
    await ui.button('save').click();
    await expect(ui.toast.last()).toHaveText('Сохранено');

    const sp = await po.pageAs(acc.login, acc.password);
    const sup = new PurchaseOrdersPage(sp);
    await sup.gotoSupplierList();
    await expect(sup.supplierCard(o.id).getByTestId('so-card-changed')).toBeVisible();
    await sup.supplierCard(o.id).click();
    await expect(sp.getByTestId('so-note-changed')).toBeVisible();
    await expect(sp.getByTestId('so-note')).toContainText('Забрать до 18:00');
    const changed = sup.supplierItem(p1.name);
    await expect(changed.getByTestId('so-item-changed')).toBeVisible();
    await expect(changed.getByTestId('so-item-qty')).toHaveValue('');
    await expect(sup.supplierItem(p2.name).getByTestId('so-item-changed')).toBeHidden();
    await expect(sup.supplierItem(p2.name).getByTestId('so-item-qty')).toHaveValue('2');

    // после ответа метка снимается
    await sup.answer(p1.name, { qty: '7', price: '50' });
    await expect(changed.getByTestId('so-item-changed')).toBeHidden();

    // негатив: поставщик отметил «Собрано», мы правим — отказ
    await supApi.ready(o.id);
    await ui.itemQty(p2.id).fill('3');
    await ui.button('save').click();
    await expect(ui.toastError.last()).toHaveText('Поставщик уже собрал заказ — правка недоступна');
  });

  test('TS-017: отозвать и отправить снова — ответы на месте (DS-035)', async ({ adminPage: page, po, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, st) => url.includes('/api/v1/purchase-orders/') && isConflict(st));
    const acc = await po.supplier(uniq('Поставщик'), true);
    const p = await po.product(uniq('свеча'));
    let o = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '4' }] });
    o = await po.admin.send(o);
    const supApi = await po.as(acc.login, acc.password);
    await supApi.answer(o.id, o.items[0].id, { version: o.items[0].version, supplier_qty: '4', supplier_price: '700' });

    const ui = new PurchaseOrdersPage(page);
    await ui.gotoView(o.id);
    await ui.button('recall').click();
    await expect(ui.status).toHaveText('Черновик');
    await expect(ui.item(p.id).getByTestId('po-item-supplier-qty')).toHaveText('4');

    const sp = await po.pageAs(acc.login, acc.password);
    const sup = new PurchaseOrdersPage(sp);
    await sup.gotoSupplierList();
    await expect(sup.supplierCard(o.id)).toHaveCount(0);

    await ui.button('send').click();
    await expect(ui.status).toHaveText('Отправлен');
    const again = await supApi.supplierGet(o.id);
    expect(again.items[0].supplier_qty).toBe('4.000');
    expect(again.items[0].supplier_price).toBe('700.00');

    // негатив: «Отозвать» после «Собрано»
    await supApi.ready(o.id);
    await ui.button('recall').click();
    await expect(ui.toastError.last()).toHaveText('Поставщик уже собрал заказ — отозвать нельзя');
  });

  test('TS-018: отмена с причиной — у поставщика заказа нет (DS-036, DS-041)', async ({ adminPage: page, po, uniq }) => {
    const acc = await po.supplier(uniq('Поставщик'), true);
    const p = await po.product(uniq('лампа'));
    let o = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '1' }] });
    o = await po.admin.send(o);

    const ui = new PurchaseOrdersPage(page);
    await ui.gotoView(o.id);
    await ui.button('cancel').click();
    await expect(ui.reasonOk).toBeDisabled();
    await ui.reasonInput.fill('   ');
    await expect(ui.reasonOk).toBeDisabled();
    await ui.reasonInput.fill('Нашли у другого поставщика дешевле');
    await expect(ui.reasonOk).toBeEnabled();
    await ui.reasonOk.click();
    await expect(ui.status).toHaveText('Отменён');
    await expect(ui.cancelReason).toContainText('Нашли у другого поставщика дешевле');

    const sp = await po.pageAs(acc.login, acc.password);
    const sup = new PurchaseOrdersPage(sp);
    await sup.gotoSupplierList();
    await expect(sup.supplierCard(o.id)).toHaveCount(0);
    await sup.gotoSupplierOrder(o.id);
    await expect(sup.notFound).toContainText('Заказ не найден');
  });

  test('TS-019: подсказка «уже в открытом заказе» (DS-004, DS-049)', async ({ adminPage: page, po, uniq }) => {
    const acc = await po.supplier(uniq('Поставщик'), false);
    const inSent = await po.product(uniq('открытый'), { stock: 0, min_stock: 3 });
    const inDraft = await po.product(uniq('черновой'), { stock: 0, min_stock: 3 });
    let sent = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: inSent.id, qty: '5' }] });
    sent = await po.admin.send(sent);
    await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: inDraft.id, qty: '2' }] });

    const ui = new PurchaseOrdersPage(page);
    await ui.gotoNew();
    await expect(ui.lineHint(inSent.id)).toHaveText(`уже в открытом заказе ${sent.number_display} — 5 шт`);
    await expect(ui.lineHint(inSent.id).getByRole('link')).toHaveAttribute('href', `/purchases/${sent.id}`);
    await expect(ui.lineCheck(inSent.id)).not.toBeChecked();
    await expect(ui.lineQty(inSent.id)).toHaveValue('3'); // без вычета открытого заказа
    // негатив: только черновик — подсказки нет, галочка стоит
    await expect(ui.lineHint(inDraft.id)).toHaveCount(0);
    await expect(ui.lineCheck(inDraft.id)).toBeChecked();
  });

  test('TS-020: поставщик заперт в кабинете (DS-017, DS-018)', async ({ po, uniq }) => {
    const acc = await po.supplier(uniq('Поставщик'), true);
    const other = await po.supplier(uniq('Чужой'), true);
    const p = await po.product(uniq('чужое'));
    let foreign = await po.create(po.admin, { supplier_id: other.supplier.id, items: [{ product_id: p.id, qty: '1' }] });
    foreign = await po.admin.send(foreign);

    const sp = await po.pageAs(acc.login, acc.password);
    await sp.goto('/');
    await expect(sp).toHaveURL(/\/supplier\/orders$/);
    const nav = sp.getByTestId('sidebar').locator('.app-nav-link');
    await expect(nav).toHaveCount(1);
    await expect(nav).toContainText('Заказы');
    for (const path of ['/stock', '/clients', '/purchases', '/users']) {
      await sp.goto(path);
      await expect(sp, path).toHaveURL(/\/supplier\/orders$/);
    }
    // негатив: чужой заказ по URL
    const sup = new PurchaseOrdersPage(sp);
    await sup.gotoSupplierOrder(foreign.id);
    await expect(sup.notFound).toContainText('Заказ не найден');
  });

  for (const vp of [{ w: 360, h: 640 }, { w: 768, h: 1024 }, { w: 1280, h: 800 }]) {
    for (const theme of ['light', 'dark'] as const) {
      test(`TS-021: кабинет на ${vp.w}px, тема ${theme} (DS-031)`, async ({ po, uniq }, testInfo) => {
        const acc = await po.supplier(uniq('Поставщик'), true);
        const long = await po.product(uniq('Очень длинное название запчасти для проверки переноса строк в карточке кабинета поставщика'));
        const p = await po.product(uniq('короткое'));
        let o = await po.create(po.admin, { supplier_id: acc.supplier.id, note: 'Примечание', items: [{ product_id: long.id, qty: '6' }, { product_id: p.id, qty: '1' }] });
        o = await po.admin.send(o);

        const sp = await po.pageAs(acc.login, acc.password);
        await sp.addInitScript((t) => { try { localStorage.setItem('kt-theme', t); } catch { /* приватный режим */ } }, theme);
        await sp.setViewportSize({ width: vp.w, height: vp.h });
        const sup = new PurchaseOrdersPage(sp);
        await sup.gotoSupplierOrder(o.id);
        await expect(sup.supplierItems).toHaveCount(2);
        await expect(sp.locator('html')).toHaveAttribute('data-bs-theme', theme);
        const qty = sup.supplierItem(long.name).getByTestId('so-item-qty');
        await expect(qty).toHaveAttribute('inputmode', 'decimal');
        await expect(sup.supplierItem(long.name).getByTestId('so-item-price')).toHaveAttribute('inputmode', 'decimal');
        await expect(sup.supplierReady).toBeVisible();
        await expect(sup.supplierTotal).toBeInViewport();
        const overflow = await sp.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
        expect(overflow, 'нет горизонтального скролла').toBeLessThanOrEqual(0);
        await testInfo.attach(`cabinet-${vp.w}-${theme}`, { body: await sp.screenshot({ fullPage: true }), contentType: 'image/png' });
      });
    }
  }

  test('TS-022: экраны «Закупки» на 360px без выхода за край (DS-031)', async ({ adminPage: page, po, uniq }) => {
    const acc = await po.supplier(uniq('Поставщик с очень длинным названием организации для проверки кнопок'), false);
    const p = await po.product(uniq('узкий'));
    const o = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '1' }] });
    await page.setViewportSize({ width: 360, height: 740 });
    const ui = new PurchaseOrdersPage(page);
    const overflow = () => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);

    await ui.gotoList();
    await expect(ui.row(o.id)).toBeAttached();
    expect(await overflow(), 'список').toBeLessThanOrEqual(0);
    await ui.gotoNew();
    await expect(ui.saveDraft).toBeVisible();
    expect(await overflow(), 'новый заказ').toBeLessThanOrEqual(0);
    await ui.gotoView(o.id);
    await expect(ui.button('send')).toBeVisible();
    expect(await overflow(), 'карточка').toBeLessThanOrEqual(0);
    const box = await ui.button('send').boundingBox();
    expect(box && box.x + box.width).toBeLessThanOrEqual(360);
  });

  test('TS-023: учётка поставщика на экране «Пользователи» (DS-025, DS-044)', async ({ adminPage: page, po, uniq, makeUser, allowHttpErrors }) => {
    allowHttpErrors((url, st) => st === 422 && /\/users(\?|$)/.test(url));
    const acc = await po.supplier(uniq('АвтоДеталь'), false);
    const users = new UsersPage(page);
    const login = uniq('sup-ui');
    po.trackSupplierLogin(login);
    await users.gotoNew();
    await expect(users.supplier).toBeHidden();
    await users.fill({ name: 'Продавец Ерлан', login, password: 'secret123' });
    await users.chooseRole('Поставщик');
    await expect(users.supplier).toBeVisible();
    await users.save.click();
    await expect(users.fieldError('supplier_id')).toHaveText('Выберите поставщика');
    await users.supplier.selectOption(acc.supplier.id);
    await users.password.fill('secret123');
    await users.save.click();
    await expect(page).toHaveURL(/\/users\?flash=created/);
    await expect(users.row(login)).toContainText(`Поставщик (${acc.supplier.name})`);

    const sp = await po.pageAs(login, 'secret123');
    await sp.goto('/');
    await expect(sp).toHaveURL(/\/supplier\/orders$/);

    // негатив: executor открывает /users → 403-страница
    const ex = await makeUser({ login: uniq('exec') });
    const ep = await po.pageAs(ex.login, ex.password);
    const resp = await ep.goto('/users');
    expect(resp?.status()).toBe(403);
  });

  test('TS-024: executor видит только свои заказы (DS-051, DS-060, DS-078)', async ({ page, po, uniq, makeUser, loginAs, runAdmin, allowHttpErrors }) => {
    allowHttpErrors((url, st) => st === 404 && url.includes('/purchases/'));
    const ex = await makeUser({ login: uniq('exec') });
    const exApi = await po.as(ex.login, ex.password);
    const acc = await po.supplier(uniq('Поставщик'), false);
    const pAdmin = await po.product(uniq('админский'), { stock: 0, min_stock: 2 });
    const pMine = await po.product(uniq('свой'), { stock: 0, min_stock: 2 });
    let adminOrder = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: pAdmin.id, qty: '2' }] });
    adminOrder = await po.admin.send(adminOrder);
    let mine: PoOrder = await po.create(exApi, { supplier_id: acc.supplier.id, items: [{ product_id: pMine.id, qty: '2' }] });
    mine = await exApi.send(mine);

    await loginAs(page, ex.login, ex.password);
    const ui = new PurchaseOrdersPage(page);
    await ui.gotoList();
    await expect(ui.row(mine.id)).toBeVisible();
    await expect(ui.row(adminOrder.id)).toHaveCount(0);
    const resp = await page.goto(`/purchases/${adminOrder.id}`);
    expect(resp?.status()).toBe(404);

    await ui.gotoNew();
    await expect(ui.lineHint(pAdmin.id)).toHaveCount(0);
    await expect(ui.lineCheck(pAdmin.id)).toBeChecked();
    await expect(ui.lineHint(pMine.id)).toContainText(mine.number_display);
    await expect(ui.lineCheck(pMine.id)).not.toBeChecked();

    // admin на том же экране видит подсказки по обоим заказам
    const ap = await po.pageAs(runAdmin.login, runAdmin.password);
    const adm = new PurchaseOrdersPage(ap);
    await adm.gotoNew();
    await expect(adm.lineHint(pAdmin.id)).toContainText(adminOrder.number_display);
    await expect(adm.lineHint(pMine.id)).toContainText(mine.number_display);

    // негатив: прямой POST cancel чужого → 404
    const r = await exApi.cancel(adminOrder.id, { version: adminOrder.version, reason: 'чужой' });
    expect(r.status).toBe(404);
    expect((r.body as PoError).code).toBe('PURCHASE_ORDER_NOT_FOUND');
  });

  test('TS-025: архив поставщика с открытым заказом (DS-043)', async ({ adminPage: page, po, uniq, allowHttpErrors }) => {
    allowHttpErrors((url, st) => url.includes('/api/v1/suppliers/') && isConflict(st));
    const acc = await po.supplier(uniq('Архивируемый'), false);
    const p = await po.product(uniq('деталь'));
    let o = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '1' }] });
    o = await po.admin.send(o);

    const sup = new SuppliersPage(page);
    await sup.goto();
    await sup.searchFor(acc.supplier.name);
    await sup.openRow(acc.supplier.name);
    await sup.toggleActive.click();
    await expect(sup.toastError.last()).toHaveText(`У поставщика есть незакрытые закупки: ${o.number_display}`);

    // негатив: после «Забрали» (у поставщика нет входа — прямо из sent) архив проходит
    const r = await po.admin.act(o.id, 'pick-up', o.version);
    expect(r.status).toBe(200);
    await sup.goto();
    await sup.searchFor(acc.supplier.name);
    await sup.openRow(acc.supplier.name);
    await sup.toggleActive.click();
    await sup.expectSaved();
  });

  test('TS-026: удаление и восстановление черновика (DS-037, DS-072)', async ({ adminPage: page, po, uniq }) => {
    const acc = await po.supplier(uniq('Поставщик'), false);
    const p = await po.product(uniq('удаляемая'));
    const draft = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '1' }] });
    const ui = new PurchaseOrdersPage(page);
    await ui.gotoView(draft.id);
    await ui.button('delete').click();
    await ui.reasonInput.fill('Создан по ошибке');
    await ui.reasonOk.click();
    await expect(page).toHaveURL(/\/purchases$/);
    await expect(ui.toast.last()).toContainText('Черновик удалён');
    await expect(ui.row(draft.id)).toHaveCount(0);

    const del = new DeletedPage(page);
    await del.goto('kind=purchase-orders');
    await expect(del.rowTitle(draft.id)).toHaveText(draft.number_display);
    await expect(del.rowReason(draft.id)).toContainText('Создан по ошибке');
    await del.restore(draft.id);
    await expect(del.row(draft.id)).toHaveCount(0);
    await ui.gotoList();
    await expect(ui.row(draft.id)).toBeVisible();

    // негатив: у отправленного заказа кнопки «Удалить» нет, API → 409
    const sent = await po.admin.send(await po.admin.get(draft.id));
    await ui.gotoView(sent.id);
    await expect(ui.button('delete')).toHaveCount(0);
    const r = await po.admin.remove(sent.id, { version: sent.version, reason: 'нельзя' });
    expect(r.status).toBe(409);
    expect((r.body as PoError).code).toBe('PURCHASE_ORDER_INVALID_STATUS');
  });
});
