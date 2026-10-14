package board

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// CreateSubtask — POST /api/v1/tasks/:id/subtasks (AC-007/AC-008): финальная колонка —
// 409 SUBTASK_FINAL_COLUMN; услуга не найдена/в архиве — 400 field service_id; иначе
// Insert (позиция в конец) + CopyKit (типовой комплект по ТЕКУЩЕЙ цене товара) +
// Reconcile (design-sql.md §Lock Order: «Мутации услуг/товаров... Reconcile при изменении
// состава товаров»).
func (s *CardService) CreateSubtask(ctx context.Context, a Actor, taskID string, in SubtaskCreateRequest) (*OrderCardMutationResponse, error) {
	id, ok := parseTaskID(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "subtasks.mutate")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id), attribute.String("op", "create"))

	var reconcile ReconcileResult
	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		_, col, err := s.lockRowsScope(spanCtx, tx, id)
		if err != nil {
			return err
		}
		if col.IsFinal {
			return ErrSubtaskFinalColumn
		}

		exists, active, err := s.subtasks.ServiceState(spanCtx, tx, in.ServiceID)
		if err != nil {
			return err
		}
		if !exists {
			return errorspkg.Validation(errorspkg.Field{Name: "service_id", Message: msgNotFound})
		}
		if !active {
			return errorspkg.Validation(errorspkg.Field{Name: "service_id", Message: msgArchived})
		}

		subtaskID, err := s.subtasks.Insert(spanCtx, tx, id, in.ServiceID)
		if err != nil {
			if errors.Is(err, errNotFound) {
				// гонка: услуга стала неактивной/удалена между ServiceState и Insert.
				return errorspkg.Validation(errorspkg.Field{Name: "service_id", Message: msgNotFound})
			}
			return err
		}
		span.SetAttributes(attribute.String("subtask_id", subtaskID))
		if err := s.subtasks.CopyKit(spanCtx, tx, subtaskID, in.ServiceID); err != nil {
			return err
		}

		res, err := s.engine.Reconcile(spanCtx, tx, id, col.ConsumesStock, a.ID)
		if err != nil {
			return err
		}
		reconcile = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.cardMutation(ctx, a, id, reconcile)
}

