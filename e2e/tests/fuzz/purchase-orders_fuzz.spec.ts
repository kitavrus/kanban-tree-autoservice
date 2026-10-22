import { test, expect, type PoError, type PoOrder } from '../../fixtures/purchase-orders';
import { PurchaseOrdersPage } from '../../pages/PurchaseOrdersPage';

// pw-fuzz закупок (§TD-7; docs/features/purchase-orders/3-design/design-e2e-strategy.md): формы
// ответа поставщика (supplier_qty, supplier_price, supplier_note) и строки/примечание заказа
// (qty, note, cancel_reason). Мутаторы детерминированы (без Math.random): границы, знаки, дроби,
// локаль-разделители, юникод, длины, XSS-инжекты. Инвариант: сервер отвечает 2xx (значение
// валидно) либо 400 VALIDATION с details.fields[] — никогда 5xx; XSS-строки на экране — текстом.

const XSS = ['<script>window.__pwned=1</script>', '"><img src=x onerror="window.__pwned=1">', "javascript:alert('x')"];

const NUMBERS = [
  '0', '-0', '1', '-1', '0.001', '0.0001', '99999.999', '100000', '9999999.99', '10000000', '1.23456',
  '1,5', '1 000', '١٢', '１２', '1e3', 'NaN', 'Infinity', '0x10', '', ' ', '+5', '.5', '5.', '00012',
  '9'.repeat(40), ...XSS,
];
const TEXTS = [
  '', ' ', 'я'.repeat(500), 'я'.repeat(501), 'я'.repeat(1000), 'я'.repeat(1001), '😀'.repeat(300),
  'a\u0000b', '‮обратный', 'строка\nперенос', ...XSS,
];

/** 2xx либо 400, никогда 5xx; 409 (конфликт статуса/версии) допустим как бизнес-отказ.
 *  400 по канону pkg/errorspkg: значение поля вне правил → VALIDATION с details.fields[];
 *  структурно битое тело (неверный JSON-тип поля, не декодируется) → BAD_REQUEST «invalid body»
 *  (as-is всех фич: c.Bind().Body/decode → errorspkg.BadRequest; §3 «Формат ошибок», DS-065). */
function expectSane(r: { status: number; body: unknown }, what: string, opts: { structural?: boolean } = {}) {
  expect(r.status, `${what}: ${JSON.stringify(r.body)}`).toBeLessThan(500);
  if (r.status === 400) {
    const e = r.body as PoError;
    if (opts.structural && e.code === 'BAD_REQUEST') return;
    expect(e.code, what).toBe('VALIDATION');
    expect(e.details?.fields?.length ?? 0, `${what}: details.fields[]`).toBeGreaterThan(0);
  }
}

