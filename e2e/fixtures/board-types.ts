// Wire-типы доски — 1:1 по json-тегам DTO (source: internal/features/board/*_dto.go,
// §TD-6.2). Amount-поля (StockWarning.stock/min_stock) — numeric.Amount, в TS number|string.

export type StockWarning = {
  product_id: string;
  name: string;
  stock: number | string;
  min_stock: number | string;
  kind: 'negative' | 'below_min';
};

export type Column = {
  id: string;
  name: string;
  color: string;
  is_final: boolean;
  consumes_stock: boolean;
  position: number;
  tasks_count: number;
};

export type NewColumn = {
  name: string;
  color: string;
  is_final: boolean;
  consumes_stock: boolean;
  position?: number | null;
};

export type ColumnMutationResponse = { column: Column; warnings: StockWarning[] };

export type TaskLastLog = { text: string; author_name: string; created_at: string };

export type TaskCard = {
  id: number;
  column_id: string;
  client_id: string;
  client_name: string;
  /** Клиент заказа мягко удалён — бейдж «удалён» (soft-delete-clients §6.5). */
  client_is_deleted: boolean;
  title: string;
  assignee_id: string | null;
  assignee_name: string | null;
  // order-start-duration: план работ — начало (YYYY-MM-DD) и дней на работу (1..60);
  // due_date — плановое окончание start_date + duration_days − 1 (вычисляет сервер).
  start_date: string;
  duration_days: number;
  due_date: string | null;
  is_paid: boolean;
  position: string;
  version: number;
  subtasks_total: number;
  subtasks_done: number;
  subtasks_late: number;
  total_amount: number | string;
  last_log: TaskLastLog | null;
  // pagination DS-002: заказов колонки под фильтрами GET /tasks (в GET /tasks/:id — 0).
  column_total: number;
  // executor-payouts D68: услуг done с исполнителем без выплаты (метка «к выплате: N»).
  executor_unpaid_done: number;
};

export type Task = TaskCard & {
  paid_at: string | null;
  created_at: string;
  updated_at: string;
  description_html: string;
};

/** Шапка заказа — общая часть POST и PUT /api/v1/tasks (board.TaskHeader). order-start-duration:
 *  start_date/duration_days обязательны на сервере; adminApi.createTask/updateTask без них
 *  подставляют план (новый заказ — сегодня, 1 день; правка — текущий план заказа). due_date во
 *  входе не принимается (400). */
export type TaskHeader = {
  title: string;
  column_id: string;
  assignee_id?: string | null;
  start_date?: string | null;
  duration_days?: number | null;
  is_paid?: boolean | null;
  description?: string | null;
};

/** Тело POST /api/v1/tasks (board.TaskCreateRequest). one-car-order: заказ = ровно один
 *  автомобиль — plate_number и одно из equipment_id (найденный автомобиль, клиент — его
 *  владелец) / new_equipment (новый автомобиль клиента client_id либо new_client).
 *  adminApi.createTask без plate_number/equipment_id сам дописывает новый автомобиль клиента. */
export type NewTask = TaskHeader & {
  client_id?: string;
  plate_number?: string;
  equipment_id?: string;
  new_equipment?: Record<string, never>;
  new_client?: { company_name: string; phones: Array<{ phone: string; note?: string }> };
};

/** Тело PUT /api/v1/tasks/:id (board.TaskUpdateRequest): клиента и автомобиля нет — они не
 *  меняются после создания (client_id/equipment_id в теле → 400 BAD_REQUEST). */
export type UpdateTask = TaskHeader & { version: number };

export type MoveTask = { column_id: string; prev_id?: number | null; next_id?: number | null };

export type TaskMutationResponse = { task: Task; warnings: StockWarning[] };
/** 200 DELETE /api/v1/tasks/:id (soft-delete-core §6.1): удаление мягкое — номер заказа и момент
 *  удаления (RFC 3339, UTC); warnings — после возврата товаров на склад. */
