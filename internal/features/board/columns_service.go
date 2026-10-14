package board

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

const (
	columnsDefaultLimit = 100
	columnsMaxLimit     = 1000
)

type ColumnService struct {
	pool   *pgxpool.Pool
	repo   *ColumnRepo
	engine *StockEngine
}

func NewColumnService(pool *pgxpool.Pool, repo *ColumnRepo, engine *StockEngine) *ColumnService {
	return &ColumnService{pool: pool, repo: repo, engine: engine}
}

func (s *ColumnService) List(ctx context.Context, limit, offset int) (*ColumnListResponse, error) {
	if limit <= 0 {
		limit = columnsDefaultLimit
	}
	limit = min(limit, columnsMaxLimit)
	offset = max(offset, 0)

	items, total, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	resp := &ColumnListResponse{Items: make([]ColumnResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, c := range items {
		resp.Items[i] = toColumnResponse(c)
	}
	return resp, nil
}

// Create — вся таблица под FOR UPDATE (LockAll), вставка с провизорной позицией в конец,
// затем Renumber на целевую (POST Position nil → в конец, иначе клэмп к 1..N+1).
// Create — userID пока не используется (Reconcile на создании — no-op без подзадач,
// see Service Interface §Create: "единообразие"), сохранён в сигнатуре ради единого
// профиля ColumnService.{Create,Update} и потенциального аудита создания колонки.
func (s *ColumnService) Create(ctx context.Context, userID string, in ColumnInput) (*ColumnMutationResponse, error) {
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	var result ColumnMutationResponse
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		columns, err := s.repo.LockAll(ctx, tx)
		if err != nil {
			return err
		}
		n := len(columns)
		target := n + 1
		if in.Position != nil {
			target = clamp(*in.Position, 1, n+1)
		}

		col, err := s.repo.Insert(ctx, tx, in, n+1)
		if err != nil {
			return err
		}
		if target != col.Position {
			if err := s.repo.Renumber(ctx, tx, col.ID, target); err != nil {
				return err
			}
			col.Position = target
		}
		col.TasksCount = 0
		result = ColumnMutationResponse{Column: toColumnResponse(*col), Warnings: []StockWarning{}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Update — смена consumes_stock пересчитывает склад по всем заказам колонки
// (StockEngine.Reconcile, AC-007, span columns.recalc); смена position — Renumber.
// Оба шага выполняются под тем же LockAll (ADR-104).
func (s *ColumnService) Update(ctx context.Context, id, userID string, in ColumnInput) (*ColumnMutationResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrColumnNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	var result ColumnMutationResponse
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		columns, err := s.repo.LockAll(ctx, tx)
		if err != nil {
			return err
		}
		var before *Column
		for i := range columns {
			if columns[i].ID == id {
				before = &columns[i]
				break
			}
		}
		if before == nil {
			return ErrColumnNotFound
		}
		consumesChanged := before.ConsumesStock != in.ConsumesStock
		becomesFinal := !before.IsFinal && in.IsFinal

		// becomesFinal — тот же инвариант «≥1 нефинальная колонка», что и Delete (ADR-104,
		// LV-054): PUT is_final=true на последней нефинальной колонке запрещён так же, как
		// её удаление — иначе на доске не остаётся места для незавершённых заказов.
		if becomesFinal {
			nonFinalOthers := 0
			for i := range columns {
				if columns[i].ID == id {
					continue
				}
				if !columns[i].IsFinal {
					nonFinalOthers++
				}
			}
			if nonFinalOthers == 0 {
				return ErrLastNonFinalColumn
			}

			// AC-003: заказы с незавершёнными услугами не должны оказаться в финальной
			// колонке в обход правила, которое уже действует для Move/Update заказа
			// (эскалация code review, design-errors.md/design-sql.md — MEDIUM-3).
			open, err := s.repo.OpenSubtasksForColumn(ctx, tx, id)
			if err != nil {
				return err
			}
			if len(open) > 0 {
				return ErrSubtasksNotDone.WithDetail("subtasks", open)
			}
		}

		updated, err := s.repo.Update(ctx, tx, id, in)
		if err != nil {
			if errors.Is(err, errNotFound) {
				return ErrColumnNotFound
			}
			return err
		}
		updated.Position = before.Position
		updated.TasksCount = before.TasksCount

		if in.Position != nil {
			target := clamp(*in.Position, 1, len(columns))
			if target != updated.Position {
				if err := s.repo.Renumber(ctx, tx, id, target); err != nil {
					return err
				}
				updated.Position = target
			}
		}

		warnings := []StockWarning{}
		if consumesChanged {
			recalcCtx, span := otel.Tracer("board").Start(ctx, "columns.recalc")
			taskIDs, err := s.repo.TaskIDsForUpdate(recalcCtx, tx, id)
			if err != nil {
				span.End()
				return err
			}
			byProduct := map[string]StockWarning{}
			for _, taskID := range taskIDs {
				res, err := s.engine.Reconcile(recalcCtx, tx, taskID, in.ConsumesStock, userID)
				if err != nil {
					span.End()
					return err
				}
				for _, w := range res.Warnings {
					byProduct[w.ProductID] = w
				}
			}
			span.End()
			for _, w := range byProduct {
				warnings = append(warnings, w)
			}
			sort.Slice(warnings, func(i, j int) bool { return warnings[i].ProductID < warnings[j].ProductID })
		}

		result = ColumnMutationResponse{Column: toColumnResponse(*updated), Warnings: warnings}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete — МЯГКОЕ удаление колонки (soft-delete-board-columns §6.1): только admin (роут +
// повторная проверка роли в сервисе — первой, до поиска записи, DS-003), причина обязательна
// (DS-020). Guard'ы COLUMN_NOT_EMPTY/LAST_NON_FINAL_COLUMN под тем же LockAll, что и сам UPDATE
// (ADR-104: инвариант «≥1 действующая нефинальная колонка» устойчив к гонке). Колонку
// удерживают только действующие заказы (DS-019). Уже удалена либо не существует → 404 (DS-024).
func (s *ColumnService) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*ColumnDeleteResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrColumnNotFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}

	var deletedAt time.Time
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		columns, err := s.repo.LockAll(ctx, tx)
		if err != nil {
			return err
		}
		var target *Column
		nonFinalOthers := 0
		for i := range columns {
			if columns[i].ID == id {
				target = &columns[i]
				continue
			}
			if !columns[i].IsFinal {
				nonFinalOthers++
			}
		}
		if target == nil {
			return ErrColumnNotFound
		}

		hasTasks, err := s.repo.HasTasks(ctx, tx, id)
		if err != nil {
			return err
		}
		if hasTasks {
			return ErrColumnNotEmpty
		}
		if !target.IsFinal && nonFinalOthers == 0 {
			return ErrLastNonFinalColumn
		}

		at, err := s.repo.SoftDelete(ctx, tx, id, a.ID, reason)
		if err != nil {
			if errors.Is(err, errNotFound) {
				return ErrColumnNotFound
			}
			return err
		}
		deletedAt = at
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Аудит (DS-030): факт удаления — в журнал приложения; причина в журнал не пишется.
	slog.InfoContext(ctx, "soft delete", "kind", "columns", "id", id, "user_id", a.ID, "action", "delete")
	return &ColumnDeleteResponse{ID: id, DeletedAt: deletedAt}, nil
}

// Restore — восстановление мягко удалённой колонки (soft-delete-board-columns §6.2): только
// admin. Одна транзакция под блокировкой всех колонок: три поля очищаются, колонка встаёт в
// конец доски (позиция = максимум среди действующих + 1, DS-019); название, цвет и признаки —
// прежние; удалённые заказы колонки остаются удалёнными. Не удалена → 409 NOT_DELETED (DS-024).
func (s *ColumnService) Restore(ctx context.Context, a Actor, id string) (*ColumnRestoreResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrColumnNotFound
	}

	var position int
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		columns, err := s.repo.LockAllAny(ctx, tx)
		if err != nil {
			return err
		}
		var target *Column
		for i := range columns {
			if columns[i].ID == id {
				target = &columns[i]
				break
			}
		}
		if target == nil {
			return ErrColumnNotFound
		}
		if !target.Deleted {
			return softdelete.ErrNotDeleted
		}
		pos, err := s.repo.Restore(ctx, tx, id)
		if err != nil {
			if errors.Is(err, errNotFound) {
				return softdelete.ErrNotDeleted
			}
			return err
		}
		position = pos
		return nil
	})
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "soft delete", "kind", "columns", "id", id, "user_id", a.ID, "action", "restore")
	return &ColumnRestoreResponse{ID: id, Position: position}, nil
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка колонки автотестами (soft-delete-board-columns §6.3,
// DS-027): удаляет колонку, действующую или мягко удалённую, на которую не ссылается ни один
// заказ; ссылаются заказы (включая удалённые) → 409 COLUMN_NOT_EMPTY. Колонки нет → 404
// NOT_FOUND (сервис уборки не раскрывает вид записи). Роль проверяет роут (softdelete.AdminOnly).
func (s *ColumnService) Purge(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errPurgeNotFound()
	}
	return withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.repo.LockAllAny(ctx, tx); err != nil {
			return err
		}
		if err := s.repo.Purge(ctx, tx, id); err != nil {
			switch {
			case errors.Is(err, errColumnHasTasks):
				return ErrColumnNotEmpty
			case errors.Is(err, errNotFound):
				return errPurgeNotFound()
			default:
				return err
			}
		}
		return nil
	})
}

func toColumnResponse(c Column) ColumnResponse {
	return ColumnResponse{
		ID: c.ID, Name: c.Name, Color: c.Color, IsFinal: c.IsFinal,
		ConsumesStock: c.ConsumesStock, Position: c.Position, TasksCount: c.TasksCount,
	}
}

// clamp — ограничивает v диапазоном [lo, hi] (design-sql.md §Column Renumber Query: "$2 —
// целевая позиция (1..N, clamp)").
func clamp(v, lo, hi int) int {
	switch {
	case v < lo:
		return lo
	case v > hi:
		return hi
	default:
		return v
	}
}
