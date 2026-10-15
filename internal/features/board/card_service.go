package board

import (
	"context"
	"errors"
	"html/template"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/kitavrus/kanban-tree/internal/filestore"
)

// CardService — карточка заказа (GET) + мутации строк (услуги/товары/журнал). Автомобиль
// заказа в карточке только читается (one-car-order: задаётся при создании заказа).
type CardService struct {
	pool     *pgxpool.Pool
	tasks    *TaskRepo
	card     *CardRepo
	engine   *StockEngine
	subtasks *SubtaskRepo
	products *SubtaskProductRepo
	logs     *SubtaskLogRepo
	// Вложения (фаза 7): метаданные — attachments, файлы — files (WithFiles; nil —
	// загрузка/отдача недоступны, карточка читается как раньше).
	attachments *AttachmentRepo
	files       filestore.FileStore
}

func NewCardService(pool *pgxpool.Pool, tasks *TaskRepo, card *CardRepo, engine *StockEngine, subtasks *SubtaskRepo, products *SubtaskProductRepo, logs *SubtaskLogRepo) *CardService {
	return &CardService{pool: pool, tasks: tasks, card: card, engine: engine, subtasks: subtasks, products: products, logs: logs,
		attachments: NewAttachmentRepo(pool)}
}

// WithFiles — хранилище файлов вложений (фаза 7). Сеттер, а не параметр конструктора:
// существующие вызовы NewCardService (тесты фазы 6) не меняются.
func (s *CardService) WithFiles(fs filestore.FileStore) *CardService {
	s.files = fs
	return s
}

// Card — GET /api/v1/tasks/:id/card (AC-001): 7 запросов на пуле, без tx (read committed),
// собранные в один ответ. Subtasks/Stock и вложенные Products/Logs — всегда массивы, не null.
func (s *CardService) Card(ctx context.Context, a Actor, taskID string) (*OrderCardResponse, error) {
	id, ok := parseTaskID(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	ctx, span := otel.Tracer("board").Start(ctx, "tasks.card")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id))

	task, err := s.tasks.Get(ctx, id, a.Today)
	if errors.Is(err, errNotFound) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	col, err := s.card.Column(ctx, task.ColumnID)
	if errors.Is(err, errNotFound) {
		return nil, ErrColumnNotFound
	}
	if err != nil {
		return nil, err
	}

	car, err := s.card.Car(ctx, id)
	if errors.Is(err, errNotFound) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	subtasks, err := s.card.Subtasks(ctx, id)
	if err != nil {
		return nil, err
	}
	products, err := s.card.Products(ctx, id)
	if err != nil {
		return nil, err
	}
	logs, err := s.card.Logs(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range subtasks {
		if p, ok := products[subtasks[i].ID]; ok {
			subtasks[i].Products = p
		}
		for _, lg := range logs[subtasks[i].ID] {
			subtasks[i].Logs = append(subtasks[i].Logs, CardSubtaskLog{
				ID: lg.ID, Text: lg.Text, UserID: lg.UserID, AuthorName: lg.AuthorName,
				CreatedAt: lg.CreatedAt, UpdatedAt: lg.UpdatedAt,
				CanEdit: a.IsAdmin || lg.UserID == a.ID,
			})
		}
	}

	stock, err := s.card.Stock(ctx, id)
	if err != nil {
		return nil, err
	}

	attachments, err := s.attachments.List(ctx, id)
	if err != nil {
		return nil, err
	}

	works, parts, err := s.card.Totals(ctx, id)
	if err != nil {
		return nil, err
	}

	return &OrderCardResponse{
		Task:        toTaskResponse(*task),
		WorksAmount: works,
		PartsAmount: parts,
		Column:      *col,
		Car:         *car,
		Subtasks:    subtasks,
		Stock:       stock,
		Attachments: toCardAttachments(a, attachments),
	}, nil
}

// lockRowsScope — Lock Order (design-sql.md §Lock Order): TaskColumnID → LockTargetColumn
// (FOR SHARE) → LockTask (FOR UPDATE) → колонка сменилась между чтением column_id и
// LockTask → errTaskMovedRetry (ADR-100, withTx повторяет попытку). lockRowsScope
// используют row-мутации услуг/товаров/журнала (фаза 03).
func (s *CardService) lockRowsScope(ctx context.Context, tx pgx.Tx, taskID int64) (*Task, *columnFlags, error) {
	colID, err := s.tasks.TaskColumnID(ctx, tx, taskID)
	if errors.Is(err, errNotFound) {
		return nil, nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	col, err := s.tasks.LockTargetColumn(ctx, tx, colID)
	if err != nil {
		return nil, nil, columnRefError(err)
	}

	task, err := s.tasks.LockTask(ctx, tx, taskID)
	if err != nil {
		return nil, nil, taskLockError(err)
	}

	if task.ColumnID != col.ID {
		return nil, nil, errTaskMovedRetry
	}
	return task, col, nil
}

// lockRowsScopeFor — lockRowsScope с подменой 404 на сущность строки (design-errors.md
// §Error Mapping): row-пути (`/subtasks/:id`, `/subtask-products/:id`) отдают
// SUBTASK_NOT_FOUND/SUBTASK_PRODUCT_NOT_FOUND, а не TASK_NOT_FOUND, когда заказ удалён
// параллельно между разрешением родителя и lockRowsScope (Lock → 0 строк).
func (s *CardService) lockRowsScopeFor(ctx context.Context, tx pgx.Tx, taskID int64, notFound error) (*Task, *columnFlags, error) {
	task, col, err := s.lockRowsScope(ctx, tx, taskID)
	if errors.Is(err, ErrTaskNotFound) {
		return nil, nil, notFound
	}
	return task, col, err
}

// cardMutation — Re-fetch-after-write: ответ строит Card() ПОСЛЕ commit транзакции (не из
// RETURNING строки записи), Warnings — из Reconcile мутации (пустой массив, если Reconcile
// не запускался).
func (s *CardService) cardMutation(ctx context.Context, a Actor, taskID int64, r ReconcileResult) (*OrderCardMutationResponse, error) {
	card, err := s.Card(ctx, a, strconv.FormatInt(taskID, 10))
	if err != nil {
		return nil, err
	}
	warnings := r.Warnings
	if warnings == nil {
		warnings = []StockWarning{}
	}
	return &OrderCardMutationResponse{Card: *card, Warnings: warnings}, nil
}

// PrintData — данные для печати акта (GET /tasks/:id/print/:kind, print_ssr.go, AC-015/
// AC-016): Card() (тот же 401/404-контур доступа, что и у карточки, D19 — доступно и
// executor) + шапка (design-sql.md §Print Header Query). Kind/Firm/PrintedAt заполняет
// хендлер (config.FirmInfo сервису не нужен, время печати — момент рендера, не запроса).
func (s *CardService) PrintData(ctx context.Context, a Actor, taskID string) (*PrintVM, error) {
	id, ok := parseTaskID(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	ctx, span := otel.Tracer("board").Start(ctx, "tasks.print")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id))

	card, err := s.Card(ctx, a, taskID)
	if err != nil {
		return nil, err
	}

	header, err := s.card.PrintHeader(ctx, id)
	if errors.Is(err, errNotFound) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	return &PrintVM{
		Card:        *card,
		ClientName:  header.CompanyName,
		Phones:      header.Phones,
		Description: template.HTML(card.Task.DescriptionHTML), //nolint:gosec // ADR-006: хранится только серверно-экранированный HTML
		PrintedAt:   time.Now(),
	}, nil
}