export type TaskDeleteResponse = { id: number; deleted_at: string; warnings: StockWarning[] };
/** Тело DELETE /api/v1/{ресурс}/:id (soft-delete DS-028): причина 1–500 символов. */
export type DeleteBody = { reason: string };
/** 200 POST /api/v1/tasks/:id/restore (soft-delete-core §6.2). */
export type RestoreNotice = { code: 'column_replaced' | 'client_deleted' | 'equipment_deleted'; message: string };
export type TaskRestoreResponse = { id: number; warnings: StockWarning[]; notices: RestoreNotice[] };
/** 200 DELETE /api/v1/columns/:id (board.ColumnDeleteResponse, soft-delete-board-columns §6.1). */
export type ColumnDeleteResponse = { id: string; deleted_at: string };
/** 200 POST /api/v1/columns/:id/restore (board.ColumnRestoreResponse, §6.2): колонка — в конце доски. */
export type ColumnRestoreResponse = { id: string; position: number };
/** Отказы восстановления колонки (§6.2). */
export type ColumnRestoreError = { code: 'NOT_DELETED' | 'COLUMN_NOT_FOUND' | 'FORBIDDEN'; message: string };
/** 200 DELETE /api/v1/clients/:id (clients.ClientDeleteResponse, soft-delete-clients §6.1). */
export type ClientDeleteResponse = { id: string; deleted_at: string; equipment_deleted: number };
/** 200 POST /api/v1/clients/:id/restore (clients.ClientRestoreResponse, §6.2). */
export type ClientRestoreResponse = { id: string; equipment_restored: number };
/** 200 DELETE /api/v1/equipment/:id (clients.EquipmentDeleteResponse, §6.3). */
export type EquipmentDeleteResponse = { id: string; deleted_at: string };
/** 200 POST /api/v1/equipment/:id/restore (clients.EquipmentRestoreResponse, §6.4). */
export type EquipmentRestoreResponse = { id: string };
/** Отказы удаления и восстановления клиента и единицы (§6.1–§6.4). */
export type ClientRestoreError = {
  code: 'NOT_DELETED' | 'CLIENT_DELETED' | 'CLIENT_NOT_FOUND' | 'EQUIPMENT_NOT_FOUND' | 'FORBIDDEN';
  message: string;
};
/** Виды записей каталога (часть 5, soft-delete-catalog). */
export type CatalogKind = 'products' | 'services' | 'catalog-categories';
/** 200 DELETE /api/v1/{products|services|catalog-categories}/:id
 *  (catalog.DeleteResponse, soft-delete-catalog §6.1, §6.3, §6.5). */
export type CatalogDeleteResponse = { id: string; deleted_at: string };
/** 200 POST /api/v1/{…}/:id/restore (catalog.RestoreResponse, §6.2, §6.4, §6.6, §6.8). */
export type CatalogRestoreResponse = { id: string };
/** Отказы удаления и восстановления записей каталога (§6.1–§6.8). */
export type CatalogError = {
  code:
    | 'NOT_DELETED' | 'NAME_TAKEN' | 'PRODUCT_HAS_STOCK' | 'PRODUCT_NOT_FOUND' | 'SERVICE_NOT_FOUND'
    | 'CATEGORY_NOT_FOUND' | 'FORBIDDEN' | 'VALIDATION';
  message: string;
};
/** Вид «поставщик» (часть 6, soft-delete-suppliers). */
export type PartnerKind = 'suppliers';
/** 200 DELETE /api/v1/suppliers/:id (suppliers.DeleteResponse, soft-delete-suppliers §6.3). */
export type PartnerDeleteResponse = { id: string; deleted_at: string };
/** 200 POST /api/v1/suppliers/:id/restore (§6.4). */
export type PartnerRestoreResponse = { id: string };
/** Отказы удаления и восстановления поставщика (§6.3–§6.4). */
export type PartnerError = {
  code: 'NOT_DELETED' | 'NAME_TAKEN' | 'SUPPLIER_NOT_FOUND' | 'FORBIDDEN' | 'VALIDATION';
  message: string;
};
/** Виды служебной уборки, подключённые на стенде (части 1–6 мастер-спеки soft-delete). */
export type PurgeKind = 'tasks' | 'columns' | 'clients' | 'equipment' | CatalogKind | PartnerKind;
/** 200 DELETE /api/v1/test/purge/:kind/:id (soft-delete-core §6.5, служебная уборка). */
export type PurgeResponse = { id: string };

// ---- раздел «Удалённые» (internal/features/softdelete/dto.go, §6.3, §6.4) ----
export type DeletedKind =
  | 'tasks' | 'columns' | 'clients' | 'equipment' | 'products' | 'services'
  | 'catalog-categories' | 'suppliers';
export type DeletedItem = {
  kind: DeletedKind;
  id: string;
  title: string;
  subtitle: string;
  deleted_at: string;
  deleted_by: { id: string; name: string };
  delete_reason: string;
  deleted_with: 'client' | null;
  is_archived: boolean;
};
export type DeletedListResponse = { items: DeletedItem[]; total: number; limit: number; offset: number };
export type DeletedListQuery = { q?: string; deleted_from?: string; deleted_to?: string; deleted_by?: string; limit?: number; offset?: number };
export type DeletedSummaryResponse = {
  counts: {
    tasks: number; columns: number; clients: number; equipment: number; products: number;
    services: number; catalog_categories: number; suppliers: number;
  };
};
export type ApiErrorSubtask = { id: string; service_name: string; status: 'new' | 'in_progress' | 'done' };
export type ApiErrorSubtasks = { code: 'SUBTASKS_NOT_DONE'; message: string; details: { subtasks: ApiErrorSubtask[] } };

