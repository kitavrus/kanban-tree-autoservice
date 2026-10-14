# Sequence Diagrams: phase-6-order-card

> Только новые потоки. Фазы: Request → Validation → Lock → Business → Persistence(+Reconcile) → Response. Линейные CRUD без побочных эффектов (журнал, PUT оборудования) не рисуются — по форме совпадают с §2 без Reconcile. Коды — `design-errors.md`.

## 1. Открытие карточки (deep-link `/?task=N`)
```mermaid
sequenceDiagram
    participant B as Browser (board.js + task-card.js)
    participant S as SSR GET /
    participant H as CardHandler.Card
    participant SV as CardService.Card
    participant DB as PostgreSQL
    B->>S: GET /?task=N (RequireSSR, no-store)
    S-->>B: board.html
    B->>B: parseFilters() игнорирует task; KT.taskCard.open(N)
    B->>H: GET /api/v1/tasks/N/card
    H->>SV: Card(actor, "N")
    SV->>DB: TaskRepo.Get (+description_html, total_amount, last_log)
    alt нет заказа
        SV-->>B: 404 TASK_NOT_FOUND → uiAlert, replaceState без task
    end
    SV->>DB: Column, Equipment(+history json_agg), Subtasks, Products, Logs, Stock (7 запросов, без N+1)
    SV-->>B: 200 OrderCardResponse (can_edit по актору)
    B->>B: render разделов, подсказки AC-009, итоги
```

## 2. Правка товара услуги в списывающей колонке (LWW + Reconcile)
```mermaid
sequenceDiagram
    participant B as Browser
    participant H as CardHandler
    participant SV as CardService
    participant R as Repos
    participant E as StockEngine
    participant DB as PostgreSQL
    B->>B: change qty → строка disabled
    B->>H: PUT /api/v1/subtask-products/:id {qty, price}
    H->>SV: UpdateSubtaskProduct(actor, id, req)
    SV->>SV: normalize/validate → 400 fields
    SV->>DB: BEGIN (withTx: ретрай 40P01/40001/errTaskMovedRetry ×3)
    SV->>R: ParentOf(id) → task_id, subtask_id (0 → 404 SUBTASK_PRODUCT_NOT_FOUND)
    SV->>R: TaskColumnID → LockTargetColumn FOR SHARE → LockTask FOR UPDATE
    alt column_id сменился (параллельный move)
        SV-->>SV: errTaskMovedRetry → повтор tx
    end
    SV->>R: Lock subtask_products FOR UPDATE (удалена → 404)
    SV->>R: UPDATE qty, price (LWW, tasks.version не трогаем)
    SV->>E: Reconcile(tx, task_id, consumes_stock, actor)
    E->>DB: deltas → products FOR UPDATE ORDER BY id → stock, task_stock, stock_movements
    E-->>SV: warnings (negative/below_min)
    SV->>DB: COMMIT
    SV->>SV: Card() после commit (re-fetch-after-write)
    SV-->>B: 200 {card, warnings}
    B->>B: applyCard (фокусные поля не трогать), итоги, подсказки, showWarnings
```

## 3. Добавление услуги (комплект + финальная колонка)
```mermaid
sequenceDiagram
    participant B as Browser
    participant SV as CardService.CreateSubtask
    participant DB as PostgreSQL
    B->>SV: POST /api/v1/tasks/N/subtasks {service_id}
    SV->>DB: withTx: TaskColumnID → columns FOR SHARE → tasks FOR UPDATE
    alt column.is_final
        SV-->>B: 409 SUBTASK_FINAL_COLUMN
    end
    SV->>DB: ServiceState (нет/архив → 400 fields[service_id])
    SV->>DB: INSERT subtasks (price = services.price, position = max+1)
    SV->>DB: INSERT subtask_products FROM service_products (qty, products.price)
    SV->>DB: Reconcile(N, consumes_stock) → task_consume при списывающей колонке
    SV->>DB: COMMIT → Card()
    SV-->>B: 201 {card, warnings}
```

## 4. Сохранение шапки со сменой клиента
```mermaid
sequenceDiagram
    participant B as Browser
    participant SV as TaskService.Update
    participant DB as PostgreSQL
    B->>B: клиент сменён и есть оборудование → KT.uiConfirm
    B->>SV: PUT /api/v1/tasks/N {…, description, is_paid, version}
    SV->>DB: withTx: LockTargetColumn → LockTask
    alt version ≠ current
        SV-->>B: 409 TASK_VERSION_CONFLICT
    end
    SV->>SV: paid-гард (executor) → 403 PAID_ADMIN_ONLY
    SV->>DB: ForeignInUse(N, new client)
    alt единица чужого клиента выбрана в услуге
        SV-->>B: 409 TASK_EQUIPMENT_IN_USE details.subtasks
    end
    SV->>DB: DeleteForeign → UPDATE tasks (version+1, description_html)
    SV-->>B: 200 {task, warnings}
    B->>SV: GET /api/v1/tasks/N/card (обновить оборудование)
```

## 5. Печать акта
```mermaid
sequenceDiagram
    participant B as Browser (новая вкладка)
    participant P as SSR print
    participant SV as CardService.PrintData
    B->>P: GET /tasks/N/print/handover (RequireSSR, no-store)
    alt kind ∉ {intake, handover} или нет заказа
        P-->>B: 404 StatusPages.NotFound
    end
    P->>SV: PrintData → Card + PrintHeader (клиент, телефоны)
    P-->>B: layout "print" (light) + FIRM_* + описание + услуги/товары/итог/оплата/подписи
    B->>B: window.print() / PDF средствами браузера
```