// UpdateSubtask — PUT /api/v1/subtasks/:id (LWW, AC-007/AC-011): done→не-done в финальной
// колонке — 409 SUBTASK_FINAL_COLUMN; assignee_id — активный пользователь, ТОЛЬКО при
// смене значения (SD-007); equipment_id — единица из оборудования ЭТОГО заказа. БЕЗ
// Reconcile (состав товаров не меняется) — под тем же lockRowsScope (ADR-100).
func (s *CardService) UpdateSubtask(ctx context.Context, a Actor, subtaskID string, in SubtaskUpdateRequest) (*OrderCardMutationResponse, error) {
	if uuid.Validate(subtaskID) != nil {
		return nil, ErrSubtaskNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "subtasks.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("subtask_id", subtaskID), attribute.String("op", "update"))

	var taskID int64
	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		tid, err := s.subtasks.TaskIDOf(spanCtx, tx, subtaskID)
		if errors.Is(err, errNotFound) {
			return ErrSubtaskNotFound
		}
		if err != nil {
			return err
		}
		taskID = tid
		span.SetAttributes(attribute.Int64("task_id", tid))

		_, col, err := s.lockRowsScopeFor(spanCtx, tx, tid, ErrSubtaskNotFound)
		if err != nil {
			return err
		}

		cur, err := s.subtasks.Lock(spanCtx, tx, subtaskID, tid)
		if errors.Is(err, errNotFound) {
			return ErrSubtaskNotFound
		}
		if err != nil {
			return err
		}

		if col.IsFinal && cur.Status == SubtaskStatusDone && in.Status != SubtaskStatusDone {
			return ErrSubtaskFinalColumn
		}

		assigneeID := nilIfEmpty(in.AssigneeID)
		if !samePtr(cur.AssigneeID, assigneeID) && assigneeID != nil {
			ok, err := s.tasks.ActiveAssigneeExists(spanCtx, tx, *assigneeID)
			if err != nil {
				return err
			}
			if !ok {
				return errorspkg.Validation(errorspkg.Field{Name: "assignee_id", Message: msgNotFound})
			}
		}

		// Выплата исполнителю (D65/D66): поле не прислано → флаг прежний. Правила — по
		// ПЕРЕХОДУ, а не по итоговому состоянию: 409 — у выплаченной услуги СУЩЕСТВУЮЩЕГО
		// исполнителя меняют на другого или снимают, а отметка остаётся; 400 — запрос
		// переводит флаг false→true без исполнителя. Выплаченная услуга, оставшаяся без
		// исполнителя (сотрудника удалили, ON DELETE SET NULL), правится свободно, и ей
		// можно назначить исполнителя.
		executorPaid := cur.IsExecutorPaid
		if in.IsExecutorPaid != nil {
			executorPaid = *in.IsExecutorPaid
		}
		if cur.IsExecutorPaid && executorPaid && cur.AssigneeID != nil && !samePtr(cur.AssigneeID, assigneeID) {
			return ErrSubtaskExecutorPaid
		}
		if !cur.IsExecutorPaid && executorPaid && assigneeID == nil {
			return errorspkg.Validation(errorspkg.Field{Name: "is_executor_paid", Message: msgNoAssignee})
		}

		equipmentID := nilIfEmpty(in.EquipmentID)
		if equipmentID != nil {
			attached, err := s.equipment.Attached(spanCtx, tx, tid, *equipmentID)
			if err != nil {
				return err
			}
			if !attached {
				return errorspkg.Validation(errorspkg.Field{Name: "equipment_id", Message: msgNotInOrder})
			}
			// soft-delete-clients DS-014: мягко удалённую единицу услуге назначить нельзя; уже
			// назначенная остаётся (проверка — только при смене значения).
			if !samePtr(cur.EquipmentID, equipmentID) {
				deleted, err := s.equipment.Deleted(spanCtx, tx, *equipmentID)
				if err != nil {
					return err
				}
				if deleted {
					return errorspkg.Validation(errorspkg.Field{Name: "equipment_id", Message: msgNotFound})
				}
			}
		}

		var priceStr, payoutStr *string
		if in.Price != nil {
			v := in.Price.String()
			priceStr = &v
		}
		if in.ExecutorPayout.Value != nil {
			v := in.ExecutorPayout.Value.String()
			payoutStr = &v
		}

		if err := s.subtasks.Update(spanCtx, tx, subtaskID, subtaskUpdateRow{
			Status: in.Status, AssigneeID: assigneeID, DueDate: nilIfEmpty(in.DueDate),
			Price: priceStr, EquipmentID: equipmentID,
			IsExecutorPaid: executorPaid, SetExecutorPayout: in.ExecutorPayout.Set, ExecutorPayout: payoutStr,
		}); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.cardMutation(ctx, a, taskID, ReconcileResult{Warnings: []StockWarning{}})
}

// DeleteSubtask — DELETE /api/v1/subtasks/:id (AC-007/AC-010): без ограничения по
// финальной колонке (design-errors.md §Error Mapping — SUBTASK_FINAL_COLUMN только на
// Create/PUT). CASCADE товары/журнал — Reconcile возвращает списанное на склад.
func (s *CardService) DeleteSubtask(ctx context.Context, a Actor, subtaskID string) (*OrderCardMutationResponse, error) {
	if uuid.Validate(subtaskID) != nil {
		return nil, ErrSubtaskNotFound
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "subtasks.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("subtask_id", subtaskID), attribute.String("op", "delete"))

	var taskID int64
	var reconcile ReconcileResult
	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		tid, err := s.subtasks.TaskIDOf(spanCtx, tx, subtaskID)
		if errors.Is(err, errNotFound) {
			return ErrSubtaskNotFound
		}
		if err != nil {
			return err
		}
		taskID = tid
		span.SetAttributes(attribute.Int64("task_id", tid))

		_, col, err := s.lockRowsScopeFor(spanCtx, tx, tid, ErrSubtaskNotFound)
		if err != nil {
			return err
		}

		if _, err := s.subtasks.Lock(spanCtx, tx, subtaskID, tid); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskNotFound
			}
			return err
		}

		if err := s.subtasks.Delete(spanCtx, tx, subtaskID); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskNotFound
			}
			return err
		}

		res, err := s.engine.Reconcile(spanCtx, tx, tid, col.ConsumesStock, a.ID)
		if err != nil {
			return err
		}
		reconcile = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.cardMutation(ctx, a, taskID, reconcile)
}
