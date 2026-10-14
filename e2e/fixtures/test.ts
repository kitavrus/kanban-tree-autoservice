import { test as base, expect, type Page } from '@playwright/test';
import { AdminApi, RUN, SEED_ADMIN, loginContext, type NewUser, type User } from './auth';

// §TD-4 «чистота консоли/сети»: любой console.error / pageerror (Uncaught) и любой
// ответ ≥ 400 от нашего origin валят тест. Тест, который проверяет ожидаемую ошибку,
// объявляет её через allowHttpErrors.
type Fixtures = {
  allowHttpErrors: (predicate: (url: string, status: number) => boolean) => void;
  /** Уникальный логин прогона: uniq('sidorov') → 'e2e-<run>-<worker>-sidorov'. */
  uniq: (name: string) => string;
  /** Новый сотрудник через admin API (пароль по умолчанию — password1). */
  makeUser: (u: Partial<NewUser> & { login: string }) => Promise<User & { password: string }>;
  /** Вкладка, вошедшая как указанный пользователь. */
  loginAs: (page: Page, login: string, password: string) => Promise<void>;
  /** page, вошедшая как администратор прогона. */
  adminPage: Page;
  /** Единая autouse-уборка тестовых данных: всё, что тест создал через adminApi, ловится
   *  автоматически и убирается в порядке зависимостей (см. комментарий у фикстуры). */
  _cleanup: CleanupRegistry;
  /** Заказ, созданный тестом через UI (без adminApi), тест регистрирует сам, если знает id. */
  _boardCleanup: (taskId: number) => void;
  /** Запись каталога либо тип оборудования, созданные тестом через экран (soft-delete-catalog
   *  TS-F10), тест регистрирует сам — `catalogCleanup(kind, id)`. */
  catalogCleanup: (kind: import('./board-types').CatalogKind, id: string) => void;
  /** Поставщик, созданный тестом через экран (soft-delete-suppliers
   *  TS-F11), тест регистрирует сам — `partnersCleanup(kind, id)`. */
  partnersCleanup: (kind: import('./board-types').PartnerKind, id: string) => void;
};

/** Набор одного теста для уборки — по видам, в порядке создания. */
type CleanupRegistry = {
  tasks: number[];
  columns: string[];
  clients: string[];
  equipment: string[];
  catalog: Array<{ kind: import('./board-types').CatalogKind; id: string }>;
  partners: Array<{ kind: import('./board-types').PartnerKind; id: string }>;
};

type WorkerFixtures = {
  adminApi: AdminApi;
  runAdmin: { login: string; password: string; id: string };
};

