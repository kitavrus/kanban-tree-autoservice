// Wire-типы доски — 1:1 по json-тегам DTO (source: internal/features/board/*_dto.go,
// §TD-6.2). Amount-поля (StockWarning.stock/min_stock) — numeric.Amount, в TS number|string.

export type StockWarning = {
  product_id: string;
  /** warehouses DS-004: предупреждение — по паре товар×склад. */
  warehouse_id: string;
  warehouse_name: string;
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
  // transfers-core DS-046: у заказа есть единица оборудования с location_status = in_transit
  // (бейдж «в пути» на доске).
  has_in_transit: boolean;
  // executor-payouts D68: услуг done с исполнителем без выплаты (метка «к выплате: N»).
  executor_unpaid_done: number;
};

export type Task = TaskCard & {
  paid_at: string | null;
  created_at: string;
  updated_at: string;
  description_html: string;
  /** transfers-sales DS-014: «Продано по накладным» — Σ сумм накладных «склад → клиент» заказа
   *  в статусах in_transit / received; уже входит в total_amount. */
  transfers_amount: number | string;
};

export type NewTask = {
  title: string;
  client_id: string;
  column_id: string;
  assignee_id?: string | null;
  due_date?: string | null;
  is_paid?: boolean | null;
  description?: string | null;
};

export type UpdateTask = NewTask & { version: number };

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
/** Виды записей каталога и типов оборудования (часть 5, soft-delete-catalog). */
export type CatalogKind = 'products' | 'services' | 'catalog-categories' | 'equipment-types';
/** 200 DELETE /api/v1/{products|services|catalog-categories|equipment-types}/:id
 *  (catalog.DeleteResponse / equipmenttypes.DeleteResponse, soft-delete-catalog §6.1, §6.3, §6.5, §6.7). */
export type CatalogDeleteResponse = { id: string; deleted_at: string };
/** 200 POST /api/v1/{…}/:id/restore (catalog.RestoreResponse, §6.2, §6.4, §6.6, §6.8). */
export type CatalogRestoreResponse = { id: string };
/** Отказы удаления и восстановления записей каталога (§6.1–§6.8). */
export type CatalogError = {
  code:
    | 'NOT_DELETED' | 'NAME_TAKEN' | 'PRODUCT_HAS_STOCK' | 'PRODUCT_NOT_FOUND' | 'SERVICE_NOT_FOUND'
    | 'CATEGORY_NOT_FOUND' | 'EQUIPMENT_TYPE_NOT_FOUND' | 'FORBIDDEN' | 'VALIDATION';
  message: string;
};
/** Виды «склад» и «поставщик» (часть 6, soft-delete-warehouses-suppliers). */
export type PartnerKind = 'warehouses' | 'suppliers';
/** 200 DELETE /api/v1/{warehouses|suppliers}/:id (warehouses.DeleteResponse / suppliers.DeleteResponse,
 *  soft-delete-warehouses-suppliers §6.1, §6.3). */
export type PartnerDeleteResponse = { id: string; deleted_at: string };
/** 200 POST /api/v1/{warehouses|suppliers}/:id/restore (§6.2, §6.4). */
export type PartnerRestoreResponse = { id: string };
/** Отказы удаления и восстановления склада и поставщика (§6.1–§6.4). */
export type PartnerError = {
  code:
    | 'NOT_DELETED' | 'NAME_TAKEN' | 'WAREHOUSE_IN_USE' | 'WAREHOUSE_LAST_ACTIVE' | 'WAREHOUSE_NOT_FOUND'
    | 'SUPPLIER_NOT_FOUND' | 'FORBIDDEN' | 'VALIDATION';
  message: string;
};
/** Виды служебной уборки, подключённые на стенде (части 1–6 мастер-спеки soft-delete). */
export type PurgeKind = 'tasks' | 'transfers' | 'columns' | 'clients' | 'equipment' | CatalogKind | PartnerKind;
/** 200 DELETE /api/v1/test/purge/:kind/:id (soft-delete-core §6.5, служебная уборка). */
export type PurgeResponse = { id: string };

// ---- раздел «Удалённые» (internal/features/softdelete/dto.go, §6.3, §6.4) ----
export type DeletedKind =
  | 'tasks' | 'transfers' | 'columns' | 'clients' | 'equipment' | 'products' | 'services'
  | 'catalog-categories' | 'equipment-types' | 'warehouses' | 'suppliers';
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
    tasks: number; transfers: number; columns: number; clients: number; equipment: number; products: number;
    services: number; catalog_categories: number; equipment_types: number; warehouses: number; suppliers: number;
  };
};
/** 409 служебной уборки DELETE /api/v1/test/purge/tasks/:id (soft-delete-core DS-036; мягкое удаление
 *  заказа этот отказ больше не отдаёт — soft-delete-transfers DS-006): у заказа есть действующие документы
 *  в requested / in_transit / received; transfer_ids — их номера по возрастанию. */
export type ApiErrorTaskHasTransfers = { code: 'TASK_HAS_TRANSFERS'; message: string; details: { transfer_ids: number[] } };

