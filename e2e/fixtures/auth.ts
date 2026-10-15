import { expect, request, type APIRequestContext, type BrowserContext } from '@playwright/test';

// Вход и API как из браузера: cookie-сессия sid + double-submit CSRF (cookie kt_csrf
// → заголовок X-Csrf-Token). Логин только через реальный endpoint (§TD-8).

export const BASE_URL = process.env.E2E_BASE_URL ?? 'http://localhost:8080';

// Seed-админ локального стенда (make local-up-seed / cmd/seed).
export const SEED_ADMIN = { login: 'admin', password: 'admin1234' };

// Метка прогона: логины уникальны между прогонами на постоянной БД стенда.
// В ассертах используется сама переменная, не литерал (§TD-6.2).
export const RUN = (process.env.E2E_RUN ?? Date.now().toString(36)).toLowerCase();

export type Role = 'admin' | 'executor';
// timezone — часовой пояс IANA (user-timezone DS-031/DS-056): не передан при создании → Asia/Almaty,
// не передан при правке → не меняется (internal/features/users/dto.go, §TD-6.2).
export type NewUser = { login: string; name: string; role: Role; phone?: string; password: string; is_active?: boolean; timezone?: string };
export type User = { id: string; login: string; name: string; role: Role; phone: string; is_active: boolean; timezone: string };
/** GET /api/v1/auth/me и ответ входа (internal/features/auth/dto.go). */
export type Me = { id: string; login: string; name: string; role: Role; timezone: string };
/** PUT /api/v1/users/me/timezone — тело и ответ (internal/features/users/dto.go). */
export type TimezoneBody = { timezone: string };

// Wire-типы clients (internal/features/clients/dto.go, §TD-6.2).
export type Phone = { phone: string; note: string };
/** GET /api/v1/calendar (internal/features/calendar/dto.go), без списка заказов. */
export type CalendarResponse = {
  capacity: number;
  today: string;
  from: string;
  to: string;
  days: Array<{ date: string; occupied: number; free: number; task_ids: number[] }>;
};
export type NewClient = {
  company_name: string;
  phones: Phone[];
};
export type Client = {
  id: string;
  company_name: string;
  phones: Phone[];
  equipment_count: number;
  version: number;
};

// Wire-типы equipment (internal/features/clients/dto.go `## DTO Equipment`, §TD-6.2).
// Автомобиль — только госномер (обязателен; clients.NormalizePlateNumber на сервере).
export type NewEquipment = { plate_number: string };
export type Equipment = {
  id: string;
  client_id: string;
  plate_number: string;
  created_at: string;
  updated_at: string;
};

// Wire-типы catalog (internal/features/catalog/*_dto.go, §TD-6.2). Amount-поля
// (price/stock/min_stock) — JSON-число (catalog.Amount), в TS достаточно number|string.
export type CategoryKind = 'service' | 'product';
export type NewCategory = { kind: CategoryKind; name: string };
export type Category = { id: string; kind: CategoryKind; name: string; is_active: boolean; item_count: number };

// Остаток живёт в товаре (один склад): stock/min_stock/box/place — поля товара.
export type NewProduct = {
  name: string;
  sku?: string;
  unit?: string;
  price?: number;
  stock?: number;
  min_stock?: number;
  box?: string;
  place?: string;
};
export type Product = {
  id: string;
  name: string;
  sku: string;
  unit: string;
  price: number;
  is_active: boolean;
  stock: number;
  min_stock: number;
  box: string;
  place: string;
};


// Wire-типы поставщиков (internal/features/suppliers/dto.go, §TD-6.2).
export type NewSupplier = { name: string; contact_name?: string; phone?: string; address?: string; note?: string };
export type SupplierUpdate = NewSupplier & { is_active: boolean };
export type Supplier = {
  id: string;
  name: string;
  contact_name: string;
  phone: string;
  address: string;
  note: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};
export type SupplierList = { items: Supplier[]; total: number; limit: number; offset: number };

// Wire-типы services (internal/features/catalog/services_dto.go, §TD-6.2). price: null —
// «по факту»; products[].qty — дубли product_id объединяются суммой на сервере (D10).
export type NewServiceProduct = { product_id: string; qty: number };
export type NewService = {
  category_id: string;
  name: string;
  price?: number | null;
  products?: NewServiceProduct[];
};
export type ServiceProduct = {
  product_id: string;
  /** Товар строки состава мягко удалён из каталога (soft-delete-catalog DS-014). */
  product_is_deleted: boolean;
  name: string;
  unit: string;
  price: number;
  qty: number;
};
export type Service = {
  id: string;
  category_id: string;
  /** Категория услуги мягко удалена (soft-delete-catalog DS-014). */
  category_is_deleted: boolean;
  name: string;
  price: number | null;
  is_active: boolean;
  products: ServiceProduct[];
};

