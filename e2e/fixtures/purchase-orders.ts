import { expect, request, type APIRequestContext, type Page } from '@playwright/test';
import { BASE_URL, loginContext, type NewUser, type Product, type Supplier } from './auth';
import { test as base } from './test';

// Фикстуры закупок по дефициту (docs/features/purchase-orders/draft-spec.md §11 TS-F1…TS-F3):
// wire-типы — зеркало internal/features/purchase-orders/models/dto (§TD-6.2: JSON snake_case,
// числа numeric — строки), клиент API от имени любого пользователя, детерминированный сид
// (поставщик с учёткой, поставщик без кабинета, запчасти «ниже минимума»/без дефицита) и уборка:
// закупки теста стираются служебной уборкой ДО запчастей и поставщиков (они держат их FK), учётки
// поставщиков переводятся в неактивных исполнителей (пользователи не удаляются, ADR-007) — иначе
// поставщик не убирается.

// ---- wire-типы (models/dto) ----
export type PoStatus = 'draft' | 'sent' | 'ready' | 'picked_up' | 'cancelled';
export type PoReason = 'for_orders' | 'below_min' | 'manual';
export type PoItemRequest = { id?: string | null; product_id: string; qty: string; reason?: PoReason };
export type PoCreateRequest = { supplier_id: string | null; note?: string; items: PoItemRequest[] };
export type PoUpdateRequest = { version: number; note: string; items: PoItemRequest[] };
export type PoVersionRequest = { version: number };
export type PoReasonRequest = { version: number; reason: string };
export type PoOrderItem = {
  id: string; product_id: string; name: string; sku: string; unit: string; qty: string; reason: PoReason;
  supplier_qty: string | null; supplier_price: string | null; supplier_note: string; line_amount: string | null;
  has_discrepancy: boolean; customer_changed: boolean; version: number;
};
export type PoOrder = {
  id: string; number: number; number_display: string; status: PoStatus; version: number; supplier_id: string | null;
  recipient_name: string; recipient_phone: string; supplier_has_login: boolean; note: string; can_edit: boolean;
  created_by_name: string; created_at: string; sent_at: string | null; ready_at: string | null; picked_up_at: string | null;
  cancelled_at: string | null; cancel_reason: string | null; items: PoOrderItem[]; total_amount: string | null;
};
export type PoAnswerRequest = { version: number; supplier_qty: string | null; supplier_price?: string | null; supplier_note?: string };
export type PoSupplierItem = {
  id: string; name: string; sku: string; unit: string; qty: string; supplier_qty: string | null; supplier_price: string | null;
  supplier_note: string; line_amount: string | null; customer_changed: boolean; version: number;
};
export type PoSupplierOrder = {
  id: string; number_display: string; status: PoStatus; sent_at: string | null; note: string; note_changed: boolean;
  firm_name: string; firm_phone: string; can_answer: boolean; items: PoSupplierItem[]; total_amount: string | null;
};
export type PoError = { code: string; message: string; details?: { fields?: Array<{ name: string; message: string }>; item_ids?: string[]; numbers?: string[] } };

/** JSON API закупок от имени одного пользователя (своя cookie-сессия + CSRF). */
export class PoApi {
  private constructor(private readonly ctx: APIRequestContext) {}

  static async login(login: string, password: string): Promise<PoApi> {
    const ctx = await request.newContext({ baseURL: BASE_URL });
    const api = new PoApi(ctx);
    const res = await ctx.post('/api/v1/auth/login', { data: { login, password }, headers: await api.headers() });
    expect(res.status(), `вход ${login}: ${await res.text()}`).toBe(200);
    return api;
  }

  private async headers() {
    let c = (await this.ctx.storageState()).cookies.find((x) => x.name === 'kt_csrf');
    if (!c) {
      await this.ctx.get('/login');
      c = (await this.ctx.storageState()).cookies.find((x) => x.name === 'kt_csrf');
    }
    return { 'X-Csrf-Token': c?.value ?? '' };
  }

  async raw(method: 'GET' | 'POST' | 'PUT' | 'DELETE', url: string, data?: object): Promise<{ status: number; body: unknown }> {
    const res = await this.ctx.fetch(url, { method, data, headers: method === 'GET' ? {} : await this.headers() });
    const text = await res.text();
    let body: unknown = text;
    try { body = JSON.parse(text); } catch { /* не JSON */ }
    return { status: res.status(), body };
  }

  async create(body: PoCreateRequest): Promise<PoOrder> {
    const r = await this.raw('POST', '/api/v1/purchase-orders', body);
    expect(r.status, `создание закупки: ${JSON.stringify(r.body)}`).toBe(201);
    return r.body as PoOrder;
  }

  async get(id: string): Promise<PoOrder> {
    const r = await this.raw('GET', `/api/v1/purchase-orders/${id}`);
    expect(r.status, `закупка ${id}: ${JSON.stringify(r.body)}`).toBe(200);
    return r.body as PoOrder;
  }

  async update(id: string, body: PoUpdateRequest): Promise<PoOrder> {
    const r = await this.raw('PUT', `/api/v1/purchase-orders/${id}`, body);
    expect(r.status, `правка закупки: ${JSON.stringify(r.body)}`).toBe(200);
    return r.body as PoOrder;
  }

  async act(id: string, action: 'send' | 'recall' | 'pick-up', version: number): Promise<{ status: number; body: unknown }> {
    const body: PoVersionRequest = { version };
    return this.raw('POST', `/api/v1/purchase-orders/${id}/${action}`, body);
  }

  async send(o: PoOrder): Promise<PoOrder> {
    const r = await this.act(o.id, 'send', o.version);
    expect(r.status, `отправка: ${JSON.stringify(r.body)}`).toBe(200);
    return r.body as PoOrder;
  }