export type ApiErrorSubtask = { id: string; service_name: string; status: 'new' | 'in_progress' | 'done' };
export type ApiErrorSubtasks = { code: 'SUBTASKS_NOT_DONE'; message: string; details: { subtasks: ApiErrorSubtask[] } };

// Wire-типы экрана «Остатки» (internal/features/board/stock_dto.go, §TD-6.2). Amount-поля
// (stock/min_stock/pending/forecast) — numeric.Amount, на wire JSON-число без кавычек.
export type StockScope = 'risky' | 'all';
export type StockState = 'out' | 'low' | 'will_low' | 'ok';

// Пара товар×склад в строке «Остатков» (warehouses DS-004).
export type StockWarehouseCell = {
  warehouse_id: string;
  stock: number | string;
  min_stock: number | string;
  pending: number | string;
  state: StockState;
};

export type StockRow = {
  product_id: string;
  /** Товар удалён из каталога, но на складе числится его остаток (soft-delete-catalog DS-042). */
  is_deleted: boolean;
  /** Товар в архиве (не удалён), но на удалённом складе числится его остаток — бейдж «в архиве»
   *  (board.StockRow, fix-loop 1 части 6 soft-delete-warehouses-suppliers). */
  is_archived: boolean;
  name: string;
  unit: string;
  box: string;
  place: string;
  stock: number;
  min_stock: number;
  pending: number;
  forecast: number;
  state: StockState;
  /** «В пути» — перемещения (подфаза B); в подфазе A всегда 0. */
  in_transit: number | string;
  by_warehouse: StockWarehouseCell[];
};

export type StockListResponse = {
  /** Склады отчёта: активные и — с ненулевым остатком — архивные (is_archived) и мягко удалённые
   *  (is_deleted) (board.StockWarehouse, soft-delete-warehouses-suppliers §6.5, DS-049). */
  warehouses: Array<{ id: string; name: string; is_archived: boolean; is_deleted: boolean }>;
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

export type EquipmentHistory = { task_id: number; title: string; created_at: string };

export type CardEquipment = {
  equipment_id: string;
  /** Единица мягко удалена — бейдж «удалён» в блоке «Оборудование» (soft-delete-clients §6.5). */
  is_deleted: boolean;
  type_name: string;
  brand: string;
  model: string;
  serial: string;
  completeness: string;
  condition: string;
  history: EquipmentHistory[];
  // Местоположение единицы (transfers-core DS-008, §6.9) — читается с equipment:
  // warehouse_* — только at_warehouse; active_transfer_id — незавершённый документ
  // «Перемещение»; transit_to_name — пока единица в пути: склад назначения документа либо
  // «клиент» (документ «склад → клиент», transfers-partners DS-013).
  location_status: EquipmentLocationStatus;
  warehouse_id: string | null;
  warehouse_name: string | null;
  active_transfer_id: number | null;
  transit_to_name: string | null;
  /** Направление незавершённого документа единицы (null — документа нет). */
  active_transfer_direction: import('./auth').TransferDirection | null;
};

/** equipment.location_status (CHECK миграции 0014). */
export type EquipmentLocationStatus = 'at_client' | 'at_warehouse' | 'in_transit';

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
  /** warehouses DS-005: склад списания строки. */
  warehouse_id: string;
  warehouse_name: string;
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
  equipment_id: string | null;
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
  warehouse_id: string;
  stock: number | string;
  min_stock: number | string;
  task_reserved: number | string;
};

export type OrderCardResponse = {
  task: Task;
  column: CardColumn;
  equipment: CardEquipment[];
  subtasks: CardSubtask[];
  stock: CardStock[];
  /** warehouses DS-005: склады select'а строки товара (активные + архивные, на которые ссылаются строки). */
  /** is_deleted — склад мягко удалён: есть в карточке, только если на него ссылаются строки заказа
   *  (board.CardWarehouse, soft-delete-warehouses-suppliers §6.5). */
  warehouses: Array<{ id: string; name: string; is_active: boolean; is_deleted: boolean }>;
  /** Фаза 7 (D21): вложения заказа, порядок загрузки. */
  attachments: CardAttachment[];
  /** transfers-partners §6.8: документы «Перемещение» заказа, новые сверху (всегда массив). */
  transfers: CardTransfer[];
};

/** Документ заказа в блоке «Перемещения заказа» (board.CardTransfer). warehouse_name — склад
 *  назначения (приём от клиента) либо отправления (возврат клиенту). */
export type CardTransfer = {
  id: number;
  direction: 'client_to_warehouse' | 'warehouse_to_client';
  status: 'requested' | 'in_transit' | 'received' | 'cancelled';
  warehouse_name: string;
  items_count: number;
  created_at: string;
  /** transfers-sales §6.3: сумма документа (0.00 для приёма от клиента и накладной без товаров). */
  amount: number | string;
};

/** Тело POST /api/v1/tasks/:id/equipment (board.TaskEquipmentCreateRequest). Упразднённый
 *  warehouse_id сервер игнорирует (DS-045) — в фикстурах не передаётся. */
export type NewTaskEquipment = { equipment_id: string; completeness?: string; condition?: string };

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
