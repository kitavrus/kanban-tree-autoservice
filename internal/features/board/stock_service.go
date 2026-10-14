package board

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// stockScale — NUMERIC(12,3): products.stock (эталон catalog.qtyScale) — отдельная
// декларация в board, не импорт из catalog (фичи не завязаны друг на друга, ADR-101 вынес
// саму логику диапазона в pkg/numeric, здесь только числа колонки).
var stockScale = numeric.Scale{IntDigits: 9, FracDigits: 3}

// StockService — экран «Остатки»: список/summary без транзакции, Adjust — целиком внутри
// StockRepo.Adjust (design-go-layers.md §Service Interface; своей транзакции сервису не
// нужно — в отличие от ColumnService/TaskService, у Stock нет multi-step Lock Order).
type StockService struct {
	repo  *StockRepo
	badge *navBadgeCache
}

func NewStockService(repo *StockRepo) *StockService {
	s := &StockService{repo: repo}
	s.badge = newNavBadgeCache(navBadgeTTL, func(ctx context.Context) (int, error) {
		summary, err := repo.Summary(ctx)
		if err != nil {
			return 0, err
		}
		return summary.RiskCount, nil
	})
	return s
}

// List — GET /api/v1/stock?scope=&q=&limit=&offset= (лимиты/дефолты — stock_handler.go).
func (s *StockService) List(ctx context.Context, p StockListParams) (*StockListResponse, error) {
	items, total, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, err
	}
	return &StockListResponse{Items: items, Total: total, Limit: p.Limit, Offset: p.Offset}, nil
}

// Summary — GET /api/v1/stock/summary.
func (s *StockService) Summary(ctx context.Context) (*StockSummaryResponse, error) {
	return s.repo.Summary(ctx)
}

// Adjust — PUT /api/v1/stock/:product_id (admin, D44).
func (s *StockService) Adjust(ctx context.Context, productID, userID string, in StockAdjustRequest) (*StockRow, error) {
	if uuid.Validate(productID) != nil {
		return nil, ErrStockProductNotFound
	}
	if in.Stock == nil {
		return nil, errorspkg.Validation(errorspkg.Field{Name: "stock", Message: msgRequired})
	}
	if code := in.Stock.Validate(stockScale, true); code != "" { // D44: остаток может быть < 0
		return nil, errorspkg.Validation(errorspkg.Field{Name: "stock", Message: code})
	}

	row, err := s.repo.Adjust(ctx, productID, userID, *in.Stock)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			return nil, ErrStockProductNotFound
		case errors.Is(err, errStockIncrease):
			return nil, ErrStockDeletedWriteOffOnly
		case errors.Is(err, errStockOverflow):
			return nil, errorspkg.Validation(errorspkg.Field{Name: "stock", Message: numeric.MsgTooLarge})
		}
		return nil, err
	}
	return row, nil
}

// InvalidateNavBadges — сбросить кэш бейджа: вызывается после любого успешного изменяющего
// запроса (router.go), остаток/прогноз могли поменяться.
func (s *StockService) InvalidateNavBadges() { s.badge.invalidate() }

// NavBadges — ssr.BadgeSource (ADR-004): бейдж «Остатки» в сайдбаре = risk_count, через
// кэш с TTL (nav_badge_cache.go) — не тяжёлый запрос на каждый SSR-рендер. Ошибка БД не
// должна ломать рендер страницы — предупреждение в лог, бейдж просто не показывается.
func (s *StockService) NavBadges(ctx context.Context) map[string]int {
	risk, err := s.badge.get(ctx)
	if err != nil {
		slog.WarnContext(ctx, "board: NavBadges: не удалось получить stock summary", "err", err)
		return nil
	}
	return map[string]int{"/stock": risk}
}
