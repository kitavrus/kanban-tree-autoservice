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

// CreateSubtaskProduct — POST /api/v1/subtasks/:id/products (AC-008/AC-010): товар не
// найден/в архиве — 400 field product_id; Insert (позиция в конец, цена — ТЕКУЩАЯ
// products.price) + Reconcile.
func (s *CardService) CreateSubtaskProduct(ctx context.Context, a Actor, subtaskID string, in SubtaskProductCreateRequest) (*OrderCardMutationResponse, error) {
	if uuid.Validate(subtaskID) != nil {
		return nil, ErrSubtaskNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "subtask_products.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("subtask_id", subtaskID), attribute.String("op", "create"))

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

		exists, active, err := s.products.ProductState(spanCtx, tx, in.ProductID)
		if err != nil {
			return err
		}
		if !exists {
			return errorspkg.Validation(errorspkg.Field{Name: "product_id", Message: msgNotFound})
		}
		if !active {
			return errorspkg.Validation(errorspkg.Field{Name: "product_id", Message: msgArchived})
		}

		warehouseID := in.WarehouseID
		if warehouseID == "" {
			warehouseID, err = s.products.DefaultWarehouse(spanCtx, tx, subtaskID)
			if err != nil {
				return err
			}
		} else if err := s.checkWarehouse(spanCtx, tx, warehouseID, ""); err != nil {
			return err
		}

		productID, err := s.products.Insert(spanCtx, tx, subtaskID, in.ProductID, in.Qty.String(), warehouseID)
		if err != nil {
			if errors.Is(err, errNotFound) {
				return errorspkg.Validation(errorspkg.Field{Name: "product_id", Message: msgNotFound})
			}
			return err
		}
		span.SetAttributes(attribute.String("product_id", productID))

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

// UpdateSubtaskProduct — PUT /api/v1/subtask-products/:id (LWW, AC-008/AC-010/AC-014):
// полная замена qty/price + Reconcile.
func (s *CardService) UpdateSubtaskProduct(ctx context.Context, a Actor, id string, in SubtaskProductUpdateRequest) (*OrderCardMutationResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrSubtaskProductNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "subtask_products.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("product_id", id), attribute.String("op", "update"))

	var taskID int64
	var reconcile ReconcileResult
	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		tid, subtaskID, err := s.products.ParentOf(spanCtx, tx, id)
		if errors.Is(err, errNotFound) {
			return ErrSubtaskProductNotFound
		}
		if err != nil {
			return err
		}
		taskID = tid
		span.SetAttributes(attribute.Int64("task_id", tid))

		_, col, err := s.lockRowsScopeFor(spanCtx, tx, tid, ErrSubtaskProductNotFound)
		if err != nil {
			return err
		}
		if _, err := s.subtasks.Lock(spanCtx, tx, subtaskID, tid); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskNotFound
			}
			return err
		}
		if err := s.products.Lock(spanCtx, tx, id, subtaskID); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskProductNotFound
			}
			return err
		}

		var priceStr *string
		if in.Price != nil {
			v := in.Price.String()
			priceStr = &v
		}
		curWarehouse, err := s.products.WarehouseOf(spanCtx, tx, id)
		if err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskProductNotFound
			}
			return err
		}
		warehouseID := curWarehouse
		if in.WarehouseID != "" && in.WarehouseID != curWarehouse {
			if err := s.checkWarehouse(spanCtx, tx, in.WarehouseID, curWarehouse); err != nil {
				return err
			}
			warehouseID = in.WarehouseID
		} else {
			// склад строки не меняется: если он мягко удалён, количество можно только уменьшить —
			// увеличение списало бы новый товар с удалённого склада (409 WAREHOUSE_DELETED).
			blocked, err := s.products.QtyIncreaseOnDeletedWarehouse(spanCtx, tx, id, curWarehouse, in.Qty.String())
			if errors.Is(err, errNotFound) {
				return ErrSubtaskProductNotFound
			}
			if err != nil {
				return err
			}
			if blocked {
				return ErrSubtaskWarehouseDeleted
			}
		}
		if err := s.products.Update(spanCtx, tx, id, in.Qty.String(), priceStr, warehouseID); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskProductNotFound
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

// DeleteSubtaskProduct — DELETE /api/v1/subtask-products/:id (AC-008/AC-010): Reconcile
// возвращает списанное на склад.
func (s *CardService) DeleteSubtaskProduct(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrSubtaskProductNotFound
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "subtask_products.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("product_id", id), attribute.String("op", "delete"))

	var taskID int64
	var reconcile ReconcileResult
	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		tid, subtaskID, err := s.products.ParentOf(spanCtx, tx, id)
		if errors.Is(err, errNotFound) {
			return ErrSubtaskProductNotFound
		}
		if err != nil {
			return err
		}
		taskID = tid
		span.SetAttributes(attribute.Int64("task_id", tid))

		_, col, err := s.lockRowsScopeFor(spanCtx, tx, tid, ErrSubtaskProductNotFound)
		if err != nil {
			return err
		}
		if _, err := s.subtasks.Lock(spanCtx, tx, subtaskID, tid); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskNotFound
			}
			return err
		}
		if err := s.products.Lock(spanCtx, tx, id, subtaskID); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskProductNotFound
			}
			return err
		}
		if err := s.products.Delete(spanCtx, tx, id); err != nil {
			if errors.Is(err, errNotFound) {
				return ErrSubtaskProductNotFound
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

// checkWarehouse — склад строки товара существует и активен (архивный — только если это
// текущий склад строки, current). 400 field warehouse_id.
func (s *CardService) checkWarehouse(ctx context.Context, tx pgx.Tx, id, current string) error {
	if id == current {
		return nil
	}
	return s.checkWarehouseField(ctx, tx, "warehouse_id", id)
}

// checkWarehouseField — склад существует и активен, иначе 400 по полю field (общая часть
// checkWarehouse и склада оборудования заказа, DS-008).
func (s *CardService) checkWarehouseField(ctx context.Context, tx pgx.Tx, field, id string) error {
	exists, active, err := s.products.WarehouseState(ctx, tx, id)
	if err != nil {
		return err
	}
	if !exists {
		return errorspkg.Validation(errorspkg.Field{Name: field, Message: msgNotFound})
	}
	if !active {
		return errorspkg.Validation(errorspkg.Field{Name: field, Message: msgArchived})
	}
	return nil
}