export const test = base.extend<Fixtures, WorkerFixtures>({
  // Админ прогона — отдельный от seed-админа: тесты могут менять его без риска
  // лишить стенд администратора.
  runAdmin: [
    async ({}, use, workerInfo) => {
      const seed = await AdminApi.login(SEED_ADMIN.login, SEED_ADMIN.password);
      const login = `e2e-${RUN}-${workerInfo.workerIndex}-admin`;
      const u = await seed.createUser({ login, name: 'Админ Прогона', role: 'admin', password: 'password1' });
      await seed.dispose();
      await use({ login, password: 'password1', id: u.id });
    },
    { scope: 'worker' },
  ],
  adminApi: [
    async ({ runAdmin }, use) => {
      const api = await AdminApi.login(runAdmin.login, runAdmin.password);
      await use(api);
      await api.dispose();
    },
    { scope: 'worker' },
  ],

  uniq: async ({}, use, testInfo) => {
    let n = 0;
    await use((name) => `e2e-${RUN}-${testInfo.workerIndex}-${testInfo.testId.slice(0, 6)}-${n++}-${name}`.toLowerCase());
  },

  makeUser: async ({ adminApi }, use) => {
    await use(async (u) => {
      const full = { name: `Сотрудник ${u.login}`, role: 'executor' as const, password: 'password1', ...u };
      const created = await adminApi.createUser(full);
      return { ...created, password: full.password };
    });
  },

  loginAs: async ({}, use) => {
    await use((page, login, password) => loginContext(page.context(), login, password));
  },

  adminPage: async ({ page, runAdmin }, use) => {
    await loginContext(page.context(), runAdmin.login, runAdmin.password);
    await use(page);
  },

  // _cleanup — ЕДИНАЯ autouse-уборка тестовых данных (гигиена стенда; soft-delete DS-027).
  // Обычное удаление в продукте мягкое, поэтому всё, что тест создал через adminApi, убирается
  // ФИЗИЧЕСКИ служебным адресом DELETE /api/v1/test/purge/:kind/:id (стенд — с
  // TEST_PURGE_ENABLED=true) — и действующее, и мягко удалённое тестом. adminApi общий на воркер
  // (scope: 'worker'), поэтому создающие методы оборачиваются ТОЛЬКО на время текущего теста:
  // убирается именно ЕГО набор, не чужие записи и не seed. Записи, созданные тестом через экран,
  // тест регистрирует сам: _boardCleanup(taskId), catalogCleanup(kind, id),
  // partnersCleanup(kind, id).
  //
  // Порядок — по зависимостям, один на все виды (раньше три фикстуры убирали каждая своё, и
  // держатели записи оказывались вне её набора — отсюда были десятки 409 на прогон):
  //   1. заказы (purgeTask) — держат клиентов, услуги, товары, колонки;
  //   2. колонки;
  //   3. клиенты (единицы уходят каскадом), затем единицы, заведённые у чужого клиента, — держат
  //      типы оборудования;
  //   4. услуги → товары → категории → типы оборудования (услуга держит товар составом, товар и
  //      услуга — категорию); журнал остатка товара уходит вместе с ним (служебная уборка);
  //   5. поставщики.
  // Статус уборки читается: 200 — убрано, 404 — записи уже нет (тест убрал её сам либо она ушла
  // каскадом); 409 «используется» — запись ДЕЙСТВИТЕЛЬНО держит то, что тест не регистрировал
  // (заказ, созданный через экран, записи из beforeAll):
  // она остаётся, такие отказы выводятся ОДНИМ итоговым предупреждением с числом и перечнем.
  // Любой другой статус и исключение (выключенная на стенде уборка) — отдельный console.warn:
  // глотать их молча нельзя.
  _cleanup: [
    async ({ adminApi }, use) => {
      const reg: CleanupRegistry = { tasks: [], columns: [], clients: [], equipment: [], catalog: [], partners: [] };
      const orig = {
        createTask: adminApi.createTask.bind(adminApi),
        createColumn: adminApi.createColumn.bind(adminApi),
        createClient: adminApi.createClient.bind(adminApi),
        createEquipment: adminApi.createEquipment.bind(adminApi),
        createCategory: adminApi.createCategory.bind(adminApi),
        createProduct: adminApi.createProduct.bind(adminApi),
        createService: adminApi.createService.bind(adminApi),
        createEquipmentType: adminApi.createEquipmentType.bind(adminApi),
        createSupplier: adminApi.createSupplier.bind(adminApi),
      };
      adminApi.createTask = (async (t) => {
        const task = await orig.createTask(t);
        reg.tasks.push(task.id);
        return task;
      }) as typeof adminApi.createTask;
      adminApi.createColumn = (async (c) => {
        const res = await orig.createColumn(c);
        reg.columns.push(res.column.id);
        return res;
      }) as typeof adminApi.createColumn;
      adminApi.createClient = (async (c) => {
        const r = await orig.createClient(c);
        reg.clients.push(r.id);
        return r;
      }) as typeof adminApi.createClient;
      adminApi.createEquipment = (async (clientId, e) => {
        const r = await orig.createEquipment(clientId, e);
        reg.equipment.push(r.id);
        return r;
      }) as typeof adminApi.createEquipment;
      adminApi.createCategory = (async (c) => {
        const r = await orig.createCategory(c);
        reg.catalog.push({ kind: 'catalog-categories', id: r.id });
        return r;
      }) as typeof adminApi.createCategory;
      adminApi.createProduct = (async (p) => {
        const r = await orig.createProduct(p);
        reg.catalog.push({ kind: 'products', id: r.id });
        return r;
      }) as typeof adminApi.createProduct;
      adminApi.createService = (async (s) => {
        const r = await orig.createService(s);
        reg.catalog.push({ kind: 'services', id: r.id });
        return r;
      }) as typeof adminApi.createService;
      adminApi.createEquipmentType = (async (t) => {
        const r = await orig.createEquipmentType(t);
        reg.catalog.push({ kind: 'equipment-types', id: r.id });
        return r;
      }) as typeof adminApi.createEquipmentType;
      adminApi.createSupplier = (async (sup) => {
        const r = await orig.createSupplier(sup);
        reg.partners.push({ kind: 'suppliers', id: r.id });
        return r;
      }) as typeof adminApi.createSupplier;

      await use(reg);

      Object.assign(adminApi, orig);
      const inUse: string[] = [];
      const sweep = async (kind: import('./board-types').PurgeKind, id: number | string) => {
        try {
          const status = await adminApi.purge(kind, id);
          if (status === 409) inUse.push(`${kind} ${id}`);
          else if (status !== 200 && status !== 404) console.warn(`уборка: ${kind} ${id} — неожиданный статус ${status}`);
        } catch (e) {
          console.warn(`уборка: не удалось убрать ${kind} ${id}: ${e instanceof Error ? e.message : String(e)}`);
        }
      };

      // 1. Заказы. Заказа уже нет (TASK_NOT_FOUND) — молча; всё остальное (409, выключенная на
      // стенде уборка) — неожиданно и идёт в console.warn.
      for (const id of reg.tasks) {
        await adminApi.purgeTask(id).catch((e) => {
          const msg = e instanceof Error ? e.message : String(e);
          if (!/TASK_NOT_FOUND/.test(msg)) console.warn(`уборка: не удалось удалить заказ #${id}: ${msg}`);
        });
      }
      // 2. Колонки. Тест сам убрал то, что создал, либо оставил в колонке заказ, созданный в
      // обход adminApi, — COLUMN_NOT_FOUND / COLUMN_NOT_EMPTY ожидаемо и не логируется.
      for (const id of reg.columns) {
        await adminApi.purgeColumn(id).catch((e) => {
          const msg = e instanceof Error ? e.message : String(e);
          if (!/COLUMN_NOT_FOUND|COLUMN_NOT_EMPTY/.test(msg)) console.warn(`уборка: не удалось удалить колонку ${id}: ${msg}`);
        });
      }
      // 3. Клиенты и единицы.
      for (const id of reg.clients) await sweep('clients', id);
      for (const id of reg.equipment) await sweep('equipment', id);
      // 4. Каталог и типы оборудования.
      const catalogOrder: Array<import('./board-types').CatalogKind> = ['services', 'products', 'catalog-categories', 'equipment-types'];
      for (const kind of catalogOrder) {
        for (const rec of reg.catalog.filter((r) => r.kind === kind)) await sweep(kind, rec.id);
      }
      // 5. Поставщики.
      for (const rec of reg.partners) await sweep(rec.kind, rec.id);

      if (inUse.length > 0) {
        console.warn(`уборка: не убрано записей — ${inUse.length} (409 «используется», остались ссылки): ${inUse.join(', ')}`);
      }
    },
    { auto: true },
  ],

  _boardCleanup: async ({ _cleanup }, use) => {
    await use((taskId) => { _cleanup.tasks.push(taskId); });
  },
  catalogCleanup: async ({ _cleanup }, use) => {
    await use((kind, id) => { _cleanup.catalog.push({ kind, id }); });
  },
  partnersCleanup: async ({ _cleanup }, use) => {
    await use((kind, id) => { _cleanup.partners.push({ kind, id }); });
  },

  allowHttpErrors: [
    async ({ page, baseURL }, use) => {
      const allowed: Array<(url: string, status: number) => boolean> = [];
      const problems: string[] = [];
      page.on('console', (m) => {
        // «Failed to load resource: … status of N» — браузерное эхо HTTP-ответа; сами
        // ответы ≥ 400 точно проверяет обработчик response ниже (с учётом allowHttpErrors)
        if (m.type() === 'error' && !m.text().startsWith('Failed to load resource: the server responded with a status of')) {
          problems.push(`console.error: ${m.text()}`);
        }
      });
      page.on('pageerror', (e) => problems.push(`Uncaught: ${e.message}`));
      page.on('response', (r) => {
        const url = r.url();
        if (baseURL && !url.startsWith(baseURL)) return;
        if (r.status() >= 400 && !allowed.some((f) => f(url, r.status()))) {
          problems.push(`HTTP ${r.status()} ${url}`);
        }
      });
      await use((predicate) => allowed.push(predicate));
      expect(problems, 'консоль и сеть чистые').toEqual([]);
    },
    { auto: true },
  ],
});

export { expect };