  async cancel(id: string, body: PoReasonRequest): Promise<{ status: number; body: unknown }> {
    return this.raw('POST', `/api/v1/purchase-orders/${id}/cancel`, body);
  }

  async remove(id: string, body: PoReasonRequest): Promise<{ status: number; body: unknown }> {
    return this.raw('DELETE', `/api/v1/purchase-orders/${id}`, body);
  }

  // ---- кабинет поставщика ----
  async supplierGet(id: string): Promise<PoSupplierOrder> {
    const r = await this.raw('GET', `/api/v1/supplier/purchase-orders/${id}`);
    expect(r.status, `кабинет ${id}: ${JSON.stringify(r.body)}`).toBe(200);
    return r.body as PoSupplierOrder;
  }

  async answer(orderId: string, itemId: string, body: PoAnswerRequest): Promise<void> {
    const r = await this.raw('PUT', `/api/v1/supplier/purchase-orders/${orderId}/items/${itemId}`, body);
    expect(r.status, `ответ поставщика: ${JSON.stringify(r.body)}`).toBe(200);
  }

  async ready(orderId: string): Promise<void> {
    const r = await this.raw('POST', `/api/v1/supplier/purchase-orders/${orderId}/ready`);
    expect(r.status, `«Собрано»: ${JSON.stringify(r.body)}`).toBe(200);
  }

  async purge(id: string): Promise<number> {
    return (await this.raw('DELETE', `/api/v1/test/purge/purchase-orders/${id}`)).status;
  }

  async dispose() { await this.ctx.dispose(); }
}

export type SupplierAccount = { supplier: Supplier; login: string; password: string; userId: string };

/** Набор фикстур закупок одного теста. */
export type PoKit = {
  /** API от имени админа прогона. */
  admin: PoApi;
  /** API от имени произвольного пользователя (закрывается в teardown). */
  as: (login: string, password: string) => Promise<PoApi>;
  /** Поставщик (TS-F1): withLogin — с активной учёткой supplier. */
  supplier: (name: string, withLogin: boolean) => Promise<SupplierAccount>;
  /** Запчасть: по умолчанию «ниже минимума» (stock 0, min 1) — попадает в дефицит. */
  product: (name: string, p?: { stock?: number; min_stock?: number }) => Promise<Product>;
  /** Закупку создал экран — зарегистрировать для уборки. */
  track: (orderId: string) => void;
  /** Создать закупку через API (уборка автоматическая). */
  create: (api: PoApi, body: PoCreateRequest) => Promise<PoOrder>;
  /** Вкладка отдельного браузерного контекста, вошедшая как login (поставщик рядом с админом). */
  pageAs: (login: string, password: string) => Promise<Page>;
  /** Учётку поставщика создал экран «Пользователи» — зарегистрировать для уборки по логину. */
  trackSupplierLogin: (login: string) => void;
};

export const test = base.extend<{ po: PoKit }>({
  // Зависит от _cleanup — снимается РАНЬШЕ него: закупки уходят до запчастей и поставщиков.
  po: async ({ adminApi, runAdmin, uniq, _cleanup, browser }, use) => {
    void _cleanup;
    const orders: string[] = [];
    const apis: PoApi[] = [];
    const accounts: Array<{ userId: string; login: string }> = [];
    const pages: Page[] = [];
    const admin = await PoApi.login(runAdmin.login, runAdmin.password);
    apis.push(admin);
    const kit: PoKit = {
      admin,
      as: async (login, password) => { const a = await PoApi.login(login, password); apis.push(a); return a; },
      supplier: async (name, withLogin) => {
        const supplier = await adminApi.createSupplier({ name, phone: '+7 701 555 00 11' });
        if (!withLogin) return { supplier, login: '', password: '', userId: '' };
        const login = uniq('sup');
        const nu: NewUser = { login, name: `Продавец ${name}`, role: 'supplier', password: 'password1', supplier_id: supplier.id };
        const u = await adminApi.createUser(nu);
        accounts.push({ userId: u.id, login });
        return { supplier, login, password: 'password1', userId: u.id };
      },
      product: (name, p = {}) => adminApi.createProduct({ name, sku: name.slice(-12), unit: 'шт', stock: p.stock ?? 0, min_stock: p.min_stock ?? 1 }),
      track: (id) => { orders.push(id); },
      create: async (api, body) => { const o = await api.create(body); orders.push(o.id); return o; },
      trackSupplierLogin: (login) => { accounts.push({ userId: '', login }); },
      pageAs: async (login, password) => {
        const ctx = await browser.newContext();
        await loginContext(ctx, login, password);
        const p = await ctx.newPage();
        pages.push(p);
        return p;
      },
    };
    await use(kit);

    for (const p of pages) await p.context().close();
    for (const id of orders) {
      const st = await admin.purge(id).catch(() => 0);
      if (st !== 200 && st !== 404) console.warn(`уборка: закупка ${id} — статус ${st}`);
    }
    // Учётки поставщиков не удаляются (ADR-007): снять привязку, чтобы поставщик убрался.
    for (const a of accounts) {
      if (!a.userId) {
        const r = await admin.raw('GET', `/api/v1/users?q=${encodeURIComponent(a.login)}`);
        const found = (r.body as { items?: Array<{ id: string; login: string }> }).items?.find((u) => u.login === a.login);
        if (!found) continue;
        a.userId = found.id;
      }
      await adminApi.updateUser(a.userId, { login: a.login, name: 'Бывший поставщик', role: 'executor', supplier_id: null, is_active: false })
        .catch((e) => console.warn(`уборка: учётка ${a.login}: ${e instanceof Error ? e.message : String(e)}`));
    }
    for (const a of apis) await a.dispose();
  },
});

export { expect };