// Wire-типы экрана «Остатки» (internal/features/board/stock_dto.go, §TD-6.2). Amount-поля
// (stock/min_stock/pending/forecast) — numeric.Amount, на wire JSON-число без кавычек.
export type StockScope = 'risky' | 'all';
export type StockState = 'out' | 'low' | 'will_low' | 'ok';

export type StockRow = {
  product_id: string;
  /** Товар удалён из каталога, но на складе числится его остаток (soft-delete-catalog DS-042). */
  is_deleted: boolean;
  name: string;
  unit: string;
  box: string;
  place: string;
  stock: number;
  min_stock: number;
  pending: number;
  forecast: number;
  state: StockState;
};

export type StockListResponse = {
  items: StockRow[];
  total: number;
  limit: number;
  offset: number;
};

export type StockSummaryResponse = {
  products_total: number;
  low_count: number;
  out_count: number;
  will_low_count: number;
  risk_count: number;
};

// Wire-типы карточки заказа (internal/features/board/card_dto.go, фаза 6, §TD-6.2).
// Amount-поля — numeric.Amount, на wire JSON-число без кавычек (number|string в TS).

export type CardColumn = { id: string; name: string; is_final: boolean; consumes_stock: boolean };

/** Автомобиль заказа (board.CardCar, one-car-order): ровно один, задан при создании. */
export type CardCar = {
  equipment_id: string;
  /** Госномер автомобиля. */
  plate_number: string;
  /** Автомобиль мягко удалён — бейдж «удалён» в блоке «Автомобиль» (soft-delete-clients §6.5). */
  is_deleted: boolean;
};

export type CardSubtaskProduct = {
  id: string;
  product_id: string;
  /** Товар мягко удалён из каталога — бейдж «удалён» (soft-delete-catalog DS-014). */
  product_is_deleted: boolean;
  name: string;
  unit: string;
  position: number;
  qty: number | string;
  price: number | string | null;
  line_total: number | string;
};

export type CardSubtaskLog = {
  id: string;
  text: string;
  user_id: string;
  author_name: string;
  created_at: string;
  updated_at: string;
  can_edit: boolean;
};

export type CardSubtask = {
  id: string;
  service_id: string;
  service_name: string;
  /** Услуга мягко удалена из каталога — бейдж «удалён» (soft-delete-catalog DS-014). */
  service_is_deleted: boolean;
  status: 'new' | 'in_progress' | 'done';
  assignee_id: string | null;
  assignee_name: string | null;
  assignee_is_active: boolean | null;
  due_date: string | null;
  price: number | string | null;
  position: number;
  total: number | string;
  // executor-payouts D62–D65: выплата исполнителю (сумма — отдельная от price).
  is_executor_paid: boolean;
  executor_paid_at: string | null;
  executor_payout: number | string | null;
  products: CardSubtaskProduct[];
  logs: CardSubtaskLog[];
};

// Wire-типы отчёта «Выплаты» (internal/features/executorpayouts/dto.go, §TD-6.2).
export type PayoutItem = {
  subtask_id: string;
  task_id: number;
  task_title: string;
  client_name: string;
  service_name: string;
  status: 'new' | 'in_progress' | 'done';
  assignee_id: string;
  assignee_name: string;
  price: number | string | null;
  executor_payout: number | string | null;
  is_executor_paid: boolean;
  executor_paid_at: string | null;
};
export type PayoutListResponse = { items: PayoutItem[]; total: number; limit: number; offset: number };
export type PayoutSummaryItem = {
  assignee_id: string;
  assignee_name: string;
  unpaid_count: number;
  paid_count: number;
  to_pay: number | string;
  paid: number | string;
};
export type PayoutSummaryResponse = { items: PayoutSummaryItem[]; total: number; limit: number; offset: number };
export type PayoutMarkResponse = { updated: number };

export type CardStock = {
  product_id: string;
  stock: number | string;
  min_stock: number | string;
  task_reserved: number | string;
};

export type OrderCardResponse = {
  task: Task;
  column: CardColumn;
  car: CardCar;
  subtasks: CardSubtask[];
  stock: CardStock[];
  /** Фаза 7 (D21): вложения заказа, порядок загрузки. */
  attachments: CardAttachment[];
};

/** 1:1 с internal/features/board/card_dto.go CardAttachment (фаза 7). */
export type AttachmentKind = 'photo' | 'video' | 'pdf';
export type CardAttachment = {
  id: string;
  kind: AttachmentKind;
  original_name: string;
  content_type: string;
  size: number;
  user_id: string;
  author_name: string;
  created_at: string;
  url: string;
  can_delete: boolean;
};

export type OrderCardMutationResponse = { card: OrderCardResponse; warnings: StockWarning[] };