test.describe('FUZZ purchase-orders', () => {
  test('строки и примечание заказа: qty/note — 2xx либо 400, без 5xx', async ({ po, uniq }) => {
    const acc = await po.supplier(uniq('Фазз'), false);
    const p = await po.product(uniq('фазз-запчасть'));
    for (const qty of NUMBERS) {
      const r = await po.admin.raw('POST', '/api/v1/purchase-orders', { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty }] });
      expectSane(r, `create qty=${JSON.stringify(qty)}`);
      if (r.status === 201) po.track((r.body as PoOrder).id);
    }
    const base = await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '1' }] });
    let version = base.version;
    for (const note of TEXTS) {
      const r = await po.admin.raw('PUT', `/api/v1/purchase-orders/${base.id}`, {
        version, note, items: [{ id: base.items[0].id, product_id: p.id, qty: '1' }],
      });
      expectSane(r, `note len=${[...note].length}`);
      if (r.status === 200) version = (r.body as PoOrder).version;
    }
    // мусорные типы полей и тела
    for (const body of [{ items: 'x' }, { supplier_id: 5, items: [] }, { items: [{ product_id: p.id, qty: true }] }, { items: [{ product_id: null, qty: null }] }]) {
      expectSane(await po.admin.raw('POST', '/api/v1/purchase-orders', body), `body ${JSON.stringify(body)}`, { structural: true });
    }
  });

  test('cancel_reason: 2xx либо 400, без 5xx', async ({ po, uniq }) => {
    const p = await po.product(uniq('фазз-отмена'));
    for (const reason of TEXTS) {
      const o = await po.create(po.admin, { supplier_id: null, items: [{ product_id: p.id, qty: '1' }] });
      expectSane(await po.admin.cancel(o.id, { version: o.version, reason }), `cancel reason len=${[...reason].length}`);
    }
  });

  test('ответ поставщика: supplier_qty/price/note — 2xx либо 400, без 5xx', async ({ po, uniq }) => {
    const acc = await po.supplier(uniq('Фазз-кабинет'), true);
    const p = await po.product(uniq('фазз-ответ'));
    const o = await po.admin.send(await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '2' }] }));
    const sup = await po.as(acc.login, acc.password);
    const url = `/api/v1/supplier/purchase-orders/${o.id}/items/${o.items[0].id}`;
    let version = o.items[0].version;
    const send = async (body: Record<string, unknown>, what: string) => {
      const r = await sup.raw('PUT', url, { version, ...body });
      expectSane(r, what);
      if (r.status === 200) version = (r.body as { version: number }).version;
    };
    for (const v of NUMBERS) {
      await send({ supplier_qty: v, supplier_price: '1' }, `supplier_qty=${JSON.stringify(v)}`);
      await send({ supplier_qty: '1', supplier_price: v }, `supplier_price=${JSON.stringify(v)}`);
    }
    for (const note of TEXTS) await send({ supplier_qty: '1', supplier_price: '1', supplier_note: note }, `supplier_note len=${[...note].length}`);
  });

  test('XSS в примечании и комментарии поставщика отображается текстом', async ({ adminPage: page, po, uniq }) => {
    const acc = await po.supplier(uniq('Фазз-XSS'), true);
    const p = await po.product(uniq('фазз-xss'));
    const note = XSS.join(' ');
    const o = await po.admin.send(await po.create(po.admin, { supplier_id: acc.supplier.id, note, items: [{ product_id: p.id, qty: '1' }] }));
    const sup = await po.as(acc.login, acc.password);
    await sup.answer(o.id, o.items[0].id, { version: o.items[0].version, supplier_qty: '1', supplier_price: '1', supplier_note: XSS[1] });

    const ui = new PurchaseOrdersPage(page);
    await ui.gotoView(o.id);
    await expect(page.getByTestId('po-note')).toHaveValue(note);
    await expect(ui.item(p.id).getByTestId('po-item-supplier-note')).toHaveText(XSS[1]);
    expect(await page.evaluate(() => (window as unknown as { __pwned?: number }).__pwned)).toBeUndefined();

    const sp = await po.pageAs(acc.login, acc.password);
    const cab = new PurchaseOrdersPage(sp);
    await cab.gotoSupplierOrder(o.id);
    await expect(sp.locator('#so-note-text')).toHaveText(note);
    await expect(cab.supplierItem(p.name).getByTestId('so-item-note')).toHaveValue(XSS[1]);
    expect(await sp.evaluate(() => (window as unknown as { __pwned?: number }).__pwned)).toBeUndefined();
  });
  test('канбан закупок: мусор в view/supplier/q и XSS в имени поставщика не ломают доску', async ({ adminPage: page, po, uniq }) => {
    const acc = await po.supplier(uniq(XSS[1]), false);
    const p = await po.product(uniq('фазз-канбан'));
    const o = await po.admin.send(await po.create(po.admin, { supplier_id: acc.supplier.id, items: [{ product_id: p.id, qty: '1' }] }));
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    const ui = new PurchaseOrdersPage(page);

    const junk = ['', ' ', 'bazaar', 'all', 'DONE', '__proto__', 'constructor', '%00', 'я'.repeat(150), '😀', '../../etc', ...XSS];
    for (const v of junk) {
      const q = new URLSearchParams({ view: v, supplier: v, q: v }).toString();
      await ui.gotoList('?' + q);
      await expect(page.getByTestId('po-board'), `query ${q}`).toBeVisible();
      await expect(page.getByTestId('po-col'), `query ${q}`).toHaveCount(5);
      expect((await ui.search.inputValue()).length, 'поиск обрезан до 100 символов').toBeLessThanOrEqual(100);
      // неизвестный быстрый фильтр сбрасывается на «Все».
      if (!['', 'all', 'DONE'].includes(v)) await expect(ui.view('')).toHaveAttribute('aria-pressed', 'true');
    }

    await ui.gotoList();
    for (const v of [...XSS, 'я'.repeat(1000), '\\', '(', '[', '*']) {
      await ui.search.fill(v);
      await expect(page.getByTestId('po-board')).toBeVisible();
    }
    await ui.search.fill('');
    await expect(ui.row(o.id).getByTestId('po-row-recipient')).toHaveText(acc.supplier.name);
    await ui.supplierFilter.selectOption(acc.supplier.id);
    await expect(ui.row(o.id)).toBeVisible();
    expect(await page.evaluate(() => (window as unknown as { __pwned?: number }).__pwned)).toBeUndefined();
    expect(errors, 'JS-ошибки на странице').toEqual([]);
  });
});
