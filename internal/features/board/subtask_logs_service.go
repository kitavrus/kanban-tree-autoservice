package board

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// logAllowed — D27: запись меняет её автор либо admin; иначе 403 SUBTASK_LOG_FORBIDDEN.
// Проверка идёт внутри транзакции записи — после 404 (родитель / строка), до изменения.
func logAllowed(a Actor) func(LogOwner) error {
	return func(o LogOwner) error {
		if !a.IsAdmin && o.UserID != a.ID {
			return ErrSubtaskLogForbidden
		}
		return nil
	}
}

// CreateLog — POST /api/v1/subtasks/:id/logs (AC-012): заказ под FOR UPDATE не берётся
// (журнал склад не трогает, design-sql.md §Lock Order), но блокируется FOR KEY SHARE от
// параллельного мягкого удаления (soft-delete-core DS-026).
func (s *CardService) CreateLog(ctx context.Context, a Actor, subtaskID string, in SubtaskLogRequest) (*OrderCardMutationResponse, error) {
	if uuid.Validate(subtaskID) != nil {
		return nil, ErrSubtaskNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	ctx, span := otel.Tracer("board").Start(ctx, "subtask_logs.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("subtask_id", subtaskID), attribute.String("op", "create"))

	// Заказ услуги блокируется от параллельного мягкого удаления внутри Insert (DS-026).
	taskID, err := s.logs.Insert(ctx, subtaskID, a.ID, in.Text)
	if errors.Is(err, errNotFound) {
		return nil, ErrSubtaskNotFound
	}
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int64("task_id", taskID))
	return s.cardMutation(ctx, a, taskID, ReconcileResult{Warnings: []StockWarning{}})
}

// UpdateLog — PUT /api/v1/subtask-logs/:id (D27, AC-012): 404 (родитель/строка) →
// 403 SUBTASK_LOG_FORBIDDEN (автор ≠ актор И актор не admin) → запись.
func (s *CardService) UpdateLog(ctx context.Context, a Actor, id string, in SubtaskLogRequest) (*OrderCardMutationResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrSubtaskLogNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	ctx, span := otel.Tracer("board").Start(ctx, "subtask_logs.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("op", "update"))

	owner, err := s.logs.Update(ctx, id, in.Text, logAllowed(a))
	if errors.Is(err, errNotFound) {
		return nil, ErrSubtaskLogNotFound
	}
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int64("task_id", owner.TaskID))
	return s.cardMutation(ctx, a, owner.TaskID, ReconcileResult{Warnings: []StockWarning{}})
}

// DeleteLog — DELETE /api/v1/subtask-logs/:id (D27, AC-012).
func (s *CardService) DeleteLog(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrSubtaskLogNotFound
	}

	ctx, span := otel.Tracer("board").Start(ctx, "subtask_logs.mutate")
	defer span.End()
	span.SetAttributes(attribute.String("op", "delete"))

	owner, err := s.logs.Delete(ctx, id, logAllowed(a))
	if errors.Is(err, errNotFound) {
		return nil, ErrSubtaskLogNotFound
	}
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int64("task_id", owner.TaskID))
	return s.cardMutation(ctx, a, owner.TaskID, ReconcileResult{Warnings: []StockWarning{}})
}
