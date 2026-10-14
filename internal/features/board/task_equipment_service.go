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

// AddEquipment — POST /api/v1/tasks/:id/equipment (AC-004/AC-005): LockTask (без колонки —
// design-sql.md §Lock Order, оборудование её не читает), проверка принадлежности клиенту
// заказа, Insert идемпотентно.
func (s *CardService) AddEquipment(ctx context.Context, a Actor, taskID string, in TaskEquipmentCreateRequest) (*OrderCardMutationResponse, error) {
	id, ok := parseTaskID(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "task_equipment.mutate")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id), attribute.String("op", "add"))

	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		task, err := s.tasks.LockTask(spanCtx, tx, id)
		if err != nil {
			return taskLockError(err)
		}

		clientID, err := s.equipment.EquipmentClient(spanCtx, tx, in.EquipmentID)
		if errors.Is(err, errNotFound) {
			return errorspkg.Validation(errorspkg.Field{Name: "equipment_id", Message: msgNotFound})
		}
		if err != nil {
			return err
		}
		if clientID != task.ClientID {
			return errorspkg.Validation(errorspkg.Field{Name: "equipment_id", Message: msgForeignClient})
		}

		_, err = s.equipment.Insert(spanCtx, tx, id, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.cardMutation(ctx, a, id, ReconcileResult{Warnings: []StockWarning{}})
}

// UpdateEquipment — PUT /api/v1/tasks/:id/equipment/:equipment_id. equipment_id — path,
// не тело: невалидный формат/отсутствующая строка → 404 (design-errors.md: «404 — только
// id в пути»).
func (s *CardService) UpdateEquipment(ctx context.Context, a Actor, taskID, equipmentID string, in TaskEquipmentUpdateRequest) (*OrderCardMutationResponse, error) {
	id, ok := parseTaskID(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	if uuid.Validate(equipmentID) != nil {
		return nil, ErrTaskEquipmentNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "task_equipment.mutate")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id), attribute.String("op", "update"))

	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.tasks.LockTask(spanCtx, tx, id); err != nil {
			return taskLockError(err)
		}
		if err := s.equipment.Update(spanCtx, tx, id, equipmentID, in); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrTaskEquipmentNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.cardMutation(ctx, a, id, ReconcileResult{Warnings: []StockWarning{}})
}

// RemoveEquipment — DELETE /api/v1/tasks/:id/equipment/:equipment_id. Единица выбрана в
// открытых услугах заказа → 409 TASK_EQUIPMENT_IN_USE + details.subtasks (AC-004).
func (s *CardService) RemoveEquipment(ctx context.Context, a Actor, taskID, equipmentID string) (*OrderCardMutationResponse, error) {
	id, ok := parseTaskID(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	if uuid.Validate(equipmentID) != nil {
		return nil, ErrTaskEquipmentNotFound
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "task_equipment.mutate")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id), attribute.String("op", "remove"))

	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.tasks.LockTask(spanCtx, tx, id); err != nil {
			return taskLockError(err)
		}
		open, err := s.equipment.UsedBy(spanCtx, tx, id, equipmentID)
		if err != nil {
			return err
		}
		if len(open) > 0 {
			return ErrTaskEquipmentInUse.WithDetail("subtasks", open)
		}
		if err := s.equipment.Delete(spanCtx, tx, id, equipmentID); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrTaskEquipmentNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.cardMutation(ctx, a, id, ReconcileResult{Warnings: []StockWarning{}})
}
