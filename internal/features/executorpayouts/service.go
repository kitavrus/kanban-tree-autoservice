package executorpayouts

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// ErrPayoutNotFound — услуги нет либо у неё нет исполнителя: в отчёте «Выплаты» такой
// строки нет (сумму услуги без исполнителя правят в карточке заказа, D72).
var ErrPayoutNotFound = errorspkg.New("EXECUTOR_PAYOUT_NOT_FOUND", http.StatusNotFound,
	"Услуга не найдена среди выплат: она удалена или у неё снят исполнитель")

type Service struct {
	repo *Repo
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{repo: NewRepo(pool)} }

// List — GET /api/v1/executor-payouts: услуги с исполнителем под фильтрами.
func (s *Service) List(ctx context.Context, a Actor, q ListQuery) (*PayoutListResponse, error) {
	f, err := parseFilter(q, a)
	if err != nil {
		return nil, err
	}
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return &PayoutListResponse{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

// Summary — GET /api/v1/executor-payouts/summary: итоги по исполнителям, те же фильтры.
func (s *Service) Summary(ctx context.Context, a Actor, q ListQuery) (*PayoutSummaryResponse, error) {
	f, err := parseFilter(q, a)
	if err != nil {
		return nil, err
	}
	items, total, err := s.repo.Summary(ctx, f)
	if err != nil {
		return nil, err
	}
	return &PayoutSummaryResponse{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

// Mark — POST /api/v1/executor-payouts/mark: вправе и admin, и executor (D62) — гардов по
// роли нет. Ссылки из тела (несуществующая услуга, услуга без исполнителя) — 400 VALIDATION
// по индексу subtask_ids («404 — только id в пути»); вся отметка атомарна.
func (s *Service) Mark(ctx context.Context, in MarkRequest) (*MarkResponse, error) {
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	updated, err := s.repo.Mark(ctx, in.SubtaskIDs, *in.IsExecutorPaid)
	if err != nil {
		var fe *fieldsError
		if errors.As(err, &fe) {
			return nil, errorspkg.Validation(fe.Fields...)
		}
		return nil, err
	}
	return &MarkResponse{Updated: updated}, nil
}

// UpdatePayout — PUT /api/v1/executor-payouts/:id (D72): правка только суммы выплаты; вправе
// и admin, и executor (D62). Битый uuid в пути → 400 по полю id; услуги нет/без исполнителя →
// 404 EXECUTOR_PAYOUT_NOT_FOUND. Флаг, дата отметки, цена, статус и исполнитель не меняются.
func (s *Service) UpdatePayout(ctx context.Context, rawID string, in PayoutUpdateRequest) (*PayoutItem, error) {
	u, err := uuid.Parse(strings.TrimSpace(rawID))
	if err != nil {
		return nil, errorspkg.Validation(errorspkg.Field{Name: "id", Message: msgInvalid})
	}
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	var payout *string
	if in.ExecutorPayout.Value != nil {
		v := in.ExecutorPayout.Value.String()
		payout = &v
	}
	item, err := s.repo.UpdatePayout(ctx, u.String(), payout)
	if errors.Is(err, errNotFound) {
		return nil, ErrPayoutNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}