type Cookies = { cookies(): Promise<Array<{ name: string; value: string }>> };

async function csrfOf(jar: Cookies, get: () => Promise<unknown>): Promise<string> {
  let c = (await jar.cookies()).find((x) => x.name === 'kt_csrf');
  if (!c) {
    await get();
    c = (await jar.cookies()).find((x) => x.name === 'kt_csrf');
  }
  expect(c, 'cookie kt_csrf выдана').toBeTruthy();
  return c!.value;
}

/** Вход в контексте браузера (page.request делит cookies с вкладками контекста). */
export async function loginContext(context: BrowserContext, login: string, password: string) {
  const token = await csrfOf(context, () => context.request.get(`${BASE_URL}/login`));
  const res = await context.request.post(`${BASE_URL}/api/v1/auth/login`, {
    data: { login, password },
    headers: { 'X-Csrf-Token': token },
  });
  expect(res.status(), `вход ${login}: ${await res.text()}`).toBe(200);
}

/** Admin-клиент JSON API (отдельный от браузера). */
export class AdminApi {
  private constructor(private readonly ctx: APIRequestContext) {}

  static async login(login: string, password: string): Promise<AdminApi> {
    const ctx = await request.newContext({ baseURL: BASE_URL });
    const api = new AdminApi(ctx);
    const res = await ctx.post('/api/v1/auth/login', { data: { login, password }, headers: await api.headers() });
    expect(res.status(), `admin-вход ${login}: ${await res.text()}`).toBe(200);
    return api;
  }

  private async headers() {
    const state = { cookies: async () => (await this.ctx.storageState()).cookies };
    return { 'X-Csrf-Token': await csrfOf(state, () => this.ctx.get('/login')) };
  }

