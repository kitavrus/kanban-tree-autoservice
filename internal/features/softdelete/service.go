package softdelete

import (
	"context"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Service — чтение раздела «Удалённые»: список одного вида и счётчики вкладок. Источники
// видов подключаются Register; в части 1 — только заказы.
type Service struct {
	sources map[Kind]Source
}

func NewService() *Service { return &Service{sources: map[Kind]Source{}} }

// Register — подключить источник вида (части 2–6 добавляют свои).
func (s *Service) Register(kind Kind, src Source) *Service {
	s.sources[kind] = src
	return s
}

// List — GET /api/v1/deleted. tz — часовой пояс пользователя (границы дня периода).
func (s *Service) List(ctx context.Context, tz string, q ListQuery) (*ListResponse, error) {
	kind, f, fields := parseQuery(q, tz)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}
	resp := &ListResponse{Items: []Item{}, Limit: f.Limit, Offset: f.Offset}
	src, ok := s.sources[kind]
	if !ok {
		return resp, nil
	}
	items, total, err := src.List(ctx, f)
	if err != nil {
		return nil, err
	}
	if items != nil {
		resp.Items = items
	}
	resp.Total = total
	return resp, nil
}

// Summary — GET /api/v1/deleted/summary: счётчик по каждому виду реестра.
func (s *Service) Summary(ctx context.Context) (*SummaryResponse, error) {
	counts := make(map[string]int, len(Kinds))
	for _, k := range Kinds {
		counts[k.CountKey()] = 0
		src, ok := s.sources[k]
		if !ok {
			continue
		}
		n, err := src.Count(ctx)
		if err != nil {
			return nil, err
		}
		counts[k.CountKey()] = n
	}
	return &SummaryResponse{Counts: counts}, nil
}