  async createUser(u: NewUser): Promise<User> {
    const res = await this.ctx.post('/api/v1/users', { data: u, headers: await this.headers() });
    expect(res.status(), `создание ${u.login}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async updateUser(id: string, u: Omit<NewUser, 'password'> & { password?: string; is_active: boolean }): Promise<User> {
    const res = await this.ctx.put(`/api/v1/users/${id}`, { data: u, headers: await this.headers() });
    expect(res.status(), `правка ${u.login}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async createClient(c: NewClient): Promise<Client> {
    const res = await this.ctx.post('/api/v1/clients', { data: c, headers: await this.headers() });
    expect(res.status(), `создание клиента ${c.company_name}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async updateClient(id: string, c: Omit<NewClient, 'phones'> & { phones: Phone[]; version: number }): Promise<Client> {
    const res = await this.ctx.put(`/api/v1/clients/${id}`, { data: c, headers: await this.headers() });
    expect(res.status(), `правка клиента ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async getClient(id: string): Promise<Client> {
    const res = await this.ctx.get(`/api/v1/clients/${id}`);
    expect(res.status(), `клиент ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async createEquipment(clientId: string, e: NewEquipment): Promise<Equipment> {
    const res = await this.ctx.post(`/api/v1/clients/${clientId}/equipment`, { data: e, headers: await this.headers() });
    expect(res.status(), `создание оборудования ${e.plate_number}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async createCategory(c: NewCategory): Promise<Category> {
    const res = await this.ctx.post('/api/v1/catalog-categories', { data: c, headers: await this.headers() });
    expect(res.status(), `создание категории ${c.name}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async createProduct(p: NewProduct): Promise<Product> {
    const res = await this.ctx.post('/api/v1/products', { data: p, headers: await this.headers() });
    expect(res.status(), `создание товара ${p.name}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  // ---- suppliers (internal/features/suppliers, DS-007) ----

  /** Список поставщиков: isActive не задан — все, включая архивных. */
  async listSuppliers(f: { q?: string; isActive?: boolean } = {}): Promise<SupplierList> {
    const p = new URLSearchParams({ limit: '500' });
    if (f.q) p.set('q', f.q);
    if (f.isActive !== undefined) p.set('is_active', String(f.isActive));
    const res = await this.ctx.get(`/api/v1/suppliers?${p}`);
    expect(res.status(), `поставщики: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async supplierByName(name: string): Promise<Supplier> {
    const list = await this.listSuppliers({ q: name });
    const s = list.items.find((x) => x.name === name);
    expect(s, `поставщик ${name}`).toBeTruthy();
    return s!;
  }

  async createSupplier(s: NewSupplier): Promise<Supplier> {
    const res = await this.ctx.post('/api/v1/suppliers', { data: s, headers: await this.headers() });
    expect(res.status(), `создание поставщика ${s.name}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async updateSupplier(id: string, s: SupplierUpdate): Promise<Supplier> {
    const res = await this.ctx.put(`/api/v1/suppliers/${id}`, { data: s, headers: await this.headers() });
    expect(res.status(), `правка поставщика ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /**
   * ФИЗИЧЕСКАЯ уборка заказа — служебный адрес DELETE /api/v1/test/purge/tasks/:id
   * (soft-delete-core §6.5, DS-027, DS-036; стенд обязан быть запущен с TEST_PURGE_ENABLED=true).
   * Обычный DELETE заказа теперь мягкий — тестовые заказы копились бы в разделе «Удалённые».
   * Работает и для действующего, и для мягко удалённого заказа. Бросает: 404 «Не найдено» —
   * заказа уже нет (сообщение содержит TASK_NOT_FOUND — уборка фикстуры его не логирует);
   * любой другой 404 — адрес не зарегистрирован, уборка выключена на стенде (ошибка с подсказкой).
   */
  async purgeTask(id: number): Promise<void> {
    const del = await this.ctx.delete(`/api/v1/test/purge/tasks/${id}`, { headers: await this.headers() });
    if (del.status() === 200) return;
    const text = await del.text();
    if (del.status() === 404) {
      const body = (() => { try { return JSON.parse(text) as { code?: string; message?: string }; } catch { return null; } })();
      if (body?.code === 'NOT_FOUND' && body.message === 'Не найдено') throw new Error(`purgeTask #${id}: TASK_NOT_FOUND — заказа уже нет`);
      throw new Error(`purgeTask #${id}: служебная уборка выключена — запустите стенд с TEST_PURGE_ENABLED=true (ответ: ${text})`);
    }
    throw new Error(`purgeTask #${id}: HTTP ${del.status()} ${text}`);
  }

  // ---- мягкое удаление (soft-delete-core §6.1–§6.4; типы — board-types.ts, §TD-6.2) ----

  /** Адрес ресурса вида kind: заказы (часть 1), колонки (часть 3), клиенты и единицы
   *  оборудования (часть 4), каталог и типы оборудования (часть 5), поставщики (часть 6). */
  private softDeletePath(kind: import('./board-types').DeletedKind, id: number | string): string {
    if (kind === 'tasks') return `/api/v1/tasks/${id}`;
    if (kind === 'columns') return `/api/v1/columns/${id}`;
    if (kind === 'clients') return `/api/v1/clients/${id}`;
    if (kind === 'equipment') return `/api/v1/equipment/${id}`;
    // часть 5 (soft-delete-catalog): товары, услуги, категории каталога и типы оборудования.
    if (kind === 'products' || kind === 'services' || kind === 'catalog-categories') return `/api/v1/${kind}/${id}`;
    // часть 6 (soft-delete-suppliers): поставщики.
    if (kind === 'suppliers') return `/api/v1/${kind}/${id}`;
    throw new Error(`softDelete: вид ${kind} подключается своей частью мастер-спеки soft-delete`);
  }

  /** Мягкое удаление записи с причиной: DELETE /api/v1/{ресурс}/:id, тело {reason}. */
  async softDelete(kind: 'columns', id: number | string, reason?: string): Promise<import('./board-types').ColumnDeleteResponse>;
  async softDelete(kind: 'clients', id: number | string, reason?: string): Promise<import('./board-types').ClientDeleteResponse>;
  async softDelete(kind: 'equipment', id: number | string, reason?: string): Promise<import('./board-types').EquipmentDeleteResponse>;
  async softDelete(kind: import('./board-types').CatalogKind, id: number | string, reason?: string): Promise<import('./board-types').CatalogDeleteResponse>;
  async softDelete(kind: import('./board-types').PartnerKind, id: number | string, reason?: string): Promise<import('./board-types').PartnerDeleteResponse>;
  async softDelete(kind: import('./board-types').DeletedKind, id: number | string, reason?: string): Promise<import('./board-types').TaskDeleteResponse>;
  async softDelete(kind: import('./board-types').DeletedKind, id: number | string, reason = 'e2e: создан по ошибке'): Promise<
    | import('./board-types').TaskDeleteResponse | import('./board-types').ColumnDeleteResponse
    | import('./board-types').ClientDeleteResponse | import('./board-types').EquipmentDeleteResponse
    | import('./board-types').CatalogDeleteResponse | import('./board-types').PartnerDeleteResponse
  > {
    const data: import('./board-types').DeleteBody = { reason };
    const res = await this.ctx.delete(this.softDeletePath(kind, id), { data, headers: await this.headers() });
    expect(res.status(), `мягкое удаление ${kind} ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** Восстановление записи: POST /api/v1/{ресурс}/:id/restore. Статус не проверяется — повтор ждёт 409. */
  async restore(kind: import('./board-types').DeletedKind, id: number | string): Promise<{ status: number; body: import('./board-types').TaskRestoreResponse }> {
    const res = await this.ctx.post(`${this.softDeletePath(kind, id)}/restore`, { headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** Восстановление клиента (вместе с единицами, ушедшими с ним): POST /api/v1/clients/:id/restore
   *  (soft-delete-clients §6.2). Статус не проверяется — повтор ждёт 409. */
  async restoreClient(id: string): Promise<{ status: number; body: import('./board-types').ClientRestoreResponse | import('./board-types').ClientRestoreError }> {
    const res = await this.ctx.post(`/api/v1/clients/${id}/restore`, { headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** Восстановление единицы: POST /api/v1/equipment/:id/restore (§6.4); клиент единицы в
   *  удалённых → 409 CLIENT_DELETED. Статус не проверяется. */
  async restoreEquipment(id: string): Promise<{ status: number; body: import('./board-types').EquipmentRestoreResponse | import('./board-types').ClientRestoreError }> {
    const res = await this.ctx.post(`/api/v1/equipment/${id}/restore`, { headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** Восстановление записи каталога либо типа оборудования: POST /api/v1/{ресурс}/:id/restore
   *  (soft-delete-catalog §6.2, §6.4, §6.6, §6.8). Статус не проверяется — ждут и 409 NAME_TAKEN. */
  async restoreCatalog(kind: import('./board-types').CatalogKind, id: string): Promise<{
    status: number;
    body: import('./board-types').CatalogRestoreResponse | import('./board-types').CatalogError;
  }> {
    const res = await this.ctx.post(`${this.softDeletePath(kind, id)}/restore`, { headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** Восстановление поставщика: POST /api/v1/suppliers/:id/restore
   *  (soft-delete-suppliers §6.4). Статус не проверяется — ждут и 409 NAME_TAKEN. */
  async restorePartner(kind: import('./board-types').PartnerKind, id: string): Promise<{
    status: number;
    body: import('./board-types').PartnerRestoreResponse | import('./board-types').PartnerError;
  }> {
    const res = await this.ctx.post(`${this.softDeletePath(kind, id)}/restore`, { headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** Попытка мягкого удаления поставщика без проверки статуса. */
  async tryDeletePartner(kind: import('./board-types').PartnerKind, id: string, reason = 'e2e: создан по ошибке'): Promise<{
    status: number;
    body: import('./board-types').PartnerDeleteResponse | import('./board-types').PartnerError;
  }> {
    const data: import('./board-types').DeleteBody = { reason };
    const res = await this.ctx.delete(this.softDeletePath(kind, id), { data, headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** Попытка мягкого удаления записи каталога без проверки статуса (ждут 409 PRODUCT_HAS_STOCK). */
  async tryDeleteCatalog(kind: import('./board-types').CatalogKind, id: string, reason = 'e2e: создан по ошибке'): Promise<{
    status: number;
    body: import('./board-types').CatalogDeleteResponse | import('./board-types').CatalogError;
  }> {
    const data: import('./board-types').DeleteBody = { reason };
    const res = await this.ctx.delete(this.softDeletePath(kind, id), { data, headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** Запись каталога по точному названию (в том числе архивная; удалённых список не отдаёт) —
   *  для записей, созданных тестом через экран: их id нужен для уборки (TS-F10). */
  async findCatalogByName(kind: import('./board-types').CatalogKind, name: string): Promise<{ id: string; name: string } | undefined> {
    const params: Record<string, string> = { include_archived: 'true', limit: '2000' };
    if (kind !== 'catalog-categories') params.q = name;
    const res = await this.ctx.get(`/api/v1/${kind}`, { params });
    expect(res.status(), `поиск ${kind} «${name}»: ${await res.text()}`).toBe(200);
    const list = (await res.json()) as { items: Array<{ id: string; name: string }> };
    return list.items.find((x) => x.name === name);
  }

  /**
   * ФИЗИЧЕСКАЯ уборка записи вида kind — служебный адрес DELETE /api/v1/test/purge/:kind/:id
   * (soft-delete DS-027; стенд — с TEST_PURGE_ENABLED=true). Общий помощник уборки тестовых
   * данных (TS-F9): клиенты и единицы оборудования удаляются мягко и без него копились бы в
   * разделе «Удалённые». Работает и для действующей, и для мягко удалённой записи. Возвращает
   * HTTP-статус: 200 — убрано; 404 — записи уже нет; 409 — запись используется (на клиента либо
   * единицу ссылаются заказы или строки документов — сначала убрать их, напр. purgeTask).
   */
  async purge(kind: import('./board-types').PurgeKind, id: number | string): Promise<number> {
    const res = await this.ctx.delete(`/api/v1/test/purge/${kind}/${id}`, { headers: await this.headers() });
    const status = res.status();
    if (status === 404) {
      const body = (await res.json().catch(() => null)) as { code?: string; message?: string } | null;
      if (!(body?.code === 'NOT_FOUND' && body.message === 'Не найдено')) {
        throw new Error(`purge ${kind} ${id}: служебная уборка выключена — запустите стенд с TEST_PURGE_ENABLED=true`);
      }
    }
    return status;
  }

  /** Уборка клиента теста вместе с его единицами: сначала заказы клиента (purgeTask — с их
   *  документами), затем сам клиент. Ошибки уборки не роняют тест — пишутся в лог. */
  async purgeClient(id: string, taskIds: number[] = []): Promise<void> {
    for (const taskId of taskIds) await this.purgeTask(taskId).catch(() => undefined);
    const status = await this.purge('clients', id).catch((e) => {
      console.warn(`purgeClient ${id}: ${e instanceof Error ? e.message : String(e)}`);
      return 0;
    });
    if (status === 409) console.warn(`purgeClient ${id}: запись используется — остались заказы либо документы клиента`);
  }

  /** Список удалённого одного вида: GET /api/v1/deleted?kind=… */
  async listDeleted(kind: import('./board-types').DeletedKind, filters: import('./board-types').DeletedListQuery = {}): Promise<import('./board-types').DeletedListResponse> {
    const params: Record<string, string> = { kind };
    for (const [k, v] of Object.entries(filters)) if (v !== undefined && v !== '') params[k] = String(v);
    const res = await this.ctx.get('/api/v1/deleted', { params });
    expect(res.status(), `список удалённого ${kind}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** Счётчики вкладок раздела «Удалённые»: GET /api/v1/deleted/summary. */
  async deletedSummary(): Promise<import('./board-types').DeletedSummaryResponse> {
    const res = await this.ctx.get('/api/v1/deleted/summary');
    expect(res.status(), `счётчики удалённого: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async createService(s: NewService): Promise<Service> {
    const res = await this.ctx.post('/api/v1/services', { data: { products: [], ...s }, headers: await this.headers() });
    expect(res.status(), `создание услуги ${s.name}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  // ---- board (internal/features/board/*_dto.go, §TD-6.2 — типы в board-types.ts) ----

  async createColumn(c: import('./board-types').NewColumn): Promise<import('./board-types').ColumnMutationResponse> {
    const res = await this.ctx.post('/api/v1/columns', { data: c, headers: await this.headers() });
    expect(res.status(), `создание колонки ${c.name}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async updateColumn(id: string, c: import('./board-types').NewColumn): Promise<import('./board-types').ColumnMutationResponse> {
    const res = await this.ctx.put(`/api/v1/columns/${id}`, { data: c, headers: await this.headers() });
    expect(res.status(), `правка колонки ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** МЯГКОЕ удаление колонки с причиной (soft-delete-board-columns §6.1, TS-F8): DELETE
   *  /api/v1/columns/:id, тело {reason} → 200 {id, deleted_at}; колонка уходит в раздел
   *  «Удалённые». Физическая уборка тестовых данных — purgeColumn. */
  async deleteColumn(id: string, reason = 'e2e: создана по ошибке'): Promise<import('./board-types').ColumnDeleteResponse> {
    return this.softDelete('columns', id, reason);
  }

  /** Восстановление колонки: POST /api/v1/columns/:id/restore (§6.2). Статус не проверяется —
   *  повтор ждёт 409 NOT_DELETED. */
  async restoreColumn(id: string): Promise<{ status: number; body: import('./board-types').ColumnRestoreResponse | import('./board-types').ColumnRestoreError }> {
    const res = await this.ctx.post(`/api/v1/columns/${id}/restore`, { headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /**
   * ФИЗИЧЕСКАЯ уборка колонки — служебный адрес DELETE /api/v1/test/purge/columns/:id
   * (soft-delete-board-columns §6.3, DS-027, TS-F8; стенд — с TEST_PURGE_ENABLED=true): обычный
   * DELETE колонки теперь мягкий — тестовые колонки копились бы во вкладке «Колонки» раздела
   * «Удалённые». Работает и для действующей, и для мягко удалённой колонки. Бросает: 404 «Не
   * найдено» — колонки уже нет (сообщение содержит COLUMN_NOT_FOUND); 409 — на колонку
   * ссылаются заказы (сообщение содержит COLUMN_NOT_EMPTY); любой другой 404 — адрес не
   * зарегистрирован, уборка выключена на стенде (ошибка с подсказкой).
   */
  async purgeColumn(id: string): Promise<void> {
    const del = await this.ctx.delete(`/api/v1/test/purge/columns/${id}`, { headers: await this.headers() });
    if (del.status() === 200) return;
    const text = await del.text();
    const body = (() => { try { return JSON.parse(text) as { code?: string; message?: string }; } catch { return null; } })();
    if (del.status() === 404) {
      if (body?.code === 'NOT_FOUND' && body.message === 'Не найдено') throw new Error(`purgeColumn ${id}: COLUMN_NOT_FOUND — колонки уже нет`);
      throw new Error(`purgeColumn ${id}: служебная уборка выключена — запустите стенд с TEST_PURGE_ENABLED=true (ответ: ${text})`);
    }
    if (del.status() === 409 && body?.code === 'COLUMN_NOT_EMPTY') throw new Error(`purgeColumn ${id}: COLUMN_NOT_EMPTY — на колонку ссылаются заказы`);
    throw new Error(`purgeColumn ${id}: HTTP ${del.status()} ${text}`);
  }

  async listColumns(): Promise<{ items: import('./board-types').Column[] }> {
    const res = await this.ctx.get('/api/v1/columns?limit=100');
    expect(res.status()).toBe(200);
    return res.json();
  }

  // ---- календарь загрузки сервиса (internal/features/calendar) ----

  /** Без from/to — текущий месяц (сервер берёт «сегодня» в поясе пользователя). */
  async getCalendar(from = '', to = ''): Promise<CalendarResponse> {
    const res = await this.ctx.get(`/api/v1/calendar?from=${from}&to=${to}`);
    expect(res.status(), `календарь ${from}..${to}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async getCalendarSettings(): Promise<{ max_cars: number }> {
    const res = await this.ctx.get('/api/v1/calendar/settings');
    expect(res.status(), `настройки календаря: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async setCalendarMaxCars(maxCars: number): Promise<void> {
    const res = await this.ctx.put('/api/v1/calendar/settings', { data: { max_cars: maxCars }, headers: await this.headers() });
    expect(res.status(), `вместимость ${maxCars}: ${await res.text()}`).toBe(200);
  }

  async createTask(t: import('./board-types').NewTask): Promise<import('./board-types').Task> {
    const res = await this.ctx.post('/api/v1/tasks', { data: t, headers: await this.headers() });
    expect(res.status(), `создание заказа ${t.title}: ${await res.text()}`).toBe(201);
    const body: import('./board-types').TaskMutationResponse = await res.json();
    return body.task;
  }

  /** Не проверяет статус — BD-010 (конфликт версии) ждёт 409, а не 200. */
  async updateTask(id: number, t: import('./board-types').UpdateTask): Promise<{ status: number; body: import('./board-types').TaskMutationResponse }> {
    const res = await this.ctx.put(`/api/v1/tasks/${id}`, { data: t, headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  async getTask(id: number): Promise<import('./board-types').Task> {
    const res = await this.ctx.get(`/api/v1/tasks/${id}`);
    expect(res.status(), `заказ ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async moveTask(id: number, m: import('./board-types').MoveTask): Promise<{ status: number; body: import('./board-types').TaskMutationResponse }> {
    const res = await this.ctx.post(`/api/v1/tasks/${id}/move`, { data: m, headers: await this.headers() });
    return { status: res.status(), body: await res.json() };
  }

  /** HTTP-статус GET заказа: 200 — на месте, 404 — удалён, в т.ч. мягко (проверки удаления заказа). */
  async taskHttpStatus(id: number): Promise<number> {
    return (await this.ctx.get(`/api/v1/tasks/${id}`)).status();
  }

  /** Мягкое удаление заказа с причиной (soft-delete-core §6.1) — заказ уходит в «Удалённые»;
   *  физическая уборка тестовых данных — purgeTask. */
  async deleteTask(id: number, reason = 'e2e: создан по ошибке'): Promise<import('./board-types').TaskDeleteResponse> {
    return this.softDelete('tasks', id, reason);
  }

  // ---- order card: услуги/товары/оборудование заказа (internal/features/board/card_dto.go,
  // фаза 6, ADR-101 — фикстуры e2e/fixtures/board.ts ходят сюда вместо pg-вставок) ----

  async getCard(taskId: number): Promise<import('./board-types').OrderCardResponse> {
    const res = await this.ctx.get(`/api/v1/tasks/${taskId}/card`);
    expect(res.status(), `карточка заказа ${taskId}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** Фаза 7: вложение заказа (multipart POST /tasks/:id/attachments, поле file). */
  async uploadAttachment(taskId: number, file: { name: string; mimeType: string; buffer: Buffer }): Promise<import('./board-types').OrderCardMutationResponse> {
    const res = await this.ctx.post(`/api/v1/tasks/${taskId}/attachments`, {
      multipart: { file },
      headers: await this.headers(),
    });
    expect(res.status(), `вложение заказа ${taskId}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async createSubtask(taskId: number, serviceId: string): Promise<import('./board-types').OrderCardMutationResponse> {
    const res = await this.ctx.post(`/api/v1/tasks/${taskId}/subtasks`, {
      data: { service_id: serviceId },
      headers: await this.headers(),
    });
    expect(res.status(), `создание услуги заказа ${taskId}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async updateSubtask(
    id: string,
    s: {
      status: 'new' | 'in_progress' | 'done';
      assignee_id?: string | null;
      due_date?: string | null;
      price?: number | string | null;
      equipment_id?: string | null;
      // executor-payouts (D65): нет поля → значение не меняется (и флаг, и сумма);
      // executor_payout: null — очистить сумму.
      is_executor_paid?: boolean;
      executor_payout?: number | string | null;
    },
  ): Promise<import('./board-types').OrderCardMutationResponse> {
    const res = await this.ctx.put(`/api/v1/subtasks/${id}`, {
      data: { assignee_id: null, due_date: null, price: null, equipment_id: null, ...s },
      headers: await this.headers(),
    });
    expect(res.status(), `правка услуги ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  // ---- executor payouts (internal/features/executorpayouts, D69/D70) ----

  async listExecutorPayouts(params: { assignee?: string; status?: 'all' | 'paid' | 'unpaid'; only_done?: boolean } = {}): Promise<import('./board-types').PayoutListResponse> {
    const q = new URLSearchParams({ assignee: params.assignee ?? 'all', limit: '1000' });
    if (params.status) q.set('status', params.status);
    if (params.only_done) q.set('only_done', 'true');
    const res = await this.ctx.get(`/api/v1/executor-payouts?${q}`);
    expect(res.status(), `список выплат: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async executorPayoutsSummary(assignee: string): Promise<import('./board-types').PayoutSummaryResponse> {
    const res = await this.ctx.get(`/api/v1/executor-payouts/summary?assignee=${encodeURIComponent(assignee)}`);
    expect(res.status(), `итоги выплат: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** D72: правка только суммы выплаты услуги из отчёта (null — очистить). */
  async updateExecutorPayout(subtaskId: string, payout: number | null): Promise<import('./board-types').PayoutItem> {
    const res = await this.ctx.put(`/api/v1/executor-payouts/${subtaskId}`, {
      data: { executor_payout: payout },
      headers: await this.headers(),
    });
    expect(res.status(), `сумма выплаты услуги ${subtaskId}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async markExecutorPayouts(subtaskIds: string[], paid: boolean): Promise<import('./board-types').PayoutMarkResponse> {
    const res = await this.ctx.post('/api/v1/executor-payouts/mark', {
      data: { subtask_ids: subtaskIds, is_executor_paid: paid },
      headers: await this.headers(),
    });
    expect(res.status(), `отметка выплат: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async createSubtaskProduct(
    subtaskId: string,
    p: { product_id: string; qty: number },
  ): Promise<import('./board-types').OrderCardMutationResponse> {
    const res = await this.ctx.post(`/api/v1/subtasks/${subtaskId}/products`, { data: p, headers: await this.headers() });
    expect(res.status(), `добавление товара услуге ${subtaskId}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  async deleteSubtaskProduct(id: string): Promise<import('./board-types').OrderCardMutationResponse> {
    const res = await this.ctx.delete(`/api/v1/subtask-products/${id}`, { headers: await this.headers() });
    expect(res.status(), `удаление товара услуги ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async deleteSubtask(id: string): Promise<import('./board-types').OrderCardMutationResponse> {
    const res = await this.ctx.delete(`/api/v1/subtasks/${id}`, { headers: await this.headers() });
    expect(res.status(), `удаление услуги ${id}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** Единица в оборудование заказа. */
  async addTaskEquipment(taskId: number, equipmentId: string): Promise<import('./board-types').OrderCardMutationResponse> {
    const data: import('./board-types').NewTaskEquipment = { equipment_id: equipmentId };
    const res = await this.ctx.post(`/api/v1/tasks/${taskId}/equipment`, {
      data,
      headers: await this.headers(),
    });
    expect(res.status(), `оборудование заказа ${taskId}: ${await res.text()}`).toBe(201);
    return res.json();
  }

  /** Убрать единицу из оборудования заказа (DELETE /tasks/:id/equipment/:equipment_id). */
  async removeTaskEquipment(taskId: number, equipmentId: string): Promise<import('./board-types').OrderCardMutationResponse> {
    const res = await this.ctx.delete(`/api/v1/tasks/${taskId}/equipment/${equipmentId}`, { headers: await this.headers() });
    expect(res.status(), `единица ${equipmentId} из заказа ${taskId}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  // ---- stock (internal/features/board/stock_dto.go, §TD-6.2 — типы в board-types.ts) ----

  async getStockSummary(): Promise<import('./board-types').StockSummaryResponse> {
    const res = await this.ctx.get('/api/v1/stock/summary');
    expect(res.status(), `stock summary: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async getStock(params: { scope?: import('./board-types').StockScope; q?: string; limit?: number } = {}): Promise<import('./board-types').StockListResponse> {
    const p = new URLSearchParams();
    if (params.scope) p.set('scope', params.scope);
    if (params.q) p.set('q', params.q);
    p.set('limit', String(params.limit ?? 1000));
    const res = await this.ctx.get(`/api/v1/stock?${p.toString()}`);
    expect(res.status(), `stock list: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** Правка остатка товара (остаток живёт в товаре; тело — {stock}). */
  async adjustStock(productId: string, stock: number): Promise<import('./board-types').StockRow> {
    const res = await this.ctx.put(`/api/v1/stock/${productId}`, { data: { stock }, headers: await this.headers() });
    expect(res.status(), `правка остатка ${productId}: ${await res.text()}`).toBe(200);
    return res.json();
  }

  /** Пояс владельца сессии этого клиента (PUT /api/v1/users/me/timezone, DS-033). */
  async setMyTimezone(body: TimezoneBody): Promise<TimezoneBody> {
    const res = await this.ctx.put('/api/v1/users/me/timezone', { data: body, headers: await this.headers() });
    expect(res.status(), `смена своего пояса: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async me(): Promise<Me> {
    const res = await this.ctx.get('/api/v1/auth/me');
    expect(res.status(), `auth/me: ${await res.text()}`).toBe(200);
    return res.json();
  }

  async findByLogin(login: string): Promise<User> {
    const res = await this.ctx.get(`/api/v1/users?q=${encodeURIComponent(login)}`);
    expect(res.status()).toBe(200);
    const body: { items: User[] } = await res.json();
    const u = body.items.find((x) => x.login === login);
    expect(u, `сотрудник ${login}`).toBeTruthy();
    return u!;
  }

  async dispose() {
    await this.ctx.dispose();
  }
}
